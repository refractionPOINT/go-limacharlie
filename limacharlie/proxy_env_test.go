package limacharlie

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// Go reads HTTPS_PROXY / HTTP_PROXY once per process, so these tests re-run
// themselves in a child process with the proxy variables set.
const proxyTestChild = "LC_PROXY_ENV_TEST_CHILD"

// recordingProxy answers every request (plain HTTP proxying and CONNECT) with
// a refusal and records what the client asked it for.
type recordingProxy struct {
	mu   sync.Mutex
	seen []string
}

func (p *recordingProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	p.seen = append(p.seen, r.Method+" "+r.Host)
	p.mu.Unlock()
	http.Error(w, "refused by the test proxy", http.StatusForbidden)
}

func (p *recordingProxy) saw(entry string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.seen {
		if s == entry {
			return true
		}
	}
	return false
}

func runInChildWithProxy(t *testing.T, test string, proxyURL string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+test+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), proxyTestChild+"=1",
		"HTTPS_PROXY="+proxyURL, "HTTP_PROXY="+proxyURL, "NO_PROXY=", "https_proxy=", "http_proxy=", "no_proxy=")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// The REST client sends its calls through HTTP_PROXY / HTTPS_PROXY, as Go's
// default transport does: an SDK user behind a corporate proxy can reach the API.
func TestRESTClientHonoursProxyEnvironment(t *testing.T) {
	if os.Getenv(proxyTestChild) == "1" {
		org, err := NewOrganizationFromClientOptions(ClientOptions{
			OID: "00000000-0000-4000-8000-000000000001",
			JWT: "test-jwt",
			URL: "http://api.proxy-test.example",
		}, &LCLoggerEmpty{})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = org.WhoAmI()
		return
	}
	proxy := &recordingProxy{}
	srv := httptest.NewServer(proxy)
	defer srv.Close()
	out := runInChildWithProxy(t, "TestRESTClientHonoursProxyEnvironment", srv.URL)
	if !proxy.saw("GET api.proxy-test.example") {
		t.Fatalf("the REST call did not go through the proxy (proxy saw %v); child output:\n%s", proxy.seen, out)
	}
}

// Spout's websocket dials through HTTPS_PROXY too (a CONNECT to the stream host).
func TestSpoutHonoursProxyEnvironment(t *testing.T) {
	if os.Getenv(proxyTestChild) == "1" {
		s := &Spout{}
		_, _ = s.connectWebSocket(LiveStreamRequest{})
		return
	}
	proxy := &recordingProxy{}
	srv := httptest.NewServer(proxy)
	defer srv.Close()
	out := runInChildWithProxy(t, "TestSpoutHonoursProxyEnvironment", srv.URL)
	if !proxy.saw("CONNECT stream.limacharlie.io:443") {
		t.Fatalf("the websocket dial did not go through the proxy (proxy saw %v); child output:\n%s", proxy.seen, strings.TrimSpace(out))
	}
}

package limacharlie

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClientAuthTestSuite struct {
	suite.Suite
}

func TestClientAuthSuite(t *testing.T) {
	suite.Run(t, new(ClientAuthTestSuite))
}

// authTestServer is an httptest server standing in for both the REST API and
// the JWT exchange endpoint. apiHandler is invoked for every API path, while
// the JWT exchange lives under /jwt.
type authTestServer struct {
	server *httptest.Server
	// apiCalls counts the requests made to the API endpoint.
	apiCalls int32
	// jwtCalls counts the token exchanges.
	jwtCalls int32
}

func newAuthTestServer(apiHandler func(apiCall int32, w http.ResponseWriter, r *http.Request)) *authTestServer {
	s := &authTestServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/jwt", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.jwtCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jwt":"refreshed-token"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&s.apiCalls, 1)
		apiHandler(n, w, r)
	})
	s.server = httptest.NewServer(mux)
	return s
}

func (s *authTestServer) Close() {
	s.server.Close()
}

func (s *authTestServer) APICalls() int32 {
	return atomic.LoadInt32(&s.apiCalls)
}

func (s *authTestServer) JWTCalls() int32 {
	return atomic.LoadInt32(&s.jwtCalls)
}

// client builds a Client pointed at the test server. A non-empty JWT is set so
// the initial priming refresh does not run and call counts stay meaningful.
func (s *authTestServer) client(apiKey string) *Client {
	return &Client{
		options: ClientOptions{
			OID:    "fba6e992-ce4f-4d9e-99dc-b548f00df7f9",
			APIKey: apiKey,
			JWT:    "initial-token",
		},
		logger:     &LCLoggerEmpty{},
		httpClient: s.server.Client(),
		baseURL:    s.server.URL,
		jwtURL:     s.server.URL + "/jwt",
	}
}

// A request the API keeps refusing with Unauthorized must surface an error to
// the caller, even though the token refresh performed in between succeeds.
func (s *ClientAuthTestSuite) TestUnauthorizedIsReturnedToCaller() {
	srv := newAuthTestServer(func(_ int32, w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	})
	defer srv.Close()

	c := srv.client("843e80c8-e273-4b3e-93bd-41151b4b933a")
	resp := Dict{}
	err := c.reliableRequest(context.Background(), http.MethodPost, "test", makeDefaultRequest(&resp))

	if s.Error(err, "an Unauthorized response must be reported to the caller") {
		s.Contains(err.Error(), "401")
	}
	// The token is refreshed once; a second Unauthorized with a fresh token
	// is definitive, so the request is not hammered further.
	s.EqualValues(2, srv.APICalls())
	s.EqualValues(1, srv.JWTCalls())
}

// The refresh-and-retry path must keep working: an expired token that becomes
// valid after a refresh yields a successful call.
func (s *ClientAuthTestSuite) TestRetrySucceedsAfterTokenRefresh() {
	srv := newAuthTestServer(func(n int32, w http.ResponseWriter, r *http.Request) {
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"expired"}`))
			return
		}
		// The retry must carry the refreshed token.
		if r.Header.Get("Authorization") != "bearer refreshed-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"stale token on retry"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"value":"ok"}`))
	})
	defer srv.Close()

	c := srv.client("843e80c8-e273-4b3e-93bd-41151b4b933a")
	resp := Dict{}
	err := c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&resp))

	s.NoError(err)
	s.Equal("ok", resp["value"])
	s.EqualValues(2, srv.APICalls())
	s.EqualValues(1, srv.JWTCalls())
	s.Equal("refreshed-token", c.options.JWT)
}

// Without an API key there is nothing to refresh, so the Unauthorized error is
// returned immediately.
func (s *ClientAuthTestSuite) TestUnauthorizedWithoutAPIKey() {
	srv := newAuthTestServer(func(_ int32, w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	defer srv.Close()

	c := srv.client("")
	err := c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&Dict{}))

	s.Error(err)
	s.EqualValues(1, srv.APICalls())
	s.EqualValues(0, srv.JWTCalls())
}

// A failing token exchange must be reported rather than swallowed.
func (s *ClientAuthTestSuite) TestFailingTokenRefreshIsReported() {
	srv := &authTestServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/jwt", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&srv.jwtCalls, 1)
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&srv.apiCalls, 1)
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv.server = httptest.NewServer(mux)
	defer srv.Close()

	c := srv.client("843e80c8-e273-4b3e-93bd-41151b4b933a")
	err := c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&Dict{}))

	s.Error(err)
	s.EqualValues(1, srv.APICalls())
	s.EqualValues(1, srv.JWTCalls())
}

// Client errors other than Unauthorized keep being reported as before, and are
// not retried since the outcome cannot change.
func (s *ClientAuthTestSuite) TestClientErrorsAreReportedOnce() {
	for _, status := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound} {
		srv := newAuthTestServer(func(_ int32, w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(`{"error":"nope"}`))
		})

		c := srv.client("843e80c8-e273-4b3e-93bd-41151b4b933a")
		err := c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&Dict{}))

		if s.Error(err, "status %d must be reported", status) {
			s.Contains(err.Error(), "nope")
		}
		s.EqualValues(1, srv.APICalls(), "status %d must not be retried", status)
		s.EqualValues(0, srv.JWTCalls())
		srv.Close()
	}
}

// A successful call is unaffected: no refresh, single request.
func (s *ClientAuthTestSuite) TestSuccessfulRequestUnaffected() {
	srv := newAuthTestServer(func(_ int32, w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "bearer initial-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"value":"ok"}`))
	})
	defer srv.Close()

	c := srv.client("843e80c8-e273-4b3e-93bd-41151b4b933a")
	resp := Dict{}
	s.NoError(c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&resp)))
	s.Equal("ok", resp["value"])
	s.EqualValues(1, srv.APICalls())
	s.EqualValues(0, srv.JWTCalls())
}

// The API path is built from the configured base URL and API version.
func (s *ClientAuthTestSuite) TestUnauthorizedErrorCarriesAPIDetails() {
	srv := newAuthTestServer(func(_ int32, w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/"+currentAPIVersion+"/") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"missing permission"}`))
	})
	defer srv.Close()

	c := srv.client("843e80c8-e273-4b3e-93bd-41151b4b933a")
	err := c.reliableRequest(context.Background(), http.MethodPost, "installationkeys/"+c.options.OID, makeDefaultRequest(&Dict{}))

	if s.Error(err) {
		s.Contains(err.Error(), "missing permission")
	}
}

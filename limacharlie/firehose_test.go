package limacharlie

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

func TestFirehose(t *testing.T) {
	oid := os.Getenv("_OID")
	apiKey := os.Getenv("_KEY")

	o, err := NewOrganization(ClientOptions{
		OID:    oid,
		APIKey: apiKey,
	})
	if err != nil {
		t.Errorf("NewOrganization: %v", err)
	}
	fh, err := NewFirehose(o, FirehoseOptions{
		ListenOnPort: 3000,
		ListenOnIP:   net.ParseIP("127.0.0.1"),
		ParseMessage: true,
	}, &FirehoseOutputOptions{
		Type:              "event",
		IsDeleteOnFailure: true,
	})
	if err != nil {
		t.Errorf("NewFirehose: %v", err)
	}
	if err := fh.Start(); err != nil {
		t.Errorf("Start: %v", err)
	}

	testFeed := []string{
		"{\"a\": 42}",
		"{\"a\": 43}",
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		time.Sleep(2 * time.Second)

		conn, err := getTestFeeder()
		if err != nil {
			t.Errorf("getTestFeeder: %v", err)
			return
		}
		defer conn.Close()

		for _, l := range testFeed {
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte(fmt.Sprintf("%s\n", l))); err != nil {
				t.Errorf("conn.Write: %v", err)
				return
			}
		}

		time.Sleep(1 * time.Second)

		fh.Shutdown()
	}()

	received := []FirehoseMessage{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for m := range fh.Messages {
			received = append(received, m)
		}
	}()

	wg.Wait()

	if len(received) != len(testFeed) {
		t.Errorf("received n: %v", received)
	} else {
		if received[0].RawContent != testFeed[0] || received[1].RawContent != testFeed[1] {
			t.Errorf("wrong received: %v %v", received, testFeed)
		}
		if _, ok := received[0].Content["a"]; !ok {
			t.Errorf("parsed missing: %+v", received[0])
		}
		if _, ok := received[1].Content["a"]; !ok {
			t.Errorf("parsed missing: %+v", received[1])
		}
	}
}

func getTestFeeder() (net.Conn, error) {
	return tls.DialWithDialer(&net.Dialer{
		Timeout: 5 * time.Second,
	}, "tcp", fmt.Sprintf("127.0.0.1:3000"), &tls.Config{
		InsecureSkipVerify: true,
	})
}

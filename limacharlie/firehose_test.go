package limacharlie

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFirehose(t *testing.T) {
	a := assert.New(t)
	o := getTestOrgFromEnv(a)

	fh, err := NewFirehose(o, FirehoseOptions{
		ListenOnPort: 3000,
		ListenOnIP:   net.ParseIP("127.0.0.1"),
		ParseMessage: true,
	}, &FirehoseOutputOptions{
		Type:              "event",
		IsDeleteOnFailure: true,
	})
	a.NoError(err)
	a.NoError(fh.Start())

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
		a.NoError(err)
		defer conn.Close()

		for _, l := range testFeed {
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			_, err := conn.Write([]byte(fmt.Sprintf("%s\n", l)))
			a.NoError(err)
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

	a.Equal(len(received), len(testFeed))
	a.Equal(received[0].RawContent, testFeed[0])
	a.Equal(received[1].RawContent, testFeed[1])
	_, ok := received[0].Content["a"]
	a.True(ok)
	_, ok = received[1].Content["a"]
	a.True(ok)
}

func getTestFeeder() (net.Conn, error) {
	return tls.DialWithDialer(&net.Dialer{
		Timeout: 5 * time.Second,
	}, "tcp", fmt.Sprintf("127.0.0.1:3000"), &tls.Config{
		InsecureSkipVerify: true,
	})
}

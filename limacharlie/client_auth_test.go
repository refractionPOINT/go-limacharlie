package limacharlie

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/suite"
)

const (
	authTestOID = "fba6e992-ce4f-4d9e-99dc-b548f00df7f9"
	// The token minted by the stand-in exchange endpoint.
	authTestRefreshedJWT = "refreshed-token"
)

type ClientAuthTestSuite struct {
	suite.Suite
}

func TestClientAuthSuite(t *testing.T) {
	suite.Run(t, new(ClientAuthTestSuite))
}

// authFixture drives a Client against a MockServer whose API and token-exchange
// routes are replaced by handlers the test controls, counting the calls made to
// each so the retry and refresh behavior can be asserted precisely.
type authFixture struct {
	ms       *MockServer
	c        *Client
	apiCalls int32
	jwtCalls int32
}

// newAuthFixture builds the fixture. The api handler is invoked for every API
// request and receives the 1-based attempt number. The token exchange succeeds
// by default, returning authTestRefreshedJWT; refreshWith overrides it.
func (s *ClientAuthTestSuite) newAuthFixture(api func(attempt int32, w http.ResponseWriter, r *http.Request)) *authFixture {
	f := &authFixture{ms: NewMockServer(authTestOID)}
	f.ms.CustomHandlers["/"+currentAPIVersion+"/"] = func(w http.ResponseWriter, r *http.Request) {
		api(atomic.AddInt32(&f.apiCalls, 1), w, r)
	}
	f.refreshWith(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jwt":%q}`, authTestRefreshedJWT)
	})
	c, err := f.ms.NewClient()
	s.Require().NoError(err)
	// The mock client carries both an API key and a JWT, so the priming
	// refresh does not run and every exchange counted below is one the
	// Unauthorized retry performed.
	f.c = c
	return f
}

// refreshWith replaces the token exchange handler.
func (f *authFixture) refreshWith(h http.HandlerFunc) {
	f.ms.CustomHandlers["/jwt"] = func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&f.jwtCalls, 1)
		h(w, r)
	}
}

func (f *authFixture) Close()          { f.ms.Close() }
func (f *authFixture) APICalls() int32 { return atomic.LoadInt32(&f.apiCalls) }
func (f *authFixture) JWTCalls() int32 { return atomic.LoadInt32(&f.jwtCalls) }

// unauthorized writes an Unauthorized answer shaped like the API gateway's.
func unauthorized(w http.ResponseWriter, detail string) {
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":%q}`, detail)
}

// A request the API keeps refusing with Unauthorized must surface an error to
// the caller, even though the token refresh performed in between succeeds. The
// error must still carry the message the API returned: callers classify
// failures by matching on it.
func (s *ClientAuthTestSuite) TestUnauthorizedIsReturnedToCaller() {
	f := s.newAuthFixture(func(_ int32, w http.ResponseWriter, _ *http.Request) {
		unauthorized(w, "RECORD_NOT_FOUND")
	})
	defer f.Close()

	resp := Dict{}
	err := f.c.reliableRequest(context.Background(), http.MethodPost, "test", makeDefaultRequest(&resp))

	if s.Error(err, "an Unauthorized response must be reported to the caller") {
		s.Contains(err.Error(), "401")
		s.Contains(err.Error(), "RECORD_NOT_FOUND")
	}
	// The token is refreshed once; a second Unauthorized carrying a fresh
	// token is definitive, so the request is not hammered further.
	s.EqualValues(2, f.APICalls())
	s.EqualValues(1, f.JWTCalls())
}

// The refresh-and-retry path must keep working: an expired token that becomes
// valid after a refresh yields a successful call.
func (s *ClientAuthTestSuite) TestRetrySucceedsAfterTokenRefresh() {
	f := s.newAuthFixture(func(attempt int32, w http.ResponseWriter, r *http.Request) {
		if attempt == 1 {
			unauthorized(w, "expired")
			return
		}
		// The retry must carry the refreshed token.
		if r.Header.Get("Authorization") != "bearer "+authTestRefreshedJWT {
			unauthorized(w, "stale token on retry")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"value":"ok"}`))
	})
	defer f.Close()

	resp := Dict{}
	err := f.c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&resp))

	s.NoError(err)
	s.Equal("ok", resp["value"])
	s.EqualValues(2, f.APICalls())
	s.EqualValues(1, f.JWTCalls())
	s.Equal(authTestRefreshedJWT, f.c.options.JWT)
}

// Without an API key there is nothing to exchange, so the Unauthorized error is
// returned immediately.
func (s *ClientAuthTestSuite) TestUnauthorizedWithoutAPIKey() {
	f := s.newAuthFixture(func(_ int32, w http.ResponseWriter, _ *http.Request) {
		unauthorized(w, "no creds")
	})
	defer f.Close()
	f.c.options.APIKey = ""

	err := f.c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&Dict{}))

	s.Error(err)
	s.EqualValues(1, f.APICalls())
	s.EqualValues(0, f.JWTCalls())
}

// A failing token exchange must be reported, and must not bury the error the
// API returned: both texts have to reach the caller.
func (s *ClientAuthTestSuite) TestFailingTokenRefreshReportsBothErrors() {
	f := s.newAuthFixture(func(_ int32, w http.ResponseWriter, _ *http.Request) {
		unauthorized(w, "ETAG_MISMATCH")
	})
	defer f.Close()
	f.refreshWith(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	err := f.c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&Dict{}))

	if s.Error(err) {
		s.Contains(err.Error(), "ETAG_MISMATCH", "the API error must survive the refresh failure")
		s.Contains(err.Error(), "403", "the refresh failure must be reported too")
	}
	s.EqualValues(1, f.APICalls())
	s.EqualValues(1, f.JWTCalls())
}

// A JWT primed at the start of the call is already fresh, so an Unauthorized
// answer to the first attempt is a refusal: it must not trigger a second
// exchange of the token that was just minted.
func (s *ClientAuthTestSuite) TestPrimedTokenIsNotExchangedTwice() {
	f := s.newAuthFixture(func(_ int32, w http.ResponseWriter, _ *http.Request) {
		unauthorized(w, "nope")
	})
	defer f.Close()
	// No JWT yet: the client primes one before the first attempt.
	f.c.options.JWT = ""

	err := f.c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&Dict{}))

	s.Error(err)
	s.EqualValues(1, f.APICalls())
	s.EqualValues(1, f.JWTCalls(), "only the priming exchange should happen")
}

// Requests that override the Authorization header (the ai-sessions routes send
// a raw API key) do not use the client JWT at all, so refreshing it cannot
// change an Unauthorized answer and must not be attempted.
func (s *ClientAuthTestSuite) TestRequestAuthorizationOverrideSkipsRefresh() {
	const override = "Bearer raw-api-key"
	seen := atomic.Value{}
	f := s.newAuthFixture(func(_ int32, w http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Get("Authorization"))
		unauthorized(w, "denied")
	})
	defer f.Close()

	req := makeDefaultRequest(&Dict{}).withExtraHeaders(map[string]string{"Authorization": override})
	err := f.c.reliableRequest(context.Background(), http.MethodGet, "test", req)

	s.Error(err)
	s.Equal(override, seen.Load(), "the request's own Authorization header must be the one sent")
	s.EqualValues(1, f.APICalls())
	s.EqualValues(0, f.JWTCalls())
}

// Client errors other than Unauthorized are reported and not retried, since
// re-issuing the identical request cannot change the outcome.
func (s *ClientAuthTestSuite) TestClientErrorsAreReportedOnce() {
	for _, status := range []int{http.StatusBadRequest, http.StatusForbidden, http.StatusNotFound} {
		func() {
			f := s.newAuthFixture(func(_ int32, w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				w.Write([]byte(`{"error":"nope"}`))
			})
			defer f.Close()

			err := f.c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&Dict{}))

			if s.Error(err, "status %d must be reported", status) {
				s.Contains(err.Error(), "nope")
			}
			s.EqualValues(1, f.APICalls(), "status %d must not be retried", status)
			s.EqualValues(0, f.JWTCalls())
		}()
	}
}

// Server-side errors keep their retry behavior.
func (s *ClientAuthTestSuite) TestServerErrorsAreRetried() {
	f := s.newAuthFixture(func(attempt int32, w http.ResponseWriter, _ *http.Request) {
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"value":"ok"}`))
	})
	defer f.Close()

	resp := Dict{}
	s.NoError(f.c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&resp)))
	s.Equal("ok", resp["value"])
	s.EqualValues(3, f.APICalls())
	s.EqualValues(0, f.JWTCalls())
}

// A successful call is unaffected: no refresh, single request.
func (s *ClientAuthTestSuite) TestSuccessfulRequestUnaffected() {
	f := s.newAuthFixture(func(_ int32, w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			unauthorized(w, "missing token")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"value":"ok"}`))
	})
	defer f.Close()

	resp := Dict{}
	s.NoError(f.c.reliableRequest(context.Background(), http.MethodGet, "test", makeDefaultRequest(&resp)))
	s.Equal("ok", resp["value"])
	s.EqualValues(1, f.APICalls())
	s.EqualValues(0, f.JWTCalls())
}

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testToken = "test-token"
	testPort  = 5180
)

type guardRequest struct {
	target string
	host   string
	origin string
	cookie *http.Cookie
}

func serveGuarded(t *testing.T, request guardRequest) *httptest.ResponseRecorder {
	t.Helper()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, request.target, nil)

	req.Host = request.host
	if req.Host == "" {
		req.Host = "127.0.0.1:5180"
	}

	if request.origin != "" {
		req.Header.Set("Origin", request.origin)
	}

	if request.cookie != nil {
		req.AddCookie(request.cookie)
	}

	recorder := httptest.NewRecorder()
	NewGuard(testToken, testPort).Wrap(next).ServeHTTP(recorder, req)

	return recorder
}

func sessionCookie(t *testing.T) *http.Cookie {
	t.Helper()

	response := serveGuarded(t, guardRequest{target: "/?token=" + testToken})
	cookies := response.Result().Cookies()
	require.Len(t, cookies, 1)

	return cookies[0]
}

func TestNewTokenHas256RandomBits(t *testing.T) {
	t.Parallel()

	first, second := NewToken(), NewToken()

	assert.Len(t, first, 43)
	assert.NotEqual(t, first, second)
}

func TestTokenIsExchangedForASessionCookie(t *testing.T) {
	t.Parallel()

	response := serveGuarded(t, guardRequest{target: "/?token=" + testToken})

	assert.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, "/", response.Header().Get("Location"))

	cookies := response.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)
	assert.Equal(t, "/", cookies[0].Path)
}

func TestRedirectKeepsPathAndOtherParameters(t *testing.T) {
	t.Parallel()

	response := serveGuarded(t, guardRequest{target: "/overview?token=" + testToken + "&x=1"})

	assert.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, "/overview?x=1", response.Header().Get("Location"))
}

func TestRedirectStaysOnTheServer(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"//attacker.example/", "/\\attacker.example/"} {
		response := serveGuarded(t, guardRequest{target: path + "?token=" + testToken})

		assert.Equal(t, "/attacker.example/", response.Header().Get("Location"))
	}
}

func TestWrongTokenIsUnauthorized(t *testing.T) {
	t.Parallel()

	response := serveGuarded(t, guardRequest{target: "/?token=wrong"})

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Empty(t, response.Result().Cookies())
}

func TestAPIRequiresTheSessionCookie(t *testing.T) {
	t.Parallel()

	for name, cookie := range map[string]*http.Cookie{
		"no cookie":    nil,
		"wrong value":  {Name: "exasol_launcher_ui_session_5180", Value: "earlier-token"},
		"another port": {Name: "exasol_launcher_ui_session_6000", Value: testToken},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := serveGuarded(t, guardRequest{target: "/api/v1/deployments", cookie: cookie})

			assert.Equal(t, http.StatusUnauthorized, response.Code)
			assert.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
		})
	}
}

func TestAPIAcceptsTheSessionCookie(t *testing.T) {
	t.Parallel()

	response := serveGuarded(t, guardRequest{
		target: "/api/v1/deployments",
		cookie: sessionCookie(t),
	})

	assert.Equal(t, http.StatusTeapot, response.Code)
}

func TestApplicationPagesNeedNoSession(t *testing.T) {
	t.Parallel()

	response := serveGuarded(t, guardRequest{target: "/overview"})

	assert.Equal(t, http.StatusTeapot, response.Code)
}

func TestHostMustAddressTheServer(t *testing.T) {
	t.Parallel()

	for host, expected := range map[string]int{
		"127.0.0.1:5180":        http.StatusTeapot,
		"localhost:5180":        http.StatusTeapot,
		"attacker.example:5180": http.StatusForbidden,
		"127.0.0.1:6000":        http.StatusForbidden,
		"127.0.0.1":             http.StatusForbidden,
	} {
		t.Run(host, func(t *testing.T) {
			t.Parallel()

			response := serveGuarded(t, guardRequest{target: "/", host: host})

			assert.Equal(t, expected, response.Code)
		})
	}
}

func TestOriginMustBeTheServer(t *testing.T) {
	t.Parallel()

	for origin, expected := range map[string]int{
		"http://127.0.0.1:5180":    http.StatusTeapot,
		"http://localhost:5180":    http.StatusTeapot,
		"https://attacker.example": http.StatusForbidden,
		"http://127.0.0.1:6000":    http.StatusForbidden,
		"null":                     http.StatusForbidden,
	} {
		t.Run(origin, func(t *testing.T) {
			t.Parallel()

			response := serveGuarded(t, guardRequest{
				target: "/api/v1/deployments",
				origin: origin,
				cookie: sessionCookie(t),
			})

			assert.Equal(t, expected, response.Code)
		})
	}
}

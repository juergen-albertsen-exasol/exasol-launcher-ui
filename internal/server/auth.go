package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	tokenBytes     = 32
	tokenParameter = "token"
	apiPathPrefix  = "/api/"

	reconnectHint = "Open the access URL that exasol-local-ui printed when it started."
)

// NewToken returns a random 256-bit session token.
func NewToken() string {
	token := make([]byte, tokenBytes)
	_, _ = rand.Read(token)

	return base64.RawURLEncoding.EncodeToString(token)
}

// Guard admits requests that address the server itself, exchanges the access
// URL's token for a session cookie, and requires that cookie for the API.
type Guard struct {
	token          string
	cookieName     string
	allowedHosts   map[string]bool
	allowedOrigins map[string]bool
}

func NewGuard(token string, port int) *Guard {
	portSuffix := ":" + strconv.Itoa(port)

	return &Guard{
		token: token,
		// Cookies are not scoped by port, so servers on different ports need
		// distinct cookie names to keep their sessions apart.
		cookieName: "exasol_local_ui_session_" + strconv.Itoa(port),
		allowedHosts: map[string]bool{
			LoopbackAddress + portSuffix: true,
			"localhost" + portSuffix:     true,
		},
		allowedOrigins: map[string]bool{
			"http://" + LoopbackAddress + portSuffix: true,
			"http://localhost" + portSuffix:          true,
		},
	}
}

func (g *Guard) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !g.allowedHosts[request.Host] {
			writeProblem(response, http.StatusForbidden, "The request must address this server.")

			return
		}

		origin := request.Header.Get("Origin")
		if origin != "" && !g.allowedOrigins[origin] {
			writeProblem(response, http.StatusForbidden, "Cross-origin requests are not allowed.")

			return
		}

		query := request.URL.Query()
		if query.Has(tokenParameter) {
			g.exchangeToken(response, request, query.Get(tokenParameter))

			return
		}

		if strings.HasPrefix(request.URL.Path, apiPathPrefix) && !g.hasSession(request) {
			writeProblem(response, http.StatusUnauthorized, reconnectHint)

			return
		}

		next.ServeHTTP(response, request)
	})
}

func (g *Guard) exchangeToken(response http.ResponseWriter, request *http.Request, token string) {
	if !g.matches(token) {
		writeProblem(response, http.StatusUnauthorized, reconnectHint)

		return
	}

	//nolint:gosec // The server speaks plain HTTP on loopback, so the cookie cannot be Secure.
	http.SetCookie(response, &http.Cookie{
		Name:     g.cookieName,
		Value:    g.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	query := request.URL.Query()
	query.Del(tokenParameter)

	// A path starting with "//" or "/\\" would redirect to another host.
	target := url.URL{
		Path:     "/" + strings.TrimLeft(request.URL.Path, "/\\"),
		RawQuery: query.Encode(),
	}

	//nolint:gosec // The target is a path on this server.
	http.Redirect(response, request, target.RequestURI(), http.StatusSeeOther)
}

func (g *Guard) hasSession(r *http.Request) bool {
	cookie, err := r.Cookie(g.cookieName)

	return err == nil && g.matches(cookie.Value)
}

func (g *Guard) matches(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(g.token)) == 1
}

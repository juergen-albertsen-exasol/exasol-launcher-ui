package server

import (
	"net/http"

	"github.com/exasol/exasol-local-ui/internal/launcher"
)

// NewHandler returns the handler for all requests the server answers.
func NewHandler(launcherClient *launcher.Client) http.Handler {
	mux := http.NewServeMux()
	(&api{launcher: launcherClient}).register(mux)
	mux.HandleFunc("GET /{$}", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = response.Write([]byte("Exasol Local UI\n"))
	})

	return mux
}

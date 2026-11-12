package server

import (
	"net/http"
)

// NewHandler returns the handler for all requests the server answers.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Exasol Local UI\n"))
	})

	return mux
}

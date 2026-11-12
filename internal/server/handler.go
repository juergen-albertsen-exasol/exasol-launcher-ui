package server

import (
	"context"
	"net/http"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/launcher"
)

// Config holds what the handler needs to answer requests.
type Config struct {
	Launcher *launcher.Client
	// Ping defaults to database.Ping.
	Ping func(ctx context.Context, target database.Target) (time.Duration, error)
}

// NewHandler returns the handler for all requests the server answers.
func NewHandler(config Config) http.Handler {
	if config.Ping == nil {
		config.Ping = database.Ping
	}

	mux := http.NewServeMux()
	(&api{launcher: config.Launcher, ping: config.Ping}).register(mux)
	mux.HandleFunc("GET /{$}", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = response.Write([]byte("Exasol Local UI\n"))
	})

	return mux
}

package server

import (
	"context"
	"io/fs"
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
	// WebApp holds the compiled browser application.
	WebApp fs.FS
}

// NewHandler returns the handler for all requests the server answers.
func NewHandler(config Config) http.Handler {
	if config.Ping == nil {
		config.Ping = database.Ping
	}

	mux := http.NewServeMux()
	(&api{launcher: config.Launcher, ping: config.Ping}).register(mux)
	mux.Handle("/", newWebApp(config.WebApp))

	return mux
}

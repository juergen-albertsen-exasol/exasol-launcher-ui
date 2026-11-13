package server

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/opener"
	"github.com/exasol/exasol-local-ui/internal/operations"
)

// Config holds what the handler needs to answer requests.
type Config struct {
	Launcher *launcher.Client
	// Ping defaults to database.Ping.
	Ping func(ctx context.Context, target database.Target) (time.Duration, error)
	// Operations runs long launcher commands.
	Operations *operations.Manager
	// OpenFolder defaults to opener.Open.
	OpenFolder func(ctx context.Context, path string) error
	// WebApp holds the compiled browser application.
	WebApp fs.FS
}

// NewHandler returns the handler for all requests the server answers.
func NewHandler(config Config) http.Handler {
	if config.Ping == nil {
		config.Ping = database.Ping
	}

	if config.OpenFolder == nil {
		config.OpenFolder = opener.Open
	}

	mux := http.NewServeMux()
	(&api{
		launcher:   config.Launcher,
		ping:       config.Ping,
		openFolder: config.OpenFolder,
		operations: config.Operations,
	}).register(mux)
	mux.Handle("/", newWebApp(config.WebApp))

	return mux
}

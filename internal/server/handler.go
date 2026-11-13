package server

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/imports"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/opener"
	"github.com/exasol/exasol-local-ui/internal/operations"
	"github.com/exasol/exasol-local-ui/internal/sql"
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
	// SQL holds the SQL sessions; it defaults to a manager that connects
	// through the driver.
	SQL *sql.Manager
	// Uploads keeps uploaded files; it defaults to the user's temporary
	// folder.
	Uploads *imports.Uploads
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

	if config.SQL == nil {
		config.SQL = sql.NewManager(sql.NewDriverOpener())
	}

	if config.Uploads == nil {
		config.Uploads = imports.NewUploads(imports.DefaultUploadsDir())
	}

	mux := http.NewServeMux()
	(&api{
		launcher:   config.Launcher,
		ping:       config.Ping,
		openFolder: config.OpenFolder,
		operations: config.Operations,
		sql:        config.SQL,
		uploads:    config.Uploads,
	}).register(mux)
	mux.Handle("/", newWebApp(config.WebApp))

	return mux
}

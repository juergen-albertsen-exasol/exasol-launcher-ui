package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"
)

const (
	LoopbackAddress = "127.0.0.1"
	DefaultPort     = 5180

	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

// Listen binds the loopback address only, because the API acts with the
// deployment's administrative database credentials.
func Listen(ctx context.Context, port int) (net.Listener, error) {
	address := net.JoinHostPort(LoopbackAddress, strconv.Itoa(port))

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if errors.Is(err, syscall.EADDRINUSE) {
		return nil, fmt.Errorf("port %d is already in use by another process", port)
	}

	if err != nil {
		return nil, fmt.Errorf("cannot listen on port %d: %w", port, err)
	}

	return listener, nil
}

// Serve handles requests on listener until ctx is cancelled.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler) error {
	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		// Requests end with the server, which ends open event streams.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	serveErr := make(chan error, 1)

	go func() {
		serveErr <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("cannot shut down the server: %w", err)
	}

	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/exasol/exasol-local-ui/internal/browser"
	"github.com/exasol/exasol-local-ui/internal/server"
)

const usageExitCode = 2

type options struct {
	port int
	open bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := run(ctx, os.Args[1:], os.Stdout, os.Stderr)

	stop()
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}

	if err != nil {
		return usageExitCode
	}

	if err := serve(ctx, opts, stdout, stderr); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)

		return 1
	}

	return 0
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	var opts options

	flags := flag.NewFlagSet("exasol-local-ui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.IntVar(&opts.port, "port", server.DefaultPort, "port to serve on, on 127.0.0.1")
	flags.BoolVar(&opts.open, "open", false, "open the access URL in the default browser")

	if err := flags.Parse(args); err != nil {
		return opts, err
	}

	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))

		return opts, errors.New("unexpected argument")
	}

	return opts, nil
}

func serve(ctx context.Context, opts options, stdout, stderr io.Writer) error {
	listener, err := server.Listen(ctx, opts.port)
	if err != nil {
		return err
	}

	accessURL := (&url.URL{Scheme: "http", Host: listener.Addr().String(), Path: "/"}).String()
	_, _ = fmt.Fprintf(stdout, "Exasol Local UI is available at %s\n", accessURL)

	if opts.open {
		if err := browser.Open(ctx, accessURL); err != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: %v\n", err)
		}
	}

	return server.Serve(ctx, listener, server.NewHandler())
}

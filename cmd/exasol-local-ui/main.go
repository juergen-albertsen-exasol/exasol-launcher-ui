package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/exasol/exasol-local-ui/internal/browser"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/server"
)

const usageExitCode = 2

type options struct {
	port   int
	open   bool
	exasol string
}

type environment struct {
	stdout      io.Writer
	stderr      io.Writer
	openBrowser func(ctx context.Context, url string) error
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := run(ctx, os.Args[1:], environment{
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		openBrowser: browser.Open,
	})

	stop()
	os.Exit(exitCode)
}

func run(ctx context.Context, args []string, env environment) int {
	opts, err := parseOptions(args, env.stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}

	if err != nil {
		return usageExitCode
	}

	if err := serve(ctx, opts, env); err != nil {
		_, _ = fmt.Fprintf(env.stderr, "Error: %v\n", err)

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
	flags.StringVar(&opts.exasol, "exasol", launcher.DefaultExecutable,
		"launcher executable, looked up on PATH unless it is a path")

	if err := flags.Parse(args); err != nil {
		return opts, err
	}

	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))

		return opts, errors.New("unexpected argument")
	}

	return opts, nil
}

func serve(ctx context.Context, opts options, env environment) error {
	launcherClient := launcher.New(opts.exasol)
	if _, err := launcherClient.Check(ctx); err != nil {
		return err
	}

	listener, err := server.Listen(ctx, opts.port)
	if err != nil {
		return err
	}

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("unexpected listener address %s", listener.Addr())
	}

	token := server.NewToken()
	accessURL := (&url.URL{
		Scheme:   "http",
		Host:     address.String(),
		Path:     "/",
		RawQuery: url.Values{"token": {token}}.Encode(),
	}).String()
	handler := server.NewGuard(token, address.Port).Wrap(server.NewHandler(launcherClient))

	_, _ = fmt.Fprintf(env.stdout, "Exasol Local UI is available at %s\n", accessURL)

	if opts.open {
		if err := env.openBrowser(ctx, accessURL); err != nil {
			_, _ = fmt.Fprintf(env.stderr, "Warning: %v\n", err)
		}
	}

	return server.Serve(ctx, listener, handler)
}

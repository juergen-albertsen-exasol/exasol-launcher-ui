package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var accessURLPattern = regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/\S*`)

type startedServer struct {
	accessURL string
	port      int
	stop      func() int
}

func startServer(t *testing.T, args ...string) startedServer {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	stdoutReader, stdoutWriter := io.Pipe()
	exitCode := make(chan int, 1)

	go func() {
		exitCode <- run(ctx, args, stdoutWriter, io.Discard)
		_ = stdoutWriter.Close()
	}()

	line, err := bufio.NewReader(stdoutReader).ReadString('\n')
	require.NoError(t, err)

	go func() { _, _ = io.Copy(io.Discard, stdoutReader) }()

	match := accessURLPattern.FindStringSubmatch(line)
	require.NotNil(t, match, "no access URL in %q", line)

	port, err := strconv.Atoi(match[1])
	require.NoError(t, err)

	stopped := false
	stop := func() int {
		if stopped {
			return 0
		}

		stopped = true

		cancel()

		select {
		case code := <-exitCode:
			return code
		case <-time.After(10 * time.Second):
			t.Fatal("server did not stop")

			return -1
		}
	}
	t.Cleanup(func() { stop() })

	return startedServer{accessURL: match[0], port: port, stop: stop}
}

func listenOnFreePort(t *testing.T) (net.Listener, int) {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	address, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	return listener, address.Port
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, port := listenOnFreePort(t)
	require.NoError(t, listener.Close())

	return port
}

func getStatus(t *testing.T, address string) int {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
	require.NoError(t, err)

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	return response.StatusCode
}

func TestServesOnTheChosenPortUntilInterrupted(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	started := startServer(t, "--port", strconv.Itoa(port))

	assert.Equal(t, port, started.port)

	assert.Equal(t, http.StatusOK, getStatus(t, started.accessURL))

	assert.Equal(t, 0, started.stop())
}

func TestOccupiedPortFailsStartup(t *testing.T) {
	t.Parallel()

	listener, occupiedPort := listenOnFreePort(t)
	defer listener.Close()

	port := strconv.Itoa(occupiedPort)

	var stderr bytes.Buffer

	exitCode := run(t.Context(), []string{"--port", port}, io.Discard, &stderr)

	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr.String(), port)
}

func TestUnknownFlagFailsStartup(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t, 0, run(t.Context(), []string{"--bogus"}, io.Discard, io.Discard))
}

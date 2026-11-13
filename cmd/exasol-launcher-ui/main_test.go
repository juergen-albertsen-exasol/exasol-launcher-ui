package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var accessURLPattern = regexp.MustCompile(
	`http://127\.0\.0\.1:(\d+)/\S*\?token=([A-Za-z0-9_-]+)`)

type startedServer struct {
	accessURL string
	port      int
	stop      func() int
}

func startServer(t *testing.T, env environment, args ...string) startedServer {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	stdoutReader, stdoutWriter := io.Pipe()
	exitCode := make(chan int, 1)

	env.stdout = stdoutWriter
	if env.stderr == nil {
		env.stderr = io.Discard
	}

	if env.openBrowser == nil {
		env.openBrowser = func(context.Context, string) error {
			t.Error("unexpected browser start")

			return nil
		}
	}

	args = append([]string{"--exasol", launchertest.NewFake(t).Executable}, args...)

	go func() {
		exitCode <- run(ctx, args, env)
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

func noRedirectClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func get(t *testing.T, address string, cookies ...*http.Cookie) (int, http.Header, []*http.Cookie) {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
	require.NoError(t, err)

	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}

	response, err := noRedirectClient().Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	return response.StatusCode, response.Header, response.Cookies()
}

func TestDefaultPortIs5180(t *testing.T) {
	t.Parallel()

	opts, err := parseOptions(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, 5180, opts.port)
	assert.False(t, opts.open)
}

func TestServesOnTheChosenPortUntilInterrupted(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	started := startServer(t, environment{}, "--port", strconv.Itoa(port))

	assert.Equal(t, port, started.port)

	status, _, _ := get(t, started.accessURL)
	assert.Equal(t, http.StatusSeeOther, status)

	assert.Equal(t, 0, started.stop())
}

func TestOpenStartsTheBrowserWithTheAccessURL(t *testing.T) {
	t.Parallel()

	opened := make(chan string, 1)
	started := startServer(t, environment{
		openBrowser: func(_ context.Context, url string) error {
			opened <- url

			return nil
		},
	}, "--port", "0", "--open")

	assert.Equal(t, started.accessURL, <-opened)
}

func TestAccessURLEstablishesASession(t *testing.T) {
	t.Parallel()

	started := startServer(t, environment{}, "--port", "0")
	base := "http://127.0.0.1:" + strconv.Itoa(started.port)

	status, header, cookies := get(t, started.accessURL)

	assert.Equal(t, http.StatusSeeOther, status)
	assert.Equal(t, "/", header.Get("Location"))
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, cookies[0].SameSite)

	unauthorized, _, _ := get(t, base+"/api/v1/anything")
	assert.Equal(t, http.StatusUnauthorized, unauthorized)

	authorized, _, _ := get(t, base+"/api/v1/anything", cookies[0])
	assert.NotEqual(t, http.StatusUnauthorized, authorized)
}

func TestCookieFromAnEarlierStartIsUnauthorized(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	first := startServer(t, environment{}, "--port", strconv.Itoa(port))
	_, _, cookies := get(t, first.accessURL)
	require.Len(t, cookies, 1)
	require.Equal(t, 0, first.stop())

	second := startServer(t, environment{}, "--port", strconv.Itoa(port))
	require.NotEqual(t, first.accessURL, second.accessURL)

	status, _, _ := get(t, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/v1/anything", cookies[0])
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestAccessURLCarriesAToken(t *testing.T) {
	t.Parallel()

	started := startServer(t, environment{}, "--port", "0")

	parsed, err := url.Parse(started.accessURL)
	require.NoError(t, err)
	assert.Equal(t, "/", parsed.Path)
	assert.GreaterOrEqual(t, len(parsed.Query().Get("token")), 22)
}

func TestOccupiedPortFailsStartup(t *testing.T) {
	t.Parallel()

	listener, occupiedPort := listenOnFreePort(t)
	defer listener.Close()

	port := strconv.Itoa(occupiedPort)

	var stderr bytes.Buffer

	exitCode := run(t.Context(), []string{
		"--port", port, "--exasol", launchertest.NewFake(t).Executable,
	}, environment{
		stdout: io.Discard,
		stderr: &stderr,
	})

	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr.String(), port)
}

func TestMissingLauncherFailsStartup(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "exasol")

	var stderr bytes.Buffer

	exitCode := run(t.Context(), []string{"--port", "0", "--exasol", missing}, environment{
		stdout: io.Discard,
		stderr: &stderr,
	})

	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr.String(), missing)
}

func TestLauncherIsLookedUpOnPath(t *testing.T) {
	t.Parallel()

	opts, err := parseOptions(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, "exasol", opts.exasol)
}

func TestWebDirIsServed(t *testing.T) {
	t.Parallel()

	webDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(webDir, "index.html"), []byte("dev"), 0o600))

	started := startServer(t, environment{}, "--port", "0", "--web-dir", webDir)

	status, _, _ := get(t, "http://127.0.0.1:"+strconv.Itoa(started.port)+"/overview")
	assert.Equal(t, http.StatusOK, status)
}

func TestMissingWebDirFailsStartup(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "web")

	var stderr bytes.Buffer

	exitCode := run(t.Context(), []string{
		"--port", "0", "--web-dir", missing, "--exasol", launchertest.NewFake(t).Executable,
	}, environment{stdout: io.Discard, stderr: &stderr})

	assert.NotEqual(t, 0, exitCode)
	assert.Contains(t, stderr.String(), missing)
}

func TestDeploymentFlagOpensItsOverview(t *testing.T) {
	t.Parallel()

	started := startServer(t, environment{}, "--port", "0", "--deployment", "my test")

	parsed, err := url.Parse(started.accessURL)
	require.NoError(t, err)
	assert.Equal(t, "/deployments/my test", parsed.Path)

	status, header, _ := get(t, started.accessURL)
	assert.Equal(t, http.StatusSeeOther, status)
	assert.Equal(t, "/deployments/my%20test", header.Get("Location"))
}

func TestUnknownFlagFailsStartup(t *testing.T) {
	t.Parallel()

	exitCode := run(t.Context(), []string{"--bogus"}, environment{
		stdout: io.Discard,
		stderr: io.Discard,
	})

	assert.NotEqual(t, 0, exitCode)
}

func TestVersionPrintsDevForSourceBuilds(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	exitCode := run(t.Context(), []string{
		"--version", "--exasol", filepath.Join(t.TempDir(), "missing"),
	}, environment{stdout: &stdout, stderr: io.Discard})

	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "exasol-launcher-ui dev\n", stdout.String())
}

func TestVersionPrintsTheReleaseVersion(t *testing.T) {
	t.Parallel()

	binary := filepath.Join(t.TempDir(), "exasol-launcher-ui")
	build := exec.CommandContext(t.Context(), "go", "build",
		"-ldflags", "-X main.version=0.1.0", "-o", binary, ".")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))

	output, err = exec.CommandContext(t.Context(), binary, "--version").Output()

	require.NoError(t, err)
	assert.Equal(t, "exasol-launcher-ui 0.1.0\n", string(output))
}

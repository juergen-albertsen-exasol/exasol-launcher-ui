package launcher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultExecutable = "exasol"

	listingCacheTTL = 10 * time.Second

	scanBufferSize = 64 * 1024
	maxLineSize    = 1024 * 1024
)

// Deployment is one entry of `exasol deployments list --json`.
type Deployment struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Status         string `json:"status"`
	Infrastructure string `json:"infrastructure"`
	Installation   string `json:"installation"`
}

// Info is the part of `exasol info --json` that the UI uses.
type Info struct {
	DeploymentState string      `json:"deploymentState"`
	Connection      *Connection `json:"connection"`
}

// Connection holds the SQL connection details that `exasol info --json`
// reports for deployments whose connection details are stable.
type Connection struct {
	Host                       string `json:"host"`
	Port                       int    `json:"dbPort"`
	Username                   string `json:"username"`
	InsecureSkipCertValidation bool   `json:"insecureSkipCertValidation"`
}

// LifecycleResult is the JSON document `exasol start --json` and
// `exasol stop --json` print when they finish.
type LifecycleResult struct {
	DeploymentState string `json:"deploymentState"`
	DatabaseReady   bool   `json:"databaseReady"`
}

// Client runs the launcher's JSON CLI.
type Client struct {
	executable string
	now        func() time.Time
	version    atomic.Pointer[string]

	mu       sync.Mutex
	listing  []Deployment
	listedAt time.Time
}

func New(executable string) *Client {
	return &Client{executable: executable, now: time.Now}
}

// Executable returns the launcher executable the client runs.
func (c *Client) Executable() string {
	return c.executable
}

// Check verifies that the launcher can be run and returns its version.
func (c *Client) Check(ctx context.Context) (string, error) {
	var output struct {
		Version string `json:"version"`
	}

	if err := c.runJSON(ctx, &output, "version", "--json"); err != nil {
		return "", err
	}

	c.version.Store(&output.Version)

	return output.Version, nil
}

// Deployments returns the launcher's deployments in listing order. The
// listing takes the launcher up to five seconds, so it is cached briefly
// unless refresh is set.
func (c *Client) Deployments(ctx context.Context, refresh bool) ([]Deployment, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !refresh && c.listing != nil && c.now().Sub(c.listedAt) < listingCacheTTL {
		return c.listing, nil
	}

	var listing []Deployment
	if err := c.runJSON(ctx, &listing, "deployments", "list", "--json"); err != nil {
		return nil, err
	}

	if listing == nil {
		listing = []Deployment{}
	}

	c.listing = listing
	c.listedAt = c.now()

	return listing, nil
}

// InvalidateListing makes the next Deployments call ask the launcher again.
func (c *Client) InvalidateListing() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.listing = nil
}

// Deployment returns the listed deployment with the given name.
func (c *Client) Deployment(ctx context.Context, name string) (Deployment, bool, error) {
	deployments, err := c.Deployments(ctx, false)
	if err != nil {
		return Deployment{}, false, err
	}

	for _, deployment := range deployments {
		if deployment.Name == name {
			return deployment, true, nil
		}
	}

	return Deployment{}, false, nil
}

// Info describes the deployment in deploymentDir.
func (c *Client) Info(ctx context.Context, deploymentDir string) (Info, error) {
	var info Info

	err := c.runJSON(ctx, &info, "info", "--json", "--deployment-dir", deploymentDir)

	return info, err
}

// Stream runs a long launcher command, calls onLine for each line it writes
// to stderr, and decodes its stdout as JSON into target unless target is nil.
// A failed command's error carries the last line it wrote to stderr.
func (c *Client) Stream(
	ctx context.Context,
	onLine func(line string),
	target any,
	args ...string,
) error {
	command := c.executable + " " + strings.Join(args, " ")

	var stdout bytes.Buffer

	cmd := exec.CommandContext(ctx, c.executable, args...)
	cmd.Stdout = &stdout

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot read the output of %q: %w", command, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot run the launcher %q: %w", c.executable, err)
	}

	lastLine := ""
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, scanBufferSize), maxLineSize)

	for scanner.Scan() {
		line := readableLogLine(strings.TrimRight(scanner.Text(), "\r"))
		if strings.TrimSpace(line) != "" {
			lastLine = line
		}

		onLine(line)
	}

	if err := cmd.Wait(); err != nil {
		if lastLine != "" {
			return fmt.Errorf("%q failed: %s", command, strings.TrimSpace(lastLine))
		}

		return fmt.Errorf("%q failed: %w", command, err)
	}

	if target == nil {
		return nil
	}

	if err := json.Unmarshal(stdout.Bytes(), target); err != nil {
		return fmt.Errorf("cannot decode the output of %q%s: %w", command, c.versionSuffix(), err)
	}

	return nil
}

func (c *Client) runJSON(ctx context.Context, target any, args ...string) error {
	command := c.executable + " " + strings.Join(args, " ")

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, c.executable, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if _, isExit := errors.AsType[*exec.ExitError](err); !isExit {
			return fmt.Errorf("cannot run the launcher %q: %w", c.executable, err)
		}

		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("%q failed: %s", command, message)
		}

		return fmt.Errorf("%q failed: %w", command, err)
	}

	if err := json.Unmarshal(stdout.Bytes(), target); err != nil {
		return fmt.Errorf("cannot decode the output of %q%s: %w", command, c.versionSuffix(), err)
	}

	return nil
}

// readableLogLine turns a JSON log record, which the launcher writes when its
// stderr is no terminal, into "message key=value", prefixed with the level
// unless it is INFO. Other lines pass unchanged.
func readableLogLine(line string) string {
	if !strings.HasPrefix(line, "{") {
		return line
	}

	var record map[string]any
	if json.Unmarshal([]byte(line), &record) != nil {
		return line
	}

	message, isText := record["msg"].(string)
	if !isText {
		return line
	}

	parts := []string{}

	if level, hasLevel := record["level"].(string); hasLevel && level != "INFO" {
		parts = append(parts, level)
	}

	parts = append(parts, message)

	keys := make([]string, 0, len(record))
	for key := range record {
		if key != "time" && key != "level" && key != "msg" {
			keys = append(keys, key)
		}
	}

	slices.Sort(keys)

	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, record[key]))
	}

	return strings.Join(parts, " ")
}

func (c *Client) versionSuffix() string {
	version := c.version.Load()
	if version == nil {
		return ""
	}

	return " (launcher " + *version + ")"
}

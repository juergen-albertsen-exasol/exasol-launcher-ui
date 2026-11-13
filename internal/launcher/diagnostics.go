package launcher

import (
	"context"
	"encoding/json"
)

// LocalDiagnostics is the output of `exasol diag local`.
type LocalDiagnostics struct {
	Platform          string            `json:"platform"`
	PlatformSupported bool              `json:"platformSupported"`
	VMRunning         *bool             `json:"vmRunning"`
	Ports             map[string]int    `json:"ports"`
	PortHealth        map[string]string `json:"portHealth"`
	DatabaseReady     *bool             `json:"databaseReady"`
	DatabaseError     string            `json:"databaseError"`
	Warning           string            `json:"warning"`
	Message           string            `json:"message"`
}

// DiagnoseLocal reports the local runtime state of the deployment in
// deploymentDir.
func (c *Client) DiagnoseLocal(
	ctx context.Context,
	deploymentDir string,
) (LocalDiagnostics, error) {
	var diagnostics LocalDiagnostics

	err := c.runJSON(ctx, &diagnostics, "diag", "local", "--deployment-dir", deploymentDir)

	return diagnostics, err
}

// DiagnosticInfo returns the full report `exasol diag info` prints for the
// deployment in deploymentDir.
func (c *Client) DiagnosticInfo(
	ctx context.Context,
	deploymentDir string,
) (json.RawMessage, error) {
	var report json.RawMessage

	err := c.runJSON(ctx, &report, "diag", "info", "--deployment-dir", deploymentDir)

	return report, err
}

// Unlock removes the lock marker of the deployment folder in deploymentDir.
func (c *Client) Unlock(ctx context.Context, deploymentDir string) error {
	return c.run(ctx, "diag", "unlock", "--deployment-dir", deploymentDir)
}

package launcher

import "context"

// SidecarEndpoint is an address a sidecar serves on.
type SidecarEndpoint struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	URL  string `json:"url"`
}

// SidecarStatus is one entry of `exasol sidecar status --json`.
type SidecarStatus struct {
	Name               string            `json:"name"`
	Enabled            bool              `json:"enabled"`
	Running            bool              `json:"running"`
	RestartRequired    bool              `json:"restartRequired"`
	State              string            `json:"state"`
	Health             string            `json:"health"`
	HealthError        string            `json:"healthError"`
	Error              string            `json:"error"`
	LastOperationError string            `json:"lastOperationError"`
	Endpoints          []SidecarEndpoint `json:"endpoints"`
}

// CatalogSidecar is one entry of `exasol sidecar list --json`.
type CatalogSidecar struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Supported   bool   `json:"supported"`
	Enabled     bool   `json:"enabled"`
}

// MCPClient is an AI client `exasol mcp instructions` has instructions for.
type MCPClient struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MCPInstructions registers the MCP server's endpoint with one client, either
// by running Command or by adding Content to File.
type MCPInstructions struct {
	Client  string `json:"client"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Command string `json:"command"`
	File    string `json:"file"`
	Content string `json:"content"`
	Note    string `json:"note"`
}

// SidecarStatus reports the sidecars saved for the deployment in
// deploymentDir.
func (c *Client) SidecarStatus(
	ctx context.Context,
	deploymentDir string,
) ([]SidecarStatus, error) {
	var sidecars []SidecarStatus

	err := c.runJSON(ctx, &sidecars,
		"sidecar", "status", "--json", "--deployment-dir", deploymentDir)

	return sidecars, err
}

// SidecarCatalog reports the catalog sidecars for the deployment in
// deploymentDir.
func (c *Client) SidecarCatalog(
	ctx context.Context,
	deploymentDir string,
) ([]CatalogSidecar, error) {
	var sidecars []CatalogSidecar

	err := c.runJSON(ctx, &sidecars,
		"sidecar", "list", "--json", "--deployment-dir", deploymentDir)

	return sidecars, err
}

// MCPClients lists the AI clients with MCP setup instructions.
func (c *Client) MCPClients(ctx context.Context, deploymentDir string) ([]MCPClient, error) {
	var clients []MCPClient

	err := c.runJSON(ctx, &clients,
		"mcp", "instructions", "--json", "--deployment-dir", deploymentDir)

	return clients, err
}

// MCPInstructions returns the setup instructions of the deployment's MCP
// server for client.
func (c *Client) MCPInstructions(
	ctx context.Context,
	deploymentDir string,
	client string,
) (MCPInstructions, error) {
	var instructions MCPInstructions

	err := c.runJSON(ctx, &instructions,
		"mcp", "instructions", client, "--json", "--deployment-dir", deploymentDir)

	return instructions, err
}

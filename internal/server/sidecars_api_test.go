package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exasol/exasol-launcher-ui/internal/launcher/launchertest"
	"github.com/exasol/exasol-launcher-ui/internal/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultDir = "/deployments/default"

	savedSidecars = `[
	  {"name": "mcp", "enabled": true, "running": true, "restartRequired": false,
	   "state": "running", "reconciliation": "complete", "exitCode": 0, "health": "serving",
	   "endpoints": [{"ip": "127.0.0.1", "port": 4896, "url": "http://127.0.0.1:4896/mcp"}]},
	  {"name": "kafka", "enabled": true, "running": true, "restartRequired": false,
	   "state": "running", "reconciliation": "complete", "exitCode": 0, "health": "",
	   "endpoints": [{"ip": "127.0.0.1", "port": 19092}]}
	]`
	catalogSidecars = `[
	  {"name": "mcp", "description": "Exasol MCP server", "architectures": ["arm64"],
	   "supported": true, "enabled": true},
	  {"name": "dash-server", "description": "Dashboards", "architectures": ["arm64"],
	   "supported": true, "enabled": false},
	  {"name": "gpu-tools", "description": "GPU tools", "architectures": ["amd64"],
	   "supported": false, "enabled": false, "unsupported": "needs amd64"}
	]`
	mcpClients = `[
	  {"id": "claude-code", "name": "Claude Code"},
	  {"id": "claude-desktop", "name": "Claude Desktop"},
	  {"id": "codex", "name": "Codex"},
	  {"id": "cursor", "name": "Cursor"},
	  {"id": "vscode", "name": "VS Code (GitHub Copilot)"},
	  {"id": "gemini", "name": "Gemini CLI"},
	  {"id": "opencode", "name": "OpenCode"},
	  {"id": "continue", "name": "Continue"}
	]`
)

func sidecarArgs(command, path string) []string {
	return []string{"sidecar", command, "--json", "--deployment-dir", path}
}

func mcpArgs(client ...string) []string {
	args := append([]string{"mcp", "instructions"}, client...)

	return append(args, "--json", "--deployment-dir", defaultDir)
}

func newSidecarsFake(t *testing.T) *launchertest.Fake {
	t.Helper()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fake.Respond(savedSidecars, sidecarArgs("status", defaultDir)...)
	fake.Respond(catalogSidecars, sidecarArgs("list", defaultDir)...)
	fake.Respond(mcpClients, mcpArgs()...)

	return fake
}

func TestSidecarsListSavedThenCatalogSidecars(t *testing.T) {
	t.Parallel()

	response := callAPI(t, newSidecarsFake(t), "/api/v1/deployments/default/sidecars")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, []sidecarView{
		{
			Name: "mcp", Description: "Exasol MCP server", Catalog: true,
			Enabled: true, Running: true, State: "running", Health: "serving",
			Endpoints: []endpointView{
				{IP: "127.0.0.1", Port: 4896, URL: "http://127.0.0.1:4896/mcp"},
			},
		},
		{
			Name: "kafka", Enabled: true, Running: true, State: "running",
			Endpoints: []endpointView{{IP: "127.0.0.1", Port: 19092}},
		},
		{
			Name: "dash-server", Description: "Dashboards", Catalog: true,
			Endpoints: []endpointView{},
		},
	}, decode[[]sidecarView](t, response))
}

func TestSidecarHealthFailuresAreReported(t *testing.T) {
	t.Parallel()

	fake := newSidecarsFake(t)
	fake.Respond(`[{"name": "mcp", "enabled": true, "running": true, "state": "running",
	  "health": "failing", "healthError": "connection refused",
	  "lastOperationError": "enable timed out", "endpoints": []}]`,
		sidecarArgs("status", defaultDir)...)

	response := callAPI(t, fake, "/api/v1/deployments/default/sidecars")

	require.Equal(t, http.StatusOK, response.Code)

	mcp := decode[[]sidecarView](t, response)[0]
	assert.True(t, mcp.Running)
	assert.Equal(t, "failing", mcp.Health)
	assert.Equal(t, "connection refused", mcp.HealthError)
	assert.Equal(t, "enable timed out", mcp.LastOperationError)
}

func TestACloudDeploymentRunsNoSidecars(t *testing.T) {
	t.Parallel()

	fake := newSidecarsFake(t)
	fake.Fail("Error: sidecars are not supported for this deployment type\n", 1,
		sidecarArgs("status", "/deployments/test")...)

	response := callAPI(t, fake, "/api/v1/deployments/test/sidecars")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "sidecars are not supported for this deployment type",
		decode[problem](t, response).Detail)
}

func TestSidecarsOfAnUnknownDeploymentAreNotFound(t *testing.T) {
	t.Parallel()

	response := callAPI(t, newSidecarsFake(t), "/api/v1/deployments/other/sidecars")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestMCPClientsAreListed(t *testing.T) {
	t.Parallel()

	response := callAPI(t, newSidecarsFake(t), "/api/v1/deployments/default/mcp/clients")

	require.Equal(t, http.StatusOK, response.Code)

	clients := decode[[]mcpClientView](t, response)
	ids := make([]string, 0, len(clients))

	for _, client := range clients {
		ids = append(ids, client.ID)
	}

	assert.Equal(t, []string{
		"claude-code", "claude-desktop", "codex", "cursor",
		"vscode", "gemini", "opencode", "continue",
	}, ids)
	assert.Equal(t, mcpClientView{ID: "claude-code", Name: "Claude Code"}, clients[0])
}

func TestMCPInstructionsCarryTheRegistrationCommand(t *testing.T) {
	t.Parallel()

	fake := newSidecarsFake(t)
	fake.Respond(`{"client": "claude-code", "name": "Claude Code",
	  "url": "http://127.0.0.1:4896/mcp",
	  "command": "claude mcp add --transport http exasol-personal http://127.0.0.1:4896/mcp"}`,
		mcpArgs("claude-code")...)

	response := callAPI(t, fake, "/api/v1/deployments/default/mcp/clients/claude-code")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, mcpInstructionsView{
		ID: "claude-code", Name: "Claude Code", URL: "http://127.0.0.1:4896/mcp",
		Command: "claude mcp add --transport http exasol-personal http://127.0.0.1:4896/mcp",
	}, decode[mcpInstructionsView](t, response))
}

func TestMCPInstructionsCarryTheConfigurationEntry(t *testing.T) {
	t.Parallel()

	fake := newSidecarsFake(t)
	fake.Respond(`{"client": "cursor", "name": "Cursor", "url": "http://127.0.0.1:4896/mcp",
	  "file": "~/.cursor/mcp.json", "content": "{\"url\": \"http://127.0.0.1:4896/mcp\"}\n"}`,
		mcpArgs("cursor")...)

	response := callAPI(t, fake, "/api/v1/deployments/default/mcp/clients/cursor")

	require.Equal(t, http.StatusOK, response.Code)

	instructions := decode[mcpInstructionsView](t, response)
	assert.Equal(t, "~/.cursor/mcp.json", instructions.File)
	assert.Contains(t, instructions.Content, "http://127.0.0.1:4896/mcp")
	assert.Empty(t, instructions.Command)
}

func TestMCPInstructionsNeedAnEnabledMCPServer(t *testing.T) {
	t.Parallel()

	fake := newSidecarsFake(t)
	fake.Fail("Error: the MCP server is not enabled for this deployment\n", 1,
		mcpArgs("claude-code")...)

	response := callAPI(t, fake, "/api/v1/deployments/default/mcp/clients/claude-code")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "the MCP server is not enabled for this deployment",
		decode[problem](t, response).Detail)
}

func TestMCPInstructionsOfAnUnlistedClientAreNotFound(t *testing.T) {
	t.Parallel()

	fake := newSidecarsFake(t)

	response := callAPI(t, fake, "/api/v1/deployments/default/mcp/clients/--help")

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.NotContains(t, fake.Calls(),
		"mcp instructions --help --json --deployment-dir "+defaultDir)
}

func newSidecarOperationsFixture(t *testing.T) operationsFixture {
	t.Helper()

	fixture := newOperationsFixture(t)
	fixture.fake.Respond(savedSidecars, sidecarArgs("status", defaultDir)...)
	fixture.fake.Respond(catalogSidecars, sidecarArgs("list", defaultDir)...)

	return fixture
}

func (f operationsFixture) listedSidecars(t *testing.T) []string {
	t.Helper()

	response := f.call(t, http.MethodGet, "/api/v1/deployments/default/sidecars")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	names := []string{}

	for _, sidecar := range decode[[]sidecarView](t, response) {
		if sidecar.Enabled {
			names = append(names, sidecar.Name)
		}
	}

	return names
}

func TestEnablingACatalogSidecar(t *testing.T) {
	t.Parallel()

	fixture := newSidecarOperationsFixture(t)
	enableArgs := []string{"sidecar", "enable", "dash-server", "--deployment-dir", defaultDir}
	fixture.fake.RespondWithProgress("Pulling the image\n", "", enableArgs...)
	fixture.fake.RespondAfter(enableArgs, `[
	  {"name": "mcp", "enabled": true, "running": true, "state": "running", "endpoints": []},
	  {"name": "kafka", "enabled": true, "running": true, "state": "running", "endpoints": []},
	  {"name": "dash-server", "enabled": true, "running": true, "state": "running",
	   "endpoints": []}
	]`, sidecarArgs("status", defaultDir)...)

	response := fixture.call(t, http.MethodPost,
		"/api/v1/deployments/default/sidecars/dash-server/enable")

	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())

	started := decode[operationView](t, response)
	assert.Equal(t, operations.KindEnableSidecar, started.Kind)
	assert.Equal(t, "dash-server", started.Sidecar)
	assert.Equal(t, "default", started.Deployment)
	assert.Equal(t, "/api/v1/operations/"+started.ID, response.Header().Get("Location"))

	fixture.waitFor(t, started.ID, "succeeded")
	assert.Equal(t, []string{"mcp", "kafka", "dash-server"}, fixture.listedSidecars(t))
}

func TestDisablingASidecarAndDeletingItsDefinition(t *testing.T) {
	t.Parallel()

	fixture := newSidecarOperationsFixture(t)
	removeArgs := []string{
		"sidecar", "disable", "kafka", "--remove", "--deployment-dir", defaultDir,
	}
	fixture.fake.RespondWithProgress("", "", removeArgs...)
	fixture.fake.RespondAfter(removeArgs, `[
	  {"name": "mcp", "enabled": true, "running": true, "state": "running", "endpoints": []}
	]`, sidecarArgs("status", defaultDir)...)

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/deployments/default/sidecars/kafka/disable",
		strings.NewReader(`{"remove_definition": true}`))
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())

	started := decode[operationView](t, response)
	assert.Equal(t, operations.KindRemoveSidecar, started.Kind)
	assert.Equal(t, "kafka", started.Sidecar)

	fixture.waitFor(t, started.ID, "succeeded")
	assert.Equal(t, []string{"mcp"}, fixture.listedSidecars(t))
}

func TestDisablingASidecarKeepsItsDefinition(t *testing.T) {
	t.Parallel()

	fixture := newSidecarOperationsFixture(t)
	fixture.fake.RespondWithProgress("", "",
		"sidecar", "disable", "kafka", "--deployment-dir", defaultDir)

	started := fixture.post(t, "/api/v1/deployments/default/sidecars/kafka/disable")

	assert.Equal(t, operations.KindDisableSidecar, started.Kind)
	fixture.waitFor(t, started.ID, "succeeded")
}

func TestAnUnlistedSidecarIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newSidecarOperationsFixture(t)

	response := fixture.call(t, http.MethodPost,
		"/api/v1/deployments/default/sidecars/--all/enable")

	assert.Equal(t, http.StatusNotFound, response.Code)

	for _, call := range fixture.fake.Calls() {
		assert.NotContains(t, call, "--all")
	}
}

func TestASidecarOperationWaitsForARunningOperation(t *testing.T) {
	t.Parallel()

	fixture := newSidecarOperationsFixture(t)
	startArgs := lifecycleArgs("start", defaultDir)
	fixture.fake.RespondWithProgress("", runningResult, startArgs...)
	fixture.fake.Hold(startArgs...)

	defer fixture.fake.Release(startArgs...)

	first := fixture.call(t, http.MethodPost, "/api/v1/deployments/default/start")
	require.Equal(t, http.StatusAccepted, first.Code)

	second := fixture.call(t, http.MethodPost,
		"/api/v1/deployments/default/sidecars/mcp/disable")
	assert.Equal(t, http.StatusConflict, second.Code)
}

package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	runningLocalDiagnostics = `{
	  "platform": "darwin/arm64",
	  "platformSupported": true,
	  "vmRunning": true,
	  "ports": {"db": 8563},
	  "portHealth": {
	    "sidecar.mcp.0": "reachable",
	    "sidecar.kafka.1": "refused",
	    "sidecar.kafka.0": "reachable",
	    "db": "reachable"
	  },
	  "databaseReady": true
	}`
	stoppedLocalDiagnostics = `{
	  "platform": "darwin/arm64",
	  "platformSupported": true,
	  "vmRunning": false,
	  "message": "The platform is ready to run the local deployment. Run ` + "`exasol start`" +
		` to start it, then run ` + "`exasol diag local`" + ` again for more detail."
	}`
	diagnosedSidecars = `[
	  {"name": "kafka", "endpoints": [{"ip": "127.0.0.1", "port": 19092}]},
	  {"name": "mcp", "endpoints": [{"ip": "127.0.0.1", "port": 4896,
	    "url": "http://127.0.0.1:4896/mcp"}]}
	]`
	diagnosticReport = `{
	  "backend": "local",
	  "deploymentState": "running",
	  "connection": {"host": "127.0.0.1", "dbPort": 8563, "username": "sys"}
	}`
)

func diagArgs(command, path string) []string {
	return []string{"diag", command, "--deployment-dir", path}
}

func sidecarStatusArgs(path string) []string {
	return []string{"sidecar", "status", "--json", "--deployment-dir", path}
}

func newDiagnosticsFake(t *testing.T) *launchertest.Fake {
	t.Helper()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fake.Respond(runningLocalDiagnostics, diagArgs("local", defaultDir)...)
	fake.Respond(diagnosedSidecars, sidecarStatusArgs(defaultDir)...)
	fake.Respond(diagnosticReport, diagArgs("info", defaultDir)...)

	return fake
}

func port(name string, hostPort int, reachability string) portView {
	return portView{Name: name, HostPort: &hostPort, Reachability: reachability}
}

func TestARunningLocalDeploymentReportsItsRuntime(t *testing.T) {
	t.Parallel()

	response := callAPI(t, newDiagnosticsFake(t), "/api/v1/deployments/default/diagnostics/local")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	ready := true
	assert.Equal(t, localRuntimeView{
		Platform:          "darwin/arm64",
		PlatformSupported: true,
		VMRunning:         true,
		DatabaseReady:     &ready,
		Ports: []portView{
			port("db", 8563, "reachable"),
			port("sidecar kafka", 19092, "reachable"),
			{Name: "sidecar kafka 2", Reachability: "refused"},
			port("sidecar mcp", 4896, "reachable"),
		},
	}, decode[localRuntimeView](t, response))
}

func TestSidecarPortsLackHostPortsWhenTheSidecarStatusFails(t *testing.T) {
	t.Parallel()

	fake := newDiagnosticsFake(t)
	fake.Fail("Error: broken\n", 1, sidecarStatusArgs(defaultDir)...)

	response := callAPI(t, fake, "/api/v1/deployments/default/diagnostics/local")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, portView{Name: "sidecar mcp", Reachability: "reachable"},
		decode[localRuntimeView](t, response).Ports[3])
}

func TestAStoppedLocalDeploymentReportsTheLaunchersMessage(t *testing.T) {
	t.Parallel()

	fake := newDiagnosticsFake(t)
	fake.Respond(stoppedLocalDiagnostics, diagArgs("local", defaultDir)...)

	response := callAPI(t, fake, "/api/v1/deployments/default/diagnostics/local")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	runtime := decode[localRuntimeView](t, response)
	assert.False(t, runtime.VMRunning)
	assert.Contains(t, runtime.Message, "Run `exasol start` to start it")
	assert.Empty(t, runtime.Ports)
	assert.NotContains(t, fake.Calls(), "sidecar status --json --deployment-dir "+defaultDir)
}

func TestACloudDeploymentHasNoLocalRuntime(t *testing.T) {
	t.Parallel()

	fake := newDiagnosticsFake(t)

	response := callAPI(t, fake, "/api/v1/deployments/test/diagnostics/local")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "this deployment is not using the local preset",
		decode[problem](t, response).Detail)
	assert.NotContains(t, fake.Calls(), "diag local --deployment-dir /deployments/test")
}

func TestARefusedDiagnosisIsAConflict(t *testing.T) {
	t.Parallel()

	fake := newDiagnosticsFake(t)
	fake.Fail("Error: another operation is in progress\n", 1, diagArgs("local", defaultDir)...)

	response := callAPI(t, fake, "/api/v1/deployments/default/diagnostics/local")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "another operation is in progress", decode[problem](t, response).Detail)
}

func TestDiagnosticsOfAnUnknownDeploymentAreNotFound(t *testing.T) {
	t.Parallel()

	for _, resource := range []string{"local", "info"} {
		response := callAPI(t, newDiagnosticsFake(t),
			"/api/v1/deployments/other/diagnostics/"+resource)

		assert.Equal(t, http.StatusNotFound, response.Code, resource)
	}
}

func TestTheReportIsTheLaunchersReport(t *testing.T) {
	t.Parallel()

	response := callAPI(t, newDiagnosticsFake(t), "/api/v1/deployments/default/diagnostics/info")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	assert.JSONEq(t, diagnosticReport, response.Body.String())
}

func TestTheReportWithholdsThePassword(t *testing.T) {
	t.Parallel()

	const password = `pa"ss<word>`

	dir := t.TempDir()
	secrets, err := json.Marshal(map[string]string{"dbPassword": password})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.json"), secrets, 0o600))

	listing, err := json.Marshal([]launcher.Deployment{{
		Name: "default", Path: dir, Infrastructure: "local", Installation: "local",
	}})
	require.NoError(t, err)

	fake := launchertest.NewFake(t)
	fake.Respond(string(listing), listArgs...)
	fake.Respond(`{"connection": {"password": "pa\"ss<word>"}, "note": "pa\"ss<word>"}`,
		diagArgs("info", dir)...)

	response := callAPI(t, fake, "/api/v1/deployments/default/diagnostics/info")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.JSONEq(t, `{"connection": {"password": "********"}, "note": "********"}`,
		response.Body.String())
}

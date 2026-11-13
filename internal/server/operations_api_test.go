package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/exasol/exasol-local-ui/internal/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	runningResult = `{"deploymentState": "running", "databaseReady": true}`
	stoppedResult = `{"deploymentState": "stopped", "databaseReady": false}`
)

func lifecycleArgs(command, path string) []string {
	return []string{command, "--json", "--deployment-dir", path}
}

type operationsFixture struct {
	fake    *launchertest.Fake
	handler http.Handler
}

func newOperationsFixture(t *testing.T) operationsFixture {
	t.Helper()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)

	client := launcher.New(fake.Executable)
	manager := operations.NewManager(t.Context(), client, client.InvalidateListing)

	return operationsFixture{
		fake:    fake,
		handler: NewHandler(Config{Launcher: client, Operations: manager}),
	}
}

func (f operationsFixture) call(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)

	return recorder
}

func (f operationsFixture) post(t *testing.T, target string) operationView {
	t.Helper()

	return decode[operationView](t, f.call(t, http.MethodPost, target))
}

func (f operationsFixture) finishes(result, command, path string) {
	f.fake.RespondWithProgress("", result, lifecycleArgs(command, path)...)
}

func (f operationsFixture) waitFor(t *testing.T, operationID string, status string) operationView {
	t.Helper()

	var view operationView

	require.Eventually(t, func() bool {
		response := f.call(t, http.MethodGet, "/api/v1/operations/"+operationID)
		view = decode[operationView](t, response)

		return view.Status == status
	}, 10*time.Second, 10*time.Millisecond)

	return view
}

func TestStoppingARunningDeployment(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	fixture.fake.RespondWithProgress("Stopping the VM\n", stoppedResult,
		lifecycleArgs("stop", "/deployments/default")...)

	response := fixture.call(t, http.MethodPost, "/api/v1/deployments/default/stop")

	require.Equal(t, http.StatusAccepted, response.Code)

	started := decode[operationView](t, response)
	assert.Equal(t, "default", started.Deployment)
	assert.Equal(t, operations.KindStop, started.Kind)
	assert.Equal(t, "running", started.Status)
	assert.Equal(t, "/api/v1/operations/"+started.ID, response.Header().Get("Location"))

	finished := fixture.waitFor(t, started.ID, "succeeded")
	assert.Equal(t, &operationResult{DeploymentState: "stopped"}, finished.Result)
	assert.Equal(t, []string{"Stopping the VM"}, finished.Lines)
	assert.NotNil(t, finished.FinishedAt)
}

func TestStartingReportsDatabaseReadiness(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	fixture.finishes(runningResult, "start", "/deployments/test")

	started := fixture.post(t, "/api/v1/deployments/test/start")
	finished := fixture.waitFor(t, started.ID, "succeeded")

	expected := &operationResult{DeploymentState: "running", DatabaseReady: true}
	assert.Equal(t, expected, finished.Result)
}

func TestASecondOperationOnTheSameDeploymentIsRefused(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	startArgs := lifecycleArgs("start", "/deployments/default")
	fixture.fake.RespondWithProgress("", runningResult, startArgs...)
	fixture.fake.Hold(startArgs...)

	defer fixture.fake.Release(startArgs...)

	first := fixture.call(t, http.MethodPost, "/api/v1/deployments/default/start")
	require.Equal(t, http.StatusAccepted, first.Code)

	second := fixture.call(t, http.MethodPost, "/api/v1/deployments/default/start")
	assert.Equal(t, http.StatusConflict, second.Code)
}

func TestOperationsOnDifferentDeploymentsRunSideBySide(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	stopTest := lifecycleArgs("stop", "/deployments/test")
	fixture.fake.RespondWithProgress("", stoppedResult, stopTest...)
	fixture.finishes(runningResult, "start", "/deployments/default")
	fixture.fake.Hold(stopTest...)

	defer fixture.fake.Release(stopTest...)

	require.Equal(t, http.StatusAccepted,
		fixture.call(t, http.MethodPost, "/api/v1/deployments/test/stop").Code)
	assert.Equal(t, http.StatusAccepted,
		fixture.call(t, http.MethodPost, "/api/v1/deployments/default/start").Code)
}

func TestOperationOfAnUnknownDeploymentIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)

	response := fixture.call(t, http.MethodPost, "/api/v1/deployments/unknown/start")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestAFailedStopCarriesTheLauncherError(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	stopArgs := lifecycleArgs("stop", "/deployments/default")
	fixture.fake.Fail("Error: the VM did not stop\n", 1, stopArgs...)

	started := fixture.post(t, "/api/v1/deployments/default/stop")
	failed := fixture.waitFor(t, started.ID, "failed")

	assert.Contains(t, failed.Error, "Error: the VM did not stop")
	assert.Nil(t, failed.Result)
}

func TestOperationsAreListedNewestFirst(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	fixture.finishes(stoppedResult, "stop", "/deployments/test")
	startArgs := lifecycleArgs("start", "/deployments/default")
	fixture.fake.RespondWithProgress("", runningResult, startArgs...)
	fixture.fake.Hold(startArgs...)

	defer fixture.fake.Release(startArgs...)

	stopTest := fixture.post(t, "/api/v1/deployments/test/stop")
	fixture.waitFor(t, stopTest.ID, "succeeded")

	startDefault := decode[operationView](t,
		fixture.call(t, http.MethodPost, "/api/v1/deployments/default/start"))

	list := decode[[]operationView](t, fixture.call(t, http.MethodGet, "/api/v1/operations"))

	require.Len(t, list, 2)
	assert.Equal(t, startDefault.ID, list[0].ID)
	assert.Equal(t, "running", list[0].Status)
	assert.Equal(t, stopTest.ID, list[1].ID)
	assert.Equal(t, "succeeded", list[1].Status)
}

func TestAnUnknownOperationIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)

	assert.Equal(t, http.StatusNotFound,
		fixture.call(t, http.MethodGet, "/api/v1/operations/unknown").Code)
	assert.Equal(t, http.StatusNotFound,
		fixture.call(t, http.MethodGet, "/api/v1/operations/unknown/events").Code)
}

func TestFollowingARunningStart(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	startArgs := lifecycleArgs("start", "/deployments/default")
	fixture.fake.RespondWithProgress(
		"Starting the VM\nWaiting for the database\n", runningResult, startArgs...)
	fixture.fake.Hold(startArgs...)

	server := httptest.NewServer(fixture.handler)
	defer server.Close()

	started := fixture.post(t, "/api/v1/deployments/default/start")

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		server.URL+"/api/v1/operations/"+started.ID+"/events", nil)
	require.NoError(t, err)

	response, err := server.Client().Do(request)
	require.NoError(t, err)

	defer response.Body.Close()

	assert.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))

	events := bufio.NewScanner(response.Body)

	var received []string

	for events.Scan() {
		line := events.Text()
		if !strings.HasPrefix(line, "data: ") {
			if strings.HasPrefix(line, "event: ") {
				received = append(received, line)
			}

			continue
		}

		received = append(received, line)

		if len(received) == 4 {
			fixture.fake.Release(startArgs...)
		}
	}

	require.Len(t, received, 6)
	assert.Equal(t, []string{
		"event: line", `data: "Starting the VM"`,
		"event: line", `data: "Waiting for the database"`,
		"event: status",
	}, received[:5])

	var outcome operationView
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(received[5], "data: ")), &outcome))
	assert.Equal(t, "succeeded", outcome.Status)
}

func TestProgressWithholdsTheDatabasePassword(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.json"),
		[]byte(`{"dbPassword": "`+storedPassword+`"}`), 0o600))

	listing, err := json.Marshal([]launcher.Deployment{{Name: "default", Path: dir}})
	require.NoError(t, err)

	fake := launchertest.NewFake(t)
	fake.Respond(string(listing), listArgs...)
	fake.RespondWithProgress("connecting as sys/"+storedPassword+"\n", runningResult,
		lifecycleArgs("start", dir)...)

	client := launcher.New(fake.Executable)
	fixture := operationsFixture{
		fake: fake,
		handler: NewHandler(Config{
			Launcher:   client,
			Operations: operations.NewManager(t.Context(), client, nil),
		}),
	}

	started := fixture.post(t, "/api/v1/deployments/default/start")
	finished := fixture.waitFor(t, started.ID, "succeeded")

	assert.Equal(t, []string{"connecting as sys/********"}, finished.Lines)
}

func TestAFinishedOperationRefreshesTheListing(t *testing.T) {
	t.Parallel()

	fixture := newOperationsFixture(t)
	fixture.finishes(stoppedResult, "stop", "/deployments/default")

	listCalls := func() int {
		count := 0

		for _, call := range fixture.fake.Calls() {
			if call == "deployments list --json" {
				count++
			}
		}

		return count
	}

	fixture.call(t, http.MethodGet, "/api/v1/deployments")
	started := fixture.post(t, "/api/v1/deployments/default/stop")
	fixture.waitFor(t, started.ID, "succeeded")

	before := listCalls()
	fixture.call(t, http.MethodGet, "/api/v1/deployments")

	assert.Equal(t, before+1, listCalls())
}

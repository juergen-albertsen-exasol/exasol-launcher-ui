package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	twoDeployments = `[
	  {"name": "default", "path": "/deployments/default", "status": "database_ready",
	   "infrastructure": "local", "installation": "local"},
	  {"name": "test", "path": "/deployments/test", "status": "stopped",
	   "infrastructure": "aws", "installation": "exasol"}
	]`
	runningInfo = `{
	  "deploymentDir": "/deployments/default",
	  "deploymentState": "running",
	  "connection": {
	    "host": "127.0.0.1", "dbPort": 8563, "username": "sys",
	    "insecureSkipCertValidation": true, "password": "must-not-leak"
	  }
	}`
)

var listArgs = []string{"deployments", "list", "--json"}

func callAPI(t *testing.T, fake *launchertest.Fake, target string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	NewHandler(launcher.New(fake.Executable)).ServeHTTP(recorder, request)

	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var value T
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &value), recorder.Body.String())

	return value
}

func TestDeploymentsMatchTheLauncherListing(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)

	response := callAPI(t, fake, "/api/v1/deployments")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
	assert.Equal(t, []deploymentSummary{
		{Name: "default", Status: "database_ready", Infrastructure: "local", Installation: "local"},
		{Name: "test", Status: "stopped", Infrastructure: "aws", Installation: "exasol"},
	}, decode[[]deploymentSummary](t, response))
}

func TestNoDeploymentsYieldsAnEmptyList(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(`[]`, listArgs...)

	response := callAPI(t, fake, "/api/v1/deployments")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `[]`, response.Body.String())
}

func TestListingFailureIsReported(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Fail("Error: broken", 1, listArgs...)

	response := callAPI(t, fake, "/api/v1/deployments")

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Contains(t, decode[problem](t, response).Detail, "broken")
}

func TestRunningDeploymentIsDescribed(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fake.Respond(runningInfo, "info", "--json", "--deployment-dir", "/deployments/default")

	response := callAPI(t, fake, "/api/v1/deployments/default")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, deploymentDetails{
		Name:  "default",
		State: "running",
		Connection: &connectionDetails{
			Host: "127.0.0.1", Port: 8563, Username: "sys", InsecureSkipCertValidation: true,
		},
	}, decode[deploymentDetails](t, response))
	assert.NotContains(t, response.Body.String(), "must-not-leak")
}

func TestStoppedDeploymentHasNoConnection(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fake.Respond(`{"deploymentState": "stopped"}`,
		"info", "--json", "--deployment-dir", "/deployments/test")

	response := callAPI(t, fake, "/api/v1/deployments/test")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"name": "test", "state": "stopped"}`, response.Body.String())
}

func TestUnknownDeploymentIsNotFound(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)

	response := callAPI(t, fake, "/api/v1/deployments/unknown")

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
}

func TestUnknownAPIPathIsNotFound(t *testing.T) {
	t.Parallel()

	response := callAPI(t, launchertest.NewFake(t), "/api/v1/unknown")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

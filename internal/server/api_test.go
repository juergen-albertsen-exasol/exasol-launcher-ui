package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
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

type pingFunc = func(ctx context.Context, target database.Target) (time.Duration, error)

func callAPI(t *testing.T, fake *launchertest.Fake, target string) *httptest.ResponseRecorder {
	t.Helper()

	return callAPIWithPing(t, fake, target, nil)
}

func callAPIWithPing(
	t *testing.T,
	fake *launchertest.Fake,
	target string,
	ping pingFunc,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	NewHandler(Config{Launcher: launcher.New(fake.Executable), Ping: ping}).
		ServeHTTP(recorder, request)

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
		{
			Name: "default", Path: "/deployments/default", Status: "database_ready",
			Infrastructure: "local", Installation: "local",
		},
		{
			Name: "test", Path: "/deployments/test", Status: "stopped",
			Infrastructure: "aws", Installation: "exasol",
		},
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

const storedPassword = "s3cretPassw0rd"

// fakeWithDeploymentDir lists one deployment "default" whose directory holds
// a secrets.json, and describes it with infoJSON.
func fakeWithDeploymentDir(t *testing.T, infoJSON string) *launchertest.Fake {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.json"),
		[]byte(`{"dbPassword": "`+storedPassword+`"}`), 0o600))

	listing, err := json.Marshal([]launcher.Deployment{{
		Name: "default", Path: dir, Status: "database_ready",
		Infrastructure: "local", Installation: "local",
	}})
	require.NoError(t, err)

	fake := launchertest.NewFake(t)
	fake.Respond(string(listing), listArgs...)
	fake.Respond(infoJSON, "info", "--json", "--deployment-dir", dir)

	return fake
}

func TestReachableDatabaseReportsTheRoundTrip(t *testing.T) {
	t.Parallel()

	var pinged database.Target

	fake := fakeWithDeploymentDir(t, runningInfo)
	response := callAPIWithPing(t, fake, "/api/v1/deployments/default/health/ping",
		func(_ context.Context, target database.Target) (time.Duration, error) {
			pinged = target

			return 12 * time.Millisecond, nil
		})

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"status": "reachable", "round_trip_ms": 12}`, response.Body.String())
	assert.Equal(t, database.Target{
		Host: "127.0.0.1", Port: 8563, Username: "sys", Password: storedPassword,
		InsecureSkipCertValidation: true,
	}, pinged)
}

func TestUnreachableDatabaseReportsTheRedactedError(t *testing.T) {
	t.Parallel()

	fake := fakeWithDeploymentDir(t, runningInfo)
	response := callAPIWithPing(t, fake, "/api/v1/deployments/default/health/ping",
		func(_ context.Context, target database.Target) (time.Duration, error) {
			return 0, errors.New("connection refused for " + target.String() +
				" with " + target.Password)
		})

	assert.Equal(t, http.StatusOK, response.Code)

	result := decode[pingResult](t, response)
	assert.Equal(t, "unreachable", result.Status)
	assert.Contains(t, result.Error, "sys@127.0.0.1:8563")
	assert.NotContains(t, response.Body.String(), storedPassword)
}

func TestStoppedDeploymentIsUnreachable(t *testing.T) {
	t.Parallel()

	fake := fakeWithDeploymentDir(t, `{"deploymentState": "stopped"}`)
	response := callAPIWithPing(t, fake, "/api/v1/deployments/default/health/ping",
		func(context.Context, database.Target) (time.Duration, error) {
			t.Error("unexpected ping")

			return 0, nil
		})

	result := decode[pingResult](t, response)
	assert.Equal(t, "unreachable", result.Status)
	assert.Contains(t, result.Error, "stopped")
}

func TestPingOfAnUnknownDeploymentIsNotFound(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(`[]`, listArgs...)

	response := callAPI(t, fake, "/api/v1/deployments/default/health/ping")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func callOpenFolder(
	t *testing.T,
	fake *launchertest.Fake,
	name string,
	openFolder func(ctx context.Context, path string) error,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/v1/deployments/"+name+"/open-folder", nil)
	recorder := httptest.NewRecorder()
	NewHandler(Config{Launcher: launcher.New(fake.Executable), OpenFolder: openFolder}).
		ServeHTTP(recorder, request)

	return recorder
}

func TestOpenFolderOpensTheListedDirectory(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)

	var opened string

	response := callOpenFolder(t, fake, "test", func(_ context.Context, path string) error {
		opened = path

		return nil
	})

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "/deployments/test", opened)
}

func TestOpenFolderOfAnUnknownDeploymentIsNotFound(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)

	response := callOpenFolder(t, fake, "unknown", func(context.Context, string) error {
		t.Error("unexpected open")

		return nil
	})

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestOpenFolderFailureIsReported(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)

	response := callOpenFolder(t, fake, "default", func(context.Context, string) error {
		return errors.New("no file manager")
	})

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Contains(t, decode[problem](t, response).Detail, "no file manager")
}

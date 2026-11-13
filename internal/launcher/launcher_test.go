package launcher_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const listing = `[
  {"name": "default", "path": "/deployments/default", "status": "database_ready",
   "infrastructure": "local", "installation": "local", "future": true},
  {"name": "test", "path": "/deployments/test", "status": "stopped",
   "infrastructure": "local", "installation": "local"}
]`

var listArgs = []string{"deployments", "list", "--json"}

func TestCheckReportsTheLauncherVersion(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)

	version, err := launcher.New(fake.Executable).Check(t.Context())

	require.NoError(t, err)
	assert.Equal(t, launchertest.Version, version)
}

func TestCheckNamesAMissingExecutable(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "exasol")

	_, err := launcher.New(missing).Check(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
}

func TestDeploymentsDecodesTheListingInOrder(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(listing, listArgs...)

	deployments, err := launcher.New(fake.Executable).Deployments(t.Context(), false)

	require.NoError(t, err)
	assert.Equal(t, []launcher.Deployment{
		{
			Name: "default", Path: "/deployments/default", Status: "database_ready",
			Infrastructure: "local", Installation: "local",
		},
		{
			Name: "test", Path: "/deployments/test", Status: "stopped",
			Infrastructure: "local", Installation: "local",
		},
	}, deployments)
}

func TestDeploymentsReturnsAnEmptyListing(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(`[]`, listArgs...)

	deployments, err := launcher.New(fake.Executable).Deployments(t.Context(), false)

	require.NoError(t, err)
	assert.NotNil(t, deployments)
	assert.Empty(t, deployments)
}

func TestDeploymentsCachesTheListingBriefly(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(listing, listArgs...)
	client := launcher.New(fake.Executable)
	now := time.Now()
	launcher.SetClock(client, func() time.Time { return now })

	_, err := client.Deployments(t.Context(), false)
	require.NoError(t, err)
	_, err = client.Deployments(t.Context(), false)
	require.NoError(t, err)
	assert.Len(t, fake.Calls(), 1)

	_, err = client.Deployments(t.Context(), true)
	require.NoError(t, err)
	assert.Len(t, fake.Calls(), 2)

	now = now.Add(time.Minute)
	_, err = client.Deployments(t.Context(), false)
	require.NoError(t, err)
	assert.Len(t, fake.Calls(), 3)
}

func TestDeploymentFindsAListedName(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(listing, listArgs...)
	client := launcher.New(fake.Executable)

	deployment, found, err := client.Deployment(t.Context(), "test")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "/deployments/test", deployment.Path)

	_, found, err = client.Deployment(t.Context(), "unknown")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestInfoSelectsTheDeploymentDirectory(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(`{
	  "deploymentDir": "/deployments/default",
	  "deploymentState": "running",
	  "connection": {
	    "backend": "local", "host": "127.0.0.1", "dbPort": 8563, "username": "sys",
	    "insecureSkipCertValidation": true
	  }
	}`, "info", "--json", "--deployment-dir", "/deployments/default")

	info, err := launcher.New(fake.Executable).Info(t.Context(), "/deployments/default")

	require.NoError(t, err)
	assert.Equal(t, launcher.Info{
		DeploymentState: "running",
		Connection: &launcher.Connection{
			Host: "127.0.0.1", Port: 8563, Username: "sys", InsecureSkipCertValidation: true,
		},
	}, info)
}

func TestInfoOmitsUnstableConnectionDetails(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(`{"deploymentState": "stopped"}`, "info", "--json", "--deployment-dir", "/d")

	info, err := launcher.New(fake.Executable).Info(t.Context(), "/d")

	require.NoError(t, err)
	assert.Equal(t, "stopped", info.DeploymentState)
	assert.Nil(t, info.Connection)
}

func TestFailuresCarryTheLauncherMessage(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Fail("Error: deployment directory is locked", 1, listArgs...)

	_, err := launcher.New(fake.Executable).Deployments(t.Context(), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deployments list --json")
	assert.Contains(t, err.Error(), "deployment directory is locked")
}

func TestDecodeFailuresNameTheCommandAndVersion(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(`{"not": "a list"}`, listArgs...)
	client := launcher.New(fake.Executable)
	_, err := client.Check(t.Context())
	require.NoError(t, err)

	_, err = client.Deployments(t.Context(), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deployments list --json")
	assert.Contains(t, err.Error(), launchertest.Version)
}

func TestStreamReportsProgressAndDecodesTheResult(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.RespondWithProgress("first\nsecond\n",
		`{"deploymentState": "stopped", "databaseReady": false}`, "stop", "--json")

	var (
		lines  []string
		result launcher.LifecycleResult
	)

	err := launcher.New(fake.Executable).Stream(t.Context(), func(line string) {
		lines = append(lines, line)
	}, &result, "stop", "--json")

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, lines)
	assert.Equal(t, launcher.LifecycleResult{DeploymentState: "stopped"}, result)
}

func TestStreamFailureCarriesTheLastProgressLine(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Fail("working\nError: deployment is locked\n\n", 1, "stop", "--json")

	err := launcher.New(fake.Executable).
		Stream(t.Context(), func(string) {}, &struct{}{}, "stop", "--json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Error: deployment is locked")
}

func TestInvalidatedListingIsReadAgain(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(listing, listArgs...)
	client := launcher.New(fake.Executable)

	_, err := client.Deployments(t.Context(), false)
	require.NoError(t, err)

	client.InvalidateListing()

	_, err = client.Deployments(t.Context(), false)
	require.NoError(t, err)
	assert.Len(t, fake.Calls(), 2)
}

func TestStreamMakesJSONLogRecordsReadable(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Fail(`{"time":"2026-11-13T12:16:01+01:00","level":"INFO","msg":"starting deployment"}
{"time":"2026-11-13T12:16:02+01:00","level":"WARN","msg":"port busy","port":8563}
plain text
{"time":"2026-11-13T12:16:03+01:00","level":"ERROR","msg":"start failed"}
`, 1, "start", "--json")

	var lines []string

	err := launcher.New(fake.Executable).Stream(t.Context(), func(line string) {
		lines = append(lines, line)
	}, &struct{}{}, "start", "--json")

	assert.Equal(t, []string{
		"starting deployment",
		"WARN port busy port=8563",
		"plain text",
		"ERROR start failed",
	}, lines)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERROR start failed")
}

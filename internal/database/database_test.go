package database_test

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const password = "s3cretPassw0rd"

func writeSecrets(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.json"), []byte(content), 0o600))

	return dir
}

func closedPort(t *testing.T) int {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	address, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	require.NoError(t, listener.Close())

	return address.Port
}

func TestReadPasswordReturnsTheStoredPassword(t *testing.T) {
	t.Parallel()

	dir := writeSecrets(t, `{"dbPassword": "`+password+`", "other": 1}`)

	stored, err := database.ReadPassword(dir)

	require.NoError(t, err)
	assert.Equal(t, password, stored)
}

func TestReadPasswordFailures(t *testing.T) {
	t.Parallel()

	for name, dir := range map[string]string{
		"missing file": t.TempDir(),
		"invalid JSON": writeSecrets(t, `{"dbPassword": "`+password),
		"no password":  writeSecrets(t, `{}`),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := database.ReadPassword(dir)

			require.Error(t, err)
			assert.NotContains(t, err.Error(), password)
		})
	}
}

func TestRedactMasksThePassword(t *testing.T) {
	t.Parallel()

	err := database.Redact(errors.New("login "+password+" failed for "+password), password)

	assert.Equal(t, "login ******** failed for ********", err.Error())
	assert.NoError(t, database.Redact(nil, password))
}

func TestPingFailureNamesTheTargetAndWithholdsThePassword(t *testing.T) {
	t.Parallel()

	port := closedPort(t)

	_, err := database.Ping(t.Context(), database.Target{
		Host: "127.0.0.1", Port: port, Username: "sys", Password: password,
		InsecureSkipCertValidation: true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sys@127.0.0.1:")
	assert.NotContains(t, err.Error(), password)
}

// TestPingReachesARunningLocalDeployment needs the launcher on PATH and a
// deployment whose database is ready, and skips otherwise.
func TestPingReachesARunningLocalDeployment(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipped in short mode")
	}

	if _, err := exec.LookPath(launcher.DefaultExecutable); err != nil {
		t.Skip("no launcher on PATH")
	}

	client := launcher.New(launcher.DefaultExecutable)

	deployments, err := client.Deployments(t.Context(), false)
	if err != nil {
		t.Skipf("cannot list deployments: %v", err)
	}

	for _, deployment := range deployments {
		if deployment.Status != "database_ready" {
			continue
		}

		info, err := client.Info(t.Context(), deployment.Path)
		require.NoError(t, err)
		require.NotNil(t, info.Connection)

		storedPassword, err := database.ReadPassword(deployment.Path)
		require.NoError(t, err)

		roundTrip, err := database.Ping(t.Context(), database.Target{
			Host:                       info.Connection.Host,
			Port:                       info.Connection.Port,
			Username:                   info.Connection.Username,
			Password:                   storedPassword,
			InsecureSkipCertValidation: info.Connection.InsecureSkipCertValidation,
		})
		require.NoError(t, err)
		assert.Positive(t, roundTrip)

		return
	}

	t.Skip("no deployment with a ready database")
}

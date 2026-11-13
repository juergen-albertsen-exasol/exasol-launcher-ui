package launcher_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const awsConfiguration = `{
  "infrastructure": {
    "identity": {"name": "aws", "displayName": "Exasol in AWS",
                 "description": "Provisioning in AWS.", "kind": "name", "selector": "name:aws"},
    "options": {"volume-type": "gp3", "cluster-size": 1, "s3-archive-enabled": true}
  },
  "installation": {
    "identity": {"name": "ubuntu", "displayName": "Exasol on Ubuntu",
                 "description": "Installs Exasol."},
    "options": {}
  }
}`

const (
	awsDir     = "/deployments/aws"
	defaultDir = "/deployments/default"
)

func TestConfigurationKeepsTheOptionOrder(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(awsConfiguration, "config", "get", "--json", "--deployment-dir", awsDir)

	configuration, err := launcher.New(fake.Executable).Configuration(t.Context(), awsDir)

	require.NoError(t, err)
	assert.Equal(t, launcher.PresetIdentity{
		Name: "aws", DisplayName: "Exasol in AWS", Description: "Provisioning in AWS.",
	}, configuration.Infrastructure.Identity)
	assert.Equal(t, launcher.Options{
		{Name: "volume-type", Value: json.RawMessage(`"gp3"`)},
		{Name: "cluster-size", Value: json.RawMessage(`1`)},
		{Name: "s3-archive-enabled", Value: json.RawMessage(`true`)},
	}, configuration.Infrastructure.Options)
	assert.Equal(t, launcher.Options{}, configuration.Installation.Options)
}

func TestSetConfigurationPassesOptionFlags(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	args := []string{
		"config", "set", "--cpu-count=4", "--ports=db:8564", "--deployment-dir", defaultDir,
	}
	fake.Respond("", args...)

	err := launcher.New(fake.Executable).SetConfiguration(t.Context(), "/deployments/default",
		[]launcher.Setting{{Name: "cpu-count", Value: "4"}, {Name: "ports", Value: "db:8564"}})

	require.NoError(t, err)
	assert.Equal(t,
		[]string{"config set --cpu-count=4 --ports=db:8564 --deployment-dir " + defaultDir},
		fake.Calls())
}

func TestResetConfigurationNamesTheOptions(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond("", "config", "reset", "cpu-count", "memory-mb", "--deployment-dir", defaultDir)

	err := launcher.New(fake.Executable).ResetConfiguration(t.Context(), "/deployments/default",
		[]string{"cpu-count", "memory-mb"})

	require.NoError(t, err)
}

func TestConfigurationFailuresCarryTheLauncherErrorWithoutUsage(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	args := []string{"config", "set", "--cpu-count=many", "--deployment-dir", defaultDir}
	fake.Fail("Usage:\n\texasol config set [flags]\n\n"+
		"Error: invalid argument \"many\" for \"--cpu-count\" flag: invalid number \"many\"\n",
		1, args...)

	err := launcher.New(fake.Executable).SetConfiguration(t.Context(), "/deployments/default",
		[]launcher.Setting{{Name: "cpu-count", Value: "many"}})

	launcherError, isLauncherError := errors.AsType[*launcher.LauncherError](err)
	require.True(t, isLauncherError, err)
	assert.Equal(t, `invalid argument "many" for "--cpu-count" flag: invalid number "many"`,
		launcherError.Message)
}

package operations_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/exasol/exasol-local-ui/internal/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	defaultDeployment = launcher.Deployment{Name: "default", Path: "/deployments/default"}
	testDeployment    = launcher.Deployment{Name: "test", Path: "/deployments/test"}
)

func startArgs(d launcher.Deployment) []string {
	return []string{"start", "--json", "--deployment-dir", d.Path}
}

func stopArgs(d launcher.Deployment) []string {
	return []string{"stop", "--json", "--deployment-dir", d.Path}
}

func newManager(t *testing.T, fake *launchertest.Fake, finished func()) *operations.Manager {
	t.Helper()

	return operations.NewManager(t.Context(), launcher.New(fake.Executable), finished)
}

func waitUntilFinished(
	t *testing.T,
	manager *operations.Manager,
	operationID string,
) (operations.Snapshot, []string) {
	t.Helper()

	var (
		snapshot operations.Snapshot
		lines    []string
	)

	require.Eventually(t, func() bool {
		var found bool
		snapshot, lines, found = manager.Get(operationID)

		return found && snapshot.Status != operations.StatusRunning
	}, 10*time.Second, 10*time.Millisecond)

	return snapshot, lines
}

func TestStartSucceedsWithProgressAndResult(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.RespondWithProgress("Starting the local VM\nWaiting for the database\n",
		`{"deploymentState": "running", "databaseReady": true}`, startArgs(defaultDeployment)...)

	var finished atomic.Int32

	manager := newManager(t, fake, func() { finished.Add(1) })

	started, err := manager.Start(defaultDeployment, operations.KindStart, "")
	require.NoError(t, err)
	assert.Equal(t, operations.StatusRunning, started.Status)
	assert.Equal(t, "default", started.Deployment)
	assert.Equal(t, operations.KindStart, started.Kind)

	snapshot, lines := waitUntilFinished(t, manager, started.ID)

	assert.Equal(t, operations.StatusSucceeded, snapshot.Status)
	assert.Equal(
		t,
		&launcher.LifecycleResult{DeploymentState: "running", DatabaseReady: true},
		snapshot.Result,
	)
	assert.NotNil(t, snapshot.FinishedAt)
	assert.Equal(t, []string{"Starting the local VM", "Waiting for the database"}, lines)
	assert.Eventually(
		t,
		func() bool { return finished.Load() == 1 },
		time.Second,
		10*time.Millisecond,
	)
}

func TestFailureCarriesTheLauncherMessage(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Fail("Stopping the VM\nError: the VM did not stop\n", 1, stopArgs(defaultDeployment)...)

	manager := newManager(t, fake, nil)

	started, err := manager.Start(defaultDeployment, operations.KindStop, "")
	require.NoError(t, err)

	snapshot, _ := waitUntilFinished(t, manager, started.ID)

	assert.Equal(t, operations.StatusFailed, snapshot.Status)
	assert.Contains(t, snapshot.Error, "Error: the VM did not stop")
	assert.Nil(t, snapshot.Result)
}

func TestOneOperationPerDeployment(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.RespondWithProgress(
		"",
		`{"deploymentState": "running", "databaseReady": true}`,
		startArgs(defaultDeployment)...)
	fake.RespondWithProgress(
		"",
		`{"deploymentState": "stopped", "databaseReady": false}`,
		stopArgs(testDeployment)...)
	fake.Hold(startArgs(defaultDeployment)...)

	manager := newManager(t, fake, nil)

	first, err := manager.Start(defaultDeployment, operations.KindStart, "")
	require.NoError(t, err)

	_, err = manager.Start(defaultDeployment, operations.KindStop, "")
	require.ErrorIs(t, err, operations.ErrBusy)

	other, err := manager.Start(testDeployment, operations.KindStop, "")
	require.NoError(t, err)

	fake.Release(startArgs(defaultDeployment)...)
	waitUntilFinished(t, manager, first.ID)
	waitUntilFinished(t, manager, other.ID)

	_, err = manager.Start(defaultDeployment, operations.KindStart, "")
	require.NoError(t, err)
}

func TestProgressAndErrorsWithholdTheSecret(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Fail(
		"connecting with s3cret\nError: login s3cret rejected\n",
		1,
		startArgs(defaultDeployment)...)

	manager := newManager(t, fake, nil)

	started, err := manager.Start(defaultDeployment, operations.KindStart, "s3cret")
	require.NoError(t, err)

	snapshot, lines := waitUntilFinished(t, manager, started.ID)

	assert.Equal(t, []string{"connecting with ********", "Error: login ******** rejected"}, lines)
	assert.NotContains(t, snapshot.Error, "s3cret")
}

func TestListShowsNewestFirstAndForgetsOldFinishedOperations(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.RespondWithProgress(
		"",
		`{"deploymentState": "stopped", "databaseReady": false}`,
		stopArgs(defaultDeployment)...)

	manager := newManager(t, fake, nil)
	operations.SetKeep(manager, 2)

	ids := make([]string, 0, 3)

	for range 3 {
		started, err := manager.Start(defaultDeployment, operations.KindStop, "")
		require.NoError(t, err)
		waitUntilFinished(t, manager, started.ID)

		ids = append(ids, started.ID)
	}

	list := manager.List()
	require.Len(t, list, 2)
	assert.Equal(t, ids[2], list[0].ID)
	assert.Equal(t, ids[1], list[1].ID)

	_, _, found := manager.Get(ids[0])
	assert.False(t, found)
}

func TestWatchersHearProgressAndTheEnd(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.RespondWithProgress(
		"one\n",
		`{"deploymentState": "running", "databaseReady": true}`,
		startArgs(defaultDeployment)...)
	fake.Hold(startArgs(defaultDeployment)...)

	manager := newManager(t, fake, nil)

	started, err := manager.Start(defaultDeployment, operations.KindStart, "")
	require.NoError(t, err)

	changes, stop := manager.Watch(started.ID)
	defer stop()

	fake.Release(startArgs(defaultDeployment)...)

	require.Eventually(t, func() bool {
		select {
		case <-changes:
		default:
		}

		snapshot, _, _ := manager.Get(started.ID)

		return snapshot.Status == operations.StatusSucceeded
	}, 10*time.Second, 10*time.Millisecond)

	_, lines, _ := manager.Since(started.ID, 1)
	assert.Empty(t, lines)
}

func TestWatchingAnUnknownOperationEndsAtOnce(t *testing.T) {
	t.Parallel()

	manager := newManager(t, launchertest.NewFake(t), nil)

	changes, stop := manager.Watch("unknown")
	defer stop()

	_, open := <-changes
	assert.False(t, open)
}

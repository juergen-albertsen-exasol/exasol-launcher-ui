package sql_test

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/sql"
	"github.com/exasol/exasol-local-ui/internal/sql/sqltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	deployment = "default"
	password   = "s3cretPassw0rd"
)

var target = database.Target{Host: "127.0.0.1", Port: 8563, Username: "sys", Password: password}

func openSession(t *testing.T) (*sql.Manager, *sqltest.Fake, string) {
	t.Helper()

	fake := sqltest.NewFake()
	manager := sql.NewManager(fake)
	t.Cleanup(func() { _ = manager.Close(t.Context()) })

	id, err := manager.Open(t.Context(), deployment, target)
	require.NoError(t, err)

	return manager, fake, id
}

func TestSessionsHaveDistinctRandomIDs(t *testing.T) {
	t.Parallel()

	manager, fake, first := openSession(t)

	second, err := manager.Open(t.Context(), deployment, target)

	require.NoError(t, err)
	assert.Len(t, first, 32)
	assert.NotEqual(t, first, second)
	assert.Equal(t, []database.Target{target, target}, fake.Opened())
}

func TestAQueryReportsItsColumnsRowsAndOpenSchema(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	columns := []sql.Column{
		{Name: "REGION_NAME", Type: "VARCHAR(100) UTF8"},
		{Name: "ORDERS", Type: "DECIMAL(18,0)"},
	}
	rows := [][]sql.Value{
		{sqltest.Text("EMEA"), sqltest.Text("12")},
		{nil, sqltest.Text("3")},
	}
	fake.Respond("SELECT 1", sqltest.Response{Columns: columns, Rows: rows})

	outcome, err := manager.Run(t.Context(), deployment, sessionID, "SELECT 1")

	require.NoError(t, err)
	assert.Equal(t, &sql.ResultSet{Columns: columns, Rows: rows, Total: 2}, outcome.ResultSet)
	assert.Nil(t, outcome.Schema)
}

func TestASessionKeepsItsOpenSchema(t *testing.T) {
	t.Parallel()

	manager, _, sessionID := openSession(t)

	opened, err := manager.Run(t.Context(), deployment, sessionID, "OPEN SCHEMA RETAIL")
	require.NoError(t, err)

	other, err := manager.Open(t.Context(), deployment, target)
	require.NoError(t, err)

	same, err := manager.Run(t.Context(), deployment, sessionID, "SELECT CURRENT_SCHEMA")
	require.NoError(t, err)

	fresh, err := manager.Run(t.Context(), deployment, other, "SELECT CURRENT_SCHEMA")
	require.NoError(t, err)

	assert.Equal(t, "RETAIL", *opened.Schema)
	assert.Equal(t, "RETAIL", *same.Schema)
	assert.Nil(t, fresh.Schema)
}

func TestALargeResultIsCapped(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("SELECT N FROM BIG", sqltest.Response{
		Columns: []sql.Column{{Name: "N", Type: "DECIMAL(18,0)"}},
		Total:   2_964_624,
		Generate: func(index int64) []sql.Value {
			return []sql.Value{sqltest.Text(strconv.FormatInt(index, 10))}
		},
	})

	outcome, err := manager.Run(t.Context(), deployment, sessionID, "SELECT N FROM BIG")

	require.NoError(t, err)
	assert.Len(t, outcome.ResultSet.Rows, sql.MaxRows)
	assert.Equal(t, int64(2_964_624), outcome.ResultSet.Total)
	assert.True(t, outcome.ResultSet.Truncated)
}

func TestAStatementWithoutResultSetReportsAffectedRows(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("DELETE FROM T", sqltest.Response{RowCount: 12})

	outcome, err := manager.Run(t.Context(), deployment, sessionID, "DELETE FROM T")

	require.NoError(t, err)
	assert.Nil(t, outcome.ResultSet)
	assert.Equal(t, int64(12), outcome.RowCount)
}

func TestAFailingStatementKeepsTheSession(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	failure := &sql.StatementError{Code: "42000", Message: "object X not found"}
	fake.Respond("SELECT X", sqltest.Response{Err: failure})
	fake.Respond("SELECT 1", sqltest.Response{RowCount: 0})

	_, err := manager.Run(t.Context(), deployment, sessionID, "SELECT X")
	require.ErrorIs(t, err, failure)

	_, err = manager.Run(t.Context(), deployment, sessionID, "SELECT 1")
	require.NoError(t, err)
}

func TestABrokenConnectionEndsTheSession(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("SELECT 1", sqltest.Response{
		Err: errors.New("connection lost while logged in with " + password),
	})

	_, err := manager.Run(t.Context(), deployment, sessionID, "SELECT 1")

	statementErr, ok := errors.AsType[*sql.StatementError](err)
	require.True(t, ok)
	assert.Empty(t, statementErr.Code)
	assert.NotContains(t, statementErr.Message, password)

	_, err = manager.Run(t.Context(), deployment, sessionID, "SELECT 1")
	require.ErrorIs(t, err, sql.ErrNoSession)
	assert.Zero(t, fake.OpenConnections())
}

func TestASecondStatementWhileOneRunsIsRefused(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	block := make(chan struct{})
	fake.Respond("SELECT SLOW", sqltest.Response{RowCount: 1, Block: block})

	done := runInBackground(t, manager, sessionID, "SELECT SLOW")
	<-fake.Running()

	_, err := manager.Run(t.Context(), deployment, sessionID, "SELECT SLOW")
	require.ErrorIs(t, err, sql.ErrBusy)

	close(block)
	require.NoError(t, <-done)
}

func TestAClosedSessionIsGone(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)

	require.NoError(t, manager.CloseSession(t.Context(), deployment, sessionID))

	_, err := manager.Run(t.Context(), deployment, sessionID, "SELECT 1")
	require.ErrorIs(t, err, sql.ErrNoSession)
	require.ErrorIs(t, manager.CloseSession(t.Context(), deployment, sessionID), sql.ErrNoSession)
	assert.Zero(t, fake.OpenConnections())
}

func TestASessionBelongsToItsDeployment(t *testing.T) {
	t.Parallel()

	manager, _, sessionID := openSession(t)

	_, err := manager.Run(t.Context(), "test", sessionID, "SELECT 1")
	require.ErrorIs(t, err, sql.ErrNoSession)
	require.ErrorIs(t, manager.CloseSession(t.Context(), "test", sessionID), sql.ErrNoSession)
}

func runInBackground(
	t *testing.T,
	manager *sql.Manager,
	sessionID, statement string,
) <-chan error {
	t.Helper()

	done := make(chan error, 1)

	go func() {
		_, err := manager.Run(t.Context(), deployment, sessionID, statement)
		done <- err
	}()

	return done
}

func TestCancelAbortsTheRunningStatementAndKeepsTheSession(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("SELECT SLOW", sqltest.Response{RowCount: 1, Block: make(chan struct{})})
	fake.Respond("SELECT 1", sqltest.Response{RowCount: 0})

	done := runInBackground(t, manager, sessionID, "SELECT SLOW")
	<-fake.Running()

	require.NoError(t, manager.Cancel(t.Context(), deployment, sessionID))

	statementErr, ok := errors.AsType[*sql.StatementError](<-done)
	require.True(t, ok)
	assert.Equal(t, "R0003", statementErr.Code)
	assert.Contains(t, fake.Executed(), "KILL STATEMENT IN SESSION 1")

	_, err := manager.Run(t.Context(), deployment, sessionID, "SELECT 1")
	require.NoError(t, err)
	assert.Equal(t, 1, fake.OpenConnections())
}

func TestCancelWithoutARunningStatementDoesNothing(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)

	require.NoError(t, manager.Cancel(t.Context(), deployment, sessionID))
	require.ErrorIs(t, manager.Cancel(t.Context(), "test", sessionID), sql.ErrNoSession)
	assert.Equal(t, []string{"SELECT CURRENT_SESSION"}, fake.Executed())
}

func TestClosingASessionAbortsItsRunningStatement(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("SELECT SLOW", sqltest.Response{RowCount: 1, Block: make(chan struct{})})

	done := runInBackground(t, manager, sessionID, "SELECT SLOW")
	<-fake.Running()

	require.NoError(t, manager.CloseSession(t.Context(), deployment, sessionID))
	require.Error(t, <-done)
	assert.Zero(t, fake.OpenConnections())
}

func TestIdleSessionsCloseAfterThirtyMinutes(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)

	sql.AdvanceClock(manager, 29*time.Minute)

	_, err := manager.Run(t.Context(), deployment, sessionID, "SELECT CURRENT_SCHEMA")
	require.NoError(t, err)

	sql.AdvanceClock(manager, 29*time.Minute)
	assert.Equal(t, 1, fake.OpenConnections())

	sql.AdvanceClock(manager, 31*time.Minute)
	assert.Zero(t, fake.OpenConnections())

	_, err = manager.Run(t.Context(), deployment, sessionID, "SELECT CURRENT_SCHEMA")
	require.ErrorIs(t, err, sql.ErrNoSession)
}

func TestClosingTheManagerClosesEverySession(t *testing.T) {
	t.Parallel()

	manager, fake, _ := openSession(t)
	_, err := manager.Open(t.Context(), deployment, target)
	require.NoError(t, err)

	require.NoError(t, manager.Close(t.Context()))

	assert.Zero(t, fake.OpenConnections())
	assert.True(t, fake.Closed())

	_, err = manager.Open(t.Context(), deployment, target)
	require.ErrorIs(t, err, sql.ErrNoSession)
}

func TestAConnectionFailureIsRedacted(t *testing.T) {
	t.Parallel()

	fake := sqltest.NewFake()
	fake.FailConnect(errors.New("login as sys with " + password + " failed"))

	_, err := sql.NewManager(fake).Open(t.Context(), deployment, target)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), password)
}

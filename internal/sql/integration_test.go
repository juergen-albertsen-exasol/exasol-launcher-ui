//go:build integration

package sql_test

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const integrationSchema = "EXASOL_LOCAL_UI_IT"

// localTarget returns the database of a local deployment whose database is
// ready, and skips the test when there is none.
func localTarget(t *testing.T) (string, database.Target) {
	t.Helper()

	if _, err := exec.LookPath(launcher.DefaultExecutable); err != nil {
		t.Skip("no launcher on PATH")
	}

	client := launcher.New(launcher.DefaultExecutable)

	deployments, err := client.Deployments(t.Context(), false)
	require.NoError(t, err)

	for _, deployment := range deployments {
		if deployment.Status != "database_ready" || deployment.Infrastructure != "local" {
			continue
		}

		info, err := client.Info(t.Context(), deployment.Path)
		require.NoError(t, err)
		require.NotNil(t, info.Connection)

		storedPassword, err := database.ReadPassword(deployment.Path)
		require.NoError(t, err)

		return deployment.Name, database.Target{
			Host:                       info.Connection.Host,
			Port:                       info.Connection.Port,
			Username:                   info.Connection.Username,
			Password:                   storedPassword,
			InsecureSkipCertValidation: info.Connection.InsecureSkipCertValidation,
		}
	}

	t.Skip("no local deployment with a ready database")

	return "", database.Target{}
}

func run(t *testing.T, manager *sql.Manager, name, sessionID, statement string) sql.Outcome {
	t.Helper()

	outcome, err := manager.Run(t.Context(), name, sessionID, statement)
	require.NoError(t, err, statement)

	return outcome
}

func TestStatementsRunAgainstALocalDeployment(t *testing.T) {
	t.Parallel()

	name, target := localTarget(t)
	manager := sql.NewManager(sql.NewDriverOpener())
	t.Cleanup(func() { _ = manager.Close(context.Background()) })

	sessionID, err := manager.Open(t.Context(), name, target)
	require.NoError(t, err)

	run(t, manager, name, sessionID, "CREATE SCHEMA IF NOT EXISTS "+integrationSchema)
	t.Cleanup(func() {
		_, _ = manager.Run(t.Context(), name, sessionID,
			"DROP SCHEMA IF EXISTS "+integrationSchema+" CASCADE")
	})

	opened := run(t, manager, name, sessionID, "OPEN SCHEMA "+integrationSchema)
	require.NotNil(t, opened.Schema)
	assert.Equal(t, integrationSchema, *opened.Schema)

	run(t, manager, name, sessionID,
		"CREATE OR REPLACE TABLE REGIONS (REGION_ID DECIMAL(9,0), "+
			"REGION_NAME VARCHAR(100) UTF8, AMOUNT DECIMAL(36,2))")
	inserted := run(t, manager, name, sessionID,
		"INSERT INTO REGIONS VALUES (1, 'EMEA', 12345678901234567890.25), (2, NULL, -1)")
	assert.Equal(t, int64(2), inserted.RowCount)

	query := run(t, manager, name, sessionID,
		"SELECT REGION_NAME, AMOUNT FROM REGIONS ORDER BY REGION_ID")
	require.NotNil(t, query.ResultSet)
	assert.Equal(t, []sql.Column{
		{Name: "REGION_NAME", Type: "VARCHAR(100) UTF8"},
		{Name: "AMOUNT", Type: "DECIMAL(36,2)"},
	}, query.ResultSet.Columns)
	assert.Equal(t, int64(2), query.ResultSet.Total)
	assert.Equal(t, "EMEA", *query.ResultSet.Rows[0][0])
	assert.Equal(t, "12345678901234567890.25", *query.ResultSet.Rows[0][1])
	assert.Nil(t, query.ResultSet.Rows[1][0])

	deleted := run(t, manager, name, sessionID, "DELETE FROM REGIONS WHERE AMOUNT < 0")
	assert.Equal(t, int64(1), deleted.RowCount)

	large := run(t, manager, name, sessionID, "SELECT RANGE_VALUE FROM VALUES BETWEEN 1 AND 25000")
	require.NotNil(t, large.ResultSet)
	assert.Len(t, large.ResultSet.Rows, sql.MaxRows)
	assert.Equal(t, int64(25000), large.ResultSet.Total)
	assert.True(t, large.ResultSet.Truncated)
	assert.Equal(t, "10000", *large.ResultSet.Rows[sql.MaxRows-1][0])

	_, err = manager.Run(t.Context(), name, sessionID, "SELECT REGOIN_NAME\nFROM REGIONS")
	statementErr, ok := errors.AsType[*sql.StatementError](err)
	require.True(t, ok, err)
	assert.Equal(t, "42000", statementErr.Code)
	assert.Contains(t, statementErr.Message, "REGOIN_NAME")
	assert.Equal(t, 1, statementErr.Line)
	assert.Equal(t, 8, statementErr.Column)
}

func TestCancelAndExportAgainstALocalDeployment(t *testing.T) {
	t.Parallel()

	name, target := localTarget(t)
	manager := sql.NewManager(sql.NewDriverOpener())
	t.Cleanup(func() { _ = manager.Close(context.Background()) })

	sessionID, err := manager.Open(t.Context(), name, target)
	require.NoError(t, err)

	done := make(chan error, 1)

	go func() {
		_, err := manager.Run(t.Context(), name, sessionID,
			"SELECT COUNT(*) FROM (SELECT 1 FROM VALUES BETWEEN 1 AND 100000) A, "+
				"(SELECT 1 FROM VALUES BETWEEN 1 AND 100000) B, "+
				"(SELECT 1 FROM VALUES BETWEEN 1 AND 10000) C")
		done <- err
	}()

	time.Sleep(time.Second)
	require.NoError(t, manager.Cancel(t.Context(), name, sessionID))

	statementErr, ok := errors.AsType[*sql.StatementError](<-done)
	require.True(t, ok)
	assert.Equal(t, "R0003", statementErr.Code)

	run(t, manager, name, sessionID, "OPEN SCHEMA SYS")

	exportID, err := manager.CreateExport(name, sessionID,
		"SELECT RANGE_VALUE FROM VALUES BETWEEN 1 AND 25000")
	require.NoError(t, err)

	err = manager.Export(t.Context(), name, exportID, func(rows sql.Rows) error {
		assert.Equal(t, 25000, countRows(t, rows))

		return nil
	})
	require.NoError(t, err)
}

func TestSchemasOfALocalDeployment(t *testing.T) {
	t.Parallel()

	name, target := localTarget(t)
	manager := sql.NewManager(sql.NewDriverOpener())
	t.Cleanup(func() { _ = manager.Close(context.Background()) })

	const schemaName = "EXASOL_LOCAL_UI_META"

	sessionID, err := manager.Open(t.Context(), name, target)
	require.NoError(t, err)

	run(t, manager, name, sessionID, "CREATE SCHEMA IF NOT EXISTS "+schemaName)
	t.Cleanup(func() {
		_, _ = manager.Run(context.Background(), name, sessionID,
			"DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	})
	run(t, manager, name, sessionID, "CREATE OR REPLACE TABLE "+schemaName+
		".REGIONS (REGION_ID DECIMAL(9,0), REGION_NAME VARCHAR(100) UTF8)")
	run(t, manager, name, sessionID, "CREATE OR REPLACE VIEW "+schemaName+
		".REGION_NAMES AS SELECT REGION_NAME FROM "+schemaName+".REGIONS")

	schemas, err := manager.Schemas(t.Context(), name, target)
	require.NoError(t, err)
	assert.Contains(t, schemas, sql.Schema{Name: schemaName, Objects: []sql.Object{
		{Name: "REGIONS", Kind: "table"},
		{Name: "REGION_NAMES", Kind: "view"},
	}})

	schema, err := manager.Schema(t.Context(), name, target, schemaName)
	require.NoError(t, err)
	assert.Equal(t, []sql.Column{
		{Name: "REGION_ID", Type: "DECIMAL(9,0)"},
		{Name: "REGION_NAME", Type: "VARCHAR(100) UTF8"},
	}, schema.Objects[0].Columns)

	_, err = manager.Schema(t.Context(), name, target, "NO_SUCH_SCHEMA")
	require.ErrorIs(t, err, sql.ErrNoSchema)
}

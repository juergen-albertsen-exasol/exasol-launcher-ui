//go:build integration

package sql_test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/sql"
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

func run(t *testing.T, manager *sql.Manager, name, id, statement string) sql.Outcome {
	t.Helper()

	outcome, err := manager.Run(t.Context(), name, id, statement)
	require.NoError(t, err, statement)

	return outcome
}

func TestStatementsRunAgainstALocalDeployment(t *testing.T) {
	name, target := localTarget(t)
	manager := sql.NewManager(sql.NewDriverOpener())
	t.Cleanup(func() { _ = manager.Close() })

	id, err := manager.Open(t.Context(), name, target)
	require.NoError(t, err)

	run(t, manager, name, id, "CREATE SCHEMA IF NOT EXISTS "+integrationSchema)
	t.Cleanup(func() {
		_, _ = manager.Run(t.Context(), name, id,
			"DROP SCHEMA IF EXISTS "+integrationSchema+" CASCADE")
	})

	opened := run(t, manager, name, id, "OPEN SCHEMA "+integrationSchema)
	require.NotNil(t, opened.Schema)
	assert.Equal(t, integrationSchema, *opened.Schema)

	run(t, manager, name, id,
		"CREATE OR REPLACE TABLE REGIONS (REGION_ID DECIMAL(9,0), "+
			"REGION_NAME VARCHAR(100) UTF8, AMOUNT DECIMAL(36,2))")
	inserted := run(t, manager, name, id,
		"INSERT INTO REGIONS VALUES (1, 'EMEA', 12345678901234567890.25), (2, NULL, -1)")
	assert.Equal(t, int64(2), inserted.RowCount)

	query := run(t, manager, name, id,
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

	deleted := run(t, manager, name, id, "DELETE FROM REGIONS WHERE AMOUNT < 0")
	assert.Equal(t, int64(1), deleted.RowCount)

	large := run(t, manager, name, id, "SELECT RANGE_VALUE FROM VALUES BETWEEN 1 AND 25000")
	require.NotNil(t, large.ResultSet)
	assert.Len(t, large.ResultSet.Rows, sql.MaxRows)
	assert.Equal(t, int64(25000), large.ResultSet.Total)
	assert.True(t, large.ResultSet.Truncated)
	assert.Equal(t, "10000", *large.ResultSet.Rows[sql.MaxRows-1][0])

	_, err = manager.Run(t.Context(), name, id, "SELECT REGOIN_NAME\nFROM REGIONS")
	statementErr, ok := err.(*sql.StatementError)
	require.True(t, ok, err)
	assert.Equal(t, "42000", statementErr.Code)
	assert.Contains(t, statementErr.Message, "REGOIN_NAME")
	assert.Equal(t, 1, statementErr.Line)
	assert.Equal(t, 8, statementErr.Column)
}

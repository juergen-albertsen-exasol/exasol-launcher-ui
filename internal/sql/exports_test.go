package sql_test

import (
	"errors"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/sql"
	"github.com/exasol/exasol-launcher-ui/internal/sql/sqltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countRows(t *testing.T, rows sql.Rows) int {
	t.Helper()

	count := 0

	for {
		_, err := rows.Next(t.Context())
		if errors.Is(err, io.EOF) {
			return count
		}

		require.NoError(t, err)

		count++
	}
}

func TestAnExportReadsEveryRowWithTheOpenSchemaOnItsOwnConnection(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("SELECT N FROM BIG", sqltest.Response{
		Columns: []sql.Column{{Name: "N", Type: "DECIMAL(18,0)"}},
		Total:   25_000,
		Generate: func(index int64) []sql.Value {
			return []sql.Value{sqltest.Text(strconv.FormatInt(index, 10))}
		},
	})
	_, err := manager.Run(t.Context(), deployment, sessionID, "OPEN SCHEMA RETAIL")
	require.NoError(t, err)

	exportID, err := manager.CreateExport(deployment, sessionID, "SELECT N FROM BIG")
	require.NoError(t, err)

	var rows int

	err = manager.Export(t.Context(), deployment, exportID, func(result sql.Rows) error {
		rows = countRows(t, result)

		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 25_000, rows)
	assert.Len(t, fake.Opened(), 2)
	assert.Contains(t, fake.Executed(), `OPEN SCHEMA "RETAIL"`)
	assert.Equal(t, 1, fake.OpenConnections())
}

func TestAnExportServesOneDownload(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("SELECT 1", sqltest.Response{Columns: []sql.Column{{Name: "1"}}})

	exportID, err := manager.CreateExport(deployment, sessionID, "SELECT 1")
	require.NoError(t, err)

	read := func(sql.Rows) error { return nil }

	require.NoError(t, manager.Export(t.Context(), deployment, exportID, read))
	require.ErrorIs(t, manager.Export(t.Context(), deployment, exportID, read), sql.ErrNoExport)
}

func TestAnExportExpiresAfterFiveMinutes(t *testing.T) {
	t.Parallel()

	manager, _, sessionID := openSession(t)

	exportID, err := manager.CreateExport(deployment, sessionID, "SELECT 1")
	require.NoError(t, err)

	sql.AdvanceClock(manager, 5*time.Minute)

	err = manager.Export(t.Context(), deployment, exportID, func(sql.Rows) error { return nil })
	require.ErrorIs(t, err, sql.ErrNoExport)
}

func TestAnExportNeedsASessionAndAResultSet(t *testing.T) {
	t.Parallel()

	manager, fake, sessionID := openSession(t)
	fake.Respond("DELETE FROM T", sqltest.Response{RowCount: 3})

	_, err := manager.CreateExport(deployment, "unknown", "SELECT 1")
	require.ErrorIs(t, err, sql.ErrNoSession)

	exportID, err := manager.CreateExport(deployment, sessionID, "DELETE FROM T")
	require.NoError(t, err)

	err = manager.Export(t.Context(), deployment, exportID, func(sql.Rows) error { return nil })

	statementErr, ok := errors.AsType[*sql.StatementError](err)
	require.True(t, ok)
	assert.Equal(t, sql.ErrNoResultSet.Error(), statementErr.Message)
}

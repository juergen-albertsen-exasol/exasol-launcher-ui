package sql

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/database"
)

const exportLifetime = 5 * time.Minute

var (
	ErrNoExport    = errors.New("the export does not exist, has expired, or was downloaded")
	ErrNoResultSet = errors.New("the statement returns no result set")
)

// export is a statement whose full result one download reads.
type export struct {
	deployment string
	target     database.Target
	statement  string
	schema     *string
	expires    time.Time
}

// CreateExport records statement for one download of its full result with
// the open schema of the session sessionID, and returns the export's id.
func (m *Manager) CreateExport(deployment, sessionID, statement string) (string, error) {
	found, err := m.find(deployment, sessionID)
	if err != nil {
		return "", err
	}

	pending := export{
		deployment: deployment,
		target:     found.target,
		statement:  statement,
		schema:     found.openSchema(),
		expires:    m.now().Add(exportLifetime),
	}
	exportID := newRandomID()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.exports[exportID] = pending

	return exportID, nil
}

// Export runs the statement of the export exportID of deployment on a
// separate connection and passes its result set to read. The export is
// consumed even when the statement fails.
func (m *Manager) Export(
	ctx context.Context,
	deployment, exportID string,
	read func(Rows) error,
) error {
	pending, err := m.consume(deployment, exportID)
	if err != nil {
		return err
	}

	conn, err := m.opener.Open(ctx, deployment, pending.target)
	if err != nil {
		return database.Redact(err, pending.target.Password)
	}
	defer conn.Close()

	if pending.schema != nil {
		_, err := conn.Execute(ctx, "OPEN SCHEMA "+quoteIdentifier(*pending.schema), nil)
		if err != nil {
			return asStatementError(err, pending.target.Password)
		}
	}

	resultSet := false

	_, err = conn.Execute(ctx, pending.statement, func(rows Rows) error {
		resultSet = true

		return read(rows)
	})
	if err != nil {
		return asStatementError(err, pending.target.Password)
	}

	if !resultSet {
		return &StatementError{Message: ErrNoResultSet.Error()}
	}

	return nil
}

func (m *Manager) consume(deployment, exportID string) (export, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	pending, ok := m.exports[exportID]
	if !ok || pending.deployment != deployment {
		return export{}, ErrNoExport
	}

	delete(m.exports, exportID)

	if !m.now().Before(pending.expires) {
		return export{}, ErrNoExport
	}

	return pending, nil
}

// quoteIdentifier quotes name as a delimited identifier.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

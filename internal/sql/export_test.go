package sql

import (
	"context"
	"time"
)

var (
	StatementErrorOf = statementError
	TextValue        = textValue
	FormatTypeOf     = formatType
)

// AdvanceClock moves the manager's clock forward by duration and closes the
// sessions that became idle.
func AdvanceClock(m *Manager, duration time.Duration) {
	now := m.now()
	m.now = func() time.Time { return now.Add(duration) }
	m.closeIdle(context.Background())
}

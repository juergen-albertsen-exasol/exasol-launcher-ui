package sql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
)

const (
	// MaxRows is the number of rows a statement's result reports.
	MaxRows = 10_000

	idleTimeout    = 30 * time.Minute
	janitorPeriod  = time.Minute
	sessionIDBytes = 16
)

var (
	ErrNoSession = errors.New("the SQL session does not exist or has ended")
	ErrBusy      = errors.New("another statement runs in this SQL session")
)

// Outcome is what running a statement yielded: a result set, or the number
// of rows the statement affected.
type Outcome struct {
	ResultSet *ResultSet
	RowCount  int64
	Duration  time.Duration
	// Schema is the session's open schema after the statement, or nil.
	Schema *string
}

// ResultSet holds up to MaxRows rows of a statement's result.
type ResultSet struct {
	Columns   []Column
	Rows      [][]Value
	Total     int64
	Truncated bool
}

// Manager holds the SQL sessions of all deployments.
type Manager struct {
	opener      Opener
	idleTimeout time.Duration
	now         func() time.Time

	mutex    sync.Mutex
	sessions map[string]*session
	closed   bool

	janitor     sync.Once
	stopJanitor chan struct{}
}

type session struct {
	deployment string
	conn       Conn
	password   string

	mutex    sync.Mutex
	lastUsed time.Time
	running  bool
	closed   bool
}

func NewManager(opener Opener) *Manager {
	return &Manager{
		opener:      opener,
		idleTimeout: idleTimeout,
		now:         time.Now,
		sessions:    map[string]*session{},
		stopJanitor: make(chan struct{}),
	}
}

// Open connects a new session to the database of deployment and returns
// its id.
func (m *Manager) Open(
	ctx context.Context,
	deployment string,
	target database.Target,
) (string, error) {
	m.janitor.Do(func() { go m.closeIdleSessions() })

	conn, err := m.opener.Open(ctx, deployment, target)
	if err != nil {
		return "", database.Redact(err, target.Password)
	}

	sessionID := newSessionID()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		_ = conn.Close()

		return "", ErrNoSession
	}

	m.sessions[sessionID] = &session{
		deployment: deployment,
		conn:       conn,
		password:   target.Password,
		lastUsed:   m.now(),
	}

	return sessionID, nil
}

// CloseSession ends the session id of deployment.
func (m *Manager) CloseSession(deployment, sessionID string) error {
	m.mutex.Lock()

	found, ok := m.sessions[sessionID]
	if !ok || found.deployment != deployment {
		m.mutex.Unlock()

		return ErrNoSession
	}

	delete(m.sessions, sessionID)
	m.mutex.Unlock()

	found.close()

	return nil
}

// Run runs statement in the session id of deployment. A failure of the
// statement is a *StatementError; when the connection failed, the session
// ends.
func (m *Manager) Run(
	ctx context.Context,
	deployment, sessionID, statement string,
) (Outcome, error) {
	found, err := m.claim(deployment, sessionID)
	if err != nil {
		return Outcome{}, err
	}
	defer found.release(m.now())

	// Statements ignore the request's cancellation: the driver gives up on a
	// cancelled request without reading its response, which leaves the
	// connection out of step.
	ctx = context.WithoutCancel(ctx)

	outcome, err := found.run(ctx, statement)
	if err == nil {
		return outcome, nil
	}

	statementErr := asStatementError(err, found.password)
	if statementErr.Code == "" {
		_ = m.CloseSession(deployment, sessionID)
	}

	return Outcome{}, statementErr
}

// Close ends every session and closes the opener.
func (m *Manager) Close() error {
	m.mutex.Lock()

	if m.closed {
		m.mutex.Unlock()

		return nil
	}

	m.closed = true
	sessions := m.sessions
	m.sessions = map[string]*session{}
	close(m.stopJanitor)
	m.mutex.Unlock()

	for _, found := range sessions {
		found.close()
	}

	return m.opener.Close()
}

func (m *Manager) claim(deployment, sessionID string) (*session, error) {
	m.mutex.Lock()
	found, ok := m.sessions[sessionID]
	m.mutex.Unlock()

	if !ok || found.deployment != deployment {
		return nil, ErrNoSession
	}

	found.mutex.Lock()
	defer found.mutex.Unlock()

	if found.closed {
		return nil, ErrNoSession
	}

	if found.running {
		return nil, ErrBusy
	}

	found.running = true
	found.lastUsed = m.now()

	return found, nil
}

func (m *Manager) closeIdleSessions() {
	ticker := time.NewTicker(janitorPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopJanitor:
			return
		case <-ticker.C:
			m.closeIdle()
		}
	}
}

func (m *Manager) closeIdle() {
	deadline := m.now().Add(-m.idleTimeout)

	m.mutex.Lock()

	var idle []*session

	for id, found := range m.sessions {
		if found.idleSince(deadline) {
			idle = append(idle, found)
			delete(m.sessions, id)
		}
	}

	m.mutex.Unlock()

	for _, found := range idle {
		found.close()
	}
}

func (s *session) idleSince(deadline time.Time) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return !s.running && s.lastUsed.Before(deadline)
}

func (s *session) run(ctx context.Context, statement string) (Outcome, error) {
	var resultSet *ResultSet

	start := time.Now()

	rowCount, err := s.conn.Execute(ctx, statement, func(rows Rows) error {
		var readErr error

		resultSet, readErr = readResultSet(ctx, rows)

		return readErr
	})
	if err != nil {
		return Outcome{}, err
	}

	outcome := Outcome{ResultSet: resultSet, Duration: time.Since(start)}
	if resultSet == nil {
		outcome.RowCount = rowCount
	}

	outcome.Schema, err = s.openSchema(ctx)

	return outcome, err
}

func readResultSet(ctx context.Context, rows Rows) (*ResultSet, error) {
	resultSet := &ResultSet{Columns: rows.Columns(), Total: rows.Total(), Rows: [][]Value{}}

	for len(resultSet.Rows) < MaxRows {
		row, err := rows.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, err
		}

		resultSet.Rows = append(resultSet.Rows, row)
	}

	resultSet.Truncated = resultSet.Total > int64(len(resultSet.Rows))

	return resultSet, nil
}

func (s *session) openSchema(ctx context.Context) (*string, error) {
	var schema Value

	_, err := s.conn.Execute(ctx, "SELECT CURRENT_SCHEMA", func(rows Rows) error {
		row, err := rows.Next(ctx)
		if err != nil {
			return err
		}

		schema = row[0]

		return nil
	})

	return schema, err
}

// release ends a statement's run, and closes the connection when the
// session ended meanwhile.
func (s *session) release(now time.Time) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.running = false
	s.lastUsed = now

	if s.closed {
		_ = s.conn.Close()
	}
}

// close ends the session; a running statement's release closes the
// connection once the statement returns.
func (s *session) close() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return
	}

	s.closed = true

	if !s.running {
		_ = s.conn.Close()
	}
}

func newSessionID() string {
	id := make([]byte, sessionIDBytes)
	_, _ = rand.Read(id)

	return hex.EncodeToString(id)
}

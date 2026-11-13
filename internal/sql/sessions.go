package sql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/database"
)

const (
	// MaxRows is the number of rows a statement's result reports.
	MaxRows = 10_000

	idleTimeout   = 30 * time.Minute
	janitorPeriod = time.Minute
	randomIDBytes = 16
	abortedCode   = "R0003"
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

// Manager holds the SQL sessions and pending exports of all deployments.
type Manager struct {
	opener      Opener
	idleTimeout time.Duration
	now         func() time.Time

	mutex    sync.Mutex
	sessions map[string]*session
	exports  map[string]export
	closed   bool

	janitor     sync.Once
	stopJanitor chan struct{}
}

type session struct {
	deployment string
	target     database.Target
	conn       Conn
	// databaseSession is the database's id of the session, which KILL
	// STATEMENT names.
	databaseSession string
	opener          Opener

	// mutex is held while a statement is killed, so that the kill cannot
	// reach a later statement.
	mutex    sync.Mutex
	lastUsed time.Time
	schema   *string
	running  bool
	closed   bool
}

func NewManager(opener Opener) *Manager {
	return &Manager{
		opener:      opener,
		idleTimeout: idleTimeout,
		now:         time.Now,
		sessions:    map[string]*session{},
		exports:     map[string]export{},
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
	m.janitor.Do(func() { go m.closeIdleSessions(context.WithoutCancel(ctx)) })

	conn, err := m.opener.Open(ctx, deployment, target)
	if err != nil {
		return "", database.Redact(err, target.Password)
	}

	databaseSession, err := firstValue(ctx, conn, "SELECT CURRENT_SESSION")
	if err != nil || databaseSession == nil {
		_ = conn.Close()

		return "", asStatementError(
			errors.Join(errors.New("cannot read the database session"), err), target.Password)
	}

	sessionID := newRandomID()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		_ = conn.Close()

		return "", ErrNoSession
	}

	m.sessions[sessionID] = &session{
		deployment:      deployment,
		target:          target,
		conn:            conn,
		databaseSession: *databaseSession,
		opener:          m.opener,
		lastUsed:        m.now(),
	}

	return sessionID, nil
}

// CloseSession ends the session sessionID of deployment, aborting its
// running statement.
func (m *Manager) CloseSession(ctx context.Context, deployment, sessionID string) error {
	found, err := m.remove(deployment, sessionID)
	if err != nil {
		return err
	}

	found.close(ctx)

	return nil
}

// Run runs statement in the session sessionID of deployment. A failure of
// the statement is a *StatementError; when the connection failed, the
// session ends.
func (m *Manager) Run(
	ctx context.Context,
	deployment, sessionID, statement string,
) (Outcome, error) {
	found, err := m.claim(deployment, sessionID)
	if err != nil {
		return Outcome{}, err
	}

	// Statements ignore the request's cancellation: the driver gives up on a
	// cancelled request without reading its response, which leaves the
	// connection out of step. Cancel kills the statement instead.
	statementCtx := context.WithoutCancel(ctx)

	outcome, err := found.run(statementCtx, statement)
	found.release(m.now(), outcome.Schema, err == nil)

	if err == nil {
		return outcome, nil
	}

	statementErr := asStatementError(err, found.target.Password)
	if statementErr.Code == "" {
		if removed, removeErr := m.remove(deployment, sessionID); removeErr == nil {
			removed.close(ctx)
		}
	}

	return Outcome{}, statementErr
}

// Cancel aborts the statement running in the session sessionID of
// deployment, if any.
func (m *Manager) Cancel(ctx context.Context, deployment, sessionID string) error {
	found, err := m.find(deployment, sessionID)
	if err != nil {
		return err
	}

	found.mutex.Lock()
	defer found.mutex.Unlock()

	if !found.running {
		return nil
	}

	return found.kill(ctx)
}

// Close ends every session and closes the opener.
func (m *Manager) Close(ctx context.Context) error {
	m.mutex.Lock()

	if m.closed {
		m.mutex.Unlock()

		return nil
	}

	m.closed = true
	sessions := m.sessions
	m.sessions = map[string]*session{}
	m.exports = map[string]export{}
	close(m.stopJanitor)
	m.mutex.Unlock()

	for _, found := range sessions {
		found.close(ctx)
	}

	return m.opener.Close()
}

func (m *Manager) find(deployment, sessionID string) (*session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	found, ok := m.sessions[sessionID]
	if !ok || found.deployment != deployment {
		return nil, ErrNoSession
	}

	return found, nil
}

func (m *Manager) remove(deployment, sessionID string) (*session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	found, ok := m.sessions[sessionID]
	if !ok || found.deployment != deployment {
		return nil, ErrNoSession
	}

	delete(m.sessions, sessionID)

	return found, nil
}

func (m *Manager) claim(deployment, sessionID string) (*session, error) {
	found, err := m.find(deployment, sessionID)
	if err != nil {
		return nil, err
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

func (m *Manager) closeIdleSessions(ctx context.Context) {
	ticker := time.NewTicker(janitorPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopJanitor:
			return
		case <-ticker.C:
			m.closeIdle(ctx)
		}
	}
}

func (m *Manager) closeIdle(ctx context.Context) {
	now := m.now()
	deadline := now.Add(-m.idleTimeout)

	m.mutex.Lock()

	var idle []*session

	for sessionID, found := range m.sessions {
		if found.idleSince(deadline) {
			idle = append(idle, found)
			delete(m.sessions, sessionID)
		}
	}

	for exportID, pending := range m.exports {
		if !now.Before(pending.expires) {
			delete(m.exports, exportID)
		}
	}

	m.mutex.Unlock()

	for _, found := range idle {
		found.close(ctx)
	}
}

func (s *session) idleSince(deadline time.Time) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return !s.running && s.lastUsed.Before(deadline)
}

func (s *session) openSchema() *string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.schema
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

	outcome.Schema, err = firstValue(ctx, s.conn, "SELECT CURRENT_SCHEMA")

	// A cancel that arrives as the statement finishes kills this query
	// instead, which must not fail the statement.
	if statementErr, ok := errors.AsType[*StatementError](err); ok &&
		statementErr.Code == abortedCode {
		outcome.Schema, err = firstValue(ctx, s.conn, "SELECT CURRENT_SCHEMA")
	}

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

// firstValue returns the first value of the first row of query's result.
func firstValue(ctx context.Context, conn Conn, query string) (Value, error) {
	var value Value

	_, err := conn.Execute(ctx, query, func(rows Rows) error {
		row, err := rows.Next(ctx)
		if err != nil {
			return err
		}

		value = row[0]

		return nil
	})

	return value, err
}

// release ends a statement's run, and closes the connection when the
// session ended meanwhile.
func (s *session) release(now time.Time, schema *string, succeeded bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.running = false
	s.lastUsed = now

	if succeeded {
		s.schema = schema
	}

	if s.closed {
		_ = s.conn.Close()
	}
}

// close ends the session and aborts its running statement, whose release
// then closes the connection.
func (s *session) close(ctx context.Context) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return
	}

	s.closed = true

	if s.running {
		_ = s.kill(ctx)

		return
	}

	_ = s.conn.Close()
}

// kill aborts the session's running statement from a separate connection.
func (s *session) kill(ctx context.Context) error {
	conn, err := s.opener.Open(ctx, s.deployment, s.target)
	if err != nil {
		return database.Redact(err, s.target.Password)
	}
	defer conn.Close()

	_, err = conn.Execute(ctx, "KILL STATEMENT IN SESSION "+s.databaseSession, nil)
	if err != nil {
		return asStatementError(err, s.target.Password)
	}

	return nil
}

func newRandomID() string {
	id := make([]byte, randomIDBytes)
	_, _ = rand.Read(id)

	return hex.EncodeToString(id)
}

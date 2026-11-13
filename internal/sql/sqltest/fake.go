// Package sqltest provides a fake database for tests of SQL sessions.
package sqltest

import (
	"context"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/exasol/exasol-launcher-ui/internal/database"
	"github.com/exasol/exasol-launcher-ui/internal/sql"
)

const (
	runningBuffer = 100
	killPrefix    = "KILL STATEMENT IN SESSION "
)

// Response is the fake database's answer to a statement.
type Response struct {
	// Columns makes the statement yield a result set.
	Columns []sql.Column
	// Rows are the result set's rows; Generate takes over beyond them.
	Rows [][]sql.Value
	// Total is the number of rows; it defaults to len(Rows).
	Total int64
	// Generate returns the row at index when Rows holds fewer.
	Generate func(index int64) []sql.Value
	// RowCount is the number of affected rows of other statements.
	RowCount int64
	// Err makes the statement fail.
	Err error
	// Block holds the statement until it is closed.
	Block chan struct{}
}

// Fake is a database whose answers tests define. Unknown statements fail
// with error code 42000, while COMMIT and ROLLBACK succeed. Each connection
// keeps its own open schema, which OPEN SCHEMA sets and SELECT
// CURRENT_SCHEMA reports, and KILL STATEMENT IN SESSION aborts a statement
// that blocks.
type Fake struct {
	mutex      sync.Mutex
	responses  map[string]Response
	executed   []string
	opened     []database.Target
	sessions   map[string]*conn
	open       int
	closed     bool
	connectErr error
	running    chan string
	// transactions counts the connections OpenTransaction opened.
	transactions int
}

func NewFake() *Fake {
	return &Fake{
		responses: map[string]Response{},
		sessions:  map[string]*conn{},
		running:   make(chan string, runningBuffer),
	}
}

// Respond defines the answer to statement.
func (f *Fake) Respond(statement string, response Response) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.responses[statement] = response
}

// FailConnect makes connecting fail with err.
func (f *Fake) FailConnect(err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.connectErr = err
}

// Executed returns the statements run so far, in order.
func (f *Fake) Executed() []string {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return append([]string(nil), f.executed...)
}

// Opened returns the targets of the connections opened so far.
func (f *Fake) Opened() []database.Target {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return append([]database.Target(nil), f.opened...)
}

// OpenConnections returns the number of connections not yet closed.
func (f *Fake) OpenConnections() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return f.open
}

// Closed reports whether the opener was closed.
func (f *Fake) Closed() bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return f.closed
}

// Running returns the statements as they start blocking on a Response's
// Block.
func (f *Fake) Running() <-chan string {
	return f.running
}

func (f *Fake) Open(_ context.Context, _ string, target database.Target) (sql.Conn, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.connectErr != nil {
		return nil, f.connectErr
	}

	f.opened = append(f.opened, target)
	f.open++

	opened := &conn{fake: f, session: strconv.Itoa(len(f.opened))}
	f.sessions[opened.session] = opened

	return opened, nil
}

// OpenTransaction opens a connection as Open does, which Transactions
// counts.
func (f *Fake) OpenTransaction(
	ctx context.Context,
	deployment string,
	target database.Target,
) (sql.Conn, error) {
	opened, err := f.Open(ctx, deployment, target)
	if err != nil {
		return nil, err
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.transactions++

	return opened, nil
}

// Transactions returns the number of connections OpenTransaction opened.
func (f *Fake) Transactions() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return f.transactions
}

func (f *Fake) Close() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.closed = true

	return nil
}

// Text returns value as a non-NULL sql.Value.
func Text(value string) sql.Value {
	return &value
}

type conn struct {
	fake    *Fake
	session string
	schema  sql.Value
	// abort is set while a statement blocks.
	abort chan struct{}
}

func (c *conn) Execute(
	_ context.Context,
	statement string,
	read func(sql.Rows) error,
) (int64, error) {
	c.fake.mutex.Lock()
	c.fake.executed = append(c.fake.executed, statement)
	response, found := c.fake.responses[statement]
	c.fake.mutex.Unlock()

	switch {
	case found:
	case statement == "SELECT CURRENT_SCHEMA":
		return 0, read(&rows{response: Response{
			Columns: []sql.Column{{Name: "CURRENT_SCHEMA", Type: "VARCHAR(128) UTF8"}},
			Rows:    [][]sql.Value{{c.schema}},
		}})
	case statement == "SELECT CURRENT_SESSION":
		return 0, read(&rows{response: Response{
			Columns: []sql.Column{{Name: "CURRENT_SESSION", Type: "DECIMAL(20,0)"}},
			Rows:    [][]sql.Value{{Text(c.session)}},
		}})
	case strings.HasPrefix(statement, killPrefix):
		c.fake.kill(strings.TrimPrefix(statement, killPrefix))

		return 0, nil
	case statement == "COMMIT" || statement == "ROLLBACK":
		return 0, nil
	case strings.HasPrefix(statement, "OPEN SCHEMA "):
		schema := strings.Trim(strings.TrimPrefix(statement, "OPEN SCHEMA "), `"`)
		c.schema = &schema

		return 0, nil
	default:
		return 0, &sql.StatementError{Code: "42000", Message: "unknown statement " + statement}
	}

	if response.Block != nil {
		if err := c.block(statement, response.Block); err != nil {
			return 0, err
		}
	}

	if response.Err != nil {
		return 0, response.Err
	}

	if response.Columns == nil {
		return response.RowCount, nil
	}

	return 0, read(&rows{response: response})
}

func (f *Fake) kill(session string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if target, found := f.sessions[session]; found && target.abort != nil {
		close(target.abort)
		target.abort = nil
	}
}

func (c *conn) Close() error {
	c.fake.mutex.Lock()
	defer c.fake.mutex.Unlock()

	c.fake.open--

	return nil
}

func (c *conn) block(statement string, release chan struct{}) error {
	abort := make(chan struct{})

	c.fake.mutex.Lock()
	c.abort = abort
	c.fake.mutex.Unlock()

	c.fake.running <- statement

	select {
	case <-release:
		c.fake.mutex.Lock()
		c.abort = nil
		c.fake.mutex.Unlock()

		return nil
	case <-abort:
		return &sql.StatementError{Code: "R0003", Message: "Client requested execution abort."}
	}
}

type rows struct {
	response Response
	position int64
}

func (r *rows) Columns() []sql.Column {
	return r.response.Columns
}

func (r *rows) Total() int64 {
	if r.response.Total > 0 {
		return r.response.Total
	}

	return int64(len(r.response.Rows))
}

func (r *rows) Next(context.Context) ([]sql.Value, error) {
	if r.position >= r.Total() {
		return nil, io.EOF
	}

	index := r.position
	r.position++

	if index < int64(len(r.response.Rows)) {
		return r.response.Rows[index], nil
	}

	return r.response.Generate(index), nil
}

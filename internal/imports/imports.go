package imports

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/sql"
)

// Statuses of imports and of their files.
const (
	Waiting   = "waiting"
	Running   = "running"
	Succeeded = "succeeded"
	Failed    = "failed"
	Cancelled = "cancelled"
)

// leadingStatements counts the statements before the first IMPORT: CREATE
// SCHEMA and CREATE TABLE.
const leadingStatements = 2

var (
	ErrNoImport = errors.New("the import does not exist")
	// ErrInvalid reports a request whose schema, table, or columns are not
	// usable.
	ErrInvalid = errors.New("the import request is invalid")
)

// Opener opens the connections of imports.
type Opener interface {
	sql.Opener
	// OpenTransaction connects with autocommit off, and runs IMPORT … FROM
	// LOCAL by sending the files it names from this computer.
	OpenTransaction(
		ctx context.Context, deployment string, target database.Target,
	) (sql.Conn, error)
}

// ColumnSpec is a column of the table an import creates.
type ColumnSpec struct {
	Name string
	Type string
}

// Request asks to import a source into a new table; without Columns, the
// table has the suggested columns.
type Request struct {
	Source  Source
	Schema  string
	Table   string
	Columns []ColumnSpec
}

// ImportFile is a file of an import and how far it got.
type ImportFile struct {
	Name   string
	Status string
	Rows   *int64
}

// Import is an import with its progress.
type Import struct {
	ID       string
	Source   SourceSummary
	Schema   string
	Table    string
	Status   string
	Started  time.Time
	Duration time.Duration
	Files    []ImportFile
	// Rows is the total once the import succeeded.
	Rows *int64
	// SQL holds the statements the import runs.
	SQL []string
	// Message is the database's message for a failed import.
	Message string
}

// Manager suggests and runs the imports of all deployments, keeping them
// in memory while the server runs.
type Manager struct {
	opener  Opener
	uploads *Uploads
	now     func() time.Time

	mutex   sync.Mutex
	imports []*job
	closed  bool
	running sync.WaitGroup
}

type job struct {
	deployment string
	target     database.Target
	uploadID   string

	mutex     sync.Mutex
	state     Import
	session   string
	executing bool
	cancelled bool
	// committing is set once COMMIT runs, which a cancel no longer stops.
	committing bool
	finishedAt time.Time
}

func NewManager(opener Opener, uploads *Uploads) *Manager {
	return &Manager{opener: opener, uploads: uploads, now: time.Now}
}

// Start begins importing the request's source into a new table of the
// database of deployment. A request with problems is a *ConflictError, an
// unusable one wraps ErrInvalid.
func (m *Manager) Start(
	ctx context.Context,
	deployment string,
	target database.Target,
	request Request,
) (Import, error) {
	opened, err := m.open(deployment, request.Source)
	if err != nil {
		return Import{}, err
	}
	defer opened.Close()

	columns, err := requestedColumns(request, opened.columns)
	if err != nil {
		return Import{}, err
	}

	taken, err := m.takenNames(ctx, deployment, target, request.Schema)
	if err != nil {
		return Import{}, err
	}

	if taken[request.Table] {
		alternative := availableTable(request.Table, taken)

		return Import{}, &ConflictError{Problem: Problem{
			Code: TableTaken,
			Message: request.Schema + "." + request.Table +
				" exists. Choose another name, such as " + alternative + ".",
			Alternative: alternative,
		}}
	}

	upload, err := m.uploads.Acquire(deployment, request.Source.UploadID)
	if err != nil {
		return Import{}, err
	}

	created := m.newJob(deployment, target, upload, request, columns, opened)

	m.mutex.Lock()
	if m.closed {
		m.mutex.Unlock()
		m.uploads.Release(upload.ID, false)

		return Import{}, ErrNoImport
	}

	m.imports = append(m.imports, created)
	m.running.Add(1)
	m.mutex.Unlock()

	go func() {
		defer m.running.Done()

		m.run(context.WithoutCancel(ctx), created)
	}()

	return created.view(m.now()), nil
}

// ConflictError is a problem that keeps a source from being imported.
type ConflictError struct {
	Problem Problem
}

func (c *ConflictError) Error() string {
	return c.Problem.Message
}

func requestedColumns(request Request, inferred []Column) ([]ColumnSpec, error) {
	if strings.TrimSpace(request.Schema) == "" || strings.TrimSpace(request.Table) == "" {
		return nil, fmt.Errorf("%w: the import needs a schema and a table", ErrInvalid)
	}

	if message := UnsupportedMessage(inferred); message != "" {
		problem := Problem{Code: UnsupportedColumns, Message: message}

		for _, column := range inferred {
			if column.Unsupported != "" {
				problem.Columns = append(problem.Columns, column.Name)
			}
		}

		return nil, &ConflictError{Problem: problem}
	}

	if request.Columns == nil {
		columns := make([]ColumnSpec, len(inferred))
		for index, column := range inferred {
			columns[index] = ColumnSpec{Name: column.Name, Type: column.Type}
		}

		return columns, nil
	}

	// The database maps the file's columns to the table's by position.
	if len(request.Columns) != len(inferred) {
		return nil, fmt.Errorf("%w: the table needs the files' %d columns",
			ErrInvalid, len(inferred))
	}

	for _, column := range request.Columns {
		if strings.TrimSpace(column.Name) == "" || strings.TrimSpace(column.Type) == "" {
			return nil, fmt.Errorf("%w: every column needs a name and a type", ErrInvalid)
		}
	}

	return request.Columns, nil
}

// List returns the imports of deployment, newest first.
func (m *Manager) List(deployment string) []Import {
	m.mutex.Lock()
	jobs := append([]*job(nil), m.imports...)
	m.mutex.Unlock()

	now := m.now()
	views := []Import{}

	for _, found := range slices.Backward(jobs) {
		if found.deployment == deployment {
			views = append(views, found.view(now))
		}
	}

	return views
}

// Get returns the import id of deployment.
func (m *Manager) Get(deployment, importID string) (Import, error) {
	found, err := m.find(deployment, importID)
	if err != nil {
		return Import{}, err
	}

	return found.view(m.now()), nil
}

// Cancel stops the running import id of deployment, which then rolls
// back; a finished import stays as it is.
func (m *Manager) Cancel(ctx context.Context, deployment, importID string) error {
	found, err := m.find(deployment, importID)
	if err != nil {
		return err
	}

	return m.cancelJob(ctx, found)
}

// Close cancels the running imports and waits for them to roll back.
func (m *Manager) Close(ctx context.Context) {
	m.mutex.Lock()
	m.closed = true
	jobs := append([]*job(nil), m.imports...)
	m.mutex.Unlock()

	for _, running := range jobs {
		_ = m.cancelJob(ctx, running)
	}

	m.running.Wait()
}

func (m *Manager) newJob(
	deployment string,
	target database.Target,
	upload Upload,
	request Request,
	columns []ColumnSpec,
	opened *sourceFiles,
) *job {
	table := Quote(request.Schema) + "." + Quote(request.Table)
	definitions := make([]string, len(columns))

	for index, column := range columns {
		definitions[index] = Quote(column.Name) + " " + column.Type
	}

	statements := make([]string, 0, leadingStatements+len(opened.paths)+1)
	statements = append(statements,
		"CREATE SCHEMA IF NOT EXISTS "+Quote(request.Schema),
		"CREATE TABLE "+table+" ("+strings.Join(definitions, ", ")+")")

	files := make([]ImportFile, len(opened.paths))

	for index, path := range opened.paths {
		statements = append(statements,
			"IMPORT INTO "+table+" FROM LOCAL PARQUET FILE "+quoteLiteral(path))
		files[index] = ImportFile{Name: opened.names[index], Status: Waiting}
	}

	statements = append(statements, "COMMIT")

	return &job{
		deployment: deployment,
		target:     target,
		uploadID:   upload.ID,
		state: Import{
			ID:      newRandomID(),
			Source:  opened.summary,
			Schema:  request.Schema,
			Table:   request.Table,
			Status:  Running,
			Started: m.now(),
			Files:   files,
			SQL:     statements,
		},
	}
}

func (m *Manager) find(deployment, importID string) (*job, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, found := range m.imports {
		if found.state.ID == importID && found.deployment == deployment {
			return found, nil
		}
	}

	return nil, ErrNoImport
}

func (m *Manager) cancelJob(ctx context.Context, found *job) error {
	found.mutex.Lock()
	defer found.mutex.Unlock()

	if found.state.Status != Running || found.cancelled || found.committing {
		return nil
	}

	found.cancelled = true

	if !found.executing || found.session == "" {
		return nil
	}

	// The job's mutex keeps the next statement from starting, so the kill
	// reaches only the running one.
	conn, err := m.opener.Open(ctx, found.deployment, found.target)
	if err != nil {
		return database.Redact(err, found.target.Password)
	}
	defer conn.Close()

	_, err = conn.Execute(ctx, "KILL STATEMENT IN SESSION "+found.session, nil)

	return database.Redact(err, found.target.Password)
}

func (m *Manager) run(ctx context.Context, running *job) {
	succeeded := false

	defer func() { m.uploads.Release(running.uploadID, succeeded) }()

	conn, err := m.opener.OpenTransaction(ctx, running.deployment, running.target)
	if err != nil {
		running.finish(m.now(), Failed, database.Redact(err, running.target.Password).Error())

		return
	}
	defer conn.Close()

	session, err := firstValue(ctx, conn, "SELECT CURRENT_SESSION")
	if err != nil {
		running.finish(m.now(), Failed, running.message(err))

		return
	}

	running.mutex.Lock()
	running.session = session
	running.mutex.Unlock()

	var total int64

	statements := running.state.SQL
	for index, statement := range statements {
		file := index - leadingStatements
		commit := index == len(statements)-1

		if !running.begin(file, commit) {
			rollback(ctx, conn)
			running.finish(m.now(), Cancelled, "")

			return
		}

		rows, err := conn.Execute(ctx, statement, nil)
		if err != nil {
			rollback(ctx, conn)

			if running.end(file, nil) {
				running.finish(m.now(), Cancelled, "")
			} else {
				running.finish(m.now(), Failed, running.message(err))
			}

			return
		}

		if running.end(file, &rows) && !commit {
			rollback(ctx, conn)
			running.finish(m.now(), Cancelled, "")

			return
		}

		if file >= 0 && file < len(running.state.Files) {
			total += rows
		}
	}

	succeeded = true

	running.mutex.Lock()
	running.state.Rows = &total
	running.mutex.Unlock()
	running.finish(m.now(), Succeeded, "")
}

func rollback(ctx context.Context, conn sql.Conn) {
	_, _ = conn.Execute(ctx, "ROLLBACK", nil)
}

func firstValue(ctx context.Context, conn sql.Conn, query string) (string, error) {
	var value string

	_, err := conn.Execute(ctx, query, func(rows sql.Rows) error {
		row, err := rows.Next(ctx)
		if err != nil {
			return err
		}

		if row[0] != nil {
			value = *row[0]
		}

		return nil
	})

	return value, err
}

// begin marks the statement that loads file, if any, as running, and
// reports false when the import was cancelled.
func (j *job) begin(file int, commit bool) bool {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	if j.cancelled {
		return false
	}

	j.executing = true
	j.committing = commit

	if file >= 0 && file < len(j.state.Files) {
		j.state.Files[file].Status = Running
	}

	return true
}

// end records the rows of the statement that loaded file, if any, and
// reports whether the import was cancelled meanwhile.
func (j *job) end(file int, rows *int64) bool {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	j.executing = false

	if file >= 0 && file < len(j.state.Files) {
		switch {
		case j.cancelled:
			j.state.Files[file].Status = Cancelled
		case rows == nil:
			j.state.Files[file].Status = Failed
		default:
			j.state.Files[file].Status = Succeeded
			j.state.Files[file].Rows = rows
		}
	}

	return j.cancelled
}

var transferURL = regexp.MustCompile(`\[File: [^\]]*\]`)

// message returns the database's message for err, naming the uploaded file
// where the database names its transfer.
func (j *job) message(err error) string {
	message := database.Redact(err, j.target.Password).Error()
	if statementErr, ok := errors.AsType[*sql.StatementError](err); ok {
		message = database.Redact(errors.New(statementErr.Message), j.target.Password).Error()
	}

	j.mutex.Lock()
	defer j.mutex.Unlock()

	for _, file := range j.state.Files {
		if file.Status == Failed {
			return transferURL.ReplaceAllString(message, "[File: "+file.Name+"]")
		}
	}

	return message
}

func (j *job) finish(now time.Time, status, message string) {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	j.state.Status = status
	j.state.Message = message
	j.finishedAt = now

	if status == Cancelled {
		for index := range j.state.Files {
			if j.state.Files[index].Status == Waiting || j.state.Files[index].Status == Running {
				j.state.Files[index].Status = Cancelled
			}
		}
	}
}

func (j *job) view(now time.Time) Import {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	view := j.state
	view.Files = append([]ImportFile(nil), j.state.Files...)

	end := now
	if !j.finishedAt.IsZero() {
		end = j.finishedAt
	}

	view.Duration = end.Sub(j.state.Started)

	return view
}

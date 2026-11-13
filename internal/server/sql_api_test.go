package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/exasol/exasol-local-ui/internal/sql"
	"github.com/exasol/exasol-local-ui/internal/sql/sqltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sessionsURL = "/api/v1/deployments/default/sql/sessions"

type sqlFixture struct {
	t        *testing.T
	handler  http.Handler
	database *sqltest.Fake
}

func newSQLFixture(t *testing.T, fake *launchertest.Fake) *sqlFixture {
	t.Helper()

	database := sqltest.NewFake()
	manager := sql.NewManager(database)
	t.Cleanup(func() { _ = manager.Close(t.Context()) })

	return &sqlFixture{
		t:        t,
		database: database,
		handler:  NewHandler(Config{Launcher: launcher.New(fake.Executable), SQL: manager}),
	}
}

func (f *sqlFixture) call(method, target, body string) *httptest.ResponseRecorder {
	f.t.Helper()

	request := httptest.NewRequestWithContext(
		f.t.Context(), method, target, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)

	assert.NotContains(f.t, recorder.Body.String(), storedPassword)

	return recorder
}

func (f *sqlFixture) openSession() string {
	f.t.Helper()

	response := f.call(http.MethodPost, sessionsURL, "")
	require.Equal(f.t, http.StatusCreated, response.Code, response.Body.String())

	return response.Header().Get("Location")
}

func (f *sqlFixture) run(session, statement string) *httptest.ResponseRecorder {
	f.t.Helper()

	body, err := json.Marshal(statementRequest{SQL: statement})
	require.NoError(f.t, err)

	return f.call(http.MethodPost, session+"/statements", string(body))
}

func TestOpeningASessionNamesItsURL(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))

	response := fixture.call(http.MethodPost, sessionsURL, "")

	require.Equal(t, http.StatusCreated, response.Code)

	session := decode[sqlSessionView](t, response)
	assert.Len(t, session.ID, 32)
	assert.Equal(t, sessionsURL+"/"+session.ID, response.Header().Get("Location"))
	assert.Equal(t, storedPassword, fixture.database.Opened()[0].Password)
}

func TestASessionKeepsItsOpenSchemaAcrossStatements(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond("SELECT ID FROM SALES", sqltest.Response{
		Columns: []sql.Column{{Name: "ID", Type: "DECIMAL(18,0)"}},
		Rows:    [][]sql.Value{{sqltest.Text("1")}},
	})
	session := fixture.openSession()

	opened := fixture.run(session, "OPEN SCHEMA RETAIL")
	query := fixture.run(session, "SELECT ID FROM SALES")

	assert.JSONEq(t, `"RETAIL"`, string(decode[map[string]json.RawMessage](t, opened)["schema"]))
	assert.JSONEq(t, `"RETAIL"`, string(decode[map[string]json.RawMessage](t, query)["schema"]))
	assert.Len(t, fixture.database.Opened(), 1)
}

func TestOpeningASessionOnAStoppedDatabaseConflicts(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fixture := newSQLFixture(t, fake)

	response := fixture.call(http.MethodPost, "/api/v1/deployments/test/sql/sessions", "")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, "the database of test is not running", decode[problem](t, response).Detail)
	assert.Empty(t, fixture.database.Opened())
}

func TestAnUnreachableDatabaseConflicts(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.FailConnect(fmt.Errorf("connection refused for %s", storedPassword))

	response := fixture.call(http.MethodPost, sessionsURL, "")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Contains(t, decode[problem](t, response).Detail, "connection refused")
}

func TestSessionsOfAnUnknownDeploymentAreNotFound(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fixture := newSQLFixture(t, fake)

	response := fixture.call(http.MethodPost, "/api/v1/deployments/nope/sql/sessions", "")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestAClosedSessionIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	session := fixture.openSession()

	closed := fixture.call(http.MethodDelete, session, "")
	statement := fixture.run(session, "SELECT 1")
	closedAgain := fixture.call(http.MethodDelete, session, "")

	assert.Equal(t, http.StatusNoContent, closed.Code)
	assert.Equal(t, http.StatusNotFound, statement.Code)
	assert.Equal(t, http.StatusNotFound, closedAgain.Code)
	assert.Zero(t, fixture.database.OpenConnections())
}

func TestAQueryReportsColumnsRowsAndDuration(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	statement := "SELECT REGION_NAME, COUNT(*) AS ORDERS FROM RETAIL.SALES GROUP BY 1"
	fixture.database.Respond(statement, sqltest.Response{
		Columns: []sql.Column{
			{Name: "REGION_NAME", Type: "VARCHAR(100) UTF8"},
			{Name: "ORDERS", Type: "DECIMAL(18,0)"},
		},
		Rows: [][]sql.Value{
			{sqltest.Text("EMEA"), sqltest.Text("1200")},
			{nil, sqltest.Text("7")},
		},
	})

	response := fixture.run(fixture.openSession(), statement)

	require.Equal(t, http.StatusOK, response.Code)

	body := decode[map[string]any](t, response)
	assert.Contains(t, body, "duration_ms")
	delete(body, "duration_ms")
	assert.Equal(t, map[string]any{
		"type": "result_set",
		"columns": []any{
			map[string]any{"name": "REGION_NAME", "type": "VARCHAR(100) UTF8"},
			map[string]any{"name": "ORDERS", "type": "DECIMAL(18,0)"},
		},
		"rows":       []any{[]any{"EMEA", "1200"}, []any{nil, "7"}},
		"total_rows": float64(2),
		"truncated":  false,
		"schema":     nil,
	}, body)
}

func TestALargeResultHoldsTheFirstRows(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond("SELECT N FROM BIG", sqltest.Response{
		Columns: []sql.Column{{Name: "N", Type: "DECIMAL(18,0)"}},
		Total:   2_964_624,
		Generate: func(index int64) []sql.Value {
			return []sql.Value{sqltest.Text(strconv.FormatInt(index, 10))}
		},
	})

	response := fixture.run(fixture.openSession(), "SELECT N FROM BIG")

	require.Equal(t, http.StatusOK, response.Code)

	body := decode[statementView](t, response)
	assert.Len(t, body.Rows, 10_000)
	assert.Equal(t, int64(2_964_624), *body.TotalRows)
	assert.True(t, *body.Truncated)
}

func TestAStatementWithoutResultSetReportsTheAffectedRows(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	statement := "DELETE FROM RETAIL.SALES WHERE AMOUNT < 0"
	fixture.database.Respond(statement, sqltest.Response{RowCount: 12})

	response := fixture.run(fixture.openSession(), statement)

	require.Equal(t, http.StatusOK, response.Code)

	body := decode[map[string]any](t, response)
	assert.Equal(t, "row_count", body["type"])
	assert.InDelta(t, 12, body["row_count"], 0)
	assert.NotContains(t, body, "rows")
}

func TestAFailingStatementNamesCodeMessageAndPosition(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	statement := "SELECT REGOIN_NAME FROM RETAIL.REGIONS"
	fixture.database.Respond(statement, sqltest.Response{Err: &sql.StatementError{
		Code:    "42000",
		Message: "object REGOIN_NAME not found [line 1, column 8]",
		Line:    1,
		Column:  8,
	}})

	response := fixture.run(fixture.openSession(), statement)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Equal(t, "application/problem+json", response.Header().Get("Content-Type"))
	assert.JSONEq(t, `{
	  "type": "about:blank",
	  "title": "Unprocessable Entity",
	  "detail": "object REGOIN_NAME not found [line 1, column 8]",
	  "code": "42000",
	  "line": 1,
	  "column": 8
	}`, response.Body.String())
}

func TestAStatementWhileAnotherRunsConflicts(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	block := make(chan struct{})
	fixture.database.Respond("SELECT SLOW", sqltest.Response{RowCount: 0, Block: block})
	session := fixture.openSession()

	done := make(chan int)

	go func() {
		done <- fixture.run(session, "SELECT SLOW").Code
	}()

	<-fixture.database.Running()

	second := fixture.run(session, "SELECT SLOW")

	close(block)
	assert.Equal(t, http.StatusConflict, second.Code)
	assert.Equal(t, http.StatusOK, <-done)
}

func TestAStatementNeedsABody(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))

	session := fixture.openSession()

	assert.Equal(t, http.StatusBadRequest,
		fixture.call(http.MethodPost, session+"/statements", "{").Code)
	assert.Equal(t, http.StatusBadRequest, fixture.run(session, " \n").Code)
}

func TestCancellingAStatementAnswersItsRequestWithTheCancellation(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond("SELECT SLOW", sqltest.Response{Block: make(chan struct{})})
	fixture.database.Respond("SELECT 1", sqltest.Response{RowCount: 0})
	session := fixture.openSession()

	done := make(chan *httptest.ResponseRecorder)

	go func() {
		done <- fixture.run(session, "SELECT SLOW")
	}()

	<-fixture.database.Running()

	cancelled := fixture.call(http.MethodPost, session+"/cancel", "")
	statement := <-done
	next := fixture.run(session, "SELECT 1")

	assert.Equal(t, http.StatusNoContent, cancelled.Code)
	assert.Equal(t, http.StatusUnprocessableEntity, statement.Code)
	assert.JSONEq(t, `{
	  "type": "about:blank",
	  "title": "Unprocessable Entity",
	  "detail": "Client requested execution abort.",
	  "code": "R0003"
	}`, statement.Body.String())
	assert.Equal(t, http.StatusOK, next.Code)
}

func TestCancellingAnUnknownSessionIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))

	response := fixture.call(http.MethodPost, sessionsURL+"/unknown/cancel", "")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func (f *sqlFixture) createExport(session, statement string) string {
	f.t.Helper()

	body, err := json.Marshal(statementRequest{SQL: statement})
	require.NoError(f.t, err)

	response := f.call(http.MethodPost, session+"/exports", string(body))
	require.Equal(f.t, http.StatusCreated, response.Code, response.Body.String())

	return response.Header().Get("Location")
}

func TestDownloadingALargeResultAsCSV(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond("SELECT N, NAME FROM BIG", sqltest.Response{
		Columns: []sql.Column{
			{Name: "N", Type: "DECIMAL(18,0)"},
			{Name: "NAME", Type: "VARCHAR(10) UTF8"},
		},
		Rows:  [][]sql.Value{{sqltest.Text("0"), sqltest.Text(`a "quoted", name`)}},
		Total: 2_964_624,
		Generate: func(index int64) []sql.Value {
			return []sql.Value{sqltest.Text(strconv.FormatInt(index, 10)), nil}
		},
	})
	session := fixture.openSession()

	location := fixture.createExport(session, "SELECT N, NAME FROM BIG")
	download := fixture.call(http.MethodGet, location, "")

	assert.Regexp(t, `^/api/v1/deployments/default/sql/exports/[0-9a-f]{32}$`, location)
	require.Equal(t, http.StatusOK, download.Code)
	assert.Equal(t, "text/csv; charset=utf-8", download.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename=default-result.csv`,
		download.Header().Get("Content-Disposition"))

	body := download.Body.String()
	assert.Equal(t, 2_964_625, strings.Count(body, "\r\n"))
	assert.True(t, strings.HasPrefix(body, "N,NAME\r\n0,\"a \"\"quoted\"\", name\"\r\n1,\r\n"))
}

func TestAnExportURLServesOneDownload(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond("SELECT 1", sqltest.Response{
		Columns: []sql.Column{{Name: "1", Type: "DECIMAL(1,0)"}},
		Rows:    [][]sql.Value{{sqltest.Text("1")}},
	})

	location := fixture.createExport(fixture.openSession(), "SELECT 1")

	assert.Equal(t, http.StatusOK, fixture.call(http.MethodGet, location, "").Code)
	assert.Equal(t, http.StatusNotFound, fixture.call(http.MethodGet, location, "").Code)
}

func TestAFailingExportAnswersWithTheStatementError(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))

	location := fixture.createExport(fixture.openSession(), "SELECT NOPE")
	response := fixture.call(http.MethodGet, location, "")

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Equal(t, "42000", decode[map[string]any](t, response)["code"])
}

func TestAnExportOfAnUnknownSessionIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))

	response := fixture.call(http.MethodPost, sessionsURL+"/unknown/exports", `{"sql": "SELECT 1"}`)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

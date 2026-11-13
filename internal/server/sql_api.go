package server

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/sql"
)

const (
	databaseReady     = "database_ready"
	maxStatementBytes = 1024 * 1024
	resultSetOutcome  = "result_set"
	rowCountOutcome   = "row_count"
	sqlSessionsPath   = "/api/v1/deployments/{name}/sql/sessions"
	sqlSessionPath    = sqlSessionsPath + "/{id}"
	sqlExportsPath    = "/api/v1/deployments/{name}/sql/exports"
)

type sqlSessionView struct {
	ID string `json:"id"`
}

type statementRequest struct {
	SQL string `json:"sql"`
}

type columnView struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type statementView struct {
	Type       string        `json:"type"`
	Columns    []columnView  `json:"columns,omitempty"`
	Rows       [][]sql.Value `json:"rows,omitempty"`
	TotalRows  *int64        `json:"total_rows,omitempty"`
	Truncated  *bool         `json:"truncated,omitempty"`
	RowCount   *int64        `json:"row_count,omitempty"`
	DurationMs int64         `json:"duration_ms"`
	Schema     *string       `json:"schema"`
}

// statementProblem extends the problem detail with the database's error
// code and the error's position within the statement.
type statementProblem struct {
	problem

	Code   string `json:"code"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

func (a *api) registerSQL(mux *http.ServeMux) {
	mux.HandleFunc("POST "+sqlSessionsPath, a.openSQLSession)
	mux.HandleFunc("DELETE "+sqlSessionPath, a.closeSQLSession)
	mux.HandleFunc("POST "+sqlSessionPath+"/statements", a.runStatement)
	mux.HandleFunc("POST "+sqlSessionPath+"/cancel", a.cancelStatement)
	mux.HandleFunc("POST "+sqlSessionPath+"/exports", a.createExport)
	mux.HandleFunc("GET "+sqlExportsPath+"/{id}", a.downloadExport)
}

// databaseTarget resolves the database of the requested deployment and
// answers 404 for an unknown deployment and 409 when its database is not
// running.
func (a *api) databaseTarget(
	response http.ResponseWriter,
	request *http.Request,
) (launcher.Deployment, database.Target, bool) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return deployment, database.Target{}, false
	}

	target, err := a.resolveTarget(request.Context(), deployment)
	if err != nil {
		writeProblem(response, http.StatusConflict, err.Error())

		return deployment, database.Target{}, false
	}

	return deployment, target, true
}

func (a *api) resolveTarget(
	ctx context.Context,
	deployment launcher.Deployment,
) (database.Target, error) {
	if deployment.Status != databaseReady {
		return database.Target{}, errors.New(
			"the database of " + deployment.Name + " is not running")
	}

	info, err := a.launcher.Info(ctx, deployment.Path)
	if err != nil {
		return database.Target{}, err
	}

	if info.Connection == nil {
		return database.Target{}, errors.New(
			"deployment " + deployment.Name + " reports no connection details")
	}

	password, err := database.ReadPassword(deployment.Path)
	if err != nil {
		return database.Target{}, err
	}

	return database.Target{
		Host:                       info.Connection.Host,
		Port:                       info.Connection.Port,
		Username:                   info.Connection.Username,
		Password:                   password,
		InsecureSkipCertValidation: info.Connection.InsecureSkipCertValidation,
	}, nil
}

func (a *api) openSQLSession(response http.ResponseWriter, request *http.Request) {
	deployment, target, ok := a.databaseTarget(response, request)
	if !ok {
		return
	}

	sessionID, err := a.sql.Open(request.Context(), deployment.Name, target)
	if err != nil {
		writeProblem(response, http.StatusConflict, err.Error())

		return
	}

	response.Header().Set("Location",
		"/api/v1/deployments/"+deployment.Name+"/sql/sessions/"+sessionID)
	writeJSON(response, http.StatusCreated, sqlSessionView{ID: sessionID})
}

func (a *api) closeSQLSession(response http.ResponseWriter, request *http.Request) {
	err := a.sql.CloseSession(request.Context(),
		request.PathValue("name"), request.PathValue("id"))
	if err != nil {
		answerSQLError(response, err)

		return
	}

	response.WriteHeader(http.StatusNoContent)
}

func decodeStatement(response http.ResponseWriter, request *http.Request) (string, bool) {
	var body statementRequest
	if !decodeSizedBody(response, request, &body, maxStatementBytes) {
		return "", false
	}

	if strings.TrimSpace(body.SQL) == "" {
		writeProblem(response, http.StatusBadRequest, "The request holds no statement.")

		return "", false
	}

	return body.SQL, true
}

func (a *api) runStatement(response http.ResponseWriter, request *http.Request) {
	statement, ok := decodeStatement(response, request)
	if !ok {
		return
	}

	outcome, err := a.sql.Run(request.Context(),
		request.PathValue("name"), request.PathValue("id"), statement)
	if err != nil {
		answerSQLError(response, err)

		return
	}

	writeJSON(response, http.StatusOK, newStatementView(outcome))
}

func (a *api) cancelStatement(response http.ResponseWriter, request *http.Request) {
	err := a.sql.Cancel(request.Context(), request.PathValue("name"), request.PathValue("id"))
	if err != nil {
		answerSQLError(response, err)

		return
	}

	response.WriteHeader(http.StatusNoContent)
}

func (a *api) createExport(response http.ResponseWriter, request *http.Request) {
	statement, ok := decodeStatement(response, request)
	if !ok {
		return
	}

	name := request.PathValue("name")

	exportID, err := a.sql.CreateExport(name, request.PathValue("id"), statement)
	if err != nil {
		answerSQLError(response, err)

		return
	}

	response.Header().Set("Location", "/api/v1/deployments/"+name+"/sql/exports/"+exportID)
	writeJSON(response, http.StatusCreated, sqlSessionView{ID: exportID})
}

// downloadExport streams an export's rows as RFC 4180 CSV. A failure after
// the first row aborts the response, so that the download fails rather than
// ends early.
func (a *api) downloadExport(response http.ResponseWriter, request *http.Request) {
	ctx := request.Context()
	name := request.PathValue("name")
	started := false

	err := a.sql.Export(ctx, name, request.PathValue("id"),
		func(rows sql.Rows) error {
			started = true

			response.Header().Set("Content-Type", "text/csv; charset=utf-8")
			response.Header().Set("Content-Disposition",
				mime.FormatMediaType("attachment", map[string]string{
					"filename": name + "-result.csv",
				}))

			return writeCSV(ctx, response, rows)
		})
	if err == nil {
		return
	}

	if started {
		panic(http.ErrAbortHandler)
	}

	if errors.Is(err, sql.ErrNoExport) {
		writeProblem(response, http.StatusNotFound, err.Error())

		return
	}

	answerSQLError(response, err)
}

func writeCSV(ctx context.Context, output io.Writer, rows sql.Rows) error {
	writer := csv.NewWriter(output)
	writer.UseCRLF = true

	columns := rows.Columns()
	record := make([]string, len(columns))

	for index, column := range columns {
		record[index] = column.Name
	}

	if err := writer.Write(record); err != nil {
		return err
	}

	for {
		row, err := rows.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		for index, value := range row {
			record[index] = ""
			if value != nil {
				record[index] = *value
			}
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()

	return writer.Error()
}

func newStatementView(outcome sql.Outcome) statementView {
	view := statementView{
		DurationMs: outcome.Duration.Milliseconds(),
		Schema:     outcome.Schema,
	}

	if outcome.ResultSet == nil {
		view.Type = rowCountOutcome
		view.RowCount = &outcome.RowCount

		return view
	}

	view.Type = resultSetOutcome
	view.Columns = columnViews(outcome.ResultSet.Columns)
	view.Rows = outcome.ResultSet.Rows
	view.TotalRows = &outcome.ResultSet.Total
	view.Truncated = &outcome.ResultSet.Truncated

	return view
}

func columnViews(columns []sql.Column) []columnView {
	views := make([]columnView, len(columns))
	for index, column := range columns {
		views[index] = columnView{Name: column.Name, Type: column.Type}
	}

	return views
}

func answerSQLError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoSession):
		writeProblem(response, http.StatusNotFound, err.Error())
	case errors.Is(err, sql.ErrBusy):
		writeProblem(response, http.StatusConflict, err.Error())
	default:
		if statementErr, ok := errors.AsType[*sql.StatementError](err); ok {
			writeStatementProblem(response, statementErr)

			return
		}

		writeProblem(response, http.StatusInternalServerError, err.Error())
	}
}

func writeStatementProblem(response http.ResponseWriter, err *sql.StatementError) {
	body, marshalErr := json.Marshal(statementProblem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusUnprocessableEntity),
		Detail: err.Message,
		Code:   err.Code,
		Line:   err.Line,
		Column: err.Column,
	})
	if marshalErr != nil {
		writeProblem(response, http.StatusInternalServerError, marshalErr.Error())

		return
	}

	response.Header().Set("Content-Type", "application/problem+json")
	response.WriteHeader(http.StatusUnprocessableEntity)
	_, _ = response.Write(body)
}

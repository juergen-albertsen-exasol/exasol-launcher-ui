package server

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/imports"
	"github.com/exasol/exasol-launcher-ui/internal/sql"
)

const (
	importSuggestionsPath = "/api/v1/deployments/{name}/import-suggestions"
	importsPath           = "/api/v1/deployments/{name}/imports"
	importNotFound        = "import_not_found"
	uploadIncomplete      = "upload_incomplete"
	databaseUnavailable   = "database_unavailable"
	tenths                = 10
)

type sourceView struct {
	Kind     string `json:"kind"`
	UploadID string `json:"upload_id,omitempty"`
}

type suggestionRequest struct {
	Source sourceView `json:"source"`
}

type sourceSummaryView struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

type suggestedColumnView struct {
	Name        string  `json:"name"`
	Type        *string `json:"type"`
	ParquetType string  `json:"parquet_type"`
	Sample      *string `json:"sample"`
}

type problemView struct {
	Code        string   `json:"code"`
	Message     string   `json:"message"`
	Columns     []string `json:"columns,omitempty"`
	Alternative string   `json:"alternative,omitempty"`
}

type suggestionView struct {
	Source     sourceSummaryView     `json:"source"`
	Schema     string                `json:"schema"`
	Table      string                `json:"table"`
	Rows       int64                 `json:"rows"`
	Columns    []suggestedColumnView `json:"columns"`
	Importable bool                  `json:"importable"`
	Problems   []problemView         `json:"problems"`
}

type importRequest struct {
	Source  sourceView         `json:"source"`
	Schema  string             `json:"schema"`
	Table   string             `json:"table"`
	Columns []importColumnView `json:"columns,omitempty"`
}

type importColumnView struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type importFileView struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Rows   *int64 `json:"rows,omitempty"`
}

type importView struct {
	ID      string            `json:"id"`
	Source  sourceSummaryView `json:"source"`
	Schema  string            `json:"schema"`
	Table   string            `json:"table"`
	Status  string            `json:"status"`
	Started string            `json:"started"`
	Seconds float64           `json:"seconds"`
	Files   []importFileView  `json:"files"`
	Rows    *int64            `json:"rows,omitempty"`
	SQL     string            `json:"sql"`
	Message string            `json:"message,omitempty"`
}

func (a *api) registerImports(mux *http.ServeMux) {
	mux.HandleFunc("POST "+importSuggestionsPath, a.suggestImport)
	mux.HandleFunc("POST "+importsPath, a.startImport)
	mux.HandleFunc("GET "+importsPath, a.listImports)
	mux.HandleFunc("GET "+importsPath+"/{id}", a.getImport)
	mux.HandleFunc("DELETE "+importsPath+"/{id}", a.cancelImport)
}

func (a *api) suggestImport(response http.ResponseWriter, request *http.Request) {
	var body suggestionRequest
	if !decodeSizedBody(response, request, &body, maxUploadRequest) {
		return
	}

	deployment, target, ok := a.databaseTarget(response, request)
	if !ok {
		return
	}

	suggestion, err := a.imports.Suggest(request.Context(), deployment.Name, target,
		imports.Source{Kind: body.Source.Kind, UploadID: body.Source.UploadID})
	if err != nil {
		answerImportError(response, err)

		return
	}

	writeJSON(response, http.StatusOK, newSuggestionView(suggestion))
}

func (a *api) startImport(response http.ResponseWriter, request *http.Request) {
	var body importRequest
	if !decodeSizedBody(response, request, &body, maxUploadRequest) {
		return
	}

	deployment, target, ok := a.databaseTarget(response, request)
	if !ok {
		return
	}

	var columns []imports.ColumnSpec
	if body.Columns != nil {
		columns = make([]imports.ColumnSpec, len(body.Columns))
		for index, column := range body.Columns {
			columns[index] = imports.ColumnSpec{Name: column.Name, Type: column.Type}
		}
	}

	started, err := a.imports.Start(request.Context(), deployment.Name, target, imports.Request{
		Source:  imports.Source{Kind: body.Source.Kind, UploadID: body.Source.UploadID},
		Schema:  body.Schema,
		Table:   body.Table,
		Columns: columns,
	})
	if err != nil {
		answerImportError(response, err)

		return
	}

	response.Header().Set("Location",
		"/api/v1/deployments/"+deployment.Name+"/imports/"+started.ID)
	writeJSON(response, http.StatusAccepted, newImportView(started))
}

func (a *api) listImports(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	listed := a.imports.List(deployment.Name)

	views := make([]importView, len(listed))
	for index, found := range listed {
		views[index] = newImportView(found)
	}

	writeJSON(response, http.StatusOK, views)
}

func (a *api) getImport(response http.ResponseWriter, request *http.Request) {
	found, err := a.imports.Get(request.PathValue("name"), request.PathValue("id"))
	if err != nil {
		answerImportError(response, err)

		return
	}

	writeJSON(response, http.StatusOK, newImportView(found))
}

func (a *api) cancelImport(response http.ResponseWriter, request *http.Request) {
	err := a.imports.Cancel(request.Context(), request.PathValue("name"), request.PathValue("id"))
	if err != nil {
		answerImportError(response, err)

		return
	}

	response.WriteHeader(http.StatusNoContent)
}

func newImportView(found imports.Import) importView {
	files := make([]importFileView, len(found.Files))
	for index, file := range found.Files {
		files[index] = importFileView{Name: file.Name, Status: file.Status, Rows: file.Rows}
	}

	return importView{
		ID: found.ID,
		Source: sourceSummaryView{
			Kind:  found.Source.Kind,
			Label: found.Source.Label,
			Files: found.Source.Files,
			Bytes: found.Source.Bytes,
		},
		Schema:  found.Schema,
		Table:   found.Table,
		Status:  found.Status,
		Started: found.Started.UTC().Format(time.RFC3339),
		Seconds: math.Round(found.Duration.Seconds()*tenths) / tenths,
		Files:   files,
		Rows:    found.Rows,
		SQL:     strings.Join(found.SQL, ";\n") + ";",
		Message: found.Message,
	}
}

func newSuggestionView(suggestion imports.Suggestion) suggestionView {
	columns := make([]suggestedColumnView, len(suggestion.Columns))
	for index, column := range suggestion.Columns {
		columns[index] = suggestedColumnView{
			Name:        column.Name,
			ParquetType: column.ParquetType,
			Sample:      column.Sample,
		}
		if column.Type != "" {
			columns[index].Type = &column.Type
		}
	}

	problems := make([]problemView, len(suggestion.Problems))
	for index, problem := range suggestion.Problems {
		problems[index] = problemView{
			Code:        problem.Code,
			Message:     problem.Message,
			Columns:     problem.Columns,
			Alternative: problem.Alternative,
		}
	}

	return suggestionView{
		Source: sourceSummaryView{
			Kind:  suggestion.Source.Kind,
			Label: suggestion.Source.Label,
			Files: suggestion.Source.Files,
			Bytes: suggestion.Source.Bytes,
		},
		Schema:     suggestion.Schema,
		Table:      suggestion.Table,
		Rows:       suggestion.Rows,
		Columns:    columns,
		Importable: suggestion.Importable,
		Problems:   problems,
	}
}

// answerImportError answers 422 for files that cannot be imported, 404 for
// an unknown upload or import, 400 for an unusable request, and 409 for a
// source with problems, an incomplete upload, or a database that cannot be
// reached.
func answerImportError(response http.ResponseWriter, err error) {
	if conflict, ok := errors.AsType[*imports.ConflictError](err); ok {
		writeJSONProblem(response, http.StatusConflict, codedProblem{
			Type:        blankProblem,
			Title:       http.StatusText(http.StatusConflict),
			Detail:      conflict.Problem.Message,
			Code:        conflict.Problem.Code,
			Columns:     conflict.Problem.Columns,
			Alternative: conflict.Problem.Alternative,
		})

		return
	}

	if sourceErr, ok := errors.AsType[*imports.SourceError](err); ok {
		writeCodedProblem(response, http.StatusUnprocessableEntity,
			sourceErr.Code, sourceErr.Message)

		return
	}

	if statementErr, ok := errors.AsType[*sql.StatementError](err); ok &&
		statementErr.Code != "" {
		writeProblem(response, http.StatusInternalServerError, statementErr.Error())

		return
	}

	switch {
	case errors.Is(err, imports.ErrNoUpload):
		writeCodedProblem(response, http.StatusNotFound, uploadNotFound, err.Error())
	case errors.Is(err, imports.ErrIncomplete):
		writeCodedProblem(response, http.StatusConflict, uploadIncomplete, err.Error())
	case errors.Is(err, imports.ErrNoImport):
		writeCodedProblem(response, http.StatusNotFound, importNotFound, err.Error())
	case errors.Is(err, imports.ErrUnknownSource) || errors.Is(err, imports.ErrInvalid):
		writeCodedProblem(response, http.StatusBadRequest, invalidRequest, err.Error())
	default:
		writeCodedProblem(response, http.StatusConflict, databaseUnavailable, err.Error())
	}
}

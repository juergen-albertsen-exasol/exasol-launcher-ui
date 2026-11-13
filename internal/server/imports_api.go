package server

import (
	"errors"
	"net/http"

	"github.com/exasol/exasol-local-ui/internal/imports"
	"github.com/exasol/exasol-local-ui/internal/sql"
)

const (
	importSuggestionsPath = "/api/v1/deployments/{name}/import-suggestions"
	uploadIncomplete      = "upload_incomplete"
	databaseUnavailable   = "database_unavailable"
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

func (a *api) registerImports(mux *http.ServeMux) {
	mux.HandleFunc("POST "+importSuggestionsPath, a.suggestImport)
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
// an unknown upload, and 409 for an incomplete upload or a database that
// cannot be reached.
func answerImportError(response http.ResponseWriter, err error) {
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
	case errors.Is(err, imports.ErrUnknownSource):
		writeCodedProblem(response, http.StatusBadRequest, invalidRequest, err.Error())
	default:
		writeCodedProblem(response, http.StatusConflict, databaseUnavailable, err.Error())
	}
}

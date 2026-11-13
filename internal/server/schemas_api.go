package server

import (
	"errors"
	"net/http"

	"github.com/exasol/exasol-launcher-ui/internal/sql"
)

type schemaView struct {
	Name    string       `json:"name"`
	Virtual bool         `json:"virtual"`
	Objects []objectView `json:"objects"`
}

type objectView struct {
	Name    string       `json:"name"`
	Kind    string       `json:"kind"`
	Columns []columnView `json:"columns,omitempty"`
}

func (a *api) registerSchemas(mux *http.ServeMux) {
	const schemas = "/api/v1/deployments/{name}/schemas"

	mux.HandleFunc("GET "+schemas, a.listSchemas)
	mux.HandleFunc("GET "+schemas+"/{schema}", a.getSchema)
}

func (a *api) listSchemas(response http.ResponseWriter, request *http.Request) {
	deployment, target, ok := a.databaseTarget(response, request)
	if !ok {
		return
	}

	schemas, err := a.sql.Schemas(request.Context(), deployment.Name, target)
	if err != nil {
		answerMetadataError(response, err)

		return
	}

	views := make([]schemaView, len(schemas))
	for index, schema := range schemas {
		views[index] = newSchemaView(schema)
	}

	writeJSON(response, http.StatusOK, views)
}

func (a *api) getSchema(response http.ResponseWriter, request *http.Request) {
	deployment, target, ok := a.databaseTarget(response, request)
	if !ok {
		return
	}

	schema, err := a.sql.Schema(request.Context(), deployment.Name, target,
		request.PathValue("schema"))
	if err != nil {
		answerMetadataError(response, err)

		return
	}

	writeJSON(response, http.StatusOK, newSchemaView(schema))
}

func newSchemaView(schema sql.Schema) schemaView {
	objects := make([]objectView, len(schema.Objects))
	for index, object := range schema.Objects {
		objects[index] = objectView{
			Name:    object.Name,
			Kind:    object.Kind,
			Columns: columnViews(object.Columns),
		}
	}

	return schemaView{Name: schema.Name, Virtual: schema.Virtual, Objects: objects}
}

// answerMetadataError answers 404 for an unknown schema, 500 for a failing
// query, and 409 when the database cannot be reached.
func answerMetadataError(response http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoSchema) {
		writeProblem(response, http.StatusNotFound, err.Error())

		return
	}

	if statementErr, ok := errors.AsType[*sql.StatementError](err); ok &&
		statementErr.Code != "" {
		writeProblem(response, http.StatusInternalServerError, statementErr.Error())

		return
	}

	writeProblem(response, http.StatusConflict, err.Error())
}

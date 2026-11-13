package server

import (
	"errors"
	"net/http"
	"testing"

	"github.com/exasol/exasol-launcher-ui/internal/launcher/launchertest"
	"github.com/exasol/exasol-launcher-ui/internal/sql"
	"github.com/exasol/exasol-launcher-ui/internal/sql/sqltest"
	"github.com/stretchr/testify/assert"
)

const schemasURL = "/api/v1/deployments/default/schemas"

func textRow(texts ...string) []sql.Value {
	row := make([]sql.Value, len(texts))
	for index, text := range texts {
		row[index] = sqltest.Text(text)
	}

	return row
}

func TestSchemasListTablesAndViews(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond(sql.SchemasQuery, sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA_NAME"}, {Name: "SCHEMA_IS_VIRTUAL"}},
		Rows:    [][]sql.Value{textRow("RETAIL", "false"), textRow("S3", "true")},
	})
	fixture.database.Respond(sql.ObjectsQuery, sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA"}, {Name: "NAME"}, {Name: "KIND"}},
		Rows: [][]sql.Value{
			textRow("RETAIL", "SALES", "TABLE"),
			textRow("RETAIL", "SALES_BY_DAY", "VIEW"),
		},
	})

	response := fixture.call(http.MethodGet, schemasURL, "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `[
	  {"name": "RETAIL", "virtual": false, "objects": [
	    {"name": "SALES", "kind": "table"},
	    {"name": "SALES_BY_DAY", "kind": "view"}
	  ]},
	  {"name": "S3", "virtual": true, "objects": []}
	]`, response.Body.String())
}

func TestASchemaListsColumnsWithTheirTypes(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond(sql.SchemaQuery("RETAIL"), sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA_NAME"}, {Name: "SCHEMA_IS_VIRTUAL"}},
		Rows:    [][]sql.Value{textRow("RETAIL", "false")},
	})
	fixture.database.Respond(sql.ColumnsQuery("RETAIL"), sqltest.Response{
		Columns: []sql.Column{{Name: "T"}, {Name: "K"}, {Name: "C"}, {Name: "TYPE"}},
		Rows: [][]sql.Value{
			textRow("REGIONS", "TABLE", "REGION_ID", "DECIMAL(9,0)"),
			textRow("REGIONS", "TABLE", "REGION_NAME", "VARCHAR(100) UTF8"),
		},
	})

	response := fixture.call(http.MethodGet, schemasURL+"/RETAIL", "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"name": "RETAIL", "virtual": false, "objects": [
	  {"name": "REGIONS", "kind": "table", "columns": [
	    {"name": "REGION_ID", "type": "DECIMAL(9,0)"},
	    {"name": "REGION_NAME", "type": "VARCHAR(100) UTF8"}
	  ]}
	]}`, response.Body.String())
}

func TestAnUnknownSchemaIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.database.Respond(sql.SchemaQuery("NOPE"), sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA_NAME"}, {Name: "SCHEMA_IS_VIRTUAL"}},
	})

	response := fixture.call(http.MethodGet, schemasURL+"/NOPE", "")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestSchemasOfAStoppedOrUnreachableDatabaseConflict(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	stopped := newSQLFixture(t, fake)

	unreachable := newSQLFixture(t, fakeWithDeploymentDir(t, runningInfo))
	unreachable.database.FailConnect(errors.New("connection refused"))

	stoppedResponse := stopped.call(http.MethodGet, "/api/v1/deployments/test/schemas", "")
	unreachableResponse := unreachable.call(http.MethodGet, schemasURL, "")

	assert.Equal(t, http.StatusConflict, stoppedResponse.Code)
	assert.Equal(t, http.StatusConflict, unreachableResponse.Code)
	assert.Contains(t, decode[problem](t, unreachableResponse).Detail, "connection refused")
}

package sql_test

import (
	"errors"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/sql"
	"github.com/exasol/exasol-local-ui/internal/sql/sqltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func values(texts ...string) []sql.Value {
	row := make([]sql.Value, len(texts))
	for index, text := range texts {
		row[index] = sqltest.Text(text)
	}

	return row
}

func TestSchemasListTheirTablesAndViews(t *testing.T) {
	t.Parallel()

	fake := sqltest.NewFake()
	fake.Respond(sql.SchemasQuery, sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA_NAME"}, {Name: "SCHEMA_IS_VIRTUAL"}},
		Rows:    [][]sql.Value{values("EMPTY", "false"), values("RETAIL", "false")},
	})
	fake.Respond(sql.ObjectsQuery, sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA"}, {Name: "NAME"}, {Name: "KIND"}},
		Rows: [][]sql.Value{
			values("RETAIL", "SALES", "TABLE"),
			values("RETAIL", "SALES_BY_DAY", "VIEW"),
		},
	})

	schemas, err := sql.NewManager(fake).Schemas(t.Context(), deployment, target)

	require.NoError(t, err)
	assert.Equal(t, []sql.Schema{
		{Name: "EMPTY", Objects: []sql.Object{}},
		{Name: "RETAIL", Objects: []sql.Object{
			{Name: "SALES", Kind: "table"},
			{Name: "SALES_BY_DAY", Kind: "view"},
		}},
	}, schemas)
	assert.Zero(t, fake.OpenConnections())
}

func TestASchemaListsItsColumnsInTableOrder(t *testing.T) {
	t.Parallel()

	fake := sqltest.NewFake()
	fake.Respond(sql.SchemaQuery("RETAIL"), sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA_NAME"}, {Name: "SCHEMA_IS_VIRTUAL"}},
		Rows:    [][]sql.Value{values("RETAIL", "false")},
	})
	fake.Respond(sql.ColumnsQuery("RETAIL"), sqltest.Response{
		Columns: []sql.Column{{Name: "T"}, {Name: "K"}, {Name: "C"}, {Name: "TYPE"}},
		Rows: [][]sql.Value{
			values("REGIONS", "TABLE", "REGION_ID", "DECIMAL(9,0)"),
			values("REGIONS", "TABLE", "REGION_NAME", "VARCHAR(100) UTF8"),
			values("SALES_BY_DAY", "VIEW", "DAY", "DATE"),
		},
	})

	schema, err := sql.NewManager(fake).Schema(t.Context(), deployment, target, "RETAIL")

	require.NoError(t, err)
	assert.Equal(t, sql.Schema{Name: "RETAIL", Objects: []sql.Object{
		{Name: "REGIONS", Kind: "table", Columns: []sql.Column{
			{Name: "REGION_ID", Type: "DECIMAL(9,0)"},
			{Name: "REGION_NAME", Type: "VARCHAR(100) UTF8"},
		}},
		{Name: "SALES_BY_DAY", Kind: "view", Columns: []sql.Column{{Name: "DAY", Type: "DATE"}}},
	}}, schema)
}

func TestAnUnknownSchemaIsReported(t *testing.T) {
	t.Parallel()

	fake := sqltest.NewFake()
	fake.Respond(sql.SchemaQuery("NOPE"), sqltest.Response{
		Columns: []sql.Column{{Name: "SCHEMA_NAME"}, {Name: "SCHEMA_IS_VIRTUAL"}},
	})

	_, err := sql.NewManager(fake).Schema(t.Context(), deployment, target, "NOPE")

	require.ErrorIs(t, err, sql.ErrNoSchema)
}

func TestSchemaNamesAreQuotedAsLiterals(t *testing.T) {
	t.Parallel()

	assert.Contains(t, sql.ColumnsQuery("O'NEIL"), "COLUMN_SCHEMA = 'O''NEIL'")
}

func TestAnUnreachableDatabaseFailsTheSchemaListing(t *testing.T) {
	t.Parallel()

	fake := sqltest.NewFake()
	fake.FailConnect(errors.New("refused for " + password))

	_, err := sql.NewManager(fake).Schemas(t.Context(), deployment, target)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), password)
}

package imports_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/imports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	allTypes    = "types/all_types.parquet"
	unsupported = "types/unsupported.parquet"
	yellowTrips = "nyc-taxi/yellow_tripdata/yellow_tripdata_2024-01.parquet"
)

// fixtureElement is a footer element as Fusion Cloud's fixtures write it.
type fixtureElement struct {
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	RepetitionType string              `json:"repetition_type"`
	NumChildren    int                 `json:"num_children"`
	ConvertedType  string              `json:"converted_type"`
	Scale          int                 `json:"scale"`
	Precision      int                 `json:"precision"`
	LogicalType    *fixtureLogicalType `json:"logical_type"`
}

type fixtureLogicalType struct {
	Type            string `json:"type"`
	Scale           int    `json:"scale"`
	Precision       int    `json:"precision"`
	Unit            string `json:"unit"`
	IsAdjustedToUTC bool   `json:"isAdjustedToUTC"` //nolint:tagliatelle // hyparquet's names
	BitWidth        int    `json:"bitWidth"`        //nolint:tagliatelle // hyparquet's names
	IsSigned        bool   `json:"isSigned"`        //nolint:tagliatelle // hyparquet's names
}

func footerSchema(t *testing.T, path string) []imports.Element {
	t.Helper()

	content, err := os.ReadFile("testdata/footers.json")
	require.NoError(t, err)

	var footers map[string]struct {
		Schema []fixtureElement `json:"schema"`
	}
	require.NoError(t, json.Unmarshal(content, &footers))

	fixture, found := footers[path]
	require.True(t, found, path)

	schema := make([]imports.Element, len(fixture.Schema))
	for index, source := range fixture.Schema {
		schema[index] = imports.Element{
			Name:           source.Name,
			Type:           source.Type,
			RepetitionType: source.RepetitionType,
			NumChildren:    source.NumChildren,
			ConvertedType:  source.ConvertedType,
			Scale:          source.Scale,
			Precision:      source.Precision,
		}
		if logical := source.LogicalType; logical != nil {
			schema[index].LogicalType = &imports.LogicalType{
				Type:            logical.Type,
				Scale:           logical.Scale,
				Precision:       logical.Precision,
				Unit:            logical.Unit,
				IsAdjustedToUTC: logical.IsAdjustedToUTC,
				BitWidth:        logical.BitWidth,
				IsSigned:        logical.IsSigned,
			}
		}
	}

	return schema
}

func inferredByExasol(t *testing.T, path string) [][2]string {
	t.Helper()

	content, err := os.ReadFile("testdata/inferred.json")
	require.NoError(t, err)

	var inferred map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(content, &inferred))

	var columns [][2]string
	require.NoError(t, json.Unmarshal(inferred[path], &columns))

	return columns
}

func byName(columns []imports.Column) map[string]imports.Column {
	named := map[string]imports.Column{}
	for _, column := range columns {
		named[column.Name] = column
	}

	return named
}

func TestInferColumnsGivesTheColumnsExasolCreated(t *testing.T) {
	t.Parallel()

	for _, path := range []string{allTypes, yellowTrips} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			columns := imports.InferColumns(footerSchema(t, path))
			expected := inferredByExasol(t, path)

			require.Len(t, columns, len(expected))

			for index, column := range columns {
				assert.Equal(t, expected[index][0], column.Name)
				assert.True(t, imports.SameType(column.Type, expected[index][1]),
					"%s: %s vs %s", column.Name, column.Type, expected[index][1])
			}
		})
	}
}

func TestInferColumnsNamesTypesAsPeopleKnowThem(t *testing.T) {
	t.Parallel()

	columns := byName(imports.InferColumns(footerSchema(t, allTypes)))

	assert.Equal(t, imports.Column{
		Name: "c_int32", ParquetType: "INT32", Type: "DECIMAL(10,0)",
	}, columns["c_int32"])
	assert.Equal(t, imports.Column{
		Name: "c_int64", ParquetType: "INT64", Type: "DECIMAL(20,0)",
	}, columns["c_int64"])
	assert.Equal(t, imports.Column{
		Name: "c_double", ParquetType: "DOUBLE", Type: "DOUBLE PRECISION",
	}, columns["c_double"])

	for name, types := range map[string][2]string{
		"c_uint8":        {"UINT8", "DECIMAL(3,0)"},
		"c_decimal_9_3":  {"DECIMAL(9,3)", "DECIMAL(9,3)"},
		"c_string":       {"STRING", "VARCHAR(2000000) UTF8"},
		"c_timestamptz":  {"TIMESTAMP(us, UTC)", "TIMESTAMP(9)"},
		"c_timestamp_ns": {"TIMESTAMP(ns)", "TIMESTAMP(9)"},
	} {
		assert.Equal(t, types[0], columns[name].ParquetType, name)
		assert.Equal(t, types[1], columns[name].Type, name)
	}
}

func TestInferColumnsNamesUnsupportedColumnsAndSkipsNestedFields(t *testing.T) {
	t.Parallel()

	columns := imports.InferColumns(footerSchema(t, unsupported))

	described := make([]string, len(columns))
	for index, column := range columns {
		described[index] = column.Name + " " + column.ParquetType + " " + column.Unsupported
	}

	assert.Equal(t, []string{
		"id INT32 ",
		"c_binary BINARY type",
		"c_time TIME(us) type",
		"c_uuid UUID type",
		"address STRUCT nested",
		"scores LIST nested",
		"attributes MAP nested",
	}, described)
}

func TestInferColumnsRefusesDecimalsAbovePrecision36(t *testing.T) {
	t.Parallel()

	columns := imports.InferColumns([]imports.Element{
		{Name: "root", NumChildren: 1},
		{
			Name: "big", Type: "FIXED_LEN_BYTE_ARRAY",
			LogicalType: &imports.LogicalType{Type: "DECIMAL", Precision: 38, Scale: 2},
		},
	})

	assert.Equal(t, []imports.Column{
		{Name: "big", ParquetType: "DECIMAL(38,2)", Unsupported: imports.UnsupportedType},
	}, columns)
}

func TestUnsupportedMessageNamesNestedColumns(t *testing.T) {
	t.Parallel()

	columns := slices.DeleteFunc(imports.InferColumns(footerSchema(t, unsupported)),
		func(column imports.Column) bool { return column.Unsupported == imports.UnsupportedType })

	assert.Equal(t,
		"Exasol cannot import the columns address (struct), scores (list), attributes (map): "+
			"it does not support nested types, such as structs and lists. "+
			"Remove them from the file, or import other files.",
		imports.UnsupportedMessage(columns))
}

func TestUnsupportedMessageNamesOneColumn(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"Exasol cannot import the column payload (struct): "+
			"it does not support nested types, such as structs and lists. "+
			"Remove it from the file, or import other files.",
		imports.UnsupportedMessage([]imports.Column{
			{Name: "id", ParquetType: "INT32", Type: "DECIMAL(10,0)"},
			{Name: "payload", ParquetType: "STRUCT", Unsupported: imports.Nested},
		}))
}

func TestUnsupportedMessageIsEmptyWhenEveryColumnCanBeImported(t *testing.T) {
	t.Parallel()

	assert.Empty(t, imports.UnsupportedMessage(imports.InferColumns(footerSchema(t, allTypes))))
}

func TestTableName(t *testing.T) {
	t.Parallel()

	for name, expected := range map[string]string{
		"yellow_tripdata_2024-01.parquet":           "YELLOW_TRIPDATA_2024_01",
		"retail/orders/":                            "ORDERS",
		"trip-data/yellow_tripdata_2024-01.parquet": "YELLOW_TRIPDATA_2024_01",
		"Kunden Ü.parquet":                          "KUNDEN__",
		".parquet":                                  "IMPORT",
		"online_products.parquet":                   "ONLINE_PRODUCTS",
		"events.PARQUET":                            "EVENTS",
	} {
		assert.Equal(t, expected, imports.TableName(name), name)
	}
}

func TestTableNameForFilesUsesTheirCommonBeginning(t *testing.T) {
	t.Parallel()

	months := make([]string, 12)
	for index := range months {
		months[index] = fmt.Sprintf("orders-2025-%02d.parquet", index+1)
	}

	assert.Equal(t, "ORDERS_2025", imports.TableNameForFiles(months))
	assert.Equal(t, "YELLOW_TRIPDATA_2024", imports.TableNameForFiles([]string{
		"yellow_tripdata_2024-01.parquet", "yellow_tripdata_2024-02.parquet",
	}))
	assert.Equal(t, "A", imports.TableNameForFiles([]string{"a.parquet", "b.parquet"}))
	assert.Equal(t, "TRIPS", imports.TableNameForFiles([]string{"trips.parquet"}))
}

func TestTableAlternativesAreNumbered(t *testing.T) {
	t.Parallel()

	var alternatives []string

	for alternative := range imports.TableAlternatives("ORDERS") {
		alternatives = append(alternatives, alternative)
		if len(alternatives) == 2 {
			break
		}
	}

	assert.Equal(t, []string{"ORDERS_2", "ORDERS_3"}, alternatives)
}

func TestSameColumnsComparesNamesAndParquetTypesInOrder(t *testing.T) {
	t.Parallel()

	columns := imports.InferColumns(footerSchema(t, allTypes))

	assert.True(t, imports.SameColumns(columns, slices.Clone(columns)))
	assert.False(t, imports.SameColumns(columns, columns[1:]))

	changed := slices.Clone(columns)
	changed[3].ParquetType = "INT64"
	assert.False(t, imports.SameColumns(columns, changed))
}

func TestSameTypeIgnoresSpellings(t *testing.T) {
	t.Parallel()

	assert.True(t, imports.SameType("DOUBLE PRECISION", "DOUBLE"))
	assert.True(t, imports.SameType("decimal( 9 , 3 )", "DECIMAL(9,3)"))
	assert.False(t, imports.SameType("DECIMAL(9,3)", "DECIMAL(9,2)"))
}

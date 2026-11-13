package imports_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/exasol/exasol-launcher-ui/internal/imports"
	"github.com/exasol/exasol-launcher-ui/internal/imports/importstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func open(t *testing.T, path string) *imports.File {
	t.Helper()

	file, err := imports.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	return file
}

func samples(file *imports.File) map[string]any {
	named := map[string]any{}

	for index, sample := range file.Samples() {
		if sample == nil {
			named[file.Columns[index].Name] = nil

			continue
		}

		named[file.Columns[index].Name] = *sample
	}

	return named
}

func TestOpenReadsTheFooterAsTheFixtureDescribesIt(t *testing.T) {
	t.Parallel()

	for path, file := range map[string]string{
		allTypes:    "testdata/all_types.parquet",
		unsupported: "testdata/unsupported.parquet",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			opened := open(t, file)

			assert.Equal(t, imports.InferColumns(footerSchema(t, path)), opened.Columns)
		})
	}
}

func TestOpenCountsRows(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int64(1100), open(t, "testdata/all_types.parquet").Rows)
}

func TestOpenRefusesAFileThatIsNotParquet(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "notes.parquet")
	require.NoError(t, os.WriteFile(path, []byte("not parquet"), 0o600))

	_, err := imports.Open(path)

	require.ErrorIs(t, err, imports.ErrNotParquet)
}

func TestGeneratedIntegerAndFloatingPointColumns(t *testing.T) {
	t.Parallel()

	path := importstest.Write(t, t.TempDir(), "trips.parquet", importstest.Trips(3))

	file := open(t, path)

	assert.Equal(t, []imports.Column{
		{Name: "VendorID", ParquetType: "INT32", Type: "DECIMAL(10,0)"},
		{Name: "passenger_count", ParquetType: "INT64", Type: "DECIMAL(20,0)"},
		{Name: "trip_distance", ParquetType: "DOUBLE", Type: "DOUBLE PRECISION"},
		{Name: "store_and_fwd_flag", ParquetType: "STRING", Type: imports.Varchar},
	}, file.Columns)
	assert.Equal(t, int64(3), file.Rows)
	assert.Equal(t, map[string]any{
		"VendorID": "1", "passenger_count": "2", "trip_distance": "1.5", "store_and_fwd_flag": "N",
	}, samples(file))
}

func TestGeneratedStructColumn(t *testing.T) {
	t.Parallel()

	path := importstest.Write(t, t.TempDir(), "events.parquet", []importstest.Event{
		{ID: 7, Payload: importstest.Payload{Kind: "click"}},
	})

	file := open(t, path)

	assert.Equal(t, []imports.Column{
		{Name: "id", ParquetType: "INT64", Type: "DECIMAL(20,0)"},
		{Name: "payload", ParquetType: "STRUCT", Unsupported: imports.Nested},
	}, file.Columns)
	assert.Equal(t, map[string]any{"id": "7", "payload": nil}, samples(file))
}

func TestSamplesOfEveryType(t *testing.T) {
	t.Parallel()

	values := samples(open(t, "testdata/all_types.parquet"))

	for _, name := range []string{
		"c_boolean", "c_int8", "c_uint64", "c_double", "c_decimal_4_2", "c_decimal_36_10",
		"c_string", "c_date", "c_timestamp_us", "c_timestamp_ns", "c_timestamptz",
	} {
		assert.Contains(t, values, name)
	}

	assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, values["c_date"])
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`, values["c_timestamp_us"])
	assert.Regexp(t, `^-?\d+\.\d{2}$`, values["c_decimal_4_2"])
	assert.Regexp(t, `^-?\d+\.\d{10}$`, values["c_decimal_36_10"])
}

func TestSamplesOfAnEmptyFile(t *testing.T) {
	t.Parallel()

	path := importstest.Write(t, t.TempDir(), "empty.parquet", []importstest.Trip{})

	assert.Nil(t, open(t, path).Samples())
}

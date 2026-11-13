package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/imports"
	"github.com/exasol/exasol-local-ui/internal/imports/importstest"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/exasol/exasol-local-ui/internal/sql"
	"github.com/exasol/exasol-local-ui/internal/sql/sqltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const suggestionsURL = "/api/v1/deployments/default/import-suggestions"

type importsFixture struct {
	t        *testing.T
	handler  http.Handler
	database *sqltest.Fake
	uploads  *imports.Uploads
	files    string
}

func newImportsFixture(t *testing.T, fake *launchertest.Fake) *importsFixture {
	t.Helper()

	database := sqltest.NewFake()
	database.Respond(imports.TakenNamesQuery("DATA"), sqltest.Response{
		Columns: []sql.Column{{Name: "TABLE_NAME"}},
	})

	uploads := imports.NewUploads(filepath.Join(t.TempDir(), "uploads"))
	manager := imports.NewManager(database, uploads)
	t.Cleanup(func() { _ = uploads.Close() })

	return &importsFixture{
		t:        t,
		database: database,
		uploads:  uploads,
		files:    t.TempDir(),
		handler: NewHandler(Config{
			Launcher: launcher.New(fake.Executable),
			Uploads:  uploads,
			Imports:  manager,
		}),
	}
}

func (f *importsFixture) call(method, target, body string) *httptest.ResponseRecorder {
	f.t.Helper()

	request := httptest.NewRequestWithContext(
		f.t.Context(), method, target, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)

	assert.NotContains(f.t, recorder.Body.String(), storedPassword)

	return recorder
}

// upload uploads the files at paths under the names and returns the
// upload's id.
func (f *importsFixture) upload(names []string, paths []string) string {
	f.t.Helper()

	files := make([]uploadFileView, len(paths))
	contents := make([][]byte, len(paths))

	for index, path := range paths {
		contents[index] = importstest.Read(f.t, path)
		files[index] = uploadFileView{Name: names[index], Size: int64(len(contents[index]))}
	}

	body, err := json.Marshal(uploadRequest{Files: files})
	require.NoError(f.t, err)

	created := f.call(http.MethodPost, uploadsURL, string(body))
	require.Equal(f.t, http.StatusCreated, created.Code, created.Body.String())

	view := decode[uploadView](f.t, created)
	for index, file := range view.Files {
		stored := f.call(http.MethodPut, file.URL, string(contents[index]))
		require.Equal(f.t, http.StatusNoContent, stored.Code, stored.Body.String())
	}

	return view.ID
}

func (f *importsFixture) uploadTrips(names ...string) string {
	f.t.Helper()

	paths := make([]string, len(names))
	for index := range names {
		paths[index] = importstest.Write(f.t, f.files, names[index], importstest.Trips(index+2))
	}

	return f.upload(names, paths)
}

func (f *importsFixture) suggest(uploadID string) *httptest.ResponseRecorder {
	f.t.Helper()

	return f.call(http.MethodPost, suggestionsURL,
		`{"source": {"kind": "upload", "upload_id": "`+uploadID+`"}}`)
}

func (f *importsFixture) takeTables(names ...string) {
	rows := make([][]sql.Value, len(names))
	for index, name := range names {
		rows[index] = []sql.Value{sqltest.Text(name)}
	}

	f.database.Respond(imports.TakenNamesQuery("DATA"), sqltest.Response{
		Columns: []sql.Column{{Name: "TABLE_NAME"}},
		Rows:    rows,
	})
}

func TestATableNameAndRowCountFromSeveralFiles(t *testing.T) {
	t.Parallel()

	fixture := newImportsFixture(t, fakeWithDeploymentDir(t, runningInfo))
	uploadID := fixture.uploadTrips(
		"yellow_tripdata_2024-01.parquet", "yellow_tripdata_2024-02.parquet")

	response := fixture.suggest(uploadID)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	suggestion := decode[suggestionView](t, response)
	assert.Equal(t, "DATA", suggestion.Schema)
	assert.Equal(t, "YELLOW_TRIPDATA_2024", suggestion.Table)
	assert.Equal(t, int64(2+3), suggestion.Rows)
	assert.Equal(t, "upload", suggestion.Source.Kind)
	assert.Equal(t, "2 files", suggestion.Source.Label)
	assert.Equal(t, 2, suggestion.Source.Files)
	assert.True(t, suggestion.Importable)
	assert.Empty(t, suggestion.Problems)
}

func TestIntegerAndFloatingPointTypes(t *testing.T) {
	t.Parallel()

	fixture := newImportsFixture(t, fakeWithDeploymentDir(t, runningInfo))
	uploadID := fixture.uploadTrips("trips.parquet")

	response := fixture.suggest(uploadID)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.JSONEq(t, `{
	  "source": {"kind": "upload", "label": "trips.parquet", "files": 1,
	             "bytes": `+jsonNumber(decode[suggestionView](t, response).Source.Bytes)+`},
	  "schema": "DATA", "table": "TRIPS", "rows": 2,
	  "columns": [
	    {"name": "VendorID", "type": "DECIMAL(10,0)", "parquet_type": "INT32", "sample": "1"},
	    {"name": "passenger_count", "type": "DECIMAL(20,0)", "parquet_type": "INT64",
	     "sample": "2"},
	    {"name": "trip_distance", "type": "DOUBLE PRECISION", "parquet_type": "DOUBLE",
	     "sample": "1.5"},
	    {"name": "store_and_fwd_flag", "type": "VARCHAR(2000000) UTF8",
	     "parquet_type": "STRING", "sample": "N"}
	  ],
	  "importable": true, "problems": []
	}`, response.Body.String())
}

func jsonNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}

func TestATakenTableName(t *testing.T) {
	t.Parallel()

	fixture := newImportsFixture(t, fakeWithDeploymentDir(t, runningInfo))
	fixture.takeTables("EVENTS", "EVENTS_2", "TRIPS")
	uploadID := fixture.uploadTrips("events.parquet")

	suggestion := decode[suggestionView](t, fixture.suggest(uploadID))

	assert.False(t, suggestion.Importable)
	assert.Equal(t, []problemView{{
		Code:        "table_taken",
		Message:     "DATA.EVENTS exists. Choose another name, such as EVENTS_3.",
		Alternative: "EVENTS_3",
	}}, suggestion.Problems)
}

func TestANestedColumn(t *testing.T) {
	t.Parallel()

	fixture := newImportsFixture(t, fakeWithDeploymentDir(t, runningInfo))
	path := importstest.Write(t, fixture.files, "events.parquet", []importstest.Event{
		{ID: 1, Payload: importstest.Payload{Kind: "click"}},
	})
	uploadID := fixture.upload([]string{"events.parquet"}, []string{path})

	suggestion := decode[suggestionView](t, fixture.suggest(uploadID))

	assert.False(t, suggestion.Importable)
	assert.Equal(t, suggestedColumnView{Name: "payload", ParquetType: "STRUCT"},
		suggestion.Columns[1])
	require.Len(t, suggestion.Problems, 1)
	assert.Equal(t, "unsupported_columns", suggestion.Problems[0].Code)
	assert.Equal(t, []string{"payload"}, suggestion.Problems[0].Columns)
	assert.Contains(t, suggestion.Problems[0].Message, "payload (struct)")
	assert.Contains(t, suggestion.Problems[0].Message, "nested types")
}

func TestFilesWithDifferentColumns(t *testing.T) {
	t.Parallel()

	fixture := newImportsFixture(t, fakeWithDeploymentDir(t, runningInfo))
	trips := importstest.Write(t, fixture.files, "trips.parquet", importstest.Trips(1))
	events := importstest.Write(t, fixture.files, "events.parquet", []importstest.Event{{ID: 1}})
	uploadID := fixture.upload(
		[]string{"trips.parquet", "events.parquet"}, []string{trips, events})

	response := fixture.suggest(uploadID)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)

	problem := decode[codedProblem](t, response)
	assert.Equal(t, "columns_differ", problem.Code)
	assert.Equal(t, "The files of one import need the same columns, but events.parquet has "+
		"other columns than trips.parquet. Import them separately.", problem.Detail)
}

func TestAFileThatIsNotParquet(t *testing.T) {
	t.Parallel()

	fixture := newImportsFixture(t, fakeWithDeploymentDir(t, runningInfo))
	path := filepath.Join(fixture.files, "notes.parquet")
	require.NoError(t, os.WriteFile(path, []byte("not parquet"), 0o600))
	uploadID := fixture.upload([]string{"notes.parquet"}, []string{path})

	response := fixture.suggest(uploadID)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)

	problem := decode[codedProblem](t, response)
	assert.Equal(t, "not_parquet", problem.Code)
	assert.Contains(t, problem.Detail, "notes.parquet")
}

func TestSuggestionsOfUnknownOrIncompleteUploads(t *testing.T) {
	t.Parallel()

	fixture := newImportsFixture(t, fakeWithDeploymentDir(t, runningInfo))
	incomplete := decode[uploadView](t, fixture.call(http.MethodPost, uploadsURL,
		`{"files": [{"name": "a.parquet", "size": 1}]}`))

	unknown := fixture.suggest("unknown")
	waiting := fixture.suggest(incomplete.ID)

	assert.Equal(t, http.StatusNotFound, unknown.Code)
	assert.Equal(t, http.StatusConflict, waiting.Code)
}

func TestSuggestionsOfAStoppedDatabaseConflict(t *testing.T) {
	t.Parallel()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fixture := newImportsFixture(t, fake)

	response := fixture.call(http.MethodPost, "/api/v1/deployments/test/import-suggestions",
		`{"source": {"kind": "upload", "upload_id": "any"}}`)

	assert.Equal(t, http.StatusConflict, response.Code)
}

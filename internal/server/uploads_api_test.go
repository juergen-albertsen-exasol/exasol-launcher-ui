package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/imports"
	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const uploadsURL = "/api/v1/deployments/default/uploads"

type uploadsFixture struct {
	t       *testing.T
	handler http.Handler
	uploads *imports.Uploads
}

func newUploadsFixture(t *testing.T) *uploadsFixture {
	t.Helper()

	uploads := imports.NewUploads(filepath.Join(t.TempDir(), "uploads"))
	t.Cleanup(func() { _ = uploads.Close() })

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)

	return &uploadsFixture{
		t:       t,
		uploads: uploads,
		handler: NewHandler(Config{Launcher: launcher.New(fake.Executable), Uploads: uploads}),
	}
}

func (f *uploadsFixture) call(method, target, body string) *httptest.ResponseRecorder {
	f.t.Helper()

	request := httptest.NewRequestWithContext(
		f.t.Context(), method, target, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)

	return recorder
}

func TestUploadingTwoFiles(t *testing.T) {
	t.Parallel()

	fixture := newUploadsFixture(t)

	created := fixture.call(http.MethodPost, uploadsURL, `{"files": [
	  {"name": "yellow_tripdata_2024-01.parquet", "size": 3},
	  {"name": "yellow_tripdata_2024-02.parquet", "size": 2}
	]}`)

	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	view := decode[uploadView](t, created)
	base := uploadsURL + "/" + view.ID
	assert.Equal(t, base, created.Header().Get("Location"))
	assert.Equal(t, []uploadFileView{
		{Name: "yellow_tripdata_2024-01.parquet", Size: 3, URL: base + "/files/0"},
		{Name: "yellow_tripdata_2024-02.parquet", Size: 2, URL: base + "/files/1"},
	}, view.Files)

	first := fixture.call(http.MethodPut, view.Files[0].URL, "abc")
	second := fixture.call(http.MethodPut, view.Files[1].URL, "de")

	assert.Equal(t, http.StatusNoContent, first.Code, first.Body.String())
	assert.Equal(t, http.StatusNoContent, second.Code, second.Body.String())

	upload, err := fixture.uploads.Get("default", view.ID)
	require.NoError(t, err)
	assert.Len(t, upload.Files, 2)
}

func TestAnUploadNeedsFiles(t *testing.T) {
	t.Parallel()

	fixture := newUploadsFixture(t)

	response := fixture.call(http.MethodPost, uploadsURL, `{"files": []}`)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "invalid_request", decode[codedProblem](t, response).Code)
}

func TestAFileOfAnotherSizeIsABadRequest(t *testing.T) {
	t.Parallel()

	fixture := newUploadsFixture(t)
	view := decode[uploadView](t, fixture.call(http.MethodPost, uploadsURL,
		`{"files": [{"name": "a.parquet", "size": 3}]}`))

	response := fixture.call(http.MethodPut, view.Files[0].URL, "ab")

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "size_differs", decode[codedProblem](t, response).Code)
}

func TestUnknownUploadsAndDeploymentsAreNotFound(t *testing.T) {
	t.Parallel()

	fixture := newUploadsFixture(t)
	view := decode[uploadView](t, fixture.call(http.MethodPost, uploadsURL,
		`{"files": [{"name": "a.parquet", "size": 1}]}`))

	for _, target := range []string{
		uploadsURL + "/unknown/files/0",
		uploadsURL + "/" + view.ID + "/files/1",
		uploadsURL + "/" + view.ID + "/files/first",
		"/api/v1/deployments/test/uploads/" + view.ID + "/files/0",
	} {
		response := fixture.call(http.MethodPut, target, "a")

		assert.Equal(t, http.StatusNotFound, response.Code, target)
		assert.Equal(t, "upload_not_found", decode[codedProblem](t, response).Code, target)
	}

	unknownDeployment := fixture.call(http.MethodPost, "/api/v1/deployments/nope/uploads",
		`{"files": [{"name": "a.parquet", "size": 1}]}`)

	assert.Equal(t, http.StatusNotFound, unknownDeployment.Code)
}

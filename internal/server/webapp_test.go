package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

const indexHTML = "<html>app</html>"

func serveWebApp(t *testing.T, files fstest.MapFS, target string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	NewHandler(Config{WebApp: files}).ServeHTTP(recorder, request)

	return recorder
}

func builtApp() fstest.MapFS {
	return fstest.MapFS{
		"index.html":     {Data: []byte(indexHTML)},
		"main-ABC.js":    {Data: []byte("console.log()")},
		"assets/app.css": {Data: []byte("body{}")},
	}
}

func TestApplicationFilesAreServed(t *testing.T) {
	t.Parallel()

	response := serveWebApp(t, builtApp(), "/main-ABC.js")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "console.log()", response.Body.String())
}

func TestApplicationRoutesServeTheIndex(t *testing.T) {
	t.Parallel()

	routes := []string{"/", "/overview", "/overview/details", "/index.html", "/assets"}
	for _, target := range routes {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			response := serveWebApp(t, builtApp(), target)

			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, indexHTML, response.Body.String())
			assert.Equal(t, "no-cache", response.Header().Get("Cache-Control"))
		})
	}
}

func TestMissingFilesAreNotFound(t *testing.T) {
	t.Parallel()

	response := serveWebApp(t, builtApp(), "/main-OLD.js")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestMissingBuildIsReported(t *testing.T) {
	t.Parallel()

	response := serveWebApp(t, fstest.MapFS{}, "/overview")

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.Contains(t, response.Body.String(), "task build")
}

package server

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const indexFile = "index.html"

// webApp serves the application's files, and its index for every other path
// so that application routes survive reloads.
type webApp struct {
	files      fs.FS
	fileServer http.Handler
}

func newWebApp(files fs.FS) *webApp {
	return &webApp{files: files, fileServer: http.FileServerFS(files)}
}

func (a *webApp) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writeProblem(response, http.StatusMethodNotAllowed, "The application serves GET only.")

		return
	}

	name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
	if name == "" {
		a.serveIndex(response, request)

		return
	}

	info, err := fs.Stat(a.files, name)
	if err == nil && !info.IsDir() && name != indexFile {
		a.fileServer.ServeHTTP(response, request)

		return
	}

	if errors.Is(err, fs.ErrNotExist) && strings.Contains(path.Base(name), ".") {
		writeProblem(response, http.StatusNotFound, "The application has no file "+name+".")

		return
	}

	a.serveIndex(response, request)
}

func (a *webApp) serveIndex(response http.ResponseWriter, request *http.Request) {
	index, err := fs.ReadFile(a.files, indexFile)
	if err != nil {
		writeProblem(response, http.StatusServiceUnavailable,
			"The web application is missing. Build it with `task build`.")

		return
	}

	response.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(response, request, indexFile, time.Time{}, bytes.NewReader(index))
}

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/exasol/exasol-local-ui/internal/imports"
)

const (
	uploadsPath       = "/api/v1/deployments/{name}/uploads"
	maxUploadRequest  = 64 * 1024
	invalidRequest    = "invalid_request"
	uploadNotFound    = "upload_not_found"
	uploadSizeDiffers = "size_differs"
)

type uploadRequest struct {
	Files []uploadFileView `json:"files"`
}

type uploadFileView struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url,omitempty"`
}

type uploadView struct {
	ID    string           `json:"id"`
	Files []uploadFileView `json:"files"`
}

// codedProblem extends the problem detail with a code that clients can
// act on.
type codedProblem struct {
	problem

	Code        string   `json:"code"`
	Columns     []string `json:"columns,omitempty"`
	Alternative string   `json:"alternative,omitempty"`
}

func writeCodedProblem(response http.ResponseWriter, status int, code, detail string) {
	writeJSONProblem(response, status, codedProblem{
		Type:   blankProblem,
		Title:  http.StatusText(status),
		Detail: detail,
		Code:   code,
	})
}

func writeJSONProblem[T any](response http.ResponseWriter, status int, value T) {
	body, err := json.Marshal(value)
	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return
	}

	response.Header().Set("Content-Type", "application/problem+json")
	response.WriteHeader(status)
	_, _ = response.Write(body)
}

func (a *api) registerUploads(mux *http.ServeMux) {
	mux.HandleFunc("POST "+uploadsPath, a.createUpload)
	mux.HandleFunc("PUT "+uploadsPath+"/{id}/files/{index}", a.storeUploadedFile)
}

func (a *api) createUpload(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	var body uploadRequest
	if !decodeSizedBody(response, request, &body, maxUploadRequest) {
		return
	}

	files := make([]imports.FileSpec, len(body.Files))
	for index, file := range body.Files {
		files[index] = imports.FileSpec{Name: file.Name, Size: file.Size}
	}

	created, err := a.uploads.Create(deployment.Name, files)
	if err != nil {
		writeCodedProblem(response, http.StatusBadRequest, invalidRequest, err.Error())

		return
	}

	base := "/api/v1/deployments/" + deployment.Name + "/uploads/" + created.ID
	view := uploadView{ID: created.ID, Files: make([]uploadFileView, len(created.Files))}

	for index, file := range created.Files {
		view.Files[index] = uploadFileView{
			Name: file.Name,
			Size: file.Size,
			URL:  base + "/files/" + strconv.Itoa(index),
		}
	}

	response.Header().Set("Location", base)
	writeJSON(response, http.StatusCreated, view)
}

func (a *api) storeUploadedFile(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	index, err := strconv.Atoi(request.PathValue("index"))
	if err != nil {
		writeCodedProblem(response, http.StatusNotFound, uploadNotFound, imports.ErrNoFile.Error())

		return
	}

	err = a.uploads.Store(deployment.Name, request.PathValue("id"), index, request.Body)

	switch {
	case err == nil:
		response.WriteHeader(http.StatusNoContent)
	case errors.Is(err, imports.ErrNoUpload) || errors.Is(err, imports.ErrNoFile):
		writeCodedProblem(response, http.StatusNotFound, uploadNotFound, err.Error())
	case errors.Is(err, imports.ErrSizeDiffers):
		writeCodedProblem(response, http.StatusBadRequest, uploadSizeDiffers, err.Error())
	default:
		writeProblem(response, http.StatusInternalServerError, err.Error())
	}
}

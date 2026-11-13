package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/operations"
)

type operationResult struct {
	DeploymentState string `json:"deployment_state"`
	DatabaseReady   bool   `json:"database_ready"`
}

type operationView struct {
	ID         string           `json:"id"`
	Deployment string           `json:"deployment"`
	Kind       operations.Kind  `json:"kind"`
	Status     string           `json:"status"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Result     *operationResult `json:"result,omitempty"`
	Error      string           `json:"error,omitempty"`
	Lines      []string         `json:"lines,omitempty"`
}

func viewOf(snapshot operations.Snapshot, lines []string) operationView {
	view := operationView{
		ID:         snapshot.ID,
		Deployment: snapshot.Deployment,
		Kind:       snapshot.Kind,
		Status:     string(snapshot.Status),
		StartedAt:  snapshot.StartedAt,
		FinishedAt: snapshot.FinishedAt,
		Error:      snapshot.Error,
		Lines:      lines,
	}

	if snapshot.Result != nil {
		view.Result = &operationResult{
			DeploymentState: snapshot.Result.DeploymentState,
			DatabaseReady:   snapshot.Result.DatabaseReady,
		}
	}

	return view
}

func (a *api) registerOperations(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/deployments/{name}/start", a.startOperation(operations.KindStart))
	mux.HandleFunc("POST /api/v1/deployments/{name}/stop", a.startOperation(operations.KindStop))
	mux.HandleFunc("POST /api/v1/deployments/{name}/deploy",
		a.startOperation(operations.KindDeploy))
	mux.HandleFunc("POST /api/v1/deployments/{name}/destroy", a.destroy)
	mux.HandleFunc("POST /api/v1/deployments/{name}/remove",
		a.startOperation(operations.KindRemove))
	mux.HandleFunc("GET /api/v1/operations", a.listOperations)
	mux.HandleFunc("GET /api/v1/operations/{id}", a.getOperation)
	mux.HandleFunc("GET /api/v1/operations/{id}/events", a.streamOperation)
}

type destroyRequest struct {
	RemoveFolder bool `json:"remove_folder"`
}

func (a *api) startOperation(kind operations.Kind) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		a.runOperation(response, request, kind)
	}
}

func (a *api) destroy(response http.ResponseWriter, request *http.Request) {
	var options destroyRequest

	if request.ContentLength != 0 && !decodeBody(response, request, &options) {
		return
	}

	kind := operations.KindDestroy
	if options.RemoveFolder {
		kind = operations.KindDestroyAndRemove
	}

	a.runOperation(response, request, kind)
}

func (a *api) runOperation(
	response http.ResponseWriter,
	request *http.Request,
	kind operations.Kind,
) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	// The password is only used to withhold it from the progress output.
	secret, _ := database.ReadPassword(deployment.Path)

	snapshot, err := a.operations.Start(deployment, kind, secret)
	if errors.Is(err, operations.ErrBusy) {
		writeProblem(response, http.StatusConflict,
			"An operation already runs on deployment "+deployment.Name+".")

		return
	}

	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return
	}

	response.Header().Set("Location", "/api/v1/operations/"+snapshot.ID)
	writeJSON(response, http.StatusAccepted, viewOf(snapshot, nil))
}

func (a *api) listOperations(response http.ResponseWriter, _ *http.Request) {
	snapshots := a.operations.List()

	views := make([]operationView, 0, len(snapshots))
	for _, snapshot := range snapshots {
		views = append(views, viewOf(snapshot, nil))
	}

	writeJSON(response, http.StatusOK, views)
}

func (a *api) getOperation(response http.ResponseWriter, request *http.Request) {
	snapshot, lines, found := a.operations.Get(request.PathValue("id"))
	if !found {
		writeProblem(response, http.StatusNotFound, "There is no such operation.")

		return
	}

	if lines == nil {
		lines = []string{}
	}

	view := viewOf(snapshot, lines)
	writeJSON(response, http.StatusOK, view)
}

// streamOperation sends the operation's progress lines as server-sent events
// and ends with a status event once the operation has finished.
func (a *api) streamOperation(response http.ResponseWriter, request *http.Request) {
	operationID := request.PathValue("id")

	changes, stopWatching := a.operations.Watch(operationID)
	defer stopWatching()

	snapshot, lines, found := a.operations.Since(operationID, 0)
	if !found {
		writeProblem(response, http.StatusNotFound, "There is no such operation.")

		return
	}

	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.WriteHeader(http.StatusOK)

	controller := http.NewResponseController(response)
	sent := 0

	for {
		for _, line := range lines {
			writeEvent(response, "line", line)
		}

		sent += len(lines)

		if snapshot.Status != operations.StatusRunning {
			writeEvent(response, "status", viewOf(snapshot, nil))
			_ = controller.Flush()

			return
		}

		_ = controller.Flush()

		select {
		case <-request.Context().Done():
			return
		case <-changes:
		}

		snapshot, lines, _ = a.operations.Since(operationID, sent)
	}
}

func writeEvent(response http.ResponseWriter, event string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}

	_, _ = fmt.Fprintf(response, "event: %s\ndata: %s\n\n", event, data)
}

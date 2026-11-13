package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/operations"
)

type sidecarView struct {
	Name               string         `json:"name"`
	Description        string         `json:"description,omitempty"`
	Catalog            bool           `json:"catalog"`
	Enabled            bool           `json:"enabled"`
	Running            bool           `json:"running"`
	State              string         `json:"state"`
	Health             string         `json:"health"`
	HealthError        string         `json:"health_error,omitempty"`
	Error              string         `json:"error,omitempty"`
	LastOperationError string         `json:"last_operation_error,omitempty"`
	RestartRequired    bool           `json:"restart_required"`
	Endpoints          []endpointView `json:"endpoints"`
}

type endpointView struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
	URL  string `json:"url,omitempty"`
}

type mcpClientView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mcpInstructionsView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Command string `json:"command,omitempty"`
	File    string `json:"file,omitempty"`
	Content string `json:"content,omitempty"`
	Note    string `json:"note,omitempty"`
}

func (a *api) registerSidecars(mux *http.ServeMux) {
	const deployment = "/api/v1/deployments/{name}"

	mux.HandleFunc("GET "+deployment+"/sidecars", a.listSidecars)
	mux.HandleFunc("POST "+deployment+"/sidecars/{sidecar}/enable", a.enableSidecar)
	mux.HandleFunc("POST "+deployment+"/sidecars/{sidecar}/disable", a.disableSidecar)
	mux.HandleFunc("GET "+deployment+"/mcp/clients", a.listMCPClients)
	mux.HandleFunc("GET "+deployment+"/mcp/clients/{client}", a.getMCPInstructions)
}

// sidecars returns the deployment's saved sidecars, followed by the supported
// catalog sidecars it has not saved.
func (a *api) sidecars(ctx context.Context, deployment launcher.Deployment) ([]sidecarView, error) {
	saved, err := a.launcher.SidecarStatus(ctx, deployment.Path)
	if err != nil {
		return nil, err
	}

	catalog, err := a.launcher.SidecarCatalog(ctx, deployment.Path)
	if err != nil {
		return nil, err
	}

	descriptions := map[string]string{}
	for _, entry := range catalog {
		descriptions[entry.Name] = entry.Description
	}

	views := make([]sidecarView, 0, len(saved)+len(catalog))
	listed := map[string]bool{}

	for _, sidecar := range saved {
		description, inCatalog := descriptions[sidecar.Name]
		listed[sidecar.Name] = true
		views = append(views, sidecarView{
			Name:               sidecar.Name,
			Description:        description,
			Catalog:            inCatalog,
			Enabled:            sidecar.Enabled,
			Running:            sidecar.Running,
			State:              sidecar.State,
			Health:             sidecar.Health,
			HealthError:        sidecar.HealthError,
			Error:              sidecar.Error,
			LastOperationError: sidecar.LastOperationError,
			RestartRequired:    sidecar.RestartRequired,
			Endpoints:          viewOfEndpoints(sidecar.Endpoints),
		})
	}

	for _, entry := range catalog {
		if entry.Supported && !listed[entry.Name] {
			views = append(views, sidecarView{
				Name:        entry.Name,
				Description: entry.Description,
				Catalog:     true,
				Endpoints:   []endpointView{},
			})
		}
	}

	return views, nil
}

func viewOfEndpoints(endpoints []launcher.SidecarEndpoint) []endpointView {
	views := make([]endpointView, 0, len(endpoints))
	for _, endpoint := range endpoints {
		views = append(views, endpointView{IP: endpoint.IP, Port: endpoint.Port, URL: endpoint.URL})
	}

	return views
}

// answerRefusal answers the request with 409 and the launcher's message when
// the launcher refused it, and with 500 otherwise.
func answerRefusal(response http.ResponseWriter, err error) {
	if launcherError, refused := errors.AsType[*launcher.LauncherError](err); refused {
		writeProblem(response, http.StatusConflict, launcherError.Message)

		return
	}

	writeProblem(response, http.StatusInternalServerError, err.Error())
}

func (a *api) listSidecars(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	sidecars, err := a.sidecars(request.Context(), deployment)
	if err != nil {
		answerRefusal(response, err)

		return
	}

	writeJSON(response, http.StatusOK, sidecars)
}

// findSidecar finds the deployment and its listed sidecar named in the path.
// Only listed names reach the launcher, which keeps flags out of its command.
func (a *api) findSidecar(
	response http.ResponseWriter,
	request *http.Request,
) (launcher.Deployment, string, bool) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return launcher.Deployment{}, "", false
	}

	sidecars, err := a.sidecars(request.Context(), deployment)
	if err != nil {
		answerRefusal(response, err)

		return launcher.Deployment{}, "", false
	}

	name := request.PathValue("sidecar")
	for _, sidecar := range sidecars {
		if sidecar.Name == name {
			return deployment, name, true
		}
	}

	writeProblem(response, http.StatusNotFound,
		"Deployment "+deployment.Name+" has no sidecar "+name+".")

	return launcher.Deployment{}, "", false
}

type disableRequest struct {
	RemoveDefinition bool `json:"remove_definition"`
}

func (a *api) enableSidecar(response http.ResponseWriter, request *http.Request) {
	deployment, sidecar, ok := a.findSidecar(response, request)
	if !ok {
		return
	}

	a.runOperationOn(response, deployment, operations.KindEnableSidecar, sidecar)
}

func (a *api) disableSidecar(response http.ResponseWriter, request *http.Request) {
	var options disableRequest

	if request.ContentLength != 0 && !decodeBody(response, request, &options) {
		return
	}

	deployment, sidecar, ok := a.findSidecar(response, request)
	if !ok {
		return
	}

	kind := operations.KindDisableSidecar
	if options.RemoveDefinition {
		kind = operations.KindRemoveSidecar
	}

	a.runOperationOn(response, deployment, kind, sidecar)
}

func (a *api) mcpClients(
	response http.ResponseWriter,
	request *http.Request,
) (launcher.Deployment, []launcher.MCPClient, bool) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return launcher.Deployment{}, nil, false
	}

	clients, err := a.launcher.MCPClients(request.Context(), deployment.Path)
	if err != nil {
		answerRefusal(response, err)

		return launcher.Deployment{}, nil, false
	}

	return deployment, clients, true
}

func (a *api) listMCPClients(response http.ResponseWriter, request *http.Request) {
	_, clients, ok := a.mcpClients(response, request)
	if !ok {
		return
	}

	views := make([]mcpClientView, 0, len(clients))
	for _, client := range clients {
		views = append(views, mcpClientView(client))
	}

	writeJSON(response, http.StatusOK, views)
}

func (a *api) getMCPInstructions(response http.ResponseWriter, request *http.Request) {
	deployment, clients, ok := a.mcpClients(response, request)
	if !ok {
		return
	}

	clientID := request.PathValue("client")

	listed := false
	for _, client := range clients {
		listed = listed || client.ID == clientID
	}

	if !listed {
		writeProblem(response, http.StatusNotFound,
			"There are no setup instructions for MCP client "+clientID+".")

		return
	}

	instructions, err := a.launcher.MCPInstructions(request.Context(), deployment.Path, clientID)
	if err != nil {
		answerRefusal(response, err)

		return
	}

	writeJSON(response, http.StatusOK, mcpInstructionsView{
		ID:      instructions.Client,
		Name:    instructions.Name,
		URL:     instructions.URL,
		Command: instructions.Command,
		File:    instructions.File,
		Content: instructions.Content,
		Note:    instructions.Note,
	})
}

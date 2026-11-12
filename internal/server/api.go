package server

import (
	"net/http"

	"github.com/exasol/exasol-local-ui/internal/launcher"
)

type deploymentSummary struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Infrastructure string `json:"infrastructure"`
	Installation   string `json:"installation"`
}

type deploymentDetails struct {
	Name       string             `json:"name"`
	State      string             `json:"state"`
	Connection *connectionDetails `json:"connection,omitempty"`
}

// connectionDetails deliberately has no password field: the API never
// returns the database password.
type connectionDetails struct {
	Host                       string `json:"host"`
	Port                       int    `json:"port"`
	Username                   string `json:"username"`
	InsecureSkipCertValidation bool   `json:"insecure_skip_cert_validation"`
}

type api struct {
	launcher *launcher.Client
}

func (a *api) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/deployments", a.listDeployments)
	mux.HandleFunc("GET /api/v1/deployments/{name}", a.getDeployment)
	mux.HandleFunc("/api/", func(response http.ResponseWriter, _ *http.Request) {
		writeProblem(response, http.StatusNotFound, "The API has no such resource.")
	})
}

func (a *api) listDeployments(response http.ResponseWriter, request *http.Request) {
	refresh := request.URL.Query().Get("refresh") == "true"

	deployments, err := a.launcher.Deployments(request.Context(), refresh)
	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return
	}

	summaries := make([]deploymentSummary, 0, len(deployments))
	for _, deployment := range deployments {
		summaries = append(summaries, deploymentSummary{
			Name:           deployment.Name,
			Status:         deployment.Status,
			Infrastructure: deployment.Infrastructure,
			Installation:   deployment.Installation,
		})
	}

	writeJSON(response, http.StatusOK, summaries)
}

func (a *api) getDeployment(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	info, err := a.launcher.Info(request.Context(), deployment.Path)
	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return
	}

	details := deploymentDetails{Name: deployment.Name, State: info.DeploymentState}
	if info.Connection != nil {
		details.Connection = &connectionDetails{
			Host:                       info.Connection.Host,
			Port:                       info.Connection.Port,
			Username:                   info.Connection.Username,
			InsecureSkipCertValidation: info.Connection.InsecureSkipCertValidation,
		}
	}

	writeJSON(response, http.StatusOK, details)
}

func (a *api) findDeployment(
	response http.ResponseWriter,
	request *http.Request,
) (launcher.Deployment, bool) {
	name := request.PathValue("name")

	deployment, found, err := a.launcher.Deployment(request.Context(), name)
	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return launcher.Deployment{}, false
	}

	if !found {
		writeProblem(response, http.StatusNotFound, "The launcher lists no deployment "+name+".")

		return launcher.Deployment{}, false
	}

	return deployment, true
}

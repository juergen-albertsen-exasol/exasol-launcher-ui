package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/exasol/exasol-local-ui/internal/launcher"
)

const (
	statusInitialized = "initialized"
	statusStopped     = "stopped"
	localPreset       = "local"
	maxBodyBytes      = 64 * 1024
)

type configurationView struct {
	Infrastructure presetView `json:"infrastructure"`
	Installation   presetView `json:"installation"`
	Editable       bool       `json:"editable"`
	ApplyWith      string     `json:"apply_with,omitempty"`
}

type presetView struct {
	Name        string       `json:"name"`
	DisplayName string       `json:"display_name"`
	Description string       `json:"description"`
	Options     []optionView `json:"options"`
}

// optionView withholds the value of secret options, so passwords never reach
// the browser.
type optionView struct {
	Name   string          `json:"name"`
	Value  json.RawMessage `json:"value,omitempty"`
	Secret bool            `json:"secret"`
}

type configurationPatch struct {
	Options map[string]json.RawMessage `json:"options"`
}

type configurationReset struct {
	Options []string `json:"options"`
}

func (a *api) registerConfiguration(mux *http.ServeMux) {
	const path = "/api/v1/deployments/{name}/configuration"

	mux.HandleFunc("GET "+path, a.getConfiguration)
	mux.HandleFunc("PATCH "+path, a.patchConfiguration)
	mux.HandleFunc("POST "+path+"/reset", a.resetConfiguration)
}

func isSecret(option string) bool {
	return strings.Contains(option, "password")
}

// applyWith names the operation that applies saved settings, or is empty
// when the settings cannot be changed in the deployment's status.
func applyWith(deployment launcher.Deployment) string {
	switch {
	case deployment.Status == statusInitialized:
		return "deploy"
	case deployment.Status == statusStopped && deployment.Infrastructure == localPreset:
		return "start"
	default:
		return ""
	}
}

func viewOfPreset(preset launcher.Preset) presetView {
	view := presetView{
		Name:        preset.Identity.Name,
		DisplayName: preset.Identity.DisplayName,
		Description: preset.Identity.Description,
		Options:     make([]optionView, 0, len(preset.Options)),
	}

	for _, option := range preset.Options {
		if isSecret(option.Name) {
			view.Options = append(view.Options, optionView{Name: option.Name, Secret: true})
		} else {
			view.Options = append(view.Options, optionView{Name: option.Name, Value: option.Value})
		}
	}

	return view
}

func (a *api) writeConfiguration(
	response http.ResponseWriter,
	request *http.Request,
	deployment launcher.Deployment,
) {
	configuration, err := a.launcher.Configuration(request.Context(), deployment.Path)
	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return
	}

	apply := applyWith(deployment)

	writeJSON(response, http.StatusOK, configurationView{
		Infrastructure: viewOfPreset(configuration.Infrastructure),
		Installation:   viewOfPreset(configuration.Installation),
		Editable:       apply != "",
		ApplyWith:      apply,
	})
}

func (a *api) getConfiguration(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	a.writeConfiguration(response, request, deployment)
}

// findEditableDeployment finds the deployment and the options it accepts, or
// answers the request when its settings cannot be changed now.
func (a *api) findEditableDeployment(
	response http.ResponseWriter,
	request *http.Request,
) (launcher.Deployment, []string, bool) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return launcher.Deployment{}, nil, false
	}

	if applyWith(deployment) == "" {
		detail := "Destroy deployment " + deployment.Name + " to change its settings."
		if deployment.Infrastructure == localPreset {
			detail = "Stop deployment " + deployment.Name + " to change its settings."
		}

		writeProblem(response, http.StatusConflict, detail)

		return launcher.Deployment{}, nil, false
	}

	configuration, err := a.launcher.Configuration(request.Context(), deployment.Path)
	if err != nil {
		writeProblem(response, http.StatusInternalServerError, err.Error())

		return launcher.Deployment{}, nil, false
	}

	options := []string{}

	presets := []launcher.Preset{configuration.Infrastructure, configuration.Installation}
	for _, preset := range presets {
		for _, option := range preset.Options {
			options = append(options, option.Name)
		}
	}

	return deployment, options, true
}

// checkOptionName rejects names the deployment has no option for, which also
// keeps other launcher flags out of the command, and password options, which
// would expose the password in the process list.
func checkOptionName(response http.ResponseWriter, options []string, name string) bool {
	if !slices.Contains(options, name) {
		writeProblem(response, http.StatusBadRequest, "The deployment has no option "+name+".")

		return false
	}

	if isSecret(name) {
		writeProblem(response, http.StatusBadRequest,
			"Set the option "+name+" with the launcher.")

		return false
	}

	return true
}

func decodeBody(response http.ResponseWriter, request *http.Request, target any) bool {
	return decodeSizedBody(response, request, target, maxBodyBytes)
}

func decodeSizedBody(
	response http.ResponseWriter,
	request *http.Request,
	target any,
	limit int64,
) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, limit))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeProblem(response, http.StatusBadRequest, "The request body is invalid: "+err.Error())

		return false
	}

	return true
}

// flagValue writes a JSON option value in the form the launcher's flags take.
func flagValue(raw json.RawMessage) (string, bool) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}

	switch typed := value.(type) {
	case string:
		return typed, true
	case bool, float64:
		return strings.TrimSpace(string(raw)), true
	default:
		return "", false
	}
}

func (a *api) patchConfiguration(response http.ResponseWriter, request *http.Request) {
	deployment, options, ok := a.findEditableDeployment(response, request)
	if !ok {
		return
	}

	var patch configurationPatch
	if !decodeBody(response, request, &patch) {
		return
	}

	names := make([]string, 0, len(patch.Options))
	for name := range patch.Options {
		names = append(names, name)
	}

	slices.Sort(names)

	settings := make([]launcher.Setting, 0, len(names))

	for _, name := range names {
		if !checkOptionName(response, options, name) {
			return
		}

		value, valid := flagValue(patch.Options[name])
		if !valid {
			writeProblem(response, http.StatusBadRequest,
				"The value of option "+name+" must be a number, a boolean, or a string.")

			return
		}

		settings = append(settings, launcher.Setting{Name: name, Value: value})
	}

	if len(settings) > 0 {
		err := a.launcher.SetConfiguration(request.Context(), deployment.Path, settings)
		if !answerLauncherError(response, err) {
			return
		}
	}

	a.writeConfiguration(response, request, deployment)
}

func (a *api) resetConfiguration(response http.ResponseWriter, request *http.Request) {
	deployment, options, ok := a.findEditableDeployment(response, request)
	if !ok {
		return
	}

	var reset configurationReset
	if !decodeBody(response, request, &reset) {
		return
	}

	if len(reset.Options) == 0 {
		writeProblem(response, http.StatusBadRequest, "Name the options to reset.")

		return
	}

	for _, name := range reset.Options {
		if !checkOptionName(response, options, name) {
			return
		}
	}

	err := a.launcher.ResetConfiguration(request.Context(), deployment.Path, reset.Options)
	if !answerLauncherError(response, err) {
		return
	}

	a.writeConfiguration(response, request, deployment)
}

// answerLauncherError answers the request when err is set: with 400 when the
// launcher rejected the input, and with 500 when it could not be run.
func answerLauncherError(response http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}

	if launcherError, rejected := errors.AsType[*launcher.LauncherError](err); rejected {
		writeProblem(response, http.StatusBadRequest, launcherError.Message)
	} else {
		writeProblem(response, http.StatusInternalServerError, err.Error())
	}

	return false
}

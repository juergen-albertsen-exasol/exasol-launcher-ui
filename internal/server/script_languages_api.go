package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/exasol/exasol-local-ui/internal/launcher"
)

const (
	rustAlias    = "RUST"
	rustLanguage = "rust"

	slcUnsupported = "script language container management is only supported " +
		"for local deployments"
)

type officialScriptLanguageView struct {
	Alias         string   `json:"alias"`
	Language      string   `json:"language"`
	Flavor        string   `json:"flavor"`
	Version       string   `json:"version,omitempty"`
	Aliases       []string `json:"aliases"`
	Installed     bool     `json:"installed"`
	LatestRelease bool     `json:"latest_release"`
}

type customScriptLanguageView struct {
	Alias     string `json:"alias"`
	Language  string `json:"language"`
	Source    string `json:"source"`
	Available bool   `json:"available"`
}

type scriptLanguagesView struct {
	Official []officialScriptLanguageView `json:"official"`
	Custom   []customScriptLanguageView   `json:"custom"`
}

func (a *api) registerScriptLanguages(mux *http.ServeMux) {
	const scriptLanguages = "/api/v1/deployments/{name}/script-languages"

	mux.HandleFunc("GET "+scriptLanguages, a.listScriptLanguages)
}

// scriptLanguages returns the deployment's official containers, followed by
// the Rust container that `slc install rust` provides, and its other custom
// containers. Only local deployments run script language containers.
func (a *api) scriptLanguages(
	ctx context.Context,
	deployment launcher.Deployment,
) (scriptLanguagesView, error) {
	if deployment.Infrastructure != "local" {
		return scriptLanguagesView{}, &launcher.LauncherError{Message: slcUnsupported}
	}

	entries, err := a.launcher.ScriptLanguages(ctx, deployment.Path)
	if err != nil {
		return scriptLanguagesView{}, err
	}

	view := scriptLanguagesView{
		Official: []officialScriptLanguageView{},
		Custom:   []customScriptLanguageView{},
	}
	rust := officialScriptLanguageView{
		Alias: rustAlias, Language: rustLanguage, Aliases: []string{rustAlias}, LatestRelease: true,
	}

	for _, entry := range entries {
		switch {
		case entry.Type == "official" && len(entry.Aliases) > 0:
			view.Official = append(view.Official, officialScriptLanguageView{
				Alias:     entry.Aliases[0],
				Language:  entry.Language,
				Flavor:    entry.Flavor,
				Version:   entry.Version,
				Aliases:   entry.Aliases,
				Installed: entry.Installed,
			})
		case entry.Type == "custom" && strings.EqualFold(entry.Alias, rustAlias):
			rust.Installed = true
		case entry.Type == "custom":
			view.Custom = append(view.Custom, customScriptLanguageView{
				Alias:     entry.Alias,
				Language:  entry.Language,
				Source:    entry.Source,
				Available: entry.Available == nil || *entry.Available,
			})
		default:
		}
	}

	view.Official = append(view.Official, rust)

	return view, nil
}

func (a *api) listScriptLanguages(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	languages, err := a.scriptLanguages(request.Context(), deployment)
	if err != nil {
		answerRefusal(response, err)

		return
	}

	writeJSON(response, http.StatusOK, languages)
}

package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/operations"
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
	mux.HandleFunc("POST "+scriptLanguages+"/{alias}/install",
		a.changeScriptLanguage(operations.KindInstallScriptLanguage))
	mux.HandleFunc("POST "+scriptLanguages+"/{alias}/update",
		a.changeScriptLanguage(operations.KindUpdateScriptLanguage))
	mux.HandleFunc("POST "+scriptLanguages+"/{alias}/remove",
		a.changeScriptLanguage(operations.KindRemoveScriptLanguage))
	mux.HandleFunc("POST "+scriptLanguages+"/custom/install",
		a.changeCustomScriptLanguage(operations.KindInstallCustomScriptLanguage))
	mux.HandleFunc("POST "+scriptLanguages+"/custom/update",
		a.changeCustomScriptLanguage(operations.KindUpdateCustomScriptLanguage))
}

type scriptLanguageRequest struct {
	Restart *bool `json:"restart"`
}

type customScriptLanguageRequest struct {
	Alias    string `json:"alias"`
	Language string `json:"language"`
	Source   string `json:"source"`
	Restart  *bool  `json:"restart"`
}

func noRestart(restart *bool) bool {
	return restart != nil && !*restart
}

// offers reports whether the listing offers alias for kind: install and
// update take an official container, remove an installed or custom one.
func (v scriptLanguagesView) offers(kind operations.Kind, alias string) bool {
	for _, official := range v.Official {
		if official.Alias == alias {
			return kind != operations.KindRemoveScriptLanguage || official.Installed
		}
	}

	if kind != operations.KindRemoveScriptLanguage {
		return false
	}

	for _, custom := range v.Custom {
		if custom.Alias == alias {
			return true
		}
	}

	return false
}

// localDeployment finds the deployment named in the path and its script
// languages, answering the request itself when that fails.
func (a *api) localDeployment(
	response http.ResponseWriter,
	request *http.Request,
) (launcher.Deployment, scriptLanguagesView, bool) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return launcher.Deployment{}, scriptLanguagesView{}, false
	}

	languages, err := a.scriptLanguages(request.Context(), deployment)
	if err != nil {
		answerRefusal(response, err)

		return launcher.Deployment{}, scriptLanguagesView{}, false
	}

	return deployment, languages, true
}

// changeScriptLanguage runs kind on a container the listing offers for it.
// Only listed aliases reach the launcher, which keeps flags out of its command.
func (a *api) changeScriptLanguage(kind operations.Kind) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var options scriptLanguageRequest

		if request.ContentLength != 0 && !decodeBody(response, request, &options) {
			return
		}

		deployment, languages, ok := a.localDeployment(response, request)
		if !ok {
			return
		}

		alias := request.PathValue("alias")
		if !languages.offers(kind, alias) {
			writeProblem(response, http.StatusNotFound,
				"Deployment "+deployment.Name+" has no script language container "+alias+".")

			return
		}

		a.runOperationOn(response, deployment, kind, operations.Parameters{
			ScriptLanguage: alias,
			NoRestart:      noRestart(options.Restart),
		})
	}
}

func (a *api) changeCustomScriptLanguage(kind operations.Kind) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var options customScriptLanguageRequest

		if !decodeBody(response, request, &options) {
			return
		}

		if strings.TrimSpace(options.Alias) == "" || strings.TrimSpace(options.Language) == "" ||
			strings.TrimSpace(options.Source) == "" {
			writeProblem(response, http.StatusBadRequest,
				"A custom script language container needs an alias, a language, and a source.")

			return
		}

		deployment, _, ok := a.localDeployment(response, request)
		if !ok {
			return
		}

		a.runOperationOn(response, deployment, kind, operations.Parameters{
			ScriptLanguage: options.Alias,
			Language:       options.Language,
			Source:         options.Source,
			NoRestart:      noRestart(options.Restart),
		})
	}
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
	_, languages, ok := a.localDeployment(response, request)
	if !ok {
		return
	}

	writeJSON(response, http.StatusOK, languages)
}

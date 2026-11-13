package server

import (
	"net/http"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	officialScriptLanguages = `
	  {"type": "official", "language": "java", "flavor": "java-17", "version": "11.2.0",
	   "aliases": ["JAVA", "JAVA17"], "installed": true},
	  {"type": "official", "language": "python", "flavor": "python-3.12", "version": "11.2.0",
	   "aliases": ["PYTHON3", "PYTHON312"], "installed": true},
	  {"type": "official", "language": "r", "flavor": "r-4.4", "version": "11.2.0",
	   "aliases": ["R", "R44"], "installed": false}`
	customMypy = `
	  {"type": "custom", "language": "python", "alias": "MYPY3",
	   "source": "/home/me/my-python.tar.gz", "installed": true, "available": true}`
	installedScriptLanguages = `[` + officialScriptLanguages + `,` + customMypy + `]`
)

func scriptLanguageListArgs(path string) []string {
	return []string{"slc", "list", "--json", "--deployment-dir", path}
}

func newScriptLanguagesFake(t *testing.T) *launchertest.Fake {
	t.Helper()

	fake := launchertest.NewFake(t)
	fake.Respond(twoDeployments, listArgs...)
	fake.Respond(installedScriptLanguages, scriptLanguageListArgs(defaultDir)...)

	return fake
}

func TestScriptLanguagesListOfficialRustAndCustomContainers(t *testing.T) {
	t.Parallel()

	response := callAPI(t, newScriptLanguagesFake(t),
		"/api/v1/deployments/default/script-languages")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, scriptLanguagesView{
		Official: []officialScriptLanguageView{
			{
				Alias: "JAVA", Language: "java", Flavor: "java-17", Version: "11.2.0",
				Aliases: []string{"JAVA", "JAVA17"}, Installed: true,
			},
			{
				Alias: "PYTHON3", Language: "python", Flavor: "python-3.12", Version: "11.2.0",
				Aliases: []string{"PYTHON3", "PYTHON312"}, Installed: true,
			},
			{
				Alias: "R", Language: "r", Flavor: "r-4.4", Version: "11.2.0",
				Aliases: []string{"R", "R44"},
			},
			{Alias: "RUST", Language: "rust", Aliases: []string{"RUST"}, LatestRelease: true},
		},
		Custom: []customScriptLanguageView{
			{
				Alias: "MYPY3", Language: "python", Source: "/home/me/my-python.tar.gz",
				Available: true,
			},
		},
	}, decode[scriptLanguagesView](t, response))
}

func TestAnInstalledRustContainerIsListedWithTheOfficialOnes(t *testing.T) {
	t.Parallel()

	fake := newScriptLanguagesFake(t)
	fake.Respond(`[`+officialScriptLanguages+`,
	  {"type": "custom", "language": "rust", "alias": "RUST",
	   "source": "https://github.com/exasol-labs/language-container-rs/releases/rust.tar.gz",
	   "installed": true, "available": false}]`,
		scriptLanguageListArgs(defaultDir)...)

	response := callAPI(t, fake, "/api/v1/deployments/default/script-languages")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	languages := decode[scriptLanguagesView](t, response)
	assert.Equal(t, officialScriptLanguageView{
		Alias: "RUST", Language: "rust", Aliases: []string{"RUST"},
		Installed: true, LatestRelease: true,
	}, languages.Official[len(languages.Official)-1])
	assert.Empty(t, languages.Custom)
}

func TestAnUnavailableCustomContainerIsReported(t *testing.T) {
	t.Parallel()

	fake := newScriptLanguagesFake(t)
	fake.Respond(`[{"type": "custom", "language": "python", "alias": "MYPY3",
	  "source": "/gone.tar.gz", "installed": true, "available": false}]`,
		scriptLanguageListArgs(defaultDir)...)

	response := callAPI(t, fake, "/api/v1/deployments/default/script-languages")

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.False(t, decode[scriptLanguagesView](t, response).Custom[0].Available)
}

func TestACloudDeploymentRunsNoScriptLanguageContainers(t *testing.T) {
	t.Parallel()

	fake := newScriptLanguagesFake(t)

	response := callAPI(t, fake, "/api/v1/deployments/test/script-languages")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t,
		"script language container management is only supported for local deployments",
		decode[problem](t, response).Detail)
	assert.NotContains(t, fake.Calls(),
		"slc list --json --deployment-dir /deployments/test")
}

func TestScriptLanguagesOfAnUnknownDeploymentAreNotFound(t *testing.T) {
	t.Parallel()

	response := callAPI(t, newScriptLanguagesFake(t),
		"/api/v1/deployments/other/script-languages")

	assert.Equal(t, http.StatusNotFound, response.Code)
}

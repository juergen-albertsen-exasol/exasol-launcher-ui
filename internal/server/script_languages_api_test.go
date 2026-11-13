package server

import (
	"net/http"
	"testing"

	"github.com/exasol/exasol-launcher-ui/internal/launcher/launchertest"
	"github.com/exasol/exasol-launcher-ui/internal/operations"
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

func newScriptLanguageOperationsFixture(t *testing.T) operationsFixture {
	t.Helper()

	fixture := newOperationsFixture(t)
	fixture.fake.Respond(installedScriptLanguages, scriptLanguageListArgs(defaultDir)...)

	return fixture
}

func (f operationsFixture) listedScriptLanguages(t *testing.T) scriptLanguagesView {
	t.Helper()

	response := f.call(t, http.MethodGet, "/api/v1/deployments/default/script-languages")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	return decode[scriptLanguagesView](t, response)
}

func slcArgs(args ...string) []string {
	return append(args, "--deployment-dir", defaultDir)
}

func TestInstallingAnOfficialContainer(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)
	fixture.fake.Respond(`[
	  {"type": "official", "language": "python", "flavor": "python-3.12", "version": "11.2.0",
	   "aliases": ["PYTHON3", "PYTHON312"], "installed": false}]`,
		scriptLanguageListArgs(defaultDir)...)

	installArgs := slcArgs("slc", "install", "PYTHON3", "--auto-approve")
	fixture.fake.RespondWithProgress("Pulling the image\n", "", installArgs...)
	fixture.fake.RespondAfter(installArgs, `[
	  {"type": "official", "language": "python", "flavor": "python-3.12", "version": "11.2.0",
	   "aliases": ["PYTHON3", "PYTHON312"], "installed": true}]`,
		scriptLanguageListArgs(defaultDir)...)

	response := fixture.call(t, http.MethodPost,
		"/api/v1/deployments/default/script-languages/PYTHON3/install")

	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())

	started := decode[operationView](t, response)
	assert.Equal(t, operations.KindInstallScriptLanguage, started.Kind)
	assert.Equal(t, "PYTHON3", started.ScriptLanguage)
	assert.Equal(t, "running", started.Status)
	assert.Equal(t, "/api/v1/operations/"+started.ID, response.Header().Get("Location"))

	fixture.waitFor(t, started.ID, "succeeded")
	assert.True(t, fixture.listedScriptLanguages(t).Official[0].Installed)
}

func TestRemovingAContainerWithoutARestart(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)
	fixture.fake.RespondWithProgress("", "", slcArgs("slc", "remove", "JAVA", "--no-restart")...)

	response := fixture.postBody(t,
		"/api/v1/deployments/default/script-languages/JAVA/remove", `{"restart": false}`)

	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())

	started := decode[operationView](t, response)
	assert.Equal(t, operations.KindRemoveScriptLanguage, started.Kind)
	fixture.waitFor(t, started.ID, "succeeded")
	assert.Contains(t, fixture.fake.Calls(),
		"slc remove JAVA --no-restart --deployment-dir "+defaultDir)
}

func TestRemovingACustomContainer(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)
	fixture.fake.RespondWithProgress("", "", slcArgs("slc", "remove", "MYPY3", "--auto-approve")...)

	started := fixture.post(t, "/api/v1/deployments/default/script-languages/MYPY3/remove")

	assert.Equal(t, operations.KindRemoveScriptLanguage, started.Kind)
	fixture.waitFor(t, started.ID, "succeeded")
}

func TestUpdatingTheRustContainer(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)
	fixture.fake.RespondWithProgress("", "", slcArgs("slc", "update", "RUST", "--auto-approve")...)

	started := fixture.post(t, "/api/v1/deployments/default/script-languages/RUST/update")

	assert.Equal(t, operations.KindUpdateScriptLanguage, started.Kind)
	assert.Equal(t, "RUST", started.ScriptLanguage)
	fixture.waitFor(t, started.ID, "succeeded")
}

func TestInstallingACustomContainer(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)
	fixture.fake.Respond(`[`+officialScriptLanguages+`]`, scriptLanguageListArgs(defaultDir)...)

	installArgs := slcArgs("slc", "custom", "install", "--alias=MYPY3", "--language=python",
		"--source=/home/me/my-python.tar.gz", "--auto-approve")
	fixture.fake.RespondWithProgress("Importing the container\n", "", installArgs...)
	fixture.fake.RespondAfter(installArgs, installedScriptLanguages,
		scriptLanguageListArgs(defaultDir)...)

	response := fixture.postBody(t, "/api/v1/deployments/default/script-languages/custom/install",
		`{"alias": "MYPY3", "language": "python", "source": "/home/me/my-python.tar.gz"}`)

	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())

	started := decode[operationView](t, response)
	assert.Equal(t, operations.KindInstallCustomScriptLanguage, started.Kind)
	assert.Equal(t, "MYPY3", started.ScriptLanguage)

	fixture.waitFor(t, started.ID, "succeeded")
	assert.Equal(t, "MYPY3", fixture.listedScriptLanguages(t).Custom[0].Alias)
}

func TestARefusedCustomContainerFailsTheOperation(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)
	fixture.fake.Fail("Error: the archive is no script language container\n", 1,
		slcArgs("slc", "custom", "update", "--alias=MYPY3", "--language=python",
			"--source=https://example.com/c.tar.gz", "--no-restart")...)

	response := fixture.postBody(t, "/api/v1/deployments/default/script-languages/custom/update",
		`{"alias": "MYPY3", "language": "python", "source": "https://example.com/c.tar.gz",
		  "restart": false}`)

	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())

	started := decode[operationView](t, response)
	assert.Equal(t, operations.KindUpdateCustomScriptLanguage, started.Kind)

	failed := fixture.waitFor(t, started.ID, "failed")
	assert.Contains(t, failed.Error, "the archive is no script language container")
}

func TestAnIncompleteCustomRequestIsRejected(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)

	response := fixture.postBody(t, "/api/v1/deployments/default/script-languages/custom/install",
		`{"alias": "MYPY3", "language": "python", "source": " "}`)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	for _, call := range fixture.fake.Calls() {
		assert.NotContains(t, call, "slc custom")
	}
}

func TestAnUnlistedScriptLanguageIsNotFound(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an unlisted alias":           "--all/install",
		"removing an absent official": "R/remove",
		"updating a custom container": "MYPY3/update",
	}

	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fixture := newScriptLanguageOperationsFixture(t)

			response := fixture.call(t, http.MethodPost,
				"/api/v1/deployments/default/script-languages/"+target)

			assert.Equal(t, http.StatusNotFound, response.Code)

			for _, call := range fixture.fake.Calls() {
				assert.NotRegexp(t, `^slc (install|update|remove)`, call)
			}
		})
	}
}

func TestScriptLanguagesOfACloudDeploymentCannotChange(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)

	response := fixture.call(t, http.MethodPost,
		"/api/v1/deployments/test/script-languages/JAVA/install")

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Equal(t, slcUnsupported, decode[problem](t, response).Detail)
}

func TestAScriptLanguageOperationWaitsForARunningOperation(t *testing.T) {
	t.Parallel()

	fixture := newScriptLanguageOperationsFixture(t)
	startArgs := lifecycleArgs("start", defaultDir)
	fixture.fake.RespondWithProgress("", runningResult, startArgs...)
	fixture.fake.Hold(startArgs...)

	defer fixture.fake.Release(startArgs...)

	first := fixture.call(t, http.MethodPost, "/api/v1/deployments/default/start")
	require.Equal(t, http.StatusAccepted, first.Code)

	second := fixture.call(t, http.MethodPost,
		"/api/v1/deployments/default/script-languages/R/install")
	assert.Equal(t, http.StatusConflict, second.Code)
}

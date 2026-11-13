package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/exasol/exasol-local-ui/internal/launcher"
	"github.com/exasol/exasol-local-ui/internal/launcher/launchertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	configurationListing = `[
	  {"name": "default", "path": "/deployments/default", "status": "stopped",
	   "infrastructure": "local", "installation": "local"},
	  {"name": "running", "path": "/deployments/running", "status": "database_ready",
	   "infrastructure": "local", "installation": "local"},
	  {"name": "aws", "path": "/deployments/aws", "status": "initialized",
	   "infrastructure": "aws", "installation": "ubuntu"}
	]`
	localIdentity = `{"name": "local", "displayName": "Exasol Local",
	  "description": "Exasol Local VM."}`
	awsPassword = "aws-secret-password"
)

func localConfiguration(cpuCount, memory string) string {
	return `{
	  "infrastructure": {"identity": ` + localIdentity + `, "options": {
	    "cpu-count": ` + cpuCount + `, "data-size-gb": 100,
	    "memory-mb": ` + memory + `, "ports": "db:8564"}},
	  "installation": {"identity": ` + localIdentity + `, "options": {}}
	}`
}

func configArgs(path string, args ...string) []string {
	return append(append([]string{"config"}, args...), "--deployment-dir", path)
}

func getConfigArgs(path string) []string {
	return configArgs(path, "get", "--json")
}

type configurationFixture struct {
	fake    *launchertest.Fake
	handler http.Handler
}

func newConfigurationFixture(t *testing.T) configurationFixture {
	t.Helper()

	fake := launchertest.NewFake(t)
	fake.Respond(configurationListing, listArgs...)
	fake.Respond(localConfiguration("2", "24576"), getConfigArgs("/deployments/default")...)
	fake.Respond(localConfiguration("2", "24576"), getConfigArgs("/deployments/running")...)
	fake.Respond(`{
	  "infrastructure": {
	    "identity": {"name": "aws", "displayName": "Exasol in AWS", "description": "AWS."},
	    "options": {"cluster-size": 1, "db-password": "`+awsPassword+`",
	                "s3-archive-enabled": true}},
	  "installation": {
	    "identity": {"name": "ubuntu", "displayName": "Exasol on Ubuntu", "description": "Ubuntu."},
	    "options": {"exasol-version": "2026.1.0"}}
	}`, getConfigArgs("/deployments/aws")...)

	return configurationFixture{
		fake:    fake,
		handler: NewHandler(Config{Launcher: launcher.New(fake.Executable)}),
	}
}

func (f configurationFixture) call(
	t *testing.T,
	method, target, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)

	return recorder
}

func optionValues(view configurationView) map[string]string {
	values := map[string]string{}

	for _, preset := range []presetView{view.Infrastructure, view.Installation} {
		for _, option := range preset.Options {
			values[option.Name] = string(option.Value)
		}
	}

	return values
}

func TestAStoppedLocalDeploymentIsEditable(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)

	response := fixture.call(t, http.MethodGet, "/api/v1/deployments/default/configuration", "")

	require.Equal(t, http.StatusOK, response.Code)

	view := decode[configurationView](t, response)
	assert.True(t, view.Editable)
	assert.Equal(t, "start", view.ApplyWith)
	assert.Equal(t, "local", view.Infrastructure.Name)
	assert.Equal(t, "Exasol Local", view.Infrastructure.DisplayName)
	assert.Equal(t, "Exasol Local VM.", view.Infrastructure.Description)
	assert.Equal(t, []optionView{
		{Name: "cpu-count", Value: []byte("2")},
		{Name: "data-size-gb", Value: []byte("100")},
		{Name: "memory-mb", Value: []byte("24576")},
		{Name: "ports", Value: []byte(`"db:8564"`)},
	}, view.Infrastructure.Options)
	assert.Equal(t, []optionView{}, view.Installation.Options)
}

func TestARunningDeploymentIsReadOnly(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)

	response := fixture.call(t, http.MethodGet, "/api/v1/deployments/running/configuration", "")

	require.Equal(t, http.StatusOK, response.Code)

	view := decode[configurationView](t, response)
	assert.False(t, view.Editable)
	assert.Empty(t, view.ApplyWith)
	assert.Len(t, view.Infrastructure.Options, 4)
	assert.NotContains(t, response.Body.String(), "apply_with")
}

func TestAnInitializedDeploymentIsAppliedWithDeploy(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)

	view := decode[configurationView](t,
		fixture.call(t, http.MethodGet, "/api/v1/deployments/aws/configuration", ""))

	assert.True(t, view.Editable)
	assert.Equal(t, "deploy", view.ApplyWith)
}

func TestPasswordOptionsAreWithheld(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)

	response := fixture.call(t, http.MethodGet, "/api/v1/deployments/aws/configuration", "")

	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), awsPassword)
	assert.Contains(t, decode[configurationView](t, response).Infrastructure.Options,
		optionView{Name: "db-password", Secret: true})
}

func TestConfigurationOfAnUnknownDeploymentIsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)

	assert.Equal(t, http.StatusNotFound,
		fixture.call(t, http.MethodGet, "/api/v1/deployments/unknown/configuration", "").Code)
}

func TestSettingAnOption(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)
	setArgs := configArgs("/deployments/default", "set", "--cpu-count=4")
	fixture.fake.Respond("", setArgs...)
	fixture.fake.RespondAfter(setArgs, localConfiguration("4", "24576"),
		getConfigArgs("/deployments/default")...)

	response := fixture.call(t, http.MethodPatch, "/api/v1/deployments/default/configuration",
		`{"options": {"cpu-count": 4}}`)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	values := optionValues(decode[configurationView](t, response))
	assert.Equal(t, "4", values["cpu-count"])
	assert.Equal(t, "24576", values["memory-mb"])
	assert.Equal(t, `"db:8564"`, values["ports"])
}

func TestResettingOptions(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)
	fixture.fake.Respond(localConfiguration("4", "8192"), getConfigArgs("/deployments/default")...)
	resetArgs := configArgs("/deployments/default", "reset", "cpu-count", "memory-mb")
	fixture.fake.Respond("", resetArgs...)
	fixture.fake.RespondAfter(resetArgs, localConfiguration("2", "24576"),
		getConfigArgs("/deployments/default")...)

	response := fixture.call(t, http.MethodPost, "/api/v1/deployments/default/configuration/reset",
		`{"options": ["cpu-count", "memory-mb"]}`)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	values := optionValues(decode[configurationView](t, response))
	assert.Equal(t, "2", values["cpu-count"])
	assert.Equal(t, "24576", values["memory-mb"])
}

func TestARunningDeploymentRefusesChanges(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)

	patch := fixture.call(t, http.MethodPatch, "/api/v1/deployments/running/configuration",
		`{"options": {"cpu-count": 4}}`)
	reset := fixture.call(t, http.MethodPost, "/api/v1/deployments/running/configuration/reset",
		`{"options": ["cpu-count"]}`)

	assert.Equal(t, http.StatusConflict, patch.Code)
	assert.Contains(t, decode[problem](t, patch).Detail, "Stop deployment running")
	assert.Equal(t, http.StatusConflict, reset.Code)

	for _, call := range fixture.fake.Calls() {
		assert.NotContains(t, call, "config set")
		assert.NotContains(t, call, "config reset")
	}
}

func TestARejectedValueIsReported(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)
	fixture.fake.Fail("Usage:\n\texasol config set [flags]\n\n"+
		"Error: invalid argument \"many\" for \"--cpu-count\" flag: invalid number \"many\"\n",
		1, configArgs("/deployments/default", "set", "--cpu-count=many")...)

	response := fixture.call(t, http.MethodPatch, "/api/v1/deployments/default/configuration",
		`{"options": {"cpu-count": "many"}}`)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, `invalid argument "many" for "--cpu-count" flag: invalid number "many"`,
		decode[problem](t, response).Detail)
}

func TestOnlyTheDeploymentsOwnOptionsReachTheLauncher(t *testing.T) {
	t.Parallel()

	fixture := newConfigurationFixture(t)

	for _, body := range []string{
		`{"options": {"deployment-dir": "/elsewhere"}}`,
		`{"options": {"db-password": "new-password"}}`,
		`{"options": {"cluster-size": null}}`,
	} {
		response := fixture.call(t, http.MethodPatch, "/api/v1/deployments/aws/configuration", body)
		assert.Equal(t, http.StatusBadRequest, response.Code, body)
	}

	for _, body := range []string{`{"options": ["--all"]}`, `{"options": []}`} {
		response := fixture.call(t, http.MethodPost,
			"/api/v1/deployments/aws/configuration/reset", body)
		assert.Equal(t, http.StatusBadRequest, response.Code, body)
	}

	for _, call := range fixture.fake.Calls() {
		assert.NotContains(t, call, "config set")
		assert.NotContains(t, call, "config reset")
	}
}

package server

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/launcher"
)

const (
	notLocal         = "this deployment is not using the local preset"
	sidecarPortName  = "sidecar"
	redactedPassword = "********"
)

type localRuntimeView struct {
	Platform          string     `json:"platform"`
	PlatformSupported bool       `json:"platform_supported"`
	VMRunning         bool       `json:"vm_running"`
	DatabaseReady     *bool      `json:"database_ready,omitempty"`
	DatabaseError     string     `json:"database_error,omitempty"`
	Warning           string     `json:"warning,omitempty"`
	Message           string     `json:"message,omitempty"`
	Ports             []portView `json:"ports"`
}

type portView struct {
	Name         string `json:"name"`
	HostPort     *int   `json:"host_port,omitempty"`
	Reachability string `json:"reachability"`
}

func (a *api) registerDiagnostics(mux *http.ServeMux) {
	const diagnostics = "/api/v1/deployments/{name}/diagnostics"

	mux.HandleFunc("GET "+diagnostics+"/local", a.getLocalRuntime)
	mux.HandleFunc("GET "+diagnostics+"/info", a.getDiagnosticInfo)
}

func (a *api) getLocalRuntime(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	if deployment.Infrastructure != "local" {
		writeProblem(response, http.StatusConflict, notLocal)

		return
	}

	diagnostics, err := a.launcher.DiagnoseLocal(request.Context(), deployment.Path)
	if err != nil {
		answerRefusal(response, err)

		return
	}

	writeJSON(response, http.StatusOK, localRuntimeView{
		Platform:          diagnostics.Platform,
		PlatformSupported: diagnostics.PlatformSupported,
		VMRunning:         diagnostics.VMRunning != nil && *diagnostics.VMRunning,
		DatabaseReady:     diagnostics.DatabaseReady,
		DatabaseError:     diagnostics.DatabaseError,
		Warning:           diagnostics.Warning,
		Message:           diagnostics.Message,
		Ports:             a.forwardedPorts(request.Context(), deployment, diagnostics),
	})
}

// forwardedPort is a port of `diag local`: db, ui, or the endpoint index of a
// sidecar, as in sidecar.mcp.0.
type forwardedPort struct {
	key     string
	sidecar string
	index   int
}

func parsePort(key string) forwardedPort {
	rest, isSidecar := strings.CutPrefix(key, sidecarPortName+".")
	if !isSidecar {
		return forwardedPort{key: key}
	}

	sidecar, number, found := strings.CutLast(rest, ".")
	if !found {
		return forwardedPort{key: key}
	}

	index, err := strconv.Atoi(number)
	if err != nil {
		return forwardedPort{key: key}
	}

	return forwardedPort{key: key, sidecar: sidecar, index: index}
}

func (p forwardedPort) name() string {
	if p.sidecar == "" {
		return p.key
	}

	name := sidecarPortName + " " + p.sidecar
	if p.index > 0 {
		name += " " + strconv.Itoa(p.index+1)
	}

	return name
}

// rank orders db, then ui, then other ports, then sidecar ports.
func (p forwardedPort) rank() int {
	switch {
	case p.key == "db":
		return 0
	case p.key == "ui":
		return 1
	case p.sidecar == "":
		return 2 //nolint:mnd // Third in order.
	default:
		return 3 //nolint:mnd // Last in order.
	}
}

// forwardedPorts lists the reported ports with readable names and host ports.
// Sidecar host ports come from the sidecar status, which is only read when
// sidecar ports are reported and left out when it fails.
func (a *api) forwardedPorts(
	ctx context.Context,
	deployment launcher.Deployment,
	diagnostics launcher.LocalDiagnostics,
) []portView {
	ports := make([]forwardedPort, 0, len(diagnostics.PortHealth))
	hasSidecars := false

	for key := range diagnostics.PortHealth {
		port := parsePort(key)
		hasSidecars = hasSidecars || port.sidecar != ""
		ports = append(ports, port)
	}

	slices.SortFunc(ports, func(left, right forwardedPort) int {
		return cmp.Or(
			cmp.Compare(left.rank(), right.rank()),
			cmp.Compare(left.sidecar, right.sidecar),
			cmp.Compare(left.index, right.index),
			cmp.Compare(left.key, right.key),
		)
	})

	endpoints := map[string][]launcher.SidecarEndpoint{}

	if hasSidecars {
		if sidecars, err := a.launcher.SidecarStatus(ctx, deployment.Path); err == nil {
			for _, sidecar := range sidecars {
				endpoints[sidecar.Name] = sidecar.Endpoints
			}
		}
	}

	views := make([]portView, 0, len(ports))

	for _, port := range ports {
		view := portView{Name: port.name(), Reachability: diagnostics.PortHealth[port.key]}

		if hostPort, found := diagnostics.Ports[port.key]; found && port.sidecar == "" {
			view.HostPort = &hostPort
		}

		if sidecarEndpoints := endpoints[port.sidecar]; port.sidecar != "" &&
			port.index < len(sidecarEndpoints) {
			view.HostPort = &sidecarEndpoints[port.index].Port
		}

		views = append(views, view)
	}

	return views
}

func (a *api) getDiagnosticInfo(response http.ResponseWriter, request *http.Request) {
	deployment, ok := a.findDeployment(response, request)
	if !ok {
		return
	}

	report, err := a.launcher.DiagnosticInfo(request.Context(), deployment.Path)
	if err != nil {
		answerRefusal(response, err)

		return
	}

	if password, err := database.ReadPassword(deployment.Path); err == nil {
		report = withoutPassword(report, password)
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(report)
}

// withoutPassword masks password in every string of report, however the
// string escapes it, keeping the report's formatting and key order.
func withoutPassword(report json.RawMessage, password string) json.RawMessage {
	masked := make([]byte, 0, len(report))

	for index := 0; index < len(report); index++ {
		if report[index] != '"' {
			masked = append(masked, report[index])

			continue
		}

		end := index + 1
		for end < len(report) && report[end] != '"' {
			if report[end] == '\\' {
				end++
			}

			end++
		}

		literal := report[index:min(end+1, len(report))]
		masked = append(masked, maskedLiteral(literal, password)...)
		index = end
	}

	return masked
}

func maskedLiteral(literal []byte, password string) []byte {
	var text string
	if json.Unmarshal(literal, &text) != nil || !strings.Contains(text, password) {
		return literal
	}

	replaced, err := json.Marshal(strings.ReplaceAll(text, password, redactedPassword))
	if err != nil {
		return literal
	}

	return replaced
}

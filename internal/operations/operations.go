// Package operations runs long launcher commands in the background and keeps
// their progress and outcome.
package operations

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/launcher"
)

// Kind names what an operation does.
type Kind string

const (
	KindStart            Kind = "start"
	KindStop             Kind = "stop"
	KindDeploy           Kind = "deploy"
	KindDestroy          Kind = "destroy"
	KindDestroyAndRemove Kind = "destroy_and_remove"
	KindRemove           Kind = "remove"
	KindEnableSidecar    Kind = "enable_sidecar"
	KindDisableSidecar   Kind = "disable_sidecar"
	KindRemoveSidecar    Kind = "remove_sidecar"

	KindInstallScriptLanguage       Kind = "install_script_language"
	KindUpdateScriptLanguage        Kind = "update_script_language"
	KindRemoveScriptLanguage        Kind = "remove_script_language"
	KindInstallCustomScriptLanguage Kind = "install_custom_script_language"
	KindUpdateCustomScriptLanguage  Kind = "update_custom_script_language"
)

const approve = "--auto-approve"

// Parameters say what an operation acts on besides its deployment.
type Parameters struct {
	// Sidecar names the sidecar of the sidecar kinds.
	Sidecar string
	// ScriptLanguage is the container alias of the script language kinds.
	ScriptLanguage string
	// Language and Source describe a custom script language container.
	Language string
	Source   string
	// NoRestart makes a script language change apply on the next start
	// instead of restarting the database now.
	NoRestart bool
}

// command returns the launcher arguments that run kind with params, without
// the deployment directory, and whether the command prints a lifecycle result.
func (k Kind) command(params Parameters) ([]string, bool) {
	const sidecarCommand = "sidecar"

	sidecar := params.Sidecar

	switch k {
	case KindStart, KindStop:
		return []string{string(k), "--json"}, true
	case KindDeploy:
		return []string{"deploy"}, false
	case KindDestroy:
		return []string{"destroy", approve}, false
	case KindDestroyAndRemove:
		return []string{"destroy", "--remove", approve}, false
	case KindRemove:
		return []string{"remove", approve}, false
	case KindEnableSidecar:
		return []string{sidecarCommand, "enable", sidecar}, false
	case KindDisableSidecar:
		return []string{sidecarCommand, "disable", sidecar}, false
	case KindRemoveSidecar:
		return []string{sidecarCommand, "disable", sidecar, "--remove"}, false
	case KindInstallScriptLanguage, KindUpdateScriptLanguage, KindRemoveScriptLanguage,
		KindInstallCustomScriptLanguage, KindUpdateCustomScriptLanguage:
		return k.scriptLanguageCommand(params), false
	default:
		return []string{string(k)}, false
	}
}

// scriptLanguageActions holds the `exasol slc` subcommands of the script
// language kinds.
var scriptLanguageActions = map[Kind][]string{
	KindInstallScriptLanguage:       {"install"},
	KindUpdateScriptLanguage:        {"update"},
	KindRemoveScriptLanguage:        {"remove"},
	KindInstallCustomScriptLanguage: {"custom", "install"},
	KindUpdateCustomScriptLanguage:  {"custom", "update"},
}

// scriptLanguageCommand passes custom container values as --flag=value, so
// that no value is taken for a flag.
func (k Kind) scriptLanguageCommand(params Parameters) []string {
	args := append([]string{"slc"}, scriptLanguageActions[k]...)

	if k == KindInstallCustomScriptLanguage || k == KindUpdateCustomScriptLanguage {
		args = append(args,
			"--alias="+params.ScriptLanguage,
			"--language="+params.Language,
			"--source="+params.Source)
	} else {
		args = append(args, params.ScriptLanguage)
	}

	if params.NoRestart {
		return append(args, "--no-restart")
	}

	return append(args, approve)
}

// Status is an operation's progress.
type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

const (
	maxFinished = 50
	maxLines    = 5000
	redacted    = "********"
)

// ErrBusy reports that an operation already runs on the deployment.
var ErrBusy = errors.New("an operation already runs on this deployment")

// Snapshot is an operation's state at one moment.
type Snapshot struct {
	ID         string
	Deployment string
	Kind       Kind
	// Sidecar and ScriptLanguage name what the operation acts on, if any.
	Sidecar        string
	ScriptLanguage string
	Status         Status
	StartedAt      time.Time
	FinishedAt     *time.Time
	Result         *launcher.LifecycleResult
	Error          string
	LineCount      int
}

// Streamer runs a launcher command while streaming its progress.
type Streamer interface {
	Stream(ctx context.Context, onLine func(line string), target any, args ...string) error
}

// Manager runs operations and remembers the recent ones.
type Manager struct {
	//nolint:containedctx // Operations outlive their request and end with the server.
	ctx      context.Context
	streamer Streamer
	finished func()
	now      func() time.Time
	keep     int

	mu         sync.Mutex
	operations map[string]*operation
	order      []string
	running    map[string]string
}

type operation struct {
	snapshot Snapshot
	lines    []string
	secret   string
	watchers []chan struct{}
}

// NewManager returns a manager whose operations end when ctx ends. finished,
// when set, runs after each operation ends.
func NewManager(ctx context.Context, streamer Streamer, finished func()) *Manager {
	return &Manager{
		ctx:        ctx,
		streamer:   streamer,
		finished:   finished,
		now:        time.Now,
		keep:       maxFinished,
		operations: map[string]*operation{},
		running:    map[string]string{},
	}
}

// Start runs kind on the deployment in the background. secret, when not
// empty, is withheld from the progress output.
func (m *Manager) Start(
	deployment launcher.Deployment,
	kind Kind,
	secret string,
) (Snapshot, error) {
	return m.StartWith(deployment, kind, Parameters{}, secret)
}

// StartWith runs kind with params on the deployment in the background, like
// Start.
func (m *Manager) StartWith(
	deployment launcher.Deployment,
	kind Kind,
	params Parameters,
	secret string,
) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, busy := m.running[deployment.Name]; busy {
		return Snapshot{}, ErrBusy
	}

	current := &operation{
		snapshot: Snapshot{
			ID:             rand.Text(),
			Deployment:     deployment.Name,
			Kind:           kind,
			Sidecar:        params.Sidecar,
			ScriptLanguage: params.ScriptLanguage,
			Status:         StatusRunning,
			StartedAt:      m.now(),
		},
		secret: secret,
	}
	m.operations[current.snapshot.ID] = current
	m.order = append(m.order, current.snapshot.ID)
	m.running[deployment.Name] = current.snapshot.ID

	args, reportsResult := kind.command(params)
	go m.run(current, reportsResult, append(args, "--deployment-dir", deployment.Path)...)

	return current.snapshot, nil
}

// Running returns the operation that runs on the named deployment, if any.
func (m *Manager) Running(deployment string) (Snapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	operationID, running := m.running[deployment]
	if !running {
		return Snapshot{}, false
	}

	return m.operations[operationID].snapshot, true
}

// List returns the known operations, newest first.
func (m *Manager) List() []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshots := make([]Snapshot, 0, len(m.order))
	for _, id := range slices.Backward(m.order) {
		snapshots = append(snapshots, m.operations[id].snapshot)
	}

	return snapshots
}

// Get returns the operation with the given id and its progress output.
func (m *Manager) Get(id string) (Snapshot, []string, bool) {
	return m.Since(id, 0)
}

// Since returns the operation with the given id and its progress output from
// line from on.
func (m *Manager) Since(id string, from int) (Snapshot, []string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, found := m.operations[id]
	if !found {
		return Snapshot{}, nil, false
	}

	if from > len(current.lines) {
		from = len(current.lines)
	}

	return current.snapshot, slices.Clone(current.lines[from:]), true
}

// Watch returns a channel that receives a value whenever the operation gains
// output or ends, and a function that stops watching.
func (m *Manager) Watch(operationID string) (<-chan struct{}, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	changes := make(chan struct{}, 1)

	current, found := m.operations[operationID]
	if !found {
		close(changes)

		return changes, func() {}
	}

	current.watchers = append(current.watchers, changes)

	return changes, func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		current.watchers = slices.DeleteFunc(current.watchers, func(watcher chan struct{}) bool {
			return watcher == changes
		})
	}
}

func (m *Manager) run(current *operation, reportsResult bool, args ...string) {
	var (
		result *launcher.LifecycleResult
		target any
	)

	if reportsResult {
		result = &launcher.LifecycleResult{}
		target = result
	}

	onLine := func(line string) { m.addLine(current, line) }
	err := m.streamer.Stream(m.ctx, onLine, target, args...)

	m.mu.Lock()
	finishedAt := m.now()
	current.snapshot.FinishedAt = &finishedAt

	if err != nil {
		current.snapshot.Status = StatusFailed
		current.snapshot.Error = redact(err.Error(), current.secret)
	} else {
		current.snapshot.Status = StatusSucceeded
		current.snapshot.Result = result
	}

	delete(m.running, current.snapshot.Deployment)
	notify(current)
	m.forgetOldest()
	m.mu.Unlock()

	if m.finished != nil {
		m.finished()
	}
}

func (m *Manager) addLine(current *operation, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(current.lines) < maxLines {
		current.lines = append(current.lines, redact(line, current.secret))
		current.snapshot.LineCount = len(current.lines)
	}

	notify(current)
}

// notify wakes the watchers of current; the caller holds m.mu.
func notify(current *operation) {
	for _, watcher := range current.watchers {
		select {
		case watcher <- struct{}{}:
		default:
		}
	}
}

// forgetOldest drops the oldest finished operations beyond the limit; the
// caller holds m.mu.
func (m *Manager) forgetOldest() {
	finished := 0
	for index, operationID := range slices.Backward(m.order) {
		current := m.operations[operationID]
		if current.snapshot.Status == StatusRunning {
			continue
		}

		finished++
		if finished > m.keep {
			delete(m.operations, operationID)
			m.order = slices.Delete(m.order, index, index+1)
		}
	}
}

func redact(text, secret string) string {
	if secret == "" {
		return text
	}

	return strings.ReplaceAll(text, secret, redacted)
}

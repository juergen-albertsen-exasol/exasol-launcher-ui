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

	"github.com/exasol/exasol-local-ui/internal/launcher"
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
)

// command returns the launcher arguments that run kind, without the
// deployment directory, and whether the command prints a lifecycle result.
func (k Kind) command() ([]string, bool) {
	const approve = "--auto-approve"

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
	default:
		return []string{string(k)}, false
	}
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
	Status     Status
	StartedAt  time.Time
	FinishedAt *time.Time
	Result     *launcher.LifecycleResult
	Error      string
	LineCount  int
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
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, busy := m.running[deployment.Name]; busy {
		return Snapshot{}, ErrBusy
	}

	current := &operation{
		snapshot: Snapshot{
			ID:         rand.Text(),
			Deployment: deployment.Name,
			Kind:       kind,
			Status:     StatusRunning,
			StartedAt:  m.now(),
		},
		secret: secret,
	}
	m.operations[current.snapshot.ID] = current
	m.order = append(m.order, current.snapshot.ID)
	m.running[deployment.Name] = current.snapshot.ID

	args, reportsResult := kind.command()
	go m.run(current, reportsResult, append(args, "--deployment-dir", deployment.Path)...)

	return current.snapshot, nil
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

// Package launchertest provides a fake launcher executable for tests.
package launchertest

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const script = `#!/bin/sh
dir=$(dirname "$0")
echo "$*" >> "$dir/calls"
key=$(printf '%s' "$*" | tr -c 'A-Za-z0-9' '_')
[ -f "$dir/$key.stderr" ] && cat "$dir/$key.stderr" >&2
while [ -f "$dir/$key.hold" ]; do sleep 0.02; done
[ -f "$dir/$key.stdout" ] && cat "$dir/$key.stdout"
[ -f "$dir/$key.exit" ] && exit "$(cat "$dir/$key.exit")"
[ -f "$dir/$key.stdout" ] || { echo "unexpected command: $*" >&2; exit 64; }
`

const (
	Version = "9.9.9-test"

	executableMode os.FileMode = 0o700
	dataMode       os.FileMode = 0o600
)

var nonAlphanumeric = regexp.MustCompile(`[^A-Za-z0-9]`)

// Fake is a launcher executable that answers commands with canned output.
type Fake struct {
	t          *testing.T
	dir        string
	Executable string
}

// NewFake creates a fake launcher that already answers `version --json`.
func NewFake(t *testing.T) *Fake {
	t.Helper()

	dir := t.TempDir()
	fake := &Fake{t: t, dir: dir, Executable: filepath.Join(dir, "exasol")}

	writeFile(t, fake.Executable, script, executableMode)
	fake.Respond(`{"version": "`+Version+`"}`, "version", "--json")

	return fake
}

// Respond makes the command given by args print stdout and succeed.
func (f *Fake) Respond(stdout string, args ...string) {
	f.t.Helper()

	f.remove(args, ".exit", ".stderr")
	writeFile(f.t, f.path(args, ".stdout"), stdout, dataMode)
}

// RespondWithProgress makes the command given by args print progress to
// stderr, then stdout, and succeed.
func (f *Fake) RespondWithProgress(progress, stdout string, args ...string) {
	f.t.Helper()

	f.remove(args, ".exit")
	writeFile(f.t, f.path(args, ".stderr"), progress, dataMode)
	writeFile(f.t, f.path(args, ".stdout"), stdout, dataMode)
}

// Hold makes the command given by args wait after its progress output until
// Release is called.
func (f *Fake) Hold(args ...string) {
	f.t.Helper()

	writeFile(f.t, f.path(args, ".hold"), "", dataMode)
}

// Release lets a held command finish.
func (f *Fake) Release(args ...string) {
	f.t.Helper()

	f.remove(args, ".hold")
}

// Fail makes the command given by args print stderr and exit with exitCode.
func (f *Fake) Fail(stderr string, exitCode int, args ...string) {
	f.t.Helper()

	f.remove(args, ".stdout")
	writeFile(f.t, f.path(args, ".stderr"), stderr, dataMode)
	writeFile(f.t, f.path(args, ".exit"), strconv.Itoa(exitCode), dataMode)
}

// Calls returns the argument lists the fake was run with, joined by spaces.
func (f *Fake) Calls() []string {
	f.t.Helper()

	content, err := os.ReadFile(filepath.Join(f.dir, "calls"))
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		f.t.Fatal(err)
	}

	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

func (f *Fake) path(args []string, suffix string) string {
	key := nonAlphanumeric.ReplaceAllString(strings.Join(args, " "), "_")

	return filepath.Join(f.dir, key+suffix)
}

func (f *Fake) remove(args []string, suffixes ...string) {
	for _, suffix := range suffixes {
		_ = os.Remove(f.path(args, suffix))
	}
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

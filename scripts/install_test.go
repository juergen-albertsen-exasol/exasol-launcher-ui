package scripts

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	executableMode os.FileMode = 0o755
	latestVersion              = "0.2.0"
	olderVersion               = "0.1.0"
)

var platforms = []string{"macos_arm64", "macos_x86_64", "linux_arm64", "linux_x86_64"}

type release struct {
	files map[string][]byte
}

func newRelease(t *testing.T, version string) release {
	t.Helper()

	files := map[string][]byte{}

	var checksums strings.Builder

	for _, platform := range platforms {
		name := "exasol-launcher-ui_" + platform + ".tar.gz"
		archive := tarGz(t, "exasol-launcher-ui",
			"#!/bin/sh\necho 'exasol-launcher-ui "+version+"'\n")
		sum := sha256.Sum256(archive)

		files[name] = archive
		_, _ = fmt.Fprintf(&checksums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}

	files["checksums.txt"] = []byte(checksums.String())

	return release{files: files}
}

func tarGz(t *testing.T, name, content string) []byte {
	t.Helper()

	var buffer bytes.Buffer

	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)

	require.NoError(t, archive.WriteHeader(&tar.Header{
		Name: name, Mode: int64(executableMode), Size: int64(len(content)),
	}))
	_, err := archive.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	require.NoError(t, compressed.Close())

	return buffer.Bytes()
}

// serveReleases serves releases like GitHub's download URLs, with the
// latest release under latest/download.
func serveReleases(t *testing.T, latest string, releases map[string]release) string {
	t.Helper()

	server := httptest.NewServer(
		http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
			version, file, valid := strings.Cut(
				strings.TrimPrefix(req.URL.Path, "/download/v"),
				"/",
			)
			if latestFile, isLatest := strings.CutPrefix(
				req.URL.Path,
				"/latest/download/",
			); isLatest {
				version, file, valid = latest, latestFile, true
			}

			content, found := releases[version].files[file]
			if !valid || !found {
				http.NotFound(writer, req)

				return
			}

			_, _ = writer.Write(content)
		}),
	)
	t.Cleanup(server.Close)

	return server.URL
}

type host struct {
	t       *testing.T
	home    string
	binDir  string
	env     map[string]string
	extraOn []string
}

// newHost simulates a macOS arm64 machine without the launcher, whose
// PATH holds only the system tools and a fake uname.
func newHost(t *testing.T, downloadURL string) *host {
	t.Helper()

	simulated := &host{
		t:      t,
		home:   t.TempDir(),
		binDir: t.TempDir(),
		env:    map[string]string{"EXASOL_LAUNCHER_UI_DOWNLOAD_URL": downloadURL},
	}
	simulated.setPlatform("Darwin", "arm64")

	return simulated
}

func (h *host) setPlatform(system, machine string) {
	h.t.Helper()

	script := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in -s) echo %s ;; -m) echo %s ;; esac\n",
		system, machine)
	require.NoError(h.t, os.WriteFile(filepath.Join(h.binDir, "uname"), []byte(script),
		executableMode))
}

func (h *host) installLauncher() {
	h.t.Helper()

	require.NoError(h.t, os.WriteFile(filepath.Join(h.binDir, "exasol"),
		[]byte("#!/bin/sh\n"), executableMode))
}

func (h *host) defaultPath() string {
	return filepath.Join(h.home, ".local", "bin", "exasol-launcher-ui")
}

// install pipes the installer into sh, like the documented install command.
func (h *host) install(env ...string) (string, error) {
	h.t.Helper()

	installer, err := os.ReadFile("install.sh")
	require.NoError(h.t, err)

	path := strings.Join(append(append([]string{h.binDir}, h.extraOn...),
		"/usr/bin", "/bin", "/usr/sbin", "/sbin"), ":")
	cmd := exec.CommandContext(h.t.Context(), "/bin/sh")
	cmd.Stdin = bytes.NewReader(installer)
	cmd.Env = []string{"HOME=" + h.home, "PATH=" + path, "TMPDIR=" + h.t.TempDir()}

	for key, value := range h.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	cmd.Env = append(cmd.Env, env...)
	output, err := cmd.CombinedOutput()

	return string(output), err
}

func versionOf(t *testing.T, executable string) string {
	t.Helper()

	output, err := exec.CommandContext(t.Context(), executable, "--version").Output()
	require.NoError(t, err)

	return strings.TrimSpace(string(output))
}

func twoReleases(t *testing.T) string {
	t.Helper()

	return serveReleases(t, latestVersion, map[string]release{
		olderVersion:  newRelease(t, olderVersion),
		latestVersion: newRelease(t, latestVersion),
	})
}

func TestInstallsTheLatestReleaseIntoLocalBin(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))

	output, err := machine.install()

	require.NoError(t, err, output)

	info, err := os.Stat(machine.defaultPath())
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100, "not executable")
	assert.Equal(t, "exasol-launcher-ui "+latestVersion, versionOf(t, machine.defaultPath()))
	assert.Contains(t, output, "exasol-launcher-ui "+latestVersion)
	assert.Contains(t, output, "exasol-launcher-ui --open")
}

func TestInstallsOnEverySupportedPlatform(t *testing.T) {
	t.Parallel()

	for _, platform := range []struct{ system, machine string }{
		{"Darwin", "arm64"},
		{"Darwin", "x86_64"},
		{"Linux", "aarch64"},
		{"Linux", "x86_64"},
		{"Linux", "amd64"},
	} {
		t.Run(platform.system+"_"+platform.machine, func(t *testing.T) {
			t.Parallel()

			machine := newHost(t, serveReleases(t, latestVersion, map[string]release{
				latestVersion: newRelease(t, latestVersion),
			}))
			machine.setPlatform(platform.system, platform.machine)

			output, err := machine.install()

			require.NoError(t, err, output)
			assert.FileExists(t, machine.defaultPath())
		})
	}
}

func TestInstallingAgainUpgrades(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))
	output, err := machine.install("EXASOL_LAUNCHER_UI_VERSION=" + olderVersion)
	require.NoError(t, err, output)
	require.Equal(t, "exasol-launcher-ui "+olderVersion, versionOf(t, machine.defaultPath()))

	output, err = machine.install()

	require.NoError(t, err, output)
	assert.Equal(t, "exasol-launcher-ui "+latestVersion, versionOf(t, machine.defaultPath()))
}

func TestPinsAVersionWithOrWithoutTheLeadingV(t *testing.T) {
	t.Parallel()

	for _, pinned := range []string{olderVersion, "v" + olderVersion} {
		t.Run(pinned, func(t *testing.T) {
			t.Parallel()

			machine := newHost(t, twoReleases(t))

			output, err := machine.install("EXASOL_LAUNCHER_UI_VERSION=" + pinned)

			require.NoError(t, err, output)
			assert.Equal(t, "exasol-launcher-ui "+olderVersion, versionOf(t, machine.defaultPath()))
		})
	}
}

func TestInstallsIntoTheChosenDirectory(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))
	dir := filepath.Join(t.TempDir(), "opt", "tools", "bin")

	output, err := machine.install("EXASOL_LAUNCHER_UI_INSTALL_DIR=" + dir)

	require.NoError(t, err, output)
	assert.FileExists(t, filepath.Join(dir, "exasol-launcher-ui"))
	assert.NoFileExists(t, machine.defaultPath())
}

func TestShowsThePathLineWhenTheDirectoryIsNotOnPath(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))

	output, err := machine.install()

	require.NoError(t, err, output)
	assert.Contains(t, output,
		`export PATH="`+filepath.Join(machine.home, ".local", "bin")+`:$PATH"`)
}

func TestOmitsThePathLineWhenTheDirectoryIsOnPath(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))
	machine.extraOn = []string{filepath.Dir(machine.defaultPath())}

	output, err := machine.install()

	require.NoError(t, err, output)
	assert.NotContains(t, output, "export PATH=")
}

func TestShowsTheLauncherInstallCommandWhenTheLauncherIsMissing(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))

	output, err := machine.install()

	require.NoError(t, err, output)
	assert.Contains(t, output, "needs the Exasol Personal launcher")
	assert.Contains(t, output,
		"curl https://downloads.exasol.com/exasol-personal/installer.sh | sh")
}

func TestOmitsTheLauncherHintWhenTheLauncherIsInstalled(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))
	machine.installLauncher()

	output, err := machine.install()

	require.NoError(t, err, output)
	assert.NotContains(t, output, "Exasol Personal launcher")
}

func TestRefusesAnUnsupportedArchitecture(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))
	machine.setPlatform("Linux", "i686")

	output, err := machine.install()

	require.Error(t, err)
	assert.Contains(t, output, "i686")
	assert.NoFileExists(t, machine.defaultPath())
}

func TestRefusesAnUnsupportedOperatingSystem(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))
	machine.setPlatform("FreeBSD", "x86_64")

	output, err := machine.install()

	require.Error(t, err)
	assert.Contains(t, output, "FreeBSD")
}

func TestRefusesAnUnknownVersionAndKeepsTheInstallation(t *testing.T) {
	t.Parallel()

	machine := newHost(t, twoReleases(t))
	output, err := machine.install("EXASOL_LAUNCHER_UI_VERSION=" + olderVersion)
	require.NoError(t, err, output)

	output, err = machine.install("EXASOL_LAUNCHER_UI_VERSION=9.9.9")

	require.Error(t, err)
	assert.Contains(t, output, "9.9.9")
	assert.Equal(t, "exasol-launcher-ui "+olderVersion, versionOf(t, machine.defaultPath()))
}

func TestRefusesAChecksumMismatchAndInstallsNothing(t *testing.T) {
	t.Parallel()

	tampered := newRelease(t, latestVersion)
	tampered.files["exasol-launcher-ui_macos_arm64.tar.gz"] = tarGz(t, "exasol-launcher-ui",
		"#!/bin/sh\necho tampered\n")
	url := serveReleases(t, latestVersion, map[string]release{latestVersion: tampered})
	machine := newHost(t, url)

	output, err := machine.install()

	require.Error(t, err)
	assert.Contains(t, output, "checksum mismatch")
	assert.NoDirExists(t, filepath.Dir(machine.defaultPath()))
}

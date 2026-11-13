# release-distribution Specification

## Purpose

Publish prebuilt Exasol Launcher UI binaries for every version tag, and
install or upgrade them on macOS and Linux with one command.

## Requirements
### Requirement: Version tags publish a release

Pushing a tag of the form `v<major>.<minor>.<patch>` SHALL publish a GitHub
release of that version in the repository. A tag with a suffix after the patch
number, such as `v1.2.0-rc1`, SHALL publish a pre-release, which the latest
release download URLs do not resolve to.

#### Scenario: A version tag becomes the latest release

- **WHEN** a maintainer pushes the tag `v0.1.0`
- **THEN** the repository has a release `v0.1.0` marked as the latest release

#### Scenario: A suffixed tag becomes a pre-release

- **WHEN** a maintainer pushes the tag `v0.2.0-rc1` after `v0.1.0` was released
- **THEN** the release `v0.2.0-rc1` is marked as a pre-release, and
  `releases/latest` still refers to `v0.1.0`

### Requirement: Releases contain ready-to-run binaries

Every release SHALL contain one archive per supported platform, each holding
an `exasol-launcher-ui` executable with the web application embedded, named
`exasol-launcher-ui_<os>_<arch>` with `<os>` one of `macos`, `linux`,
`windows` and `<arch>` one of `arm64`, `x86_64`. Archives SHALL be `.tar.gz`
for macOS and Linux and `.zip` for Windows. The platforms SHALL be macOS
arm64 and x86_64, Linux arm64 and x86_64, and Windows x86_64. Every release
SHALL also contain `checksums.txt` with the SHA-256 checksum of each archive,
and `install.sh`.

#### Scenario: A release lists its assets

- **WHEN** the user opens the assets of release `v0.1.0`
- **THEN** they include `exasol-launcher-ui_macos_arm64.tar.gz`,
  `exasol-launcher-ui_macos_x86_64.tar.gz`,
  `exasol-launcher-ui_linux_arm64.tar.gz`,
  `exasol-launcher-ui_linux_x86_64.tar.gz`,
  `exasol-launcher-ui_windows_x86_64.zip`, `checksums.txt`, and `install.sh`

#### Scenario: A released binary reports its version and serves the application

- **WHEN** the user extracts the archive for their platform from release
  `v0.1.0` and runs `exasol-launcher-ui --version`
- **THEN** it prints `exasol-launcher-ui 0.1.0`, and running it without flags
  serves the web application at the printed access URL

### Requirement: One command installs the latest release

The install command SHALL install the latest release's `exasol-launcher-ui`
for the host's platform into `~/.local/bin` on macOS and Linux, creating the
directory if needed, and SHALL print the installed version and the command
that starts the UI. The install command is
`curl -fsSL https://github.com/juergen-albertsen-exasol/exasol-launcher-ui/releases/latest/download/install.sh | sh`.

#### Scenario: Installing on a supported host

- **WHEN** the user runs the install command on macOS arm64 without an earlier
  installation
- **THEN** `~/.local/bin/exasol-launcher-ui` exists and is executable,
  `~/.local/bin/exasol-launcher-ui --version` prints the latest release's
  version, and the installer's output names that version and
  `exasol-launcher-ui --open`

#### Scenario: Installing again upgrades

- **WHEN** `~/.local/bin/exasol-launcher-ui` reports `0.1.0` and the user runs
  the install command after `v0.2.0` was released
- **THEN** `~/.local/bin/exasol-launcher-ui --version` prints
  `exasol-launcher-ui 0.2.0`

### Requirement: The installer can install a chosen version and directory

The installer SHALL install the release named by the
`EXASOL_LAUNCHER_UI_VERSION` environment variable (with or without the leading
`v`) instead of the latest release, and SHALL install into the directory named
by `EXASOL_LAUNCHER_UI_INSTALL_DIR` instead of `~/.local/bin`.

#### Scenario: Pinning a version

- **WHEN** the user pipes the installer to
  `EXASOL_LAUNCHER_UI_VERSION=0.1.0 sh` after `v0.2.0` was released
- **THEN** the installed executable reports `exasol-launcher-ui 0.1.0`

#### Scenario: Choosing the directory

- **WHEN** the user pipes the installer to
  `EXASOL_LAUNCHER_UI_INSTALL_DIR=/opt/tools/bin sh`
- **THEN** the executable is installed as `/opt/tools/bin/exasol-launcher-ui`

### Requirement: The installer guides the next steps

After installing, the installer SHALL tell the user how to add the install
directory to `PATH` when it is not on `PATH`, and SHALL tell the user how to
install the Exasol Personal launcher when no `exasol` executable is on `PATH`.

#### Scenario: The install directory is not on PATH

- **WHEN** the installer finishes and the install directory is not on `PATH`
- **THEN** its output shows the `export PATH=...` line that adds the directory

#### Scenario: The launcher is missing

- **WHEN** the installer finishes and no `exasol` executable is on `PATH`
- **THEN** its output says that Exasol Launcher UI needs the Exasol Personal
  launcher and shows the launcher's install command

### Requirement: The installer refuses what it cannot install safely

The installer SHALL exit with a non-zero status, leave any existing
installation unchanged, and print the reason when the host's operating system
or architecture has no release archive, when the requested version does not
exist, or when the downloaded archive does not match its checksum in
`checksums.txt`.

#### Scenario: Unsupported platform

- **WHEN** the user runs the installer on a host whose `uname -m` reports
  `i686`
- **THEN** it exits with a non-zero status and names the unsupported
  architecture

#### Scenario: Unknown version

- **WHEN** the user runs the installer with `EXASOL_LAUNCHER_UI_VERSION=9.9.9`
  and no such release exists
- **THEN** it exits with a non-zero status, names the version, and the
  existing installation still reports its earlier version

#### Scenario: Checksum mismatch

- **WHEN** the downloaded archive's SHA-256 checksum differs from the one in
  `checksums.txt`
- **THEN** the installer exits with a non-zero status, reports the mismatch,
  and installs nothing


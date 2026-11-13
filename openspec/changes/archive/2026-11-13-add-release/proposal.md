## Why

Using Exasol Launcher UI today means cloning the repository, installing Go, Node.js, and Task, and obtaining a GitHub Packages token just to build it. Now that the repository is public, users should get it the way they get the launcher: one command that installs a ready binary.

## What Changes

- Publish a GitHub release for every pushed version tag (`v1.2.3`, or `v1.2.3-rc1` as a pre-release), with prebuilt archives for macOS (arm64, x86_64), Linux (arm64, x86_64), and Windows (x86_64), a checksums file, and the installer script.
- Add an installer for macOS and Linux that users run with one command:
  `curl -fsSL https://github.com/juergen-albertsen-exasol/exasol-launcher-ui/releases/latest/download/install.sh | sh`.
  It picks the archive for the host, verifies its checksum, installs `exasol-launcher-ui` into `~/.local/bin`, and tells the user how to continue. Running it again upgrades; a version can be pinned.
- Add `exasol-launcher-ui --version`, which prints the released version.
- Document installation and upgrades in the README, and the release process for maintainers.

## Capabilities

### New Capabilities

- `release-distribution`: published releases, their assets, and the one-command installer.

### Modified Capabilities

- `ui-server`: the command reports its version with `--version`.

## Impact

- New: `.goreleaser.yaml`, a `release` GitHub Actions workflow triggered by version tags, `scripts/install.sh` with tests.
- `cmd/exasol-launcher-ui`: a version variable set at build time and the `--version` flag.
- Release builds need the `NPM_TOKEN` secret (already used by CI) to build the web application.
- Binaries are not code-signed. macOS does not quarantine files fetched with `curl`, so the installed binary runs; a signed release is left for later.
- No change to the HTTP API or the browser application.

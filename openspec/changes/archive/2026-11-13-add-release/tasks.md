Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Version

- [x] 1.1 Add the build-time `version` variable (default `dev`) and the `--version` flag that prints `exasol-launcher-ui <version>` before the launcher check, with tests for both `ui-server` scenarios

## 2. Release build

- [x] 2.1 Add `.goreleaser.yaml` (web build hook, five targets, archive names, checksums, `install.sh` as extra file, pre-releases, changelog filters) and a CI job that runs `goreleaser check` and a snapshot release, verified by the job's archives and a manual `--version` run of one of them

## 3. Installer

- [x] 3.1 Add `scripts/install.sh` with platform detection, latest and pinned versions, the install directory override, checksum verification, atomic replacement, and the next-step hints; add `scripts/install_test.go` covering every `release-distribution` installer scenario against a fake release server, and `shellcheck` to `task lint`

## 4. Publishing

- [x] 4.1 Add `.github/workflows/release.yml` that tests and releases on `v*` tags, and the post-release job that installs the tagged version on Linux x86_64, Linux arm64, and macOS and checks `--version`; verify with a `v0.1.0-rc1` pre-release after merging
- [x] 4.2 Document installation, upgrades, pinning, the Windows download, and the macOS quarantine note in the README, and the release process (tagging, pre-releases, withdrawing a release) in `docs/architecture.md`

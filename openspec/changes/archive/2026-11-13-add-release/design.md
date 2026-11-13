## Context

Exasol Launcher UI is one Go binary that embeds the Angular build from
`internal/webapp/dist`. Building it needs Go, Node.js, Task, and a GitHub
Packages token for `@exasol/ui-common`, so today only contributors can run it.
The repository `juergen-albertsen-exasol/exasol-launcher-ui` is now public.

The launcher it accompanies is installed with
`curl https://downloads.exasol.com/exasol-personal/installer.sh | sh`, which
puts `exasol` into `~/.local/bin` (`../exasol-personal/scripts/personal-installer.sh`),
and released with GoReleaser (`../exasol-personal/.goreleaser.yaml`). Users
should find the same shape here.

## Goals / Non-Goals

**Goals:**

- A release per version tag, built and published by CI without manual steps.
- One command that installs or upgrades the UI on macOS and Linux.
- A release setup that can be checked on every pull request, before the first
  tag.

**Non-Goals:**

- A Windows installer: Windows users download the `.zip` from the release
  page. A PowerShell installer can follow.
- Code signing, notarization, or provenance attestations.
- Package managers (Homebrew, apt), `go install`, or self-update from within
  the UI.
- A version display in the browser.

## Decisions

### GoReleaser builds and publishes releases

A `.goreleaser.yaml` builds the five targets with `CGO_ENABLED=0` (the driver
and Parquet library are pure Go), sets `main.version` through `-ldflags`, packs
archives named `exasol-launcher-ui_<os>_<arch>` (`darwin` shown as `macos`,
`amd64` as `x86_64`), writes `checksums.txt`, uploads `scripts/install.sh` as
`install.sh` with `release.extra_files`, marks suffixed versions as
pre-releases (`prerelease: auto`), and builds the changelog from Conventional
Commit subjects without `docs`, `test`, and `chore`.

A `before` hook runs `npm --prefix web ci` and `npm --prefix web run build`
once, so every target embeds the same web build.

Alternative: a hand-written matrix workflow with `go build` and
`gh release create`. It duplicates what GoReleaser does and diverges from the
launcher's setup.

### Pushing a tag triggers the release

`.github/workflows/release.yml` runs on `push` of tags matching `v*`, with
`contents: write`, the `NPM_TOKEN` secret for GitHub Packages, and
`goreleaser/goreleaser-action` pinned by commit like the other actions. It
runs `task test` before releasing.

Alternative: the launcher's `workflow_dispatch` on an existing tag. That fits
its docs publication and signing environment; this project has neither, so a
tag push is the simpler trigger.

### Pull requests check the release configuration

CI gains a job that runs `goreleaser check` and
`goreleaser release --snapshot --clean --skip=publish`, so a broken
configuration fails before anyone tags. GoReleaser is not added to
`tools/devtools`, whose module graph it would dwarf; maintainers who want to
run it locally install it themselves.

### The installer downloads from GitHub's release URLs

`scripts/install.sh` is POSIX `sh` and needs only `curl`, `tar`, `uname`, and
`sha256sum` or `shasum -a 256`. It:

1. maps `uname -s` and `uname -m` to `macos`/`linux` and `arm64`/`x86_64`, and
   stops for anything else;
2. downloads the archive and `checksums.txt` from
   `https://github.com/juergen-albertsen-exasol/exasol-launcher-ui/releases/latest/download/`,
   or from `.../releases/download/v<version>/` when
   `EXASOL_LAUNCHER_UI_VERSION` is set, into a temporary directory removed on
   exit;
3. verifies the archive's checksum, extracts it, and moves the executable into
   the install directory, so an existing installation is replaced only by a
   verified binary;
4. runs the installed binary with `--version` and prints the result, the
   `PATH` hint, the launcher hint, and `exasol-launcher-ui --open`.

The same file is served from `releases/latest/download/install.sh`, so the
install command always runs the installer of the latest release, which matches
the archive names it expects.

Alternative: serving the installer from `raw.githubusercontent.com/.../main`.
It would run unreleased installer changes against released archives.

`EXASOL_LAUNCHER_UI_DOWNLOAD_URL` replaces the GitHub base URL. Tests use it to
point the installer at a local server; it also serves mirrors.

### The version is a build-time variable

`cmd/exasol-launcher-ui` declares `var version = "dev"`, which GoReleaser
overrides. `--version` is a flag on the existing flag set and returns before
the launcher check, so it works without the launcher installed.

### The installer is tested with Go

`scripts/install_test.go` serves fake releases with `httptest`: archives whose
`exasol-launcher-ui` is a shell script printing a version, and a matching
`checksums.txt`. Each test runs `sh scripts/install.sh` with a temporary
`HOME`, a controlled `PATH`, and a fake `uname` where a platform is simulated,
and checks the installed file, its output, and the exit status. This keeps the
tests inside `task test`. `task lint` adds `shellcheck scripts/install.sh`.

After a release, a workflow job installs it with the real install command,
pinned to the tag with `EXASOL_LAUNCHER_UI_VERSION` so pre-releases are
covered too, on `ubuntu-latest`, `ubuntu-24.04-arm`, and `macos-latest`, and
checks `exasol-launcher-ui --version` against the tag.

## Risks / Trade-offs

- [The repository may move to the `exasol` organization] → GitHub redirects
  renamed and transferred repositories, including release downloads; update
  the URL in the installer and README with the move.
- [Checksums come from the same release as the archives] → They catch
  corrupted downloads, not a compromised release. Attestations or signing can
  follow.
- [Unsigned macOS binaries] → Files downloaded with `curl` carry no quarantine
  attribute, so Gatekeeper lets the installed binary run. Archives downloaded
  in a browser need `xattr -d com.apple.quarantine`; the README says so.
- [The release workflow needs `NPM_TOKEN`] → The secret already exists for
  CI; the workflow fails early with npm's error if it is missing.

## Migration Plan

After the change is merged, tag `v0.1.0-rc1`, check the pre-release and the
smoke job, then tag `v0.1.0`. A broken release is withdrawn by deleting the
release and its tag; `releases/latest` then refers to the previous release.

## Open Questions

- Should a PowerShell installer for Windows follow, given that the launcher
  supports local deployments there?

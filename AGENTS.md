# AI Agent Guidelines

Exasol Local UI is a prototype web UI for local Exasol Personal deployments. It
is optional: users run it beside the launcher, and the launcher itself does not
depend on it. Its pages are meant to graduate later, either into the launcher
(as an `exasol ui` command) or into the cloud Administration UI, so design
choices should keep both paths open.

## Related Repositories

This repository builds on two sibling checkouts. Read them as reference, and
do not change them from a session in this repository.

**[Exasol Personal](../exasol-personal)** (`../exasol-personal`) holds the launcher (`exasol` CLI)
that creates and operates local deployments. This UI drives the launcher
through its JSON CLI output and reads a deployment's credentials from its
deployment directory. The launcher's contracts are specified in
`openspec/specs/` there; the most relevant are `cli-output-contract`,
`lifecycle-json-output`, `deployment-info-reporting`,
`deployment-directory-resolution`, `sidecar-configuration`,
`sidecar-lifecycle`, and `credential-confidentiality`. User-facing behavior is
documented in `user-docs/content/`.

**[Admin UI](../admin-ui)** (`../admin-ui`) is the cloud Administration UI, an Angular frontend for
the Exasol Administration API. This UI keeps its look and feel and follows the
conventions in its `CLAUDE.md` (stack, standalone components, state
management, SCSS with BEM, testing), so pages can move into it later. Its
`openapi/openapi.json` defines the Administration API; shape new endpoints here
like its resources.

## Architecture

The UI is a host-side companion tool rather than a launcher sidecar, because
sidecar management, lifecycle operations, and the deployment directory are only
reachable from the host. A Go server serves the Angular bundle and a narrow
HTTP API. It uses the launcher's JSON CLI for deployment, lifecycle, and
sidecar operations, and connects to the database with the credentials stored in
the deployment directory. Health views read the database's `EXA_STATISTICS`
schema, which local deployments populate at every granularity.

## Security Invariants

The server runs SQL with the deployment's administrative credentials, so it
binds to loopback only, requires a per-session token that the startup URL
exchanges for a cookie, and rejects requests whose `Host` or `Origin` does not
match the server. Database passwords never reach the browser or the logs.

## Repository Workflow

Draft every feature's screens in Claude Design before specifying it: on the
[Exasol Local](https://claude.ai/artifact/UQ4cH4hUXciqeB2kzWMRQa) canvas, built
with the [Exasol Admin UI](https://claude.ai/artifact/WJ3Tm2ihMri7fiUcJtKRKj)
design system (light theme first). Iterate there until the design is agreed, then
implement what it shows.

Use OpenSpec for every non-trivial change: propose with `/opsx:propose`,
implement with `/opsx:apply`, and archive with `/opsx:archive` before
committing the change's OpenSpec artifacts. Write requirements as positive,
observable behavior at a stable boundary (the CLI, the HTTP API, or the
browser), give each behavior one canonical specification, and map each task to
one independently verifiable commit.

Use [Conventional Commits](https://www.conventionalcommits.org/) with a
capitalized subject and no body, for example `feat: Add sidecar list page`.
Amend the related commit for cleanup instead of adding fixup commits.

Keep code comments rare: add them only for non-obvious invariants,
constraints, or workarounds, never to restate code or refer to tasks or
OpenSpec artifacts. Keep documentation concise, link instead of duplicating,
and avoid em dashes.

## Layout

- `cmd/exasol-local-ui/`: the command, its flags, and startup.
- `internal/server/`: HTTP server, session token and origin checks, the
  `/api/v1` handlers, and serving the web application.
- `internal/launcher/`: the launcher's JSON CLI client; `launchertest/` holds
  the fake launcher executable tests use.
- `internal/database/`: reading `secrets.json`, pinging the database, password
  redaction.
- `internal/webapp/`: embeds the Angular build from `web/`.
- `web/`: the Angular application. The `go.mod` there only keeps Go tooling
  out of `node_modules`.

`task build`, `task test`, and `task lint` must pass for every commit; the
[README](README.md) covers setup (including the GitHub Packages token) and the
development loop.

## Status

Done: the server, launcher and database integration, the deployment tiles and
overview (`add-app-scaffold`, `refine-deployment-pages`), and long-running
operations with Start, Stop, and the activity indicator (`add-operations`).
Long launcher commands go through `internal/operations`; later changes reuse
it.

The canvas already draws every remaining screen. Implement these as separate
OpenSpec changes, one per session, in this order (the canvas artboard names
in parentheses):

1. `add-deployment-settings`: the Settings page (`Configuration`,
   `DestroyDialog`). `config get --json`, `config set`, `config reset`, apply
   with `deploy`, and a danger zone with `destroy`, `destroy --remove`, and
   `remove`, each confirmed by typing the deployment name.
2. `add-sidecar-management`: Sidecars and MCP server pages (`Sidecars`,
   `MCP`). `sidecar list/status/enable/disable`, `mcp status/enable/disable`,
   and `mcp instructions` with copyable client commands.
3. `add-script-languages`: the Script languages page (`ScriptLanguages`).
   `slc list/install/update/remove` and `slc custom install/update`, with the
   restart choice (`--no-restart`).
4. `add-diagnostics`: the Diagnostics page (`Diagnostics`). `diag local`,
   `diag info` (show and download), guarded `diag unlock`, and copyable
   `shell host` and `shell container` commands.
5. `add-deployment-creation`: the New deployment page (`NewDeployment`).
   `install` and `init` for every preset; test only local deployments, since
   cloud ones take long and cost money.
6. `add-launcher-tools`: the start page toolbar with Presets, Runtime cache,
   and Launcher pages (`Main`, `Presets`, `Cache`, `Launcher`).
   `presets list/export`, `cache list/clean/unlock`, `diag cache`, and
   `version --latest`.

A SQL client and health views follow later and replace `exasol connect` in
the UI.

Launcher findings that affect these changes:

- `config set --help` lists no preset options. The command accepts them as
  `--<option>` flags validated against the preset; `config get --json` reports
  them under `infrastructure.options` and `installation.options`.
- `config set` and `config reset` only work on initialized cloud deployments
  and stopped local deployments; the launcher refuses them otherwise.
- `init`, `install`, `deploy`, and `destroy` have no `--json` output; judge
  them by exit status and stream their stderr.
- Without a terminal the launcher writes JSON log records to stderr;
  `launcher.Stream` turns them into readable lines.
- Decide actions on the deployment listing's `status`, which checks the
  runtime; `info` can report a stale stored `deploymentState`.

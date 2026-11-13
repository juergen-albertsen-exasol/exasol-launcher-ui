# AI Agent Guidelines

Exasol Launcher UI is a prototype web UI for local Exasol Personal deployments.
It is optional: users run it beside the `exasol` launcher, and the launcher
does not depend on it. Its pages are meant to graduate later, either into the
launcher (as an `exasol ui` command) or into the cloud Administration UI, so
design choices should keep both paths open.

## Start Here

Read these before changing code:

- [docs/architecture.md](docs/architecture.md): components, request flow,
  security model, server packages, HTTP API conventions, launcher and database
  integration, the web application's layout, testing, and the steps for adding
  a feature.
- [README.md](README.md): what the UI does, usage, setup (including the GitHub
  Packages token), and the development loop.
- `openspec/specs/`: the canonical behavior of every capability.

`task build`, `task test`, and `task lint` must pass for every commit.

## Keep the Docs Current

Every change that alters what these documents describe updates them in the
same commit: [docs/architecture.md](docs/architecture.md) for structure,
conventions, and findings about the launcher or the database;
[README.md](README.md) for user-visible features, flags, and setup; and the
Status section below when an OpenSpec change is archived. Agents rely on these
files to work without rediscovering the repository, so stale docs are a bug.

## Related Repositories

This repository builds on two sibling checkouts. Read them as reference, and
do not change them from a session in this repository.

**[Exasol Personal](../exasol-personal)** (`../exasol-personal`) holds the
launcher (`exasol` CLI) that creates and operates local deployments. This UI
drives the launcher through its JSON CLI output and reads a deployment's
credentials from its deployment directory. The launcher's contracts are
specified in `openspec/specs/` there; the most relevant are
`cli-output-contract`, `lifecycle-json-output`, `deployment-info-reporting`,
`deployment-directory-resolution`, `sidecar-configuration`,
`sidecar-lifecycle`, and `credential-confidentiality`. User-facing behavior is
documented in `user-docs/content/`.

**[Admin UI](../admin-ui)** (`../admin-ui`) is the cloud Administration UI, an
Angular frontend for the Exasol Administration API. This UI keeps its look and
feel and follows the conventions in its `CLAUDE.md` (stack, standalone
components, state management, SCSS with BEM, testing), so pages can move into
it later. Its `openapi/openapi.json` defines the Administration API; shape new
endpoints here like its resources.

## Security Invariants

The server runs SQL with the deployment's administrative credentials, so it
binds to loopback only, requires a per-session token that the startup URL
exchanges for a cookie, and rejects requests whose `Host` or `Origin` does not
match the server. Database passwords never reach the browser or the logs. See
[Security model](docs/architecture.md#security-model).

## Repository Workflow

Draft every feature's screens in Claude Design before specifying it: on the
[design canvas](https://claude.ai/artifact/UQ4cH4hUXciqeB2kzWMRQa), built
with the [Exasol Admin UI](https://claude.ai/artifact/WJ3Tm2ihMri7fiUcJtKRKj)
design system (light theme first). Iterate there until the design is agreed,
then implement what it shows.

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

Check destructive operations on a scratch deployment from
`exasol init local -d <name>`, which creates no VM, never on real ones.

## Status

Done, each as an archived OpenSpec change: the server, launcher and database
integration, and the deployment tiles and overview (`add-app-scaffold`,
`refine-deployment-pages`); long-running operations with Start, Stop, and the
activity indicator (`add-operations`); the Settings page with configuration,
deploy, destroy, and remove (`add-deployment-settings`); the Sidecars and MCP
server pages (`add-sidecar-management`); the Script languages page
(`add-script-languages`); the Diagnostics page (`add-diagnostics`); the SQL
page with its editor, schema browser, results, and history (`add-sql-editor`);
the Import data page for Parquet files (`add-parquet-import`); and
releases with the one-command installer and `--version` (`add-release`).

The canvas already draws every remaining screen. Implement these as separate
OpenSpec changes, one per session, in this order (the canvas artboard names in
parentheses):

1. `add-deployment-creation`: the New deployment page (`NewDeployment`).
   `install` and `init` for every preset; test only local deployments, since
   cloud ones take long and cost money.
2. `add-launcher-tools`: the start page toolbar with Presets, Runtime cache,
   and Launcher pages (`Main`, `Presets`, `Cache`, `Launcher`).
   `presets list/export`, `cache list/clean/unlock`, `diag cache`, and
   `version --latest`.

Health views from `EXA_STATISTICS` follow later.

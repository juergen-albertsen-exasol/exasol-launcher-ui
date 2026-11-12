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

The first change, `add-app-scaffold`, provides the server, the launcher and
database integration, and the app shell with the deployment tiles and the
overview page. The planned order after it is sidecar management, a SQL editor,
and health views, each drafted on the design canvas first.

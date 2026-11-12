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

## Status

The repository contains no application code yet. Start with an OpenSpec
change that scaffolds the server and the Angular app; the planned order after
that is sidecar management, a SQL editor, and health views.

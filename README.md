# Exasol Launcher UI

A web UI for the [Exasol Personal](https://github.com/exasol/exasol-personal)
launcher. Run it next to the `exasol` CLI and manage your deployments, query
your database, and load data from a browser, with the look and feel of the
Exasol Administration UI.

> **Prototype.** Exasol Launcher UI is an optional companion tool; the launcher
> works without it. Its pages are meant to move later into the launcher or the
> Exasol Administration UI.

## What You Can Do

The start page shows a tile per deployment with its folder, status, and
infrastructure preset. Selecting one opens the deployment's pages:

| Page | What it offers |
| --- | --- |
| Overview | State, presets, SQL connection details, whether the database accepts connections, and Start and Stop. |
| SQL | An Exasol SQL editor with completion, a schema browser, a results grid with CSV export, cancellation, and history kept per deployment. |
| Import data | Drop Parquet files, review and edit the table Exasol would create from them, and import them with per-file progress. |
| Sidecars | Enable and disable the deployment's sidecars. |
| MCP server | Setup instructions for connecting AI clients to the deployment's MCP server. |
| Script languages | Install, update, and remove script language containers, including custom ones. |
| Settings | Change the preset options, deploy, destroy, and remove the deployment. |
| Diagnostics | The local runtime's state, the launcher's diagnostic report, shell commands to copy, and unlocking a stuck deployment folder. |

Long launcher commands such as Start, Deploy, or installing a script language
run in the background; an activity indicator follows them on every page.

## How It Works

`exasol-launcher-ui` is a single binary that serves the web application and a
small HTTP API on `127.0.0.1`. It runs the `exasol` launcher for everything
the launcher owns, and connects to the database directly, with the
credentials stored in the deployment directory, for SQL and imports. See
[docs/architecture.md](docs/architecture.md) for the details.

Because it acts with the deployment's administrative credentials, the server:

- listens on loopback only,
- accepts only the browser that opened the access URL it printed at startup,
  whose token changes with every start,
- rejects requests from other hosts and origins, and
- never sends the database password to the browser or writes it to logs.

## Usage

Install the [`exasol` launcher](https://github.com/exasol/exasol-personal) and
put it on `PATH`, then build Exasol Launcher UI (see [Setup](#setup)) and run:

```shell
bin/exasol-launcher-ui --open
```

The command prints an access URL such as `http://127.0.0.1:5180/?token=...`,
opens it, and serves until you stop it with Ctrl+C. After a restart, open the
newly printed URL.

| Flag | Meaning |
| --- | --- |
| `--port <port>` | Serve on this port of `127.0.0.1` instead of 5180. |
| `--open` | Open the access URL in the default browser. |
| `--deployment <name>` | Make the access URL open this deployment's overview instead of the tiles. |
| `--exasol <path>` | Run this launcher executable instead of `exasol` from `PATH`. |
| `--web-dir <dir>` | Serve the web application from this directory instead of the embedded build. |

## Setup

Building requires Go (see `go.mod`), Node.js (see `web/.nvmrc`), and
[Task](https://taskfile.dev).

The web application depends on `@exasol/ui-common` from GitHub Packages, which
needs a GitHub token with the `read:packages` scope. `web/.npmrc` already maps
the `@exasol` scope to GitHub Packages; add the token to your `~/.npmrc`:

```shell
echo "//npm.pkg.github.com/:_authToken=<token>" >> ~/.npmrc
```

If the GitHub CLI is logged in with that scope, `gh auth token` prints a
suitable token. CI reads it from the `NPM_TOKEN` secret.

Then build:

```shell
task build
```

## Development

| Command | Effect |
| --- | --- |
| `task build` | Build the web application and `bin/exasol-launcher-ui`, which embeds it. |
| `task test` | Run the Go and web application unit tests. |
| `task lint` | Run the Go and web application linters. |
| `task dev -- <flags>` | Rebuild the web application on change and serve it from disk. |

`task dev` runs `ng build --watch` next to the server started with `--web-dir`,
so the server's token flow stays in place; reload the browser after a rebuild.

`go test ./...` includes a database test against a running local deployment,
which skips when none is ready; `go test -short ./...` always skips it.
`go test -tags integration ./...` adds the SQL and import tests against a local
deployment.

## Documentation

- [docs/architecture.md](docs/architecture.md): how the server, launcher,
  database, and web application fit together.
- [openspec/specs/](openspec/specs/): the behavior of every capability.
- [AGENTS.md](AGENTS.md): the workflow for contributors and coding agents.
- The [design canvas](https://claude.ai/artifact/UQ4cH4hUXciqeB2kzWMRQa)
  draws every screen.

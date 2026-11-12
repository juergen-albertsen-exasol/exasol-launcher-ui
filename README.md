# Exasol Local UI

A prototype web UI for local [Exasol Personal](https://github.com/exasol/exasol-personal)
deployments, with the look and feel of the Exasol Administration UI. It opens on
a tile per deployment (name, folder, status, infrastructure preset); selecting
one shows its overview: state, presets, SQL connection details, and whether the
database accepts connections. Sidecar management, a SQL editor, and health
views are planned.

See [AGENTS.md](AGENTS.md) for the architecture and workflow,
[openspec/](openspec/) for the specifications, and the
[Exasol Local](https://claude.ai/artifact/UQ4cH4hUXciqeB2kzWMRQa) design canvas
for the screens.

## Usage

Exasol Local UI runs beside the `exasol` launcher, which must be on `PATH`.

```shell
exasol-local-ui --open
```

The command prints an access URL such as `http://127.0.0.1:5180/?token=...` and
serves until you stop it with Ctrl+C. Opening the URL starts a browser session;
the token changes with every start, so open the newly printed URL after a
restart.

| Flag | Meaning |
| --- | --- |
| `--port <port>` | Serve on this port of `127.0.0.1` instead of 5180. |
| `--open` | Open the access URL in the default browser. |
| `--deployment <name>` | Make the access URL open this deployment's overview instead of the tiles. |
| `--exasol <path>` | Run this launcher executable instead of `exasol` from `PATH`. |
| `--web-dir <dir>` | Serve the web application from this directory instead of the embedded build. |

The server listens on loopback only and acts with the deployment's
administrative database credentials, which it reads from the deployment
directory and never sends to the browser.

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

## Development

| Command | Effect |
| --- | --- |
| `task build` | Build the web application and `bin/exasol-local-ui`, which embeds it. |
| `task test` | Run the Go and web application unit tests. |
| `task lint` | Run the Go and web application linters. |
| `task dev -- <flags>` | Rebuild the web application on change and serve it from disk. |

`task dev` runs `ng build --watch` next to the server started with `--web-dir`,
so the server's token flow stays in place; reload the browser after a rebuild.

The database tests include one against a running local deployment, which skips
when the launcher or a ready deployment is unavailable; `go test -short ./...`
skips it always.

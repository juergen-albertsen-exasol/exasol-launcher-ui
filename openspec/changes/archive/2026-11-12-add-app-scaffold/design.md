## Context

The repository is empty apart from its agent guidelines. Local Exasol Personal
deployments are operated by the `exasol` launcher, which offers stable JSON
output for the commands this change needs: `exasol deployments list --json`
(name, path, status, presets) and `exasol info --json` (state and SQL
connection details). The database password lives only in the deployment
directory's `secrets.json` (`dbPassword`), readable by the deployment owner.
The cloud Administration UI is an Angular application on `@exasol/ui-common`
whose look this prototype keeps, and whose conventions it follows so pages can
move there later.

## Goals / Non-Goals

**Goals:**

- One binary that serves the app and its API, safe to run on a developer
  machine even though the API acts with administrative database credentials.
- A launcher and database integration that later pages (sidecars, SQL editor,
  health) reuse unchanged.
- An app shell that matches the Administration UI and its conventions.

**Non-Goals:**

- Sidecar management, SQL execution, and health views; they follow as separate
  changes.
- Changes to the launcher or the Administration UI.
- Remote access, multiple users, or Windows support in the prototype.

## Decisions

**Host-side binary rather than a launcher sidecar.** Sidecar management and
lifecycle operations need the launcher and the deployment directory on the
host; a container would need the engine socket, which is root-equivalent.
Alternative: a sidecar limited to SQL features, rejected because the planned
sidecar page would need a second component.

**Launcher access through its JSON CLI.** The server runs `exasol` as a
subprocess and decodes stdout, ignoring unknown fields; stderr carries notices
and is only used for error messages. Commands select a deployment with
`--deployment-dir <path>` using the path from the listing, so names and
directories cannot diverge. Alternative: importing launcher packages, rejected
because they are internal to its module and would couple the prototype to its
release cycle. The CLI is the launcher's specified contract, and anything the
UI cannot do through it marks a gap in that contract.

**Database access with the Exasol Go driver.** The server reads `dbPassword`
from `secrets.json` in the listed path at connection time and opens a
connection with the host, port, user, and certificate setting from
`exasol info`. Ping uses a fresh connection with a five-second timeout; pooling
waits for the SQL editor. Driver errors pass through a redaction step that
removes the password before they reach responses or logs.

**Token, cookie, and origin checks.** A 256-bit token from `crypto/rand` is
printed in the access URL, compared in constant time, and exchanged for an
`HttpOnly`, `SameSite=Strict` cookie, after which the redirect drops it from the
address bar. A middleware validates `Host` and `Origin` before any handler,
which defeats DNS rebinding and cross-site requests against the loopback port.
Alternative: no authentication on loopback, rejected because any local process
or web page could then run SQL as `sys` once the SQL editor exists.

**API shaped like the Administration API.** Resources live under `/api/v1`
and follow its paths (`/deployments/{id}`, `/health/ping`), with the launcher
deployment name as the identifier. This keeps a later move into the
Administration API plausible without emulating its full surface.

**Repository layout and build.** A Go module at the root holds the command and
server packages; the Angular application lives in `web/` and builds into a
directory the binary embeds. `exasol-local-ui --web-dir <dir>` serves a
directory from disk instead, so `ng build --watch` gives a fast frontend loop
behind the real server and its token flow. A Taskfile wraps build, test, lint,
and the development loop, matching the launcher's tooling.

**Deployment selection lives in the URL.** The application opens on a
Deployments page of tiles without the layout's navigation, and each
deployment's pages live under `/deployments/{name}` inside the layout, whose
navigation points at that deployment, so reloads and bookmarks keep the
selection.
`--deployment <name>` only changes the path of the printed access URL, which
the token redirect preserves, so the server keeps no UI preferences.
Alternative: a settings endpoint that tells the application what to preselect,
rejected because the URL already carries the choice.

**Screens follow the design canvas.** The tiles page and the overview are
drawn on the [Exasol Local](https://claude.ai/artifact/UQ4cH4hUXciqeB2kzWMRQa)
canvas with the [Exasol Admin UI](https://claude.ai/artifact/WJ3Tm2ihMri7fiUcJtKRKj)
design system, measured from a running Administration UI: squared buttons, the
selected item card with green side bars, and a detail panel of labelled info
boxes. Values the Administration UI's themes do not name stay in the component
styles.

**Frontend stack.** The same Angular, Material, `@exasol/ui-common`, and NgXs
versions as the Administration UI, standalone components, SCSS with BEM, and
Jest for unit tests. HTTP calls go through NgXs actions as there. A single
interceptor maps 401 responses to the reconnect message.

## Risks / Trade-offs

- [Launcher JSON output changes] → The server decodes only the fields it uses
  and reports decode failures with the launcher command and version.
- [`exasol deployments list` takes up to five seconds] → The server caches the
  listing briefly and refreshes it on demand from the selector.
- [`@exasol/ui-common` requires a GitHub Packages token] → The README documents
  the `~/.npmrc` setup; CI receives a read token as a secret.
- [`@exasol/ui-common`'s layout has no header slot] → The application moves
  the deployment selector into the layout header's controls after rendering;
  a header slot in the library would remove this workaround.
- [The token appears in terminal scrollback] → It is valid only until the
  server stops and grants only loopback access.
- [Reading `secrets.json` directly bypasses the launcher] → It is the same file
  the launcher's own sidecars draw from, and the file format is specified in
  the launcher's credential specifications.

## Open Questions

- Should the prototype later move into the launcher as `exasol ui`, or stay a
  separate install? The CLI-based integration keeps both paths open.
- Which distribution channel suits early testers: a release binary, `go
  install`, or a Homebrew tap?

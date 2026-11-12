## Why

Local Exasol Personal deployments have no web UI: the cloud Administration UI depends on an Administration API that local deployments do not run. A prototype needs a foundation first: a local server that people can start safely and that reaches the launcher and the database, and an app shell with the Administration UI's look, so that sidecar management, a SQL editor, and health views can be added as separate pages.

## What Changes

- Add an `exasol-local-ui` command that starts a loopback-only web server and prints a one-time URL, optionally opening it in the browser.
- Protect every request with a per-session token exchanged for a cookie, and reject requests whose `Host` or `Origin` does not belong to the server.
- Open on tiles of the launcher's deployments, with each one's name, folder, and status, and let the user select one.
- Show an overview of the selected deployment: its state, its SQL connection details without the password, and whether the database accepts connections with the stored credentials.
- Provide the Angular app shell with the Administration UI's layout and navigation, served by the same binary.

## Capabilities

### New Capabilities

- `ui-server`: starting the server, the printed access URL, and request authentication and origin checks.
- `deployment-overview`: listing and selecting launcher deployments, and reporting the selected deployment's state, connection details, and database reachability.
- `app-shell`: the browser application's layout, navigation, and session handling.

### Modified Capabilities

None.

## Impact

- New Go module with the server binary and an HTTP API under `/api/v1`.
- New Angular application built on `@exasol/ui-common`, which installs from GitHub Packages and needs a read token for local builds and CI.
- Runtime dependencies on the `exasol` launcher on `PATH` (for `deployments list --json` and `info --json`) and on read access to each deployment's `secrets.json`.
- New dependency on the Exasol Go driver for database connections.

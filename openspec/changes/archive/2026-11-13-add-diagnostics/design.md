## Context

The launcher diagnoses a deployment with three commands, each run with
`--deployment-dir`:

- `diag local` prints JSON with `platform`, `platformSupported`, and, for a
  local deployment, `vmRunning`. A running VM adds `ports` (host ports of
  `db` and, when set, `ui`), `portHealth` (reachability per port name:
  `db`, `ui`, and `sidecar.<name>.<index>`; one of `reachable`, `refused`,
  `blocked`, `timeout`), `databaseReady`, and `databaseError`. `warning`
  reports a VM that runs without a recorded workflow; `message` explains a
  stopped VM ("Run `exasol start` to start it...") or a deployment that is
  not local ("this deployment is not using the local preset"), which still
  exits 0. It takes about 1.5 s, and up to 30 s when the database does not
  answer.
- `diag info` prints the launcher's full deployment report as JSON:
  backend, state, nodes, connection, SSH details for cloud nodes. It holds
  no password today.
- `diag unlock` removes the deployment folder's lock marker without taking
  the lock, prints nothing, and succeeds on exit status 0.

`diag local` and `diag info` take the deployment's shared lock and fail
after one second with the launcher's "operation in progress" message while
a start, stop, or other exclusive operation holds it. `exasol shell host`
and `exasol shell container` are interactive and take `-d <name>`, which
resolves every deployment the listing reports, since the listing scans the
same deployments root.

## Goals / Non-Goals

**Goals:**

- The Diagnostics page as the canvas draws it (`Diagnostics`).
- Diagnostics endpoints shaped so the Administration API could adopt them.

**Non-Goals:**

- Running shells in the browser; the page shows commands to copy into a
  terminal.
- `diag cache`, which belongs to the start page's Runtime cache page
  (`add-launcher-tools`).
- Detecting launcher processes outside the UI before unlocking.

## Decisions

**Local runtime resource.** `GET
/api/v1/deployments/{name}/diagnostics/local` returns `{platform,
platform_supported, vm_running, database_ready?, database_error?, warning?,
message?, ports: [{name, host_port?, reachability}]}`. Ports list `db`, then
`ui`, then sidecars by name; a sidecar port `sidecar.mcp.0` is named
`sidecar mcp`, and `sidecar kafka 2` for a second endpoint. Host ports of
`db` and `ui` come from `ports`; a sidecar's comes from its endpoint in
`sidecar status --json`, which the server reads only when sidecar ports are
listed, leaving `host_port` out when the status fails. Deployments whose
infrastructure is not `local` receive 409 with the launcher's wording "this
deployment is not using the local preset", without running the launcher.
Launcher failures, such as the lock held by an operation, answer 409 with
the launcher's message. Alternative: pass the launcher's JSON through,
rejected because the canvas's port table needs the join with sidecar host
ports and readable names.

**Deployment report.** `GET .../diagnostics/info` returns the launcher's
JSON unchanged, except that the deployment's database password, should it
ever appear, is replaced by `********`, keeping passwords out of the
browser. The page downloads the same response as `<name>-info.json` from a
`Blob`, so one endpoint serves Show and Download. Alternative: a second
endpoint with `Content-Disposition: attachment`, rejected as a duplicate.

**Unlock.** `POST .../diagnostics/unlock` runs `diag unlock` as a short
command and answers 204. While an operation of this server runs on the
deployment, it answers 409 and runs nothing; the launcher client already
runs short commands one at a time. The operations manager gains a query for
a deployment's running operation. Launchers started outside the UI stay
invisible, so the confirmation says to unlock only when none runs.

**Diagnostics page.** Route segment `diagnostics`, drawer entry Diagnostics
with `fa-stethoscope`, after Settings. One detail panel holds the canvas's
info boxes:

- "Local runtime", for local deployments: rows Platform ("darwin/arm64,
  supported" or "unsupported"), VM (running with the dot, or stopped), and
  Database (ready, or not ready with its error); the `message` and `warning`
  when present; the port table (Port, Host port, Reachability, with
  `reachable` in the accent color and other states in red); and Check again.
- "Deployment info": the canvas's sentence with Show, which toggles the
  pretty-printed report below it (then reading Hide), and Download.
- "Shell access": `exasol shell host -d <name>`, `exasol shell host -d
  <name> -- [command]`, and `exasol shell container -d <name>`, each with
  the copy button of the MCP page; names that need it are quoted for the
  shell.
- "Recovery", with the red border: the canvas's sentence and Unlock…, which
  asks with `ConfirmationModalComponent` and reports "Unlocked the folder of
  default." or the API's error as a notification.

**State and refresh.** A diagnostics NgXs state keeps per deployment the
local runtime, its load error, the report, and its error. The page loads
the local runtime on entry and on Check again, and the report on the first
Show or Download. While an operation runs on the deployment, the page shows
the shared progress output above the panel, disables Check again and
Unlock, and loads the local runtime again when the operation ends.

## Risks / Trade-offs

- [A slow `diag local` holds the launcher client's one-at-a-time queue for
  up to 30 s] → The page runs it only on entry and on Check again, and the
  Database row shows the launcher's error.
- [Unlocking while an outside launcher works can leave files inconsistent]
  → The server refuses during its own operations, and the confirmation and
  the Recovery box name the risk.
- [The report shows host details and the SSH key file path] → Same trust as
  the launcher: loopback only, behind the session, and passwords replaced.
- [Shell commands for a stopped or cloud deployment may not connect] → The
  launcher explains in the terminal; the page offers the commands for every
  deployment.

## Why

When a local deployment misbehaves, finding out why takes several launcher commands: `exasol diag local` for the VM, ports, and database, `exasol diag info` for the full report, `exasol shell` to look inside, and `exasol diag unlock` when a crashed launcher left the deployment folder locked. The Diagnostics page gathers them next to the deployment's other pages.

## What Changes

- Add `GET /api/v1/deployments/{name}/diagnostics/local`, reporting a local deployment's platform, VM, database, and port reachability (`diag local`).
- Add `GET /api/v1/deployments/{name}/diagnostics/info`, returning the launcher's full deployment report (`diag info`).
- Add `POST /api/v1/deployments/{name}/diagnostics/unlock`, clearing the deployment folder lock (`diag unlock`), refused while an operation runs on the deployment.
- Add a Diagnostics page per deployment: the local runtime state with a Check again action, the deployment report to show or download, copyable `exasol shell host` and `exasol shell container` commands, and a confirmed Unlock.

## Capabilities

### New Capabilities

- `diagnostics`: reporting a deployment's runtime state and full report through the API, unlocking its folder, and the Diagnostics page.

### Modified Capabilities

- `app-shell`: the navigation drawer lists the Diagnostics page after the Settings page.

## Impact

- Launcher client: `diag local`, `diag info`, and `diag unlock` as short commands.
- New `/api/v1` endpoints, a diagnostics NgXs state, and the Diagnostics page.
- Screens follow the Exasol Local canvas (`Diagnostics`) and the Exasol Admin UI design system.

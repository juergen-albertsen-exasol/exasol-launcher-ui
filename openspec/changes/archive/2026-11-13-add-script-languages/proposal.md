## Why

Local deployments ship without any script language container, so running a Python, Java, R, or Rust UDF first needs `exasol slc` commands, the restart they trigger, and for custom containers a set of flags. The Script languages page puts the catalog, what is installed, and these operations in one place, next to the deployment's other pages.

## What Changes

- Add `GET /api/v1/deployments/{name}/script-languages`, reporting the official containers of the catalog with their install state (`slc list --json`), the Rust container that `slc install rust` provides, and the installed custom containers.
- Add `POST .../script-languages/{alias}/install`, `.../update`, and `.../remove`, and `POST .../script-languages/custom/install` and `.../custom/update`, as operations on the existing runner, each restarting the database or recording the change for the next start (`--no-restart`).
- Add a Script languages page per deployment: a page-wide restart option, the official containers with Install, Update, and Remove…, and the custom containers with Remove… and a form that installs or updates one from a local path or an HTTPS URL.
- The page says when a deployment's type runs no script language containers, and follows an operation running on the deployment.

## Capabilities

### New Capabilities

- `script-languages`: reporting a deployment's script language containers through the API, and the Script languages page with its operations.

### Modified Capabilities

- `operations`: installing, updating, and removing script language containers run as operations, which name their container.
- `app-shell`: the navigation drawer lists the Script languages page between the MCP server and Settings pages.

## Impact

- Launcher client: `slc list --json` as a short command; `slc install/update/remove` and `slc custom install/update` as operations.
- Operation runner: kinds with parameters (alias, language, source, restart).
- New `/api/v1` endpoints, a script languages NgXs state, and the Script languages page.
- Screens follow the Exasol Local canvas (`ScriptLanguages`, revised for the page-wide restart option and the custom container list) and the Exasol Admin UI design system.

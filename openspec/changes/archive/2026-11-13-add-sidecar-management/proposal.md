## Why

Local deployments run sidecars beside the database, among them the MCP server that connects AI agents to it. Seeing which sidecars run and whether they are healthy, turning them on and off, and connecting an AI client all need the launcher CLI today. These are the next launcher capabilities people reach for after start, stop, and settings.

## What Changes

- Add `GET /api/v1/deployments/{name}/sidecars`, reporting the deployment's saved sidecars with their state, health, endpoints, and failures (`sidecar status --json`), together with the catalog sidecars it has not saved yet (`sidecar list --json`).
- Add `POST .../sidecars/{sidecar}/enable` and `POST .../sidecars/{sidecar}/disable` (optionally deleting the saved definition) as operations on the existing runner.
- Add `GET /api/v1/deployments/{name}/mcp/clients` and `GET .../mcp/clients/{client}`, reporting the AI clients with setup instructions and the command or configuration entry that registers the MCP server with one (`mcp instructions --json`).
- Add a Sidecars page per deployment: a table of sidecars with description, state, health, and endpoint, Enable for disabled ones, and Disable with a confirmation that offers to delete the saved definition.
- Add an MCP server page per deployment: the MCP server's state and health with Enable or Disable, its endpoint with a copy button, its health error, and a client picker that shows the copyable registration command or configuration entry.
- Both pages say when the deployment's type runs no sidecars, and follow a sidecar operation running on the deployment.

## Capabilities

### New Capabilities

- `sidecar-management`: reporting a deployment's sidecars through the API, and the Sidecars page with Enable and Disable.
- `mcp-server`: the MCP client setup instructions through the API, and the MCP server page with Enable and Disable.

### Modified Capabilities

- `operations`: enabling and disabling a sidecar run as operations, which name their sidecar.
- `app-shell`: the navigation drawer lists the Sidecars and MCP server pages between the Overview and Settings pages.

## Impact

- Launcher client: `sidecar status --json`, `sidecar list --json`, and `mcp instructions --json`, run as short commands one at a time.
- Operation runner: sidecar kinds that carry a sidecar name.
- New `/api/v1` endpoints, a sidecars NgXs state, the Sidecars and MCP server pages, and a disable confirmation dialog.
- Screens follow the Exasol Local canvas (`Sidecars`, `MCP`) and the Exasol Admin UI design system.

Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server

- [x] 1.1 Add the launcher client's `sidecar status --json`, `sidecar list --json`, and `mcp instructions --json` calls, and the sidecar listing and MCP client endpoints with the merged listing, 404 for unlisted names, and 409 with the launcher's message, tested against fake launcher executables for every API scenario of `sidecar-management` and `mcp-server`
- [x] 1.2 Add the `enable_sidecar`, `disable_sidecar`, and `remove_sidecar` operation kinds with their sidecar name, and the enable and disable endpoints, with tests for every scenario of the new `operations` requirement

## 2. Browser application

- [x] 2.1 Add the Sidecars and MCP server routes and drawer entries, the sidecars NgXs state, operation labels for the sidecar kinds, and the Sidecars page with its table, Enable, the Disable confirmation, the stopped and unsupported notes, operation progress, and the refresh while a sidecar starts, as the design canvas draws it, with tests
- [x] 2.2 Add the MCP server page with its card, Enable and Disable, the endpoint with its copy button and health error, and the client instructions with copyable command or configuration entry, as the design canvas draws it, with tests and a manual check on a scratch local deployment

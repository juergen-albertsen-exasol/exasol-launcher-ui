# mcp-server Specification

## Purpose

Manage a deployment's MCP server and show how to register it with AI clients from its MCP server page.

## Requirements
### Requirement: The API reports MCP client setup instructions

`GET /api/v1/deployments/{name}/mcp/clients` SHALL list the AI clients that
`exasol mcp instructions --json` has setup instructions for, each with its
identifier and display name. `GET
/api/v1/deployments/{name}/mcp/clients/{client}` SHALL return, for one of
these clients, its identifier, display name, the MCP server's endpoint URL,
and either the command that registers the endpoint or the configuration file
with the entry to add to it. An identifier that is not listed SHALL receive
status 404, and a deployment for which the launcher refuses the instructions
SHALL receive status 409 with the launcher's message.

#### Scenario: Listing clients

- **WHEN** the client requests the MCP clients of `default`
- **THEN** the response lists `claude-code`, `claude-desktop`, `codex`,
  `cursor`, `vscode`, `gemini`, `opencode`, and `continue` with their display
  names

#### Scenario: A client registered by command

- **WHEN** the client requests the instructions for `claude-code` on
  `default`, whose MCP server serves on port 4896
- **THEN** the response carries the endpoint URL and a `claude mcp add`
  command that registers it

#### Scenario: A client configured through a file

- **WHEN** the client requests the instructions for `cursor` on `default`
- **THEN** the response names the configuration file and the entry with the
  endpoint URL

#### Scenario: The MCP server is not enabled

- **WHEN** the client requests the instructions for `claude-code` on a
  deployment without the MCP server enabled
- **THEN** the server responds with status 409 and the launcher's message

### Requirement: The MCP server page connects AI clients

The MCP server page of a deployment SHALL show the MCP server's state, health,
and description, with Enable when it is disabled and Disable when it is
enabled, confirmed as on the Sidecars page. For an enabled MCP server, the
page SHALL show its endpoint with a copy button and its health error, and a
client selection that shows the selected client's registration command, or
its configuration file and entry, with a copy button.

#### Scenario: Copying the registration command

- **WHEN** the user opens the MCP server page of `default` and selects Claude
  Code
- **THEN** the page shows the `claude mcp add` command for the endpoint of
  `default`, and its copy button copies the command

#### Scenario: A client configured through a file

- **WHEN** the user selects Cursor on the MCP server page of `default`
- **THEN** the page names the configuration file and shows the entry to add to
  it

#### Scenario: Enabling the MCP server

- **WHEN** the user opens the MCP server page of a deployment without the MCP
  server enabled and selects Enable
- **THEN** the page shows the progress output, and then the endpoint and the
  client instructions

#### Scenario: A failing health check is reported

- **WHEN** the MCP server of `default` runs while its health check fails
- **THEN** the MCP server page shows its health as failing with the health
  check's error below the endpoint


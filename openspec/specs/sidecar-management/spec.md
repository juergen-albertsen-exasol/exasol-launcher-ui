# sidecar-management Specification

## Purpose

Report a deployment's sidecars and enable or disable them from its Sidecars page.

## Requirements
### Requirement: The API reports a deployment's sidecars

`GET /api/v1/deployments/{name}/sidecars` SHALL return the sidecars saved for
the deployment, as `exasol sidecar status --json` reports them, followed by the
catalog sidecars the host supports that the deployment has not saved, as
`exasol sidecar list --json` reports them. Each sidecar SHALL carry its name,
whether it comes from the catalog, the catalog description when it has one,
whether it is enabled and running, its state, its health with the health
error, its operation errors, and its endpoints with address, port, and URL
when the launcher reports one. A deployment for which the launcher refuses to
report sidecars SHALL receive status 409 with the launcher's message. An
unknown name SHALL receive status 404.

#### Scenario: Saved and catalog sidecars

- **WHEN** the client requests the sidecars of the running local deployment
  `default`, which has saved the catalog sidecar `mcp` and the user-defined
  sidecar `kafka`, while the catalog also offers `dash-server`
- **THEN** the response lists `mcp` and `kafka`, enabled and running with
  their endpoints, then `dash-server` as a disabled catalog sidecar with its
  description

#### Scenario: Health failures are reported

- **WHEN** the client requests the sidecars of a deployment whose `mcp`
  sidecar runs while its readiness probe fails
- **THEN** the response reports `mcp` as running with health `failing` and the
  probe's error

#### Scenario: A cloud deployment runs no sidecars

- **WHEN** the client requests the sidecars of a cloud deployment
- **THEN** the server responds with status 409 and the launcher's message that
  sidecars are not supported for this deployment type

### Requirement: The Sidecars page manages a deployment's sidecars

The Sidecars page of a deployment SHALL list its sidecars in a table with
name, description, state, health, and endpoint, linking endpoints that have a
URL, and showing a sidecar's health error and operation errors with it. Each
disabled sidecar SHALL offer Enable, which enables it. Each enabled sidecar
SHALL offer Disable, which opens a confirmation that offers to also delete the
saved definition, and confirming disables the sidecar, deleting the definition
when chosen. When the deployment is not running, the page SHALL say that
changes take effect when the deployment starts. When the API refuses to report
sidecars, the page SHALL show its message.

#### Scenario: Listing sidecars

- **WHEN** the user opens the Sidecars page of the running deployment
  `default`
- **THEN** the page lists each sidecar of `default` with its state, health,
  and endpoint, and offers Disable for the enabled ones and Enable for the
  others

#### Scenario: Enabling a sidecar

- **WHEN** the user selects Enable for `dash-server` on the Sidecars page of
  `default`
- **THEN** the page shows the progress output while `dash-server` is enabled,
  and then lists it as enabled

#### Scenario: Disabling a sidecar and deleting its definition

- **WHEN** the user selects Disable for `kafka`, checks the option to delete
  the saved definition, and confirms
- **THEN** `kafka` is disabled with its saved definition deleted, and the page
  no longer lists it

#### Scenario: A stopped deployment records changes for its next start

- **WHEN** the user opens the Sidecars page of a stopped local deployment
- **THEN** the page says that sidecar changes take effect when the deployment
  starts

#### Scenario: A cloud deployment explains that it runs no sidecars

- **WHEN** the user opens the Sidecars page of a cloud deployment
- **THEN** the page shows the launcher's message that sidecars are not
  supported for this deployment type

### Requirement: Sidecar pages follow running operations

The Sidecars and MCP server pages SHALL show the progress output of an
operation that runs on the deployment and keep their actions unavailable
meanwhile. When the
operation ends, they SHALL show the deployment's current sidecars, or the
operation's error. While a listed sidecar's health is `starting`, they SHALL
refresh the sidecars until it is no longer starting.

#### Scenario: A sidecar becomes healthy

- **WHEN** the Sidecars page shows `mcp` with health `starting` and the
  sidecar starts serving
- **THEN** the page shows `mcp` as healthy without the user reloading it

#### Scenario: A failed enable is reported

- **WHEN** enabling `dash-server` fails
- **THEN** the Sidecars page shows the launcher's error message and the
  current sidecars


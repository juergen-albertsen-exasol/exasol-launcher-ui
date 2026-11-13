## ADDED Requirements

### Requirement: The API enables and disables sidecars as operations

The server SHALL run, in the background and with status 202, the new
operation, and its `Location` header in the response:
`exasol sidecar enable <sidecar>` for `POST
/api/v1/deployments/{name}/sidecars/{sidecar}/enable` as kind
`enable_sidecar`; and `exasol sidecar disable <sidecar>` for `POST
/api/v1/deployments/{name}/sidecars/{sidecar}/disable` as kind
`disable_sidecar`, or `exasol sidecar disable <sidecar> --remove` as kind
`remove_sidecar` when the request body sets `remove_definition` to true. These
operations SHALL name their sidecar. While another operation runs on the same
deployment, the server SHALL respond with status 409 and start nothing. A
deployment or sidecar that the sidecar listing does not contain SHALL receive
status 404.

#### Scenario: Enabling a catalog sidecar

- **WHEN** the client posts to the enable endpoint of `dash-server` on
  `default`
- **THEN** the server responds with status 202 and a running operation of kind
  `enable_sidecar` for `default` that names `dash-server`, and when it
  succeeds the sidecar listing reports `dash-server` as enabled

#### Scenario: Disabling and deleting the definition

- **WHEN** the client posts `{"remove_definition": true}` to the disable
  endpoint of `kafka` on `default`
- **THEN** the server responds with status 202 and an operation of kind
  `remove_sidecar`, and when it succeeds the sidecar listing no longer
  contains `kafka`

#### Scenario: An unlisted sidecar is not found

- **WHEN** the client posts to the enable endpoint of `--all` on `default`
- **THEN** the server responds with status 404 and runs no sidecar command

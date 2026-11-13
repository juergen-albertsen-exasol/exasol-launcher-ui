# operations Specification

## Purpose
Run long launcher commands in the background and let clients follow their progress and outcome.
## Requirements
### Requirement: The API starts and stops deployments as operations

The server SHALL run `exasol start` or `exasol stop` in the background for
`POST /api/v1/deployments/{name}/start` and `.../stop` and respond with status 202,
the new operation, and its address in the `Location` header. While another
operation runs on the same deployment, the server SHALL respond with status 409
and start nothing. An unknown name SHALL receive status 404.

#### Scenario: Stopping a running deployment

- **WHEN** the client posts to the stop endpoint of the running deployment
  `default`
- **THEN** the server responds with status 202 and an operation of kind `stop`
  for `default` with status `running`, and the launcher stops `default`

#### Scenario: A second operation on the same deployment is refused

- **WHEN** the client posts to the start endpoint of `default` while an
  operation runs on `default`
- **THEN** the server responds with status 409

#### Scenario: Operations on different deployments run side by side

- **WHEN** the client starts `default` while an operation runs on `test`
- **THEN** the server responds with status 202 and both operations run

### Requirement: Operations report their outcome

An operation SHALL have the status `running` until its launcher command ends,
then `succeeded` when the command exits with status 0, or `failed` with the
launcher's error message otherwise. A succeeded start or stop SHALL report the
deployment state and database readiness the launcher printed.

#### Scenario: A start succeeds

- **WHEN** a start operation's launcher command finishes successfully
- **THEN** the operation reports status `succeeded`, the deployment state
  `running`, and that the database is ready

#### Scenario: A stop fails

- **WHEN** a stop operation's launcher command exits with a non-zero status
- **THEN** the operation reports status `failed` and the error message the
  launcher wrote

### Requirement: The API lists operations

`GET /api/v1/operations` SHALL list the running operations and the most recent
finished operations since the server started, newest first, each with its id,
deployment, kind, status, start time, and end time once finished.
`GET /api/v1/operations/{id}` SHALL return one operation with its progress
output; an unknown id SHALL receive status 404.

#### Scenario: Running and finished operations are listed

- **WHEN** a start of `default` runs and a stop of `test` has finished
- **THEN** the list contains the running start of `default` and the finished
  stop of `test`, the newer one first

### Requirement: Clients follow an operation's progress

`GET /api/v1/operations/{id}/events` SHALL stream the operation's progress as
server-sent events: each line of progress output the launcher wrote so far,
then each new line as the launcher writes it, and a final event with the
operation's outcome. Progress output SHALL withhold the deployment's database
password.

#### Scenario: Following a running start

- **WHEN** a client opens the event stream of a running start operation
- **THEN** it receives the progress lines written so far, then each further
  line, and finally the operation's outcome when the launcher finishes

### Requirement: The API deploys, destroys, and removes deployments as operations

The server SHALL run, in the background and with status 202, the new
operation, and its `Location` header in the response:
`exasol deploy` for `POST /api/v1/deployments/{name}/deploy` as kind `deploy`;
`exasol destroy` for `POST /api/v1/deployments/{name}/destroy` as kind
`destroy`, or `exasol destroy --remove` as kind `destroy_and_remove` when the
request body sets `remove_folder` to true; and `exasol remove` for
`POST /api/v1/deployments/{name}/remove` as kind `remove`. While another
operation runs on the same deployment, the server SHALL respond with status
409 and start nothing. An unknown name SHALL receive status 404.

#### Scenario: Deploying an initialized deployment

- **WHEN** the client posts to the deploy endpoint of the initialized
  deployment `test`
- **THEN** the server responds with status 202 and an operation of kind
  `deploy` for `test` with status `running`, and the launcher deploys `test`

#### Scenario: Destroying and removing the folder

- **WHEN** the client posts `{"remove_folder": true}` to the destroy endpoint of
  `default`
- **THEN** the server responds with status 202 and an operation of kind
  `destroy_and_remove`, and when it succeeds the deployment listing no longer
  contains `default`

#### Scenario: Removing a deployment folder

- **WHEN** the client posts to the remove endpoint of `test`
- **THEN** the server responds with status 202 and an operation of kind
  `remove`, and when it succeeds the deployment folder of `test` no longer
  exists

#### Scenario: Destroy waits for a running operation

- **WHEN** the client posts to the destroy endpoint of `default` while an
  operation runs on `default`
- **THEN** the server responds with status 409

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

### Requirement: The API installs, updates, and removes script language containers as operations

The server SHALL run, in the background and with status 202, the new
operation, and its `Location` header in the response, for these requests on
`/api/v1/deployments/{name}/script-languages`:

- `POST {alias}/install`: `exasol slc install <alias>`, kind
  `install_script_language`;
- `POST {alias}/update`: `exasol slc update <alias>`, kind
  `update_script_language`;
- `POST {alias}/remove`: `exasol slc remove <alias>`, kind
  `remove_script_language`;
- `POST custom/install` and `POST custom/update` with the body fields `alias`,
  `language`, and `source`: `exasol slc custom install` or `exasol slc custom
  update` with these values, kinds `install_custom_script_language` and
  `update_custom_script_language`.

Each command SHALL restart a running database to apply the change, or, when
the request body sets `restart` to false, record the change for the next
start. These operations SHALL name their container's alias. While another
operation runs on the same deployment, the server SHALL respond with status
409 and start nothing. An alias that the script language listing does not
offer for the request SHALL receive status 404, and a custom request without
an alias, a language, or a source SHALL receive status 400.

#### Scenario: Installing an official container

- **WHEN** the client posts to the install endpoint of `PYTHON3` on `default`
- **THEN** the server responds with status 202 and a running operation of kind
  `install_script_language` that names `PYTHON3`, and when it succeeds the
  script language listing reports the Python container as installed

#### Scenario: Removing without a restart

- **WHEN** the client posts `{"restart": false}` to the remove endpoint of
  `JAVA` on the running deployment `default`
- **THEN** the server runs `exasol slc remove JAVA --no-restart` as an
  operation of kind `remove_script_language`, and the database keeps running

#### Scenario: Installing a custom container

- **WHEN** the client posts `{"alias": "MYPY3", "language": "python",
  "source": "/home/me/my-python.tar.gz"}` to the custom install endpoint of
  `default`
- **THEN** the server responds with status 202 and an operation of kind
  `install_custom_script_language` that names `MYPY3`, and when it succeeds
  the listing reports `MYPY3` among the custom containers

#### Scenario: An unlisted alias is not found

- **WHEN** the client posts to the install endpoint of `--all` on `default`
- **THEN** the server responds with status 404 and runs no script language
  command


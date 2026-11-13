## ADDED Requirements

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

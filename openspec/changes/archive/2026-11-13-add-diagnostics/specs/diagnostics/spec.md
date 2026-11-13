## ADDED Requirements

### Requirement: The API reports a local deployment's runtime state

`GET /api/v1/deployments/{name}/diagnostics/local` SHALL report, as
`exasol diag local` determines them, the host platform and whether it is
supported, whether the deployment's VM runs, whether its database is ready
with the error when it is not, the launcher's message and warning, and each
forwarded port with its name, host port, and reachability. Sidecar ports
SHALL be named after their sidecar. A deployment whose infrastructure is
not local SHALL receive status 409 with the message that the deployment is
not using the local preset, and a launcher refusal, such as a running
operation holding the deployment, SHALL receive status 409 with the
launcher's message. An unknown name SHALL receive status 404.

#### Scenario: A running local deployment

- **WHEN** the client requests the local runtime state of the running
  deployment `default`, whose database and MCP server answer
- **THEN** the response reports a supported platform, a running VM, a ready
  database, and the ports `db` on host port 8563 and `sidecar mcp` on host
  port 4896, both reachable

#### Scenario: A stopped local deployment

- **WHEN** the client requests the local runtime state of a stopped local
  deployment
- **THEN** the response reports a VM that does not run and the launcher's
  message to start the deployment

#### Scenario: A cloud deployment has no local runtime

- **WHEN** the client requests the local runtime state of a cloud deployment
- **THEN** the server responds with status 409 and the message that the
  deployment is not using the local preset

### Requirement: The API returns a deployment's full report

`GET /api/v1/deployments/{name}/diagnostics/info` SHALL return the JSON
report of `exasol diag info` for the deployment, with the deployment's
database password replaced by `********` wherever it appears. An unknown
name SHALL receive status 404.

#### Scenario: Reading the report

- **WHEN** the client requests the report of `default`
- **THEN** the response is the launcher's report, including the deployment
  state and connection details

#### Scenario: The password stays on the host

- **WHEN** the launcher's report contains the deployment's database password
- **THEN** the response carries `********` in its place

### Requirement: The API unlocks a deployment's folder

`POST /api/v1/deployments/{name}/diagnostics/unlock` SHALL run
`exasol diag unlock` for the deployment and respond with status 204. While
an operation runs on the deployment, the server SHALL respond with status
409 and leave the folder locked.

#### Scenario: Unlocking a deployment

- **WHEN** the client posts to the unlock endpoint of `default` while no
  operation runs on it
- **THEN** the server runs `exasol diag unlock` for `default` and responds
  with status 204

#### Scenario: Unlocking waits for running operations

- **WHEN** the client posts to the unlock endpoint of `default` while
  `default` starts
- **THEN** the server responds with status 409

### Requirement: The Diagnostics page shows the local runtime

The Diagnostics page of a local deployment SHALL show the platform, the VM
state, the database state, the launcher's message and warning, and a table
of the ports with their host port and reachability. Check again SHALL
refresh this state. The page SHALL refresh it when an operation on the
deployment ends and keep Check again unavailable while one runs.

#### Scenario: A running deployment

- **WHEN** the user opens the Diagnostics page of the running deployment
  `default`
- **THEN** the page shows the platform as supported, the VM as running, the
  database as ready, and the ports `db` and `sidecar mcp` as reachable

#### Scenario: Checking again

- **WHEN** the MCP server of `default` stops answering and the user selects
  Check again
- **THEN** the page shows the port `sidecar mcp` with its new reachability

#### Scenario: A stopped deployment

- **WHEN** the user opens the Diagnostics page of a stopped local deployment
- **THEN** the page shows the VM as stopped and the launcher's message to
  start the deployment

#### Scenario: A cloud deployment

- **WHEN** the user opens the Diagnostics page of a cloud deployment
- **THEN** the page offers the deployment report, the shell commands, and
  Unlock without a local runtime section

### Requirement: The Diagnostics page shows and downloads the full report

The Diagnostics page SHALL show the deployment's full report as formatted
JSON on request, and download it as a file named after the deployment.

#### Scenario: Showing the report

- **WHEN** the user selects Show on the Diagnostics page of `default`
- **THEN** the page shows the report of `default` as formatted JSON

#### Scenario: Downloading the report

- **WHEN** the user selects Download on the Diagnostics page of `default`
- **THEN** the browser saves the report as `default-info.json`

### Requirement: The Diagnostics page offers shell commands

The Diagnostics page SHALL show, each with a copy button, the launcher
commands that open a shell on the deployment's host node, run one command
there, and open a shell in its database container.

#### Scenario: Copying a shell command

- **WHEN** the user selects the copy button next to the container shell
  command on the Diagnostics page of `default`
- **THEN** the clipboard holds `exasol shell container -d default`

### Requirement: The Diagnostics page unlocks the deployment folder

The Diagnostics page SHALL offer Unlock, which asks for confirmation and
warns that unlocking while a launcher works on the deployment can leave its
files inconsistent, then unlocks the folder and reports the outcome. Unlock
SHALL be unavailable while an operation runs on the deployment.

#### Scenario: Unlocking after confirmation

- **WHEN** the user selects Unlock on the Diagnostics page of `default` and
  confirms
- **THEN** the folder of `default` is unlocked and the page reports it

#### Scenario: Unlock during an operation

- **WHEN** an operation runs on `default`
- **THEN** the Diagnostics page of `default` shows its progress and Unlock
  is unavailable

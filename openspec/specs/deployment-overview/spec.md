# deployment-overview Specification

## Purpose

List the launcher's deployments and describe one: its state, presets, connection details, and database reachability, through the HTTP API and the browser.
## Requirements
### Requirement: The API lists launcher deployments

`GET /api/v1/deployments` SHALL return one entry per deployment that
`exasol deployments list` reports, in the same order, each with its `name`,
deployment directory `path`, `status`, `infrastructure`, and `installation`.

#### Scenario: Deployments match the launcher listing

- **WHEN** the launcher reports the deployments `default` and `test`
- **THEN** the endpoint returns entries for `default` and `test` with the
  paths, statuses, and presets the launcher reports

#### Scenario: No deployments yields an empty list

- **WHEN** the launcher reports no deployments
- **THEN** the endpoint returns an empty list with status 200

### Requirement: The API describes one deployment

`GET /api/v1/deployments/{name}` SHALL return the deployment's state and its
SQL connection `host`, `port`, `username`, and whether certificate validation
is skipped, as `exasol info` reports them. The response SHALL contain the
connection details only, without the database password. An unknown name SHALL
receive status 404.

#### Scenario: A running deployment is described

- **WHEN** the client requests a running deployment
- **THEN** the response carries state `running` and the host, port, and
  username the launcher reports for it

#### Scenario: An unknown deployment is not found

- **WHEN** the client requests a name the launcher does not list
- **THEN** the server responds with status 404

### Requirement: The API reports database reachability

`GET /api/v1/deployments/{name}/health/ping` SHALL connect to the deployment's
database with its stored credentials, run `SELECT 1`, and report `reachable`
with the round-trip time in milliseconds, or `unreachable` with the error
message the connection produced. Messages SHALL show the user, host, and port
and withhold the password.

#### Scenario: A running database is reachable

- **WHEN** the client pings a deployment whose database accepts connections
- **THEN** the response reports `reachable` and a round-trip time

#### Scenario: A stopped database is unreachable

- **WHEN** the client pings a stopped deployment
- **THEN** the response reports `unreachable` with the connection error, and
  the response text contains the connection's user, host, and port and
  withholds the password

### Requirement: The application opens on deployment tiles

The browser application SHALL open on a Deployments page that shows the Exasol
logo and one tile per deployment the launcher lists, in listing order, each
with the deployment's name, status, and infrastructure preset, and no
navigation drawer. While the listing is pending, the page SHALL show a progress
indicator. Selecting a tile SHALL open that deployment's overview page.

#### Scenario: Tiles show the listed deployments

- **WHEN** the user opens the application and the launcher lists `default` and
  `test`
- **THEN** the Deployments page shows a tile for `default` and one for `test`,
  each with its name, status, and infrastructure preset

#### Scenario: A pending listing shows progress

- **WHEN** the user opens the application and the launcher has not answered
  yet
- **THEN** the Deployments page shows a progress indicator that says the
  deployments are being looked up

#### Scenario: Selecting a tile opens its overview

- **WHEN** the user selects the tile of `test`
- **THEN** the application opens the overview page of `test`

#### Scenario: An empty listing explains how to create a deployment

- **WHEN** the launcher lists no deployments
- **THEN** the Deployments page tells the user to create one with
  `exasol install`

### Requirement: The overview page shows one deployment

The overview page of a deployment SHALL show its name, state, status,
deployment directory, infrastructure and installation presets, connection
details, and database reachability. While it is open, the header's deployment selector SHALL
show that deployment and list every deployment, and choosing another
deployment SHALL open that deployment's overview page.

#### Scenario: The overview describes its deployment

- **WHEN** the user opens the overview page of `default`
- **THEN** the page shows the name, state, status, deployment directory,
  presets, connection details, and reachability of `default`, and the selector
  shows `default`

#### Scenario: Switching deployments opens their overview

- **WHEN** the user chooses `test` in the selector on the overview page of
  `default`
- **THEN** the application opens the overview page of `test`, showing its
  state, connection details, and reachability

#### Scenario: An unknown deployment is reported

- **WHEN** the user opens the overview page of a name the launcher does not
  list
- **THEN** the page reports that the launcher lists no such deployment

### Requirement: The startup flag opens a deployment's overview

The access URL SHALL open the overview page of the deployment given with
`exasol-launcher-ui --deployment <name>`.

#### Scenario: The startup flag chooses the deployment

- **WHEN** the server was started with `--deployment test` and the user opens
  the printed access URL
- **THEN** the application opens the overview page of `test`

### Requirement: The API opens a deployment's folder

`POST /api/v1/deployments/{name}/open-folder` SHALL open the deployment
directory that `exasol deployments list` reports for that name in the host's
file manager and respond with status 204. An unknown name SHALL receive status
404.

#### Scenario: A listed deployment's folder opens

- **WHEN** the client posts to the open-folder endpoint of `default`
- **THEN** the host's file manager opens the deployment directory of `default`
  and the server responds with status 204

#### Scenario: An unknown deployment is not found

- **WHEN** the client posts to the open-folder endpoint of a name the launcher
  does not list
- **THEN** the server responds with status 404

### Requirement: The overview links to the deployment folder

The overview page SHALL show the deployment directory as a link whose address
is the directory's `file://` URL. Selecting the link SHALL open the folder in
the host's file manager.

#### Scenario: Selecting the folder link opens the folder

- **WHEN** the user selects the folder link on the overview page of `default`
- **THEN** the host's file manager opens the deployment directory of `default`
  and the browser stays on the overview page

#### Scenario: The link carries the folder's address

- **WHEN** the user copies the folder link on the overview page of `default`
- **THEN** the copied address is the `file://` URL of its deployment directory

### Requirement: The overview starts and stops its deployment

The overview page SHALL offer Start for a stopped deployment and Stop for a
running one. Selecting either SHALL start the operation and, while it runs,
show the operation's progress output in place of the deployment's details,
with the actions unavailable. When the operation ends, the page SHALL show the
deployment's new state, connection details, and reachability, or the
operation's error.

#### Scenario: Starting a stopped deployment

- **WHEN** the user selects Start on the overview page of the stopped
  deployment `default`
- **THEN** the page shows that `default` is starting and the launcher's
  progress output, and when the start succeeds it shows `default` running with
  its connection details and reachability

#### Scenario: Stopping a running deployment

- **WHEN** the user selects Stop on the overview page of the running deployment
  `default`
- **THEN** the page shows the stop's progress output, and when the stop
  succeeds it shows `default` stopped and offers Start

#### Scenario: A failed operation is reported

- **WHEN** a start or stop of `default` fails
- **THEN** the overview page of `default` shows the launcher's error message

#### Scenario: Returning to a running operation

- **WHEN** the user opens the overview page of a deployment while an operation
  runs on it
- **THEN** the page shows that operation's progress output


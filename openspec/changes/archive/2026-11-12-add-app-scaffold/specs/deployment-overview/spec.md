## ADDED Requirements

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
with the deployment's name, deployment directory, status, and infrastructure
preset, and no navigation drawer. Selecting a tile SHALL open that
deployment's overview page.

#### Scenario: Tiles show the listed deployments

- **WHEN** the user opens the application and the launcher lists `default` and
  `test`
- **THEN** the Deployments page shows a tile for `default` and one for `test`,
  each with its name, deployment directory, status, and infrastructure preset

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
`exasol-local-ui --deployment <name>`.

#### Scenario: The startup flag chooses the deployment

- **WHEN** the server was started with `--deployment test` and the user opens
  the printed access URL
- **THEN** the application opens the overview page of `test`

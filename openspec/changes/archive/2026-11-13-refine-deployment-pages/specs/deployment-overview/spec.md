## ADDED Requirements

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

## MODIFIED Requirements

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

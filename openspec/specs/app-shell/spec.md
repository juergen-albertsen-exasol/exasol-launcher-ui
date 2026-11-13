# app-shell Specification

## Purpose

Frame the deployment pages in the Administration UI layout and keep the browser application usable across reloads and server restarts.
## Requirements
### Requirement: Deployment pages use the Administration UI layout

The pages of a selected deployment SHALL use the `@exasol/ui-common` layout of
the Exasol Administration UI: a header with the deployment selector, and a
navigation drawer with the Exasol logo that lists the deployment's pages,
starting with its Overview page.

#### Scenario: The shell frames the overview page

- **WHEN** the user opens the overview page of `test`
- **THEN** the header shows the deployment selector, and the navigation drawer
  shows the Exasol logo and lists the Overview page of `test` as the current
  page

### Requirement: Application routes survive reloads

The server SHALL serve the application for every non-API path, so that a
reloaded or bookmarked application route opens the same page.

#### Scenario: Reloading a route keeps the page

- **WHEN** the user reloads the browser on an application route
- **THEN** the application opens on that route

### Requirement: An expired session explains how to reconnect

When an API request receives status 401, the application SHALL show a message
telling the user to open the access URL that `exasol-local-ui` printed.

#### Scenario: A restarted server invalidates the open page

- **WHEN** the server restarts while the application is open and the page
  makes its next API request
- **THEN** the page shows the message to open the newly printed access URL

### Requirement: The Exasol logo returns to the Deployments page

The Exasol logo at the top left of every page SHALL be a link to the
Deployments page.

#### Scenario: The logo leaves an overview

- **WHEN** the user selects the Exasol logo on the overview page of `default`
- **THEN** the application opens the Deployments page

#### Scenario: The logo reloads the Deployments page

- **WHEN** the user selects the Exasol logo on the Deployments page
- **THEN** the application shows the Deployments page


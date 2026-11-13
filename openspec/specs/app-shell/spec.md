# app-shell Specification

## Purpose

Frame the deployment pages in the Administration UI layout and keep the browser application usable across reloads and server restarts.
## Requirements
### Requirement: Deployment pages use the Administration UI layout

The pages of a selected deployment SHALL use the `@exasol/ui-common` layout of
the Exasol Administration UI: a header with the deployment selector, and a
navigation drawer with the Exasol logo that lists the deployment's pages:
its Overview page, then its Settings page.

#### Scenario: The shell frames the overview page

- **WHEN** the user opens the overview page of `test`
- **THEN** the header shows the deployment selector, and the navigation drawer
  shows the Exasol logo and lists the Overview and Settings pages of `test`,
  with Overview as the current page

#### Scenario: The drawer opens the Settings page

- **WHEN** the user selects Settings in the navigation drawer on the overview
  page of `test`
- **THEN** the application opens the Settings page of `test`, and choosing
  `default` in the deployment selector opens the Settings page of `default`

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

### Requirement: Running operations stay visible on every page

While operations run, the header of every page SHALL show an activity
indicator naming the running operation, or the number of running operations
when several run. Selecting the indicator SHALL open a panel listing the
running operations, each with its elapsed time and a link to its deployment's
overview, and the recently finished operations, each with its outcome and when
it finished.

#### Scenario: An operation stays visible across pages

- **WHEN** the user starts `default` and then opens the Deployments page
- **THEN** the header shows the activity indicator naming the start of
  `default`

#### Scenario: The panel leads back to the progress

- **WHEN** the user opens the activity panel and selects the running start of
  `default`
- **THEN** the application opens the overview page of `default`, showing the
  start's progress output

#### Scenario: Finished operations are listed

- **WHEN** the user opens the activity panel after a stop of `test` finished
- **THEN** the panel lists the stop of `test` with its outcome and when it
  finished


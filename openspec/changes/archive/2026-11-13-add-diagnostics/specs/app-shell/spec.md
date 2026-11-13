## MODIFIED Requirements

### Requirement: Deployment pages use the Administration UI layout

The pages of a selected deployment SHALL use the `@exasol/ui-common` layout of
the Exasol Administration UI: a header with the deployment selector, and a
navigation drawer with the Exasol logo that lists the deployment's pages:
its Overview page, its Sidecars page, its MCP server page, its Script
languages page, its Settings page, then its Diagnostics page.

#### Scenario: The shell frames the overview page

- **WHEN** the user opens the overview page of `test`
- **THEN** the header shows the deployment selector, and the navigation drawer
  shows the Exasol logo and lists the Overview, Sidecars, MCP server, Script
  languages, Settings, and Diagnostics pages of `test`, with Overview as the
  current page

#### Scenario: The drawer opens the Settings page

- **WHEN** the user selects Settings in the navigation drawer on the overview
  page of `test`
- **THEN** the application opens the Settings page of `test`, and choosing
  `default` in the deployment selector opens the Settings page of `default`

#### Scenario: The drawer opens the Sidecars page

- **WHEN** the user selects Sidecars in the navigation drawer on the overview
  page of `test`
- **THEN** the application opens the Sidecars page of `test`

#### Scenario: The drawer opens the Script languages page

- **WHEN** the user selects Script languages in the navigation drawer on the
  overview page of `test`
- **THEN** the application opens the Script languages page of `test`

#### Scenario: The drawer opens the Diagnostics page

- **WHEN** the user selects Diagnostics in the navigation drawer on the
  overview page of `test`
- **THEN** the application opens the Diagnostics page of `test`

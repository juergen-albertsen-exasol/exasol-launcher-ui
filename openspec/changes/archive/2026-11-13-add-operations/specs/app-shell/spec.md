## ADDED Requirements

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

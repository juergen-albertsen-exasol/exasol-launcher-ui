## ADDED Requirements

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

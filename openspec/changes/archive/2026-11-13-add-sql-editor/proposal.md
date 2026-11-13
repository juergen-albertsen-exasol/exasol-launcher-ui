## Why

Running SQL against a local deployment today means leaving the UI for `exasol connect`, exapump, or a separate SQL client, and finding the deployment's password for them. A SQL page beside the deployment's other pages lets users query, explore, and change their database right where they manage it, while the password stays on the host.

## What Changes

- Add SQL sessions to the API under `/api/v1/deployments/{name}/sql`: open and close a session, run one statement in it with its result set (capped at 10,000 rows) or row count, cancel the running statement, and download a statement's full result as CSV.
- Add schema metadata endpoints: the deployment's schemas with their tables and views, and the columns of one schema's tables.
- Add a SQL page per deployment, as the canvas draws it (`Sql`, `SqlRunning`, `SqlError`, `SqlHistory`, `SqlStopped`): a CodeMirror 6 editor with Exasol SQL highlighting and completion of schema, table, and column names; Run for the statement at the cursor or the selection, Run all for every statement with one result tab each, Cancel, and Format; a schema browser with a filter; results with column types; errors with their position in the editor; a searchable history of the last 100 statements; and the editor's text kept per deployment in the browser.
- The SQL page offers Start when the database is not running and keeps the editor's text.

## Capabilities

### New Capabilities

- `sql-editor`: SQL sessions, statements, cancellation, CSV export, and schema metadata through the API, and the SQL page with its editor, schema browser, results, and history.

### Modified Capabilities

- `app-shell`: the navigation drawer lists the SQL page right after the Overview page.

## Impact

- Server: a new `internal/sql` package that holds database sessions per deployment and runs statements with the deployment's stored credentials, and new `/api/v1` endpoints; errors pass through password redaction.
- Browser: new dependencies `codemirror`, `@codemirror/*` and `@lezer/highlight`; a SQL module ported from fusion-cloud (dialect, statement splitter, completion, formatter, history storage); a SQL NgXs state and the SQL page.
- The `app-shell` delta lists the Diagnostics page that `add-diagnostics` adds, so this change archives after it.

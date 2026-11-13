Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server

- [x] 1.1 Add `internal/sql` with sessions per deployment (open, close, 30-minute idle close, close on shutdown), statements through the driver adapter with the 10,000-row cap, total rows, column types, values as text, open schema, and 422 errors with code, line, and column, plus the session and statement endpoints, with tests for every API scenario of "The API opens and closes SQL sessions" and "The API runs statements in a session" and an `integration`-tagged test against a local deployment
- [x] 1.2 Add cancelling a running statement and the one-download CSV export with its five-minute expiry on a separate connection, with tests for the cancel and export scenarios
- [x] 1.3 Add the schema listing and schema column endpoints on short connections, with tests for the schema scenarios

## 2. Browser application

- [x] 2.1 Add the CodeMirror 6 dependencies, port fusion-cloud's dialect, splitter, completion, formatter, and storage modules with their tests, and add `SqlEditorComponent` with Exasol highlighting, line numbers, the run shortcuts, error marks, and design-system theming
- [x] 2.2 Add the SQL route and drawer entry, the SQL NgXs state, and the SQL page with Run, Run all, Cancel, Format, the open schema, result tabs with the AG Grid table, truncation notice, Download CSV, errors with Go to, session renewal, and the stopped state with Start, as the canvas draws it, with tests and a manual check on a local deployment
- [x] 2.3 Add the schema panel with filter, expansion, insertion by double-click, and reload, name completion in the editor, and the catalog reload after DDL, with tests
- [x] 2.4 Keep the editor's text per deployment and add the History menu with search, insertion, and Clear, with tests

Each task is one commit that passes `task build`, `task test`, and `task lint`.
`add-sql-editor` is archived before this change starts.

## 1. Server

- [x] 1.1 Add `internal/imports` with the Parquet footer reader, the port of fusion-cloud's inference (types, table names, alternatives, same columns, unsupported-columns message), and sample values, tested with fusion-cloud's footer fixtures and generated files
- [x] 1.2 Add uploads: storage in the temporary folder, the `POST /uploads` and `PUT /uploads/{id}/files/{n}` endpoints, the 24-hour janitor, and removal on shutdown, with tests for the upload scenarios
- [x] 1.3 Add import suggestions with the table-taken check against the database and the `POST /import-suggestions` endpoint, with tests for the suggestion scenarios
- [x] 1.4 Add imports: the SQL in one transaction, per-file progress, cancellation, the in-memory list, and the `POST`, `GET`, and `DELETE` import endpoints, with tests for the import scenarios and an `integration`-tagged test that compares the inference with Exasol's on a local deployment

## 2. Browser application

- [x] 2.1 Add the import API models and service methods, the `ImportState`, and the Import data route and drawer entry after SQL, with tests
- [x] 2.2 Add the Import data page's drop area with upload progress, file errors, recent imports, and the stopped state with Start, as the canvas draws them, with tests
- [x] 2.3 Add the suggested table with editable schema, table, column names and types, Reset, problems, and the import button state, as the canvas draws it, with tests
- [x] 2.4 Add the running and finished import views with per-file progress, SQL, Cancel, leaving and returning, and "Open in the SQL editor", with tests and a manual check on a local deployment

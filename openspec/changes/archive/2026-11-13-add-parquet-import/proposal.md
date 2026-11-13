## Why

Getting a Parquet file from one's computer into a local deployment today means writing `IMPORT FROM LOCAL PARQUET` by hand, knowing which Exasol types the file's columns become, and running it through `exasol connect` or exapump. An Import data page that reads the file's schema, shows the table Exasol will create, and creates it lets users go from a file to a query in a few clicks, as Fusion Cloud's import does.

## What Changes

- Add an Import data page per deployment for Parquet files on this computer, dropped or chosen, as the canvas draws it (`ImportData`, `ImportSuggestion`, `ImportProblems`, `ImportRunning`, `ImportDone`). The canvas's Amazon S3 and HTTPS URL tabs (`ImportS3`) are left for a later change.
- Before anything is imported, the page shows the suggested table: schema, table name, row count, and every column with the Exasol type that Exasol's schema inference for Parquet gives it, its Parquet type, and a sample value. Users can change the schema, the table, and each column's name and type. Taken table names, files with different columns, and columns Exasol cannot import are reported before the import.
- Importing creates the table with `CREATE TABLE` and loads each file with `IMPORT INTO … FROM LOCAL PARQUET` in one transaction, reports progress per file, can be cancelled, and leaves no table behind on failure. A finished import opens the SQL editor with a query on the new table.
- Add the import API under `/api/v1/deployments/{name}`: uploads, import suggestions, and imports with their status, plus the list of recent imports.
- Add "Import data" to the navigation drawer, right after SQL.

## Capabilities

### New Capabilities

- `data-import`: Parquet uploads, import suggestions with Exasol's type inference, imports of uploaded files through the API, and the Import data page.

### Modified Capabilities

- `app-shell`: the navigation drawer lists the Import data page right after the SQL page.

## Impact

- Server: a new `internal/imports` package with the Parquet footer reader and Exasol's inference mapping, upload storage in a temporary folder, and imports run with `exasol-driver-go`; new `/api/v1` endpoints.
- Dependencies: `github.com/parquet-go/parquet-go`, already in the module graph through the driver.
- Browser: an import NgXs state, the Import data page and its route and drawer entry, and a way to open the SQL page with a statement.
- Builds on `add-sql-editor`: its `app-shell` delta and the SQL page must be archived first.

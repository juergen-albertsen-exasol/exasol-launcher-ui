## Context

Fusion Cloud already imports Parquet into new tables
(`~/dev/fusion-cloud/openspec/specs/data-import/spec.md`, its
`import-data` design, and `control-plane/src/imports/`): it reads each file's
footer, applies Exasol's documented Parquet type mapping (`inference.ts`),
checks the table name, and runs `CREATE TABLE` and `IMPORT INTO` in one
transaction. Its console's `ImportPanel` holds the drop area, the editable
suggestion, and the progress. This change brings the same flow to local
deployments, for files on the user's computer.

The database runs in a VM or container that cannot read the host's files,
and the server binds to loopback only, so the database cannot fetch files
from the UI either. `exasol-driver-go` v1.2.0, which the server already uses,
runs `IMPORT INTO <table> FROM LOCAL PARQUET FILE '<path>'`: it streams one
host file per statement through the driver's proxy connection, which works
for cloud deployments too. It accepts only ASCII letters, digits, spaces, and
`: < > / . _ - ~` in the path. Exasol infers a table only from Parquet it
reads itself (`CREATE TABLE … AS (IMPORT FROM PARQUET AT …)`), never from a
local file. The driver's own `ImportParquetWithInferredSchema` maps types
differently from Exasol (DATE becomes `TIMESTAMP(9)`, INT64
`DECIMAL(19,0)`, and it renames columns), which the launcher's
`local-parquet-import` documents as a known difference.

`add-sql-editor` adds `internal/sql`, the driver adapter, and the SQL page
this change opens after an import; this change builds on it.

## Goals / Non-Goals

**Goals:**

- From a Parquet file to a table and a first query in one page, with the
  table shown before it is created.
- The suggested table is what Exasol's own inference would create for the
  same file, unless the user changes it.
- The import API is shaped like Fusion Cloud's, so S3 and HTTPS sources can
  join it later as further `source` kinds.

**Non-Goals:**

- Amazon S3 and HTTPS sources, which the canvas draws and a later change
  adds.
- Formats other than Parquet, appending to existing tables, and querying in
  place.
- Reading host paths typed by the user; files arrive through the browser.
- Showing imports in the activity panel, which lists launcher operations.

## Decisions

**One inference module, in Go.** `internal/imports` ports Fusion Cloud's
`inference.ts`, including table naming, alternatives for taken names, the
same-columns check, and the unsupported-columns message, and its fixtures
(`control-plane/test/fixtures/footers.json` and `inferred.json`) become Go
test data, so both products suggest the same table for the same file. Footers
are read with `github.com/parquet-go/parquet-go`, already in the module graph
through the driver; the first row gives the samples, bounded by three seconds
as in Fusion. An `integration`-tagged test downloads the public sample files,
suggests their tables, and compares them with the tables Exasol infers for
the same files over HTTPS on a local deployment. Alternative: the driver's
`ImportParquetWithInferredSchema`, rejected because its types and column
names differ from Exasol's.

**SQL.** Every import runs on its own connection with autocommit off:
`CREATE SCHEMA IF NOT EXISTS`, `CREATE TABLE` with the suggested or changed
columns, one `IMPORT INTO … FROM LOCAL PARQUET FILE '<path>'` per file, which
also gives per-file progress, then `COMMIT`. Any failure or cancellation runs
`ROLLBACK`, which also removes a new schema. Column names are always quoted,
keeping the Parquet case as Exasol's inference does. The server names stored
files `<n>.parquet` in a folder of safe characters, so user file names never
reach the driver's path check.

**Uploads on the host.** Uploads live under the user's temporary folder in
`exasol-launcher-ui/uploads/<id>/`, created with mode 0700. The browser sends
each file with its own `PUT` so it can show progress per file; the server
streams the body to disk and checks the declared size. A janitor removes
uploads 24 hours after their last use, an import removes its upload when it
finishes, and shutdown removes the folder. Alternative: multipart form
uploads, rejected because one large request cannot report per-file progress
or be retried per file.

**Imports as server-side jobs.** `internal/imports` keeps imports in memory
per server run, newest first, and runs each in a goroutine whose context
`DELETE` cancels; cancellation reaches the driver as `abortQuery`, as in the
SQL editor's cancel. The page polls the import every second while it runs and
loads the list when it opens, so leaving and returning finds the import.
Several imports may run at once; launcher operations are not blocked, and an
import that loses its database by a Stop fails with the database's message.
Alternative: `internal/operations`, rejected because it runs launcher
commands, allows one operation per deployment, and an import must not block
Stop.

**API shape.** Following Fusion's resources and this server's snake_case:
`POST /uploads` → `{id, files: [{name, size, url}]}`; `PUT
/uploads/{id}/files/{n}`; `POST /import-suggestions` with `{source}` →
`{source: {kind, label, files, bytes}, schema, table, rows, columns: [{name,
type, parquet_type, sample}], importable, problems: [{code, message,
columns?, alternative?}]}`; `POST /imports` with `{source, schema, table,
columns?}` → 202 `Import`; `GET /imports`, `GET /imports/{id}`, `DELETE
/imports/{id}`. `source` is `{kind: "upload", upload_id}`; the `kind` leaves
room for `s3` and `https`. Errors are RFC 9457 problems with `code`, as the
SQL endpoints answer them.

**Browser.** An `ImportState` (NgXs) holds the upload, the suggestion with
the user's edits, the running import, and the recent list. The page uses the
design-system classes the canvas shows and the browser's native `drop` event
for the drop area. "Open in the SQL editor" appends the statement to the SQL
page's stored text for the deployment and navigates there, so the SQL page
needs no new input.

## Risks / Trade-offs

- [Uploads copy files on the host, needing their size again in free space]
  → Uploads are removed right after the import; the page states it.
- [The inference module drifts from Exasol's in a later Nano version] → The
  integration test compares them on the sample files.
- [`IMPORT FROM LOCAL PARQUET` is single-stream] → Acceptable for local
  files; per-file progress shows it moves.

## Open Questions

- Should a later change let users type a host path to skip the upload copy
  for very large files? The browser cannot reveal paths, so it would be a
  separate field.

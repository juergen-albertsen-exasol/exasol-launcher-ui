# data-import Specification

## Purpose

Import Parquet files from the user's computer into a new table of a
deployment's database: upload them, review and edit the table Exasol would
create, and follow the import to the table in the SQL editor.

## Requirements
### Requirement: The API stores uploaded Parquet files

The server SHALL answer `POST /api/v1/deployments/{name}/uploads` with the
names and sizes of up to 100 files with status 201, the upload's `id`, and
one URL per file. `PUT` on a file's URL with the file's bytes SHALL store it
on the host and respond with status 204. An upload SHALL be deleted once an
import of it finishes, after 24 hours without an import, and when the server
stops. A request for an unknown upload or deployment SHALL receive status
404.

#### Scenario: Uploading two files

- **WHEN** the client posts the names and sizes of
  `yellow_tripdata_2024-01.parquet` and `yellow_tripdata_2024-02.parquet` to
  `/api/v1/deployments/default/uploads` and puts each file to its URL
- **THEN** each `PUT` responds with status 204, and an import suggestion for
  the upload's `id` describes both files

#### Scenario: An upload is deleted after its import

- **WHEN** an import of an upload succeeds
- **THEN** an import suggestion for the same upload `id` receives status 404

### Requirement: The API suggests the table an import creates

The server SHALL answer `POST
/api/v1/deployments/{name}/import-suggestions` with a source, `{"kind":
"upload", "upload_id": …}`, with status 200 and the suggested table: the
schema `DATA`; a table name made from the file name without `.parquet`, or
from the common beginning of several files' names, in uppercase, with every
character other than letters, digits, and underscores replaced by an
underscore; the total row count; the number of files and bytes; and for each
top-level Parquet column its name exactly as in the file, its Parquet type,
the Exasol type that Exasol's schema inference for Parquet gives it, and a
sample value from the first row. The suggestion SHALL report problems with a
`code` and a message, and `importable` SHALL be false while it has any:
`unsupported_columns` naming every column of a nested type (struct, list,
map) or another type Exasol cannot import; and `table_taken` when the table
exists in the schema, with the table name followed by `_2`, or the next free
number, as `alternative`. Files whose columns differ in names, Parquet types,
or order SHALL receive status 422 with code `columns_differ` naming the files
that differ, and a file that is not Parquet SHALL receive status 422 with
code `not_parquet` naming it. A deployment whose database does not run SHALL
receive status 409.

#### Scenario: Types from Exasol's inference

- **WHEN** the client asks for the suggestion of an upload of the sample file
  `online_products.parquet`
- **THEN** the suggested columns have the names and types of the table that
  `CREATE TABLE … AS (IMPORT FROM PARQUET …)` creates for the same file in
  the deployment's database

#### Scenario: Integer and floating-point types

- **WHEN** the client asks for the suggestion of a file with columns of the
  Parquet types INT32, INT64, and DOUBLE
- **THEN** INT32 is suggested as `DECIMAL(10,0)`, INT64 as `DECIMAL(20,0)`,
  and DOUBLE as `DOUBLE PRECISION`

#### Scenario: A table name from several files

- **WHEN** the client asks for the suggestion of an upload of
  `yellow_tripdata_2024-01.parquet` and `yellow_tripdata_2024-02.parquet`
- **THEN** the suggested table is `DATA.YELLOW_TRIPDATA_2024`, and its row
  count is the sum of both files' rows

#### Scenario: A taken table name

- **WHEN** the client asks for the suggestion of `events.parquet` while
  `DATA.EVENTS` and `DATA.EVENTS_2` exist
- **THEN** the suggestion has the problem `table_taken` with the alternative
  `EVENTS_3`, and `importable` is false

#### Scenario: A nested column

- **WHEN** the client asks for the suggestion of a file with a struct column
  `payload`
- **THEN** the column `payload` has no Exasol type, the problem
  `unsupported_columns` names it and says that Exasol cannot import nested
  types, and `importable` is false

#### Scenario: Files with different columns

- **WHEN** the client asks for the suggestion of an upload of two files whose
  columns differ
- **THEN** the server responds with status 422 and code `columns_differ`, and
  the message names the file that differs from the first

### Requirement: The API imports uploaded files into a new table

The server SHALL answer `POST /api/v1/deployments/{name}/imports` with a
source, a schema, a table, and optionally the columns with names and Exasol
types with status 202 and the import with its `id` and the status `running`.
The import SHALL create the schema when it does not exist, create the table
with the given columns, or with the suggested ones when none are given, and
load every row of the uploaded files into it, as one transaction. `GET
/api/v1/deployments/{name}/imports/{id}` SHALL report the import's source,
schema and table, `status` (`running`, `succeeded`, `failed`, or
`cancelled`), the seconds it has taken, each file with its status and rows
once loaded, the total rows once it succeeds, the SQL it runs, and for a
failed import the database's message. `GET
/api/v1/deployments/{name}/imports` SHALL list the deployment's imports since
the server started, newest first. `DELETE` on an import's URL SHALL cancel a
running import. A failed or cancelled import SHALL leave neither the table
nor a schema it created behind. A table that exists, a source with problems,
or a database that does not run SHALL receive status 409 with the reason.

#### Scenario: Importing two uploaded files

- **WHEN** the client imports an upload of `yellow_tripdata_2024-01.parquet`
  and `yellow_tripdata_2024-02.parquet` into `DATA.YELLOW_TRIPDATA_2024` with
  the suggested columns
- **THEN** the import succeeds with 5,972,150 rows, and `SELECT COUNT(*) FROM
  DATA.YELLOW_TRIPDATA_2024` returns 5,972,150

#### Scenario: Changed names and types

- **WHEN** the client imports a file into `DATA.TRIPS` with the column
  `passenger_count` renamed to `PASSENGERS` and typed `DECIMAL(3,0)`
- **THEN** the table `DATA.TRIPS` has the column `PASSENGERS` of type
  `DECIMAL(3,0)` with the file's values

#### Scenario: A failing import

- **WHEN** the client imports a file with a column typed `DECIMAL(2,0)` whose
  values exceed 99
- **THEN** the import's status becomes `failed` with the database's message
  naming the column, and the table does not exist

#### Scenario: Cancelling an import

- **WHEN** the client deletes a running import
- **THEN** its status becomes `cancelled`, and the table does not exist

### Requirement: The Import data page chooses Parquet files

The Import data page of a deployment SHALL offer a drop area with "Choose
files" for Parquet files on this computer, as the canvas draws it
(`ImportData`, without the Amazon S3 and HTTPS URL tabs). Dropped or chosen
files SHALL upload with their progress shown, after which the page SHALL
show the suggested table. Files that cannot be read SHALL show the server's
message next to the drop area. Below it, the page SHALL list the deployment's
recent imports with the table, files, rows, seconds, and status, a failed
import's message, and "Query" for a succeeded one.

#### Scenario: Dropping files

- **WHEN** the user drops two Parquet files with the same columns on the drop
  area of the Import data page of `default`
- **THEN** the page shows each file's upload progress and then the suggested
  table for both files

#### Scenario: Files with different columns

- **WHEN** the user drops two Parquet files whose columns differ
- **THEN** the page names the file that differs and says that the files of
  one import need the same columns

### Requirement: The Import data page edits the suggested table

The page SHALL show the suggestion as the canvas draws it
(`ImportSuggestion`, `ImportProblems`): the files with their bytes and rows;
editable schema and table fields; and a table of the columns with editable
names and Exasol types, the Parquet type, and the sample. "Reset to the
suggestion" SHALL restore the suggested names and types. Problems SHALL
appear above the fields they concern, a taken table name with "Use
<alternative>", and columns Exasol cannot import marked "Cannot be imported";
while any problem remains, or the schema, the table, or a column name is
empty, "Import <rows> rows" SHALL be disabled.

#### Scenario: Choosing another table name

- **WHEN** the suggestion says that `DATA.EVENTS` exists and the user chooses
  "Use EVENTS_2"
- **THEN** the table field holds `EVENTS_2`, the problem disappears, and the
  import is enabled unless another problem remains

#### Scenario: A nested column blocks the import

- **WHEN** the suggestion has a struct column `payload`
- **THEN** the page names `payload` in a message about nested types, marks
  its row "Cannot be imported", and disables the import

### Requirement: The Import data page follows a running import

Starting an import SHALL show it as the canvas draws it (`ImportRunning`):
the target table, the seconds so far, each file with its status and rows,
the SQL it runs, and "Cancel import". The user SHALL be able to leave the page
and return to the running import. A succeeded import SHALL show the table, its
rows, and the seconds it took, as the canvas draws it (`ImportDone`), with
"Open in the SQL editor", which opens the deployment's SQL page with `SELECT *
FROM "<schema>"."<table>" LIMIT 100;` added on a new line after the editor's
text, and "Import more", which returns to the drop area. A failed import SHALL
show the database's message and return to the suggestion, keeping the user's
changes.

#### Scenario: Leaving and returning

- **WHEN** the user starts an import, opens the Overview page, and returns to
  the Import data page while the import runs
- **THEN** the page shows the running import with its progress

#### Scenario: Querying the new table

- **WHEN** an import into `DATA.YELLOW_TRIPDATA_2024` succeeds and the user
  chooses "Open in the SQL editor"
- **THEN** the SQL page of the deployment opens with `SELECT * FROM
  "DATA"."YELLOW_TRIPDATA_2024" LIMIT 100;` on the last line of the editor

### Requirement: The Import data page waits for a running database

While the deployment's database does not run, the Import data page SHALL say
that imports need a running database, offer Start, and disable choosing
files and importing; the recent imports SHALL stay listed.

#### Scenario: A stopped deployment

- **WHEN** the user opens the Import data page of a stopped deployment
- **THEN** the page offers Start, which starts the deployment as an operation,
  and "Choose files" is disabled


# sql-editor Specification

## Purpose

Run SQL against a deployment's database from its SQL page, with sessions,
cancellation, CSV export, and schema names served by the API, while the
database password stays on the host.

## Requirements
### Requirement: The API opens and closes SQL sessions

`POST /api/v1/deployments/{name}/sql/sessions` SHALL connect to the
deployment's database with the credentials stored in its deployment directory
and respond with status 201, the session's `id`, and its URL in the
`Location` header. `DELETE` on that URL SHALL close the session and respond
with status 204. A session SHALL keep its database state, such as the open
schema and an open transaction, from one statement to the next, and SHALL
close after 30 minutes without a request. A request for a closed or unknown
session SHALL receive status 404. When the database cannot be reached, opening
a session SHALL receive status 409 with the reason. An unknown deployment
SHALL receive status 404.

#### Scenario: Opening a session

- **WHEN** the client posts to `/api/v1/deployments/default/sql/sessions`
  while the database of `default` runs
- **THEN** the server responds with status 201 and a session id, and its
  `Location` header names the session's URL

#### Scenario: A session keeps its open schema

- **WHEN** the client runs `OPEN SCHEMA RETAIL` and then `SELECT * FROM
  SALES` in the same session
- **THEN** the second statement reads the table `RETAIL.SALES`

#### Scenario: A stopped database

- **WHEN** the client opens a session on a deployment whose database is
  stopped
- **THEN** the server responds with status 409 and a message that the
  database is not running

#### Scenario: A closed session

- **WHEN** the client runs a statement in a session it deleted before
- **THEN** the server responds with status 404

### Requirement: The API runs statements in a session

The server SHALL run the one statement in the body field `sql` of
`POST /api/v1/deployments/{name}/sql/sessions/{id}/statements` in the
session and respond with status 200 and the statement's outcome, its duration in milliseconds, and
the session's open schema after it ran, or null when none is open. A
statement that returns a result set SHALL report its columns with their names
and SQL types, the total number of rows, and the first 10,000 rows, each
value as text, or null for NULL, and SHALL say whether rows were left out. Any
other statement SHALL report the number of rows it affected. A statement that
fails SHALL receive status 422 with the database's error code and message
and, when the message names one, the line and column within the statement. A
request while another statement runs in the same session SHALL receive status
409. No response SHALL contain the database password.

#### Scenario: A query

- **WHEN** the client runs `SELECT REGION_NAME, COUNT(*) AS ORDERS FROM
  RETAIL.SALES GROUP BY 1` in a session
- **THEN** the server responds with status 200, the columns `REGION_NAME`
  with type `VARCHAR(100) UTF8` and `ORDERS` with type `DECIMAL(18,0)`, one
  row per region, the total row count, and the duration

#### Scenario: A large result

- **WHEN** the client runs a query that returns 2,964,624 rows
- **THEN** the response holds the first 10,000 rows, reports 2,964,624 as the
  total, and says that rows were left out

#### Scenario: A statement without a result set

- **WHEN** the client runs `DELETE FROM RETAIL.SALES WHERE AMOUNT < 0`, which
  deletes 12 rows
- **THEN** the server responds with status 200 and 12 affected rows

#### Scenario: A failing statement

- **WHEN** the client runs `SELECT REGOIN_NAME FROM RETAIL.REGIONS`
- **THEN** the server responds with status 422, the error code `42000`, the
  database's message that the object `REGOIN_NAME` was not found, and line 1
  and column 8

### Requirement: The API cancels a running statement

`POST /api/v1/deployments/{name}/sql/sessions/{id}/cancel` SHALL abort the
statement running in the session and respond with status 204; the request
that ran the statement SHALL then receive status 422 with the database's
message that the statement was cancelled. The session SHALL stay open.

#### Scenario: Cancelling a long query

- **WHEN** the client cancels a session while a statement runs in it
- **THEN** the server responds with status 204, the statement's request
  receives status 422 with the cancellation message, and the next statement
  in the session runs

### Requirement: The API exports a statement's result as CSV

The server SHALL answer `POST
/api/v1/deployments/{name}/sql/sessions/{id}/exports` with the body field
`sql` with status 201 and the export's URL in the
`Location` header. A `GET` of that URL within five minutes SHALL run the
statement with the session's open schema and respond with every row of its
result as `text/csv`, a header row of column names first, as an attachment
named after the deployment. The URL SHALL serve one download.

#### Scenario: Downloading a large result

- **WHEN** the client creates an export of a query that returns 2,964,624
  rows and requests its URL
- **THEN** the server responds with a CSV attachment of 2,964,625 lines

#### Scenario: An export URL serves one download

- **WHEN** the client requests an export's URL a second time
- **THEN** the server responds with status 404

### Requirement: The API reports a database's schemas

`GET /api/v1/deployments/{name}/schemas` SHALL return the database's schemas
by name, each saying whether it is a virtual schema and listing its tables and
views by name and kind. `GET /api/v1/deployments/{name}/schemas/{schema}`
SHALL return the schema's tables and views with their columns, each column
with its name and SQL type, in table order. An unknown schema SHALL receive
status 404, and a database that cannot be reached status 409 with the
reason.

#### Scenario: Listing schemas

- **WHEN** the client requests the schemas of `default`, which holds the
  schema `RETAIL` with the table `SALES` and the view `SALES_BY_DAY`
- **THEN** the response lists `RETAIL` with `SALES` as a table and
  `SALES_BY_DAY` as a view

#### Scenario: Reading a schema's columns

- **WHEN** the client requests the schema `RETAIL` of `default`
- **THEN** the response lists the columns of `RETAIL.REGIONS`, `REGION_ID`
  with type `DECIMAL(9,0)` followed by `REGION_NAME` with type `VARCHAR(100)
  UTF8`

### Requirement: The SQL page runs statements

The SQL page of a deployment SHALL offer an editor for Exasol SQL with
syntax highlighting, line numbers, and the toolbar actions Run, Run all, and
Format. Run, or Cmd/Ctrl+Enter, SHALL run the selected text, or the statement
at the cursor when nothing is selected. Run all, or Shift+Cmd/Ctrl+Enter,
SHALL run every statement of the editor in order, each with its own result
tab, and SHALL stop at the first statement that fails, marking the rest as
not run. Statements of one page SHALL share one session, and the toolbar
SHALL show the session's open schema. When the session has ended, the page
SHALL open a new one, say that the open schema and any open transaction
were reset, and run the statement there.

#### Scenario: Running the statement at the cursor

- **WHEN** the editor holds `OPEN SCHEMA RETAIL;` and a `SELECT` on the
  following lines, and the user presses Cmd+Enter with the cursor in the
  `SELECT`
- **THEN** the page runs only the `SELECT` and shows its result in one tab

#### Scenario: Running all statements

- **WHEN** the user selects Run all with `OPEN SCHEMA RETAIL;` followed by a
  `SELECT` in the editor
- **THEN** the page shows two result tabs, the toolbar shows `RETAIL` as the
  open schema, and the `SELECT` tab shows its rows

#### Scenario: Run all stops at a failure

- **WHEN** the user selects Run all on three statements of which the second
  fails
- **THEN** the first tab shows its result, the second its error, and the
  third that it was not run

#### Scenario: Formatting

- **WHEN** the user selects Format with `select a,b from t where a=1` in the
  editor
- **THEN** the editor holds the statement with upper-case keywords and its
  `FROM` and `WHERE` clauses on lines of their own

### Requirement: The SQL page shows results

A result tab SHALL show a statement's result set as a table with the column
names and types in its header, NULL values marked as such, numbers aligned
right, and the row count and duration below it, or the number of affected
rows for a statement without a result set. When rows were left out, the tab
SHALL say that it shows the first 10,000 of the total rows. Download CSV
SHALL save every row of the tab's result as a CSV file.

#### Scenario: A result table

- **WHEN** a query returns five rows, one of them with a NULL region
- **THEN** the tab shows five rows with the column types in the header, the
  NULL marked, and `5 rows` with the duration

#### Scenario: A truncated result

- **WHEN** a query returns 2,964,624 rows
- **THEN** the tab shows 10,000 rows and says that these are the first
  10,000 of 2,964,624, and Download CSV saves all 2,964,624 rows

### Requirement: The SQL page cancels running statements

While statements run, the toolbar SHALL show Cancel in place of Run and Run
all, which statement of how many runs, and for how long. The editor SHALL
mark the running statement, and the result tabs SHALL show which statements
finished, which runs, and which wait. Cancel SHALL abort the running
statement and leave the waiting ones not run.

#### Scenario: Cancelling Run all

- **WHEN** the user selects Cancel while the second of three statements runs
- **THEN** the first tab keeps its result, the second shows that it was
  cancelled, and the third that it was not run

### Requirement: The SQL page points at errors

A failed statement's tab SHALL show the database's error code and message
with a button that copies them. When the error names a position, the editor
SHALL underline it and mark its line, and the tab SHALL offer a button that
moves the cursor there, naming the line and column in the editor.

#### Scenario: Going to an error

- **WHEN** a statement starting on line 3 of the editor fails at line 1,
  column 10 of the statement
- **THEN** the editor underlines the word at line 3, column 10, and the tab's
  button "Go to line 3, column 10" moves the cursor there

### Requirement: The SQL page browses and completes schema names

The SQL page SHALL list the database's schemas, expanding a schema to its
tables and views, and a table or view to its columns with their types. A
filter SHALL narrow the list to the schemas, tables, and columns whose names
contain its text. Double-clicking a name SHALL insert it at the editor's
cursor. The editor SHALL complete schema names, the tables and views of a
schema after `SCHEMA.`, the columns of a table after `SCHEMA.TABLE.`, and the
columns of the tables that the statement at the cursor names. The list and
the completions SHALL reload after Reload and after a statement that creates,
changes, or drops an object.

#### Scenario: Completing a table name

- **WHEN** the user types `SELECT * FROM RETAIL.SA` in the editor
- **THEN** the editor offers the table `SALES` and the view `SALES_BY_DAY`

#### Scenario: A new table appears

- **WHEN** the user runs `CREATE TABLE RETAIL.RETURNS (ID INT)`
- **THEN** the schema list shows `RETURNS` under `RETAIL` without a reload

#### Scenario: Filtering

- **WHEN** the user types `region` into the filter
- **THEN** the list shows the `RETAIL` tables `REGIONS` and `SALES` with
  their columns whose names contain `REGION`

### Requirement: The SQL page keeps text and history in the browser

The SQL page SHALL keep the editor's text per deployment in the browser, so
it returns after a reload. Its History menu SHALL list the last 100
statements run on the deployment in this browser, newest first, each with its
outcome, when it ran, and its row count, duration, or error message. A search
field SHALL narrow the list to statements containing its text; choosing a
statement SHALL insert it at the editor's cursor, and Clear SHALL empty the
history.

#### Scenario: Text survives a reload

- **WHEN** the user types a statement on the SQL page of `default` and
  reloads the page
- **THEN** the editor holds the same text

#### Scenario: Reusing a statement

- **WHEN** the user opens History and chooses the `SELECT` that failed five
  minutes ago
- **THEN** the editor holds that statement at the cursor

### Requirement: The SQL page waits for a running database

When the deployment's database is not running, the SQL page SHALL say so and
offer Start, keep the editor's text, Format, and History available, and
disable Run and Run all; the schema list SHALL say that it is available once
the database runs. When the database runs again, the page SHALL enable
running statements and load the schemas.

#### Scenario: A stopped deployment

- **WHEN** the user opens the SQL page of a stopped deployment
- **THEN** the page shows the editor with its saved text, disabled Run and
  Run all, and Start, which starts the deployment as an operation


## Context

The server already connects to a deployment's database for the overview's
reachability check: `internal/database` reads the password from
`secrets.json`, takes host, port, and user from `exasol info --json`, and
opens a connection with `exasol-driver-go`. The SQL page needs the same
connection kept open across statements, and the password must stay on the
host, so the browser cannot connect to the database itself as fusion-cloud's
console does.

fusion-cloud's console (`~/dev/fusion-cloud/console`) has a working SQL
editor built on CodeMirror 6, whose framework-free modules port to Angular:
the Exasol dialect (`lib/sql/dialect.ts`), the statement splitter with error
positions (`splitter.ts`), name completion (`completion.ts`), the formatter
(`format.ts`), and history storage (`storage.ts`). Its React components show
how the editor, the result grid, and the schema browser fit together.

`exasol-driver-go` v1.2.0 fetches result rows lazily in chunks. Through
`database/sql`, `QueryContext` on a statement without a result set loses the
row count, and `ExecContext` cannot return rows. Its exported
`connection.Connection.SimpleExec` returns the raw response with the result
type, and `Connection.Send` sends further commands such as `fetch`. The
driver's own row decoding turns every number into `float64`. When a context
is cancelled, the driver sends `abortQuery` but returns without reading the
statement's response, which a goroutine then reads concurrently with the
connection's next request.

The canvas draws the page in five artboards: `Sql`, `SqlRunning`, `SqlError`,
`SqlHistory`, and `SqlStopped`.

## Goals / Non-Goals

**Goals:**

- The SQL page as the canvas draws it, with CodeMirror 6.
- SQL sessions, statements, export, and metadata as API resources the
  Administration API could adopt.
- The database password never reaches the browser.

**Non-Goals:**

- Several editor tabs or saved scripts per deployment; one editor per page.
- Editing result cells, charts, or explain plans.
- Statements with parameters, and running SQL as another user than the one
  the deployment stores.
- A copyable exapump export command: it would need the password in the
  browser. Download CSV covers the full result instead.
- Health views from `EXA_STATISTICS`; a later change.

## Decisions

**Sessions on the server.** A new `internal/sql` package holds sessions: each
pins one `*sql.Conn` from a `*sql.DB` per deployment, opened with the stored
credentials, and records its last use. A janitor closes sessions idle for 30
minutes, and the server closes all on shutdown. Session ids are random 128-bit
values. One statement runs per session at a time; a second request gets 409.
The browser opens one session per page visit, closes it with `DELETE` (with
`keepalive`) when the page goes away, and opens a new one when a request gets
404. Alternative: one connection per statement, rejected because `OPEN
SCHEMA`, session parameters, and transactions would not carry over.

**Running a statement.** The session calls `SimpleExec` through
`sql.Conn.Raw` and switches on the response's result type. For a result set it
decodes the column-major data itself, keeping each value's JSON text, fetches
further chunks with `Send`, reads up to 10,000 rows, closes the result set, and
reports the `numRows` the database announced as the total. Column types come from the
result's metadata, formatted as Exasol writes them (`DECIMAL(18,0)`,
`VARCHAR(100) UTF8`, `TIMESTAMP`). Every value is sent as text in the
database's own representation, so large decimals and timestamps keep their
precision, and NULL as JSON null. After each statement the session reads
`CURRENT_SCHEMA` for the toolbar. Alternative: `QueryContext` for everything,
rejected because it reports no row count for DML.

Response: `{type: "result_set", columns: [{name, type}], rows: [[...]],
total_rows, truncated, duration_ms, schema}` or `{type: "row_count",
row_count, duration_ms, schema}`.

**Errors.** A failed statement answers 422 as an RFC 9457 problem with the
extension fields `code`, `line`, and `column`, the latter two parsed from the
message's `[line L, column C]` when present. The message passes through
`database.Redact`. The browser maps the statement-relative position to the
editor with the splitter's offsets, as fusion-cloud does.

**Cancellation.** A session reads its database session id with `SELECT
CURRENT_SESSION` when it opens. `POST .../cancel` runs `KILL STATEMENT IN
SESSION <id>` on a short connection from the deployment's pool; the database
aborts the statement with `R0003 Client requested execution abort.`, which the
statement's request answers as 422, and the session stays usable. Statements
ignore the cancellation of their request. Alternatives: cancelling the
statement's context, rejected because the driver then leaves the connection
out of step (see Context); aborting the browser's request, rejected because a
closed request is not observable at the API boundary and a proxy or retry
could leave the statement running.

**CSV export.** `POST .../exports` stores the statement and the session's open
schema under a random id for five minutes and answers 201 with its URL. `GET`
of the URL, which the browser opens as a download so the cookie applies and
nothing is buffered in the page, consumes the entry, opens a separate
connection, runs `OPEN SCHEMA` when one was open, and streams the rows as RFC
4180 CSV with `Content-Disposition: attachment;
filename="<deployment>-result.csv"`. It runs on its own connection so the
editor stays usable during a long download. The export reruns the statement,
so the page offers Download CSV only for result sets.

**Metadata.** `GET /schemas` reads `EXA_ALL_SCHEMAS` (including
`SCHEMA_IS_VIRTUAL`), `EXA_ALL_TABLES`, and `EXA_ALL_VIEWS`;
`GET /schemas/{schema}` reads `EXA_ALL_COLUMNS` ordered by table and
`COLUMN_ORDINAL_POSITION`. Both run on short connections from the
deployment's pool, not the editor's session, so they answer while a
statement runs. They live under the deployment rather than under `/sql`, as
the Administration API would place a database's schemas.

**Database target.** Sessions, exports, and metadata resolve the deployment
and its connection through the same path as the reachability check: the
deployment listing's `status` decides whether the database runs, and `info`
gives host, port, and user. The `*sql.DB` per deployment is dropped when its
connection details or password change.

**Browser module.** `web/src/app/sql/` ports fusion-cloud's dialect, splitter,
completion, formatter, and storage with their tests, switching the storage
keys to `exasol-local-ui:sql:<deployment>:text|history`. A
`SqlEditorComponent` wraps CodeMirror 6 (`codemirror`, `@codemirror/state`,
`view`, `commands`, `language`, `lang-sql`, `autocomplete`, `search`, and
`@lezer/highlight`), styled with the design system's tokens so it follows the
light and dark themes, and exposes text, selection, cursor, insert, and error
marks. CodeMirror needs no workers, unlike Monaco, and fusion-cloud's code
already targets it.

**State and page.** A SQL NgXs state per deployment holds the session id, the
catalog (schemas, and columns per loaded schema), the result tabs, and the
run in progress. The page (route segment `sql`, drawer entry SQL with
`fa-terminal` after Overview) lays out the schema panel beside the toolbar,
editor, and result tabs as the canvas draws them, and the result grid uses
AG Grid, which the app already depends on, for virtual scrolling over 10,000
rows. Run all runs the statements one request at a time and stops at the
first 422. The page reloads the catalog after statements whose first keyword
is `CREATE`, `ALTER`, `DROP`, `RENAME`, or `OPEN`. When the deployment's
status is not running, the page shows the canvas's `SqlStopped` state and
starts the deployment through the existing start operation.

## Risks / Trade-offs

- [`SimpleExec`, `Send`, and the response types are exported driver
  internals that a driver update could change] → The driver version is pinned in `go.mod`, the calls
  sit in one small adapter behind an interface that the session's unit tests
  fake, and a test tagged `integration` runs both result types against a
  local deployment.
- [The editor runs as the administrative user and can drop anything] →
  Same trust as `exasol connect`: the server binds to loopback and requires
  the session token. The page shows which deployment it targets in the
  header.
- [An export reruns its statement] → Offered only for result sets; a query
  with side effects, such as one calling a script, runs twice.
- [Idle sessions hold database connections] → They close after 30 minutes,
  and the page closes its session when it goes away.
- [A restart of the database ends every session] → The page opens a new
  session on the next run and says that the session state was reset.

## Open Questions

- Should the row limit be configurable per run? The canvas uses a fixed
  10,000.

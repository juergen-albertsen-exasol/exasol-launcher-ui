## Context

The server reads launcher state through `exasol deployments list --json` and
`exasol info --json`. Lifecycle commands differ: `exasol start --json` and
`exasol stop --json` run for seconds to minutes, write progress to stderr, and
print exactly one JSON document with `deploymentState` and `databaseReady` to
stdout when they finish (the launcher's `lifecycle-json-output` contract). The
launcher locks a deployment directory while it works on it, so a second
command on the same deployment fails. Later changes add more long commands
(`deploy`, `destroy`, `install`, `slc install`, `sidecar enable`), which should
reuse what this change builds.

## Goals / Non-Goals

**Goals:**

- One runner for every long launcher command, with progress, result, and errors.
- Start and stop from the overview, as the design canvas draws them.
- Operations stay visible from every page while they run, and for a while after.

**Non-Goals:**

- Cancelling a running operation; the launcher offers no safe interruption.
- Keeping operations across server restarts.
- The other long commands; they arrive with their own changes.

## Decisions

**An in-memory operation runner.** A new `internal/operations` package keeps
operations in memory: id, deployment, kind (`start`, `stop`), status
(`running`, `succeeded`, `failed`), start and end time, the stderr lines, the
parsed stdout result, and an error message. It starts each command with a
context detached from the HTTP request and cancelled at server shutdown. It
refuses a second operation on a deployment that already has one running, which
the API reports as `409 Conflict`; the launcher's own lock stays the final
guard. It keeps the 50 most recent finished operations. Alternative: running
the launcher in the request, rejected because start waits up to 30 minutes.

**Streaming through the launcher client.** The launcher client gains a method
that runs a command, calls back for each stderr line, and decodes stdout as
JSON at the end, so the runner stays independent of process handling. Lines
pass through the existing password redaction with the deployment's stored
password before they are kept. When a command fails, its last stderr line
becomes the operation's error. The launcher writes JSON log records when its
stderr is no terminal; the client turns each into "message key=value", with
the level in front unless it is INFO.

**API shape.** `POST /api/v1/deployments/{name}/start` and `/stop` answer
`202 Accepted` with the operation and a `Location` header. `GET
/api/v1/operations` lists running and recent operations, newest first; `GET
/api/v1/operations/{id}` adds the output lines; `GET
/api/v1/operations/{id}/events` is a server-sent events stream that replays the
output so far, then sends each new line (`line` events) and a final `status`
event. Server-sent events fit a one-way log, work through the existing cookie
and Host checks, and need no extra library. Finishing an operation drops the
cached deployment listing, so the next read shows the new status.

**Browser state.** An NgXs operations state polls `GET /api/v1/operations`
every two seconds while any operation runs and every ten seconds otherwise, so
operations started in another tab appear. The overview follows its
deployment's running operation through the events stream and refreshes the
deployment and its reachability when it ends.

**Where things show.** Start (filled) shows when the listing status is
`stopped`, Stop (outlined) beside Check again when it is `running`,
`database_ready`, or `database_connection_failed`. The listing status checks
the runtime, while `info` can still report a stored `running` for a deployment
whose runtime is down. While an operation runs, the
overview card shows `operation_in_progress` and a loading button, and the
operation log replaces the detail panel. The activity indicator sits before
the deployment select in the layout header, and on the right of the start
page's green bar in white; it shows the running operation's name, or their
count when several run. Its panel lists running operations with elapsed time
and a link to their overview, and finished ones with their outcome and age.

## Risks / Trade-offs

- [The server stops during an operation] → The launcher process is cancelled
  with it, which can leave an `interrupted` deployment; the launcher's status
  reports that, and the overview shows it.
- [Launcher output contains secrets] → Lines pass through password redaction;
  the launcher's credential-confidentiality contract keeps passwords out of its
  output in the first place.
- [Polling cost] → The list is in memory and small; the ten-second idle poll is
  negligible.

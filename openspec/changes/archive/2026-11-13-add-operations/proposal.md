## Why

Most launcher capabilities the UI is to offer (start, stop, install, applying settings, destroy, script languages, sidecars) run for seconds to many minutes. Today the UI can only read state. It needs one way to run such operations in the background, show their progress, and keep them visible while people move between pages. Start and stop are the first and most used of them.

## What Changes

- Add a server-side operation runner: it runs a launcher command in the background, one at a time per deployment, keeps the launcher's progress output, and records the command's result.
- Add `POST /api/v1/deployments/{name}/start` and `.../stop`, answering `202 Accepted` with the new operation, and `409` while another operation runs on that deployment.
- Add `GET /api/v1/operations`, `GET /api/v1/operations/{id}`, and a stream of an operation's progress output.
- Show Start on the overview of a stopped deployment and Stop on a running one; while either runs, the overview shows the operation's progress log, and it refreshes when the operation ends.
- Show an activity indicator in the header of every page while operations run, with a panel of running and recently finished operations.

## Capabilities

### New Capabilities

- `operations`: running launcher operations in the background, the operations API, and following an operation's progress.

### Modified Capabilities

- `deployment-overview`: the overview starts and stops its deployment and shows the running operation's progress.
- `app-shell`: every page shows running and recent operations from its header.

## Impact

- New Go package for the operation runner; the launcher client gains a way to run a command while streaming its stderr.
- New HTTP endpoints under `/api/v1`, including a server-sent events stream.
- New NgXs state for operations, the activity indicator and panel in the shell and on the start page, and the overview's start, stop, and log.
- Screens follow the Exasol Local canvas (Overview, Overview: stopped, Overview: starting, Activity panel, Deployments) and the Exasol Admin UI design system (outline Button, Operation log, Activity).

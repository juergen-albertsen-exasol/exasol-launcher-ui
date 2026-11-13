## Context

The launcher manages a deployment's sidecars with `sidecar list`, `sidecar
status`, `sidecar enable <name>`, and `sidecar disable <name> [--remove]`, all
with `--json`. `exasol mcp enable|disable|status` are the same commands for
the sidecar named `mcp`, and `exasol mcp instructions [client] --json` reports
the AI clients with setup instructions, or for one client the endpoint URL with
either a `command` or a configuration `file` and its `content`.

The two listings differ. `sidecar list` reports the built-in catalog only
(`name`, `description`, `supported`, `enabled`, and an `unsupported` reason).
`sidecar status` reports every sidecar saved in the deployment's
`sidecars.yaml`, catalog or user-defined, with `enabled`, `running`, `state`,
`health` (`serving`, `starting`, `failing`, or empty without a readiness
probe), `healthError`, `error`, `lastOperationError`, `restartRequired`,
`reconciliation`, and `endpoints` (`ip`, `port`, optional `url`). On a stopped
deployment status reports enabled sidecars as not running; on a cloud
deployment it fails with "sidecars are not supported for this deployment
type".

Enabling waits until the sidecar serves, which can take minutes when its image
is pulled; disabling removes containers. On a stopped or initialized
deployment both only record the change for the next start. The launcher's
directory lock breaks under concurrent starts
([exasol-personal#515](https://github.com/exasol/exasol-personal/issues/515)),
so the launcher client runs short commands one at a time.

## Goals / Non-Goals

**Goals:**

- The Sidecars and MCP server pages as the canvas draws them (`Sidecars`,
  `MCP`).
- Sidecar endpoints shaped so the Administration API could adopt them.
- Enable and disable as operations on the existing runner.

**Non-Goals:**

- Editing sidecar definitions (`sidecar config`), ports, or adding
  user-defined sidecars; `sidecars.yaml` and the launcher stay the way to do
  that.
- Changing an AI client's configuration; the page shows what to run or paste.
- Sidecar descriptions for user-defined sidecars, which the launcher does not
  report.

## Decisions

**Sidecar resource.** `GET /api/v1/deployments/{name}/sidecars` returns an
array in which each sidecar is `{name, description?, catalog, enabled, running,
state, health, health_error?, error?, last_operation_error?, restart_required,
endpoints: [{ip, port, url?}]}`. It runs `sidecar status --json`, then
`sidecar list --json`, and appends catalog sidecars without a saved definition
as disabled entries, adding the catalog description to saved catalog sidecars.
Catalog entries the host does not support are left out. Status and listing
order are kept: saved sidecars first, then the rest of the catalog. A launcher
failure of `sidecar status` answers 409 with the launcher's message, which is
how a cloud deployment reports that it runs no sidecars. Alternative: one call
per command as separate resources, rejected because every page needs both and
the merge is the same everywhere.

**MCP is a sidecar.** The MCP server page reads the `mcp` entry of the sidecar
resource and enables and disables it through the sidecar endpoints, since the
launcher defines `exasol mcp` as those commands. Only the client instructions
get their own resources: `GET .../mcp/clients` (`[{id, name}]`) and `GET
.../mcp/clients/{client}` (`{id, name, url, command?, file?, content?}`). An
unknown client id answers 404; a launcher failure, such as the MCP server not
being enabled, answers 409 with its message.

**Names reach the launcher only when listed.** A sidecar name must appear in
the merged listing and a client id in the client listing before it becomes a
launcher argument; anything else answers 404. This keeps flags out of the
command line.

**Sidecar operations.** Three kinds join the runner: `enable_sidecar`
(`sidecar enable <name>`), `disable_sidecar` (`sidecar disable <name>`), and
`remove_sidecar` (`sidecar disable <name> --remove`). They print no JSON the
runner decodes and succeed on exit status 0, like `deploy`. Operations gain an
optional `sidecar` field naming their sidecar. `POST
.../sidecars/{sidecar}/enable` and `POST .../sidecars/{sidecar}/disable` with an
optional `{"remove_definition": true}` start them; the one-operation-per-
deployment rule applies, so a sidecar operation waits for a start to finish.
Labels read "Enabling mcp on default", "Disabled kafka on default", "Removing
kafka from default".

**Sidecars page.** Route segment `sidecars`, drawer entry Sidecars with
`fa-cubes`, between Overview and Settings, followed by MCP server
(`mcp`, `fa-robot`). One info box "Sidecars of <name>" holds the canvas's
table: Sidecar, Description, State, Health, Endpoint, and an action. State
shows `disabled` for disabled sidecars and the launcher's `state` otherwise,
with the running dot when running. Health shows healthy, starting, or failing
(with the health error below the row), and nothing when empty. The endpoint
is a link when it has a URL, else `ip:port`. A sidecar's `error` or
`last_operation_error` appears below its row. Enabled sidecars offer Disable…,
which opens a confirmation dialog with the checkbox "Also delete the saved
definition"; disabled ones offer Enable. When the deployment is not running, a
note says changes take effect at the next start. When the resource answers
409, the page shows the launcher's message in place of the table.

**MCP server page.** The canvas's item card (robot icon, state, title, the
catalog description, health, and Enable or Disable…), then a detail panel with
the Endpoint box (URL with `exa-clipboard-copy`, the health error below) and the
Connect a client box: a client select defaulting to the first client, the
command or the named file with its content in a copyable code block, and the
canvas's note that the client's configuration stays unchanged. Without an
enabled MCP server the panel says to enable it, and the card offers Enable.
Without an `mcp` entry, the page shows the 409 message as the Sidecars page
does.

**State and refresh.** A sidecars NgXs state keeps the listing and load error
per deployment, and the client list and selected client's instructions for the
MCP page. Pages load on entry and when an operation on their deployment ends,
and reload every 3 seconds while any listed sidecar has health `starting`.
While an operation runs on the deployment, the pages disable their actions and
show the operation's progress output above the table or panel, using the
shared progress component; a failed operation shows its error like the
Settings page.

## Risks / Trade-offs

- [Two launcher calls per listing, run one at a time] → About a second per
  page load; the pages refresh only on entry, after operations, and while a
  sidecar starts.
- [Status output can name a sidecar the catalog also lists under another
  description] → Saved entries win, and the description comes from the
  catalog by name.
- [A long sidecar operation can still race a short command at its start] →
  Accepted with the launcher client's serialization; the launcher issue tracks
  the real fix.
- [Enabling on a stopped deployment reports success without anything running]
  → The page's note says changes take effect at the next start.

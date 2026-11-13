## Context

The launcher manages a deployment's configuration with `config get --json`,
`config set --<option>=<value>`, and `config reset <option>... | --all`. The
options come from the deployment's presets: `local` has `cpu-count`,
`data-size-gb`, `memory-mb`, and `ports`; `aws` with `ubuntu` has eight
infrastructure and two installation options, including `db-password` and
`adminui-password`. `config get --json` reports each preset's identity
(`name`, `displayName`, `description`) and an `options` object of effective
values typed as number, boolean, or string. It reports no option descriptions,
defaults, or types beyond the JSON value.

The launcher allows `config set` and `config reset` only on `initialized`
deployments and on `stopped` local ones. Changes to a stopped local deployment
take effect at the next `start`; those to an initialized deployment at
`deploy`. `deploy`, `destroy`, and `remove` print no JSON and are judged by
their exit status; without a terminal they approve their own prompts.

`add-operations` built an in-memory runner for `start` and `stop`, which always
passes `--json` and decodes a lifecycle result. The overview already shows a
running operation's log through the events stream.

## Goals / Non-Goals

**Goals:**

- The Settings page and its dialogs as the canvas draws them
  (`Configuration`, `SettingsReadOnly`, `SettingsInitialized`,
  `DestroyDialog`, `RemoveDialog`).
- Configuration endpoints shaped so the Administration API could adopt them.
- Deploy, destroy, and remove as operations on the existing runner.

**Non-Goals:**

- Changing password options from the UI. Passing them as `config set`
  arguments would expose them in the process list, and their values must not
  reach the browser; the launcher remains the way to set them.
- Switching presets; the launcher requires destroying and reinitializing.
- Option descriptions and defaults, which the launcher only prints as help
  text.
- Remembering unapplied changes across reloads or server restarts.

## Decisions

**Configuration resource.** `GET /api/v1/deployments/{name}/configuration`
returns `{infrastructure, installation, editable, apply_with}`. Each preset is
`{name, display_name, description, options: [{name, value, secret}]}`, options
in the launcher's order. An option is `secret` when its name contains
`password`; its `value` is then omitted, whatever the launcher reports.
`editable` and `apply_with` (`start`, `deploy`, or absent) derive from the
listing's `status` and infrastructure preset, which check the runtime, rather
than from `info`. `PATCH` takes `{"options": {"cpu-count": 4}}`; option names
are unique across a deployment's presets because the launcher takes them all
as flat flags. Reset is `POST .../configuration/reset` with
`{"options": ["cpu-count"]}`, a command rather than a field update, like the
Administration API's actions. Alternative: one `PUT` with every value,
rejected because `config set` patches and would rewrite untouched options.

**Running config commands.** `config set` and `config reset` finish in about a
second, so the request runs them directly through a new launcher client method
that returns the command's stderr as its error, and then answers with a fresh
`config get --json`. The server checks `editable` first and answers 409; any
launcher failure after that is the launcher rejecting the input and becomes 400
with its message. Secret option names are refused before running the launcher,
so no password enters a command line from here.

**Operations without JSON output.** The runner's kinds carry their launcher
arguments: `start` and `stop` as before; `deploy`; `destroy --auto-approve`;
`destroy --remove --auto-approve` as `destroy_and_remove`; and
`remove --auto-approve`. Only `start` and `stop` pass `--json` and decode a
result; the others succeed on exit status 0. Finishing any operation already
drops the cached listing, so a removed deployment disappears from the next
listing.

**Endpoints.** `POST .../deploy`, `POST .../destroy` with an optional
`{"remove_folder": true}`, and `POST .../remove` reuse the start and stop
handler. The typed-name confirmation is the browser's; the server's session
token and origin checks already keep other sites from reaching these
endpoints.

**Settings page.** A child route `settings` of the deployment shell and a
second drawer entry, Settings with `fa-sliders`; the header title is the
route's. The page shows the canvas's Presets info box, one info box per preset
with options as a grid of fields labelled with the option name, an input per
JSON type (number input, checkbox, text), and the box's Save and Reset to
defaults…; a boolean option is a checkbox labelled Enabled under its name. A
secret option is a disabled password field reading Hidden, with the hint "Set
it with the launcher". Save sends only changed values. Reset to defaults resets the
options of that box after a confirmation. A preset without options shows "The
<name> installation preset has no options." When not editable, the fields are
disabled, Save and Reset to defaults are hidden, and an info line says "Stop
the deployment to change its settings. You can stop it on the Overview page."
for a local deployment, or "Destroy the deployment to change its settings."
otherwise.

**Apply banner.** A settings NgXs state keeps, per deployment, whether the user
saved or reset since the page last saw an operation finish on it. The banner
from the canvas shows with Apply changes while that is set and `apply_with` is
`start`, and always, with Deploy and the text "This deployment has no
resources yet. Deploy creates them with these settings.", when `apply_with` is
`deploy`.

**Danger zone and dialog.** The canvas's three rows. One confirmation dialog
serves all three, with the canvas's look: red frame, warning title "Destroy
<name>?" or "Remove the folder of <name>?", the launcher's warning text, the
folder path when it is removed, the checkbox in the destroy variant, and a
"Type <name> to confirm" field that enables the red button on an exact match.
The danger zone is hidden for deployments the listing reports as
`not_initialized`, since there is nothing to destroy, except Remove folder.

**Following operations.** Like the overview, the page subscribes to its
deployment's running operation and shows the operation log in place of the
info boxes and danger zone. When it ends, the page reloads the configuration
and the deployment, clears the unapplied flag on success, and shows the error
on failure. A succeeded `destroy_and_remove` or `remove` refreshes the listing
and navigates to the Deployments page.

## Risks / Trade-offs

- [Option labels are raw names like `cpu-count`] → They match the launcher's
  flags and documentation; friendlier labels need the launcher to report
  option metadata.
- [A preset adds a secret option whose name lacks `password`] → Its value
  would reach the browser. The current presets name every secret so; the
  launcher reporting secrets explicitly would remove the heuristic.
- [The unapplied-changes banner is lost on reload] → Starting the deployment
  from the overview applies the changes all the same.
- [Destroy is irreversible] → Typed-name confirmation, the launcher's warning
  text, and the danger zone's red styling; the server runs exactly the command
  the dialog names.

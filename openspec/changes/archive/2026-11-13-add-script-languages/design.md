## Context

The launcher manages a local deployment's script language containers (SLCs)
with `exasol slc`:

- `slc list --json` reports the catalog's official containers
  (`type: "official"`, `language`, `flavor`, `version`, `aliases`,
  `installed`) followed by installed custom containers (`type: "custom"`,
  `language`, `alias`, `source`, `available`). It also answers for cloud
  deployments, where every other `slc` command fails with "script language
  container management is only supported for local deployments".
- `slc install|update <alias-or-flavor>` and `slc remove <alias>` act on
  official containers; `remove` also removes custom ones.
- `slc install rust` and `slc update rust` install the latest
  `exasol-labs/language-container-rs` release as a custom container under the
  fixed alias `RUST`; the catalog does not list it.
- `slc custom install|update --alias A --language L --source S` installs or
  replaces a custom container from a tarball path or an HTTPS URL.

Install, update, and remove restart a running database to apply the change.
Without a terminal the launcher refuses that unless it gets `--auto-approve`
(restart now) or `--no-restart` (apply on the next start). The commands take
minutes when an image is pulled. On an initialized deployment they fail with
"deployment is not present".

## Goals / Non-Goals

**Goals:**

- The Script languages page as the canvas draws it (`ScriptLanguages`).
- SLC endpoints shaped so the Administration API could adopt them.
- All changes as operations on the existing runner.

**Non-Goals:**

- Uploading a container tarball through the browser; the source is a path on
  the host or an HTTPS URL, as for the launcher.
- Choosing a specific catalog version or a Rust release; the launcher
  installs what the catalog or the latest release resolves to.
- SQL to check that a language works; the launcher verifies activation.

## Decisions

**Script language resource.** `GET /api/v1/deployments/{name}/script-languages`
returns `{official: [...], custom: [...]}`. An official entry is `{alias,
language, flavor, version?, aliases, installed, latest_release}`, where
`alias` is the first catalog alias and the identifier the other endpoints
take. The server appends the Rust entry `{alias: "RUST", language: "rust",
flavor: "", aliases: ["RUST"], installed, latest_release: true}`, installed
when a custom container with alias `RUST` exists, which then leaves the custom
list. A custom entry is `{alias, language, source, available}`. The server
answers 409 with the launcher's wording for deployments whose infrastructure
is not `local`, without running the launcher, since `slc list` succeeds there
but nothing can change. Alternative: report Rust as a custom container,
rejected because the canvas draws it with the official ones and its Install
and Update run `slc install rust`.

**Operations with parameters.** Operations so far took a kind and an optional
sidecar. A kind now builds its command from parameters: the sidecar name, or
for SLCs the alias, language, source, and restart choice. New kinds:
`install_script_language` (`slc install <alias>`), `update_script_language`
(`slc update <alias>`), `remove_script_language` (`slc remove <alias>`),
`install_custom_script_language` and `update_custom_script_language` (`slc
custom install|update --alias=A --language=L --source=S`). Each adds
`--auto-approve` to restart or `--no-restart`, and succeeds on exit status 0.
Operations gain an optional `script_language` field naming the alias. Labels
read "Installing PYTHON3 on default", "Removed MYPY3 from default".

**Endpoints.** `POST .../script-languages/{alias}/install|update|remove` with
an optional `{"restart": false}` (default true), and `POST
.../script-languages/custom/install|update` with `{alias, language, source,
restart?}`. Install and update take an alias of the official list (Rust
included); remove takes any listed alias, official and installed or custom;
anything else answers 404 before the launcher runs. Custom fields must be
non-empty (400 otherwise) and reach the launcher as `--flag=value`, so no
value becomes a flag; the launcher validates them and its refusal fails the
operation. The one-operation-per-deployment rule applies.

**Script languages page.** Route segment `script-languages`, drawer entry
Script languages with `fa-code`, between MCP server and Settings. One detail
panel holds, as on the revised canvas: the checkbox "Restart the database now
(otherwise changes apply on the next start)", checked by default and used by
every action on the page; the info box "Official script language containers"
with Language, Flavor, Version, Aliases, State (installed with the running
dot, or not installed), and Update plus Remove… for installed entries or
Install otherwise; the info box "Custom script language containers" with a
table of installed ones (Alias, Language, Source, State: installed, or
unavailable when `available` is false, and Remove…) above a form with Alias,
Language, and Container tarball, and the buttons Update existing and Install;
then the canvas's note. Rust shows "latest exasol-labs release" as its
flavor. Remove… asks for confirmation with `ConfirmationModalComponent`,
naming whether the database restarts. Non-local deployments show the 409
message in place of the panel.

**State and refresh.** A script languages NgXs state keeps the listing and
load error per deployment. The page loads on entry and when an operation on
its deployment ends; the custom form clears after its operation succeeds.
While an operation runs on the deployment, the page disables its actions and
shows the shared progress output above the panel; a failed operation shows
its error like the other pages.

## Risks / Trade-offs

- [Restarting drops open database connections] → The restart option is
  visible above every action, and Remove… names the restart in its
  confirmation.
- [A source path is read by the launcher on the host] → Same trust as the
  launcher itself: the server binds to loopback and requires the session.
- [An update with nothing new succeeds without a visible change] → Accepted;
  the launcher's result goes to its standard output, which operations do not
  show, and the listing stays the same.
- [Operations on an initialized deployment fail] → The launcher's message
  appears as the operation's error.

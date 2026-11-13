## Why

Changing a deployment's size, ports, or cloud options, applying those changes, and tearing a deployment down all need the launcher CLI today. These are the first launcher capabilities beyond start and stop that people reach for, and the operations runner from `add-operations` now lets the UI run the long ones.

## What Changes

- Add `GET /api/v1/deployments/{name}/configuration`, reporting the deployment's presets and the effective values of their options (`config get --json`), with password options withheld, and whether the settings can be changed in the deployment's current status.
- Add `PATCH .../configuration` (`config set`) and `POST .../configuration/reset` (`config reset`), answering with the new configuration.
- Add `POST .../deploy`, `POST .../destroy` (optionally removing the folder), and `POST .../remove` as operations on the existing runner.
- Add a Settings page per deployment, reached from the navigation drawer: preset descriptions, editable option fields with Save and Reset to defaults, a banner offering to apply saved changes (Start for a stopped local deployment, Deploy for an initialized one), and a danger zone whose Destroy, Destroy and remove, and Remove folder actions are each confirmed by typing the deployment name.
- While an operation runs on the deployment, the Settings page shows its progress output; after a removal it opens the Deployments page.

## Capabilities

### New Capabilities

- `deployment-settings`: reading and changing a deployment's configuration through the API, and the Settings page with its apply banner and danger zone.

### Modified Capabilities

- `operations`: deploy, destroy, and remove run as operations.
- `app-shell`: the navigation drawer lists the Settings page after the Overview page.

## Impact

- Launcher client: `config get --json`, and running `config set` and `config reset` with their error output.
- Operation runner: kinds whose launcher commands print no JSON (`deploy`, `destroy`, `remove`).
- New `/api/v1` endpoints, a settings NgXs state, a Settings page, and a typed-name confirmation dialog.
- Screens follow the Exasol Local canvas (`Configuration`, `DestroyDialog`) and the Exasol Admin UI design system.

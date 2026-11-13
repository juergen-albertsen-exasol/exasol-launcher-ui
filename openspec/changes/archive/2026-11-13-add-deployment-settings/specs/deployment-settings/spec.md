## ADDED Requirements

### Requirement: The API reports a deployment's configuration

`GET /api/v1/deployments/{name}/configuration` SHALL return the deployment's
infrastructure and installation presets, each with its name, display name,
description, and options, as `exasol config get --json` reports them. Each
option SHALL carry its name and effective value; an option whose name contains
`password` SHALL carry neither its value nor whether one is set. The response
SHALL state whether the settings are `editable`, which is the case for an
`initialized` deployment and for a `stopped` deployment with the `local`
infrastructure preset, and which operation applies saved changes: `start` for
a `stopped` deployment and `deploy` for an `initialized` one. An unknown name
SHALL receive status 404.

#### Scenario: A stopped local deployment is editable

- **WHEN** the client requests the configuration of the stopped local
  deployment `default`
- **THEN** the response lists the `local` presets with the options
  `cpu-count`, `data-size-gb`, `memory-mb`, and `ports` and their values, is
  `editable`, and names `start` as the applying operation

#### Scenario: A running deployment is read-only

- **WHEN** the client requests the configuration of a running deployment
- **THEN** the response lists its presets and options and is not `editable`

#### Scenario: Password options are withheld

- **WHEN** the client requests the configuration of a deployment whose preset
  has the option `db-password` set
- **THEN** the response lists `db-password` without a value, and the response
  text does not contain the password

### Requirement: The API changes a deployment's configuration

`PATCH /api/v1/deployments/{name}/configuration` SHALL set the given option
values with `exasol config set` and `POST
/api/v1/deployments/{name}/configuration/reset` SHALL reset the given options
to their preset defaults with `exasol config reset`. Both SHALL respond with
the resulting configuration. A deployment whose settings are not editable SHALL
receive status 409 and keep its configuration; a value or option the launcher
rejects SHALL receive status 400 with the launcher's message.

#### Scenario: Setting an option

- **WHEN** the client patches `cpu-count` to 4 on the stopped local deployment
  `default`
- **THEN** the server responds with the configuration of `default` in which
  `cpu-count` is 4, and the other options keep their values

#### Scenario: Resetting options

- **WHEN** the client resets `cpu-count` and `memory-mb` on the stopped local
  deployment `default`
- **THEN** the server responds with the configuration of `default` in which
  both options have their preset defaults

#### Scenario: A running deployment refuses changes

- **WHEN** the client patches an option of a running deployment
- **THEN** the server responds with status 409 and the configuration stays
  unchanged

#### Scenario: A rejected value is reported

- **WHEN** the client patches `cpu-count` to `many` on the stopped local
  deployment `default`
- **THEN** the server responds with status 400 and the launcher's message

### Requirement: The Settings page edits a deployment's options

The Settings page of a deployment SHALL show its infrastructure and
installation presets with their descriptions, and one field per option with
its effective value, grouped by preset. Password options SHALL appear without
a value and stay read-only. When the settings are editable, each preset with
options SHALL offer Save, which stores the changed values, and Reset to
defaults, which after confirmation resets that preset's options; a rejected
change SHALL show the launcher's message. When the settings are not editable,
the fields SHALL be read-only and the page SHALL say what makes them editable:
stopping a local deployment, or destroying a cloud deployment.

#### Scenario: Saving a changed option

- **WHEN** the user changes CPU count to 4 on the Settings page of the stopped
  local deployment `default` and selects Save
- **THEN** the page shows `cpu-count` as 4 and offers to apply the changes

#### Scenario: Resetting a preset's options

- **WHEN** the user selects Reset to defaults for the infrastructure options of
  `default` and confirms
- **THEN** the page shows the preset defaults of the infrastructure options

#### Scenario: A running deployment's settings are read-only

- **WHEN** the user opens the Settings page of the running local deployment
  `default`
- **THEN** the page shows the options of `default` read-only and says to stop
  the deployment to change them

### Requirement: The Settings page applies saved changes

The Settings page SHALL show, after the user saves or resets options of a
stopped deployment, a banner saying that saved changes take effect when applied,
with Apply changes, which starts the deployment. For an `initialized`
deployment, the page SHALL always show a banner offering Deploy, which deploys
the deployment with its saved settings.

#### Scenario: Applying changes to a stopped local deployment

- **WHEN** the user selects Apply changes on the Settings page of `default`
  after saving a change
- **THEN** `default` starts, and the page shows the start's progress output

#### Scenario: Deploying an initialized deployment

- **WHEN** the user selects Deploy on the Settings page of the initialized
  deployment `test`
- **THEN** `test` deploys, and the page shows the deploy's progress output

### Requirement: The Settings page destroys and removes the deployment

The Settings page SHALL have a danger zone offering Destroy, Destroy and
remove, and Remove folder. Each SHALL open a dialog that describes its effect,
names the deployment folder when the folder will be removed, and enables its
confirming button only after the user types the deployment's name. In the
Destroy dialog, a checkbox SHALL choose whether the folder is also removed;
Destroy and remove opens the dialog with it checked. Confirming SHALL start
the operation. When a destroy with removal or a remove succeeds, the
application SHALL open the Deployments page.

#### Scenario: Confirmation requires the deployment name

- **WHEN** the user opens the Destroy dialog of `default` and types `defaul`
- **THEN** the Destroy button stays disabled, and typing `default` enables it

#### Scenario: Destroying keeps the folder

- **WHEN** the user confirms Destroy for `default` without removing the folder
- **THEN** the page shows the destroy's progress output, and when it succeeds
  the Settings page shows `default` as initialized and offers Deploy

#### Scenario: Destroying and removing the folder

- **WHEN** the user confirms Destroy and remove for `default`
- **THEN** the page shows the progress output, and when the operation succeeds
  the application opens the Deployments page, which no longer lists `default`

#### Scenario: Removing the folder only

- **WHEN** the user confirms Remove folder for `test`
- **THEN** the folder of `test` is removed and the application opens the
  Deployments page, which no longer lists `test`

### Requirement: The Settings page follows running operations

While an operation runs on the deployment, the Settings page SHALL show the
operation's progress output in place of the settings and danger zone. When the
operation ends, the page SHALL show the deployment's current settings, or the
operation's error.

#### Scenario: Returning to a running operation

- **WHEN** the user opens the Settings page of `default` while `default`
  starts
- **THEN** the page shows the start's progress output, and when the start ends
  it shows the settings of `default`

#### Scenario: A failed destroy is reported

- **WHEN** a destroy of `default` fails
- **THEN** the Settings page of `default` shows the launcher's error message
  and the settings of `default`

## ADDED Requirements

### Requirement: The API installs, updates, and removes script language containers as operations

The server SHALL run, in the background and with status 202, the new
operation, and its `Location` header in the response, for these requests on
`/api/v1/deployments/{name}/script-languages`:

- `POST {alias}/install`: `exasol slc install <alias>`, kind
  `install_script_language`;
- `POST {alias}/update`: `exasol slc update <alias>`, kind
  `update_script_language`;
- `POST {alias}/remove`: `exasol slc remove <alias>`, kind
  `remove_script_language`;
- `POST custom/install` and `POST custom/update` with the body fields `alias`,
  `language`, and `source`: `exasol slc custom install` or `exasol slc custom
  update` with these values, kinds `install_custom_script_language` and
  `update_custom_script_language`.

Each command SHALL restart a running database to apply the change, or, when
the request body sets `restart` to false, record the change for the next
start. These operations SHALL name their container's alias. While another
operation runs on the same deployment, the server SHALL respond with status
409 and start nothing. An alias that the script language listing does not
offer for the request SHALL receive status 404, and a custom request without
an alias, a language, or a source SHALL receive status 400.

#### Scenario: Installing an official container

- **WHEN** the client posts to the install endpoint of `PYTHON3` on `default`
- **THEN** the server responds with status 202 and a running operation of kind
  `install_script_language` that names `PYTHON3`, and when it succeeds the
  script language listing reports the Python container as installed

#### Scenario: Removing without a restart

- **WHEN** the client posts `{"restart": false}` to the remove endpoint of
  `JAVA` on the running deployment `default`
- **THEN** the server runs `exasol slc remove JAVA --no-restart` as an
  operation of kind `remove_script_language`, and the database keeps running

#### Scenario: Installing a custom container

- **WHEN** the client posts `{"alias": "MYPY3", "language": "python",
  "source": "/home/me/my-python.tar.gz"}` to the custom install endpoint of
  `default`
- **THEN** the server responds with status 202 and an operation of kind
  `install_custom_script_language` that names `MYPY3`, and when it succeeds
  the listing reports `MYPY3` among the custom containers

#### Scenario: An unlisted alias is not found

- **WHEN** the client posts to the install endpoint of `--all` on `default`
- **THEN** the server responds with status 404 and runs no script language
  command

## ADDED Requirements

### Requirement: The API deploys, destroys, and removes deployments as operations

The server SHALL run, in the background and with status 202, the new
operation, and its `Location` header in the response:
`exasol deploy` for `POST /api/v1/deployments/{name}/deploy` as kind `deploy`;
`exasol destroy` for `POST /api/v1/deployments/{name}/destroy` as kind
`destroy`, or `exasol destroy --remove` as kind `destroy_and_remove` when the
request body sets `remove_folder` to true; and `exasol remove` for
`POST /api/v1/deployments/{name}/remove` as kind `remove`. While another
operation runs on the same deployment, the server SHALL respond with status
409 and start nothing. An unknown name SHALL receive status 404.

#### Scenario: Deploying an initialized deployment

- **WHEN** the client posts to the deploy endpoint of the initialized
  deployment `test`
- **THEN** the server responds with status 202 and an operation of kind
  `deploy` for `test` with status `running`, and the launcher deploys `test`

#### Scenario: Destroying and removing the folder

- **WHEN** the client posts `{"remove_folder": true}` to the destroy endpoint of
  `default`
- **THEN** the server responds with status 202 and an operation of kind
  `destroy_and_remove`, and when it succeeds the deployment listing no longer
  contains `default`

#### Scenario: Removing a deployment folder

- **WHEN** the client posts to the remove endpoint of `test`
- **THEN** the server responds with status 202 and an operation of kind
  `remove`, and when it succeeds the deployment folder of `test` no longer
  exists

#### Scenario: Destroy waits for a running operation

- **WHEN** the client posts to the destroy endpoint of `default` while an
  operation runs on `default`
- **THEN** the server responds with status 409

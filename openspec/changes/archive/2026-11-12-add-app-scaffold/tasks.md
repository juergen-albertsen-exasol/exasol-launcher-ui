Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server foundation

- [x] 1.1 Create the Go module, the `exasol-local-ui` command with `--port` and `--open`, the loopback listener with the occupied-port error, a Taskfile with `build`, `test`, `lint`, and `dev`, and CI running them
- [x] 1.2 Add token generation, the access-URL-to-cookie exchange, the `/api/` session check, and the `Host` and `Origin` middleware, with tests for every `ui-server` scenario

## 2. Launcher and database integration

- [x] 2.1 Add the launcher client (`--exasol`, startup check, `deployments list --json`, `info --json` by deployment directory) with a brief listing cache, tested against a fake launcher executable
- [x] 2.2 Add `GET /api/v1/deployments` and `GET /api/v1/deployments/{name}`, including 404 for unknown names and responses that carry no password
- [x] 2.3 Add the database client reading `secrets.json`, password redaction for errors, and `GET /api/v1/deployments/{name}/health/ping`, with unit tests and a test against a running local deployment that skips when none is available

## 3. Browser application

- [x] 3.1 Scaffold the Angular application in `web/` with the Administration UI's versions, `@exasol/ui-common` layout, header, and navigation drawer; embed its build in the binary, serve it for every non-API path, and add `--web-dir`
- [x] 3.2 Add each deployment's path to the listing, the deployments NgXs state, the Deployments tiles page without navigation as the design canvas draws it, the overview page at `/deployments/{name}` inside the layout with its Overview navigation entry with state, connection details, and reachability, the header selector that opens another deployment's overview, and the `--deployment` access URL path
- [x] 3.3 Add the 401 interceptor and reconnect message, and document setup (including the GitHub Packages token), development loop, and usage in the README

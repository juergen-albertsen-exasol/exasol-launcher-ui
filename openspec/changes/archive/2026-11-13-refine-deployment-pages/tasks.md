Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server

- [x] 1.1 Add `POST /api/v1/deployments/{name}/open-folder` with the platform opener (including Windows), 404 for unknown names, and tests that use a fake opener

## 2. Browser application

- [x] 2.1 Show the folder on the tiles and the overview as a `file://` link that calls the endpoint, as the design canvas draws it, with tests
- [x] 2.2 Drop the folder from the tiles, show a progress indicator while the deployments are looked up, and link the Exasol logo to the Deployments page on every page, as the design canvas draws it, with tests

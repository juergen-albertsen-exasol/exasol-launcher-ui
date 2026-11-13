Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server

- [x] 1.1 Add the launcher client's `diag local` and `diag info` calls and the local runtime and report endpoints, with named sidecar ports and their host ports, 409 for non-local deployments and launcher refusals, and the password replaced in the report, tested against fake launcher executables for every API scenario of `diagnostics`
- [x] 1.2 Add the launcher client's `diag unlock` call, the operations manager's query for a deployment's running operation, and the unlock endpoint with 204, and 409 while an operation runs, with tests

## 2. Browser application

- [x] 2.1 Add the Diagnostics route and drawer entry, the diagnostics NgXs state, and the Diagnostics page with the local runtime, Check again, the report with Show and Download, and operation progress, as the design canvas draws it, with tests
- [x] 2.2 Add the shell commands with copy buttons and the confirmed Unlock to the Diagnostics page, with tests and a manual check on a scratch local deployment

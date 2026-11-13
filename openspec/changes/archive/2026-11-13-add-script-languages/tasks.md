Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server

- [x] 1.1 Add the launcher client's `slc list --json` call and the script language listing endpoint with the Rust entry, the custom containers, and 409 for non-local deployments, tested against fake launcher executables for every API scenario of `script-languages`
- [x] 1.2 Give operation kinds parameters, add the five script language kinds with `--auto-approve` or `--no-restart` and their alias, and the install, update, remove, and custom endpoints with 404 for unlisted aliases and 400 for incomplete custom requests, with tests for every scenario of the new `operations` requirement

## 2. Browser application

- [x] 2.1 Add the Script languages route and drawer entry, the script languages NgXs state, operation labels for the new kinds, and the Script languages page with the restart option, the official and custom tables, Install, Update, Remove with confirmation, the custom form, the unsupported message, and operation progress, as the design canvas draws it, with tests and a manual check on a scratch local deployment

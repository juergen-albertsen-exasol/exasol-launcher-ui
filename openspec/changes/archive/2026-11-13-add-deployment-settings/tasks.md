Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server

- [x] 1.1 Add the launcher client's `config get --json`, `config set`, and `config reset` calls, and the configuration endpoints (`GET`, `PATCH`, `POST .../reset`) with secret withholding, `editable` and `apply_with`, 409 and 400 handling, tested against fake launcher executables for every API scenario of `deployment-settings`
- [x] 1.2 Let operation kinds carry their launcher arguments and decode JSON only for start and stop, and add the deploy, destroy (with `remove_folder`), and remove endpoints, with tests for every scenario of the new `operations` requirement

## 2. Browser application

- [x] 2.1 Add the Settings route and drawer entry, the settings NgXs state, and the Settings page with preset info boxes, option fields, Save, Reset to defaults, the read-only state, and the apply and deploy banners, as the design canvas draws them, with tests
- [x] 2.2 Add the danger zone, the typed-name confirmation dialog, the operation log while an operation runs, and the navigation to the Deployments page after a removal, as the design canvas draws them, with tests and a manual check against a local deployment

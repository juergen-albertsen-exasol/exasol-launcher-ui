Each task is one commit that passes `task build`, `task test`, and `task lint`.

## 1. Server

- [x] 1.1 Add the launcher client's streaming run (stderr lines, JSON stdout) and the in-memory operation runner (one running operation per deployment, outcome, redacted output, recent history, listing cache invalidation), tested against fake launcher executables
- [x] 1.2 Add the start and stop endpoints, `GET /api/v1/operations`, `GET /api/v1/operations/{id}`, and the server-sent events stream, with tests for every `operations` scenario

## 2. Browser application

- [x] 2.1 Add the operations NgXs state with polling, the activity indicator in the layout header and on the start page's bar, and the activity panel, as the design canvas draws them, with tests
- [x] 2.2 Add Start and Stop to the overview card, the operation log in place of the details while an operation runs, the refresh when it ends, and failure messages, as the design canvas draws them, with tests and a manual check against a local deployment

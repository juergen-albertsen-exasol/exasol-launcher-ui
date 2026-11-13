## ADDED Requirements

### Requirement: The command reports its version

`exasol-launcher-ui --version` SHALL print `exasol-launcher-ui <version>` and
exit with status 0 without starting the server. Released builds SHALL report
their release version without the leading `v`; builds from source without a
release version SHALL report `dev`.

#### Scenario: A released build reports its version

- **WHEN** the user runs `exasol-launcher-ui --version` with the binary of
  release `v0.1.0`
- **THEN** it prints `exasol-launcher-ui 0.1.0` and exits with status 0

#### Scenario: A source build reports dev

- **WHEN** the user runs `bin/exasol-launcher-ui --version` after `task build`
- **THEN** it prints `exasol-launcher-ui dev` and exits with status 0

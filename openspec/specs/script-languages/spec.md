# script-languages Specification

## Purpose

Report a local deployment's script language containers and install, update, or
remove official and custom ones from its Script languages page.

## Requirements
### Requirement: The API reports a deployment's script language containers

`GET /api/v1/deployments/{name}/script-languages` SHALL return the official
script language containers of the catalog, as `exasol slc list --json`
reports them, followed by the Rust container that `exasol slc install rust`
installs, and the installed custom containers. Each official container SHALL
carry the alias that identifies it, its language, flavor, version, aliases,
and whether it is installed; the Rust container SHALL be marked as the latest
release instead of a catalog version. Each custom container SHALL carry its
alias, language, source, and whether its image is available. A deployment
whose infrastructure is not local SHALL receive status 409 with the message
that script language container management is only supported for local
deployments. An unknown name SHALL receive status 404.

#### Scenario: Official, Rust, and custom containers

- **WHEN** the client requests the script languages of `default`, which has
  installed the Java and Python containers and the custom container `MYPY3`,
  while the catalog also offers R
- **THEN** the response lists Java and Python as installed and R as not
  installed, then Rust as a not installed latest release, and `MYPY3` with
  its language and source among the custom containers

#### Scenario: An installed Rust container

- **WHEN** the client requests the script languages of a deployment on which
  `exasol slc install rust` has run
- **THEN** the response lists Rust as installed with the official containers,
  and not among the custom containers

#### Scenario: A cloud deployment runs no script language containers

- **WHEN** the client requests the script languages of a cloud deployment
- **THEN** the server responds with status 409 and the message that script
  language container management is only supported for local deployments

### Requirement: The Script languages page manages official containers

The Script languages page of a deployment SHALL list the official containers,
Rust included, with language, flavor, version, aliases, and whether each is
installed. Each container that is not installed SHALL offer Install; each
installed one SHALL offer Update and Remove, where Remove asks for
confirmation first. The page SHALL offer one restart option, chosen by
default, that decides for every action on the page whether the database
restarts now or the change applies on the next start. When the API refuses
to report script languages, the page SHALL show its message.

#### Scenario: Listing containers

- **WHEN** the user opens the Script languages page of `default`
- **THEN** the page lists Java and Python with Update and Remove, and R and
  Rust with Install, with Rust described as the latest exasol-labs release

#### Scenario: Installing with a restart

- **WHEN** the user selects Install for R on the Script languages page of the
  running deployment `default`
- **THEN** the page shows the progress output while R is installed and the
  database restarts, and then lists R as installed

#### Scenario: Removing for the next start

- **WHEN** the user clears the restart option, selects Remove for Java, and
  confirms
- **THEN** Java is removed without restarting the database, and the page
  lists Java as not installed

#### Scenario: A cloud deployment explains that it runs no containers

- **WHEN** the user opens the Script languages page of a cloud deployment
- **THEN** the page shows the message that script language container
  management is only supported for local deployments

### Requirement: The Script languages page manages custom containers

The Script languages page SHALL list the installed custom containers with
alias, language, source, and whether each is available, each offering
Remove with confirmation. A form with alias, language, and container tarball
SHALL install a new custom container or update an installed one from a local
path or an HTTPS URL, using the page's restart option.

#### Scenario: Installing a custom container

- **WHEN** the user enters `MYPY3`, `python`, and the path of a container
  tarball, and selects Install
- **THEN** the page shows the progress output while the container is
  installed, and then lists `MYPY3` among the custom containers

#### Scenario: Updating a custom container

- **WHEN** the user enters the alias `MYPY3` of an installed custom container
  with a new tarball, and selects Update existing
- **THEN** the container behind `MYPY3` is replaced with the new tarball

#### Scenario: A refused container is reported

- **WHEN** the launcher rejects the custom container, for example because the
  archive is no script language container
- **THEN** the page shows the launcher's error and keeps the entered values

### Requirement: The Script languages page follows running operations

The Script languages page SHALL show the progress output of an operation that
runs on the deployment and keep its actions unavailable meanwhile. When the
operation ends, it SHALL show the deployment's current script language
containers, or the operation's error.

#### Scenario: An install finishes

- **WHEN** the Script languages page shows the progress of installing Python
  on `default` and the operation succeeds
- **THEN** the page lists Python as installed and offers its actions again


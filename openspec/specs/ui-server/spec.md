# ui-server Specification

## Purpose

Start Exasol Local UI as a loopback-only server that only the browser holding its session token can use.

## Requirements
### Requirement: The command starts a loopback web server

`exasol-local-ui` SHALL serve the application on `127.0.0.1`, on port 5180 by
default or on the port given with `--port`, and SHALL keep serving until it is
interrupted. On startup it SHALL print the access URL, and with `--open` it
SHALL also open that URL in the default browser.

#### Scenario: Default startup prints the access URL

- **WHEN** the user runs `exasol-local-ui` without `--deployment`
- **THEN** it prints a URL of the form `http://127.0.0.1:5180/?token=<token>`
  and the application answers on that address

#### Scenario: A chosen port is used

- **WHEN** the user runs `exasol-local-ui --port 6000`
- **THEN** the printed URL and the served address use port 6000

#### Scenario: Startup opens the browser on request

- **WHEN** the user runs `exasol-local-ui --open`
- **THEN** the default browser opens the printed access URL

#### Scenario: An occupied port fails startup

- **WHEN** the user starts the server on a port another process already uses
- **THEN** the command exits with a non-zero status and an error naming the port

### Requirement: The command locates the launcher

The server SHALL run the `exasol` launcher found on `PATH`, or the executable
given with `--exasol`. Startup SHALL fail with an error naming the expected
executable when the launcher cannot be run.

#### Scenario: Missing launcher fails startup

- **WHEN** the user starts the server and no runnable `exasol` executable is on
  `PATH` or at the `--exasol` path
- **THEN** the command exits with a non-zero status and an error naming the
  launcher executable it tried

### Requirement: Requests require the session token

Each server start SHALL generate a new random token of at least 128 bits.
Opening the access URL SHALL set an `HttpOnly`, `SameSite=Strict` session cookie
and redirect to the same page without the token in the URL. Requests to
`/api/` SHALL succeed only with that session cookie and SHALL receive status 401
otherwise.

#### Scenario: The access URL establishes a session

- **WHEN** a browser opens the printed access URL
- **THEN** the response sets the session cookie and redirects to `/`

#### Scenario: API requests without a session are unauthorized

- **WHEN** a client calls an `/api/` endpoint without the session cookie, or
  with a cookie from an earlier server start
- **THEN** the server responds with status 401

### Requirement: Requests must address the server itself

The server SHALL respond with status 403 to any request whose `Host` header is
not `127.0.0.1:<port>` or `localhost:<port>`, and to any request carrying an
`Origin` header other than `http://127.0.0.1:<port>` or
`http://localhost:<port>`.

#### Scenario: A foreign host name is refused

- **WHEN** a request arrives with `Host: attacker.example:5180`
- **THEN** the server responds with status 403

#### Scenario: A cross-origin request is refused

- **WHEN** a request with a valid session cookie carries
  `Origin: https://attacker.example`
- **THEN** the server responds with status 403


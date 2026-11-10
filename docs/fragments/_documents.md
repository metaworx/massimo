> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.

# The documents

Every document of the project, in the README and on the site's docs index
(`packaging/site/build.sh` builds a page of each one listed here, and fails
on a document under `docs/` that is not).

## Using it

- [README.md](README.md): what Massimo is, installing it, the server,
  the command line, the versions and channels
- [packaging/README.md](packaging/README.md): the packages and services,
  the apt repository, Homebrew, updates
- [docs/ui.md](docs/ui.md): using the web UI
- [docs/app.md](docs/app.md): the apps for Android and iOS
- [docs/proxy.md](docs/proxy.md): the server as a Cube for other clients
- [docs/homeassistant.md](docs/homeassistant.md): Home Assistant,
  through the proxy today and with an integration of its own later
- [docs/reverse-proxy.md](docs/reverse-proxy.md): `massimo serve` behind a
  reverse proxy

## The API

- [docs/api-v1.md](docs/api-v1.md),
  [docs/api-v1-openapi.yaml](docs/api-v1-openapi.yaml): the HTTP API

## How it works

- [docs/portal-protocol.md](docs/portal-protocol.md):
  everything learned about the Cube, the portal and the radio commands
- [docs/firmware-extraction.md](docs/firmware-extraction.md):
  reading the Cube's firmware over JTAG, hardware facts and the procedure
- [docs/background.md](docs/background.md): the evaluation before the project,
  licence reasoning, routes to a public server, sources
- [docs/design.md](docs/design.md): the design proposal, target environments,
  phases and their status
- [docs/store.md](docs/store.md): the design of the server's persistent storage:
  engine, schema, users and roles, migration of installed boxes
- [docs/server.md](docs/server.md): the server mode: encryption at rest,
  the public server, sync, and what the store prepares for them
- [docs/update-windows.md](docs/update-windows.md):
  the design for updates from the web UI on Windows: the options,
  the recommendation, the changes per file, the CI and real-PC tests

## Contributing

- [GUIDELINES/README.md](GUIDELINES/README.md): how the project is worked on:
  the guidelines, the project's contract and its coding standard
- [docs/roadmap.md](docs/roadmap.md): what comes next, in order
- [docs/testing-writes.md](docs/testing-writes.md):
  the test plan for the configuration writes on real hardware,
  [docs/reports/write-test-2026-11-05.md](docs/reports/write-test-2026-11-05.md) its results
- [docs/testing-windows.md](docs/testing-windows.md):
  the test plan for the Windows service on a real PC,
  [docs/reports/windows-test-2026-11-05.md](docs/reports/windows-test-2026-11-05.md) its results
- [tools/ui-test/README.md](tools/ui-test/README.md): `make ui-test`,
  the web UI smoke test with Playwright, and its setup
- [docs/ui-testing.md](docs/ui-testing.md): testing the web UI:
  the smoke test, Cypress and Playwright compared, screenshots,
  the recommendation

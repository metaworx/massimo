> **Fragment** — inlined by `tools/sync-docs.sh`; not a standalone document.

# {{project_name}} — Project Contract (v1.4.0)

<!-- TEMPLATE: installed as {{guidelines_root}}/project/_CONTRACT.md. Replace
     every {{TODO: …}}, delete rows that do not apply. Shared documents
     never name a project or carry a project path - they reference this file. -->

What binds work in this project, for everyone working on it.
Inlined into `/AGENTS.md` for agents and into `{{guidelines_root}}/README.md` for people,
so that the two cannot drift apart.

> `{{TODO: …}}` is a value you replace, braces and all.
> The other double-braced names, such as `{{project_name}}`,
> are filled in when the documents are generated: leave them as they are.

## 1. Project Facts

> No absolute paths here.
> An agent is already inside the checkout,
> so the repository root is `git rev-parse --show-toplevel`;
> a checkout can live anywhere and this file is committed.
> Machine-specific values belong in `{{guidelines_root}}/config.local.ini`,
> which is not tracked.

| Fact                  | Value |
|-----------------------|-------|
| Project name          | {{project_name}} |
| Language(s)           | declared as `project.languages` in `{{guidelines_root}}/config.ini`, kept honest by `{{shared_root}}/tools/detect-languages.sh --check` |
| Source directories    | `cmd/massimo/` (the command line and `serve`), `internal/` (the packages: `protocol/cube`, `cubelan`, `engine`, `server` with the web UI in `server/ui/`, `proxy`, `backup`, `store`, `update`, `install`, …), `packaging/` (unit files, plists, the installers' assets), `tools/ui-test/` (the Playwright smoke test), `docs/` (the protocol findings, the API, the designs, the handovers) |
| Shipped-code paths    | declared as `project.shipped` in `{{guidelines_root}}/config.ini` and listed below; kept honest by `{{shared_root}}/tools/changelog.sh check` (rule: `{{shared_root}}/COMMIT.md` §5) |
| Version manifest      | none: the version is the nearest git tag (`git describe --tags`, baked in by `make`); a release is cut with `make release-alpha` (`BUMP=minor` for a feature, `DRY=1` to see it first) on a clean `main` equal to `origin/main`: `changelog.sh cut` moves `[Unreleased]` into the release's section in a `chore(release):` commit, tags it with that section as the message, and `main`, `alpha` and the tag are pushed atomically; `README.md`, "Versions" |
| Test gate command     | `MASSIMO_ENV_FILE=/dev/null go test ./...` (the gate of `{{shared_root}}/lang/go/TESTING.md`; `make race`, `-race` for `internal/server`, `internal/store`, `internal/cubelan`, `internal/proxy`, `internal/serve`, `internal/announce` and `internal/notify`, is recommended, and CI's Race job runs it), then `make cross` (every platform compiles), and `make ui-test` for a change under `internal/server/ui/` (the pytest suite of `tools/ui-test/`, needs the Playwright venv, `tools/ui-test/README.md`; one test with `PYTEST_ADDOPTS='-k "<test name> and 390-de"' make ui-test`) |
| Lint command          | `gofmt -l .` prints nothing and `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...` exits 0, as `{{shared_root}}/lang/go/LINTING.md` requires per platform; `govulncheck ./...` runs in CI for linux and windows; no other linter |
| Commit summary        | declared as `[commit]` in `{{guidelines_root}}/config.ini` and shown below; held to it by `{{shared_root}}/tools/commit-msg.sh` (rule: `{{shared_root}}/COMMIT.md` §3.1) |

A commit touching any path listed below must carry a `CHANGELOG.md` bullet.
The list is generated from `project.shipped`,
so it cannot drift from what the check enforces - edit the config,
not this block:

<!-- BEGIN GENERATED SHIPPED-PATHS - do not edit; run "{{shared_root}}/tools/changelog.sh update-contract" -->
- `cmd/**`
- `internal/**`
- `packaging/**`
- `go.mod`
- `go.sum`
- `Makefile`
- `.github/**`
- `third_party/**`
- `:!docs/**`
- `:!tools/**`
- `:!GUIDELINES/**`
- `:!*.md`
- `:!.gitignore`
- `:!.gitattributes`
- `:!.gitmodules`
- `:!LICENSE`
<!-- END GENERATED SHIPPED-PATHS -->

The commit form is generated from `[commit]` in `{{guidelines_root}}/config.ini` -
edit the config, not this block:

<!-- BEGIN GENERATED COMMIT-FORM - do not edit; run "{{shared_root}}/tools/sync-docs.sh" -->
<!-- END GENERATED COMMIT-FORM -->

## 2. Primary References

- `/AGENTS.md` — runtime behavior contract, gating flow, action-plan workflow.
- `{{shared_root}}/lang/<language>/TESTING.md` and `LINTING.md` — language
  baselines for each language in `project.languages`;
  this file's facts table overrides their example commands.
- `{{shared_root}}/COMMIT.md` — commit workflow, gating, message policy.
- `{{shared_root}}/ENVIRONMENTS.md` — host/agent command conventions.
- `CONTRIBUTING.md` — repository-specific contributor conventions.

## 3. Project-Specific Conventions

The coding standard at the end of `docs/review-2026-11-05.md` and the
"Conventions" of `docs/handover-session.md` are the rules; the ones an
agent trips over most:

- **Secrets and identities.**
  No keys, IVs, tokens, real device names or real Cube serials in code,
  fixtures, documents or commit messages; placeholders such as `KEQ0000000`,
  and for a radio address `0a0101`:
  `tools/identifiers` fails `go test` on a serial, a MAC or a radio address
  that is not one, unless its `allowed.txt` names it with a reason.
  `.env`, backups and logs are gitignored.
  The checkout's `.env` points at real hardware,
  so tests and manual runs use `MASSIMO_ENV_FILE=/dev/null` or the fake Cube
  (`massimo fakecube`).
- **The Cube.**
  Protocol bytes are built only in `internal/protocol/cube` and tested against
  captured lines byte for byte;
  a command the hardware has not confirmed says so in its comment and in
  `docs/portal-protocol.md`.
  Hardware findings go into `docs/portal-protocol.md` the day they are made,
  with the date and the Cube firmware.
  A Cube serves one LAN client at a time:
  commands go through the proxy of a running `serve`,
  never to a Cube another program holds;
  a radio write costs duty cycle (about 1.25 % each, 36 s of air time an hour),
  so nothing is sent to a real Cube without the owner's go.
- **Tests** run against the fake Cube with the recorded greeting;
  none needs hardware or the network.
  A new API route gets a test that checks the exact line sent to the fake Cube,
  and `TestOpenAPICoversRoutes` fails when `docs/api-v1-openapi.yaml` and the
  routes differ.
- **The web UI** is plain HTML, CSS and JavaScript under `internal/server/ui/`,
  embedded with `go:embed`: no build step, no framework, no inline handlers,
  no `innerHTML`, API text only through `textContent`.
  `index.html` is the page's skeleton; a feature's markup (its settings panel,
  the `<template>`s its script clones) is its partial under `ui/partials/`,
  listed in `uiPartials` in `ui.go`.
  Shared elements come from `components.js`,
  and a label with its control is the one row style, `.field`,
  or `.field.wide` for a control on a line of its own.
  Every text goes through `t()` or `data-i18n`, its key in all six
  `ui/i18n/<lang>.json` (FR/IT/NL/PL machine-translated and marked);
  CSS variables for light and dark, phone width first (390 px),
  verified with `make ui-test` at 1100 and 390 px.
- **Documents move with the code**:
  `README.md` for the command line,
  `docs/api-v1.md` and the OpenAPI file for every route,
  `packaging/` and `docs/update-windows.md` for the installers,
  `docs/ui.md` for the UI,
  `docs/roadmap.md` and `docs/handover-session.md` at the end of a session by
  the coordinating session only,
  but for a plan's items on the roadmap:
  the session whose plan retires prunes them itself, at the retirement.
- **Branches and pull requests**:
  develop on `main` through pull requests the owner merges;
  `alpha` is the release branch,
  only ever fast-forwarded by `make release-alpha`.
  A branch merges `origin/main` before its pull request (merge,
  never rebase a stack).
  Commits are signed (SSH, the repository's git config);
  run `timeout 60 git commit …` because a locked key agent hangs instead of failing.
- **Hosts.**
  WSL 2 in NAT mode has no LAN broadcast and no inbound connections:
  `massimo discover`,
  the proxy and discovery answers from a `serve` in WSL do not work there;
  hardware work goes through the Raspberry Pi's `serve` or the Windows side.
  Windows executables run from WSL through interop and use Windows networking;
  elevation needs `gsudo` and the owner's UAC confirmation.

## 4. Document Governance

- This document follows the shared governance rules in `{{shared_root}}/GOVERNANCE.md`.

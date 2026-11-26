> **Fragment** — inlined by `tools/sync-docs.sh`; not a standalone document.

# {{project_name}} — Project Contract (v1.22.0)

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
| Source directories    | `cmd/massimo/` (the command line and `serve`), `internal/` (the packages: `protocol/cube`, `cubelan`, `engine`, `server` with the web UI in `server/ui/`, `proxy`, `backup`, `store`, `update`, `install`, …), `packaging/` (unit files, plists, the installers' assets), `tools/ui-test/` (the Playwright smoke test), `tools/brand/` (the generator of the mark and every file made from it), `docs/` (the protocol findings, the API, the designs, the handovers) |
| Shipped-code paths    | declared as `project.shipped` in `{{guidelines_root}}/config.ini` and listed below; kept honest by `{{shared_root}}/tools/changelog.sh check` (rule: `{{shared_root}}/COMMIT.md` §5) |
| Version manifest      | for the server none: the version is the nearest git tag (`git describe --tags`, baked in by `make`); for the Android app `app/version.json`, the changelog's manifest (`GUIDELINES/config.ini`), its versionCode counted by `packaging/app-version.sh` and written by `make release` into the release commit (AP MobileApp, D12-A); a release is cut with `make release <flavour>`, the flavour `alpha`, `beta`, `rc` or `stable` (`BUMP=minor` for a feature, `BUMP=patch` for a fix of a stable release, `DRY=1` to see it first; `make build-release` is the build) on a clean `main` equal to `origin/main`, or on a maintenance branch `maint/<x.y>` equal to its `origin` copy for a release of x.y once `make freeze <x.y>` has cut it: `changelog.sh cut` moves `[Unreleased]` into the release's section in a `chore(release):` commit, tags it with that section as the message, and the branch and the tag are pushed atomically; CI tests the tag, releases it and then fast-forwards `release/<flavour>` to it; `README.md`, "Versions" |
| Test gate command     | `make ui` first, the web UI's bundle the server embeds (every Makefile target that compiles Go runs it), then `MASSIMO_ENV_FILE=/dev/null go test ./...` (the gate of `{{shared_root}}/lang/go/TESTING.md`; `make race`, `-race` for `internal/server`, `internal/store`, `internal/cubelan`, `internal/proxy`, `internal/serve`, `internal/announce`, `internal/notify`, `internal/engine`, `internal/cubeio` and `internal/certs`, is recommended, and CI's Race job runs it), then `make cross-check` (every platform compiles and links, and no binary is written; `make cross` is the build into `dist/`), and `make ui-test` for a change under `internal/server/ui/` (the pytest suite of `tools/ui-test/`, needs the Playwright venv, `tools/ui-test/README.md`; one test with `PYTEST_ADDOPTS='-k "<test name> and 390-de"' make ui-test`), and `make lan-test` for a change to discovery, the mDNS announcement (`internal/announce`), `discover` or the proxy's discovery answers (Docker with Compose v2, `tools/lan-test/README.md`), and `make app-emulator-test` for a change under `app/` or to what of the server the app drives (the sign-in and the login by key, a user's tokens, the connect page), where KVM is open to the user (the Android SDK's pinned emulator and image and JDK 17, `tools/app-emulator/README.md`, "What it needs"); CI's job Android emulator runs it in any case |
| Lint command          | `gofmt -l .` prints nothing and `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...` exits 0, as `{{shared_root}}/lang/go/LINTING.md` requires per platform; `make actionlint` exits 0 for a change under `.github/` (actionlint pinned in `tools/actionlint/`, the `run:` scripts by shellcheck where it is installed, as CI's job is); `make ui-check` exits 0 for a change under `internal/server/ui/src/` (`tsc --noEmit`, then Biome's two hooks rules of `tools/ui-check/biome.jsonc`, TypeScript and Biome locked in `tools/ui-check/`, needs Node.js and npm, as CI's job UI types runs it); `govulncheck ./...` runs in CI for linux and windows; no other linter |
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
- `app/**`
- `:!docs/**`
- `:!tools/**`
- `:!GUIDELINES/**`
- `:!*.md`
- `:!.gitignore`
- `:!.gitattributes`
- `:!.gitmodules`
- `:!LICENSE`
- `:!packaging/site/site.py`
- `:!packaging/site/doc.html.in`
- `:!packaging/site/docs.js`
- `:!packaging/site/site.css`
- `:!packaging/site/privacy.html`
- `:!*_test.go`
- `:!**/testdata/**`
- `:!packaging/apt/test.sh`
- `:!packaging/deb/test.sh`
- `:!packaging/deb/systemd-test.sh`
- `:!packaging/deb/systemd-cases.sh`
- `:!packaging/launchd/template-test.sh`
- `:!app/android/app/src/test/**`
- `:!app/android/app/src/androidTest/**`
<!-- END GENERATED SHIPPED-PATHS -->

A change touching only the paths listed below may skip the test suite:
no test reads them, and `{{shared_root}}/tools/fingerprint.sh` leaves them out
of the code fingerprint a CI gate compares.
None of them is shipped.
The list is generated from `[ci] untested` - edit the config, not this block,
and delete both where the project declares no such list:

<!-- BEGIN GENERATED UNTESTED-PATHS - do not edit; run "{{shared_root}}/tools/fingerprint.sh update-contract" -->
- `GUIDELINES/wip/**`
- `GUIDELINES/project/**`
- `AGENTS.md`
- `GUIDELINES/README.md`
- `CHANGELOG.md`
- `CHANGELOG.d/**`
- `LICENSE`
<!-- END GENERATED UNTESTED-PATHS -->

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

The rules of the code and of the work on it;
the ones an agent trips over most come first:

- **Secrets and identities.**
  No keys, IVs, tokens, real device names or real Cube serials in code,
  fixtures, documents or commit messages; placeholders such as `KEQ0000000`,
  and for a radio address `0a0101`:
  `tools/identifiers` fails `go test` on a serial, a MAC or a radio address
  that is not one, unless its `allowed.txt` names it with a reason.
  Keys, IVs and tokens are redacted in every printout, a log's included;
  real room and device names appear only in the owner's pastes.
  `.env`, backups and logs are gitignored.
  The checkout's `.env` points at real hardware,
  so tests and manual runs use `MASSIMO_ENV_FILE=/dev/null` or the fake Cube
  (`massimo fakecube`).
- **The Cube.**
  Protocol bytes are built only in `internal/protocol/cube` and tested against
  captured lines byte for byte;
  a command the hardware has not confirmed says so in its comment and in
  `docs/cube-protocol.md`.
  Hardware findings go into `docs/cube-protocol.md` the day they are made,
  with the date and the Cube firmware;
  the lessons of the hardware days are there ("Lessons from the hardware days")
  and in `packaging/README.md` ("Learnings").
  A Cube serves one LAN client at a time:
  commands go through the proxy of a running `serve`,
  never to a Cube another program holds (`-no-direct` refuses the direct
  fallback, `-direct` forces it);
  a radio write costs duty cycle (about 1.25 % each, 36 s of air time an hour),
  so nothing is sent to a real Cube without the owner's go.
- **Go code.**
  The Go of `go.mod`, the standard library first.
  `gofmt` and `go vet` clean, and no `//nolint` outside `third_party/`.
  Every file opens with the SPDX line `AGPL-3.0-or-later` and the copyright
  line; JavaScript and CSS carry a one-line licence comment.
  A package comment says what the package is for;
  every exported symbol has a comment that is a sentence,
  an unexported helper one where its name does not say it.
  Errors are wrapped with context (`fmt.Errorf("serve: %w", err)`),
  a sentinel error lives in the package that defines the condition
  (`cubelan.ErrNotConnected`), an HTTP error goes through `fail(status, …)`;
  input never panics.
  Logging is `slog` key-value pairs: Info for an event a user wants in the log,
  Warn for anything the Cube refused, Error for what failed on our side,
  Debug for per-line traffic.
- **Tests** run against the fake Cube with the recorded greeting;
  none needs hardware or the network.
  A new API route gets a test that checks the exact line sent to the fake Cube,
  and `TestOpenAPICoversRoutes` fails when `docs/api-v1-openapi.yaml` and the
  routes differ.
- **The API.**
  A client needs a token: `MASSIMO_API_TOKEN` is the owner's key,
  every other user has one of their own (`docs/api-v1.md`, "Authentication");
  without one the API answers 401.
- **The web UI** is under `internal/server/ui/`, embedded with `go:embed`:
  the skeleton, the parts drawn into it, and the plain scripts.
  - **The skeleton** is `index.html` with the frames in `ui/partials/`
    (`uiPartials` in `ui.go`): the page's sections, the cards' frames,
    and the places the parts draw into.
    Its static texts take `data-i18n`, which `i18n.js` applies;
    no script writes one (`TestUINoHTMLInjection`).
  - **A part** is Preact with signals, in TSX under `internal/server/ui/src/<region>/`,
    bundled by `make ui` (esbuild as a Go tool, `tools/uibundle`) into `ui/parts.js`,
    which is not committed.
    Preact is vendored under `third_party/preact/`;
    no other library and no other build step.
    A part is built once per key and updated in place:
    data never replaces an element the user can hold.
    A field that shows stored data keeps a draft.
    A part that reads a signal reads everything it depends on as signals.
    A part gives the focus in the handler of the user's action
    (or in a timer that handler sets, where a listener of the page must
    run first: the click outside an open menu, which closes it),
    or in a layout effect, in the drawing that shows its place.
    A part calls `t()` and carries no `data-i18n`.
    Its shared elements come from `internal/server/ui/src/shared/`,
    which knows no region (`TestSharedPartsKnowNoRegion`):
    a region writes no copy of one,
    and the change that first needs one that is missing writes it there,
    with its row in `docs/ui.md` ("How the page is built").
    `make ui-check` type-checks the parts and checks their hooks:
    a hook is called at the top of a part's drawing and is known by its name,
    and an effect lists what it reads or says why not in a `biome-ignore`
    comment with its reason.
    A file under `ui/src/` opens with the SPDX and copyright lines,
    then the `@file` header of `{{shared_root}}/lang/ts/LINTING.md`
    (`TestUIFileHeaders`);
    `make ui-check` is the parts' lint and `make ui-test` their test:
    they have no unit-test runner.
  - **The data are signals:** `state` of `app.js` and `themeState` of
    `theme.js` stay the plain scripts' objects, and `shared/data.ts` makes
    their fields signals.
    A field is written whole, never in place (`TestStateWrittenWhole`):
    by the loads of `app.js` (the Cubes, the rooms, the settings, a sign-out's
    clearing), and beside them by `home.js` (a Cube found),
    `rooms.js` (a device's state), `header.js` (the server's answer, a new
    version), `login.js` (who is in) and `theme.js` and the scheme editor
    (the colour schemes); `docs/ui.md` ("The data") names each field's.
    A part that draws from them follows them, and reads nothing again
    because they were written (`test_data_updates.py`).
    Who is in is `shared/me.ts`', the settings `state.settings`'.
    An answer that comes after a sign-out changes nothing:
    a part's request and a plain load take the session's number before it
    (`shared/session.ts`).
  - **The plain scripts** are the texts (`i18n.js`) and the theme
    (`theme.js`), applied before the page draws;
    the session, the sign-in by OPAQUE and through a provider, the first
    owner asked for, a running page held still behind the card the user
    opened, and the settings' pushes (`login.js`), with the providers'
    list (`providers.js`);
    the API, the settings, the loads, the stream and the page's start (`app.js`);
    what stays plain of the header: the radio budget, the server's clock,
    a Cube's link sign and word, the certificate's text and the version
    check (`header.js`);
    the home notice's facts and the Cubes found (`home.js`);
    the rooms' actions (`rooms.js`, `tiles.js`);
    the week programme editor (`week-program.js`) and its kit (`config.js`);
    the toast and the service worker (`notify.js`, `notifytext.js`, `sw.js`);
    and the connect page (`connect.js`), which clones the `<template>`s
    of `connect.html`.
    A plain script builds an element with the builders of `components.js`
    and writes no copy of one (the week programme editor's and the
    connect page's own inputs aside).
    It looks up the skeleton's ids alone (`TestUIPage`),
    and in the whole page only what every element of a kind shares,
    a part's too: the menus, one open at a time and closed by Escape
    and by a click outside (`components.js`), the help slots of the cards'
    heads (`app.js`), the static texts (`applyI18n`, which finds none in
    a part) and the brand's marks (`brandMarks`, which passes over one a
    part has drawn); `TestUIPage` refuses any other
    `document.querySelector` of a plain script.
    Anything else of a part it is handed by the part,
    or reaches through `window.MassimoParts`,
    which is also the UI tests' one handle.
  - **Both**:
    no inline handlers, no `innerHTML`,
    API text only as text (`textContent`, in a part a JSX text child).
    A label with its control is the one row style, `.field`,
    or `.field.wide` for a control on a line of its own.
    Every icon-only control is labelled, and every control keyboard operable.
    Every text's key is in all six
    `ui/i18n/<lang>.json` (FR/IT/NL/PL machine-translated and marked);
    CSS variables for light and dark, phone width first (390 px),
    verified with `make ui-test` at 1100 and 390 px.
- **Documents move with the code**:
  `README.md` for the command line,
  `docs/api-v1.md` and the OpenAPI file for every route,
  `packaging/` and `docs/update-windows.md` for the installers,
  `docs/ui.md` for the UI,
  `docs/roadmap.md` and the session handover, `GUIDELINES/wip/HANDOVER.md`
  (a standing document: rewritten in place, never dated),
  at the end of a session by the coordinating session only,
  but for a plan's items on the roadmap:
  the session whose plan retires prunes them itself, at the retirement.
- **Branches and pull requests**:
  develop on `main` through pull requests the owner merges;
  `release/alpha` is the release branch, only ever fast-forwarded,
  by the release once it is out (`{{guidelines_root}}/project/CI.md`, "Releases");
  `release/devel` and `release/nightly` are tags, moved by CI.
  At a line's feature freeze `make freeze <x.y>` cuts `maint/<x.y>` from `origin/main`;
  completions and fixes for x.y then target `maint/<x.y>`,
  every x.y release is cut there,
  and `main` opens for the next line.
  The session that merged into `maint/<x.y>`, or cut a release there,
  merges it upward into `main` with a `merge:` commit and pushes that
  straight away (AP MaintBranch, D1-A);
  an old maintenance branch releases nothing after the next freeze (D3-A).
  A branch merges `origin/main` before its pull request (merge,
  never rebase a stack),
  or `origin/maint/<x.y>` where that is its base.
  Commits are signed (SSH, the repository's git config);
  run `timeout 60 git commit …` because a locked key agent hangs instead of failing.
- **Hosts.**
  WSL 2 in NAT mode has no LAN broadcast and no inbound connections:
  `massimo discover`,
  the proxy and discovery answers from a `serve` in WSL do not work there;
  hardware work goes through the Raspberry Pi's `serve` or the Windows side.
  Windows executables run from WSL through interop and use Windows networking;
  elevation needs `gsudo` and the owner's UAC confirmation.
  The owner's Windows service, the Pi and the Mac are touched only on the
  owner's word in the session.

## 4. Document Governance

- This document follows the shared governance rules in `{{shared_root}}/GOVERNANCE.md`.

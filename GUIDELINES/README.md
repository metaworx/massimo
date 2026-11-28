<!-- GENERATED FILE - DO NOT EDIT.
     Source: project/_README.md + project/_CONTRACT.md + shared/_README.md
     Regenerate: GUIDELINES/shared/tools/sync-docs.sh
     Contract version: v1.1.0 -->


# Massimo (v1.1.0)

Massimo is open-source successor software for eQ-3 MAX! heating systems (the MAX!
Cube LAN gateway, radiator and wall thermostats, window contacts,
eco switches) after the vendor's end of life:
one Go binary per platform that keeps the Cubes connected,
serves a JSON API and a web UI,
answers as a Cube for the original software and other programs,
and backs up and restores a Cube.
It is for the owners of such systems and for the people and agents who develop it;
`README.md` at the root is the user's document.

## Working on it

```bash
git clone --recurse-submodules git@github.com:metaworx/massimo.git
cd massimo
make              # go test ./... and the binary
make cross        # every target, without cgo, into dist/
make cross-check  # every target built, none kept
```

Go 1.26 is the toolchain (`go.mod` says the minimum, CI pins 1.26).
Everything runs against the fake Cube: `./massimo fakecube -listen
127.0.0.1:62910 -greeting internal/protocol/cube/testdata/cube-fw0113-greeting.txt`
in one terminal, `MASSIMO_ENV_FILE=/dev/null ./massimo serve -addr
127.0.0.1:62910 -proxy off` in another, the web UI at
<http://127.0.0.1:8099/>. The UI smoke test needs Python with Playwright
once (`tools/ui-test/README.md`), then `make ui-test`.
Real hardware is reached through the `.env` file (gitignored; `README.md`,
"Cube variables") and only through the proxy of a running `serve`.

## The rules that apply here

The conventions below are not AI configuration:
they are how this project is written,
and an agent is simply another party required to follow them.

- The contract that follows is generated into `/AGENTS.md` as well,
  so people and agents are held to the same text.
- The shared conventions live in `GUIDELINES/shared` — commit policy,
  quality pass, per-language testing, linting and code style.
- `GUIDELINES/README.md` explains the gate protocol:
  what an agent asks before it changes anything,
  and what `EXEC` and its relatives authorise.

Both generated documents come from the fragments in `GUIDELINES`.
Editing a generated file is wasted work — change the fragment and re-run
`GUIDELINES/shared/tools/sync-docs.sh`.

---


# Massimo — Project Contract (v1.25.0)

What binds work in this project, for everyone working on it.
Inlined into `/AGENTS.md` for agents and into `GUIDELINES/README.md` for people,
so that the two cannot drift apart.

> `{{TODO: …}}` is a value you replace, braces and all.
> The other double-braced names, such as `Massimo`,
> are filled in when the documents are generated: leave them as they are.

## 1. Project Facts

> No absolute paths here.
> An agent is already inside the checkout,
> so the repository root is `git rev-parse --show-toplevel`;
> a checkout can live anywhere and this file is committed.
> Machine-specific values belong in `GUIDELINES/config.local.ini`,
> which is not tracked.

| Fact                  | Value |
|-----------------------|-------|
| Project name          | Massimo |
| Language(s)           | declared as `project.languages` in `GUIDELINES/config.ini`, kept honest by `GUIDELINES/shared/tools/detect-languages.sh --check` |
| Source directories    | `cmd/massimo/` (the command line and `serve`), `internal/` (the packages: `protocol/cube`, `cubelan`, `engine`, `server` with the web UI in `server/ui/`, `proxy`, `backup`, `store`, `update`, `install`, …), `packaging/` (unit files, plists, the installers' assets), `tools/ui-test/` (the Playwright smoke test), `tools/brand/` (the generator of the mark and every file made from it), `docs/` (the protocol findings, the API, the designs, the handovers) |
| Shipped-code paths    | declared as `project.shipped` in `GUIDELINES/config.ini` and listed below; kept honest by `GUIDELINES/shared/tools/changelog.sh check` (rule: `GUIDELINES/shared/COMMIT.md` §5) |
| Version manifest      | for the server none: the version is the nearest git tag (`git describe --tags`, baked in by `make`); for the Android app `app/version.json`, the changelog's manifest (`GUIDELINES/config.ini`), its versionCode counted by `packaging/app-version.sh` and written by `make release` into the release commit (AP MobileApp, D12-A); a release is cut with `make release <flavour>`, the flavour `alpha`, `beta`, `rc` or `stable` (`BUMP=minor` for a feature, `BUMP=patch` for a fix of a stable release, `DRY=1` to see it first; `make build-release` is the build) on a clean `main` equal to `origin/main`, or on a maintenance branch `maint/<x.y>` equal to its `origin` copy for a release of x.y once `make freeze <x.y>` has cut it: `changelog.sh cut` moves `[Unreleased]` into the release's section in a `chore(release):` commit, tags it with that section as the message, and the branch and the tag are pushed atomically; CI tests the tag, releases it and then fast-forwards `release/<flavour>` to it; `README.md`, "Versions" |
| Test gate command     | `make ui` first, the web UI's bundle the server embeds (every Makefile target that compiles Go runs it), then `MASSIMO_ENV_FILE=/dev/null go test ./...` (the gate of `GUIDELINES/shared/lang/go/TESTING.md`; `make race`, `-race` for `internal/server`, `internal/store`, `internal/cubelan`, `internal/proxy`, `internal/serve`, `internal/announce`, `internal/notify`, `internal/engine`, `internal/cubeio` and `internal/certs`, is recommended, and CI's Race job runs it), then `make cross-check` (every platform compiles and links, and no binary is written; `make cross` is the build into `dist/`), and `make ui-test` for a change under `internal/server/ui/` (the pytest suite of `tools/ui-test/`, needs the Playwright venv, `tools/ui-test/README.md`; one test with `PYTEST_ADDOPTS='-k "<test name> and 390-de"' make ui-test`), and `make lan-test` for a change to discovery, the mDNS announcement (`internal/announce`), `discover` or the proxy's discovery answers (Docker with Compose v2, `tools/lan-test/README.md`), and `make app-emulator-test` for a change under `app/` or to what of the server the app drives (the sign-in and the login by key, a user's tokens, the connect page), where KVM is open to the user (the Android SDK's pinned emulator and image and JDK 17, `tools/app-emulator/README.md`, "What it needs"); CI's job Android emulator runs it in any case |
| Lint command          | `gofmt -l .` prints nothing and `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...` exits 0, as `GUIDELINES/shared/lang/go/LINTING.md` requires per platform; `make actionlint` exits 0 for a change under `.github/` (actionlint pinned in `tools/actionlint/`, the `run:` scripts by shellcheck where it is installed, as CI's job is); `make ui-check` exits 0 for a change under `internal/server/ui/src/` (`tsc --noEmit`, then Biome's two hooks rules of `tools/ui-check/biome.jsonc`, TypeScript and Biome locked in `tools/ui-check/`, needs Node.js and npm, as CI's job UI types runs it); `govulncheck ./...` runs in CI for linux and windows; no other linter |
| Commit summary        | declared as `[commit]` in `GUIDELINES/config.ini` and shown below; held to it by `GUIDELINES/shared/tools/commit-msg.sh` (rule: `GUIDELINES/shared/COMMIT.md` §3.1) |

A commit touching any path listed below must carry a `CHANGELOG.md` bullet.
The list is generated from `project.shipped`,
so it cannot drift from what the check enforces - edit the config,
not this block:

<!-- BEGIN GENERATED SHIPPED-PATHS - do not edit; run "GUIDELINES/shared/tools/changelog.sh update-contract" -->
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
no test reads them, and `GUIDELINES/shared/tools/fingerprint.sh` leaves them out
of the code fingerprint a CI gate compares.
None of them is shipped.
The list is generated from `[ci] untested` - edit the config, not this block,
and delete both where the project declares no such list:

<!-- BEGIN GENERATED UNTESTED-PATHS - do not edit; run "GUIDELINES/shared/tools/fingerprint.sh update-contract" -->
- `GUIDELINES/wip/**`
- `GUIDELINES/project/**`
- `AGENTS.md`
- `GUIDELINES/README.md`
- `CHANGELOG.md`
- `CHANGELOG.d/**`
- `LICENSE`
<!-- END GENERATED UNTESTED-PATHS -->

The commit form is generated from `[commit]` in `GUIDELINES/config.ini` -
edit the config, not this block:

<!-- BEGIN GENERATED COMMIT-FORM - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
Every commit summary opens as `config.ini`'s `[commit]` keys ask (COMMIT.md §3.1):
`format = conventional`, `component = required`, `keywords = conventional`, `attribution = assisted-by`.

- **Form:** `type(component): headline`, for example `feat(api): Cache registry lookups`.
- **Component:** required, but of a release and a merge: a name of letters, digits, `.`, `_`, `/` and `-`; behind a role's token, after a colon: `fix(security:api): Close a hole`.
- **An incompatible change:** `feat(api)!: Remove a call`, or a `BREAKING CHANGE:` trailer.
- **A plan's own commit:** `chore(aps): +Name v1.0` (COMMIT.md §6).
- **An agent's commit:** ends with `Assisted-by: AGENT_NAME:MODEL_VERSION` - the tool or harness, then the exact model id, e.g. `Assisted-by: claude-code:claude-opus-5-5` (COMMIT.md §3.3).

| Role | Opens with | For |
|------|------------|-----|
| task | `feat:`, `docs:`, `refactor:`, `perf:`, `test:`, `build:`, `ci:`, `revert:` | features, logic changes, documentation updates |
| fix | `fix:` | bug fixes |
| security | `fix(security):` | security fixes and hardening |
| cleanup | `style:` | formatting and layout only, no functional change |
| wip | `wip:` | an intermediate state; a non-wip commit follows |
| update | `chore:` | environment, dependency or test-config updates, plans |
| release | `chore(release):` | the version bump, made by changelog.sh cut |
| merge | `merge:` | a commit with more than one parent |
<!-- END GENERATED COMMIT-FORM -->

## 2. Primary References

- `/AGENTS.md` — runtime behavior contract, gating flow, action-plan workflow.
- `GUIDELINES/shared/lang/<language>/TESTING.md` and `LINTING.md` — language
  baselines for each language in `project.languages`;
  this file's facts table overrides their example commands.
- `GUIDELINES/shared/COMMIT.md` — commit workflow, gating, message policy.
- `GUIDELINES/shared/ENVIRONMENTS.md` — host/agent command conventions.
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
  never to a Cube another program holds (`-no-direct` refuses the direct fallback,
  `-direct` forces it); a radio write costs duty cycle (about 1.25 % each,
  36 s of air time an hour),
  so nothing is sent to a real Cube without the owner's go.
- **Go code.**
  The Go of `go.mod`, the standard library first.
  `gofmt` and `go vet` clean, and no `//nolint` outside `third_party/`.
  Every file opens with the SPDX line `AGPL-3.0-or-later` and the copyright line;
  JavaScript and CSS carry a one-line licence comment.
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
  A flaky test's fix that is a pattern becomes a documented kit helper
  that every place of the pattern uses,
  the tests' front page (`docs/testing.md`) names it,
  and a local fix stays only where a sweep finds the pattern once.
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
  - **A part** is Preact with signals,
    in TSX under `internal/server/ui/src/<region>/`,
    bundled by `make ui` (esbuild as a Go tool,
    `tools/uibundle`) into `ui/parts.js`, which is not committed.
    Preact is vendored under `third_party/preact/`;
    no other library and no other build step.
    A part is built once per key and updated in place:
    data never replaces an element the user can hold.
    A field that shows stored data keeps a draft.
    A part that reads a signal reads everything it depends on as signals.
    A part gives the focus in the handler of the user's action (or in a timer
    that handler sets, where a listener of the page must run first:
    the click outside an open menu, which closes it), or in a layout effect,
    in the drawing that shows its place.
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
    then the `@file` header of `GUIDELINES/shared/lang/ts/LINTING.md` (`TestUIFileHeaders`);
    `make ui-check` is the parts' lint and `make ui-test` their test:
    they have no unit-test runner.
  - **The data are signals:**
    `state` of `app.js` and `themeState` of `theme.js` stay the plain scripts' objects,
    and `shared/data.ts` makes their fields signals.
    A field is written whole, never in place (`TestStateWrittenWhole`):
    by the loads of `app.js` (the Cubes, the rooms, the settings,
    a sign-out's clearing), and beside them by `home.js` (a Cube found),
    `rooms.js` (a device's state), `header.js` (the server's answer,
    a new version),
    `login.js` (who is in) and `theme.js` and the scheme editor (the colour schemes);
    `docs/ui.md` ("The data") names each field's.
    A part that draws from them follows them, and reads nothing again
    because they were written (`test_data_updates.py`).
    Who is in is `shared/me.ts`', the settings `state.settings`'.
    An answer that comes after a sign-out changes nothing:
    a part's request and a plain load take the session's number before it
    (`shared/session.ts`).
  - **The plain scripts** are the texts (`i18n.js`) and the theme (`theme.js`),
    applied before the page draws; the session,
    the sign-in by OPAQUE and through a provider, the first owner asked for,
    a running page held still behind the card the user opened,
    and the settings' pushes (`login.js`),
    with the providers' list (`providers.js`); the API, the settings, the loads,
    the stream and the page's start (`app.js`); what stays plain of the header:
    the radio budget, the server's clock, a Cube's link sign and word,
    the certificate's text and the version check (`header.js`);
    the home notice's facts and the Cubes found (`home.js`);
    the rooms' actions (`rooms.js`, `tiles.js`);
    the week programme editor (`week-program.js`) and its kit (`config.js`);
    the toast and the service worker (`notify.js`, `notifytext.js`, `sw.js`);
    and the connect page (`connect.js`),
    which clones the `<template>`s of `connect.html`.
    A plain script builds an element with the builders of `components.js`
    and writes no copy of one (the week programme editor's and the
    connect page's own inputs aside).
    It looks up the skeleton's ids alone (`TestUIPage`),
    and in the whole page only what every element of a kind shares,
    a part's too: the menus,
    one open at a time and closed by Escape and by a click outside (`components.js`),
    the help slots of the cards' heads (`app.js`),
    the static texts (`applyI18n`,
    which finds none in a part) and the brand's marks (`brandMarks`,
    which passes over one a part has drawn);
    `TestUIPage` refuses any other `document.querySelector` of a plain script.
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
  by the release once it is out (`GUIDELINES/project/CI.md`,
  "Releases"); `release/devel` and `release/nightly` are tags, moved by CI.
  At a line's feature freeze `make freeze <x.y>` cuts `maint/<x.y>` from `origin/main`;
  completions and fixes for x.y then target `maint/<x.y>`,
  every x.y release is cut there,
  and `main` opens for the next line.
  The session that merged into `maint/<x.y>`, or cut a release there,
  merges it upward into `main` with a `merge:` commit and pushes that
  straight away (AP MaintBranch, D1-A);
  an old maintenance branch releases nothing after the next freeze (D3-A).
  A plan's own commits (its registration, revisions, NOTEs,
  analyses and retirement) go onto the branch its work merges into:
  `maint/<x.y>` while any of its blocks ship in x.y, `main` otherwise,
  and reach `main` by the upward merge (D5-A);
  a plan open at the freeze is retired on `maint/<x.y>` where its work shipped in x.y.
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

- This document follows the shared governance rules in `GUIDELINES/shared/GOVERNANCE.md`.

---


# Working with the agent (v1.8.0)

This is the human half of the contract.
The agent's rules live in `/AGENTS.md`;
this explains what the agent will do to you, what the words it sends you mean,
and what it needs back.

## 1. The short version

The agent does not change your repository and then tell you.
For anything non-trivial it **stops and asks first**,
in a fixed format called a *gate message*, and waits.
You reply with one word — usually `EXEC` — and it proceeds.

Two moments always stop:
before a non-trivial code change (you approve the plan) and before every
`git commit` (you approve the message).
Everything else is ordinary work.

## 2. Gate messages

A gate message is the agent's request for permission.
It always carries its fields in the same order, so you can skim it;
the ones in italics appear only where they apply:

- **Checkpoint** — where the work stands
- **Last Action** — what it just did, in a line, with IDs you can check
- *Pending* — work the gate does not cover: an unpushed commit, a running job
- *CHANGELOG bullet*, *Proposed commit message*, *Files to be committed* —
  wherever a signal would commit
- *Open decisions* — numbered `D1`, `D2`, the options lettered,
  the recommended one first
- *Important notes* — numbered: what you need in order to decide
  and would not otherwise see
- **Confirmation needed** — each signal it offers,
  and everything that signal does

After sending one the agent must be silent until you answer.
If you see it send a gate and then keep working,
that is a violation of its own contract, and worth saying so.

Read **Confirmation needed** first, then the open decisions.
You can answer briefly - `EXEC+ D2-B`, or "as recommended".
A reply without a signal is taken as a question to weigh, and nothing is done.

## 3. Execution signals

One word, at the start or anywhere in your reply:

| Signal      | Means |
|-------------|-------|
| `EXEC`      | Do the gated action. Only that. |
| `EXEC+`     | Do it, then carry on with the next planned step. |
| `ROLLBACK`  | Undo the last edit. Required — the agent may not undo unasked. |
| `ERR`       | That went wrong; stop and diagnose before trying again. |
| `PLAN`      | Produce or revise the plan; do not execute yet. |
| `ASK`       | Answer my question; do not treat it as an instruction to act. |

The gate always tells you what `EXEC` will do and where `EXEC+` will go next.
If it doesn't, that is a defect in the gate, not something for you to infer.

`EXEC++` and `EXEC+++` are retired — say it in words instead.

You can also just answer in prose.
"Yes, but rename the second one first" is a perfectly good reply;
the signals exist for speed, not ceremony.

## 4. Action Plans

For anything beyond a trivial edit the agent writes an **Action Plan** before
touching code: what it found, what it intends to do step by step,
and the commit message it expects to propose.
You see a summary in the gate;
the full plan is written to a file (see §5) so it survives the conversation.

Plans are versioned — `AP Bild v1.2` — and each revision is a new file,
never an overwrite.
If you reject a plan, the next one is v1.3 and the reasoning that led there is
still on disk.

## 5. UAMF — where the agent's writing goes

A **User-Accessible Message File** is anything the agent wrote for you to read
rather than for the machine to run: action plans, analyses, migration notes.
They land in `GUIDELINES/messages/` with a timestamp in the name,
and they are **never overwritten** — a revision is always a new file.

This matters when a session ends badly.
The conversation may be gone; the reasoning is still there.

## 6. Recovering from a bad turn

Send `ERR`.
The agent stops and replies with a compact recovery block:
what it believes happened, what state things are in, and what it proposes.
Send `ERR` twice in a row and it escalates — it stops proposing and asks you to direct it.

`ROLLBACK` is separate and stronger: it authorises undoing the last edit.
The agent cannot do this on its own initiative, by design,
so if something needs reverting you have to say so.

## 7. What the agent must never do without you

- Commit — every commit is gated, with the full message shown first
- Undo your work — `ROLLBACK` only
- Overwrite a UAMF — revisions are new files
- Delete and recreate a file to make a large edit — it edits in place,
  so your IDE's local history survives
- Push, or anything else that leaves your machine, unless you asked

If any of these happens without your say-so, the agent has broken its contract.
Tell it; the rule it violated is in `/AGENTS.md` and it can name the section.

## 8. If this directory looks empty

Everything under `GUIDELINES/shared` lives in a git submodule,
which a plain clone leaves unpopulated.
The document you are reading is committed in the project,
so it survives that — but every link into `GUIDELINES/shared` will be dead until:

```bash
git submodule update --init GUIDELINES/shared
```

Cloning with `--recurse-submodules` does it up front.
In an overlay - a `GUIDELINES` that is a repository of its own,
in a project that does not carry the guidelines -
this document and the submodule are the overlay's,
and the command is run there:
`git -C GUIDELINES submodule update --init`.
If the same checkout is used from both Windows and WSL,
this saves a recurring annoyance:

```bash
git -C GUIDELINES/shared config core.fileMode false
```

## 9. Document Governance

- This document follows the shared governance rules in `GUIDELINES/shared/GOVERNANCE.md`.

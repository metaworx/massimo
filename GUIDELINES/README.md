<!-- GENERATED FILE - DO NOT EDIT.
     Source: project/_README.md + project/_CONTRACT.md + shared/_README.md
     Regenerate: GUIDELINES/shared/tools/sync-docs.sh
     Contract version: v1.0.0 -->


# Massimo (v1.0.0)

Massimo is open-source successor software for eQ-3 MAX! heating systems
(the MAX! Cube LAN gateway, radiator and wall thermostats, window
contacts, eco switches) after the vendor's end of life: one Go binary
per platform that keeps the Cubes connected, serves a JSON API and a web
UI, answers as a Cube for the original software and other programs, and
backs up and restores a Cube. It is for the owners of such systems and
for the people and agents who develop it; `README.md` at the root is the
user's document.

## Working on it

```bash
git clone --recurse-submodules git@github.com:metaworx/massimo.git
cd massimo
make            # go test ./... and the binary
make cross      # every target, without cgo
```

Go 1.26 is the toolchain (`go.mod` says the minimum, CI pins 1.26).
Everything runs against the fake Cube: `./massimo fakecube -listen
127.0.0.1:62910 -greeting internal/protocol/cube/testdata/cube-fw0113-greeting.txt`
in one terminal, `MASSIMO_ENV_FILE=/dev/null ./massimo serve -addr
127.0.0.1:62910 -proxy off` in another, the web UI at
<http://127.0.0.1:8099/>. The UI smoke test needs Python with Playwright
once (`tools/ui-test/README.md`), then `make ui-test`. Real hardware is
reached through the `.env` file (gitignored; `README.md`, "Cube
variables") and only through the proxy of a running `serve`.

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


# Massimo — Project Contract (v1.0.0)

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
| Source directories    | `cmd/massimo/` (the command line and `serve`), `internal/` (the packages: `protocol/cube`, `cubelan`, `engine`, `server` with the web UI in `server/ui/`, `proxy`, `backup`, `store`, `update`, `install`, …), `packaging/` (unit files, plists, the installers' assets), `tools/ui-test/` (the Playwright smoke test), `docs/` (the protocol findings, the API, the designs, the handovers) |
| Shipped-code paths    | declared as `project.shipped` in `GUIDELINES/config.ini` and listed below; kept honest by `GUIDELINES/shared/tools/changelog.sh check` (rule: `GUIDELINES/shared/COMMIT.md` §5) |
| Version manifest      | none: the version is the nearest git tag (`git describe --tags`, baked in by `make`); a release is cut with `make release-alpha` (`BUMP=minor` for a feature), which tags `main`'s head and fast-forwards `alpha`; `README.md`, "Versions" |
| Test gate command     | `gofmt -l .` prints nothing; `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...`; `MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`, `internal/cubelan`, `internal/proxy`); `make cross`; and `make ui-test` for a change under `internal/server/ui/` (needs the Playwright venv, `tools/ui-test/README.md`) |
| Lint command          | `gofmt -l .` and `go vet ./...`; there is no other linter |
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
- `:!docs/**`
- `:!tools/**`
- `:!GUIDELINES/**`
- `:!*.md`
- `:!.gitignore`
- `:!.gitattributes`
- `:!.gitmodules`
- `:!LICENSE`
<!-- END GENERATED SHIPPED-PATHS -->

The commit form is generated from `[commit]` in `GUIDELINES/config.ini` -
edit the config, not this block:

<!-- BEGIN GENERATED COMMIT-FORM - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
Every commit summary opens as `config.ini`'s `[commit]` keys ask (COMMIT.md §3.1):
`format = conventional`, `component = required`, `keywords = conventional`, `attribution = assisted-by`.

- **Form:** `type(component): headline`, for example `feat(api): Cache registry lookups`.
- **Component:** required, but of a release and a merge: a name of letters, digits, `.`, `_`, `/` and `-`; behind a role's token, after a colon: `fix(security:api): Close a hole`.
- **An incompatible change:** `feat(api)!: Remove a call`, or a `BREAKING CHANGE:` trailer.
- **A plan's own commit:** `chore(aps): +Name v1.0` (COMMIT.md §6).
- **An agent's commit:** ends with `Assisted-by: AGENT_NAME:MODEL_VERSION` - the tool or harness, then the exact model id, e.g. `Assisted-by: ClaudeCode:claude-opus-5-5` (COMMIT.md §3.3).

| Role | Opens with | For |
|------|------------|-----|
| task | `feat:`, `docs:`, `refactor:`, `perf:`, `test:`, `build:`, `ci:`, `revert:` | features, logic changes, documentation updates |
| fix | `fix:` | bug fixes |
| security | `fix(security):` | security fixes and hardening |
| cleanup | `style:` | formatting and layout only, no functional change |
| wip | `style(wip):` for a rename, `chore(wip):` otherwise | an intermediate state; a non-wip commit follows |
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

The coding standard at the end of `docs/review-2026-11-05.md` and the
"Conventions" of `docs/handover-session.md` are the rules; the ones an
agent trips over most:

- **Secrets and identities.** No keys, IVs, tokens, real device names or
  real Cube serials in code, fixtures, documents or commit messages;
  placeholders such as `KEQ0000000`. `.env`, backups and logs are
  gitignored. The checkout's `.env` points at real hardware, so tests
  and manual runs use `MASSIMO_ENV_FILE=/dev/null` or the fake Cube
  (`massimo fakecube`).
- **The Cube.** Protocol bytes are built only in `internal/protocol/cube`
  and tested against captured lines byte for byte; a command the
  hardware has not confirmed says so in its comment and in
  `docs/portal-protocol.md`. Hardware findings go into
  `docs/portal-protocol.md` the day they are made, with the date and the
  Cube firmware. A Cube serves one LAN client at a time: commands go
  through the proxy of a running `serve`, never to a Cube another
  program holds; a radio write costs duty cycle (about 1.25 % each,
  36 s of air time an hour), so nothing is sent to a real Cube without
  the owner's go.
- **Tests** run against the fake Cube with the recorded greeting; none
  needs hardware or the network. A new API route gets a test that
  checks the exact line sent to the fake Cube, and
  `TestOpenAPICoversRoutes` fails when `docs/api-v1-openapi.yaml` and the
  routes differ.
- **The web UI** is plain HTML, CSS and JavaScript under
  `internal/server/ui/`, embedded with `go:embed`: no build step, no
  framework, no inline handlers, API text only through `textContent`,
  every text through `i18n.js` (six languages, FR/IT/NL/PL machine-
  translated and marked), CSS variables for light and dark, phone width
  first (390 px), verified with `make ui-test` at 1100 and 390 px.
- **Documents move with the code**: `README.md` for the command line,
  `docs/api-v1.md` and the OpenAPI file for every route, `packaging/`
  and `docs/update-windows.md` for the installers, `docs/ui.md` for the
  UI, `docs/roadmap.md` and `docs/handover-session.md` at the end of a
  session by the coordinating session only.
- **Branches and pull requests**: develop on `main` through pull
  requests the owner merges; `alpha` is the release branch, only ever
  fast-forwarded by `make release-alpha`. A branch merges `origin/main`
  before its pull request (merge, never rebase a stack). Commits are
  signed (SSH, the repository's git config); run `timeout 60 git
  commit …` because a locked key agent hangs instead of failing.
- **Hosts.** WSL 2 in NAT mode has no LAN broadcast and no inbound
  connections: `massimo discover`, the proxy and discovery answers from a
  `serve` in WSL do not work there; hardware work goes through the
  Raspberry Pi's `serve` or the Windows side. Windows executables run
  from WSL through interop and use Windows networking; elevation needs
  `gsudo` and the owner's UAC confirmation.

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

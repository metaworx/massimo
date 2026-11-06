<!-- GENERATED FILE - DO NOT EDIT.
     Source: shared/_AGENTS.md + project/_CONTRACT.md
     Regenerate: GUIDELINES/shared/tools/sync-docs.sh
     Contract version: v3.26.0 -->


# AI Agent Guidelines (v3.26.0)

Core behavioral rules for AI agents working on this codebase.  
All agents MUST comply.
This document is intentionally concise;
refer to linked documents for extended guidance.

## Contents

<!-- BEGIN GENERATED CONTENTS - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
- AI Agent Guidelines (v3.26.0)
  1. Critical Behavioral Rules (STRICT)
    - 1.1 Gate Message Mechanism
    - 1.2 Action Plan (AP) Requirement
    - 1.3 Commit Confirmation Gate
    - 1.4 `undo_edit` Authorization (`ROLLBACK`)
    - 1.5 Safe File Edits
  2. Instruction Precedence (STRICT)
    - 2.1 Rule Map & Canonical Owners (STRICT)
  3. Task Interpretation & Keywords
    - 3.1 First Response Contract
  4. User-Accessible Message Files (UAMF)
  5. Action Plan (AP)
    - 5.1 Format & Versioning
    - 5.2 Required Sections
    - 5.3 Persistence
    - 5.4 Notes
  6. Commit Policy (STRICT)
  7. Additional References
  8. Document Governance
- Massimo — Project Contract (v1.0.0)
  1. Project Facts
  2. Primary References
  3. Project-Specific Conventions
  4. Document Governance
<!-- END GENERATED CONTENTS -->

## 1. Critical Behavioral Rules (STRICT)

### 1.1 Gate Message Mechanism

- After sending a gate message, the agent MUST **stop** — no further actions,
  shell commands,
  or status updates are permitted until the user responds with a new `<issue_update>`.
- What a gate message is, and the Pause Latch and No-Input rules,
  are `GUIDELINES/shared/GATE_WORKFLOW.md` §3.
- The gate message MUST follow the **Universal Gate Template**
  (`GUIDELINES/shared/GATE_WORKFLOW.md` §4), which owns its fields.
- The gate MUST state, in the user's terms,
  **what each signal it offers will do**.
  A signal the user cannot predict the effect of is not consent.
  Which signals a gate offers, `EXEC+` and variants of the gate's own included,
  is `GUIDELINES/shared/GATE_WORKFLOW.md` §6.
- The signals are listed as text, at the end of the gate message itself.
  A runtime that can end a turn only through a tool call, as Roo's can,
  carries a short label there and nothing more:
  the gate is the text, and the tool is not where the choice is explained.
- A gate message is **brief**: what the user needs in order to decide,
  in plain short language.
  How the work went is not retold; `GUIDELINES/shared/GATE_WORKFLOW.md` §4.
- Its open decisions and notes are **numbered**,
  each decision with a recommended option, so that a short answer can name them.
- A gate message is the **only** valid way to request user confirmation.
  Echoing "waiting for input" via shell commands is a **violation** of this rule.
- A visual workflow diagram is available in `GUIDELINES/shared/GATE_WORKFLOW.md`.

### 1.2 Action Plan (AP) Requirement

- Every non‑trivial `[CODE]` task requires an Action Plan.
- Full AP format and versioning rules are defined in [§5](#5-action-plan-ap).
- Each AP revision MUST be written in `GUIDELINES/wip/` (§5.1)
  **before** any project writes.
- Unless the execution is directly authorized in the same user message -
  by an `EXEC`‑family keyword, or by `PLAN++`, which clears the plan gate -
  the AP MUST be presented as a **gate message** (see [§1.1](#11-gate-message-mechanism)).

### 1.3 Commit Confirmation Gate

- Before `git commit`, the agent MUST present a commit gate.
  [`GUIDELINES/shared/COMMIT.md`](GUIDELINES/shared/COMMIT.md) §7.1 states what it
  must contain, and §7.2 the commands that follow it.
- A run under `EXEC++` or `PLAN++` commits under that signal instead,
  within the scope it was given (COMMIT.md §7.1),
  and so does a signal that includes committing the plan's own files (§5.1).

### 1.4 `undo_edit` Authorization (`ROLLBACK`)

- A rollback action (calling `undo_edit`) requires explicit user authorization.
  `GUIDELINES/shared/GATE_WORKFLOW.md` §7.3 states what authorizes it and what the
  gate must say where authorization is absent.

### 1.5 Safe File Edits

- **Do not delete and recreate files** when making large changes.
  Instead, write the new content to a temporary file and atomically replace the
  original (`mv` on Linux/macOS, `Move-Item` on Windows).
  This preserves local IDE history.
- Temporary files SHOULD be placed in `GUIDELINES/temp/`,
  which is scratch: it may be deleted wholesale at any time.
  User-visible artifacts belong in `GUIDELINES/messages/`,
  which is durable and never purged,
  or in `GUIDELINES/wip/` while a plan rests on them (§4).
  Why `temp/` and `messages/` are siblings is in `GUIDELINES/README.md`.
- Always prefer history‑preserving edit tools (e.g., `apply_patch`,
  in‑place edits) over raw shell writes.

## 2. Instruction Precedence (STRICT)

1. Runtime/system rules (conflicts noted explicitly).
2. Direct user instruction (current session).
3. This document (the shared agent contract).
4. Local conventions

**IMPORTANT:**

- Writing a *new* [UAMF](#4-user-accessible-message-files-uamf),
  or scratch to `GUIDELINES/temp/` (§1.5),
  does NOT constitute a source-file edit, and is ALLOWED under every signal.
- In `GUIDELINES/wip/`, any signal MAY add a NOTE or an analysis;
  all else there is for `PLAN+`, `PLAN++` and the `EXEC` family (§5.1).

### 2.1 Rule Map & Canonical Owners (STRICT)

| Need                                                                                      | Canonical location              |
|-------------------------------------------------------------------------------------------|---------------------------------|
| Gate lifecycle, execution signals, pause behavior, mixed-signal gate handling, `ERR` recovery, the Universal Gate Template | `GUIDELINES/shared/GATE_WORKFLOW.md` |
| Commit gate, commit message format and shape, changelog obligation, commit tooling | `GUIDELINES/shared/COMMIT.md`        |
| Test depth and commands                                                                   | `GUIDELINES/shared/lang/<language>/TESTING.md`       |
| Quality pass over changed source files, of any language                                   | `GUIDELINES/shared/QUALITY.md` |
| Lint/style commands and policy                                                            | `GUIDELINES/shared/lang/<language>/LINTING.md`       |
| Document governance rules and canonical history ownership                                 | `GUIDELINES/shared/GOVERNANCE.md`     |
| Agent-host command/tool-name conventions (Windows-hosted vs WSL-based, per-agent specifics) | `GUIDELINES/shared/ENVIRONMENTS.md`  |
| CI conventions: what the pipeline runs, in what order, and why                             | `GUIDELINES/project/CI.md` |
| AP requirements (`MUST`)                                                                  | this document §1.2 and §5      |

If overlap exists, follow the canonical owner document for that rule family.

## 3. Task Interpretation & Keywords

- **`ASK`** – Standalone: Send `<answer>` in [chat] mode.
  Combined with other keywords:
  include answer in new gate message (if task-relevant) or with the result of
  the gated action.
- **`PLAN`** – Investigate, and present the preliminary findings as a TL;DR gate
  (`GUIDELINES/shared/GATE_WORKFLOW.md` §7.1).
  It writes no plan; `PLAN+`, `PLAN++`, and `EXEC` on a task without one do.
  - **`PLAN+`** – Write the AP revision without a TL;DR gate (§5.1),
    and send the plan gate.
  - **`PLAN++`** – `PLAN+` with its gate cleared:
    it runs the scope the gate offering it stated,
    which the agent restates before it starts and adds nothing to;
    a retirement, a release or a push only where that gate stated it.
    Sent where no gate stated a scope, it runs the plan's blocks only.
- **`EVAL`** – Evaluate the proposals and questions in the message:
  weigh each, recommend with the reason,
  and say what is settled and what is still open.
  It writes no plan revision and resumes no work - mid-run included -
  whatever state the work is in:
  only an `EXEC`, `PLAN+` or `PLAN++` signal does.
  That is what makes a message carrying it safe to send.
  A long evaluation MAY go to a UAMF in `GUIDELINES/messages/`.
- **`EXEC`** – Execute gated action(s).
  The signals and their scope are `GUIDELINES/shared/GATE_WORKFLOW.md` §6.
  It commits the plan revision it runs under, and the NOTEs on it (§5.1).
- **`ROLLBACK`** – Authorize an `undo_edit` action (see
  [§1.4](#14-undo_edit-authorization-rollback)).
- **`ERR`** – Apply recovery protocol (detailed in `GUIDELINES/shared/GATE_WORKFLOW.md`).
- **`UAMF`** – Instructs agent to write a
  [UAMF](#4-user-accessible-message-files-uamf) message file.

Latest `<issue_update>` overrides earlier `<issue_description>`.

### 3.1 First Response Contract

- Non-trivial `[CODE]` task without inline `EXEC`:
  a TL;DR gate, as `PLAN` gives (§3);
  the plan is written when `PLAN+` or `PLAN++` asks for it.
- A reply to a plan gate that carries no keyword is an `EVAL`:
  its proposals and questions are weighed, and the plan is not revised
  until a `PLAN+` or `PLAN++` signal says so.
- If the same user message includes clear execution authorization (`EXEC` family):
  execute only the authorized scope.
  A non-trivial task with no plan yet gets one first,
  as `PLAN+` writes it (§5.1).
- Before `git commit`: always send a commit gate with full proposed commit message.

## 4. User-Accessible Message Files (UAMF)

A UAMF is a stored artefact, written for the user to read,
that follows strict rules:

- Its name begins with a timestamp, `YYYY-MM-DD_HH-NN_`,
  then describes its subject.
- It is **never overwritten**.
  A revision is a new file,
  named with the time it is written and the same subject,
  the subject ending `_v2`, then `_v3`;
  the previous one stays where it is.
- Within the agent turn that writes it, the agent MAY still edit it:
  to fill in a copy of the previous revision it started from,
  or to correct a mistake it notices after writing.
  That ends once work based on it has started,
  or it has been presented to the user.
- **An Action Plan is not a UAMF** - it is revised in place,
  its history git's (§5.1).
  Files in `messages/`, and NOTEs and analyses in `wip/`, are UAMFs.

Where a file is written depends on what it is and how long it needs to live:

| Directory | Tracked | For |
|-----------|---------|-----|
| `GUIDELINES/messages/` | no  | the default: analyses and findings written for the user |
| `GUIDELINES/wip/`      | yes | each open Action Plan (§5), its NOTEs (§5.4), and any analysis it rests on |
| `GUIDELINES/temp/`     | no  | scratch that is not a UAMF at all |

**An analysis in `wip/`** is named `YYYY-MM-DD_HH-NN_ANALYSIS_<Name>_<slug>.md`,
`<Name>` that of the plan it rests on.
One in `messages/` that a plan comes to rest on is copied there,
keeping its timestamp; the original stays.
Its fate at retirement is `GUIDELINES/shared/COMMIT.md` §6;
one kept for another plan keeps its name.

In an overlay - a guidelines root that is a repository of its own,
in a project that does not carry the guidelines -
tracked means tracked by the overlay,
and a plan is committed there (`GUIDELINES/shared/COMMIT.md` §7.3).

Citation rules follow from that, and they are absolute:

- **`messages/` is never cited.**
  Not from a document, not from a commit message, not from another UAMF.
  It is untracked: it exists in one working copy and nobody else can follow the reference.
- **`wip/` may be cited from another `wip/` document** — an Action Plan naming
  the analysis it rests on — **and from a commit message**,
  which is immutable and dated,
  so it names something that existed then and that git can still produce.
- **Nothing that outlives the change may cite either.**
  If a conclusion is worth citing from a contract,
  a shared document or a README, it belongs *in* that document.

## 5. Action Plan (AP)

### 5.1 Format & Versioning

- Title: `AP <Name> v<Major>.<Minor>: <2-5 word description>`,
  `<Name>` 1-3 PascalCase words naming its subject (e.g. `Bild`, `Mock`,
  `DecisionEngine`), never a signal of §3.
- Examples: `AP Bild v1.0: Extract decision functions`,
  `AP FilterFix v1.0: Fix type validation`.
- **One file:**
  `GUIDELINES/wip/YYYY-MM-DD_HH-NN_AP_<Name>_<slug>.md`,
  with the timestamp and slug of its first revision and no version in its name,
  so that a link to it, and a NOTE naming it, hold across its revisions.
  `<slug>` is lower-case words joined by `_`.
- **A name of its own:**
  commit messages cite a plan by `<Name>`, so it is one no retired plan used;
  `GUIDELINES/shared/tools/check-plans.sh` warns of one that was.
- **Revised in place** - each revision raises the version in the title
  and adds an entry to the append-only `Change History`;
  `git log -p -- <plan>` shows each revision's diff.
- **Minor** rises with every revision,
  **Major** where the plan changes direction or is replaced,
  its open points carried over.
  The same file then holds a new plan:
  a new TL;DR, Discussion, Analysis and Blocks,
  and a `Change History` restarted with an entry naming what it replaces
  and what it carries over.
  The commit carrying it retires the old plan's NOTEs.
  A new subject is a new plan, with a name of its own.
- **Committed** - the one statement of when:
  - `PLAN+` and `PLAN++` commit an uncommitted revision before overwriting it;
  - `PLAN++` and every `EXEC`-family signal commit the revision they run:
    the first revision of a plan of several blocks in an `[UPDATE] APs:` commit
    of its own, before the run's first project write; any other in one,
    or in the block commit that first names it (`GUIDELINES/shared/COMMIT.md` §6);
  - a NOTE is committed before a revision folds it in;
  - until then, a presented revision stays uncommitted.
- **Edited only before it is presented,** within the turn that writes it;
  after that, any change is the next revision.
  A typo MAY be corrected instead, with the user's consent -
  a variant the next gate offers, such as `EXEC typo`:
  in place in a revision not yet committed,
  amended into its commit where not yet pushed,
  that gate offering the rebase beside the next revision where it is not the last.
  Pushed, it waits for the next revision.
- A NOTE (§5.4) is not a revision, and leaves the version as it is.
- A plan opened before this rule - a file per revision,
  the version in its name - stays in that form until it retires:
  its next revision is a new file, timestamped when written,
  with its first revision's slug, and the link (§5.3) is to its newest.
  Its earlier revision files are committed, where they are not,
  before its next one is written.

### 5.2 Required Sections

- **TL;DR**, first after the title:
  what the plan does and how, for a reader with half a minute.
  - One or two sentences;
    then bullets in telegram style - keywords, `→` from before to after,
    no sentence where a phrase does;
    then at most a few short paragraphs, or one table.
    Unlike a commit's `Key changes` (`GUIDELINES/shared/COMMIT.md` §3.2),
    these bullets are scanned, not read.
  - It SHOULD stay within about 150 words.
  - It MUST include `### Blocks`:
    every block of the Implementation Plan, numbered as there, one line each,
    and outside that budget.
  - It summarises and adds nothing.
    Where it and the body differ, the body holds.
  - Each revision rewrites it for the plan as it then stands.
- **Discussion** (if any)
- **Analysis**
- **Implementation Plan** (step‑by‑step;
  include a **Verification** checkpoint after each logical block)
- **Proposed commit message** (for changes since the session start or last commit)
- **Change History** (all previous version entries)

### 5.3 Persistence

- While a plan is open, every message the agent sends starts with a link to the plan,
  and to its NOTEs, so that nobody has to look for it:
  the plan the message concerns,
  or each open plan where it concerns none in particular.
- Write each AP revision in `GUIDELINES/wip/` before modifying any project files.
  When it is committed is §5.1; which commit carries it,
  `GUIDELINES/shared/COMMIT.md` §6.
  A commit that cites a plan git does not have would be citing nothing.
- Registering, noting and retiring an AP - which commit carries each,
  when a plan may retire, what stays reachable afterwards,
  and the MUST NOT retire on the agent's own judgement -
  is `GUIDELINES/shared/COMMIT.md` §6.

### 5.4 Notes

A NOTE records a small change or a decision beside its plan,
without a new revision.

- Its **name** is `YYYY-MM-DD_HH-NN_NOTE_<Name>_v<X.Y>_<slug>.md`,
  in `GUIDELINES/wip/` beside the plan,
  where `<Name>` and `<X.Y>` are those of the plan revision it amends.
- Its **title** is `NOTE <Name> v<X.Y>: <slug words>`.
- It is **for** a decision, a measured fact,
  or a change to a step while the plan's blocks stand.
  Where the blocks change, it SHOULD be a new revision instead.
- It **records** where the work went differently from the plan,
  where that matters; the completion gate names it
  (`GUIDELINES/shared/GATE_WORKFLOW.md` §7.7).
  A run's deviations MAY be collected in one NOTE, committed at its end,
  rather than one at each step.
- It is a **UAMF** (§4), never overwritten,
  so a correction is another NOTE, with a slug of its own - never a `_v2`.
- Its **record** is a `File-revision-v1` trailer in the commit adding it,
  carrying the version its title names.
- It **stays** until its plan retires, or a Major revision replaces it (§5.1) -
  except a NOTE recording a **decision**,
  which MAY be folded into the plan's next revision:
  its substance in the body, its title in that revision's Change History entry,
  its file removed when the revision is written and the removal committed with it.
- A NOTE recording **how a run went** is never folded:
  it records how the plan was carried out, and the completion gate names it.

## 6. Commit Policy (STRICT)

The rules are [`GUIDELINES/shared/COMMIT.md`](GUIDELINES/shared/COMMIT.md),
and an agent committing here is held to all of them:
§2 one functional change with its tests, §3 the message and its shape,
§4 the rules a tag carries, §5 the `CHANGELOG.md` obligation, §7 the gate,
the signals and the commands.

## 7. Additional References

Paths below use two placeholders,
substituted when this document is inlined into a project's `AGENTS.md`:
`GUIDELINES` is the project's agent directory and `GUIDELINES/shared` is
the shared guidelines submodule.
Their values come from `GUIDELINES/config.ini`.

- **This document, below** – project facts: paths, versions,
  exact test and lint commands.
  They are inlined here from the project's `project/_CONTRACT.md`.
- `GUIDELINES/shared/COMMIT.md` – commit workflow, gating,
  message and changelog policy.
- `GUIDELINES/shared/GATE_WORKFLOW.md` – gate lifecycle and `ERR` recovery protocol.
- `GUIDELINES/shared/ENVIRONMENTS.md` – agent-host command and tool-name conventions.
- `GUIDELINES/shared/GOVERNANCE.md` – document versioning and history rules.
- `GUIDELINES/shared/QUALITY.md` – the quality pass to run over every changed source file.
- `GUIDELINES/shared/lang/<language>/TESTING.md`, `LINTING.md` and,
  where the language has one, `CLI.md` – language baselines.
- `GUIDELINES/shared/tools/README.md`,
  `GUIDELINES/shared/tools/RUNTIME_TOOLS.md` – helper scripts and runtime tools.
- `GUIDELINES/project/CI.md` – CI conventions, if the project has any.

## 8. Document Governance

- Version updated on every change (SemVer).
- History follows `GUIDELINES/shared/GOVERNANCE.md` §1.2: one line per version,
  in the commit that made it.
- Drift-check: when a specialized rule document changes ownership semantics,
  update §2.1 in the same change.

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

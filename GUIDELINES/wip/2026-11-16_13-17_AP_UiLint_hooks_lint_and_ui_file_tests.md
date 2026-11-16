# AP UiLint v1.0: Hooks lint, UI file tests

## TL;DR

The web UI's files are held to the rules the contract already states,
by two Go tests and by Biome's two hooks rules. Nothing about how the
code is laid out is checked.

- headers, licence lines → `TestUIFileHeaders`, over `ui/src/` and the
  plain `ui/*.js` and `ui/*.css`
- `innerHTML` and its kin, inline handlers, `data-i18n` in a part →
  `TestUINoHTMLInjection`
- hooks: unchecked → Biome's `useHookAtTopLevel` and
  `useExhaustiveDependencies`, in `make ui-check` beside `tsc`;
  `@biomejs/biome` locked in `tools/ui-check/`, no second TypeScript
- the counter's `use()` → `useLoad()`, a hook by its name, so that
  every call of it is checked
- CI: no new job; "UI types" runs the longer `make ui-check`
- the branch is cut after AP UiParts' Block 6 is merged (D7-A)

### Blocks

1. Tests: `TestUIFileHeaders` and `TestUINoHTMLInjection`, their rules
   in `docs/ui.md`
2. Lint: Biome in `make ui-check`, the counter as `useLoad()`, the
   findings settled, the contract, the documents

## Discussion

The owner asked, 2026-11-16, whether the ESLint configuration and the
header fixer of another project were worth taking over for the parts.
The answer was no as they stand: their rules are layout, the opposite
of the parts' (Allman braces, spaces inside parentheses, single
quotes), and the fixer cannot pass the SPDX lines. The owner chose
D1-B, a correctness-only lint, and D2-B, the headers as a Go test.

The session running AP UiParts ("UI refactor") reviewed the proposal
at the owner's request. Its points, all taken:

- the header test now, the lint after settling the tool, the counter's
  name and the moment;
- the bans as a Go test over the plain scripts and the parts alike,
  not a lint rule; `jsdoc/require-file-overview` dropped, the Go test
  checking more;
- `data-i18n` on a part's element banned (rule 3 of AP UiParts);
- `MarkUse` with `[blink]` harmless; `Island`'s keys its dependencies
  by design, never to be "fixed";
- the ban on one region importing from another stays with AP UiParts'
  Block 6, which adds the helper for it.

The second TL;DR gate offered D3 to D7; the owner answered
`PLAN+ as recommended`.

**Decisions:**

- **D1-B:** a lint of correctness rules only, no layout rules.
- **D2-B:** the headers held by a Go test, not by the shared
  `jsdoc-fix-header.mjs`.
- **D3-A:** Biome. 2 packages, locked beside TypeScript 7 in
  `tools/ui-check/`, a native binary; the only one of the three tried
  that flags the counter. (B, oxlint, missed the counter; C, ESLint,
  needed 162 packages and TypeScript 6.0.)
- **D4-A:** `MarkUse` lists `[blink]`; `Island` carries one
  `biome-ignore` comment with its reason. (B was a comment on
  `MarkUse` too.)
- **D5-B:** the tests cover the plain `.js` and `.css` files' licence
  lines too: no test holds them today. (A was `ui/src/` alone.)
- **D6-A:** `use()` becomes `useLoad()` in this plan's Block 2, with
  the rule that needs it. (B was AP UiParts' next block.)
- **D7-A:** the branch is cut after AP UiParts' Block 6 is merged:
  that block changes `ui_test.go`, `docs/ui.md` and `icon.tsx` too.
  (B was now, with a small conflict for the second pull request.)

## Analysis

### The three linters, tried on 2026-11-16

Over the 39 files under `internal/server/ui/src/` at main after #331,
and a planted file with hooks after an early return. The trials are in
`GUIDELINES/temp/eslint-try/` and `GUIDELINES/temp/native-try/`
(scratch).

| | ESLint 10.15 | oxlint 1.93 | Biome 2.6.8 |
|---|---|---|---|
| Packages | 162, and TypeScript 6.0: `typescript-eslint` 8.72 takes < 6.1 | 3; its launcher is Node | 2; a native binary |
| Time | 1.3 s | 0.2 s | 0.05 s |
| A hook after an early return | flagged | flagged | flagged |
| Hooks inside the counter's `use()` | not flagged | not flagged | flagged |
| `load.useLoad()` after an early return | not tried | not tried | flagged |
| Disable comment | `eslint-disable` | `eslint-disable` | `biome-ignore`, its reason required |

Biome's findings on the parts:

| File | Rule | Finding | Settled by |
|------|------|---------|------------|
| `shared/counter.ts:36-37` | `useHookAtTopLevel` | `use()` calls `useLang()` and `useIcons()` but is no hook by its name | D6-A: `useLoad()` |
| `shared/icon.tsx:34` | `useExhaustiveDependencies` | `MarkUse`'s effect does not list `blink` | D4-A: `[blink]` |
| `shared/island.tsx:25` | `useExhaustiveDependencies` | `fill` not listed; `lang` "more than necessary" | D4-A: `biome-ignore` with its reason |

`[blink]` (the review): the effect calls `keepBlink(element, blink)`;
`MarkUse` is keyed on its state, so the effect runs again only where
the same element comes to stand for another key, and then the new
key's phase is the right one.

`Island`: `fill` is a new function at every draw. Listed, it would fill
the island again at every poll and lose what the user holds in the
plain piece inside: the fault AP UiParts removes. `lang` is listed so
that the island is filled again in a new language; Biome cannot see
that `fill` reads it.

### The counter

`counter()` (`shared/counter.ts`) returns `{ bump, use }`; every part of
a region calls `load.use()` at the top of its drawing (5 calls on main:
`cubes/Form.tsx`, `Found.tsx`, `Panel.tsx`, `index.tsx` twice). A call
after an early return or in a branch misses data, rule 2 of AP UiParts.
A hook is known by its name, `use` and a capital letter; `use` alone is
none, so no linter checks its calls, and Biome flags the hooks inside
it. Named `useLoad`, the method is a hook, its calls are checked
(`load.useLoad()`, tried above), and the hooks inside it are allowed.

### The rules the Go tests hold

From the contract and `docs/ui.md`; no test holds them today:

- every `.ts` and `.tsx` under `ui/src/`, the `.d.ts` included: the SPDX
  line, then the copyright line, then a `/**` block whose `@file` names
  the file's own path from the repository root, then a description
  (the `@file` header of `GUIDELINES/shared/lang/ts/LINTING.md`);
- every `ui/*.js` and `ui/*.css`: the SPDX and copyright lines first,
  in either comment form (`//` or `/* … */`);
- no `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`,
  `dangerouslySetInnerHTML`, `setAttribute("on…")` in `ui/*.js` or under
  `ui/src/`; no `data-i18n=` attribute under `ui/src/`; no `on…=`
  attribute in `ui/index.html` or `ui/partials/`.

All files pass today (checked by script on 2026-11-16; no plain script
uses `innerHTML`, `data-i18n` appears under `ui/src/` only in a comment).
The tests read the files from the disk, as `TestSharedPartsKnowNoRegion`
does, skipping comments where a word in prose would match: `data-i18n`
is matched as an attribute, `data-i18n=`.

### Not in this plan

- `keepBlink` forgets a key it has not heard of for 60 s, so two marks
  of the same key made more than a minute apart blink out of step (the
  review: read in the code, not seen). A fix for the mark, for AP
  UiParts.
- No region imports from another: AP UiParts' Block 6.

## Implementation Plan

Branch `claude/uilint` from origin/main once AP UiParts' Block 6 is
merged, after the plan's `chore(aps): +UiLint v1.0` on main.

### Block 1: Tests

1. `internal/server/ui_test.go`, beside `TestSharedPartsKnowNoRegion`
   and using the helper Block 6 adds where it fits:
   - `TestUIFileHeaders`: the headers and licence lines of the Analysis,
     naming the file and the line that fails;
   - `TestUINoHTMLInjection`: the bans of the Analysis, naming the file,
     the line and the rule broken;
   - each fails when it finds no file, as the shared parts' test does.
2. `docs/ui.md`, "How the page is built": a line on each test, with the
   rules they hold.
3. Commit `test(server): The web UI's files keep their headers and insert no HTML`.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test -count=1 -run
'TestUIFileHeaders|TestUINoHTMLInjection' ./internal/server/` passes;
each check fails on a planted mistake (a part without its copyright
line, a wrong `@file` path, an `innerHTML` in a plain script, a
`data-i18n` in a part, an `onclick=` in a partial), the plant removed.

### Block 2: Lint

1. `refactor(ui)`: the counter's `use()` becomes `useLoad()`, its
   comment and the file's header saying why the name; every call
   follows.
2. `tools/ui-check/`: `npm install --save-exact --ignore-scripts
   @biomejs/biome@<latest>`; `biome.jsonc` beside it, its comment saying
   what it is for: the linter alone, no formatter, no assist, no VCS,
   `useHookAtTopLevel` and `useExhaustiveDependencies` as errors, no
   other rule; the current form of the configuration, no deprecated
   field. `README.md`: Biome beside TypeScript, how to update it.
3. `Makefile` `ui-check`: after `tsc`, `biome lint` over
   `internal/server/ui/src` with that configuration; its comment.
4. The findings: `MarkUse` `[blink]`; `Island` one
   `biome-ignore lint/correctness/useExhaustiveDependencies: <reason>`.
5. `.github/workflows/ci.yml`: the UI types job's comment names Biome;
   `.github/dependabot.yml`: the comment's "the one package there"
   names both.
6. `GUIDELINES/project/_CONTRACT.md` v1.14.0: the lint command's
   `make ui-check` names Biome's hooks rules; the web UI's "A part"
   bullet says the same; `GUIDELINES/shared/tools/sync-docs.sh` writes
   it into `AGENTS.md` and `GUIDELINES/README.md`.
7. `docs/ui.md` (near "`make ui-check` type-checks"),
   `GUIDELINES/project/CI.md` (the UI types job, the Dependabot
   paragraph).
8. `CHANGELOG.d/Changed/claude-ui-toolchain-ci.md` amended: the job UI
   types type-checks the parts and checks their hooks.
9. Commits: `refactor(ui): A region's counter is read with useLoad()`,
   `build(ui): make ui-check checks the parts' hooks with Biome`,
   `docs(contract): make ui-check checks the parts' hooks`.

**Verification:** `make ui-check` exits 0; it fails on the planted
file's two mistakes (a hook after an early return,
`load.useLoad()` in a branch), the plant removed; `make actionlint`;
`make ui`, `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make cross`,
`gofmt -l .`, `go vet` for linux, windows and darwin;
`make ui-test` for the touched regions' test files (the counter's
rename), as `tools/ui-test/README.md` runs one;
`GUIDELINES/shared/tools/check` before the push. Push the branch, open
the pull request for the owner.

## Proposed commit message

One per step, as in the Implementation Plan; the first:

    test(server): The web UI's files keep their headers and insert no HTML

    The contract asks every file of the web UI for its licence lines, a
    part for its @file header, and every file to insert no HTML and no
    inline handler; nothing held it but review (AP UiLint v1.0, Block 1).
    Two tests read the files from the disk, as the shared parts' test
    does, and name the file and the line that breaks a rule.

    Key changes:
    - internal/server/ui_test.go: TestUIFileHeaders holds the SPDX and
      copyright lines of ui/*.js, ui/*.css and ui/src/, and a part's
      @file header with its own path; TestUINoHTMLInjection holds
      innerHTML and its kin, inline handlers, and data-i18n in a part
    - docs/ui.md: the two tests in "How the page is built"
    - No CHANGELOG bullet: only tests change

    Assisted-by: claude-code:claude-opus-5-5

## Change History

- **v1.0** (2026-11-16): first revision, from the owner's question,
  the review of the session running AP UiParts, and the TL;DR gate's
  `PLAN+ as recommended` (D1-B, D2-B, D3-A, D4-A, D5-B, D6-A, D7-A).

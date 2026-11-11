# AP DocOnly v1.0: Skip the suite for documents

## TL;DR

A change that touches only documents no test reads runs the Guidelines
checks alone: What to test answers `code=false`, every other job is
skipped, and `main` makes no devel build of unchanged code.

- plan registration, NOTE, retirement: 15 min suite + devel build
  → What to test + Guidelines, about a minute
- the list, git pathspecs in What to test: `GUIDELINES/wip/**`,
  `GUIDELINES/project/**`, `AGENTS.md`, `GUIDELINES/README.md`,
  `CHANGELOG.md`, `CHANGELOG.d/**`, `LICENSE`
- guard: a changed path in `project.shipped` (config.ini) → `code=true`
- the range: a pull request's merge commit against its base; a push's
  `before..after`; a tag always `code=true`; a failure → everything runs
- `tools/identifiers` also in the Guidelines job, so a skipped Vet and test
  cannot let a serial through in CI.md or the changelog

### Blocks

1. The `code` answer in What to test, and the jobs that heed it
2. The identifiers check in the Guidelines job
3. Documents and the changelog fragment

## Discussion

The owner asked on 2026-11-11 to skip the tests for a pull request such as
`chore(aps): -ScrollSettle`, which changes only `GUIDELINES/wip/` and the
generated `AGENTS.md`, and which other paths could be ignored.

**The owner's decisions** (`PLAN++`, 2026-11-11, as recommended):

- **D43-A:** the set above. Not B, prose under `docs/` as well:
  a finer list to keep right, and Site would still have to run.
- **D44-A:** `go test ./tools/identifiers` as a step of the Guidelines job,
  always. Not B, only when `code=false`; not C, none.
- **D45-A:** a push to `main` follows the same rule, so no devel build
  of a commit whose binaries would not change. Not B, pull requests only.
- **D46-A:** a plan of its own, through a pull request.
  Not B, a NOTE on ParallelBuild.
- **D47-A:** the list lives in ci.yml's What to test, as git pathspecs,
  `project.shipped` read from config.ini as the guard.
  Not B, a config.ini key first: that is the guidelines' schema;
  the proposal of a key and a tool went to the "CLI scripts libraries"
  session to evaluate, and massimo moves to them once they are released.
  Not C, everything not shipped: `tools/` and `docs/` are not shipped,
  and tests read them.

## Analysis

**What reads each path.** Only the Guidelines job reads
`GUIDELINES/wip/` (plans, NOTEs, citations), the generated `AGENTS.md`
and `GUIDELINES/README.md` (checked against the contract they come from),
and the changelog (`changelog.sh check`). `tools/identifiers`, a test in
Vet and test, reads every tracked file but `GUIDELINES/wip/`, the
project's documents and changelog included; hence D44-A.

**Not in the set.** `docs/**`: `TestOpenAPICoversRoutes` reads
`docs/api-v1-openapi.yaml`, `docs/fragments/` feeds README's generated
parts, Site builds and checks the docs. `README.md`, `CONTRIBUTING.md`:
generated parts, Site, identifiers. `GUIDELINES/shared`: Site renders
with its `md_html.py`, the rolling notes use its `changelog.sh`.
`.gitattributes`, `.gitignore`: line endings and what `make` counts as
dirty. `tools/**`, `packaging/**`, `.github/**` and the code.

**The range.** A pull request's run checks out its merge commit, whose
first parent is the base: `HEAD^1..HEAD`, as for `ui` and `lan`. A push
to `main` holds one or more commits: `github.event.before..HEAD`, with the
history fetched (`fetch-depth: 0` on a push), an unknown `before` (all
zeros, or not in the history) meaning `code=true`. A tag is a release and
is always tested whole. The test is `git diff --quiet <range> -- .
':!GUIDELINES/wip/**' …`: git matches the pathspecs as `project.shipped`
is matched; then `git diff --quiet <range> -- <project.shipped>` must find
nothing, the list and the shipped paths being disjoint by intent.

**The jobs.** Every test job, Site and Build add
`needs.changes.outputs.code != 'false'` to their condition; Guidelines
keeps `suite` alone; All tests passed runs, and passes with the jobs
skipped; Release waits for Build, which is skipped, and is skipped too.
The required checks pass a skipped job; Guidelines is one of them and
runs. A gate that fails answers nothing, so everything runs, as now.

**The nightly build.** It takes the newest commit of `main` whose All tests
passed succeeded; a doc-only commit's did, its code being that of the
commit before it, which was tested.

## Implementation Plan

### Block 1: The code answer

1. What to test: the checkout for every event but a release commit's push,
   `fetch-depth` 2 for a pull request and 0 for a push; the `code` output.
2. The test jobs, Site and Build: `&& needs.changes.outputs.code != 'false'`.

**Verification:** the gate's script, run locally for a pull request and a
push with only `GUIDELINES/wip/` changed (`false`), with a Go file
(`true`), with `AGENTS.md` and `docs/ui.md` (`true`), for a tag (`true`);
shellcheck; ci.yml parses.

### Block 2: The identifiers check

1. The Guidelines job: `setup-go`, then
   `MASSIMO_ENV_FILE=/dev/null go test ./tools/identifiers`.

**Verification:** the PR's Guidelines job runs it.

### Block 3: Documents

1. `GUIDELINES/project/CI.md`, section 2: the list, why each other path is
   not in it, the guard, the nightly build.
2. `CHANGELOG.d/Changed/doc-only.md`.

**Verification:** reflow; `changelog.sh check`; `go test ./...`.

## Proposed commit message

```
perf(pipeline): A change to documents alone runs no tests

Key changes:
- What to test answers code=false when a pull request or a push to main
  changes only GUIDELINES/wip/, GUIDELINES/project/, the generated
  AGENTS.md and GUIDELINES/README.md, the changelog or LICENSE, and no
  path of project.shipped; the test jobs, Site and Build are skipped
- the Guidelines job runs tools/identifiers itself
- CI.md section 2 states the list and why the other paths are not in it

Plan: AP DocOnly v1.0 (Blocks 1-3)

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-11): first revision, D43-A to D47-A.

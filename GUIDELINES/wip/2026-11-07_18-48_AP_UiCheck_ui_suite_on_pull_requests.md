# AP UiCheck v1.0: The UI suite on pull requests

## TL;DR

CI's Screenshots job takes about 7½ minutes on every pull request.
Its pull-request runs are made shorter, while main keeps the full run:
- it runs only for a pull request that changes the web UI, its suite or `ci.yml`;
- when it does run there, it runs two of the suite's four cases.

- The time → the UI suite (370-400 s: 51 tests × 4 cases), not the screenshot set (33 s).
- Pull request → a `changes` job checks the paths; Screenshots needs it and is skipped on a `false`.
- Paths (D1-B) → `internal/server/ui/**`, `tools/ui-test/**`, plus `.github/workflows/ci.yml` (D4).
- Cases on a pull request (D2-A) → 1100-en and 390-de, 102 of 204, about 3½ min.
- Screenshot set (D3-A) → taken whenever the job runs.
- Push to main → unchanged: every job, all four cases.
- Required check "Screenshots" → a job skipped by its condition counts as passed; the ruleset is unchanged.
- Path check failed → the job runs; only a `false` skips it.

### Blocks

1. UiCheck: the `changes` job, the Screenshots job's condition,
   and the cases on a pull request in `ci.yml`; `CI.md` (v1.11.0)
   and a `CHANGELOG.md` bullet; verified on the pull request's own run.

## Discussion

The TL;DR gate of 2026-11-07 (evening) was answered `PLAN++ D1-B`;
D2 and D3 take the recommended option.
The gate's three blocks, `ci.yml`, then `CI.md` and the bullet,
then the pull request, are one block here:
`changelog.sh check` wants the bullet in the commit that touches `.github/**`,
and `CI.md` holds the reasons for what the YAML does,
so the three files are one change;
the pull request is the block's verification.

### D1. Which changes run the job on a pull request: B

`internal/server/ui/**` and `tools/ui-test/**`,
the paths for which the contract asks for `make ui-test`.
A change to the API behind the page that breaks it shows up on main's run
of the merge commit, which still runs everything.

### D2. The cases on a pull request: A

1100-en and 390-de: both widths and both languages, half the cases.
Main runs all four.

### D3. The screenshot set on a pull request: A

Taken whenever the job runs: 33 s,
and the artifact `ui-screenshots` is what a review of a UI change looks at.

### D4. `ci.yml` among the paths (raised for the completion gate)

D1-B names two paths; `.github/workflows/ci.yml` is added,
so that a change to the job itself is tested on its pull request,
and Block 1's verification, the job on this plan's own pull request, holds.
Without it, a broken Screenshots job would first fail on main.
To be dropped if the owner prefers D1-B as worded.

## Analysis

### 1. Where the time goes

The four newest green runs of `ci.yml` (three pull requests, one push to main),
the Screenshots job:

| Step | Seconds |
|------|---------|
| Initialize containers (the Playwright image) | 19-20 |
| Smoke test (`tools/ui-test/run.sh`, the pytest suite) | 370-401 |
| Screenshots (`run.sh screenshots`) | 32-33 |
| Everything else | about 15 |
| The job | 442-472 |

The next slowest job, Vet and test, takes 232-315 s,
so on a pull request the Screenshots job sets the run's length.
The suite is 51 tests, each run for four cases (`conftest.py`, the `case`
fixture: 1100-en, 390-en, 1100-de, 390-de), 204 in all, about 1.9 s each.

### 2. The required check

The ruleset on main requires the check "Screenshots"
(`gh api repos/metaworx/massimo/rules/branches/main`).
GitHub reports a job skipped by its `if:` as passed for a required check.
A workflow-level `paths:` filter would not run the workflow at all,
and the required check would wait forever, so the condition is on the job.

### 3. The changed paths

A pull request's run checks out its merge commit (`refs/pull/<n>/merge`),
whose first parent is the base branch's tip:
`git diff --name-only HEAD^1 HEAD` is what the pull request changes,
with `fetch-depth: 2`.
The list is captured before it is matched, so that a `grep -q` ending the pipe
early cannot turn a match into a failure under `pipefail`.
A failing step leaves the output empty, and the job then runs:
its condition skips it only on `false`.
A push to main skips the `changes` job;
the Screenshots job's `!cancelled()` lets it run after a skipped need.

### 4. The cases on a pull request

`PYTEST_ADDOPTS` reaches pytest through `run.sh` (its header says so).
`-k "1100-en or 390-de"` selects 102 of the 204 cases, every test once per case
(`pytest --collect-only`, 2026-11-07).
No test skips by its case, so none drops out on a pull request.

## Implementation Plan

### Block 1: UiCheck

1. `.github/workflows/ci.yml`:
   - a job `changes` ("Changed paths"), for pull requests only,
     with the output `ui`, `true` when the pull request changes
     `internal/server/ui/`, `tools/ui-test/` or `.github/workflows/ci.yml`;
   - the Screenshots job `needs: changes`, with
     `if: ${{ !cancelled() && (github.event_name != 'pull_request' || needs.changes.outputs.ui != 'false') }}`;
   - the Smoke test step's `PYTEST_ADDOPTS`: `-k "1100-en or 390-de"`
     on a pull request, empty on a push;
   - the comments on the triggers and on the job say so.
2. `GUIDELINES/project/CI.md` v1.11.0:
   section 2, the table's row and the paragraph on the Screenshots job.
3. `CHANGELOG.md`: a bullet under Changed.

**Verification:**
the workflow parses (`python3 -c 'import yaml; yaml.safe_load(...)'`);
`GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`;
the pull request's own run: `changes` answers `true` (it changes `ci.yml`),
the Screenshots job runs its smoke test on 102 cases,
and every other job is as before.
The skip itself shows on the next pull request that changes none of the paths.

## Proposed commit message

```text
ci(workflows): run the UI suite on a pull request only for the UI, on two of its four cases

The Screenshots job took about 7½ minutes on every pull request, 6½ of them for the UI suite's 204 cases; a pull request now runs it only when it changes internal/server/ui/, tools/ui-test/ or ci.yml, and then on the cases 1100-en and 390-de, while every push to main still runs all four (AP UiCheck v1.0, Block 1).
```

## Change History

- v1.0 (2026-11-07): the first revision, from the TL;DR gate of the same evening;
  D1-B settled by `PLAN++ D1-B`, D2-A and D3-A as recommended,
  D4 (`ci.yml` among the paths) raised for the completion gate.

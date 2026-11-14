# AP CiReport v1.0: say which job failed

## TL;DR

All tests passed names each test job with its result,
so that a failure no longer has to be found by counting positions
in a list of bare results; on a pull request it says so in one comment it keeps up to date.

- `results: success … abandoned …` → a table of each job's name and result in the job's summary
- a job that did not pass → an error annotation naming it
- a job no runner took → "cancelled before a runner took it", not "cancelled"
- a pull request → one comment, written at the first failure, edited by every later run, "passed" when green
- a push to `main`, a tag → the summary and annotations alone (D2-A)
- the verdict → still `needs`, unchanged; the report never fails the job

### Blocks

1. The report: the step, its permissions, CI.md, the fragment

## Discussion

The owner asked on 2026-11-14, after pull request #225's All tests passed failed with
`results: success success … abandoned success success`,
whether the job could say in a PR comment which job failed, and how,
and what happens without a pull request.
The macOS LaunchDaemon job had been cancelled twice by GitHub:
no hosted runner took it ("The job was not acquired by Runner of type hosted
even after multiple attempts"), with no step run.

**The owner's decisions**, 2026-11-14, `PLAN++` with:

- **D1-B.** The summary table and the annotations on every run,
  and on a pull request one sticky comment, edited in place on each run.
- **D2-A.** Without a pull request, the summary and annotations alone:
  GitHub's failure email already reaches whoever pushed.
- **D3-A.** The names from the run's jobs through the API (`actions: read`),
  which also tell a job no runner took from one cancelled while running.

**Refined here:**

- The comment is **written at the first run that fails**, and from then on edited
  by every run, "passed" included;
  a pull request that never fails gets none, so a green pull request stays quiet.
- **Which jobs** the report lists: every job of the run but All tests passed itself
  and the jobs of the workflows it calls (Build, Release, Deploy site),
  which are not among its `needs` and partly run after it.
  The API knows a job by its name only, not by its id in `ci.yml`,
  so these three are named in the step; CI.md says so, for whoever adds a fourth.
- **The verdict** stays the `needs`' results, in the step that fails the job, as today.
  Where the API cannot be read, the report falls back to the `needs`' ids and results,
  and a failed comment (Dependabot's and a fork's read-only token) is a warning.

**Not in this plan:** reporting the jobs of Build and Release,
and an issue or commit comment for `main` (D2-B, D2-C).

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-14,
whose gate stated: the plan written, registered on main, built on a branch,
committed, pushed and the pull request opened, without a merge or a retirement.
Branch `claude/ci-report`.

## Analysis

**Today** (`.github/workflows/ci.yml`, job `tests`): one step prints
`join(needs.*.result, ' ')` and exits 1 unless each is `success` or `skipped`;
a second writes the Tested status. Permissions: `statuses: write`.

**The run's jobs** (`GET /repos/{repo}/actions/runs/{id}/jobs?filter=latest`)
give each job's display name, matrix jobs expanded (`govulncheck (linux)`),
its status and conclusion, its runner's name and its steps;
`filter=latest` takes a re-run attempt's jobs and the ones it carried over.
A job no runner took is `cancelled` with no runner name and no step,
as macOS LaunchDaemon was on #225 (attempt 2: 17:00:50 to 17:15:51 UTC, no steps).
The called workflows' jobs are named `Build`, `Release`, `Deploy site`
when skipped as a whole, and `Build / <job>` and so on when they run.

**After Block 1**, a new first step of `tests`, `continue-on-error`:

1. The jobs from the API, less the excluded ones, each with a result in words:
   passed, failed, skipped, timed out, cancelled,
   "cancelled before a runner took it", or the status of one not completed;
   those that did not pass first, then by name.
   Where the API gives nothing, the `needs`' ids and results.
2. The summary (`$GITHUB_STEP_SUMMARY`): a heading with the verdict and the table.
3. An `::error::` annotation for each job that did not pass.
4. On a pull request: the comment that starts with `<!-- ci-report -->`
   and was written by `github-actions[bot]` is edited;
   none yet and the verdict a failure, one is written.
   It names the head commit, the run's attempt and its link,
   and the jobs that did not pass.

Permissions of the job: `actions: read`, `pull-requests: write` beside `statuses: write`.
The job checks out no code, so the write reaches only `gh api` calls of the step.

| File | Change |
|------|--------|
| `.github/workflows/ci.yml` | the step, the permissions, the job's comment |
| `GUIDELINES/project/CI.md` | v1.50.0: the job's row in the table, and why the report is so |
| `CHANGELOG.d/Changed/claude-ci-report.md` | the fragment |

## Implementation Plan

Nothing reaches GitHub but the plan's registration, the branch and the pull request.
One functional commit, citing `AP CiReport v1.0, Block 1`.

### Block 1: The report

1. `ci.yml`: the step and permissions, as in the Analysis; the job's comment.
2. `CI.md`: the table's row for All tests passed; a paragraph beside
   "The ruleset on `main` requires All tests passed and Guidelines alone":
   the report, the excluded jobs, the comment's life, the read-only token.
3. `CHANGELOG.d/Changed/claude-ci-report.md`:

   ```
   - CI's All tests passed names each test job with its result, a job
     no runner took told apart, and on a pull request keeps one comment
     saying which jobs did not pass.
   ```

**Verification:**
`make actionlint` exits 0;
the step's script, taken from `ci.yml`, run locally against run 43463002754 of #225
(read only, no pull request number, so no comment) prints the table with
"macOS LaunchDaemon: cancelled before a runner took it" first
and its annotation, and against a green run of `main` the table alone;
the comment lookup read against #225's comments;
the guidelines check (`GUIDELINES/shared/tools/check`);
the pull request's own run shows the summary.
The comment's write is first seen at a pull request's failure.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +CiReport v1.0

All tests passed prints its jobs' results without their names, so a failure is found by counting positions, and a job no runner took reads as cancelled. The owner decided on 2026-11-14 to name each job with its result and, on a pull request, to keep one comment saying which did not pass. PLAN++ CiReport, with D1-B, D2-A and D3-A.

Key changes:
- GUIDELINES/wip/2026-11-14_15-28_AP_CiReport_say_which_job_failed.md: the plan, one block

File-revision-v1: GUIDELINES/wip/2026-11-14_15-28_AP_CiReport_say_which_job_failed.md::v1.0::2026-11-14::All::the first revision, by PLAN++ CiReport
Assisted-by: claude-code:claude-opus-5-5
```

Block 1: `ci(pipeline): All tests passed names each job that did not pass, and says so on the pull request`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's D1-B, D2-A and D3-A.

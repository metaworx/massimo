# AP UiMatrix v1.1: split the UI job by width

## TL;DR

The Screenshots job runs the web UI's suite in its four cases one after
another in four workers; it is the run's critical path. A matrix runs
each width, with both languages, on a runner of its own, so a pull
request's CI takes a little over half the time, and a failure names its
width.

- Screenshots job → a matrix of two legs, "UI test (1100 px,
  screenshots)" and "UI test (390 px)", each the suite in two cases,
  in four workers (D2-B, D3-A)
- `UI_TEST_CASES` → `conftest.py` takes the cases from it, all four
  without it; `make ui-test` unchanged
- the screenshot set → in the 1100 px leg after its suite (D1-A)
- the run → about 13 min → about 7½
- the legs' artifacts named by their width

### Blocks

1. Cases: `UI_TEST_CASES` in `conftest.py`, `run.sh`'s header,
   `tools/ui-test/README.md`
2. Matrix: the job in `ci.yml` as two legs, `GUIDELINES/project/CI.md`

## Discussion

The owner's question of 2026-11-14: could the suite, run in every
language, be split so that its parts run in parallel on CI. It runs in
two languages, not six: every test at 1100 and 390 px, in English and
German, four cases (`conftest.py`'s `case` fixture); the other four
languages are checked for their keys only, by Go tests. `PLAN+ split`
asked for this plan; the owner's EVAL of v1.0 settled its decisions, and
`PLAN++` asked for this revision and its run:

- D1-A: the screenshot set in one leg, after its suite;
- D2-B: one leg per width, each with both languages, two legs;
- D3-A: the legs named "UI test (…)", with "screenshots" in the name of
  the leg that takes the set: "UI test (1100 px, screenshots)" and
  "UI test (390 px)".

**Grouping in the run's graph**, asked beside the decisions: GitHub's
graph shows jobs, never steps; a matrix draws as one node that opens to
its legs, a called workflow (Build, Release, Deploy site) as a box of its
jobs. The two legs are such a node. Grouping other jobs (the Go tests,
the platforms) would only tidy the graph, and splitting the Windows and
macOS jobs into jobs would set up a runner for each phase; neither is
part of this plan.

## Analysis

**The job now** (`ci.yml`, `screenshots`): the Playwright container,
Go, pip, then `tools/ui-test/run.sh` (744 runs: 186 tests × 4 cases,
four pytest-xdist workers with `--dist loadfile`, each against a server
set of its own), then `run.sh screenshots`. On pull request run
43627577873 of 2026-11-14: the job 12:40, of which the suite 11:13
(pytest: 664 s), the set 0:52, setting up 0:30. The whole run took
13:04; the next longest jobs were "Vet and test" 5:14, macOS 4:48,
the Debian packages 4:25. On a push to main the job is skipped when
the pull request's run passed the same code (`ui_done`).

**A width per leg.** Each leg runs 372 runs in four workers: about half
the suite's 664 s, about 5½ min where the files share out evenly;
`--dist loadfile` keeps a file in one worker, so the largest file sets
the floor. With 0:40 of setting up, a leg takes about 6 min, the 1100 px
one about 7 with the set: the run about 7½ min, set by that leg. Where
the 390 px leg turns out the slower, the set moves there, and the names
with it; the NOTE records the times.

**Selecting the cases.** Every test takes `page`, whose context takes
`case` (`browser_context_args`), so every test runs in the four cases
and no test is lost by narrowing the fixture's params. `-k 390`
would select by the test's id as well, which a test name could match;
the fixture's own list is exact. `UI_TEST_CASES` holds the case ids,
comma-separated, so a leg passes `1100-en,1100-de`; an unknown id stops
the run (pytest usage error), so a typo in the matrix fails the leg
instead of running nothing.

**What does not change.** "All tests passed" needs `screenshots`;
`needs.screenshots.result` of a matrix is `success` only where every
leg passed, so the gate and the Tested status's `ui=` read it as now.
Its report takes the names from the run's jobs, which list each leg by
its name. The required checks are "All tests passed" and Guidelines
only, so no ruleset changes. `run.sh` retries the failed tests of its
own leg and fails a flaky one, as now.

**The cost.** The repository is public: the minutes cost nothing.
A run has 15 jobs; with one more, 16 run at once, under the 20 of the
free plan. The legs set up two containers instead of one (0:30 each).

## Implementation Plan

Every commit cites `AP UiMatrix v1.1, Block <n>`; its documents and
CHANGELOG fragment, where `changelog.sh` asks for one, are in the same
commit.

### Block 1: Cases

- `tools/ui-test/conftest.py`: `CASES`, the four `(width, lang)` pairs
  by id; the `case` fixture's params those named in `UI_TEST_CASES`
  (comma-separated ids), all four where it is empty or unset; an
  unknown id raises `pytest.UsageError` naming the known ones.
- `tools/ui-test/run.sh`: `UI_TEST_CASES` in the header's environment
  list.
- `tools/ui-test/README.md`: how to run some cases
  (`UI_TEST_CASES=390-en,390-de make ui-test`).

**Verification:** `UI_TEST_CASES=390-de make ui-test` collects 186
runs, all `390-de`; `UI_TEST_CASES=1100-en,1100-de` collects 372;
`UI_TEST_CASES=bogus` exits with the usage error; without it, 744.

### Block 2: Matrix

- `ci.yml`, job `screenshots`: `strategy: fail-fast: false` and a
  matrix of two entries (`include`), each with its `width`, its
  `cases` (`1100-en,1100-de`, `390-en,390-de`), its `label`
  (`1100 px, screenshots`, `390 px`) and `shots` (true for 1100 only);
  `name: UI test (${{ matrix.label }})` (D3-A); `UI_TEST_CASES:
  ${{ matrix.cases }}` on the suite's step; the set's step and its
  artifact `ui-screenshots` only where `matrix.shots` (D1-A);
  `ui-test-results` becomes `ui-test-results-${{ matrix.width }}`;
  the job's comment.
- `GUIDELINES/project/CI.md`: the table's entry, the paragraph on "the
  run's longest" job with the measured times, and the section on the
  container where it names the job; the document's version and history.
- `make actionlint`.

**Verification:** `make actionlint`; on the pull request's run, two
legs, each 372 passed, the set's artifact from the 1100 px leg only,
"All tests passed" green, and the run's and the legs' times taken into
this plan's NOTE; one leg failed on purpose (a pushed commit with a
failing assertion, then dropped) shows "All tests passed" failing and
naming that leg.

## Proposed commit messages

- `chore(aps): +UiMatrix v1.0` and `chore(aps): +UiMatrix v1.1`, this
  plan, on main.
- Block 1, `test(ui-test): The suite runs the cases UI_TEST_CASES names`.
- Block 2, `ci(pipeline): The UI suite runs each width in a job of its own`.

## Change History

- v1.0 (2026-11-14): first revision, by `PLAN+ split`.
- v1.1 (2026-11-14): the owner's EVAL of v1.0, by `PLAN++`: D1-A, D2-B
  (a leg per width, not per case), D3-A with "screenshots" in the name
  of the leg that takes the set; the grouping of jobs in the run's graph
  answered in the Discussion, not part of the plan.

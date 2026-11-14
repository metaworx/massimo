# AP UiMatrix v1.0: split the UI job by case

## TL;DR

The Screenshots job runs the web UI's suite in its four cases one after
another in four workers; it is the run's critical path. A matrix runs
each case on a runner of its own, so a pull request's CI takes about
half the time, and a failure names its case.

- Screenshots job → a matrix of four legs, `1100-en`, `390-en`,
  `1100-de`, `390-de`, each the suite in one case, in four workers
- `UI_TEST_CASES` → `conftest.py` takes the cases from it, all four
  without it; `make ui-test` unchanged
- the screenshot set → in one leg after its suite (D1-A)
- the run → about 13 min → about 6, "Vet and test" (5 min) then
  the longest job
- the four legs' artifacts named by their case

### Blocks

1. Cases: `UI_TEST_CASES` in `conftest.py`, `run.sh`'s header,
   `tools/ui-test/README.md`
2. Matrix: the job in `ci.yml` as four legs, `GUIDELINES/project/CI.md`

## Discussion

The owner's question of 2026-11-14: could the suite, run in every
language, be split so that its parts run in parallel on CI. It runs in
two languages, not six: every test at 1100 and 390 px, in English and
German, four cases (`conftest.py`'s `case` fixture); the other four
languages are checked for their keys only, by Go tests. `PLAN+ split`
asked for this plan.

Open decisions:

1. **D1: where the screenshot set is taken**
   - **A (recommended): in the `1100-en` leg, after its suite.** As now,
     only once the suite passed in that leg; that leg takes about 50 s
     longer than the others, still under "Vet and test".
   - B: a job of its own beside the legs, not waiting for a suite: one
     more container to set up, one more need of "All tests passed",
     and a set taken where a test fails.
2. **D2: how fine the split is**
   - **A (recommended): one leg per case, four legs.** The cases are
     already the suite's unit; each leg has its own runner with four
     CPUs for its four workers.
   - B: one leg per width, each with both languages: two set-ups less,
     the run about two minutes longer.
3. **D3: the legs' name**
   - **A (recommended): "UI test (1100-en)" and so on.** The job does
     more than the screenshots, and "All tests passed" names a failed
     leg by it; the job's id stays `screenshots`, so its needs and the
     Tested status's `ui=` do not change.
   - B: keep "Screenshots (1100-en)".

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

**A case per leg.** Each leg runs 186 tests in four workers: about a
quarter of the suite's 664 s, about 3 min where the files share out
evenly; `--dist loadfile` keeps a file in one worker, so the largest
file sets the floor. With 0:40 of setting up, a leg takes about 4 min,
the `1100-en` one about 5 with the set: the run about 6 min, set by
"Vet and test".

**Selecting the case.** Every test takes `page`, whose context takes
`case` (`browser_context_args`), so every test runs in the four cases
and no test is lost by narrowing the fixture's params. `-k 390-de`
would select by the test's id as well, which a test name could match;
the fixture's own list is exact. `UI_TEST_CASES` holds the ids,
comma-separated; an unknown id stops the run (pytest usage error), so
a typo in the matrix fails the leg instead of running nothing.

**What does not change.** "All tests passed" needs `screenshots`;
`needs.screenshots.result` of a matrix is `success` only where every
leg passed, so the gate and the Tested status's `ui=` read it as now.
Its report takes the names from the run's jobs, which list each leg by
its name. The required checks are "All tests passed" and Guidelines
only, so no ruleset changes. `run.sh` retries the failed tests of its
own leg and fails a flaky one, as now.

**The cost.** The repository is public: the minutes cost nothing.
A run has 15 jobs; with three more, 18 run at once, under the 20 of
the free plan, but a second pull request's run at the same time queues
some legs. The legs set up four containers instead of one (0:30 each).

## Implementation Plan

Every commit cites `AP UiMatrix v1.0, Block <n>`; its documents and
CHANGELOG fragment, where `changelog.sh` asks for one, are in the same
commit.

### Block 1: Cases

- `tools/ui-test/conftest.py`: `CASES`, the four `(width, lang)` pairs
  by id; the `case` fixture's params those named in `UI_TEST_CASES`
  (comma-separated ids), all four where it is empty or unset; an
  unknown id raises `pytest.UsageError` naming the known ones.
- `tools/ui-test/run.sh`: `UI_TEST_CASES` in the header's environment
  list.
- `tools/ui-test/README.md`: how to run one case
  (`UI_TEST_CASES=390-de make ui-test`).

**Verification:** `UI_TEST_CASES=390-de make ui-test` collects 186
runs, all `390-de`; `UI_TEST_CASES=1100-en,390-en` collects 372;
`UI_TEST_CASES=bogus` exits with the usage error; without it, 744.

### Block 2: Matrix

- `ci.yml`, job `screenshots`: `name: UI test (${{ matrix.case }})`
  (D3-A), `strategy: fail-fast: false, matrix: case: [1100-en, 390-en,
  1100-de, 390-de]`, `UI_TEST_CASES: ${{ matrix.case }}` on the suite's
  step; the set's step and its artifact `ui-screenshots` only where
  `matrix.case == '1100-en'` (D1-A); `ui-test-results` becomes
  `ui-test-results-${{ matrix.case }}`; the job's comment.
- `GUIDELINES/project/CI.md`: the table's entry, the paragraph on "the
  run's longest" job with the measured times, and the section on the
  container where it names the job; the document's version and history.
- `make actionlint`.

**Verification:** `make actionlint`; on the pull request's run, four
legs, each 186 passed, the set's artifact from `1100-en` only, "All
tests passed" green, and the run's and the legs' times taken into this
plan's NOTE; one leg failed on purpose (a pushed commit with a failing
assertion, then dropped) shows "All tests passed" failing and naming
that leg.

## Proposed commit messages

- `chore(aps): +UiMatrix v1.0`, this plan, on main.
- Block 1, `test(ui-test): The suite runs the cases UI_TEST_CASES names`.
- Block 2, `ci(pipeline): The UI suite runs each case in a job of its own`.

## Change History

- v1.0 (2026-11-14): first revision, by `PLAN+ split`.

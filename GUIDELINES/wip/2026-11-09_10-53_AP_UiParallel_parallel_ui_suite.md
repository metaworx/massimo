# AP UiParallel v1.0: Parallel web UI suite

## TL;DR

The web UI suite runs in four pytest-xdist workers,
each against a server set of its own that `run.sh` starts,
and without a DOM-snapshot trace on the first pass:
the failed tests run once more with it.

- 436 runs: serial with the trace 26:12 → 4 workers without it 3:26 (16 cores)
- CI on 4 CPUs: push 17 min → about 4:21; a pull request 9 min → about 4:21, all four cases
- UDP 23272 → `serve -discover-listen 127.0.0.1:0`: no skip, nothing run afterwards
- `UI_TEST_WORKERS` (default 4; 1 = today's serial run); `-k` and `PYTEST_ADDOPTS` unchanged
- trace on every test → off; `--lf` with the trace into `results/retry/`; the run still fails
- three tests that race the page → fixed
- `run.sh screenshots` → one server set, serial, as today

### Blocks

1. DiscoverListen: `serve -discover-listen` sets the discovery socket's address,
   a test, the README, the bullet
2. Workers: `run.sh` starts N server sets, checks they are its own,
   runs xdist by file; conftest maps a worker to its set; the discovery skips go;
   `requirements.txt`, the suite's README
3. TraceRetry: no trace on the first pass, the failed tests once more with it;
   the three races fixed
4. CiWorkers: the Screenshots job runs all four cases in four workers,
   on a push and on a pull request; `CI.md`, the bullet

## Discussion

The owner asked for a parallel runner of the web UI suite (`tools/ui-test`),
after an EVAL in the AptUpdate session on 2026-11-09:
the suite's 440 runs took 26:12 on the WSL box at load 8.5,
the CI Screenshots job 10 to 18 minutes.

**The owner's decisions:**

- **D4-A, D5-A, D6-A** (2026-11-09, the EVAL): shards by case in `run.sh`;
  the tests needing UDP 23272 marked and run serially afterwards;
  a plan of its own, in this session.
- **"re-evaluate pytest-xdist and D5-B"** (2026-11-09), during the investigation:
  re-opened D4 and D5.
- **`PLAN++`** (2026-11-09) on the TL;DR gate, naming no option:
  the recommended ones, **D4-B, D5-B, D7-A and D8-A**:
  - **D4-B:** pytest-xdist with `--dist loadfile`, a server set per worker;
  - **D5-B:** a `serve` flag for the discovery socket's address;
  - **D7-A:** no trace on the first pass, the failed tests once more with it;
  - **D8-A:** CI runs all four cases in four workers, on a pull request too.

**Constraints the owner set:**
the four cases share one server, and the tests within a worker stay serial;
`run.sh` keeps reading `UI_TEST_API_PORT`, `UI_TEST_CUBE_PORT`,
`UI_TEST_DISCOVERY_PORT` and `UI_TEST_DOWN_PORT`;
`run.sh screenshots` stays serial;
the flake risk of the 5 to 10 s waits is measured,
and what the worker count does to it is said (Analysis, "Load").

**Names chosen here**, routine and not the owner's decisions:

- the flag `serve -discover-listen <addr>`, `serve.Config.DiscoveryListen`,
  passed on as `server.Options.DiscoveryAddr`, which exists;
  no settings-file variable, since it is for tests
- `UI_TEST_WORKERS`; the per-set variables `UI_TEST_URL_<n>`,
  `UI_TEST_DOWN_URL_<n>` and `UI_TEST_SERVE_LOG_<n>` between `run.sh` and conftest
- the ports of set n: the API's and the second server's `+ 2n`,
  the fake Cube's and its discovery's `+ n`
- the branch `claude/ui-parallel`

**Not in this plan:**
splitting a file's tests across workers (`--dist load`),
which would break the files whose tests depend on their order;
a trace without DOM snapshots (D7-B);
running fewer cases locally.

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-09.
Nothing reaches a Cube: the suite runs against the fake Cube.

## Analysis

### Measured on 2026-11-09

A scratch prototype outside the repository started N server sets as `run.sh`
starts one and ran the 436 runs of origin/main at that time,
with the discovery tests serially afterwards (16 to 46 s, included).
The 16-core WSL box; "4 CPUs" is `taskset -c 0-3`,
which matched CI: serial with the trace, the pull request's two cases,
9:15 here against CI's 8:43 to 9:00.

| 436 runs (min:s) | trace | failures | no trace | failures |
|---|---|---|---|---|
| serial | 26:12 (the owner's) | – | 11:34 | 2 |
| 4 case shards | 10:29 | 0 | – | – |
| xdist, 4 workers | 7:29 | 0 | 3:26 | 2 |
| xdist, 8 workers | 4:56 | 0 | – | – |
| 4 CPUs, xdist 4 | 8:23 | 0 | 4:21 | 3 |
| 4 CPUs, the PR pair, serial | 9:15 | 0 | 5:40 | 1 |
| 4 CPUs, the PR pair, xdist 2 | – | – | 3:06 | 0 |

The shards wait for their slowest case: with the trace the 1100 px cases
took 10:27 and the 390 px ones 7:04.
Without it the four cases take about 180 s each.

**The trace.** pytest-playwright's `--tracing retain-on-failure` records every
test with DOM snapshots (`snapshots=True`, not configurable)
and keeps the trace of a failed one.
A probe in System settings at 1100 px:
an `evaluate` and a locator cost 11 ms without the trace,
14 ms with screenshots only, 840 to 890 ms with snapshots
(79 ms at 390 px, the same 2171 nodes).
The suite's `wait_until` polls with `evaluate`,
so the trace slows every wait and every step, the wide cases most.

**Load.** With the trace there was no failure in six runs,
up to 8 workers and 16 servers at once,
so the 5 to 10 s waits held under that load and more workers added no failures.
Without the trace the failures were the same few tests in serial runs too:
three tests race the page, and the trace's slowness hid it.

- `test_opaque.py`, `test_passwords_registered_in_the_browser`:
  after the second password it waits for `#users-msg` shown and `ok`,
  which the add's "<name> created." already is (4 failures in 5 runs);
- `test_cubes.py`, `test_name_saved_and_taken_back`:
  counts `.cube-differs` (and finds `.take-file`) right after the state changed,
  before the panel is drawn again;
- `test_restart.py`, `test_sign_in_offers_the_key_after_an_open_server`:
  `askLogin()` right after `dialog.close()` returns the dialog's promise
  still pending, since `loginPromise` is cleared by the `close` event.

So the worker count does not raise the flake risk measured here;
removing the trace does, until the three races are fixed (Block 3).

### pytest-xdist

`-n N --dist loadfile` sends all the tests of a file to one worker, in order,
so a file's tests that build on each other stay as they are.
Worker `gw<n>` reads `PYTEST_XDIST_WORKER`;
conftest, imported first in every worker, copies `UI_TEST_URL_<n>`,
`UI_TEST_DOWN_URL_<n>` and `UI_TEST_SERVE_LOG_<n>` into the plain names
before anything reads them (`URL`, `TOKEN` and the test files' own imports).
The controller process keeps set 0's, which it does not use.
`-k`, from the command line or `PYTEST_ADDOPTS`, selects as before:
xdist distributes what remains.
pytest-xdist 3.8 and execnet 2.1 are pytest-dev's, MIT;
pytest-playwright 0.10 supports xdist,
and deletes the `--output` directory at the session's start.

Case shards (D4-A) would need a `-k` of their own,
and the last `-k` wins (checked):
a shard's would drop CI's and the README's.

### The discovery socket

`server.Options.DiscoveryAddr` exists, defaults to `:23272`,
and eight Go tests set it to `127.0.0.1:0`;
`discoverOnce` and the proxy's responder bind it.
The fake Cube answers a discovery request to the sender's address,
so a server on `127.0.0.1:0` gets its answers.
With `-discover-listen 127.0.0.1:0` on every server set,
no set needs 23272, two sets do not compete for it,
and the tests that skipped where another program held it can fail instead.
A real Cube answers only to UDP 23272, and the original software asks there:
the flag's help says it is for tests.

### `run.sh`

- `UI_TEST_WORKERS` (default 4), N server sets for the suite,
  one for `run.sh screenshots`;
  1 runs pytest without xdist, as today.
- Set n: the fake Cube on `UI_TEST_CUBE_PORT + n` with its discovery on
  `UI_TEST_DISCOVERY_PORT + n`, the server on `UI_TEST_API_PORT + 2n`,
  the second server on `UI_TEST_DOWN_PORT + 2n` (default `UI_TEST_API_PORT + 1`),
  each set in a directory of its own, both servers with
  `-discover-listen 127.0.0.1:0`, the operator and the guest added.
  The defaults keep set 0 on today's ports.
- A set is ready when its server's log has `api listening` with its own
  address and the fake Cube is connected;
  then every process started must still run.
  Today, where another session's server holds the port,
  `run.sh` tests that server, which takes the same token.
- Without pytest-xdist and N > 1:
  `UI_TEST_REQUIRED=1` fails, else a line says so and one set runs serially.
- No trace on the first pass (`pytest.ini` drops `--tracing retain-on-failure`;
  the screenshot and the video of a failed test stay).
  On exit 1, the failed tests run once more with
  `--lf --tracing retain-on-failure --output results/retry`
  and the same arguments;
  `run.sh` says whether they failed again and exits 1 either way.

### CI

The Screenshots job installs `requirements.txt`, which gains `pytest-xdist`,
and drops the pull request's `PYTEST_ADDOPTS` filter:
four workers on the runner's 4 CPUs, all four cases,
about 4:21 for the suite against 8:43 to 9:00 for a pull request's two cases
and 17 minutes for a push's four.
`ui-test-results` uploads `tools/ui-test/results/`, `retry/` with it.

### What changes, by file

| File | Block | Change |
|------|-------|--------|
| `cmd/massimo/serve.go` | 1 | `-discover-listen` |
| `internal/serve/serve.go`, `serve_test.go` | 1 | `Config.DiscoveryListen` → `DiscoveryAddr`; a search through it |
| `README.md`, `docs/api-v1.md` | 1 | the flag beside `-discover-target`; the 503 |
| `tools/ui-test/run.sh` | 2, 3 | the sets, the check, xdist; the retry |
| `tools/ui-test/conftest.py` | 2 | a worker's set |
| `tools/ui-test/test_discover.py`, `test_cubes.py` | 2 | the skips become failures |
| `tools/ui-test/requirements.txt`, `README.md` | 2, 3 | `pytest-xdist`; the workers, the ports, the retry |
| `tools/ui-test/pytest.ini` | 3 | no trace |
| `tools/ui-test/test_opaque.py`, `test_cubes.py`, `test_restart.py` | 3 | the races |
| `.github/workflows/ci.yml` | 4 | no filter, the comment |
| `GUIDELINES/project/CI.md` | 4 | the job's cases and time |
| `CHANGELOG.md` | 1, 4 | the flag; the job |

### Risks

- A shared venv: `pytest-xdist` goes into
  `/home/mdr/projects/massimo/tools/ui-test/.venv`, which every session uses;
  it does nothing without `-n`.
- Four times the ports: two sessions running the suite at once need ports
  apart by more than `2 × UI_TEST_WORKERS`; the readiness check stops a run
  that would test another session's server.
- Other races may remain that no run has shown yet;
  the retry with the trace is where they would show first.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`make ui-test` for Blocks 2 to 4 (free ports, the shared venv),
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`.
One commit per block, citing `AP UiParallel v1.0, Block <n>`.

### Block 1: DiscoverListen

1. `internal/serve/serve.go`: `Config.DiscoveryListen`, passed as
   `server.Options.DiscoveryAddr`.
2. `cmd/massimo/serve.go`: `-discover-listen <addr>`, its help naming 23272
   as the Cubes' port and the flag as one for tests.
3. `internal/serve/serve_test.go`: an instance on `127.0.0.1:0` with a fake
   discovery responder as its target finds KEQ0000000 through
   `POST /api/v1/cubes/discover`.
4. `README.md`, the discovery paragraph; `docs/api-v1.md`, the 503.
5. CHANGELOG, Added: `serve -discover-listen`.

**Verification:** the test gate; `massimo serve -help` lists the flag.

### Block 2: Workers

1. `run.sh`: the sets, the readiness and liveness check, `UI_TEST_WORKERS`,
   xdist's `-n` and `--dist loadfile`, the fallback, `-discover-listen`;
   its header comment.
2. `conftest.py`: a worker's set, in its docstring too.
3. `test_discover.py`, `test_cubes.py`: the skips for a missing discovery answer
   become assertions; the docstrings.
4. `requirements.txt`: `pytest-xdist>=3.6`; installed in the shared venv.
5. `tools/ui-test/README.md`: the workers, the ports of a set, the recipes.
6. No CHANGELOG bullet: `tools/**` is exempt.

**Verification:** the test gate; `make ui-test` with 4 workers and with
`UI_TEST_WORKERS=1`, all passing but the races Block 3 fixes;
a port held by a stand-in server stops the run with its name.

### Block 3: TraceRetry

1. `pytest.ini`: `--tracing retain-on-failure` dropped; the comment.
2. `run.sh`: the retry with the trace into `results/retry/`, the line saying
   whether the failed tests failed again, exit 1.
3. `test_opaque.py`: the wait names the "<name> saved." message.
4. `test_cubes.py`: `.take-file` and the empty `.cube-differs` waited for.
5. `test_restart.py`: `loginPromise` cleared before `askLogin()` again.
6. `tools/ui-test/README.md`: the retry and where its traces are.
7. No CHANGELOG bullet: `tools/**` is exempt.

**Verification:** the test gate; `make ui-test` three times with 4 workers,
no failure; a test failing on purpose (scratch, not committed) gives a trace
under `results/retry/` and exit 1.

### Block 4: CiWorkers

1. `.github/workflows/ci.yml`: the Smoke test step without `PYTEST_ADDOPTS`;
   the comments say four workers and all four cases.
2. `GUIDELINES/project/CI.md`: section 2's job table and the Screenshots
   paragraphs, section 3's job notes.
3. CHANGELOG, Changed: the Screenshots job runs all four cases, in four
   workers, on a pull request too.

**Verification:** the test gate; the pull request's Screenshots job,
its time against today's.
Then the branch merges `origin/main` where it moved, is pushed and gets its
pull request.

## Proposed commit message

The plan's registration, a commit of its own on main before the first project write:

```
chore(aps): +UiParallel v1.0

The web UI suite runs in four pytest-xdist workers, each against a server set of its own, without a DOM-snapshot trace on the first pass.

Key changes:
- GUIDELINES/wip/2026-11-09_10-53_AP_UiParallel_parallel_ui_suite.md: the plan, four blocks, the owner's PLAN++ with D4-B, D5-B, D7-A and D8-A, and the measurements behind them.

File-revision-v1: GUIDELINES/wip/2026-11-09_10-53_AP_UiParallel_parallel_ui_suite.md::v1.0::2026-11-09::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
`feat(serve): serve -discover-listen sets the discovery socket's address`,
`test(ui-test): The suite runs in workers, a server set each`,
`test(ui-test): No trace on the first pass, the failed tests again with it`,
`ci(screenshots): All four cases in four workers, on a pull request too`.

## Change History

- v1.0 (2026-11-09): the first revision, from the owner's EVAL decisions
  D4-A, D5-A and D6-A, re-opened by "re-evaluate pytest-xdist and D5-B",
  and the owner's `PLAN++` with D4-B, D5-B, D7-A and D8-A.

# AP CleanRuns v1.0: Runs clean up after themselves

## TL;DR

The test and build targets stop leaving their output in every checkout:
one shared install for the type check, a cross check that writes nothing,
named work directories that a later run can sweep, and clean targets for the
rest (`wip/2026-11-25_17-12_ANALYSIS_CleanRuns_cleanup_after_runs.md`).

- `tools/ui-check/node_modules` per checkout (64 MB) → one install per lock
  and platform in the main checkout's `GUIDELINES/temp/ui-check/`
- the gate's `make cross` (`dist/`, ~80 MB) → `make cross-check`, to `/dev/null`
- UI work dir `tmp.*` → `massimo-ui-test.*` with its PID; dead runs' swept
- `results/` of an earlier UI run → emptied at the start
- `KEEP=1` → keeps a suite's work dir
- new `make clean-tests`, `make distclean`; `make clean` as it is
- what CI uploads → never removed

### Blocks

1. SharedCheck: one `ui-check` install for every checkout; its README, CI.md
2. CrossCheck: `make cross-check` in the gate and CI; the contract, CI.md
3. WorkDirs: the suites' named work dirs, the UI sweep, `KEEP=1`; READMEs
4. CleanTargets: `make clean-tests` and `make distclean`; README.md

## Discussion

### What is decided

The owner, 2026-11-25, `PLAN++` on the EVAL gate of the same day,
every decision taking its recommended option:

- **D1-A (as revised in the EVAL).**
  One install of TypeScript and Biome per lock file and platform,
  in the main checkout's `GUIDELINES/temp/ui-check/<hash>-<platform>/`,
  found through `git rev-parse --git-common-dir`;
  installed in a temporary directory and renamed into place;
  touched at each use, and one unused for 30 days removed.
  `UI_CHECK_CACHE` names another directory; with no git,
  or where the main checkout cannot be written,
  the run installs into its own `tools/ui-check/node_modules`, as before.
- **D2-A.**
  `make cross-check` builds every platform of `make cross` with
  `go build -o /dev/null`, writing nothing; the test gate,
  its documents and CI's Vet and test take it;
  `make cross` stays the build into `dist/`.
- **D3-A.**
  `run.sh` of the UI suite empties `results/` at the start of a suite's run;
  its work directory is `massimo-ui-test.XXXXXX` under `TMPDIR`,
  holding its PID,
  and a run removes those whose run is gone and that are older than a day.
- **D4-A.**
  `KEEP=1` keeps the work directory of the UI,
  LAN and emulator suites and prints its path.
  Cleaning is the default locally and in CI: nothing CI reads is removed.
- **D5-A.**
  `make clean` stays as it is, since `build-release` runs it first;
  `make clean-tests` removes both suites' `results/`,
  `.pytest_cache/` and `__pycache__/`; `make distclean` removes, beside both,
  what make installed in the checkout: `tools/ui-check/node_modules`,
  `tools/brand/.venv` and the app's Gradle build directories.
  Never `.env`, stores, `tools/ui-test/.venv`,
  or a cache that every checkout shares.

### Decided in the writing

- **The branch** is `claude/clean-runs`, one pull request, a commit per block.
  The plan's own commit comes first, on `origin/main`, not pushed in this run.
- **Two sessions installing at once**
  (D1): the install is renamed into place with Node's `fs.renameSync`,
  which fails where the directory is there already;
  the one that loses removes its own copy.
  `mv` would move the second copy into the first, and `mv -T` is GNU's alone.
- **One platform list**
  (D2): the cross targets write to `$(call crossout,<file>)`, `$(OUT)/<file>`,
  or `/dev/null` where `cross-check` runs `make cross` with `CROSS_DISCARD=1`.
- **A kept work directory**
  (D3, D4) holds a file `kept`, which the UI suite's sweep passes over:
  what `KEEP=1` kept stays until it is removed by hand.
- **The LAN and emulator suites' work directories** are named too,
  `massimo-lan-test.XXXXXX` and `massimo-app-emulator.XXXXXX`,
  so that a kept or a killed run's directory says whose it is;
  no sweep (D3 is the UI suite's).

## Analysis

The survey, the measurements and the options are in
`wip/2026-11-25_17-12_ANALYSIS_CleanRuns_cleanup_after_runs.md`.
What it does not hold:

### For Block 1: finding the main checkout

`git rev-parse --path-format=absolute --git-common-dir` gives
`/home/mdr/projects/massimo/.git` from a worktree and from an exported copy
inside a worktree's `GUIDELINES/temp/` (checked 2026-11-25);
its parent is the main checkout.
`GUIDELINES/temp/` is ignored by `GUIDELINES/.gitignore`.
In CI the common directory is the job's checkout's.
`tsc` and `biome` run from an install outside the checkout unchanged:
`tsconfig.json` has `"types": []`, and Biome takes `--config-path`.

### For Block 2: a build to `/dev/null`

`GOOS=windows`, `GOOS=linux GOARCH=arm GOARM=6` and `GOOS=darwin GOARCH=arm64`,
each `CGO_ENABLED=0 go build -trimpath -o /dev/null`,
built and wrote nothing (2026-11-25).
Nothing in Vet and test reads `dist/` after its cross build;
the `packages` artifact comes from the Debian job's own `make packages`.

### For Block 3: what removes the UI suite's results today

pytest-playwright 0.10 removes its `--output` directory at the first test of a
run; `run.sh` removes `results/logs` before pytest.
A run that stops before its first test leaves the earlier run's results.
The work directory is `mktemp -d`'s `tmp.XXXXXXXXXX` and goes in the `EXIT`
trap, which a kill does not run.

## Implementation Plan

Every commit is on `claude/clean-runs`.
Every `massimo` command runs with `MASSIMO_ENV_FILE=/dev/null`;
nothing goes to a real Cube.
Locally only what a block changes runs; CI runs the rest after the push,
which this run does not make.

### Block 1: SharedCheck

- **Goal:**
  `make ui-check` leaves nothing in the checkout,
  and installs once per lock file for all of them (D1-A).
- **Changes:**
  1. `tools/ui-check/install.sh` (new): the key,
     the first 16 digits of the lock's SHA-256 (`sha256sum` or `shasum -a 256`)
     and Node's `platform-arch`; the directory `UI_CHECK_CACHE`,
     else the main checkout's `GUIDELINES/temp/ui-check`, else none;
     an install there in `.new.XXXXXX` by `npm ci --ignore-scripts`,
     renamed into place, the loser's copy removed; the directory touched;
     directories unused for 30 days and `.new.*` older than a day removed;
     without a directory, `npm ci` into `tools/ui-check` as before;
     prints the `.bin` directory.
  2. `Makefile`, `ui-check`: runs the script, then `tsc` and `biome` from the
     directory it prints; its comment.
  3. `.gitignore`:
     the comment on `node_modules/`.
- **Tests:**
  `make ui-check` twice (the second runs no npm);
  with `UI_CHECK_CACHE` in a scratch directory,
  two installs at once leave one directory and no `.new.*`;
  a planted directory 40 days old goes; from a copy outside git,
  the install lands in the copy's `tools/ui-check/`;
  `shellcheck` where installed.
- **Documents:**
  `tools/ui-check/README.md` (where the install lives, the override,
  the pruning, the fallback); `GUIDELINES/project/CI.md`,
  section 3 (UI types installs into the job's checkout).
- **CHANGELOG:**
  `CHANGELOG.d/Changed/claude-clean-runs-ui-check.md`.
- **Commit:**
  `build(ui-check): One install of TypeScript and Biome serves every checkout`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Block 2: CrossCheck

- **Goal:**
  the test gate's cross build writes nothing (D2-A).
- **Changes:**
  1. `Makefile`:
     `crossout` and `CROSS_DISCARD`; the seven cross targets write through it;
     `cross-check` runs `make cross CROSS_DISCARD=1`; `.PHONY`.
  2. `.github/workflows/ci.yml`, Vet and test: `make cross-check`.
- **Tests:**
  `make cross-check` once: seven builds, no `dist/`;
  `go build -x -o /dev/null` shows the link step;
  `make cross` still writes `dist/`; `make actionlint`.
- **Documents:**
  `GUIDELINES/project/_CONTRACT.md`, the test gate's line (v1.21.0),
  then `GUIDELINES/shared/tools/sync-docs.sh` for `AGENTS.md`
  and `GUIDELINES/README.md`; `GUIDELINES/project/_README.md`, the command list;
  `GUIDELINES/project/CI.md`, sections 2 and 3.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-clean-runs-cross-check.md`.
- **Commit:**
  `build(make): make cross-check builds every platform and keeps nothing`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Block 3: WorkDirs

- **Goal:**
  a UI run starts from an empty `results/`,
  a killed run's work directory goes at a later run,
  and `KEEP=1` keeps a suite's (D3-A, D4-A).
- **Changes:**
  1. `tools/ui-test/run.sh`:
     in a suite's run, `results/` removed at the start;
     the work directory `massimo-ui-test.XXXXXX` under `TMPDIR`, with `pid`;
     the sweep of those whose PID is gone, older than a day, without `kept`;
     `KEEP=1`: the directory kept with `kept`, its path printed.
  2. `tools/lan-test/run.sh` and `tools/app-emulator/run.sh`:
     the named work directory; `KEEP=1` as above.
- **Tests:**
  one UI test in one case with `UI_TEST_WORKERS=1` and free ports:
  the work directory gone; again with `KEEP=1`: kept, path printed;
  planted directories: a dead PID two days old goes, a live PID stays,
  a `kept` one stays; `make lan-test KEEP=1` once;
  `bash -n` and `shellcheck` of the three; `go test ./tools/app-emulator`;
  the emulator suite's run is CI's.
- **Documents:**
  `tools/ui-test/README.md`, "Running all, one file, one test";
  `tools/lan-test/README.md`; `tools/app-emulator/README.md`, "The results".
- **CHANGELOG:**
  none: `tools/` is not shipped.
- **Commit:**
  `test(tools): The suites' work directories are named, swept and kept with KEEP=1`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Block 4: CleanTargets

- **Goal:**
  what the suites and make leave in a checkout goes with one command,
  and nothing else does (D5-A).
- **Changes:**
  `Makefile`: `clean-tests` (`tools/ui-test/results`,
  `tools/ui-test/.pytest_cache`, `tools/app-emulator/results`,
  `__pycache__` under `tools/` outside a `.venv`); `distclean` (`clean`,
  `clean-tests`, `tools/ui-check/node_modules`, `tools/brand/.venv`,
  `app/android/app/build`, `app/android/build`, `app/android/.gradle`,
  `app/android/.kotlin`); `.PHONY`.
- **Tests:**
  in a scratch copy with each of those planted, and `.env`, `massimo.db`,
  `tools/ui-test/.venv` beside them:
  `clean-tests` and `distclean` remove the first and leave the rest.
- **Documents:**
  `README.md`, the line on `make clean`.
- **CHANGELOG:**
  `CHANGELOG.d/Added/claude-clean-runs-clean-targets.md`.
- **Commit:**
  `build(make): make clean-tests and make distclean remove what the runs leave`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Before the pull request

The run's NOTE (`NOTE CleanRuns v1.0: how the run went`),
then the completion gate, which offers the push of the plan's commit to `main`,
the branch's push and its pull request.

## Proposed commit message

The plan: `chore(aps): +CleanRuns v1.0, +ANALYSIS CleanRuns`

Block 2's commit:

```
build(make): make cross-check builds every platform and keeps nothing

For AP CleanRuns v1.0, Block 2 (D2-A): the test gate's cross build
writes every platform's binary to /dev/null instead of dist/, so that
a gate run leaves no 80 MB behind; make cross stays the build.

Key changes:
- Makefile: crossout and CROSS_DISCARD, cross-check.
- ci.yml, Vet and test: make cross-check.
- _CONTRACT.md v1.21.0: the test gate names make cross-check.
- CI.md: sections 2 and 3.
- CHANGELOG.d/Added/claude-clean-runs-cross-check.md.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-25): first revision, from the owner's `PLAN++` on the EVAL
  gate of 2026-11-25: D1-A as revised there, D2-A to D5-A.

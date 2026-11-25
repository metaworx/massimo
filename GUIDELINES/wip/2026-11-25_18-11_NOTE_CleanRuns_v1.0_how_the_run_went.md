# NOTE CleanRuns v1.0: how the run went

The run of `PLAN++` on AP CleanRuns v1.0
(`wip/2026-11-25_17-31_AP_CleanRuns_runs_clean_up_after_themselves.md`),
2026-11-25.

## What is done

All four blocks are committed on `claude/clean-runs`, a commit each,
after the plan's own commit, "chore(aps): +CleanRuns v1.0, +ANALYSIS CleanRuns".
Nothing is pushed and no pull request is open:
the gate offering `PLAN++` stated no push.
The branch merges cleanly with `origin/main` as it stood at the run's end (22
commits on, `README.md` the one file both change).

## Where the run went differently from the plan

1. **Block 2: `GUIDELINES/project/_README.md` carries a version.**
   Its command list changed, so it is v1.1.0,
   with its record in Block 2's commit; the plan named the file,
   not its version.
2. **Block 2: two more mentions of `make cross`** became `make cross-check`:
   the armv6 paragraph of `GUIDELINES/project/CI.md`, section 2,
   and the armv6 job's comment in `ci.yml`,
   which both said what Vet and test builds.
3. **Block 3: `tools/lan-test/run.sh`'s own version** went from 1.6.0 to 1.7.0.
4. **The second reader's findings** were fixed and folded into Blocks 1 and 3,
   before anything was pushed:
   - `-mtime +0` matches anything older than a second on macOS,
     whose find rounds a day up:
     the day's limits are `-mmin +1440`, the 30 days `-mmin +43200`;
   - with `UI_CHECK_CACHE` at a directory shared with other programs,
     the pruning could have taken theirs:
     it now removes only an entry holding the ui-check `package.json`;
   - the pruning, the `touch` and a `mktemp` that cannot write no longer fail
     the check: the first two are ignored,
     the last falls back to the checkout's own install;
   - the UI suite's sweep passes over another user's directories (`-O`)
     and goes on where one cannot be removed;
   - `KEEP=1` writes `kept` at the run's start,
     so a run killed before its cleanup keeps it;
   - the key says `-musl` on a Linux without glibc,
     which Biome's binaries tell apart;
   - the lock is hashed from standard input, so its path cannot change the key;
   - `tools/ui-check/README.md` says that an editor no longer finds TypeScript
     or Biome's schema in `tools/ui-check/node_modules`.
5. **A retest ran longer than meant.**
   The sweep's retest asked for a fake Cube port that another session had held
   earlier, so that the run would stop after the sweep;
   the port was free by then,
   and `-k nothing` selected the 168 tests whose names hold it.
   It was stopped after five minutes; its trap stopped its servers,
   and the directory it kept under `KEEP=1` had its `kept` from the start,
   which is the fix of item 4 working.

## How it was verified

- **Block 1:**
  - `make ui-check` took 6.9 s for the first install and 1.8 s for the next run,
    without npm.
  - Two installs at once left one directory, and both used it.
  - In a scratch `UI_CHECK_CACHE`, the pruning removed only what is the check's:
    its 40-day install and its 2-day copy.
    It left a stranger's 40-day directory and a stranger's 2-day `.new.*`.
  - A copy outside git installed into its own `tools/ui-check/`,
    and so did one whose `UI_CHECK_CACHE` could not be written.
- **Block 2:**
  - `make cross-check` built all seven platforms in 36 s and left no `dist/`.
  - `go build -x -o /dev/null` shows the link step.
  - `make -n cross` still names `dist/` for all seven.
  - `make actionlint` passes.
- **Block 3:**
  - One UI test in one case passed, through `run.sh` and through `make`.
    It left no work directory, and an earlier failure's `results/` was gone.
    With `KEEP=1` its directory stayed.
  - Of five planted directories,
    the sweep removed the dead and the PID-less one, 25 hours old.
    It left the live one, the kept one and a dead one of 23 hours.
  - `make lan-test KEEP=1` passed its 15 checks, kept its directory,
    and left no container or network.
  - `shellcheck` passes on all four scripts.
  - `go test ./tools/app-emulator` passes.
- **Block 4:**
  in a scratch directory with every target planted beside `.env`, `massimo.db`,
  `tools/ui-test/.venv`, the app's sources and the web UI's brand,
  `clean-tests` and then `distclean` removed the targets and left the rest.
- **Each commit:**
  `commit-msg.sh` and `changelog.sh check --staged` passed before it.
  `GUIDELINES/shared/tools/check` passes over all of them: 12 checks, all well.
- **Left to CI**, as the owner asks of a run on the shared host:
  - the Go suite, the race detector and the armv6 tests;
  - the whole UI suite;
  - the emulator suite, which CI runs for a change under `tools/app-emulator/`
    and which takes the same path without `KEEP`.

## Seen and left

- **The scratch directory of the run that prompted this plan:**
  another worktree's `GUIDELINES/temp/` holds 11 GB of exported copies,
  whose session is still running suites in them.
  It is that session's.
- **Go's build cache:**
  34 GB, trimmed by Go itself.
  Copies built without `-trimpath` each compile every package again.
- **The emulator's AVD `massimo-test`:**
  one for every checkout, so two emulator runs at once collide.
- **Docker:**
  two stale images, `massimo-mdns-test:avahi` and `massimo-armv6-test`.
- **Playwright's temporary directories:**
  a killed run's are left in `TMPDIR`, outside the work directory,
  and the sweep does not see them.
- **The shared install:**
  `/home/mdr/projects/massimo/GUIDELINES/temp/ui-check/` now holds the first
  one, made by this run's `make ui-check`.
  It is what Block 1 is for, so it stays.

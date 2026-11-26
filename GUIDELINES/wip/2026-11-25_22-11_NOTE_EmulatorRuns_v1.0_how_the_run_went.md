# NOTE EmulatorRuns v1.0: how the run went

The run of `D3B PLAN++` on AP EmulatorRuns v1.0
(`wip/2026-11-25_20-32_AP_EmulatorRuns_each_run_its_own_avd_and_ports.md`),
2026-11-25.

## What is done

Both blocks are committed on `claude/emulator-runs`, one commit each,
after the plan's own commit, "chore(aps): +EmulatorRuns v1.0, +ANALYSIS EmulatorRuns".
Nothing is pushed and no pull request is open:
the gate offering `PLAN++` stated no push.
The old shared AVD, `~/.android/avd/massimo-test.*`, is still there:
D4-A removes it after the merge.

## Where the run went differently from the plan

1. **Block 1 changed CI's path filters too.**
   A change to `tools/workdir/` alone would have run neither suite,
   so `ci.yml`'s job What to test adds it to the UI and Android filters.
   `packaging/ci_scripts_test.go` now tests both filters;
   there was no test of the Android one before.
   `GUIDELINES/project/CI.md` is v1.75.0 and the change carries a CHANGELOG
   fragment, since `ci.yml` is shipped;
   the plan said "CHANGELOG: none".
2. **Block 1, the helper's own rules.**
   `workdir.sh` refuses a name that is not letters, digits, `-` and `_`,
   since an empty one would sweep `TMPDIR/.*`,
   and it passes over a `pid` line that is not a number or starts with `0`,
   since `kill -0 0` always succeeds.
3. **Block 1, accepted: an old kept directory is swept.**
   An emulator run kept with `KEEP=1` by the `run.sh` of AP CleanRuns,
   merged the same day, has no `kept` file,
   so the new sweep removes it once it is a day old.
   There was none on jaguar.
4. **Block 2: the lock is held by a holder process.**
   The plan held the lock on fd 9 of `run.sh` and started the emulator without it.
   The second reader showed two ways that still kept a slot taken after its run.
   An `adb` server started again by any later `adb` call inherits the descriptor.
   So does the lock screen's guard loop, which runs on after a `kill -9` of `run.sh`.
   `slot_hold` now hands both locks to one `tail --pid` outside the run's children
   and closes them in `run.sh`.
   A second pass showed that Ctrl-C to the process group killed that holder
   while the cleanup still ran, so the holder ignores INT, TERM, HUP and QUIT.
   `run.sh` now needs `tail --pid` beside `flock` and `ss`.
5. **Block 2: a lock per checkout.**
   Before, a second run of the same checkout died at "emulator-5584 runs already".
   With slots it would have taken the next one,
   then removed the first run's `results/` and built in the same `app/android`.
   `slot_checkout` locks `tools/app-emulator` before anything is removed.
6. **Block 2: the console ports count down.**
   Slot k's console port is 5584 - 2k:
   adb finds an emulator by itself, after its server restarts,
   only on 5554 to 5584.
   The servers' ports still go up, 9443 + 2k and so on.
   `ss` also checks the console port + 1, adb's.
   A port variable is checked as a port.
   A lock path that is a link or not a regular file is passed over.
   `APP_EMU_PORT` still takes 5554 to 5682, as before.
7. **On jaguar, slots 0 and 1 are taken.**
   The phone rig's servers listen on 9443, 9444 and 9445,
   so Block 1's run took 9463 and 9464 by hand,
   and every run of Block 2 started at slot 2.

## How it was verified

- **Block 1:**
  - `go test` of `tools/workdir/` (planted directories, both shells),
    `tools/app-emulator/` and `packaging/`;
    `shellcheck`, `gofmt`, `go vet` for the three systems;
    `make actionlint`.
  - One UI test in one case passed; its work directory went.
    With `KEEP=1` it was kept, its path printed.
    Another session's live directory stayed.
  - `make app-emulator-test` with `ForgetAllTest` alone passed.
    The AVD was in `/tmp/massimo-app-emulator.*/avd/` while it ran,
    and the directory went at its end.
    `~/.android/avd` kept its times and sizes.
    The emulator booted in 111 s.
- **Block 2:**
  - `go test` of `tools/app-emulator/` three times over,
    with fakes for adb and `ss`.
    The cases: the slot taken and passed over,
    the holder (no child holds a lock, the locks outlive a group signal,
    and they are free after a `kill -9`), and the checkout lock.
    Both builders mutated `slots.sh` a dozen ways each,
    and the tests caught every mutation.
  - Two runs started 50 ms apart, from this worktree and from a copy
    exported into `GUIDELINES/temp/emulator-runs/second`, did so twice.
    Both times they took slots 2 and 3: one found the other's lock and took the next.
    A third run of this worktree stopped at the checkout lock,
    and the first run's `results/` stayed as it was.
    After the runs no lock was held, no work directory was left,
    and no emulator or `netsimd` was running.
  - Of those four runs one passed;
    three failed at the enrollment while other sessions' tests held the
    host's load average near 20 on 16 cores.
    Their slots, ports, AVDs and the helper's touches were each run's own.
    The boot took about 165 s each at a load near 2 and 335 s near 20.
  - On the final code, alone: on slot 2 (console 5580) the enrollment failed
    at a load near 10; with the console port given as 5584, and then as
    5580, it passed at a load below 8.
    Of the branch's eight runs, four failed, all at a load of 10 or more.
- **Why the enrollment failed** (a sub-agent's reading of a failed run's
  device log beside two passed ones; the console port plays no part):
  in every run the lock screen comes back some seconds after `run.sh` sets
  the PIN, as the emulator's first setup ends, and the run takes it away.
  In the failed run SystemUI had stopped answering before that, and the
  system left it running behind its "isn't responding" dialog,
  where a passed run had it restarted;
  no PIN the guard typed reached the lock screen ("Verifying lockscreen
  credential" never came), Settings' PIN screen was stopped under it and
  the enrollment ended, so the 70 touches the helper made reached no
  enrollment.
  This is the suite's own and older than this plan;
  two runs at once only make the load that exposes it likelier.
  Ways out it names: set the PIN once `user_setup_complete` is 1,
  and `settings put global hide_error_dialogs 1`, so that a SystemUI that
  stops answering is restarted.
  `tools/app-emulator/README.md`, "The ports", says what a busy host does.
- **Left to CI:** the whole emulator suite, strict, and the UI suite in its
  four cases, on the pull request.

# NOTE SuiteGaps v1.0: how the run went

How the owner's `PLAN++ D5-B` ran the three blocks of
[AP SuiteGaps v1.0](2026-11-09_18-11_AP_SuiteGaps_manual_checks_into_the_suite.md)
on 2026-11-09, a pull request each from `origin/main`:
Block 1 is #145, Block 2 #146 and Block 3 #147.

## What was measured

- **The job Test (armv6, QEMU)** took 2 min 9 s on the runner, apt included,
  and passed on #145's first run.
- **`make test-armv6`** passed all 22 packages in 169 s in a container with
  Debian 13's `qemu-user` and no network, with the fix of `TestEventsPageQuery`.
- **The six dumps** are 67 kB in all, 35 to 86 rows each.
- **The benchmarks** run once each with `-short` in 1.7 s on the PC,
  the password benchmarks included.

## Where the run went differently from the plan

1. **Backups by hand start with v0.11.0, not v0.10.0:**
   the Analysis' table of the releases said v0.10.0 took them,
   but its `POST /api/v1/cubes/{sn}/backups` answers 404.
   The script tries the request with every release and names a refusal;
   its header and `docs/store.md` say v0.11.0.
2. **`fill` takes a `testing.TB`** in Block 3, so that the benchmarks use
   `TestEventsPageQuery`'s rows; the plan's table of files named
   `events_test.go` for Block 1 only.
3. **CI.md's versions:** Block 1 raised it to v1.16.0 (a new job),
   Block 3 to v1.15.2 (a step in Vet and test),
   so that the second of the two to merge sets the version in its merge.
4. **The Guidelines check fails on all three pull requests** because of a plan
   that is not this one: AP LostCube, registered on `main` as bcdacfa during
   the run, has a heading "Block 3's risk" in its Analysis,
   which `check-plans.sh` counts as a second Block 3 beside its TL;DR's three.
   The fix is that plan's session's; every other check of #145 and #146 passed.
5. **The test's checks** read the settings as well as the log, the users and
   the backups; the plan's Analysis named the three.

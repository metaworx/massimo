# NOTE StoreTestClose v1.0: how the run went

How AP StoreTestClose v1.0
([plan](2026-11-16_04-05_AP_StoreTestClose_tests_that_close_and_wait.md))
was carried out, by `EXEC++`, then `EXEC++ D3-A` after the stop in
Block 3. The pull request is #333.

## Where it went differently

1. **Block 2 also gave TestDisagreeResendsMassimosAuto the wait.**
   It has the same gap as TestDisagreeResendsMassimosSetpoint: a room
   setpoint, then a check before the pump confirmed it. It passed with
   the pump held back only because its checks leave room.
2. **Block 3 grew a shipped-code fix (D3-A).**
   With the pump held back, the fixed TestVirtualWallThermostatEvents
   found that the stream of `devices=virtual` carried the greeting's
   `mismatch` of the real wall thermostat. `docs/api-v1.md` says that
   view leaves the real wall thermostats out. The pump marked only its
   own device events as a real wall thermostat's. `publish`
   (`internal/server/resume.go`) now marks any event of one that is not
   marked yet. The new TestVirtualViewLeavesOutTheRealWallsEvents covers
   it, and a `CHANGELOG.d/Fixed` bullet ships with it.
3. **The new test failed on CI's armv6 job, and a follow-up commit
   fixed it.** Under QEMU, the pump came to the greeting only after the
   streams had opened, so the greeting's own two mismatches of the wall
   thermostat reached the view `all` and were counted too. The test now
   counts only the mismatches it sends.
   - Not reproduced under QEMU in Docker, not even limited to a fifth of
     a CPU.
   - Reproduced exactly by sending two unmarked mismatches first.
4. **Block 4's loaded package run was run twice.** A session restart
   stopped the first one before it wrote anything.
5. **The push came before the NOTE,** at the owner's word ("push to run
   the jobs on CI"), so that CI ran while the loaded run went.

## Measured

| Run | Load | Result |
|-----|------|--------|
| Windows test binary of `internal/store`, through interop, ProgramData in a temp directory | - | TestMigration12 failed before Block 1, the package passes after it |
| Pump held 150 ms before `confirmState`, TestDisagreeResendsMassimosSetpoint | - | old 5 of 5 fail, new 5 of 5 pass |
| Pump held 150 ms before `publishVWTs`, TestVirtualWallThermostatEvents | - | old 5 of 5 fail, new 5 of 5 pass |
| Drift and re-send tests, `-race -count=20` | idle | pass |
| 16 busy loops, `-race -count=50` of the four tests of Blocks 2 and 3 | about 20 | pass, 795 s |
| 16 busy loops, `-race -count=3` of `internal/server` | 26 to 30 | pass, 2499 s |

Before the run, the same whole-package loaded run on main failed 4 times
in 3 passes. Three tests failed: these two, and TestFailedRadiogram,
which #332 fixed. No `DATA RACE` was reported. The failure at load 12 of
2026-11-16, never captured, was most likely one of the three.

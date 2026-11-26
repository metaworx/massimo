# ANALYSIS EmulatorRuns: the emulator suite's AVD collision

Written 2026-11-25 for the owner's "PLAN the AVD collision"; read-only:
no project file was changed and no emulator started.

## What two runs at once share

| Shared | Where | Collides |
|--------|-------|----------|
| The AVD `massimo-test` | `~/.android/avd/` (`ANDROID_AVD_HOME`), one for every checkout, 1.2 GB now | **yes**: the emulator refuses a second instance of one AVD |
| The console port | 5584 (`APP_EMU_PORT`) | yes, caught at once |
| The servers' six ports | 9443/9444, 62941/62942, 23941/23942 | yes, caught at the servers' start |
| adb's server, `~/.gradle`, the SDK, the console token | host-wide | no: adb is per serial, Gradle locks its caches |
| `results/`, `app/android/app/build/`, the work dir | per checkout or per run | no |
| `/data/local/tmp/…`, `adb reverse`, `fingers.sh`, the device's log | per emulator | no |

The emulator's own text (emulator 37.3.10, `qemu-system-x86_64`):
"Running multiple emulators with the same AVD is an experimental feature.
Please use -read-only flag to enable this feature."
The lock is the AVD's `multiinstance.lock`.

## What a second run does today

1. **Defaults:** it stops at once, before anything is touched:
   "emulator-5584 runs already … APP_EMU_PORT takes another".
2. **Only `APP_EMU_PORT` changed:** it rewrites the shared `massimo-test.ini`
   and `config.ini`, then starts an emulator on the shared AVD, which refuses.
   It stops at server a's port, 9443 ("its port in use?").
3. **All seven ports changed:** the servers and Gradle's assemble run for
   minutes. Only then does the boot wait find the emulator gone,
   "the emulator stopped as it booted", with the reason in `emulator.log` alone.

There is no remedy today but an `ANDROID_AVD_HOME` of the session's own
plus seven port variables.

**Not tested:** whether the refused start's `-wipe-data` touches the running
emulator's data before the lock check refuses it.
Testing that needs the shared AVD under a live run.
An AVD of the run's own makes the question moot.

**The second-run case:** a checkout whose pins differ (another API level)
rewrites the shared `config.ini`, so the AVD on disk is whichever checkout ran last.
It does no harm, since each run writes it again before its start.

## Why an AVD of the run's own costs nothing

`-wipe-data` with `-no-snapshot` already remakes every image in the AVD at
each start: `userdata-qemu.img`, `cache.img`, `encryptionkey.img` and
`hardware-qemu.ini` all carry the run's start time in today's AVD.
CI makes the AVD fresh on every run.
Keeping it between runs saves nothing, and leaves 1.2 GB in the home directory.

The AVD can keep its name, `massimo-test`, in a directory of its own.
Gradle's JUnit file names carry it (`TEST-massimo-test(AVD) - 14-_app-.xml`,
which `tally/main_test.go` uses), so they stay as they are.

## The ports

A slot `k` would give a run all seven ports at once:
console 5584+2k, servers 9443+2k and 9444+2k, Cubes 62941+2k and 62942+2k,
discovery 23941+2k and 23942+2k.

- The slot is held with `flock(1)` on a lock file of its own,
  which the kernel releases however the run ends.
  `flock` is on jaguar and on CI's Ubuntu.
  The suite is Linux-only (KVM).
- A slot is passed over where adb already lists `emulator-<port>`,
  or where one of its ports is listening.
  The phone rig's server uses 9443 in WSL.
- A port variable given still wins.

The alternative is ports the system picks (`127.0.0.1:0`), read back from the
programs' logs.
The fake Cube logs no port for its discovery listener
(`cmd/massimo/fakecube.go:71`), so that needs a change to shipped Go code.
The console port needs a lock either way.

**The UI suite** has the same pattern:
set 0 at 62919/8119/23919, set n offset from it,
and sessions in other worktrees pick ranges by hand.
It is not part of this plan.

## A killed run

With the AVD in the work dir, a run stopped by `SIGKILL`, or by a WSL that
shuts down, leaves about 1.2 GB in `TMPDIR`.
The UI suite's sweep (`tools/ui-test/run.sh`) removes a dead run's dir:
owned by the user, no live pid, not kept by `KEEP=1`, older than a day.
The same rule fits here.

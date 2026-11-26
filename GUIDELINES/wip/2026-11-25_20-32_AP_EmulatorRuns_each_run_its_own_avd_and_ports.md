# AP EmulatorRuns v1.1: Each run its own AVD and ports

## TL;DR

Two emulator runs from two checkouts can run at once:
each run makes its AVD in its own work directory and holds a slot that gives it
seven ports of its own; one helper makes and sweeps the UI and emulator suites'
work directories (`wip/2026-11-25_20-08_ANALYSIS_EmulatorRuns_avd_collision.md`);
and on a busy host the enrollment no longer ends under a lock screen that
comes back.

- shared `~/.android/avd/massimo-test` → an AVD per run, in its work directory
  (Block 1, built)
- console 5584 and six server ports, fixed → slot `k` of 0-7 by `flock`
  (Block 2, built)
- the PIN set while the first setup still runs, its lock screen back after it
  → the PIN once the first setup has ended
- a SystemUI that stops answering, left behind its dialog →
  `hide_error_dialogs` 1: killed, and started again
- `APP_EMU_PORT` 5554-5682 → 5554-5584, the ports adb finds by itself
- the old shared AVD → removed after the merge, while no emulator runs

### Blocks

1. OwnAvd: the AVD in the run's work directory; the shared work-dir helper
2. PortSlots: the seven ports by slot, its helper tested
3. LockRace: the PIN after the first setup; no error dialogs
4. PortRange: `APP_EMU_PORT` within the ports adb finds by itself

## Discussion

### What is decided

The owner, 2026-11-25, `D3B PLAN++` on the TL;DR gate of the same day:

- **D1-A.**
  Each run's AVD `massimo-test` is in its work directory,
  `$work/avd/`, made at each run as `-wipe-data` remakes its images anyway,
  and removed with the work directory; `KEEP=1` keeps it with the rest.
  `ANDROID_AVD_HOME` is no longer read.
- **D2-A.**
  A run holds slot `k`, the first of 0 to 7 that it can lock and that is free:
  its console port 5584 + 2k, the servers' 9443 + 2k and 9444 + 2k,
  the fake Cubes' 62941 + 2k and 62942 + 2k,
  their discovery ports 23941 + 2k and 23942 + 2k.
  A slot is passed over where adb lists its emulator or one of its ports listens.
  A port variable given replaces that port alone.
- **D3-B.**
  One helper, `tools/workdir/workdir.sh`, makes a suite's work directory,
  sweeps the dead runs' ones, and keeps or removes it at the end;
  the UI suite and the emulator suite source it.
- **D4-A.**
  `~/.android/avd/massimo-test.avd` and `massimo-test.ini` are removed by this
  session after the merge, once no emulator runs;
  a checkout still on the old `run.sh` makes them again.

The owner, 2026-11-26, `PLAN++ D5-A D6-B` on the completion gate of
2026-11-25:

- **D5-A.**
  The enrollment's lock-screen race is fixed in this plan, before the push:
  the PIN is set only once the emulator's first setup has ended,
  and `settings put global hide_error_dialogs 1` makes the system restart a
  SystemUI that stops answering rather than leave it behind a dialog;
  verified with two runs at once under load.
  (B, a plan of its own after the merge; C, the README's description alone.)
- **D6-B.**
  `APP_EMU_PORT` takes an even port from 5554 to 5584 alone,
  the console ports adb finds by itself after its server restarts;
  5586 to 5682, taken before, are refused at the start.
  (A, the range kept.)

### Decided in the writing

- **The branch** is `claude/emulator-runs`, one pull request, a commit per block.
  The plan's own commit comes first, on `origin/main`, not pushed in this run.
- **The helper is POSIX `sh`**, since the UI suite's `run.sh` is `sh` and the
  emulator's `bash`; it sets no shell option, and its variables carry the prefix
  `workdir_`.
- **The sweep waits for every PID in `pid`**, one per line:
  the emulator suite adds the emulator's,
  so that an emulator left running by a killed run keeps its AVD.
- **The slot's lock files are in `/tmp`, not `TMPDIR`**,
  `massimo-app-emulator-slot.<k>.lock`, so that runs with different `TMPDIR`s
  see one another; they are opened for reading, and stay.
- **The lock is held on fd 9 of `run.sh`**, taken after `adb devices`,
  which starts adb's server, and the emulator starts with fd 9 closed:
  neither outlives the run holding a lock.
  A run's other children hold it until they end;
  Java closes the descriptors of what Gradle starts.
- **`lan-test` keeps its own work directory**: it is small and has no sweep.

In the writing of v1.1:

- **Blocks 1 and 2 are committed**, and where they went differently from v1.0
  is `wip/2026-11-25_22-11_NOTE_EmulatorRuns_v1.0_how_the_run_went.md`;
  among it, the lock is held by one holder process, not on `run.sh`'s fd 9,
  and the console ports count down, 5584 - 2k.
  v1.0's text below is left as it was planned.
- **D6-B is a block of its own**, Block 4, after Block 3,
  rather than the follow-up on Block 2 the gate named,
  so that its commit cites the block it is;
  it waits for Block 3's runs, since `run.sh` is not edited while a run
  reads it.
- **The given port's check moves into `slots.sh`**, so that the tests of
  `scripts_test.go` reach it; `run.sh` calls it where it checks today.
- **A busy emulator without a busy host.**
  The runs that verify Block 3 pin their emulators to a few of the host's
  cores (`taskset`), so that they starve one another and their own builds,
  and not the other sessions' tests, which share the host.
- **The mechanism is checked by itself** on an emulator of the pins started
  by hand, on a slot's port with that slot's lock held and every adb command
  naming its serial: SystemUI stopped, through the image's `adb root`,
  with and without `hide_error_dialogs`.

## Analysis

The survey and the options are in
`wip/2026-11-25_20-08_ANALYSIS_EmulatorRuns_avd_collision.md`.
What it does not hold:

### For Block 1: the AVD in a directory of its own

The emulator takes the AVD from `ANDROID_AVD_HOME`, which `run.sh` sets for the
emulator alone, as it sets `ANDROID_HOME` there already.
The AVD keeps its name, `massimo-test`, which Gradle puts in its JUnit files'
names (`TEST-massimo-test(AVD) - 14-_app-.xml`);
two runs' files are in their own checkouts' `build/`.

### For Block 2: what holds a lock

`flock(1)` locks an open file, and a child inherits the descriptor.
adb's client starts the server with `fork` and `exec` and closes nothing,
so a server started under the lock would hold it as long as it runs.
`run.sh` already asks `adb devices` before anything else,
which starts the server; the lock comes after it.
The emulator may start helpers of its own (`crashpad_handler`, `netsimd`).
It runs with `9<&-`. An emulator left by a killed run then holds no lock,
but adb lists it, and the slot is passed over.
`ss -Hltun` reads the listening sockets without connecting to them:
a connection to the phone rig's server on 9443 would put a TLS error in its log.
`flock` is in util-linux and `ss` in iproute2, both on jaguar and on CI's Ubuntu.

### For Block 3: the lock screen that comes back

Of Block 2's eight runs four failed at the enrollment, each at a host's load
average of 10 or more.
A reading of a failed run's device log beside two passed ones
(the NOTE of v1.0, "Why the enrollment failed") found:

- In every run the lock screen shows again some seconds after `run.sh` sets
  the PIN, as the emulator's first setup ends;
  `unlocked pin` and the guard take it away by typing the PIN.
- In the failed run SystemUI had stopped answering before that,
  and the system left it running behind its "isn't responding" dialog,
  where a passed run had it started again:
  no PIN typed reached the lock screen, Settings' PIN screen was stopped under
  it, and the enrollment ended.

Two changes, each closing one half:

- **The PIN after the first setup.**
  The first setup ends by setting `user_setup_complete` (secure) and
  `device_provisioned` (global) to 1.
  Set before the PIN, they put the lock screen's return where `unlocked`
  takes it away by a dismiss, with no PIN to type.
  Block 3 first reads in the kept runs' logs when the lock screen comes back
  against the moment those settings turn 1,
  and where it comes later, waits for it as well.
- **No error dialogs.**
  With `hide_error_dialogs` 1, the system shows no "isn't responding" dialog,
  and kills the app it would have shown one for.
  SystemUI is a persistent process, so the system starts it again,
  and its lock screen with it, which the guard's PIN then reaches.
  It hides a dialog of the app's own too,
  where the device's log still holds the ANR, which `deaths` reads.

A race a busy host exposes needs a busy emulator to be shown,
and first without the fix, so that the fix's passes say something.

### For Block 4: the console ports adb finds

adb's server finds the emulators by itself on the console ports 5554 to 5584
(its local scan, 5555 to 5585, the console's port + 1).
An emulator on another port is known to the server that the emulator itself
told at its start; after a restart of that server, by `adb kill-server`,
a crash or a client of another version, it is lost,
and the run fails at its next adb call.
The slots stay within 5570 to 5584;
nothing in the repository or CI gives `APP_EMU_PORT`.

## Implementation Plan

Every commit is on `claude/emulator-runs`.
Every `massimo` command runs with `MASSIMO_ENV_FILE=/dev/null`;
nothing goes to a real Cube.
The emulator runs under `sg kvm -c`, since this session predates the group.
CI runs the whole emulator suite and the UI suite after the push,
which this run does not make.

### Block 1: OwnAvd

- **Goal:**
  a run's AVD is its own and goes with it (D1-A),
  and the suites' work directories come from one helper (D3-B).
- **Changes:**
  1. `tools/workdir/workdir.sh` (new, POSIX `sh`, sourced):
     `workdir_make <name>` sweeps, makes `<name>.XXXXXX` under `TMPDIR`
     (default `/tmp`) with the caller's PID in `pid` and `kept` under `KEEP=1`,
     and prints its path;
     `workdir_pid <dir> <pid>` adds a PID;
     `workdir_sweep <name>` removes `<name>.*` that are this user's,
     not kept, none of whose PIDs runs, and older than a day (`-mmin +1440`);
     `workdir_done <dir> <who>` removes it, or under `KEEP=1` keeps it
     and says so.
  2. `tools/workdir/doc.go`, `workdir_test.go`, `README.md` (new).
  3. `tools/ui-test/run.sh`: the inline sweep, `mktemp` and `KEEP`
     replaced by the helper; nothing else changes.
  4. `tools/app-emulator/run.sh`:
     the work directory from the helper, the emulator's PID added;
     the AVD in `$work/avd`, `ANDROID_AVD_HOME` set for the emulator alone;
     the header's settings without `ANDROID_AVD_HOME`.
- **Tests:**
  `go test ./tools/workdir/` with planted directories:
  a dead PID two days old goes, a live PID or any live PID of two stays,
  `kept` stays, one an hour old stays, one without `pid` two days old goes,
  something not a directory stays;
  `KEEP=1`: `kept` written, the directory kept and named;
  `go test ./tools/app-emulator/`;
  `shellcheck` of the three scripts;
  one UI test in one case with `UI_TEST_WORKERS=1` and free ports:
  the work directory gone; again with `KEEP=1`: kept, its path printed;
  `make app-emulator-test` with `APP_EMU_CLASS` set to one class:
  it passes, the AVD is in the work directory while it runs,
  `~/.android/avd` is untouched, and the work directory is gone at the end.
- **Documents:**
  `tools/workdir/README.md`;
  `tools/app-emulator/README.md`, "What it does" step 1 and "The results"
  (the AVD in the work directory, its size under `KEEP=1`);
  `tools/ui-test/README.md` where it names the sweep.
- **CHANGELOG:**
  none: `tools/` is not shipped.
- **Commit:**
  `test(tools): Each emulator run has an AVD of its own`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Block 2: PortSlots

- **Goal:**
  a second run takes ports of its own without a setting (D2-A).
- **Changes:**
  1. `tools/app-emulator/slots.sh` (new, `bash`, sourced):
     `slot_take` locks `/tmp/massimo-app-emulator-slot.<k>.lock` on fd 9 for the
     first `k` from 0 to 7 where the lock is free, adb lists no `emulator-<port>`
     and `ss` shows none of the seven ports listening;
     it sets `slot`, `port`, `serial` and the six server ports,
     a variable given replacing its port; where no slot is free it says why
     and fails.
  2. `tools/app-emulator/run.sh`:
     the ports from `slot_take` after `adb devices`;
     a given `APP_EMU_PORT` whose emulator runs stops the run as today;
     the slot and its ports said; the emulator started with `9<&-`;
     `flock` and `ss` among the tools checked before the run.
  3. `tools/app-emulator/scripts_test.go`:
     the slot's cases against a fake adb and a fake `ss`.
- **Tests:**
  `go test ./tools/app-emulator/`: nothing taken gives slot 0 and the
  defaults; slot 0's lock held, its emulator listed or one of its ports
  listening each give slot 1; all eight taken fails with the reason;
  a port variable given replaces that port alone;
  `shellcheck`;
  two `make app-emulator-test` runs at once with `APP_EMU_CLASS`,
  one from this worktree and one from a copy exported into
  `GUIDELINES/temp/emulator-runs/`: slots 0 and 1, both pass.
- **Documents:**
  `tools/app-emulator/README.md`, "What it needs" (`flock`, `ss`),
  "What it does" step 2, "The servers", and the settings;
  the `run.sh` header.
- **CHANGELOG:**
  none: `tools/` is not shipped.
- **Commit:**
  `test(app-emulator): Each run takes a slot of seven ports of its own`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Block 3: LockRace

- **Goal:**
  on a busy host the enrollment does not end under a lock screen that comes
  back (D5-A).
- **Changes:**
  1. `tools/app-emulator/run.sh`, with the boot's other settings:
     `settings put global hide_error_dialogs 1`, a warning where it is refused.
  2. `run.sh`, after the boot and before the PIN:
     wait until `user_setup_complete` and `device_provisioned` are 1,
     within the boot's time (`APP_EMU_BOOT_TIMEOUT` from the builds' end),
     and for the lock screen's return after them where the logs show it later;
     the time it took said; where it does not come, a stop that names the
     device's log.
     Then the lock screen away and the PIN, as today.
  3. `run.sh`: the comments of `unlocked` and `guard`,
     and the header's lines on a busy host.
- **Tests:**
  `go test ./tools/app-emulator/`; `shellcheck`;
  the runs, every emulator on a slot and every adb command naming its serial:
  - **without the fix,** on Block 2's code:
    two runs at once, their emulators pinned to the same few cores,
    up to two pairs, until one fails at the enrollment as the NOTE of v1.0
    describes, or saying that none did;
  - **the mechanism:** SystemUI stopped by hand on an emulator of the pins,
    and a key sent to it: with `hide_error_dialogs` 1 it is killed and
    started again, and the lock screen takes the PIN; without, the dialog stays;
  - **with the fix:** two pairs at once under the same pinning,
    and one run alone, unpinned: each passes, and its `keyguard.log` and
    device log show where the lock screen came back.
- **Documents:**
  `tools/app-emulator/README.md`, "What it does" where the PIN is set,
  and "The ports", the busy host's paragraph; the `run.sh` header.
- **CHANGELOG:**
  none: `tools/` is not shipped.
- **Commit:**
  `test(app-emulator): Set the PIN once the first setup has ended`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Block 4: PortRange

- **Goal:**
  a console port given is one adb finds by itself after its server restarts
  (D6-B).
- **Changes:**
  1. `tools/app-emulator/slots.sh`: `slot_port_given` checks `APP_EMU_PORT`,
     where given: even, from 5554 to 5584, or a message that says so.
  2. `run.sh`: calls it where it checks the port today;
     the header's range.
  3. `scripts_test.go`: the check's cases;
     the console's port given in `TestSlotTakesTheFirstFree`, 5600 today,
     one within the range and outside the slots.
- **Tests:**
  `go test ./tools/app-emulator/`: 5554, 5570 and 5584 taken;
  5553, 5583, 5586, 5682 and a word refused, with the message;
  `shellcheck`;
  `run.sh` with `APP_EMU_PORT=5586` stops before it starts anything.
- **Documents:**
  `tools/app-emulator/README.md`, "What it does" step 2 and "The ports";
  the `run.sh` header.
- **CHANGELOG:**
  none: `tools/` is not shipped.
- **Commit:**
  `test(app-emulator): APP_EMU_PORT takes only ports adb finds by itself`.
- **Verification:**
  the tests above; `GUIDELINES/shared/tools/check`.

### Before the pull request

The run's NOTE (`NOTE EmulatorRuns v1.1: how the run went`),
then the completion gate, which offers the push of the plan's commits and
NOTEs to `main`, the branch's push and its pull request.
D4-A's removal follows the merge.

## Proposed commit message

The revision: `chore(aps): +EmulatorRuns v1.1`

Block 3's commit:

```
test(app-emulator): Set the PIN once the first setup has ended

For AP EmulatorRuns v1.1, Block 3 (D5-A): on a busy host the lock
screen came back after the PIN, as the emulator's first setup ended,
and where SystemUI had stopped answering by then the PIN never reached
it and the enrollment ended under it; run.sh now sets the PIN once the
first setup has ended, and turns the error dialogs off, so that a
SystemUI that stops answering is started again.

Key changes:
- run.sh: hide_error_dialogs 1; the wait for user_setup_complete and
  device_provisioned before the PIN.
- README.md: what it does, a busy host.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-25): first revision, from the owner's `D3B PLAN++` on the
  TL;DR gate of 2026-11-25: D1-A, D2-A, D3-B, D4-A.
- v1.1 (2026-11-26): Blocks 3 and 4, from the owner's `PLAN++ D5-A D6-B` on
  the completion gate of 2026-11-25: the enrollment's lock-screen race
  (D5-A) and `APP_EMU_PORT` within 5554 to 5584 (D6-B);
  Blocks 1 and 2 built, how they went in the NOTE of v1.0.

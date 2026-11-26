# AP EmulatorRuns v1.0: Each run its own AVD and ports

## TL;DR

Two emulator runs from two checkouts can run at once:
each run makes its AVD in its own work directory and holds a slot that gives it
seven ports of its own; one helper makes and sweeps the UI and emulator suites'
work directories (`wip/2026-11-25_20-08_ANALYSIS_EmulatorRuns_avd_collision.md`).

- `~/.android/avd/massimo-test`, one for every checkout (1.2 GB) → the AVD in
  `massimo-app-emulator.XXXXXX/avd/`, made at each run, gone with it
- `ANDROID_AVD_HOME` → no longer read
- console 5584 and six server ports, fixed → slot `k` of 0-7, each port + 2k,
  held by `flock`; a variable given still wins
- the UI suite's sweep, inline → `tools/workdir/workdir.sh`, sourced by both
- the emulator's PID beside the run's: no sweep under a live emulator
- the old shared AVD → removed after the merge, while no emulator runs

### Blocks

1. OwnAvd: the AVD in the run's work directory; the shared work-dir helper
2. PortSlots: the seven ports by slot, its helper tested

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

### Before the pull request

The run's NOTE (`NOTE EmulatorRuns v1.0: how the run went`),
then the completion gate, which offers the push of the plan's commit to `main`,
the branch's push and its pull request.
D4-A's removal follows the merge.

## Proposed commit message

The plan: `chore(aps): +EmulatorRuns v1.0, +ANALYSIS EmulatorRuns`

Block 2's commit:

```
test(app-emulator): Each run takes a slot of seven ports of its own

For AP EmulatorRuns v1.0, Block 2 (D2-A): run.sh holds the first free
slot of eight by flock, and takes its console port and the servers' six
from it, so that a second checkout's run needs no setting.

Key changes:
- tools/app-emulator/slots.sh: slot_take.
- run.sh: the ports from the slot; the emulator without the lock's fd.
- scripts_test.go: the slot's cases.
- README.md: what it needs, the ports.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-25): first revision, from the owner's `D3B PLAN++` on the
  TL;DR gate of 2026-11-25: D1-A, D2-A, D3-B, D4-A.

# AP RestoreRace v1.0: Reference before done

## TL;DR

A restore that has finished no longer gets an automatic backup of the
state it restored: the file restored becomes the backup reference before
the restore says `done`, no longer after.

- `runRestore`: `done` set → then `rt.bk.restored(hash)` in the goroutine
  → `restored(hash)` first, then `done`
- the window between them: `finishCommand` (a step flush of up to 2 s,
  store writes), the log, the event → none
- `TestRestoreKeepsBackups`, flaky on armv6 under QEMU → passes under load
- a new test: at `done`, the reference is already the file's hash
- `### Fixed` bullet

### Blocks

1. The order in `runRestore`, a regression test, `CHANGELOG.md`

## Discussion

Asked by the owner on 2026-11-10, after "Test (armv6, QEMU)" failed once
on run 42776394653 (PR #159, commit 71ad86c):
`after a restore by id: auto,before_restore,upload,before_restore,auto`.
The owner asked whether it is only the armv6 test: it is the code's
race, on every platform; emulation only makes the window long enough.

**The owner's decisions** (`EXEC as recommended`, the TL;DR gate, 2026-11-10):

- **D1-A:** fix the order in the code.
  Not B, a test waiting on a condition, which hides the race;
  a look in the window still shows the Cube as changed in the API.
- **D2-A:** a regression test of the invariant itself:
  once the restore reads `done`, the watch's reference is the file's hash.
  Not B, the existing test alone, which catches it only under load.

## Analysis

`startRestore` takes the `before_restore` backup, whose `stored()` makes
its hash, the Cube's state before the restore, the reference. The steps
run in a goroutine; `backupCheck` skips the Cube while `restoring()`
returns a run. `runRestore` set `run.State = "done"`, emitted
`restore_done`, ran `finishCommand` and logged, and only after it
returned did the goroutine call `rt.bk.restored(ConfigHash(file))`.

In between, `backupCheck` no longer skips the Cube and compares its
configuration, now the file's, with the reference still the state before:
it is dirty from that look. Once `backups.auto_min` minutes pass in the
window, the automatic backup is taken, of the state just restored.
`restored()` afterwards clears the dirty state, so a shorter window shows
only in the API (`changed`, `due`), for that moment.

The test's minute is 20 ms, `auto_min` 2: a window of 40 ms with two
looks. `finishCommand` flushes the run's steps, up to 2 s, and writes the
command's state to SQLite, which under QEMU takes that long.
With a minute of a minute, the window would have to last two minutes:
in production practically never.

Measured on 2026-11-10 (jaguar, `-race -count=40 -cpu=1`, one CPU,
every CPU loaded): 6 of 40 failed, both on the restore by id (line 308)
and the restore from a file (line 285); with the fix, through
`go test -overlay`, 40 of 40 passed.

The other armv6 flakes named (TestLostFromALossToTheNextGreeting,
TestSilentCubeIsLostAfterTwoPolls, TestQueueObserver) are in
`internal/cubelan` and `internal/engine`, not this cause.

## Implementation Plan

### Block 1: The reference before `done`

1. `internal/server/restore_api.go`: `runRestore` takes the file's
   configuration hash and calls `rt.bk.restored(hash)` when the plan ran
   without an error, before it takes `restoreMu` to set `done`; the
   goroutine in `startRestore` no longer does. `restoreMu` and `bk.mu`
   are never held together, so no lock order is added.
2. `internal/server/backups_api_test.go`: `TestRestoreDoneAfterItsReference`:
   a stored Cube with a backup; a restore from a file with another room
   name; the test reads `restoreStatus()` in a tight loop, and at the
   first `done` the watch's `ref` is `backup.ConfigHash` of the file.
3. `CHANGELOG.md`, `### Fixed`: a bullet.

**Verification:**

- the new test fails without the fix (`go test -overlay` with the old
  `restore_api.go`) and passes with it;
- `-race -count=50` of both tests; `-count=40 -cpu=1` on one loaded CPU;
- `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`, `make cross`;
- `gofmt -l .`, `go vet` for linux, windows and darwin;
- `make test-armv6` needs `qemu-arm`, not installed on jaguar: CI's job
  on the pull request instead.

## Proposed commit message

```
fix(server): A finished restore gets no automatic backup of its state

Block 1 of AP RestoreRace v1.0. A restore said done before the file it restored became the backup reference; a backup check in between saw the restored configuration against the state before the restore and, once backups.auto_min passed, took an automatic backup of the state just restored. The window held finishCommand's step flush and store writes, long enough under QEMU to fail TestRestoreKeepsBackups on the armv6 job.

Key changes:
- runRestore makes the file the reference before it sets done.
- TestRestoreDoneAfterItsReference checks the reference at done.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): first revision.

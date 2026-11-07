# AP BackupListRace v1.0: The backup list and its state together

## TL;DR

`GET …/backups` sometimes shows a new automatic backup beside the change it
has just covered, failing `TestBackupAutomatic` about once in twenty runs.
One lock makes storing the row and marking the Cube clean one step for the list.

- Cause → `takeBackup` stores the row, then calls `stored()`;
  `listBackups` reads the rows, then `state()`, nothing tying the two.
- Both orders → new row beside the old change (the CI failure),
  or old rows beside the clean state.
- UI → loads the list on view and after an action;
  a load in the gap keeps "a backup follows at …" beside that backup.
- Fix (D1-A) → `backupWatch.rows`, an `RWMutex`: writer across `AddBackup` +
  `stored()`, reader across `Backups` + `state()`; the hash computed first.
- Test → unchanged; its check at line 70 now holds.
- Proof → `-race -count=200` (50 clean runs are luck 1 time in 22 at 6 %).

### Blocks

1. BackupListRace: the lock in `takeBackup` and `listBackups`,
   a `CHANGELOG.md` bullet; verified with `-race -count=200`
   of `TestBackupAutomatic` and the project's test gate and lint.

## Discussion

The TL;DR gate of 2026-11-08 was answered `PLAN++ as recommended`:
D1-A, the plan's one block, two signed commits on `claude/gallant-dirac-551fd7`,
no push, no pull request, no retirement.

### D1. Where the fix goes: A, in the server

A lock in `backupWatch`, held for writing only while a backup's row is
stored and the Cube made clean, and for reading while the list reads its
rows and the state.
It closes both orders, and makes the test's `Changed == nil` after
"auto,auto" hold without changing the test.

Rejected:
- **B, the test alone** (wait for "auto,auto" and `Changed == nil` together):
  the API and the UI would keep showing the stale state.
- **C, a `Changed` older than the newest `taken_at` read as clean**:
  it compares the scheduler's clock with the store's, so a clock stepped back
  (a Raspberry Pi syncing NTP after boot) hides a real change,
  and it misses the old rows beside the clean state.

## Analysis

### 1. The window

`takeBackup` (`internal/server/backups.go`) stores the row with
`st.AddBackup` and then calls `rt.bk.stored(hash)`,
which clears `dirtySince` and `lastChange`.
`listBackups` (`internal/server/backups_api.go`) reads `st.Backups` and then
`rt.bk.state()`.
A list whose rows are read after the insert and whose state is read before
`stored()` shows the new backup and the change it covered;
one whose rows are read before the insert and whose state after `stored()`
shows the old rows as clean.

The CI failure (run 42403167408, job 128495135179, for pull request 95) and
the local ones are the first:
`changed` about 80 ms before the new backup's `taken_at`,
which is `backups.auto_min` (3 × 20 ms) plus the tick, the old dirty state.
Locally, `MASSIMO_ENV_FILE=/dev/null go test -race -count=50 -run
'TestBackupAutomatic$' ./internal/server/` failed 3 of 50 (2026-11-08),
each at `backups_api_test.go:71`; the race detector reported nothing,
since every field is read under `backupWatch.mu`.

### 2. The other tests and callers

- `TestBackupAutomatic` line 60 checks `Changed == nil` after the first
  backup only after a 300 ms sleep; line 81 asserts nothing after a backup.
- `TestRestoreKeepsBackups` checks reasons and counts after sleeps,
  never `changed`.
- `bk.state()` has no caller but `listBackups`.
- The restore stores its `upload` row without `stored()` on purpose:
  the reference stays the `before_restore` backup until `restored()`
  after the run, and `backupCheck` does not look while a restore runs.
  It is not this window.

### 3. The lock

`rows` is taken before `mu`, never the other way:
`stored()` and `state()` take `mu` inside it, and `backupCheck` takes `mu` alone.
The writer holds it for one SQLite insert;
`readNTP` and the snapshot, which can take seconds through the queue,
come before it, and the configuration hash is computed before it too.
Lists read it shared, and no path holds a store connection while it waits
for `rows`.

### 4. Out of scope

Deleting the newest backup leaves its hash as the reference,
and deleting all of them turns off "a Cube without a backup gets one at once"
until a restart (`deleteBackup` does not touch `rt.bk`).
Read from the code, not tested; handed to a separate session.

## Implementation Plan

### Block 1: BackupListRace

1. `internal/server/backups.go`:
   - `backupWatch` gains `rows sync.RWMutex`, its comment saying what it
     holds together and the order against `mu`;
   - `takeBackup` computes `backup.ConfigHash(f)` first, then under
     `rows.Lock()` stores the row and, where that succeeded, calls `stored()`.
2. `internal/server/backups_api.go`: `listBackups` reads the settings first,
   then under `rows.RLock()` the rows and `state()`.
3. `CHANGELOG.md`, under `[Unreleased]` → Fixed, not reflowed:

   ```markdown
   - The backup list no longer shows a pending change next to the backup that
     has just covered it (`docs/api-v1.md`, "GET …/backups").
   ```

**Verification:**
- `MASSIMO_ENV_FILE=/dev/null go test -race -count=200 -run 'TestBackupAutomatic$' ./internal/server/` passes 200 of 200;
- the test gate: `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `-race` for `internal/server`, `internal/cubelan` and `internal/proxy`,
  then `make cross`;
- the lint: `gofmt -l .` prints nothing,
  `go vet ./...` for linux, windows and darwin exits 0;
- `changelog.sh check --staged --message <file>` and `commit-msg.sh` pass.

## Proposed commit message

```text
fix(server): The backup list shows a new backup with the clean state

GET …/backups read a Cube's backups and its pending change one after the other, while takeBackup stored the row and only then marked the Cube clean; a list between the two showed the new backup beside the change it had covered. TestBackupAutomatic failed on it about once in twenty runs.

Key changes:
- takeBackup stores the row and marks the Cube clean under one lock, with the hash computed before it
- listBackups reads the rows and the pending change under the same lock, as a reader

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-08): the first revision, from the TL;DR gate of the same night;
  D1-A settled by `PLAN++ as recommended`.

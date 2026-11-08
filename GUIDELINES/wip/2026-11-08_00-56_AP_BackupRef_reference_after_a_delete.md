# AP BackupRef v1.0: The reference after a delete

## TL;DR

Deleting a Cube's newest backup leaves the automatic backup's in-memory
reference (`rt.bk`) as it was; so may a prune with `backups.keep_newest` 0.
The watch now forgets its reference when the newest backup goes,
and the next look reads it again and compares afresh.

- Newest deleted → `changed`/`due` absent though the configuration differs
  from the backup now newest → reference re-read, change shown,
  backup after `auto_min`
- All deleted → no backup until `serve` restarts → one at once (D2-A)
- `forget()` → `known`, `none`, `ref`, **`cur`** and the dirty times cleared;
  `known = false` alone fails, `backupCheck` compares only when the hash
  differs from `cur`
- Under BackupListRace's `rows` lock (merged since the gate) →
  `deleteBackup`'s newest check, delete and `forget()` one step;
  the first look's read shares it; no generation counter
- Only when the deleted row was the newest (D1-A)

### Blocks

1. The reference after a delete: `forget()`, `deleteBackup` and the first look
   under `rows`, two tests, the docs, a `CHANGELOG.md` bullet.
2. The reference after a prune: `s.pruneBackups` for the three callers,
   forgetting every Cube's reference when `keep_newest` is 0, a test,
   the docs, a `CHANGELOG.md` bullet.

## Discussion

The TL;DR gate of 2026-11-08 was answered
`PLAN++ as recommended with the addition fix as set in important notes 1,
committing the plan into main before branching off`:
D1-A, D2-A, D3-A, `forget()` clearing `cur` and the dirty times as well,
the plan's registration pushed to `main` on its own,
and Blocks 1 and 2 as signed commits on a branch from it;
no retirement, push of the branch, pull request or merge.

### D1. When a delete forgets: A, only the newest

Deleting an older backup changes nothing,
and after a restore by id the file restored stays the reference.
Rejected, B, after every delete:
after a restore by id, deleting any old backup would make the Cube differ
from `before_restore`, and an automatic backup would follow.

### D2. A Cube whose last backup was deleted: A, one at once

As `docs/store.md` section 9 (e) and the header of `internal/server/backups.go`
say; `docs/api-v1.md`'s "at its first connection" is reworded to match.
Rejected, B, only after the next change: the history would stay empty as
deleted, but against two documents and the code's own comment.

### D3. The prune: A, Block 2

With `backups.keep_newest` 0 a prune can remove a Cube's newest backup,
the same stale reference. Rejected, B, a note in `docs/store.md` only.

## Analysis

### 1. Both defects, confirmed

Two tests against the fake Cube (`startStored`, `BackupMinute` 20 ms),
run on the commit before BackupListRace merged:

- `TestBackupDeleteNewest`: first backup, a room renamed, the second backup,
  `auto_min` and `auto_max` 1000, the newest deleted;
  the configuration's hash differs from the backup now newest's,
  yet `changed` and `due` stay absent for a second (50 looks).
- `TestBackupDeleteAll`: the first backup deleted;
  no backup follows within a second.

Both failed. In a scratch copy, `known = false` and `touch()` after a delete
of the newest made the second pass and not the first:
`backupCheck` recomputes the dirty state only when the hash differs from `cur`,
and after a delete it does not.
With `cur` and the dirty times cleared as well, both passed,
and `go test -race ./internal/server` passed.

### 2. The lock, since BackupListRace

BackupListRace (merged with pull request 101) added `backupWatch.rows`,
an `RWMutex` taken before `mu`:
`takeBackup` holds it across `AddBackup` and `stored()`,
`listBackups` shares it across the rows and `state()`.

- `deleteBackup` holds it across `NewestBackup`, `DeleteBackup` and `forget()`:
  no backup is stored between the check and the delete,
  so "the deleted row was the newest" is still true when it forgets.
  An unreadable newest forgets too: a re-read costs one row.
- `backupCheck`'s first look shares it across `newestBackupHash` and setting
  the reference: a delete and its `forget()` come wholly before or after,
  so the hash of a row deleted meanwhile is never kept.
  This replaces the generation counter of the gate's sketch.
- The prune runs in `takeBackup` after `rows` is released;
  `forget()` after it takes each Cube's `rows` in turn.

### 3. The prune

`PruneBackups` keeps the newest `keep_newest` of each Cube whatever their age,
so the newest goes only with `keep_newest` 0
(the settings allow 0 to 1000).
It returns a count over every Cube, tombstones included,
not which Cubes lost their newest;
with `keep_newest` 0 and a count above 0,
every Cube's watch forgets, and each re-reads one row at its next look.
Its three callers, `takeBackup`, `pruneBackupsDaily` and the settings change,
use one helper, which also logs a failure as each did.

### 4. What stays

- The change shows at the next look, within a minute (`backupMinute`),
  as for any configuration change: `forget()` clears the dirty state,
  which the next look sets again against the new reference,
  from the time of that look.
- A Cube that was dirty when its newest backup went starts its `auto_max`
  window again from that look: the reference it counted from is gone.

## Implementation Plan

### Block 1: The reference after a delete

1. `internal/server/backups.go`:
   - `forget()` on `backupWatch`: `known`, `none`, `ref`, `cur`,
     `dirtySince` and `lastChange` cleared under `mu`, then `touch()`;
     its comment says the caller holds `rows`;
   - the `rows` comment names `deleteBackup` and the first look;
   - `backupCheck` reads the newest backup's hash and sets the reference
     under `rows.RLock()`;
   - the header: the reference is read again once the newest backup is deleted,
     and a Cube without a backup gets one at once, also after its last is deleted.
2. `internal/server/backups_api.go`: `deleteBackup`, under `rt.bk.rows.Lock()`,
   reads `NewestBackup`, deletes, and calls `forget()` where the row deleted
   was the newest or the newest was not readable.
3. `internal/server/backups_api_test.go`: `TestBackupDeleteNewest` and
   `TestBackupDeleteAll` as in Analysis 1, with a `within` helper.
4. `docs/store.md` section 9 (e) and `docs/api-v1.md` ("Backups" and
   "DELETE …/backups/{id}"): the reference after a delete,
   one at once after the last.
5. `CHANGELOG.md`, under `[Unreleased]` → Fixed, not reflowed:

   ```markdown
   - Deleting a Cube's newest backup makes the one now newest the
     reference of its automatic backup, and deleting its last one takes a
     new one at once, without a restart of `serve`.
   ```

**Verification:**
- both new tests fail without steps 1 and 2 and pass with them;
  `-race -count=20` of `TestBackupDelete|TestBackupAutomatic|TestRestoreKeepsBackups`;
- the test gate: `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `-race` for `internal/server`, `internal/cubelan` and `internal/proxy`,
  then `make cross`;
- the lint: `gofmt -l .` prints nothing,
  `go vet ./...` for linux, windows and darwin exits 0;
- `changelog.sh check` and `commit-msg.sh` pass.

### Block 2: The reference after a prune

1. `internal/server/backups.go`: `s.pruneBackups(ctx, st, now)`:
   `st.PruneBackups`, a failure logged unless the context is done;
   where rows went and `keep_newest` is 0 (or the settings are unreadable),
   `forget()` for every runtime under its `rows`.
   `takeBackup` and `pruneBackupsDaily` call it,
   and `internal/server/settings_api.go` after a retention change.
2. `internal/server/backups_api_test.go`: `TestBackupPruneNewest`:
   the first backup, `keep_newest` 0, `keep_days_auto` 1,
   `s.pruneBackups` two days ahead; a new automatic backup follows within a second.
3. `docs/store.md` section 9 (e) and the header of `internal/server/backups.go`:
   "deleted or pruned".
4. `CHANGELOG.md`, under `[Unreleased]` → Fixed:

   ```markdown
   - A prune that removes a Cube's newest backup, as `backups.keep_newest`
     0 allows, does the same.
   ```

**Verification:** the new test fails without step 1 and passes with it;
then as Block 1.

## Proposed commit message

Block 1:

```text
fix(server.backups): A deleted newest backup is no longer the reference

Deleting a Cube's newest backup left its hash as the automatic backup's reference in memory: GET …/backups showed no change against the backup now newest, and a Cube whose last backup was deleted got none until serve restarted. deleteBackup now has the watch forget its reference when the row deleted was the newest; the next look reads it from the backup now newest, or finds none and takes one at once, and compares afresh.

Key changes:
- backupWatch.forget() clears the reference, the hash last seen and the dirty times, so that the next look compares again
- deleteBackup checks the newest, deletes and forgets under the rows lock, and the first look reads the reference under it too
- two tests: deleting the newest shows the change against the one before, deleting all takes a backup at once
- docs/store.md section 9 (e) and docs/api-v1.md, "Backups"

AP BackupRef v1.0, Block 1.

Assisted-by: claude-code:claude-opus-5-5
```

Block 2:

```text
fix(server.backups): A pruned newest backup is no longer the reference

With backups.keep_newest 0 a prune can remove a Cube's newest backup, which then stayed the automatic backup's reference as a deleted one did. The three prunes go through one helper, which has every Cube's watch forget its reference when rows went and keep_newest is 0.

Key changes:
- s.pruneBackups for takeBackup, the daily prune and a retention change
- a test pruning the only backup two days ahead, after which one follows at once
- docs/store.md section 9 (e)

AP BackupRef v1.0, Block 2.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-08): the first revision, from the TL;DR gate of the same night;
  D1-A, D2-A and D3-A settled by `PLAN++ as recommended`,
  with `forget()` clearing `cur` and the dirty times;
  BackupListRace's `rows` lock in place of the sketch's generation counter.

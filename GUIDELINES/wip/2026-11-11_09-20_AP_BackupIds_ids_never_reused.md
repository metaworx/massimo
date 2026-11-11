# AP BackupIds v1.0: Backup ids never reused

## TL;DR

SQLite gives the highest id of `backups` to the next row once that row is
deleted; migration 0013 makes the table again with `AUTOINCREMENT`,
so a pruned backup's id is never given to another backup.

- `id INTEGER PRIMARY KEY` → `… AUTOINCREMENT`,
  the high mark kept in `sqlite_sequence`
- every hard delete covered: the prune past `keep_newest`, the tombstone purge,
  a Cube's cascade
- additive, `min_reader` 1; ids, uuids, `version`, `deleted_at`, `origin`
  copied as they are
- tests: both reuse paths in the store, 0013's copy and sequence, the
  server's prune test by id
- the stores of v0.13.0-alpha.1 and alpha.2 as release dumps;
  `store.sql` written again

### Blocks

1. Migration 0013 with its tests, the dumps, the documents and the fragment

## Discussion

Found on 2026-11-08 in `internal/server`'s `TestBackupPruneNewest` (AP
BackupRef): after a prune removed a Cube's only backup,
the next automatic backup was id 1 again in every run.
The API names backups by id (`GET`/`DELETE …/backups/{id}`, `…/diff?against=`,
the restore's `?backup=`),
so a list loaded before a prune could delete or download another backup than it
showed.

**The owner's decisions** (`PLAN++`, 2026-11-11, as recommended):

- **D1-A:**
  a migration making `backups` again with `AUTOINCREMENT`, as 0005 made it.
  Not B, keeping the highest id as a tombstone (every delete path would have to
  remember it, the cascade from `cubes` included,
  and the tombstone would never go); not C, the API by uuid (every backup route,
  the UI and the documents change).
- **D2-A:**
  branched from `origin/main`.
  Pull request #104 (AP BackupRef) merged on 2026-11-08,
  so its test comment "the next may get its id" is corrected here,
  and its lines in `docs/store.md` are already on `main`.

`main` moved between the TL;DR gate and this plan:
migrations 0008 to 0012 are taken, so the migration is 0013,
`store.sql` is generated beside the migrations,
the releases' stores are tested from dumps,
and a CHANGELOG bullet is a fragment in `CHANGELOG.d/` (guidelines v4.56.0,
COMMIT.md 5.5).

## Analysis

**Confirmed** on 2026-11-08 with a scratch test against the store's own API,
`keep_newest` 0: backups 1 and 2, 2 pruned as an old `auto`, the next one id 2.
The tombstone path: 2 deleted, its tombstone purged past `keep_days_manual`,
the next one id 2.
The same table made again by hand with `AUTOINCREMENT`:
`sqlite_sequence` follows the rename (`backups` = 1), the row keeps its uuid,
and after the same prune the next one is id 3.

**Why AUTOINCREMENT.**
Without it SQLite takes `max(rowid) + 1` for a new row,
so deleting the highest id hands it out again.
With it SQLite keeps the highest id ever used in `sqlite_sequence` and never
goes below it, whichever path deleted the row: `PruneBackups`' two `DELETE`s,
and the `ON DELETE CASCADE` from `cubes` should a Cube ever be deleted.
The cost is one row in `sqlite_sequence` updated with each insert,
nothing for a table that gains a row an hour at most.
PostgreSQL's identity columns,
which section 5 of `docs/store.md` maps `INTEGER PRIMARY KEY` to,
never reuse a value either, so the mapping holds.

**The rebuild.**
SQLite cannot add `AUTOINCREMENT` to a table,
so 0013 makes it again as 0005 did: a new table, every row copied with its id,
the old one dropped, the new one renamed, the index made.
No table references `backups`, so the `DROP TABLE` fires no foreign key action.
The copy sets `sqlite_sequence` to the highest id copied;
`ALTER TABLE … RENAME` carries the entry to the new name.

**Additive.**
The columns and their meaning stay; only how a new id is chosen changes,
and that is in the table's own schema,
so a binary built for version 12 that opens a file at 13 (`min_reader` 1)
allocates ids the same way.
`min_reader` stays 1 and no `massimo.db.before-13` is written.

**Replication** is not built yet; docs/server.md keys it by `uuid`,
with `version`, `deleted_at` and `origin`, which the copy keeps as they are.

**Limits.**
Ids reused before the upgrade stay so.
The sequence starts at the highest id left,
so an id pruned before the upgrade can be given out once more after it;
nothing in the file remembers it.

**Not here.**
`commands` (`GET /api/v1/commands/{id}`,
pruned after `commands.keep_days`) can reuse an id the same way,
but a rebuild of it would fire `audit.command_id`'s `ON DELETE SET NULL` at the
`DROP TABLE` and clear the audit's links, so it needs a plan of its own.
`events` ids, the log's `?before=` cursor,
repeat only when a Cube's newest event is pruned,
which a connected Cube does not let happen.

**The restore by id** checks the plan's hash, which covers the steps,
so a reused id could restore only a backup with the same steps;
with the fix no id is reused, and no test is added for it.

## Implementation Plan

### Block 1: Migration 0013

1. `internal/store/migrations/0013_backups_autoincrement.sql`:
   the header (`min_reader: 1`, why, why additive),
   `backups_new` with `INTEGER PRIMARY KEY AUTOINCREMENT` and 0005's columns,
   the copy, the drop, the rename, `backups_cube_time`.
2. `go test ./internal/store -run TestStoreSQL -update` writes
   `migrations/store.sql` again.
3. `internal/store/backups_test.go`:
   a pruned newest backup and a purged tombstone leave their ids unused (the two
   paths above); 0013 keeps a file's rows, ids, uuids and bodies,
   starts `sqlite_sequence` at the highest id,
   and a prune of the newest after it leaves its id unused.
4. `internal/server/backups_api_test.go`, `TestBackupPruneNewest`: the next
   backup's id is above the pruned one's, and the comment says why.
5. `tools/store-fixture/make.sh v0.13.0-alpha.1` and `v0.13.0-alpha.2`:
   the releases' stores as dumps in `internal/store/testdata/releases/`,
   so that `TestMigratesReleasedStores` migrates them through 0013.
6. Documents: `docs/store.md` section 5 (the listing's `id`,
   the PostgreSQL mapping, "Migrations" naming 0013) and section 9 (e) (Files,
   Tests); `docs/api-v1.md` "Backups": an id names one backup for good.
7. `CHANGELOG.d/Fixed/claude-backup-ids.md`, the bullet.
8. The scratch test in `GUIDELINES/temp/backupid/` removed.

**Verification:**
`gofmt -l .` empty; `go vet` for linux, windows and darwin;
`MASSIMO_ENV_FILE=/dev/null go test ./...`;
`go test -race` for `internal/store` and `internal/server`; `make cross`;
`changelog.sh check --staged`, `commit-msg.sh`,
`reflow.sh --check` on the documents.

## Proposed commit message

```
fix(store): A pruned backup's id is never given out again
```

## Change History

- v1.0 (2026-11-11): first revision, D1-A, D2-A.

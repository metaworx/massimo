# AP StoreBackups v1.0: the backup history

## TL;DR

Store stage (e): each Cube's backups go into the store's `backups` table, gzip,
taken by the server after a change of the Cube's configuration, by hand,
before every restore and from every uploaded file; listed, downloaded, compared,
restored and pruned through the API and a Settings panel, Backup and restore,
which this stage builds whole.

- automatic → only after a change:
  the configuration's hash against the newest backup's,
  a quiet window `backups.auto_min` (60 min),
  at the latest `backups.auto_max` (120 min) after the first change;
  at once for a Cube without one
- retention → `backups.keep_days_auto` (90), `backups.keep_days_manual` (365),
  `backups.keep_newest` (10 per Cube, whatever their age), server-wide settings
- API → `…/backups` list, take, download, delete, diff;
  plan and restore by `backup=<id>`
- restore → its own `before_restore` backup first, refused without it;
  an upload is stored
- diff → the restore planner's comparison, read-only,
  through `cubelan.SnapshotOf`
- migration 0005 → the empty `backups` table again, reasons `auto`, `manual`,
  `before_restore`, `upload`
- UI → Settings → Backup and restore: download, take now, the list,
  restore from a file or a stored backup with the plan and its progress,
  compare, delete

The decisions are the owner's of 2026-11-07, in the Discussion.
Run order 1 to 5, each block on a branch stacked on the one before.

### Blocks

1. StoreBackups: migration 0005, `internal/store/backups.go` (add, list, read,
   delete, prune), the five settings' keys and defaults, tests,
   on `claude/store-e`
2. ServerBackups: the configuration hash,
   the watch after a change and the automatic backup, the routes,
   restore by id with its own backup and the stored upload,
   the settings in the API, the API documents, on `claude/store-e-2`
3. BackupDiff: `cubelan.SnapshotOf`, the diff of two backups from the planner,
   its route, on `claude/store-e-3`
4. UiBackups: the panel's partial, `backup.js`, the stream's restore events,
   six translations, the pytest test, docs/ui.md, the screenshots,
   on `claude/store-e-4`
5. StoreDocs: docs/store.md section 9 (e) as built, sections 5 and 10,
   the NOTE, on `claude/store-e-4`

## Discussion

The PLAN gate of 2026-11-07 put seven decisions to the owner;
the owner's answers revised two of them and named five settings.
Taken:

- **D1.
  The panel.**
  A (taken): stage (e) builds the whole Settings → Backup and restore panel,
  the download and the restore from a file of proposal section 3 included,
  so that it is written once with the list in it;
  Massimo WSL lead was told, so that the brief's UI session is not started.
- **D2.
  When the server takes a backup by itself.**
  A (taken, as revised): only after a change.
  The configuration of a Cube is the decoded part of a backup without the live
  state (`backup.File` with `State`, `TakenAt`, `Tool`,
  `NTP` and `Lines` left out);
  its SHA-256 is compared with that of the newest stored backup,
  or after a restore with that of the file restored.
  A Cube whose hash differs is dirty from the first such change; its backup is
  taken once no change has come for `backups.auto_min` minutes,
  and at the latest `backups.auto_max` minutes after it became dirty.
  A Cube with no backup gets one at its first connection.
  The check runs at a connection, at a `config`, `metadata`, `paired` or
  `device_forgotten` event and at a restore's end; changes made outside Massimo
  reach the snapshot at the latest with the six-hourly refresh.
  NTP servers are not compared: reading them is a LAN query (`F:`),
  made only when a backup is taken.
  B: exactly 24 hours after the first change.
  C: daily.
- **D3.**
  Dissolved into D2: the reference hash lives in memory,
  read from the newest backup at a connection; no column.
- **D4.
  Retention (the owner's five settings).**
  Server-wide whole numbers in the store's `settings`, through
  `GET`/`PUT /api/v1/settings` (capability `settings`, audited):
  `backups.auto_min` (minutes, 60, at most `backups.auto_max`),
  `backups.auto_max` (minutes, 120, at least `backups.auto_min`),
  `backups.keep_days_auto` (days, 90), `backups.keep_days_manual` (days, 365,
  for `manual`, `upload` and `before_restore`), `backups.keep_newest` (10).
  The newest `backups.keep_newest` of a Cube are kept whatever their age;
  beyond them a backup goes when it is older than its reason's days,
  0 days keeping that reason's for ever.
  A `PUT` that leaves `auto_min` above `auto_max` is refused, nothing written.
- **D5.
  Deleting a backup** needs `restore` (owner and manager),
  so that an operator cannot erase a history others rely on.
- **D6.
  A restore's own backup.**
  The restore takes and stores a `before_restore` backup before its first step,
  and is refused with 503 when that fails;
  a file the restore was sent in its body is stored as `upload`.
  A Cube without a row in the store restores as before, without either.
- **D7.
  The diff** is the restore planner's comparison, read-only:
  the steps that would turn the older backup (`against`) into the other,
  with a snapshot built from the older one's raw lines (`cubelan.SnapshotOf`).

Taken in the plan, for the owner to see:

- **Migration 0005** rebuilds the `backups` table with the reasons `auto`,
  `manual`, `before_restore` and `upload` in place of `connect` and `daily`,
  which belonged to the daily backups D2 replaced.
  No binary has written the table;
  the copy maps the old names to `auto` all the same.
  `min_reader` stays 1.
- The backup taken by hand takes an optional `note`.
- A download is as stored unless `?format=json|yaml` asks otherwise;
  `GET …/backup` (the live Cube) stays as it is.

## Analysis

### 1. What is there (main at 4740636, 2026-11-07)

- The `backups` table is in `0001_initial.sql` with `uuid`, `cube_id`,
  `taken_at`, `reason` (checked against five names), `format`, `tool`, `size`,
  `sha256`, `body` (gzip), `user_id`, `note`, `version`, `deleted_at`, `origin`,
  and `backups_cube_time`; nothing writes it.
- `internal/backup`:
  `Take` builds a `File` from a snapshot (the decoded rooms and devices without
  hidden ones, the raw lines with everything),
  `Marshal`/`Unmarshal` JSON and YAML,
  `NewPlan` compares a file with a `Current` snapshot and lists the steps,
  `Plan.Run` sends them.
- The server: `GET …/backup?format=` takes a backup of the live Cube;
  `POST …/restore/plan` and `POST …/restore?plan=` read the file from the body
  (1 MB at most), `GET …/restore` shows the last;
  the steps go out as one command with `restore_step` and `restore_done` events,
  which the page does not listen to.
- The capabilities `backup` (owner, manager, operator) and `restore` (owner,
  manager) exist; `restore.plan` and `restore.start` are audited.
- The page has no backup panel; `components.js` has `downloadBlob`,
  `filePicker`, `confirmBox` and `detailsPanel`.
- Measured: the six-device fixture Cube's backup is 24 KB of JSON,
  2.2 KB with gzip; thirty devices about 11 KB a backup.

### 2. What changes, by file

| File | Block | Change |
|------|-------|--------|
| `internal/store/migrations/0005_backups_reasons.sql` | 1 | the table again with the four reasons |
| `internal/store/backups.go`, `backups_test.go` | 1 | new: `Backup`, `AddBackup`, `Backups`, `Backup`, `BackupBody`, `NewestBackup`, `DeleteBackup`, `PruneBackups`, `BackupSettings`, the keys |
| `internal/backup/backup.go` | 2 | `ConfigHash` |
| `internal/server/backups.go` | 2 | new: the watch, the automatic backup, `takeBackup`, the prune |
| `internal/server/backups_api.go`, `backups_api_test.go` | 2, 3 | new: the routes |
| `internal/server/restore_api.go` | 2 | `backup=<id>`, `before_restore`, `upload`, the reference after a restore |
| `internal/server/server.go`, `settings_api.go`, `audit.go` | 2 | routes, the five settings and their check, two audit actions |
| `internal/cubelan/snapshot.go` | 3 | `SnapshotOf` |
| `internal/server/ui/partials/backup.html`, `backups-panel.html`, `backup.js` | 4 | new |
| `internal/server/ui/app.js`, `index.html`, `ui.go`, `i18n/*.json`, `style.css` | 4 | the script, the partials, the stream's restore events, texts |
| `tools/ui-test/test_backup.py`, `README.md` | 4 | new test |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/ui.md`, `docs/store.md`, `CHANGELOG.md` | 2 to 5 | documents |

### 3. Risks

- The one connection: a backup is one insert of about 11 KB;
  the prune deletes a handful of rows; neither holds the store for long.
- A restore now waits for its backup (an `F:` query and an insert),
  about the time of a plan.
- A false change after an update of Massimo: a newer version decoding a field
  the older did not gives another hash, so one automatic backup follows;
  harmless, and said in docs/store.md.
- The tests cannot wait minutes:
  the minute is a package variable the tests lower,
  as the events writer's timings are.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`
and `internal/store`), `gofmt -l .` empty, `go vet ./...` for linux,
windows and darwin, `make cross`,
`make ui-test` where a file under `internal/server/ui/` changed,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube or the network: the tests run against the fake Cube.
Each block is one or more functional commits citing `AP StoreBackups v1.0, Block <n>`,
each with its CHANGELOG bullet, signed with `timeout 60 git commit …`,
on its branch, with main merged in before the block starts.
The commit headline's component carries the part it changes: `store.backups`,
`server.backups`, `backup`, `ui.backups`, `ui-test`, `docs(store)`.

### Block 1: StoreBackups

1. `0005_backups_reasons.sql`, `min_reader` 1:
   `backups_new` with the reasons `auto`, `manual`, `before_restore`, `upload`,
   the rows copied with `connect` and `daily` as `auto`, the old table dropped,
   the new renamed, its index again.
2. `internal/store/backups.go`:
   the keys and defaults of D4;
   `Backup{ID, UUID, CubeID, TakenAt, Reason, Format, Tool, Size, SHA256, UserID, Note}`; `AddBackup(ctx, b, body)` (gzip, size and SHA-256 of the body as given, the format `json` or `yaml`, a reason of the four); `Backups(ctx, cubeID)` newest first, without bodies and tombstones; `Backup(ctx, cubeID, id)`, `BackupBody(ctx, cubeID, id)` (the file as given); `NewestBackup(ctx, cubeID)`; `DeleteBackup(ctx, cubeID, id)`, a tombstone with the body emptied; `PruneBackups(ctx, now)` per Cube by D4, tombstones older than `backups.keep_days_manual` removed too; `BackupSettings(ctx)`, the five values with their defaults.
3. Tests: the round trip of a body, the list's order and fields, a tombstone,
   the prune by reason and age with the newest kept, 0 days keeping all,
   the migration's copy of an old reason.

**Verification:**
the test gate with `-race ./internal/store/...`; `check`.

### Block 2: ServerBackups

1. `backup.ConfigHash(f *File) string`:
   the SHA-256 of the JSON of the file's `CubeConfig`,
   `Rooms` and `Devices` with `State` cleared.
2. `internal/server/backups.go`:
   per runtime with a store row,
   the reference hash (from the newest backup at the connection),
   the current hash, `dirtySince` and `lastChange`;
   `noteConfig` after the events of D2;
   a goroutine per runtime with a tick of a minute (`backupTick`,
   `backupMinute` for the tests) that takes the `auto` backup when due;
   `takeBackup(ctx, rt, reason, user, note)`: `F:` for the NTP servers,
   `backup.Take` with the hidden devices left out, JSON, `AddBackup`,
   the reference updated, `PruneBackups` after.
3. `internal/server/backups_api.go`:
   `GET …/backups` (`backup`),
   `POST …/backups` with an optional `{"note": …}` (`backup`,
   audited `backup.take`), `GET …/backups/{id}` (`backup`, `?format=`),
   `DELETE …/backups/{id}` (`restore`, audited `backup.delete`);
   404 for another Cube's id, 503 without a store row.
4. `restore_api.go`:
   `backup=<id>` on both requests reads the stored file;
   `startRestore` stores `before_restore` first (503 when it cannot) and an
   upload as `upload`, with the user; after a restore,
   the reference is the file restored.
5. `settings_api.go`:
   the five whole-number settings, their check together,
   a change of a retention prunes at once.
6. docs/api-v1.md (a section "Backups", the restore's `backup=`, the settings),
   the OpenAPI file, the CHANGELOG under Added.
7. Tests: a change through the API makes the Cube dirty and its backup follows
   after the quiet window,
   a burst of changes waits for `auto_max` at the latest, no change no backup,
   a first connection takes one; the routes' answers and capabilities;
   a restore stores `before_restore` and `upload`; a restore by id;
   the settings' check.

**Verification:**
the test gate with `-race ./internal/server/... ./internal/store/...`;
`TestOpenAPICoversRoutes`; `check`.

### Block 3: BackupDiff

1. `cubelan.SnapshotOf(msgs []cube.Message, now time.Time) (*Snapshot, error)`:
   a snapshot from a greeting's lines, as a connection builds it.
2. `GET …/backups/{id}/diff?against={id}` (`backup`): `NewPlan` of the backup
   against a snapshot of `against`'s lines with its NTP servers, the restore's
   options from the query; the answer `{steps, notes, radio_commands}`.
3. Tests: a backup against itself is empty; against an edited copy (a name,
   a setting, a programme) it lists those three;
   an unknown or foreign id is 404; the documents.

**Verification:**
the test gate; `check`.

### Block 4: UiBackups

1. `partials/backup.html`
   (Settings, after Cubes, `needs-backup`): the format choice,
   **Download backup**, **Take a backup now**, **Restore from file…**, the list;
   `partials/backups-panel.html`: a row's template and the plan's view.
2. `backup.js`:
   the list with time, reason, size, who and note, each row with **Download**,
   **Restore…**,
   **Compare with the one before** and **Delete** (`needs-restore`);
   restore from a file or a stored one shows the plan (steps, notes,
   radio commands and the duty cycle) and **Send N commands**,
   then the steps' progress from `restore_step` and the end from `restore_done`;
   the panel hides the list on a Cube without a store row.
3. `app.js`:
   the stream's `restore_step` and `restore_done` reach the log and the panel.
4. Texts in six languages; `make ui-test` with `test_backup.py`: take one,
   the list shows it, download, compare with itself,
   a restore's plan of a stored backup, delete; the settings shots retaken.
5. docs/ui.md "Backup and restore".

**Verification:**
the test gate; `make ui-test` at 1100 and 390 px in English and German;
the shots looked at; `check`.

### Block 5: StoreDocs

1. docs/store.md: section 9 (e) "Implemented on <date> (AP StoreBackups v1.0)",
   section 5's schema and note on `backups`, section 10's status;
   packaging/README.md's size note gains the backups.
2. The NOTE on how the run went; the completion gate.

**Verification:**
`check`; the completion gate names the NOTE.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +StoreBackups v1.0

Store stage (e), the backup history (the Cubes' backups in the store, taken after a change, by hand and before a restore, listed, compared, restored and pruned, with the Settings panel), gets its plan registered before the first project write.

Key changes:
- GUIDELINES/wip/2026-11-07_01-33_AP_StoreBackups_backup_history.md: the plan, five blocks, with the owner's decisions of 2026-11-07.

File-revision-v1: GUIDELINES/wip/2026-11-07_01-33_AP_StoreBackups_backup_history.md::v1.0::2026-11-07::All::the first revision
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits, each written at its end:

1. `feat(store.backups): the Cubes' backups in the store, pruned by reason and age`
2. `feat(server.backups): backups after a change, by hand and before a restore`
3. `feat(backup): the difference between two stored backups`
4. `feat(ui.backups): Settings → Backup and restore`,
   `test(ui-test): the backup panel in the pytest suite`
5. `docs(store): stage (e) implemented, and the decisions it took`

## Change History

- v1.0 (2026-11-07): the first revision, from docs/store.md section 9 (e),
  the PLAN gate's decisions and the owner's answers of 2026-11-07.

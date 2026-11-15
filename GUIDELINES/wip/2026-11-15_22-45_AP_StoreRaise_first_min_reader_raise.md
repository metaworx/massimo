# AP StoreRaise v1.1: The store's first min_reader raise

## TL;DR

One migration, 0023, raises the store's `min_reader` from 1 to 23. It
drops what was kept only for older binaries, tidies the schema, and gives
an integration's users names and columns that need no fixed key. A raising
migration has a down part, which the store also keeps, so that a store can
go back to a binary that is too old for it.

- **Downgrade:** older binary refuses the raised store → `massimo store
  downgrade -to <N>`, run by the rollback, or by hand with the newer binary
  (D1-D).
- **Gone:** the `events` view, `fillSources`, `cube_options`
  (→ `cubes.proxy_virtual`), `integrations.key`, `user_id`,
  `system_user_id`, `tokens.deleted_at`, `commands.deleted_at`, two unused
  indexes.
- **Backups:** the bodies → `backup_data (id, data)`.
- **Users:** an integration's are `<uuid>:`, `<uuid>:system` and
  `<uuid>:<subject>`, with `users.integration_id`.
- **Who did it:** `effective_user_id` and `real_user_id` in
  `user_identities`, `log` and `audit`; the log's backfilled.
- **Removed rows:** one rule, a UNIQUE that ignores them; no more
  `-del-…` names.
- **Replicated:** `user_identities` gets the owned columns, `roles` a uuid.
- **Shown:** "Home › Anna, as anna".

### Blocks

1. Down parts and `massimo store downgrade`
2. Queries that use their indexes, writers that fill `updated_by`
3. Migration 0023 and the store
4. An integration's users under its name: API, UI, command line

## Discussion

The owner, 2026-11-15, in an evaluation of the tables an integration and
its users fill (AP IntegrationUsers' follow-ups), then `PLAN+ StoreRaise`,
two EVALs of v1.0, and `PLAN+ v1.1 as recommended`. The analysis it rests
on, with a sub-agent's reading of the whole schema against the code:
[ANALYSIS StoreRaise](2026-11-15_21-50_ANALYSIS_StoreRaise_store_schema_min_reader_raise.md)
(its D189 to D198 were working labels; v1.0's Discussion maps them).

**Settled before v1.0:** a plan of its own, which AP IntegrationUsers v1.1
builds on; `users.integration_id`; `log.real_user_id` always filled and
backfilled; an integration's editable name is its own user's
`display_name`, `integrations.name` the reported one, `integrations.key`
gone; a reported name is copied on only while the `display_name` still
equals the one reported before; `:` in an integration user's name; the
`events` view goes; `cube_options` merges into `cubes`; the backup bodies
in a table of their own.

**Decisions:**

- **D1-D:** a raising migration has a down part of plain SQL, which the
  binary embeds and writes into the store (`schema_version.down_sql`).
  `massimo store downgrade -to <N>` runs them. The update's rollback runs
  the newer binary's downgrade (the kept `.failed` one), else the store's
  own down parts. `massimo.db.before-<N>` stays as the last resort.
  - Only `down_sql`: the binary keeps its up migrations, and no older
    binary runs a newer one.
  - No guard on the SQL: whoever can write the file can already change it
    with any SQLite client.
  - No release first: an update need not pass through every release, so a
    v0.14 binary cannot be counted on to know the mechanism. The newer
    binary downgrades for it.
- **D2-A:** `effective_user_id` (whose rights) and `real_user_id` (who did
  it), also for an admin acting as another user; "connecting" would be
  the key's holder, which the token already names. The JSON gets
  `effective_user` and `real_user`; the old keys stay, sent and accepted,
  no longer documented.
- **D3-A:** `<integrations.uuid>:<subject>`: a uuid, as decision 7 asks.
- **D4-A:** one raising migration.
- **D5-B:** `backup_data (id, data)`, `id` the backup's.
- **D6-B:** replication settled in 0023: `user_identities` gets
  `updated_at`, `version`, `origin`, `deleted_at`, and its hard deletes
  become tombstones; `roles` gets a uuid.
- **D7 (the owner's):** `tokens.deleted_at` dropped, `revoked_at` renamed
  `deleted_at`, so that every table's tombstone is `deleted_at`.
- **D8-C:** `integrations.user_id` and `system_user_id` dropped: a user's
  integration is `users.integration_id`, and the key's holder and the
  system user are `<uuid>:` and `<uuid>:system` in it.
- **D9-A:** also `commands.deleted_at` and `commands_cube_time` dropped
  (both unused), `audit_user` replaced by an index the filter uses,
  `auth_providers.auto_domains` a JSON array,
  `integrations.request_expires` → `request_expires_at`, a CHECK on
  `user_identities.last_level`.
- **D10-A:** one rule for a removed row's unique name: a partial UNIQUE
  `WHERE deleted_at IS NULL`, for `users`, `roles`, `auth_providers`,
  `integrations` and `rooms`; the users' `-del-…` suffixes stripped.

**Not in this plan:** the other `*_at` names, `enabled` beside `disabled`,
`log.device`; the backups' other columns.

## Analysis

### The downgrade

- **Why:** after a failed health check, `massimo update` puts the previous
  binary back (`cmd/massimo/update.go:257`, `internal/update/install.go:116`),
  as `-rollback` does. Once the newer binary migrated the store, that
  binary refuses it (`internal/store/migrate.go:150`). apt, brew and the
  Windows installer never downgrade.
- **The down parts:** `NNNN_<name>.down.sql` beside a migration whose
  `min_reader` rises; a test requires one for each. A migration that keeps
  `min_reader` needs none: an older binary reads it.
- **In the store:** `schema_version.down_sql`, added by migration 0022
  (additive). A binary writes the down part of each version it knows
  where the row has none, at every start.
- **`massimo store downgrade -to <N>`:** with the service stopped, runs the
  down parts from the newest version to the one after `N`, in one
  transaction, foreign keys off and checked before the commit, the
  `schema_version` rows above `N` deleted. The SQL comes from the binary,
  else from the store. A copy `massimo.db.before-downgrade-<version>`
  first, as `copyBefore` does.
- **The rollback:** where the store is too new for the binary being put
  back, it runs `<.failed binary> store downgrade -to <its schema>`; where
  that binary is gone or fails, the store's down parts; the result in the
  update status.
- **A binary up to v0.16** has no such rollback: its update then leaves the
  service down, and `packaging/README.md` says to run the `.failed`
  binary's `store downgrade`.
- **Lossy** only in what the newer schema added: for 0023, the new
  columns' values, the soft-deleted identities, the stripped name
  suffixes.

### The down part of 0023

Back to the schema of `store.sql` at version 21 (from git), the tests
comparing `sqlite_master`:

- `integrations.key` from the name as `integrationKey` makes it, with
  `-2`, `-3` where taken; `user_id` and `system_user_id` from
  `users.integration_id` and the names; the users' names back to `<key>`,
  `<key>.system` and `<key>.<8 hex>`;
- `backups.body` from `backup_data`; `cube_options` from `cubes`; the view
  and its triggers made again;
- the renames reversed; `real_user_id` and the replication columns
  dropped; soft-deleted identities removed;
- the removed users renamed `-del-<time>` again; `users.name` UNIQUE.

### The migration's parts

- **Kept only for older binaries:** the `events` view and its three
  INSTEAD OF triggers (0014; only tests use them); `fillSources`
  (`events.go:300`, 543–565), whose UPDATE runs once; `cube_options`
  (0018) → `cubes.proxy_virtual INTEGER NOT NULL DEFAULT 0`.
- **`backup_data (id INTEGER PRIMARY KEY REFERENCES backups (id) ON DELETE
  CASCADE, data BLOB NOT NULL)`**, filled where a backup is not a
  tombstone, then `ALTER TABLE backups DROP COLUMN body` (SQLite 3.35+,
  which modernc.org/sqlite 1.62 is past), keeping the AUTOINCREMENT
  sequence. A deleted backup's data row goes.
- **The log and the audit:** `user_id` → `effective_user_id`;
  `real_user_id` added and backfilled from it, an audit row whose detail
  has `via` and `for.id` taking that identity's own user; a partial index
  on each. Done before `integrations.key` goes, since `via` names it.
  `log` is altered, never rebuilt: the UPDATE touches only the rows with
  a user, which `log_user` finds.
- **An integration's users:** `users.integration_id` from
  `integrations.user_id`, `system_user_id` and
  `user_identities.own_user_id`; the names `<uuid>:`, `<uuid>:system`,
  `<uuid>:<subject>`; the own user's `display_name` as it is.
- **`integrations`** made again, without `key`, `user_id`,
  `system_user_id`, with `request_expires_at`.
- **`user_identities`** made again: `effective_user_id`, `real_user_id`,
  `updated_at`, `version`, `origin`, `deleted_at`, the `last_level`
  CHECK; its partial uniques ignore removed rows.
- **`users`** made again: `UNIQUE` → `CREATE UNIQUE INDEX users_name ON
  users (name) WHERE deleted_at IS NULL`; the suffixes
  `-del-YYYYMMDD-HHNN` and `-del-YYYYMMDD-HHNN-<id>` stripped.
- **`roles`:** a uuid, written by an `afterMigration` step (SQL makes no
  UUIDv7), and the partial unique on `key`; `AddRole` adds a row instead of
  taking a removed one over (`roles.go:172`). `cubes` keeps taking over
  its removed row: a serial is the same device coming back.
- **`tokens`:** `deleted_at` dropped, `revoked_at` → `deleted_at`; the API
  keeps `revoked_at`.
- **The rest of D9.**
- The whole migration runs with `-- foreign_keys: off` (0014, 0021);
  a table is made again as `_new`, copied, dropped and renamed, never
  renamed aside first (SQLite 3.26+ rewrites the children's references).
- **Names inside old rows' JSON** (`log.data`, the audit's `via`) are
  history, kept as written.

### The name rule

- Regular names: `^[a-z0-9][a-z0-9._@-]{0,63}$` (`users.go:33`), unchanged.
- An integration's: `^<uuid>:([a-z0-9]{1,64}|system)?$`, set by the
  store. A Home Assistant user id outside it is refused at the first call
  with a 400.
- `/api/v1/users/{name}` takes `:` unescaped.

### The display

- **The chain:** "<own user's display_name> › <the user's display_name>",
  and ", as <name>" where the identity is linked, wherever a user is
  shown: the page, the command line, the server's own log lines. It comes
  from `users.integration_id`; nothing parses the name.
- **The API:**
  - a user carries `integration` (its uuid) and `display`;
  - an integration carries `name` (the own user's `display_name`, set by
    `PUT /api/v1/integrations/{uuid}`) and `reported_name`; `key` and
    `user` leave its view;
  - log and audit rows carry `effective_user` and `real_user`, with `user`
    kept as before; the filter by user matches either.
- **The Home Assistant integration** titles its entry `Massimo
  (<user.name>)` (massimo-homeassistant, `config_flow.py:48`), which would
  become a uuid: it takes `display_name`. That repository's session is
  told.

## Implementation Plan

### Block 1: Down parts and `massimo store downgrade`

1. **Migration 0022** (`-- min_reader: 1`): `schema_version.down_sql TEXT`.
2. **The runner** (`migrate.go`): embeds `*.down.sql`, writes each into
   its `schema_version` row at every start where empty; a raising
   migration without a down part fails the tests.
3. **`massimo store downgrade -to <N>`** (`cmd/massimo`), with the store
   function behind it; refuses a running `serve` (the store's lock).
4. **The rollback** (`internal/update`, `cmd/massimo/update.go`): the
   store's version against the binary put back; the downgrade as in the
   Analysis.
5. **Tests:**
   - a test-only raising migration with its down part: up, down, the
     schema as before, the rows kept;
   - the down part taken from the store where the binary has none;
   - the rollback with a store too new (Linux, and the Windows test
     binary through interop).
6. **Docs:** `docs/store.md` ("Migrations"), `packaging/README.md`
   (rollback, no longer "none so far"; v0.16 and earlier), `README.md` for
   the command, `docs/update-windows.md`.
7. CHANGELOG fragment `Added`.

- **Verification:** the Go gate, `make race`, `make cross`, the lint
  commands, `GUIDELINES/shared/tools/check`.

### Block 2: Queries that use their indexes, writers that fill `updated_by`

1. `commands.go:314`, `:338`: the states written literally,
   `state IN ('queued', 'sent')`, so that `commands_pending` serves them.
2. `audit.go:158`: the user filter resolves the name to an id and filters
   `a.user_id = ?`.
3. `UpdateCube`, `SetSetting` and the rooms' writers take who changed it
   and fill `updated_by`; `rooms.origin` and `tokens.origin` on every
   write.
4. **Tests:** `EXPLAIN QUERY PLAN` names the index for both; the columns
   filled.
5. CHANGELOG fragment `Fixed`.

- **Verification:** as Block 1.

### Block 3: Migration 0023 and the store

1. **`0023_raise.sql`** (`-- min_reader: 23`, `-- foreign_keys: off`), in
   this order: the view; `fillSources`' UPDATE; `cube_options`;
   `backup_data`; the log and the audit; the integration's users;
   `integrations`; `user_identities`; `users`; `roles` (and its
   `afterMigration` step); `tokens`; the rest of D9.
2. **`0023_raise.down.sql`:** as "The down part of 0023".
3. **The store's code:**
   - the new column names (`EffectiveUserID`, `RealUserID`, tokens'
     `DeletedAt`);
   - `cube_options.go` removed, `scanCube` reads `proxy_virtual`;
   - `AddBackup` writes both rows in one transaction, the data read from
     `backup_data`;
   - `freeIntegrationKey` and the 8-hex names removed; `IntegrationUser`
     names a new user `<uuid>:<subject>`; `IntegrationOfUser` by
     `users.integration_id`;
   - the copy rule for reported names;
   - the log's and the audit's writers fill `real_user_id`;
   - `DeleteUser` no longer renames; `AddRole` adds a row; the identities'
     deletes become tombstones, their reads skip them;
   - `fillSources` removed.
4. **`store.sql`** regenerated ("min_reader 23").
5. **Tests:**
   - the v0.16.0-alpha.1 store migrated: no table loses rows; names,
     `integration_id`, the backfill's counts, the suffixes stripped, the
     foreign-key check;
   - up then down: the schema equal to version 21's `store.sql` from git,
     and the v21 binary's store tests passing on it;
   - a binary for 22 refused, with the downgrade named;
   - the view's tests and `TestOlderReaderOnNine` removed;
   - a store with 500 000 log rows migrated and downgraded, the times in
     the NOTE.
6. **Docs:** `docs/store.md` (section 5, section 6 on removed names,
   "Migrations", the identifiers, the backups and `cube_options` notes).
7. CHANGELOG fragment `Changed`: earlier releases cannot open a migrated
   store without `massimo store downgrade`.

- **Verification:** as Block 1, `make race` with `-count=1`.

### Block 4: An integration's users under its name

1. **The API** as "The display": users, integrations (`PUT` sets the
   name), log and audit rows, the filter.
2. **The UI:** users, log and audit show the chain; the integration's
   panel edits its name and shows the reported one where they differ; the
   texts in the six languages.
3. **The command line:** `massimo user list` and the log's printout show
   the chain.
4. **The server's own log lines** name an integration's user by the chain.
5. **Tests:** Go for the API, the old JSON keys included;
   `test_integrations.py` and the users' and log's UI tests at 1100 and
   390 px.
6. **Docs:** `docs/api-v1.md`, the OpenAPI file (`TestOpenAPICoversRoutes`),
   `docs/ui.md`, `README.md`.
7. CHANGELOG fragment `Changed`.
8. **The Home Assistant integration's session** is told: the entry's
   title from `display_name`.

- **Verification:** as Block 1, plus the touched UI test files
  (`make ui-test` with `-k`).

## Proposed commit messages

- `chore(aps): StoreRaise v1.1`: this revision.
- Block 1: `feat(store): A store goes back to an older binary with massimo store downgrade`
- Block 2: `perf(store): The pending commands and the audit's user filter use their indexes`
  and `fix(store): Changes to Cubes, settings and rooms say who made them`
- Block 3: `feat(store)!: Schema 23, the first a release before it cannot read`,
  with a `BREAKING CHANGE:` trailer
- Block 4: `feat(server): An integration's users are shown under its name`

## Change History

- **v1.0** (2026-11-15): first version, on the owner's `PLAN+ StoreRaise`,
  after the evaluation of an integration's tables and a sub-agent's
  reading of the schema; D1 to D9 open.
- **v1.1** (2026-11-15): the owner's `PLAN+ v1.1 as recommended`, after
  two EVALs: D1-D (down parts in the binary and the store, `massimo store
  downgrade`, no release first; Block 1 rewritten, the raise becomes 0023
  after the additive 0022), D2-A with the JSON's old keys kept, D3-A,
  D4-A, D5-B (`backup_data`), D6-B (`user_identities` and `roles`
  replicated), D7 (the owner's: `tokens.revoked_at` → `deleted_at`), D8-C
  (`integrations.user_id` dropped too), D9-A, and D10-A, new (one rule for
  removed rows).

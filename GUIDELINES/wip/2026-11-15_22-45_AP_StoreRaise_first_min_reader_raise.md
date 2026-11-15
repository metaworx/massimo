# AP StoreRaise v1.0: The store's first min_reader raise

## TL;DR

One migration, 0022, raises the store's `min_reader` from 1 to 22. It
drops what was kept only for older binaries, tidies the schema, and gives
an integration's users names and columns that need no fixed key. A
release before it makes the update's rollback survive a raise.

- **Rollback:** the old binary refuses a raised store, so the service stays
  down → the update helper puts `massimo.db.before-<N>` back (D1-A).
- **Gone:** the `events` view and its triggers, `fillSources`,
  `cube_options` (→ `cubes.proxy_virtual`), `integrations.key`,
  `integrations.system_user_id`, `commands.deleted_at`, two unused indexes.
- **Backups:** the bodies → `backup_bodies`, one row per backup (D5-A).
- **Users:** an integration's are `<integrations.uuid>:<subject>`
  (`<uuid>:` and `<uuid>:system` for its own and its system user), with
  `users.integration_id` (D3-A).
- **Who did it:** `user_id` → `effective_user_id`, and `real_user_id`
  beside it in `user_identities`, `log` and `audit`, the log's backfilled
  (D2-A).
- **Names:** an integration's editable name is its own user's
  `display_name`; `integrations.name` and `user_identities.name` are the
  names as reported, copied on only while unchanged.
- **Shown** as "Home › Anna, as anna".

### Blocks

1. A rollback across a store migration
2. Queries that use their indexes, writers that fill `updated_by`
3. Migration 0022 and the store
4. An integration's users under its name: API, UI, command line

## Discussion

The owner, 2026-11-15, in an evaluation of the tables an integration and
its users fill (AP IntegrationUsers' follow-ups), then `PLAN+ StoreRaise`.
The analysis it rests on, with a sub-agent's reading of the whole schema
against the code:
[ANALYSIS StoreRaise](2026-11-15_21-50_ANALYSIS_StoreRaise_store_schema_min_reader_raise.md).
Its decision labels D189 to D198 were working labels; this plan numbers
its own:

| Analysis | Here | Subject |
|----------|------|---------|
| D196 | D1 | the rollback across a raise |
| D189 | D2 | the user columns' names |
| D192 | D3 | an integration user's name |
| D195 | D4 | one migration or two |
| — | D5 | the backup bodies' table |
| D198 | D6 | the replicated tables |
| — | D7 | `tokens.deleted_at` |
| D194 | D8 | `integrations.system_user_id` and `user_id` |
| D195 | D9 | the clean-up set |

**Settled by the owner:**

- **A plan of its own**, which AP IntegrationUsers v1.1 builds on
  (`PLAN+ StoreRaise`).
- **`users.integration_id`**, nullable, set for an integration's users
  (the owner's proposal; analysis D190-A).
- **`log.real_user_id` always filled** where a row has a user: for a
  regular user the same as the effective one; **backfilled** in the
  migration ("I would backfill"; analysis D191-F).
- **An integration's editable name is its own user's
  `users.display_name`;** `integrations.name` stays the name Home
  Assistant reported, and `integrations.key` goes: no fixed identifiers
  beyond `id` and `uuid` ("I hate it when we have three fixed
  unchangeable identifiers in one table").
- **The copy rule:** a name reported anew is copied into the
  `display_name` only while that still equals the name reported before,
  for an integration's own user and for each identity's user (analysis
  D193-A).
- **`:` separates** an integration user's name, which regular names cannot
  contain (the owner's `<integrations.id>:<subject>` or
  `<integrations.uuid>:<subject>`; which of the two is D3).
- **The view `events` goes with the raise** (the owner; 0014 says so).
- **`cube_options` merges into `cubes`** (the owner's question,
  "proxy_virtual would be added (moved) to `cubes`?", read as yes).
- **The backup bodies in a table of their own** (the owner's proposal).

**Open decisions:**

- **D1, the rollback across a raise:**
  - **A (recommended):** the update helper, rolling back to a binary the
    store is too new for, puts `massimo.db.before-<N>` back and keeps the
    newer file as `massimo.db.after-<N>`, with an Error in the log naming
    both. Block 1 ships in a release before Block 3's, since the
    rollback runs in the older binary.
  - **B:** the helper refuses the rollback across a raise and says what
    to do by hand; the service stays on the new binary.
  - **C:** none; `packaging/README.md` says what to do by hand.
- **D2, the user columns' names:**
  - **A (recommended):** `effective_user_id` (whose rights) and
    `real_user_id` (who did it), as Unix's effective and real user ids,
    in `user_identities`, `log` and `audit`; the API's JSON keeps `user`
    and `own_user`.
  - **B:** `effective_user_id` and `connected_user_id` (the owner's
    words); "connected" reads as the linked user, the other column.
  - **C:** keep `user_id` and `own_user_id`, and name the log's new column
    `own_user_id`.
- **D3, an integration user's name:**
  - **A (recommended):** `<integrations.uuid>:<subject>`: a uuid, as
    decision 7 asks of a replicated row's references; up to 69
    characters, 87 with a removal's suffix.
  - **B:** `<integrations.id>:<subject>`: 34 characters, but a row id
    local to this store.
- **D4, one migration or two:**
  - **A (recommended):** one, 0022: one copy of the whole store on the SD
    card, one release boundary.
  - **B:** two, 0022 the clean-up and 0023 the integration's users: two
    smaller reviews, two copies.
- **D5, the backup bodies' table:**
  - **A (recommended):** `backup_bodies (backup_id INTEGER PRIMARY KEY
    REFERENCES backups (id) ON DELETE CASCADE, body BLOB NOT NULL)`: the
    code's name `body` kept, the key saying what it points at.
  - **B:** `backup_data (id, data)`, the owner's first words.
- **D6, the replicated tables:** `user_identities` has a uuid but no
  `version`, `origin` or `deleted_at`, and is hard-deleted; `roles`
  replicates without a uuid.
  - **A (recommended):** not in this plan; an analysis of what replicates
    first, since decision 14's list leaves both unclear.
  - **B:** settle both in 0022.
- **D7, `tokens.deleted_at`,** never read or written:
  - **A (recommended):** keep: decision 14's column, which
    `store_test.go` holds; `revoked_at` stays the token's tombstone.
  - **B:** drop it with `commands.deleted_at`.
- **D8, the integration's user columns:**
  - **A (recommended):** drop `integrations.system_user_id`, which the
    identity with subject `system` already names; keep
    `integrations.user_id`, the plain "this row's user", as in `tokens`.
  - **B:** keep both.
- **D9, the clean-up set beyond what the raise needs:**
  - **A (recommended):** `commands.deleted_at` and `commands_cube_time`
    dropped, `audit_user` replaced by `(user_id) WHERE user_id IS NOT NULL`,
    `auth_providers.auto_domains` a JSON array,
    `integrations.request_expires` → `request_expires_at`, a CHECK on
    `user_identities.last_level`.
  - **B:** only what the raise needs: the view, `fillSources`,
    `cube_options`, the bodies and the integration's changes.

**Not in this plan:** moving nothing else of the backups (the bodies'
table settles it); the other `*_at` names, `enabled` beside `disabled`,
`log.device` (renamed only where a table is touched anyway); the
replication of `user_identities` and `roles` (D6).

## Analysis

### Why the raise breaks the rollback

- `massimo update` swaps the binary, restarts the service and waits for
  the health check; on a failure it puts the previous binary back
  (`cmd/massimo/update.go:257`, `internal/update/install.go:116`), as
  `massimo update -rollback` does.
- That is the older binary's code: the one that ran the update.
  The newer binary has migrated the store meanwhile, and the older one
  refuses it (`internal/store/migrate.go:150`, "needs a binary for
  version 22 or later").
- `copyBefore` (`migrate.go:303`) has left `massimo.db.before-22`, the
  store as it was. Putting it back loses every write since: minutes after
  a failed health check, possibly days after a manual rollback.
- apt and brew never downgrade; the Windows installer does not either.
  Only `massimo update`'s two paths are concerned.
- The helper finds the store as `serve` does (`-data`, else
  `MASSIMO_DATA` from the service's env file), opens it read-only, reads
  `schema_version` and computes the copy's version as `migrate.go:149`
  does: the lowest version whose `min_reader` exceeds the restored
  binary's `SchemaVersion()`.

### The migration's parts

- **Kept only for older binaries:**
  - `events` and `events_insert`, `events_update`, `events_delete`
    (0014): only tests use them;
  - `fillSources` (`events.go:300`, 543–565): its UPDATE runs once in the
    migration, the function goes;
  - `cube_options` (0018): `cubes.proxy_virtual INTEGER NOT NULL DEFAULT
    0`, copied over; a change then raises `cubes.version` and fills
    `cubes.updated_by`.
- **The backup bodies:** `backup_bodies`, filled from `backups.body` where
  the backup is not a tombstone, then `ALTER TABLE backups DROP COLUMN
  body` (modernc.org/sqlite 1.62 is past 3.35), which keeps the
  AUTOINCREMENT sequence; no 0013-style rebuild. A deleted backup stays a
  tombstone in `backups`, and its body row goes.
- **An integration's users,** in SQL alone, since every value is in the
  store:
  - `users.integration_id` from `integrations.user_id`,
    `integrations.system_user_id` and `user_identities.own_user_id`;
  - names: the own user `<uuid>:`, the system user `<uuid>:system`, a Home
    Assistant user `<uuid>:<subject>` from its identity; a removed user's
    `-del-…` suffix kept;
  - the own user's `display_name` stays what it is (the reported name at
    approval), now the editable one;
  - `integrations.key` and `integrations.system_user_id` dropped (D8): a
    rebuild of `integrations` with `-- foreign_keys: off`, as 0021 made
    it, since `user_identities` references it.
- **The user columns** (D2): `RENAME COLUMN` in `user_identities`, `log`
  and `audit`, their indexes made again under the new names;
  `real_user_id` added to `log` and `audit`, backfilled from the
  effective user; in the audit, a row whose detail has `via` (the key)
  and `for.id` (the subject) takes that identity's own user instead.
  The log's rows say nothing of an identity and keep the effective user.
  A partial index on each.
- **The clean-up** (D9-A) as listed there; the `last_level` CHECK needs
  `user_identities` made again, which the renames do not.
- **Never rebuilt:** `log` (tens of MB on a Pi 1): only `ALTER TABLE`
  and one UPDATE over the rows with a user, which `log_user` finds.
- **Not rewritten:** names inside old rows' JSON (`log.data`, the audit's
  `via: <key>`): history, shown as written.

### The name rule

- Regular names keep `^[a-z0-9][a-z0-9._@-]{0,63}$` (`users.go:33`).
- An integration's: `^<uuid>:([a-z0-9]{1,64}|system)?(-del-[0-9-]+)?$`,
  checked by the store, never by a user's input. A subject outside it
  is refused at the first call with a 400.
- Lookups by name keep working: `/api/v1/users/{name}` takes `:` unescaped.

### The display

- `users.integration_id` says a user is an integration's; the page, the
  command line and the server's own log lines show "<own user's
  display_name> › <the user's display_name>", and ", as <name>" where the
  identity is linked. Nothing parses the name.
- The API: a user carries `integration` (its uuid) and `display`, the
  string as shown; an integration carries `name` (the own user's
  `display_name`, editable by `PUT /api/v1/integrations/{uuid}`) and
  `reported_name` (`integrations.name`); `key` leaves the integration's
  view.
- The Home Assistant integration titles its entry `Massimo (<user.name>)`
  (massimo-homeassistant, `config_flow.py:48`), which would become a
  uuid: it takes `display_name`; that repository's plan changes it, told
  by this plan's session.

## Implementation Plan

### Block 1: A rollback across a store migration

1. `internal/update`: `StoreTooNew(dataDir string, reader int)` reads the
   store read-only and returns the copy to put back, or none.
2. **The rollback** (`massimo update`'s, after a failed health check, and
   `-rollback`): with the service stopped, where the store is too new for
   the binary put back, `massimo.db` → `massimo.db.after-<N>` (with
   `-wal`, `-shm`), `massimo.db.before-<N>` → `massimo.db`; an Error line
   and the update status naming both. Where no copy is there, the
   rollback is refused with what to do (D1-B's text).
3. **The data directory** as `serve` resolves it, from the same env file.
4. **Tests:** a store whose `schema_version` has a `min_reader` above
   `SchemaVersion()`, with and without its copy; Linux and the Windows
   test binary through interop.
5. **Docs:** `packaging/README.md`'s rollback paragraph (no longer "none
   so far"), `docs/update-windows.md`.
6. CHANGELOG fragment `Fixed`.

- **Verification:** the Go gate, `make race`, `make cross`, the lint
  commands, `GUIDELINES/shared/tools/check`.
- **Release:** Block 3 merges only once a release carries Block 1; the
  gate after Block 1 asks the owner for it.

### Block 2: Queries that use their indexes, writers that fill `updated_by`

1. `commands.go:314`, `:338`: the states written literally,
   `state IN ('queued', 'sent')`, so that `commands_pending` serves them.
2. `audit.go:158`: the user filter resolves the name to an id first and
   filters `a.user_id = ?`.
3. `UpdateCube`, `SetSetting` and the rooms' writers take who changed
   it and fill `updated_by`; `rooms.origin` and `tokens.origin` on every
   write.
4. **Tests:** `EXPLAIN QUERY PLAN` names the index for both; the columns
   filled.
5. CHANGELOG fragment `Fixed`.

- **Verification:** as Block 1.

### Block 3: Migration 0022 and the store

1. `internal/store/migrations/0022_raise.sql`, `-- min_reader: 22`,
   `-- foreign_keys: off`: every part of the Analysis, in this order:
   the view; `fillSources`' UPDATE; `cube_options`; `backup_bodies`;
   `log` and `audit` renamed, `real_user_id` added and backfilled, the
   indexes (before `integrations.key` goes, since the audit's `via` names
   it); `users.integration_id` and the names; `integrations` made again;
   `user_identities` made again with the renames and the CHECK; the rest
   of D9.
2. **The store's code:** the columns' new names (`EffectiveUserID`,
   `RealUserID`); `cube_options.go` removed, `scanCube` reads
   `proxy_virtual`; `AddBackup` writes both rows in one transaction, the
   body read from `backup_bodies`; `freeIntegrationKey` and the 8-hex
   names removed, `IntegrationUser` names a new user `<uuid>:<subject>`;
   the copy rule for reported names; the log's and the audit's writers
   fill `real_user_id`; `fillSources` removed.
3. **`store.sql`** regenerated ("min_reader 22").
4. **Tests:** the v0.16.0-alpha.1 store migrated, no table losing rows,
   the names and `integration_id` as above, the backfill's counts, the
   foreign-key check; a binary for 21 refused with the copy's name;
   the view's tests and `TestOlderReaderOnNine` removed; a migration of a
   store with 500 000 log rows timed, the time in the NOTE.
5. **Docs:** `docs/store.md` (section 5, "Migrations", the identifiers
   note, the backups and `cube_options` notes).
6. CHANGELOG fragment `Changed`, saying that earlier releases cannot open
   a migrated store.

- **Verification:** as Block 1, `make race` with `-count=1`.

### Block 4: An integration's users under its name

1. **The API:** a user's `integration` and `display`; an integration's
   `name` (editable) and `reported_name`, `key` gone; the log's and the
   audit's rows carry the real user beside the effective one, and the
   filter by user matches either; the audit's `for` keeps the level only.
2. **The UI:** users, log and audit show the chain; the integration's
   panel edits its name and shows the reported one where they differ; the
   texts in the six languages.
3. **The command line:** `massimo user list` and the log's printout show
   the chain.
4. **The server's own log lines** name an integration's user by the chain.
5. **Tests:** Go for the API; `test_integrations.py` and the users' and
   log's UI tests at 1100 and 390 px.
6. **Docs:** `docs/api-v1.md`, the OpenAPI file (`TestOpenAPICoversRoutes`),
   `docs/ui.md`, `README.md` for the command line.
7. CHANGELOG fragment `Changed`.
8. **The Home Assistant integration's session** is told: the entry's
   title from `display_name`.

- **Verification:** as Block 1, plus the touched UI test files
  (`make ui-test` with `-k`).

## Proposed commit messages

- `chore(aps): +StoreRaise v1.0`: this plan and the analysis it rests on.
- Block 1: `fix(update): A rollback puts back the store an older binary can read`
- Block 2: `perf(store): The pending commands and the audit's user filter use their indexes`
  and `fix(store): Changes to Cubes, settings and rooms say who made them`
- Block 3: `feat(store)!: Schema 22, the first a release before it cannot read`,
  with a `BREAKING CHANGE:` trailer
- Block 4: `feat(server): An integration's users are shown under its name`

## Change History

- **v1.0** (2026-11-15): first version, on the owner's `PLAN+ StoreRaise`,
  after the evaluation of an integration's tables and a sub-agent's
  reading of the schema; D1 to D9 open.

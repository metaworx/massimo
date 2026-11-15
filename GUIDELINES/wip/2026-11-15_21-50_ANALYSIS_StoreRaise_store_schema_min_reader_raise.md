# The store's first min_reader raise: integration users and a schema clean-up

An analysis for the owner, 2026-11-15, schema version 21 on `origin/main`.
It collects what the evaluation of the integration's tables settled in chat,
the findings of a sub-agent that read the whole schema against the code,
and the decisions left.
Nothing is planned or changed yet;
the work would go into AP IntegrationUsers v1.1, or a plan of its own (D195).

**Decision numbers.**
Since the chat numbered them, D177 to D188 were taken on main
by AP RoomConfirm and NOTE RadioHealth v1.2.
The decisions of this evaluation are renumbered from D189:

| In chat | Now | Subject |
|---------|-----|---------|
| D177 | D189 | the user columns' names |
| D178 | D190 | `users.integration_id` |
| D179 | D191 | the log's second user column |
| D180 | D192 | an integration user's name |
| D181 | D193 | the copy rule for reported names |
| D182 | D194 | the integration's names |
| new  | D195 to D198 | the raise, the rollback, the clean-up, the replicated tables |

The session's earlier D23 to D32 (IntegrationUsers v1.1's follow-ups) overlap
numbers on main as well (D23 is Packaging's);
the plan revision gives them numbers above the highest then on main and lists both.

## 1. The integration's tables, as the evaluation left them

| Table.column | Now | Proposed |
|--------------|-----|----------|
| `integrations.key` | the fixed handle, prefix of the user names | dropped (D194-C) |
| `integrations.name` | Home Assistant's location name | unchanged: the name as reported |
| own user's `users.display_name` | the reported name, at approval | the editable name of the integration (D194-C) |
| `integrations.user_id` | the integration's own user, holding the key | kept: the plain "this row's user", as in `tokens` |
| `integrations.system_user_id` | the system user | dropped: the identity with subject `system` already names it (finding 10) |
| `users.integration_id` | — | new, nullable: the integration a user belongs to (D190-A) |
| `users.name` of an integration's user | `home`, `home.system`, `home.0123abcd` | `<uuid>:`, `<uuid>:system`, `<uuid>:<subject>`, under a rule of their own (D192-F) |
| `user_identities.user_id` | the user it acts as | `effective_user_id` (D189-C) |
| `user_identities.own_user_id` | the user made for it | `real_user_id` (D189-C) |
| `user_identities.name` | the name reported at the last call | unchanged: the name as reported |
| `log.user_id`, `audit.user_id` | the acting user | `effective_user_id` (D189-C) |
| `log.real_user_id`, `audit.real_user_id` | — | new: who did it; for a regular user the user (D191-E) |

- **Shown** as "Home › Anna": the own user's `display_name` › the identity's own user's `display_name`,
  and ", as anna" where the identity is linked.
  The page and the command line take the integration from `users.integration_id`,
  never from the name.
- **The copy rule (D193-A):** a newly reported name is copied into the `display_name`
  only while that still equals the name reported before,
  for the integration's own user and for each identity's own user alike.
  Today [integrations.go:595](../../internal/store/integrations.go) overwrites it whenever Home Assistant's name changes.
- **The name rule** must allow `:` and the length:
  36 + 1 + 32 = 69, and 87 once a removal adds its `-del-YYYYMMDD-HHNN` (0015).
  Regular names stay `^[a-z0-9][a-z0-9._@-]{0,63}$`, so nobody can take one.
- **`log.real_user_id` gets an index**, `(real_user_id) WHERE real_user_id IS NOT NULL`,
  as `log_user` has.
- **Backfill:** "always filled" means an `UPDATE log SET real_user_id = user_id`
  over every row with a user: a write of the whole log, tens of MB on a Pi 1,
  in the migration.
  The alternative is no backfill and `COALESCE(real_user_id, effective_user_id)` in the reads;
  D191 asks.
- **Falls away:** `freeIntegrationKey` and its `-2` suffixes, the "longer where taken" step,
  D29 (the key editable at approval) and D32 (the reserved prefix).

## 2. What the schema keeps for older binaries

Little, the sub-agent found, and all of it goes cleanly in the raising migration:

1. **The view `events` and its three INSTEAD OF triggers** (0014):
   only tests use them (`log_test.go`, `releases_test.go`, `store_test.go:122`).
2. **`fillSources`**, an UPDATE of the rows without a source at every start
   ([events.go:300](../../internal/store/events.go)):
   only binaries before 0009 write such rows.
   The migration runs it once, the function goes.
3. **`cube_options`** (0018) is a table of its own only
   "so that a binary built for an earlier version reads `cubes` as before":
   it merges into `cubes.proxy_virtual`, and a change then raises `cubes.version`.

## 3. The other findings, with my reading

The raise does not need these; it is a good moment for some.

| # | What | Proposal | My reading |
|---|------|----------|------------|
| 4 | `commands_pending` is never used: both queries bind the states (`state IN (?, ?)`, [commands.go:314](../../internal/store/commands.go)), which a partial index cannot match; checked | write the states literally (code only) | take it: `DropUnfinished` scans a table that grows without `commands.keep_days` |
| 5 | `audit_user` is never used: the audit filters by name through the join | filter by id, index `(user_id)` partial | take it |
| 6 | `commands_cube_time`: nothing queries commands by Cube | drop | take it, unless a per-Cube command list is planned |
| 7 | `backups.body`, the ~11 KB blob, sits before four columns, so every list walks its overflow pages | rebuild with `body` last, or bodies in a table of their own | measure on the Pi 1 first; not in this migration |
| 8 | `tokens.deleted_at`, `commands.deleted_at` never read or written | drop | `commands.deleted_at` yes; `tokens.deleted_at` is decision 14's and a test requires it: keep |
| 9 | `cubes.updated_by`, `settings.updated_by` always NULL; `rooms.origin` never written | write them (code only) | take it, separately from the migration |
| 10 | `integrations.system_user_id` repeats the system identity | drop (rebuild of the small table 0022 rebuilds anyway) | take it; see section 1 |
| 11 | `user_identities` has a uuid and is edited but has no `updated_at`, `version`, `origin`, `deleted_at`, and is hard-deleted in four places | decide whether it replicates | D198 |
| 12 | `roles` replicates without a uuid; three different rules for a removed row's unique name | one rule; a uuid for roles (needs a hook) | D198 |
| 13 | `auth_providers.auto_domains` is comma-separated text beside JSON arrays | a JSON array (store only) | take it |
| 14 | `first_seen`, `request_expires`, `window_start`, `locked_until` beside `*_at`; `enabled` beside `disabled`; `log.device` beside `rf_address` | rename | only where a table is touched anyway: `integrations.request_expires` → `request_expires_at` |
| — | `user_identities.last_level` | CHECK `IN ('', 'view', 'control', 'admin')` | take it |

Checked and fine (the sub-agent's list, abridged):
the log's indexes are all used, `log_cube` included (id order within a Cube);
the partial uniques of `user_identities` and `rooms` are usable;
AUTOINCREMENT on `backups` and `commands` is intended;
no hard DELETE reaches cubes, users, tokens, roles, providers or integrations, so the CASCADEs never fire;
every closed enum but `last_level` has a CHECK;
`cubes.deleted_at` and `integrations.deleted_at` are decision 14's, kept.

## 4. The raise itself

- **The automatic rollback breaks.**
  After a failed health check, `massimo update` puts the previous binary back
  ([update.go:261](../../cmd/massimo/update.go)), and so does `massimo update -rollback`.
  Once the migration ran, that binary refuses the store
  ([migrate.go:150](../../internal/store/migrate.go): "needs a binary for version 22 or later")
  and the service stays down until someone puts `massimo.db.before-22` back by hand.
  `packaging/README.md` ("Rollback ... is safe", "none so far") must change in any case.
  The rollback runs in the older binary, so a fix has to ship a release before the raise.
  apt and brew never downgrade.
- **The copy** `massimo.db.before-22` (VACUUM INTO, 0600) is made for an existing file
  and never deleted: the store's full size on the SD card.
  Going back to it loses every log line, audit row, backup and setting since.
  The docs say when it may go.
- **The value** is 22: the integration changes and `cube_options` need it,
  and every later migration keeps at least 22.
- **Rebuilds** follow 0014 and 0021 (`_new`, copy, drop, rename, `-- foreign_keys: off`);
  never rename the old table aside (SQLite 3.26+ rewrites the children's `REFERENCES`);
  `log` is not rebuilt, only altered.
- **Files with it:** `store.sql` regenerated ("min_reader 22"),
  `docs/store.md` (section 5, "Migrations"), `packaging/README.md`'s rollback paragraph,
  a CHANGELOG bullet saying earlier releases cannot open a migrated store,
  and the tests that use the view and `fillSources`.

## 5. Decisions

1. **D189, the user columns' names:**
   - **C (recommended):** `effective_user_id` and `real_user_id` in `user_identities`,
     `log` and `audit`; the API's JSON names stay.
   - **A:** keep `user_id` and `own_user_id`.
   - **B:** `acting_user_id` and `made_user_id`.
2. **D190, `users.integration_id`:**
   - **A (recommended):** nullable, set for an integration's users; names stay unique over all users.
   - **C:** none.
3. **D191, the log's second user column:**
   - **E (recommended):** `real_user_id` in `log` and `audit`, written for every row with a user,
     with its index; old rows read through `COALESCE`, no backfill.
   - **F:** the same with the backfill, a write of the whole log in the migration.
4. **D192, an integration user's name:**
   - **F (recommended):** `<integrations.uuid>:<subject>`, own user `<uuid>:`, system user `<uuid>:system`,
     under a rule of their own (`:`, up to 100 characters).
   - **G:** `<integrations.id>:<subject>`: shorter, but a local row id, against decision 7.
5. **D193, the copy rule:**
   - **A (recommended):** copy a reported name only while the `display_name` still equals the one before.
6. **D194, the integration's names:**
   - **C (recommended):** the editable name is the own user's `display_name`,
     `integrations.name` the reported one, `key` and `system_user_id` dropped.
7. **D195, the raise:**
   - **A (recommended):** one migration, 0022, `min_reader: 22`, with section 1, section 2,
     and from section 3 the index of finding 5, the findings 6, 8 (`commands` only), 10, 13,
     `request_expires_at` and the CHECK on `last_level`;
     in a plan of its own, `StoreRaise`, which IntegrationUsers v1.1 builds on.
   - **B:** the same inside IntegrationUsers v1.1.
8. **D196, the rollback across the raise:**
   - **A (recommended):** a release first whose update helper, rolling back to a binary
     the store is too new for, puts `massimo.db.before-<N>` back
     (the newer file kept beside it, the loss logged); 0022 ships in a later release.
   - **B:** the helper refuses to roll back across a raise and says what to do.
   - **C:** accept it and document it.
9. **D197, the code-only fixes** (findings 4, 5 and 9):
   - **A (recommended):** a block of their own before the migration, since they need no raise.
10. **D198, the replicated tables** (findings 11 and 12):
    - **A (recommended):** not now: a short analysis of what replicates first,
      since decision 14's list leaves `user_identities` and `roles` unclear.
    - **B:** decide in this plan.

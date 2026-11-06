# NOTE StoreUsers v1.0: commands history

Decision D6, taken by the owner on 2026-11-06 during the run (`EXEC++ D6-A`),
on the proposal of a second table that finished commands move to after 24 hours.

- **One `commands` table**, no history table: the audit's foreign key,
  the command route and the sync of docs/server.md all read one place,
  and a copy-and-delete job has a failure mode of its own.
- **Retention is a setting**, `commands.keep_days` in `settings`,
  read by the prune that runs on every insert: 0, the default,
  keeps every finished command;
  a value removes finished rows older than that many days.
  It replaces the fixed 30 days of the plan's Analysis ("Commands (D3-B)")
  and is served by `GET`/`PUT /api/v1/settings` in Block 7.
- **The idempotency key is cleared after 24 hours** as planned; the row stays.
- **The index over the state is partial**, `WHERE state IN ('queued', 'sent')`,
  so that the pending rows are found at the cost of the pending rows whatever
  the table holds.
  It is in migration 0002, whose commit is amended since nothing is pushed.

The blocks stand as in v1.0; Block 7 reads the setting.

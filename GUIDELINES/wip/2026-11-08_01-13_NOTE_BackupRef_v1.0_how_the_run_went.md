# NOTE BackupRef v1.0: how the run went

The run of `PLAN++` on 2026-11-08: the plan registered on `main` on its own,
then Blocks 1 and 2 as signed commits on the branch, verified as the plan says.
Where it went differently:

1. **Block 2's test tells the new backup by its time, not its id.**
   The prune removes a row with `DELETE`,
   and `backups.id` is `INTEGER PRIMARY KEY` without `AUTOINCREMENT`,
   so SQLite gives the next row the id just freed:
   the backup taken at once after pruning the only one was `id=1` again
   in every run, and a check of a new id failed although the fix worked.
   The test compares `taken_at` instead.
2. **A finding, not fixed here:** an id of the API can name another backup
   once a prune removed the row with the highest id -
   with `backups.keep_newest` 0,
   or when the purge of tombstones older than `backups.keep_days_manual`
   removes a deleted newest.
   A list loaded before then deletes or downloads the new backup by the old id;
   a restore by id is refused where the plan differs,
   since the restore checks the plan's hash (read from the code, not tested).
   `AUTOINCREMENT` needs a rebuild of the table, a migration of its own;
   left to a plan of its own.
3. **Block 2 also rewords `docs/api-v1.md`**, "Backups":
   "deleted or pruned", the sentence Block 1 wrote, beside `docs/store.md`
   and the header of `internal/server/backups.go` that the plan names.
4. **`TestBackupDeleteNewest` checks one thing more** than Analysis 1 says:
   no backup follows the delete before `backups.auto_min`.

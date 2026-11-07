# NOTE StoreBackups v1.0: how the run went

The run of Blocks 1 to 5 under `PLAN++` on 2026-11-07,
where the work went differently from the plan, a line each.

- **The branches**:
  `claude/store-e` holds the plan and Block 1, `claude/store-e-2` Block 2,
  `claude/store-e-3` Block 3, `claude/store-e-4` Blocks 4 and 5 with this NOTE,
  each stacked on the one before;
  main (a3d0ca1) has not moved since the first branch, so nothing was merged;
  nothing is pushed.
- **Migration 0005** was not in the PLAN gate:
  the reasons `connect` and `daily` belonged to the daily backups the owner
  replaced, and SQLite cannot change a CHECK, so the empty table is made again;
  the plan's "Taken in the plan" says so.
- **`GET …/backups` answers an object**, `{backups, changed, due}`,
  not a bare list:
  the panel's line "a backup follows at …" needs when the configuration changed
  and when the backup is due.
- **A row's time is the store's**, to the millisecond, not the file's,
  which is to the second:
  a restore's `before_restore` and `upload` of one second came out in the wrong
  order (`TestRestoreKeepsBackups`).
- **Names in the store**:
  `Store.Backup` was taken by the online copy of the file,
  so a stored backup is `StoredBackup`; `Store.ReadOnly` is new,
  for the server's loop.
- **The minute of the windows is `Options.BackupMinute`**,
  not a package variable as the plan said:
  the tests' servers outlive their tests,
  and a variable changed by one would race with the next.
- **The hash leaves out the wall thermostat's display choice** as well:
  the file takes it from the live state,
  which comes after the configuration at a connection.
- **The diff of an edited device is one step** with its settings and its
  programme, as the planner sends them: the edited copy gives two steps,
  not three.
- **The page** gains `apiRaw` for a file's body and answer,
  and the stream's `restore_step` and `restore_done`,
  which no listener took before, so the log shows them live too;
  German says "zurückspielen" as its capability does.
- **The screenshots**:
  the settings shots and the slideshow taken again three times,
  for two style fixes seen in them (the rows' buttons at the left,
  room under the list); the appearance shots,
  which came out shifted by a pixel only, left as they were.
- **The UI suite** skipped the eight tests that need the discovery port in one
  full run, while another program held UDP 23272; on their own,
  `test_cubes.py` and `test_discover.py` passed, 24 of 24.

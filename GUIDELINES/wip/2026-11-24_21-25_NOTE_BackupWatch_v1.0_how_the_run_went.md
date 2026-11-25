# NOTE BackupWatch v1.0: how the run went

Plan: `wip/2026-11-24_19-47_AP_BackupWatch_backups_after_restarts.md`.
Run under the owner's `PLAN++ as recommended` of 2026-11-24,
and `EXEC D6-A` the same day for Block 3.

## The commits

| Commit | What |
|--------|------|
| `fix(server.backups): A change's automatic backup keeps its time across a restart` | Block 1 |
| `fix(ui.backups): Backup and restore shows each Cube's list after a load onto its page` | Block 2 |
| `feat(server.backups): The backups of a Cube no longer served can be listed and downloaded` | Block 3, with D6-A |

## A decision: D6-A

The plan's Analysis said that `GET /api/v1/cubes` lists every row.
It does not: it lists a found Cube only while discovery hears it
(`unconfigured()` keeps the `foundPresent` ones).
D5-A had been chosen on that premise, so a Cube that was replaced and unplugged,
the main case of D4-B, would have got no fold.
The run stopped at Block 3 with D6:

- **A (taken).** A route of its own, `GET /api/v1/backups/unserved`
  (capability `backup`), lists the Cube rows that are not served, not removed and
  keep backups, with the serial, the name, the count and the newest backup's time.
  The not-served folds read it, not `state.found`.
  It stands outside `/api/v1/cubes/…`, where `{sn}` would shadow it.
- **B.** Such rows in `GET /api/v1/cubes` with a marker, every reader of the found
  Cubes filtering them out.
- **C.** Folds only for found Cubes that discovery hears.

D5-A's `backups` count on the Cube views stays, as built.

## Where the run went differently from the plan

1. **Block 1, a `ref` column in `backup_pending`.**
   The table keeps the hash of the reference the windows count against.
   A start whose newest backup has another hash takes the row as left over
   and deletes it, so that a row a binary for schema 25 left behind,
   or a restore by id, cannot make a backup due at once.
2. **Block 1, the stop.**
   - A Cube without any backup is skipped at the stop:
     its first backup is due at the first look after the next start anyway.
   - The looks take a semaphore, so that the stop gives up on a look under way
     once its 5 s are out; a backup within a deadline reads the NTP servers in at
     most half the time left.
   - `docs/update-windows.md` names the backup in the service's stop.
   - Not tested: a Cube not connected at the stop keeps its row.
     The client needs about 20 s to notice a silent fake Cube; the path only logs.
3. **Block 2, the load test** also covers a reload onto Cube settings with a Cube
   stored, where a second read would happen; it checks that one read is sent.
   A box drawn while its page is hidden reads its list when the page is shown.
4. **Block 3.**
   - The card head's count includes the backups of the Cubes not served.
   - `store.BackupCounts` is keyed by serial, since found Cubes are known by serial.
   - `TestDiscoveryFindsUnconfiguredCubes` lists the fields of a found Cube;
     it now allows `backups`.
   - A fold's head shows its row's name.
5. **Local verification limited to what was touched.**
   Per block: `go test` of the packages touched (`internal/store`, `internal/server`,
   `internal/serve`), `gofmt`, the three `go vet`, `make ui`, `make ui-check`,
   `test_backup.py` in its four cases (24 passed), `changelog.sh check`,
   `reflow.sh --check` and `commit-msg.sh`;
   `GUIDELINES/shared/tools/check` over the branch.
   The whole Go suite, `make race`, `make cross` and the whole UI matrix are left to CI.

## Left to the owner

Checking on breno that a change's backup keeps its time across an update,
and that a stop takes a pending one.

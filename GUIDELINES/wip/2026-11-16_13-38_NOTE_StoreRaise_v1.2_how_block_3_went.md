# NOTE StoreRaise v1.2: how block 3 went

Block 3, run by a sub-agent on the owner's `EXEC block 3`, 2026-11-16:
#335, not merged. Testing locally covered only the touched packages; the
whole suite, race, cross, Windows and armv6 were left to CI, at the
owner's word during the run.

## Where the run went differently

1. **Numbers:** the plan's 0023, "version 21" and "a binary for 22" are
   0024 (`-- min_reader: 24`), version 23 and a binary for 23.
2. **The losses** are listed once, L1 to L8, in the header of
   `0024_raise.down.sql`; `losses0024` in `raise_test.go` and
   `docs/store.md` cite them by number.
3. **D14-A's test** (`TestReleasedStoresDownAgain`) migrates each dump to
   the stop once, copies the file and then goes up to 24: migrating twice
   writes different times (0021's `strftime('now')`).
4. **L7, one serial with several rows:** the extra row's log, backups and
   rooms are deleted; its commands and audit rows are kept with `cube_id`
   NULL, as those keys have no ON DELETE action. For the owner to confirm.
5. **Until Block 4** the API's integration `key` fields (the integration's
   view, the users' integration marks and links, `/me`) carry the
   integration's uuid, and the page shows the uuid where it showed the
   key. For the owner to confirm.
6. **A finding:** nothing tombstones a Cube today, since DELETE keeps the
   row as found. D11-A's restore is reachable only through `AddCube` with
   `UUID` set; Block 5 adds its API.
7. **The audit's backfill** reads `detail` in the SET and in a JOIN, never
   in a WHERE, to satisfy the content-column rule of `rule_test.go`.
8. **Not run:** the v23 binary's store tests on a downgraded file. The
   byte-equal `sqlite_master` and row comparisons stand in for it. For the
   owner to confirm.
9. **The server** also changed: the audit and the change events carry the
   real user of a call made through an integration, `via` names the
   integration's uuid, and `proxy_virtual` is read from and written to the
   Cube's row.

## Measured

- 500 000 log rows (149 MB, a tenth with a user), a 16-core desktop at a
  load of 3 to 7: the raise 1.96 s with its `massimo.db.before-24` copy,
  the down part 2.19 s with its copy; filling the store took 5.3 s.
  `MASSIMO_STORE_RAISE_ROWS=500000 go test -run TestRaiseTimes
  ./internal/store`. Not measured on a Raspberry Pi.
- `TestReleasedStoresDownAgain`: all 11 release dumps.
- Locally: `go test` of `internal/store`, `internal/update`,
  `cmd/massimo` and `internal/server`; `gofmt`; `go vet` on three
  platforms; `GUIDELINES/shared/tools/check`, all 12.
- CI on #335: every job passed, Race, armv6, Windows service, macOS
  LaunchDaemon and LAN included; no flake seen.

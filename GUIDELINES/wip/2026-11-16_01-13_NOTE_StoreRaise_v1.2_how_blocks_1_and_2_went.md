# NOTE StoreRaise v1.2: how blocks 1 and 2 went

Blocks 1 and 2, run by a sub-agent on the owner's `EXEC AP StoreRaise v1.2`,
2026-11-15/16: #321 (Block 1) and #323 (Block 2), not merged.

## Where the run went differently

1. **Numbers:** `0022` was taken on main (`0022_repair_capability.sql`).
   Block 1's additive migration is `0023_down_parts.sql`; Block 3's raise
   becomes **0024**.
2. **Where a downgrade stops:** before the first migration whose
   `min_reader` exceeds the target binary's version, not at "the one after
   N": the additive migrations in between need no down part, and that
   binary reads them.
3. **Down parts from the first raise on:** a later additive migration is
   built on the raised schema, so every migration with `min_reader` above
   1 needs a down part (`TestRaisingMigrationsHaveDownParts`), not only the
   raising one. A rule for the owner to confirm.
4. **Root never opens the store:** the update helper runs as root, and the
   store's `down_sql` is writable by the service's user. The rollback asks
   the binary it put back whether it opens the store (`store info`), and
   runs the downgrade in the set-aside binary as the owner of `massimo.db`.
   The store's own down parts run in the binary put back, after a failed
   health check only; `-rollback` names the command instead.
5. **The service is stopped first:** systemd `stop`, and new
   `install.StopService` (Windows) and `install.StopLaunchd` (macOS);
   `RestartLaunchd` also starts a job not loaded. macOS not tried on a Mac.
6. **`Pending`** needed a unary `+` on `parent_id IS NULL` as well as the
   literal states, or the planner kept `commands_parent`.
7. **`SetSettingBy`, `DeleteSettingBy`** are new; `SetSetting` and
   `DeleteSetting` keep their signatures and mean the server's own write.
   `UpdateCube` gains `by`. `AddCube` has none (Block 5 reworks it), and
   `TouchToken` writes no `origin`, since it does not bump `version`.
8. **armv6:** CI's first run of #321 failed under QEMU, where a copy of the
   test binary cannot be started; those tests skip with `-short`.

## Measured

- Block 1: `go test ./...` 1102 tests; `-race` over RACEPKGS; Windows test
  binaries through interop (`internal/update`, `cmd/massimo` pass;
  `internal/store`'s `TestMigration12` leaves a store open and fails the
  temp-dir cleanup on Windows, untouched here); as root in Docker, nothing
  in the data directory ends up root's.
- Block 2: `TestWallReset` is flaky under `-race` on main too (10 of 20
  runs there, 1 of 20 on the branch).

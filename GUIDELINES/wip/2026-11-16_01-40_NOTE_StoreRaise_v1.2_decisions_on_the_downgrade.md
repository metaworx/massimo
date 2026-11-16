# NOTE StoreRaise v1.2: decisions on the downgrade

The owner, 2026-11-16, on the gate after Blocks 1 and 2
(NOTE StoreRaise v1.2, "how blocks 1 and 2 went"):

- **D12-A:** every migration from the first raise on has a down part, the
  additive ones included, as `TestRaisingMigrationsHaveDownParts` holds.
- **D13-A:** root never opens the store: the rollback runs the downgrade in
  the set-aside binary as the owner of `massimo.db`, and the store's own
  down parts only in the binary put back, after a failed health check.
- **D14-A (the owner's question, "should the test also compare each step
  with the dumped releases?"):** yes, in Block 3, where 0024 brings the
  first down part that changes the schema. For each dump in
  `internal/store/testdata/releases/`: the dump migrated up to the newest
  version and downgraded for the dump's own release must equal the dump
  migrated only as far as that downgrade stops, `sqlite_master` and the
  rows alike, but for what the note on 0024's losses names. Until 0024 no
  dump lies above a raise, so the test would compare nothing yet.

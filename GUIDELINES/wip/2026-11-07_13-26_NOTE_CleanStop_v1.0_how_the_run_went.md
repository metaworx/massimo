# NOTE CleanStop v1.0: how the run went

How the blocks of [AP CleanStop v1.0](2026-11-07_13-12_AP_CleanStop_store_closed_at_every_stop.md)
went where they differ from the plan, under one `PLAN++` on 2026-11-07.

1. **`Checkpoint`'s result.**
   `PRAGMA wal_checkpoint(TRUNCATE)` answers `0|0|0` once it has copied and
   emptied the WAL, so the plan's `Frames` and `Done` would always be 0.
   `Checkpoint` reports `WALBytes`, the `-wal` file's size before it,
   and `Busy`; `Close` turns a busy checkpoint into an error.
   The session of AP Restart was told both shapes, the second as a correction.
2. **The documents went with Block 2.**
   `docs/store.md` section 3 and the README's line on `store backup`
   changed in the commit whose CHANGELOG bullet cites them,
   rather than in Block 4, which kept the pull request alone.
3. **Block 3 found the gap on Windows too.**
   The Windows test, built from origin/main's code,
   has the request in flight answered `500`:
   the store closed under it while the process lived on.
   On this branch it is answered `200`, and the WAL is gone.
   The Windows run left the live service's `service.log` untouched.
4. **SIGHUP stays with AP Restart.**
   No signal handling here:
   that plan's `-sighup auto|reload|stop` owns SIGHUP,
   and its reload is to call `Store.Checkpoint`.
   Its session was sent the function, the log line,
   and the `serveHTTP` hunk and CHANGELOG bullet both branches carry
   word for word, so that they merge cleanly in either order.

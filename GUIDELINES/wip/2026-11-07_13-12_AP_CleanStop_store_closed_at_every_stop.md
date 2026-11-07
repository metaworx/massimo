# AP CleanStop v1.0: store closed at every stop

## TL;DR

Every stop of `serve` brings the WAL into `massimo.db`.
Requests still running at the stop get their answer and their audit row,
and a close that fails says so.
A checkpoint is ready for the SIGHUP reload of AP Restart.

- stale `massimo.db-wal` on the Pi and jaguar → left by v0.7 to v0.9,
  fixed in v0.10.0-alpha.1 (SIGTERM, the Windows stop); no data lost
- `serveHTTP` returned before Shutdown → waits up to 5 s (AP Restart's hunk, verbatim);
  event streams end with the run → a UI left open costs the stop nothing
- `Store.Close` = `db.Close()`, error dropped →
  `Checkpoint` (TRUNCATE), then close, logged; the 10 s wait warns when over
- SIGHUP → AP Restart's `-sighup` owns it; its reload calls `Store.Checkpoint`
- Windows → the service stop tested with the store, through interop
- update's unfinished command → old binaries; fixed in v0.10 (D2-A: rows left)

### Blocks

1. The stop answers the requests in flight; event streams end with the run
2. `Store.Checkpoint`, a checkpointing `Close`, and a reported close in `serve`
3. Windows: the service stop with the store, tested through interop
4. Documents, the pull request

## Discussion

The owner answered the TL;DR gate on 2026-11-07 with `PLAN++`,
and asked for SIGHUP to commit the WAL into the file too,
prepared here or done directly in coordination with the session of
AP Restart, which adds `-sighup`.

- **D1-A.** `Close` runs a TRUNCATE checkpoint before it closes,
  so `massimo.db` alone is current after every clean stop,
  even while another program holds the file open.
- **D2-A.** The update rows already dropped as "server restarted" on the Pi
  and jaguar stay as they are: the binaries before v0.10 wrote them,
  and v0.10.0-alpha.1 acknowledges the update's `202`.
- **SIGHUP, prepared.**
  AP Restart (D8 there) gives SIGHUP to a settings reload,
  `-sighup auto|reload|stop`; a second handler here would compete for the
  signal and change what a SIGHUP in a terminal does.
  This plan adds `Store.Checkpoint` instead, which `Close` uses,
  and that session's reload calls it.
  The session was told on 2026-11-07, with the function's shape and the log
  line to write.
- **`serveHTTP`, shared.**
  AP Restart's block 2 already made `serveHTTP` wait for its Shutdown.
  This branch takes that hunk verbatim, comment included,
  so that the two branches merge cleanly in either order;
  that session was told not to reword it.

## Analysis

What the investigation found (the TL;DR gate of 2026-11-07):

- **The production files.**
  The Pi's two stops were made by v0.7.0-alpha.1 and v0.8.0-alpha.1,
  which had no handler for SIGTERM: a build of each, stopped with SIGTERM,
  exits with 143 and leaves `massimo.db` at 4096 bytes and the WAL in place.
  jaguar's service was stopped by binaries whose stop ended the process
  before `serve` returned.
  Both were fixed in v0.10.0-alpha.1 ("stop on SIGTERM as on SIGINT",
  "wait for serve before reporting the stop").
  The WAL holds the data, and the next start reads it.
- **No stale WAL in the current code.**
  About 20 stops with the fake Cube, on origin/main, on the commit the owner
  tried and on v0.10.0-alpha.1, after 0.3 s to 75 s, with an event stream
  open, with the fake Cube gone first, and with `sqlite3`'s checkpoint
  beforehand: every one removed the WAL.
  325 KB is that commit's WAL while it runs:
  the owner's listing probably ran before the exit.
  `TestServeStopsOnSIGTERM` checks the same.
- **The last events.**
  A disconnect 10 to 80 ms before SIGTERM is in the store after the exit,
  in five runs out of five: the writer drains its queue at the end of the run.
- **The update's command row.**
  Before v0.10.0-alpha.1 every `202` left its command `sent`,
  and only a restore's run finished one; the next start dropped it.
  v0.10.0-alpha.1 acknowledges a `202` that no run reports back to.
- **What is still wrong.**
  - `http.Server.Serve` returns as soon as Shutdown starts, and `runServe`
    did not wait for Shutdown: a request still being answered at the stop
    was cut by the exit, its audit and command rows unwritten,
    and it could still hold the store's one connection when `Close` ran,
    which then leaves the WAL.
    AP Restart's block 2 fixes the wait; the event stream ends only on its
    request's context, so with the wait alone a stop with the web UI open
    takes the whole 5 s grace.
  - `defer st.Close()` drops the error, and `Close` checkpoints only where
    SQLite's last close can: another connection open to the file
    (`massimo config`, `sqlite3`, a backup tool) skips it,
    and `massimo.db` stays behind.
  - When the server's run takes longer than the 10 s `runServe` waits,
    the store closes under it, silently.

## Implementation Plan

### Block 1: The stop answers the requests in flight

1. `cmd/massimo/servetls.go`: `serveHTTP` waits for its Shutdown goroutine
   before it returns - AP Restart's hunk, verbatim.
2. `internal/server/server.go`, `events`: the stream also ends when the
   server's run context ends.
3. `cmd/massimo/serve_stop_test.go`, the built binary as
   `TestServeStopsOnSIGTERM` builds it:
   - a request whose body is still arriving when SIGTERM is sent gets its
     answer once the rest arrives, and its audit row is in the store after
     the exit; it fails before step 1;
   - with an event stream open on the fake Cube, the stop takes well under
     the 5 s grace, and the stream ends cleanly; it fails with step 1
     alone.
4. `CHANGELOG.md`, `### Fixed`.

**Verification:** the two tests fail before and pass after;
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `-race` for `internal/server`.

### Block 2: A checkpointing close, reported

1. `internal/store/store.go`: `Checkpoint(ctx) (Checkpoint, error)`,
   `PRAGMA wal_checkpoint(TRUNCATE)`, its three numbers as
   `Checkpoint{Busy bool; Frames, Done int}`; nothing on a read-only store.
   `Close` checkpoints, then closes, and returns either error.
2. `cmd/massimo/serve.go`: the store's close is logged, `store closed`
   with what the checkpoint moved, or the error at Error;
   a run that did not end within the 10 s is warned of before the close.
3. Tests:
   - `internal/store`: with a second connection open, a copy of
     `massimo.db` alone after `Close` holds what was written;
     it fails before step 1 (the copy has no tables);
   - `Checkpoint` while the store is open moves every frame,
     and the copy holds the rows;
   - `TestServeStopsOnSIGTERM` finds `store closed` in the log.
4. `CHANGELOG.md`, `### Fixed`.

**Verification:** the store test fails before and passes after;
the suite as in Block 1.

### Block 3: Windows, the service stop with the store

1. `cmd/massimo/serve_stop_windows_test.go`: `runServe` in the test's
   process with a fake Cube, a data directory and `ProgramData` of its own;
   the service manager's stop through `theService`;
   then `runServe` returned nil, the WAL is gone, the disconnect before the
   stop is in the store, and a request in flight at the stop was answered.
2. Run on jaguar through interop: the test binary cross-compiled,
   `ProgramData` a temporary directory, never the live service's.
3. `CHANGELOG.md`: none if the test only confirms; a bullet under the
   fix it finds otherwise.

**Verification:** the test passes on Windows; `make cross`,
`GOOS=windows go vet ./...`.

### Block 4: Documents, the pull request

1. `docs/store.md`: the checkpoint at every stop and through
   `Store.Checkpoint`; the WAL left by a process that is killed;
   `massimo store backup` stays the way to copy a running store.
2. `README.md` where it says how to copy the store, if it needs a line.
3. The branch merges origin/main, is pushed, and its pull request opened;
   the session of AP Restart gets its number.

**Verification:** `gofmt -l .`, `go vet` for the three platforms,
`changelog.sh check`, the reflow check of the changed documents.

## Proposed commit messages

- `chore(aps): +CleanStop v1.0`
- `fix(serve): Answer the requests in flight before the store closes`
- `fix(store): Checkpoint the WAL at every close, and report the close`
- `test(service.windows): The service stop closes the store`
- `docs(store): The checkpoint at every stop`

## Change History

- v1.0 (2026-11-07): the first revision, from the TL;DR gate;
  D1-A and D2-A as recommended, SIGHUP left to AP Restart with
  `Store.Checkpoint` prepared for it.

# AP SendTurn v1.0: Send waits its turn

## TL;DR

`Client.Send` waits for the command in flight, bounded by the caller's
context, instead of failing with `ErrBusy`; only the poll still gives up
at once.

- `Send`: `c.pending != nil` → `ErrBusy` becomes **a one-slot turn**,
  taken with `select` on the turn or `ctx.Done()`; the connection is
  checked once the turn is held; `ReplyTimeout` counts from the write
- poll (`session`): `Send` → unexported `trySend`, which keeps `ErrBusy`;
  a busy line counts no miss, and the session loop never waits for the turn
- gains: every queue command (setpoints, programmes, `c:` re-reads), the
  backup's `f:` read (`server.go`), `massimo backup` → wait, no 502
- a dropped connection still ends a waiting `Send` with `ErrNotConnected`
- no timeout parameter (D2-A): the context is the bound

### Blocks

1. The turn in `Send`, `trySend` for the poll, the tests, the CHANGELOG fragment

## Discussion

Seen on 2026-11-13 in `internal/server` tests with a 50 ms poll:
`command failed … error="cubelan: another command is waiting for its reply"`.
A queue command sent while the poll's `l:` waits for its reply fails with
`ErrBusy`, and the API answers 502, with no retry. In production the window
is a poll's round trip once a minute, but a user's setpoint or programme
write can fail for no reason of its own.

**The owner's decisions** (2026-11-13):

- **D1-A:** the wait goes in `Send`: one place, and every caller gains, the
  backup's `f:` read and the command line's included. Not B, the queue
  retrying `ErrBusy` after a pause: queue commands only, and a retry loop
  with timing of its own.
- **D2-A:** no timeout parameter on `Send`. The context bounds the wait
  (`context.WithTimeout` for at most *d*, none for as long as the line is
  up); "return at once" is the poll's alone, as `trySend`. Not B,
  `Send(ctx, m, wait *time.Duration)`: two limits on one call, a pointer
  for `nil`, and `engine.Gateway`, its fakes and every caller changed.

## Analysis

**Who calls `Send`.** The poll loop (`l:`, `client.go` `session`); the
engine's queue (`internal/engine/queue.go`: `z:`, `l:`-type lines,
`SendRadio` for `s:`/`r:`), which runs one command at a time, so the
follow-up's `c:` re-reads (`cubeio.Writer.Reread`, through the queue) never
meet a queue command; `getBackup`'s `f:` (`internal/server/server.go`);
`massimo backup` (`cmd/massimo/backup.go`). A queue command can therefore
meet only the poll and the two direct callers.

**The turn.** A `chan struct{}` of capacity 1 in the client, made in `New`.
`send(ctx, m, wait)`:

1. a command with no reply is written as now, without the turn;
2. not connected → `ErrNotConnected` at once, before any wait;
3. the turn is taken without blocking; failing that, `trySend` returns
   `ErrBusy`, `Send` waits for it or for `ctx.Done()` (→ `ctx.Err()`);
4. held, it is given back when `send` returns; the connection is checked
   again (it may have dropped during the wait), then `pending` is set and
   the line written, as now.

While the turn is held `pending` is the holder's or nil: only the holder
sets it, and the reply (`handle`), `clearPending` and the session's end
clear it. The `pending != nil` check therefore goes.

**What bounds a wait.** The command ahead holds the turn at most
`ReplyTimeout` (10 s by default) after its write, or until its context
ends or the connection drops, which closes its reply channel; the waiter's
own context bounds the rest. Waiters are not served in order; the queue
sends one at a time, so there is at most one queue command, the backup's
`f:` and a command line's waiting together.

**The poll.** It keeps `trySend`: an in-flight command proves the line as
well as the poll would (the comment at the `ErrBusy` case says so), and a
waiting poll would hold up the session loop's reaction to `ctx.Done()`
and a read error. Nothing else changes in the missed-poll count.

**Not in scope.** A reply that comes after its `Send` gave up (reply
timeout, context) can still answer the next command of the same reply
type; that was so before and is unchanged.

## Implementation Plan

### Block 1: The turn

1. `internal/cubelan/client.go`: the `turn` field and its comment; `New`
   makes it; `Send` → `send(ctx, m, true)` with the steps of the Analysis;
   `trySend` → `send(ctx, m, false)`; the poll calls `trySend`; `ErrBusy`'s
   and `Send`'s comments say who gets it and that `Send` waits.
2. Tests, `internal/cubelan/client_test.go`, the fake Cube silent
   (`SetSilent`) to hold a reply:
   - a second `Send` waits for the first, is not written while it waits,
     and both are answered once the Cube speaks;
   - a waiting `Send` ends with its context (`context.DeadlineExceeded`),
     its line never written;
   - a waiting `Send` ends with `ErrNotConnected` when the connection
     drops (`Drop`, a long `Backoff`);
   - a poll that finds the line busy sends nothing and counts no miss: the
     connection stays up past two poll intervals of a held command.
3. Test, `internal/engine`: a real client with a 50 ms poll and the queue;
   the fake Cube silent, a poll's `l:` held; a queue command enqueued then
   waits and succeeds once the Cube speaks (before: `ErrBusy` at once).
4. `CHANGELOG.d/Fixed/claude-sendturn.md`.

**Verification:** `gofmt -l .` empty; `go vet ./...` for linux, windows,
darwin; `MASSIMO_ENV_FILE=/dev/null go test ./...`; `make race`, the new
tests repeated with `-race -count=20`; `make cross`; the engine test
fails with `ErrBusy` against the old `Send`.

## Proposed commit message

    fix(cubelan): A command waits for the one in flight instead of failing busy

    Client.Send waited for no one: a command sent while another waited for its reply failed with ErrBusy, which reached the API as a 502. The poll's l: once a minute was enough to make a user's setpoint or programme write fail.

    Key changes:
    - Send takes a one-slot turn, waiting for it as long as its context allows; the reply timeout counts from the write
    - the poll uses trySend, which still returns ErrBusy at once, so a busy line counts no missed poll
    - tests: a held poll reply overlapping a queue command, a waiting Send ending with its context and with a dropped connection, a poll skipping a busy line

    Assisted-by: claude-code:claude-opus-5-5

## Change History

- v1.0 (2026-11-13): the first revision, by PLAN++; D1-A, D2-A.

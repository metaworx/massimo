# AP CubeLost v1.0: lost Cube logged once

## TL;DR

A Cube that goes away is reported once, and its return once,
instead of a WARN line, a stored `disconnected` event and a reload of every
open page at each retry of the backoff.

- loss → one Warn `cube connection lost` + one `disconnected` event:
  a greeted connection ends, or the first attempt fails after the start
  or after a deliberate `Reconnect` (D1-A, D5-A)
- each retry → Debug `cube connection attempt failed`, `error`,
  `retry_in` (D3-A)
- down a day → Warn `cube still unreachable`, `down_for`, no event (D2-A)
- greeting after a loss → Info `cube connection re-established`,
  `down_for` (D4-A); any other greeting → `cube connected`, as now
- `EventConnected`, a deliberate reconnect that works → unchanged
- `Options.Backoff` (1 s), `Options.RemindEvery` (24 h) → set short by the tests
- `docs/api-v1.md`, the OpenAPI kinds, `docs/ui.md` Log; CHANGELOG Fixed

### Blocks

1. One loss, one return: `Run`, its tests, the documents, the CHANGELOG
2. Merge origin/main, push, open the pull request

## Discussion

The owner answered the TL;DR gate on 2026-11-07 with `PLAN++` as recommended:

- **D1-A.**
  The Cube is up once a session got its greeting, the client's `online` state.
  A Cube that takes the TCP connection and drops it before the greeting stays
  one outage.
- **D2-A.**
  While the Cube stays unreachable,
  a Warn `cube still unreachable` with `down_for` every 24 hours, and no event:
  a long outage stays visible after the log rotates.
- **D3-A.**
  Each failed attempt is logged at Debug, with the error and `retry_in`.
  `serve` logs at Info and has no level switch (`cmd/massimo/serve.go`),
  so the lines show only once one exists; that is a change of its own.
- **D4-A.**
  The down time is in the `re-established` log line only;
  `EventConnected` and the API stay as they are.
- **D5-A.**
  A deliberate `Reconnect` whose next dial fails counts as a loss,
  logged and emitted once: the Cube is gone, whoever closed the connection.
  It falls out of the same rule, with no state of its own.

A failure at the start is logged as `cube connection lost`, as the owner asked,
and its end as `cube connection re-established`, so that the two always pair.

## Analysis

`Run` (`internal/cubelan/client.go`) treats every session that ends the same:
a dial that fails,
a session closed before its greeting and a connection that was up for days each
emit `EventDisconnected` and log `cube connection lost` at Warn,
with the error and the backoff.
The backoff starts at 1 s and doubles while it is under a minute,
so it settles at 1m4s, and a Cube that stays down costs, each minute:

- a WARN line in the service log;
- a `disconnected` event in the store: it is a stored kind (`eventKinds`,
  `internal/server/log.go`), through `runtime.remember`;
- a `loadCubes()` on every open page (`internal/server/ui/app.js`,
  its `disconnected` listener).

The consumers of `EventDisconnected`:

- `internal/server/server.go`, `pump`: forwards it to the stream and the log;
  nothing else reacts to it.
- `cmd/massimo/watch.go`:
  prints a line per event, now one per loss.
- `internal/cubelan/client_test.go`:
  `TestClientReconnects` and `TestConnectedFollowsTheConnection` wait for one
  after a greeted connection ends, which stays a loss;
  `TestReconnectRefreshesTheSnapshot` checks that a deliberate reconnect emits
  none, which stays so.
- `cmd/massimo/serve_stop_windows_test.go`:
  finds one in the store after the fake Cube closes a greeted connection,
  which stays a loss.
- `tools/ui-test/test_stream.py`:
  a `disconnected` event sent by hand leaves no rejected promise;
  the UI does not change.

`TestSetAddressCutsTheBackoffShort` relies on the 1 s first wait,
which stays the default.

**The state.**
`Run` keeps when the Cube was lost and when it was last warned of,
both zero while it is up.
It hands the time of the loss to `session`, whose reader logs
`re-established` with `down_for` at the greeting when there was one,
and `cube connected` otherwise;
a session that got its greeting clears it.
No field is shared between the goroutines.

## Implementation Plan

### Block 1: One loss, one return

1. `internal/cubelan/client.go`:
   - `Options.Backoff`, the wait after the first failure, 0 for 1 s;
     `Options.RemindEvery`, 0 for 24 h; both for the tests,
     the defaults set in `New`.
   - `Run`:
     after a session, with the deliberate reconnect handled first as now -
     - no loss yet → `EventDisconnected` and the Warn `cube connection lost`
       (`addr`, `error`, `retry_in`), the loss's time kept;
     - a loss, last warned at least `RemindEvery` ago → Warn
       `cube still unreachable` (`addr`, `down_for`, `error`);
     - otherwise → Debug `cube connection attempt failed` (`addr`, `error`,
       `retry_in`).
   - `session(ctx, lost)`:
     at the greeting, Info `cube connection re-established` (`addr`, `down_for`,
     rounded to the second) after a loss, `cube connected` otherwise.
   - The doc comments of `Run` and `EventDisconnected` (`snapshot.go`) say what
     a loss is.
2. `internal/cubelan/client_test.go`, with the log captured at Debug and the
   events counted, against the fake Cube:
   - a fake Cube closed, failing several retries, restarted at the same address,
     then closed again: two `cube connection lost`, two `EventDisconnected`,
     one `re-established` with `down_for`, one `cube connected`,
     and the retries at Debug;
   - a closed port from the start: several retries, one loss, one event,
     no reminder; then a fake Cube there: one `re-established`;
   - `RemindEvery` short: reminders while down, one event still;
   - a deliberate `Reconnect` whose dial then fails: one loss and one event,
     then `re-established` once the dial works.
3. Documents:
   - `docs/api-v1.md`, the events: `disconnected` once per loss,
     not at each retry, and what a loss is;
   - `docs/api-v1-openapi.yaml`, the stream's kinds: the same, in short;
   - `docs/ui.md`, Log: a lost connection once,
     however often the server retries.
4. `CHANGELOG.md`, `### Fixed`, in the wrapped style of its neighbours.

**Verification:**
the new tests fail before step 1 and pass after;
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `-race` for `internal/cubelan` and
`internal/server`; `make cross`; `gofmt -l .` prints nothing;
`go vet ./...` for linux, windows and darwin;
`GOOS=windows go test -c ./cmd/massimo` compiles the Windows stop test,
which is not run (it is Windows-only, and the owner's service is not touched);
`changelog.sh check`.
No `make ui-test`: the UI does not change.

### Block 2: The pull request

1. The branch merges origin/main (a merge, never a rebase),
   the suite of Block 1 runs again where the merge brought changes.
2. The branch is pushed and its pull request opened against `main`;
   no merge, no auto-merge.

**Verification:**
the pull request lists the plan's commit and Block 1's;
its CI is reported at the completion gate.

## Proposed commit messages

- `chore(aps): +CubeLost v1.0`
- `fix(cubelan): Log a lost Cube connection once, not at every retry`

## Change History

- v1.0 (2026-11-07): the first revision, from the TL;DR gate;
  D1-A to D5-A as recommended.

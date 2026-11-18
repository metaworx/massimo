# NOTE TestInfrastructure v1.1: how block 3 went

Where Block 3, Watcher, went differently from the plan,
what the watcher gained on the way, what was left and for which block,
and what verified it.

## What it made

- **The watcher, `testkit.Watch`:**
  a mark; a wait from it for one match, for several in any order,
  or for the n-th, skipping the rest; a check once the action is over;
  `Never` with an end; the end of a feed and its reason.
  On a timeout it says what came since the mark.
- **Its feeders:** `LogWatch`, a slog handler, and `ReadLog` for a child's
  output; `StreamWatch` for an event stream; `ChanWatch` for a channel;
  `cubewatch.Lines` for the fake Cube's lines (D18-A).
- **The fake Cube's `OnReceive`** (D7-A):
  a call at each line received, after the earlier ones and before its answer.
- **Every package's tests moved onto it:**
  the log buffers, the event-stream readers, the fake Cube's line counters,
  the Cube client's `waitEvent`, the queue's `waitFor`
  and `builtServe.waitFor` (D16-A) are gone.
- **`docs/testing.md`:** the watcher, its feeders and its rules.

## Different from the plan

1. **Who did it.**
   A builder on Opus wrote the watcher;
   three builders then moved the tests at the same time, on disjoint paths
   (`internal/server`; the Cube client, the queue, the proxy, `serve` and the
   Cube I/O; the command line and the other packages);
   a fourth added what the three had missed and moved their workarounds onto it;
   one second reader read the whole block;
   the session made its small fixes and the commits from recorded trees.
2. **Four commits, one pull request,** as Block 2:
   the watcher, then the server's tests, the Cube client's, the others'.
3. **No forward cursor in the log's API:**
   `before=` pages back by id, `since=` takes a time cut to the millisecond,
   and the log held in memory has no ids.
   So the waits that read the log's API for one new line
   (about 45, and `waitOutcome`, `waitUnconfirmed`, `waitWatch`)
   stay on `testkit.Eventually`.
   A forward cursor would change the product: it is left to the owner.
4. **No `ClientWatch`:**
   only the Cube client's own tests read its events,
   and a feeder that imported the client could not be used by them;
   `testkit.ChanWatch` serves them.
5. **What the watcher gained beyond the sketch,**
   from the builders' call sites:
   `Await` returns the cursor after its item, for the next one;
   `AwaitN`, `Has` and `HasNot`; `End`, `Ended` and `Err`;
   `Contains` on a record's line.
   A record's attributes are strings, read from slog's text handler,
   the same for a record in the process and a line of a child.
6. **The "nothing came" sleeps** (NOTE v1.1, item 5):
   seven sleeps or selects became a `Never` or a wait,
   each with an end that comes after what it guards:
   a later event or line, a test event, or a probe command queued behind
   (`engine`'s moved queue, `cubeio`'s pairing);
   so did a drain of the client's events.
   The 12 of `internal/server`, 7 of the Cube client's packages
   and 3 of the command line stay:
   each waits on a timer's absence, which no item shows,
   and Block 4's barrier does not end them either.
   The Cube client's say so in a comment.
7. **Two tests no longer call `Wait` while the child's pipe is read,**
   which `os/exec` forbids (`reload_test.go`, `restart_test.go`).

## Seen, left for later blocks

For Block 8 (Holes):

- `certs/acme_test.go:124`: "Watch checks at once" checks nothing after `Watch`.
- `certs/selfsigned_test.go:106`: "expires" matches any line of a made
  certificate, whatever its reason.
- `logfile/logfile_test.go:95`: the `Within`'s result is dropped.
- `cubelan/zero_test.go`, `send_turn_test.go`
  (`TestWaitingSendEndsWithItsContext`),
  and the proxy's "did not reach the Cube" checks:
  "nothing written" is checked at once, with no end.
- `cubelan/client_test.go:838`, `:861`:
  a count of `EventDisconnected` on a `ChanWatch` can miss an event still in
  the channel.
- `server/vwt_test.go:230`, `:233`: the cross-view `Never`s start late;
  from 0 they would check the earlier events too.
- `server/stream_admit_test.go:66`: the keep-alives counted may all predate
  the revocation.
- `server/eventline_test.go`: a doubled line still passes.
- `server/followup_test.go:95`: `cs > 4` is a loose bound.
- Struck from NOTE v1.1's list: `client_test.go:118`'s sleep is two waits now.

## Verification

- **The builders:** each its packages under `-race` at scale 2,
  20 times (the server's 99 moved tests 20 times, the package 3 times;
  the command line 5 times), at load 1 to 15; the Windows install tests
  through interop, 20 times.
- **Block 1's run 1,** the server's poll at 20 ms:
  the 35 tests that compare the Cube's lines passed 700 runs of 700,
  where 60 of 600 had failed;
  the whole package with that poll failed only `TestSettingsAPI`,
  which asserts the default poll, against 11 failures of the tests as they were.
- **The second reader** found nothing that must be fixed;
  the session fixed a latent race in `pairing_api_test.go`
  (an `n:` gets no answer, so the fake may hold it after the API's),
  `AwaitAll` with a nil match, a plain end's `Err`,
  and two sentences of `docs/testing.md`.
- **The session, before the commits:**
  `gofmt`, and `go vet` for linux, windows and darwin;
  `go test ./...` (107 s at load 1.2);
  under `-race` at scale 2, in two streams side by side at load 1 to 2:
  the Cube client with the fake Cube, and the proxy, the queue, announce,
  the Cube I/O, backup, certs, safefile and the kit, 5 runs;
  `serve`, the command line and the store 2, the server 1, all passed;
  each step's tree vetted on its own, and the first's tests run.
- **After the merge of `main`:** the bundle, `go vet` for the three
  platforms, the documents, the tests of the server, the command line,
  the kit and `tools/identifiers`, and the guidelines check (12 of 12).
- **Left to CI:** `make test-armv6` at scale 4, `make race`,
  the Windows and macOS jobs.
- **Pull request #356.**

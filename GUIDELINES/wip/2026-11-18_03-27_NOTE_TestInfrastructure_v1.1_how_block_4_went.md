# NOTE TestInfrastructure v1.1: how block 4 went

Where Block 4, Barrier, went differently from the plan,
what it left and for which block, and what verified it.

## What it made

- **The hooks (D7-A):**
  the Cube client numbers each event it puts on `Events`
  (`cubelan.Event.Seq`), keeps the number of the last,
  and counts the Cube's lines it is handling (`Client.EventCounts`);
  the server's pump keeps the number of the last event it handled,
  stored once the turn's handling is over (`runtime.pumped`).
- **The barrier:** `rt.pumpSync(t)` and `pumpSync(t, s)` in the server's tests:
  they wait until no line is in hand and the pump has reached the last event
  sent; `TestPumpSync` fails without it under a slow pump.
- **`docs/testing.md`, "The pump":**
  what holds after the barrier, when a test needs it, what it does not cover.

## Different from the plan

1. **Who did it.**
   One builder on Opus, then one second reader;
   the session made the reader's small fixes and the commits.
2. **Two commits, one pull request:**
   the client's hooks with their tests, then the pump's and the server's tests.
3. **A number, not a count, of the events the pump handled:**
   `Events` drops its oldest event where nobody reads it,
   so a count of the events handled would never reach the count sent;
   a dropped event is always followed by a newer one,
   and a pump past that one is past both.
4. **A second count, the lines in hand,** and one emit at a time:
   a line's change shows in the snapshot, and so in the API,
   before its events go out (`online` before `EventConnected`);
   without the count the barrier could return before them.
   One emit at a time also keeps two emits on a full channel from losing the
   newer event, which no user sees: the channel is full only where nobody reads.
5. **The local fixes replaced:**
   `greetingPublished`, `caughtUp`, the drift rig's `confirmed` wait
   (its name kept, D15-A), `TestWallReset`'s waits on the stream,
   the virtual view's "finding: test" marker, `TestLogSourceFilter`'s second wait,
   and the auto-reset starts' waits for the connected event.
   The NTP trick of `settleState` and `reportAndSettle` stays:
   it waits for the queue, not the pump;
   `settleState` counts its lines after a `pumpSync`.
6. **Only `startWith` calls the barrier;** the four tests that failed with a
   slow pump call it after their start. The other starts are Block 5's.
7. **No sleep before "nothing came" became a `Never`:**
   the 12 of `internal/server` wait on the server's timers
   (a backup window, a follow-up's delays, a write's deadline, a retry,
   a discovery run), which the barrier does not end.

## Seen, left for later blocks

For Block 8 (Holes):

- `TestPolledStateChangesReachTheLog`'s last wait indexes the last event,
  and panics on an empty log rather than fail.
- `TestProgramConfigEvents` and the virtual view's tests wait for a burst of a
  fixed size (`AwaitN(10)`).
- `reportAndSettle` can still return on an old line (the plan's item).
- `TestEventCountsCoverTheGreeting` rests on timing:
  were the connected event sent outside the line, it would still pass most runs.

## Verification

- **The slow pump** (100 ms an event, Block 1's run 2), the whole server package:
  with the tests as they were, the same four failed;
  with the block, `-count=3` passed (1362 s), and once more on the final tree;
  the second reader's own overlay showed the same.
- **The builder:** under `-race` at scale 2, the changed server tests 20 times,
  `internal/cubelan` 5, `internal/server` once, at load 0.1 to 2.8.
- **The second reader** found nothing that must be fixed:
  the lock cannot deadlock, the lines' count always comes back down,
  the pump's number is stored after every effect a test reads.
  The session fixed its points: the hooks' comment, the commit's reason for no
  CHANGELOG bullet, and two sentences of `docs/testing.md`.
- **The session, before the commits:**
  `gofmt`, and `go vet` for linux, windows and darwin;
  `go test ./...` (111 s at load 0.3 to 1);
  the slow pump over the server package once (460 s);
  under `-race` at scale 2, beside it:
  the Cube client, the proxy and the Cube I/O 5 runs,
  the changed server tests 5, `internal/server` once, all passed;
  the first commit's tree vetted on its own, and its tests run.
- **After the merge of `main`,** which brought only Block 3's NOTE:
  the guidelines check (12 of 12).
- **Left to CI:** `make test-armv6` at scale 4, `make race`,
  the Windows and macOS jobs.
- **Pull request #357.**

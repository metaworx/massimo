# Testing: how a test waits

The Go tests run against the fake Cube and need no hardware and no network
(the contract, "Tests").
Most of what they check happens a little later than the call that caused it:
a line reaches the fake Cube, the event pump hands an event on,
a backup is written.
This document says how a test waits for that.
The web UI's tests are [docs/ui-testing.md](ui-testing.md)
and [tools/ui-test/README.md](../tools/ui-test/README.md).

## The rule

A test waits for the thing it checks, never for a fixed time.

- **A thing that is to happen** is waited for until it holds,
  and the test goes on the moment it does.
- **A thing that is not to happen** is checked until something that shows it
  would have happened by then: a later line, an event, a count.
  A sleep proves nothing on a slow runner, where the thing would have come later.
- **Every wait's length takes the scale**, so that a slow runner waits longer:
  every deadline after which the test fails.
  Lengthening that can only let a slow runner pass;
  lengthening any other time can change what the test checks.

## The kit

`internal/testkit` holds the waits; only test files import it,
and its `TestOnlyTestsImportTheKit` fails where another file does.

| Call | What it does |
|------|--------------|
| `testkit.Eventually(t, what, cond)` | checks `cond` every 10 ms until it holds; fails after 5 s, scaled: "timed out after 5s waiting for *what*", the length as scaled (10s under `make race`) |
| `testkit.Within(d, cond)` | reports whether `cond` holds within `d`, scaled, and fails nothing |
| `testkit.Never(t, what, cond, end)` | fails where `cond` holds before `end` does, or where `end` does not come within 5 s, scaled; checks `cond` once more when `end` holds |
| `testkit.Scale(d)` | `d` times the scale: for a deadline the kit does not hold, such as a `select` on a channel |
| `testkit.For(d)`, `testkit.Every(d)` | options of the three waits: another length than `testkit.Wait` (5 s), another tick than `testkit.Tick` (10 ms) |

`what` names what the test waits for, as the failure will read:
"the second backup", "the cube's connect".
Like `t.Fatal`, `Eventually` and `Never` are called from the test's own goroutine;
`Within` fails nothing and may be called from any.

Where the failure should show what the test saw,
the test says it itself:
`if !testkit.Within(5*time.Second, cond) { t.Fatalf("…: %v", seen) }`.

A helper that names what it waits for keeps its name and waits through the kit,
as `waitConnected` in `internal/server` does:
the name says what the test needs, the kit how long it waits.

## The scale

`MASSIMO_TEST_SCALE` multiplies every wait: a positive number, 1 where it is
not set.
A value that is not a positive number stops the tests at their first wait.

| Run | Scale | Why |
|-----|------:|-----|
| `go test ./...`, `make test` | 1 | |
| `make race` | 2 | the race detector slows the tests down |
| `make test-armv6` | 4 | QEMU runs them several times slower than the host |

CI's jobs run the same targets, and so the same scales.
A wait that holds returns at once, so a larger scale costs time only where a
test would fail anyway.

What does not take the scale:

- **A window after which the test passes** because nothing came,
  such as a read that is to time out:
  a longer one may meet a later line that belongs there.
- **A sleep**, which paces the test against the product's own timers.
- **A time the test checks as the product's behaviour**,
  such as "connected within 700 ms after the address changed".
- **A time the test gives the product** as an option,
  such as a poll interval or a backup window:
  the product's own clock is not the test's wait.

## The watcher

`Eventually` waits for a state: a count, a flag, an answer of the API.
A test that waits for **the next line or event after an action** -
the fake Cube's next `s:`, the log's next warning, the stream's next state -
waits through a watch instead:
a state can already hold from before the action,
or be met by a line that is not the one the test caused.

A `testkit.Watch[T]` keeps every item it is fed, in the order fed.
The test marks it **before** the action, and waits from the mark:

```go
lines := cubewatch.Lines(fc)
m := lines.Mark()
// the action
got, next := lines.Await(t, m, "the s: line", cubewatch.Type('s'))
```

| Call | What it does |
|------|--------------|
| `w.Mark()` | the cursor after the last item fed: "from now on" |
| `w.Await(t, from, what, match)` | the first item from the cursor that matches, the rest skipped, and the cursor after it, from which a next wait goes on; fails after 5 s, scaled |
| `w.AwaitN(t, from, what, n, match)` | the first `n` items from the cursor that match, and the cursor after the last: "three failed retries"; the failure says how many came |
| `w.AwaitAll(t, from, what, matches...)` | an item for each match, in any order, each item for one match at most |
| `w.Count(from, match)`, `w.Since(from, match)` | how many items from the cursor match, and those items: for a test that compares them as a whole once they have come |
| `w.Seen(from, match)` | a condition that holds once a match came: the `cond` of an `Eventually`, or the `end` of a `Never` |
| `w.Never(t, from, what, match, end)` | as `testkit.Never`: fails where a match comes before `end` holds |
| `w.Has(t, from, what, match)`, `w.HasNot(t, from, what, match)` | the check, once the action is over, that a match has come, and returns it with the cursor after it; or that none has: fails at once, with what came |
| `w.Ended()`, `w.Err()` | whether the feed is over, such as a stream that ended; and why, as `End` said: `nil` while it goes on or where it ended plainly, a stream's read error where it was cut |

A `nil` match takes every item.
`Await`, `AwaitN` and `Never` take the kit's options (`testkit.For`, `testkit.Every`).
A wait is woken by each item, so it goes on the moment its item came.
Where it times out, the failure says what it waited for, how long,
and what came since the mark, the last 20 items:

```
timed out after 5s waiting for the s: line; 3 since the mark:
	1 l:
	2 f:ntp.homematic.com,ntp.homematic.com
	3 l:
```

A feed that ended (`w.End`) fails a wait at once, with its reason.

- **Mark before the action**, never after:
  an item that comes before the mark is not waited for,
  and one that comes between the action and a late mark is lost.
- **A filter in place of an exact count:**
  `w.Since(m, cubewatch.Except('l'))` is what the Cube received since the mark
  without the polls, which come whenever the poll's clock says.
- **A wait after a wait** goes on from the cursor the first returned,
  not from a new `Mark`, which would skip what came meanwhile.
- **"Nothing came"** is `w.Never` with an end that shows it would have come:
  often an item of the same watch, `w.Seen(m, isPoll)`.
- **"At least n"** is `w.AwaitN`, not an `Eventually` on a `Count`,
  whose failure would not say what came.
- **Once the action is over** - the process ended, the call returned
  with its log written - nothing of it is on its way any more:
  `w.Has` checks that an item came, `w.HasNot` that none did, at once.
  Where an item could still be on its way, they are `Await` and `Never`.

The feeders:

| Feed | Call | Item |
|------|------|------|
| a logger the test hands the product | `log, w := testkit.LogWatch(level, tee...)` | `testkit.Record`: `Level`, `Msg`, `Attrs` (groups joined by dots, values as text), `Line`; matched by its whole message with `testkit.Msg(msg, key, value, …)` or `testkit.MsgAt(level, msg, …)`, by a part of its line (without the time) with `testkit.Contains(text…)` or `testkit.ContainsAt(level, text…)` |
| a child process's log (`cmd/massimo`) | `testkit.ReadLog(w, stderr)` | the same `Record`, read from slog's text or JSON lines |
| an event stream the test opened | `testkit.StreamWatch(t, resp.Body)` | `testkit.Event`: `ID`, `Kind`, `Data`, or a `Comment` such as the keep-alive; matched with `testkit.Kind(kind, contains…)`; `w.Err()` tells a stream the server ended (`nil`) from one that was cut |
| a channel the test would read, such as a Cube client's `Events()` | `testkit.ChanWatch(t, ch, show)` | the channel's |
| the lines a fake Cube receives | `cubewatch.Lines(fc)` | `cube.Message`; filtered with `cubewatch.Type('s')` and `cubewatch.Except('l', 'f')` |

`testkit` imports no package of the module:
their tests import the kit, and an import back would be a cycle.
The fake Cube's lines are watched in `internal/testkit/cubewatch`,
a test-only package too,
through the fake Cube's hook `OnReceive`, which changes nothing the fake does.
`cubewatch` does not import the Cube client,
so that the client's tests can use it;
the client's events are a `testkit.ChanWatch`.
The fake Cube's own tests cannot import `cubewatch`;
one that waits for its lines feeds a `testkit.Watch` from `OnReceive` itself.
A `ChanWatch` takes its items from the channel in a goroutine of its own,
so "once the action is over" holds for it only once the item that shows it
came through the watch: a `Has` or `HasNot` right after the action can miss
an item still in the channel.

## Still written by hand

- **A sleep before "nothing came"** stays where the test has no end to wait for
  yet, and takes no scale.

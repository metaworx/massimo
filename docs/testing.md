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

## Still written by hand

- **A wait on a channel or on lines** -
  the Cube client's events, the fake Cube's lines, a child process's output -
  is a `select` or a loop of its own, its deadline `testkit.Scale`d.
- **A sleep before "nothing came"** stays where the test has no end to wait for
  yet, and takes no scale.

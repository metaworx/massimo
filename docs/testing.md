# Testing: every suite, and how a test stays steady

The front page of the project's tests:
the suites and how each runs,
the rules a steady test keeps,
the authors' rule for the fix of a flaky test,
and the kits' helpers by the pattern each closes,
the Go tests' and the web UI's, each with where it is described in full.
How a Go test waits follows them, in full ("How a Go test waits").
The web UI's kit in full is in [docs/ui-testing.md](ui-testing.md),
its sections 1 and 2;
how its suite runs, in [tools/ui-test/README.md](../tools/ui-test/README.md).
Which suites and checks a change needs before its pull request,
the contract says ([AGENTS.md](../AGENTS.md), "Project Facts"):
its test gate and its lint command.

## The suites

No suite but the plans on hardware reaches a real Cube:
the fake Cube (`massimo fakecube`) stands in for one, with placeholders,
and a run on the machine sets `MASSIMO_ENV_FILE=/dev/null`:
the checkout's `.env` points at real hardware,
as the contract says ("Secrets and identities").
CI's jobs run the same targets (`GUIDELINES/project/CI.md`, section 2).

| Suite | Run with | What it runs | In full |
|-------|----------|--------------|---------|
| Go | `MASSIMO_ENV_FILE=/dev/null go test ./...` after `make ui`, the contract's gate; `MASSIMO_ENV_FILE=/dev/null make test`, which builds the bundle first | every package's tests against the fake Cube, the guards among them: `tools/identifiers` (placeholders only) and `tools/ui-test/idioms` (the UI suite's old idioms counted) | "How a Go test waits", below |
| Go with the race detector | `make race`; one package or test with `RACEPKGS=` and `TESTFLAGS=` | the packages whose goroutines meet (`RACEPKGS`) under `-race`, every wait twice as long | "The scale", "The race detector" |
| Go on 32-bit ARM | `make test-armv6`, with `qemu-arm` (Debian's `qemu-user`) | every package's tests with `-short`, run by QEMU as the Raspberry Pi 1's core, every wait four times as long | "The scale" |
| Fuzz | `make fuzz` (`FUZZTIME`, 10 s a target) | the fuzz targets of the packages that parse untrusted bytes, one after the other; in CI on a push to `main` | the `Makefile`'s `fuzz` |
| Web UI | `make ui-test`; a file or a test with `PYTEST_ADDOPTS`, some cases with `UI_TEST_CASES` | the pytest suite of `tools/ui-test/`, Playwright with Chromium, in four cases (1100 and 390 px, English and German), each worker against a server set of its own over a fake Cube | [tools/ui-test/README.md](../tools/ui-test/README.md); its kit [docs/ui-testing.md](ui-testing.md), sections 1 and 2 |
| UI soak | [ui-soak.yml](../.github/workflows/ui-soak.yml): daily at 00:20 UTC, by hand `gh workflow run ui-soak.yml --ref main` (`-f repeat=<n>`, `-f seed=<n>`); on a machine `UI_TEST_REPEAT=3 UI_TEST_SHUFFLE=<seed> make ui-test` | the UI suite, each test three times, in the order of a seed, the four cases a job each; red where a test failed once, and gating nothing: a report of the tests to fix | `GUIDELINES/project/CI.md`, section 2; [tools/ui-test/README.md](../tools/ui-test/README.md) |
| LAN | `make lan-test`, with Docker and Compose v2 | discovery, `serve`'s mDNS announcement and `massimo discover` on a LAN of hosts in Docker, against avahi | [tools/lan-test/README.md](../tools/lan-test/README.md) |
| Android emulator | `make app-emulator-test`, with KVM, the pinned Android SDK and JDK 17 | the app's instrumented tests on an emulator, against two servers built from the tree with the fake Cube, the fingerprint simulated | [tools/app-emulator/README.md](../tools/app-emulator/README.md) |
| Packages | `make deb-test`, `make deb-systemd-test`, `make apt-test`, after `make packages`, with Docker | the Debian package installed and purged in a container, under systemd, and from an apt repository built with a throwaway key | [packaging/README.md](../packaging/README.md), "Debian package", "The apt repository" |
| On hardware | by hand, at the owner's word | the configuration writes on a real Cube; the Windows service on a real PC; each plan's report in `docs/reports/` | [docs/testing-writes.md](testing-writes.md), [docs/testing-windows.md](testing-windows.md) |

A radio write costs duty cycle:
the plans on hardware send nothing to a real Cube without the owner's go,
as the contract says ("The Cube").

## A steady test

A steady test fails only where the product is wrong:
it passes on a slow, loaded or shuffled run as it passes alone.
A test that failed once and passed when run again is flaky:
CI's retry names it, the soak finds it,
and it is fixed at its cause, never with a longer wait or a retry.
The rules:

- **It waits for what it checks, never for a fixed time.**
  A thing that is to happen is waited for until it holds;
  a thing that is not to happen is checked against an end
  that shows it would have come by then.
- **Every deadline takes the scale**
  (`MASSIMO_TEST_SCALE`, `UI_TEST_SCALE`), so that a slow runner waits longer;
  a window that waits for nothing to come does not,
  nor a time the test checks as the product's.
- **A window the product times is the test's to set**:
  through an option, a hold or a clock of the kit's,
  never by fitting the test's steps into it.
- **It marks the moment before its action and waits from it**:
  for the line, the event, the request or the answer the action brought,
  not for a state that may have held before it;
  what it compares, it reads at one moment.
- **It starts from a known state and leaves it as it found it.**
  A Go test starts a server, a fake Cube and a store of its own;
  a UI test file starts from the recorded Cube
  and leaves the server it shares as it found it.
  What a test shows the page, it tells the page's reads, not the page alone.
- **Where the fake does less than a Cube, the fake learns it**,
  or the test names what it stands in for.
- **The host's noise is the host's**:
  a test holds the ports it uses,
  skips only where the platform lacks what it needs,
  and waits longer on a slow, loaded or busy host.
- **Its failure says what it waited for and what came**,
  so that a flake seen once, in the soak or in CI's retry,
  is fixed at its cause without being caught again.

## The authors' rule

The owner's rule for whoever fixes a flaky test (AP SteadyTests, 2026-11-27),
in the contract's "Tests" too:
a fix that is a pattern is no local fix.

- **The sweep first.**
  Before the fix is written,
  its author looks for the pattern's other places in every suite.
- **The fix is a helper of the kit**:
  of `internal/testkit`, the fake Cube or a package's rig for the Go tests,
  of `tools/ui-test/helpers.py` or `conftest.py` for the web UI's;
  every place the sweep found takes it.
- **It is documented** in full where its kit is described:
  this page's sections below "How a Go test waits" for the Go tests,
  `docs/ui-testing.md`, section 1, for the web UI's;
  and by name on this page, under its pattern ("The kit by pattern"),
  so that the next test's author finds it before writing a copy.
- **A local fix stays only where the sweep finds the pattern once**,
  and its commit says so.
- **No test file copies a helper of the kit**:
  the idioms guard counts a copy ("The guards and the soak").

## The kit by pattern

The flaky tests the suites had fell into five patterns (AP SteadyTests),
and each pattern has the helpers below.
"In full" is the section that describes a helper:
of this page for the Go tests,
of `docs/ui-testing.md`, section 1, by its entry's title, for the web UI's,
unless it names another document.

### A window the test should set

A product timer or a rig's window that a slow runner overruns:
a write applied before the test read it as pending,
a deadline run out before the test's check,
a client's timeout, a fade that the test's next step came after.
The test sets the window, or holds what it checks while it checks it.

| Suite | Helper | When a test takes it | In full |
|-------|--------|----------------------|---------|
| Go | `fc.HoldApply()` of the fake Cube | a write checked while it is on its way: pending, its tag, a 412 for a tag from before it | "A write on its way" |
| Go | `r.dueNow(room)` of `internal/server`'s rig, the writes' deadline an hour (`confirming`) | what a write's deadline brings: its warning, its re-send | "A write's deadline" |
| Go | `testkit.NewClock(at)`; the drift tests' `checkClock` | a day, a window or a timer the test moves on itself | "The kit" |
| Go | an option of the product's that only the test sets, 0 for its default (`cubelan.NetWaits`, `Options.discoveryLate`) | a product timer the test would wait out | "The scale" |
| Go | `MASSIMO_TEST_SCALE`, `testkit.Scale(d)` | a deadline the kit's waits do not hold, such as a `select` on a channel | "The scale" |
| UI | `held_timers(page, *kinds)` | a state the page takes back on a timer of its own: "Copied", the toast, the settings' push | "A timed state held" |
| UI | `held_poll(page)` | a measure of what the page does by itself, which its poll would change | "The poll held" |
| UI | `pause_clock(page)`, with the marker `clock` | a timer of the page's that the test runs itself | `tools/ui-test/README.md`, "The fixtures" |
| UI | `scaled(seconds)`, `UI_TEST_SCALE` | a wait or a loop of the test's own | "The scale" |
| UI | `reads.age(rid, seconds, already=False)` | a look that depends on a value's age, which grows with the run | "How old the values are" |

### A wait on the wrong moment

A wait that holds before the moment its check is about:
a state that held before the action, a line the action did not cause,
a check before the request it checks,
an answer that Playwright has and the page has not read yet,
two reads where one moment is compared.

| Suite | Helper | When a test takes it | In full |
|-------|--------|----------------------|---------|
| Go | `testkit.Watch`: `Mark`, then `Await`, `AwaitN` or `AwaitAll` from the mark; `Has`, `HasNot` once the action is over; `Never` with an end | the next line or event after an action: the fake Cube's lines (`cubewatch.Lines`), the server's log (`testkit.LogWatch`), a stream (`testkit.StreamWatch`), a channel (`testkit.ChanWatch`) | "The watcher" |
| Go | `testkit.Eventually`, `testkit.Within`, `testkit.Never` | a state that is to come, or one that is not to come before an end | "The kit" |
| Go | `logMark`, `logAfter`, `logAfterN`, `logLines` | the next line of the log's API after an action, in `internal/server` | "The log's cursor" |
| Go | `pumpSync` | after a change seen through the API, before the log, the stream's events, the alerts or the next change | "The pump" |
| UI | `answered(page, held=…)`, `ApiReads` | before a test counts what an action read, or checks what the answers drew | "The page's reads all answered" |
| UI | `taken(page, predicate)` | a check of what an answer drew, or a step it must come before: the page has read its body | "The answer taken" |
| UI | `wait_held(page, sent, n=…)`, `sent_requests(page, method, pattern)` | what a request carried, or that a write was not sent again | "The request itself" |
| UI | `boxes(page, *targets)` | where several elements are, compared: their boxes read at one moment | "One moment for several boxes" |
| UI | `wait_until`, `settled(page, look, ok)` | a state the page is to draw; a look that takes several calls | "Waits and cleanups"; "What a test tells of a device or a room" |
| UI | `stream.mark()`, `stream.wait(…)` | the page's next event after a mark | "The stream watched" |
| UI | `wait_started`, `wait_log_live` | the page's start; the log taking the stream's state events | "The page's start"; "The log live" |
| UI | `open_fold`, `close_fold`, `toggle_fold` | a step inside a fold, after the fold's `toggle` | "A fold opened" |
| UI | `mark`, `replaced`, `frames` | that data which came took nothing from the user | "The same element" |
| UI | `safe_route(page, pattern, handler)` | every route of a test's: a request still on its way when its page closes is let go | "One safe route" |
| UI | an end of the page's own: a later read through the same route, a timer it has not set | that an action brought nothing | "Nothing came, against an end" |

### The server's own traffic, or an earlier test's leftovers

What reaches a test that it did not cause:
the server's poll, a reconnect and the events they bring,
and what a test before it left on a server or a fake Cube it shares:
a write on its way, a session, a warning, an integration, a setting.

| Suite | Helper | When a test takes it | In full |
|-------|--------|----------------------|---------|
| Go | `startServer(t, …)` | every test of `internal/server`: a server, a fake Cube and a store of its own | "The start" |
| Go | `quiet()` | a test that no poll or refresh of the server's may meet | "The start" |
| Go | `cubewatch.Except('l')` | what the fake Cube received since a mark, without the polls | "The watcher" |
| Go | `storekit.Open(t, dir)` | a store of the test's own, a copy of the template | "The store's template" |
| Go | `serialTests`, `testkit.Swapping(t)`, `testkit.ParallelOrListed` | a test that changes what the whole process shares | "Parallel tests" |
| UI | `serve -poll 0`; `POST …/cubes/<serial>/refresh` | the server sends its fake Cube nothing between a test's steps; a test that needs a read asks for it | "The server sets"; "A test that needs a read asks for it" |
| UI | `known_cube`, which resets the fake Cube (`Reset`, through its control's `POST /reset`), and `writes_shown` | each test file from the recorded Cube, with no write on its way | "A test file starts from the recorded Cube" |
| UI | `server_as_found`, `server_left`, `LEFT_FOR_GOOD` | each test file leaves the server as it found it, or fails naming what it left | "A test file leaves the server as it found it" |
| UI | `ended_sessions` | the sessions a test's sign-ins made, ended after it | "A test's sessions ended" |
| UI | `clear_preferences` | before every page: the token's user's preferences gone, the log's warnings seen | "The warnings seen" |
| UI | `own_id(what)` | what a test makes on the shared server that no API removes | "An id of the test's own" |
| UI | `server_holds_settings(holds, token=None, **values)` | a test whose outcome depends on whether the server holds the user's settings | "Whether the server holds the user's settings" |
| UI | `Reads`: `reads.cube`, `reads.device`, `reads.room`, `reads.change`, `reads.keep` | what a test shows the page, kept through the page's reads and the server's own events | "The page's reads answered"; "What the reads keep"; "What a test tells of a device or a room" |
| UI | `isolate_log(page, reads)` | a log of the test's own lines | "A log of the test's own lines" |
| UI | `stub_me(page, capabilities)` | the page signed in with given capabilities, without a user made on the server | `tools/ui-test/README.md`, "The fixtures" |

### The fake doing less than a Cube

A test that passes against the fake only because it does less than a Cube,
or fails because of it:
a command the fake answers and never applies,
a line it records only while it answers,
a device or a problem it never has.

| Suite | Helper | When a test takes it | In full |
|-------|--------|----------------------|---------|
| Go | `TestFakeHandlesEveryCommand`, with its `acknowledgedOnly` | a new command builder: the fake carries it out, or the test names why not | "A write's deadline" |
| Go | `fc.OnReceive` and `cubewatch.Lines`, with `fc.SetSilent` | a line written behind a poll the fake's silence holds | "The watcher" |
| Go | `takeConfig` | a C line a reconnect is to keep, as a Cube's greeting carries what its devices took | "Parallel tests" |
| Go | `fc.Pause()`, `fc.Resume()` | a Cube that goes and comes back on its address | "Ports" |
| UI | the fake's `-free-slots 50` and `-pair` (`run.sh`) | a Cube's free memory slots; a device that joins pairing | "The server sets"; `tools/ui-test/README.md` |
| UI | `reads.device(rf, like=…)`; the guard's stubs (`CUBES_STUB`, `FOUND_STUB`, `JOINED_STUB`) | a device, a problem or a found Cube the fixture lacks, told with GETs alone | "What a test tells of a device or a room"; section 2, "How the regions came to pass it" |

### Host noise

What the host does beside the test:
a slow or loaded runner, the pace of the race detector or of QEMU,
a port another program takes, the tests running beside it.

| Suite | Helper | When a test takes it | In full |
|-------|--------|----------------------|---------|
| Go | `testkit.Listen(t)`, `testkit.ListenPacket(t)`; `heldPorts` in `internal/server` | a port the test uses, held from its first listen to its end | "Ports" |
| Go | `testkit.BindOrSkip(t, err)` | a listen that failed: a skip only where the platform lacks the address | "Ports" |
| Go | `make race`, with its scale and `RACEGCFLAGS` | a `-race` run, by hand too | "The scale"; "The race detector" |
| Go | `awayWait` | a wait for the Cube client to see a fake that closed, on a busy machine | "Parallel tests" |
| UI | `UI_TEST_SCALE`; `run.sh`'s retry, at four times the scale, with the trace | a slow or loaded runner; the retry's trace slows the page too | "The scale" |
| UI | the run's ports (`UI_TEST_API_PORT`, `UI_TEST_CUBE_PORT`, `UI_TEST_DISCOVERY_PORT`, `UI_TEST_CONTROL_PORT`) | a run beside another, each on ports of its own | `tools/ui-test/README.md` |

### The guards and the soak

A guard keeps a pattern closed once its helper is there;
the soak finds a flaky test that the runs of its pull request did not.

| Suite | Guard | What it holds | In full |
|-------|-------|---------------|---------|
| UI | `tools/ui-test/idioms/` (`go test ./tools/ui-test/idioms`) | per test file, the fixed pauses (`wait_for_timeout`), the plain routes, a copy of a kit helper's name and the writes of the page's state, each against its list, whose counts may fall and may not rise | "The old idioms counted"; "The page's state written" |
| UI | `guard` (`conftest.py`, autouse) | a test fails on a console error, a page error, a route's problem (`safe_route`) or a horizontal scroll left at its end | `tools/ui-test/README.md`, "The fixtures" |
| Go | `TestOnlyTestsImportTheKit`; `TestParallelOrListed` | only test files import the kit; each test runs in parallel or is listed with its reason | "The kit"; "Parallel tests" |
| UI | the soak: `UI_TEST_REPEAT`, `UI_TEST_SHUFFLE`, `soak_summary.py` | each test three times in a seed's order: one that leans on the test before it, on a file before it on its worker or on a window a loaded runner overruns fails in some of its repeats | `GUIDELINES/project/CI.md`, section 2; `tools/ui-test/README.md` |

## How a Go test waits

The Go tests run against the fake Cube and need no hardware and no network (the contract,
"Tests").
Most of what they check happens a little later than the call that caused it:
a line reaches the fake Cube, the event pump hands an event on,
a backup is written.
The sections from here to the end say how a test waits for that.

## The rule

A test waits for the thing it checks, never for a fixed time.

- **A thing that is to happen** is waited for until it holds,
  and the test goes on the moment it does.
- **A thing that is not to happen** is checked until something that shows it
  would have happened by then: a later line, an event, a count.
  A sleep proves nothing on a slow runner,
  where the thing would have come later.
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

A test of a day, a window or a timer moves a clock on instead of waiting:
`testkit.NewClock(at)` stands at `at` until the test's `Add`,
which runs the timers of `AfterFunc` it passes, in their order.
A product that takes a clock with a timer type of its own takes the kit's
through a small adapter of the test's package,
as `kitClock` is the server's `notify.Clock`.
The room checks' clock of the drift tests (`checkClock`) is another:
the real time plus an offset the test moves on,
so that the Cube's time, read at the greeting, moves on by itself too.

## The scale

`MASSIMO_TEST_SCALE` multiplies every wait: a positive number,
1 where it is not set.
A value that is not a positive number stops the tests at their first wait.

| Run | Scale | Why |
|-----|------:|-----|
| `go test ./...`, `make test` | 1 | |
| `make race` | 2 | the race detector slows the tests down |
| `make test-armv6` | 4 | QEMU runs them several times slower than the host |

CI's jobs run the same targets, and so the same scales.
A wait that holds returns at once, so a larger scale costs time only where a
test would fail anyway.

**A `-race` run by hand** takes what `make race` gives its tests:
scale 2, and `RACEGCFLAGS` ("The race detector").
A plain `go test -race` gives each wait half that time,
and a store-heavy test needs about twice its time with `checkptr` on in SQLite,
so a wait that holds under `make race` can time out there:
that timeout is the run's, not a flaky test's.
Where a document names `go test -race ./...`, as the shared Go baseline does,
it means `make race` here.
One package or one test runs through the target:

```
make race RACEPKGS=./internal/server TESTFLAGS='-run TestLogAfter'
```

or, after `make ui`, by hand with both:

```
MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=2 go test -race -count=1 \
	'-gcflags=modernc.org/...=-d=checkptr=0' ./internal/server
```

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

**A product timer a test waits out** - a wait for an answer that never comes,
a window that runs to its end - is shortened through an option:
a field of the product's options or of the call's, 0 for the product's default,
which only the test sets.
A new one is never a package variable the test swaps,
which keeps the test off the parallel ones ("Parallel tests");
the tests that still swap one,
such as `TestWallReset` its `wallSilence` and `cmd/massimo`'s apt tests `healthEvery`,
are on their package's `serialTests`.
The network configuration's reads and writes take `cubelan.NetWaits`,
which `massimo netconfig` leaves 0.
Its tests against the fake Cube give the calls it answers deadlines of half a second,
scaled, and 5 s for a restart;
the calls it leaves unanswered get short waits of their own (`silentWaits`),
not scaled, since those are waited out in full.
The search's wait for the N and c answers of an answer is
`cubelan.SearchOptions.LateWait`, the server's `Options.discoveryLate`:
a second from the answer, which outlasts a test's run of 200 ms
wherever a proxy answers the identify request alone;
500 ms, scaled, in the runs of `discovering`.
Of such an option, a deadline for an answer that comes takes the scale,
since a slow runner would fail it, as `NetWaits.Ask` does;
a window waited out where nothing answers, as `NetWaits.Reread`,
and a pace the product keeps, as `NetWaits.Settle`, do not.
Where the length is what the test checks,
such as the second a late Cube gets (`TestSearchWaitsForALateAnswer`),
the test keeps the product's.

**A client's timeout against a local fake** is such a deadline
where the fake's answer is to come:
`internal/update`'s `shortAnswerWait` is 250 ms, scaled,
since a slow runner's answers take longer.
Where the timeout is what the test checks,
it is given to the call the test checks alone:
`TestDownloadOutlastsTheClientTimeout` reads the release with a client without one,
and only its download gets the client's 250 ms, which it outlasts;
the release's first answers took longer than that once on Windows.
A Cube client's `ReplyTimeout` is a deadline for the fake's answers too,
but the tests that shorten it wait it out where the fake is silent,
and check the loss it brings against the poll's interval, which is not scaled:
they keep the length they give.

A test that waits longer than it needs to shows in CI:
the jobs Vet and test, Race and Test (armv6, QEMU)
name their 20 slowest tests and their 10 slowest packages in the run's summary
(`GUIDELINES/project/CI.md`, section 3).

## A write on its way

The fake Cube applies a radio command once it acknowledged it:
at once, or `ApplyAfter` later, as a device takes it at its wakeup.
A test that checks a write while it is on its way -
pending, its tag, a 412 for a tag from before it -
does not race that window:
on a slow runner its read comes after the apply,
or after the re-read that confirms it.
It holds the fake's apply instead, for as long as it needs:

```go
release := fc.HoldApply() // the devices asleep: the write stays on its way
t.Cleanup(release)
// the write, and what the test checks while it is on its way
release() // they wake and take it
// the wait for the confirmation
```

- **While a hold lasts** the fake applies nothing:
  each command that falls due meanwhile,
  at its acknowledgement or `ApplyAfter` later,
  is applied at the release, in the order they fell due,
  before the release returns.
- **Holds nest**, and only the last release applies;
  a second call of one release does nothing,
  so `t.Cleanup(release)` beside the test's own call is safe.
- **A test that never releases** keeps the write on its way to its end,
  as `TestRoomProgramTagOfTheMembers` does.
- **A helper that checks a write's answer** holds the apply over its own
  call and releases it as it returns, as the drift rig's `setRoom2` does:
  a rig that polls often, as that one every 50 ms,
  shows the devices' new state to a slow handler before it judges the write,
  which it then finds confirmed, not pending.
- **`ApplyAfter` stays** for a test of the delay itself,
  as `TestApplyDelayDiscardAndOff`.

**The follow-up outlasts the release.**
The server re-reads a written C line after each of its `FollowUpDelays`
and judges the write not confirmed at the last re-read:
a release after it leaves the write unconfirmed.
`TestProgramTag` re-reads every 100 ms, 600 times.
The follow-up ends at the first re-read that shows the write,
so only a held write waits for more.
A write the L entry confirms, such as a setpoint, needs no re-read,
but its deadline (`confirmWithin`) counts from the answer,
and takes the scale, as in `TestRoomCommandConfirmed`,
or the test sets it ("A write's deadline").

## A write's deadline

A write that a device does not show is judged at its deadline,
`confirmWithin` after the Cube's answer (`confirm.go`):
its warning, and for a room setpoint of the API's its re-send (`resend.go`).
A test of what comes then does not race that window either:
it would have to check all it checks while the write is pending,
and make each newer write, before the window ran out.
It gives its writes a deadline they never reach, an hour,
and sets the moment itself:

```go
r := startServer(t, confirming(time.Hour, 0)) // the drift rig: resendOptions(time.Hour, …)
// the write, and what the test checks while it is pending
r.dueNow(2) // room 2's write is due: the follow-up judges it at once
// the wait for its warning, its re-send
```

- **`r.dueNow(room)`** (`confirm_test.go`) makes the deadline of the room's
  open write now and wakes the follow-up, which judges it at once.
  It waits for a write to the room whose deadline is still to come:
  called again after a warning, it makes the re-send's own write due,
  once the re-send has gone.
- **Before the call** the test waits for what the deadline is to find,
  such as the members that hear showing the write,
  since the deadline judges what the Cube's snapshot shows at that moment.
- **The queue's hold** (`engine.Options.SlotWait`) is no write's deadline,
  and nothing ends it early.
  A test of a re-send behind it checks the hold's length as the product's:
  the re-send's line comes the hold's wait after the hold's line,
  as in `TestUnconfirmedRoomCommandResentBehindTheHold`,
  not that nothing came by a moment the test happens to read.
  A test that must act during the hold, as
  `TestUnconfirmedRoomCommandNotResent`'s "superseded behind the hold",
  still has the hold's wait to do it in.

**The fake carries out every command the server builds, or says why not.**
`TestFakeHandlesEveryCommand` in `internal/cubelan/fakecube`
reads the builders of `internal/protocol/cube` from their source,
sends each command to a fake, and checks its effect in the fake's lines.
A command the fake only answers, as it answers `z:`,
is named in the test's `acknowledgedOnly` with its reason,
and changes none of the fake's lines.
A new builder fails the test until its case does one or the other.

## The race detector

`-race` turns on `checkptr` too, which checks, as the code runs,
each conversion of an `unsafe.Pointer` to a typed pointer and its arithmetic.
`modernc.org/sqlite` is SQLite's C translated to Go and made of such steps:
under `-race`, checking them was half the time of opening a fresh store,
which the migrations make and most tests now copy ("The store's template").
So `make race` turns `checkptr` off for the packages under `modernc.org` alone
(`RACEGCFLAGS` in the `Makefile`),
and the store-heavy tests take about half as long.
The race detector stays on in every package, SQLite's included,
and `checkptr` in Massimo's own packages and in every other dependency.
SQLite's pointer steps are its own project's to test.
Go gives a package the last `-gcflags` whose pattern matches it, not their sum,
so a `-gcflags=all=…` in `TESTFLAGS` checks SQLite's steps again,
as `make race RACEGCFLAGS=` does alone.
A `-gcflags` in `GOFLAGS` comes before the `Makefile`'s,
so under `make race` a `-gcflags=all=…` there no longer reaches SQLite's packages:
put it in `TESTFLAGS`.

## The watcher

`Eventually` waits for a state: a count, a flag, an answer of the API.
A test that waits for **the next line or event after an action** -
the fake Cube's next `s:`, the server log's next warning,
the stream's next state -
waits through a watch instead,
and for the next line of the log's API through the log's cursor (below):
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
`Await`, `AwaitN` and `Never` take the kit's options (`testkit.For`,
`testkit.Every`).
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
- **A count of tries** a retry loop makes is counted from the change too,
  where the test changes what the tries meet:
  `TestProxyRetriedUntilTheAddressIsThere` counts the listens since its error changed,
  since the slot's own count reaches any number before the change.
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
The fake receives each line as it reads it,
also while it is silent (`SetSilent`), when it holds every answer:
a wait for a command written behind a poll the silence holds sees it come.
`cubewatch` does not import the Cube client,
so that the client's tests can use it;
the client's events are a `testkit.ChanWatch`.
The fake Cube's own tests cannot import `cubewatch`;
one that waits for its lines feeds a `testkit.Watch` from `OnReceive` itself.
A `ChanWatch` takes its items from the channel in a goroutine of its own,
so "once the action is over" holds for it only once the item that shows it
came through the watch: a `Has` or `HasNot` right after the action can miss
an item still in the channel.

## The log's cursor

The log's API (`GET …/log`, `docs/api-v1.md`) is no feed a watch could take:
a client reads it,
and a watch fed by a goroutine reading it would need a read to catch up at each mark.
A test of `internal/server` that waits for the log's next line after an action -
a device's state line, an `error_reset`, a warning of a write not confirmed -
marks the fake Cube's log before the action and waits from the mark,
as with a watch (`logcursor_test.go`, AP LogCursor):

```go
m := logMark(t, ts)
// the action
ev, next := logAfter(t, m, "the error_reset", logLine("error_reset", "0a010d"))
```

| Call | What it does |
|------|--------------|
| `logMark(t, ts)`, `logMarkAs(t, ts, token)` | the cursor after the newest line of the fake Cube's log, at any level: "from now on"; its read writes the store's pending lines first, so that a line kept before the mark is behind it; `logMarkAs` for a server that asks for a token |
| `logStart(ts)` | the cursor before the log's first line, for a helper that counts the whole log, "the log has n", as `waitWatch`, `waitOutcome` and `waitSlotsEvents` do, or takes its first line of a kind, as `waitUnconfirmed` |
| `logAfter(t, c, what, match)` | the first line after the cursor that matches, and the cursor after it, from which a next wait goes on; fails after 5 s, scaled |
| `logAfterN(t, c, what, n, match)` | the first `n` lines after the cursor that match, and the cursor after the last; the failure says how many came |
| `logLines(t, c, match)` | the lines after the cursor that match, as the log holds them now: once the action is over, what came since the mark, or that none came after the lines a wait took |
| `logLine(kind, device)` | a match of a line of `kind`, and of `device` where it is not `""`, at info and above: the line a page shows without `level=` |

A cursor is the id of the last line before it.
Each check of a wait reads the log forward with `after=<id>` from the last line it read,
at every level, page after page,
in the test's own goroutine:
it reads each line once, misses none, and stops at no page's `limit`,
where a count over one page stopped at its 500 or 1000 lines.
Since it reads every level, a match says the level it takes:
`logLine` and `atInfo` take what a page without `level=` shows,
a match of its own takes a debug line too.
A `nil` match takes every line.
A failure says what came since the cursor, the last 20 lines,
each with its id, level, kind, device, changes and summary,
as a watch's does.
A cursor beyond the log's newest line, one the log never gave out,
is the API's `409`, and fails the test at once (AP LogCursor, D3-A).

The helpers read one route, a Cube's own:
the fake Cube's, `GET /api/v1/cubes/KEQ0000100/log` (`cubeLogPath`).
A test of every Cube's log, `GET /api/v1/log`, reads that route directly.
A Cube without its row in the store keeps its log in memory,
whose ids lie far above the store's:
a mark taken then is beyond the store's newest line
once the Cube's log moves to the store with its row,
and every read from it is answered `409`.
With `stored()` the Cube has its row before the start returns.

- **Mark before the action**, never after, as for a watch;
  after `pumpSync` where the pump could still write an earlier report's line,
  which the wait would take for the action's, as `settleState` marks.
- **A wait after a wait** goes on from the cursor the first returned.
- **"The log has n"** - n warnings, n slots lines -
  is `logAfterN` from `logStart`,
  and `logLines` after the cursor it returned is empty:
  none came after them.
  So is "this line, and no second one" after a mark,
  as TestAutoReset checks its second `error_reset`.
- **Right after the start**,
  the log holds what the start waited for ("The start"):
  the greeting's lines, the connected line, `server_started`,
  what the pump's turn of the connected line writes.
  A test reads them directly, without a wait.
- **A line written by a call that returned**, such as `s.logServer`'s,
  is in the log at the next read, the store's pending lines written first;
  a wait from a mark before the call returns it at its first read.

The cursor or a watch of the server's log (`withLog`, `testkit.LogWatch`):

- **The cursor** is for what a client of the API reads:
  a line of the log as the store or the memory keeps it,
  with its id, its level, its summary, its command and who started it;
  a filter's page (`kind=`, `device=`, `command=`) is read directly
  once the cursor has its lines.
- **`testkit.LogWatch`** is for what the server writes to its process log,
  serve's text log:
  the message and attributes a user reads there, the Cube client's lines,
  a warning that is no line of the API's log.
  A line of the API's log is printed there too,
  but a test of what the API's log keeps waits through the cursor:
  the printed line has no id,
  and is printed before the store keeps it, or drops it.

## The pump

The API shows a Cube's devices,
rooms and connection from the Cube client's snapshot,
which changes the moment the client reads the Cube's line.
The server's event pump takes the client's event after that,
and only then writes the log's line, publishes the stream's event,
confirms a write or queues a reset.
A test that saw a change through the API and goes on to read the log,
the stream or an alert, or changes the Cube again,
can be ahead of the pump: on a loaded runner it is.

In `internal/server`'s tests, `rt.pumpSync(t)` waits until the runtime's pump
has handled every event its client had sent out at the call,
those of the Cube's lines the client was handling then included;
`pumpSync(t, s)` does it for each of the server's Cubes.
After it:

- **the log** holds the pump's lines: `GET …/log` writes the store's pending
  events before it reads;
- **a subscriber's channel** holds the pump's events,
  as `TestWallReset`'s does;
  a stream read over HTTP still needs a watch's `Await`,
  since the handler writes it in a goroutine of its own;
- **what the pump does in its turn** is done:
  a room's confirmation, a reset queued, a backup's window touched.

When a test needs it:

- **after the start, before the first action**, where what the test checks is
  measured against the greeting:
  a configuration's or a list's changes, a state line's.
  The start calls it ("The start");
- **after it saw a change through the API**,
  before it reads the log, the stream's events or the alerts,
  or changes the Cube again;
- **before a test event** (`mark`) that ends a `Never` on the stream:
  what the pump makes of the steps before is then published before it.

What it does not cover:

- **a change the client makes outside the Cube's lines** -
  a lost connection, a first dial that fails
  (the `disconnected` of a Cube that never answers, `secondCube`),
  a held connection that resumes, a pairing -
  which the test waits for by its event or its line,
  or by the call that made it, as a device forgotten by the removal;
- **what the pump starts and does not wait for**:
  a command it queues (its line on the fake Cube, through `cubewatch`),
  a re-read, a notification;
- **the server's timers**,
  such as a backup window, a follow-up's delays or a write's deadline.

It rests on three values that exist for tests:
the Cube client's count of the Cube's lines it is handling,
and the number of the last event it sent out (`cubelan.Event.Seq`;
both through `Client.EventCounts`),
and the pump's number of the last event it handled (`runtime.pumped`).
An event dropped because no one read it leaves no gap:
a newer one follows it, and the pump that handled that one is past both.

## The start

Every test of `internal/server` starts its server with
`startServer(t, opts...)` (`start_test.go`),
over a fake Cube KEQ0000100 with the recorded greeting.
It returns once the Cube is connected,
through its runtime whatever the API's token asks,
the pump has handled the greeting (`pumpSync`),
and the store's writer has written what came before,
the server's start included: the same for every test,
which waits for none of it again.
With `withCubes` it waits for nothing:
no Cube, and not the store's writer, which `Run` may not have started yet.
The rig it returns:

| Field | What it is |
|-------|------------|
| `r.fc`, `r.addr`, `r.lines` | the fake Cube, its address, and the watch of every line it received (`cubewatch.Lines`); `nil` with `withCubes` |
| `r.s`, `r.ts`, `r.rt` | the server, its API (`httptest`), and KEQ0000100's runtime; `r.rt` is `nil` with `withCubes` |
| `r.st` | the store, `nil` without one |
| `r.log` | the watch of the server's log, with `withLog` |
| `r.stop()` | stops the API, then `Run`, and waits for its end; the cleanup calls it, and a test that stops the server itself, as a restart does |

The options:

| Option | What it does |
|--------|--------------|
| `withOptions(o)` | the server's options; the start sets their Cubes, and a gap of 1 ms where `o.Queue` sets none |
| `editOptions(f)` | changes them, after `withOptions` whatever the order |
| `stored()`, `onStore(st)` | a store, a copy of the template ("The store's template") or one given, whose row of KEQ0000100 is the Cube, added where it has none; `seedStore(f)` writes to it before the server starts |
| `withGreeting(g)`, `changeGreeting(f)`, `rooms()` | another greeting, a change to it, the rooms fixture (`fakecube.RoomsFixture`) with the commands applied |
| `withFake(f)`, `withCube(f)` | a change to the fake before it starts, to the Cube's configuration |
| `withCubes(cfg…)`, `alsoCube(cfg)` | other Cubes in place of the fake's, for a Cube that is late, held or found; one beside it; the start waits for neither |
| `again(r)`, `overFake(fc, addr)` | the store and the fake of an earlier start: the server started again; a fake started already |
| `beforeRun(f)` | a change to the server before it runs |
| `quiet()` | a poll once an hour, so none while the test runs, and no refresh |
| `withLog(level)` | the server's log into `r.log` |
| `listenOn(ports)` | the proxies' listeners from the ports the test holds ("Ports") |

A test file bundles what all its tests need into an option of its own,
as `storedRooms`, `resetsOn`, `confirming` and `discovering` do;
a rig with waits of its own builds on the start and keeps its name (`startDrift`,
`startSlotsCube`, `startMismatchCube`, `startMovingCube`).
The recorded greeting is read once for every test:
`recorded(t)` returns a copy of it, the test's to change.
A test of a route's change line reads it with `changeSummaries(t, r, action, target)`
(`logchange_test.go`, AP KeyLife, Block 4):
once the store has written what the server handed it,
the summaries of the change lines of that action on that target (`token:4`),
oldest first, from the newest 100 of KEQ0000100's log,
as the owner's key reads them,
whose `changes` and `subject` the test then checks (`TestPutToken`,
`TestKeyEndDate`, `TestKeyHeirs`).
One test runs the server by hand, since it tests `Run`'s start and end:
`TestNothingStartsAfterRun`.

## Parallel tests

The tests of `internal/server`, `internal/store`, `cmd/massimo`,
`internal/cubelan` with its fake Cube, and `internal/serve` run in parallel:
every test calls `t.Parallel()` as its first statement,
but those on its package's `serialTests` in `parallel_test.go`,
each with its reason.
Go runs the listed tests first, one after the other,
and the parallel ones once the listed ones have ended,
as many at a time as `-parallel` says (by default as many as there are CPUs).
The other packages' tests still run one after the other.

A test goes on the list where it:

- **swaps a package variable** (`lookupCNAME`, `argonKey`, `pruneChunk`)
  or calls `t.Setenv`, which panics in a parallel test;
- **changes what the whole process shares**:
  its environment, its working directory (`t.Chdir`),
  a signal sent to it (`TestCatchSighup`),
  or what it counts for all its goroutines,
  such as the allocations of `runtime.MemStats`;
- **binds a fixed port**,
  such as a Cube added through the API as the only one,
  whose proxy listens on the Cube's port, :62910;
- **checks a time as the product's**,
  such as "the observer's calls return within 500 ms",
  which the tests beside it would stretch.

`cmd/massimo` keeps its options in package variables,
such as the selected Cube of `-sn`, the log level and the update's hooks,
and reads its settings from the environment;
`internal/serve` reads the settings file's Cubes from the environment.
A test of either that sets them stays on the list,
until the product takes them as arguments.

A swap goes through `testkit.Swapping(t)`, called before it,
in the test or in the helper that makes the swap, as `fakeDiscovery` does.
It sets an environment variable through `t.Setenv`,
so a swap in a parallel test panics at once instead of racing with the tests beside it.

`TestParallelOrListed` in each of the packages calls
`testkit.ParallelOrListed(t, ".", serialTests)`,
which reads the package's test files, those of every system,
and fails where a test neither calls `t.Parallel()` first nor is on the list,
where a listed test calls it,
where the list names a test that is not there or gives no reason,
and where `Parallel()` is called outside a test's own body and a subtest's
function literal, such as in a helper.
The kit's own test holds the guard to each of these.

A subtest calls `t.Parallel()` where it builds its own server,
as those of `TestUnconfirmedRoomCommandNotResent` do;
one that goes on from what the subtest before it left does not.
The guard does not check which subtests call it.

What the parallel tests share:

- **the recorded greeting**, read once;
  `recorded(t)` returns the test's own copy;
- **a helper's state across tests**,
  such as `serverStore`'s map of each test's store, which is a `sync.Map`;
- **the environment as `TestMain` left it**:
  `internal/serve`'s unsets every `MASSIMO_CUBE_*` variable before the tests run,
  its parallel tests fail where one is set (`noCubeEnv`),
  and `clearCubeEnv` makes the plain forms the serial test's,
  so that what a settings file it loads sets is undone at its end;
- **massimo built once** (`builtBinary` in `cmd/massimo`),
  which the tests that run `massimo serve` as a process start side by side,
  each with a data directory of its own and the API at port 0,
  in `massimo-test-bin-*` in the temp directory:
  `TestMain` removes it, trying for 5 s,
  and names on stderr one it could not remove (seen with `-v`),
  and a run sweeps one a killed run left once it is a day old;
- **the store's template**, made once per test binary and only read;
  each test writes a copy of its own ("The store's template");
- **SQLite's allocator**,
  since `modernc.org/libc` takes one lock for every `malloc` and `free` of the process:
  the tests that open a fresh store wait for each other while its migrations run,
  which is why most tests open a copy of the template instead.
  The store's tests still take no less time on more than four cores:
  under `make race` their time is that of the migrations' own tests,
  which migrate released stores and take up to 50 seconds each.

On a busy machine, a step that a quiet one always wins can come second:

- **a wait for the Cube client to see a fake that closed** is `awayWait`,
  the kit's wait and the client's reply timeout,
  since a poll the close cut off waits that out first;
- **a C line set with the fake's `SetConfig`** is not in its greeting,
  so a reconnect brings the old one back:
  `takeConfig` sets both, as the Cube's greeting carries what its devices took.

## The store's template

A store the migrations make costs about a tenth of a second,
and under `make race` most of a second,
much of it under the one lock of SQLite's allocator ("Parallel tests"):
sixteen opened side by side take as long as sixteen one after the other.
The migrations therefore run once per test binary, into a template,
and a test that opens a store itself opens a copy of it,
which costs a few milliseconds, 25 under `make race`,
unless it needs a fresh store (below).

`storekit.Open(t, dir)` (`internal/testkit/storekit`) opens the store of `dir`,
of a new temporary directory where `dir` is `""`,
and closes it at the test's end;
where `dir` has no `massimo.db`, its file is first a copy of the template,
and a file already there is opened as it is, as a restart's is.
`internal/server`'s `testStore` and the start's `stored()`,
`internal/serve`'s `testStore`,
and the tests of `cmd/massimo` that open a store for a server they build
open their stores through it.
`storekit.OpenAt(t, dir, now)` is `Open` with the store's clock `now`,
a `testkit.Clock`'s `Now`, for a test that moves the store's days on
where the product counts them from the times it wrote,
as the idle limit of API keys does (AP KeyLife, Block 3;
`internal/server`'s `storeOptionsAt`).
Only the store's times follow it; a server's own checks keep the real time,
so such a test starts the store's clock days back and lets it catch up with the
real time rather than run ahead of it (`TestKeyIdle` of `internal/server`:
an ended token is told from a guess by the server's clock).

`storekit.Raw(t, st)` opens a second connection to the file of `st`,
past the store, as another process holds one,
waiting for a lock as the store does, and closes it at the test's end:
for a test that writes what the store never writes,
such as a row an older version left,
or that holds the file's lock while the store works.

- **The template** is a store that `store.Open` makes in a temporary directory,
  at the version `test`, the first time a test asks for it:
  it is closed, its file read, and the directory removed at once.
  The close brings the WAL into the file;
  a `-wal` or `-shm` left behind, which would hold what a copy lacks,
  or a file below the binary's schema version, which every copy would migrate,
  fails each test that asks for the template.
- **A copy** is the template's bytes, mode 0600.
  `store.Open` finds nothing to migrate,
  and makes `node.key` and `opaque.key` as it does for a fresh store.
  What the file holds is the template's in every copy of the binary:
  `node.id` and `tenant.id`, the seeded rows and their times;
  and `Created()` reports false.
- **A test opens a fresh store** where a copy is not what it checks:
  a migration's own test, an upgrade, a raise or a downgrade,
  and a test of what the migrations make of an empty file,
  such as the schema (`TestStoreSQL`, `TestSyncColumns`),
  the seeded roles and the node's identity.
  An additive migration's own test is
  `additiveMigration(t, prev, v, name, fill, older...)`
  (`internal/store/additive_test.go`, AP KeyLife, Block 3): a file at `prev`,
  migrated to `v` alone and filled there in plain SQL,
  keeps `min_reader` 24 and opens with a binary for `prev` and for each version
  of `older`, such as 26, the 0.17 line's,
  and the migration's down part takes it back to the schema of `prev`,
  every token row kept; it returns a copy of the filled file at `v`,
  which the test opens with this binary, migrated to the newest version.
  A test does not read a file at `v` with this binary's reads,
  which need every later migration's columns:
  `TestTokenMakerMigration` did, and broke at the next migration.
  A test that needs two distinct node identities,
  such as a test of two nodes' sync,
  opens fresh stores too: copies share `node.id` and `tenant.id`.
- **Where the product opens the store itself**,
  as `serve.Open`, a `massimo serve` process and `massimo user` do,
  it makes its store on the empty data directory, as at a first start,
  and the test copies nothing into it.

The half of the template that needs no store is the kit's,
as `testkit` holds the watch that `cubewatch` builds on ("The watcher"):
`testkit.FileTemplate(name, build, check)` makes the file `name` once per test binary
in a temporary directory, refuses a `-wal` or `-shm` beside it,
takes its bytes, runs `check` on it, and removes the directory;
`testkit.WriteNew(path, b)` writes a copy, mode 0600,
where no file is at `path` yet.
`storekit` builds on them with `store.Open` and a read-only open,
and so do `internal/store`'s own tests (`template_test.go`),
which cannot import `storekit`, since it imports the store:
`open(t, dir)` opens a copy where `dir` has no store yet,
`copied(t)` is a temporary directory that holds one,
for a test that opens it with options of its own, such as a clock or a log,
and `openFresh(t, dir)` runs the migrations.

## Ports

A test holds a port for as long as it needs it, and hands the listener on.
It never closes a listener to learn a free port,
nor closes one and opens another on its port:
between the two, another program can take it,
such as another package's tests under `make race`.

- **`testkit.Listen(t)`, `testkit.ListenPacket(t)`**:
  a TCP listener, a UDP socket, on the loopback at a port the system chose,
  closed at the test's end.
  The test hands them on: `fc.Serve(testkit.Listen(t))`,
  or to a product's listen hook.
  A UDP socket held and never read is an address where nothing answers,
  such as a discovery target without a Cube.
- **`testkit.BindOrSkip(t, err)`** ends a test whose own listen failed:
  it skips where the platform lacks the address,
  as macOS has 127.0.0.1 alone of the loopback's 127.0.0.0/8,
  and fails on any other error, such as a port another program holds,
  which a skip would turn into a green run with the test not run.
  CI's jobs Vet and test, Race and Test (armv6,
  QEMU) name every test that skipped, with its reason,
  in the run's summary (`GUIDELINES/project/CI.md`, section 3);
  the Windows and macOS jobs do not report their skips.
- **The server's proxies** listen through `heldPorts` (`ports_test.go`)
  and the start's `listenOn(ports)`, over the hook `Options.proxyListen`.
  `ports.hold(t)` is an address whose listener goes to the first listen at it.
  `ports.taken(t)` is one the proxy gets only once the test frees it:
  until then its listens fail as at a port another program holds,
  since the test holds it.
  `ports.listens(addr)` counts the listens at an address:
  `TestProxyRetryStops` holds a port it never gives,
  and checks that a stopped slot tries it no more.
- **The announcer** takes its two sockets through `options.listen`
  (`heldSockets` in `internal/announce`).
- **The Cube goes and comes back** with `fc.Pause()` and `fc.Resume()`,
  in place of a fake closed and a second one started on its address.
  `Pause` drops the clients and refuses new ones;
  the address stays the fake's.
  On Linux a client's dial is refused, as at an address where nothing listens:
  the listener closes, and a socket bound without listening holds the port.
  Elsewhere the listener stays open, and the fake resets each connection it
  accepts before any line: the dial succeeds, and the connection ends at once.
  The Cube client takes both as a failed attempt.
  `Resume` greets as before, with what changed in the fake meanwhile,
  and the port is its listener's alone again.
  A `SingleClient` fake keeps a dead session it holds across the pause,
  and refuses as before once resumed.
  A fake that is never paused is unchanged,
  but for a `SingleClient` fake on a listener that is not TCP's,
  which refuses a second client by a close;
  `massimo fakecube` has no pause, and listens on TCP.
  For the moment of a `Resume`, another listener can take the port,
  as in the race of a port freed and taken again; `Resume` then fails.
- **An address where no Cube is** is a paused fake
  (`goneFake` in `internal/cubelan`'s tests).

Where a product takes an address alone, no listener can be handed on,
and the free-port helper stays there, its comment saying why:

- `freeUDP` in `internal/server`, for the server's own discovery socket
  (`Options.DiscoveryAddr`), which the test also sends to;
- `freeAddr` in `internal/serve`, for the proxy of `MASSIMO_CUBE_PROXY`;
- `freeAddr` in `cmd/massimo`, for serve's `-proxy`
  and the Windows service test's `-listen`.

## Still written by hand

- **A sleep before "nothing came"** stays where the test has no end to wait for yet,
  and takes no scale: the server's timers, which `pumpSync` does not end,
  are such waits.
- **An address nothing listens on**, for a dial that is to fail
  where no fake Cube stands for it,
  is a listener opened and closed: `deadAddr` in `internal/env`,
  `dead` in `cmd/massimo`'s `options_test.go`,
  `closed` in `internal/server`'s `proxied_test.go`.

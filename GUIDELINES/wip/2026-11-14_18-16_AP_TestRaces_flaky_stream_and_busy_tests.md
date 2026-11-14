# AP TestRaces v1.0: Flaky stream and busy tests

## TL;DR

Three tests of `internal/server` fail now and then because they go on before
something in the background is done; the stream and the link are right.
Both fixes are in the tests' code only.

- **`TestStreamResumes`, `TestStreamResyncs`:**
  `startWith` returns once the client is connected → once the runtime has also
  published the greeting's events, `connected` last (D1-A).
- **`TestABusyCubeIsListedAsBusy`:**
  the server starts while the fake may not have taken the other client yet → it
  starts once the fake has (`fc.Connections() == 1`),
  as the `cubelan` twin does (D2-A).
- **Measured:**
  10 failures in 600 stream runs → 0 in 900, and 0 in 200 beside 64 busy loops;
  the busy test 1 in 60 and 1 in 150 beside 128 busy loops (`-cpu=2`) → 0 in
  150.

### Blocks

1. `startWith` waits for the greeting's events
2. The busy-Cube test waits for the other client to be taken

## Discussion

The owner asked on 2026-11-14 why two stream tests fail on `main`
without load (`TestStreamResumes` 1 in 10, `TestStreamResyncs` 1 in 40,
3 in 40 on a branch that does not touch the stream), and why
`TestABusyCubeIsListedAsBusy` failed once under heavy parallel load.

**The owner's decisions** (`EXEC++ D2-A`, 2026-11-14):

- **D1-A:**
  the stream tests are fixed in `startWith`, for every test that uses `start`.
  Not B, the two tests made tolerant: the other users of `start` stay exposed,
  and the `seq+1` edge of "future" goes untested.
  Not C, the API's `connected` held back until the greeting is published:
  it changes what the API says, and no client needs it.
- **D2-A:**
  the wait is in the test.
  Not B, a fakecube helper: exported API on a package `massimo fakecube` ships,
  for two callers.
  Not C, reading the other client's greeting: more lines for the same guarantee.

## Analysis

The measurements ran through `go test -overlay`, with the changed test
files in the session's scratch directory.

### The stream tests

- `startWith` (`server_test.go`) polls `GET /api/v1/cubes` until the Cube is
  `connected`, which is `rt.client.Connected()`:
  set by the cubelan client when the greeting is complete.
- The server's event loop turns the client's events into the stream's
  afterwards, on its own goroutine: `config`, a `state` per device,
  then `connected`.
  A test that counted what was published after `start` returned found
  late events in 10 starts of 300, up to all eight.
- **`TestStreamResumes`:**
  a late `state` lands after the id the test resumes from,
  so the resumed stream replays it, rightly;
  the test takes any `state` for a state burst ("header:
  a resumed stream sent state").
- **`TestStreamResyncs`:**
  "future" is `rt.seq + 1` when the test reads it.
  One late event later that id is issued,
  the stream resumes with nothing to replay and the test waits in vain ("no
  event"); two,
  and the first event is that `state` (`first event "state", want resync`).
- Under `-race` the timing differs, the cause is the same.

### The busy-Cube test

- A `SingleClient` fake refuses a second client on Linux by cutting its
  backlog to 0 and parking a connection of its own in the accept queue
  (`fakecube/oneclient.go`); it does so when it accepts the first.
- `net.Dial` for the test's "other" client returns once the kernel has queued
  it, before the fake has accepted it.
  Under load the server's first dial gets into the queue before the backlog is
  cut: the dial succeeds, no greeting comes,
  and the client waits for one up to its read deadline (24 hours without a poll
  interval).
  No dial times out, so the link stays `offline`.
- The failing run under load was the one where the fake had not taken the other
  client when the server started; five others like it passed,
  the server's dial coming late enough.
- `fc.Connections()` counts a client under the lock that also cuts the backlog,
  so a count of 1 means the fake refuses.
  `internal/cubelan/link_linux_test.go` waits for it already.

## Implementation Plan

### Block 1: `startWith` waits for the greeting's events

1. **`internal/server/server_test.go`:**
   `startWith` returns once the Cube is connected and every runtime's ring holds
   a `connected` event; a helper `greetingPublished` says so, its comment why.
2. **No CHANGELOG fragment:**
   a test helper alone changes, nothing a user of massimo sees;
   the commit says so.

- **Verification:**
  `-count=300` of both stream tests passes,
  and `-count=100` beside 64 busy loops;
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server`.

### Block 2: The busy-Cube test waits for the other client

1. **`internal/server/link_linux_test.go`:**
   after dialling the other client, wait for `fc.Connections() == 1`,
   with a comment saying why.
2. **No CHANGELOG fragment**, as in Block 1.

- **Verification:**
  `-count=150 -cpu=2` beside 128 busy loops passes.
- **The gate, after both blocks:**
  `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `go test -race -count=1` over the race packages, `make cross`, `gofmt -l .`,
  `go vet` on three platforms, `GUIDELINES/shared/tools/check`.

## Proposed commit message

For Block 1:

```
test(server): A started server has published its greeting's events

For AP TestRaces v1.0, Block 1: startWith returned once the client was connected, while the server's event loop could still be publishing the greeting's events; TestStreamResumes and TestStreamResyncs then met them in their resumed streams.

Key changes:
- internal/server/server_test.go: startWith waits for every runtime's connected event (greetingPublished)
- No CHANGELOG bullet: a test helper alone changes, nothing a user of massimo sees

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-14): first revision, D1-A, D2-A.

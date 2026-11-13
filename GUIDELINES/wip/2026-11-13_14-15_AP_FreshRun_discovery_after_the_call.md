# AP FreshRun v1.0: discovery after the call

## TL;DR

A `POST /api/v1/cubes/discover` that comes while a run is going gets that run,
begun before the call, so it can miss a change made just before it;
the tests are made to wait for a run of their own first,
then the server gives every such call the next run.

- `TestCubeConfiguredBySerialFollowsItsAddress` → joins the start's run under load, times out
  → its helper posts until it gets a run that began after the call
- a call during a run → that run's result, `ran_at` before the call
  → the run after it, started as that one ends, shared by every call that came meanwhile
- the loop's own runs (start, interval, every minute, wake) → share a run going, as today
- the helper's repeat → an assertion that `ran_at` is never before the call

### Blocks

1. The tests wait for a run of their own: `postDiscovery` posts again after a joined run
2. A call gets the next run: the follow-up run in `discover`; tests; the documents; the fragment

## Discussion

On 2026-11-13 `TestCubeConfiguredBySerialFollowsItsAddress`
(`internal/server/discovery_test.go`) failed once in a full
`MASSIMO_ENV_FILE=/dev/null go test ./...` with
"timed out waiting for the cube at 127.0.0.3", and passed alone five times
and in a full rerun.
The owner asked why it is sensitive to load, to reproduce it,
and to make it robust without hiding a real bug.

**The owner's decisions**, 2026-11-13, `PLAN++ D1-A D2-B`
on the TL;DR gate and the evaluation of D2-B after it:

- **D1-A.** The tests' helper `postDiscovery` returns only a run that began after the call;
  every call site has it, and nothing of the server's is reached into.
- **D2-B.** A call to `POST /api/v1/cubes/discover` gets a run that began after it:
  the run going is followed by one more, shared by every call that came while it ran.

**Refined here, routine:**

- **Who gets the next run:** `postDiscover` alone, the API's call.
  The discovery loop's runs keep sharing a run going:
  a Cube added during a run is matched by it, since the answers are taken
  after its wait (`noteSerial` reads the configuration then).
- **`ran_at`** of the follow-up run is when it starts, after the run before it ends.
- **A caller that gives up** (its request cancelled) leaves the follow-up run to the others;
  it runs even when nobody waits any longer, as a run does today.

**Names chosen here**, routine:
`discNext` (the follow-up run, waiting to start), `discoverAfter` (the API's call);
the branch `claude/fresh-run`.

**Not in this plan:**
a Cube configured by serial whose connection is refused does not ask for discovery;
it waits for the periodic run (30 minutes by default) or a call.
That is a design question of its own, not raised by the owner.

**Who runs it:**
this session, under the owner's `PLAN++ D1-A D2-B` of 2026-11-13,
whose gate stated Block 1 runs to its pull request, without a merge:
the plan registered on main, Block 1 committed on `claude/fresh-run`
and its pull request opened; Block 2 waits for the next gate.

## Analysis

**The failure, reproduced** on 2026-11-13 on jaguar (16 cores, WSL 2)
with 64 busy loops beside the test binary:
2 failures in 40 runs, 3 in 150, each after 5.27 s; a pass takes about 0.55 s.
A copy of the test with the server's Debug log and the `POST`'s answer showed
the same in every failure:

1. The start's run hears the Cube at 127.0.0.2; `noteSerial` sets the address
   (`rt.client.SetAddress`), and the client connects within about a millisecond.
2. The test sees `connected` at 127.0.0.2, silences 127.0.0.2, lets 127.0.0.3 answer,
   closes the first fake Cube, and posts.
3. The start's run has not yet cleared `s.discRun`
   (`discover` does it after `discoverOnce` returns: the store, `settle`, the events);
   the `POST` shares it and gets its answer, `ran_at` about 270 ms before the call,
   the Cube at 127.0.0.2.
4. No other run comes: the test's `DiscoverInterval` is 0 and the Cube has an address.
   The client retries 127.0.0.2 with its backoff (1 s, 2 s, 4 s) until `eventually`'s 5 s run out.

Not the cause: the 200 ms answer wait (127.0.0.3 was heard in every run that started
after the move), the 5 s timeout, the 10 ms poll, the port shared by 127.0.0.2 and 127.0.0.3,
or the order of the tests (the package runs none in parallel).
With the helper of Block 1 in the copy, 200 runs under the same load passed,
4 of them after a joined run.

**The same weakness** in the other tests that post after the start's run:
`TestAnUnknownAnswerIsNotFollowed` passes without testing anything when its call
shares the start's run, its check being that nothing moved;
`TestOneSerialFromACubeAndAProxy`'s "three runs" can be two.

**The server's behaviour** is the documented one today (`docs/api-v1.md`,
`POST /api/v1/cubes/discover`: "A call while a run is going waits for it and gets its result,
the same `ran_at`"). A run sends its requests at its start,
so a call during it can get an answer from before a change the caller just made:
a Cube plugged in, or one configured by serial that took another address,
then followed only at the next periodic run.
Discovery is UDP alone, no radio and no Cube's LAN slot,
so one more run a call is cheap; shared by every call during a run,
a burst of calls costs one run.
A call during a run then waits for the rest of it and a run of its own,
up to about twice the wait (6 s by default).

**After Block 2:** `discover` keeps `discRun`, the run going, and `discNext`,
the run to start when it ends.
`discoverAfter`, the API's call, joins `discRun` when none is going,
else `discNext`, made where there is none;
the end of a run starts `discNext` where there is one.
`discover`, the loop's, is as today.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/server/discovery_test.go` | 1, 2 | `postDiscovery` waits for a run of its own; in Block 2 an assertion; the tests of the follow-up run |
| `internal/server/discovery.go`, `server.go` | 2 | `discNext`, `discoverAfter`, `postDiscover` through it |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 2 | a call during a run gets the next one |
| `CHANGELOG.d/Changed/claude-fresh-run.md` | 2 | the fragment |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1 ./internal/server`,
and the follow test 200 times under load
(64 busy loops, the test binary with `-test.count=200`).
Nothing reaches a Cube, a box or GitHub but the plan's registration and Block 1's pull request.
One functional commit per block, citing `AP FreshRun v1.0, Block <n>`.

### Block 1: The tests wait for a run of their own

1. `postDiscovery` notes the time, posts, and posts again while the answer's `ran_at`
   is before it; a third joined run fails the test.
   Its comment says why: a call during a run shares it.
2. No other change to the tests: every caller of `postDiscovery` has it.
3. Test-only: no CHANGELOG fragment (`changelog.sh check` confirms at the commit).

**Verification:** the test gate; the follow test 200 times under load without a failure.

### Block 2: A call gets the next run

1. `server.go`: `discNext *discoveryRun` beside `discRun`, under `discMu`.
2. `discovery.go`: the start of a run in a function of its own (`startRun`, `discMu` held);
   `discoverAfter(ctx)` joins `discRun` where none is going, else `discNext`, made where none is;
   the end of a run starts `discNext` where there is one; `postDiscover` calls `discoverAfter`.
   The file's comment and `discover`'s say who shares what.
3. Tests: a call during a run gets a run that began after it;
   two calls during one run share one follow-up run (the same `ran_at`);
   the loop's run during a run shares it.
4. `postDiscovery`'s repeat becomes an assertion: the answer's `ran_at` is never before the call.
5. `docs/api-v1.md` and `docs/api-v1-openapi.yaml`: a call while a run is going waits for
   the run after it, which every call that came meanwhile shares;
   `CHANGELOG.d/Changed/claude-fresh-run.md`:

   ```
   - A search for Cubes that comes while one is going gets the run after
     it, so that it hears a Cube plugged in or moved just before the
     call; the calls that came meanwhile share that run.
   ```

**Verification:** the test gate.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +FreshRun v1.0

TestCubeConfiguredBySerialFollowsItsAddress fails under load: its discovery call shares the start's run, begun before the Cube moved. The owner decided on 2026-11-13 to make the tests wait for a run of their own and to give an API call the next run. PLAN++ FreshRun, with D1-A and D2-B.

Key changes:
- GUIDELINES/wip/2026-11-13_14-15_AP_FreshRun_discovery_after_the_call.md: the plan, two blocks, the reproduction in its Analysis

File-revision-v1: GUIDELINES/wip/2026-11-13_14-15_AP_FreshRun_discovery_after_the_call.md::v1.0::2026-11-13::All::the first revision, by PLAN++ FreshRun
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `test(server): Discovery's tests wait for a run that began after their call`;
Block 2, `feat(api): A search for Cubes during a run gets the run after it`.

## Change History

- v1.0 (2026-11-13): the first revision,
  from the owner's D1-A and D2-B on the TL;DR gate.

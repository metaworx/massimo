# AP LostSearch v1.0: search for a lost Cube

## TL;DR

A Cube whose connection is lost is searched for at once and every minute
until it is back, so that one that took another address is followed
without waiting for the periodic run or a call of the API.

- loss → nothing until the 30-minute run → a search at the loss (`wakeDiscovery`)
- the loop's minute → while a Cube has no address → also while one is lost (`Client.Lost()`)
- which Cubes → every lost one (D2-B); one with a fixed address only where
  periodic discovery is on, so that `-discover-interval 0` keeps meaning "on request"
- a fixed-address Cube that moved → the existing warning "answers discovery from another address"
- the minute → an option the tests shorten

### Blocks

1. Search at the loss and while lost: the loss event, the loop's condition, tests, the documents, the fragment
2. The LAN test: a serve following its Cube by serial when the Cube takes another address

## Discussion

The owner asked on 2026-11-13, after AP FreshRun, to close the gap it left open:
a Cube that follows discovery and loses its connection is not searched for;
it waits for the periodic run (30 minutes by default) or a search through the API,
and in the meantime its client retries the old address.
A scratch test confirmed it the same day: the fake Cube moved from 127.0.0.2 to 127.0.0.3,
nobody posted, and 8 s later the Cube was still lost at 127.0.0.2, with no connection to 127.0.0.3.

**The owner's decisions**, 2026-11-13, `PLAN++ D2-B` on the TL;DR gate:

- **D1-A.** Searched at the loss, then every minute while the Cube stays lost
  (the gate's recommendation).
- **D2-B.** Every lost Cube, not only those following discovery:
  a fixed-address Cube that moved then gets the warning
  "cube answers discovery from another address than configured".
- **D3-A.** The LAN test gets a step for it (the gate's recommendation).

**Refined here:**

- **A fixed-address Cube is searched for only where periodic discovery is on**
  (`DiscoverInterval` > 0, `serve`'s default 30 minutes).
  `-discover-interval 0` says "only on request through the API";
  the search for a Cube without an address is the one exception today,
  since nothing else can connect it, and a following Cube joins it,
  since only discovery can move it.
  It also keeps the package's tests off the LAN:
  most start a server with a fixed-address fake Cube, no interval,
  and the default discovery socket (`:23272`, the broadcast addresses),
  and many of them lose their Cube on purpose.
  The owner may widen it at the gate.
- **The first search comes at the loss**, `Lost()` being set before the event;
  a Cube still booting is heard by the next, a minute later.
- **A deliberate reconnect is no loss** (AP LostCube's D1-A): `Lost()` stays false,
  and nothing is searched for.

**Names chosen here**, routine:
`needSearch` (was `needAddress`), `Options.AddressRetry` (the minute, for the tests);
the branch `claude/lost-search`; the LAN test's host `follower`.

**Not in this plan:**
a real Cube taking a new DHCP lease, which is the owner's check on the hardware,
in "Run Hardware/Owner tests" only on the owner's word.

**Who runs it:**
this session, under the owner's `PLAN++ D2-B` of 2026-11-13,
whose gate stated the blocks run on a branch to a pull request, without a merge:
the plan registered on main, both blocks committed on `claude/lost-search`,
pushed, and the pull request opened.

## Analysis

**Today**, on main: `cubelan.Client.Run` reports a loss once,
"cube connection lost" and `EventDisconnected`, after setting `Lost()`,
and dials the same address again with its backoff (1 s doubling to a minute);
`SetAddress` from discovery wakes its dial.
The server's event loop (`internal/server/server.go`, the runtime's events) logs and publishes the event,
and asks nothing of discovery.
`discoveryLoop` (`internal/server/discovery.go`) runs at the start and every `DiscoverInterval`,
when `wakeDiscovery` asks (a Cube added without an address),
and every `addressRetry` (a minute) while `needAddress`: a Cube without an address.
`noteSerial` moves a following Cube to the address it answers from,
and warns of a fixed-address one answering from another.

**After Block 1:**

- The event loop calls `s.wakeDiscovery()` on `EventDisconnected` of a Cube `searchLost` takes:
  one following discovery (no address configured, or `FollowDiscovery`),
  or any where `DiscoverInterval` > 0.
- `needSearch`: a Cube without an address, or one `searchLost` takes that is `Lost()`.
- The loop's next run after one: `AddressRetry` while `needSearch`, as today for an address.

**Tests in the package:** a lost following Cube is followed to its new address
without a call of the API; one still lost is searched for again after `AddressRetry`
(shortened); a fixed-address Cube without an interval is not searched for
(no request reaches a responder at its address);
a fixed-address Cube with an interval that answers from another address is warned of.

**The LAN test** (`tools/lan-test/`): a host `follower`, `serve` with the Cube by serial alone
(`MASSIMO_CUBE_KEQ0000100_NAME`), no proxy, no announcement, `-discover-interval 0`, `-poll 5s`.
After the box's checks, the Cube's cable back, the follower connects to the Cube;
then the Cube's address changes on the bridge (`ip addr` in the Cube's host, `NET_ADMIN`),
the follower loses it by its polls, searches, and connects at the new address.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/server/server.go` | 1 | `Options.AddressRetry`; the loss event wakes discovery |
| `internal/server/discovery.go` | 1 | `searchLost`, `needSearch`; the comments |
| `internal/server/discovery_test.go` | 1 | the four tests |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 1 | the runs `serve` makes |
| `CHANGELOG.d/Changed/claude-lost-search.md` | 1 | the fragment |
| `tools/lan-test/compose.yaml`, `run.sh`, `README.md` | 2 | the follower and its step |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over `internal/server`, and `make lan-test`.
Nothing reaches a Cube, a box or GitHub but the plan's registration and the pull request.
One functional commit per block, citing `AP LostSearch v1.0, Block <n>`.

### Block 1: Search at the loss and while lost

1. `server.go`: `Options.AddressRetry` (0 is a minute), its comment;
   in the runtime's event loop, `EventDisconnected` of a Cube `searchLost` takes wakes discovery.
2. `discovery.go`: `searchLost(rt)`; `needAddress` becomes `needSearch`;
   `addressRetry` the default of the option; the file's and the loop's comments.
3. Tests, as in the Analysis, the minute shortened to 300 ms.
4. `docs/api-v1.md` and the OpenAPI description, where they list the runs `serve` makes:
   also at a Cube's loss and every minute while it is lost;
   `CHANGELOG.d/Changed/claude-lost-search.md`:

   ```
   - A Cube whose connection is lost is searched for on the LAN at once
     and every minute until it is back, so that one that took another
     address is followed without waiting for the periodic search.
   ```

**Verification:** the test gate; the package's tests leave the LAN alone
(a run of `go test ./internal/server` with UDP 23272 held by another socket logs no
"discovery port in use").

### Block 2: The LAN test

1. `compose.yaml`: the `follower` host.
2. `run.sh`: step 10 - the Cube's cable back, the follower connected to it;
   its address changed on the bridge; the follower logs the address from discovery
   and connects at the new one, within 60 s.
3. `README.md`: the host and the step.

**Verification:** `make lan-test` passes, the follower's step included;
the test gate.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +LostSearch v1.0

A Cube whose connection is lost is not searched for: one that took another address waits for the periodic run or a search through the API. The owner decided on 2026-11-13 to search at the loss and every minute while it lasts, for every lost Cube. PLAN++ LostSearch, with D1-A, D2-B and D3-A.

Key changes:
- GUIDELINES/wip/2026-11-13_16-56_AP_LostSearch_search_for_a_lost_cube.md: the plan, two blocks

File-revision-v1: GUIDELINES/wip/2026-11-13_16-56_AP_LostSearch_search_for_a_lost_cube.md::v1.0::2026-11-13::All::the first revision, by PLAN++ LostSearch
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `feat(server): A lost Cube is searched for at once and every minute until it is back`;
Block 2, `test(lan-test): A serve follows its Cube to another address after the loss`.

## Change History

- v1.0 (2026-11-13): the first revision,
  from the owner's D2-B and the gate's D1-A and D3-A.

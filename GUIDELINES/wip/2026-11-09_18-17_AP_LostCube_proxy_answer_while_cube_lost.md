# AP LostCube v1.0: proxy answer while the Cube is lost

## TL;DR

serve's LAN proxy answers UDP discovery for its Cube from a greeting it never
forgets,
so other servers and the original software still find an unplugged Cube;
and serve takes a Cube that went silent for lost only after three polls and
half a minute.

- discovery answer → while a greeting is known → while serve has the Cube:
  none from the loss it reports to the next greeting,
  a deliberate reconnect being no loss (D1-A)
- the proxy's TCP side unchanged (D2-A);
  the server itself stays findable by mDNS, whatever its Cube does
- a silent Cube → lost after 3 polls + 30 s → also after two unanswered polls in
  a row, within 2 polls + 10 s (D3-E)
- between polls → Go's keepalive, 150 s, never reached → 15 s + 3 × 5 s,
  after a check that the Cube answers the probes (D3-E)
- one serial from two addresses → its own plan (D4-A)

### Blocks

1. SilentProxy: the client's loss state, the proxy's discovery answer,
   tests, `docs/proxy.md`, the CHANGELOG
2. MissedPolls: two unanswered polls end the connection, a silent fake Cube,
   tests, the README, the CHANGELOG
3. Keepalive: the dialer's keepalive, after the check on the box;
   the CHANGELOG

## Discussion

**The owner's question** of 2026-11-09, from the FoundCubes check on real
hardware (AP FoundCubes, merged and retired):
with the test Cube's network cable pulled,
a broadcast search from another serve still listed the Cube,
now from the address of the box that proxies it only;
a search aimed at the Cube's own address showed it gone,
as FoundCubes intends.
The serve holding the Cube noticed nothing until the cable was back
("connection reset by peer", re-established in 2 s).
The owner puts the outage at 30 to 60 s.

**Decided** at the TL;DR gate and its EVAL,
by the owner's `PLAN++` with the recommended options:

- **D1-A.**
  The proxy answers discovery while serve has the Cube:
  silent from the loss serve reports ("cube connection lost") to the next
  complete greeting; a deliberate reconnect keeps it answering.
  B, only while `Client.Connected()`,
  would silence the proxy for the seconds of every deliberate reconnect
  (the refresh every 6 h, a follow-up, a new address),
  and another serve searching then logs the Cube gone and found again.
  C, the stale-after as a grace period, is at least an hour, a day by default:
  the fault stays, later.
- **The owner's proposal at the EVAL,** a second discovery for the server,
  always answered,
  so that an app on a phone always finds the server to configure it, exists:
  serve announces itself by mDNS as `_massimo._tcp` (AP ServerAnnouncements),
  from its start, with or without a Cube, whatever the Cube does,
  and the apps browse for it (`docs/app.md`, `discover(ms)`).
  The Cube protocol's port 23272 cannot carry it:
  its reply describes a Cube (serial, radio address, firmware),
  and the original software would list a server's answer as one more Cube.
  Block 1 says so in `docs/proxy.md`.
- **D2-A.**
  The proxy's TCP side is unchanged.
- **D3-E.**
  Both: two unanswered polls in a row end the connection (Block 2),
  and a keepalive tuned to find a dead Cube between polls (Block 3),
  once the box shows that the Cube answers keepalive probes.
  F, a user timeout as well, is not taken:
  a socket option per platform for the case Block 2 covers.
- **D4-A.**
  One serial heard from two addresses is not in this plan.

**Names chosen here**, routine and not decisions of the owner's:
`Client.Lost`, `pollMisses`, the fake Cube's `SetSilent`,
the blocks SilentProxy, MissedPolls and Keepalive,
the tests named in the Implementation Plan,
the branch `claude/lost-cube`.

**Not in this plan:**

- the TCP side while the Cube is lost (D2):
  a client given the address still connects and reads the last snapshot,
  and its writes fail, as today;
- one serial from two addresses (D4):
  with no proxy serial set, another serve hears the Cube's serial from the
  Cube and from the proxy in one run,
  and one that follows that Cube's address by discovery (`note` in
  `internal/server/discovery.go`) moves to each answer in turn;
  read from the code, not tested;
- a user timeout (`TCP_USER_TIMEOUT` on Linux,
  `TCP_MAXRT` on Windows, `TCP_RXT_CONNDROPTIME` on macOS);
- the hardware finding itself (the Cube reset a connection held across the
  outage):
  `docs/portal-protocol.md` takes it from the session that made the check,
  with the Cube's firmware.

**Who runs it:**
this session.
The plan's own commit goes straight onto `main`;
the work branch `claude/lost-cube` starts after it, from `origin/main`,
and becomes one pull request with a commit per block.
Blocks 1 and 2 run under the owner's `PLAN++`;
Block 3 waits for the check on the box and the owner's signal.

## Analysis

### Why the proxy keeps answering

`(*Proxy).discoveryReply` answers an identify request whenever
`Client.Snapshot().Hello()` is set.
The snapshot takes the H line at each greeting and never drops it,
which is right for the snapshot:
the proxy's TCP greeting and the API read it while the Cube is away.
It is wrong for discovery, which tells the LAN that a Cube is there.

The client knows two things about its connection:

| Moment | `Connected()` | the loss Run reports |
|--------|---------------|----------------------|
| connected, greeting complete | true | none |
| a deliberate reconnect (refresh, follow-up, `ReconnectIfMoved`) | false | none |
| from a loss to the next greeting | false | since the loss |
| before the first greeting | false | no H line yet either |

The loss is what `Run` reports once, with `EventDisconnected` and the
warning "cube connection lost";
it is held in `Run`'s local `lost` only.
Block 1 keeps it on the client as well, for the proxy to read.

### When serve notices a dead Cube

The client polls with `l:` every poll interval (a minute by default,
10 s to an hour) and waits 10 s for the L reply;
a poll without one logs "poll failed" and nothing else.
The reader's deadline is 3 polls + 30 s (`readDeadline`):
3 min 30 s at the default, about 3 h at an hour.
Go's dialer turns keepalive on with 15 s idle, 15 s between probes and 9 probes,
150 s of silence, which a poll every minute never leaves;
and Linux sends no probe while a poll's bytes are unacknowledged.

In the owner's test, 30 to 60 s:
the first poll after the pull fails at most 70 s after it,
the loss comes after 3 min 30 s, so neither came, as the code says.
When the cable was back, the Cube no longer knew the connection and reset
it at the next segment.

What the blocks would have done in that test:

- Block 1 alone: nothing, since no loss is reported.
- Block 2: nothing either, since its first miss comes at most 70 s and
  the loss at most 2 polls + 10 s after the pull.
- Block 3: a dead Cube about 30 s after the pull,
  where the pull falls far enough before the next poll,
  then a silent proxy until the reconnect after the cable is back;
  otherwise the poll's bytes stop the probes and Block 2 takes over.

### Block 3's risk

A Cube that does not answer keepalive probes would be dropped after every 30 s
without traffic,
and the Cube is known to lose its configuration when clients connect
and disconnect too often (`internal/cubelan`'s package comment).
The probes go out today already, Go's default,
but nothing has shown that the Cube answers them:
with a poll every minute nine unanswered probes never add up.
So before Block 3 the box holding the test Cube is read, by the owner's word,
without sudo and sending nothing new:
`ss -tio` on serve's connection to the Cube shows `timer:(keepalive,…)`,
and `lastack` falling back to about zero every 15 s while `lastrcv` grows means
the Cube acknowledges the probes.

### Tests

All against the fake Cube on loopback.
A loss is the fake closed: the client's next dial is refused.
The Cube back is a new fake on the same address.
A pulled cable is a fake that keeps its connections and answers nothing (Block
2's silent mode), then answers what it held back when it speaks again,
as TCP delivers it once the cable is back.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over the Makefile's `RACEPKGS`,
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`
from the main checkout.
Nothing reaches a real Cube or a box but the reading before Block 3.
One functional commit per block, citing `AP LostCube v1.0, Block <n>`,
with its CHANGELOG bullet, written in the wrapped style of its neighbours.

### Block 1: SilentProxy

1. `internal/cubelan/client.go`:
   `Client.Lost()`, true from the loss `Run` reports to the next complete
   greeting; a connection closed by `Reconnect` is none.
2. `internal/proxy/proxy.go`:
   `discoveryReply` answers nothing while `Client.Lost()`, with a Debug line;
   its comment and the package's say so.
3. Tests:
   - `internal/cubelan/client_test.go`, `TestLostFromALossToTheNextGreeting`:
     false once ready and throughout a `Reconnect`,
     true once the fake is closed,
     false again once a fake on the same address has greeted.
   - `internal/proxy/proxy_test.go`, `TestProxyDiscoverySilentWhileCubeLost`:
     answered; once the fake is closed and the client lost, no answer;
     once a fake on the same address has greeted, answered again.
4. `docs/proxy.md`, "Discovery": the answer follows the Cube,
   and the server itself is found by mDNS whatever its Cube does.
5. `CHANGELOG.md`, Unreleased, Fixed:

   ```
   - serve's proxy no longer answers discovery for a Cube it has lost,
     so other servers and the original software see the Cube go; serve
     stays findable by mDNS (`docs/proxy.md`, "Discovery").
   ```

**Verification:**
the test gate;
the proxy's test fails on the old code (it is answered while the Cube is
lost) and passes after step 2.

### Block 2: MissedPolls

1. `internal/cubelan/fakecube/fakecube.go`:
   `SetSilent(on bool)`: while on, the fake keeps its connections,
   sends no greeting and holds every line it reads unanswered; off,
   it carries on with what it held.
2. `internal/cubelan/client.go`:
   - `Send`'s reply timeout is an error type of its own, its text the same;
   - `session` counts the polls that time out with no line read between them,
     and the second (`pollMisses`) ends the connection with an error naming
     them, which `Run` reports as the loss;
   - a poll refused as busy, and one while pairing runs (`Inclusion().Active`,
     since whether the Cube answers `l:` then is unconfirmed), count nothing;
   - the first miss still logs "poll failed";
     `readDeadline`'s comment calls it the backstop.
3. Tests, `internal/cubelan/client_test.go`:
   - `TestSilentCubeIsLostAfterTwoPolls`:
     a silent fake is lost well before the read deadline,
     and connected again once it speaks;
   - `TestOneMissedPollKeepsTheConnection`:
     silent for one poll, then speaking: no loss, one connection;
   - `TestSilentCubeWhilePairingIsKept`:
     no loss while pairing runs; lost once it ends.
4. `README.md`, where `serve` keeps its connection to the Cube:
   when it takes the Cube for lost.
5. `CHANGELOG.md`, Unreleased, Changed:

   ```
   - serve takes a Cube that leaves two polls in a row unanswered for
     lost, within two polls and 10 s, and reconnects; it waited three
     polls and 30 s (`README.md`, "Server").
   ```

**Verification:**
the test gate;
`TestSilentCubeIsLostAfterTwoPolls` fails on Block 1's code
(the read deadline takes 30 s) and passes after step 2.

### Block 3: Keepalive

Runs only once the box has shown that the Cube acknowledges keepalive probes,
at the owner's signal.
Where it does not, a NOTE records it and the block is dropped.

1. `internal/cubelan/client.go`:
   the default dialer's `KeepAliveConfig`: on, 15 s idle, 5 s between probes,
   3 probes; a test reads it from the dialer `New` makes.
2. `CHANGELOG.md`:
   Block 2's bullet amended:
   between polls a dead Cube is found within about 30 s.

**Verification:**
the test gate;
on the box, after the owner's go,
a cable pull of 30 to 60 s gives "cube connection lost"
and a broadcast search without the Cube; that check is the owner's.

## Proposed commit message

The plan's registration,
a commit of its own on `main` before the first project write:

```
chore(aps): +LostCube v1.0

serve's LAN proxy keeps answering discovery for a Cube it has lost, and serve takes a silent Cube for lost only after three polls and 30 s; the plan makes the answer follow the Cube and the loss come sooner, from the owner's cable pull during the FoundCubes check.

Key changes:
- GUIDELINES/wip/2026-11-09_18-17_AP_LostCube_proxy_answer_while_cube_lost.md: the plan, three blocks, the third after a reading on the box that holds the test Cube.

File-revision-v1: GUIDELINES/wip/2026-11-09_18-17_AP_LostCube_proxy_answer_while_cube_lost.md::v1.0::2026-11-09::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `fix(proxy): Discovery is not answered for a Cube serve has lost`
2. `feat(cubelan): Two unanswered polls in a row end the Cube's connection`
3. `feat(cubelan): Keepalive finds a dead Cube between polls`

## Change History

- v1.0 (2026-11-09): the first revision, from the owner's cable pull
  during the FoundCubes check;
  D1-A, D2-A, D3-E and D4-A by the owner's `PLAN++`.

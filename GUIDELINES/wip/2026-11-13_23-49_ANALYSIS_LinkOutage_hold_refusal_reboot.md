# Analysis LinkOutage: hold the connection, recognise the refusal, reboot the Cube

2026-11-13. The task: keep a Cube reachable through a short outage of serve's own network,
from the measurements on breno (Pi 5, Cubes with firmware 1.1.3).
It includes the owner's question in the session:
why not hold our side for the 41 minutes the Cube holds its own,
and Reboot in System settings → Cubes, with a warning about pending radiograms,
a confirmation and a capability of its own.

## TL;DR

- **Hold** a lost connection for **45 min** instead of letting the keepalive end it at about 30 s.
  45 min is just above the longest lockout measured (41½ min).
  The Cube is marked lost after the two missed polls, as LostCube did; the socket stays open,
  and the session resumes when the Cube's answer arrives.
- **End the hold early** where it cannot resume:
  the Cube resets it, discovery finds the Cube at another address,
  or discovery has had answers from the Cube for 3 min without the connection resuming.
- **Recognise the refusal:**
  - the loss was serve's own (a timeout, not the Cube's reset or close);
  - the dials time out;
  - the Cube answers discovery from the same address.

  Then the log, `GET /api/v1/cubes` and the Cubes panel say "the Cube still holds an old connection",
  not "offline".
- **Reboot the Cube** is the owner's decision only (capability `cubes.reboot`), never automatic:
  `POST /api/v1/cubes/{sn}/reboot`, a button in System settings → Cubes
  with a confirmation naming the pending radiograms, and `massimo netconfig -reboot`.
- **The fake Cube** gets the Cube's one-client rule:
  a second client's dial times out, and a session that ends during an outage stays held.
- **docs/cube-protocol.md** has nothing of 2026-11-13 yet; Block 1 records it.

## What happens today

- `internal/cubelan/client.go:127`: `keepAlive` = 15 s idle, 5 s apart, 3 probes.
  An idle connection whose link is gone ends about 30 s later with ETIMEDOUT.
  This is the "read: connection timed out" at 26–30 s on breno.
- `client.go:404`: two polls in a row unanswered (`pollMisses`) end the session too,
  and `conn.Close()` runs.
- The Cubes in the middle of a poll survived because a poll's unacknowledged bytes stop the keepalive.
  Linux then retransmits for about 15 min (`tcp_retries2`),
  so these connections lived through the minute of flapping.
- After the end, `Run` dials every second, then doubling up to a minute.
  The Cube still holds the dead session and drops our SYNs:
  its reset with seq 0 and no ACK is ignored in SYN-SENT, so every dial ends in "i/o timeout".
- The lessons already say that a Cube with a client drops a second client's SYNs silently.
  The Cube that holds serve's dead session is that case, with serve as its own second client.

## 1. The hold (the owner's question: wait as long as the Cube does)

The owner's question is right, and it changes the recommendation.
A hold costs almost nothing while it lasts:

- **The heating does not need the Cube.**
  The devices run their own programmes, so a held Cube costs only visibility and control,
  and the loss shows in the UI as it does today.
- **Holding wins in every case:**
  - **The Cube kept its side:** TCP's retransmissions reach it once the link is back,
    at most 2 min apart, and the session resumes with no new connection and no lockout.
  - **The Cube dropped its side** (it restarted, or its own timeout ran out):
    the next retransmission or probe gets a reset, the session ends at once,
    and the dial succeeds.
  - **The Cube moved:** LostSearch finds it at its new address,
    and `ReconnectIfMoved` drops the held connection, as it does today.
  - **The Cube is off:** a dial would fail anyway.
- **Ending early buys nothing:** while the Cube holds its side,
  a new connection is refused until the Cube lets go, 16 to 42 minutes as measured.

What the hold changes in the session:

- **Losing:** the missed polls still mark the loss:
  the warning "cube connection lost", `Lost()`, and `EventDisconnected`.
  The proxy goes silent and LostSearch starts, as today.
  The socket stays open.
- **While held:**
  - No new polls are written: the missed `l:` are still on their way,
    and their L answer is the sign the Cube is back.
  - `Send` refuses with `ErrNotConnected`, as between connections today.
    A setpoint written into the held socket would reach the Cube minutes later,
    after the caller had been told it failed.
  - `Reconnect` waits for the resumption;
    `ReconnectIfMoved` still drops a connection to an old address.
- **Resuming:** the first line from the Cube ends the hold, with "cube connection re-established",
  the time it was down, and `EventConnected` with `CauseLoss`.
  The L line answers the missed poll, so the device states are current.
  The H line, the budget, stays as last known until the next S reply.
- **Ending:** the hold ends:
  - at 45 min;
  - on a reset or close from the Cube;
  - when discovery moves the Cube;
  - when discovery has had answers from the Cube's address for 3 min without a resumption.
    That one catches a Cube that dropped its side without a reset,
    or a source address of ours that is gone, the case of the earlier lesson.

  `Run` then dials as today.
- **With polling off** (`-poll 0`) nothing marks the loss but the keepalive,
  so the hold does not apply: today's 30 s end.
- **Loss detection gets slower:** an idle connection is marked lost after 1 to 2 min
  at the default poll of 1 min, instead of about 30 s.
  An outage shorter than the time to the second missed poll is not marked lost at all:
  one "poll failed" warning, and nothing else.

How the kernel is kept from ending it first (D2):

- **Linux** `TCP_USER_TIMEOUT` = the hold.
  It bounds the retransmissions, and the keepalive too:
  with it set, the kernel ignores the probe count and ends the connection after
  the timeout's worth of unanswered probes.
  The keepalive keeps probing every 5 s, so a restarted Cube answers a probe with a reset within seconds.
- **Windows** `TCP_MAXRT` (seconds) and **macOS** `TCP_RXT_CONNDROPTIME`, both set to the hold.
  The keepalive count is raised to cover the hold on both,
  since neither has a timeout that governs keepalive.
- **The session's own deadline** bounds the hold on every platform.
- Both constants are in `golang.org/x/sys` v0.50.0, already a dependency.
  `keepalive_linux_test.go` checks the new values.
- **A serve flag `-cube-hold`** (default 45m, with its variable) lets lan-test and the owner's checks shorten it.

## 2. Recognising the refusal

The signs serve already has:

| Sign | Where |
|------|-------|
| The loss was serve's own: a timeout, missed polls or the hold's end, not the Cube's reset or EOF | `cubelan.Client`: the session's error |
| The dials end in a timeout, not a refusal or "no route" | `cubelan.Client`: the dial's error, `net.Error.Timeout()` |
| The Cube answers discovery from the same address | `internal/server/discovery.go`: LostSearch searches for a lost Cube at once and every minute |

Ping and ARP add nothing.
A discovery answer from the Cube's address proves the Cube is up and the path works.
Ping would need raw sockets or a sysctl.

- **When all three hold for 2 dials:** a Warn line, once, says the Cube still holds an earlier connection,
  probably serve's own from before the outage;
  that it lets go after up to about 40 min;
  and that a reboot frees it at the price of the radiograms it has not delivered.
- **Without a loss of serve's own**, for example the first dial after serve's start, the same signs read
  "another program holds the Cube":
  the MAX! software, a second serve, or an integration that connects directly.
  The reboot's confirmation says so,
  since a reboot would knock that program off, and it would likely reconnect first.
- **`GET /api/v1/cubes`** gets a field `link`:
  `held` (with `since` and `until`), `refused` (with `since` and `cause`: `own` or `other`),
  or `rebooting`.
  The word "lockout" is taken already: it is the token lockout of UiLockout.
- **The UI** shows the state in the Cube's panel in System settings → Cubes,
  and where a lost Cube shows today.
  The Reboot button is in the same panel.

## 3. Reboot: the owner's word, not automatic

Why serve should not reboot by itself after N minutes:

- A reboot drops the radiograms not yet delivered.
  Those are setpoints and configuration the owner believes are sent,
  and they are lost without a sign but the device's later state.
- After a reboot, a room discards commands until every member has transmitted,
  and the eco switch stays "not reported".
- The signs fit another program holding the Cube just as well.
  An automatic reboot would knock that program off, again and again.
- Waiting costs only visibility: the lockout ends by itself after 16 to 42 min,
  and the heating runs meanwhile.
- With the 45-min hold, a lockout after an outage shorter than the hold should not happen at all.
  What is left is rare enough for a person to decide.

What the reboot does:

- **API:** `POST /api/v1/cubes/{sn}/reboot`, capability `cubes.reboot`, seeded for no role,
  so only the owner has it until a role is given it, as for the computer's `reboot`.
  It sends `R`, the frame of `cubelan.RebootFrame`, from serve's discovery socket,
  which receives the answer (status 2).
  - With pending radiograms it answers `409`, with the count and its time,
    unless the request confirms that count.
  - It is audited.
  - Serve marks the restart as deliberate ("cube rebooting", no loss warning),
    ends any hold, and dials with its backoff until the greeting.
    On breno that took 61 s.
- **The pending count:** the H line arrives only at a connection's start and in S replies,
  so the count is the last known, with its time.
  While no connection is up, nothing is sent and the count can only fall,
  so the last known count is an upper bound.
- **UI:** a "Reboot the Cube" button in the Cube's panel (System settings → Cubes),
  shown with `cubes.reboot`.
  Its confirmation names:
  - the pending radiograms, as of when;
  - that commands are discarded for a while after the restart;
  - that another program may hold the Cube, where the cause is not serve's own loss;
  - that the Cube is gone for about a minute.
- **CLI:** `massimo netconfig -addr <ip> -reboot`, or `-sn <serial> -broadcast -reboot`.
  - It uses netconfig's UDP plumbing, its question (`-yes` skips it) and `-dry-run`.
  - The CLI has no API client, so it cannot see serve's pending count.
    It warns, and names where the count is.
  - On serve's own box the Cube's answer goes to serve's port 23272,
    so the CLI reports "sent" without an answer there.

## 4. The fake Cube

The fake gets the Cube's one-client rule:

- **With `SingleClient`:** while a session is open, a second client's SYN is dropped,
  so its dial times out as on hardware.
  On Linux this is the listen backlog cut to 0 with one connection parked unaccepted,
  set on the existing listener.
  Checked on jaguar today: the dial ended in "i/o timeout" after its full timeout,
  and dialled at once after the unlock.
- **A session that ends while the fake is silent**, the outage of `SetSilent`, stays held,
  since its FIN would have been lost in the outage.
  It stays held until the fake's own hold ends (`-hold-dead`, 41 min by default, short in tests)
  or an `R` reaches the fake's discovery responder.
  The responder and the server are joined for that.
- **Control routes** `/silent` and `/release`, and the flags of `massimo fakecube`.
- **Other platforms:** the fake closes a second connection at once.
  The lockout tests are Linux-only, as `keepalive_linux_test.go` is.
  The hold's own tests use `SetSilent` and run everywhere.
- **`make lan-test`** stages the outage with a real link drop in serve's container:
  - shorter than the hold: the session resumes, and the fake counts one connection;
  - longer than a shortened `-cube-hold`: `link` is `refused`;
    a reboot through the API follows, then the reconnect.

## 5. What goes into docs/cube-protocol.md (Block 1)

"Lessons from the hardware days", dated 2026-11-13, firmware 1.1.3:

- the outage;
- the idle connections lost at 26–30 s, while the busy ones survived;
- the lockout:
  - SYNs answered by a reset with seq 0 and no ACK, which Linux ignores;
  - 41½ min, and over 15¾ min in the reproduction;
- no answer to a SYN from the old 4-tuple;
- `R` frees the Cube: no ping at once, ping again within 43 s, reconnected after 61 s.

The earlier lesson ("Removing the IP address … the Cube needed a power cycle") is the same case.
It gets a pointer to `R`.

## Open decisions

**D1. How long serve holds a lost connection.**
- **A (recommended):** 45 min, ending early on a reset, a move,
  or 3 min of discovery answers without a resumption.
  It is just above the longest lockout measured, and the hold costs nothing while it lasts.
- **B:** 5 min.
  It covers a router's reboot, but after a longer outage the Cube refuses serve for up to about 40 min.
- **C:** no limit, until the Cube resets it.
  A Cube that drops its side without a reset would never be dialled again
  but for the 3-min rule.

**D2. Keeping the kernel from ending the held connection.**
- **A (recommended):** `TCP_USER_TIMEOUT` on Linux, `TCP_MAXRT` on Windows,
  `TCP_RXT_CONNDROPTIME` on macOS, all set to the hold, and the keepalive count raised where needed.
  The keepalive keeps probing, so a restarted Cube is noticed within seconds.
- **B:** `TCP_USER_TIMEOUT` alone.
  It is one knob, but Windows and macOS keep today's 30 s end.
- **C:** keepalive off during the hold.
  A restarted Cube would be noticed only at the next retransmission, up to 2 min later.

**D3. When the loss shows.**
- **A (recommended):** after the two missed polls, as LostCube put it.
  That is 1 to 2 min at the default poll instead of about 30 s, the same on every platform,
  and a short outage is not shown at all.
- **B:** on Linux also from the keepalive's unanswered probes (`TCP_INFO`), at about 30 s as today.
  It needs a code path of its own for one platform.

**D4. Recognising the refusal.**
- **A (recommended):** from the three signs serve has
  (a loss of its own, dial timeouts, discovery answers from the same address),
  shown as `link: refused` with the cause, in the log once, and in the Cubes panel.
- **B:** also ping and ARP.
  They need raw sockets or a sysctl for ping, and prove nothing a discovery answer does not.

**D5. The way out.**
- **A (recommended):** a reboot on the owner's word only: the API, the Cubes panel with its confirmation, and the CLI.
- **B:** also an automatic reboot after N minutes of refusal, opt-in, off by default.
  It is the same code plus a setting, but it loses radiograms unseen and fights another program.
- **C:** an automatic reboot by default. Same costs as B, for everyone.

**D6. The capability.**
- **A (recommended):** a new `cubes.reboot`, seeded for no role, as the computer's `reboot` is.
  The owner can give it to a manager without the Cube configuration of `cubes`.
- **B:** under `cubes`. No new capability,
  but whoever configures Cubes can then also drop their radiograms.

**D7. The command-line form.**
- **A (recommended):** `massimo netconfig -reboot`, direct over UDP.
  It uses netconfig's plumbing and question, and warns that it cannot see the pending count.
- **B:** a command through serve's API.
  It would show the count, but it would be the CLI's first API client,
  with a token, an address and TLS to handle.

**D8. The fake Cube's refusal.**
- **A (recommended):** the kernel's own: SYNs dropped through the backlog trick on Linux,
  so serve's dial times out as on hardware.
  The lockout tests are Linux-only, and lan-test uses the same mode.
- **B:** a test dialer of the fake that times out while locked.
  It is portable, but it bypasses the network, and lan-test would still need A.

## Blocks

1. docs: the 2026-11-13 measurements in `docs/cube-protocol.md` (Lessons).
2. fakecube: the one-client rule, a dead session held through an outage,
   its end by time or `R`, control routes and flags; Linux tests.
3. cubelan: the hold:
   - the session keeps the socket after the missed polls;
   - it resumes on the next line, and its ends are the four above;
   - the socket options per platform;
   - `-cube-hold`;
   - tests against the fake.
4. Recognising the refusal: the client's loss and dial causes;
   the server's combination with discovery;
   `link` in `GET /api/v1/cubes` and its OpenAPI;
   the Warn line; the UI text in six languages.
5. Reboot:
   - `cubelan.Reboot`;
   - `POST …/reboot`, `cubes.reboot`, the 409 with pending, the audit;
   - the deliberate restart in the client;
   - the Cubes panel's button and confirmation;
   - `netconfig -reboot`;
   - the docs (api-v1, store.md §6, ui.md, README).
6. lan-test: the short outage resumes, the long one is refused, the reboot reconnects.

Each block that touches `internal/**` or `cmd/**` carries a CHANGELOG fragment.
The test gate is `go test ./...`, `make race`, `make cross`,
`make ui-test` (Block 5) and `make lan-test` (Blocks 3, 4 and 6),
with `gofmt` and `go vet` on all three platforms.

Nothing is sent to a real Cube in any block.
A breno check (the owner's nft drop, shorter and longer than a shortened hold,
then a reboot from the panel) is the owner's, on the owner's word.

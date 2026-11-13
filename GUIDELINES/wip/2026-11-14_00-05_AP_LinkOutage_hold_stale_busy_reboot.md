# AP LinkOutage v1.0: hold, stale, busy, reboot

## TL;DR

serve holds a lost Cube connection for as long as the Cube holds its own,
so that a short outage of serve's network resumes on the same connection.
It also tells a Cube that refuses it from one that is off,
and the owner can reboot a Cube from the Cubes panel, the API or the command line.

- idle connection, link down → about 30 s → the keepalive ends it,
  and the Cube refuses serve for 16 to 42 min
  → **held** for 45 min, and it resumes when the link is back
- the loss is shown → after two missed polls, and on Linux after three unanswered
  keepalive probes (about 30 s, as today)
- the hold ends early → the Cube resets the connection, it moved,
  or it has answered discovery for 3 min without the connection resuming
- dials time out while the Cube answers discovery:
  - after serve closed the connection itself → `stale`:
    "the Cube still holds serve's old connection";
  - otherwise → `busy`: "another client holds the Cube"
- the way out → **Reboot the Cube** (UDP `R`), only on a person's word:
  - `POST /api/v1/cubes/{sn}/reboot`, capability `cubes.reboot`, seeded for no role;
  - a button in System settings → Cubes, whose confirmation names the pending radiograms;
  - `massimo netconfig -reboot`
- the fake Cube gets the Cube's one-client rule:
  a second client's dial times out, and a session that ends during an outage stays held

### Blocks

1. The 2026-11-13 measurements in `docs/cube-protocol.md`
2. The fake Cube's one-client rule, its dead session, `R` that frees it
3. The hold: the session keeps a lost connection, the socket options, `-cube-hold`
4. `stale` and `busy`: the client's link state, discovery's part, the API, the log, the UI
5. Reboot the Cube: `cubelan.Reboot`, the API route and capability, the Cubes panel, `netconfig -reboot`
6. `make lan-test` stages the outage, the refusal and the reboot

## Discussion

On 2026-11-13 the owner rebooted the FRITZ!Box.
Two idle Cube connections of breno's serve were lost, and the Cubes then refused serve
for 41½ minutes, until a UDP `R` freed one of them.
The owner asked for a proposal that keeps a Cube reachable through such an outage.
The analysis is [2026-11-13_23-49_ANALYSIS_LinkOutage_hold_refusal_reboot.md](2026-11-13_23-49_ANALYSIS_LinkOutage_hold_refusal_reboot.md).

**The owner's decisions**, 2026-11-14, `PLAN++` on the TL;DR gate:

- **During the analysis:** the owner asked why serve does not hold its side
  for the 41 minutes the Cube holds its own,
  and asked for Reboot in System settings → Cubes,
  with a warning about pending radiograms, a confirmation and a capability.
- **D1-A.** The hold is 45 min.
  It ends early on a reset, on a move,
  or after 3 min of discovery answers without a resumption.
- **D2-A.** The kernel is kept from ending the held connection:
  `TCP_USER_TIMEOUT` on Linux, `TCP_MAXRT` on Windows, `TCP_RXT_CONNDROPTIME` on macOS,
  each set to the hold, and the keepalive count raised where the hold needs it.
- **D3-B, "if not too much work, otherwise D3-A".**
  On Linux the loss is also marked from the keepalive's unanswered probes (`TCP_INFO`),
  at about 30 s as today; the two missed polls hold everywhere.
  This plan takes B: a reading of `TCP_INFO` on the poll loop's tick,
  a stub elsewhere, and a seam for the test.
- **D4-A, refined by the owner into two causes:**
  - **`busy`:** the refusal may come from another client's existing connection,
    "hence automatic reboot must be user interaction";
  - **`stale`:** a refusal of serve's own old connection
    "would only require that we closed it".

  So `stale` means: the dials time out, the Cube answers discovery,
  and serve's own side closed the last connection to this Cube,
  for whatever reason (missed polls, the keepalive, the hold's end, a deliberate reconnect).
  `busy` means the same signs without that, such as the first dial after serve's start.
- **D5-A.** Reboot only on a person's word, never automatically.
- **D6-A.** A new capability `cubes.reboot`, seeded for no role.
- **D7-A.** `massimo netconfig -reboot`, over UDP.
- **D8-A.** The fake refuses through the kernel: the SYN is dropped,
  so the dial times out as on hardware; those tests are Linux-only.

**Refined here, routine:**

- **The API word** is `link`, with the values
  `connected`, `held`, `stale`, `busy`, `rebooting` and `offline`.
  "Lockout" stays the token lockout's.
- **While held**, `Send` and `Reconnect` answer `ErrNotConnected` at once.
  A setpoint written into the held socket would reach the Cube minutes later,
  after its caller had been told it failed.
  A periodic refresh must not drop the held connection.
  `ReconnectIfMoved` still drops a connection to an old address.
- **The resumption is the first line from the Cube while held**,
  normally the L answer to a missed poll.
  A hold marked by the keepalive (D3-B) has no poll in flight, so it writes one `l:`.
- **A reboot is a deliberate end**, as `Reconnect` is: no loss warning,
  and `link` is `rebooting` until the greeting or for 3 min.
- **`-cube-hold`** is a serve flag (default 45m; 0 is no hold),
  with the variable `MASSIMO_CUBE_HOLD`.
  It exists for lan-test and the owner's checks.
  With polling off (`-poll 0`) there is no hold.

**Names chosen here, routine:**

- in `cubelan`: `Options.Hold`, `Options.HoldHeard`, `Client.Heard`, `Client.Link`,
  `Client.Rebooting`, `Reboot`;
- in `fakecube`: `Server.SingleClient`, `Server.HoldDead`, `Server.Release`;
- the fragments `claude-link-outage-*`;
- the branch `claude/link-outage`.

**Not in this plan:**

- An automatic reboot (D5-B and D5-C).
- A command-line client of serve's API (D7-B).
- Ping or ARP probes (D4-B).

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-14.
The gate offering it stated:
- the plan is registered on main, the registration pushed to main;
- Blocks 1 to 6 are committed on `claude/link-outage`, with their CHANGELOG fragments;
- the branch is pushed and its pull request opened;
- the run stops at the completion gate.

There is no merge and no retirement, and nothing is sent to a Cube.

## Analysis

The analysis file holds the measurements and the reasoning.
In short:

**Today** (`internal/cubelan/client.go`):

- **The keepalive:** `keepAlive`, 15 s idle, 5 s apart, 3 probes,
  ends an idle connection whose link is gone about 30 s later, with ETIMEDOUT.
- **The missed polls:** two in a row (`pollMisses`) end the session too.
- **The Cube's side:** it keeps the session, and drops the SYNs of serve's next dials,
  answering with a reset with seq 0 and no ACK, which SYN-SENT ignores.
  Each dial then ends in "i/o timeout".
- **The busy connections survived:** a poll's unacknowledged bytes stop the keepalive,
  and Linux retransmits for about 15 min.

**The hold costs almost nothing:**

- The devices heat on their own programmes while the Cube is unreachable.
- **The Cube kept its side:** TCP's retransmissions reach it, at most 2 min apart,
  once the link is back, and the session resumes.
- **The Cube dropped its side:** it answers the next segment with a reset,
  and serve dials at once.
- **The Cube moved:** LostSearch finds it, and `ReconnectIfMoved` drops the held connection.
- **A Cube that drops its side without a reset**, or a source address of ours that is gone,
  never resumes: the 3-min rule ends that hold.

**Linux `TCP_USER_TIMEOUT`** bounds both the retransmissions and the keepalive.
With it set, the kernel ignores the probe count and ends the connection
once the timeout has passed with probes unanswered.
`TCP_INFO`'s `tcpi_probes` counts the keepalive probes the Cube has not answered,
and it falls back to 0 when an ACK arrives.

**The fake's refusal, checked on jaguar on 2026-11-13:**

- `listen(fd, 0)` on a listening socket, with one connection parked unaccepted,
  makes the next dial end in "i/o timeout" after its full timeout;
- after the parked connection is accepted and the backlog restored,
  the next dial connects at once.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `docs/cube-protocol.md` | 1, 5 | the Lessons of 2026-11-13; `R` as the way out |
| `internal/cubelan/fakecube/fakecube.go`, `listen_linux.go`, `listen_other.go`, `control.go`, `discovery.go` | 2 | the one-client rule, the dead session, `Release`, `R`, the control routes |
| `cmd/massimo/fakecube.go` | 2 | `-single-client`, `-hold-dead` |
| `internal/cubelan/client.go`, `hold_*.go`, `keepalive_linux_test.go`, `client_test.go`, `hold_test.go` | 3, 4 | the hold, the socket options, `TCP_INFO`, `Heard`, `Link`, `Rebooting` |
| `internal/server/…` (cubes API, discovery, routes, audit, auth) | 3–5 | `-cube-hold`, `Heard`, `link`, the reboot route, `cubes.reboot` |
| `cmd/massimo/serve.go`, `netconfig.go` | 3, 5 | `-cube-hold`; `-reboot` |
| `internal/server/ui/…` | 4, 5 | the link state, the Reboot button and its confirmation, six languages |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/store.md`, `docs/ui.md`, `README.md` | 3–5 | the documents of each |
| `tools/lan-test/…` | 6 | the outage, the refusal, the reboot |
| `CHANGELOG.d/…/claude-link-outage-*.md` | 2–5 | one fragment per change a user sees |

## Implementation Plan

Each block ends with the contract's test gate:
- `MASSIMO_ENV_FILE=/dev/null go test ./...`;
- `gofmt -l .` empty, and `go vet ./...` for linux, windows and darwin;
- `make cross`;
- `go test -race -count=1` over the race packages for Blocks 3 to 5;
- `make ui-test` for Blocks 4 and 5;
- `make lan-test` for Blocks 3, 4 and 6.

Commits are functional, each citing `AP LinkOutage v1.0, Block <n>`.
Nothing is sent to a real Cube.

### Block 1: The measurements

1. `docs/cube-protocol.md`, "Lessons from the hardware days": one entry, 2026-11-13,
   firmware 1.1.3. It holds:
   - the outage (link down and up four times in about a minute);
   - the idle connections lost at 26–30 s by the keepalive, the busy ones kept;
   - the dead session kept by the Cube:
     - SYNs answered by a reset with seq 0 and no ACK, which Linux ignores in SYN-SENT;
     - 41½ min, and over 15¾ min in the reproduction with an nft drop of 26 s;
   - no answer to a SYN from the old 4-tuple;
   - `R` frees the Cube: no ping at once, ping again within 43 s, serve reconnected 61 s after it;
   - a reboot drops undelivered radiograms.
2. The earlier entry ("Removing the IP address … needed a power cycle") is the same case.
   It gets a pointer to `R` as the way out.

**Verification:** the guidelines' `check` for the document's lines.
No shipped path, so no fragment.

### Block 2: The fake Cube's one-client rule

1. `fakecube.Server.SingleClient`: while a session is open, or a dead one is held,
   a new client is refused.
   - **On Linux** the accept loop stops accepting,
     the listener's backlog is cut to 0, and one connection is parked unaccepted,
     so the next SYN is dropped and the dial times out.
     The backlog and the loop come back with the release.
   - **Elsewhere** a second connection is accepted and closed at once.
2. **A dead session:** a client connection that ends while the fake is silent
   (`SetSilent`) is a FIN lost in the outage.
   The fake keeps it as held, and refuses as in 1,
   until `HoldDead` has passed (default 41 min), `Release` is called,
   or an `R` reaches the fake's discovery responder.
   For that, the responder gets an `OnRestart` hook,
   which `massimo fakecube` and the tests join to the server's `Release`.
3. **Control routes** `POST /silent`, `DELETE /silent` and `POST /release`.
4. **`massimo fakecube`** gets `-single-client` and `-hold-dead`.
5. **Tests:**
   - a second client's dial times out while a client is connected
     (Linux; the dial timeout short);
   - a session closed while silent is held, and `Release` and `R` free it;
   - `HoldDead` frees it;
   - a session closed while not silent frees the slot at once.
6. The fragment `CHANGELOG.d/Added/claude-link-outage-fakecube.md`.

**Verification:** the test gate.

### Block 3: The hold

1. **`cubelan.Options`:**
   - `Hold`: how long a lost connection is held; 0 is 45 min, negative is none.
     No hold with `PollInterval` 0.
   - `HoldHeard`: how long discovery may answer without a resumption; 0 is 3 min.
2. **The session**, in `client.go:session`, with Run's loss bookkeeping (`lost`, `warned`)
   moved into a state that both share:
   - **Entering the hold:** at the two missed polls, or on Linux at `tcpi_probes` ≥ 3
     (read every `keepAlive.Interval` while no hold runs), the session does not return.
     It reports the loss as Run does today:
     the warning "cube connection lost" (with `holding_until`), `Lost`, `EventDisconnected`.
     It clears `online`, stops polling,
     and writes one `l:` where no poll is in flight (the keepalive case).
   - **Resuming:** the first line read while held ends the hold:
     "cube connection re-established" with `down_for`,
     `EventConnected` with `CauseLoss`, `online` and not `Lost`, and polling again.
   - **Ending:** the hold ends:
     - on a read error, the Cube's reset or close;
     - at `Hold`;
     - after `HoldHeard` of discovery answers (`Heard`) with no resumption;
     - on `ReconnectIfMoved`.

     Run then dials, with no second loss warning.
   - **While held:** `Send`, `trySend` and `Reconnect` answer `ErrNotConnected`.
3. **The socket options**, through the default dialer's `Control`:
   - **Linux:** `TCP_USER_TIMEOUT` = `Hold`, the keepalive 15 s/5 s/3 as today.
   - **Windows:** `TCP_MAXRT` = `Hold` in seconds, and the keepalive 15 s/15 s
     with a count covering `Hold` (at most 255).
   - **macOS:** `TCP_RXT_CONNDROPTIME` = `Hold` in seconds, and the same keepalive.
   - **No hold:** today's options.
4. **`TCP_INFO`** in a Linux file, a stub elsewhere,
   and an unexported seam in `Client` for the test.
5. **serve:** `-cube-hold` and `MASSIMO_CUBE_HOLD` into `cubelan.Options.Hold`,
   in `README.md`, `cmd/massimo/serve.go`, and the server's options and settings where the poll is.
6. **Tests:**
   - a silent fake: the loss after two polls; with the fake audible again,
     the resumption on the same connection (`Connections() == 1`),
     one loss and one re-established line;
   - the hold's end at `Hold`, then a reconnect;
   - the end by `HoldHeard`;
   - the fake closing during the hold ends it at once;
   - `Send` and `Reconnect` during the hold;
   - the keepalive seam enters the hold and writes one `l:`;
   - `keepalive_linux_test.go` reads `TCP_USER_TIMEOUT`;
   - a Windows test reads `TCP_MAXRT`, where `getsockopt` allows.

   The existing loss tests stay green.
7. The fragment `CHANGELOG.d/Changed/claude-link-outage-hold.md`.

**Verification:** the test gate, `make race`,
and `make lan-test` (its checks still pass; the outage check comes in Block 6).

### Block 4: `stale` and `busy`

1. **The client** records:
   - whether serve's side ended the last connection
     (missed polls, the keepalive, the hold's end, a deliberate close,
     any end that is not the Cube's EOF or reset);
   - the dials that timed out in a row since then (`net.Error.Timeout()`);
   - when discovery last heard the Cube from its address (`Heard`).
2. **`Client.Link()`** gives the state, `since`, and for `held` `until`:
   - `connected`;
   - `held`;
   - `rebooting`;
   - `stale`: two or more dials in a row timed out, the Cube was heard within 150 s,
     and serve's side closed the last connection;
   - `busy`: the same signs otherwise;
   - `offline`.
3. **The server:** the discovery run that hears a configured Cube from its client's address
   calls `Heard`. `GET /api/v1/cubes` (and the single Cube's route) carries `link`,
   in `docs/api-v1.md` and the OpenAPI file.
4. **The log:** one Warn when a Cube becomes `stale` or `busy`, naming what it means:
   - `stale`: the Cube still holds serve's old connection, lets go after up to about 40 min,
     and a reboot frees it at the price of its undelivered radiograms;
   - `busy`: another client holds the Cube.
   Info when it clears.
5. **The UI:** the Cube's panel in System settings → Cubes, and where a lost Cube shows,
   names the state in words:
   - "Connection held since …, until …";
   - "Holds serve's old connection";
   - "Busy with another client";
   - "Rebooting".
   The keys are in all six languages, FR/IT/NL/PL marked machine-translated.
6. **Tests:**
   - the client's `Link` across the hold, `stale` after a silent outage longer than the hold
     with a single-client fake (Linux), and `busy` with another client holding the fake;
   - the server's `link` in `GET /api/v1/cubes`;
   - the Warn once;
   - a UI test of the panel's text.
7. The fragment `CHANGELOG.d/Added/claude-link-outage-link.md`.

**Verification:** the test gate, `make ui-test`, `make lan-test`.

### Block 5: Reboot the Cube

1. **`cubelan.Reboot(ctx, pc, addr, serial)`:** `R` to the Cube's port 23272;
   the status (2) when an answer arrives within a second.
2. **The client's `Rebooting(d)`:** a deliberate end of a live or held connection,
   with the Info "cube rebooting" and no loss warning.
   `link` is `rebooting` until the greeting or `d` (3 min).
3. **`POST /api/v1/cubes/{sn}/reboot`**, capability `cubes.reboot`:
   - `409` with `pending_radiograms` and `budget.updated_at` when radiograms are pending,
     unless the body confirms that count (`{"pending": n}`, n ≥ the count);
   - otherwise `R` through serve's own UDP socket, then `Rebooting`, then `202`
     with `{"sent": true, "status": 2 | null}`;
   - audited. The capability goes into `auth.go`, `docs/store.md` §6 (seeded for no role),
     `docs/api-v1.md`, the OpenAPI file, and the roles editor's labels.
4. **The UI:** "Reboot the Cube" in the Cube's panel (System settings → Cubes),
   shown with `cubes.reboot`, through the UI's existing confirmation pattern.
   The text names:
   - the pending radiograms and when they were counted;
   - that rooms discard commands for a while after it;
   - that the Cube is gone for about a minute;
   - for `busy`, that another client holds the Cube and may take it back.
5. **`massimo netconfig -addr <ip> -reboot`**, and `-sn <serial> -broadcast -reboot`:
   it asks first (`-yes`), and `-dry-run` prints the frame.
   It warns that it cannot see serve's pending count, and where that count is,
   and reports the answer or "sent, no answer".
   Its help, and `README.md`.
6. **`docs/cube-protocol.md`:** `massimo` sends `R` on a person's word, from where.
   `docs/ui.md` describes the button.
7. **Tests:**
   - `Reboot` against the fake responder (`Reboots()`);
   - the route: capability, 409 with pending, the confirm, the audit row,
     and `link` going `rebooting` and then `connected`
     against a single-client fake that `R` frees;
   - `netconfig -reboot` with `-dry-run` and against the fake;
   - a UI test of the button, its confirmation and the request.
8. The fragment `CHANGELOG.d/Added/claude-link-outage-reboot.md`.

**Verification:** the test gate, `make ui-test`.

### Block 6: lan-test

1. The fake Cube of the LAN test runs `-single-client`.
2. **A check of the outage:** serve's container loses its link (`ip link set … down`)
   for about 40 s, shorter than a `-cube-hold` of 2 min.
   The session resumes: the fake counts one connection,
   and serve's log has one loss and one re-established line.
3. **A check of the refusal:** an outage longer than the hold.
   `link` becomes `stale`, a reboot through the API
   (with a token of the owner's) frees the fake, and `link` returns to `connected`.
4. Its README.

**Verification:** `make lan-test`, twice.

## Proposed commit messages

The registration, a commit of its own on main:

```
chore(aps): +LinkOutage v1.0

A short outage of serve's network left two Cubes refusing serve for 41½ minutes on 2026-11-13, until a UDP R freed one. The owner decided on 2026-11-14 that serve holds a lost connection as long as the Cube does, tells a Cube that holds serve's old connection (stale) from one another client holds (busy), and lets a person reboot a Cube. PLAN++ LinkOutage, with D3-B, the owner's D4 and the rest as recommended.

Key changes:
- GUIDELINES/wip/2026-11-14_00-05_AP_LinkOutage_hold_stale_busy_reboot.md: the plan, six blocks
- GUIDELINES/wip/2026-11-13_23-49_ANALYSIS_LinkOutage_hold_refusal_reboot.md: the analysis it rests on

File-revision-v1: GUIDELINES/wip/2026-11-14_00-05_AP_LinkOutage_hold_stale_busy_reboot.md::v1.0::2026-11-14::All::the first revision, by PLAN++ LinkOutage
File-revision-v1: GUIDELINES/wip/2026-11-13_23-49_ANALYSIS_LinkOutage_hold_refusal_reboot.md::v1::2026-11-13::All::the analysis of the TL;DR gate
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits are written at each block, citing `AP LinkOutage v1.0, Block <n>`.

## Change History

- **v1.0** (2026-11-14): first revision, by the owner's `PLAN++`.
  It takes D1-A, D2-A, D3-B, D4 split into `stale` and `busy`, and D5-A to D8-A.

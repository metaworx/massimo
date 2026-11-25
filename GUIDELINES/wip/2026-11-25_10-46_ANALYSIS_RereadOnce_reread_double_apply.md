# ANALYSIS RereadOnce: A re-read's C line applied twice

## TL;DR

The finding holds.
Since 2026-11-07 the client applies every C reply itself, before `Send` returns;
the older applies in `cubeio.Writer.Reread` and the proxy's `c:` only repeat it.
One re-read: two `config` events, two log lines, two store rows.

- `handle` applies L and C replies (client.go:593-606) → `ApplyLocal` in cubeio.go:107 and proxy.go:445 redundant
- no case where the client skips it: unknown device, greeting, late reply all applied
- cost: one more stream event, debug line, store row per re-read; unreadable C line warned twice; no double confirmation
- latent: `ApplyLocal` goes through the reply match → a C line applied while another `c:` waits answers it with the wrong device (shown by a test); the queue's 200 ms gap hides it
- fix: drop both applies (D1-A), `ApplyLocal` answers no `Send` (D2-A); Go suite green with both as an overlay

## What happens

### The client applies the reply

- Every queued command goes to the Cube through `engine.Queue.execute`,
  `q.gw.Send` (engine/queue.go:492).
  The server's gateway is the client itself:
  `slotsGateway` embeds `*cubelan.Client` (server/slots.go:104, server/runtimes.go:138);
  `cmd/massimo/backup.go:202` hands the client to the queue directly.
  `Writer.Client` is the same client.
- `Client.handle` (cubelan/client.go:572) takes the reply for the waiting `Send`
  (579-592).
  Only a reply that is neither L nor C goes to `Send` without the snapshot (593-596).
  An L or C reply is applied (606), its events emitted (621-626),
  and only then handed to `Send` (627-629).
  `Send`'s doc says so: "An L or C reply is in the snapshot,
  its events emitted, when Send returns it" (707-708).
- The snapshot's C apply (cubelan/snapshot.go:249-263) stores the line,
  adds a device it did not know (`noteLocked`, 260-262),
  clears it from the forgotten ones (256),
  marks it fresh in a greeting (257-259),
  and always returns one `EventConfig`, changed or not.

### The second apply

- `Writer.Reread` (cubeio/cubeio.go:104-109) waits for the `c:` and then calls
  `w.Client.ApplyLocal(res.Replies[0])` (107).
- The proxy's `passthrough` (proxy/proxy.go:441-448) does the same for a client's `c:`.
  The proxy answers a `c:` for a device it knows from the snapshot
  (`local`, proxy.go:345-355),
  so this path is a device the server does not know yet, one just paired.
- `ApplyLocal` (cubelan/client.go:869-873) calls `handle` again:
  the snapshot takes the same line a second time and emits a second `EventConfig`.

### Is the second apply needed anywhere?

None of the cases below needs it:

| Case | What the client does |
|------|----------------------|
| A device not in the snapshot yet | applied and added (snapshot.go:255-262) |
| The reply during a greeting | applied, marked fresh (257-259); the case of 42ec70b8 |
| A reply after `Send` timed out | pending cleared (client.go:770); the line comes as one of the Cube's own and is applied; `Reread` gets the error and applies nothing |
| A reply for a `t:` of several devices (581) | only `t:` waits for more than one line |
| A C line that cannot be read | kept as received with a warning (608-610), no event; the second apply warns again |
| Another gateway than the client | none in production; `Writer.Client` must be the queue's client anyway |

## Why it was added

- `60073f8e` (2026-11-04), the first client:
  `handle` gave a reply to `Send` and returned;
  only an L reply fell through to the snapshot.
  A C reply reached the snapshot nowhere.
- `9a8aa6ac` (2026-11-04), the configuration API:
  `refreshConfig` re-read a device after a write and applied the reply itself.
  It was the only apply then.
- `160cc703` (2026-11-05) moved it into `cubeio.Writer.Reread`.
- `07ca4dd6` (2026-11-05), the proxy's pairing:
  "A C line passed through for a device the server did not know is added to its copy".
  For the same reason: the client did not apply it.
- `42ec70b8` (2026-11-07), AP ReviewFixes, finding 17:
  a re-read answered by a greeting's C line lost the device at the greeting's end,
  so the client now applies a C reply as an L reply.
  From here on both later applies repeat the client's.
- `0366d9ed` (2026-11-08), AP ReplyOrder:
  the apply comes before the hand-off.
  Its message notes "cubeio's re-read applies the C line itself" -
  the overlap was seen, and left.
- `24b6df9b` (2026-11-07) gave the re-read its request's ID; it did not touch the apply.

## What it costs

Each re-read, in `serve`, doubles what the pump does for a `config` event
(server/server.go, the pump from 1029):

- **The stream:** a second `config` event (1160), and a second slot of the replay ring.
- **The log:** a second line (1156) and a second row in the store,
  which keeps every level (`rt.remember`, 1157; store/events.go:208).
  The second line has no changes and is at debug (`detail`, 1230-1250),
  even where the first one was at info with the changes.
- **The page** (server/ui/app.js:1112-1121): the event twice in its log,
  shown at level debug only (ui/src/log/store.ts:98);
  `updateDevice` twice, which is cheap;
  for a just-paired device not yet in the lists, `loadRooms` twice,
  since the second event comes before the first load's answer.
  The reload for new presets runs once: the second event's `changed` is empty.
- **The warnings:** an unreadable C line warns twice (client.go:609).

What does not double:

- the configuration confirmation: `checkConfig` (confirm_config.go:258)
  removes a write once it is shown, so the second call finds nothing;
- `configChanged` (etag.go:264): the second is empty,
  so `publishVirtualConfig` (server.go:1176) runs once;
- `publishVWTs` (1173; vwt.go:297) and `checkMismatches` (1185)
  publish and log only what changed;
- no event is built from an older snapshot: the pump reads the snapshot at its turn,
  so both events show the same.

How often: every re-read.
That is a configuration or programme write through the API
(`Writer.Do` with re-reads, config_api.go:824, 1158, 1203; `Reread`, 452, 938),
each step of the follow-up, several per write (followup.go:211),
a repair (repair_api.go:395), a paired device (server.go:1149),
and a proxy client's `c:` for an unknown device.
A restore re-reads nothing (backup/plan.go:582).
Nothing more goes to the Cube: `c:` is a LAN command, read once either way,
and it costs no duty cycle.

The flake of `TestSameConfigAtDebug` (server/logdetail_test.go:255) came from it:
the second line came after the test's mark.
Its `pumpSync` (286) holds it now.

### Two latent faults beyond the duplicate

1. **`ApplyLocal` can answer a waiting `Send`.**
   It goes through `handle` and its reply match (client.go:579-592).
   A C line applied while another `c:` waits for its reply is taken as that reply.
   A scratch test shows it: `c:0a0108` returned the C line of `0a010d`.
   That `Send`'s own reply then comes as a line of the Cube's own,
   so the snapshot ends right.
   But a second `Reread` would apply the wrong line once more,
   and a proxy client would get the other device's C line as its answer.
   In `serve` the queue resolves the ticket and then waits its gap,
   200 ms by default (engine/queue.go:238, 379-383),
   while the apply runs microseconds after `Wait` returns: it is rare.
   Tests that run the queue with a gap of 1 ms (cubeio_test.go:36,
   proxy_test.go, slots_test.go:38) leave a narrower window.
2. **The second apply is of a line already read.**
   If a greeting began in between - a reconnect through the queue, after the gap -
   it would mark the device fresh with the previous connection's line,
   and could overwrite the greeting's own.
   A device deleted meanwhile would survive the greeting's end.
   This needs the apply to come later than the gap and a reconnect: it is theoretical.

## How a test shows it

Four scratch tests, run against the fake Cube with `go test -overlay`,
the tree left untouched
(their sources were scratch, in the session's working copy alone):

| Test (package) | What it counts | Today | Without the second apply |
|----------------|----------------|-------|--------------------------|
| `TestScratchRereadOneConfigEvent` (cubeio) | the client's `EventConfig` of one `Reread` | 2 (seq 17, 18) | 1 |
| `TestScratchRereadOneConfigLine` (server) | the log's `config` lines of one re-read, any level, after `pumpSync` | 2 (both debug, no changes) | 1 |
| `TestScratchProxyCOneConfigEvent` (proxy) | the client's `EventConfig` of a LAN client's `c:` for a just-paired device | 2 | 1 |
| `TestScratchApplyLocalAnswersAWaitingSend` (cubelan) | what a waiting `c:` returns when another device's C line is applied | the other device's line | still waiting (with D2-A) |

The cubeio test drains the greeting's events, calls `w.Reread(ctx, 0x0a010a)`,
and counts the config events of that device until 300 ms pass without one.
The server test takes `logMark`, re-reads `0a010d`, calls `pumpSync`,
and counts with `logLines` the lines of kind `config` of that device.
The cubelan test speaks the greeting over a `net.Pipe`,
starts `Send(c:B)`, reads the `c:` on the Cube's side without answering,
and calls `ApplyLocal` with the C line of A.

With both applies removed, `go test ./internal/... ./cmd/...` passed,
and with D2-A added it passed again;
`go test -race` passed for cubeio, proxy, cubelan and engine;
`go vet` was clean for the three packages changed.
`TestProxyPairing` still finds the paired device in the server's copy:
the client's own apply puts it there before the proxy answers.

## Open decisions

**D1 - How the double apply ends**

- **A (recommended):** remove `ApplyLocal` from `Reread` and from the proxy's `c:`;
  the client's `Send` is the one apply.
  About ten lines fewer and three tests; the full Go suite passed with it.
- **B:** the snapshot emits no event for a C line equal to the one it holds.
  It changes the event of every C line:
  a greeting's would need an exception, the log's reference is set from them,
  and an unchanged re-read's debug line, which `TestSameConfigAtDebug` checks, goes.
  It still applies twice, and leaves the fault of D2.
- **C:** the pump drops a `config` event whose line equals the last one it saw.
  One more map per Cube; the client's events, the CLI and the tests still see two.

**D2 - `ApplyLocal` and a waiting `Send`**

- **A (recommended):** `ApplyLocal` folds its line into the snapshot without the reply match;
  `handle` is split into the routing and the fold.
  About 10 lines and one cubelan test.
  After D1-A its only production caller is the metadata mirror,
  whose M lines no `Send` waits for, so this is hardening for the next caller.
- **B:** leave it, and say in its doc that it must not be given a line a `Send` waits for.
  Nothing to change now; the trap stays.

**D3 - The tests**

- **A (recommended):** one test per path, each failing on main today:
  cubeio (the events of one `Reread`), proxy (a `c:` for an unknown device),
  server (the log lines of one re-read, any level),
  and with D2-A cubelan (an applied C line answers no waiting `c:`).
- **B:** the server test alone; the proxy's path then has no count of its own.

**D4 - `TestSameConfigAtDebug`**

- **A (recommended):** keep its `pumpSync` and rewrite the comment above it:
  the wait is the rule of docs/testing.md, "The pump",
  before a test changes the Cube again, double apply or not.
- **B:** remove the `pumpSync` with the comment.
  The test then relies on one line per re-read, which is more fragile.

**D5 - The plan's name**

- **A (recommended):** `RereadOnce`, the outcome; no plan has used it.
- **B:** `DoubleApply`, the finding; no plan has used it either.

## Draft of the plan's blocks

For **AP RereadOnce** (D5-A):

- **Block 1 - A re-read applies its C line once** (D1-A, D3-A, D4-A):
  `Reread` without `ApplyLocal`, its doc saying the client's `Send` applies the reply;
  the proxy's `c:` block (proxy.go:441-448) removed;
  the cubeio, proxy and server tests;
  `TestSameConfigAtDebug`'s comment;
  a `CHANGELOG.d/Fixed/` fragment
  ("A configuration re-read is one `config` event and one log line, not two").
  *Verification:* the three tests fail on main (two each) and pass;
  `TestProxyPairing`, `TestSameConfigAtDebug` and `TestDoRereadsConfig` pass;
  the gate (`make ui`, `go test ./...`, `make race`, `make cross`),
  `gofmt -l .` empty, `go vet` for linux, windows and darwin.
- **Block 2 - `ApplyLocal` answers no `Send`** (D2-A):
  `handle` split into the routing and the fold, `ApplyLocal` folding only;
  its doc names the metadata mirror and says it answers no `Send`;
  the cubelan test.
  *Verification:* the test fails on main (the waiting `c:` takes the applied line)
  and passes; `make race` for internal/cubelan; the gate as in Block 1.
- **At the retirement:** the entry in docs/roadmap.md, item 12, "Small ones", is pruned.

## Not verified

- `make ui-test`: the page now gets one `config` event per re-read.
  Its reloads come from the first event, so no change is expected; not run.
- `make race` for internal/server, `make cross`,
  and `go vet` for windows and darwin: not run.
- Nothing was sent to a real Cube; the finding is about the server's bookkeeping alone.

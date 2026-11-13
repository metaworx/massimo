# AP ProxyRetry v1.0: retry failed proxy listeners

## TL;DR

A Cube's proxy listener that fails to listen is logged once and forgotten,
so a proxy address the host takes on later stays dead until `serve` restarts;
each configured listener gets a slot that retries with a backoff,
and the API and the UI show a listener that does not listen, and why.

- failed listen → logged, dropped → retried: 1 s, doubling, at most 30 s,
  until it listens or is stopped
- one path for the API's proxy address, `-proxy`,
  and a listener whose accept fails later
- logging → Error at the first failure and at a new error, Info once it listens,
  Debug per try
- `GET /api/v1/cubes` → `proxy_failing`; `GET /api/v1/system` → `proxies`,
  every listener
- UI → a warning under the Cube's "Proxy address";
  a "Cube proxies" row and a warning badge in System Info
- each block's documents in its own commit; Block 4 only for what is left

### Blocks

1. The retrying listener: slots, backoff, logging; tests; `docs/proxy.md`,
   `README.md`; the fragment
2. The API: `proxy_failing` and `proxies`; tests; `docs/api-v1.md`, OpenAPI
3. The UI: `noticeEl` in `components.js`; the Cube's warning,
   System Info's row and badge; `docs/ui.md`; `make ui-test`
4. The remaining general documents

## Discussion

Seen on 2026-11-13 on a Raspberry Pi running the Debian package:
a Cube's `proxy_address` was set, through `PUT /api/v1/cubes/{sn}`,
to an address the host did not carry yet.
`serve` logged at every start

```
level=ERROR msg="proxy listener not started" source=server addr=<addr>:62910 serial=<sn> error="listen tcp <addr>:62910: bind: cannot assign requested address"
```

and after the address was added to the interface the proxy stayed down:
`GET /api/v1/cubes` showed no `proxy_listening`.
The workaround was to change the proxy address back and forth,
or to restart `serve`.

**The owner's decisions**, 2026-11-13, `PLAN++` on the TL;DR gate,
taking its recommendations:

- **D1-A.**
  Every failure to listen is retried, not only `EADDRNOTAVAIL`:
  a port still held by an old `serve` during a restart is the same case,
  and no per-platform error test is needed.
- **D2-A.**
  The backoff starts at 1 s and doubles up to 30 s, without end:
  an address added later is taken within 30 s.
- **D3-A.**
  Error at the first failure and whenever the error changes,
  Info once it listens (with the tries it took), Debug for each try:
  a listener failing for hours does not fill the stored log.
- **D4-A.**
  No `IP_FREEBIND` on Linux: a mistyped address would show as listening,
  and nobody could reach it.

**The owner's instructions** with that signal:

- **Block 3** was to build on the error display the "Web UI updates" session is
  making; this session asked it, on 2026-11-13,
  for the details and for reusable components.
  Its answer, the same day:
  that display is an evaluation without a plan (AP TileBadges, room tiles only,
  if the owner signals it), nothing to build on;
  what is on main covers both places -
  `sectionBadges(card, [badgeEl(text, "warn")])` for a section's badge (as
  `update.js` does for pending maintenance),
  the `.notice` class for a lasting inline warning, `ageEl(iso)` for "since".
  It suggested the one reusable piece to add here, in `components.js`:
  `noticeEl(text, { since, next } = {})`, a `p.notice` with `ageEl(since)`
  and the next try in its title; TileBadges would then build on it.
  Block 3 does that.
- **Documents move with the code** (the contract):
  each block carries its own documents in its commit;
  Block 4 is only for the general documents that belong to no block.

**Refined here, routine:**

- **What a slot is:**
  the `proxyRun` of today, kept from the first try:
  its state (trying or listening), the last error, since when it fails,
  the next try, and the number of tries.
  `s.proxies` holds every slot; what lists listeners - `proxyListening`,
  `listeners()` for System Info, the discovery replies -
  takes the listening ones.
  `proxySerial` keeps every slot,
  as it already counts a proxy "which may be starting again".
- **The proxy itself** (`proxy.New`) is made once per slot;
  each listen that succeeds is served by it (`ServeTCP`),
  and added to the responder, and removed from it when that listener fails.
- **A listener whose accept fails later** (`proxy listener failed`) goes back to trying,
  where today `dropProxy` drops it for good.
- **`startProxy`** still binds once at once,
  so that a `PUT` answers with the outcome of the first try;
  it returns whether the listener listens now.
- **`startCubeProxy`'s rule** "no proxy address and other Cubes served:
  no proxy" counts slots, failing ones too, as it counts listeners today.
- **The retry's delays** are an `Options` field for the tests (`ProxyRetry`),
  zero meaning the defaults; `listenTCP` a package variable, so that a test can
  return `EADDRNOTAVAIL` on every platform.
- **`next_try`** is the time of the next try, not a countdown.
  The UI shows it as a time in the notice's tooltip, not "in 20 s":
  `refreshAges` runs once a minute, and the backoff is 1-30 s.

**Names chosen here**, routine:
`proxyRun` keeps its name; `proxyFailure` (the API's failing listener),
`proxyStatus` (System Info's listener); the branch `claude/proxy-retry`.

**Not in this plan:**
noticing an address the moment it appears (netlink, `NotifyAddrChange`):
per platform, and 30 s is short enough.

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-13, whose gate stated:
the plan registered on main and pushed;
Blocks 1-4 on `claude/proxy-retry`, one or more commits each,
with their fragment and gated on the test gate and the lint;
the branch pushed and a pull request opened;
no merge, no retirement, nothing sent to real hardware.

## Analysis

**Today** (`internal/server/runtimes.go`):

- `startProxy(pl)` resolves the runtime,
  calls `net.Listen("tcp", pl.Addr)` once,
  and on an error logs `proxy listener not started` and returns `false`.
  Nothing holds the listener after that.
- Only `updateCube` (`cubes_api.go`) starts it again, and only where
  `ProxyAddress` or `ProxySerial` changed.
- `Run` starts the `-proxy` listeners (`s.opts.Proxy`) the same way:
  same gap, and no route at all to start them again.
- A listener whose `ServeTCP` fails later is taken out by `dropProxy`, for good.
- `proxyListening` and `listeners()` read `s.proxies`,
  which holds only listening ones;
  so the API has no way to say a configured listener does not listen.

**After Block 1:**
`s.proxies` holds a slot for every listener configured, listening or trying;
`stopProxies` cancels a slot that is trying as well as one listening,
and waits for its goroutine; a slot's goroutine loops: listen; on an error,
record it, log as D3-A says, wait the backoff or the end of its context;
on success, add to the responder, serve,
and on a later failure remove from the responder and go back to trying.

**The UI today:**
the Cube's form shows "Listening on …" (`cubes.listening`) when
`proxy_listening` has entries, nothing otherwise;
System Info lists the interfaces the API or a proxy listens on,
from `listeners()`.

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `internal/server/runtimes.go`, `server.go` | 1 | slots, the retry loop, `ProxyRetry`, `listenTCP`; `Run`'s `-proxy` listeners through it |
| `internal/server/discovery.go`, `system_api.go` | 1 | the listening slots only, where a listener is listed |
| `internal/server/proxy_retry_test.go` | 1 | the tests of the retry |
| `docs/proxy.md`, `README.md` | 1 | a listener that cannot listen is retried |
| `CHANGELOG.d/Fixed/claude-proxy-retry.md` | 1 | the fragment |
| `internal/server/server.go`, `system_api.go` | 2 | `proxy_failing` on a Cube, `proxies` in the system's view |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 2 | the two fields |
| `internal/server/ui/components.js`, `cubes.js`, `update.js`, `partials/system-info.html`, `style.css`, `i18n/*.json` | 3 | the warning, the row, the badge |
| `docs/ui.md` | 3 | what the two places show |
| `packaging/README.md` | 4 | where a general note is due: an address the host takes on after `serve` starts |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over the race packages,
and for Block 3 `make ui-test`.
Tests run against the fake Cube; no real serial or address in code or documents,
placeholders such as `KEQ0000000` and the documentation addresses.
Every commit cites `AP ProxyRetry v1.0, Block <n>`.

### Block 1: The retrying listener

1. `server.go`:
   `Options.ProxyRetry` (first and longest delay; zero the defaults,
   1 s and 30 s);
   the comment of `Proxy` says that a listener that cannot listen is retried.
2. `runtimes.go`:
   `proxyRun` gains its state - `listening`, `err`, `failingSince`, `nextTry`,
   `tries` - under `s.proxyMu`;
   `listenTCP` the package variable for `net.Listen`.
   `startProxy` makes the slot and the proxy, tries once at once,
   then starts the slot's goroutine (serve, or wait and try again);
   it returns whether it listens.
   `dropProxy` goes; a failure in `ServeTCP` returns the slot to trying.
   `stopProxies` stops trying slots too.
   `proxyListening` and `listeners()` take the listening slots.
3. Logging (D3-A): `proxy listener not started` at Error with `addr`, `serial`,
   `error` and `retry_in`, at the first failure and when the error text changes;
   `proxy listener retry` at Debug for each try;
   `cube proxy listening` at Info as today,
   with `tries` where it took more than one.
4. Tests (`proxy_retry_test.go`), with `ProxyRetry` at a few milliseconds:
   - a port held by a test listener: not listening,
     then listening once it is released,
     and a proxy client gets the greeting there;
   - `EADDRNOTAVAIL` from `listenTCP`: retried,
     then listening once `listenTCP` binds;
   - a change of proxy address, and a removal of the Cube,
     stop a slot that is trying (no goroutine left, `go test -race`);
   - a `-proxy` listener (`Options.Proxy`) retried as well;
   - the log: one Error for a repeated failure, a second for a new error.
     `TestProxyPortInUse` keeps its meaning:
     nothing listens while the port is taken.
5. `docs/proxy.md`
   ("Several Cubes on one host", and the listeners' paragraph):
   a listener that cannot listen - an address the host does not carry yet,
   a port in use - is tried again, at most every 30 s, and listens once it can;
   `README.md` (`-proxy`): the same in a sentence.
6. `CHANGELOG.d/Fixed/claude-proxy-retry.md`:

   ```
   - A Cube's proxy listener that cannot listen, on an address the host
     does not carry yet or a port in use, is tried again every 30 s at
     most, instead of staying down until serve restarts.
   ```

**Verification:**
the test gate; `go test -race -count=1 ./internal/server` five times.

### Block 2: The API

1. `server.go` (`cubeView`): `proxy_failing`, a list of
   `{addr, error, since, next_try, tries}` for the Cube's listeners that do not listen.
2. `system_api.go` (`systemView`): `proxies`, every listener:
   `{serial, addr, listening, error, since, next_try}`.
3. Tests: both fields while a port is held, and after it is released;
   `TestOpenAPICoversRoutes` still passes.
4. `docs/api-v1.md` (the Cube's fields,
   `GET /api/v1/system`) and `docs/api-v1-openapi.yaml`: the two fields;
   the `PUT /api/v1/cubes/{sn}` text:
   an address that cannot be listened on yet is answered with `proxy_failing`,
   and tried again.
5. The fragment of Block 1 is extended where the API is worth a word
   (`changelog.sh check` confirms).

**Verification:**
the test gate.

### Block 3: The UI

1. `components.js`:
   `noticeEl(text, { since, next } = {})` - a `p.notice`,
   `ageEl(since)` appended where `since` is given, `next` (a time,
   `fmtShortTime`) in its title; a comment saying what it is for,
   as its neighbours have.
2. `cubes.js` (`cubeDetail`): under "Proxy address",
   `noticeEl` while `proxy_failing` has entries -
   "Not listening on <addr>: <error>", since when, the next try in the tooltip -
   in place of "Listening on …".
3. `update.js`, `partials/system-info.html`: a "Cube proxies" row,
   each listener with its Cube, address, and state,
   a failing one through `noticeEl`; the section's badge,
   `sectionBadges(card, [badgeEl(text, "warn")])`, while one fails,
   and cleared with an empty list once none does.
4. `i18n/*.json`:
   the new keys in all six, under `proxy.` and `sysinfo.`,
   FR/IT/NL/PL machine-translated and marked.
5. `tools/ui-test/`:
   a test with a failing listener at 1100 and 390 px (the fake server's proxy
   address on a port held by the test).
6. `docs/ui.md`:
   what the two places show.

**Verification:**
the test gate with `make ui-test`.

### Block 4: The remaining general documents

1. `packaging/README.md`, "Learnings":
   a proxy address the host takes on after `serve` starts (a second address
   added by hand or late by the network manager) is picked up within 30 s;
   no restart needed.
2. Anything else found by `grep` for `proxy listener` and `proxy_listening` in `docs/`
   and `packaging/` that the blocks did not reach.
   Documents only: no fragment where nothing shipped changes.

**Verification:**
`changelog.sh check`, `reflow.sh` on the files touched.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +ProxyRetry v1.0

A Cube's proxy listener that fails to listen is never tried again: on a Raspberry Pi on 2026-11-13 a proxy address the host took on after serve started stayed dead until the address was changed or serve restarted. PLAN++ ProxyRetry, taking the recommendations D1-A to D4-A.

Key changes:
- GUIDELINES/wip/2026-11-13_20-03_AP_ProxyRetry_retry_failed_proxy_listeners.md: the plan, four blocks

File-revision-v1: GUIDELINES/wip/2026-11-13_20-03_AP_ProxyRetry_retry_failed_proxy_listeners.md::v1.0::2026-11-13::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

- Block 1, `fix(server): A proxy listener that cannot listen is tried again`
- Block 2, `feat(api): A Cube's proxy listener that does not listen, and why`
- Block 3,
  `feat(ui): The Cubes settings and System Info show a proxy that does not listen`
- Block 4, `docs(packaging): A proxy address the host takes on later`

## Change History

- v1.0 (2026-11-13): the first revision,
  from the owner's `PLAN++` on the TL;DR gate, D1-A to D4-A.

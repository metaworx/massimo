# NOTE ProxyRetry v1.0: how the run went

The run of `PLAN++` on 2026-11-13, Blocks 1 to 4 on `claude/proxy-retry`,
and where it went differently from the plan.

## Block 1

- **The test hooks are options, not a package variable.**
  The plan had `listenTCP`, a package variable,
  and an exported `Options.ProxyRetry`.
  A package variable swapped by one test races the proxy goroutines
  of an earlier test's server that are still ending,
  so both became unexported fields of `Options`,
  as `pushRetry` is:
  `proxyRetry` (the first and the longest wait) and `proxyListen`.
- **The listener's discovery entry** is added where the listen succeeds
  and removed when `ServeTCP` returns,
  inside the slot's goroutine,
  so that a stop racing a successful try leaves nothing in the responder.

## Block 2

- **`cube_proxies`, not `proxies`**, in `GET /api/v1/system`:
  the answer already has `proxy`, the reverse proxy in front of the server.

## Block 3

- **The "Web UI updates" session's answer**:
  its error display is an evaluation without a plan,
  nothing to build on;
  it pointed at what is on main (`sectionBadges`, `badgeEl`, `.notice`,
  `ageEl`) and suggested `noticeEl` in `components.js`, which this block adds.
- **Since when is a time, not an age** (`notice.since`, `fmtShortTime`):
  `ageEl`'s tooltip reads "Received …",
  and under a minute it shows "now".
  The next try is the notice's tooltip, a time to the second.
- **Badges in three places**, not only System Info's:
  the Cube's panel head and the Cubes card count the listeners
  that do not listen too, as they count the Cubes not connected.
- **Two `Added` fragments** beside Block 1's `Fixed` one,
  one for the API and one for the UI,
  since each block's commit carries its own bullet.
- **The screenshots retaken** (`make ui-screenshots`), as the Makefile asks
  at a change of the UI, at the default ports:
  System settings' three, and the Cubes and Maintenance shots,
  which the new row moves by a pixel.
  The profile shots also differed, from changes before this run,
  and were left as they were.
- **The UI suite**: 648 passed at 1100 and 390 px, de and en.

## Block 4

- `packaging/README.md`, "Learnings", alone:
  a `grep` for `proxy listener` and `proxy_listening` in `docs/` and `packaging/`
  found nothing else the blocks had not reached.

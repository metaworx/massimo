# AP SystemInfo v1.0: Server facts in their places

## TL;DR

A server never lists its own proxies' Cubes as found,
and the web UI's header loses the tall parts that made it flicker:
the found Cubes become a link, the server's facts a System Info tile,
the Cube's facts its card in System settings → Cubes,
and the header keeps the current Cube's budget on one line.

- own proxy's answer → never found: the proxies running now,
  not only `serve`'s first ones
- header flicker on a slow scroll → the header out of scroll anchoring,
  a slow-scroll test
- "N Cubes found, not configured" pane → a link to System settings → Cubes
  (`cubes`), else text; the Cube menu's found entries jump there too
- the header's expert list → a System Info tile before Software update,
  `GET /api/v1/system`: version, built with, started, certificate,
  installation, host, client address
- firmware, address, radio, Cube clock → each Cube's card: an info block,
  and duty cycle, free slots, pending in its head
- header, expert mode: the current Cube's name, duty cycle, free slots, pending,
  queued when not zero; kept when the header compacts

### Blocks

1. OwnCubes: discovery leaves out the answers of every proxy the server runs,
   by its serial and by its address; the test first; its own pull request
2. ScrollGuard: a slow-scroll UI test at 390 and 1100 px,
   seen failing on today's header, and the header taken out of scroll anchoring
3. FoundLink: the header's found line a link or text,
   the Cube menu's found entries jumping to System settings → Cubes,
   the found pane's hints moved there
4. SystemInfo: `GET /api/v1/system` with the installation,
   the host and the client address; the System Info tile;
   the documents and six languages
5. CubeInfo: each Cube card's head with the host and its budget,
   its info block, both refreshed in place
6. HeaderStats: the header's one line for the current Cube in place of the expert list;
   the documents

Blocks 2 to 6 are one pull request.

## Discussion

The owner's four points of 2026-11-12, weighed under `EVAL` the same night:

1. "a server that proxies it's cubes, will show the proxied entries as new cubes
   and offers to add them.
   A server should NEVER discover it's own cubes -
   or filter them out in all results!"
   It was the Raspberry Pi 5 without a Cube of its own,
   which proxies several Cubes on five static addresses of its own;
   its main address is none of them.
2. Scrolling down from the top with expert mode on,
   or with "N Cubes found, not configured" unfolded, does not scroll and the
   screen flickers, unless the scroll is strong enough to pass those parts.
3. "N Cubes found, not configured" is to be a link to System settings,
   with the right; otherwise just information, not a foldable pane.
4. The header's expert list moves to a new tile "System Info" before Software
   update, one column, a table, with the client's address,
   the system the server runs on and the installation;
   the Cube's facts go to the Cube.

**The owner's decisions** (2026-11-13):

- **D14-A:**
  a new plan in this session, this one.
- **D15-B, as the owner put it:**
  the Cube menu keeps its found entries; with the capability to add Cubes,
  one jumps to System settings → Cubes, as the header's link does; without it,
  they are information.
- **D16-A, as the owner put it:**
  in expert mode the header shows, for the Cube chosen, its name first,
  then duty cycle, free slots and pending, and queued where it is not zero.
- **D17-A:**
  System Info for everyone who can open System settings, without expert mode.
- **D18, the owner's layout:**
  the address stays in the head of each Cube's card, without the port;
  an info block in the card holds address and port, firmware,
  radio and the Cube clock, with duty cycle, free slots and pending; duty cycle,
  free slots and pending also go to the card's head;
  those of the current Cube also stay in the page's header.
- **D19-A and D20-A**, as recommended, by `PLAN++ SystemInfo`:
  the header's line stays when the header compacts on scroll (D19-A);
  a Cube's info block is shown to everyone who sees the card (D20-A).

**The scope** the evaluation stated was five blocks.
The scroll guard and its test, the evaluation's last block,
come second here as a block of their own,
so that the test is seen failing on today's header before anything else changes
it; the work is the same.

**Read as:**
"clock" is the expert list's Cube clock;
the clock at the top of the header, which everyone sees, stays.
Queued is in the info block too, as it is in the header.
The certificate's notice under the header,
for a self-signed or ending certificate, stays: it is a warning for everyone,
not an expert fact.

**Names chosen here**, routine and not the owner's decisions:

- the route `GET /api/v1/system`, with the capability `read`,
  as `GET /api/v1/update`
- its fields `version`, `started_at`, `tls` (as `/healthz` has them),
  `install`, `os`, `arch`, `hostname`, `client_ip`
- `install`:
  the package marker's manager (`apt`, `brew`, …) where there is one,
  else `windows-service`, `systemd`, `launchd`, else `other`
- the tile `sys-info` in `partials/system-info.html`;
  the header's line `cube-stats`
- the branches `claude/own-cubes` (Block 1) and `claude/system-info` (Blocks 2 to 6)

**Not in this plan:**
the found Cubes' list in System settings → Cubes, which stays as it is
but for the hints it takes over;
the version in the header's top line;
the Cube clock at the top of the header;
the pending notice under the header, for everyone.

**Who runs it:**
this session, under the owner's `PLAN++ SystemInfo` of 2026-11-13.
The pull requests are pushed and opened; merging them is the owner's.
Nothing is sent to a real box.

## Analysis

### Own Cubes (Block 1)

`discoverOnce` (`internal/server/discovery.go`) leaves out an answer `ownAnswer`
claims: one from an address of this host (`localIPs`, every interface address,
the five static ones included) whose serial is a configured Cube's or the
discovery serial of a listener in `s.opts.Proxy`.

`s.opts.Proxy` holds the listeners `serve` started with.
A Cube added through the API, or whose proxy address or "Serial in discovery
replies" changed since, gets its listener from `startCubeProxy`,
which `s.proxies` holds and `s.opts.Proxy` does not.
Such a listener answers with its own serial (`proxy.Options.DiscoverySerial`,
the store's `proxy_serial`, there so that the original software can tell the
proxy from the Cube), and no configured Cube has that serial:
the answer passes as a new Cube, and the page cannot hide it either,
since it hides only configured serials.
A restart of `serve` would make the entries go,
until the next change of a proxy setting.

The fix: `ownAnswer` asks the listeners running now (`s.proxies`):

- an answer from a listener's own address (a listener on a host it names)
  is this server's, whatever its serial;
- an answer whose serial is one a running listener answers with,
  or a configured Cube's `proxy_serial`,
  is this server's when it comes from this host's address,
  or when Search marked it a proxy's (`cubelan.KindProxy`),
  whatever its address (NAT, a container).

An entry already found goes at the next complete run (`settle`).

### The flicker (Block 2)

`stickyHeader` (`components.js`) compacts the sticky header at 96 px down and
grows it back under 8 px.
Compacting hides the expert list, the found pane, the pending notice and
others (`style.css`, `body.scrolled`),
and the lost height is kept as the header's bottom margin,
so that what follows does not move.

Scroll anchoring, on by default in Chromium, Firefox and Edge,
keeps an element in view where it was across a layout change.
It skips fixed elements, not sticky ones,
so its anchor can be one in the header, which moves when the header compacts.
The correction takes the page back by that move;
where the hidden parts are taller than the 88 px between the two thresholds,
it lands under 8 px, the header grows back, the anchor moves back,
and the page goes down again: the flicker, until a scroll is big enough.
That is the owner's observation: expert mode on, or the found pane open.

`overflow-anchor: none` on `.top` takes the header and what it holds out of the
anchor's choice; the anchor is then in the page,
which the margin keeps in place.
A test that scrolls in small steps is to show it failing first.

### The found line (Block 3)

`renderFound` (`app.js`) fills `<details id="cube-found">` with the found rows
(`foundRow`, `cubes.js`), two hints, the settings file's lines and a third hint.
System settings → Cubes already lists the found Cubes with Add (`renderCubes`,
`needs-cubes`).
The line becomes a button that opens System settings
(`showView("system")`) and brings the Cubes card into view (`showCard`),
for a user with `cubes`; plain text otherwise.
The hints and the settings file's lines go under System settings' found list.
The Cube menu's found entries (`renderCubeSelect`) do the same jump with
`cubes`, and stay disabled without it.

### System Info (Block 4)

The header's expert list (`index.html`,
`#budget`) takes the version and its parenthesis ("built with"),
the start time and the certificate from `/healthz`, which needs no sign-in.
`GET /api/v1/system` returns them, and what `/healthz` does not tell a stranger:
the installation, `runtime.GOOS` and `GOARCH`, `os.Hostname()`,
and the client's address as the server sees it (`clientIP`,
through the trusted proxies).
The installation is found by `serve` (`cmd/massimo`) at the start,
from `install.Packaged` and the service manager it runs under (`restart.go`:
the Windows service, `systemdStarted`, launchd),
and passed in the server's options.
The tile, `partials/system-info.html`, goes before `sys-software`,
a table of one column, loaded when System settings opens.
The page's version check and the certificate's notice keep using `/healthz`.

### Each Cube's card (Block 5)

`GET /api/v1/cubes` (`summary`,
`server.go`) carries each Cube's address (`host:port`), firmware, radio address,
budget, pending, queued and clock.
`cubePanel` (`cubes.js`) shows the serial and the address in the head.
The head shows the host without the port, and duty cycle,
free slots and pending; the info block,
a table at the top of the card's left column, holds address, port, firmware,
radio, Cube clock, duty cycle, free slots, pending and queued.
`cubesPolled` redraws the list only while no card is open,
so that a redraw does not take what is typed;
the heads and the open cards' info blocks are refreshed in place instead.

### The header's line (Block 6)

The expert list goes from the header.
One line, `expert-only`, shows the current Cube's name,
then duty cycle, free slots and pending, and queued where it is not zero,
from `renderHeader`'s Cube; it stays when the header compacts (D19-A).

### What changes, by file

- `internal/server/discovery.go`, its test (Block 1)
- `internal/server/ui/style.css`, `tools/ui-test/test_scroll.py` (Block 2)
- `internal/server/ui/index.html`, `app.js`, `cubes.js`, `style.css`,
  `tools/ui-test/test_discover.py`, `test_cubes.py` (Block 3)
- `internal/server/system_api.go` and its test, `server.go`,
  `cmd/massimo/serve.go`, `internal/serve/serve.go`,
  `internal/server/ui/partials/system-info.html`, `ui.go`, `app.js` or a
  script of its own, `docs/api-v1.md`, `docs/api-v1-openapi.yaml`,
  a UI test (Block 4)
- `internal/server/ui/cubes.js`, `partials/cube-panel.html`, `style.css`,
  a UI test (Block 5)
- `internal/server/ui/index.html`, `app.js`, `style.css`, UI tests (Block 6)
- `internal/server/ui/i18n/*.json`, `docs/ui.md` and a `CHANGELOG.d` fragment,
  with the block they belong to

### Risks

- Headless Chromium may not show the flicker.
  Then the NOTE says so, and the guard stays,
  with the test as a guard against its return.
- Other sessions run the UI suite on the default ports:
  free ones are picked first.

## Implementation Plan

### Block 1: OwnCubes

1. A test in `internal/server/discovery_test.go`:
   a Cube added through the API with a proxy address and a `proxy_serial` of its
   own; a discovery run whose request reaches its proxy; nothing found.
   It fails before the fix.
2. A second: a proxy's answer with that serial from another address,
   marked a proxy's; nothing found.
3. `ownAnswer` asks `s.proxies` and the configured Cubes' `proxy_serial`,
   as the Analysis says.
4. `CHANGELOG.d/Fixed/claude-own-cubes.md`.

**Verification:**
the two tests fail before and pass after;
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `go test -race` of the server,
`gofmt -l .`, `go vet` for three platforms, `make cross`,
`make lan-test` (discovery).
Branch `claude/own-cubes` from `main`, its own pull request.

### Block 2: ScrollGuard

1. `tools/ui-test/test_scroll.py`:
   expert mode on and the found pane open,
   the page scrolled down in small steps from the top;
   the page goes down at each step and the header stays compact past 96 px.
   At 390 and 1100 px.
   It is run on today's header first.
2. `overflow-anchor: none` on `.top` in `style.css`, with its reason.
3. `CHANGELOG.d/Fixed/claude-scroll-flicker.md`.

**Verification:**
the test fails before the guard, or the NOTE says it did not; it passes after;
`make ui-test`.

### Block 3: FoundLink

1. `index.html`:
   `#cube-found` a line with a button, no `<details>`.
2. `app.js`:
   `renderFound` fills the line; the button, for `cubes`,
   opens System settings and the Cubes card; text otherwise.
   `renderCubeSelect`: the found entries enabled with `cubes`,
   jumping there.
3. `cubes.js`:
   the hints and the settings file's lines under the found list.
4. The texts in six languages; `docs/ui.md`.
5. `test_discover.py`, `test_cubes.py`: the line, the jump,
   the hints in their new place, the text without `cubes`.
6. `CHANGELOG.d/Changed/claude-found-link.md`.

**Verification:**
`make ui-test`; the Go gate for the embedded files.

### Block 4: SystemInfo

1. `cmd/massimo`:
   the installation from the marker and the service manager,
   a function with a table test;
   passed through `serve.Config` to `server.Options`.
2. `internal/server/system_api.go`:
   `GET /api/v1/system`; its test checks each field and the client's address
   through a trusted proxy.
3. `docs/api-v1.md` and `docs/api-v1-openapi.yaml`;
   `TestOpenAPICoversRoutes` passes.
4. `partials/system-info.html` in `uiPartials`, before `sys-software`;
   the script that fills it when System settings opens.
5. The texts in six languages; `docs/ui.md`.
6. A UI test: the tile's rows and values, at 390 and 1100 px.
7. `CHANGELOG.d/Added/claude-system-info.md`.

**Verification:**
the Go gate, `make cross`, `make ui-test`.

### Block 5: CubeInfo

1. `cubes.js`:
   the head with the host and duty cycle, free slots and pending;
   the info block from a template in `partials/cube-panel.html`;
   both refreshed in place by `cubesPolled`.
2. `style.css`:
   the info block's table, the head's figures at 390 px.
3. The texts in six languages; `docs/ui.md`.
4. A UI test: the head, the block's rows,
   a value changing while the card is open.
5. `CHANGELOG.d/Changed/claude-cube-info.md`.

**Verification:**
`make ui-test`; the Go gate.

### Block 6: HeaderStats

1. `index.html`:
   the expert list goes, the line `cube-stats` comes.
2. `app.js`:
   `renderHeader` fills it;
   `renderStarted` and `renderTLS` fill the System Info tile,
   `checkVersion` as before.
3. `style.css`:
   the line, kept when the header compacts.
4. The texts in six languages, the keys left unused removed; `docs/ui.md`.
5. The UI tests that read the expert list read the tile or the line;
   `test_scroll.py` keeps passing.
6. `CHANGELOG.d/Changed/claude-header-stats.md`.

**Verification:**
`make ui-test`, the Go gate, `make cross`.
Branch `claude/system-info` from `main`, one pull request for Blocks 2 to 6.

## Proposed commit message

Block 1:

```
fix(discovery): A server never finds its own proxies' Cubes

…

Key changes:
- internal/server/discovery.go: …
- CHANGELOG.d/Fixed/claude-own-cubes.md: …

AP SystemInfo v1.0, Block 1.

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 2 to 6, one commit each, in the same form:
`fix(ui): The header no longer flickers on a slow scroll from the top`,
`feat(ui): The found Cubes are a link to System settings`,
`feat(ui): System Info in System settings`,
`feat(ui): Each Cube's facts in its card`,
`feat(ui): The header shows the current Cube's budget on one line`.

## Change History

- v1.0 (2026-11-13): the first revision, by `PLAN++ SystemInfo`,
  with D14-A, D15-B, D16-A, D17-A and D18 as the owner gave them,
  D19-A and D20-A as recommended.

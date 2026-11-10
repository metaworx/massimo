# AP DiscoveryCore v1.0: one search for serve and discover

## TL;DR

`massimo discover` and serve's search share the wire format but not the search:
two loops, and two ways of telling what an answer is.
The server folds a Cube's serial heard from the Cube and from a proxy into one entry,
the last answer winning, so a serve following that Cube moves to each in turn,
every run.

- two loops → one `cubelan.Search` on a socket,
  or fed by serve's responder (D8-A, D10-A)
- serve asks `c` only → `N` and `c`, every Cube at once (D9-A)
- a proxy told by its MAC and mDNS,
  in `discover` only → also by its silence to `N` and `c`, for both;
  else unknown (D11-A)
- one serial from two addresses → one found Cube at its own address,
  with its proxies; a follower stays put while its address answers,
  else a proxy, else the Cube (D4-A)
- found Cube in the API and UI → its proxies and the addresses it uses

### Blocks

1. OneSearch: `cubelan.Search` for `discover`, serve and the responder;
   each answer marked; tests; the README
2. StickyFollow: the server keeps a run's answers per serial, one entry,
   one move at most; tests; the API docs; the LAN test
3. ShownProxies: a found Cube's proxies and addresses in the API and the web UI;
   docs; the UI and LAN tests

## Discussion

**The owner's question** of 2026-11-10, after AP LostCube retired:
its D4 was one serial heard from two addresses,
and the owner asked why `massimo discover` and serve do not use the same code
for discovery,
why serve does not ask for the network settings as `discover` does,
and why not one function that takes a socket, queries,
marks and resolves the network settings, and returns a list.

**Decided** at that EVAL, by the owner's `PLAN++` as recommended:

- **D4-A.**
  Tell a Cube's answers apart, and follow sticky with the proxy preferred:
  a following serve keeps its address while it is among the run's answers,
  otherwise takes a proxy's, otherwise the Cube's own;
  at most one change per run.
  The found list shows the Cube once, with its proxies.
  B, sticky only, leaves a new follower picking the Cube's own address
  and fighting the proxy's serve; C, a serial of the proxy's own by default,
  has a follower by the real serial hear only the Cube.
- **D8-A.**
  One discovery core, in this plan: the marking shared,
  then the server's follow, then the collecting loop in `cubelan` for both.
- **D9-A.**
  serve's search asks `N` and `c` as each answer arrives, every Cube at once,
  from the same socket.
- **D10-A.**
  One function in `cubelan`: a socket,
  an optional feed of the datagrams it reads, the targets and the wait;
  it returns the list, each answer marked, its network settings filled in.
  B, a callback per complete Cube,
  would hand a Cube's answer over before a proxy's had arrived:
  the order dependence D4 removes.
- **D11-A.**
  An answer is a Cube's when it answered `N` or `c`, has an eQ-3 MAC,
  or comes from the bootloader; a proxy's when its MAC is not eQ-3's, or,
  for `discover`, its address is an mDNS-announced server's; otherwise unknown,
  and an unknown address is followed only while it is the current one.

**Names chosen here**, routine and not decisions of the owner's:
`cubelan.Search`, `SearchOptions`, `Packet`, `Kind` with `cube`,
`proxy` and `unknown`, the fake's `SetIdentifyOnly`,
the server's option `LookupMAC`, the JSON fields `kind`, `proxies`, `gateway`,
`netmask` and `dns`, the blocks OneSearch, StickyFollow and ShownProxies,
the tests named in the Implementation Plan, the branch `claude/discovery-core`.

**Not in this plan:**

- `massimo netconfig`, which reads and writes one Cube's settings with its own requests;
- a serial of the proxy's own by default (D4-C),
  and the original software's first answer per serial,
  which `-proxy-serial` and the firewall answer (`docs/proxy.md`, "Discovery");
- `discover` on the box that runs serve,
  where 23272 is taken (`docs/portal-protocol.md`,
  "Lessons from the hardware days").

**Who runs it:**
this session.
The plan's own commit goes straight onto `main`;
the work branch `claude/discovery-core` starts after it, from `origin/main`,
and becomes one pull request with a commit per block.

## Analysis

### What the two share, and where they split

Shared, in `internal/cubelan`: the request bytes,
the reply parsing (`ParseIdentifyReply`, `ParseConfigReply`),
the default targets (`DiscoveryTargets`: broadcast, multicast,
every interface's broadcast) and the MAC lookup (`LookupMAC`,
from the ARP cache, on Linux, Windows and macOS).

Split:

| | `massimo discover` (`cubelan.Discover`) | serve (`proxy.Responder.Discover`) |
|---|---|---|
| socket | its own, 23272, else any port | the responder's, which holds 23272 for the proxies' answers |
| after an answer | `N`, then `c`, one Cube after the other, up to 1 s each | `c` only, at once, the DHCP flag |
| what an answer is | `markProxies`: MAC not eQ-3's, or an announced server's address | none: its own proxies' answers dropped (`ownAnswer`), the rest folded by serial |

The socket is the one forced difference:
a Cube answers discovery only to UDP 23272, whatever the request's source port,
and serve holds that port so that its proxy answers discovery.
Its search therefore goes out from that socket,
and `Responder.Serve`, which keeps answering other hosts' requests,
hands it the answers it reads;
two readers on one socket would split its datagrams between them.
serve without a proxy already runs a responder on a socket of its own.

The rest grew apart:
`discover` came first, serve's search a day later for the found list,
and the proxy marking was added to `discover` alone with the mDNS announcement.

### One serial from two addresses

With no `-proxy-serial`, the default, the proxy answers with the Cube's serial.
`note` (`internal/server/discovery.go`) keys the found map by serial and overwrites
the entry's address and MAC with each answer.
For a Cube followed by discovery (configured by serial,
or added from the found list), the Cube's answer, usually the first,
moves the client to the Cube, the proxy's answer moves it back:
two "cube address from discovery" lines and two `address_changed` events every
run, and a reconnect queued for each.
The second supersedes the first by its key while that waits,
but one that has started dials the Cube,
and a client between connections dials a new address at once.
The Cube drops a second client's SYNs silently,
so that dial costs ten seconds;
landing while the proxy's serve reconnects, it takes the Cube's one LAN slot.
The found list and the store's row show whichever answered last.

### Telling the answers apart

A massimo proxy answers `I` only: `discoveryReply` refuses every other opcode.
A Cube answers `N` and `c`, and so does the fake Cube.
So an answer to `N` or `c` marks a Cube on any platform,
without ARP or mDNS, on loopback too, which makes it testable.
A lost reply cannot make a Cube a proxy while ARP has its eQ-3 MAC;
where neither says, the answer is unknown,
and the follower takes it only while it is already its address.

Preferring the proxy: since AP LostCube,
a proxy answers only while its serve holds the Cube,
so a direct connection cannot get the Cube's one LAN slot then.

### Costs

`N` and `c` are two UDP requests per answer per run, LAN only,
no radio and no duty cycle.
A run lasts its wait plus at most a second for an answer that came late,
since the requests go out as each answer arrives;
a proxy, which answers neither, extends nothing unless it answered late.
`discover` with several Cubes gets faster: it waited up to two seconds per Cube.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`go test -race -count=1` over the Makefile's `RACEPKGS`,
`make lan-test` (discovery changes),
`make ui-test` at 1100 and 390 px for Block 3,
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`.
Nothing reaches a real Cube or a box: the fake Cube answers on loopback,
and the LAN test runs in Docker.
One functional commit per block, citing `AP DiscoveryCore v1.0, Block <n>`,
its CHANGELOG bullet a fragment in `CHANGELOG.d/` (COMMIT.md §5.5).

### Block 1: OneSearch

1. `internal/cubelan/search.go`:
   - `Packet` (a datagram and its source),
     `Kind` (`cube`, `proxy`, `unknown`) and `Discovered.Kind` (`kind` in JSON);
   - `SearchOptions`:
     targets (nil: `DiscoveryTargets`), wait,
     `Packets` (the socket's datagrams when another reader owns it; nil:
     Search reads it), `MAC` (nil: `LookupMAC`);
   - `Search(ctx, pc, opts)`:
     `I` twice to every target; for each new serial and address,
     `N` and `c` sent to the answer's source at once;
     their replies fill `Gateway`, `Netmask`, `DNS` and `DHCP`;
     at the wait's end up to a second more for an answer asked late; the MAC;
     the kind by D11-A; one entry per serial and address, in arrival order.
2. `internal/cubelan/discover.go`:
   `Discover` and `discover` keep their signatures,
   open the socket as now and call `Search`; `ask`, `queryNetwork`,
   `queryConfig` go.
3. `internal/proxy/proxy.go`:
   `Responder.Serve` hands every datagram that is no request to the running
   searches;
   `Responder.Discover(ctx, opts)` registers a feed and calls `cubelan.Search`;
   `discoveryAnswer`, `parseDiscoveryAnswer` and `configWait` go.
4. `cmd/massimo/discover.go`:
   `markProxies` keeps the mDNS rule only: an answer not marked a Cube,
   from an announced server's address, is a proxy;
   `proxy` in the table and `-json` stays.
5. `internal/server`:
   `Options.LookupMAC` for the tests;
   `discoverOnce` passes it with its targets and wait;
   `note` takes the MAC from the answer.
   The server folds by serial as before.
6. `internal/cubelan/fakecube/discovery.go`:
   `SetIdentifyOnly`, answering `I` only, as a massimo proxy.
7. Tests, `internal/cubelan/search_test.go`:
   - `TestSearchMarksTheAnswers`:
     a fake answering everything is a Cube, with its gateway and DHCP flag;
     an identify-only fake is unknown on loopback,
     a proxy with a MAC not eQ-3's injected, a Cube with an eQ-3 one;
   - `TestSearchFromAFeed`:
     a search fed through `Packets` while another goroutine reads the socket;
   - `TestSearchWaitsForALateAnswer`:
     an answer near the end gets its `N` and `c` in.

   `TestMarkProxies` (`cmd/massimo`) and the server's discovery tests pass unchanged.
8. `README.md`, the `discover` command:
   a proxy is told by its silence to `N` and `c` too.
9. `CHANGELOG.d/Changed/claude-discovery-core-search.md`:

   ```
   - serve's search asks each Cube for the addresses it uses as well, and
     massimo discover asks all Cubes at once; a proxy is told by its MAC
     or by its silence to N and c (`README.md`, "discover").
   ```

**Verification:**
the test gate;
`make lan-test`'s ten checks pass unchanged, check 7 now on `kind`.

### Block 2: StickyFollow

1. `internal/server/discovery.go`:
   - `discoverOnce` groups a run's answers by serial, in arrival order,
     after `ownAnswer`; `note` stays for one answer;
   - the entry and the store's row take the Cube's own answer:
     the first marked a Cube, else the first unknown, else the first proxy;
     the other answers' addresses are kept on the entry, in memory;
   - a Cube followed by discovery: its address while among the run's answers,
     else the first proxy's, else the Cube's own;
     an unknown one only while current;
     at most one move, one `address_changed` and one reconnect a run.
2. `POST /api/v1/cubes/discover` lists each Cube once,
   as `GET /api/v1/cubes` does.
3. Tests, `internal/server/discovery_test.go`:
   - `TestOneSerialFromACubeAndAProxy`:
     the fake Cube at 127.0.0.2 and an identify-only fake at 127.0.0.3,
     each with a TCP fake, MACs injected;
     a Cube configured by serial ends at 127.0.0.3 after three runs,
     with one `address_changed`, never connected at 127.0.0.2;
     the serial listed once, at 127.0.0.2.
     It fails on Block 1's code.
   - `TestAnUnknownAnswerIsNotFollowed`:
     with the Cube silent and only an unknown answer,
     the follower keeps its address.
4. `docs/api-v1.md`:
   `POST /api/v1/cubes/discover` and the follow rule;
   `docs/api-v1-openapi.yaml` where it says the same.
5. `tools/lan-test/run.sh`, check 4: the serial listed once,
   at the Cube's address.
6. `CHANGELOG.d/Fixed/claude-discovery-core-follow.md`:

   ```
   - A Cube heard from its own address and from a proxy is one found Cube,
     and serve following it by discovery stays at one of them: it moved to
     each answer in turn, every run (`docs/api-v1.md`).
   ```

**Verification:**
the test gate;
`TestOneSerialFromACubeAndAProxy` fails on Block 1's code and passes after step 1;
`make lan-test`.

### Block 3: ShownProxies

1. `internal/server/discovery.go`:
   the found Cube's `proxies`, `gateway`, `netmask` and `dns`,
   in memory as `dhcp` is, in `GET /api/v1/cubes`,
   `POST /api/v1/cubes/discover` and `cube_found`.
2. `internal/server/ui/cubes.js`:
   the found Cube's line shows "via proxy at …",
   and its gateway beside the radio address and the MAC (expert);
   the text in all six `ui/i18n/<lang>.json`.
3. `docs/api-v1.md` and `docs/api-v1-openapi.yaml`: the fields;
   `docs/ui.md`: the found Cube's line;
   `docs/proxy.md`, "Discovery": other servers list the proxy under the Cube.
4. Tests: the server's discovery tests read `proxies` and `gateway`;
   `tools/ui-test/test_discover.py` the gateway on the found line;
   `tools/lan-test/run.sh`, check 4: the box's address among the Cube's proxies.
5. `CHANGELOG.d/Added/claude-discovery-core-proxies.md`:

   ```
   - A found Cube lists the proxies that answer for it and the addresses it
     uses, gateway, netmask and DNS; the web UI shows "via proxy at …"
     (`docs/api-v1.md`, "GET /api/v1/cubes").
   ```

**Verification:**
the test gate;
`make ui-test` at 1100 and 390 px;
`make lan-test`.

## Proposed commit message

The plan's registration,
a commit of its own on `main` before the first project write:

```
chore(aps): +DiscoveryCore v1.0

massimo discover and serve's search share the wire format but not the search, and the server folds a Cube's serial heard from the Cube and from a proxy into one entry, the last answer winning; the plan gives both one search in cubelan, marks each answer, and has the server list the Cube once and follow it sticky.

Key changes:
- GUIDELINES/wip/2026-11-10_22-41_AP_DiscoveryCore_one_search_for_serve_and_discover.md: the plan, three blocks, D4-A, D8-A, D9-A, D10-A and D11-A.

File-revision-v1: GUIDELINES/wip/2026-11-10_22-41_AP_DiscoveryCore_one_search_for_serve_and_discover.md::v1.0::2026-11-10::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `feat(cubelan): One search for serve and discover, each answer marked`
2. `fix(discovery): A Cube and its proxy are one found Cube, followed sticky`
3. `feat(discovery): A found Cube lists its proxies and the addresses it uses`

## Change History

- v1.0 (2026-11-10): the first revision,
  from the owner's questions after AP LostCube's D4; D4-A, D8-A, D9-A,
  D10-A and D11-A by the owner's `PLAN++`.

# AP DiscoveryShare v1.1: Share the discovery port

## TL;DR

`serve` shares UDP 23272 with the original MAX! software on the same PC,
and leaves it the Cubes' answers while it searches:
serve's discovery socket becomes dual-stack,
which Windows and Linux rank below the software's IPv4 socket for unicast.

- Blocks 1–3 (#237, merged): the port shared, own-host searches also
  answered by broadcast, the documents
- serve's discovery socket: udp4 → dual-stack (`udp`, `[::]`);
  the software's IPv4 socket gets every unicast while it is open, in either
  order; serve alone still gets everything
- serve's own `Discover` and `Reboot`: wait while the software searches
  (3 s after its last search, at most 30 s); a run it interrupts is cut short,
  no Cube logged gone
- a Cube answer no own operation asked for → Warn, once a minute
- documents: the measured rules, what the software does, #237's corrections

### Blocks

1. A shared discovery socket for serve
2. Answers to the own host by broadcast
3. Documents and the changelog
4. A dual-stack discovery socket
5. Serve's own UDP work waits for the software's search
6. A Warn line for a Cube answer serve did not ask for
7. Documents and the changelog for Blocks 4 to 6

## Discussion

The finding (2026-11-14): on the owner's Windows PC the service's `serve`
(v0.14.0-alpha.1, LocalSystem) holds UDP 0.0.0.0:23272;
the MAX! software 1.4.6 (Java 8) on the same PC fails its Cube search,
`new MulticastSocket(...)` → "Unrecognized Windows Sockets error: 0: Cannot bind".
Connecting to a proxy by IP works, and so does `massimo discover`.

**The owner's decisions** (`PLAN++`, 2026-11-14, as recommended):

- **D1-A:** serve's sockets share the port, and serve answers a request from
  its own host by broadcast as well.
  Not B, sharing alone (the app binds and still finds nothing),
  not C, a switch to turn the proxy's discovery off,
  not D, binding serve to its specific addresses (Windows only).
- **D2-A:** serve alone shares; `massimo discover` and `netconfig` keep their plain bind.

**After #237** (2026-11-14 afternoon): the owner ran the MAX! software beside
the service on #237's build. It bound the port and found nothing:
the Cubes answer unicast, and Windows gave every answer to serve's socket,
bound first at boot. A test with the service stopped (the owner's `EXEC`)
showed that a holder which steps aside leaves the software every answer,
and the owner's own run of the software listed all of them.
The owner asked why serve's socket should not be dual-stack:
measured, a dual-stack socket is outranked for unicast by the software's IPv4
socket on Windows and Linux, whatever the order, which needs no hand-over.

**The owner's decisions** (`PLAN++ as recommended`, 2026-11-14):

- **D5:** a dual-stack discovery socket, all the time (the owner's proposal),
  instead of closing and re-opening serve's socket on a search.
- **D6-A:** a Cube answer that no own operation asked for is logged at Warn,
  once per source and opcode per minute;
  the user can then stop the service for the other program's work.
  Network settings belong to Massimo anyway,
  and the Cubes get no more firmware (end of life).
- **D7-A:** Block 2's broadcast stays, for macOS and a host without IPv6,
  where the socket falls back to udp4.

## Analysis

The measurements and Microsoft's rules are in
`wip/2026-11-14_11-47_ANALYSIS_DiscoveryShare_discovery_port_23272_sharing.md`.
In short:

- **Where serve binds 23272:** `Server.ensureResponder`
  (`internal/server/runtimes.go`), for good, whenever a proxy listens on 62910,
  and `Server.discoverOnce` (`internal/server/discovery.go`) per run without a
  responder. Both are plain `net.ListenPacket`.
- **Windows:** across accounts, the user's SO_REUSEADDR bind succeeds only
  where the first socket set SO_REUSEADDR too (WSAEACCES otherwise,
  which Java 8 shows as "error 0"). Unicast to the shared port reaches
  the socket bound **first**; broadcasts reach both (measured, same account).
- **Linux:** SO_REUSEADDR on both, no UID check; SO_REUSEPORT alone does not
  admit Java, which sets SO_REUSEADDR only. Unicast: serve's udp4 socket beats
  Java's dual-stack one; between two udp4 sockets the last bound wins, or a
  hash with SO_REUSEPORT. Broadcasts reach both.
- **macOS:** BSD shares a wildcard port only with SO_REUSEPORT on both;
  the JDK sets it there with SO_REUSEADDR. Not measured.
- **Why the broadcast:** the Cubes answer unicast to `<host>:23272`, and so does
  the proxy's answer to the app, whose source port is 23272: on serve's PC both
  land in one socket, serve's. A broadcast reaches both. The directed broadcast
  of the subnet, not 255.255.255.255: Windows sent that from the APIPA address,
  and the app connects to the reply's source address.
- **Which subnet:** for a proxy on a specific address (`Responder.Add` with an IP),
  that address's subnet, sent from its socket; for a wildcard proxy, the
  requester's address's subnet, sent from the shared socket.
- **Serve's own search** goes out from the shared socket and loops back as a
  request from the own host, several times per run (one per target);
  broadcasting answers to it would add LAN traffic every run for nobody.
  So no broadcast while the responder's own search (`Responder.Discover`) runs;
  an app's request in that window gets the unicast replies of today.
- **The CLI stays plain:** sharing 23272 there would put the proxy's unicast
  answer into serve's socket, and `massimo discover` would find nothing on
  serve's PC; today it falls back to any port and gets the answer.
- **Tests that still hold:** `TestDiscoveryWithoutThePort` holds the port with a
  plain socket first: Linux and macOS refuse the shared second bind (the first
  set no option), and so does Windows (default then SO_REUSEADDR, same account,
  specific address: WSAEACCES).
- **What serve gives up:** with the app started first, serve's own search may
  lose the Cubes' unicast answers to the app (Windows); today serve gets no
  discovery port at all in that order. Another local program could likewise
  take unicast answers on Linux; a discovery answer carries nothing secret,
  and anyone on the LAN can already answer a discovery request.

### After #237: the software's side, and who gets unicast

Measured 2026-11-14 on spare ports, the Windows sender a remote one (WSL);
the real Cubes only with the service stopped, by the owner's `EXEC`.

- **The software** (`MaxLocalBackend-1.4.6.jar`, read with `javap`):
  `CubeFinder` opens a `MulticastSocket(23272)` per interface (wildcard,
  IPv4 and IPv6 sockets under Java 8 on Windows, dual-stack on Linux), joins
  224.0.0.1, sends the hello to 255.255.255.255 and 224.0.0.1, listens 2 s
  and closes; it takes a Cube's address from the reply's **source** address,
  so a reply cannot be relayed or forged (Windows refuses a raw UDP datagram
  with a foreign source). `CubeUdpProxy` (network settings, firmware) opens
  23272 per operation and sends unicast to the Cube's address.
- **Windows, udp4 beside the software:** unicast to the socket bound first,
  in every order, with or without the multicast join.
  With the service stopped: a plain holder took all 10 answers and the
  software's `CubeFinder` got none; a holder that closed and re-opened its
  socket on a search from its own host (in about 0.5 ms; the answers came
  3.5 ms to 1 s later) left it all 10, 5 runs of 5, and the owner's own run
  of the software listed them all.
- **Dual-stack beside the software's IPv4 socket:** the IPv4 socket gets every
  unicast in either order: Windows 5 of 5, Linux 200 of 200 on 16 CPUs
  (with SO_REUSEADDR and SO_REUSEPORT; with SO_REUSEADDR alone the last bound
  wins on Linux). Broadcasts reach both. On Linux a udp4 socket outranks a
  dual-stack one, and a dual-stack one with SO_REUSEPORT is filed after the
  others, so it loses ties; Windows prefers the IPv4 socket too.
- **A dual-stack socket sends** limited and directed broadcasts (Windows and
  Linux) and 224.0.0.1 (Linux; Windows refuses it from udp4 too on the
  owner's PC, a route of its own), and receives IPv4 as IPv4-mapped addresses,
  which `Serve` turns back into four-byte ones.
- **The cost:** while the software's socket is open, serve's own searches and
  reboots lose the Cubes' answers to it; so they wait for it (Block 5).
  An IPv4 program that keeps 23272 open, shared, keeps them for as long.
  `massimo discover` and `netconfig` bind plain and are refused, as before.
- **Telling the software's search from serve's own:** both come from the host's
  own address and port 23272 and carry the same bytes; serve's own come back
  through the loopback within microseconds of being sent. A search from the
  own host and the responder's port that does not follow one of serve's own
  sends within 200 ms is the software's.
- **macOS** is not measured; Block 2's broadcast still covers serve's proxies
  there, and on a host without IPv6, where Go opens udp4.

## Implementation Plan

### Block 1: A shared discovery socket for serve

1. `internal/sharedport/` (new package): `Control`, a `net.ListenConfig.Control`
   that sets SO_REUSEADDR on Windows and SO_REUSEADDR + SO_REUSEPORT on Linux,
   macOS and the BSDs, nothing elsewhere; `ListenPacket(network, address)`.
   Files per platform as `internal/announce/sockopt_*.go` has them.
2. `internal/announce/responder.go`: `sharedport.Control` in place of
   `shareAddress`; `internal/announce/sockopt_*.go` removed.
3. `internal/server/runtimes.go` `ensureResponder` and
   `internal/server/discovery.go` `discoverOnce`: `sharedport.ListenPacket`.
4. Tests: `internal/sharedport`: two shared sockets on one port bind;
   a plain socket after a shared one is refused, and a shared one after a
   plain one too. `internal/server`: a shared socket binds the discovery
   address while serve's responder holds it.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`,
  `make cross`, `gofmt -l .`, `go vet` on the three platforms;
  the `sharedport` and `server` tests on Windows through interop.

### Block 2: Answers to the own host by broadcast

1. `internal/proxy/proxy.go` `Responder`:
   - `Serve`: for a request whose source is one of the host's own IPv4
     addresses, not the loopback, each reply also goes to the directed
     broadcast of the subnet, to the port the shared socket is bound to;
     from the member's socket and its address's subnet where it has one,
     from the shared socket and the requester's subnet otherwise.
   - Not while `Discover` runs (a counter under `mu`).
   - The host's networks come from a function field (`net.InterfaceAddrs`
     by default), so that tests set them.
   - `replyTargets`, a pure function: the source, the member's address,
     the networks and the search flag → where the reply goes.
2. Tests (`internal/proxy`): `replyTargets` table (remote requester; own
   requester, wildcard and specific proxy; port 23272 and another;
   loopback; /31 and /32; search running);
   `Serve` with the networks set to the loopback's and a recording sender,
   or on Linux a listener on the loopback's broadcast where it receives.

- **Verification:** as Block 1, and `make lan-test` (discovery);
  on Windows through interop, a `serve` with `-discover-listen` on a spare
  port, a fake Cube behind its proxy, and the Java 8 `MulticastSocket`
  probe on that port: the probe binds and gets the proxy's answer.
  The real service stays untouched.

### Block 3: Documents and the changelog

1. `docs/proxy.md`: the original software on serve's PC: it binds beside
   serve, sees serve's proxies, not the Cubes serve holds; the ports'
   sharing per system.
2. `docs/cube-protocol.md` "Lessons from the hardware days": the line on
   `massimo discover` on serve's box gains the sharing rule
   (unicast to one socket, Windows the first, Linux the last).
3. `CHANGELOG.d/Fixed/claude-discovery-share.md`.

- **Verification:** `reflow.sh` no worse, `changelog.sh check --staged`,
  `GUIDELINES/shared/tools/check`. After a release: the owner's Windows
  service and the MAX! software on the same PC (Run Hardware/Owner tests);
  the Mac, optional.

### Block 4: A dual-stack discovery socket

1. `internal/server/runtimes.go` `ensureResponder` and
   `internal/server/discovery.go` `discoverOnce`: network `udp` instead of
   `udp4`: dual-stack on the wildcard, IPv4 for an IPv4 address
   (`-discover-listen 127.0.0.1:…`).
2. `internal/proxy/proxy.go` `Responder.Serve`: an IPv4-mapped source is
   turned into a four-byte one before anything uses it (collectors, replies,
   logs).
3. Tests: `internal/sharedport`: a dual-stack socket beside an IPv4 one on a
   port, both shared: unicast reaches the IPv4 one, a broadcast both
   (Linux and Windows; skipped elsewhere and without IPv6).
   `internal/proxy`: a mapped source reaches the collectors as four bytes.

- **Verification:** the suite, `make race`, `make cross`, `gofmt`,
  `go vet` ×3, `make lan-test`; the `sharedport` test on Windows through interop.

### Block 5: Serve's own UDP work waits for the software's search

1. `internal/proxy/proxy.go` `Responder`:
   - its own sends of a request (`Discover`, `Reboot`) are noted, through a
     `PacketConn` wrapper handed to `cubelan.Search` and `cubelan.Reboot`;
   - a request from one of the host's addresses (`hostNets`) and the
     responder's own port, more than 200 ms after its own last send, is
     another program's search: its time is noted, and a `Discover` running
     then is marked cut short;
   - `Discover` and `Reboot` first wait until 3 s after the last such
     search, at most 30 s, or the context's end;
   - `Discover` cut short returns its answers with `ErrSearchShared`,
     so that `discoverOnce` settles nothing and logs no Cube gone.
2. Tests with a scripted `PacketConn`: another program's search is noted, one
   of the responder's own is not; a `Discover` waits for the window and one
   it interrupts returns `ErrSearchShared`; the server's run with that error
   keeps its Cubes.

- **Verification:** as Block 4.

### Block 6: A Warn line for a Cube answer serve did not ask for

1. `internal/cubelan`: `ParseUDPReply`, the reply's serial and opcode.
2. `internal/proxy/proxy.go` `Responder.Serve`: a Cube's reply that arrives
   while no `Discover` or `Reboot` runs, and none ended within 5 s, is logged
   at Warn ("a Cube answered another program on this host"), with the source,
   serial and opcode and the hint to stop serve while that program works with
   the Cube; once per source and opcode per minute.
3. Tests: such a reply is logged once; one during or just after an own
   operation and a repeat within the minute are not.

- **Verification:** as Block 4.

### Block 7: Documents and the changelog for Blocks 4 to 6

1. `docs/proxy.md` "Discovery": the dual-stack socket and why; the software on
   serve's PC finds serve's proxies, the Cubes and other servers, and its
   network settings dialogue works while serve does not search; serve's own
   work waits; the Warn line; an IPv4 program that keeps the port.
2. `docs/cube-protocol.md` "Lessons from the hardware days": what the software
   does on 23272, and who gets unicast on a shared port (#237's line corrected).
3. `CHANGELOG.d/Fixed/claude-discovery-dualstack.md`.

- **Verification:** `reflow.sh` no worse, `changelog.sh check --staged`,
  `GUIDELINES/shared/tools/check`; on Windows through interop, the `FinderX`
  probe beside a responder on a spare port with WSL as the Cube.
  Then the owner: the service on the pull request's build (devel channel)
  and a search from the MAX! software.

## Proposed commit message

```
fix(server): The original software binds the discovery port beside serve

On a PC running serve, the original MAX! software could not bind UDP 23272 and found no Cube: serve bound it without sharing, and on Windows a service's plain bind refuses every other account. serve's discovery socket now shares the port, and serve answers a request from its own host by the subnet's broadcast too, since unicast to a shared port reaches one socket only.

AP DiscoveryShare v1.0, Blocks 1 to 3.

Assisted-by: claude-code:claude-opus-5-5
```

For Blocks 4 to 7:

```
fix(server): The original software gets the Cubes' answers beside serve

With the discovery port shared, the original MAX! software on serve's PC bound it and still found no Cube: the Cubes answer unicast, and Windows gave every answer to serve's socket, bound first. serve's discovery socket is now dual-stack, which Windows and Linux rank below the software's IPv4 socket, so the software gets the answers while it searches; serve's own searches and reboots wait for it, and a Cube answer no own operation asked for is logged at Warn.

AP DiscoveryShare v1.1, Blocks 4 to 7.

Assisted-by: claude-code:claude-opus-5-5
```

One commit per block, each citing its block.

## Change History

- v1.0 (2026-11-14): first revision, D1-A and D2-A by `PLAN++`.
- v1.1 (2026-11-14): Blocks 4 to 7 after #237: the dual-stack socket (D5,
  the owner's), serve's own UDP work waiting for the software's search, the
  Warn line (D6-A), Block 2's broadcast kept (D7-A); the measurements under
  "After #237".

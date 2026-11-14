# AP DiscoveryShare v1.0: Share the discovery port

## TL;DR

`serve` binds UDP 23272 so that other programs can bind it too,
and answers a discovery request from its own host by broadcast,
so that the original MAX! software on the same PC finds `serve`'s proxies.

- serve's 23272 sockets: plain bind → SO_REUSEADDR (Windows),
  SO_REUSEADDR + SO_REUSEPORT (Linux, macOS); new `internal/sharedport`,
  which `announce` uses for 5353 too
- `massimo discover`, `netconfig`: unchanged, plain bind, fallback to any port
- a request from one of the host's own addresses: unicast replies as before,
  plus the directed broadcast of the subnet, to the responder's port;
  none while serve's own search runs
- documents: the sharing rules, the original software on serve's PC

### Blocks

1. A shared discovery socket for serve
2. Answers to the own host by broadcast
3. Documents and the changelog

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

## Proposed commit message

```
fix(server): The original software binds the discovery port beside serve

On a PC running serve, the original MAX! software could not bind UDP 23272 and found no Cube: serve bound it without sharing, and on Windows a service's plain bind refuses every other account. serve's discovery socket now shares the port, and serve answers a request from its own host by the subnet's broadcast too, since unicast to a shared port reaches one socket only.

AP DiscoveryShare v1.0, Blocks 1 to 3.

Assisted-by: claude-code:claude-opus-5-5
```

One commit per block, each citing its block.

## Change History

- v1.0 (2026-11-14): first revision, D1-A and D2-A by `PLAN++`.

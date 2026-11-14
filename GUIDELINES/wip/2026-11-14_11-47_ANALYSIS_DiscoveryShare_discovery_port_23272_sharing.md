# Discovery port 23272: sharing it with the original software

2026-11-14. Finding: the Windows service's `serve` (v0.14.0-alpha.1, LocalSystem)
holds UDP 0.0.0.0:23272; the MAX! software 1.4.6 (Java 8) on the same PC fails
`new MulticastSocket(23272)` with "Unrecognized Windows Sockets error: 0: Cannot bind".

## Where serve binds 23272

All plain `net.ListenPacket("udp4", ":23272")`, no socket option:

| Where | When | On failure |
|-------|------|-----------|
| `internal/server/runtimes.go` `ensureResponder` | at start, for good, when any proxy listens on 62910 (`serve/setup.go`: `Discovery: port == env.CubePort`, no switch) | Warn, proxy not announced |
| `internal/server/discovery.go` `discoverOnce` | per discovery run, only when there is no responder | Warn, `errNoDiscoverySocket` (503) |
| `internal/cubelan/discover.go` `discover` (`massimo discover`) | per run | falls back to `:0` |
| `internal/cubelan/netconfig.go` `listenDiscovery` (`massimo netconfig`) | per run | falls back to `:0` |

The responder's member sockets (`Responder.Add`) bind `<proxy IP>:0`, not 23272.
`internal/announce` already shares 5353 with `shareAddress`
(Windows SO_REUSEADDR; Unix SO_REUSEADDR + SO_REUSEPORT).

## Measured (port 23999, scratch programs, no hardware, no service touched)

Java = Java 8 `new MulticastSocket(port)` as `CubeFinder` does (it sets SO_REUSEADDR;
on Linux it binds dual-stack `[::]`). Go = udp4 wildcard. Same user account.

### Windows 11 (jaguar, through interop)

| First | Second | Bind | Unicast to host IP / 127.0.0.1 | Broadcast (limited, directed) |
|-------|--------|------|------|------|
| Go plain | Java | **fails, "error 0: Cannot bind"** (the finding, reproduced) | Go | Go |
| Go SO_REUSEADDR | Java | ok | **Go only** | both |
| Go SO_EXCLUSIVEADDRUSE | Java | fails | Go | Go |
| Java | Go plain / EXCLUSIVE | Go fails (WSAEADDRINUSE) | Java | Java |
| Java | Go SO_REUSEADDR | ok | **Java only** | both |
| Go plain on 192.168.178.101 | Java wildcard | ok | to that IP: Go; 127.0.0.1: Java | directed: both |

Windows gives unicast to the **first** socket bound.

### Linux (WSL 2 kernel 6.18)

| First | Second | Bind | Unicast | Broadcast |
|-------|--------|------|---------|-----------|
| Go plain or SO_REUSEPORT only | Java | fails, EADDRINUSE | Go | Go |
| Go SO_REUSEADDR (+/- REUSEPORT) | Java | ok | Go (udp4 beats Java's `[::]`) | both |
| Java | Go SO_REUSEADDR | ok | Go | both |
| Go A SO_REUSEADDR | Go B SO_REUSEADDR | ok | **B, the last bound** | both |
| Go A REUSEADDR+REUSEPORT | Go B same | ok | hashed per source (A here) | both |

### Across accounts (Windows, Microsoft's table, not measured)

"Using SO_REUSEADDR and SO_EXCLUSIVEADDRUSE", different user accounts,
both wildcard: first default → second SO_REUSEADDR = **WSAEACCES**;
first SO_REUSEADDR → second SO_REUSEADDR = **Success**.
Enhanced socket security: "the first socket to call bind on the port must have
SO_REUSEADDR set", the same-account case being the exception.
So the service (LocalSystem) must set SO_REUSEADDR for the user's app to bind at all.
Linux: SO_REUSEADDR on UDP has no UID check; SO_REUSEPORT requires the same UID.

### macOS (not measured)

BSD needs SO_REUSEPORT on both sockets to share a wildcard unicast port;
the JDK sets SO_REUSEPORT along with SO_REUSEADDR on datagram sockets there.
Setting both on serve's socket, as `announce` does, covers it. Owner check on the Mac.

## Consequence

Sharing fixes the bind error but not the search on serve's own host:
the Cubes answer unicast to `<host>:23272`, and so does the proxy's answer to a request
from the host itself (its source port is 23272), and unicast reaches one socket only -
on Windows serve's, started first as a service; on Linux serve's, the udp4 socket.
The app would bind and still find nothing.
Broadcasts reach both sockets on both systems, so serve must answer a request
that came from one of its host's own addresses by the directed broadcast of that
address's subnet (source address measured right: 192.168.178.101 for .255;
255.255.255.255 left from 169.254.x, the APIPA interface - so not the limited broadcast).

`massimo discover` and `netconfig` must keep their plain bind with `:0` fallback:
sharing 23272 there would put the proxy's unicast answer into serve's socket.

## Scratch

`GUIDELINES/temp/udpshare/` (hold, send, Finder.java, win.sh) and
`C:\Users\mdr\AppData\Local\Temp\udpshare\`.

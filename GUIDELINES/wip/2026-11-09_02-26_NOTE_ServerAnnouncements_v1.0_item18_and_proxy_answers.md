# NOTE ServerAnnouncements v1.0: item 18 and proxy answers

Two facts for Block 2, recorded on 2026-11-09 at the owner's `EXEC note18`:
the answer to the first half of roadmap item 18,
and a hardware finding reported by the coordinating session.

## Roadmap item 18, first half: can Massimo announce its own MAC address?

No, and it need not.
The Cube's discovery reply (`internal/cubelan/discover.go`, `parseUDPReply`)
is the signature `eQ3MaxAp`, the serial, `>`, the opcode,
then the RF address and the firmware; it has no field for a MAC address,
so the proxy cannot announce one inside the Cube protocol,
and whether the original software would tolerate extra bytes could only be
tried on hardware.
The MAC a client sees for a Cube is its own ARP cache's entry for the
address that answered (`internal/cubelan/arp.go`);
for a proxy that is the host's network card,
which does not begin with eQ-3's prefix `00:1A:22`.
That is exactly what lets `discover` mark the proxy by the MAC prefix
(the first NOTE's addition 1), so no announcement is needed.

Block 2 records this in `docs/proxy.md`, "Clients find the proxy by themselves":
why a proxy's MAC is the host's and is not announced.
With Block 2 merged, roadmap item 18 is done in both halves and leaves the roadmap;
the coordinating session keeps that file.

## A proxy answers a search with the real Cube's serial

Reported by the coordinating session on 2026-11-09, from a check on the
Windows side against the Raspberry Pi's `serve`:
the Pi's proxy answers a discovery search with the real Cube's serial,
so a search returns that serial twice, once from the Cube and once from the
proxy (`proxy.discoveryReply` answers with the Cube's serial unless a
`-proxy-serial` is set).
Block 2's `discover` therefore lists both answers of one serial, each with
its address and MAC, and marks the proxy's by the MAC prefix or by the
announced server's address; it never collapses answers by serial.
The finding itself belongs in `docs/portal-protocol.md`, written by the
session that made it; the implementing session checks that it is there
and says so at its gate where it is not.

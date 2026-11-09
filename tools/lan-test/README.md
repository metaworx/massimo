# LAN test

`make lan-test` runs `run.sh`:
discovery, `serve`'s mDNS announcement and `massimo discover`
on a LAN of four hosts in Docker, against avahi,
the responder a Raspberry Pi OS box and a Linux desktop run.
The Go tests ask a fake Cube on the loopback, on another port,
and check the announcement against our own browser only;
here each host has an address of its own and the real ports,
and the announcement meets another implementation.

It needs Docker with Compose v2 and takes under a minute,
more the first time the image is built, which installs avahi.
It runs in WSL 2 as well,
where the LAN has neither multicast nor broadcast in NAT mode but a Docker
bridge has both.

## What it checks

`compose.yaml` puts four hosts on one bridge network of their own:

- **cube**:
  `massimo fakecube` with the UI suite's greeting (`tools/ui-test/greeting.txt`,
  the serial `KEQ0000100`), answering discovery on UDP 23272,
  with the MAC `00:1a:22:00:00:01`: eQ-3's prefix, as a Cube's,
  and a placeholder (`tools/identifiers`);
  it may take its link down (`NET_ADMIN`), which is how its cable is pulled;
- **searcher**:
  `massimo serve` with no Cube, no proxy and no announcement,
  searching only when asked, through `POST /api/v1/cubes/discover` on its own address,
  as the web UI's search does;
- **box**:
  avahi-daemon, then `massimo serve -listen 0.0.0.0:8099 -tls self-signed`
  against the Cube, its proxy answering discovery for it;
- **client**:
  avahi-daemon of its own.

`run.sh` builds a static `massimo` into a temporary directory,
which every host mounts at `/m`,
and brings up the Cube, the searcher and the client.
With no proxy on the LAN, it checks:

1. a search lists `KEQ0000100` at the Cube's address, with its MAC,
   `configured: false`, and the searcher logs "cube found on the LAN";
2. the Cube's cable pulled (`ip link set eth0 down` on the Cube),
   the next search lists nothing,
   and the searcher logs "cube gone from the LAN";
3. the cable back, a search finds it again,
   and the searcher logs "cube found on the LAN" once more.

No proxy runs until then because a proxy answers discovery for its Cube
from the greeting it keeps:
with the box up, a search would still list the serial while the Cube is pulled,
and check 2 would test the proxy, not the search.
Then it brings up the box and checks:

4. a search lists `KEQ0000100` twice,
   from the Cube's address and from the box's, as the API lists each answer;
5. the box's `serve` logs "announced on the LAN" and no "not announced":
   it shares UDP 5353 with avahi-daemon;
6. `avahi-browse -rpt _massimo._tcp` resolves the server,
   on the box and on the client, at the box's address and port 8099,
   with the TXT `node` of the box's log, `v` of the build, `tls=1` and an `fp`;
7. `massimo discover -json` on the client lists the server with that node,
   and two answers of `KEQ0000100`:
   the box's (the proxy) marked `proxy`, the Cube's not;
8. `serve` stopped, its goodbye takes the server out of the client's avahi.

The cable is pulled by the Cube's link, not by `docker network disconnect`:
a container connected again gets a new MAC, not of eQ-3's prefix,
and `discover` would mark the Cube a proxy.

Each check passed prints a line;
a failed one prints why and the four hosts' logs,
and `run.sh` exits 1 (2 without Docker).
The hosts are taken down whatever happened,
under a Compose project of their own (`massimo-lan-<pid>`),
so that two runs do not meet.

## Running it

```bash
make lan-test
```

```bash
tools/lan-test/run.sh --help
```

The image of every host is `massimo-lan-test:avahi` (`Dockerfile`, `avahi.sh`),
built from `debian:trixie-slim` at the first run and kept by Docker's cache.
CI runs the test when discovery's or the announcement's paths change
(`GUIDELINES/project/CI.md`).

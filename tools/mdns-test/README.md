# mDNS test

`make mdns-test` runs `run.sh`:
`serve`'s mDNS announcement and `massimo discover` against avahi,
the responder a Raspberry Pi OS box and a Linux desktop run,
on three hosts in Docker.
The Go tests check the announcement against our own browser only,
by unicast on the loopback;
this is where it meets another implementation.

It needs Docker with Compose v2 and takes about a minute,
half of it installing avahi the first time an image is built.
It runs in WSL 2 as well,
where the LAN has no multicast but a Docker bridge has.

## What it checks

`compose.yaml` puts three hosts on one bridge network of their own:

- **cube**:
  `massimo fakecube` with the UI suite's greeting (`tools/ui-test/greeting.txt`,
  the serial `KEQ0000100`), answering discovery on UDP 23272,
  with the MAC `00:1a:22:00:00:01`: eQ-3's prefix, as a Cube's,
  and a placeholder (`tools/identifiers`);
- **box**:
  avahi-daemon, then `massimo serve -listen 0.0.0.0:8099 -tls self-signed`
  against the Cube, its proxy answering discovery for it;
- **client**:
  avahi-daemon of its own.

`run.sh` builds a static `massimo` into a temporary directory,
which every host mounts at `/m`, brings the hosts up and checks:

1. the box's `serve` logs "announced on the LAN" and no "not announced":
   it shares UDP 5353 with avahi-daemon;
2. `avahi-browse -rpt _massimo._tcp` resolves the server,
   on the box and on the client, at the box's address and port 8099,
   with the TXT `node` of the box's log, `v` of the build, `tls=1` and an `fp`;
3. `massimo discover -json` on the client lists the server with that node,
   and two answers of `KEQ0000100`:
   the box's (the proxy) marked `proxy`, the Cube's not;
4. `serve` stopped, its goodbye takes the server out of the client's avahi.

Each check passed prints a line;
a failed one prints why and the three hosts' logs,
and `run.sh` exits 1 (2 without Docker).
The hosts are taken down whatever happened,
under a Compose project of their own (`massimo-mdns-<pid>`),
so that two runs do not meet.

## Running it

```bash
make mdns-test
```

```bash
tools/mdns-test/run.sh --help
```

The image of the box and the client is `massimo-mdns-test:avahi` (`Dockerfile`,
`avahi.sh`),
built from `debian:trixie-slim` at the first run and kept by Docker's cache.
CI runs the test when the announcement's paths change
(`GUIDELINES/project/CI.md`).

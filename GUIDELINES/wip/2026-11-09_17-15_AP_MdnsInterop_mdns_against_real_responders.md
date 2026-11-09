# AP MdnsInterop v1.0: mDNS against real responders

## TL;DR

`serve`'s mDNS announcement and `massimo discover` are tested against our own
code alone, by unicast on the loopback.
This plan tests them against the responders a box and a client really run:
avahi in Docker, Apple's mDNSResponder and Windows' own mDNS in CI.

- the announcement → our browser,
  on the loopback → avahi beside `serve` on one box,
  and `avahi-browse` and `discover` on another, in Docker
- `discover`'s proxy mark → a fake Cube with eQ-3's placeholder MAC beside
  `serve`'s proxy, the proxy marked and the Cube not
- macOS → the existing job also finds a second `serve` with `dns-sd`
- Windows → the existing job checks the announcement beside Windows' mDNS
- `make mdns-test`, which needs Docker; locally in WSL too, where the LAN has
  no multicast and a Docker bridge has

### Blocks

1. Docker: `tools/mdns-test/`, the box, the Cube and the client,
   `make mdns-test`, its README
2. CI: the mDNS job on Linux, the macOS and Windows steps, `CI.md`, the bullet

## Discussion

**Where it comes from.**
The owner's `EXEC D21-A D22-A` of 2026-11-09, on an EVAL of the manual tests
of AP ServerAnnouncements' session: the announcement was found by multicast
on `eth0` once, by hand, in WSL;
`avahi-browse` and `dns-sd` were never tried;
our responder shares UDP 5353 with the system's own, which nothing tests.
The two owner checks that plan left, the announcement on a real LAN and
notifications on a phone, stay with the "Run Hardware/Owner tests" session;
this plan makes what Docker and CI can reach automatic.

**Open at the plan gate:**

- **D1.
  The Docker test's form.**
  A (recommended): a Compose file and a `run.sh` that brings it up, checks,
  and takes it down whatever happened (`docker compose down`), as
  `tools/ui-test/run.sh` starts and stops its servers.
  B: `docker run` alone, as the probes below did: no Compose file, the network,
  the names and the cleanup in the script.
- **D2.
  The contract's test gate.**
  A (recommended): `make mdns-test` for a change to the announcement
  (`internal/announce`), `discover` or the proxy's discovery answers,
  as `make ui-test` is for the UI; it needs Docker.
  B: CI alone runs it.
- **D3.
  When CI runs the Linux job.**
  A (recommended): on every push to `main` and every pull request; it takes
  about a minute and a half, most of it installing avahi.
  B: only when the paths of D2 change, through the job Changed paths,
  as Screenshots does.

**Names chosen here**, routine and not the owner's decisions:
the directory `tools/mdns-test/`, the target `make mdns-test`,
the CI job `mdns` named "mDNS against avahi",
the containers `box`, `cube` and `client`,
the branch `claude/mdns-interop`.

**Not in this plan:**
a Web Push check against a real push service or browser (the EVAL's third line,
not taken); an IPv6-only network (the probes ran IPv4 and IPv6 sockets,
the checks IPv4); a real Cube.

**Who runs it:**
a session of its own, or this one, after the plan's registration on `main`.

## Analysis

### Measured on 2026-11-09

Two probes, by hand, with a static `massimo` of `main` at `190b19b` in
`debian:trixie-slim` containers on a Docker bridge network of their own,
Docker 29.1.3 in WSL 2 (Ubuntu 24.04):

1. **Discovery and the proxy mark.**
   A fake Cube with the MAC `00:1a:22:00:00:01` (eQ-3's prefix,
   a placeholder `tools/identifiers` accepts) answering discovery on 23272 with
   the serial `KEQ0000100`;
   a box running `serve -listen 0.0.0.0:8099` against it,
   its proxy answering discovery too; a client running `massimo discover`.
   The box logged "announced on the LAN"; `discover` listed the server by mDNS,
   with its node, and the two answers of `KEQ0000100`,
   the box's marked `proxy` and the Cube's not.
2. **avahi beside `serve`.**
   The box with `avahi-daemon` (and D-Bus) running first, then `serve`:
   no warning, four sockets on 5353 (avahi's and ours, IPv4 and IPv6),
   and `avahi-browse -rpt _massimo._tcp` on the box resolved the service
   with its host name, address, port and TXT (`name`, `tls`, `v`, `node`).
   Installing avahi took about 20 s.

So multicast crosses a Docker bridge, `SO_REUSEADDR` and `SO_REUSEPORT` share
5353 with avahi on Linux, and a container may carry a MAC of eQ-3's prefix.

### The Docker test (Block 1)

`tools/mdns-test/`:

- `compose.yaml`
  (D1-A): one bridge network;
  - `cube`:
    `massimo fakecube` with `tools/ui-test/greeting.txt`,
    `-discovery 0.0.0.0:23272 -discovery-serial KEQ0000100`,
    `mac_address: 00:1a:22:00:00:01`;
  - `box`:
    the image of `Dockerfile` (`debian:trixie-slim`, `avahi-daemon`,
    `avahi-utils`, `dbus`), D-Bus and avahi started,
    then
    `massimo serve -listen 0.0.0.0:8099 -tls self-signed -discover-interval 0`
    against `cube:62910`, its proxy on, its store in the container;
  - `client`:
    the same image, avahi of its own, for `avahi-browse` and `massimo discover`.
    The binary is built static (`CGO_ENABLED=0`) into the directory and mounted
    read-only, as the probes did;
    nothing is pushed or pulled but the base image and avahi's packages.
- `run.sh`:
  builds the binary, `docker compose up`,
  waits for "announced on the LAN" in the box's log, then checks:
  1. the box's log has no warning about 5353 (the port shared with avahi);
  2. `avahi-browse -rpt _massimo._tcp` on the box and on the client resolve the
     service, the client's to the box's address, with `tls=1`, an `fp`,
     the `node` and the `v` of the binary;
  3. `massimo discover -json` on the client lists the server with that node,
     and two answers of `KEQ0000100`: the box's `proxy: true`, the Cube's
     (MAC `00:1a:22:00:00:01`) not;
  4. `serve` stopped, its goodbye (TTL 0) removes the service from the
     client's avahi within a few seconds (`avahi-browse` without it).
  Then `docker compose down`, whatever happened;
  the logs of the three containers when a check fails;
  exit 1 on any failure.
- `README.md`:
  what it checks, how to run it, what it needs (Docker with Compose v2,
  about a minute and a half),
  and that it is the only test of the announcement against another
  implementation.
- `Makefile`:
  `mdns-test`, which runs `tools/mdns-test/run.sh`.

### CI (Block 2)

- **Linux:**
  a job `mdns` on `ubuntu-latest`, which has Docker and Compose:
  `make mdns-test` (D3).
- **macOS:**
  the job "macOS LaunchDaemon" starts a second `serve` after its install steps,
  on `0.0.0.0:18097`, its store in a directory of its own,
  against the fake Cube already running, with `-announce on`;
  then `dns-sd -B _massimo._tcp local.` for five seconds must list it
  and `dns-sd -L <name> _massimo._tcp local.` must give its port and TXT.
  This is Apple's mDNSResponder sharing 5353 with our responder,
  on one Mac; the second `serve` stops before the job goes on.
- **Windows:**
  the job "Windows service" starts a second `serve` on `0.0.0.0:18097` likewise;
  its log must say "announced on the LAN" and no warning about 5353,
  beside Windows' own mDNS (the DNS client service),
  and `massimo discover -json` must list it by multicast.
  Windows has no `dns-sd` of its own, so the browser is ours here.
- `GUIDELINES/project/CI.md`:
  the new job in section 2's table and the two steps in its descriptions of the
  macOS and Windows jobs; section 7 for the runner's Docker.
- `CHANGELOG.md`:
  the CI change and the target are shipped paths (`.github/**`, `Makefile`):
  a bullet under Changed.

### Risks

- **A runner's multicast:**
  GitHub's macOS and Windows runners may filter multicast,
  or the Windows firewall the loopback's; the probes ran in WSL only.
  Block 2 measures it on its first run;
  a step that cannot work there is dropped, with a NOTE,
  rather than made to pass.
- **avahi's time:**
  the box and the client install avahi at each run, about 20 s each,
  unless Docker's cache keeps the image (locally it does; in CI it does not).
- **A flaky goodbye:**
  avahi removes a service on a goodbye or after its TTL;
  check 4 waits for the removal up to ten seconds rather than a fixed time.

| File | Block | Change |
|---|---|---|
| `tools/mdns-test/compose.yaml`, `Dockerfile`, `run.sh`, `README.md` | 1 | the test |
| `Makefile` | 1 | `mdns-test` |
| `GUIDELINES/project/_CONTRACT.md`, `AGENTS.md`, `GUIDELINES/README.md` | 1 | the test gate (D2-A) |
| `.github/workflows/ci.yml` | 2 | the `mdns` job, the macOS and Windows steps |
| `GUIDELINES/project/CI.md` | 2 | the job and the steps |
| `CHANGELOG.md` | 1, 2 | the target; the CI checks |

## Implementation Plan

Each block ends with the contract's test gate
(`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet` for linux, windows and darwin,
`make cross`)
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD` from the
main checkout; Block 1 adds `make mdns-test`, Block 2 the pull request's CI.
Nothing reaches a real Cube or the network but Docker's base image and avahi's
packages.
One commit per block, citing `AP MdnsInterop v1.0, Block <n>`,
with its CHANGELOG bullet; one pull request for both.

### Block 1: Docker

1. `tools/mdns-test/Dockerfile`, `compose.yaml`,
   `run.sh` as the Analysis describes; `run.sh` with `set -euo pipefail`,
   a trap that takes the containers down, and a line per check passed.
2. `tools/mdns-test/README.md`.
3. `Makefile`:
   `mdns-test`.
4. With D2-A: the contract's test gate names `make mdns-test` (v1.5.0),
   `AGENTS.md` and `GUIDELINES/README.md` generated again.
5. `CHANGELOG.md`, Unreleased, Changed:

   ```
   - `make mdns-test` checks the mDNS announcement and `discover` against
     avahi in Docker (`tools/mdns-test/README.md`).
   ```

**Verification:**
the test gate; `make mdns-test` passes twice in a row;
it fails with the box's log when `serve` runs with `-announce off` (a scratch
change, not committed).

### Block 2: CI

1. `.github/workflows/ci.yml`:
   the job `mdns` (D3), the macOS and Windows steps as the Analysis describes.
2. `GUIDELINES/project/CI.md`:
   sections 2 and 7.
3. `CHANGELOG.md`, Unreleased, Changed, amending Block 1's bullet:

   ```
   - `make mdns-test` checks the mDNS announcement and `discover` against
     avahi in Docker, and CI against mDNSResponder and Windows' mDNS as well
     (`tools/mdns-test/README.md`).
   ```

**Verification:**
the test gate; the pull request's CI with the three checks green,
or a step that cannot work on its runner dropped with a NOTE.

## Proposed commit message

The plan's registration,
a commit of its own on `main` before the first project write:

```
chore(aps): +MdnsInterop v1.0

The mDNS announcement and massimo discover tested against the responders a box and a client really run: avahi in Docker, Apple's mDNSResponder and Windows' mDNS in CI.

Key changes:
- GUIDELINES/wip/2026-11-09_17-15_AP_MdnsInterop_mdns_against_real_responders.md: the plan, two blocks, the probes of 2026-11-09 behind it.

File-revision-v1: GUIDELINES/wip/2026-11-09_17-15_AP_MdnsInterop_mdns_against_real_responders.md::v1.0::2026-11-09::All::the first revision
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `test(announce): make mdns-test checks the announcement against avahi in Docker`
2. `ci(mdns): The announcement against avahi, mDNSResponder and Windows' mDNS`

## Change History

- v1.0 (2026-11-09): the first revision,
  from the owner's `EXEC D21-A D22-A` on the EVAL of AP ServerAnnouncements'
  manual tests; D1 to D3 open at its gate.

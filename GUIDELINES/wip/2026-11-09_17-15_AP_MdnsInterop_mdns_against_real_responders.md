# AP MdnsInterop v1.1: mDNS and discovery on real hosts

## TL;DR

`serve`'s mDNS announcement,
its discovery and `massimo discover` are tested against our own code alone,
on the loopback, with 127.0.0.2 standing in for a LAN address.
This plan tests them on hosts of their own: avahi and the real ports,
a cable pulled and a second server on the LAN in Docker,
Apple's mDNSResponder and Windows in CI.

- the announcement → avahi beside `serve` and on a client, in Docker (#144,
  Blocks 1 and 2)
- macOS and Windows → a second `serve` beside the system's responder (Block 2,
  fixed after its first CI run: `-announce on` ended the flags)
- discovery → a search through the API on its own host,
  the real 23272 and 62910, `docker network disconnect` as the cable,
  a second server's proxy answering too (Block 3)
- Windows → `internal/cubelan` and the server's discovery tests in CI's
  Windows job (Block 4)
- the proxy and a lost Cube → once the owner has decided it (Block 5, later)
- `make lan-test`, which needs Docker (D4)

### Blocks

1. Docker: `tools/mdns-test/`, the box, the Cube and the client,
   `make mdns-test`, its README (done, #144)
2. CI: the mDNS job on Linux, the macOS and Windows steps, `CI.md`,
   the bullet (#144), and the steps' fix after the first run
3. LAN: a searching server in Docker, the cable pulled and put back,
   a second server's proxy; the test named `lan-test` (D4)
4. Windows: `internal/cubelan` and the server's discovery tests in CI's Windows
   job, checked first from WSL
5. The proxy and a lost Cube, once decided: the second server's Cube pulled,
   its proxy checked (D5)

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

**The second request.**
On 2026-11-09 the owner asked, through the FoundCubes session, for a Docker LAN
test in this plan and for the discovery tests in CI's Windows job,
and gave `PLAN+` for both here.
FoundCubes' fix had been checked on hardware from the Windows side with the
test Cube's cable out for a minute:
a search of the Cube's address alone found it, lost it at the first search,
found it again;
a broadcast search still listed it, from the Raspberry Pi only,
whose proxy answers discovery from the greeting it never clears
(`internal/proxy/proxy.go`, `discoveryReply`).
Whether a proxy should answer for a lost Cube is being decided apart
(the session "Decide whether a proxy answers for a lost Cube");
Block 5 checks what is decided.

**Decided at v1.0's gate**
(NOTE gate_decisions): D1-A, D2-A, D3-B.

**Open at v1.1's gate:**

- **D4.
  The test's name, once it tests the LAN as well.**
  A (recommended): `tools/lan-test/`,
  `make lan-test` and the CI job "LAN against avahi",
  renamed in Block 3 with the contract's test gate (v1.6.0),
  `CI.md` and the README.
  B: `mdns-test` stays, its README saying it tests discovery too.
- **D5.
  Where Block 5 runs.**
  A (recommended): here, on the Docker hosts Block 3 brings up,
  once the decision is on `main`.
  B: in the plan that carries out the decision, which then extends this test.

**Open at v1.0's gate, decided:**

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
the branch `claude/mdns-interop`;
for Blocks 3 to 5 the container `searcher` and the branch `claude/lan-test`.

**Not in this plan:**
a Web Push check against a real push service or browser (the EVAL's third line,
not taken); an IPv6-only network (the probes ran IPv4 and IPv6 sockets,
the checks IPv4); a real Cube.

**Who runs it:**
this session: Blocks 1 and 2 are #144, the fix of Block 2's steps goes into it;
Blocks 3 and 4 are a pull request of their own once #144 is merged;
Block 5 waits for the decision.

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

### The first CI run (Block 2)

The run of #144 on 2026-11-09 (run 42662761265):
the Linux job and the Windows step passed; the macOS step failed.
Its second `serve` announced itself,
but listened as a proxy on `:62910` though the step said `-proxy off`:
`-announce` is a boolean switch (`IsBoolFlag`),
so `-announce on` ended the flags at "on",
and `-proxy off` and `-discover-interval 0` were never read.
`dns-sd -Z`, killed after six seconds, left an empty file:
its output, written to a file, was lost with the process,
or mDNSResponder never saw the server; the run cannot tell which.
The fix:

- both steps pass `-announce` alone;
- on macOS `dns-sd` runs under `script -q`, a terminal of its own, so that
  every line it prints is in the file when it is stopped;
- the macOS step runs `massimo discover -json` as well, our own browser,
  so that a failure says whether the runner's multicast or mDNSResponder is at
  fault; where mDNSResponder alone fails,
  the `dns-sd` check is dropped with a NOTE, as the Risks below say.

The same parsing makes `serve -announce off` announce, and `-allow-reboot off`
allow a reboot: a defect of `serve`, not of this plan, which the gate of v1.1
puts to the owner apart.

### The LAN test (Block 3)

The Go tests ask a fake Cube on the loopback, on another port,
with 127.0.0.2 as the LAN.
In containers each host has its own address and the real ports:
discovery's broadcast (`udpBroadcast`, `interfaceBroadcasts`),
the answer to the request's own address, `ownAnswer` with real addresses,
and two servers on one LAN.

- A fourth host, `searcher`, in the box's image with `curl`:
  `massimo serve -discover-interval 0 -proxy off`, no Cube configured,
  its searches through `POST /api/v1/cubes/discover` on its own address.
- `run.sh` brings up `cube`, `searcher` and `client` first,
  so that no proxy answers for the Cube while it is pulled, then:
  1. a search lists `KEQ0000100` at the Cube's address, its MAC,
     `configured: false`, and the searcher logs "cube found on the LAN";
  2. `docker network disconnect` takes the Cube off the network:
     the next search lists nothing, and the log says "cube gone from the LAN";
  3. `docker network connect` puts it back: found again,
     with a new "cube found on the LAN" line;
  4. the box comes up, its `serve` configured with the Cube and its proxy on:
     a search lists `KEQ0000100` from the Cube's address and from the box's.
  Then the mDNS checks of Block 1, the goodbye last.
- Whether the found list keeps two answers of one serial is measured first;
  where it keeps one, check 4 reads the searcher's log for both addresses.
- With D4-A, `tools/mdns-test/` becomes `tools/lan-test/` (`git mv`),
  `make mdns-test` `make lan-test`, the CI job `lan` "LAN against avahi",
  and the contract's test gate, `CI.md` and the README follow;
  the Changed paths add nothing (they hold discovery's paths already).

### The Windows discovery tests (Block 4)

CI's Windows job runs `go test ./internal/update/... ./internal/install/...
./cmd/...` and `go test -run Update ./internal/server/`; nothing on Windows
tests discovery.
Block 4 adds `go test ./internal/cubelan/...` and the server's
`TestSearchListsOnlyCubesThatAnswered`, `TestFailedRunChangesNothing`,
`TestRemovedCubeIsListedAsFound`, `TestFoundCubeSurvivesRestart` and
`TestFoundCubeStoredOnce`, none of which has run on Windows.
They are checked first from WSL: `GOOS=windows go test -c` for both packages,
the binaries run through interop with `ProgramData` in a temporary directory,
away from the live service's; a test that fails there is fixed,
or left out of the job with the reason in a NOTE.

### The proxy and a lost Cube (Block 5, later)

Once the decision is on `main`: the box's `serve` runs with `-poll 5s`,
since a connected `serve` notices a lost Cube only when its read deadline fires,
three polls and 30 s, 45 s at 5 s; the Cube is taken off the network,
and after that time the searcher's search must show what the proxy was decided
to do.

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
| `.github/workflows/ci.yml` | 2 | the steps' fix |
| `tools/lan-test/*` (D4-A, `git mv`), `Makefile`, the contract, `CI.md`, `ci.yml` | 3 | the LAN checks, the name |
| `.github/workflows/ci.yml`, `GUIDELINES/project/CI.md` | 4 | the Windows job's tests |
| `tools/lan-test/*` | 5 | the proxy and a lost Cube |
| `CHANGELOG.md` | 3, 4 | the bullet amended |

## Implementation Plan

Each block ends with the contract's test gate
(`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet` for linux, windows and darwin,
`make cross`)
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD` from the
main checkout; Block 1 adds `make mdns-test`, Block 2 the pull request's CI.
Nothing reaches a real Cube or the network but Docker's base image and avahi's
packages.
One commit per block, citing `AP MdnsInterop v<X.Y>, Block <n>`,
with its CHANGELOG bullet; Blocks 1 and 2 with their fix are #144,
Blocks 3 and 4 a pull request of their own,
Block 5 another once the decision is on `main`.

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

**The fix after the first run** (v1.1), a commit on #144:

1. `.github/workflows/ci.yml`:
   both steps pass `-announce` alone;
   the macOS step runs `dns-sd -Z` under `script -q`
   and `massimo discover -json` beside it.
2. No CHANGELOG change: the bullet says what the steps check.

**Verification:**
#144's CI green; where mDNSResponder alone fails, the `dns-sd` check
dropped with a NOTE and the `discover` check kept.

### Block 3: LAN

1. With D4-A, `git mv tools/mdns-test tools/lan-test`; `make lan-test`,
   the CI job, the contract (v1.6.0), `CI.md` and the README renamed.
2. The image gains `curl`; `compose.yaml` the host `searcher`;
   `run.sh` the four LAN checks before the mDNS ones,
   the hosts brought up in two steps.
3. The README: what the LAN checks show,
   and why no proxy runs while the Cube is pulled.
4. `CHANGELOG.md`:
   the bullet amended:

   ```
   - `make lan-test` checks discovery, the mDNS announcement and
     `discover` in Docker, against avahi, with a Cube's cable pulled and a
     second server's proxy (`tools/lan-test/README.md`).
   ```

**Verification:**
the test gate; `make lan-test` passes twice in a row;
check 2 fails when the Cube is not disconnected (a scratch change).

### Block 4: Windows

1. From WSL: the test binaries of `internal/cubelan` and `internal/server` built
   for Windows and run through interop, `ProgramData` in a temporary directory;
   what fails, fixed or left out with a NOTE.
2. `.github/workflows/ci.yml`:
   the Windows job's unit tests run them.
3. `GUIDELINES/project/CI.md`:
   the Windows job's tests.
4. `CHANGELOG.md`:
   the bullet amended for the Windows job.

**Verification:**
the test gate; the tests pass through interop and in the pull request's Windows
job.

### Block 5: The proxy and a lost Cube (later)

1. Once the decision is on `main`: the box's `serve` with `-poll 5s`;
   `run.sh` takes the Cube off the network after the LAN checks,
   waits the read deadline,
   and checks what the searcher's search shows of the proxy.
2. The README and the bullet.

**Verification:**
the test gate; `make lan-test` passes twice.

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

This revision, before Block 2's fix:

```
chore(aps): +MdnsInterop v1.1

The LAN test in Docker, the Windows discovery tests and the proxy's check added to the plan, and the fix of Block 2's macOS step after #144's first run.

Key changes:
- GUIDELINES/wip/2026-11-09_17-15_AP_MdnsInterop_mdns_against_real_responders.md: v1.1, Blocks 3 to 5, the first CI run.

File-revision-v1: GUIDELINES/wip/2026-11-09_17-15_AP_MdnsInterop_mdns_against_real_responders.md::v1.1::2026-11-09::All::Blocks 3 to 5, the first CI run
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `test(announce): make mdns-test checks the announcement against avahi in Docker`
2. `ci(mdns): The announcement against avahi, mDNSResponder and Windows' mDNS`,
   and `fix(ci): The mDNS steps read every flag, and dns-sd's output is kept`
3. `test(discovery): make lan-test pulls a Cube's cable and adds a second server`
4. `ci(windows): The discovery tests on Windows`
5. `test(discovery): A proxy and a lost Cube`

## Change History

- v1.0 (2026-11-09): the first revision,
  from the owner's `EXEC D21-A D22-A` on the EVAL of AP ServerAnnouncements'
  manual tests; D1 to D3 open at its gate.
- v1.1 (2026-11-09): Blocks 3 to 5, the owner's `PLAN+` on the FoundCubes
  session's request (a Docker LAN test, the Windows discovery tests);
  the first CI run of #144 and the fix of Block 2's steps;
  D4 and D5 open at its gate.

# NOTE TestInfrastructure v1.1: how block 6 went

Where Block 6, Ports, went differently from the plan,
what it left and for which block, and what verified it.

## What it made

- **`testkit.Listen` and `testkit.ListenPacket`:**
  a TCP listener and a UDP socket on the loopback, held until the test ends,
  to hand on.
- **The fake Cube's `Pause` and `Resume`** (D7-A):
  the fake drops its clients and refuses new ones without giving up its port,
  and takes them again on the same address.
  On Linux a dial is refused, as where nothing listens:
  the listener closes while a socket bound without listening holds the port,
  and `Resume` listens again before it lets the holder go.
  Elsewhere the listener stays open and each connection is reset at once;
  the Cube client takes both as a failed attempt.
- **The moves:** the Cube client's four rebinds and the proxy's pause and
  resume their fake; two tests of an address where nothing answers use a fake
  paused from the start; the server's proxy takes a held listener through the
  start's `listenOn`; the discovery targets and the announcer's sockets are held.
- **`docs/testing.md`, "Ports".**

## Different from the plan

1. **Who did it.**
   One builder on Opus, then one second reader, twice:
   the builder made the reader's five points in the fake Cube,
   and the reader read them again;
   the session made the reader's last points, in comments,
   and the commits.
2. **Two commits, one pull request:**
   the kit's listeners with the fake Cube's pause, then the moves.
3. **Three free-port helpers stay** (D3-A says they go):
   - `freeUDP` in the server's discovery tests,
     for the server's own discovery socket:
     the server opens it again on every run, by address,
     and a socket the test held would take a share of its datagrams;
   - `freeAddr` in `internal/serve` and in `cmd/massimo`:
     the proxy's option and a child's `-proxy` take an address alone,
     and the server's hook for a listener is not exported.
   Each says why in its comment, and `docs/testing.md` lists them.
4. **More moved than the six helpers and five rebinds:**
   eight port picks written out in the server's tests,
   and three tests that took a port and freed it.
5. **Three discovery targets lost their skip:**
   a held socket on 127.0.0.1 always binds, so a failure there fails (D9-A).
6. **The refusal is Linux's alone:**
   a bound socket elsewhere may take and drop the dial, a timeout, not a refusal;
   so outside Linux the fake accepts and resets.
7. **The reader's points, made in the fake Cube:**
   a `Pause` racing a `Close` could leak the holder;
   a single-client fake holding a dead session took a client after `Resume`;
   the resumed port stayed open to a listener that shares it,
   so `Resume` now listens plainly beside the holder before it closes it;
   the fallback path has a Linux test.
   One change reaches a fake that is never paused:
   a single-client fake on a listener that is not TCP's refuses a second client
   by a close, where it kept it waiting; `massimo fakecube` listens on TCP.
8. **What stays open, in comments:**
   for the moment of a `Resume`, a listener that reuses addresses,
   as Go's do, can take the port; `Resume` then fails and the fake stays paused.
   A program can aim at the port only where it learned its number before:
   the old race of a port freed and taken again.

## Seen, left for later blocks

For Block 8 (Holes): none new.
On Windows, `TestAnUnreachableCubeIsReportedOnce` now meets a dial that
succeeds and is reset, where it met none; it checks the same lines.

## Verification

- **The builder:** `go test ./...`;
  Block 1's rebind run, `-race -count=50` at scale 2,
  beside a `-race` run of `internal/server` (141 and 460 s);
  the changed packages under `-race` 10 times;
  the pause tests and the moved Cube client tests under QEMU armv6;
  after the reader's points, the fake Cube 20 times under `-race`,
  each new test failing on a fault planted for it.
- **The second reader:** `go vet` for linux, windows, darwin and armv6;
  the fake Cube's and the Cube client's tests on Windows through interop
  and under QEMU armv6; a probe of the reopen's window
  (29 of 3000 rounds, with four goroutines listening on the address).
- **The session, before the commits:**
  `gofmt`, and `go vet` for linux, windows, darwin and armv6;
  `go test ./...` (111 s at load 0.5 to 1.6);
  under `-race` at scale 2, beside a `-race` run of `internal/server`:
  Block 1's rebind run 50 times, with the two tests of a paused fake;
  the fake Cube, the proxy, the announcer and the kit 10 runs;
  the moved server tests 3, all passed;
  the first commit's tree vetted on its own, and its tests run.
- **After the merge of `main`,** which brought only Block 5's NOTE:
  the guidelines check (12 of 12).
- **Left to CI:** `make test-armv6` at scale 4, `make race`,
  the Windows and macOS jobs.
- **Pull request #359.**

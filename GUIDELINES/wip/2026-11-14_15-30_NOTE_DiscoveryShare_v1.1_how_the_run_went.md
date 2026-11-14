# NOTE DiscoveryShare v1.1: how the run went

Where the run of AP DiscoveryShare v1.1 (Blocks 4 to 7) went differently
from the plan, and what it measured.

## The branch

- **#237 was merged before the run**, so Blocks 4 to 7 are on a new branch from
  main, `claude/discovery-dualstack`, with the revision as its first commit.
  The scope was to push to #237; the push and the new pull request wait for
  the PR gate.

## Block 4

- **The reboot's socket of its own** (`sendReboot`, used where serve has no
  responder) is shared and dual-stack too, and goes through a responder of its
  own, as a run's does; the plan named the responder's and the run's only.
- **On Windows through interop:** `TestDualStackLeavesUnicastToIPv4` passed
  three times, with a loopback sender as well as the earlier remote one.

## Block 5

- **No test at the server's level:** the responder's host networks are not
  reachable from the server's tests, whose sockets are on the loopback.
  The proxy's tests cover the cut short; `discoverOnce` passes it on in four
  lines.
- **Block 2's broadcast** is now left out for the responder's own request only,
  where it was left out for any request while a search ran: the responder can
  now tell them apart, and another program's search during a run of serve's
  gets the broadcast too.
- **The tests fail without the code:** with the wait taken out,
  `TestDiscoverWaitsForAnotherProgramsSearch` fails; with the cut short taken out,
  `TestDiscoverCutShortByAnotherProgramsSearch` does.

## Block 6

- **An identify answer is logged at Debug, not Warn:** another server's proxies
  answer a search on their own host by the subnet's broadcast (Block 2), which
  reaches every serve on the LAN, and would have raised a false warning there.
  The Warn is for the other opcodes, the network settings and firmware ones the
  owner named, which go unicast.
- With the repeat suppression taken out, or identify answers warned,
  `TestResponderWarnsOfAnswersItDidNotAskFor` fails.

## Block 7

- **`docs/api-v1.md` too**, beside the plan's documents: a discovery run waits for
  another program's search on the host, up to 30 s, and one it interrupts lists
  only what it heard.

## The checks

- **Windows end to end, port 23992**: the real Responder over a fake Cube,
  the `CubeFinder` copy (Java 8) searching beside it, WSL sending three
  "Cube" answers unicast to the host. With a udp4 responder, as #237 had,
  the copy got none of them; with the dual-stack one, all three, and the
  proxy's answers.
- **Flaky tests met on the way**, both failing on main as well, neither in the
  code changed here: `TestStreamResumes` (main 1 of 10 runs) and
  `TestStreamResyncs` (main 1 of 40, the branch 3 of 40);
  `TestABusyCubeIsListedAsBusy` failed once under the load of a parallel
  build and passed 10 of 10 alone.

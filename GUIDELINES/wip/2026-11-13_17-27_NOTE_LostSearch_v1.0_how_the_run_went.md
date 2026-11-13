# NOTE LostSearch v1.0: how the run went

Where the run of AP LostSearch v1.0 went differently from the plan,
and what it measured.

## Block 1

- **A test that reached the LAN:** the plan's check, the package's tests run with
  UDP 23272 held by another socket, found `TestAddressChangedLogged`
  logging "discovery port in use".
  It sets `FollowDiscovery`, moves the Cube to 127.0.0.2, where nothing listens,
  and the loss then searched on the default socket and the broadcast addresses.
  It now searches on the loopback (`DiscoveryAddr`, a free port as its target).
  Afterwards only `TestDiscoveryWithoutThePort`, which wants it, logs it.
- **The new tests under load** (64 busy loops):
  one failure in 60 runs at first, the fixed Cube's run at the start missing
  the answer within a 50 ms wait, with an hour's interval and nothing to repeat it;
  the test now keeps the package's 200 ms, takes the warning from the searches
  after the loss, which repeat, and waits for the run at the start to end.
  After that, 373 runs of the five tests without a failure.
- **The tests fail without the fix:** with the loss's wake and `needSearch`'s lost Cubes
  taken out, the three tests of a search fail; the fourth, no search without
  an interval, passes either way, as it should.
- **`README.md`** too, beside the plan's documents:
  its paragraph on the runs `serve` makes.

## Block 2

- **`GUIDELINES/project/CI.md`** too (v1.49.0), beside the plan's files:
  its description of the LAN job names the follower.
- **The step fails without Block 1:** `make lan-test` against the code before it
  ended at check 10 with the follower still at the Cube's old address, disconnected.
  With it, the follower followed the Cube from 172.22.0.4 to 172.22.255.254.

## Left for the owner

- **CI's LAN job** runs on a pull request only for changes under
  `internal/announce/`, `internal/serve/`, `internal/proxy/`, `internal/cubelan/`,
  `cmd/massimo/discover.go`, `cmd/massimo/serve.go`, `tools/lan-test/` and `ci.yml`;
  `internal/server/discovery.go`, where discovery's runs are decided, is not among them.
  This pull request runs it through `tools/lan-test/`.

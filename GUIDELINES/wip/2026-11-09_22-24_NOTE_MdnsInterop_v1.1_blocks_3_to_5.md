# NOTE MdnsInterop v1.1: blocks 3 to 5

How Blocks 3 to 5 went, on 2026-11-09, at the owner's `EXEC Block 3-5`,
where it differs from the plan or the plan left it to a measurement.

## Block 3: the LAN test

- **One serial from two addresses.**
  Measured first, as the plan asked:
  `POST /api/v1/cubes/discover` lists every answer of its run,
  so with the box's proxy up the search lists `KEQ0000100` twice,
  from the Cube's address and from the box's.
  Check 4 reads the API, not the log:
  the log's "cube found on the LAN" names the serial once,
  at the address of the answer that came first.
- **The cable is the Cube's link,
  not `docker network disconnect` and `connect`.**
  A container connected again gets a new MAC, not of eQ-3's prefix,
  which `massimo discover` marks a proxy,
  and it loses the alias `cube` that the box connects to.
  `ip link set eth0 down` and `up` on the Cube keep its MAC and its address:
  the Cube runs on the test's image, which gains `iproute2`,
  with `NET_ADMIN`.
- **Checked:**
  `make lan-test` passed three times, in about 35 s with the image cached;
  with the cable left in, check 2 failed
  ("the first search after the Cube's cable was pulled still lists it").

## Block 4: Windows

- **A test failed on Windows, and is fixed.**
  Through interop, `internal/cubelan`'s `TestDiscoverKeepsEveryAnswerOfASerial`
  found no answer at all ("answers from []").
  A probe showed why: with a socket on `127.0.0.1:<port>`,
  Windows lets a second socket bind the wildcard on the same port,
  where Linux refuses it ("address already in use"),
  and delivers the answers sent to `127.0.0.1:<port>` to the first socket.
  The test's fake Cube held the port `discover` bound, and read its own answers.
  `discover` now binds a port apart from the one it asks on,
  any port in the test;
  `Discover` binds and asks on 23272 as before.
  A `serve` binds the wildcard `:23272`,
  so only `-discover-listen <address>`, for tests, would meet this on Windows.
- **More tests than the plan named.**
  The server's tests of discovery all pass on Windows:
  the plan's five and `TestAddFoundCube`,
  `TestCubeConfiguredBySerialFollowsItsAddress`, `TestDiscoveryDHCPFlag`,
  `TestDiscoveryFindsUnconfiguredCubes`,
  `TestDiscoveryLeavesOutTheProxysOwnAnswers` and `TestDiscoveryWithoutThePort`;
  the Windows job runs all eleven.
- **Checked:**
  the 30 tests of `internal/cubelan` and the eleven passed twice through interop,
  with `ProgramData` in a temporary directory;
  the live service's `service.log` kept its line count.
- **The bullet:**
  a second one, for CI, rather than Block 3's amended,
  which would not fit in three lines.

## Block 5: the proxy and a lost Cube

- **The timing is AP LostCube's, not the read deadline's.**
  The plan waited three polls and 30 s;
  AP LostCube (D3-E, #149) takes a Cube for lost after two unanswered polls
  and 10 s, so check 8 waits up to 45 s.
- **It needs #149.**
  On `main` before it (`0df55d6`) check 8 failed: 45 s after the pull,
  the search still listed `KEQ0000100` from the box's address.
  On #149's code (`e3d7443`) it passed twice,
  check 8 in 26 s ("poll failed", then "cube connection lost",
  two polls in a row unanswered),
  and once more on its next head (`f0ad059`), which changed an engine test;
  with #149 merged, all ten checks passed on `main` (`bfb1216`).
- **The bullet:**
  unchanged; `tools/` is not shipped,
  and "with a Cube's cable pulled and a second server's proxy" covers it.

## Found on the way

`serve -announce off` announced, and `serve -allow-reboot off` allowed the reboot:
a switch flag ends at its own name, `off` stayed an argument,
and `serve` never checked for one.
Found by Block 2's macOS step; fixed in a pull request of its own.

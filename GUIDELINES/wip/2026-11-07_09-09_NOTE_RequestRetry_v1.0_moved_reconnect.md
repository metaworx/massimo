# NOTE RequestRetry v1.0: moved reconnect

Where the run of [AP RequestRetry
v1.0](2026-11-07_08-45_AP_RequestRetry_update_request_rename.md) went beyond its
one block, on the owner's `EXEC Fix CI` of 2026-11-07.

1. **The pull request's CI failed elsewhere**:
   "Vet and test" on Linux,
   `TestCubeConfiguredBySerialFollowsItsAddress` timed out waiting for the Cube at
   its new address (run 42353079275).
   Not the update race:
   the reconnect that discovery, or a `PUT` of a Cube's address,
   queues for a Cube at a new address closed whatever connection the client had
   when the queue ran it.
   Since `SetAddress` wakes the dial loop, an unreleased change on `main`,
   that was often the connection just made to the new address,
   so the Cube got a second one and the test's one connection never held.
2. **Reproduced**:
   2 failures in 20 runs under `-race` on `main`,
   none in 30 on the code before the wake;
   `TestMovedReconnectKeepsTheNewConnection` (`internal/engine`) fails on `main`
   three runs of three, with only `Command.Moved` added there.
3. **Fixed** in its own commit on the plan's branch:
   that reconnect is `engine.Command.Moved`,
   and `cubelan.Client.ReconnectIfMoved` drops only a connection to another
   address than the client's, decided under the client's lock;
   the discovery test passes 40 runs of 40 under `-race`.
   The CHANGELOG bullet of the unreleased wake is amended (COMMIT.md 5.2),
   no bullet added.

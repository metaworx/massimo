# NOTE LostOrder v1.0: how the run went

Block 1 went as planned, with one step more.

## The return too

The plan left the return as it was, since its tests waited for the line.
The armv6 suite under QEMU in Docker then failed
`TestAFailedReconnectIsALoss` on this branch:
`0 re-established lines, want 1`.
It waits for the second `EventConnected` and reads the log at once,
and the greeting emitted the event before it logged
"cube connected" or "cube connection re-established":
the same order as the loss's, in the other direction.

So the greeting now logs its line before the event, as the loss does,
and the two tests that waited for the line after the event
(`TestALostCubeIsReportedOnce`, `TestAnUnreachableCubeIsReportedOnce`)
check it directly, their comment saying why.
`Lost` and `Connected` are still set first,
so that a `Reconnect` and `Ready` see the greeting as before.

## The armv6 suite locally

`make test-armv6` runs in Docker on jaguar without anything installed:
an image `massimo-armv6-test`, `golang:1.26.8` with Debian's qemu-user
10.0.13, `-cpu arm1176` as CI, the checkout and the module cache mounted
read-only, the build cache in the volume `massimo-armv6-gocache`.

## `internal/cubelan` under `-race -count=50`

One run takes about 13.5 s, 50 take 677 s:
past `go test`'s default timeout of 10 minutes,
so `-timeout 40m` for that many.

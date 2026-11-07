# github.com/bytemare/ksf v0.5.0, patched

This is `github.com/bytemare/ksf` v0.5.0 (MIT, see `LICENSE`), the key
stretching functions that `github.com/bytemare/opaque` v0.18.0 pulls in,
with the sources it builds from and nothing else.
`go.mod` replaces the module with this directory.

The one change: `argon2id/argon2.go` passes `math.MaxUint32` to
`fmt.Errorf` as an untyped constant, which does not fit an `int` on a
32-bit target, so the module does not build for linux/arm (the
Raspberry Pi 1's armv6, armv7) or 386.
Both calls take `uint64(math.MaxUint32)` here.

Drop the directory and the `replace` once a release of the module builds
for 32-bit targets: `GOARCH=arm GOARM=6 go build ./...` tells.

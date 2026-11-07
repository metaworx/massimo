# NOTE PasswordCost v1.1: how the run went

Blocks 6 to 11 ran under the owner's `PLAN++` of 2026-11-07,
continued by `EXEC` once the owner had allowed the two downloads,
one signed commit each on the branch `claude/opaque-sign-in`,
with main merged in before the push.
Where it went differently from the plan:

- **The branch starts from pull request 87's head, not from main (D11-B).**
  Pull request 87 was open and green, not merged, when the `PLAN++` came.
  The plan's revision and the six blocks are on top of it,
  and #87 itself was left as it is; main was merged in before the push.
- **The downloads.**
  The auto-mode classifier refused `go get github.com/bytemare/opaque` under the `PLAN++`;
  the run stopped at a gate, and went on once the owner allowed it.
- **Interop checked before the code.**
  A scratch run of `bytemare/opaque`'s server against `@serenity-kit/opaque`
  1.1.0's client in Node, before any project write: a record registered in
  Node signs in against Go, the session keys match, the export key is stable,
  a wrong password and a fake record fail.
  A record made with the Go client opens in the browser only with `opaque-ke`'s
  key stretching, a salt of 16 zero bytes and an output of 64; with the Go
  library's defaults it does not.
  Sizes as planned: record 192, KE1 96, KE2 320, KE3 64 bytes.
- **`bytemare/ksf` 0.5.0 does not build for 32-bit targets**, the Raspberry
  Pi 1 among them: two `fmt.Errorf` calls take `math.MaxUint32` as an `int`.
  No release fixes it, so `third_party/bytemare-ksf` holds the release with
  those two lines cast to `uint64`, through a `replace`, and `config.ini` ships
  `third_party/**`.
  A commit of its own (`docs(build)`) removed the upstream README and retitled
  `MASSIMO.md` once the document checks read the tracked copy.
  An upstream fix would let the copy go; filing one is the owner's call.
- **The key file has a format of its own**, the AKE private key (32 bytes)
  and the OPRF seed (64): the library's own encoding of the key material does
  not decode without a server identity.
- **The credential identifier is the login name, not the user's uuid.**
  Names do not change, and a name then meets the same OPRF key with a record,
  with a hash of version 1 or with no user, so an upgrade does not show in the
  evaluation (`TestOpaqueUnknownName`).
- **The registration is one route**, `POST /api/v1/users/{name}/register`,
  whose record goes to the user add or the password change in place of `password`,
  not a start and a finish of its own: nothing is kept between the two steps,
  and the existing routes keep their checks of who may do what.
  Its name being the credential identifier, the first owner registers before
  the user exists.
- **The lockout at the start needed two more pieces.**
  A start that reached the limit locked the address and the name, and the
  limiter's `succeed` does not lift a lock: a right password at the tenth try
  would have left its own session locked out.
  A finish that proves the password now lifts the lock its own start set (`forgive`).
  And since a v1 user's fallback takes such a start over,
  the sign-ins check the address's lockout in their handlers (`lockedOut`),
  no longer in `authorize`.
- **Measured, not estimated**, on one core (`BenchmarkOpaqueLogin`):
  a start 0.76 ms and 20 KB on amd64, 5.1 ms on a 32-bit build,
  the finish 108 ns; a hash 43.8 ms and 109 ms.
  By the hashes' ratio, about 0.1 s a sign-in on the Pi 1 against about 2 s,
  where the plan estimated 50 to 80 ms.
  In headless Chromium a sign-in by OPAQUE took 0.21 to 0.24 s, the argon2id
  at 19 MiB and two passes in WebAssembly included.
  Nothing ran on the owner's Pi.
- **The fallback to the password sign-in came with Block 7**, not only Block 9,
  so that a user of version 1 can sign in in the browser at every commit;
  Block 9 added `login`, the attempt counted once, and the upgrade.
- **The CLI registers version 2 (Block 8).**
  A user it adds or gives a password signs in in the browser (`tools/ui-test`
  checks it with the operator),
  and no longer by `POST /api/v1/login` with the password:
  a script signs in with an API key (README.md says so).
  The UI suite's preferences test took an API key of the operator for that reason.
- **The first owner's form** is tested through the API only (`TestOpaqueRegister`);
  the UI suite's server has users from its start.
- **The audit** has a row per start (`login.start`) beside the finish's (`login`),
  and keeps none of the messages.
- **Migration 0007, not 0006.**
  Main took 0006 for the restart's capability while this run went on;
  the merge renamed this branch's migration and its mentions.
- **One `-race` run of `internal/server` failed once** during Block 7, without
  its failure kept; the runs after it passed, three of them at once.

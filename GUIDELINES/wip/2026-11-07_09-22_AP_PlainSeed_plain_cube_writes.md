# AP PlainSeed v1.0: The plain Cube's two writes in a safe order

## TL;DR

When `serve` learns the serial of the Cube that `MASSIMO_CUBE_ADDRESS` names without one,
it writes the Cube's row,
then the setting tying the plain lines to it;
a stop between the two leaves a store the next start refuses as two Cubes.
The setting goes first.

- in `seedLearned`, Seed → SetSetting(`seed.plain_serial`) becomes SetSetting → Seed
- either write cut short then leaves a store the next start serves as one Cube
- the test first, failing on `main`:
  an SQLite trigger refuses one of the writes
- `TestServeSingleCubeWithoutSerial`, flaky on CI's Windows runner,
  then sees the setting whenever it sees the row
- a store already holding the row without the setting: left as it is (D1)
- a bullet under Fixed in `CHANGELOG.md`

### Blocks

1. PlainSeed: the cut-short test, the two writes swapped, the CHANGELOG bullet;
   verified by the test gate, the test on Windows through interop,
   and the pull request's Windows service job.

## Discussion

### D1. A store already holding the row without the setting: B, left as it is

Settled as recommended by `PLAN++` at the TL;DR gate of 2026-11-07.
Only a stop within the few milliseconds between the two writes makes that state,
and after the swap nothing does.
The error it gives names the way out:
`MASSIMO_CUBE_SN=<serial>` in the settings file makes the plain lines that Cube's.
A, `serveSetup` adopting the one row whose address is the plain address,
would be more code for a case that has probably never happened.

## Analysis

### 1. The failure

`main`'s run for the merge of #75 (run 42351812356),
the Windows service job's unit tests,
as the coordinating session reported it:

```text
serve_test.go:150: second start [] serve: several cubes configured; name the cube of each proxy listener as -proxy <serial>=127.0.0.1:60077, or pick one with -sn
```

The next commit's run was green.

### 2. The two writes

`seedLearned` (`internal/server/store.go`) runs once the plain Cube reported its serial:
`Store.Seed` adds its row from the file's values,
`noteRow` notes the file's differences,
and `SetSetting(store.PlainSerialKey, serial)` records that the plain lines
belong to that serial.
`serveSetup` (`cmd/massimo/serve.go`) reads the setting at the next start:
set, the plain lines join the file's Cubes under that serial and are compared
with its row;
not set, they are served as a Cube of their own beside every row,
and with a proxy listener and no serial that is "several cubes".
The test waits for the row, stops the server and starts again;
on a slow runner the setting is not written yet, or the stop cancels it.

### 3. Reproduced

Waiting for the race does not reproduce it here:
150 runs on Linux and 60 on the Windows side passed.
A trigger does, in a scratch worktree of `main`:
refusing the setting's insert cuts `seedLearned` short after the row,
and the next start fails with the error of section 1.
With the writes swapped, a refused setting leaves nothing written
(the plain lines are served again and the serial learned again),
and a refused row leaves the setting,
by which the next start seeds the row from the file itself;
both serve one Cube.

### 4. Risks

None found: the setting alone is a state `serveSetup` already handles,
and the writes keep their errors in the log.

## Implementation Plan

### Block 1: PlainSeed

1. `cmd/massimo/serve_seed_test.go`:
   `TestServePlainSeedCutShort`, for each write an SQLite trigger on a second
   connection that refuses it,
   the server run until it logs the refused write,
   then the next `serveSetup`, wanting one Cube and no error;
   the refused setting fails on `main`.
2. `seedLearned` writes the setting, then the row, then notes it;
   its comment says why.
3. `CHANGELOG.md`, under Fixed.

**Verification:**
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `-race` for `internal/server`,
`gofmt -l .` empty, `go vet` for linux, windows and darwin, `make cross`,
`GUIDELINES/shared/tools/check --commit-range origin/main..HEAD`;
the test binary of `cmd/massimo` on the Windows side through interop;
the pull request's Windows service job.

## Proposed commit message

```text
fix(server.store): write the plain Cube's setting before its row

When serve learned the serial of the Cube MASSIMO_CUBE_ADDRESS names without one, it wrote the Cube's row and then the setting tying the plain lines to it, so a stop between the two left a store the next start refused as several Cubes; the setting is now written first, and either write cut short leaves a store the next start serves as one Cube (AP PlainSeed v1.0, Block 1).
```

## Change History

- v1.0 (2026-11-07): the first revision,
  from the TL;DR gate of the same morning;
  D1 settled as recommended by `PLAN++`.

# AP LostOrder v1.0: Warning before lost

## TL;DR

A lost Cube is reported in the order `Lost()` promises:
the warning "cube connection lost" first, then `Lost()`, then the
`EventDisconnected`, so that whoever sees Lost sees the warning,
and whoever reads the event sees Lost.

- `Run`: `lost.Store(true)` → emit → warn becomes warn → `Store(true)` → emit
- `TestSilentCubeIsLostAfterTwoPolls`, flaky on armv6 under QEMU → passes
- `TestLostFromALossToTheNextGreeting`: the event count after Lost
  → waited for, since `countEvents` reads the channel on its own
- a new test: a log handler that holds the warning; Lost stays false meanwhile
- `### Fixed` bullet

### Blocks

1. The order in `Run`, the two tests, `CHANGELOG.md`

## Discussion

Asked by the owner on 2026-11-10, after "Test (armv6, QEMU)" failed
`TestSilentCubeIsLostAfterTwoPolls` on PR #160 (run 42783888098) and
earlier on run 42714080920: `cube connection lost lines: []`.

**The owner's decisions** (`EXEC+`, the TL;DR gate, 2026-11-10):

- **D1-A:** reorder in the code, and wait for the event count.
  Not B, the tests alone, which leaves Lost ahead of its warning.
- **D2-A:** a test of the order that fails on every run against the old one.
  Not B, which the existing tests catch rarely.
- **D3, the owner's variant:** a branch on top of PR #160's,
  its pull request into `main`, so that merging it lands both.

## Analysis

`Run`, at the first failure after a connection or since the start,
set `c.lost`, emitted `EventDisconnected` and then logged the warning.
`Lost()`'s doc and `docs/proxy.md` say Lost holds from the loss the
server logs; the code set it before.

- `TestSilentCubeIsLostAfterTwoPolls` waits for `Lost()` and reads the
  log at once: under QEMU the warning was not written yet
  (runs 42783888098, 42714080920).
- `TestLostFromALossToTheNextGreeting` waits for `Lost()` and counts the
  events at once: the event is sent after Lost, and `countEvents` reads it
  on a goroutine of its own, so the count can still be 0 whatever the
  order (run 42715980901).

Measured on 2026-11-10, jaguar, in Docker
(`golang:1.26.8` with Debian's qemu-user 10.0.13, `-cpu arm1176`, as CI):
without the change 2 of 50 failed, both the event count;
with it, through `go test -overlay`, 100 of 100 passed.
On one loaded CPU (`-race -cpu=1`), 1 of 100 before, 0 of 100 after.
`TestSilentCubeIsLostAfterTwoPolls` failed in none of 150 runs here:
a warning held up between Lost and the log is what the new test makes
certain.

The return has the same order, `Store(false)`, the event,
"re-established", and its tests already wait for the line:
left as it is.
`TestQueueObserver`'s failure (run 42696517202) is fixed on `main`
already, its bullet in `[Unreleased]`.

## Implementation Plan

### Block 1: The warning before Lost

1. `internal/cubelan/client.go`, `Run`:
   the warning, then `c.lost.Store(true)`, then the event, with a comment
   saying why.
2. `internal/cubelan/client_test.go`:
   - `TestLostFromALossToTheNextGreeting` waits for the disconnected event
     after Lost, then checks there is exactly one;
   - `TestLostAfterItsWarning`: a `slog.Handler` that, on
     "cube connection lost", signals and waits; while it waits, `Lost()`
     is false; released, Lost follows.
3. `CHANGELOG.md`, `### Fixed`: a bullet.

**Verification:**

- the new test fails against the old `client.go` (`go test -overlay`)
  and passes with the change;
- `internal/cubelan` `-race -count=50`; the two lost tests
  `-race -count=100 -cpu=1` on one loaded CPU;
  100 times under QEMU in Docker, and the whole armv6 suite there;
- `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`, `make cross`;
- `gofmt -l .`, `go vet` for linux, windows and darwin.

## Proposed commit message

```
fix(cubelan): A lost Cube's warning comes before Lost and the event

Block 1 of AP LostOrder v1.0. Run set Lost and sent EventDisconnected before it logged "cube connection lost", although Lost holds from the loss the server logs; a test that saw Lost read the log too early under QEMU, and failed the armv6 job.

Key changes:
- Run warns, then sets Lost, then emits the event.
- TestLostAfterItsWarning holds the warning and checks Lost meanwhile.
- TestLostFromALossToTheNextGreeting waits for the event it counts.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-10): first revision.

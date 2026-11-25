# NOTE RereadOnce v1.0: how the run went

The run of Blocks 1 and 2 under the owner's `PLAN++ both, D1-A`,
on the plan's branch, one commit per block.

## Block 1: ApplyOnce

- Commit `fix(cubeio): A re-read applies its C line once`.
- `Writer.Reread` and the proxy's passthrough of a `c:` apply nothing;
  the client's `Send` is the one apply.
  The comment above the proxy's write says where the C line of a `c:`
  reaches the server's copy, in place of the block that went.
- `TestSameConfigAtDebug` keeps its `pumpSync`, with the comment of D4-A.
- Tests: `TestRereadIsOneConfigEvent` (cubeio),
  `TestProxyConfigOfAnUnknownDeviceIsOneEvent` (proxy),
  `TestRereadIsOneConfigLine` (server).
  With main's `cubeio.go` and `proxy.go` as a `go test -overlay`
  each gave 2 (the server's two lines both at debug);
  on the branch each gives 1.
  `TestProxyPairing`, `TestSameConfigAtDebug` and `TestDoRereadsConfig` pass.
- Documents: none said that a re-read applies its line itself;
  `docs/proxy.md`'s "the answer added to the copy" still holds.
  `docs/roadmap.md`'s entry waits for the retirement.
- `CHANGELOG.d/Fixed/claude-reread-once.md`.

## Block 2: FoldOnly

- Commit `fix(cubelan): A line applied by the mirror answers no waiting command`.
- `handle` routes; `fold` puts a line into the snapshot and emits its events;
  `ApplyLocal` calls `fold` alone, its doc naming `ApplyMetadataWrite`.
- Test: `TestApplyLocalAnswersNoSend` (cubelan).
  With main's `client.go` as an overlay the waiting `c:` of `0a010d`
  returned the C line of `0a010a`; on the branch it returns its own.
- `CHANGELOG.d/Fixed/claude-reread-once-fold-only.md`.

## Verification

`make ui`; `go test -count=1` of `internal/cubeio`, `internal/proxy`,
`internal/server` and `internal/cubelan`;
`make race` over `internal/cubeio`, `internal/proxy` and `internal/cubelan`;
`gofmt -l .` empty; `go vet` for linux, windows and darwin;
`GUIDELINES/shared/tools/check`: all exit 0.
The four new tests passed 30 times in a row (the server's 10).
Not run, CI's by the owner's rule: `go test ./...`, the whole `make race`,
`make cross`, `make ui-test`.
The pull request, #437, passed every job of its CI run (45607933022),
the whole Go suite, the race job, the UI suite and the other platforms' among them.

## Deviations

1. **The tests count without a silence window.**
   The analysis's scratch tests counted events until 300 ms passed
   without one; the new ones read the client's `EventCounts` after the
   call and wait until the watch holds its last event,
   as `docs/testing.md`, "The rule", asks.
2. **The proxy test's unknown device** is a C line the fake answers
   (`SetConfig`) and leaves out of its greeting, not a pairing:
   the pairing path stays `TestProxyPairing`'s.
3. **The cubelan test** holds the `c:` with the fake's `SetSilent`
   and the Send-turn tests' helpers, not a `net.Pipe`.
4. **Block 2's bullet is a second fragment**,
   since COMMIT.md §5.5 takes one bullet per file.
5. **An N line given to `ApplyLocal`** now goes to the snapshot,
   which keeps nothing of it but the time of its last line,
   rather than to the pairing.
   No caller gives it one.

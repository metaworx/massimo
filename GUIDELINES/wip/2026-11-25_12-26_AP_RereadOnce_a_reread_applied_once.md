# AP RereadOnce v1.0: A re-read applied once

## TL;DR

A re-read's C reply is applied by the client's `Send`,
and again by `cubeio.Writer.Reread` and the proxy's `c:`,
so one re-read gives two `config` events, two log lines and two store rows.
The second applies go, and `ApplyLocal` answers no waiting `Send`
(`wip/2026-11-25_10-46_ANALYSIS_RereadOnce_reread_double_apply.md`).

- `Writer.Reread`, the proxy's `c:` → no `ApplyLocal`: `Send` is the one apply
- one re-read → one `config` event, one log line, one store row
- `ApplyLocal` → folds into the snapshot only, without the reply match
- a test per path, each failing on main

### Blocks

1. ApplyOnce: a re-read applies its C line once
2. FoldOnly: `ApplyLocal` answers no `Send`

## Discussion

### What is decided

The owner, 2026-11-25, on the TL;DR gate of the same day:

- **D1-A.** Both second applies go; the client's `Send` is the one apply.
  The owner first wrote "D1 A+B";
  the gate weighed B, the snapshot emitting no event for an unchanged C line,
  which would remove the debug line of an unchanged re-read that AP Logging
  chose and `TestSameConfigAtDebug` checks.
  The owner then signalled `PLAN++ both, D1-A`,
  the gate having said that D1 takes A unless the owner says A+B.
- **D2-A.** `ApplyLocal` folds its line into the snapshot without the reply match.
  Its consequence, asked by the owner: no behaviour changes today,
  since after D1-A its one caller is the metadata mirror,
  whose M lines no `Send` waits for;
  a later caller can no longer answer a waiting `Send` with another line.
- **D3-A.** A test per path, each failing on main:
  cubeio, the proxy, the server, and with D2-A cubelan.
- **D4-A.** `TestSameConfigAtDebug` keeps its `pumpSync`,
  with a new comment: the wait is the rule of `docs/testing.md`, "The pump".
- **D5-A.** The plan's name is `RereadOnce`.

### Decided in the writing

- **The branch** is `claude/reread-once`, from main after the registration;
  one pull request carries both blocks.

## Analysis

The finding, its history, its cost and the scratch tests are in
`wip/2026-11-25_10-46_ANALYSIS_RereadOnce_reread_double_apply.md`.
What each block rests on:

### For Block 1: the second apply

- `Client.handle` (`internal/cubelan/client.go`) applies an L or C reply to
  the snapshot and emits its events before it hands the reply to `Send`;
  `Send`'s doc says so.
- `Writer.Reread` (`internal/cubeio/cubeio.go`) then calls
  `w.Client.ApplyLocal(res.Replies[0])`,
  and the proxy's passthrough of a `c:` (`internal/proxy/proxy.go`) does the same.
  `ApplyLocal` goes through `handle` again: a second `EventConfig`.
- Both came when the client applied no C reply itself;
  since `42ec70b8` (2026-11-07) they only repeat it.
- None of the cases needs the second apply:
  a device not in the snapshot yet, a reply in a greeting,
  a reply after `Send` timed out, an unreadable C line.
- What it costs, per re-read: a second stream event, a second log line at
  debug, a second store row, and for a just-paired device a second rooms read
  in the page.
  Nothing more goes to the Cube.

### For Block 2: `ApplyLocal` and a waiting `Send`

`ApplyLocal` goes through `handle`'s reply match,
so a C line applied while another `c:` waits is taken as that command's reply;
a scratch test shows `c:0a0108` returning the C line of `0a010d`.
In `serve` the queue's gap of 200 ms hides it.
After Block 1 its one caller is the metadata mirror;
splitting `handle` into the routing and the fold lets `ApplyLocal` fold alone.

## Implementation Plan

Each block is a commit on `claude/reread-once`.
Locally only the touched packages run; CI runs the rest.
Every `go test` runs with `MASSIMO_ENV_FILE=/dev/null`.
Each fix's proof is cited in the pull request.

### Block 1: ApplyOnce

- **Goal:** one re-read gives one `config` event and one log line.
- **Changes:**
  1. `internal/cubeio/cubeio.go`: `Reread` without `ApplyLocal`,
     its doc saying that the client's `Send` applies the reply.
  2. `internal/proxy/proxy.go`: the apply after a passed-through `c:` removed.
  3. `internal/server/logdetail_test.go`: `TestSameConfigAtDebug`'s comment
     (D4-A).
- **Tests:** cubeio, the events of one `Reread`;
  the proxy, a client's `c:` for a device the server does not know,
  one event and the device in the server's copy;
  the server, the log's `config` lines of one re-read, any level,
  after `pumpSync`.
  Each fails on main and passes;
  `TestProxyPairing`, `TestSameConfigAtDebug` and `TestDoRereadsConfig` pass.
- **Documents:** any that says a re-read applies its line itself,
  found by a search; none is known.
- **CHANGELOG:** `CHANGELOG.d/Fixed/claude-reread-once.md`:
  a configuration re-read gives one `config` event and one log line, not two.
- **Commit:** `fix(cubeio): A re-read applies its C line once`
- **Verification:** the three tests on main and on the branch;
  `make ui`; `go test` of `internal/cubeio`, `internal/proxy`,
  `internal/server`;
  `make race RACEPKGS='./internal/cubeio ./internal/proxy'`;
  `gofmt -l .`; `go vet` for linux, windows and darwin;
  `GUIDELINES/shared/tools/check`.

### Block 2: FoldOnly

- **Goal:** `ApplyLocal` cannot answer a waiting `Send`.
- **Changes:** `internal/cubelan/client.go`:
  `handle` split into the routing of a reply and the fold into the snapshot;
  `ApplyLocal` folds only, its doc naming the metadata mirror
  and saying that it answers no `Send`.
- **Tests:** cubelan, a C line applied while a `c:` of another device waits
  leaves that `Send` waiting; it fails on main and passes.
- **Documents:** none.
- **CHANGELOG:** a bullet as `changelog.sh check` takes it for the branch's
  second commit: a line applied by the server's mirror is never taken for
  another command's reply.
- **Commit:** `fix(cubelan): A line applied by the mirror answers no waiting command`
- **Verification:** the test on main and on the branch;
  `go test ./internal/cubelan`; `make race RACEPKGS=./internal/cubelan`;
  `gofmt -l .`; `go vet` for linux, windows and darwin;
  `GUIDELINES/shared/tools/check`.

### At the retirement

`docs/roadmap.md`, item 12, "Small ones": the double apply's entry is pruned.

## Proposed commit message

The plan: `chore(aps): +RereadOnce v1.0, +ANALYSIS RereadOnce`

Block 1:

```
fix(cubeio): A re-read applies its C line once

For AP RereadOnce v1.0, Block 1 (D1-A, D3-A, D4-A): the client's Send
applies a C reply to the snapshot before it returns it, and
Writer.Reread and the proxy's passthrough of a c: applied it again, so
one re-read gave two config events, two log lines and two store rows.
The second applies go.

Key changes:
- internal/cubeio/cubeio.go: Reread applies nothing itself.
- internal/proxy/proxy.go: a passed-through c: applies nothing itself.
- Tests in cubeio, proxy and server count one event and one line.
- CHANGELOG.d/Fixed/claude-reread-once.md: one event per re-read.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-25): first revision, from the owner's `PLAN++ both, D1-A`
  on the TL;DR gate of 2026-11-25: D1-A to D5-A.

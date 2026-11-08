# AP ReplyOrder v1.0: reply state before Send returns

## TL;DR

An L or C reply goes into the Cube client's snapshot before the `Send` that asked
for it returns, so that a reading right after sees the reply's state;
`TestForgetDevice`'s rare failure under the race detector was the other order.

- `cubelan.Client.handle` → an L or C reply applied (and its events emitted)
  first, then handed to the waiting `Send`; other replies as before
- `emit` never blocks (it drops the oldest), so `Send` waits for nothing new
- `Send`'s doc → the state of an L or C reply is in the snapshot when it returns
- a seam before the apply → a test that fails on the old order every run,
  not only under `-race`
- the work branch starts from the DNS hooks branch (pull request #116),
  so that one merge brings both

### Blocks

1. ReplyFirst: the order in `handle`, the seam and its test, `Send`'s doc,
   the CHANGELOG

## Discussion

CI's Race job failed on the DNS hooks branch on 2026-11-08 in `TestForgetDevice`
(`internal/cubelan`), "not back after its C line",
a package that branch does not touch.
It is the only failed Race job among the last 40 runs of CI,
and 600 runs of the test under `-race` here passed.
The cause is in the client, not the test:
`handle` hands a reply to the waiting `Send` (`p.ch <- m`) and only then folds an
L or C line into the snapshot (`c.snap.apply`),
so `Send(l:)` or `Send(c:)` can return while the snapshot is still the old one,
and the test reads it in that moment.

No production path reads the snapshot right after such a `Send` today:
`cubeio.Reread` applies the C reply itself through `ApplyLocal`,
and the client's periodic `l:` reads nothing afterwards.
A later caller would meet the same stale state, and the flake stays in CI.

**The owner's decisions** at that gate on 2026-11-08:
D12-A, the fix as a plan of its own;
`PLAN++` with the plan committed on top of `main` and the work branch from the
DNS hooks branch, so that merging this plan's pull request closes #116 as well.

**Names chosen here**, routine and not decisions of the owner's:
the seam `Client.beforeApply`, nil outside tests;
the test `TestSendReturnsWithTheReplyApplied`.

**Not in this plan:**
the order of the other replies, which carry no state of their own,
and the greeting's lines, which no `Send` waits for.

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-08.

## Analysis

What changes, by file:

| File | Change |
|------|--------|
| `internal/cubelan/client.go` | `handle`: the apply before the hand-over for L and C; `beforeApply`; `Send`'s doc |
| `internal/cubelan/client_test.go` | `TestSendReturnsWithTheReplyApplied` |
| `CHANGELOG.md` | the bullet under Unreleased, Fixed |

**How the order changes:**
today a reply of the type `Send` waits for is sent on the pending channel at once,
and an L or C line then falls through to `c.snap.apply` and the events.
After the change, `handle` notes the pending reply under the lock as now,
and for an L or C line applies it and emits its events before the channel send;
an A, H or S reply is handed over at once and returns, as before.
`emit` drops the oldest event when the channel is full,
so the hand-over waits for no reader.
The comment about a C line of the greeting answering a re-read stays true:
the line is applied either way.

**The seam:**
`beforeApply func(cube.Message)`, a field the test sets before `Run`,
called right before `c.snap.apply`.
The test sleeps there for 50 ms on an L or C line:
on the old order `Send` returns during the sleep and the snapshot lacks the reply;
on the new one the sleep delays `Send` and the snapshot has it.

**Risks:**

- A `Send` caller that also reads `Events()` sees a reply's events queued before
  its reply instead of after; nothing in the tree depends on that order,
  which the race detector shows as unordered today anyway.
- The 50 ms of the test is a delay, not a deadline:
  it only has to be longer than the moment between the hand-over and the read.

## Implementation Plan

The block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`make race`'s packages under `-race`, and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube or the network; the fake Cube answers.
One functional commit citing `AP ReplyOrder v1.0, Block 1`,
with its CHANGELOG bullet.

### Block 1: ReplyFirst

1. `internal/cubelan/client_test.go`:
   `TestSendReturnsWithTheReplyApplied` against the fake Cube with the recorded
   greeting: once connected, the test forgets a device of the fixture,
   sets `beforeApply` to sleep 50 ms on an L or C line,
   sends `c:` for the device and finds its configuration in the snapshot when
   `Send` returns, then sends `l:` and finds its state there.
   Run first on the old order, where it must fail.
2. `internal/cubelan/client.go`:
   the field `beforeApply`;
   `handle` hands an L or C reply over after the apply and the events;
   `Send`'s doc says so.
3. The CHANGELOG bullet under Fixed:
   the Cube client folds a device list or a configuration it was asked for into
   its state before the command returns, so that a read right after sees it;
   a race test failed on the old order now and then.

**Verification:**
the test gate;
the new test fails on the old order and passes on the new;
`TestForgetDevice` and the new test 200 times each under `-race`, none failing.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +ReplyOrder v1.0

The Cube client hands an L or C reply to the waiting Send before it folds the line into its snapshot, which made TestForgetDevice fail now and then under the race detector; the plan is registered on top of main, and its work branches from the DNS hooks branch so that one merge brings both.

Key changes:
- GUIDELINES/wip/2026-11-08_17-41_AP_ReplyOrder_reply_state_before_send_returns.md: the plan, one block, the owner's D12-A in its Discussion.

File-revision-v1: GUIDELINES/wip/2026-11-08_17-41_AP_ReplyOrder_reply_state_before_send_returns.md::v1.0::2026-11-08::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The block's commit:
`fix(cubelan): Send returns once an L or C reply is in the snapshot`.

## Change History

- v1.0 (2026-11-08): the first revision, from the Race failure on the DNS hooks
  branch and the owner's D12-A of the same day.

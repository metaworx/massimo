# NOTE ReviewFixes v1.1: how the go stack went

How Blocks 1 to 3 of [AP ReviewFixes
v1.1](2026-11-07_01-32_AP_ReviewFixes_review_fixes.md) went,
under one `EXEC++` on 2026-11-07,
against the finding numbers of [the
analysis](2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md).
The pull requests are stacked:
#72 (Block 1, `claude/review-go`, to `main`),
#73 (Block 2, `claude/review-go-2`, to `claude/review-go`),
#75 (Block 3, `claude/review-go-3`, to `claude/review-go-2`).

## Reproduced and fixed

Each with a test that failed on the code before it (D3-A),
shown in its pull request:

- Block 1: findings 1, 11, 12, 16, 21, 22, 25; lows 37, 46, 54.
- Block 2: findings 2, 3, 18, 19, 23, 24, 26; lows 55, 56, 57, 58, 59, 61.
- Block 3: findings 6, 7, 8, 9, 10, 13, 14, 15, 17; lows 39, 47, 48, 50, 51.

The reuse lows 41, 43 and 62 are fixed too.
Their finding is the duplication or the dead code itself,
which no test can fail on;
the tests that hold the behaviour pass unchanged,
41 and 62 gained a test of the one rule they now share.

## Not reproduced, left as they are (D3-A)

1. **Finding 20**, the Windows service's Stop ending the process before
   `serve` shuts down:
   Windows-only code (`cmd/massimo/service_windows.go`) that no test here can run.
   Read, the defect is as described;
   the completion gate asks whether to fix it without a failing test.
2. **Low 45**, a stale inclusion timer ending the next inclusion:
   the race needs the timer's callback blocked on the inclusion's lock across an
   end and a new start,
   which no test on the old code can arrange without a seam.

## Reproduced, not fixed

3. **Low 60**, `Latest` verifying up to 30 releases before it filters by channel:
   the planned filter by `target_commitish` would drop every release since v0.2.0-alpha.1,
   which all name `main` there whatever their channel,
   since release.yml creates them from the tag without `--target`;
   it waits for release.yml to set one (the other stack's file) or for another
   unsigned hint of the channel.

## What went differently

4. **Finding 11** found a second spoof:
   only the first `X-Forwarded-For` line was read,
   so a client's own line hid the one a proxy such as HAProxy adds;
   fixed in the same commit, the lines read as one list.
5. **Finding 12 (D5-A)**:
   the row of refused requests carries `count`, `statuses` and `last`;
   it is written when its minute ends, at the address's next refusal after it,
   for the oldest address when more than 4096 are open,
   and when the server stops.
   A refusal of a known user (a capability it lacks) stays a row of its own,
   without its body.
6. **Finding 3**:
   the existing files test expected `MASSIMO_TLS_CERT` kept after `-tls off`;
   it now expects it gone, as the plan's fix says.
7. **Finding 19**:
   the test shows the store closed after SIGTERM (its `-wal` file gone),
   not the events flushed, since the built `serve` it runs has no Cube.
8. **Finding 8**:
   a 202 that no run reports back to is acknowledged with its answer;
   no status watcher was built,
   since the server restarts during an update and the helper reports in
   `GET /api/v1/update`.
9. **Finding 7 and low 48** change the queue's `Observer` contract:
   `Enqueued`, and `Finished` of a superseded command,
   come under the queue's lock and must return at once;
   the server's observer hands its writes to one ordered worker,
   which a command's audit row, a restore's end and the server's stop wait for.
10. **Finding 13**:
    the fake Cube gained `ReplyGap` to space a `t:`'s `A:` lines,
    and the proxy now answers its client one `A:` per device the client named,
    where it answered one.
11. **Finding 14**:
    a reconnect queued while the Cube is away waits for its greeting,
    at most the queue's minute, where it failed at once with `ErrNotConnected`.
12. **Finding 15**:
    `engine.Queue.Withdraw` is new;
    a pairing whose `n:` went out by the time its caller gave up keeps its inclusion,
    which the inclusion's own timer ends.
13. **Low 47**:
    no new index;
    `user_id IS ?` lets the existing `commands_key` serve the lookup,
    the key's 24 hours are checked in the query,
    and a write prunes at most once an hour rather than on a timer of its own.
14. **A flaky test of Block 1**, found by Block 2's gate under `-race`:
    the refusal audit's unit test opened two addresses at one fake time and held
    its lock across a `Fatalf`, which hung the package for its ten minutes.
    Fixed by a commit on #72's branch, merged into Block 2's.
15. **Two commits without a CHANGELOG bullet**,
    where the brief asked one of every commit under `internal/`: that test fix,
    and low 43's removal of a dead field,
    neither of which changes anything a reader of the result sees (COMMIT.md 5.2).
16. **The evidence on the old code**:
    each block's new tests ran against the previous branch with the block's test
    files copied in; the unit tests of new code were left out there.
    Block 2's install test file does not compile against the old `setEnvLines`,
    so its result is the run made before the fix.
17. **The quality pass** ran without the IDE, which this session has not:
    gofmt, `go vet` for linux, windows and darwin, shellcheck on the hook,
    a YAML parse of the OpenAPI file, and the repository's `check`.

# NOTE UpdateHistory v1.1: how the run went

Plan: `wip/2026-11-14_19-05_AP_UpdateHistory_last_updates.md`.
Run under the owner's `PLAN++` of 2026-11-24, with AP AutoUpdate.

## The pull request

Block 4 is #412, from main, not merged:

| Commit | What |
|--------|------|
| `feat(ui/update-history): The last updates show the date first and the channel` | Block 4 as planned |
| `fix(server/update-history): An update ends with the helper's outcome or apt's version; a rollback says so` | three defects of the history |

AP AutoUpdate's branch stands on #412's.

## Where the run went differently from the plan

1. **`helperFinal` split from `helperEnded`.**
   The plan ends an entry at a read "with the same `helperEnded`".
   A read uses `helperFinal` alone: `failed`, `rolled-back` or `up-to-date`,
   newer than the entry.
   The `restarting` rule stays in `helperEnded`, used at a start only.
   While the helper restarts the server the old version still answers,
   and its status names the running version as the one it came from;
   at a read that is no rollback, so `restarting` never ends an entry there.
2. **Three defects fixed on #412,** from another session's finding.
   - A second start ends the open entry with the helper's outcome where it has
     one (`noteUpdateStart`); an entry the helper has no outcome for stays open.
   - The start after an update Massimo started completes the waiting entry with
     the version it runs, whatever apt chose,
     an older one in between or a newer one.
     The old guard, `To` equal to the version or empty, is dropped.
     Where the helper had already said `up-to-date` for the entry,
     the entry keeps that outcome and the new version is from outside.
   - A rolled-back entry says "failed and rolled back" after the version it reached,
     not that version alone; `test_last_updates` expects it.
3. **The CHANGELOG split.**
   `CHANGELOG.d/Changed/` keeps the date first and the channel.
   `CHANGELOG.d/Fixed/claude-update-history-open.md` holds the three defects,
   since the history was released in 0.16.0-alpha.1.
4. **Local verification limited to what was touched.**
   `go test ./internal/server/`, `make ui`, `make ui-check`, `gofmt`,
   the three `go vet`, `test_update.py` in its four cases,
   and `GUIDELINES/shared/tools/check` ran here;
   the whole suites are left to CI.

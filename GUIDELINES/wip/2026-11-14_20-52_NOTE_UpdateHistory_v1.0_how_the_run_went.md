# NOTE UpdateHistory v1.0: how the run went

Plan: `wip/2026-11-14_19-05_AP_UpdateHistory_last_updates.md`.
Run under the owner's `PLAN++ split` (this session: UpdateHistory,
then LogAlerts), with the owner's word to base it on #259 (CopyFacts).

## The pull request

Blocks 1 to 3 are #261, on the CopyFacts branch, not merged:

| Block | Commit |
|-------|--------|
| 1 | `feat(server/update-history): The server keeps its last updates` |
| 2 | `feat(api/update-history): The update's answer lists the last updates` |
| 3 | `feat(ui/update-history): Software update shows the last three updates` |

main, with the DiscoveryShare retirement, is merged in.
#259 goes first; #261's base then moves to main.

## Where the run went differently from the plan

1. **The helper's outcome.**
   An entry the update helper ended without reaching its version gets
   `result` at the next start: `failed`, `rolled-back` or `up-to-date`.
   Without it a failed update would wait for ever, shown as under way.
   A rollback after the new version ran is read from the helper's
   `restarting` state:
   it writes `rolled-back` only after it has started the old version again.
   That start then marks the entry and adds no "outside" one.
2. **`by` and `outside`.**
   `by` is the log's form, `{name, display_name}`, not a bare name.
   `outside` says outright that Massimo did not start an update,
   so that one started with the owner's key, which has no `by`,
   is not taken for one from outside.
3. **The manager.**
   `manager` is `UpdateOptions.Manager`, set where apt or brew installs
   through the helper.
   A box another package manager owns (dnf) has none, and its updates
   read "outside Massimo" without a name.
4. **The UI suite.**
   The whole suite had 7 failures on both passes, at a load of 33 to 37:
   `test_cubes.py` (found Cube, the Cube panel's pill) and `test_log.py`
   (device names once the rooms come).
   Alone, all 12 runs of them passed, and CI's run of #261 is green.

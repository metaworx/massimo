# NOTE TestInfrastructure v1.1: decisions before block 9

The owner's answers at the gate after Block 8, 2026-11-18,
and what each makes of the plan's last steps.

## Decided

- **D10-A, with a comment.**
  A job that failed and passed on a re-run still counts, and leaves a record:
  it is named in the pull request's report and in a line of the run's summary,
  and CI comments on the pull request, so that its author is notified.
  The owner's words: "A but it should comment on the PR so that authors get
  notified".
  Block 9 does it.
- **D19-A.**
  `TestDisagreeResendsMassimosSetpoint` keeps the rule that a disagreement not
  yet reported is dropped, and starts again, while a write to the room confirms;
  a case is added that pins it: the members disagree while the write is still
  confirming.
- **D20-A.**
  `test_a_jump_down_stays_where_it_went` accepts a forward move only as large as
  what grew above the view, measured on an element above it.
- **D21-A.**
  The session "UI refactor" goes on now, before Block 9;
  it was told on 2026-11-18.

## Open

- **D22. A forward cursor for the log's API:**
  the owner asked for an explanation, given at the gate that follows.

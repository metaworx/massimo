# NOTE ServerLimits v1.0: how the run went

The run of AP ServerLimits v1.0's two blocks, from the owner's "PLAN++ all A"
of 2026-11-27, which took D1-A to D8-A.

## The run

- The plan was registered on main on 2026-11-27;
  the blocks waited for #473 (AP SteadyTests), as the plan says,
  which merged on 2026-11-28.
- Each block was built by a builder of its own, read by a second reader,
  reworked on the reading and read again, then committed, signed, here.
- Block 1 is pull request #505, on a branch from main;
  Block 2 is #508, on Block 1's branch, in two commits:
  the fix of `-stale-after` first, then the rows.
  Each branch merged main before its pull request; Block 1's met one conflict,
  in `docs/ui.md`, where main had broken the lines anew;
  Block 2's met none.
- The owner merges; #505 comes first.

## Where it went differently from the plan

- **Block 1: the window rule's refusal has a code.**
  The plan left its English text to AP ErrorCodes' Block 4,
  which gave codes only to the refusals a page could meet,
  and no page sent the window limits until this block.
  By ErrorCodes' rule ("What gets a code: every refusal a page can meet")
  the code came with the card: `setting_alert_under_warning`,
  its params the alert and the warning, its English text as before,
  its text in the six tables.
- **Block 1: a failed read of the settings.**
  It says so on the Limits rows as well as under Announce on the LAN,
  and the next read that succeeds takes it back from all three;
  under Announce it used to stay until the page was shown again.
- **Block 1: who follows a change.**
  The stream sends a setting's `change` to every subscriber, `settings` held or
  not, so the rooms are read again on every page and for every role,
  with no change to the server.
- **Block 2: the stale flag in a test.**
  The test checks that a threshold's change reads the rooms again and that a
  stale tile is drawn from that read, stubbed;
  it does not wait for the server's own flag to turn,
  since the least threshold the API takes is an hour.
- **Block 2: the shared `Question`.**
  It gained two optional props, `stay` (the focus left where it is) and
  `off` (Yes off while what it waits for runs), for the log's question:
  while it asks, the focus stays on the list, so that the arrow keys move on
  through the times and the question follows the choice,
  and Shorten waits for the row's earlier save.
  Its other users are as they were.
  The keyboard's test is skipped on macOS, where Chromium opens a closed list
  at an arrow key instead of taking its next choice.
- **Block 2: the labels.**
  One set for the card: hours up to two days, days from three, years for the
  log; Block 1's 24 h step reads "24 hr".
- **Block 2: a code built and taken out.**
  `setting_number_range`, for a whole-number setting out of its range,
  was taken out on the second reading:
  no page can send one (the log's row sends its steps, and the API keys' idle
  days field checks its own range), so by ErrorCodes' rule it keeps none.
- **Block 2: documents put right on the way.**
  `docs/ui.md`'s "Values may be old" said 24 hours for every thermostat;
  `README.md` named "System settings → Maintenance".
- **The test file** of the card is `tools/ui-test/test_limits.py`,
  renamed from Block 1's `test_window_limits.py` in Block 2.

## What is open

- #505 and #508, for the owner to merge.
- The plan's retirement, at the owner's word, with roadmap item 6 pruned.

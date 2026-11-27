# NOTE ErrorCodes v1.0: how the run went

The run of AP ErrorCodes v1.0's six blocks, from the owner's "EXEC+ as
recommended" at the plan gate, which took D6-A and D7-A, and then, during
it, "EXEC++ with parallel tasks (subagents), where possible".

## The run

- Blocks 1 and 6 were built side by side, then Blocks 2, 3 and 4 side by side
  on a merge of main and Block 1, then Block 5 on the merges of Blocks 3 and 4
  into Block 2; each by a builder of its own, checked and committed here.
- Pushes stayed as the EXEC+ gate stated them:
  the plan onto maint/0.17 and upward into main, and Block 1's pull request
  (#478, merged by the owner, then upward into main by another session).
  Blocks 2 to 6 are committed on local branches, not pushed.
- Block 4's commit was amended before any push:
  its test helpers `refusalOf` and `codeOf` met Block 2's functions of the same
  names in the merge, and became `answerRefusal` and `answerCode`.
- Every merge of two blocks met the six tables' `err.*` keys,
  each side's inserted in sorted order;
  they were resolved as the union of both sides' lines, sorted.
  The pull requests of Blocks 2 to 5 will meet the same against each other.

## Where it went differently from the plan

- **Block 1.**
  The page's helper is `refusalText`, not `errorText`:
  `rooms.js` has a global `errorText` already.
  The lock's result goes through `commandError`;
  `copySettings` gave no code until Block 2.
  On maint/0.17 a command's row and an Idempotency-Key replay drop a refusal's
  code (`withoutSecrets` strips `code`); Block 5 mends it on main only.
- **Block 2.**
  A repair's preview carries the code of its nothing or its refusal,
  or no build refusal would ever show translated;
  `refusalParams` names a `<x>_room` parameter and a nested `reason`.
  `config_valve_pct_invalid` shows the API's field name,
  `repair_hidden_room` the room 255 as a number.
- **Block 3.**
  `rebootState`, `restartState` and `updateBlocked` give their refusal as an
  error, since TestErrorCodes accepts only a declared code at a site;
  `update_none_offered` is a code of its own.
- **Block 4.**
  `user_name_bad` maps `store.ErrBadName`, a sentinel the plan left to Block 5,
  because `username.tsx` reads that refusal.
  `first_owner_not_local` cannot be reached through the API
  (the open server's door refuses first): tested directly.
- **Block 5.**
  No migration: every kept error lives in a JSON column.
  A command error of no known kind is `command_failed` with its reason;
  `issueTicket`'s other errors are 500s.
  Left without a code: the restart's `refused[]`, from the settings file's
  checks, already framed in the page's language;
  GET update's `check_error` and `source_error`;
  the System info's proxy and certificate errors.
- **Block 6.**
  The live region is `#bubble-status` (was `#copy-status`), the bubble
  `#bubble`; it follows its control frame by frame and goes when the control
  leaves the page; a scroll takes it away only where it moved the control.
  The checks of the class `copied` were in `test_maint_updates.py`,
  not in `test_pairing.py`.

## Tests

- Locally: `make ui`, the touched packages' Go tests, gofmt, vet for three
  platforms, `make ui-check`, Block 1's `make cross-check`,
  the UI files of each block in the four cases, and the guidelines check.
  The whole suites are CI's.
- Failures seen in long runs on a loaded host, each passing alone or on the
  retry:
  - `test_message_line_after_the_sign_in` (Block 1, three runs of its file;
    52 of 52 on a quieter rerun), listed in SteadyTests' NOTE;
  - `test_system_info_keeps_its_folds_a_copy_and_a_selection[390-en]`
    (Block 3), listed there too;
  - `test_reboot_waits_and_is_taken_back[390-en]` (Block 5),
    on Chromium's `net::ERR_NETWORK_CHANGED`, not listed there.

## Open

- Blocks 2 to 6 merge origin/main before their pull requests:
  Block 6 meets main in four UI test files (`helpers.py`, `test_copy.py`,
  `test_maint_updates.py`, `test_password_suggest.py`),
  Blocks 2 and 5 in `test_repair.py`.
- FR, IT, NL and PL texts are machine-translated; German by the builders.

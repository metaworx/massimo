# NOTE PasswordRules v1.0: block 5 after its ci

What the pull request's CI found after the NOTE "how block 5 went" was written,
and what was done.

## A test of another file

`test_restart.py`'s `test_store_and_settings_in_effect` failed in all four cases
of the first run.
It held the store and the settings in effect to the last two parts of System
info, and the credits' fold now stands last, for every role.
The builder had run the two UI test files the block changes;
the whole suite was left to CI, which found it.

- **A third commit** on the pull request: "test(ui-test):
  The store and the settings in effect stand before the credits".
  The test holds the two to their place before the credits,
  which is what `docs/ui.md` says.
- **Run before the push:**
  `test_restart.py` and `test_system_info.py` in all four cases, 72 passed;
  `GUIDELINES/shared/tools/check`, twelve checks well.

## The second run

One job of the second run failed on a test that is not this block's:
`test_room_marks.py`'s `test_a_burst_of_reloads_reads_the_rooms_twice`,
at 390 px in English, saw a third read of the rooms and passed on its retry.

- **Why it is not this block's:**
  the block touches System info, its rules of the stylesheet,
  one static route and tests, and nothing of the rooms;
  the same code had passed that test in all four jobs of the first run,
  and the only change between the runs was `test_restart.py`.
- The job was run again and passed, and the pull request was merged.

## For the later blocks

A block that adds a part to a page runs the whole UI suite in one case before
its pull request.
Block 2 did so on its committed tree.

## For the completion gate

The test of the rooms above joins the unsteady tests the run met,
which the gate lists.

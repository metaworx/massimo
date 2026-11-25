# NOTE PasswordRules v1.0: block 4 after its ci

What the pull request's CI found after the NOTE "how block 4 went" was written,
and what was done.

## One of the block's own tests

One job of the first run failed, at 1100 px in German,
on a test this block added:
`test_the_choice_is_kept_and_the_keyboard_reaches_the_dice`.
After Enter on the dice,
the new password's field was still empty five seconds later.
It passed on the suite's retry and in the three other cases,
and in every run before the push.
It was not run again: a new test of the block that fails once is the block's to
cure.

- **The cause, shown by the builder:**
  the test opened My profile's password fold by a click on its summary and
  pressed the dice at once.
  A fold's `toggle` event comes after the click, in a task of its own,
  and carries the fold's state to the form,
  where a change of it is a new start of its password fields.
  A press that came before the event,
  while the first press still fetched the word list,
  was dropped by that new start,
  as the block means a suggestion across a new start to be.
  With the list's request held and both in one task,
  the press filled nothing, every time, in all four cases;
  with the fold's event taken first, it filled.
- **No user meets it:**
  the event comes within the moment of the click,
  long before a person's next tap or key.
  The page stays as it is.
- **A third commit** on the pull request: "test(ui-test):
  The suggestion's tests wait for a fold's toggle before a press".
  Every test of the file that opens or closes My profile's password fold or a
  user's panel takes the fold's event before it goes on.
- **Run before the push:**
  the file in all four cases, three times, 76 passed each time;
  `GUIDELINES/shared/tools/check`, twelve checks well.
- The second run was green, and the pull request was merged.

## For other tests

A UI test that opens a fold and acts on a form inside it at once has the same
race wherever the form takes the fold's state as a new start.
The two helpers that take a fold's event are in `test_password_suggest.py`;
the suite's `helpers.py` is where they would serve every test.

## For the completion gate

The helpers' move to `helpers.py`, with a look at the other tests that open a
fold and press at once, is offered there.

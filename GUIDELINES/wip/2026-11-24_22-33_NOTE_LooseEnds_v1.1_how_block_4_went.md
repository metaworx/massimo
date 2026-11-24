# NOTE LooseEnds v1.1: how block 4 went

Where Block 4, Clock, went differently from the plan,
and what verified it.
It is tests alone: `tools/` is no shipped path.

## What it made

- **One helper,** `pause_clock` in `tools/ui-test/helpers.py`:
  Playwright's clock pauses at the page's now and 50 ms on, in seconds.
  The four pauses that handed it milliseconds call it (`test_admin_updates.py`
  twice, `test_first_owner.py`, `test_restart.py`),
  and so does the pause of `test_personal_updates.py`, which had the right form.
- **A guard** in `conftest.py`:
  a test module that calls `clock.pause_at` itself fails its collection,
  with the helper's name.
- **The README** of the suite tells the mark `clock`,
  the helper and why seconds.
- **One commit:**
  "test(ui-test): Playwright's clock pauses at the page's now, in seconds".

## Different from the plan

1. **The helper asks again.**
   The plan had one line.
   Under the trace of the suite's second pass a call takes longer,
   and Playwright then refused the pause: the time asked for was past.
   The helper catches that one error and asks again from the page's new now,
   twice as far on each time, up to four times.
   The other way: a larger fixed margin,
   which would run more of the page's timers at the pause.
2. **The helper is in `helpers.py`,** where the tests' kit is;
   the guard is in `conftest.py`.
3. **No test had passed for the wrong reason.**
   The builder listed the page's timers before each old pause:
   the jump far ahead ran each once (the poll,
   the minute's and the second's tick, a scheduled save of the settings,
   the stream's renewal, a result line's fade),
   all before the action under test.
   With the old pause and the fault planted,
   each of the four still failed for its reason.
   So no change to the page was needed.

## What the reading found

One reader read the recorded tree through git and ran the tests in its own export.
No defect; four small points, all done:

- the helper's text and the README said that no timer of the page runs until the
  test runs it; that holds after the pause,
  while the pause itself runs once what is due up to its time;
- the guard read the tokens of every test module in every worker,
  about two seconds each: it now looks for the word first;
- the README says what the guard catches: a call written out,
  not one through another name;
- the helper takes a fraction of a millisecond count too.

The reader planted three of the faults itself and saw the tests fail,
also in the second pass under the trace,
and saw the guard fail a module with a direct call and pass one with the word in
a comment, a string and a docstring.

## Seen and left

- **A server test failed once** while a UI run went beside it,
  and passed when run again; its name was not kept.
  Block 5 is about such tests.
- **The guard** catches the call as written, `….clock.pause_at(`;
  a call through another name for the clock gets past it.

## What verified it

- **The builder:**
  the four changed test files and the parts' guard in all four cases,
  438 passed; after the reading's points the same files in one case, 126 passed;
  `make ui`, `gofmt`, `go vet` for linux, windows and darwin, each exit 0.
- **The reader:**
  the five tests with the faults planted, all failed;
  without them and with `test_personal_updates.py` whole, 31 passed.
- **The coordinating session:**
  the tree's seven files, all under `tools/ui-test/`;
  the commit on its base and its merge with main, in which no file met;
  `GUIDELINES/shared/tools/check` on the merge in a scratch clone,
  twelve checks, all well.

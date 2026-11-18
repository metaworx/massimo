# NOTE TestInfrastructure v1.1: how block 7 went

Where Block 7, Ui, went differently from the plan,
what it left and for which block, and what verified it.

## What it made

- **`reads`, one answer to the page's own reads** (D13-A):
  a GET is answered with the server's data, changed by what the test told;
  the page's poll and the reads that events and actions start keep it.
  The 26 handlers that fetched and changed an answer,
  the found Cubes' fixture of pull request #348 among them,
  and 7 tests that put data into the page's state use it.
- **`isolate_log`:** the lines of the server's own events stay out of the log
  a test reads; 19 tests use it.
- **A watcher of the page's event stream:** an init script records the events
  the page's listeners handle; `Stream` waits from a mark for the next match
  and shows what came on a timeout.
- **`wait_log_live`:** a probe the log never shows, in place of 18 waits of
  1.6 s, 16 of which waited for nothing.
- **`safe_route`** in place of 18 copies of a handler that let an error go;
  **`undo` and `cleanup`:** a cleanup says what it could not undo,
  and runs every step.
- **The server sets' automatic backups are quiet** (D13-A):
  `run.sh` takes one backup of each set's Cube once it is ready,
  so none is due within a run; their poll stays.
- **`docs/ui-testing.md` §9 and `tools/ui-test/README.md`.**

## Different from the plan

1. **Who did it.**
   One builder on Opus, then one second reader;
   the builder made the reader's points;
   the session made the commit.
2. **One commit, one pull request:** the helpers and their moves share files.
3. **The backups quiet, not off:**
   no setting turns them off, and the server backs a Cube with no backup up at
   once, whatever its intervals say (`backups.go`, `w.none`).
   One backup at the set's start makes the Cube clean;
   a change of its configuration makes one due after 60 minutes,
   longer than a run.
   A test that deletes the newest backup does not bring one:
   the one before is the newest again.
   `test_backup` asserts what it did:
   its own backup is still the newest by hand,
   and its comparison with the one before now always runs.
   The builder first found no setting and left it;
   the second reader found the way without a product change.
4. **Device states the tests set stay as they were** (`updateDevice`,
   a dispatched `state` event, about 22 sites in 6 files):
   the poll does not reload the rooms while the stream is open,
   and none failed with a 300 ms poll;
   an event that reloads the rooms would take them back.
   `test_updated_is_the_cubes_age` keeps its held poll:
   the page takes the clock's skew from every answer's date.
5. **The whole suite once in one case, not twice** (D174-C):
   the full matrix is CI's;
   locally, the whole suite in 1100-en, and every changed file in the four cases.
6. **The reader's points, made:**
   the guard's problems reach a test that fails first;
   `safe_route` aborts a request whose fetch failed, where it left it open;
   every step of a cleanup runs;
   `reads` holds its page weakly;
   five docstrings corrected.

## Seen, left for later blocks

For Block 8 (Holes):

- `test_log_command_detail`: `.first` of the command lines is not tied to the
  test's own write.
- `test_log_range_is_the_last_20_by_default`: `len(kept) <= 17` is exactly 17
  once the log is isolated before the stubbed load.
- `test_log_server_lines`: its wait can now be met only by the test's line;
  a fault planted for it would show it.
- `test_refresh_now_keeps_the_list_open` and
  `test_vwt_settings_kept_and_the_disk_opens_them`:
  a fixed 1.5 s and a count of drawings in 2 s, which fail with a 300 ms poll.
- "Nothing came" after a fixed sleep: `test_new_key_shown_once` (500 ms),
  the guest case of `test_preferences` (1500 ms),
  `test_back_up_now_turns_until_answered` (300 ms).
- `test_a_jump_down_stays_where_it_went` is untouched, for the owner's reading.

## Verification

- **The page's poll at 300 ms, a build overlay,** the whole suite in 1100-en:
  before the block, 4 tests failed in both passes;
  after it, only the two not moved
  (the drawings and the pile's times), in each of the four cases.
- **The builder:** the whole suite in 1100-en, 364 passed;
  the 19 changed files in the four cases, 680 passed;
  after the reader's points, the touched files and the backup tests in the
  four cases (212 passed) and the whole suite in 1100-en again (364 passed).
- **The second reader:** the changed files in 1100-en and 390-de;
  six faults planted in copies, two of which found the guard's lost report and
  the request left open.
- **The session, before the commit:**
  the changed files in 390-de, 183 passed (load 3.3 to 1.5);
  `shellcheck` on `run.sh`; `tools/identifiers`;
  the guidelines check after the merge of `main` (12 of 12),
  once `docs/ui-testing.md`'s revision trailer and three long lines
  were put right in the commit.
- **Left to CI:** the whole matrix of the UI tests.
- **Pull request #360.**

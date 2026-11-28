# NOTE ServerBackup v1.0: the second round

The owner's `EXEC` of 2026-11-28, after the four pull requests were opened:
main's documents had been reflowed,
so #492 to #495 would conflict at their next merge of their base,
and #492 and #493 had failing tests.
The owner asked for a documents reflow on each branch before it merged
its base, and in a conflicting hunk for the branch's own text,
then `reflow.sh --fix --unwrap` on the file again.
It corrects "how the run went" where this round changed what it says.

## What changed

- **The migration is 0103,** not 0100:
  `main` took 0100 to 0102 meanwhile (AP MobileApp and AP KeyLife).
  Core's merge of `main` renamed ours,
  moved its paragraph in `docs/store.md` after `main`'s 0102,
  and wrote `store.sql` again;
  our store tests follow `main`'s two changed ones.
- **The failing test was ours:**
  `test_notifications.py` counted the notification rows without
  Block 2's new kind, which Block 4 corrected only later.
  Core now carries the count itself
  ("test(ui-test): The notification rules count the server backups' kind").
- **A reflow commit on each branch** before its merge,
  over the documents the branch changes, as `main`'s were;
  the targets' contract and CI document took a patch version each,
  and a minor one at the merge, where `main` had passed them.
- **`main`'s new test rules,** met at the merges:
  route stubs through `safe_route`, waits on `answered` and `settled`
  in place of fixed pauses and the parts guard's removed `settle`;
  a commit each on `ui` and `start`, citing the block whose test it is.
- **Code joins at the merges:** `audit.go`'s new token code beside ours,
  the log kinds `server_backup`, `battery` and `wall_reset`,
  `main`'s `update_running` refusal beside ours,
  and the new box's open-server check by the `restart_open` code.

## Where the round went otherwise

- **The conflicting hunks were joined paragraph by paragraph,**
  not taken whole from the branch:
  in several hunks `main` had added text of its own
  (its migrations' paragraphs, settings),
  which the branch's text alone would have dropped.
  Each joined document was checked to be its base's text
  with exactly the branch's word changes, then reflowed again.
- **Tests:** on each branch the Go packages of the brief and of `main`'s
  merge, `make ui-check`, and the UI files the merge touched in one case
  (core 579, targets 579, ui 606 after a fix, passed);
  on `start`'s head the whole UI suite in 1100-en, 935 passed,
  and `make target-test` passed.
- **#503 and #504,** which bring more test fixes, are still open;
  a later merge of `main` brings them.

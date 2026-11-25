# NOTE AutoUpdate v1.1: how the run went

Plan: `wip/2026-11-24_19-23_AP_AutoUpdate_install_updates_at_night.md`.
Run under the owner's `PLAN++` of 2026-11-25, D12 to D16 taken A,
after the owner's word to merge main into #414 because CI failed.

## The pull request

#414, its base main since #412's merge, not merged:

| What | Commit |
|------|--------|
| CI fix, Block 2 | `fix(update/auto-update): A file: apt source opens on Windows, and the arm test keeps the box's own index` |
| CI fix, Block 2 | `fix(server/auto-update): A second update asked for while the first starts is refused before its request` |
| merge | `merge: main, with UpdateHistory v1.1, StartConfiguration and PasswordRules, into the AutoUpdate branch` |
| merge | `merge: main, with StartConfiguration's Blocks 4 to 7 and Flakes, into the AutoUpdate branch` |
| merge | `merge: main, with AutoUpdate v1.1 and StartConfiguration's Blocks 8 to 11, into the AutoUpdate branch` |
| Block 6 | `feat(server/auto-update): The night waits while the next start refuses the settings file` |

Block 7 is not built: D16-A gives it a pull request of its own from
main once #414 is merged. #427, whose route it reads, is on main.

## Where the work went differently

1. **CI on Windows and armv6 (Block 2).**
   The tests built `file:` URLs from Windows paths,
   and the arm case's second index replaced the box's own.
   The reader now takes a `file:` URL's absolute path,
   a Windows drive letter included, and refuses a relative one.
2. **A second update at the same moment (Block 2).**
   CI's Windows service job failed at `test ! -e "$data/update-request"`:
   Block 2 had moved `updateRelease`'s read of the helper's status in
   front of the release check, so a second POST no longer saw the first
   one's helper at work. On Windows the starter's guard stopped it and
   left its request behind; on Linux it would have been a 202 and a
   second helper run. `beginUpdate` reads the status again under its lock.
   The Logs step of `platforms.yml` now prints `update-request`.
   Left as on main: a second POST between the first one's request and the
   helper's first status on Linux still passes.
   The schema 26 refusal of v0.16.0-alpha.1 in that job is not the cause:
   every run there takes the rolled-back path since migration 0024.
3. **The base.** GitHub moved #414's base to main when #412 merged,
   and the push of the merge ran CI against main, so no close and reopen.
4. **Block 6** is as planned, but for:
   the wait's info line is written at each change of the reason,
   not once a window; the radio's test also checks the postponed `why`.
5. **v1.0's text** that the build had left behind (the base on #412,
   once a night by `update.auto_tried`, Install now through
   `PUT /api/v1/update/answer`) is corrected in v1.1's Analysis.
6. **The UI test** `test_shell_updates.py::test_pages_read_at_each_showing_what_they_read`
   failed once in CI (1100-de) and passed its retry and every local run.

## Left open

- Block 7's CHANGELOG: v1.1 gives it a `Changed` fragment of its own;
  if StartConfiguration's card bullet is still unreleased then,
  amending it instead is the other reading of COMMIT.md §5.2.
- From v1.0, still the owner's: the brew receipt on the Mac,
  a real night on an apt box.

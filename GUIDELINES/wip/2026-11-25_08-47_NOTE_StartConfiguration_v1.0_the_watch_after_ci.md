# NOTE StartConfiguration v1.0: the watch after CI

Plan: `wip/2026-11-25_00-36_AP_StartConfiguration_the_settings_file_and_the_start.md`.
After `wip/2026-11-25_07-52_NOTE_StartConfiguration_v1.0_how_the_run_went.md`,
which the run's first CI of #430 followed.

## A race in the settings file's watch

#430's first CI failed `TestServeLogsTheStartsNotes` on Linux:
the test fixes the settings file right after serve's start,
and the watch never read it again.

- **The cause, older than this plan:**
  serve's watch took its first look at the file when it began,
  after the store's open.
  An edit made between the start's read and that look was taken as the
  file the start read, and missed until the next edit;
  on a slow box the window is seconds.
- **The fix, on #425's branch:**
  `fix(serve): An edit of the settings file made during the start is read again`.
  The watch starts from the file's size and time taken just before the
  start reads it (`startStamp`), so such an edit is read again, once.
  `TestWatchSettingsFromTheStart` fails on the old first look.
  A fragment, `CHANGELOG.d/Fixed/claude-watch-from-the-start.md`.
- **The stack:** merged into #427's branch, then #430's, never rebased.

## The runs after it

- #425: green at its first attempt.
- #427: its Android job lost Google's Maven repository; green at its second.
- #430:
  - its Windows job failed once in
    `internal/cubelan/fakecube`'s `TestPauseKeepsADeadSession`
    (`pause_test.go:252: EOF`), a package outside the stack;
    in the same run `cmd/massimo`, the start's guard with it,
    and `internal/env` passed on Windows with real Go;
  - UI flakes, each passing on its own retry:
    `test_confirm.py::test_unconfirmed_marks` (390-de),
    `test_maint_updates.py::test_another_setting_changed_during_the_own_put_is_read_after_it`
    (390-de), known from AP MobileApp's NOTE of its block 18;
  - green at its third attempt.

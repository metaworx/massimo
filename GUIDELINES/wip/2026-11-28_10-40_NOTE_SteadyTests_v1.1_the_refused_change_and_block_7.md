# NOTE SteadyTests v1.1: the refused change and block 7

The owner, 2026-11-28, on the run's checkpoint gate:

- **D14-A.** A settings change whose save met a 401,
  and which the page's message line reported as not sent,
  is not sent later by the page on its own:
  not with the settings it pushes after the sign-in,
  whether or not the server held the user's settings before.
  "Not sent" stays true, and the user makes the change again if they want it.
  Block 13 found it:
  `test_message_line_after_the_sign_in` checks one PUT of the change.
  On the merged tree of the upward merge with Blocks 12 to 14,
  that check failed in the whole 1100-en run, in the first pass and the retry,
  with the change sent twice.
  It passed alone and with its own file.
  The fix goes into `maint/0.17`, in a pull request of its own.
  The test sets for itself, through a kit helper,
  whether the server holds the user's settings, and checks both cases.
- **D15-A.** Block 7 goes onto #473 beside Blocks 4, 5, 6, 8 and 11,
  so that one CI run covers them all.
- **D16.** The owner asked for `PLAN` on four flakes from AP MobileApp's run
  of 2026-11-28, so that a v1.2 can take them:
  1. `TestRepairPlaceHoldsTheRoomsState` on armv6.
  2. The emulator's first boot past 420 s in CI.
  3. The app's process killed with Google Play services' process.
  4. `ConnectPageTest`'s own five-second evaluation.

  Nothing is decided on these yet.

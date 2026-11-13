# NOTE HeaderPages v1.0: what the pull request found

What [AP HeaderPages v1.0](2026-11-13_20-35_AP_HeaderPages_header_line_and_personal_page.md)
needed after its pull request #230 opened, beside
[how the run went](2026-11-13_21-11_NOTE_HeaderPages_v1.0_how_the_run_went.md),
whose item 6 it corrects in one point.

1. **The Guidelines check** found the line naming the personal page in
   `docs/api-v1.md` at 95 characters; it is broken in two.
   `GUIDELINES/shared/tools/check` runs before each push now.
2. **The LAN job**, which the change to `internal/cubelan` made CI run,
   failed "the follower does not log the Cube lost" with that line in the
   follower's log, on CI and here.
   `tools/lan-test/run.sh` runs with `pipefail`, and `grep -q` stops at
   the first match: the log still being written into the pipe dies of
   SIGPIPE, and the pipe fails although the line was found.
   Its log checks go through `has`, a grep that reads all of its input;
   main had passed by the luck of the timing.
3. **The compacted header on a phone hides its brand again**, as on
   main: kept in the page, out of sight (the first NOTE, item 6), it made
   the page jump 53 px up as the header compacted, and a switch clicked
   right after a scroll to it missed (`test_opaque.py` at 390 px, on the
   TileBadges branch).
   axe checks Notifications from the page's top instead, where the
   header and its level-one heading are whole.
4. **Verification**: `make lan-test` passes with item 2; the page keeps
   its place as the header compacts at 390 px, measured before and after;
   the UI tests of the header, the scroll, System settings, the password
   sign-in and Notifications pass, and the whole suite on the TileBadges
   branch, which holds this one.

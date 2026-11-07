# NOTE SendButtons v1.0: how the run went

The run of Blocks 1 to 5 under the owner's `PLAN++ SendButtons` of 2026-11-07,
on branch `claude/send-buttons`, pull request 89.
Every block went as planned except for the points below.

1. **Block 3: `setOff`, a helper the plan did not name.**
   `setUpdateBusy` greys Check while an update runs,
   and Check's own request kept it busy;
   at its end `setBusy` gave back the state it had before,
   so Check came back on in the middle of an install,
   and `test_busy_while_the_helper_installs` showed it.
   `setOff(btn, off)` turns a button off or on;
   a busy one keeps turning and takes that state at its end.
   `sendWhenChanged` uses it too,
   so a Save whose request fails comes back as the form now is.
2. **Block 4: `watch.expected` in `tools/ui-test/conftest.py`.**
   The test of the server's 409 makes the browser log
   "Failed to load resource … 409", which the guard fails as a console error.
   A test now lists the refusals it causes on purpose,
   by the URL's end and the status; `tools/ui-test/README.md` says so.
3. **Block 5: the Appearance shots stay as on main.**
   This machine shifts them by a pixel, as it did on 2026-11-07 for AP Redact;
   the slideshow is built with main's.
   Ten shots changed, the configuration tabs' and Settings → Cubes',
   each with its Send or Save greyed and nothing else.

Also measured: the full UI suite went from 188 tests to 232
(the new ones in `test_send.py`, `test_users.py`, `test_login.py`,
`test_cubes.py` and `test_backup.py`), about 8 minutes from this worktree;
each new test failed on the commit before its fix.

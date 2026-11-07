# NOTE Navigation v1.1: how the run went

The run of Blocks 1 to 7 under the owner's `PLAN++` and `EXEC++` of 2026-11-07,
on branch `claude/navigation`, started from `claude/send-buttons` (pull request 89).
Every block went as planned except for the points below.

1. **Block 2: no My profile item yet.**
   The 👤 menu got who is signed in, My settings and Sign out;
   My profile came with its page in Block 3.
2. **Block 3: `show_view` in the tests, three blocks early.**
   Block 3 changed every click on the settings in the tests anyway,
   so the helper that opens a page as a user does came then;
   Block 5 taught it ☰.
3. **Block 3: a viewer, not the operator, is the role without ⚙.**
   The default operator has `backup` and sees System settings → Backup and restore,
   so the test stubs `/me` with a viewer's capabilities.
   The same holds for the address test of Block 4.
4. **Block 5: the owner moved 👤 to the right of 🏠 during the run:**
   🏠 👤 📄 ⚙ at full width and 🏠 👤 ☰ below 600 px,
   in place of the plan's 👤 🏠 ☰ (D3, narrow).
   At full width the time with ↻ goes under the brand where a long version
   (a development build's) leaves no room;
   the test asks for the connection, the Cube choice and the pages on the first line.
   Below 600 px the Cube choice keeps its connection pill on its line,
   which the German "verbunden" had pushed onto one of its own.
5. **Block 6: the second server** of `run.sh` serves Cellar Cube (KEQ0000200)
   at 127.0.0.1:9, a closed port, asks discovery there too,
   and listens on `UI_TEST_DOWN_PORT`, by default `UI_TEST_API_PORT` + 1.
6. **Block 7: main's merge** brought the server's restart into Settings → Software,
   the open server's banner and CI's UI suite on pull requests;
   13 files conflicted.
   The Software panel, which Block 3 had shown only with `update`,
   is shown again to every role that sees System settings, as on main:
   Update needs `update`, Restart `restart`, and `restart` opens ⚙ too.
   Main's restart tests open System settings through `show_view`.

Also measured: the UI suite went from 232 tests to 284 on this branch,
and to 308 with main's, about 12 minutes from this worktree.
Block 1's test failed on the list icon before its fix;
the tests of the later blocks look for what their block adds.
The run was cut off twice by the app, not by the owner, between Blocks 4 and 6;
nothing was lost.
Block 6's change to `run.sh`, applied while Block 5's suite ran that script,
made `make` fail after its 272 tests had passed;
the tests were not affected.

# NOTE GlobalMenu v1.2: how Block 10 went

How Block 10 of [AP GlobalMenu v1.2](2026-11-15_11-39_AP_GlobalMenu_menu_pages_backups_and_age.md)
went where it differed from the plan.

1. **Show for a role with `cubes`:** the plan said a role that may open
   System settings. The toast's Show now follows the found Cubes'
   entries in the Cube choice, which only a role with `cubes` may
   choose: they open the same place, for the role that may add them.
2. **The mark outside ☰'s corner:** the disc is 18 px with a 12 px
   drawing, and sits 5 px past ☰'s corner, as a badge does. Before, the
   mark was 13 px, inside the corner. On a ☰ that is not highlighted,
   the disc is the button's own colour.
3. **The mark's test in `test_log_alerts.py`:** the plan named
   `test_main_menu.py`, but the mark's other checks are in
   `test_log_alerts.py`.
4. **A stale fragment corrected:** `claude-log-alerts.md` still named the
   log's button, which Block 3 moved into ☰. It now names ☰ and its Log
   entry, with the disc.
5. **The screenshot set:** only ☰'s open shots change (Sign out), and the
   slideshow with them. The toast is not in the set. The toast and the
   mark on the highlighted ☰ were looked at in temporary shots, in German
   in the dark theme, at 390 and 1100 px.
6. **The action's button drawn when needed:** the first whole suite
   failed in 23 tests, all for one cause. The toast's action button,
   empty and hidden in the page, had no accessible name, and the suite
   checks every button on every page it opens. The button is now drawn
   only for a toast that has an action, with its label, and is found by
   its class in the toast: `TestUIPage` requires every id a script looks
   up with `$` to be in the page.
7. **The whole suite left to CI (D174):** the plan's Verification named
   `make ui-test` whole. After the fix in point 6, the owner chose to run
   only the files a change touches locally, and to leave the whole suite
   to CI's UI jobs on the pull request. A second whole run was stopped
   at 61 %, with no failure until then.
8. **A flaky test, a real race:** on the last tree,
   `test_rules_kept_on_the_server` failed once at 390-de and passed on
   the retry. Its cause is older than this block. A switched rule's
   save, a second later, clears the message line when it succeeds, and
   with it a range error typed into another rule meanwhile. It is left
   to a task of its own.
9. **Verification:**
   - `test_discover.py`, `test_notifications.py`, `test_main_menu.py`
     and `test_log_alerts.py`, in all four cases, on the last tree: 79
     passed, and the one above on its retry.
   - `test_discover.py`, `test_notifications.py`, `test_settings.py`,
     `test_rooms.py` and `test_users.py`, the last three among those the
     first whole run had failed: 144 passed after the fix.
   - `go test ./...`, `make cross`, `gofmt`, the three `go vet` and
     `GUIDELINES/shared/tools/check` on the last tree.

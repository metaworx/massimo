# NOTE GlobalMenu v1.0: how the run went

How the run of [AP GlobalMenu v1.0](2026-11-15_11-39_AP_GlobalMenu_menu_pages_backups_and_age.md)
went where it differed from the plan.

1. **From main, not stacked:** the owner merged #272 and #281 before the
   run began, so the branch starts from main and its pull request goes
   to main.
2. **D135-A applied (Block 2, Block 4):** the switch holds every
   settings page, Maintenance included, so each page links to the other
   three, as the plan answered the owner's question.
3. **A long title (Block 4):** with four settings pages, System settings
   in German no longer fits beside its buttons on a phone. The buttons
   and the sections menu are one group (`.page-tools`), which the row
   wraps under its title where both do not fit; the title stays whole.
4. **System settings' roles (Block 4):** a role whose capabilities only
   Maintenance uses (`update`, `restart`, `reboot`) has Maintenance and
   no longer an empty System settings (`hasSettingsPage`).
5. **☰'s Sign out tested on a page of its own (Block 3):** the shared
   page fixture counted the 401s after the test's token was forgotten;
   the test signs in as the operator, as `test_login.py` does.
6. **Backups built once per Cube (Block 5):** System settings' folds are
   kept as they are when the page is shown again, keyed by serial, the
   approach the evaluation of 2026-11-15 recommends; a single Cube's
   fold is open. The language change no longer calls `renderPlan`
   alone: each box draws its restore.
7. **The server's clock (Block 6):** taken from every answer's `Date`
   header. The screenshot set's answers carry a fixed one, so that the
   Cube's age is the same on every run; the tests that write the time
   by hand hold the age's timer.
8. **Kind icons shared (Block 7):** `kindIcons` came out of
   `problemsEl`; the Cube choice's list and the pills draw the same.
9. **The screenshot set (Block 8):** it carries the session's earlier
   change (Cube settings and System info, System info's machine fixed,
   so that no host name or address of the machine taking the set is in
   it). ⚙'s shot is gone; ☰ open is shot at full width too. The set had
   not been taken since 2026-11-13, so 41 shots change and the old
   `user-menu` shots go.
10. **A slip:** Block 4's commit first went in with a line of
    `docs/ui.md` over 90 characters, the line check not gating the
    commit; it was amended before anything was pushed.
11. **A test missed in Block 4:** the Cube intervals' test, merged from
    main with AP CubePoll, still opened System settings; the whole suite
    on the last tree found it, and it opens Maintenance in a commit of
    its own.
12. **Verification:** the whole UI suite after Block 3 and on the last
    tree (928 passed with that fix); Blocks 4 to 7 with the tests their
    changes touch (from 36 to 308 each), not the whole suite each, for
    the run's time.
    `go test ./...`, `make cross`, `gofmt`, the three `go vet` and
    `GUIDELINES/shared/tools/check` on the last tree; each new shot
    looked at.

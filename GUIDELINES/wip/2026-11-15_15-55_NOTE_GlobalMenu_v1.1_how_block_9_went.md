# NOTE GlobalMenu v1.1: how Block 9 went

How Block 9 of [AP GlobalMenu v1.1](2026-11-15_11-39_AP_GlobalMenu_menu_pages_backups_and_age.md)
went where it differed from the plan.

1. **D159-A, the page shown as wide on every page:** the owner's EVAL
   after the plan, then `EXEC`.
   - Below 900 px its button has the width of the longest name among the
     pages the role may open (`components.js` `fitPageTabs`). The names
     are measured when a settings page is shown, after a change of
     language and after a change of the signed-in user.
   - It is at most what the row leaves beside the sections menu's place,
     which is kept on every page. The buttons after it then keep their
     places too.
   - Measured at 390 px: English 152 px, every name whole. German,
     French, Italian, Dutch and Polish reach 172 px, the row's limit:
     System settings' name ends in "…" in all five, and System info's in
     Polish.
   - A name is measured to the fraction, through a `Range`. `scrollWidth`
     rounds it down, and the longest English name lost its last letter.
2. **Cube settings' sections menu:** the owner's word with the `EXEC`.
   - Since Block 5 the page has two parts: the Cube's panel, and Backup
     and restore. Its row now has the sections menu, which lists both.
   - The panel is not a section card, as it does not fold. A part marked
     `page-section` is listed by its `data-toc` ("Cube"), and `showCard`
     focuses its summary.
   - The entries show names without icons, as in System settings and
     Maintenance.
3. **900 px, not 600:** below 900 px, every button but the page shown's
   shows its icon alone. All four names do not fit beside the sections
   menu in the longer languages at 600 px; Italian's longest name alone
   is 160 px.
4. **Not bold:** a bold name widened the page shown's button by 8 px at
   full width, which moved the buttons after it.
5. **System info's badge:** its button is the last one, so the badge
   shown where a proxy fails narrows that button alone. This is the
   owner's correction of the EVAL's note.
6. **The pointer:** `screenshots.py` moves it off the page before each
   shot. Besides the settings pages, eight room and configuration shots
   change, as the setpoint's ▲▼ and a chip's hover are no longer drawn.
7. **Verification:**
   - `test_page_switch.py` and `test_cube_page.py`: 28 passed.
   - The whole UI suite: 928 passed on the tree before D159-A, and
     932 on the last tree.
   - `go test ./...`, `make cross`, `gofmt`, the three `go vet` and
     `GUIDELINES/shared/tools/check` on the last tree.
   - Each new shot looked at.

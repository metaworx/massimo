# NOTE GlobalMenu v1.3: how Blocks 11 to 13 went

How Blocks 11 to 13 of [AP GlobalMenu v1.3](2026-11-15_11-39_AP_GlobalMenu_menu_pages_backups_and_age.md)
went where they differed from the plan.

## Block 11: PileParts

1. **A list well without `ok` shows no check.** Block 12 needed a list
   that stays where nothing is wrong (`always`) without the green check
   in its summary, where its lead says the state. The check is drawn
   where something is wrong or `ok` is given; that went into Block 11's
   own commit before its push.
2. **No CHANGELOG bullet:** nothing changes on the page until Block 12,
   which carries it.
3. **Stacked on #347, then on main:** #347 was merged as one commit, and
   GitHub then saw #350's copies of its changes as conflicts. A merge of
   main went through without one.

## Block 12: HeaderPile

4. **`ServerSign.tsx` and `CubeProblems.tsx` stay modules:** the plan
   folded them into `Pile.tsx`. They give the pile's lead icon, its
   state's item and its entries; the pile builds them.
5. **`pollNow` goes:** Refresh now turns with `useBusy`, as every item
   that sends does, so `send.ts` keeps no poll of its own.
6. **Refresh now ticks itself:** a part that reads a signal is not drawn
   again with its parent, so the times would not move from the pile's
   timer. Refresh now keeps the one-second timer itself while the list
   is open, and the pile is not drawn every second.
7. **The Cube's time on the browser's clock:** "Cube read" is the
   Cube's time less the server's clock skew, as the age above the pile
   counts it.
8. **One time-of-day formatter:** the log's `fmtLogTime` is now
   `fmtTimeOfDay` in `app.js`, shared by the log and the pile.
9. **The line of its own measured with its gaps:** the pile's effect
   counts the row's tracks from `gridTemplateColumns` and adds their
   gaps; without them the pile kept the row too long. With them, 200 px
   is a line of its own already before the header compacts, so the test
   takes 230 px.
10. **The tests changed beyond the plan's list:** `test_tls.py` (a poll
    in place of the refresh button), `test_cube_problems.py` (the
    server's sign first among the pill's icons, the list's ok line),
    `test_system_layout.py` and `test_log.py` (`fmtTimeOfDay`).
    `test_header_updates.py` and `test_menu_fit.py` pass unchanged.
11. **The second reader's findings, fixed:** the missing gaps, the
    Cube's time on the server's clock, the copied formatter, the pile
    drawn every second, a lost comment of `checkVersion`, stale comments
    and docs, two dead CSS rules, and the pile's label without a Cube.
    Not done: `state.lastPoll`, which is older than this block.
12. **Flaky tests in the pull requests' runs:**
    - #351: `test_discover.py::test_found_cube_with_removed_rows`, which
      #348 on main fixes; main merged in.
    - #350: `test_cube_page.py::test_cube_page_saves_as_system_settings_shows`
      looked at the outcome under Cube settings' panel before the
      action runner had said it, after its reload. It failed in English
      and then, the name left changed, in German. It now waits for the
      line with `wait_until`, the UI tests' wait until AP
      TestInfrastructure's Block 7.

## Block 13: TileNotices

13. **Show's pill closed as it opened:** the page closes an open menu
    on a click outside it (`components.js`), and the tile's Show is
    outside the pill. A first fix stopped Show's click, which the second
    reader found hid it from every other listener, a sent House panel's
    among them. Show now opens the pill after the click.
14. **The second reader's findings, fixed:**
    - a finding that names no device of the room said only the device
      row's words under the room's name; it now says its whole sentence;
    - the not-ready icon was the radio's tower, one of the radio's own
      drawings; it is a pause sign of its own;
    - the line counted the stale devices as one and the pill each; the
      line now counts the pill's own entries;
    - the pill said "not reported" of a device the row calls out of
      date; it says what the row says, with its age;
    - the since badge and the link look were second copies; `Since` is
      in `badge.tsx` beside `Age`, and Show shares the found Cubes' rule;
    - `staleNote`, stale comments and docs went;
    - no test covered the not-ready line: `test_not_ready_line` does.
15. **The screenshot set taken again:** the plan did not name it, but
    the set makes the first room stale, and its tile showed the old
    note. 21 shots and the slideshow changed.
16. **A test that read the list too early:** with Show opening the pill
    after the click, `test_mismatch_marks` read a badge before the list
    was open, once at 1100-en. It waits for the list now, as the stale
    test does.

## The merges

17. **Merged at the owner's word:** #350 and #351 were merged into main
    with merge commits, each once its checks were green, and Block 13
    was built from main after them. #351's Android job had failed on
    Maven Central's 429 (too many requests); its re-run passed.

18. **Verification:**
    - Block 11: `test_problem_list.py`, `test_header.py`,
      `test_parts_guard.py` and `test_main_menu.py` in all four cases,
      152 passed; after main's merge, `test_cube_page.py`, 20 passed.
    - Block 12: thirteen files in all four cases, 284 passed; after the
      second reader's fixes the header's, the log's, the guard's and the
      sign's, 260 passed; after main's merges `test_discover.py`,
      `test_cube_page.py`, `test_header.py`, `test_server_sign.py`,
      `test_sign_in_resend.py` and `test_login.py`.
    - Block 13: `test_rooms.py`, `test_parts_guard.py`,
      `test_room_updates.py`, `test_log.py`, `test_unsaved.py`,
      `test_problem_list.py` and `test_cube_problems.py` in all four
      cases, 423 passed and the mismatch test above; the notice tests
      three times after its fix.
    - Each block: `make ui-check`, gofmt, the three `go vet`,
      `go test ./internal/server/ ./tools/...` and
      `GUIDELINES/shared/tools/check`. The whole UI suite, `make race`
      and `make cross` are CI's (D174).

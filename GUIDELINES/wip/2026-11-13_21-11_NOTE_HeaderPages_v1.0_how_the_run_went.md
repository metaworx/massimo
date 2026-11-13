# NOTE HeaderPages v1.0: how the run went

How [AP HeaderPages v1.0](2026-11-13_20-35_AP_HeaderPages_header_line_and_personal_page.md)
went under the owner's `PLAN++` with the recommended picks of 2026-11-13,
on the branch `claude/header-pages`, where it went differently from the
plan.

1. **The tight logo ends at the last letter's ink** (Block 1).
   The letters' advance would leave the "o"'s right side bearing as a
   margin of about 1 %; the tight width takes it off.
   `brand.css` names the mark's proportions too (`--brand-mark-ratio`),
   the tight mark being 460 by 422, not square.
2. **The word under the logo is one span** (Block 2).
   The h1 is a grid of one cell: the word's two spans, "M" and "assimo",
   each took that cell and lay over each other until the sprite came.
   Wrapped in `.brand-word`, the word is one item; it stands at the
   cell's foot, and the drawing, aligned by its baseline, gives the h1
   its baseline, which a box without one takes from its bottom edge.
3. **The refresh button's tap area is about 41 px high, not 44**
   (Block 2): any more reaches over the Cube choice under it on a phone.
   A tap 8 px above or under it still hits it, or the control that is
   there. While it asks, its icon turns, in place of the ring after the
   text that other buttons show.
4. **The Cube's clock needed the server** (Block 2):
   `cube_time` did not say when it was read, so `cubelan.Snapshot` keeps
   the greeting's time (`HelloAt`) and a Cube's summary carries it as
   `cube_time_at`, with a test and both API documents.
5. **↑ follows the first card's head** (Block 3): it watched the row of
   section buttons that went; back at the top the focus is on ☰.
6. **The personal page's details** (Block 4):
   - the cards' bodies are `#profile-sec`, `#prefs-sec` and
     `#notify-sec`: Notifications' partial already has `#notify-body`;
   - the rule that widened My settings to 760 px for the colour editor
     would have narrowed the whole page; My settings now takes the grid's
     whole width while the editor is open;
   - the compacted header on a phone puts its brand out of sight rather
     than out of the page: a page opened at a card is scrolled, and axe
     found no level-one heading on Notifications at 390 px;
   - 👤's tooltip and label say "Personal settings", of whom where a user
     is signed in; the display name's help names the page;
   - the page's title wraps on a phone, beside Forget key.
7. **Tests**: `show_view` of a card waits until the menu's smooth scroll
   has settled; a switch clicked during it missed at 390 px.
   `test_oidc.py` checks the view `me` and the address `#profile`.
8. **The screenshot set**: the shot `user-menu` is now `me`;
   `docs/screenshots/` is CI's to take (`GUIDELINES/project/CI.md`),
   so its old `user-menu` files stay until the next set.
9. **Verification**:
   - every scope of the brand tool but `svg` and `ui` wrote what v1.2.0
     wrote, byte for byte (`grayorange`, both themes); `svg` added the
     six tight files; the tight drawings fill their boxes to the pixel;
   - `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make cross`,
     `gofmt -l .` and `go vet` for linux, windows and darwin;
   - `make ui-test`, the whole suite: 643 of 648 at first, the five
     fixed in item 7, then those files again, 96 passed;
   - the header and the personal page looked at, 1100 px light and
     390 px dark.

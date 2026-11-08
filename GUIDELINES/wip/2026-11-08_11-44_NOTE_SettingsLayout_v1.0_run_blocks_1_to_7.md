# NOTE SettingsLayout v1.0: run blocks 1 to 7

How `EXEC++ 1-7 pr` (2026-11-08) went where it differs from the plan,
[AP SettingsLayout v1.0](2026-11-08_09-53_AP_SettingsLayout_settings_pages_layout.md).

## Deviations

1. **Block 2** also did the two columns inside a Cube's panel
   (roadmap item 15.8), which the plan's Block 2 left out though the mock
   showed them, and each backup's actions at the right of its row.
   The Cube panel's form lends its parts to the grid (`display: contents`).
2. **Block 3** needed three fixes its own suite found before it was pushed:
   - Compacting the header shortened a page little taller than the window
     until it could no longer stay scrolled, and the page jumped to and fro
     on a phone (a pairing test failed now and then). The compacted header
     keeps the height it gave up as its margin, and the page's row has its
     border before it shows; a test holds the page as long scrolled as not.
   - `--top-h` went stale at 1100 px: the `ResizeObserver` watched the
     header's content box, which compacting (a padding change) leaves alone.
     It watches the border box now, and compacting sets it as well.
   - The version no longer wrapping ran a build's long version under the
     page buttons at 390 px; it ends in … now.
3. **Block 4**: the 👤 and ☰ menus are markup, so their icons needed no
   script; `menuItem` came with Block 5, its first caller built by script
   (the Cube choice).
4. **Block 5** also showed the address in a Cube panel's head and the
   serial in the Rooms notice's detail, both D5-B; `header.cubeDisconnected`
   went with the select that used it. Existing UI tests moved from the
   select to the menu (`test_cubes.py`, `test_discover.py`,
   `test_header.py`, `test_home.py`), with `choose_cube` and `cube_item` in
   `helpers.py`.
5. **Block 6**: the hidden devices show the radio address always, since
   Hide takes it; the dot between serial and radio address is text, not
   CSS, which neither copying nor `inner_text` sees.
6. **Block 7**: My settings is markup, not built with `fieldRow`, so the
   plan's `fieldRow(…, { deviceOnly })` and `deviceTable` would have had no
   caller and were not written; the column is the rows' CSS, the tooltip
   `renderDeviceSwitches`'.
7. **Block 2's commit** was amended before any push: its CHANGELOG bullet
   was 74 characters wide. `changelog.sh check --staged` says so in its
   message with exit status 0, so a chain on `&&` had committed it; the
   later blocks were committed on the message.

## How it was verified

Every block's state ran the whole UI suite (`make ui-test`, 1100 and 390 px,
English and German): Blocks 1, 2, 5 and 7 here, Blocks 3, 4 and 6 in
temporary worktrees holding exactly their commit's tree. Block 3's later
fixes ran the layout, header, pairing and rooms tests on Blocks 3 and 4,
and the final tree the whole suite again. Every block ran the Go checks
(`gofmt -l .`, `go vet` on linux, windows and darwin, `go test ./...`,
`make cross`). The pages were looked at in screenshots at 1100 and 390 px,
light and dark, from a `serve` against the fake Cube.

# NOTE GlobalMenu v1.5: the pile's list placed as it opens

A decision beside [AP GlobalMenu v1.5](2026-11-15_11-39_AP_GlobalMenu_menu_pages_backups_and_age.md),
taken by the owner on 2026-11-18 on a finding of the session "AP
TestInfrastructure": `test_header.py::test_pile_list_as_wide_as_its_content`
failed at 390 px on a pull request that changed neither, and passed on
the retry.

The cause is Block 14's. The pile's list hangs right-aligned from the
pill (`.menu-items`), which at 390 px puts it out of the window, and
`dropdown()` of `components.js` places a list only in a frame after its
`toggle` event, one frame already drawn: a test that reads the list at
once finds it out of the window, and a user sees it there for a frame at
every open. And an open list is placed for where its summary was: when
the pill grows or the row folds, it hangs out of the window, at x −128
or past the right edge. The same race made
`test_cube_problems.py::test_header_problems` flaky at 390 px. That
session measured both on main and with its fix, and wrote the fix as a
diff it did not apply.

- **D202-A:** its fix whole, the product and the tests: a list placed as
  it opens, before it is first drawn (a `MutationObserver` on `open`);
  every open list of the header's row placed again after each fit of the
  row (`placeMenus`, from `fold.ts`); a list placed again keeps its
  scroll; the tests wait for the list placed, and one checks that it
  follows its pill.
- **D203-B:** on a branch of its own from `main`, a pull request of its
  own; the one that merges after the other (this or the Cube choice's
  colour) merges the other's line of `fold.ts`.
- **D204-A:** this NOTE, a fix of Block 14's behaviour, not a block of
  its own.
- **D205-A:** `test_cubes.py::test_cube_facts`, which fails in the four
  cases on main too (a device's problem an earlier test leaves on the
  server), goes to a session of its own, from a chip.

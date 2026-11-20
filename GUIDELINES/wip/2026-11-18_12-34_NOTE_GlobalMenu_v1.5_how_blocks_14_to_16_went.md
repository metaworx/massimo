# NOTE GlobalMenu v1.5: how Blocks 14 to 16 went

How Blocks 14 to 16 of [AP GlobalMenu v1.5](2026-11-15_11-39_AP_GlobalMenu_menu_pages_backups_and_age.md)
went where they differed from the plan.

## Block 14: HeaderRow

1. **Built twice, pushed once:** Block 14 was first built as v1.4 had
   it (the whole logo on a phone, the pile and the Cube choice on a
   second row, a group of them that gave way inside itself). The owner's
   second look turned it into the folding header of v1.5 before it was
   pushed, and its commit was rebuilt in place. The group went again: the
   row is flat, and measured.
2. **Rebased onto main, not pushed yet:** the message check wants the
   plan revision a commit names in its history, and the branch started
   from v1.4. The unpushed branch was rebased onto main, and main was
   merged once more before the push, for CI.md's flowchart.
3. **Main's Block 8 met on the way:** AP TestInfrastructure's Block 8
   had rewritten the two header tests that Block 14 replaces. Block 14's
   tests were kept, with Block 8's held poll, the busy check of Refresh
   now, the read of the rooms and "closed, the times stand still". So the
   list's times follow the clock only while the list is open, where the
   first build had them tick always.
4. **The clock set at each render:** the first test of the times found
   them a second short, the clock's last tick older than the data; the
   header's render now sets the clock as the data come.
5. **The version signed out:** with the whole logo on a phone, the
   version beside it signed out scrolled the page sideways at 390 px; it
   wraps under the logo now.
6. **A shot of the list open:** the pile's list open is a shot of its
   own, at 1100 and 390 px, which the set did not have.
7. **Not done:** until the header's first render the row is not measured,
   so on a phone the whole logo shows for a moment and then folds to the
   M. The render comes with the first answer.

## Block 15: TileLine

8. **As planned,** carried over onto the rebuilt Block 14 unchanged.

## Block 16: CubeSign

9. **The link's row as the list's head:** the plan had the panel's pill
   always carry a "connected" badge and leave the link's kind out of the
   summary (a `skip` for `Problems`). A kind is named whatever its tone,
   so the label would have said a problem of the network. The row is the
   pill list's `head` instead (`CubeLink`), and the Cube's own entry has
   no badge of the link; no `skip` was needed.
10. **The House tile's text key:** `house.allRooms` was taken ("All
    rooms"); the tooltip's key is `house.roomsOf`.
11. **The House title follows the Cubes:** a rename the poll brings does
    not load the rooms again; `MassimoParts.rooms.cubes()`, called by
    `renderCubeSelect`, draws the title again, and it names the Cube
    whose rooms are shown.

## The run

12. **A second reader** checked the three blocks after their commits. It
    found no loop in the fold. What it found was fixed in a commit of its
    own: the cube button empty before a Cube is chosen; a Cube's cube
    blue on hover in the choice's list; the House title and a rename; the
    panel pill's label saying "not connected · nothing wrong"; a stale
    current time as the list opens; expert mode's times moving while
    hidden; the fold missing a pill that shrank on its own row; stale
    docs and comments; dead CSS; `cubeEntry`'s `own` meaning two things.
13. **The shots once:** the set and the slideshow were taken once, after
    all three blocks and the fixes, in a commit of their own.
14. **The plan's commit gated late:** v1.5 was pushed onto main before
    its message check had passed: the push was not tied to the check's
    exit code. Checked afterwards as history, it passes. A message that
    names a plan revision the same commit adds fails the check as a
    file; the check as history is the one to gate on.
15. **Two runs stopped by the owner** (the whole touched set once, the
    vet and the guidelines check once); each was run again.
16. **Verification:**
    - UI tests in all four cases: the header's files after the rebuild,
      156 passed; Block 16's files, 500 passed, one test changed for the
      sign's new place; the other touched files, 296 passed; after the
      second reader's fixes, 448 passed and 60 skipped; the notice tests
      of Block 15 twice, 12 passed each.
    - `make ui-check`, gofmt, the three `go vet`, `go test
      ./internal/server/ ./tools/...` and `GUIDELINES/shared/tools/check`
      on the last tree. The whole UI suite, `make race` and `make cross`
      are CI's (D174).

# NOTE SystemInfo v1.0: how the run went

How [AP SystemInfo v1.0](2026-11-13_00-45_AP_SystemInfo_server_facts_in_their_places.md)
went under the owner's `PLAN++ SystemInfo` of 2026-11-13,
where it went differently from the plan.

1. **Block 1** went as planned, in pull request #198,
   which the owner merged during the run.
   Blocks 2 to 6 are pull request #202.
2. **The flicker's cause was another** (Block 2).
   The Analysis took the header for the scroll anchor.
   `overflow-anchor: none` on `.top` changed nothing:
   the test still went from 96 px to 0 at the next step.
   Measured: `stickyHeader` measures the compacted header before it sets the
   margin that keeps the page's length,
   so the page is laid out once with what follows the header moved up,
   and the browser's anchoring takes the scroll back by as much.
   The fix is in `components.js`: the anchoring is off while the header changes,
   until the page is laid out with the margin.
   `style.css` is unchanged by the block.
3. **The link's jump on a phone** (Block 3).
   At 390 px the smooth scroll to the Cubes card stopped short:
   the cards above it grow as their data comes,
   and a smooth scroll keeps the target it started with.
   It also did so with Block 2's change taken out, so that was not the cause.
   `showCard` brings the card at once for a page just opened,
   and again while the cards around it change size for two seconds,
   until the user scrolls (`keepCardInView`);
   the section buttons keep their smooth scroll.
4. **`discover.hintAdd` went** (Block 3):
   "the server keeps it in its store, under System settings → Cubes",
   said inside System settings → Cubes.
5. **System Info's script is `update.js`'s**, beside Software update and
   Maintenance, rather than a file of its own (Block 4).
   From 900 px System Info and Software update share the first row,
   and Maintenance has the next to itself,
   where it had shared one with Software update.
   `certText` serves the header and the tile.
6. **`os_name`, beyond the fields the plan named** (Block 4):
   the distribution's name from `/etc/os-release`,
   for "the system it's running on"; absent on Windows and macOS.
   The client's row reads "This browser's address",
   which needs no "du" or "Sie" in German.
7. **The found line's contrast**, a fix of Block 3 in a commit of its own,
   "fix(ui): The found Cubes' line in the text's colour".
   The line kept the warning colour of the pane it replaced:
   4.33 to 1 on the page's background, under WCAG AA's 4.5.
   axe refused it in the Notifications tests whenever a found Cube was listed,
   which first showed in Block 5's run;
   the suite's retry, on servers that had found none, passed them.
   The line is in the text's colour, and Block 3's bullet is amended,
   as for a change of this release.
8. **Verification:**
   - Block 3's jump (item 3) was tested with Block 4's files in the working tree;
     Block 3 alone then passed its 64 tests in a worktree at its commit.
   - The contrast fix (item 7) was tested with Block 6's files,
     in the run that passed all 588 tests.
   - `TestStreamResyncs` failed once while the UI suite ran beside the Go tests,
     and passed three runs of its own and the next full run.
   - Block 5 was committed after a changelog check that failed,
     the two chained without a gate:
     the check had found Block 6's bullet, then untracked, one column too wide.
     Run again with that bullet set aside, it passed at Block 5's commit.
   - After the blocks, the branch took `main` with Block 1 by a merge;
     the merged tree passed the Go gate, `make cross` and `make ui-test`.

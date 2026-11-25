# NOTE AutoUpdate v1.1: how block 7 went

Plan: `wip/2026-11-24_19-23_AP_AutoUpdate_install_updates_at_night.md`.
Run under the owner's `EXEC block7` of 2026-11-25, after #414's merge.

## The pull request

#445, from main, not merged:

| What | Commit |
|------|--------|
| Block 7 | `feat(ui/auto-update): Update and Install now say the server stays down until the settings file is fixed` |
| merge | `merge: main, with ServerTraffic, RereadOnce and RoomsPage's Blocks 1 to 3, into the update question branch` |

After the run of v1.1's Blocks 1 to 6, #414 took one more commit, of its
CI: `test(cubelan): A test that waits for the fake's freed slot keeps the connection that found it`.
Two tests of the one-client fake Cube dialled again right after a probe
that held its slot, which ends before the greeting on Windows.
On Windows they failed 18 of 300 runs before, none of 300 after.

## Where the work went differently

1. **`updateQuestion` is in `Ask.tsx`**, not `Software.tsx`:
   Install now's question needs it there too, and `Software.tsx` imports
   `Ask.tsx` already, so the plan's place would have made the parts'
   first import cycle.
2. **Install now turns while it reads**, as Update does,
   and Tonight and Skip this version are off meanwhile,
   so that neither answer races the `now`.
3. **After its read**, Install now does nothing where a load meanwhile
   offers another version or none, and Update asks nothing where no
   update can be installed any more: the question never names a release
   the page knows to be gone.
4. **The look.** `.update-card-ask` keeps the card's lines' margin and
   wrapping as a `div` (`4px 0`, `overflow-wrap: anywhere`); its old
   `6px 0` never applied under `.settings .update p`. The question's box
   sits 6 px under the question's text.
5. **The tests.** `test_update_asks_in_the_page` and
   `test_update_through_apt` wait for the question's box, which now comes
   after the read. The whole UI suite ran once in 1100-en before the merge
   of main: 814 passed and `test_new_batteries.py::test_new_batteries_sent`
   failed once on a detached button, passing its retry
   (main has since fixed that test).

## Left open

- No UI test for a sign-out or a newer version taking Install now's
  question back; the plan's list names none, and the code does it.
- From v1.0, still the owner's: the brew receipt on the Mac,
  a real night on an apt box.
- AP AutoUpdate retires at the owner's word, once this pull request is merged.

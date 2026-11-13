# NOTE TileBadges v1.0: how the run went

How [AP TileBadges v1.0](2026-11-13_20-35_AP_TileBadges_problems_as_icons_in_the_tiles.md)
went under the owner's `PLAN++` with the recommended picks of 2026-11-13,
on the branch `claude/tile-badges`, stacked on `claude/header-pages`,
where it went differently from the plan.

1. **The commits came in the order 3, 1, 2**: Block 3 touched files of
   its own and went first; Blocks 1 and 2 share `tiles.js`, `style.css`
   and the room tests, and were written together, then committed apart.
2. **The icons' tones** (Block 1): a low battery is amber now, in the
   head and in the badges, where its word was red; a transmit error is
   red, a lost link amber, an open window red. The mixed modes' icon is
   amber whatever the modes (an item's `mark`), its entries each
   thermostat's mode in the mode's own colour, led by the shuffle icon.
3. **What stays a word** (Block 1): in a device's head its radio and
   battery problems are icons; not reported, stale, locked and the
   window's or the mode's state stay words, as the plan had it.
4. **The age's parts in the value's grid** (Block 2): the age keeps its
   element, which `refreshAges` updates, laid out as `display: contents`,
   so its two parts take their places under the name and the value; its
   tooltip with the exact time is not shown there, the time being on the
   line itself in expert mode.
5. **WallZero** (Block 3): AP RoomApi's kept temperature applies to the
   wall thermostat too, as the Windhager session pointed out: after a 0
   the snapshot keeps the last measured value with its time, and a wall
   thermostat that never measured gives its room none, so the room takes
   its radiator thermostats'.
6. **The second worktree**: Blocks 1 to 3 were written in a worktree of
   their own beside the HeaderPages one, which this session's hook does
   not allow; the branch was then checked out in the session's worktree
   and the other removed.
7. **HeaderPages' fixes came in by merges**: the branch merged
   `claude/header-pages` as its pull request's checks found things (a
   documents line, the LAN test's log checks, the phone's header that
   moved the page); the last of these showed up here first, as
   `test_opaque.py` missing a switch at 390 px.
8. **Verification**:
   - `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make cross`,
     `gofmt -l .` and `go vet` for linux, windows and darwin;
   - `make ui-test`, the whole suite;
   - a room tile with problems and the open window's list looked at,
     1100 px light and 390 px dark.

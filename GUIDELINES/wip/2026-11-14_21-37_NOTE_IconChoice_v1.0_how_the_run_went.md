# NOTE IconChoice v1.0: how the run went

The run of `PLAN++ split` on 2026-11-14 in its own session,
after AP RoomTile, Blocks 1 and 2 on `claude/icon-choice`,
pull request #266,
and where it went differently from the plan.

## Block 1

- **The sash has no small form.**
  The plan named "the sash set (closed frame, open sash, its small
  form)"; the mock (version 8) put the sash's mark inside its frame,
  so the sash keeps its size and is its own base.
  The tower has its small form, moved aside as the mock drew it.
- **The triangles carry their exclamation mark**, as AP Signs' other
  marks do; the mock's version 8 drew them bare.
  The crosses are the mock's.
- **`SIGNS` keeps the network, the cog and the battery;**
  the radio and the window are `ICON_CHOICES` in `components.js`,
  `signOf` giving `signEl` either.
  `PROBLEM_KINDS` holds no icon for the radio and the window:
  `problemIcon` gives the chosen one.
- **The setting's names are read in Block 1** (`ICON_SETTINGS`),
  the default without one, so that Block 1's tests set the choice in the
  page before Block 2 adds the setting.
- **`test_rooms.py`'s open window** is the sash's now, the default
  having changed; the fragment of Block 1 is under Changed for that
  reason, Block 2's under Added.

## Block 2

- **A preview beside each choice**, the window closed, open, amber and
  red, the radio amber and red: a select cannot show a drawing.
- **The page drawn again** (`redrawSigns`) wherever the choice changes,
  from the settings page or from the user's settings arriving from the
  server: the rooms, the Cubes and the header's problems.
- **`test_preferences.py`** counted the switches of My settings:
  nine now, ten cells in the device column.

## Merging with RoomTile

A trial merge with `claude/room-tile` (#262) conflicts only in
`tiles.js`'s `PROBLEM_KINDS`:
RoomTile's holiday kind beside this branch's `problemIcon`.
Whichever pull request merges second takes main and resolves it.

## The gate

- **`make ui-test`**: 743 passed.
  One `test_header` test failed while the race tests ran beside the
  suite: the page's `/me` request did not come within its 5 s.
  It passed on the retry and again alone, with `test_icon_choice.py`.
- **The rest:** `go test ./...`, gofmt, the three `go vet`,
  `make cross`, the race packages with `-race -count=1`,
  and the guidelines check (12, all well).

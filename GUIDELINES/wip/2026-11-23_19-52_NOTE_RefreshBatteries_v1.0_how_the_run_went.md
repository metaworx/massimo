# NOTE RefreshBatteries v1.0: how the run went

The plan's five blocks, run on 2026-11-23 after `PLAN++ as recommended`,
on the branch `claude/refresh-batteries`, as one pull request of five commits:
"feat(api): A Cube's device list can be read again at once" (Block 1),
"feat(ui): Refresh now, in Maintenance and beside a Cube's Reboot" (Block 2),
"feat(api): New batteries make a device's battery level ok at once" (Block 3),
"feat(ui): A device's row offers New batteries" (Block 4)
and "feat(ui): The battery levels say what to do" (Block 5);
then "merge: main into claude/refresh-batteries",
which brought AP RoomMajority v1.0 and v1.1, plan files only;
pull request #407.
An Opus sub-agent built each block without a commit;
the session read each block's diff before its signed commit, with no second reader.

## How it ran

- **Checked per block**, each exit 0, every Go test with `MASSIMO_ENV_FILE=/dev/null`:
  `go test` of the Go packages the block touched;
  `gofmt -l .` and `go vet` for linux, windows and darwin; `make ui-check`;
  the UI test files the block touched, in the four cases (1100 and 390 px,
  `en` and `de`).
- **Checked on the merged branch**, each exit 0: `make ui`;
  `go test` of `internal/battery`, `internal/notify`, `internal/server`
  and `tools/apitypes`; `gofmt -l .`; `go vet` for the three systems;
  `make ui-check`; `GUIDELINES/shared/tools/check`.
- **Left to CI**: the whole Go suite, the race job, `make cross`, Windows,
  armv6 and the whole UI matrix.

## Departures from the steps

- **The refresh route's answers** (Block 1): `409` while the Cube's serial or
  address is not known, while it reboots, or when a newer refresh replaced the call;
  `502` with the reconnect's error.
  Its audit line is a change line, as a Cube's reboot's is.
- **Two tables of capabilities** (Block 1).
  In `docs/api-v1.md` a paragraph ("A user changes only users…") cut the table
  in two, so that the rows from `update` on had no header;
  it moved below the table, a defect fixed on the way.
  `docs/store.md` §6's table of capabilities and routes had no `restart` row;
  one was added.
- **The log's action texts** (Block 2) for `cube.refresh`,
  and for `cube.reboot`, which had none before.
- **Maintenance's "Refresh the Cubes" is a row of its own** (Block 2),
  needing `restart`, not a button beside the refresh interval,
  which needs `settings`: so a manager sees it.
- **The French button** (Block 2) reads "Rafraîchir maintenant",
  since "Relire maintenant" is the Reload button already.
- **New batteries' answers** (Block 3): a device without a current report gets
  `409`, as one whose flag is on; `from` is always in the answer;
  the Cube itself is refused with `400`.
  The rebuild replays the device states and the marks by time, then by id.
- **A stale `api.d.ts`** (Block 3).
  Block 3's first commit left the generated `internal/server/ui/src/api.d.ts`
  stale, which CI's `tools/apitypes` test would have failed.
  Block 4's builder found it, and it was amended into Block 3's commit
  before anything was pushed.
- **A `battery` event from the stream** (Block 4) set no device's level on the page:
  it had no listener. One was added; battery events reach the live log now too.
- **The button is first among the row's actions** (Block 4), not beside the
  battery badge: that badge is an icon in the row's problems summary,
  with no place beside it.
- **The flag's own log words were renamed** (Block 5), against the Analysis,
  which kept them: "battery warning", in German "Batteriewarnung",
  so that the flag does not read as a level beside the new words.
- **`problem.battery` reads "batteries to replace"** (Block 5),
  since it names a kind inside a comma-separated list.
- **No short form at 390 px** (Block 5, D12-D's "where a badge does not fit"):
  in the 240 px problem lists the longer words wrap to two lines inside the badge,
  at both widths, so none was needed.
- **The English badges keep the owner's capitals** (Block 5),
  "Batteries ok", "Batteries weak, replace soon", while other badges are lower case.

## Found, not changed

- `eventKinds` (`internal/server/log.go`) and `LOG_KINDS`
  (`internal/server/ui/src/log/kinds.ts`) lack `battery` and `wall_reset`:
  the log's kind filter cannot choose them, and `wall_reset` has no live listener.
  This predates the plan, and is left as a follow-up.

## Open for the owner

- The capitals of the English badges: kept, or lower case as the other badges.
- Whether the 240 px problem lists should grow, so that the phrases fit on one line.

# AP RefreshBatteries v1.0: Refresh now and new batteries

## TL;DR

Two buttons that spare a wait, and battery words that say what to do.
Neither button sends a radiogram.

- a Cube's device list read again → up to 30 minutes, at serve's refresh
  → **Refresh now**: a reconnect at once, in Maintenance and beside a Cube's Reboot
- a device with new batteries → "low" for an hour after its flag went
  → **New batteries** in its row: ok at once, once its flag is off
- who did it → a `battery` event marked `replaced`, with the user,
  replayed after a restart
- "battery weak" / "battery low" → near-synonyms to a native reader
  → "Batteries weak, replace soon" / "Batteries low, replace now"
- the API's codes `ok`, `weak`, `low` → unchanged

### Blocks

1. Refresh route: `POST /api/v1/cubes/{sn}/refresh`, a reconnect through the queue
2. Refresh now: Maintenance, and the Cube page beside Reboot
3. New batteries on the server: the tracker, the event, the rebuild, the route
4. New batteries in a device's row
5. The battery words, in six languages

## Discussion

### What the owner asked for

On 2026-11-23, on one of the owner's Cubes, all three devices of one room showed a low
battery. The Cube's flags had gone off at 16:17 and 16:24, before the Cube was
rebooted; the display's level turned ok at 17:07, the two radiator thermostats' an
hour after their flags went, as `internal/battery` holds a low level (`ClearLow`).
The owner asked where the battery state is kept, and for a button in Maintenance
that reads the Cube again.

The answer: the device measures and sends the flag; the Cube keeps the last one it
heard in its device list; Massimo shows the flag (`battery_low`) and its own level
(`battery`), kept from the flag's history in its log.
A re-read would not have cleared those tiles, since the hour is Massimo's rule;
a mark for new batteries would.
So the owner took both (D7, D8), and asked whether `weak` and `low` are clear
to a native speaker: they are not, so the words change (D12), the codes do not (D13).

### What is decided

The owner, 2026-11-23, `PLAN++ as recommended`, after the EVALs of D9 to D13.
D1 to D6 of the same conversation went elsewhere; this plan's numbers start at D7,
and its next is D14.

- **D7-A. Refresh now:** a button that makes serve reconnect to a Cube at once,
  as its half-hourly refresh does: the Cube sends its whole greeting again.
  No radiogram.
- **D8-A. New batteries:** a device's action that sets its battery level to ok at
  once, offered while its level is weak or low and its flag is off;
  with the flag still on it is refused, since the device has not reported its new
  batteries yet.
- **D9-A. Who may refresh:** `restart`, managers and owners: both places that show
  the button are theirs already, and no store change is needed.
  Operators have neither Maintenance nor the Cube page.
- **D10-B. Where:** in Maintenance, beside "Refresh the Cubes every", for every Cube;
  and per Cube on the Cube page, beside Reboot.
- **D11-A. Who may mark new batteries:** `configure`, operators and guests too,
  who handle the devices; it changes a level shown, and the device's flag still rules.
- **D12-D. The words:** "Batteries weak, replace soon", "Batteries low, replace now",
  "Batteries ok"; in German "Batterien schwach, bald wechseln",
  "Batterien leer, jetzt wechseln", "Batterien ok" (the devices' own word for their
  symbol is "leer"). Where a device row's badge does not fit at 390 px,
  the short form "replace soon" / "replace now" there.
- **D13-A. The codes:** `ok`, `weak` and `low` stay, in the API, the log's events and
  the notification rules (`battery_weak`, `battery_low`): with D12-D they are the very
  words people read, and new ones would need a second field until a v2 of the API,
  aliases in users' stored notification settings, and two vocabularies in the log.

## Analysis

- **The refresh** (`internal/server/server.go`, `refresh`): every `RefreshInterval`
  (Maintenance, "Refresh the Cubes every", `refresh_interval`) serve enqueues
  `engine.Command{Key: "refresh", Priority: engine.PriorityLow, Reconnect: true}`;
  the new greeting replaces the snapshot, and `snapshot refreshed` is logged.
  An address change does the same at `PriorityNormal` with `Moved`
  (`internal/server/cubes_api.go`). A refresh sends nothing by radio and holds no slot.
- **Where the buttons go:** Maintenance is a part (`internal/server/ui/src/maintenance/`),
  its sections shown by capability (`settings`, `restart`, `reboot`); the page needs one
  of the system capabilities (`src/chrome/pages.ts`).
  The Cube page needs `cubes` or `pair` and has the Cube's Reboot
  (`src/cubes/Actions.tsx`, `cubes.reboot`).
- **The battery level** (`internal/battery`): `weak` once the flag is seen,
  `low` once it held `HoldLow` (15 minutes), `ok` again `ClearLow` (an hour) after a low
  one's flag went, or after a long valve move or a day (`ClearWall`) for a weak one.
  `internal/server/battery.go` feeds each device state to the tracker, emits a `battery`
  event per change of level (low at warn), and at a start rebuilds the levels by
  replaying the log's device states of the last 8 days (`rebuildBattery`).
  A state without a valid report (after a Cube restart) is skipped, so a reboot does
  not restart the hour.
  So a mark for new batteries must be in the log in a form the rebuild replays, or a
  restart brings `low` back.
- **Notifications** (`internal/notify/rules.go`, `battery`): `weak` sends
  `battery_weak` after 15 minutes, `low` sends `battery_low` at once, any other level
  cancels a waiting `battery_weak`; an `ok` from a mark needs nothing new.
- **The words** live in `internal/server/ui/i18n/<lang>.json`:
  `badge.batteryLow`, `badge.batteryWeak` (a device's row, `src/model/device.ts`),
  `problem.battery`, `log.battery.{ok,weak,low}` and `log.sum.battery` (a level's event),
  `notify.battery_{low,weak}.{label,title,body}` and `help.notify.battery_*`.
  The flag's own lines in the log (`log.batteryLow`, `log.ch.batteryOk`,
  `src/log/text.ts`) name the device's flag, not the level, and stay.

## Implementation Plan

An Opus sub-agent builds each block without a commit, a second reader reads the
larger ones, and the session verifies the tree and makes the signed commit;
the five commits are one pull request, `main` merged in before it.
Verification is local for what a block touches; the whole Go suite, `make race`,
`make cross` and the whole UI matrix are left to CI.

### Block 1: Refresh route

- `POST /api/v1/cubes/{sn}/refresh`, capability `restart` (D9-A): enqueues the
  refresh as the address change does (`PriorityNormal`, `Reconnect`, without `Moved`),
  waits for it, and answers when the new greeting has replaced the snapshot,
  with the Cube's budget as other answers carry it; the reconnect's error otherwise
  (a Cube not reachable, or held by another client), and `409` while the Cube reboots.
- Tests against the fake Cube: a second connection and a fresh snapshot after the call;
  `403` without `restart`; `TestOpenAPICoversRoutes`.
- Docs: the route in `docs/api-v1.md` and `docs/api-v1-openapi.yaml` (`x-capability`),
  the `restart` rows of the capability tables in `docs/api-v1.md` and `docs/store.md`;
  an `Added` CHANGELOG fragment.

**Verification:** `go test ./internal/server`, `gofmt -l .`,
`go vet` for linux, windows and darwin.

### Block 2: Refresh now

- Maintenance: "Refresh now" beside "Refresh the Cubes every", shown with `restart`,
  refreshing every Cube in turn, each Cube's result beside it (D10-B).
- The Cube page: "Refresh" beside Reboot, shown with `restart` (D10-B).
- Texts in all six `i18n/<lang>.json` (FR, IT, NL, PL machine-translated and marked).
- Docs: `docs/ui.md` for both places; an `Added` CHANGELOG fragment.

**Verification:** `make ui-check`; the touched UI test files in the four cases
(1100 and 390 px, `en` and `de`); `go test ./internal/server` for the UI's Go tests.

### Block 3: New batteries on the server

- `internal/battery`: `Replaced(dev, at)`: a weak or low device whose flag is off
  becomes ok at `at`, its change returned; nothing for an ok device or a flag that is on.
- `POST /api/v1/cubes/{sn}/devices/{rf}/battery` with `{"replaced": true}`,
  capability `configure` (D11-A): `200` with the level and the one before
  (`changed: false` for a device already ok), `409` while the device's flag is on,
  `404` for no such device, `400` for a virtual wall thermostat or another body.
- The change is a `battery` event as today, its `summary` with `cause: "replaced"`,
  and the user; `rebuildBattery` replays these events with the device states in the
  order of their times, so the level outlives a restart.
- Tests: the tracker's rules; the route's answers; a restart keeps the level ok;
  a waiting `battery_weak` is cancelled.
- Docs: `docs/api-v1.md` (the battery paragraph, the route, the `configure` row,
  the `battery` event's `cause`), the OpenAPI file, `docs/store.md`'s `configure` row;
  an `Added` CHANGELOG fragment.

**Verification:** `go test ./internal/battery ./internal/server ./internal/notify`,
`gofmt -l .`, `go vet` for the three systems.

### Block 4: New batteries in a device's row

- A device's row offers "New batteries" beside its battery badge while its level is
  weak or low and its flag is off, with `configure`; the answer shows in the row,
  and the badge goes with the level's event.
- Texts in all six languages; `docs/ui.md`; an `Added` CHANGELOG fragment.

**Verification:** `make ui-check`; the touched UI test files in the four cases.

### Block 5: The battery words

- D12-D in the badges, the problem list, the level's log lines, the notifications'
  labels, titles, bodies and help, in all six languages; the codes stay (D13-A).
- The UI test at 390 px decides where a badge takes the short form.
- `docs/ui.md` where it quotes the words; a `Changed` CHANGELOG fragment.

**Verification:** `make ui-check`; the touched UI test files in the four cases;
`go test ./internal/server` (the i18n files' tests).

### The pull request

Branch `claude/refresh-batteries`, the five commits, `main` merged in before the
pull request; `GUIDELINES/shared/tools/check` over the range; the run green.
The owner merges it.

## Proposed commit messages

- The plan: `chore(aps): +RefreshBatteries v1.0`
- Block 1: `feat(api): A Cube's device list can be read again at once`
- Block 2: `feat(ui): Refresh now, in Maintenance and beside a Cube's Reboot`
- Block 3: `feat(api): New batteries make a device's battery level ok at once`
- Block 4: `feat(ui): A device's row offers New batteries`
- Block 5: `feat(ui): The battery levels say what to do`

## Change History

- v1.0 (2026-11-23): first revision, by `PLAN++ as recommended`,
  with D7-A, D8-A, D9-A, D10-B, D11-A, D12-D and D13-A.

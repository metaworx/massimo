# AP CubePoll v1.0: Cube intervals in Maintenance

## TL;DR

`serve` asks each Cube for its device list every 10 s instead of every
minute, as the original MAX! software does, and System settings →
Maintenance sets the three Cube intervals (poll, refresh, discovery),
each with a line on what it does, applied at the next restart.

- **`poll_interval`:** default 1 min → 10 s; `massimo watch -poll` too.
- **`GET /api/v1/settings`:** `fixed_by` on an interval where a flag
  (`-poll`, `-refresh`, `-discover-interval`, `-stale-after`) decides
  the run.
- **Maintenance**, under Announce on the LAN: **Poll the Cubes every**,
  **Refresh the Cubes every**, **Look for Cubes every**; a choice of
  steps, Off for the last two, a stored value outside them kept;
  read-only with the flag where one decides, as Announce.
- **The card's badge** counts the store's settings that wait for the
  restart too, not only the settings file's.

### Blocks

1. The 10 s default and `fixed_by`
2. The three intervals in Maintenance

## Discussion

The owner, on 2026-11-14 during the transmit-error tests (D102): the
original software asks the Cube every 10 s, so Massimo should too,
"maybe a config value on the cube?".
Then, on 2026-11-15: "PLAN 10s default and integration into system
settings under Maintenance", and after the TL;DR gate:
"PLAN++ D119-B with explanations of what they do".

**The owner's decisions:**

- **D117-A:** one setting for every Cube, its default 10 s.
  Not B, a value per Cube: no Cube needs another today.
- **D118-A:** a new interval applies at the next restart, as today;
  the card's Restart button is beside it.
  Not B, at once: each Cube client's ticker, its lost-Cube marking and
  read deadline would have to follow a change.
- **D119-B:** the refresh and discovery intervals in the card as well,
  each with a line on what it does.

**A correction to the TL;DR gate:** it gave as D118-A's reason that
"the card's badge already counts it". It does not: the badge counts
the settings file's changes that wait (`GET /api/v1/restart`,
`pending`), and Announce says its own wait in its line only.
Block 2 makes the badge count the store's waiting settings as well,
which is what the gate promised.

## Analysis

- **What a poll costs:** `l:` is answered from the Cube's memory; no
  radio time, no slot. The original software polls every 10 s while its
  window is open (`docs/cube-protocol.md`), and its logs of 2026-02-23 to
  03-05 show one Cube polled so for 10½ days, about 89 000 device lists.
- **`internal/server/settings_api.go`:** `DefaultPollInterval` is the
  default of the setting, of `serve -poll` and of a store without the
  setting; the range stays 10 s to 1 h.
  `settingView.FixedBy` exists (Announce's); the intervals have none.
- **`internal/serve/serve.go`, `open`:** a given value (`cfg.PollInterval`
  and the others, set only by a flag) wins over the store's; the loop
  knows which, so it can name the flag for `fixed_by`.
- **The lost-Cube timing** (`internal/cubelan`): two unanswered polls
  mark a Cube lost, `2 × (poll + 10 s reply time)`: 140 s → 40 s, the
  keepalive's 30 s below it. The read deadline, `3 × poll + 30 s`:
  210 s → 60 s. The proxy goes silent for a lost Cube that much sooner.
- **The proxy** answers a client's `l:` from serve's snapshot
  (`internal/proxy/proxy.go`, `local`), so Home Assistant and the
  original software get data at most 10 s old; serve's own polls are
  not forwarded to them.
- **The store:** a state row is written on a change, so a faster poll
  writes few more rows: a change that is undone within the minute.
- **Names:** My settings has **Poll every**, the page's poll of the
  server; the card's says **Poll the Cubes every**.
- **The UI** (`partials/maintenance.html`, `update.js`): the Announce
  row is the model, a `.field` with a `select`, a hint line and a
  message line, loaded with `GET /api/v1/settings` for `settings`.
  A choice of steps, labelled by `fmtUnit` (`app.js`, `Intl` units),
  needs no unit texts and can't send a value out of range; a value set
  through the API that is not a step is shown as an extra option.
  The default is sent as `null`, so the store holds no copy of it.

## Implementation Plan

### Block 1: The 10 s default and `fixed_by`

1. **`internal/server/settings_api.go`:** `DefaultPollInterval = 10 *
   time.Second`; `settingsView` sets `FixedBy` on a duration setting
   from `Options.SettingsFixedBy` (setting key → flag).
2. **`internal/server/server.go`:** `Options.SettingsFixedBy`; the
   `PollInterval` comment ("one minute").
3. **`internal/serve/serve.go`:** `open` fills `SettingsFixedBy` with
   the flag of each given value.
4. **`cmd/massimo`:** `serve -poll`'s help ("else 10s"); `watch -poll`
   10 s by default, its usage line too.
5. **Tests:** the default in the settings route; `fixed_by` with and
   without a given value (`internal/serve`, `internal/server`).
6. **Documents:** README (the poll and the lost Cube, "Server"
   settings), `docs/api-v1.md` and the OpenAPI file (the poll interval,
   the settings example, `fixed_by` on the intervals).
7. **`CHANGELOG.d/Changed/`**, a fragment.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `-race -count=1` on `internal/server` and `internal/serve`,
  `make cross`, `gofmt -l .`, `go vet` on three platforms,
  `GUIDELINES/shared/tools/check`.

### Block 2: The three intervals in Maintenance

1. **`partials/maintenance.html`:** three `.field` rows under Announce,
   with `settings`: a `select`, a hint and a message line each.
2. **`update.js`:**
   - the steps: poll 10 s to 1 h; refresh Off, 30 min to 7 days;
     discovery Off, 1 min to 1 day;
   - each row's line: what the interval does, then this run's value,
     or that the change waits for the restart, or, with `fixed_by`,
     the row read-only and what sets it;
   - a change is saved at once (`PUT /api/v1/settings`), the default
     as `null`;
   - the badge counts the waiting store settings (Announce and the
     three) with the settings file's.
3. **Six languages** (FR/IT/NL/PL machine-translated, as the files
   mark); **`docs/ui.md`** (Maintenance).
4. **`tools/ui-test/`:** a test modelled on `test_announce_setting.py`:
   the rows and their lines, a change saved and read back, the wait and
   the badge, the default cleared; at 1100 and 390 px.
5. **`CHANGELOG.d/Added/`**, a fragment.

- **Verification:** `make ui-test`, the Go gate,
  `GUIDELINES/shared/tools/check`.

## Proposed commit message

For Block 1:

```
feat(serve/poll): Poll the Cubes every 10 s by default

serve asks each Cube for its device list every 10 s instead of every minute, as the original software does, and GET /api/v1/settings names the flag that decides an interval for the run (AP CubePoll v1.0, Block 1).

Assisted-by: claude-code:claude-opus-5-5
```

For Block 2:

```
feat(ui/maintenance): Set the Cube intervals in Maintenance

System settings → Maintenance sets how often the server polls, refreshes and looks for the Cubes, each with a line on what it does, applied at the next restart (AP CubePoll v1.0, Block 2).

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-15): first revision, D117-A, D118-A, D119-B.

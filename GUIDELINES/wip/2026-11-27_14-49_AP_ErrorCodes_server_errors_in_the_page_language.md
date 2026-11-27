# AP ErrorCodes v1.0: Server errors in the page's language

## TL;DR

Every refusal of the API carries a stable `code` and its `params`
beside the English `error`;
the page says it in its own language from its six tables,
the English text where it has no key.
Then the "Copied" bubble becomes `showBubble(text, ctrl)`.

- answer →
  `{"error": "room 1 has no thermostat", "code": "room_no_thermostat", "params": {"room": 1}}`
- code → declared once (English format, parameter names);
  unknown codes impossible
- page → the thrown error's message already translated:
  the ~96 places showing it unchanged
- `room`, `device` params → by name in the page
- this case → `room_thermostat_unplaced`,
  pointing to the repair's "Place in its listed room" (D4-A)
- toast → none; a refusal stays where the action was (D5-A)
- branches → Block 1 on `maint/0.17` (D3-B), the rest on `main`
- open → D6, D7

### Blocks

1. Codes: the mechanism, the room and device writes' refusals,
   the command results; on `maint/0.17`
2. Rooms: the rest of rooms, devices, pairing, repair
3. Cubes: Cubes, discovery, reboot, backups, restore, updates, the log
4. Users: users, roles, sign-in, tokens, integrations, providers, settings
5. Passed: Go errors passed through, stored and streamed errors;
   no refusal left without a code
6. Bubble: `showBubble(text, ctrl)`, the "Copied" bubble its first user

## Discussion

### What the owner decided

The owner saw, in a German page, the tile's line "manuell 19,5 °C:
room 1 has no thermostat" after setting the setpoint of a room whose only
thermostat had no room in its C line.
At the TL;DR gate the owner widened it: "this would apply to all error messages
(coming from the server)", and asked whether the page's requests should carry
`Accept-Language`.
At the EVAL that followed, 2026-11-27:

- **D1-A.**
  The page translates, from a code and its parameters beside the English text.
  The server translates nothing; no `Accept-Language` is sent for it
  (the browser sends its own, the browser's languages, not the page's setting).
- **D2-A.**
  One plan, Blocks 1–5, a pull request each.
- **D3-B.**
  Block 1 on `maint/0.17`, merged upward into `main`; Blocks 2–5 on `main`.
- **D4-A.**
  Two codes for this refusal:
  `room_no_thermostat` and `room_thermostat_unplaced`,
  the second naming the repair.
- **D5-A.**
  No toast flag on `fail()`: where a refusal shows is the page's decision,
  by where the user acted, as today.

With `PLAN+ as recommended` the owner added:
"the extraction of the 'copied bubble' into a `showBubble(message, [ctrl])`
where `ctrl` would be the control above which the bubble should appear,
in the screen's bottom center if `undefined`.
(the implementation details can differ, this explains the concept)" (Block 6).

### Open: the owner's

- **D6.
  Block 6's branch.**
  - **A (recommended).**
    `main`, for 0.18: a refactor with a new form (the bottom centre) no caller
    of 0.17 needs.
  - **B.**
    `maint/0.17`, with or after Block 1:
    0.17 gets a function nothing in it calls without a control,
    after the freeze.
- **D7.
  The list of codes in the documents.**
  - **A (recommended).**
    A table in `docs/api-v1.md`, "Error codes", by area,
    held to the declared codes by `TestErrorCodes`:
    a client, Home Assistant's integration or a script,
    reads what it may branch on,
    and a code once released is never renamed.
  - **B.**
    None: the source is the list; a client has to read Go.

### What stays as it is

- `error` keeps the server's English text at every site:
  the log, the audit, Home Assistant and scripts read it as before.
- The codes already given keep their names:
  the integration chain's six (`bad_header`, `not_integration`,
  `integration_disabled`, `instance_pending`, `user_disabled`,
  `forbidden`) and the repair's `room_name_needed`;
  their top-level fields stay beside the new `params`:
  `no_state`, `etag`, the repair's `room`.
- Errors the page makes itself are translated already:
  not sent, no answer, signed out.
- The connect page's errors come from the app's bridge and the network,
  not from the server's API: not in this plan.
- `profile.linkError.<word>`:
  the words of the provider's redirect, not API errors.

## Analysis

### How a refusal reaches the page today

- `fail(status, format, args…)` (`internal/server/server.go`)
  makes an `httpError`;
  the route wrapper writes `{"error": msg}`,
  with `no_state` or `etag` where set.
- `apiRequest` (`app.js`, about line 340) and `apiRaw` (about line 483) throw
  `new Error(data.error)`, keeping `status` and `data`.
- About 96 places show `e.message`: the tiles' `errorText` (`rooms.js`),
  `showTileResult`, the cards' lines in the parts, `showMessage`.
- One place reads the English text:
  `src/shared/username.tsx` matches `/user name/` on a 400.

So translating the message where the error is thrown reaches every place at
once, and none of them changes.

### The size

458 `fail()` calls, about 265 distinct texts, in 36 files;
13 `writeError` calls and 5 direct `writeJSON` errors beside them.
By status: 192 × 400 (most of them input a page never sends), 82 × 409,
63 × 500, 43 × 404, 27 × 403, 22 × 503, 10 × 502, 6 × 401.
About 79 calls pass a Go error through (`"%v", err`).

What gets a code: every refusal a page can meet.
What keeps none: input no page sends (a body that does not parse,
a malformed id) and our own 500s,
whose English text is the server's detail and the log's.
The page shows those as today.

### The answer

```json
{"error": "room 1 has no thermostat", "code": "room_no_thermostat", "params": {"room": 1}}
```

- `code`:
  lower-case words joined by `_`, stable, never renamed;
  one code may come with more than one status (a 409 on a write,
  a 404 on a read).
- `params`:
  the values the text names, by name; numbers as numbers,
  radio addresses as six hex digits; absent where there are none.
- No feature name in `/healthz`:
  a client that finds no `code` falls back to the status, as it does today.

### The declaration

A code is declared once, in `internal/server/errcodes.go`, by area: its name,
its English format and the names of its parameters in the format's order,
for example

```go
var codeRoomNoThermostat = errorCode("room_no_thermostat", "room %d has no thermostat", "room")
```

and a site refuses with it and the status:
`return refuse(http.StatusConflict, codeRoomNoThermostat, room.ID)`.
`fail(status, format, args…)` stays for what keeps no code.
The names may differ in the build; the shape is the point:
a site cannot give a code that is not declared,
and `TestErrorCodes` walks the declared codes (each unique and well-formed,
as many verbs as parameter names,
its `err.<code>` in all six tables with the same placeholders, and, with D7-A,
its row in `docs/api-v1.md`).

### The page

- `apiRequest` and `apiRaw` throw an error whose `message` is the page's text:
  `t("err." + code, params)` where English has the key,
  else `data.error`, else the status line;
  `code`, `params` and `data` stay on it.
  A plain script and a part get it alike, through `api()`.
- A parameter named `room` (an id) or `device` (a radio address) is shown by the
  name the page has for it, where the request was for the selected Cube;
  else as given.
- `errorText(obj)` in `app.js`, declared in `src/plain/app.d.ts`,
  does the same for an object with `error`, `code` and `params`
  that is no thrown error: a stored or streamed one (Block 5).
- Text only: `t()` gives a string, set as `textContent` or a JSX text child.

### This case (D4-A)

`setRoomSetpoint` (`server.go`),
the room's configuration (`config_api.go`, about line 881)
and the room's programme (about lines 1186 and 1246)
find the room's thermostats by their C lines
(`cubeio.RoomAddressee`, `roomThermostats`).
The owner's room has a thermostat in the metadata (`cubeio.RoomMembers`)
whose C line names no room, or another.
Then the answer is `room_thermostat_unplaced` with `room` and `device`,
and its text names the repair "Place in its listed room" (`repair.place`),
which moves the device into the listed room (`repair_api.go`, `buildPlace`).
Where the metadata has no thermostat in the room either: `room_no_thermostat`.
The English `error` says the same in the server's words.

### The bubble (Block 6)

Today `copied(el, ok)` (`components.js`) sets `data-copied` and the class
`copied` on the control,
and CSS draws "Copied" above it (`.copyable.copied::after`) for 1.4 s;
a polite live region, `#copy-status`, made at the first copy,
says it for a screen reader.
`copyable`, `copyIcon`, `copyEl` and the parts' `src/shared/copy.tsx` call it.
Eight UI test checks look at the class `copied` (`test_copy.py`,
`test_keep_open.py`, `test_pairing.py`, `test_password_suggest.py`).

`showBubble(text, ctrl, {ms})`:

- one element, made at the first bubble and kept,
  on the page and the connect page alike (both load `components.js`),
  and the live region beside it, made the same way;
- with `ctrl`: fixed above its centre, inside the window's edges, below it where
  there is no room above;
- without: at the window's bottom centre, above the safe area;
- 1.4 s by default; a newer one in the place of the one shown; gone at a scroll;
- the text said once in the polite live region;
- `copied(el, ok)` becomes `showBubble(t(…), el)`.

## Implementation Plan

Each block is a pull request of its own;
deviations from the plan go into one NOTE per run (AGENTS.md §5.4).
The plan's own commits go onto `maint/0.17` first and upward into `main`,
since Block 1 ships in 0.17 (MaintBranch).
Block 1 branches from `origin/maint/0.17`, its pull request onto it,
and the session that merged it merges `maint/0.17` upward into `main`;
Blocks 2–6 branch from `origin/main` after that (D3-B, D6-A).
UI tests run on free ports, the four cases 1100-de, 1100-en, 390-de, 390-en;
the full suites are CI's.
Every test uses the fake Cube and `MASSIMO_ENV_FILE=/dev/null`;
placeholders only (`KEQ0000000`, `0a0101`).

### Block 1: Codes

- **Goal:**
  the mechanism, and codes for what a tile and a room's or device's card send:
  the owner's case in the page's language (D1-A, D4-A, D5-A).
- **Changes:**
  1. `internal/server/errcodes.go` (new): `errorCode`, the declarations,
     `refuse`; `httpError` gains `code` and `params`;
     the route wrapper writes them in every branch (plain, `no_state`, `etag`);
     the integration chain's codes and `room_name_needed` declared as the
     others, their answers as before.
  2. Codes for the lookups every write passes: no Cube, no room, no device,
     the Cube not connected yet (503).
  3. Codes for the room and device writes: setpoint, mode, boost, lock,
     the room's and the device's configuration, the room's programme;
     `room_no_thermostat` and `room_thermostat_unplaced` at the four sites (D4-A).
  4. `commandError` (`config_api.go`): codes for `engine.ErrSuperseded`,
     `engine.ErrDiscarded` (and with `no_state`, the devices as `params`),
     `engine.ErrUnwanted`, `cubelan.ErrNotConnected`, `cubelan.ErrBusy`
     and a deadline passed.
  5. `app.js`:
     `apiRequest` and `apiRaw` throw the page's text; `errorText(obj)`;
     `room` and `device` by name; `src/plain/app.d.ts`.
  6. `ui/i18n/<lang>.json`:
     `err.<code>` for every code of this block in all six;
     FR/IT/NL/PL machine-translated, as their tables are marked.
- **Tests:**
  - `TestErrorCodes` (`internal/server`): the declarations as above.
  - The owner's case against the fake Cube,
    its greeting with the thermostat's C line naming no room:
    `PUT …/rooms/{id}/setpoint` answers 409 `room_thermostat_unplaced` with
    `room` and `device`; a room without a thermostat in the metadata either:
    `room_no_thermostat`; the configuration's and the programme's sites alike.
  - `commandError`:
    each sentinel to its code, the exact body.
  - `tools/ui-test/test_error_texts.py` (new):
    a room's setpoint refused with the server's exact body (`page.route`,
    the body taken from the Go test's) shows the room's line in German at `de`
    and in English at `en`, the room by its name;
    a code without a key shows the English `error`; nothing as HTML.
  - Each seen failing before the change.
- **Documents:**
  `docs/api-v1.md` "Errors" (`code`, `params`;
  with D7-A the table "Error codes");
  the OpenAPI `Error` schema gains `code` and `params`; `docs/ui.md`,
  how a refusal shows.
- **CHANGELOG:**
  `CHANGELOG.d/Fixed/claude-error-codes-rooms.md`:
  "A refused setpoint, mode or room setting is said in the page's language;
  the API's errors carry a `code` and its `params`."
- **Commit:**
  `fix(api): A refusal carries a code, and the page says it in its language`
- **Verification,** each by its exit code: `make ui`;
  `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`;
  `gofmt -l .` prints nothing;
  `go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...`;
  `make ui-check`; `make cross-check`;
  `test_error_texts.py` and `test_rooms*.py` in the four cases;
  `GUIDELINES/shared/tools/check`.

### Block 2: Rooms

- **Goal:**
  codes for the rest of rooms and devices: names, moves, removal, virtual wall
  thermostats, pairing, batteries, the repair, the programme's tag.
- **Changes:**
  `server.go`, `config_api.go`, `repair_api.go`, `vwt.go`, `pairing_api.go`,
  `battery_api.go`, `proxy_virtual.go`, `etag.go`: `refuse` with declared codes
  for every refusal a page can meet; `err.<code>` in all six tables.
- **Tests:**
  `TestErrorCodes`; the existing route tests assert the code beside the text
  for each refusal they cover; a new one where a code has none.
- **Documents:**
  `docs/api-v1.md` (D7-A).
- **CHANGELOG:**
  `Changed`: "The refusals of rooms, devices,
  pairing and the repair are said in the page's language."
- **Commit:**
  `feat(api): Codes for the refusals of rooms, devices, pairing and the repair`
- **Verification:**
  as Block 1's, the UI tests of the cards touched.

### Block 3: Cubes

- **Goal:**
  codes for Cubes, discovery, the proxy, a Cube's reboot and refresh,
  backups and restore, updates, the log and its alerts, the server's restart.
- **Changes:**
  `cubes_api.go`, `discovery.go`, `cube_reboot.go`, `reboot_api.go`,
  `cube_refresh.go`, `backups.go`, `backups_api.go`, `restore_api.go`,
  `update_api.go`, `update_answer.go`, `log.go`, `logalerts.go`,
  `restart_api.go`, `serverinfo_api.go`, `integrations_found.go`;
  `err.<code>` in all six.
- **Tests, Documents, Verification:**
  as Block 2's.
- **CHANGELOG:**
  `Changed`: "The refusals of Cubes, backups,
  restores and updates are said in the page's language."
- **Commit:**
  `feat(api): Codes for the refusals of Cubes, backups, restores and updates`

### Block 4: Users

- **Goal:**
  codes for users, roles, sign-in (password, OPAQUE, providers), tokens, keys,
  integrations, settings, push, preferences, the audit, `Idempotency-Key`.
- **Changes:**
  `users_api.go`, `roles_api.go`, `login_api.go`, `opaque_api.go`, `oidc.go`,
  `auth.go`, `me_api.go`, `integrations_api.go`, `providers_api.go`,
  `settings_api.go`, `push_api.go`, `audit.go`, `commands.go`;
  `src/shared/username.tsx` tests the code instead of the English text;
  `err.<code>` in all six.
- **Tests, Documents, Verification:**
  as Block 2's; the sign-in's UI tests in the four cases.
- **CHANGELOG:**
  `Changed`: "The refusals of users, sign-in, tokens,
  integrations and settings are said in the page's language."
- **Commit:**
  `feat(api): Codes for the refusals of users, sign-in, integrations and settings`

### Block 5: Passed

- **Goal:**
  no refusal a page can meet left without a code,
  the errors passed through and those stored or streamed included.
- **Changes:**
  1. The about 79 calls passing a Go error:
     the sentinels and typed errors of `store`, `password`, `backup`,
     `update`, `cubeio` and the others,
     mapped to codes in one place where a page meets them;
     what stays (a body that does not parse) keeps none.
  2. Stored and streamed errors carry `code` and `params` beside `error`:
     a command's record (`GET /api/v1/commands/{id}`, an idempotent replay),
     backup and restore runs' results, a Cube's reboot, the restart's refusal;
     a store migration where a stored one needs it, numbered in `main`'s block;
     one stored before keeps its English text.
  3. The parts showing them (`backups`, `maintenance`, `sysinfo`, `log`)
     through `errorText(obj)`.
  4. `TestErrorCodes` also refuses a `fail()` with 401, 403, 404, 409, 412, 413,
     429, 502 or 503 (a scan of the sources).
- **Tests:**
  as Block 2's; each stored kind's code across a restart of the server;
  the UI tests of the cards touched.
- **Documents:**
  `docs/api-v1.md`, `docs/store.md` where a migration adds a column.
- **CHANGELOG:**
  `Changed`: "Every refusal and every failed run is said in the page's
  language."
- **Commit:**
  `feat(api): Codes for the errors passed through and those stored`

### Block 6: Bubble

- **Goal:**
  `showBubble(text, ctrl)`, as the owner asked; "Copied" its first user (D6-A).
- **Changes:**
  1. `components.js`:
     `showBubble(text, ctrl, {ms})` as in the Analysis;
     `copied(el, ok)` through it; `src/plain/components.d.ts`.
  2. `style.css`:
     `.bubble` in place of `.copyable.copied::after`,
     CSS variables for light and dark.
- **Tests:**
  the eight checks of the class `copied` look at the bubble instead;
  a new one in `test_copy.py`: the bubble above a control,
  below one at the window's top, at the bottom centre without one,
  the live region's text once, in the four cases.
- **Documents:**
  `docs/ui.md`, the bubble among the plain scripts' builders.
- **CHANGELOG:**
  none where nothing a user sees changes; else `Changed`:
  "'Copied' stays inside the window at its edges."
- **Commit:**
  `refactor(ui): showBubble shows a short word above a control or at the foot`
- **Verification:**
  `make ui`; `MASSIMO_ENV_FILE=/dev/null go test ./internal/server/...`,
  the UI's own Go tests; `make ui-check`; the four test files in the four cases;
  `GUIDELINES/shared/tools/check`.

## Proposed commit message

The plan's own, onto `maint/0.17`:
`chore(aps): +ErrorCodes v1.0`,
its trailer
`File-revision-v1: GUIDELINES/wip/2026-11-27_14-49_AP_ErrorCodes_server_errors_in_the_page_language.md::v1.0::2026-11-27::All::first revision`;
then `merge: maint/0.17 into main, with +ErrorCodes v1.0`.

Block 1:

```
fix(api): A refusal carries a code, and the page says it in its language

For AP ErrorCodes v1.0, Block 1 (D1-A, D4-A, D5-A): a refusal of the API carries a stable code and its parameters beside its English error, and the page shows it from its own tables, the English text where it has none. A room setpoint refused because the room's thermostat has no room on the Cube now says so in the page's language and names the repair that places it.

Key changes:
- internal/server: errcodes.go, codes declared once; refuse; code and params in every error answer; the codes of the room and device writes and of the command results.
- app.js: the thrown error's message in the page's language; room and device by name; errorText.
- ui/i18n: err.<code> in all six tables, FR/IT/NL/PL machine-translated.
- Tests: TestErrorCodes; the owner's case and its sibling against the fake Cube; test_error_texts.py.
- Docs: docs/api-v1.md "Errors", the OpenAPI Error schema, docs/ui.md; a Fixed fragment.

Assisted-by: claude-code:claude-opus-5-5
```

Blocks 2–6: as their **Commit** lines,
each with its Key changes and the same trailer.

## Change History

- v1.0 (2026-11-27): first revision,
  from the owner's answers at the TL;DR gate and its EVAL (D1-A, D2-A, D3-B,
  D4-A, D5-A) and "PLAN+ as recommended" with the bubble (Block 6);
  D6 and D7 open.

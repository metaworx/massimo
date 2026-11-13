# AP LogCubes v1.0: All Cubes in the log

## TL;DR

The log names a device by its room and name, and shows the lines of
every Cube at once where the user chooses All Cubes, each line then
naming its Cube too.

- device column → serial or name → "Room › Device"; no name → "Room › type" (D1-A, D3-A)
- names → today's, then the line's kept one, then type or S/N (D4-A)
- the Cube → never on a line → only under All Cubes: "Cube › Room › Device" (D2)
- a "Cubes" filter → This Cube (default) / All Cubes, shown with more than one Cube (D7-A)
- All Cubes → `GET /api/v1/log`, every Cube's lines and the server's once, each with `cube_serial`
- names under All Cubes → every Cube's rooms, loaded once (D8-A)
- live lines under All Cubes → asked for every 10 s (D9-A)
- serve's text log → unchanged (D5-A)

Blocks 2 and 3 build on the fix "fix(ui): The log names its devices once
the rooms come" (D6-A), its own pull request, merged first.

### Blocks

1. The all-Cubes log in the store and the API: `GET /api/v1/log`, `cube_serial`, tests, documents, fragment
2. Room and device on a line: "Room › Device", the fallbacks, the serial in expert mode, six languages, tests
3. The Cubes filter: This Cube / All Cubes, its address, the polling, every Cube's names, tests, documents

## Discussion

On 2026-11-13 the owner found the log naming devices by their serial
numbers, which tell a user nothing, and asked to show the names of the
Cube, the room and the device.
The names were there: a reload on the log with the Cube remembered, or a
switch of Cube there, drew the log before the rooms arrived, and nothing
drew it again. That fix is D6-A, its own pull request.

The owner then asked for a way to show the log of every Cube at once,
naming the Cube only in that mode, and proposed a Cube filter in the log:
the current Cube by default, All Cubes, another Cube, or the three values
current, unassigned and all.

**The owner's decisions**, 2026-11-13, `EXEC fix, PLAN++` on the evaluation,
which takes every recommendation:

- **D1-A.** The device column names "Room › Device".
  Not B, the names behind the changed value: ragged lines, the name hard to find.
- **D2** (the owner's): the Cube's name only where the lines of several
  Cubes are shown, before the room: "Cube › Room › Device".
- **D3-A.** A device without a name is named by its room and type,
  "Kitchen › Radiator thermostat"; the S/N goes to expert mode, beside the
  radio address, and to the tooltip; the text filter still finds it.
- **D4-A.** Today's names first, then the name the line kept
  (a state line's, a removed device's), then the type or S/N.
- **D5-A.** serve's text log stays as it is: serial and radio address.
- **D6-A.** The reload fix, its own pull request, first.
- **D7-A.** A "Cubes" filter among the log's choices with two values:
  This Cube, the header's, with the server's lines as now, the default;
  and All Cubes. Shown only with more than one Cube.
  Not B, a third value for the server's lines alone (unassigned):
  This Cube already shows them, and the source choice narrows to them.
  Not C, every Cube by name: the header already picks the Cube, and a second
  choice could show another Cube than the header names.
- **D8-A.** Under All Cubes the page loads every Cube's rooms once and names
  from them, the kept name as the fallback.
  Not B, the server naming each line as it reads it: it changes the API's
  event objects for one view.

**Refined here** (the plan's own, by its recommendation):

- **D9-A.** Under All Cubes the lines that come later are asked for every
  10 s while the log is shown (`since` the newest line), and the stream of
  the header's Cube adds none, so that every line comes one way.
  Not B, a stream per Cube: as many connections as Cubes, each with its
  ticket. Not C, a server-wide stream: a route and its tickets for one view.
- **D10-A.** `cube_serial`, not `cube`: `cube` is already the found Cube's
  object on a `cube_found` event.
- **D11-A.** A device under All Cubes is chosen by its S/N; a radio address
  names a device on its own Cube only, so the route reads one as every
  Cube's lines of that address.

## Analysis

### The store

`log` rows carry `cube_id`, NULL for the server's own
(`internal/store/migrations/store.sql`).
`Store.Events` walks one Cube's rows newest first along its index, and with
`Server` the server's rows in a second walk, merged by id.
All Cubes is the same with a walk per Cube: `EventQuery.CubeIDs`, each
walked with the same terms and limit, merged with the server's by id, the
newest `Limit` kept. The paging by `before` id holds across Cubes, ids being
one sequence.

### The route

`GET /api/v1/cubes/{sn}/log` (`internal/server/log.go`, `getLog`) parses the
query (`parseLogQuery`), resolves a device by radio address to its serial on
that Cube (`resolveDevice`), and reads the store (`storedLog`) or, without
one, the Cube's ring (`filterEvents`).
`GET /api/v1/log` takes the same query, the `read` capability like the
Cube's log (there is no per-Cube access), every runtime with a store id, and
sets `cube_serial` on every row of a Cube.
Without a store, each Cube's ring holds the server's lines too
(`logServer` remembers them in every ring): the route merges the rings by
time and keeps a server line (no device, source `system` or `server`) once.

### The page

`state.rooms` holds the header's Cube's rooms (`state.roomsOf`);
`logDeviceName` and `logTargetText` name from them.
The stream (`connectEvents`) is the header's Cube's and carries the
server's lines too.
The log's choices live in `logState` and the address (`logHash`,
`readLogHash`); `fillLogSelects` fills the device choice from the rooms.
`state.cubes` names each Cube (`name`, else its S/N).

## Implementation Plan

### Block 1: The all-Cubes log in the store and the API

1. `store.EventQuery.CubeIDs`: walk each, merge with the server's by id;
   `CubeID` keeps its meaning. A store test with two Cubes and the server's rows.
2. `GET /api/v1/log` in `log.go`, registered beside the Cube's log:
   the query of `parseLogQuery`, a radio address as a device term on every
   Cube (D11-A), `cube_serial` on each Cube's line (D10-A), the ring merge
   without a store.
3. `event.CubeSerial` (`json:"cube_serial,omitempty"`), set only by this route.
4. `docs/api-v1.md`, "The log", and `docs/api-v1-openapi.yaml`.
5. Tests in `log_api_test.go`: two Cubes and a server line, each line's
   `cube_serial`, the server's line once, paging by `before` across Cubes,
   a filter by serial, a radio address, without a store.
6. `CHANGELOG.d/Added/claude-log-cubes-api.md`.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`
for server and store, `TestOpenAPICoversRoutes`, gofmt and go vet for the
three platforms, `make cross`.

### Block 2: Room and device on a line

Needs D6-A on the branch.

1. `logDeviceName` names "Room › Device" from the rooms loaded (D1-A):
   the room by the device's `room_id`; an unlisted device without a room.
2. No name (D3-A): "Room › type" by `typeName`; no room either, the type.
   The line's kept name before that (D4-A).
3. The S/N in expert mode beside the radio address, and as the device's
   tooltip; the text filter matches name, room, S/N and address.
4. `log.devicePath` in the six `ui/i18n/<lang>.json` ("{room} › {device}"),
   FR/IT/NL/PL marked as machine-translated.
5. `docs/ui.md`, the log.
6. `test_log.py`: the path, the unnamed device, the S/N in expert mode;
   the tests that read a bare name now read the path.
7. `CHANGELOG.d/Changed/claude-log-cubes-names.md`.

**Verification:** `make ui-test` (the log's tests at 1100 and 390 px, then all),
`go test ./internal/server/` (the UI's own checks).

### Block 3: The Cubes filter

Needs Blocks 1 and 2.

1. A "Cubes" choice in `ui/partials/log.html`, a `.field` like its
   neighbours, shown with more than one Cube: This Cube, All Cubes (D7-A).
2. `logState.cubes` ("" or "all"), in the address as `cubes=all`;
   Clear sets it back; a Cube switch in the header keeps it.
3. All Cubes: `loadLog` and `loadOlder` read `api/v1/log`; every Cube's rooms
   loaded once and the log drawn again as they come (D8-A); every 10 s the
   lines `since` the newest, dropped where already there, while the log is
   shown; the stream adds none (D9-A).
4. A line names "Cube › Room › Device" (D2); a Cube's own line its Cube;
   the server's lines no Cube. The device choice lists every Cube's devices,
   one group per Cube, by S/N (D11-A).
5. The six languages; `docs/ui.md`.
6. `test_log.py`: a second Cube added through the API for the test, the
   choice shown then and not before, All Cubes reading `/api/v1/log`, the
   Cube's name on a line, the address, Clear, the polling with a held
   answer; at 1100 and 390 px.
7. `CHANGELOG.d/Added/claude-log-cubes-filter.md`.

**Verification:** `make ui-test` whole, `go test ./...`, gofmt and vet.

## Proposed commit messages

The plan's registration, a commit of its own on main before the work branch:

```
chore(aps): +LogCubes v1.0

The log names devices by their serial numbers where the rooms come late, and shows one Cube's lines at a time. The owner asked on 2026-11-13 for the Cube's, the room's and the device's names, and for every Cube's lines at once; EXEC fix, PLAN++ takes every recommendation of the evaluation.

Key changes:
- GUIDELINES/wip/2026-11-13_17-37_AP_LogCubes_all_cubes_in_the_log.md: the plan, three blocks

File-revision-v1: GUIDELINES/wip/2026-11-13_17-37_AP_LogCubes_all_cubes_in_the_log.md::v1.0::2026-11-13::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1, `feat(api): GET /api/v1/log reads every Cube's log at once`;
Block 2, `feat(ui): The log names a device by its room and name`;
Block 3, `feat(ui): The log shows every Cube's lines on request`.

## Change History

- v1.0 (2026-11-13): the first revision, by `PLAN++` on the evaluation,
  D1-A to D8-A, with D9-A to D11-A refined here.

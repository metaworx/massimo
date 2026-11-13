# AP RoomApi v1.1: Virtual wall thermostat per room

## TL;DR

Every room gets a virtual wall thermostat (VWT), the room's one source of temperature and its
stable device in every client. The plan also carries the Home Assistant integration's other
requests to Massimo: the kept last temperature, programme ETags, a fake Cube that follows writes,
and boost over the air.

- **VWT:** `wall_thermostat` with `virtual: true`.
  - Serial `VWT` and the room number in 7 digits; the room number as its radio address
    (collision → a free one, override per room); `room-<id>` as an alias.
  - Created and removed with its room, never moved; the room gets a `room_uid` (UUID).
- **Views:** `?devices=default|real|virtual|all`.
  - `default`: VWT only where no real wall thermostat (additive).
  - `real`: today's answer.
  - `virtual`: real wall thermostats → hidden, every VWT shown.
  - `all`: everything.
- **Proxy clients:** see the `virtual` view.
- **Temperature:** the real wall thermostat's, else the radiators' median or mean; stale values
  left out while a fresh one exists. Every client reads it from the VWT.
- **Commands:** setpoint, programme, room config → the room; lock, offset, display → the real
  wall thermostat. A second real wall thermostat in a room → `409`.
- **Programme ETag:** over every member, also in the body; a write on its way → `412`.
- **Fake Cube:** applies what it acknowledges, with a control port and fixtures.
- **Boost:** a hardware test, then the route.
- **Features (v1.1):** `/healthz` → `"features": {"vwt": "1.0", …}`, the running server's
  abilities by name and version, so that a client of main's builds knows what it may use.

### Blocks

1. FakeCube: the fake Cube applies writes; control port; fixtures (one PR, first)
2. LastTemp: the last reported temperature and its time are kept
3. VwtRead: view, `room_uid` and settings, `?devices=`, events, notifications, status
4. VwtWrite: commands, the one-wall-thermostat limit, the finding in `portal-protocol.md`
5. VwtUi: the tile, the expert view, device management, the VWT settings
6. ProgramEtag: ETag and If-Match, pending writes, `GET …/devices/{rf}/program`
7. VwtProxy: the `virtual` view for proxy clients, behind a per-Cube setting
8. ProxyTest: the original software through the proxy, on the owner's go
9. BoostTest: one boost on the owner's test Cube and room
10. Boost: the route, the UI control, the docs
11. Features: the `features` map in `/healthz` (v1.1)

## Discussion

**The requests.** The Home Assistant integration (metaworx/massimo-homeassistant, its AP
Integration) asked Massimo for five changes on 2026-11-12. The owner accepted them as the
integration's design and asked that nothing be built half-way:

1. boost over the air;
2. keep a thermostat's last reported temperature;
3. room-level values;
4. a version check on programme writes;
5. a fake Cube that follows writes.

The owner widened request 3 into the virtual wall thermostat. The model was then worked out over
the evening of 2026-11-12, with the integration session's feedback twice.

**Decided** by the owner (the D numbers of the evaluation):

- **D1-A:** one plan with the specification in its Analysis. The integration starts on it once the
  plan is registered on main.
- **D2-A:** the VWT's radio address is the room number. A collision with a real device or the Cube
  takes a free address from `000100` to `0001ff`; the room's settings allow an override.
  `room-<id>` is an alias.
- **D3:** the type is `wall_thermostat` with `virtual: true`.
- **D4:** the views are `?devices=default|real|virtual|all`, as the TL;DR gives them. The owner
  added `virtual` with `PLAN++`.
- **D5-A:** the VWT's mode is `boost` while any thermostat of the room boosts; otherwise the real
  wall thermostat's, else that of the radiator thermostat that reported last.
  `consistent: false` while they disagree.
- **D6-A:** a stale value is left out of the median while a fresh one exists.
- **D7-A:** proxy clients see VWTs, behind a per-Cube setting that is off until Block 8 passes.
  With `PLAN++`, the owner set the view proxy clients get to `virtual`.
- **D8:** where a real wall thermostat exists, the VWT's lock, offset and display commands go to
  it, and its `battery_low`, `error`, `link_error` and `panel_locked` mirror it.
- **D9-A:** Massimo refuses a second real wall thermostat in a room with `409`.
- **D10-A:** in a room with a real wall thermostat, the room's temperature is that thermostat's,
  the value the radiators regulate by. `temp_source` was dropped; `temp_method` stays.
- **D11-A:** the serial is `VWT0000005` for room 5; it may repeat across Cubes. Clients identify a
  room by `<cube serial>:<room_uid>`.
- **D12-A:** `room_uid` is a UUID.
- **D13:** Block 1 goes into a PR of its own at once, so the integration can build on it. Later
  blocks stack on top, each either a PR or commits pushed in batches for CI, decided block by block.

**From the owner's `PLAN++` as well:**
- **Block 9, the boost test,** runs on the Massimo of the breno bridge host, against the Cube and
  room the owner named. Their names are real identifiers and stay out of this plan. The owner's
  word for that test is given; nothing else is sent there.
- **The settings UI** (the per-Cube proxy setting, the VWT's settings) is coordinated with the
  session "Web UI updates".

**Decided with v1.1** (2026-11-13, the owner's `PLAN++ D19-A D18-A`):

- **D19-A:** `/healthz` carries a `features` map, the Home Assistant integration's
  request (its D12), reviewed and with `proxy_virtual` added (A8).
- **D18-A:** PR #209, the revert of the slow-scroll test's wait, closed unmerged: the
  session "Web UI updates" found the failure's cause in the test's setup (an injected
  found Cube the next poll removes, the header shrinking while the page is scrolled)
  and fixes it there.

**The integration's answers** (2026-11-12):
- it reads `?devices=all`, backs each room's climate entity by its VWT, and keeps real wall
  thermostats as devices of their own;
- it identifies a room by `<cube serial>:<room_uid>`;
- it requires the Massimo version with VWTs, read from `/healthz`;
- it asked for the fixtures of Block 1.

## Analysis

### A1. The rule underneath

The original software allows **one wall thermostat per room**. The owner saw it on 2026-11-12:
moving a wall thermostat into a room that has one is refused with "You cannot add any more
devices of this type. The maximum number of devices of this type has been reached for this room."

- A room's wall thermostat sends its measured temperature to the room's radiator thermostats,
  which regulate by it.
- The original software reads the Cube anew at every start and caches nothing. So it can be shown
  a wall thermostat in every room.

### A2. Specification: the virtual wall thermostat

**Identity.**

```json
{
  "rf_address": "000005",
  "serial": "VWT0000005",
  "type": "wall_thermostat",
  "virtual": true,
  "name": "<the room's name>",
  "room_id": 5,
  "room_uid": "3f0c…",
  "reported": true,
  "state": {…},
  "received": {…},
  "sources": {"actual_temp": ["0a0102", "0a0103"], "mode": ["0a0104"]}
}
```

- **One per visible room**, a room without a thermostat included. It is created and removed with
  its room and never moved.
  - A deleted room takes its VWT and settings along, with `device_forgotten`.
  - A new room has one at once.
- **`virtual: true`** is absent on real devices.
- **The name** is the room's, following a rename.
- **`room_uid`** is a UUID in the store per room, never reused, on the room in `GET …/rooms` and on
  its VWT.
  - A room deleted and another created under the same id while the server was down cannot be told
    apart, and keeps its uid.
- **The address:**
  - by default the room number, `00 00 <id>`;
  - where a real device or the Cube has that address, a free one from `000100` to `0001ff`, kept
    in the store and logged at warn;
  - an override in the room's VWT settings, refused with `409` where it is taken;
  - `room-<id>` is accepted wherever `{rf}` is.
- **The serial:** `VWT` + the room number in 7 digits. No eQ-3 serial starts with V.
  `tools/identifiers` allows the pattern.

**The views:** `?devices=` on `GET …/rooms`, `GET …/devices` and `GET …/events`.

| Value | Real wall thermostats | VWTs |
|-------|-----------------------|------|
| `default` | shown | only in rooms without a real wall thermostat |
| `real` | shown | none (today's answer) |
| `virtual` | hidden | in every room |
| `all` | shown | in every room |

- Radiator thermostats, shutter contacts and eco switches are shown in every view.
- An unknown value is `400`.
- In `default` and `virtual`, every room has exactly one wall thermostat.
- `GET …/devices/{rf}` answers for every device whatever the view.
- A stream's view filters its events, the state burst and resumed events alike.
- **Never counted or saved:**
  - device counts (`connected`'s `devices`, `GET /cubes`);
  - backups and restores;
  - a room's `ready`;
  - notifications, whose rules skip VWT events.

**State.** A wall thermostat's fields, computed from the room's visible real devices; hidden
devices never count.

| Field | With a real wall thermostat | Without one |
|-------|-----------------------------|-------------|
| `actual_temp` | the wall thermostat's (with two, their median) | median or mean of the radiator thermostats' (`temp_method`) |
| `mode` | `boost` while any thermostat boosts, else the wall thermostat's | `boost` while any boosts, else that of the radiator thermostat that reported last |
| `setpoint`, `until` | from the same thermostat as `mode` | the same |
| `valve_pct` | the highest of the radiator thermostats' | the same |
| `window_open` | any shutter contact open | the same |
| `battery_low`, `error`, `link_error`, `panel_locked` | the wall thermostat's | false |
| `consistent` | false while the thermostats disagree on mode, setpoint or `until` | the same |

- **The median:** the median of two values is their mean; results are rounded to 0.1 °C.
- **`received`:** for `actual_temp`, the oldest time of the values used; for the other fields,
  the newest of their inputs.
- **`sources`:** `actual_temp` lists the addresses used; `mode` lists the ones mode and setpoint
  came from (the boosting ones during a boost).
- **`stale`:** true when every source of `actual_temp` is stale.
  - A stale value is left out while a fresh one exists.
  - Where all are stale, they are used, and the VWT is stale.
- **`reported`:** true once any thermostat of the room has reported.
- **No thermostat in the room:** the VWT carries `window_open` alone.
- **Events:** a `state` event for the VWT follows each change of its state.
  - It comes right after the member's `state` event that caused it.
  - It names that event's command.
  - It resumes like any other event.

**The room's temperature** is its VWT's `actual_temp`, with `received.actual_temp` as its age.
The web UI's tiles, `massimo status`, Home Assistant and every other client show it. The tile's
old rule (`tiles.js`: the wall thermostat's, else the first radiator thermostat's) goes.

**Commands** on a VWT:

| Request | Does |
|---------|------|
| `PUT …/setpoint` | the room's setpoint (boost once Block 10 is in) |
| `PUT …/config` | radio fields → the room's config; `temp_offset`, `display_actual` → the real wall thermostat, `400` without one; VWT settings → the store |
| `PUT …/program` | the room's programme, with If-Match (A3) |
| `PUT …/lock` | the real wall thermostat's key lock; `409` without one |
| rename, move, `DELETE` | `409`: it goes with its room |
| `GET …/devices/{rf}` | the room's config and programme, the VWT settings, `etag` |

- **Each command inherits** the capabilities, radio budget, command id, `Idempotency-Key` and
  audit of the route it maps to.
- **The audit** names the room (or the real wall thermostat) as the target, with `"via"` naming
  the VWT.

**Its settings**, in the store per room, needing `configure`:

| Setting | Values | Default |
|---------|--------|---------|
| `temp_method` | `median`, `mean` | `median` |
| `rf_address` | an address, or none for the default | none |

A change sends a `config` event and, where the value changes, a `state` event.

**One wall thermostat per room.**
- `PUT …/devices/{rf}` refuses to move or add a second real wall thermostat into a room with
  `409`, before anything is sent.
- A Cube that already has two (set up by another tool) is handled:
  - both are shown in `real` and `all`;
  - the median takes both;
  - `default` shows no VWT there.
- The original software probably limits other types too ("devices of this type"; eQ-3 names 8
  radiator thermostats and 8 window contacts). Only the limit seen is enforced; the others are
  recorded in `portal-protocol.md` as unconfirmed.

### A3. Specification: the programme's ETag

- **The tag covers every thermostat of the room.** A write to any member changes it.
- **`program_consistent: false`** is set while the members' programmes differ.
- **Where it is given:** the `ETag` header, and `etag` in the body of:
  - `GET …/rooms/{id}/program`;
  - the new `GET …/devices/{rf}/program`;
  - a VWT's `GET …/devices/{rf}`.
- **`PUT …/program`** (room or device) takes `If-Match`.
  - A tag that doesn't match answers `412` with the current tag.
  - Without `If-Match`, the write is unconditional, as today.
- **A write on its way:** a write accepted but not yet seen in the re-read device configuration
  counts as a change.
  - `GET` shows the pending programme, marked `pending: true`, with its own tag.
  - A `PUT` with the tag from before it answers `412`.
- **The `config` event** of a room's device carries `changed` (`program`, `temps`, `settings`),
  and `etag` where the programme changed.

### A4. Specification: the proxy

Proxy clients see the `virtual` view, while the per-Cube setting `proxy_virtual` is on. It is off
by default until Block 8 has passed.

1. **The greeting, the `L` line and the `C` lines carry each VWT, and leave out the real wall
   thermostats.** The proxy already does this for hidden devices.
   - A VWT's `L` entry has a wall thermostat's form, with its state.
   - Its `C` line holds the room's settings and programme, under its address and serial.
2. **`m:` writes** are stripped of VWTs and get the real wall thermostats put back, as hidden
   devices are today.
   - A rename of a VWT is ignored.
   - A multi-part write the proxy cannot parse is refused while the setting is on, instead of
     passed through: the Cube must never store a VWT.
3. **`s:` commands that name a VWT:**

   | Command | Goes to |
   |---------|---------|
   | 0x40 setpoint, 0x10 programme, room-addressed or to the VWT | the room |
   | 0x11, 0x12 config to the VWT | the real wall thermostat, else the room's config |
   | 0x82 display, 0x80 lock | the real wall thermostat; without one, answered by the proxy and sent nowhere |
   | 0x20 to 0x23 links and groups between a VWT and a real device | the same command with the real wall thermostat in its place; without one, answered and sent nowhere |

   The original software sends link commands when it adds or moves a device in a room.

4. **`t:` deleting a VWT** is answered with an `A` line, and nothing is sent.
5. **What is answered locally costs no air time.**
6. **The known cost:** pairing a real wall thermostat through the original software is refused
   by it, since every room shows one. That is done through Massimo's web UI, or with
   `proxy_virtual` off.

### A5. Specification: the fake Cube (Block 1)

- **It applies what it acknowledges**, after a delay it is given (`-apply-after`, default
  immediately):
  - 0x40 mode and setpoint, room-addressed to every thermostat of the room;
  - boost, and the return to the previous mode after the device's boost time, or after
    `-boost-for` when given;
  - 0x10 programme, into the device's C line;
  - 0x80 lock.

  Its `L` answer and its C lines then show the change, as the Cube's do.
- **A control port** (`-control <addr>`, HTTP on loopback) for tests that drive it from outside:
  - set a device's state fields (actual temperature, battery, mode);
  - remove a device from its room and put it back;
  - drop the connections, so that a greeting is sent anew.
- **Fixtures** in `internal/cubelan/fakecube/testdata/` and for `tools/ui-test/`, with placeholder
  serials and addresses only:
  - a room with a wall thermostat and two radiator thermostats at different temperatures;
  - a room with radiator thermostats only (three, for the median);
  - a room with a shutter contact only;
  - a thermostat that can be made stale;
  - a room whose wall thermostat can be removed and added again.
- **Staleness:** `serve -stale-after <d>` (a test flag, default 24 h), so a test needn't wait a day.
- **The integration's `tests/conftest.py`** relies on today's behaviour (no state after a
  setpoint). Applying writes is therefore on by default, and `-apply=false` keeps the old
  behaviour for it. The integration session is told when the PR is up.

### A6. The code each block touches

- **`internal/cubelan/fakecube`, `cmd/massimo/fakecube.go`:** Block 1.
  - Today the fake answers `s:` with an `S` line and nothing more.
  - Its `L` answer is the greeting's last `L` or one set with `SetDeviceList`.
- **`internal/protocol/cube/devicelist.go:100`, `internal/cubelan/times.go:89`:** Block 2.
  - An `actual_temp` of 0 is dropped with its time today.
  - On vacation the bytes carry the end date, and the value is never set.
- **`internal/view`:** the read model shared by the API, `status` and backups. Blocks 3 and 4 add
  the VWT there as pure functions; backups keep calling the existing ones.
- **`internal/server`:** routes, `{rf}` resolution, events, audit (Blocks 3, 4, 6, 10).
  - `config_api.go:310` `moveDevice` gets the limit.
  - `server.go:1608` refuses boost today.
- **`internal/store`:** a migration with `rooms` (cube, room id, uid, `temp_method`,
  `rf_address`). Every store table already keys a device serial with its Cube (log and audit
  carry `cube_id`), so a repeating VWT serial collides nowhere.
- **`internal/notify`:** the rules skip VWT events.
- **`cmd/massimo/status.go:163`:** the room line shows the VWT's temperature.
- **`internal/server/ui/tiles.js:51`:** the tile rule (Block 5).
- **`internal/proxy`:** `local` (`l:`, `c:`), `passthrough` (`s:`, `t:`, `m:`), `withHidden`
  (Block 7).
  - Today's multi-part `m:` fallback passes the write through with a warning.
- **`/healthz`** carries the version the integration checks. VWTs reach it with the first
  release after Block 4; releases are the owner's.

### A8. Specification: the features map (v1.1)

`GET /healthz`, which needs no sign-in, carries `features`: an object, each key a
feature's name, each value its version `"MAJOR.MINOR"`, a string without `v`.

```json
{"ok": true, "version": "…", "started_at": "…",
 "features": {"event_ids": "1.0", "idempotency": "1.0", "commands": "1.0", "vwt": "1.0",
              "program_etag": "1.0", "boost": "1.0", "proxy_virtual": "1.0"}}
```

- **MINOR** rises when a feature gains something compatible: a field, a route. A client
  written for 1.0 works with 1.3, and one that needs 1.2 asks for a minor of at least 2.
  **MAJOR** marks an incompatible change, which under `/api/v1` does not happen: every
  feature here stays 1.x.
- **A name never changes meaning;** a feature that goes is removed from the map, never
  redefined. A server without the field has none of them.
- **The running server's abilities,** built at its start, not the build's: without a
  store (`serve -addr`) `idempotency` and `commands` are left out, since their routes
  and headers need it. Permissions stay in `/api/v1/me`.

| Name | What it promises | First release |
|------|------------------|---------------|
| `event_ids` | the stream's ids, resume by `Last-Event-ID` or `last_event_id`, `resync` | 0.13.0-alpha.1 |
| `idempotency` | `Idempotency-Key` on writes (with a store) | 0.8.0-alpha.1 |
| `commands` | `GET /api/v1/commands/{id}`, command ids in answers (with a store) | 0.8.0-alpha.1 |
| `vwt` | virtual wall thermostats, `?devices=`, `room_uid`, `temp_method` | after 0.15.0-alpha.1 |
| `program_etag` | the programme's tag, `If-Match`, `412`, `pending`, `changed` and `etag` on config events, `GET …/devices/{rf}/program` | after 0.15.0-alpha.1 |
| `boost` | `{"mode": "boost"}` on the setpoint routes | after 0.15.0-alpha.1 |
| `proxy_virtual` | the Cube option `proxy_virtual` exists (`PUT /api/v1/cubes/{sn}`) | after 0.15.0-alpha.1 |

Left out on purpose: the kept temperature (no client acts on it, and every name is a
promise), and what `/api/v1/me` or the routes themselves already tell.

## Implementation Plan

Every block brings its tests, `docs/api-v1.md` and the OpenAPI file for each route change,
`docs/ui.md` for the UI, and a `CHANGELOG.d` fragment.

### Block 1: FakeCube

1. Device model in `fakecube`: per-device state parsed from the greeting's `L` and `C` lines,
   rooms from `M`.
2. Apply 0x40 (room and device addressed, boost with its return), 0x10, 0x80, after
   `-apply-after`; `-apply=false` keeps today's behaviour.
3. The control port (`-control`): state fields, remove and put back, drop.
4. The fixtures (A5).
5. `serve -stale-after`.
6. Tests: each command against the `L` answer and the C line, byte for byte where the Cube's lines
   are known.
7. PR at once. The integration session is told when it is up.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make race`, `make cross`, lint;
existing fake Cube tests unchanged.

### Block 2: LastTemp

1. `devicelist.go`: keep the device's last `actual_temp` when an entry reports 0 or carries a
   vacation end date. The snapshot merges it.
2. `times.go`: the received time stays with the kept value.
3. Tests with recorded entries: 0, vacation, a new value.

**Verification:** tests, race; `docs/api-v1.md` ("`actual_temp` when the device reported one" →
the last reported).

### Block 3: VwtRead

1. Migration: `rooms` with uid (UUID), `temp_method`, `rf_address`; rows follow the metadata
   (created for a new room, deleted with a room).
2. `view`: `RoomThermostat` (A2's state rules), the views, address allocation.
3. Server: `?devices=` on rooms, devices and events; VWT `state` events after a member's; resume
   with the view.
4. `room_uid` on rooms; `GET …/devices/{rf}` for a VWT.
5. Notifications skip VWT events; `status` shows the VWT's temperature.
6. `tools/identifiers`: the `VWT` pattern allowed.
7. Docs: api-v1 (views, the VWT), OpenAPI, `docs/store.md`.

**Verification:** table tests for the state rules (each fixture room, stale, boost, two wall
thermostats); route tests for each view; the resume test with a view; ui-test unchanged
(`default` is additive).

### Block 4: VwtWrite

1. `{rf}` resolution of VWT addresses, `room-<id>` and the serial.
2. The command table of A2, each with a test that checks the exact line the fake Cube receives.
3. VWT settings through `PUT …/config` (store), with events.
4. The one-wall-thermostat limit in `moveDevice`, with a test.
5. `docs/portal-protocol.md`: the original software's limit, with the date and its message.

**Verification:** tests, race; `TestOpenAPICoversRoutes`.

### Block 5: VwtUi

1. Coordinate with the session "Web UI updates" before starting: its settings work and this
   block's.
2. Tiles read the VWT (`default` view); the old rule goes.
3. Expert mode reads `all` and shows the VWT in each room, with `temp_method` and the address
   override.
4. Device management (pairing, moves, deletions) reads `real`.
5. i18n keys in all six languages; `docs/ui.md`.

**Verification:** `make ui-test` at 1100 and 390 px, new tests for the tile and the expert view.

### Block 6: ProgramEtag

1. The tag over all members; `program_consistent`; `etag` in the bodies.
2. `GET …/devices/{rf}/program`.
3. `If-Match` on both `PUT` routes; pending writes; `412`.
4. `changed` and `etag` on `config` events.
5. The UI's programme editor sends `If-Match` and offers a reload on `412`.

**Verification:** tests with the fake Cube's `-apply-after` for the pending window; ui-test for
the editor.

### Block 7: VwtProxy

1. `proxy_virtual` per Cube (setting, API, UI through Block 5's settings).
2. Greeting, `L`, `C`: the `virtual` view.
3. `m:`: strip VWTs, put real wall thermostats back; multi-part writes parsed, refused where not.
4. `s:` and `t:` per the A4 table.
5. Tests: every proxy line checked for VWTs and real wall thermostats, setting on and off.

**Verification:** tests, race, `make lan-test` (the proxy's discovery answers are unchanged).

### Block 8: ProxyTest (on the owner's go)

The original software through the proxy of the breno bridge host's `serve`, with `proxy_virtual`
on, checking:
- the greeting;
- a setpoint;
- a programme;
- a config;
- a move of a radiator thermostat into a room with a VWT;
- a rename;
- a metadata write.

The findings go into `portal-protocol.md` the same day. After it passes, `proxy_virtual` defaults
to on.

**Verification:** the original software shows one wall thermostat per room with Massimo's room
temperature, and nothing it sends reaches a device it would not reach without the proxy.

### Block 9: BoostTest (the owner's go given with `PLAN++`)

1. On the breno bridge host's `serve`, the owner's test Cube and room: one 0x40 with mode boost,
   room-addressed, through the queue.
2. Check that the `L` entries show mode 3 within minutes, and the return to the previous mode
   after the boost time.
3. Once from auto. From manual only on the owner's further word.
4. One radio write, about 1.25 % duty cycle.
5. Findings into `portal-protocol.md` the same day, with the firmware, including the boost
   duration (5 min at 80 % in our captures, 10 min elsewhere).

**Verification:** the device list before, during and after, recorded with redacted identifiers.

### Block 10: Boost

1. `{"mode": "boost"}` on the room and device setpoint routes (and a VWT's); `temp` ignored.
2. A boost control on the room's tile or detail in the web UI.
3. Docs: api-v1 (the setpoint table, the line "Boost cannot be started over the air" removed),
   OpenAPI, `docs/ui.md`, `docs/homeassistant.md`.

**Verification:** a test with the exact line to the fake Cube; ui-test; only after Block 9 passed.

### Block 11: Features (v1.1)

1. A `features` map built at the server's start from what it can do (the store or
   none), in `/healthz`.
2. Tests: `/healthz` with and without a store, each name present only where its routes
   answer.
3. Docs: `docs/api-v1.md` under `GET /healthz` (the table of A8), the OpenAPI file.
4. One PR; the integration session is told once it is merged.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, lint, `make cross`.

## Proposed commit message

Per block, for example for Block 1:

```
feat(fakecube): Apply what the fake Cube acknowledges

AP RoomApi v1.0, Block 1.

Key changes:
- The fake Cube keeps each device's state and applies setpoints,
  boosts, programmes and locks after an optional delay.
- A control port sets states and moves devices for tests.
- Fixtures for the room thermostat's median, staleness and removal.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- **v1.1** (2026-11-13): Block 11, the `features` map in `/healthz` (A8), on the owner's
  D19-A, the Home Assistant integration's request with `proxy_virtual` added; D18-A
  recorded. Blocks 1 to 7, 9 and 10 are merged; Block 8 waits for the owner's go.
- **v1.0** (2026-11-13): first revision. It rests on the evaluation of the Home Assistant
  integration's requests (four revisions on 2026-11-12, untracked), its D1 to D13, and the
  integration session's two rounds of feedback.

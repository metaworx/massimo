# AP Pairing v1.0: copy, removal, panel

## TL;DR

Pairing gets the three parts it lacks,
on the server's existing inclusion mode and add-to-room:
the room's settings on a device put into a room,
a route that removes a device from the Cube, and the web UI for both.

- put into a room → links and group as today, then 0x40 auto,
  0x11 to the room's thermostats, 0x12, the programme days that differ,
  0x11 again; `"copy": false` skips; on a move too
- `DELETE …/devices/{rf}` → 0x21 per partner, 0x23, `t:01,1,<rf>`,
  `m:` without it, the snapshot forgets it; capability `structure`
- fake Cube → `t:` answered with `A:` and the device dropped;
  `-pair` on the command line
- UI → "Pair a device" in Settings → Cubes; "Put into a room" under
  "Devices without a room"; "Remove from the Cube" in the Devices tab;
  the `paired` events heard
- tests → exact lines against the fake Cube; pytest for both flows
- hardware → the owner's: #10's six steps, the copy,
  `t:` on a device that answers

No analysis file: the Analysis below holds the findings of 2026-11-06.
The owner's decisions D1 to D3 are in the Discussion.
Run order 1, 2, 3.

### Blocks

1. PairCopy: the copy on add and move, `copy` in the body,
   exact-line tests, the API documents
2. DeviceRemove: `cubeio.RemoveCommands`, `Snapshot.Forget`,
   `DELETE …/devices/{rf}`, the fake Cube's `t:`,
   the proxy's copy, the documents
3. PairPanel: `fakecube -pair`, the pairing flow in Settings → Cubes,
   "Put into a room", "Remove from the Cube", the stream events,
   six translations, the pytest tests, docs/ui.md, screenshots

## Discussion

The findings were presented as a TL;DR gate on 2026-11-06 (`PLAN pairing`);
the owner chose option A of each decision the same evening.

- **D1.
  The copy on add and move: A.**
  On by default, as the original software never leaves a device at factory values;
  `"copy": false` in the body skips it.
  Only the programme days that differ from the device's own are sent,
  since 0x10 costs 8 % of an hour's radio budget per day.
  The rejected B, off by default, would leave a new device on its factory
  programme until someone configures it.
- **D2.
  Removal's capability: A.**
  `structure`, reworded "move and remove devices, delete rooms":
  whoever restructures the house may remove a device; the UI asks in the page,
  since `t:` makes the Cube send the device a factory reset.
  The rejected B, a capability of its own,
  would add a switch to every role and a migration of their sets.
- **D3.
  The pairing flow's home: A.**
  Settings → Cubes, in the Cube's panel, ending with name and room;
  a device paired by the original software through the proxy lands in the
  same "Devices without a room" list, which gets "Put into a room".
  The rejected B, a button on the Rooms page,
  would be a click nearer but not say which Cube when there are several.

Two facts shape the blocks:

- **Budget.**
  0x11 and 0x12 cost 3 % each, 0x10 8 % per day,
  so a factory-fresh thermostat takes about 65 % of an hour's budget with every day sent;
  the engine's queue and its budget reconnect carry that,
  and the API's answer reports the budget as every write does.
- **Hardware.**
  Everything here lands against the fake Cube.
  The six steps of pull request #10, the copy,
  and whether `t:` drops a device that answers are the owner's hardware check,
  which the handover's check 3 gets at the end of the run.
  A device that never answers is never dropped by the Cube (seen on hardware);
  the removal route's document says so and leaves room 255 as the way to park
  such a device.

## Analysis

### 1. What is there

- **The API**:
  `POST`, `GET`, `DELETE /api/v1/cubes/{sn}/pairing` (`pairing_api.go`),
  the events `pairing_started`, `paired`, `pairing_ended`,
  the `pair` capability;
  the proxy relays a client's `n:` and `x:` into the same mode (docs/proxy.md,
  "Pairing").
- **Add to room**:
  `PUT …/devices/{rf}` with `room_id` on an unlisted device runs
  `cubeio.Writer.AddDevice`: `MoveCommands`' in-part (wakeup,
  0x20 per linkable partner in both directions, 0x22),
  the metadata with the device, a `c:` re-read;
  a device whose C line names another room is unlinked first (the out-part:
  0x21 per partner, 0x23).
  A move runs both parts, then the metadata.
  Neither copies settings; docs/api-v1.md says so and points to the config
  and programme routes.
- **The builders** in `internal/protocol/cube/commands.go`,
  tested byte for byte: `CmdSetTemperature` (0x40), `CmdSetTemperatures` (0x11),
  `CmdSetValveConfig` (0x12), `CmdSetDayProgram` (0x10), `CmdLinkPartner` (0x20,
  0x21), `CmdSetGroup` (0x22, 0x23), `CmdDeleteDevices` (`t:`),
  `CmdWakeup` (`z:`).
- **The room's values**:
  `cubeio.RoomRadiator(snap, room, rf)` gives the configuration of a radiator
  thermostat of the room other than `rf`, or the original software's defaults;
  `getRoomProgram` reads the room's programme from its first thermostat's C line;
  `roomThermostats(rt, room)` lists radiators and walls.
- **The engine**:
  `engine.Command{Key, Priority, Target, RoomID, Wakeup, Lines}`;
  `Wakeup` sends `z:` for `RoomID` first; `s:` lines consume budget,
  `t:` and `m:` do not.
- **The fake Cube** (`internal/cubelan/fakecube`):
  `Server.Pair *Pairing` joins after `n:` (an `N` line, `c:` answered for it,
  an empty `N` at the timeout or on `x:`), built by `PairingFrom`;
  not on `massimo fakecube`'s command line, which has `-listen`, `-greeting`,
  `-duty`, `-discovery`, `-discovery-serial`.
  It handles `q l c s r f e d n x` and not `t`.
- **The UI**:
  `tiles.js` lists the unlisted devices under "Devices without a room"
  (`unlisted.title`) with `renderDevice` and no action;
  the room configuration's Devices tab (`config.js`,
  `renderDeviceForm`) has name, room and offset per device;
  Settings → Cubes (`cubes.js`, `cubePanel`,
  `partials/cube-panel.html`) holds the Cube's form, the hidden devices,
  Remove and the found Cubes; the stream (`app.js`) listens to `state`,
  `metadata` and `cube_found` only.
  `components.js` has `confirmBox`, `fieldRow`, `formEl`, `formFooter`,
  `selectInput`, `showResult`, `fromTemplate`, `detailsPanel`.
  600 keys in each of six translation files.
- **The tests**:
  `tools/ui-test/` is a pytest suite over four cases (1100 and 390 px,
  en and de) against one server and one fake Cube started by `run.sh` with `greeting.txt`;
  a test leaves the server as it found it.

### 2. What the original software does

From docs/portal-protocol.md, "Pairing a device,
as the original software does it": after `n:`, the `N` line,
the links and the group (steps 1 to 3),
it sends 0x40 set temperature auto with temperature 0 to the new device,
a wakeup and 0x11 preset temperatures to every thermostat of the room with the
room's values, a wakeup and 0x12 to the new device (boost 5 min 80 %,
descaling Saturday 12:00, maximum valve 100 %, offset 0),
a wakeup and 0x10 for all seven days to the new device and 0x11 once more,
then `m:` (steps 4 to 8).
A move repeats steps 4 to 8 for the new room after the unlink and the link.
Deleting a device from a room: wakeup, 0x21 per partner,
`t:01,1,<rf>`, `m:` without it; `t:` is answered `A:` per address,
makes the Cube send the device a factory reset (FHEM's finding),
and leaves a device that never answers in the list.

### 3. What is missing, and where it goes

| Part | Files |
|------|-------|
| The copy | `internal/cubeio/pairing.go` (`CopyCommands`), `internal/server/config_api.go` (`updateDevice`, `addDevice`, `copy`), their tests, docs/api-v1.md, docs/api-v1-openapi.yaml, docs/portal-protocol.md, CHANGELOG.md |
| The removal | `internal/cubeio/remove.go` (`RemoveCommands`), `internal/cubelan/snapshot.go` (`Forget`), `internal/cubelan/client.go` (the reply `t:` expects), `internal/server/config_api.go` (`removeDevice`, the route under `structure`), `internal/proxy/proxy.go` (a relayed `t:` forgets in the server's copy), `internal/cubelan/fakecube/fakecube.go` (`t:`), their tests, docs/api-v1.md (the route, the capability table), the OpenAPI file, docs/portal-protocol.md, CHANGELOG.md |
| The fake's flag | `cmd/massimo/fakecube.go` (`-pair`), `tools/ui-test/run.sh`, `tools/ui-test/README.md`, README.md (`fakecube`) |
| The UI | `internal/server/ui/cubes.js`, `tiles.js`, `config.js`, `app.js` (the events), `partials/pairing.html` (templates) listed in `ui.go`'s `uiPartials`, `partials/cube-panel.html`, `style.css` where a row needs it, `i18n/{en,de,fr,it,nl,pl}.json`, docs/ui.md, docs/screenshots where a shot changed, `tools/ui-test/test_pairing.py`, CHANGELOG.md |

### 4. Risks

- The copy's budget (Discussion): sending only the days that differ,
  and nothing to a shutter contact or a push button, keeps it as low as it goes.
- `t:` on a device that answers has not been seen on hardware:
  the tests encode the expectation (`A:` per address,
  the device gone from the next `L:`),
  the document marks it unverified until the owner's day.
- A UI test cannot read the fake Cube's lines;
  it reads the page and the server's log,
  and the Go tests hold the exact lines.
- The four cases of the suite share one server:
  the pairing test pairs, puts into a room and removes in one test,
  and the fake lets its device join again after `t:`.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`,
`internal/cubelan` and `internal/proxy`), `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
`make ui-test` where a file under `internal/server/ui/` changed,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube, the network, or the owner's Windows service;
addresses and serials in tests and documents are placeholders (`0abcde`,
`TEQ0000001`, `KEQ0000000`).
Each block is one or more functional commits citing `AP Pairing v1.0, Block <n>`,
each with its CHANGELOG bullet, and one pull request stacked on the
previous block's branch, as AP Tls did, with `origin/main` merged into the
first branch before its pull request.
The commit headline's component carries the part it changes:
`server.pairing`, `ui.pairing`, `fakecube`, `ui-test`.

### Block 1: PairCopy

1. `internal/cubeio/pairing.go`:
   `CopyCommands(snap *cubelan.Snapshot, rf cube.RFAddress, typ cube.DeviceType, room uint8) []engine.Command`,
   for a radiator or wall thermostat (nothing for a contact or a push button),
   in the original software's order:
   - `{Key: "copy-mode:<rf>", Target: rf}` with 0x40 auto, temperature 0,
     to the device;
   - `{Key: "copy-temps:<room>", RoomID: room, Wakeup: true}` with 0x11 to
     every thermostat of the room, the new one included:
     comfort, eco, maximum, minimum, window-open temperature and minutes
     from `RoomRadiator(snap, room, rf)`,
     each device's own temperature offset kept (the new device's is 0);
   - `{Key: "copy-valve:<rf>", Target: rf, RoomID: room, Wakeup: true}`
     with 0x12 from the same reference (boost, descaling, maximum valve,
     valve offset), which a wall thermostat takes with the same payload;
   - `{Key: "copy-program:<rf>", Target: rf, RoomID: room, Wakeup: true}` with
     one 0x10 per day of the room's programme (the first thermostat's,
     as `getRoomProgram` reads it) whose switch points differ from the device's
     own C line after the `c:` re-read, then 0x11 to the device once more;
     no command at all when no day differs.
   `Linkable`, `isThermostat` and `RoomRadiator` serve as they are;
   the reference's values come from the room's other thermostat,
   else the defaults `RoomRadiator` returns.
2. `AddDevice` takes `copy bool` and runs the copy after the metadata write
   and the re-read; `updateDevice`'s move does the same after its metadata
   write and re-read.
   The result of each command goes through `commandError`,
   and the first failure ends the copy with the error,
   the device in its room with what was sent so far.
3. `internal/server/config_api.go`:
   the body gains `"copy": false` (default true, a move or an add only;
   with `name` alone it is ignored);
   the answer gains `"copied": {"temperatures": <devices>, "valve": true, "days": <n>}`,
   absent when nothing was copied;
   one log line `settings copied` with the counts.
4. Tests (`config_api_test.go`, `pairing_api_test.go`):
   the exact lines the fake Cube receives for a radiator thermostat put into a
   room holding a radiator and a wall thermostat (links, group, `m:`, `c:`,
   then 0x40, `z:` and three 0x11, `z:` and 0x12, `z:`,
   the 0x10 lines and 0x11),
   with the fake's C line for the new device carrying two of the room's days so
   that five are sent; `"copy": false` sends the links, the group and `m:` only;
   a shutter contact gets no copy; a move copies for the new room;
   a discarded 0x11 answers the error and leaves the device in its room.
5. Documents:
   docs/api-v1.md (`PUT …/devices/{rf}`: the copy, `copy`,
   `copied`, the budget sentence; "Pairing": the client flow unchanged),
   docs/api-v1-openapi.yaml (`copy`, `copied`),
   docs/portal-protocol.md ("Massimo pairs the same way" gains the copy),
   CHANGELOG.md under `### Changed`.

**Verification:**
the test gate; the exact lines of step 4;
`go test -race ./internal/server/...` clean;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 2: DeviceRemove

1. `internal/cubelan/snapshot.go`:
   `Forget(rf cube.RFAddress)` drops the device's C line, state and list entry;
   `Devices()` no longer names it; `internal/cubelan/client.go`:
   `ForgetDevice(rf)` with the snapshot's lock and an event `device_forgotten`
   to the observers;
   the reply `t:` expects is `A:` where the client has such a table.
2. `internal/cubeio/remove.go`:
   `RemoveCommands(snap, rf, typ, room uint8, members []cube.DeviceMeta) (unlink engine.Command, del engine.Command, ok bool)`:
   `unlink` is `MoveCommands`' out-part for the room the metadata lists,
   else the C line's room, else none;
   `del` is `{Key: "delete:<rf>"}` with `t:01,1,<rf>` (force, as the
   original software sends it).
   `Writer.RemoveDevice(ctx, md, c)` runs `unlink` where there is one,
   `del`, then the metadata without the device where it was listed,
   then `ForgetDevice`.
3. `internal/server/config_api.go`:
   `DELETE /api/v1/cubes/{sn}/devices/{rf}` under `structure` (`removeDevice`):
   a hidden device is `404` as `visibleDevice` says, the Cube itself `400`,
   the Cube not connected `503`; on success `{"ok": true, "budget": …}`,
   a log line `device removed`,
   a `metadata` event so that the UI reloads its rooms;
   the audit and the command row as every write gets them.
4. `internal/proxy/proxy.go`:
   a client's `t:` that passed `withoutHidden` and was answered `A:` forgets
   those devices in the server's copy, so that the original software's
   deletions show at once.
5. `internal/cubelan/fakecube/fakecube.go`:
   `t:` answers `A:` per address (as hardware did),
   removes the device from the C lines it answers and from its L line,
   and lets a `Pair` device join again; `Keep map[cube.RFAddress]bool`
   names devices it keeps anyway, for the never-answering case.
6. Documents: docs/api-v1.md (the route's section after `PUT …/devices/{rf}`;
   the capability table's `structure` row: "move and remove devices,
   delete rooms";
   the paragraph on hidden devices points to the route for the ordinary case),
   docs/api-v1-openapi.yaml (`x-capability: structure`;
   `TestOpenAPICoversRoutes` holds it),
   docs/portal-protocol.md (`t:` on a device that answers marked *unverified*),
   docs/ui.md untouched until Block 3, CHANGELOG.md under `### Added`.
7. Tests: the exact lines for a listed thermostat (wakeup,
   0x21 to each partner and from the device to each, 0x23, `t:01,1,<rf>`,
   `m:` without it), an unlisted device (`t:` alone),
   a device in no room by the metadata but a room by its C line (unlinked from that room),
   hidden `404`, the Cube `400`; after it the device is absent from `/devices`,
   the rooms and the counts, and the `metadata` event came; the proxy test:
   a client's `t:` for a visible device drops it from the server's copy,
   one for a hidden device is refused as today; the fake's `t:` and `Keep`.

**Verification:**
the test gate with `-race` for `internal/server`,
`internal/cubelan` and `internal/proxy`; `TestOpenAPICoversRoutes`;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 3: PairPanel

1. `cmd/massimo/fakecube.go`:
   `-pair <template rf>,<rf>,<serial>[,<delay>]` (delay 2 s unless given)
   builds `Server.Pair` with `PairingFrom` from the greeting;
   `tools/ui-test/run.sh` passes `-pair <a radiator of greeting.txt>,0abcde,TEQ0000001`;
   `tools/ui-test/README.md` and README.md's `fakecube` line name it.
2. The stream (`app.js`): `pairing_started`,
   `paired` and `pairing_ended` are heard;
   `paired` reloads the devices (the unlisted list),
   the other two drive the flow's state.
3. Settings → Cubes, in the Cube's panel (`cubes.js`,
   `partials/cube-panel.html`,
   `partials/pairing.html` with `tpl-pairing` and `tpl-pair-device`,
   listed in `uiPartials`): a **Pair a device** button, shown with `pair`,
   opening the flow in the panel: a seconds field (60;
   1 to 600) and **Start** (`POST …/pairing`);
   a countdown from `until` with **Stop** (`DELETE …/pairing`);
   each device that joins (from `paired`,
   and from `GET …/pairing` when the panel opens while pairing runs) as a row
   with its type and serial and a form: name,
   room (a select of the Cube's rooms),
   a switch **Copy the room's settings** on by default with a help text naming the budget,
   **Put into a room** (`PUT …/devices/{rf}` with `name`, `room_id`, `copy`);
   the result with `showResult`; `pairing_ended` ends the countdown and says so.
   The flow is written from `components.js` and the `.field` rows;
   its state lives in `cubes.js` and survives a poll.
4. "Devices without a room" (`tiles.js`): with `structure`,
   each device gets **Put into a room…**, which opens the same `tpl-pair-device`
   form under the row, and **Remove from the Cube**,
   which asks with `confirmBox` (the text says the device is reset and must
   be paired again) and calls `DELETE …/devices/{rf}`.
5. The room configuration's Devices tab (`config.js`, `renderDeviceForm`):
   **Remove from the Cube** with `structure`, the same `confirmBox`;
   on success the `metadata` event reloads the rooms and the panel closes
   when its room is gone.
6. Texts: every new key in `i18n/en.json` and `de.json`,
   and machine-translated and marked in `fr`, `it`, `nl`, `pl`;
   `TestUITranslations` holds them; `TestUIPage` holds the partial and its ids.
7. Tests: `tools/ui-test/test_pairing.py`: in Settings → Cubes,
   Start with 5 seconds; the fake's device joins; name and room given;
   the device appears in the room's device list and the log shows the commands;
   then, from the Devices tab, Remove from the Cube, the confirm,
   the device gone from the room and the room's tile as before;
   the countdown's end text;
   a second test of **Put into a room…** on a device paired while the panel was
   closed (pair, close, reopen the page, put into a room, remove);
   each test ends with the server as it found it.
8. Documents: docs/ui.md (Settings → Cubes "Pair a device";
   Rooms "Devices without a room" and the Devices tab's Remove),
   README.md's Features bullet on pairing,
   `make ui-screenshots` for the shots that changed (the settings page
   where the panel's button shows), CHANGELOG.md under `### Added`.

**Verification:**
the test gate; `make ui-test` at 1100 and 390 px in English and German with the new tests;
`make ui-screenshots` and the changed shots looked at;
`GUIDELINES/shared/tools/check --commit-range` clean.

After the run, the coordinating session puts the hardware steps into
docs/handover-session.md's check 3: #10's six steps,
the copy on a real thermostat, `t:` on a device that answers.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +Pairing v1.0

Pairing's three missing parts (the room's settings on a device put into a room, a route that removes a device from the Cube, the web UI for both) get their plan registered before the first project write.

Key changes:
- GUIDELINES/wip/2026-11-06_20-35_AP_Pairing_copy_remove_panel.md: the plan, three blocks, the owner's decisions D1 to D3 in its Discussion.

File-revision-v1: GUIDELINES/wip/2026-11-06_20-35_AP_Pairing_copy_remove_panel.md::v1.0::2026-11-06::All::the first revision
Assisted-by: claude-code:claude-fable-5-1
```

The blocks' commits, each written at its gate, with the part as the component:

1. `feat(server.pairing): the room's settings on a device put into a room`
2. `feat(server.pairing): remove a device from the Cube`,
   with `test(fakecube): t: answered and the device dropped` where it is a
   commit of its own
3. `test(fakecube): a device that joins, from the command line`,
   `feat(ui.pairing): pair a device and put it into a room from the page`,
   `feat(ui.pairing): remove a device from the Cube in the page`,
   `test(ui-test): pairing and removal in the pytest suite`

## Change History

- v1.0 (2026-11-06): the first revision, from the findings of the same day's
  `PLAN pairing` and the owner's decisions D1 to D3.

# AP ClockWakeup v1.0: Wakeups and clocks

## TL;DR

Three actions the owner can trigger, each sending at most a few radiograms:
a wakeup of a room or a device, the Cube's clock set from the server's,
and the time sent to one device. The two untested lines are tried on the
test Cube first.

- a room's or a device's wakeup → only before the queue's configuration
  writes → a button: room menu, device's Repair…
- the Cube's clock → shown (`cube_time`), never compared or set
  → a warning when 5 minutes off, and "Set the Cube's clock" (`v:` with the time)
- a device's clock → only the Cube's own push, after its restart
  → "Send the time" (radiogram 0x03) to one device
- `z:1e` → read as 30 s, named minutes by the original
  → measured: about one minute, whatever the value (D3)

Blocks 3 and 4 shrink to what Block 1 confirms.

### Blocks

1. Test on the test Cube: the Cube's clock and the time sent to a device
2. Wakeups on demand: two routes, the room menu and the device's Repair…
3. The Cube's clock: a check at each greeting, and "Set the Cube's clock"
4. Time to a device: the builder, a route, "Send the time"

## Discussion

### What the owner asked for

On 2026-11-23 the owner asked whether a command sets a device's time from
the Cube; there is none. Then: `PLAN` the two things - setting the Cube's
own clock and the time sent to one device - "and also sending a wakeup call
to both the room and an individual device, if that's possible and not
already implemented". The TL;DR gate followed; on 2026-11-26 the owner had
session "AP FakeGroup" measure the wakeup's unit (D3) during its hardware
checks, and on 2026-11-27 sent `PLAN++ ClockWakeup as recommended`.

### What is decided

The owner, 2026-11-27, `PLAN++ as recommended`:

- **D1-A. Who may wake and send the time:** `repair`, managers and owners:
  both cost radio time and serve a diagnosis.
- **D2-A. Who may set the Cube's clock:** `cubes`, owners by default:
  a wrong Cube clock shifts every programme of that Cube.
- **D3-A. The wakeup's duration:** always `1e`, the queue's and the original
  software's value. Measured since (below): the value does not set the window.
- **D4-A. Where Block 1 runs:** the test Cube, nothing on the owner's
  household Cubes.
- **D5-A. A wakeup of every device (`z:…,A`):** not now.

## Analysis

### Wakeups, as measured

Session "AP FakeGroup" measured on the owner's test Cube (firmware 1.1.3),
2026-11-26 and 27, one radiator thermostat alone in its room;
its own plan records the findings in `docs/cube-protocol.md`:

- A room wakeup `z:1e,G,<room>` keeps the thermostat listening about one
  minute, ending between 60 and 70 s; `z:05` gave the same window,
  so the value does not set it (from 5 to 30 at least).
- Radiograms in the window do not prolong it.
- The wakeup lasts only while the LAN connection that sent it stays:
  sent on a new connection, it was over 12 s later.
  `serve` holds one connection, so its wakeups hold;
  its periodic refresh or Refresh now, a reconnect, would end one.
- A device wakeup `z:1e,D,<rf>` (built as `cube.CmdWakeup`, never sent by
  Massimo before) is answered `A:`, costs 2-3 % of the duty cycle,
  holds a slot a minute or more, and reaches a device in no room.
- A device that missed a room wakeup was flagged with the radiogram it then
  missed, not with 0xF1 WakeUp: a wakeup alone is no reachability test.
- Costs: a room wakeup about 3 %, a device wakeup 2-3 %,
  a radiogram to a sleeping device 9-11 %, to an awake one under 1 %.
  The duty cycle counts the last hour's air time.

So a wakeup on demand makes a device take what follows within a minute
cheaply; its answer can say what the Cube accepted and the budget, not who
heard it.

### The Cube's clock

- The H line carries the Cube's date and time to the minute
  (`docs/cube-protocol.md`, "The H line"); the API shows them as `cube_time`
  with `cube_time_at`, and nothing compares them with the server's clock.
- The Cube takes its time from NTP (`f:`, its servers).
- `cube.CmdSetTime` builds `v:<winter rule + summer rule>,<seconds>`,
  seconds since 2000-01-01 00:00 UTC as 8 hex digits; the original software
  1.4.6 has the same form (`MaxLocalApi.setCubeTime`, lowercase, unpadded)
  but was never seen sending it, so the epoch and the form are unconfirmed.
  `v:` without the time is verified (the time zone, `A:`).
- Unknown: whether the Cube pushes the new time to its devices after a `v:`
  (radio cost), and whether NTP overrides a set time at once.

### The time sent to a device

- Radiogram 0x03 TimeInformation (`cube.RadiogramTimeInformation`) is the
  Cube's own push after its restart. Massimo has no builder.
- FHEM's `14_CUL_MAX.pm` (`CUL_MAX_GetTimeInformationPayload`,
  `CUL_MAX_SendTimeInformation`), for its USB radio sticks, sends five bytes
  of local time: year - 2000, day, hour, minute with the month's bits 2-3 in
  its top two bits, second with the month's bits 0-1 in its top two bits
  ("month encoding is just guessed"); flags 0x04, group 0, to the device's
  own address; it answers a device's request with an empty payload the same
  way, and sends every thermostat the time twice a day.
- Unknown: whether the Cube passes a client's `s:` with 0x03 on;
  which flags it wants (FHEM's 0x04 with group 0, or 0x00 as Massimo's
  direct commands); whether the device takes it.

### Where things are

Routes as the other repairs (`internal/server/repair_api.go`) and device
actions; the queue's wakeup (`internal/engine/queue.go`, `Wakeup` with
`RoomID`); the Cube page's actions (`internal/server/ui/src/cubes/Actions.tsx`);
a device's Repair… menu and the room menu (`internal/server/ui/src/rooms/`);
the drift warning naming a thermostat's clock (`internal/server/drift.go`,
`hour_off`). The code goes to `main`, for 0.18.

## Implementation Plan

Block 1 runs with the owner, on the owner's go, through the path session
"AP FakeGroup" uses to the test Cube; each later block is built by an Opus
sub-agent without a commit, read by the session, and committed signed;
Blocks 2 to 4 are one pull request against `main`.
Verification is local for what a block touches; the whole suites are CI's.

### Block 1: Test on the test Cube

- **The Cube's clock:** read the H line; `v:` with the Cube's own rules and a
  time 7 minutes ahead; read the H line of the next connections for an hour,
  and the duty cycle; then `v:` with the right time.
- **The time sent to a device:** with the thermostat in auto, a device wakeup,
  then `s:` 0x03 with a time two hours off, as FHEM builds it (flags 0x04,
  group 0); the owner reads the device's clock, or its next setpoint shows
  the programme of the shifted hour; then the right time. Flags 0x00 where
  the first is discarded or not taken.
- The lines come from the builders (`CmdSetTime`, `CmdWakeup`) and, for 0x03,
  a scratch builder under `GUIDELINES/temp/`; each answer, slot and duty
  reading is kept.
- Findings into `docs/cube-protocol.md` the same day, with the date and the
  firmware, beside AP FakeGroup's.

**Verification:** each line's answer and the readings recorded; nothing left
changed on the test Cube (its clock and the thermostat's right again).

### Block 2: Wakeups on demand

- `POST /api/v1/cubes/{sn}/rooms/{id}/wakeup` and
  `POST /api/v1/cubes/{sn}/devices/{rf}/wakeup`, capability `repair` (D1-A):
  `z:1e,G,<room>` or `z:1e,D,<rf>` through the queue, held by its duty and
  slot holds; the answer is the Cube's acceptance and the budget.
- The UI: "Wake up" in the room's menu and in the device's Repair… menu,
  for `repair`; the result says the room or device listens about a minute.
- Tests against the fake Cube: the exact line, `403` without `repair`,
  `TestOpenAPICoversRoutes`.
- Docs: `docs/api-v1.md` (both routes, the `repair` row), the OpenAPI file,
  `docs/store.md`'s `repair` row, `docs/ui.md`; an `Added` CHANGELOG fragment.

**Verification:** `go test ./internal/server ./internal/engine`; `gofmt`;
`go vet` for linux, windows and darwin; `make ui-check`; the touched UI test
files in the four cases.

### Block 3: The Cube's clock

- At each greeting the server compares `cube_time` with its own clock and
  warns in the log where they differ by more than 5 minutes,
  the Cube's settings showing it; the warning clears when they agree.
- Where Block 1 confirmed `v:` with the time:
  `POST /api/v1/cubes/{sn}/clock`, capability `cubes` (D2-A), sending the
  server's time with the Cube's own rules, then a refresh that reads it back;
  "Set the Cube's clock" on the Cube page beside Refresh.
  `CmdSetTime`'s comment and `docs/cube-protocol.md` then say it is verified.
- Tests, docs (`docs/api-v1.md`, OpenAPI, `docs/store.md`, `docs/ui.md`),
  a CHANGELOG fragment.

**Verification:** as Block 2.

### Block 4: Time to a device

- Only where Block 1 confirmed it:
  `cube.CmdSetDeviceTime(rf, now)` in `internal/protocol/cube`, tested
  byte for byte against Block 1's line;
  `POST /api/v1/cubes/{sn}/devices/{rf}/time`, capability `repair` (D1-A),
  after a device wakeup; "Send the time" in the device's Repair… menu,
  and named as the remedy where a drift warning names the device's clock.
- Tests, docs, a CHANGELOG fragment.

**Verification:** as Block 2, and `go test ./internal/protocol/cube`.

## Proposed commit messages

- The plan: `chore(aps): +ClockWakeup v1.0`
- Block 1: `docs(cube-protocol): A Cube's clock set over the LAN, and a device's time`
- Block 2: `feat(api): A room or a device can be woken up on demand`
- Block 3: `feat(api): The Cube's clock is checked against the server's, and set from it`
- Block 4: `feat(api): A device can be sent the time`

## Change History

- v1.0 (2026-11-27): first revision, by `PLAN++ as recommended`,
  with D1-A to D5-A and AP FakeGroup's measurements of the wakeup.

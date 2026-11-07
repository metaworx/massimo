# Handover: verifying the configuration writes on real hardware

For an agent or a person at the Cube.
The configuration API and the UI panels (temperatures, device settings, offsets,
names, moves,
week programmes) were written and tested against the fake Cube only;
the bytes match the protocol builders that `restore` already used on real thermostats,
but the new endpoints themselves have not touched a device yet.
This is the test plan.
Stop at the first discard or at any value that does not come back as sent,
and record what happened.

## Setup

- `serve` running on the Mint box with both Cubes, the UI reachable,
  `-log serve.log` (or `MASSIMO_LOG` in `.env`) so every device state
  change and every passed command is on file.
- Work on the live house Cube,
  with `SN` set to its serial (in the Pi's `/etc/massimo/env`),
  in a room with one radiator thermostat and no wall thermostat,
  for example Halle 1 (room 1, thermostat 07631e) or Küche (room 2,
  thermostat 05cdc6).
- Everything below also works through the proxy: `status` and `backup`
  use the proxy automatically with `MASSIMO_CUBE_<SN>_PROXY` set.

## Before

```bash
go run ./cmd/massimo -show-hidden backup -sn "$SN" -out verify-before.yaml
```

Note the duty cycle and the free slots from `status`;
every step below costs about 2 to 3 % duty cycle and must not be run with the
duty cycle above 80 %.

## Steps

Each step: do the change in the UI (Configure… in the tile's ⋯ menu)
or with the curl line, wait for the answer in the panel, then run

```bash
go run ./cmd/massimo -show-hidden backup -sn "$SN" -out verify-after.yaml && diff verify-before.yaml verify-after.yaml
```

The diff must show exactly the intended change in the device's configuration
(and `taken_at`, the H line and possibly the L line), nothing else.
The serve log must show the `z:` wakeup of the room, the `s:` line,
the Cube's answer (not discarded) and the `c:` re-read.

1. **Eco temperature of the room, +0.5 and back.**
   UI: Temperatures tab, eco +0.5, Send.
   Or:

   ```bash
   curl -s -X PUT localhost:8099/api/v1/cubes/$SN/rooms/1/config -d '{"eco_temp": 17.5}'
   ```

   Expected: one 0x11 radiogram per thermostat of the room,
   direct addressing (flags 00),
   the diff shows `eco_temp: 17.5` for that thermostat.
   The thermostat's display shows the new eco value when its eco button is pressed.
   Then back to the old value the same way.
2. **Offset of the thermostat, +0.5 and back.**
   UI: Devices tab, the thermostat's offset.
   Or:

   ```bash
   curl -s -X PUT localhost:8099/api/v1/cubes/$SN/devices/07631e/config -d '{"temp_offset": 0.5}'
   ```

   Expected: one 0x11 radiogram, diff shows `temp_offset: 0.5`.
3. **Boost duration, 5 → 10 minutes and back.**
   UI: Devices tab, room settings.
   Or `{"boost_minutes": 10}` on the room's config.
   Expected: one 0x12 radiogram,
   the thermostat shows 600 s when boost is started (press Boost once,
   then again to stop; this costs a boost).
4. **Week programme, one day.**
   UI: Week programme tab, move one switch point of one day by 5 minutes, Send.
   Or:

   ```bash
   curl -s -X GET localhost:8099/api/v1/cubes/$SN/rooms/1/program
   ```

   then PUT the same JSON for one day with one time changed to `…/rooms/1/program`.
   Expected: one 0x10 radiogram with the room flag (flags 04, room id),
   the diff shows the changed `until`.
   Then back.
5. **Rename a device and back.**
   UI: Devices tab, name.
   Or `{"name": "Thermostat Halle 1x"}` on `…/devices/07631e`.
   Expected: an `m:` line, no radiogram, the diff shows the name,
   the legacy software (through the proxy) shows the new name after reconnecting.
6. **Rename a room and back**, the same way on `…/rooms/1`.
7. **Move a device, only if steps 1 to 6 passed**,
   and only with the window contact of that room (a contact's move costs a few
   radiograms that the contact picks up at its next transmission): Devices tab,
   move the contact to another room, confirm, then back.
   Expected: the 0x21/0x23 then 0x20/0x22 sequence in the log, two `m:` writes,
   the contact listed under the new room, then back.
   Open and close the window once after each move so the contact fetches its new links;
   the room stays "not ready" until the contact reported.

## After

A last backup and a diff against `verify-before.yaml` must be empty except for `taken_at`,
the H line and the L line.
`status` must show every real room ready.
Report: which steps passed, the log excerpts of any failure,
the duty cycle before and after.

## Known limits

- The room-level write uses the first device as the target for the
  queue's error-reset prelude; a device in error state other than the
  first may need a second attempt.
- The Cube drops unanswered radiograms slowly (about one per half hour);
  `status` shows them as pending.
- Nothing here changes room 255, the room for hidden devices, or its devices.

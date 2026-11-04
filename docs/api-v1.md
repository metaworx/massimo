# Massimo HTTP API, version 1

The API is served by `massimo serve` (default `127.0.0.1:8099`). Every
response is JSON unless stated otherwise. The machine-readable
description is in [api-v1-openapi.yaml](api-v1-openapi.yaml).

The server keeps one persistent connection and one command queue per
Cube. Reads answer from the server's copy of the Cube's state, which the
Cube refreshes on every change and which the server polls once a minute.
Writes go through the queue, which serialises commands, respects the
radio duty cycle and the Cube's memory slots, and resets a device's error
state when needed.

## Web UI

The server also serves a web page at `/` (files under `/ui/`) built on
this API, plain HTML and JavaScript embedded in the binary, no setup.
It shows one tile per room in the manner of the original software: the
measured temperature (IS, from a wall thermostat, or from a radiator
thermostat when it reported one; "–" otherwise) and the setpoint (SET)
side by side at the same size, with up and down chevrons next to SET
(shown on hover, always on touch screens) that change it in 0.5 °C
steps and send the value as a manual setpoint two seconds after the
last click, with Save and Cancel while it waits; AUTO, MANU and
HOLIDAY buttons, HOLIDAY opening the vacation panel and, while the room
is in vacation mode, highlighted with the end date next to it; a menu
with vacation, lock and unlock of the room's thermostats, the device
list and, for an empty room, deletion. A House tile sets every room to
ECO or COMFORT (manual at the room's eco or comfort temperature from
its thermostat's configuration), AUTO or MANU; each of its buttons
opens an inline panel with everything on one screen, no wizard: the
mode (ECO, AUTO, COMFORT, or MANU with a temperature), "Permanently"
or "Until" a date and time (tomorrow 06:00 by default; a temperature
with "Until" is sent as vacation mode), the rooms to apply it to with
an "All rooms" box, and Send, which runs the room commands one after
another and lists the result per room. Enter in any field sends.
Vacation for one room is an inline panel in its tile in the same way,
and deleting an empty room asks inline.
Clicking a room name opens its devices with their states, flags,
temperature offsets and per-thermostat lock. Setpoints are set per
room only, as in the original software. The gear in the header opens
the settings: the poll interval (10 s by default; the header is
refreshed on it, and the rooms as well while the event stream is
down; the ↻ button in the header runs the same refresh at once, rooms
included, and the header shows the time of the last successful poll
with seconds), the Expert switch (off by default; on, a second header
line shows the server version, the Cube's firmware, IP address and
radio address, the duty cycle, free slots, queued and pending
commands and the Cube clock, and the tiles show radio addresses and
device types), whether the Cube clock is shown in the browser's time
zone or the Cube's own, and "forget token". On every poll
the page compares the server's version from `/healthz` with the one
it was loaded with and reloads itself when they differ, immediately
when nothing is open, otherwise it offers a "New version on the
server, reload" button. The Cube clock is converted from the Cube's time zone rules
(`config.tz_winter` and `tz_summer` of the cube detail); hovering it
shows the same instant as UTC. The selected Cube and the settings are
kept in the browser. Results of every action appear in the tile,
including the devices to nudge when the Cube discarded a room command.
The page follows the event stream. With a token configured, the page
asks for it once and keeps it in the browser; the event stream alone
takes the token as `?token=` because browsers open it without
headers, every other route needs the header. The page and its files
are public, the API behind them is not.

## Authentication

With `MASSIMO_API_TOKEN` (or `-token`) set, every `/api` request must
carry `Authorization: Bearer <token>`; otherwise the response is `401`.
`/healthz` is always open. Without a token the API is unauthenticated,
which is only acceptable on the default localhost listener.

## Identifiers

- `{sn}` is a Cube's serial number, as printed on it (`KEQ0537249`);
  case does not matter.
- `{rf}` is a device's radio address, six hex digits (`012ca9`).
- `{id}` is a room number as the Cube assigns it (1..255).

## Errors

Errors are `{"error": "<message>"}` with a matching status: `400` bad
input, `401` missing or wrong token, `404` unknown Cube, device or room,
`409` the command was replaced by a newer one for the same target before
it was sent, `502` the Cube discarded the command or the connection
failed, `503` the Cube has not sent its greeting yet.

A room command is discarded by the Cube while it holds no state for a
device in that room: one that never reported, or one that has not
reported since the Cube restarted (window contacts only transmit on a
window change). The error message then names the devices and the
response carries their addresses as `no_state`. Make them report (open
and close the window, turn the thermostat's wheel), replace a dead
battery, or remove the device from the Cube:

```json
{"error": "engine: command discarded by the cube: the cube holds no state for room 4 members 005447, 1570d3 …", "no_state": ["005447", "1570d3"]}
```

## Routes

### GET /healthz

`{"ok": true, "version": "massimo v0.1.0 …"}`

### GET /api/v1/cubes

Every configured Cube:

```json
[
  {
    "serial": "JEQ0192422",
    "address": "192.168.1.9",
    "connected": true,
    "firmware": "1.1.3",
    "rf_address": "022839",
    "budget": {"duty_cycle_pct": 4, "free_memory_slots": 50, "updated_at": "2026-11-04T15:28:11+01:00"},
    "pending_radiograms": 0,
    "updated": "2026-11-04T15:28:11+01:00",
    "rooms": 5,
    "devices": 16,
    "queued": 0,
    "cube_time": "2026-11-04 15:28"
  }
]
```

`budget` is what the Cube last reported: the share of the hourly radio
budget already used, and the free memory slots (each unacknowledged
radiogram occupies one). `pending_radiograms` is the number of slots in
use, that is commands the Cube has sent and no device has acknowledged
yet; a setpoint shows in the states at once, but the mode follows only
with the device's report, so a non-zero value says the states may
still change. `queued` is the number of commands waiting in the server.

### GET /api/v1/cubes/{sn}

The same object with `config` (portal URL and flag, push button
settings, time zone rules) and `room_list`, the rooms as below.

### GET /api/v1/cubes/{sn}/rooms

Rooms with their devices and live states:

```json
[
  {
    "id": 5,
    "name": "Bad",
    "ready": true,
    "devices": [
      {
        "rf_address": "012ca9",
        "type": "radiator_thermostat",
        "name": "Thermostat Bad",
        "room_id": 5,
        "serial": "JEQ0328398",
        "reported": true,
        "state": {"mode": "auto", "setpoint": 22, "valve_pct": 0, "battery_low": false, "error": false, "link_error": false}
      }
    ]
  }
]
```

Device types: `radiator_thermostat`, `radiator_thermostat_plus`,
`wall_thermostat`, `shutter_contact`, `eco_switch`. `reported` says
whether the Cube holds state for the device; it is false for every
device after a Cube restart until the device transmits again (window
contacts only do so on a window change), and `state` is then absent.
`ready` on a room is true when all its devices have reported, which is
the condition for the Cube to accept a room command. Thermostats carry `mode` (`auto`,
`manual`, `vacation`, `boost`), `setpoint`, `actual_temp` when the device
reported one, `valve_pct` for radiator thermostats, `until` for vacation
mode; shutter contacts carry `window_open`. `error` is the Cube's
transmit error flag, `link_error` a failed link with a partner device,
`panel_locked` (present when true) the key lock on the device.

### GET /api/v1/cubes/{sn}/devices

Every device the Cube knows, in the Cube's order. Devices that are
paired but not assigned to a room have `"unlisted": true` and no name;
they do not appear under rooms.

Devices in room 255, the ghost room, and devices listed in
`MASSIMO_CUBE_<SN>_IGNORE` are invisible to the API unless the server
runs with `-show-ghosts`: they are absent from every list and count,
their addresses answer `404`, their events are not streamed, and a
room that holds only such devices is absent as well, including from
room commands. The use case is a device that is paired but never
reports, which blocks every room command for the room the Cube counts
it in and which the Cube never drops; it is parked in room 255 on the
Cube and hidden here (see [portal-protocol.md](portal-protocol.md)).

### GET /api/v1/cubes/{sn}/devices/{rf}

One device with its configuration: `firmware`, and for thermostats
`thermostat` or `wall_thermostat` with the preset temperatures, valve or
boost settings and the week program keyed by weekday (`monday` …
`sunday`, each a list of `{"temp": 17, "until": "07:00"}`). A wall
thermostat also reports `display_actual`.

### PUT /api/v1/cubes/{sn}/devices/{rf}/setpoint

Set mode and setpoint of one thermostat. Body:

| mode       | fields                                   | effect                                        |
|------------|------------------------------------------|-----------------------------------------------|
| `manual`   | `temp` (4.5 … 30.5, steps of 0.5)        | hold `temp`                                   |
| `auto`     | optional `temp`                          | follow the week program (`temp` until the next switch point) |
| `vacation` | `temp`, `until` (`"YYYY-MM-DD HH:MM"`, half hours) | hold `temp` until `until`, then auto |

```json
{"mode": "manual", "temp": 21}
```

The request waits for the Cube's answer (up to two minutes when the
radio budget is exhausted) and returns

```json
{"ok": true, "discarded": false, "budget": {"duty_cycle_pct": 5, "free_memory_slots": 49, "updated_at": "…"}}
```

A device that belongs to a room is addressed as its room, because the
Cube discards a direct command to a room member; every thermostat of
that room therefore takes the new setpoint, which is how MAX! treats
setpoints. A device outside any room is addressed directly. Boost
cannot be started over the air.

### PUT /api/v1/cubes/{sn}/rooms/{id}/setpoint

Same body and answer, addressed to every device of the room, which is
what the original software does.

### DELETE /api/v1/cubes/{sn}/rooms/{id}

Removes an empty room by writing the metadata without it; answers
`{"ok": true, "rooms": 5}`. A room that still has devices, hidden ones
included, is refused with `409`, and the ghost room is not found.

### PUT /api/v1/cubes/{sn}/rooms/{id}

Rename a room: `{"name": "Küche"}`, up to 32 bytes. A metadata write;
answers `{"ok": true}`.

### PUT /api/v1/cubes/{sn}/devices/{rf}

Rename a device and/or move it to another room:
`{"name": "Thermostat Küche", "room_id": 2}`, either field alone is
fine. A move is done as the original software does it: the device is
unlinked from every partner of its old room and its group removed,
then linked with every partner of the new room and the new group set,
each after a wakeup of the room concerned, then the metadata is
written. `room_id` must name an existing room; 0 is refused, a device
outside every room would be a ghost. A device that is paired but not
assigned to a room answers `409`; pair it with the original software.

### PUT /api/v1/cubes/{sn}/devices/{rf}/config

Write a thermostat's settings. Every field is optional and merged into
the device's current configuration:

```json
{"comfort_temp": 21, "eco_temp": 17, "max_setpoint": 30.5, "min_setpoint": 4.5,
 "temp_offset": -0.5, "window_open_temp": 12, "window_open_minutes": 15,
 "boost_minutes": 5, "boost_valve_pct": 80, "decalc_day": "saturday", "decalc_hour": 12,
 "max_valve_pct": 100, "valve_offset_pct": 0, "display_actual": true}
```

Temperatures 4.5 to 30.5 in steps of 0.5, the offset −3.5 to 3.5,
window minutes 0 to 60 in steps of 5, boost 0 to 30 in steps of 5 or
60, boost valve 0 to 100 in steps of 5, hours 0 to 23, valve
percentages 0 to 100; `display_actual` is for wall thermostats only.
The temperature fields go as radiogram 0x11, the boost, descaling and
valve fields as 0x12, the display as 0x82, each directly to the device
after a wakeup of its room. Wall thermostats take the same radiograms;
the values they do not store are filled in from a radiator thermostat
of the room. Shutter contacts and eco switches answer `400`. The answer
is the same as for a setpoint; afterwards the server re-reads the
device's configuration from the Cube.

### GET /api/v1/cubes/{sn}/rooms/{id}/config

The room's settings as the UI edits them: the common fields above
(without `temp_offset` and `display_actual`) from the room's first
radiator thermostat, else its wall thermostat, named in `source`, and
a `devices` list with each thermostat's `rf_address`, `type`, `name`,
`temp_offset` and, for wall thermostats, `display_actual`.

### PUT /api/v1/cubes/{sn}/rooms/{id}/config

The same body as for a device, without `temp_offset` and
`display_actual`, applied to every thermostat of the room after one
wakeup: temperatures to radiator and wall thermostats, boost to both,
descaling and valve limits to radiator thermostats only. The answer
lists every device:

```json
{"ok": true, "devices": [{"rf_address": "012ca9", "name": "…", "ok": true, "discarded": false}], "budget": {…}}
```

`ok` is true when every device took the write; `502` only when none
did.

### GET /api/v1/cubes/{sn}/rooms/{id}/program

The week programme of the room's first radiator thermostat (else its
wall thermostat), keyed by weekday as in a backup, plus `source`.

### PUT /api/v1/cubes/{sn}/rooms/{id}/program

Write the week programme of a whole room, any subset of days:

```json
{"monday": [{"temp": 17, "until": "07:00"}, {"temp": 21, "until": "20:30"}, {"temp": 17, "until": "24:00"}],
 "saturday": [{"temp": 19, "until": "24:00"}]}
```

Each day has 1 to 13 points with increasing times, the last at 24:00
(`"0700"` and `"until_min": 420` are accepted as well). One radiogram
0x10 per day (two for a day with more than 7 points) goes to the room
as a group command, which every thermostat of the room takes, as the
original software sends it. `PUT …/devices/{rf}/program` writes the
same body to one thermostat directly. Both answer like a setpoint.

### PUT /api/v1/cubes/{sn}/devices/{rf}/lock

Lock or unlock the buttons of one thermostat (the key lock, "LoC" on the
display, `panel_locked` in its state). Body `{"locked": false}`; the
answer is the same as for a setpoint. The command is radiogram 0x80,
which the original software lists but never sends; it is sent directly
to the device after a wakeup of its room and takes effect immediately
(both directions verified on radiator thermostats). On the device itself the lock is toggled by pressing Auto/Manu
and the second button (Boost or the moon/sun key, Mode/Menu and
Boost/OK on a wall thermostat) together.

### GET /api/v1/cubes/{sn}/backup?format=yaml|json

A backup file as `massimo backup` writes it: rooms, devices with
configuration and state, Cube settings, NTP servers, plus the raw
protocol lines. YAML comes as an attachment named
`massimo-backup-<serial>-<time>.yaml`.

### GET /api/v1/cubes/{sn}/events

A `text/event-stream`. On connect, one `state` event per device with its
current data; afterwards an event whenever the Cube reports a change:

```
event: state
data: {"kind":"state","device":"012ca9","state":{…},"time":"2026-11-04T15:30:12+01:00"}

event: disconnected
data: {"kind":"disconnected","error":"read tcp …: i/o timeout","time":"…"}
```

Kinds: `connected`, `disconnected` (the server reconnects by itself),
`hello` (new budget or clock from the Cube), `metadata` (rooms or names
changed), `config` (a device configuration arrived), `state`. A comment
line is sent every 30 seconds to keep the connection alive.

## The LAN proxy

Besides the API, `massimo serve` answers as a Cube on TCP port 62910
and UDP discovery, so that programs speaking the Cube protocol share
the one connection the Cube allows. See [proxy.md](proxy.md).

## Not in version 1

Pairing and removing devices, creating rooms, user accounts and
ownership. Pairing and removing need the original software for now;
`massimo restore` from an edited backup covers everything else in bulk.

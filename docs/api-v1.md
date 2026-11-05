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
side by side at the same size, each with a small icon above its label
(a thermometer for IS, a gauge for SET), with up and down chevrons
next to SET (shown on hover, always on touch screens) that change it
in 0.5 °C steps and send the value as a manual setpoint two seconds
after the last click, with Save and Cancel while it waits; AUTO, MANU
and HOLIDAY buttons, HOLIDAY opening the vacation panel and, while the
room is in vacation mode, highlighted with the end date next to it;
AUTO and MANU close an open vacation panel. Right of the mode buttons
an info button opens the device list and a cog the configuration, one
of the two at a time, a click on the highlighted one closes it. The
⋯ menu keeps lock and unlock of the room's thermostats and, for an
empty room, deletion. A House tile sets every room to
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
The device list (room name or info button) shows each device with its
mode or window state as a filled badge at the right (green auto and
closed, amber manual, blue holiday, red open), warnings outlined
before it, type and address on a second line, and the values in two
columns with the decimals aligned: valve and offset left, set and
actual right in bold with the gauge and thermometer icons, and the
per-thermostat lock. Setpoints are set per room only, as in the
original software. The cog of a tile
expands the tile into the room's configuration, one screen per
section with Send and Cancel, Enter sends: Room (rename), Temperatures
(comfort, eco, open-window temperature and minutes, maximum and
minimum; only the changed fields are written, to every thermostat of
the room, with the result per device), Devices (boost duration and
valve opening, descaling day and hour, valve limits for the room, then
per device its name, temperature offset, the display mode of a wall
thermostat, and a move to another room behind an inline confirm), Week
programme (day buttons: a click edits that day and selects every day
with the same programme; the "Only {day}" switch, off by default,
sends an edit to all selected days, on to the edited day alone, which
then leaves the group; the day's switch points as rows "until … at …
°C" with add and remove, the last fixed at 24:00, a bar of the day
above them; a "Copy {day} to …" section with every day as a toggle,
the days already sharing the programme greyed out, "All days" and
Copy, after which the copied days join the group while the switch is
off; Reset day and Reset all left of Cancel and Send; Send writes the
days that differ from the loaded programme), and Copy (a Rooms section
with a switch per room, "All rooms" and "None", and a Settings section
with Temperatures, Week programme and Radiator settings as switches;
what is stored for this room goes to each selected room, one after
another, through `PUT …/rooms/{id}/config` with the chosen fields and
`PUT …/rooms/{id}/program`, with a result per room). A help button
next to each setting shows a short explanation of it. All texts come
from one translation table (`ui/i18n.js`), English and German so far;
the language follows the browser or the Language setting. Icon buttons carry labels for
screen readers, the mode buttons are a group with their pressed
state, the page icons a navigation with the current page marked,
every control has a visible focus ring, and Escape closes an open menu
or panel; the pages pass an axe-core check without violations. Three
icon buttons at the
right end of the header switch between the pages Rooms, Log and
Settings: the log lists the Cube's recent events from
`GET …/log` and then the live ones, one line each with the local time,
kind, device and a short state text, with a filter box and a clear
that only empties the view; the tiles keep their state meanwhile. The
short server version stands next to the brand; the Cube selector
shows the Cube's name when one is configured. The settings page has
the poll interval (10 s by default; the header is
refreshed on it, and the rooms as well while the event stream is
down; the ↻ button in the header runs the same refresh at once, rooms
included, and the header shows the time of the last successful poll
with seconds), the Expert switch (off by default; on, a second header
line shows the server version, the Cube's firmware, IP address and
radio address, the duty cycle, free slots, queued and pending
commands and the Cube clock, and the tiles show radio addresses and
device types), whether the Cube clock is shown in the browser's time
zone or the Cube's own, and "forget token". In the tiles a thermometer
marks the measured temperature (IS) and a target the setpoint (SET);
AUTO and MANU close an open holiday panel and drop a pending stepper
value. The device list shows per device its name with the mode or
window state as a badge and the warnings next to it, the type and
address beneath, and valve, offset, set and actual in a grid with the
decimal points aligned. On every poll
the page compares the server's version from `/healthz` with the one
it was loaded with and reloads itself when they differ, immediately
when nothing is open, otherwise it offers a "New version on the
server, reload" button. The Cube clock is converted from the Cube's time zone rules
(`config.tz_winter` and `tz_summer` of the cube detail); hovering it
shows the same instant as UTC. The selected Cube and the settings are
kept in the browser. Results of every action appear in the tile,
including the devices to nudge when the Cube discarded a room command.
The page follows the event stream. With a token configured, the page
asks for it once and keeps it in the browser; every route needs the
header except the event stream, which browsers open without headers
and which takes a ticket (see Authentication): the page gets a new one
for every connection and, after an error, closes the stream and opens
it again with a fresh ticket, waiting longer after each failure. The
tiles and the device list show how old the values are from `received`
and `last_seen`, and warn about stale devices. The page and its files
are public, the API behind them is not.

## Authentication

With `MASSIMO_API_TOKEN` (or `-token`) set, every `/api` request must
carry `Authorization: Bearer <token>`; otherwise the response is `401`.
`/healthz` is always open. Without a token the API is unauthenticated,
which is only acceptable on the default localhost listener.

Ten failed token checks from one address within a minute lock that
address out for five minutes: every `/api` request from it is then
answered `429` with a `Retry-After` header, the right token included.
The address is the TCP peer's; `X-Forwarded-For` is not trusted, since
anyone can set it, so behind a reverse proxy the limit applies to the
proxy as a whole, and the proxy should limit by client itself.

The event stream is opened by browsers without headers, so it takes a
ticket instead: `POST /api/v1/cubes/{sn}/events/ticket` with the token
returns one, and `GET /api/v1/cubes/{sn}/events?ticket=<ticket>` opens
the stream with it. A ticket is good for one connection to that Cube's
stream within a minute; a client gets a new one for every connection,
also after the stream broke off (an `EventSource` stops retrying after
a `401`, so it reports the error and the client opens a new one).
`?token=<token>` on the stream is still accepted but deprecated since
2026-11-05, and the response says so with the header
`Deprecation: @1793836800` (RFC 9745): it puts the token into URLs,
browser history and proxy logs, and it will be removed in a later
release. A wrong ticket counts as a failed token check.

## Identifiers

- `{sn}` is a Cube's serial number, as printed on it (`KEQ0537249`);
  case does not matter.
- `{rf}` is a device's radio address, six hex digits (`012ca9`).
- `{id}` is a room number as the Cube assigns it (1..255).

## Errors

Errors are `{"error": "<message>"}` with a matching status: `400` bad
input, `401` missing or wrong token, `429` locked out after repeated
wrong tokens, `404` unknown Cube, device or room,
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

Every configured Cube, then every Cube discovery found on the LAN that
is not configured:

```json
[
  {
    "serial": "JEQ0192422",
    "name": "Ground floor",
    "address": "192.168.1.9",
    "configured": true,
    "mac": "00:1a:22:0a:0b:0c",
    "last_seen": "2026-11-04T15:00:02+01:00",
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
  },
  {
    "serial": "KEQ0000000",
    "configured": false,
    "address": "192.168.1.23",
    "mac": "00:1a:22:01:02:03",
    "rf_address": "0a0b0c",
    "firmware": "1.1.3",
    "found_at": "2026-11-04T14:30:01+01:00",
    "last_seen": "2026-11-04T15:00:02+01:00"
  }
]
```

`configured` tells the two apart. A configured Cube has the fields
below; `mac` and `last_seen` (when it last answered discovery) only
once discovery has seen it. A found Cube has only the fields shown:
`address` is the IP it answered from, `found_at` when the server first
saw it, `last_seen` its latest answer; `mac` comes from the host's ARP
cache and may be missing. To use it, configure it (its address, or its
serial with `-sn` or a `_NAME`, see the README) and restart serve. The
answers of serve's own proxies (docs/proxy.md) are not listed.

`budget` is what the Cube last reported: the share of the hourly radio
budget already used, and the free memory slots (each unacknowledged
radiogram occupies one). `pending_radiograms` is the number of slots in
use, that is commands the Cube has sent and no device has acknowledged
yet; a setpoint shows in the states at once, but the mode follows only
with the device's report, so a non-zero value says the states may
still change. `queued` is the number of commands waiting in the server.
`observed_since` is when the server first heard from the Cube (see the
ages of device values below).

The Cube reports its budget only in the answer to a radio command and
when a connection starts, so `budget` and `pending_radiograms` do not
change on their own while devices acknowledge. After radio writes
through the API, once the devices had two minutes and the re-reads
below are done, the server reconnects to the Cube for a fresh report if
radiograms were still pending, at most every 15 minutes and only while
the periodic refresh (`serve -refresh`) is on; the stream then sends
`hello` and `connected`, and `connected` is false for the moment of the
reconnect. Likewise the Cube changes a device's configuration only once
the device has acknowledged a write, which took one to four minutes on
hardware: after a configuration, programme or room write the server
reads the device's configuration again after 15 s, 30 s, 1, 2 and 2
minutes, until it changed, and the stream sends a `config` event when
it does. Until then `GET …/devices/{rf}` and the room configuration show
the old values.

### POST /api/v1/cubes/discover

Looks for Cubes on the LAN now (UDP discovery, about three seconds) and
answers with every Cube that answered, configured or not, as the
objects of the found Cubes above:

```json
{
  "ran_at": "2026-11-04T15:00:00+01:00",
  "found": [
    {"serial": "JEQ0192422", "configured": true, "address": "192.168.1.9", "mac": "00:1a:22:0a:0b:0c",
     "rf_address": "022839", "firmware": "1.1.3", "found_at": "2026-11-04T14:30:01+01:00", "last_seen": "2026-11-04T15:00:02+01:00"},
    {"serial": "KEQ0000000", "configured": false, "address": "192.168.1.23", "mac": "00:1a:22:01:02:03",
     "rf_address": "0a0b0c", "firmware": "1.1.3", "found_at": "2026-11-04T14:30:01+01:00", "last_seen": "2026-11-04T15:00:02+01:00"}
  ]
}
```

No body. A call while a run is going waits for it and gets its result,
the same `ran_at`. `503` when the server cannot have the discovery
port (UDP 23272, the only one the Cubes answer to), for example because
another program holds it. `serve` also runs discovery at the start and
every 30 minutes (`-discover-interval`, `0` for on request only), and
every minute while a Cube configured by serial has no address yet. The
first time a Cube that is not configured answers, every Cube's event
stream sends `cube_found`, once per serial and server run:

```
event: cube_found
data: {"kind":"cube_found","cube":{"serial":"KEQ0000000","configured":false,"address":"192.168.1.23",…},"time":"…"}
```

A configured Cube that answers from another address than configured is
logged; one configured by serial only, without an address, is connected
at the address it answers from and reconnected when that changes.

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

How old the values are: a device with state also has `received`, the
time per value of `state` (same names) at which the server saw its
current value appear, `last_seen`, when the device's entry in the
Cube's device list last changed in any way, and, for radiator and wall
thermostats, `stale: true` once that is more than 24 hours ago; a room
with a stale device is `stale` too. For example:

```json
"received": {"mode": "2026-11-05T08:12:40+01:00", "setpoint": "2026-11-05T08:12:40+01:00",
             "actual_temp": "2026-11-05T06:01:10+01:00", "valve_pct": "2026-11-05T09:40:02+01:00", …},
"last_seen": "2026-11-05T09:40:02+01:00"
```

The Cube keeps no times and repeats every value in each device list,
so these are the times the server saw a value change, which is as close
as a client gets to when the device sent it. A device that reports the
same value again does not move its time, and the Cube itself changes
a setpoint in the list right after a set-temperature command, before
the devices confirm it. Values that were already there when the server
first connected carry that time, the Cube's `observed_since`, and may
be older; a reconnect keeps the times of values that did not change.
Radiator thermostats report the measured temperature only now and then
(after the valve moved), which is what `received.actual_temp` is for.
`stale` is a hint: a thermostat out of range or with a flat battery
goes quiet, but so may one whose valve stays shut outside the heating
season. Shutter contacts and eco buttons transmit only on events and
are never stale.

### GET /api/v1/cubes/{sn}/devices

Every device the Cube knows, in the Cube's order. Devices that are
paired but not assigned to a room have `"unlisted": true` and no name;
they do not appear under rooms.

Hidden devices, those in room 255 and those listed in
`MASSIMO_CUBE_<SN>_IGNORE`, are invisible to the API unless the server
runs with `-show-hidden`: they are absent from every list and count,
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
included, is refused with `409`, and room 255 is not found.

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
outside every room would be a device that blocks no room but cannot
be addressed by one either.

A device that is paired but in no room yet (`"unlisted": true`, such
as one just paired, see Pairing) needs `room_id`; `name` alone answers
`409`. It is added to the room as the original software adds a new
device: after a wakeup of the room (`z:1e,G,<room>`), one 0x20 add link
partner radiogram per room member it links with in each direction (to
a radiator thermostat with the room flag and its room, to every other
device directly, and from the new device to each partner; two shutter
contacts are not linked with each other), then 0x22 set group, then the
metadata with the device listed under its name in the room, then a
re-read of its C line. A device whose C line names another room (paired
long ago and never listed) is first unlinked from that room with 0x21
remove link partner and 0x23 remove group, as in a move. The room's
temperatures, valve settings and week programme are not copied to the
new device, unlike in the original software, which sends 0x40 auto,
0x11, 0x12 and 0x10 afterwards: use `PUT …/rooms/{id}/config` and
`PUT …/rooms/{id}/program` for that. The answer is
`{"ok": true, "budget": {…}}`.

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

### Pairing: POST, GET, DELETE /api/v1/cubes/{sn}/pairing

`POST` with `{"seconds": 60}` (optional; 1 to 600, default 60) puts
the Cube into inclusion mode with `n:<seconds>`; a device whose pairing
button is pressed then joins. The Cube may end the mode earlier on its
own. The answer is the pairing state:

```json
{
  "active": true,
  "started": "2026-11-05T16:02:10+01:00",
  "until": "2026-11-05T16:03:10+01:00",
  "paired": []
}
```

`409` while pairing already runs, `503` while the Cube is not
connected, `400` for a bad `seconds`. `GET` returns the same state at
any time; after the end `active` is false, `ended` says when, and
`paired` keeps the devices of the last pairing until the next one
starts:

```json
{
  "active": false,
  "started": "2026-11-05T16:02:10+01:00",
  "until": "2026-11-05T16:03:10+01:00",
  "ended": "2026-11-05T16:02:41+01:00",
  "paired": [{"type": "radiator_thermostat", "rf_address": "0a0b0c", "serial": "TEQ0000001"}]
}
```

`DELETE` ends pairing with `x:` (also when the server counts it as
over already, so that a Cube still in the mode stops) and returns the
state. Pairing ends when the Cube says so (an empty `N` line), on
`DELETE`, or ten seconds after `until`, whichever is first.

The event stream reports `pairing_started`, then a `paired` event per
device that joins, with `"device": "<rf>"` and `"paired": {"type": …,
"rf_address": …, "serial": …}`, and `pairing_ended`. A joined device
is in the device list right after (`GET …/devices`, with
`"unlisted": true` and no name; the server fetches its configuration
with `c:`) but in no room and not under `rooms`. Put it into a room
with `PUT …/devices/{rf}` and `{"name": "…", "room_id": N}` (above).
A client pairing a device therefore: `POST …/pairing`, waits for the
`paired` event (or polls `GET …/pairing`), asks the user for a room
and a name, `PUT …/devices/{rf}`, and `DELETE …/pairing` if more time
is left and no further device is to come. Pairing through the LAN proxy,
as the original software does it, uses the same mode (proxy.md).

### GET /api/v1/cubes/{sn}/backup?format=yaml|json

A backup file as `massimo backup` writes it: rooms, devices with
configuration and state, Cube settings, NTP servers, plus the raw
protocol lines. YAML comes as an attachment named
`massimo-backup-<serial>-<time>.yaml`.

### GET /api/v1/cubes/{sn}/log?limit=200

The last events of the Cube, oldest first, the same objects the event
stream sends (`kind`, `device`, `state`, `error`, `time`); the server
keeps the last 500. For a log view that starts with history and
continues with the stream.

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
changed), `config` (a device configuration arrived), `state`,
`pairing_started`, `paired` (with `paired`, the device that joined),
`pairing_ended` (see Pairing) and `cube_found` (with `cube`, a Cube that
is not configured, see POST /cubes/discover). A comment line is sent every 30 seconds
to keep the connection alive.

With a token configured the stream needs `?ticket=<ticket>` (below) or
the bearer header; `?token=<token>` still works, deprecated.

### POST /api/v1/cubes/{sn}/events/ticket

A single-use ticket for that Cube's event stream, valid for a minute:

```json
{"ticket": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822c", "expires_at": "2026-11-05T15:31:12+01:00"}
```

Without a token configured the stream is open and the ticket not
needed, but the route answers all the same.

### GET /api/v1/update?refresh=1

Whether a newer signed release is out on the server's update channel
(the branch it was built from, or `MASSIMO_UPDATE_CHANNEL`). The server
asks GitHub at most every ten minutes; `refresh` asks now.

```json
{
  "current": "v0.1.0-alpha.9",
  "channel": "alpha",
  "latest": "v0.1.0-alpha.10",
  "release_url": "https://github.com/metaworx/massimo/releases/tag/v0.1.0-alpha.10",
  "available": true,
  "checked_at": "2026-11-05T13:40:00+01:00",
  "can_update": true,
  "helper": {"state": "done", "from": "v0.1.0-alpha.8", "to": "v0.1.0-alpha.9", "channel": "alpha", "time": "…"}
}
```

`latest` is only a release whose manifest verifies against the key
compiled into the server (see `internal/update`). `check_error` says why
a check failed. `can_update` is true when POST would start an update;
otherwise `blocked` says why it cannot be started from here (no API
token, no update helper, Windows). `helper` is the update helper's last
run: `state` is one of `checking`, `installing`, `restarting`, `done`,
`up-to-date`, `failed`, `rolled-back`, with `message` on failure.

### POST /api/v1/update

Asks the update helper to install the newest release on the channel:
the server writes the channel into its request file, and the helper
(`massimo-update.service`, running as root; see
[packaging/README.md](../packaging/README.md)) downloads and verifies the
release, test-runs it, replaces the binary, restarts the server and
rolls back if the new version does not answer `/healthz`. The request
names no version and no file, so the most the API can do is install the
newest signed release. Answers `202` with `{"ok":true,"requested":"v…",
"channel":"…"}`; follow the progress with GET.

Needs the API token: without `MASSIMO_API_TOKEN` it answers `403` even
though the other routes are open. `403` also when no helper is
configured (`serve -update-request`) or on Windows; `409` when already up
to date, when the channel has no release or while an update is running;
`502` when GitHub could not be asked.

## The LAN proxy

Besides the API, `massimo serve` answers as a Cube on TCP port 62910
and UDP discovery, so that programs speaking the Cube protocol share
the one connection the Cube allows. See [proxy.md](proxy.md).

## Not in version 1

Removing devices, creating rooms, user accounts and ownership.
Removing needs the original software for now; `massimo restore` from an
edited backup covers everything else in bulk.

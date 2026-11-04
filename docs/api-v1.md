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
radiogram occupies one). `queued` is the number of commands waiting.

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
transmit error flag, `link_error` a failed link with a partner device.

### GET /api/v1/cubes/{sn}/devices

Every device the Cube knows, in the Cube's order. Devices that are
paired but not assigned to a room have `"unlisted": true` and no name;
they do not appear under rooms.

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

## Not in version 1

Pairing and removing devices, renaming, week programs and settings
writes, user accounts and ownership. Settings are written with
`massimo restore` from an edited backup for now; pairing needs the
original software.

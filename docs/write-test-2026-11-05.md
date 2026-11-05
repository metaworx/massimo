# Write test on hardware, 2026-11-05

Steps 1 to 6 of [handover-write-test.md](handover-write-test.md) were
run against the live house Cube JEQ0192422 (firmware 1.1.3). They used
room 2 and its only radiator thermostat 05cdc6 (firmware 22), not room 1,
because room 1's thermostat has its keys locked, so its eco and boost
buttons cannot be pressed to check a value. Step 7 (moving a device) was
not run; it needs a separate go.

**Result: every step passed.** Every write was accepted by the Cube (none
discarded) and came back from the Cube exactly as sent. After every
revert the configuration was identical to the backup taken before. The
final backup differs from the first only in `taken_at`, the H and L
lines, and two measured temperatures, and every room is ready.

## Setup

- `massimo serve` v0.2.0-alpha.1 as a systemd service on a Raspberry
  Pi 1 (see [packaging/README.md](../packaging/README.md)), API on port
  80, the Cube's proxy on a separate address of the Pi; nothing else
  connected to the Cube.
- Writes with curl against the Pi's API, Bearer token from the env
  file. Backups with `massimo -show-ghosts backup -sn JEQ0192422`
  through the proxy.

## Steps

| Step | Write (then back) | Radio | Answer | Seen in the Cube's configuration | Result |
|---|---|---|---|---|---|
| 1 | room config `eco_temp` 16.5 → 17 | 0x11 | ok, not discarded | `eco_temp: 17`; on the thermostat the eco button showed 17 °C (comfort 20 °C unchanged) | passed |
| 2 | device config `temp_offset` 0 → 0.5 | 0x11 | ok, not discarded | `temp_offset: 0.5` only | passed |
| 3 | room config `boost_minutes` 5 → 10 | 0x12 | ok, not discarded | `boost_minutes: 10` only | passed |
| 4 | room programme, Monday's first switch point 07:00 → 07:05 | 0x10, room | ok, not discarded | Monday `until: 07:05`, other days unchanged | passed |
| 5 | device name + "x" | none (`m:`) | ok | name only | passed |
| 6 | room name + "x" | none (`m:`) | ok | room name only | passed |

Each revert left the configuration identical to the first backup.

Duty cycle: 4 % before, 14 % after the eight radio writes (about 1.25 %
each, lower than the 2 to 3 % the plan expected). Free memory slots:
50 before and after.

## Findings

1. **The server's view of a write is stale.** After a write the server
   re-reads the device's configuration at once, but the Cube only
   updates its C line when the thermostat has acknowledged, which took
   up to a few minutes here. The server does not read it again, so the
   API, the UI and backups through the proxy kept showing the old value
   (eco 16.5) for minutes, although the thermostat already used the new
   one. The same goes for the budget and `pending_radiograms`: they only
   change with the next command reply or reconnect, so `status` through
   the proxy kept showing 48 free slots. For the test, `serve` was
   restarted before each check, which makes the Cube send all its data
   afresh. Follow-up: re-read the configuration (and the H line) once
   the pending count of the Cube drops, or poll it for a few minutes
   after a write. Addressed on 2026-11-05: the server re-reads the
   device's C line after 15 s, 30 s, 1, 2 and 2 minutes until it
   changes, and reconnects for the budget afterwards if radiograms were
   pending (docs/api-v1.md, under GET /cubes).
2. **The log cannot show the sequence the plan asks for.** The command
   queue only logs discarded commands and budget waits, so the `z:`,
   `s:` and `c:` lines and the Cube's answer of an accepted command do
   not appear in the serve log. Follow-up: log accepted commands too,
   with target, command and budget. Addressed on 2026-11-05: "room woken
   up" and "command accepted by the cube" lines.
3. **One radiogram stayed pending for about 35 minutes.** After step 1a
   the Cube held two radiograms, and one of them stayed pending until
   about 16:17, while all later writes were delivered. It cleared by
   itself, and nothing was lost.
4. Unrelated to the test: a window in room 4 was opened and closed
   around 15:41; the thermostat of that room showed its window-open
   temperature in one backup.

## Open items from the install (until confirmed)

- DHCP reservations in the mobile router's network: the Cube at .118
  and the Pi at .48.
- The Pi's fixed addresses .200 (web UI and API) and .201 (Cube proxy)
  are outside the router's DHCP range of 20 to 199 (confirmed by
  Martin). The earlier .49 and .50 are no longer used.

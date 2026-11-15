# Transmit error (D102): the restart of .22 (D115)

2026-11-15, morning; follows `2026-11-14_20-30_ANALYSIS_RadioHealth_transmit_error_d102_v2.md`
and the analysis of 2026-11-14 19:15.
Cubes by their own IP suffix: .21, .22, .23, .24, .25 (proxies .31–.35).
Devices by room and type in breno's store ("rad r3": a radiator thermostat
of room 3); no serials, radio addresses or names. Times are local (CET).

What was done: the owner put a battery into one of .22's window contacts and
cycled its window three times, then pressed Reboot in .22's panel
(2026-11-14 22:32). On the owner's word this session restarted `massimo` on
breno once before that (21:24:57), to read the slots from the greetings.
Everything else was read from breno's store, read-only, and from two
proxies' greetings (connect and read, nothing sent).
**This session has sent nothing to any Cube.**

## TL;DR

- **The restart freed .22's slots: 1 → 50 free**, and they have stayed free:
  50 at each of its 11 greetings since, over 13 hours (to 11:37 today),
  but one, 49 two minutes after a device removal.
  Commands took slots for minutes and gave them back
  (a setpoint 49 → 50 in 2 min, a removal 46 → 50 within half an hour).
- **So the 49 slots held stale radiograms**, from February or earlier,
  for devices that never answered; the restart dropped them.
  Which devices, the Cube does not tell: the contact woken with a battery
  freed none (still 1 free after its three window cycles).
- **The restart cleared every flag, and nothing was flagged again:**
  nine flagged thermostats cleared at the greeting;
  every thermostat reported within 5 minutes, the radiator with the new
  battery too (22:37:24), so the Cube's time push after its start
  likely found them all; the six contacts without batteries stay silent and
  unflagged. No flag on .22 in the 13 hours since.
- **.25's room-6 wall thermostat**, reset at 18:09 on 11-14, is still clear
  after 17 hours.
- **New, overnight on .25:** two removals of a room-1 radiator thermostat
  through the API were discarded by the Cube with 48 slots free,
  and within a minute of each, room 1's members were flagged:
  the wall thermostat and four radiator thermostats are flagged since,
  the radiators with byte 4 = **0x21 RemoveLinkPartner**,
  the removal's own link radiograms.
- **Corrections from the log-mining session** (2026-11-15, the original
  software's logs and jars): byte 4 kept 0x03 after each of February's four
  resets, so it is read only with the flag (the 19:15 analysis's 0x09 for
  the 18:09 entry needs a second look); the original software never sends
  0x81, so the v2 analysis's "daylight-saving push" of 02-12 was the Cube's
  own or another tool's.

## 1. .22 before the restart

- 1 of 50 slots free at every greeting from 2026-11-11 (store) and on
  2026-02-19 (the original software's log), duty 1 %.
- Ten devices flagged, nine of them reporting.
- **The contact test (D115-B, first half):** one contact, with a new
  battery, reported at 21:04 and then each of three open-and-close cycles
  (21:15–21:21), each change at serve's next one-minute poll, no flags.
  The greetings after `massimo`'s restart at 21:24:57 still gave 1 free.
  So nothing was waiting for that contact, or what it fetched was refilled at once
  (unlikely: no client sends to .22).

## 2. The restart (D115-B, second half)

| Time | What the store shows |
|---|---|
| 22:32:11 | the owner's Reboot (`POST …/reboot`, `R` over UDP), acknowledged |
| 22:32:24 | .22's greeting: duty 0 %, **50 free**, 17 devices; nine thermostats' flags gone, eight entries without data |
| 22:33:24 | wall r3 reports (its own link_error set; that is the device's report) |
| 22:34:24 | rad r1, wall r1, wall r5, rad r5 report |
| 22:35:24 | rad r4, wall r4 report |
| 22:37:24 | rad r3, the radiator with the new battery, reports: first data since .22's previous restart |
| 22:47:24 | the contact with the battery reports again |
| 22:56 | the watcher ends: no flag in 24 minutes |

The two room-2 thermostats kept their data across the restart
(flags cleared, entries valid); every other entry was empty until its device reported.

**Since then (13 hours):**

| Time | Greeting or command | Free slots |
|---|---|---|
| 23:04 | a setpoint, acknowledged | 49 |
| 23:06, 23:18 | greetings | 50, 50 |
| 00:32 | a device removal, three steps, acknowledged | 46 |
| 00:34, 00:59 | greetings | 49, 50 |
| 03:18 | pairing started | 50 |
| 03:35 to 11:37 | seven greetings | 50 each |

No device of .22 has been flagged since the restart.
Some entries lost their data for a few minutes overnight (wall r3, wall r4,
the contact), each back at the device's next report; no flag came with it.

**Compared with .25's restart of 2026-02-19**, which flagged four devices
within 6 minutes (0xF1 wakeup, then 0x03 time) because it had not heard them:
.22 heard every thermostat within 5 minutes, so its time push found them.
That the six silent contacts were not flagged fits a Cube that sends no
time to a window contact (a contact has no clock); likely, not shown.

## 3. The other Cubes, passively

- **.25, room 6:** the wall thermostat reset at 18:09 on 11-14 is still
  clear. Its room-6 radiator thermostat is flagged, as before.
- **.25, overnight:** two removals of a room-1 radiator thermostat
  (`DELETE …/devices/…`, 04:28:26 and 04:41:23),
  each answered "discarded" with 48 slots free;
  the device was not flagged when they were sent.
  Room 1's wall thermostat and three radiator thermostats were flagged
  at the next poll after the first (04:29:16), a fourth radiator thermostat
  after the second (04:42:08).
  The proxy's greeting, read at 11:55 with nothing sent, gives the four
  radiators byte 4 = 0x21 RemoveLinkPartner: Massimo's removal sends 0x21
  to each partner in the room, and those radiograms failed
  (the wall thermostat's own entry was not read).
  Why the Cube answered "discarded" with slots free is open.
  The original software would count these as AssignFailed and reset them
  only before a link change, not before a room command.
  A room-1 setpoint between the removals (04:37:06, 20.5 °C) was accepted,
  and the flagged members' entries showed 20.5 a minute later; whether they
  took it, the rows do not tell (the docs say the Cube writes a room setpoint
  into the entries at once; the log-mining session reads the logs as
  showing it only once the device has acknowledged).
  The five stay flagged until an `r:` or a restart of .25;
  nothing was sent.
- **.21:** one radiator thermostat (room 2) flagged, as before.
- **.23, .24:** no flags; .24 took radio commands from a client behind
  its proxy at 01:39–01:40 (48 → 47 → 48 free; 49 at the greeting two
  minutes later, 50 at the next).

## 4. The answers, updated

- **Q2, what clears the flag:** `r:` or a restart, nothing else.
  A restart clears it for good where the Cube hears its devices at the
  start; it flags at once whatever it does not hear (February).
- **Q4, .22's slots:** stale radiograms, dropped by the restart; not the
  flags (the flags cleared at the same time but are a separate matter:
  `r:` clears flags and frees no slot).
  A Cube can sit with one slot free for months without any other sign
  than the greeting's number, and then accepts one more radiogram:
  if that one waits for a silent device, it refuses every command after it.
- **Q3, what sets it:** also a client's link radiograms (0x21, .25's room 1,
  above). The v2 analysis's 0x81 of 02-12 was not the original software's,
  which never sends 0x81 (the log-mining session).
- **Byte 4:** the failed radiogram's type, read only with the flag:
  after `r:` it kept 0x03 in February's four resets (the log-mining session),
  and a restart puts 0x09 back (every entry of .22 now).
- Q1 and Q5 stand as in the v2 analysis.

## 5. For the held decisions

D102, D110, D111, D113 and D116 are still held until the tests are done;
D112-A is settled. What this adds:

- **D102 (a flagged device that reports):** nothing new against B;
  a restart is not a substitute, since it flags whatever is silent at that moment.
- **D110 (room commands to a flagged room):** .25's room 1 is a live case
  now, with five flagged members: a room command to it gets no `r:` from
  Massimo today. By byte 4 they failed a link radiogram, which the original
  software would not reset before a room command (D111's question too).
  The log-mining session's evidence that the setpoint shows only once a
  device has acknowledged it bears on D110 and on its D135-B.
- **D113 (the docs change):** moot as worded. #256 brought the 18:09 finding
  to `main`, and the rest goes in one pull request from this session
  (`EXEC docs`).
- **D116 (a slot watch):** .22's case is the argument for it: 49 of 50 slots
  held for at least nine months, seen only in the greeting.

## 6. What remains

- **Passively:** whether .22's slots fill again over the next days
  (serve's greetings, every 6 hours and at each reconnect);
  whether .25's reset wall thermostat stays clear.
- **.25's room 1:** five members flagged; a reset is one `r:` line,
  sent only on the owner's word.
- **The discarded removals on .25:** why, open.

## How it was read

`GUIDELINES/temp/d102/watch_cortile.py` and `watch_reboot.py` (the watchers
of 11-14), `after_reboot.py` (each Cube's greetings, commands and flag
changes since 22:32), `cielo_night.py` and `cmd98.py` (.25's commands of
the night), `greet_labels.py` (a proxy's greeting by device type and room,
with byte 4); each piped to breno over SSH, `sudo -n python3 -I`,
`file:/var/lib/massimo/massimo.db?mode=ro`.

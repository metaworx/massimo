# Write test on hardware, 2026-11-05

Steps 1 to 6 of [testing-writes.md](../testing-writes.md) were run
against the live house Cube (firmware 1.1.3).
They used room 2 and its only radiator thermostat 05cdc6 (firmware 22),
not room 1, because room 1's thermostat has its keys locked,
so its eco and boost buttons cannot be pressed to check a value.
Step 7 (moving a device) followed later the same day,
see [Step 7](#step-7-moving-the-window-contact).

**Result: every step passed.**
Every write was accepted by the Cube (none discarded) and came back from the
Cube exactly as sent.
After every revert the configuration was identical to the backup taken before.
The final backup differs from the first only in `taken_at`, the H and L lines,
and two measured temperatures, and every room is ready.

## Setup

- `massimo serve` v0.2.0-alpha.1 as a systemd service on a Raspberry Pi 1 (see
  [packaging/README.md](../../packaging/README.md)), API on port 80,
  the Cube's proxy on a separate address of the Pi;
  nothing else connected to the Cube.
- Writes with curl against the Pi's API, Bearer token from the env file.
  Backups with `massimo -show-ghosts backup -sn <serial>` through the proxy.

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

Duty cycle: 4 % before, 14 % after the eight radio writes (about 1.25 % each,
lower than the 2 to 3 % the plan expected).
Free memory slots: 50 before and after.

## Step 7: moving the window contact

Room 2's window contact 0a010f was moved to room 1 and back with
`PUT …/devices/0a010f {"room_id": …}`,
the window opened and closed after each move.
Duty cycle 4 % before.

- **7a, room 2 → room 1: passed.**
  Answer ok.
  The Cube listed the contact under room 1 at once,
  the configuration diff showed only its `room_id: 2 -> 1`,
  and 4 radiograms were pending (duty cycle 12 %, then 15 %).
  After the window was opened and closed, the contact had fetched them all:
  0 pending, 50 free slots, room 1 ready.
- **7b, room 1 → room 2: passed, with a leftover link error.**
  Answer ok, the contact back under room 2,
  configuration diff against the first backup empty,
  4 radiograms pending (duty cycle 18 %).
  Before the window could be opened,
  a network outage power-cycled the Cube and the Pi (the Cube twice;
  it needed a second power cycle to answer on TCP again).
  Afterwards the Cube had no radiograms pending,
  so whether the contact fetched all four is unknown.
  Functional check: with the window open,
  room 2's thermostat went to its window-open temperature (5 °C) within a minute
  and back to 18 °C a minute after closing,
  and room 1's thermostat was unaffected,
  so the contact is linked to room 2's thermostat again.
  But the contact's reports now alternate between `link_error` set and clear
  (set at 18:03 and 18:06, clear at 18:05), which it did not do before the test;
  most likely a link to a room 1 device was not removed because a radiogram was
  lost in the power cycles.
  Fix if it stays: move the contact to room 1 and back once more.
- **7c, the move repeated (18:14 to 18:37): passed,
  and it cleared the link error.**
  `serve` v0.3.0-alpha.1 by then.
  To room 1: 4 radiograms pending, duty cycle 13 %.
  The first window cycle delivered 2 of them,
  cleared `link_error` and set the contact's `error` flag,
  because a radiogram had gone unanswered.
  With the window open, room 1's thermostat went to 5 °C and room 2's did not react.
  A second window cycle delivered the other 2 (0 pending).
  Back to room 2: 5 radiograms (the move's 4 and the queue's error reset),
  duty cycle 20 %.
  The first window cycle delivered only the error reset and cleared `error`;
  the second (window open for 2 minutes) delivered the remaining 4.
  Room 2's thermostat went to 5 °C a minute after opening and back to 18 °C after closing,
  and room 1's stayed at 19 °C.
  End state: 0 pending, 50 free slots, no `error` or `link_error` on any device,
  every room ready.
  The configuration diff against the first backup is empty apart from the contact's name,
  which was changed in the web UI during the test.

A contact fetches its radiograms only right after it transmits, and not
always all of them at once, so a move can take several window cycles.
Checking the pending count fresh from the Cube (here by restarting `serve`)
after each cycle shows when it is done.
A Cube restart drops radiograms it still waits to deliver.
A move whose radiograms are lost leaves the Cube's metadata right but the
device's links half done,
and nothing reports that except the device's link-error flag.

## Findings

1. **The server's view of a write is stale.**
   After a write the server re-reads the device's configuration at once,
   but the Cube only updates its C line when the thermostat has acknowledged,
   which took up to a few minutes here.
   The server does not read it again, so the API,
   the UI and backups through the proxy kept showing the old value (eco 16.5) for minutes,
   although the thermostat already used the new one.
   The same goes for the budget and `pending_radiograms`:
   they only change with the next command reply or reconnect,
   so `status` through the proxy kept showing 48 free slots.
   For the test, `serve` was restarted before each check,
   which makes the Cube send all its data afresh.
   Follow-up: re-read the configuration (and the H line) once the pending count
   of the Cube drops, or poll it for a few minutes after a write.
   Addressed on 2026-11-05: the server re-reads the device's C line after 15 s,
   30 s, 1, 2 and 2 minutes until it changes,
   and reconnects for the budget afterwards if radiograms were pending (docs/api-v1.md,
   under GET /cubes).
2. **The log cannot show the sequence the plan asks for.**
   The command queue only logs discarded commands and budget waits, so the `z:`,
   `s:` and `c:` lines and the Cube's answer of an accepted command do
   not appear in the serve log.
   Follow-up: log accepted commands too, with target, command and budget.
   Addressed on 2026-11-05:
   "room woken up" and "command accepted by the cube" lines.
3. **One radiogram stayed pending for about 35 minutes.**
   After step 1a the Cube held two radiograms,
   and one of them stayed pending until about 16:17,
   while all later writes were delivered.
   It cleared by itself, and nothing was lost.
4. Unrelated to the test: a window in room 4 was opened and closed around 15:41;
   the thermostat of that room showed its window-open temperature in one backup.

## Open items from the install (until confirmed)

- DHCP reservations in the mobile router's network: the Cube at .118 and the Pi.
  Without one the Pi's DHCP address changed from .48 to .41 after the outage;
  it stayed reachable on its fixed .200.
- During the update helper's restart of `serve` the proxy was gone for a few seconds,
  and `massimo status` on another machine fell back to the Cube itself,
  taking its only client slot until it gave up.
  Commands run from other machines should name the proxy with `-addr` instead of
  relying on the fallback.
- The Pi's fixed addresses .200 (web UI and API) and .201 (Cube proxy) are
  outside the router's DHCP range of 20 to 199 (confirmed by the owner).
  The earlier .49 and .50 are no longer used.

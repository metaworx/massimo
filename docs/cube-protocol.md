# MAX! Cube protocol

What is known about the MAX! Cube: its LAN protocol (TCP 62910) and UDP protocol (23272),
the radio commands it sends, and how it and the original software talked to eQ-3's portal
("MAX! internet control").
Reconstructed from the ERNW/Insinuator analysis (2016),
from the behaviour of the original local software (version 1.4.6),
and from experiments with real Cubes (firmware 1.1.3) in 2026.
Facts marked *unverified* still need confirmation on hardware.
Serials and radio addresses in the examples are placeholders since 2026-11-08,
`KEQ0000…` and `0a…`, in the lines quoted as captured too,
which are otherwise as the hardware sent them.

This is the single record of what is known about the Cube's behaviour,
on the LAN as well as towards the portal;
until 2026-11-13 it was `docs/portal-protocol.md`.
Massimo's own portal tools, `massimo portal` and `portal-crypto`,
are gone since then, kept on the branch `archive/portal`
with `docs/firmware-extraction.md`, the attempt to read the Cube's firmware:
without the firmware, the key from the QR code serves nothing.

## Contents

- [Verified on hardware](#verified-on-hardware), the summary
- [Transport](#transport)
- [Two clients of the portal](#two-clients-of-the-portal)
- [Local commands involved (TCP 62910)](#local-commands-involved-tcp-62910)
- [Portal requests made by the PC software](#portal-requests-made-by-the-pc-software)
- [The complete setup
  sequence](#the-complete-setup-sequence-captured-from-the-original-software-2026-11-04)
- [Lookup service](#lookup-service)
- [What is public](#what-is-public-research-2026-11-04)
- [Recovering the IV from a Cube](#recovering-the-iv-from-a-cube)
- [Observed polling](#observed-polling-cube-firmware-113-november-2026)
- [One LAN client at a time](#one-lan-client-at-a-time)
- [LAN commands verified against the original
  software](#lan-commands-verified-against-the-original-software-146)
  - [Verified on hardware with
    Massimo](#verified-on-hardware-with-massimo-2026-11-04-cube-jeq-live-devices)
  - [Pairing a device, as the original software does
    it](#pairing-a-device-as-the-original-software-does-it), followed by moves,
    wall thermostats, the key lock, set-temperature addressing,
    room commands and never-reporting devices
- [What the PC software (1.4.6) knows about the
  portal](#what-the-pc-software-146-knows-about-the-portal)
- [Cube UDP protocol (port
  23272)](#cube-udp-protocol-port-23272-discovery-network-setup-firmware)
- [Open questions](#open-questions)

## Verified on hardware

On two Cubes with firmware 1.1.3 and their thermostats,
wall thermostats and window contacts, November 2026;
the sections below give the details and dates.
Everything else comes from captures of the original software (1.4.6),
its decompiled code or public analyses, and says so.

- **Encryption**:
  AES-128-CBC with a per-Cube key (the QR code) and a per-Cube IV,
  which `e:` with zero bytes reveals over the LAN.
- **Portal polling**:
  after `u:` and `i:00000000,00000000` the Cube POSTs its full state (H, M, C…,
  L) every few minutes; command lines in the reply have no effect.
- **One LAN client at a time** on TCP 62910; a second one waits.
- **Written by Massimo and seen on the devices**:
  0x11 preset temperatures, 0x10 one day of the week programme,
  0x12 valve settings (each after a `z:` room wakeup), `m:` renames,
  `i:ffffffff,00000000` (portal polling off), 0x80 key lock in both directions;
  their cost in duty cycle and memory slots.
- **Set temperature (0x40)**:
  a device in a room is accepted only when addressed as its room.
  The setpoint shows in the L line at once,
  the mode only with the devices' next report.
- **Room commands are discarded while any member has no state** (no valid L entry):
  after a Cube restart until every member has transmitted,
  and for good with a paired device that never reports.
  The room that counts is the room byte of the device's C line,
  not the metadata.
  Such a device cannot be deleted (`t:` is acknowledged, the device stays),
  so it is parked in room 255 and hidden.
- **Budget**:
  the Cube still accepts commands at 100 % duty cycle,
  and every unacknowledged radiogram holds a memory slot;
  the Cube also spends radio time on its own retries.
- **Key lock state** is bit 0x20 of the second flags byte of an L entry.
- **Network configuration over UDP 23272**:
  `c` reads the stored static configuration and the DHCP flag,
  `N` the addresses in use, `C` writes (status 1 applied at once, 2 restarting),
  from a capture of the original software; see the Cube UDP protocol section.

Not yet verified with Massimo on hardware:
the configuration endpoints of the API and the UI panels on them (test plan in
docs/testing-writes.md), and pairing.

## Transport

- `POST <portal url>` (default `http://max.eq-3.de:80/cube`), headers
  `Opt: "http://www.eq-3.com/MAX"; ns=MAX` and `MAX-Serial: <serial>`.
- Body: AES-128-CBC ciphertext of CR LF separated protocol lines, zero padded,
  **not** base64 encoded on the wire.
- Key and IV are fixed per device.
  The key is printed on the circuit board as a QR code:
  `K` followed by 32 hex digits.
  It is not derivable from the serial, MAC,
  RF address or chip ID (74 hash-based candidates tested, none matched).
  The IV never crosses the wire;
  it is recovered with the local `e:` command (see below).
- The portal replies with the same encoding.
  Only the body is encrypted; there is no TLS,
  no authentication tag and no replay protection.

## Two clients of the portal

1. **The Cube itself** polls the portal URL once polling is enabled (see `i:` below).
   What it sends while polling and how commands come back has not been captured yet.
2. **The local PC software** also talks to the portal directly,
   using the Cube as an encryption oracle: it builds the lines,
   sends them to the Cube with `e:`, and POSTs the ciphertext.
   This is how user accounts were created and remote access toggled.

## Local commands involved (TCP 62910)

| Command | Reply | Meaning |
|---|---|---|
| `u:<url>` | `A:` | Store the portal URL. Verified. |
| `i:<interval>,<0>` | `A:` | Portal polling. Both values are 32-bit, 8 hex digits. The original software sends `i:00000000,00000000` after a successful portal login and `i:ffffffff,00000000` (-1, "infinite") to disable. Verified syntax; effect on hardware: *unverified*. |
| `e:<base64>` | `E:<base64>` | Encrypt with the device key. Verified. |
| `d:<base64>` | `D:<base64>` | Decrypt; plaintext comes back zero padded. Verified. |

The Cube's own configuration (`C:` message for the Cube) holds the URL at
offset 0x55 and a "portal enabled" byte at 0x12.

## Portal requests made by the PC software

Every request starts with `H:<serial>,<rf address>,<firmware>` (short form)
and then one of:

| Line | Reply | Meaning |
|---|---|---|
| `B:<user>,<md5(password + serial)>,<enabled 0/1>,<password>` | `b:OK` or an error text | Create or update the portal account; the third field enables remote access. Verified with the original software against this implementation. |
| `G:` | `g:<user>,<true/false>` | Read account and remote-access state (*unverified* field order). |
| `J:1` / `J:0` | `j:OK` | Enable or disable remote access on the portal. |
| `P:…` / `O:…` | `p:` / `o:` | Set/get user data. Not analysed. |
| `V:` / `W:<code>` | `v:` / `w:` | Product activation. Not analysed. |

After `B:` or `J:` succeeds, the PC sends `i:` to the Cube with 0 (enabled)
or -1 (disabled).

The original portal was observed replying `i:000051b1,00000000,ffffffff` in
front of `b:OK`.
The PC software ignores that line; it may be meant for the Cube's own polling.
Meaning of the fields: unknown.

## The complete setup sequence (captured from the original software, 2026-11-04)

With a hosts entry for `www.max-portal.eq-3.de` pointing at this server:

1. `GET /lookup` (headers `Opt`, `MAX-Serial`) → `200`, body = device URL.
2. LAN: `u:<device URL>` → `A:`.
3. LAN `e:` of `H:<serial>,<rf>,<fw>` + `G:` → `POST <device URL>` → reply
   decrypted with LAN `d:` → `g:<user>,<true|false>`.
4. Same with `B:<user>,<md5>,1,<password>` → `b:OK`.
5. LAN: `i:00000000,00000000` → `A:`.
   The Cube polls the device URL from then on.

Note that the Cube resolves the device URL through the network's DNS, not
the PC's hosts file, so the URL returned by `/lookup` must be reachable
by the Cube (an IP address, or a name the router resolves).

## Lookup service

Before contacting the portal,
the PC software did a `GET http://www.max-portal.eq-3.de/lookup` with the same
two headers;
the response (body or `Content-Location` header) was the per-device portal URL,
which was stored in the Cube with `u:` if it differed.
The service is gone; the URL is now set directly.

The Cube's factory default was a lookup URL as well:
the C message of a Cube documented in 2014 (Bouni's protocol notes) still
carries the stale tail `0/lookup` after `http://max.eq-3.de:80/cube\0`,
which fits exactly `http://max-portal.eq-3.de:80/lookup`.
So a Cube may itself follow the lookup procedure (GET,
then POST to the returned URL).
Untested with Massimo's fake portal (on the branch `archive/portal`):
set `u:http://<host>:80/lookup` and watch for a GET.

## What is public (research, 2026-11-04)

The only published decrypted portal traffic is in the 2016 Insinuator article:
the PC software's `B:` request and the real portal's reply
`i:000051b1,00000000,ffffffff\r\nb:OK\r\n` plus zero padding.
That `i:` line therefore answered a PC request, not a Cube poll;
the reply to a poll was never published.
No captures, dumps or replacement portals exist anywhere else (FHEM, openHAB,
ioBroker, Home Assistant, GitHub, Wayback Machine checked;
www.max-portal.eq-3.de was never archived).
Other portal hosts: max-portal.elv.de (ELV),
smarthome.md.de:8080 (mobilcom-debitel).

Hardware: the Cube is an Atmel AT91SAM7X256/512 (ARM7TDMI) with a TI CC1101 on a
TRX868 module; JTAG on header ST3, UART0 on ST1, debug on ST2,
erase jumper J1 (a-culfw CUBe port, ELVjournal 1/2012 schematic).
The a-culfw reflash erases the chip,
and nobody has published a dump of the stock firmware.
A JTAG read-out of an unmodified Cube would be the only way to get it,
and only if the chip's security bit is not set.

## Recovering the IV from a Cube

Send 32 zero bytes to `e:` (`e:AAAA…AAA=`).
In CBC with a zero first block,
the first ciphertext block is the IV encrypted with the key,
so decrypting that block with the key yields the IV.
`massimo portal-crypto iv` did this and also verified the chaining with the second block;
it is on the branch `archive/portal`.

Verified on a real Cube: the same plaintext always gives the same ciphertext,
the first block of a 16-byte and a 32-byte message is identical,
and the second block differs from the first for identical plaintext blocks,
which rules out ECB and a per-message IV.

## Observed polling (Cube firmware 1.1.3, November 2026)

After `u:` and `i:00000000,00000000` the Cube POSTs to the URL.
Each poll carries the full state: `H:` (long form, new connection id each time),
`M:`, every `C:` and `L:`.
Request headers:

    POST /cube HTTP/1.1
    Host: <host>
    connection: close
    Content-Length: <n>
    Opt: "http://www.eq-3.com/MAX", ns=MAX
    MAX-Serial: <serial>

Gaps between polls grew from about 4 to 10–12 minutes within the first hour
after `i:00000000,00000000`, regardless of the reply (`i:` with various values,
`{conn}` echoed, or no reply line at all).
`i:00000078,00000000` (120) did not shorten the gaps,
so the first field is not an interval in seconds;
whether other units apply is untested.
Both `u:` and `i:` make the Cube poll within about 20 s, which has to be
kept apart from interval effects when measuring.

Capture of 2026-11-04 02:04–02:21 (PC side, Wireshark):
`i:00000078,00000000` over the LAN was followed by a poll 6 s later;
the next attempt came 7 min 18 s after that,
as eight SYNs over 15 s that nothing answered (the portal was not listening on
that port at the time), then a RST.
Unanswered attempts therefore look like missing polls in the portal log;
the gaps of 29,
58 and 91 minutes seen later that morning are close to multiples of one period
and probably include such failed attempts.
A Wireshark capture on the portal host filtered on the Cube's address shows them.

In the same capture the duty cycle went from 0 to 68 % and all six
thermostats got the error flag (L entry byte 5 = 0x0c, byte 4 = 0x03)
between 02:05 and 02:10 without any LAN command: the Cube spends radio
time on its own retries to unreachable devices (they were not mounted).
A client must expect budget to disappear without having sent anything.

**Commands in the reply are ignored.**
Lines `f:…` (NTP servers),
`v:…` (time zone) and `m:…` (metadata) sent in the poll reply had no effect,
while the same lines over the LAN work.
How the original portal delivered commands to the Cube is still unknown;
the reply channel is not it, at least not in this form.

## One LAN client at a time

The Cube serves a single TCP connection on port 62910.
A second client, from the same or another computer,
waits until the first one closes its connection or times out (observed with
`massimo serve` holding the connection: `backup`,
`status` and the original software all hung until `serve` was stopped).
Every client therefore either shares one connection or takes turns,
which is why Massimo's server should also act as a Cube for other clients.

## LAN commands verified against the original software (1.4.6)

Captured on 2026-11-04 while operating the original UI:

| UI action | Line sent | Reply |
|---|---|---|
| Set time zone | `v:<base64 of winter rule + summer rule>` (24 bytes, **no time field**) | `A:` |
| Any command to a device in error state | `r:<count>,<base64 addresses>` first | `S:` |
| Room 1 manual 15 °C | `s:AARAAAAACgEKAV4=` | `S:` |
| Room 3 auto | `s:AARAAAAACgEIAwA=` | `S:` |
| All rooms 6.5 °C until 04.11.2026 02:00 | `s:AARAAAAACgEKAY2kmgQ=` … | `S:` |

A second capture (week program, presets, valve settings, renames):

| UI action | Lines sent | Reply |
|---|---|---|
| Edit week program of room 1 | `z:1e,G,01` (wake the room for 30 s), then one `s:` (0x10) per changed day, e.g. `s:AAQQAAAACgEKAQRO3VTzRQoxIDEgMSAxIA==` (Wednesday, 4 points padded with the last) | `A:`, then `S:` per day |
| Temperature offset -2 °C | `r:` if in error state, then `s:AAARAAAACgEKACwePQkDCgM=` (0x11; flags 00, room 00) | `S:` |
| Boost 5 min / 80 % | `s:AAQSAAAACgEKATAM/wA=` (0x12) | `S:` |
| Rename room or device, reorder rooms | `m:00,<base64 metadata>` (index only, no count) | `A:` |

All of these match the builders in `internal/protocol/cube` byte for byte.
A 0x11 command to a device in error state that was **not** preceded by `r:` came
back as discarded (`S:14,1,31`); the software then reports "Command discarded".
While the UI is open the software polls `l:` every 10 s and disconnects after 30
s without user activity.
It reads the next line after a command as its reply (seen 2026-11-14 through
Massimo's proxy, software 1.4.6): a line it did not ask for comes back as
"S: expected", "ACK expected" or "device list expected", each reply one line late,
while the commands themselves still reach the Cube.

### Verified on hardware with Massimo (2026-11-04, Cube JEQ…, live devices)

Through `massimo restore` on a radiator thermostat:
0x11 preset temperatures (eco 17 → 16.5 shown on the device),
0x10 week program (one day), 0x12 valve settings (boost 5 → 10 min,
shown as 600 s on the device), each after a `z:` room wakeup;
`m:` (rename of a wall thermostat, confirmed by the next M line);
`i:ffffffff,00000000` (portal polling off, confirmed by the C line).
Budget per write: two memory slots while in flight,
duty cycle +3 % for 0x11/0x12, +8 % for one day of 0x10.

### Pairing a device, as the original software does it

Log of pairing a wall thermostat (NEQ…, rf 0a010e, fw 16) into room 4,
which held a radiator thermostat (0a0103) and two shutter contacts (005447,
1570d3):

1. `n:` inclusion mode; the Cube answers `N:<type><rf><serial>ff` when a device joins.
   Devices paired earlier but never put into a room are listed as new devices
   too (they have a C line but no M entry).
2. `z:1e,G,04` wakeup, then one 0x20 (add link partner) radiogram per pair in
   both directions:
   to the radiator thermostat with the room flag (`00 04 20 … 0a0103 04 0a010e 03`),
   to each shutter contact (`00 00 20 … 005447 00 0a010e 03`),
   and to the new device for each partner (`00 00 20 … 0a010e 00 0a0103 01`,
   `… 005447 04`, `… 1570d3 04`);
   then 0x22 set group (`00 00 22 … 0a010e 00 04`).
3. `c:0a010e` to fetch the new device's configuration.
4. 0x40 set temperature, auto, temp 0 (`… 40 … 0a010e 00 00`).
5. `z:` wakeup, then 0x11 preset temperatures to **every** thermostat of the room,
   radiator and wall alike,
   with the room's values (`27 22 3d 09 07 0a 03` = 19.5/17/30.5/4.5, offset 0,
   window 5.0 °C, 15 min).
6. `z:` wakeup, 0x12 to the new device (`30 0c ff 00` = boost 5 min 80 %,
   decalc Sat 12:00, max valve 100 %, offset 0).
7. `z:` wakeup, 0x10 for all seven days to the new device (Mon..Fri,
   Sat, Sun; 7 switch points, padded with the last one), then 0x11 once
   more to the new device.
8. `m:` with the device added to the room and device list.

Massimo pairs the same way (`POST /api/v1/cubes/{sn}/pairing`,
or `n:` through the proxy) and relies on the following,
which comes from other implementations and is *unverified* on hardware:
`n:` and `x:` get no immediate reply; the Cube sends one `N` line (type,
address, ten characters of serial,
0xff) per device as it joins and an empty `N` line when the mode ends,
by its time or by `x:`; the metadata does not change by pairing;
commands such as `c:` work while the mode lasts.
Massimo also ends the mode itself ten seconds after the requested time.

Massimo gives a thermostat it puts into a room, after pairing or in a move
(`PUT /api/v1/cubes/{sn}/devices/{rf}`, 2026-11-06),
the room's settings as in steps 4 to 7:
0x40 auto with temperature 0 directly to the device (flags 0, room 0,
as in the log above),
0x11 to every thermostat of the room with the values of its first other thermostat,
each keeping its own offset, 0x12 to the device,
and 0x10 only for the days that differ from the device's own,
then 0x11 to the device; `"copy": false` skips it.
The payloads and the order of steps 4 to 7 are the original software's,
but Massimo writes the metadata (step 8) before them, with the links.
That a device which was in a room before takes the direct 0x40 is *unverified*,
since a direct 0x40 to a room member was discarded on 2026-11-04 (below):
Massimo sends it without the error reset and the retry,
and its discard ends nothing but says so in the answer.

Moving a device to another room (same software,
wall thermostat from room 4 to room 5): wakeup of the old room,
0x23 remove group (`… 23 … 0a010e 00 04`),
0x21 remove link partner for every device of the old room in both directions
(same addressing as 0x20), `m:` with the device taken out of the room,
then wakeup of the new room,
0x20 add link partner for every device of the new room in both directions,
0x22 set group, `c:` fetch, 0x40 auto, 0x11 to the room's thermostats, 0x12,
0x10 × 7, 0x11, `m:` with the device in the new room.
Links are therefore the only per-device data that is not readable from the Cube;
a restore derives them from the room layout.

Boost over the air (2026-11-13, Cube firmware 1.1.3, through Massimo's proxy):
one room-addressed 0x40 with mode 3 in its top bits
(`00 04 40 00 00 00 <rf of a room member> <room> c0|<setpoint>`),
the setpoint the room had (5.5 °C), to a room of a radiator thermostat and
a wall thermostat, both in manual.
The Cube answered `S:12,0,31`; within 40 seconds both L entries showed mode 3
and the radiator thermostat's valve 100 %,
its C line's boost setting (30 minutes at 100 % on these devices;
5 minutes at 80 % in the earlier captures, the devices' defaults differ).
The devices showed the boost's remaining minutes on their displays.
After the boost time the radiator thermostat's entry returned to manual at 5.5 °C,
valve 0 %, 31 minutes after the command;
the wall thermostat's followed two minutes later, with its next report.
Nothing else was sent: a boost ends by itself in the mode the devices had.
One radio write, the duty cycle 18 % before and 21 % after.

One wall thermostat per room (2026-11-12, the original software 1.4.6, Cube firmware
not noted): moving a wall thermostat into a room that has one already is refused
by the software before it sends anything, with "Attention: You cannot add any more
devices of this type. The maximum number of devices of this type has been reached
for this room."
The message speaks of "devices of this type", so the software probably limits the
other types per room as well;
eQ-3's documents name up to 8 radiator thermostats and 8 window contacts per wall
thermostat. Those limits are not confirmed:
only the wall thermostat's was seen.
The Cube's metadata itself could hold two
(one written by another tool), and nothing says how the radiator thermostats
would choose between two temperatures and two setpoints.
Massimo refuses a second wall thermostat in a room the same way
(`PUT …/devices/{rf}`, `409`) and copes with a Cube that has two
(docs/api-v1.md, "The virtual wall thermostat").

So wall thermostats take 0x11 and 0x12 with the same payloads as radiator thermostats,
and their C line ends with offset,
window-open temperature and boost (3 bytes after the week program at 0x16).
Their L entry (12 bytes) carries the setpoint at offset 8 and the measured
temperature at 12 (with bit 7 of byte 8 as the ninth bit);
byte 7 is not a valve but the display mode: 0x01 shows the setpoint,
0x04 the measured temperature,
set with radiogram 0x82 and one byte (4 or 0) after a room wakeup
(`00 00 82 … 0a010e 00 04`).

The original software's `FrameType` enum (MaxEssentialsBackend 1.4.6) lists
every radiogram the system knows, with the codes:
0x00 SystemInformation (pair ping), 0x01 Inclusion (pair pong), 0x02 Answer,
0x03 TimeInformation, 0x10 ConfigWeekTemperatureProfile,
0x11 ConfigTemperatures, 0x12 ConfigValveFunctions, 0x20 AddLinkPartner,
0x21 RemoveLinkPartner, 0x22 SetGroupAddress, 0x23 RemoveGroupAddress,
0x30 ShutterContactState, 0x40 ControlMode (set temperature),
0x42 SetPointAndCurrentTemperature (wall thermostat), 0x43 ComfortTemperature,
0x44 EcoTemperature, 0x50 PushButtonState, 0x60 HeatingThermostatState,
0x70 CurrentTemperatureAndHumidity, **0x80 LockManualControls**,
0x81 DaylightSavingTimeMode, 0x82 SwitchDisplaySetPointActual, 0xF0 WakeUp,
0xF1 Reset.
Only `FrameType` itself references `LockManualControls`:
the software has no UI for the key lock and never sends 0x80.
The lock state is readable,
as bit 0x20 of the second flags byte of an L entry (confirmed on hardware:
unlocking a thermostat at the device turned its flags from 0x1238 to 0x1218).
Massimo sends 0x80 with one payload byte (1 lock, 0 unlock) like 0x82,
directly to the device after a room wakeup (`z:1e,G,<room>` then
`00 00 80 000000 <rf> 00 00`).
Verified on hardware 2026-11-04 in both directions:
two locked radiator thermostats (fw 22) were unlocked over the air and one locked again,
the Cube answered `S:` not discarded,
the lock bit followed with the device's next report and the device showed the lock,
and the duty cycle rose by 2 % per command.
On the device the lock is toggled with Auto/Manu plus the second button,
see the eQ-3 manuals.

Set temperature (0x40) addressing, verified 2026-11-04:
a device that is in a room is only accepted as its room (flags 0x04, room id);
both flags 0 with the room id and flags 0 with room 0 were discarded by the Cube
for such a device.
A device without a room (just paired) takes flags 0 and room 0.

Room commands and never-reported members, 2026-11-04: on a Cube with four rooms,
0x40 to the two rooms whose members had all reported was accepted,
0x40 to the two rooms that each held a paired contact that had never reported
(no valid L entry) was discarded, for any recipient, with or without a wakeup,
and the original software got the same `CommandDiscarded` for one of them.
The Cube apparently refuses a group command while a member cannot acknowledge it.

The contacts in question turned out to be fine.
The rule, confirmed after a power cycle of the Cube:
**the Cube discards a room command while any member of the room has no state in
the L line** (valid bit 0x10 of the first flags byte clear).
A restart clears every entry;
a device gets its entry back when it next transmits.
Radiator and wall thermostats do so on their own within minutes to hours,
a window contact only on a window change, the eco switch only on a button press.
Right after the restart only the rooms whose members had all reported accepted commands;
opening and closing the two windows of a refused room and turning its
thermostat's wheel made that room accept within a minute.
The original software is subject to the same rule.
A driver must therefore expect discards for a while after a Cube restart (seen
as a reconnect) and may tell the user which devices it is waiting for.

Devices that never report:
three devices paired (C lines) but in no room (no M entry),
listed by the original software as "new devices", never get an entry either.
`t:<count>,<force>,<base64 addresses>` — the same line the software sends for
"delete devices" — was answered with `A:` for each of them,
but the devices stayed in the device list.
Moving them into a room in the original software,
removing them from that room and deleting the room (by hand in the UI,
logged as `arrangeNewDevice`, `removeDevices`,
`removeRoom`) did not remove them either; the next backup still listed them,
unnamed, and a power cycle of the Cube did not drop them.
The FHEM project notes that `t:` makes the Cube send a factory reset to the device,
so a device that never answers is apparently never dropped.
Massimo deletes a device the same way (`DELETE /api/v1/cubes/{sn}/devices/{rf}`,
2026-11-06):
wakeup, 0x21 per partner in both directions and 0x23 for a device in a room,
`t:01,1,<rf>`, `m:` without it, and its copy forgets the device on the `A:`.
That a device which answers is dropped from the C lines and the next `L:` is
*unverified* on hardware.

Which room such a device blocks is decided by the room byte of its own C line,
not by the metadata (experiment 2026-11-04, one never-reporting device 0a0101):
moving it into room 1 in the original software (`m:`, `z:`, 0x20 links,
0x22 group 1) set its C-line room to 1 and room 1 commands were discarded at once;
deleting it from room 1 (`z:`, 0x21, `t:01,1,…`,
`m:` without it) left the C-line room at 1 and room 1 stayed blocked,
the device being "new" again;
putting it back into a parking room (`arrangeNewDevice`: 0x22 group 6, `c:`,
`m:`) set the byte to 6 and room 1 accepted commands again.
This is how the previous owner's leftovers blocked rooms 4 and 5 (their C lines said 4,
4 and 5 while they were unlisted).
Earlier the same day, deleting the three never-reporting devices from a parking
room and then removing that room put the bytes back to 4, 4 and 5,
so removing the room seems to restore an older value; keep the parking room,
with the devices in it, instead.
The device types of the three are shutter contacts;
their serials start with IEQ.

After a 0x40 set-temperature command the Cube updates the setpoint in the room
members' L entries at once,
while the mode bits change only when the devices report back,
about 45 s later in one observation (a room shown as auto at the new temperature
in between).
A client that needs the mode should wait for the next L line rather than read
the one right after the command.

A window contact showed the link-error bit (0x40 of the second flags byte) after
its room's wall thermostat had been moved out and back in;
the bit cleared with the contact's next transmission (one open and close of the window).

The byte at offset 4 of an `L` entry (undocumented) was observed as 0x09,
0x03, 0xf1 and 0xf6; it changed after failed transmissions to that device.

Duty cycle and memory slots: with the thermostats out of range,
each command raised the duty cycle (0x44 → 0x64 = 100 %) and consumed one free
memory slot (50 → 38) because the Cube keeps the unacknowledged radiogram queued.
At 100 % the Cube still accepted commands (`S:64,0,…`, not discarded).
A driver must therefore watch both numbers.

## What the PC software (1.4.6) knows about the portal

Decompiled from the jars (javap), 2026-11-04.
Classes under `de.eq3.max.al.local.remoteproxy`.

- Lookup: `GET <LookupURL>` (default `http://www.max-portal.eq-3.de/lookup`)
  with the `Opt`/`MAX-Serial` headers.
  `200` → the body is the device-app URL;
  `301` → `Content-Location` is the new lookup URL (max. 3 hops).
  If the result differs from the Cube's stored URL the software sends `u:<url>`
  over the LAN.
  With the lookup disabled, `http://max.eq-3.de/cube` is used.
- PC requests are `H:<serial>,<rf>,<fw>\r\n` plus one line,
  encrypted through the Cube's `e:`.
  Lines: `G:` (get credentials), `B:<user>,
  <md5(pw+serial)>,<1|0>,<pw>` (set), `J:<1|0>` (remote access on/off),
  `P:<key>,<value>` / `O:<key>` (user data),
  `V:` / `W:<code>` (product activation).
  Replies are decrypted through `d:` and parsed as `<letter>:<f0>,<f1>,…`:
  `b:OK`, `g:<user>,<true|false>`, `j:OK`, `o:<value>`,
  `v:<activated>,<activationDate>,<expiryDate>,<daysTillExpiry>,
  <informAboutExpiry>,<webShopURI>,<base64 businessName>[,<base64 texts>]`,
  `w:<activated>,<expiryDate>,<daysTillExpiry>,<criteriaKeyViolation>`.
- **The software never parses the `i:` line of a reply.**
  Its three fields are not named anywhere.
  The LAN command is built as `i:<interval>,<token>` with `token = 0`;
  the software only ever sends `i:00000000,00000000` (after `b:OK`/`j:OK` with
  remote access on) or `i:ffffffff,00000000` (off).
- The software never sends device commands through the portal;
  everything goes over TCP 62910.
  No portal server code is bundled.
- Other constants: software version check `GET http://max.eq-3.de/software`,
  software updates `http://www.max.eq-3.de/maxupdate/version.txt` and `digest.txt`,
  downloads `http://www.max-portal.elv.de/max/lib/`,
  notifications `http://max.eq-3.de/messaging`.
  The software refuses Cubes with firmware outside 273..275 (1.1.3 = 275).

## Cube UDP protocol (port 23272): discovery, network setup, firmware

From `de.eq3.max.al.local.update.*` and `CubeFinder`.
Requests go unicast to the Cube,
or to 224.0.0.1 / 255.255.255.255 for discovery.
Packet:

    bytes 0..7   "eQ3Max*\0"
    bytes 8..17  serial, padded with '*' ("**********" = any Cube)
    byte 18      opcode
    bytes 19..   payload

Opcodes: `I` identify, `N` get IP, `B` enter bootloader, `U` init update,
`W` write frame, `R` reboot, `c`/`C` get/set network configuration (ip, gateway,
netmask, dns1, dns2, dhcp flag, encryption flag),
`h`/`H` get/set portal URL (`H`: port big-endian 16 bit, host, ',', path).

Replies: `"eQ3MaxAp"` (application) or `"eQ3MaxBl"` (bootloader), serial, `>`,
opcode, data.
For `I` the data is the RF address (4 bytes,
leading zero) and the firmware version (2 bytes), as seen on the wire,
the serial and the address replaced by placeholders:
`eQ3MaxApKEQ0000100>I 00 0a 01 07 01 13`.

Network configuration, **confirmed on hardware** (2026-11-06,
a Cube with firmware 1.1.3,
from a Wireshark capture of the original software 1.4.6 setting a static address
and then DHCP again):

- `N` gives the addresses in use: ip, gateway, netmask, dns1,
  dns2 (four bytes each); the original software reads the ip only.
- `c` gives the **stored static configuration and the flags**,
  not the DHCP lease: ip, gateway, netmask, dns1, dns2 (four bytes each),
  the DHCP flag and the encryption flag (one byte each), 22 bytes.
  With DHCP on, a factory Cube reports 192.168.0.222/16 with gateway and DNS
  192.168.0.1 and flag 1; the stored addresses are then only the fallback.
  `massimo discover` takes gateway,
  netmask and DNS from `N` and only the DHCP flag from `c`;
  `massimo netconfig` shows both, labelled.
- `C` writes the configuration; its payload is exactly the `c` reply's layout.
  The frame as the software sent it and `massimo netconfig` sends it:

      bytes 0..7   "eQ3Max*\0" bytes 8..17  the Cube's serial,
      padded with '*' (never "**********") byte 18      'C' bytes 19..22 ip
      bytes 23..26 gateway bytes 27..30 netmask bytes 31..34 dns1 (zero when
      none) bytes 35..38 dns2 (zero when none) byte 39      DHCP flag,
      0 or 1 byte 40      encryption flag, 0 or 1

  41 bytes.
  The static write on the wire carried
  `c0a801de c0a80101 ffffff00 c0a80101 00000000 00 01` (192.168.1.222/24,
  gateway and DNS 192.168.1.1, DHCP off, encryption on);
  the DHCP write afterwards was the same bytes with the DHCP flag `01`.
  The encryption flag (1 on that Cube) was copied unchanged;
  Massimo copies it from a `c` read too.
- The Cube answers `C` with one status byte: `01` after a static write,
  sent **from the new address** within 30 ms, so the address applies at once;
  `02` after a write that turns DHCP on,
  after which the Cube restarts by itself and takes a lease (2 = restart,
  as in the firmware update replies).
- The software sends `R` (reboot, with the serial) right after `C`.
  After the static write the Cube answered `R` with `02` from the new address;
  after the DHCP write the `R`s went unanswered because the Cube was already restarting.
  `massimo netconfig` sends `R` after a status `01` only,
  waits a second for its answer and goes on without one.
- `R` alone restarts the Cube, which frees one that still holds a dead session
  ("Lessons from the hardware days", 2026-11-13).
  Massimo sends it only on a person's word, since a restart drops the radiograms
  the Cube has not delivered: **Reboot the Cube** in System settings → Cubes,
  `POST /api/v1/cubes/{sn}/reboot` from the server's discovery socket,
  and `massimo netconfig -reboot`.
- After the move, the original software's own network dialogue failed to look
  the Cube up again although the `I` replies arrived,
  so it could not set DHCP back in the same run; a restart of the software did.
  `massimo netconfig` finds the Cube again with `N` (at the old address,
  at the new one and, after a DHCP write,
  as a broadcast with the serial) and reads `c` from the address that answered,
  allowing a few seconds for the restart.

Run of `massimo netconfig` against the same Cube (2026-11-06, firmware 1.1.3):
a static write got status `01`, the `R` after it status `02`,
and the re-read after the restart answered at once at the same address with mode static.
A DHCP write got status `02` and the Cube restarted;
it answered pings again about 30 to 40 s after the write and then reported DHCP
at its lease, with the static values written before as the stored fallback.
`netconfig` therefore waits up to 60 s after a DHCP write or a status `02`,
sending `N` every 2.5 s (unicast to the old and new addresses,
broadcast with the serial).

Because the serial selects the Cube, `C` also works as a broadcast,
the way back for a Cube on a wrong address (`netconfig -sn <serial> -broadcast -dhcp`).
It first sends `c` the same way and, when the Cube answers,
writes its stored addresses and encryption flag back with only the DHCP flag changed;
without an answer it sends zeros for the addresses,
which then replace the stored fallback.
Not tried on hardware.

Firmware update: `N`, `I`, `B` (then wait for the bootloader to answer `N`),
`U`, then one `W` per frame (`index16, length16, data`,
reply `status, index16` with status 0 fail / 1 ok / 2 restart),
then wait for the reboot.
The frames come from `firmware.enc` in the jars: ASCII hex,
each frame `length16` + data,
1170 frames alternating 274 and 18 bytes plus a final 34.
**The PC software does not decrypt it**;
the content has the entropy of ciphertext and the key lives in the Cube's bootloader,
so the firmware cannot be analysed from the software alone.

## Lessons from the hardware days

What the days at the Cube taught beyond the protocol lines,
kept here because they cost time to learn.

- A Cube with a client drops a second client's SYNs silently:
  a direct test connection times out and looks like a dead Cube.
  Test through the proxy.
- A Cube that moved networks or booted during an unstable network can answer
  ping but not TCP 62910 or UDP 23272; a power cycle fixes it.
- Removing the IP address `serve` used as its source before restarting `serve`
  leaves the Cube holding a dead session, since the FIN cannot be sent;
  the Cube needed a power cycle.
  Restart `serve` first, then change addresses.
  Linux picks the first static address on the interface as the source for Cube
  connections, not the DHCP one.
  A UDP `R` frees such a Cube without the power cycle (the next entry).
- An outage of the network of the box running `serve` leaves the Cube holding
  `serve`'s connection after `serve` gave it up, and the Cube refuses `serve`
  until it lets go (2026-11-13, firmware 1.1.3, measured with tcpdump on a Pi):
  - The owner rebooted the router, so the box's link went down and up four times
    in about a minute.
    The two Cubes whose connections were idle were lost after 26–30 s:
    the keepalive (15 s idle, 5 s apart, 3 probes) ended them,
    with "read: connection timed out",
    and the kernel's reset never reached the Cubes.
    The three Cubes in the middle of a poll kept their connections:
    TCP retransmits unacknowledged bytes for many minutes,
    and keepalive probes wait until they are acknowledged.
  - The Cubes kept the dead session and answered every new SYN with a reset
    with sequence number 0 and no ACK.
    Linux ignores such a reset in SYN-SENT,
    so every dial ended in "i/o timeout".
    This is the refusal a second client meets (the first entry),
    with `serve` its own second client.
  - The refusal ended by itself after 41½ minutes,
    for both Cubes at the same second.
    In a reproduction, which dropped the box's traffic to one Cube's port 62910
    for 26 s with an nft rule, it had not ended after 15¾ minutes.
  - A SYN from the dead connection's own address and port got no answer at all,
    not even a challenge ACK, so the kernel cannot reset the old connection,
    and the sequence number for a reset written by hand is unknown.
  - A UDP `R` (`"eQ3Max*\0"`, the serial, `R`, to port 23272) freed the Cube:
    it stopped answering ping at once, answered again within 43 s,
    and `serve` reconnected 61 s after the `R`.
    A restart drops the radiograms the Cube has not delivered yet (below).
- Two routers on the same 192.168.1.0/24 (wired and Wi-Fi) on one box make
  routing ambiguous; the Wi-Fi side needed SO_BINDTODEVICE.
- A Cube restart drops undelivered radiograms.
  A device move whose radiograms are lost leaves the metadata right and the
  device's links half done; the only sign is the device's link_error flag,
  and the fix is repeating the move.
- A window contact fetches radiograms only right after it transmits
  and not always all at once: a move needed two window cycles per direction,
  with an error reset first.
  Read the pending count fresh from the Cube to know when a move is done.
- Functional link check for a contact: window open,
  that room's thermostat goes to its window-open temperature within a minute,
  other rooms do not react, and it goes back after closing.
- Cost: about 1.25 % duty cycle per radio configuration write,
  about 8 % per device move; 1 % per hour is about 36 s of air time.
- The Cube sends its H line (budget, free slots,
  hence pending radiograms) only when a connection starts and in S replies;
  no command asks for it.
  The Cube changes a device's C line only after the device acknowledged (1 to 4 min);
  the setpoint shows in L at once, the mode only with the device's report.
- The Cube answers UDP discovery only to port 23272,
  whatever the request's source port, so on the box running `serve`,
  `massimo discover` sees only `serve`'s own proxy answer;
  use `POST /api/v1/cubes/discover` there.
  `serve` shares the port (2026-11-14, measured on Windows 11 and Linux
  with Java 8's `MulticastSocket`, as the original software 1.4.6 binds it,
  no Cube involved): a unicast datagram to a shared port reaches one socket,
  on Windows the one bound first, on Linux the one bound last,
  `serve`'s udp4 socket ahead of Java's dual-stack one; a broadcast reaches all.
  So a Cube's answer to the original software on `serve`'s box goes to `serve`,
  which answers a search from its own host by broadcast as well.
  Across Windows accounts, as between the service and a user,
  the port is shared only where the first socket set SO_REUSEADDR.
  GitHub creates no push event for more than three tags pushed at once,
  which is unrelated to Cubes but was learned the same night.
- After a Cube restart, the eco switch stays "not reported" until someone presses it;
  harmless.
- A wall thermostat with new batteries reports a measured temperature of 0
  in its L entry until it has measured (2026-11-13, Cube firmware 1.1.3,
  seen as 0.0 °C in a room on the owner's Cube):
  0 means "not measured", as it does for a radiator thermostat,
  and Massimo keeps the last measured value meanwhile.
- The original MAX! software cannot find a Cube again in the same run after it
  moved the Cube to a static address; a restart of the software does.

## Open questions

- Does `i:00000000,00000000` start the Cube's polling immediately,
  and at which rate?
- What does the Cube POST while polling (state? only `H:`?), and in which
  form does the portal deliver commands (`s:`-style lines in the reply?).
- ~~Is the IV identical on every Cube or per device?~~ Per device:
  two Cubes (KEQ…, JEQ…) have different IVs (2026-11-04).
- Meaning of the `i:` reply fields.
  The LAN command is `i:<interval>,
  <token>`; the reply has a third field. Hypothesis to test with long
  measurements: the first field sets the poll interval (0x51b1 = 20913; in
  tenths of a second that is 34.9 min, in the range of the gaps seen).
- The third `B:` field is "remote access enabled".

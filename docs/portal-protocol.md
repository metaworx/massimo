# MAX! Cube portal protocol

How the Cube and the original software talked to eQ-3's portal ("MAX!
internet control"). Reconstructed from the ERNW/Insinuator analysis (2016),
from the behaviour of the original local software (version 1.4.6), and from
experiments with real Cubes (firmware 1.1.3) in 2026. Facts marked
*unverified* still need confirmation on hardware.

## Transport

- `POST <portal url>` (default `http://max.eq-3.de:80/cube`), headers
  `Opt: "http://www.eq-3.com/MAX"; ns=MAX` and `MAX-Serial: <serial>`.
- Body: AES-128-CBC ciphertext of CR LF separated protocol lines, zero padded,
  **not** base64 encoded on the wire.
- Key and IV are fixed per device. The key is printed on the circuit board as
  a QR code: `K` followed by 32 hex digits. It is not derivable from the
  serial, MAC, RF address or chip ID (74 hash-based candidates tested, none
  matched). The IV never crosses the wire; it is recovered with the local
  `e:` command (see below).
- The portal replies with the same encoding. Only the body is encrypted;
  there is no TLS, no authentication tag and no replay protection.

## Two clients of the portal

1. **The Cube itself** polls the portal URL once polling is enabled (see
   `i:` below). What it sends while polling and how commands come back has
   not been captured yet.
2. **The local PC software** also talks to the portal directly, using the
   Cube as an encryption oracle: it builds the lines, sends them to the Cube
   with `e:`, and POSTs the ciphertext. This is how user accounts were
   created and remote access toggled.

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

The original portal was observed replying `i:000051b1,00000000,ffffffff`
in front of `b:OK`. The PC software ignores that line; it may be meant for
the Cube's own polling. Meaning of the fields: unknown.

## The complete setup sequence (captured from the original software, 2026-11-04)

With a hosts entry for `www.max-portal.eq-3.de` pointing at this server:

1. `GET /lookup` (headers `Opt`, `MAX-Serial`) → `200`, body = device URL.
2. LAN: `u:<device URL>` → `A:`.
3. LAN `e:` of `H:<serial>,<rf>,<fw>` + `G:` → `POST <device URL>` → reply
   decrypted with LAN `d:` → `g:<user>,<true|false>`.
4. Same with `B:<user>,<md5>,1,<password>` → `b:OK`.
5. LAN: `i:00000000,00000000` → `A:`. The Cube polls the device URL from then on.

Note that the Cube resolves the device URL through the network's DNS, not
the PC's hosts file, so the URL returned by `/lookup` must be reachable
by the Cube (an IP address, or a name the router resolves).

## Lookup service

Before contacting the portal, the PC software did a
`GET http://www.max-portal.eq-3.de/lookup` with the same two headers; the
response (body or `Content-Location` header) was the per-device portal URL,
which was stored in the Cube with `u:` if it differed. The service is gone;
the URL is now set directly.

The Cube's factory default was a lookup URL as well: the C message of a
Cube documented in 2014 (Bouni's protocol notes) still carries the stale
tail `0/lookup` after `http://max.eq-3.de:80/cube\0`, which fits exactly
`http://max-portal.eq-3.de:80/lookup`. So a Cube may itself follow the
lookup procedure (GET, then POST to the returned URL). Untested with our
portal: set `u:http://<host>:80/lookup` and watch for a GET.

## What is public (research, 2026-11-04)

The only published decrypted portal traffic is in the 2016 Insinuator
article: the PC software's `B:` request and the real portal's reply
`i:000051b1,00000000,ffffffff\r\nb:OK\r\n` plus zero padding. That `i:`
line therefore answered a PC request, not a Cube poll; the reply to a
poll was never published. No captures, dumps or replacement portals
exist anywhere else (FHEM, openHAB, ioBroker, Home Assistant, GitHub,
Wayback Machine checked; www.max-portal.eq-3.de was never archived).
Other portal hosts: max-portal.elv.de (ELV), smarthome.md.de:8080
(mobilcom-debitel).

Hardware: the Cube is an Atmel AT91SAM7X256/512 (ARM7TDMI) with a TI
CC1101 on a TRX868 module; JTAG on header ST3, UART0 on ST1, debug on
ST2, erase jumper J1 (a-culfw CUBe port, ELVjournal 1/2012 schematic).
The a-culfw reflash erases the chip, and nobody has published a dump of
the stock firmware. A JTAG read-out of an unmodified Cube would be the
only way to get it, and only if the chip's security bit is not set.

## Recovering the IV from a Cube

Send 32 zero bytes to `e:` (`e:AAAA…AAA=`). In CBC with a zero first block,
the first ciphertext block is the IV encrypted with the key, so decrypting
that block with the key yields the IV. `massimo portal-crypto iv` does this
and also verifies the chaining with the second block.

Verified on a real Cube: the same plaintext always gives the same
ciphertext, the first block of a 16-byte and a 32-byte message is identical,
and the second block differs from the first for identical plaintext blocks,
which rules out ECB and a per-message IV.

## Observed polling (Cube firmware 1.1.3, November 2026)

After `u:` and `i:00000000,00000000` the Cube POSTs to the URL. Each poll
carries the full state: `H:` (long form, new connection id each time),
`M:`, every `C:` and `L:`. Request headers:

    POST /cube HTTP/1.1
    Host: <host>
    connection: close
    Content-Length: <n>
    Opt: "http://www.eq-3.com/MAX", ns=MAX
    MAX-Serial: <serial>

Gaps between polls grew from about 4 to 10–12 minutes within the first
hour after `i:00000000,00000000`, regardless of the reply (`i:` with
various values, `{conn}` echoed, or no reply line at all). `i:00000078,00000000` (120) did not shorten the gaps, so the first field
is not an interval in seconds; whether other units apply is untested.
Both `u:` and `i:` make the Cube poll within about 20 s, which has to be
kept apart from interval effects when measuring.

Capture of 2026-11-04 02:04–02:21 (PC side, Wireshark): `i:00000078,00000000`
over the LAN was followed by a poll 6 s later; the next attempt came 7 min
18 s after that, as eight SYNs over 15 s that nothing answered (the portal
was not listening on that port at the time), then a RST. Unanswered
attempts therefore look like missing polls in the portal log; the gaps of
29, 58 and 91 minutes seen later that morning are close to multiples of
one period and probably include such failed attempts. A Wireshark capture
on the portal host filtered on the Cube's address shows them.

In the same capture the duty cycle went from 0 to 68 % and all six
thermostats got the error flag (L entry byte 5 = 0x0c, byte 4 = 0x03)
between 02:05 and 02:10 without any LAN command: the Cube spends radio
time on its own retries to unreachable devices (they were not mounted).
A client must expect budget to disappear without having sent anything.

**Commands in the reply are ignored.** Lines `f:…` (NTP servers), `v:…`
(time zone) and `m:…` (metadata) sent in the poll reply had no effect,
while the same lines over the LAN work. How the original portal delivered
commands to the Cube is still unknown; the reply channel is not it, at
least not in this form.

## LAN commands verified against the original software (1.4.6)

Captured on 2026-11-04 while operating the original UI:

| UI action | Line sent | Reply |
|---|---|---|
| Set time zone | `v:<base64 of winter rule + summer rule>` (24 bytes, **no time field**) | `A:` |
| Any command to a device in error state | `r:<count>,<base64 addresses>` first | `S:` |
| Room 1 manual 15 °C | `s:AARAAAAAFNCOAV4=` | `S:` |
| Room 3 auto | `s:AARAAAAAFM49AwA=` | `S:` |
| All rooms 6.5 °C until 04.11.2026 02:00 | `s:AARAAAAAFNCOAY2kmgQ=` … | `S:` |

A second capture (week program, presets, valve settings, renames):

| UI action | Lines sent | Reply |
|---|---|---|
| Edit week program of room 1 | `z:1e,G,01` (wake the room for 30 s), then one `s:` (0x10) per changed day, e.g. `s:AAQQAAAAFNCOAQRO3VTzRQoxIDEgMSAxIA==` (Wednesday, 4 points padded with the last) | `A:`, then `S:` per day |
| Temperature offset -2 °C | `r:` if in error state, then `s:AAARAAAAFNCOACwePQkDCgM=` (0x11; flags 00, room 00) | `S:` |
| Boost 5 min / 80 % | `s:AAQSAAAAFNCOATAM/wA=` (0x12) | `S:` |
| Rename room or device, reorder rooms | `m:00,<base64 metadata>` (index only, no count) | `A:` |

All of these match the builders in `internal/protocol/cube` byte for byte.
A 0x11 command to a device in error state that was **not** preceded by
`r:` came back as discarded (`S:14,1,31`); the software then reports
"Command discarded". While the UI is open the software polls `l:` every
10 s and disconnects after 30 s without user activity.

### Verified on hardware with Massimo (2026-11-04, Cube JEQ…, live devices)

Through `massimo restore` on a radiator thermostat: 0x11 preset
temperatures (eco 17 → 16.5 shown on the device), 0x10 week program
(one day), 0x12 valve settings (boost 5 → 10 min, shown as 600 s on the
device), each after a `z:` room wakeup; `m:` (rename of a wall
thermostat, confirmed by the next M line); `i:ffffffff,00000000` (portal
polling off, confirmed by the C line). Budget per write: two memory
slots while in flight, duty cycle +3 % for 0x11/0x12, +8 % for one day
of 0x10.

### Pairing a device, as the original software does it

Log of pairing a wall thermostat (NEQ…, rf 15667d, fw 16) into room 4,
which held a radiator thermostat (0147b7) and two shutter contacts
(005447, 1570d3):

1. `n:` inclusion mode; the Cube answers `N:<type><rf><serial>ff` when a
   device joins. Devices paired earlier but never put into a room are
   listed as new devices too (they have a C line but no M entry).
2. `z:1e,G,04` wakeup, then one 0x20 (add link partner) radiogram per
   pair in both directions: to the radiator thermostat with the room
   flag (`00 04 20 … 0147b7 04 15667d 03`), to each shutter contact
   (`00 00 20 … 005447 00 15667d 03`), and to the new device for each
   partner (`00 00 20 … 15667d 00 0147b7 01`, `… 005447 04`, `… 1570d3
   04`); then 0x22 set group (`00 00 22 … 15667d 00 04`).
3. `c:15667d` to fetch the new device's configuration.
4. 0x40 set temperature, auto, temp 0 (`… 40 … 15667d 00 00`).
5. `z:` wakeup, then 0x11 preset temperatures to **every** thermostat of
   the room, radiator and wall alike, with the room's values
   (`27 22 3d 09 07 0a 03` = 19.5/17/30.5/4.5, offset 0, window 5.0 °C,
   15 min).
6. `z:` wakeup, 0x12 to the new device (`30 0c ff 00` = boost 5 min 80 %,
   decalc Sat 12:00, max valve 100 %, offset 0).
7. `z:` wakeup, 0x10 for all seven days to the new device (Mon..Fri,
   Sat, Sun; 7 switch points, padded with the last one), then 0x11 once
   more to the new device.
8. `m:` with the device added to the room and device list.

Moving a device to another room (same software, wall thermostat from
room 4 to room 5): wakeup of the old room, 0x23 remove group (`… 23 …
15667d 00 04`), 0x21 remove link partner for every device of the old
room in both directions (same addressing as 0x20), `m:` with the device
taken out of the room, then wakeup of the new room, 0x20 add link partner
for every device of the new room in both directions, 0x22 set group,
`c:` fetch, 0x40 auto, 0x11 to the room's thermostats, 0x12, 0x10 × 7,
0x11, `m:` with the device in the new room. Links are therefore the
only per-device data that is not readable from the Cube; a restore
derives them from the room layout.

So wall thermostats take 0x11 and 0x12 with the same payloads as
radiator thermostats, and their C line ends with offset, window-open
temperature and boost (3 bytes after the week program at 0x16). Their L
entry (12 bytes) carries the setpoint at offset 8 and the measured
temperature at 12 (with bit 7 of byte 8 as the ninth bit); byte 7 is
not a valve but the display mode: 0x01 shows the setpoint, 0x04 the
measured temperature, set with radiogram 0x82 and one byte (4 or 0)
after a room wakeup (`00 00 82 … 15667d 00 04`).

The original software's `FrameType` enum (MaxEssentialsBackend 1.4.6)
lists every radiogram the system knows, with the codes: 0x00
SystemInformation (pair ping), 0x01 Inclusion (pair pong), 0x02 Answer,
0x03 TimeInformation, 0x10 ConfigWeekTemperatureProfile, 0x11
ConfigTemperatures, 0x12 ConfigValveFunctions, 0x20 AddLinkPartner, 0x21
RemoveLinkPartner, 0x22 SetGroupAddress, 0x23 RemoveGroupAddress, 0x30
ShutterContactState, 0x40 ControlMode (set temperature), 0x42
SetPointAndCurrentTemperature (wall thermostat), 0x43
ComfortTemperature, 0x44 EcoTemperature, 0x50 PushButtonState, 0x60
HeatingThermostatState, 0x70 CurrentTemperatureAndHumidity, **0x80
LockManualControls**, 0x81 DaylightSavingTimeMode, 0x82
SwitchDisplaySetPointActual, 0xF0 WakeUp, 0xF1 Reset. Only `FrameType`
itself references `LockManualControls`: the software has no UI for the
key lock and never sends 0x80. The lock state is readable, as bit 0x20
of the second flags byte of an L entry. Massimo sends 0x80 with one
payload byte (1 lock, 0 unlock) like 0x82, directly to the device after
a room wakeup; it is not yet verified on hardware. On the device the
lock is toggled with Auto/Manu plus the second button, see the eQ-3
manuals.

Set temperature (0x40) addressing, verified 2026-11-04: a device that
is in a room is only accepted as its room (flags 0x04, room id); both
flags 0 with the room id and flags 0 with room 0 were discarded by the
Cube for such a device. A device without a room (just paired) takes
flags 0 and room 0.

Room commands and never-reported members, 2026-11-04: on a Cube with
four rooms, 0x40 to the two rooms whose members had all reported was
accepted, 0x40 to the two rooms that each held a paired contact that
had never reported (no valid L entry) was discarded, for any recipient,
with or without a wakeup, and the original software got the same
`CommandDiscarded` for one of them. The Cube apparently refuses a group
command while a member cannot acknowledge it.

The contacts in question turned out to be fine. The rule, confirmed
after a power cycle of the Cube: **the Cube discards a room command
while any member of the room has no state in the L line** (valid bit
0x10 of the first flags byte clear). A restart clears every entry; a
device gets its entry back when it next transmits. Radiator and wall
thermostats do so on their own within minutes to hours, a window
contact only on a window change, the eco switch only on a button press.
Right after the restart only the rooms whose members had all reported
accepted commands; opening and closing the two windows of a refused
room and turning its thermostat's wheel made that room accept within a
minute. The original software is subject to the same rule. A driver
must therefore expect discards for a while after a Cube restart (seen
as a reconnect) and may tell the user which devices it is waiting for.

Devices that never report: three devices paired (C lines) but in no
room (no M entry), listed by the original software as "new devices",
never get an entry either. `t:<count>,<force>,<base64 addresses>` — the
same line the software sends for "delete devices" — was answered with
`A:` for each of them, but the devices stayed in the device list.
Moving them into a room in the original software, removing them from
that room and deleting the room (by hand in the UI, logged as
`arrangeNewDevice`, `removeDevices`, `removeRoom`) did not remove them
either; the next backup still listed them, unnamed, and a power cycle
of the Cube did not drop them. The FHEM project notes that `t:` makes
the Cube send a factory reset to the device, so a device that never
answers is apparently never dropped. Workaround: assign each such device
to a room of its own, so that it is a member of no room that matters.

The byte at offset 4 of an `L` entry (undocumented) was observed as 0x09,
0x03, 0xf1 and 0xf6; it changed after failed transmissions to that device.

Duty cycle and memory slots: with the thermostats out of range, each
command raised the duty cycle (0x44 → 0x64 = 100 %) and consumed one free
memory slot (50 → 38) because the Cube keeps the unacknowledged radiogram
queued. At 100 % the Cube still accepted commands (`S:64,0,…`, not
discarded). A driver must therefore watch both numbers.

## What the PC software (1.4.6) knows about the portal

Decompiled from the jars (javap), 2026-11-04. Classes under
`de.eq3.max.al.local.remoteproxy`.

- Lookup: `GET <LookupURL>` (default `http://www.max-portal.eq-3.de/lookup`)
  with the `Opt`/`MAX-Serial` headers. `200` → the body is the device-app
  URL; `301` → `Content-Location` is the new lookup URL (max. 3 hops). If
  the result differs from the Cube's stored URL the software sends
  `u:<url>` over the LAN. With the lookup disabled, `http://max.eq-3.de/cube`
  is used.
- PC requests are `H:<serial>,<rf>,<fw>\r\n` plus one line, encrypted
  through the Cube's `e:`. Lines: `G:` (get credentials), `B:<user>,
  <md5(pw+serial)>,<1|0>,<pw>` (set), `J:<1|0>` (remote access on/off),
  `P:<key>,<value>` / `O:<key>` (user data), `V:` / `W:<code>` (product
  activation). Replies are decrypted through `d:` and parsed as
  `<letter>:<f0>,<f1>,…`: `b:OK`, `g:<user>,<true|false>`, `j:OK`,
  `o:<value>`, `v:<activated>,<activationDate>,<expiryDate>,<daysTillExpiry>,
  <informAboutExpiry>,<webShopURI>,<base64 businessName>[,<base64 texts>]`,
  `w:<activated>,<expiryDate>,<daysTillExpiry>,<criteriaKeyViolation>`.
- **The software never parses the `i:` line of a reply.** Its three fields
  are not named anywhere. The LAN command is built as
  `i:<interval>,<token>` with `token = 0`; the software only ever sends
  `i:00000000,00000000` (after `b:OK`/`j:OK` with remote access on) or
  `i:ffffffff,00000000` (off).
- The software never sends device commands through the portal; everything
  goes over TCP 62910. No portal server code is bundled.
- Other constants: software version check `GET http://max.eq-3.de/software`,
  software updates `http://www.max.eq-3.de/maxupdate/version.txt` and
  `digest.txt`, downloads `http://www.max-portal.elv.de/max/lib/`,
  notifications `http://max.eq-3.de/messaging`. The software refuses Cubes
  with firmware outside 273..275 (1.1.3 = 275).

## Cube UDP protocol (port 23272): discovery, network setup, firmware

From `de.eq3.max.al.local.update.*` and `CubeFinder`. Requests go unicast
to the Cube, or to 224.0.0.1 / 255.255.255.255 for discovery. Packet:

    bytes 0..7   "eQ3Max*\0"
    bytes 8..17  serial, padded with '*' ("**********" = any Cube)
    byte 18      opcode
    bytes 19..   payload

Opcodes: `I` identify, `N` get IP, `B` enter bootloader, `U` init update,
`W` write frame, `R` reboot, `c`/`C` get/set network configuration
(ip, gateway, netmask, dns1, dns2, dhcp flag, encryption flag), `h`/`H`
get/set portal URL (`H`: port big-endian 16 bit, host, ',', path).

Replies: `"eQ3MaxAp"` (application) or `"eQ3MaxBl"` (bootloader), serial,
`>`, opcode, data. For `I` the data is the RF address (4 bytes, leading
zero) and the firmware version (2 bytes), as seen on the wire:
`eQ3MaxApKEQ0537249>I 00 0b 95 73 01 13`.

Firmware update: `N`, `I`, `B` (then wait for the bootloader to answer
`N`), `U`, then one `W` per frame (`index16, length16, data`, reply
`status, index16` with status 0 fail / 1 ok / 2 restart), then wait for
the reboot. The frames come from `firmware.enc` in the jars: ASCII hex,
each frame `length16` + data, 1170 frames alternating 274 and 18 bytes
plus a final 34. **The PC software does not decrypt it**; the content has
the entropy of ciphertext and the key lives in the Cube's bootloader, so
the firmware cannot be analysed from the software alone.

## Open questions

- Does `i:00000000,00000000` start the Cube's polling immediately, and at
  which rate?
- What does the Cube POST while polling (state? only `H:`?), and in which
  form does the portal deliver commands (`s:`-style lines in the reply?).
- ~~Is the IV identical on every Cube or per device?~~ Per device: two
  Cubes (KEQ…, JEQ…) have different IVs (2026-11-04).
- Meaning of the `i:` reply fields. The LAN command is `i:<interval>,
  <token>`; the reply has a third field. Hypothesis to test with long
  measurements: the first field sets the poll interval (0x51b1 = 20913; in
  tenths of a second that is 34.9 min, in the range of the gaps seen).
- The third `B:` field is "remote access enabled".

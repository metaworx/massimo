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

The byte at offset 4 of an `L` entry (undocumented) was observed as 0x09,
0x03 and 0xf1; it changed after failed transmissions to that device.

Duty cycle and memory slots: with the thermostats out of range, each
command raised the duty cycle (0x44 → 0x64 = 100 %) and consumed one free
memory slot (50 → 38) because the Cube keeps the unacknowledged radiogram
queued. At 100 % the Cube still accepted commands (`S:64,0,…`, not
discarded). A driver must therefore watch both numbers.

## Open questions

- Does `i:00000000,00000000` start the Cube's polling immediately, and at
  which rate?
- What does the Cube POST while polling (state? only `H:`?), and in which
  form does the portal deliver commands (`s:`-style lines in the reply?).
- Is the IV identical on every Cube (firmware constant) or per device?
- Meaning of the `i:` reply fields and of the third `B:` field.

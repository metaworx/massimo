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
| `B:<user>,<md5(password + serial)>,<0/1>,<password>` | `b:OK` or an error text | Create or update the portal account. The third field is a boolean whose meaning is not known. |
| `G:` | `g:<user>,<true/false>` | Read account and remote-access state (*unverified* field order). |
| `J:1` / `J:0` | `j:OK` | Enable or disable remote access on the portal. |
| `P:…` / `O:…` | `p:` / `o:` | Set/get user data. Not analysed. |
| `V:` / `W:<code>` | `v:` / `w:` | Product activation. Not analysed. |

After `B:` or `J:` succeeds, the PC sends `i:` to the Cube with 0 (enabled)
or -1 (disabled).

The original portal was observed replying `i:000051b1,00000000,ffffffff`
in front of `b:OK`. The PC software ignores that line; it may be meant for
the Cube's own polling. Meaning of the fields: unknown.

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

Gaps between polls grew from about 4 to 10 minutes within the first half
hour, with the reply `i:000051b1,00000000,ffffffff` and also with
`i:0000003c,…`; the first `i:` field is therefore not a poll interval in
seconds, or the reply is ignored.

## Open questions

- Does `i:00000000,00000000` start the Cube's polling immediately, and at
  which rate?
- What does the Cube POST while polling (state? only `H:`?), and in which
  form does the portal deliver commands (`s:`-style lines in the reply?).
- Is the IV identical on every Cube (firmware constant) or per device?
- Meaning of the `i:` reply fields and of the third `B:` field.

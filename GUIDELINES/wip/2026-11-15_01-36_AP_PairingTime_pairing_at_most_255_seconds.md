# AP PairingTime v1.0: Pairing at most 255 seconds

## TL;DR

The Cube reads only the low byte of `n:`'s four hex digits, so a
pairing lasts at most 255 s; Massimo caps it there everywhere and
writes the finding into the protocol document.

- `cube.MaxInclusion` 255 s, new; `CmdInclusion` clamps to it
  (`n:00ff`); `cubeio.MaxPairingTime` 10 min → `cube.MaxInclusion`
- API `seconds` 1–600 → 1–255, `400` above (D2-A); OpenAPI `maximum: 255`
- proxy: a client's `n:` above 255 s → 255 s, logged with the time
  asked (D1-A)
- UI: the pairing time's `max` and check 600 → 255, `pairing.secondsBad`
  in all six languages
- fake Cube: takes the low byte as the real one
- `docs/cube-protocol.md`, the pairing section only: the finding of
  2026-11-14, firmware 1.1.3; the empty `N` at the end of the time
  verified, `x:` and a joining device still not
- `docs/api-v1.md`, `docs/proxy.md`, `docs/ui.md`: 600 s, ten minutes → 255 s

### Blocks

1. Finding: the pairing time in `docs/cube-protocol.md`
2. Cap: 255 s in the builder, cubeio, the API, the proxy, the UI and
   the fake Cube; tests with the exact `n:` line; docs; CHANGELOG fragment

## Discussion

The owner's brief of 2026-11-15, from breno's store (the `log` table:
`change` rows with action `pairing.start`, then `pairing_started` and
`pairing_ended`) on one of the owner's Cubes, firmware 1.1.3, 2026-11-14:

- `POST …/pairing` with 60 s sent `n:003c`; the end came 60.1 s later;
- with 300 s it sent `n:012c`; the end came 44.2 s later, and 44 = 0x2c,
  the low byte of 0x012c;
- both ends came from the Cube: Massimo's own timer ends a pairing only
  ten seconds after the time asked (`inclusionGrace`), and no `x:` was
  sent; so the Cube sends the empty `N` line when the time is up;
- no device joined in either.

The reading: the Cube takes only the low byte, so pairing lasts at most
255 s, and 256 s (`n:0100`) would be read as 0, which `CmdInclusion`'s
comment calls "no timeout" and which is not verified. The original
software 1.4.6 also sends four hex digits (`Conversion.toHex16` of
`inclusionModeTimeoutSeconds`); its configured value was not found.

`PLAN`, then `PLAN++` with the gate's recommendations:

- D1-A: a client's `n:` above 255 s through the proxy is capped at 255 s
  and logged. `n:` has no reply, so a refused one (B) would leave the
  original software in pairing mode with no `N` line and no end; passing
  the low byte on as the Cube would (C) turns 300 s into 44 s and 256 s
  into an unverified 0.
- D2-A: the API refuses 256 to 600 with `400`, as it refused more than
  600 before; such a request paired for the wrong time anyway, and
  Massimo's own callers (the UI, the proxy) stay within the range; the
  Home Assistant integration does not pair.

## Analysis

**Where the 600 s live** (origin/main of 2026-11-15):

- `internal/cubeio/pairing.go`: `MaxPairingTime = 10 * time.Minute`,
  checked by `StartPairing` ("pairing time must be 1 s to 10m0s");
- `internal/server/pairing_api.go`: `seconds` checked against it,
  `400 seconds must be 1..600`;
- `internal/proxy/proxy.go` (`pairing`): the client's hex time, 60 s
  without one or with 0, then `min(timeout, MaxPairingTime)`;
  the line to the Cube is built again by `CmdInclusion`, so the client's
  digits never pass through as they are;
- `internal/server/ui/partials/pairing.html` (`max="600"`),
  `internal/server/ui/cubes.js` (`startPairing`, `secs > 600`), and
  `pairing.secondsBad` in the six `ui/i18n/<lang>.json`;
- `docs/api-v1.md` ("Pairing"), `docs/api-v1-openapi.yaml`
  (`maximum: 600`), `docs/proxy.md` ("Pairing": "ten minutes"),
  `docs/ui.md` ("at most 600").

**The builder.** `CmdInclusion` writes `%04x` of the seconds, as the
original software does; it clamps to `cube.MaxInclusion`, so that no
later caller can send a time the Cube reads short. A zero time still
sends a bare `n:`; no caller sends it (cubeio refuses less than 1 s, the
proxy makes 0 into 60 s), and its comment says that what the Cube does
with it is not verified.

**Massimo's own end.** The client ends a pairing ten seconds after the
time asked (`inclusionGrace`) where the Cube's empty `N` line does not
come; with the cap, the time asked is the Cube's own, so that stays.

**The fake Cube** takes `n:`'s time modulo 256, as the real one, and 60 s
for 0 as before; the comment on `Pairing` and the package comment of
`inclusion.go` say that the end of the time is seen on hardware and the
end by `x:` is not.

**The protocol document.** Another session writes other findings of
2026-11-14 (transmit-error flag, `r:`, memory slots) into
`docs/cube-protocol.md`, so Block 1 keeps to "Pairing a device, as the
original software does it": the paragraph on what Massimo relies on is
rewritten (the end of the time verified; `x:`, a joining device's `N`
line, the metadata and `c:` during the mode still not), and a dated
paragraph gives the finding, without serials, radio addresses or the
owner's names. The summary list "Verified on hardware" at the top keeps
"pairing" among the unverified, which stays true for pairing a device.

## Implementation Plan

Every commit cites `AP PairingTime v1.0, Block <n>`.

### Block 1: Finding

- `docs/cube-protocol.md`, "Pairing a device, as the original software
  does it": the finding of 2026-11-14 (firmware 1.1.3, one of the
  owner's Cubes): `n:003c` ended after 60.1 s, `n:012c` after 44.2 s,
  both by the Cube's empty `N` line; the low byte read, at most 255 s,
  `n:0100` read as 0 presumably, what 0 does unverified; the original
  software's four hex digits; no device joined. The paragraph on what
  Massimo relies on rewritten to match.

**Verification:** `GUIDELINES/shared/tools/check` (reflow of the
document), no serial, radio address or name in the new text
(`MASSIMO_ENV_FILE=/dev/null go test ./tools/identifiers/...`).

### Block 2: Cap

- `internal/protocol/cube/commands.go`: `MaxInclusion`, `CmdInclusion`
  clamps to it, its comment on 0.
- `internal/cubeio/pairing.go`: `MaxPairingTime = cube.MaxInclusion`,
  the error in seconds.
- `internal/server/pairing_api.go`: unchanged code, the range from the
  constant.
- `internal/proxy/proxy.go`: the time asked logged where it is capped.
- `internal/cubelan/inclusion.go`, `internal/cubelan/fakecube/fakecube.go`:
  the comments, and the fake's low byte.
- UI: `pairing.html` `max="255"`, `cubes.js` 255, `pairing.secondsBad`
  in the six languages.
- Tests: `CmdInclusion` 255 s and 300 s → `n:00ff`; the API with 256 →
  `400`, with 255 → `200`, the fake Cube received `n:00ff` and `until` is
  255 s after `started`; the proxy with a client's `n:012c` → the fake
  Cube received `n:00ff`; the fake with `n:0101` ends after a second;
  `StartPairing` refuses 256 s; the UI's time field refuses 256
  (`tools/ui-test/test_pairing.py`).
- Docs: `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/proxy.md`,
  `docs/ui.md`.
- CHANGELOG fragment, `Fixed`.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .`,
`go vet` for linux, windows and darwin, `make cross`, `make ui-test`,
`GUIDELINES/shared/tools/check`.

## Proposed commit messages

- `chore(aps): +PairingTime v1.0`, this plan, on main.
- Block 1, `docs(cube): A Cube keeps pairing mode for at most 255 seconds`.
- Block 2, `fix(pairing): Pairing asks the Cube for at most 255 seconds`.

## Change History

- v1.0 (2026-11-15): first revision, by `PLAN++` after `PLAN`, D1-A and
  D2-A.

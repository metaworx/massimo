# AP ZeroSlot v1.0: radio address 000000 is no device

## TL;DR

A Cube can list an entry at radio address 000000: no device, no room,
and Massimo shows it as an unnamed radiator whose Remove would send a
factory reset to 000000. That entry stops being a device. Massimo sends
nothing to it, and keeps the greeting exactly as the Cube sent it.

- view → no device at 000000: no tile, no "Devices without a room"
  entry, no device in the API
- serve's log → one warning per Cube and connect, naming the entry
- copy of the greeting → as received: the L entry and a second
  Cube-typed C line are replayed by the proxy and kept in backups
  (D69-A)
- every command naming 000000 → 400 before anything is sent
- `docs/cube-protocol.md` → the finding, with its date and firmware

### Blocks

1. NotADevice: 000000 out of the view and the API, the copy kept as
   received, the warning; tests from captured lines
2. Refused: every route and command naming 000000 answers 400, nothing
   reaches the Cube; tests
3. Docs: the finding in `docs/cube-protocol.md`, `docs/api-v1.md`, a
   fragment

The three blocks are one pull request.

## Discussion

On 2026-11-13 the owner noticed an "(unnamed) · 000000" device under
"Devices without a room" on one of their Cubes. A read-only look found
the Cube's L line holding `0b 00 00 00 09 00 …`: radio address 000000,
no flags. It has no C line of a device and no room. The entry was
verified on the Cube itself on 2026-11-14, with serve stopped for four
seconds and nothing sent: the greeting carries the same L entry and a C
line `C:000000,…` of 207 bytes, its address and type bytes zero (type
0, the Cube's own). No record says how it got there.

Massimo shows it with Lock, Put into a room… and Remove from the Cube.
Remove sends a wakeup, the 0x21 and 0x23 radiograms and `t:01,1,<rf>`.
That makes the Cube send a factory reset to the address. To 000000 it
could reach more than intended, so the owner was told not to press its
buttons.

Settled in the owner's evaluation of 2026-11-13/14:

- D67-A: 000000 is never a device, commands to it are refused, a warning
  says so;
- D69-A: the greeting is kept as received, so the proxy hands the
  original software what the Cube sent, and a backup holds it;
- the entry is not removed from the Cube by Massimo: what clears it is
  the owner's decision, outside this plan.

`PLAN++ as recommended` on 2026-11-14.

## Analysis

**The copy** (`internal/cubelan/snapshot.go`, `Apply`). A C line goes
into `s.configs` by its radio address; one of `cube.DeviceCube`'s type
is not noted as a device (`noteLocked`). An L line's entries
(`cube.ParseDeviceList`) go into `s.states` and are noted as devices.
The 000000 entry is therefore a device of the copy: `view.Devices` and
`view.RoomsOf` list it, the type guessed from the entry's length
(`guessType`). The proxy replays the copy's greeting. A second C line
of the Cube's type at 000000 is kept in `configs` but not replayed,
which is how the original software got a greeting the Cube did not
send. Where the replay leaves it out is found in Block 1.

**The view** (`internal/view`). `Devices`, `DevicesOf` and `RoomsOf`
build the API's devices from the copy, its metadata and the hidden
list. One filter there keeps 000000 out of every list the API and the
UI show: tiles, the "Devices without a room" list, `GET …/devices`,
the log's device names and the push notifications (`push_api.go` uses
`view.Devices`).

**The commands** (`internal/server`, `internal/protocol/cube`
`commands.go`). The device routes take a radio address or a serial
(`GET/PUT/DELETE /api/v1/cubes/{sn}/devices/{dev}…`, the room moves,
the lock, the remove, pairing's add). `cube.RFAddress` parses six hex
digits. A refusal in the one place the routes resolve a device, plus
one in the command builders, guards both the API and netconfig.

**The warning.** serve logs at Warn what the Cube refused or sent
wrong. One line per Cube at the end of each greeting that holds the
entry is enough: it names the Cube (by its serial, as the log already
does) and says the entry is not shown.

## Implementation Plan

### Block 1: NotADevice

- `internal/protocol/cube`: `RFAddress.Zero()` (or a named constant
  `NoAddress`), with a comment that a Cube may list an entry there.
- `internal/cubelan/snapshot.go`: an entry at 000000 is not noted as a
  device (`noteLocked`), nor counted in `endGreetingLocked`'s list; it
  stays in the copy's lines as received, so that the proxy replays it.
  A C line of the Cube's type at an address other than the Cube's own
  is kept in the replay.
- `internal/view`: no device at 000000 in `Devices`, `DevicesOf`,
  `RoomsOf`.
- `internal/cubelan/client.go`: a Warn at the greeting's end where the
  copy holds the entry: "the Cube lists an entry at radio address
  000000; it is not a device and is not shown".
- Tests from captured lines (placeholder serial KEQ0000000): the view
  has no device at 000000, the proxy's replay holds the L entry and the
  C line byte for byte, the warning once per greeting.

**Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
`make race`.

### Block 2: Refused

- `internal/server`: the device lookup refuses 000000 with
  `fail(400, "radio address 000000 is not a device")`; the room moves,
  the lock, the remove, pairing's add and the virtual thermostat's
  links included.
- `internal/protocol/cube/commands.go`: the builders that take a
  device's radio address return an error for 000000.
- netconfig and the CLI go through the same builders.
- Tests: every route answers 400 and the fake Cube receives nothing;
  the builders' error.

**Verification:** `go test ./...`, `make cross`.

### Block 3: Docs

- `docs/cube-protocol.md`: the finding (2026-11-13/14, firmware 1.1.3,
  "one of the owner's Cubes"): the L entry, the C line, what Massimo
  does with them; a command to 000000 is never sent.
- `docs/api-v1.md` and the OpenAPI file: the 400.
- A CHANGELOG fragment.

**Verification:** `GUIDELINES/shared/tools/check`, `gofmt -l .`, the
three `go vet`.

## Proposed commit messages

The registration, with the other plans of the same evaluation, a commit
of its own on main.

The blocks:
Block 1, `fix(cubelan): Radio address 000000 is not a device`;
Block 2, `fix(server): Nothing is sent to radio address 000000`;
Block 3, `docs(cube): A Cube can list an entry at radio address 000000`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation of
  2026-11-13/14 and `PLAN++ as recommended`.

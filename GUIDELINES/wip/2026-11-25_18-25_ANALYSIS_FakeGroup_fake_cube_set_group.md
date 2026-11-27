# ANALYSIS FakeGroup: The fake Cube's set group and remove group

2026-11-25, session "Fix flaky tests", on the owner's
`PLAN fake cube's 0x22/0x23`.
An investigator read the protocol notes and the server's sequences,
and ran the Go tests of the fake's users with each design as an overlay;
nothing tracked was written.

## TL;DR

The fake Cube carries out no 0x22 or 0x23, so a device the server puts into
a room keeps its old room byte in its C line, takes its old room's commands,
and since AP ToldData shows a lasting room mismatch.

- 0x22 sets the C line's room byte → confirmed on hardware twice
  (2026-11-04, 2026-11-16)
- a 0x23 the device took → its effect on the byte never seen
- design (b): 0x22 sets the byte, 0x23 leaves it, its comment says why
  → matches the notes best; every server sequence ends in the same room
  under all three designs
- with (b) as an overlay → no test fails;
  `startNewcomer`'s hook that stood in for the 0x22 can go
- blocks: the fake takes the group (Go, docs, CHANGELOG);
  a UI check that a room setpoint reaches a placed device

## What a Cube does

The server builds 0x22 and 0x23 with `cube.CmdSetGroup`
(internal/protocol/cube/commands.go:537-544), an `s:` line of
11 bytes:

| Byte | Value |
|------|-------|
| 0    | 0x00 |
| 1    | flags 0x00: addressed to the device, never with the room flag |
| 2    | 0x22 SetGroupAddress, or 0x23 RemoveGroupAddress |
| 3-5  | 000000 |
| 6-8  | the device's radio address |
| 9    | the header's room, 0 |
| 10   | the group: the room's number |

The original software's lines have the same form
(`00 00 22 … 0a010e 00 04`, `… 23 … 0a010e 00 04`):
a 0x23 names the group it removes, the old room.

A C line's room byte is byte 5 of its decoded payload,
after the length byte, the address (1-3) and the type (4)
(`cube.DecodeConfig`, internal/protocol/cube/config.go:169).

What docs/cube-protocol.md has seen:

- **0x22 sets the room byte to its group: confirmed.**
  - 2026-11-04, the original software, the never-answering
    shutter contact 0a0101 (lines 774-788):
    0x22 group 1 set its C-line room to 1,
    and room 1's commands were discarded at once;
    0x22 group 6 (`arrangeNewDevice`) set it to 6.
  - 2026-11-16, Massimo, Cube firmware 1.1.3 (lines 1227-1229):
    a radiator thermostat placed from no room into its listed room,
    a wakeup and three radiograms, had its C line name the room
    18 s later.
- **A move whose device failed the 0x23 kept the old room:
  seen once.**
  2026-11-16, firmware 1.1.3 (lines 1206-1211):
  Massimo's move into an empty room sent the unlink half's four
  radiograms, its 0x23 first, then the 0x22; the Cube accepted them,
  the device was flagged ten seconds later with byte 4 0x23,
  and its C line kept the old room, which the C-line check warned of
  6 minutes later.
  The 0x22 after the 0x23 did not set the new room either:
  the device apparently took neither (a slot was still held
  12 minutes later).
  It says nothing of a 0x23 that the device took.
- **What a 0x23 the device took does to the room byte:
  not seen.**
  No C line was read between a move's 0x23 and its 0x22.
  The original software sends a 0x23 only in a move,
  before the 0x21s and with the old room's number
  (lines 584-592); its removal sends none (2026-11-15,
  lines 1151-1169).
  A removal without one (`z:`, 0x21, `t:`, `m:`) left 0a0101's byte
  at 1 (2026-11-04).
  Removing a parking room in the original software put three devices'
  bytes back to older values, 4, 4 and 5, by lines not recorded.
- **When the byte changes is not settled either.**
  It changed at once for a device that never answers (2026-11-04),
  and not for one that failed the move's 0x23 although the Cube had
  accepted the 0x22 (2026-11-16);
  for a configuration write the Cube changes the C line once the
  device acknowledged, 1 to 4 minutes on (line 1051).

`CmdSetGroup`'s comment does not say that a 0x23's effect is
unconfirmed, as the contract asks of such a command.

The fake (internal/cubelan/fakecube) keeps its C lines in three places,
read by `configRawLocked` (apply.go:409-430) in this order:
`s.configs`, the lines `SetConfig` set or a command changed;
the greeting's C lines; and `Pair.Config`, the joined device's,
in no room (`PairingFrom` sets byte 5 to 0, fakecube.go:910).
`editConfigLocked` (apply.go:447-458) writes a change into
`s.configs`, which `c:` answers with (fakecube.go:795-797) and a new
greeting carries (`liveGreetingLocked`, `withJoinedConfigLocked`);
a `t:` deletes the entry (fakecube.go:381), and a device joining again
starts from `Pair.Config`.
`applyNow` (apply.go:65-106) has no case for 0x22 or 0x23,
so a device the server places keeps its old room byte.
Its room commands go to the thermostats whose C line names the room
(`roomThermostatsLocked`), so a placed device also keeps taking its
old room's commands and none of its new room's.

## The server's sequences

O is the room the C line names before, L the room the list names,
N the new room; `[r:]` is a reset where a member is flagged.

1. **Add a device in no room**, `PUT …/devices/{rf}` with `room_id`,
   the pairing's placement too (`cubeio.Writer.AddDevice`,
   internal/cubeio/pairing.go:103-131):
   `z:N`, `[r:]`, 0x20s, 0x22 N; `m:`; `c:`;
   then the copy: 0x40; `z:`, 0x11s; `z:`, 0x12; `z:`, 0x10s, 0x11.
2. **Add an unlisted device whose C line names O**:
   `z:O`, `[r:]`, 0x23 O, 0x21s; then as 1.
3. **Move a listed device**, `PUT …/devices/{rf}`
   (`moveDevice`, internal/server/config_api.go:394-456):
   `z:L`, `[r:]`, 0x23 L, 0x21s; `z:N`, `[r:]`, 0x20s, 0x22 N;
   `m:`; `c:`; the copy.
   3'. Listed in no room: 0x23 0 without a wakeup, then the in half.
4. **The placement repair**, O not 0
   (`buildPlace`, internal/server/repair_api.go:681-723):
   `z:O`, `[r:]`, 0x23 O, 0x21s; `z:L`, `[r:]`, 0x20s, 0x22 L;
   `c:`; no `m:`.
   4'. From no room: `z:L`, `[r:]`, 0x20s, 0x22 L; `c:`.
5. **Re-link**, per member
   (`RelinkCommands`, internal/cubeio/repair.go:102-125):
   `z:R`, `[r:]`, 0x20s, 0x22 R.
6. **Unlink and re-link**, per member:
   `z:R`, `[r:]`, 0x21s, no 0x23; then 5.
7. **Removal**, `DELETE …/devices/{rf}`
   (internal/cubeio/remove.go:28-49):
   `z:R`, `[r:]`, one 0x21 with the room flag; `t:01,1,rf`; `m:`.
8. **Restore**, where the backup's room differs from the C line's
   (`planMove`, internal/backup/plan.go:530-552):
   `m:` (the names, first); `z:O`, 0x23 O, 0x21s;
   `z:N`, 0x20s, 0x22 N; then `z:O` and the configuration lines.
   8'. C line in no room: 0x23 0 without a wakeup, then the in half.
9. **Pairing**: `n:` … `x:`, no 0x22.
   Massimo has no parking room of its own; the original software's
   `arrangeNewDevice` (0x22 6, `c:`, `m:`) passes the proxy as sent.

The fake's C line after each, the 0x22 taken:

| Sequence | Today | (a) | (b) | (c) |
|----------|-------|-----|-----|-----|
| 1, 4', 8' | 0 | N | N | N |
| 2, 4, 8 | O | O, 0, N | O, N | O, 0, N |
| 3, O = L | O | O, 0, N | O, N | O, 0, N |
| 3, O ≠ L | O | O, 0, N | O, N | O, N |
| 3', O ≠ 0 | O | O, 0, N | O, N | O, N |
| 5, 6 | R | R | R | R |
| 7 | deleted; a `Keep` device R | same | same | same |
| out half taken, in half fails | O | 0 | O | 0 where O = L |
| a deaf device (`SetDeaf`) | O | O | O | O |

Every sequence ends in the same room under the three designs
wherever its 0x22 is taken.
The server reads the C line at the end (`c:`) and in the follow-up;
the state between the halves shows only in a greeting read between
them, or to a proxy client's own `c:`.
Today the fake keeps O, or 0, after every placement, move and
restore: the list says N, and the C-line check warns of a room
mismatch once the change's leave-out ends, 5 minutes after it
(`mismatchSettle`, congruence.go:81, 377-387).

## The designs

- **(a)** 0x22 sets the byte to its group, 0x23 sets it to 0.
  It ignores the group the 0x23 names, which the original software
  fills with the old room: a 0x23 naming the list's room L of a
  device whose C line names O would clear O. The furthest guess.
- **(b)** 0x22 sets it, 0x23 leaves it.
  It carries out only what was seen; the 0x23 is the one radiogram
  whose effect is not known, and the fake says so.
  A move whose in half fails leaves the device in O, unlinked there,
  as today.
- **(c)** 0x22 sets it, 0x23 sets 0 where its group is the byte's
  room, else leaves it.
  The reading of the radiogram's name and payload: the device leaves
  the group the 0x23 names. A guess: no C line between a move's
  halves was ever read.

(b) matches the notes best: it adds nothing they do not show.
(c) is the better model if a C line read between a move's halves
should show 0; until then it is a guess.
The deaf device keeps its byte under each, as on 2026-11-16.
That the byte may change without the device's answer, as for 0a0101
on 2026-11-04, the fake does not model: `-apply-after` delays it as
an acknowledgement would.
Hardware could settle the 0x23: a 0x23 alone to one device and a `c:`
after it, then its 0x22 back; two radio writes, the device out of its
group meanwhile, so only on the owner's word.

## What changes

A scratch overlay (`go test -overlay`) gave `applyNow` a case for each:
0x22 writes the group into byte 5 of the C line (`editConfigLocked`,
unless it names it already); 0x23 is (a) 0, (b) nothing, (c) 0 where
its group is the byte's room. A deaf device takes neither.
It added five fakecube tests and two server tests (Blocks, below).

`make ui`, then with `MASSIMO_ENV_FILE=/dev/null`
`go test -count=1 ./internal/cubelan/... ./internal/server
./internal/proxy ./internal/cubeio ./internal/backup ./internal/engine
./internal/serve ./cmd/massimo`:

| Run | Exit | Failed |
|-----|------|--------|
| without the overlay | 0 | none |
| (b) | 1 | two discovery tests (below) |
| (b) again | 0 | none |
| (c) | 1 | `TestApplyRemoveGroupLeavesTheRoom`, the new test of (b) |
| (a) | 1 | the same |
| the new tests, today's fake | 1 | three fakecube, both server |

The discovery tests (`TestDiscoveryHearsAServedCubeOnThisHost`,
`TestOneSerialFromACubeAndAProxy`) passed five times on their own
with (b), in the second run of (b) and in (a) and (c): another
session's `serve` with discovery ran on the host meanwhile.

No existing test fails under any design.

What behaves differently: a tracing variant of (b), run with
`-v -parallel 1` over the server, cmd/massimo, proxy, cubeio, backup
and serve (exit 0), printed every group radiogram the fake took, by
test. Sixteen reached a fake that applies (the rooms fixture's), all
in internal/server/config_api_test.go:

- Twelve 0x22s to 0a0201 over `startNewcomer`, in seven `TestCopy…`
  tests (two in `TestCopyTakesANewerSetpoint`) and
  `TestRepairPlaceHoldsTheRoomsState`: each found the room set already
  by the test's hook, so the fake changed nothing.
- `TestMoveGivesTheRoomsState`: 0x23 2 to 0a0204, then 0x22 1, which
  moves its C line from room 2 to room 1 (today it stays in 2): it is
  room 1's member for the room checks and the fake's room commands
  from then on. The test asserts the copy's lines and the hold, the
  same either way.
- `TestHoldOnlyOnceTheLinkWentThrough`, "a move whose link radiogram is
  discarded": 0x23 2 to 0a0204, and no 0x22, its 0x20 discarded.
  Under (b) the C line stays in 2, as today; under (a) and (c) it
  would end in no room. The test asserts the hold alone.
- `TestCopyLeavesTheNewcomerOutOfTheRoomsState`: 0x23 1 to 0a0201,
  in room 1 by its C line, then 0x22 1. Under (b) it stays in 1;
  under (a) and (c) it passes through 0.

None reached a fake that applies in cmd/massimo, proxy, cubeio, backup
or serve: their fakes do not set `Apply`.

The one test that stands in for the missing group is
`startNewcomer` (internal/server/config_api_test.go:1123-1126,
1155-1162): an `OnReceive` hook gives 0a0201's C line the room of every
0x22 it sees, a discarded one too, rebuilt from the greeting's line.
Without the hook, today's fake fails `TestRepairPlaceHoldsTheRoomsState`
and `TestCopyKeepsTheRoomsManualSetpoint` (the room checks never take
the newcomer as room 1's); with (b) and without the hook, all eleven
tests over `startNewcomer` pass, three times.
No test expected the mismatch: `TestMismatchLeftOut` and the other
congruence tests write the C lines into the greeting themselves.

Under (b) the fake also changes for every client:

- a placed or moved device takes its new room's commands, and no longer
  its old room's (`roomThermostatsLocked`);
- the server's C-line check finds no room mismatch after a placement,
  a move, a placement repair or a restore, and the leave-out of the
  structure change ends at the first check after the `c:`, not
  5 minutes later;
- the room's third tier then judges the device as a member: a device
  put in without the copy can be warned of for its programme or
  temperatures, once the follow-up of its write has ended.

The UI suite: only test_pairing.py places a device against the fake:
`test_pair_put_into_a_room_and_remove` (with the copy) and
`test_put_into_a_room_from_devices_without_a_room` (without) put the
joined 0abcde into room 1; `test_remove_from_devices_without_a_room`
deletes it unplaced. Each removes the device at its end (`t:` deletes
its C line, and it joins again in no room), well before the 5 minutes
after which today's fake would show a room mismatch; what they assert
reads the list, so none sees a difference.
No UI test sees a room mismatch from the fake: test_rooms.py's
`test_mismatch_marks` and test_log.py's `test_log_mismatch_lines` stub
the findings, test_repair.py stubs the repairs, test_unsaved.py's move
aborts its PUT, and test_room_updates.py's put form sends nothing.
The UI greeting's devices are all valid, so 0abcde in room 1 changes
no room's readiness.

## Blocks

One functional change, design (b):

**Block 1. The fake Cube takes the group.**

- internal/cubelan/fakecube/apply.go: a case for 0x22 in `applyNow`,
  `setGroupLocked` writing byte 5 (a named offset) through
  `editConfigLocked`; 0x23 named in the file's comment as not applied,
  its effect unseen; the comment's list of what is applied.
- Tests, internal/cubelan/fakecube: a 0x22 sets the room in `c:` and
  in a new greeting, and moves the room commands (`TestApplyGroup`);
  a 0x23 leaves it (`TestApplyRemoveGroupLeavesTheRoom`); a move's
  lines end in the new room (`TestApplyMoveLines`); a deaf device
  keeps it (`TestApplyGroupDeaf`); the joined device takes it, in
  `c:` and a new greeting, and a `t:` and a new join put it back in
  no room (`TestApplyGroupOfTheJoinedDevice`).
- Tests, internal/server: a move leaves the C line in the new room,
  no line finding, and its leave-out ended at the next check; an add
  of a device in no room leaves its C line in the room. The
  `startNewcomer` hook and its comment go.
- internal/protocol/cube/commands.go: `CmdSetGroup`'s comment says
  what is confirmed (0x22 sets the C line's room byte) and what is
  not (a 0x23's effect).
- cmd/massimo/fakecube.go: `-apply`'s usage names the group.
- Docs: README.md's fakecube text, the sentence that a placed
  device's C line keeps room 0 replaced, the group in the list of what
  the fake carries out; docs/cube-protocol.md, beside the experiment
  of 2026-11-04 (lines 774-788): what a 0x23 the device took does to
  the room byte has not been seen, and the fake leaves it.
- CHANGELOG.d/Fixed/claude-fake-group.md:

  ```
  - `massimo fakecube` takes the set group radiogram (0x22): a device
    put into a room or moved names that room in its C line, so no room
    mismatch follows (README.md).
  ```
- Verification: `make ui`; the test set above and `go test ./...`;
  `gofmt -l .`, `go vet` for linux, windows and darwin; the guidelines
  check.

**Block 2. The UI suite sees the room.**

- tools/ui-test/test_pairing.py,
  `test_put_into_a_room_from_devices_without_a_room` (no copy): after
  the put, a room setpoint to room 1 reaches 0abcde's state, which needs
  its C line in room 1; today's fake leaves it out.
- The four cases share the server: the test puts room 1's state
  back at its end.
- Verification: that test alone, both languages (`PYTEST_ADDOPTS`),
  as the gate asks for a test file changed.
- No CHANGELOG fragment: tools/ is not shipped.

Left open: docs/roadmap.md item 12 names the fake's 0x22 and 0x23;
the plan's session prunes it at the retirement, keeping the 0x23
until a C line read between a move's halves shows what it does.

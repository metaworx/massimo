# ANALYSIS FakeGroup: The z: wakeup and r: straight to the test Cube

2026-11-26, session "Fix flaky tests" (fork for AP FakeGroup),
on the owner's `EXEC repeat D18-A`, the Windows service stopped.
The test Cube (firmware 1.1.3), one radiator thermostat alone in room 7;
each command on a new LAN connection of its own, each reading from the
`H:` line of another short connection (duty cycle and free slots,
no radio time). Each probe re-set the thermostat's own room (0x22).

## TL;DR

- with a new connection for each command, no wakeup kept the thermostat
  awake: the first radiogram after `z:1e`, `z:05` and `z:3c`, at 12 to 17 s,
  cost 5 to 6 points each
- through the service's proxy (one standing connection to the Cube),
  `z:1e` had kept it awake past 45 s → a wakeup appears to end with the
  LAN connection that sent it, or at the next client's greeting
- a radiogram to the sleeping thermostat cost 11 points alone, 9 with two
  `r:` lines while it was pending; both slots back within 14 s
  → `r:` does not make the Cube send it again
- the Cube's duty cycle held at 46 % for the 30 minutes read:
  it counts the air time of the last hour, an entry leaving the count an
  hour after it was spent
- the value's effect on the window is still open: it needs one standing
  connection

## The runs

| Time | Line | Duty before → after | Slot |
|---|---|---|---|
| 22:03:14 | `z:1e,G,07` | 0 → 3 | |
| 22:03:31 | 0x22, 17 s after | 3 → 9 | back after 5 s |
| 22:07:53 | `z:05,G,07` | 9 → 12 | |
| 22:08:05 | 0x22, 12 s after | 12 → 17 | back after 5 s |
| 22:12:23 | `z:3c,G,07` | 17 → 20 | |
| 22:12:40 | 0x22, 17 s after | 20 → 26 | back after 5 s |
| 22:17:03 | 0x22, no wakeup, no `r:` | 26 → 37 | back within 14 s |
| 22:22:33 | 0x22, no wakeup, `r:` at 4 s and 8 s | 37 → 46 | back within 9 s |
| 22:23:51 to 22:53:58 | readings every 2 minutes | 46 → 46 | 50 |

## What it shows

**A wakeup and the LAN connection.** Here each wakeup went on a connection
that closed at once, and the radiogram came on a new one: it found the
thermostat asleep every time, whatever the value.
Through the service's proxy the wakeup and the radiograms went on the
service's one standing connection, and the thermostat heard radiograms
cheaply for 45 s after `z:1e`.
The likeliest reading: a wakeup lasts only while the connection that sent
it stays, or a new client's greeting ends it. Massimo's server and the
original software each keep one connection, so their wakeups hold;
`massimo cube`, a connection per call, cannot use one.

**`r:` and a pending radiogram.** Two `r:` lines while a radiogram was
pending added nothing: 9 points against 11 without.
So the 59 points of 20:18 (through the proxy, `r:` every 2 s) came from the
radiogram staying pending for 56 s, not from `r:` re-sending it;
why it stayed pending so long is not known.

**The count.** No reading fell in 30 minutes, although nothing was sent:
the figure is the air time of the last hour, and the first of today's
22:03 entries leave it at 23:03.
That also explains the service's 66 % from 20:23 to about 21:20.
The earlier v2 analysis called this reading a guess; it holds.

## Still open

- Whether the wakeup's value sets its window, and in what unit: a series
  on one standing connection, the readings from `r:` answers,
  `z:05` against `z:1e`.
- Why one radiogram stayed pending 56 s at 20:18.

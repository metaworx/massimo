# ANALYSIS FakeGroup: The z: wakeup series on the test Cube (v2)

2026-11-26, session "Fix flaky tests" (fork for AP FakeGroup),
on the owner's `EXEC D17-A now`.
The test Cube (firmware 1.1.3), through the proxy of the owner's Windows
service, on one connection; one radiator thermostat alone in room 7.
Each probe was a harmless 0x22 re-setting the thermostat's own room;
between probes an `r:` for the thermostat read the duty cycle and the free
slots every 2 s (`r:` for the empty address 000000 got no answer).

## TL;DR

- `z:1e,G,07`: radiograms at 15, 30 and 45 s cheap (duty +0 to 1, slot back
  in 3 s); at 60 s expensive (+11, slot not back within 13 s)
- with the earlier run (cheap at 70 s, expensive at 4 min):
  the wakeup lasts about 45 to 70 s → `1e` is neither 30 s nor 30 min;
  units of 2 s (60 s) fit both runs best
- radiograms every 15 s did not keep the thermostat awake past the window
- after the expensive radiogram, with `r:` lines every 2 s, the Cube kept
  sending it for 56 s and spent 59 points of duty cycle (4 → 63 %);
  without `r:` polling (19:10) the same took 9 → the series stopped at its
  60 % limit; `z:05` and `z:3c` were not measured
- v2: the service's log, read afterwards, puts the cost in those 56 s
  and shows no radio command of the service's own after the run

## The run

| Time | After the wakeup | Line | Duty | Slot |
|---|---|---|---|---|
| 20:17:39 | 0 s | `z:1e,G,07` | 0 → 3 % | |
| 20:17:54 | 15 s | 0x22 | 3 → 3 | back after 3 s |
| 20:18:09 | 30 s | 0x22 | 3 → 4 | back after 3 s |
| 20:18:24 | 45 s | 0x22 | 4 → 4 | back after 3 s |
| 20:18:39 | 60 s | 0x22 | 4 → 15 | not back within 13 s |
| 20:18:41 to 20:19:35 | | `r:` every 2 s, nothing else | 4 → 63 | 49, at times 48, back to 50 at 20:19:35 |
| 20:19:35 to 20:22:54 | | `r:` every 2 s | 63 | 50 |
| 20:22:54 | 0 s | `z:05,G,07` | | |
| 20:23:05 | 10 s | 0x22 | 66 → 66 | back after 3 s; the run stopped at 60 % |

The thermostat ends in room 7 without an error flag (20:23:56).

## What it shows

**The window.** In this run the thermostat heard radiograms cheaply up to
45 s after the wakeup and needed the long wake-up burst at 60 s.
In the run of 19:06 a radiogram at 70 s was cheap and one at 4 minutes cost
9 %. So the window ends between 45 and 70 s, measured from the Cube's `A:`;
the thermostat hears the wakeup some seconds after it, which may explain the
spread. `1e` = 30 in units of 2 s, 60 s, fits both runs;
seconds (30 s) and minutes (30 min) fit neither.

**Radiograms do not prolong it.** Three radiograms 15 s apart inside the
window did not keep the thermostat awake at 60 s.

**The cost after a radiogram to a sleeping device.**
The service's log (each `r:` is one acknowledged command there, with the
Cube's duty and free slots) shows the rise in 56 s, 20:18:41 to 20:19:35:
4, 7, 9, 12, 15 … 60, 63 %, the free slots at 49 and now and then 48,
then 50 and the rise ending.
In the run of 19:10 an expensive radiogram followed by refreshes alone
(no `r:`) cost 9 points, its slot back within 20 s.
The likeliest reading: an `r:` for a device whose radiogram is still
pending makes the Cube start sending it again (the second slot at 48);
not proven. Either way, a radiogram to a device that is not awake can
cost far more than its 1 % share, and an `r:` should not be sent while one
of its radiograms is pending.

**The 66 % afterwards** is the Cube's rolling hour: the service sent
nothing by radio after the run (its log: a configuration re-read at 20:28
and a reconnect at 21:13, both over the LAN), and the 59 points of
20:18:41 to 20:19:35 leave the hour at about 21:19 to 21:20.

## Not settled

- Whether the value sets the window (`z:05` at 10 to 30 s, `z:3c` at
  90 s to 3 min would tell); a run after the duty cycle has fallen,
  without `r:` polling while a radiogram is pending.
- Whether `r:` re-sends a pending radiogram (an `r:` once, at a pending
  radiogram, against none).

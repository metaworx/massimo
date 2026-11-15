# NOTE LegacyLogs v1.0: how blocks 6 to 13 went

2026-11-15, the runs of `EXEC++ Blocks 6–8` (PR #291) and `EXEC++ Blocks 9–13` (branch `claude/legacy-logs-state`, stacked on #291's). Where the work went differently from the plan:

## Block 6, the room-command fallback

- **The room's week programme too.** It names its device the same way as the room setpoint, in place of a fallback of its own that helped only where the master was 0.
- **The order, by the owner's D167 and D169-A** (AP DeviceUnPairingFix, its plan on main):
  - the master where it is a thermostat of the room;
  - else the room's wall thermostat;
  - else its first radiator thermostat.

  The plan had the two fallbacks the other way round. #291 was changed in "fix(server): A room command falls back to the wall thermostat first", before it was merged.
- **The function moves later:** AP DeviceUnPairingFix's Block 2 moves it into `internal/cubeio`, shared with removals and moves.

## Block 7, the names

- **Its renaming commit carries a CHANGELOG bullet.** `massimo decode`'s JSON names change with it (`flag_04`, `auto_dst`), and every commit touching shipped code owes one.

## Block 9, the duty hold

- **The budget is read again by a reconnect,** which costs no radio time. The command is sent anyway only where the gateway cannot reconnect.
- **The setting `duty_wait` applies at the next start,** as CubePoll's intervals do. It has no flag.

## Block 10, the stale thresholds

- **The two thresholds are a `view.Staleness`** passed through the views in place of one duration.
- **`-stale-after` sets both.**

## Block 11, the battery levels

- **The weak notification waits 15 minutes,** so that a wall thermostat, which goes from weak to low within them, gets `battery_low` alone.
- **Two delivery tests changed.** They followed a notification from the fake Cube to the user, using `battery_low`, which is now told only after minutes. They use a device error instead. The battery's own path is tested in `internal/notify` and in the server's battery test.
- **The full UI suite caught a count:** the notification settings list one more kind. Fixed in "test(ui-test): The notification rules count the weak battery's row".

## Block 13, out-of-date entries

- **The frozen values are `last_known` beside `state`.** The plan had them in `state` itself, marked, but the virtual wall thermostat, the rooms' readiness and the battery levels read a present `state` as a reported one.
- **`state` stays absent** for such a device, and `out_of_date: true` marks it.

## Outside the blocks

- **A test flake on #291's CI** (armv6 under QEMU): the proxy discovery test's two "free" ports were one. Fixed in "test(server): The proxy discovery test takes two distinct free ports".

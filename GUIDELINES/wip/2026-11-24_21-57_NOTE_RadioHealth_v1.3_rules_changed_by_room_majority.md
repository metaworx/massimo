# NOTE RadioHealth v1.3: rules changed by room majority

AP RoomMajority (merged with #410, retired on 2026-11-24, `chore(aps): -RoomMajority`)
changed three of this plan's rules.
The code and `docs/api-v1.md` follow the rules below;
this plan's text, where it states the old ones, is to be read with this NOTE.

## The one order (RoomMajority.D9)

Wherever one thermostat stands for its room,
one order decides it (`internal/roomorder`):

1. a thermostat the Cube holds no valid state for is passed over at every step;
2. what more than half of the room's thermostats share;
3. the room's master, where it is a thermostat of the room;
4. the room's wall thermostat;
5. its first radiator thermostat, then each other one, in the Cube's order.

## D188, the room checks' re-send

Its order is now:

1. Massimo's own room command from the last 15 minutes, as before;
2. the state the room's thermostats showed before a structure change brought a
   newcomer into it, held for the room
   (taken by `PUT …/devices/{rf}` and the repair's Place);
   otherwise what more than half of the thermostats show;
3. on a tie, the state of the thermostat whose mode or setpoint someone changed
   last; a switch point counts only against thermostats in auto,
   and a first sighting, a boost's end or a holiday's end at its until is
   no one's change;
4. the master's, the wall thermostat's, the radiator thermostats' state,
   the first with one;
5. none.

The hold is dropped when the thermostats agree, at a later change by hand to
another state, at a newer room command of Massimo's other than the held state,
after 24 hours, and once a held holiday has ended.
Its one re-send is exempt from the once-a-day limit.
`from` gains `newest`, `master` and `radiator_thermostat`,
and `before_change: true` marks a held state.

## D163-A, the configuration's repair

It re-sends what the report names: the room's configuration over its thermostats,
the device's own included, not over "the others".
Each group (the temperatures, the valve settings) and each day of the programme
is what more than half share, else the first of the order's.
It refuses only a room without another thermostat with a valid state,
and re-sends only the days that differ.

## Block 7's report of members that differ (D162-A, third tier)

The report takes the same room-level configuration and compares the programme
day by day.
While a thermostat of the room has not reported since the Cube's restart,
every finding of the room's thermostats stays as it was, neither found nor cleared.
A thermostat the Cube marks out of date is passed over and keeps its finding,
as do those of a room with fewer than two thermostats with a valid state.

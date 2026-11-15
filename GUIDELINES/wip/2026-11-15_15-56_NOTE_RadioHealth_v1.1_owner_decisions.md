# NOTE RadioHealth v1.1: owner decisions

The owner's answers to the plan gate of AP RadioHealth v1.1
(`wip/2026-11-15_12-58_AP_RadioHealth_communication_errors_and_resets.md`),
2026-11-15, and the evaluation they asked for.

## Settled

- **D102-B**, with a warning: a flagged device that reports gets one `r:`,
  at most once a day; if its flag is back by its next report, or later
  that day, a Warn, and no automatic reset for it until it stays clear for
  a day; an Error if it is still flagged after 24 hours.
  The flag is set only by a failed radiogram of the Cube, so a flag back
  by the next report means the Cube failed again since the reset.
- **D110-A:** one `r:01` per flagged member of the numbered room before a
  room command.
- **D111-A:** 0x08 only.
- **D164-A:** checks and repair in this plan.
- **D165-A:** C lines and room masters against the metadata at every
  greeting, C line and metadata write.
- **D166-A:** a new `repair` capability, the owner's and `manager`'s, in
  the integration role's ceiling; "unlink all and re-link" needs `repair`
  and `structure`.

## The owner's question on D102: a job queue?

None is needed. The owner asked whether D102 needs something checked at a
given moment, set 24 hours ahead, or a check at every incoming report.

- The reset, and the warning when the flag comes back, are checked at
  every incoming report of the device: `serve` polls each Cube every 10 s,
  and a changed entry is an event.
- "Still flagged after 24 hours" is a timer keyed by the device, set when
  the flag is seen and cancelled when it clears: the notify engine's
  `delay` and `cancel` (`internal/notify/rules.go`) work that way already,
  and its periodic `scan` covers a device that has gone silent.
- The times (flagged since, last reset) are kept per device in memory and
  seeded at `serve`'s start from the store's log of flag changes, so a
  restart does not set the clocks back.

## Open, as recommended in the evaluation

- **D157-B, with conditions:** an automatic re-link of a room whose
  member's 0x40 lasts:
  - only when every thermostat of the room reported within 15 minutes
    and no member has 0x08 (window contacts left out; a silent partner is
    reported as such instead, which a re-link cannot fix);
  - no wakeup to probe: it makes devices listen, not report;
  - once per room and day, a Warn when it acts, marked seen when cleared;
  - cleared: the slots back at their count before it, then every member
    that had 0x40 without it at two reports in a row, and no new 0x08;
  - an Error when 0x40 lasts 24 hours, repaired or not;
  - only after Block 11's manual re-link passed on the spare Cube and on
    hardware, its setting off until then.

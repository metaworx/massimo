# NOTE RadioHealth v1.2: decisions of blocks 5 and 7

The questions Blocks 5 and 7 of AP RadioHealth v1.2
(`wip/2026-11-15_12-58_AP_RadioHealth_communication_errors_and_resets.md`)
raised on 2026-11-15, each written as recommended; the owner decides.
They follow D180 to D191
(`wip/2026-11-15_22-04_NOTE_RadioHealth_v1.2_owner_decisions_of_the_run.md`).

- **D192. A reconnect for Block 5's second reading.** A Cube's free slots
  are read from S replies and from the H line of each greeting; the
  second reading an hour after the first can come only with the periodic
  refresh (6 hours by default), so the warning can come that late.
  - **A (recommended):** no reconnect of its own; the slots stayed held
    for months, so a warning hours late is acceptable.
  - **B:** reconnect once, an hour after the first full reading (no
    radio time).
- **D193. The original software's room-list writes through the proxy**
  get no quiet minutes in Block 7, as Massimo's own structure changes do:
  a move made with the original software warns, then clears within
  minutes.
  - **A (recommended):** the same quiet minutes, keyed on the proxy's
    `m:` writes, a small follow-up.
  - **B:** leave it.
- **D194. The UI test fixture's window contact has an L entry and no C
  line,** so Block 7 reports a listed device without a C line only where
  the Cube has no L entry for it either (on hardware, a device the Cube
  reports always has its C line).
  - **A (recommended):** give the fixture its C line, in a change of its
    own run against the full suite, and the check the plan's wording.
  - **B:** keep the narrower rule.

# NOTE RadioHealth v1.2: owner decisions of blocks 5 to 7

The owner's answers of 2026-11-15, under `EXEC`, to the decisions still
open in the run of AP RadioHealth v1.2
(`wip/2026-11-15_12-58_AP_RadioHealth_communication_errors_and_resets.md`):
D181 and D189 to D191
(`wip/2026-11-15_22-04_NOTE_RadioHealth_v1.2_owner_decisions_of_the_run.md`),
D192 to D194
(`wip/2026-11-15_23-02_NOTE_RadioHealth_v1.2_decisions_of_blocks_5_and_7.md`),
and how each is read.

- **D181, a third way:** the owner: "the most severe one, then the
  oldest one". The flag alert names, of the radiograms that failed while
  the flag stayed set, the most severe, and of two as severe the oldest.
  Read as severity, highest first, by the original software's categories
  (`docs/cube-protocol.md`, "FrameType"): Assignment and Remove (0x20 to
  0x23, 0xF0), which change the system's structure; then Configuration
  (0x10 to 0x12, 0x82); then Control (0x40, 0x41, 0x43, 0x44, 0x80);
  then the rest (the time push 0x03, the wakeup 0xF1, codes the list
  lacks).
- **D189-A, D190-A, D191-A.**
- **D192-A, with new thresholds:** no reconnect of its own; the owner:
  "not required. user can set this now in maintenance settings" (the
  refresh interval). "I would make the threshold of empty slots to be <5
  for 1hr for a warning, <2 an error." Read as: free slots below 5 at two
  readings an hour apart, a Warn; below 2 at two readings an hour apart,
  an Error; each once, and an Info once they are back at 5 or more.
- **D193-B:** left as it is; the owner: "such a user probably knows what
  they do."
- **D194-A:** the UI test fixture's window contact gets its C line, and
  Block 7 reports a listed device without a C line as the plan says; the
  full suite runs in CI.

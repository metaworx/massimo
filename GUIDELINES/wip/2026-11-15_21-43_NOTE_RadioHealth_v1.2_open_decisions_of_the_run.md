# NOTE RadioHealth v1.2: open decisions of the run

The questions the run of AP RadioHealth v1.2
(`wip/2026-11-15_12-58_AP_RadioHealth_communication_errors_and_resets.md`)
raised on 2026-11-15, under the owner's `EXEC RadioHealth` and "the entire
AP is due". The blocks were built by sub-agents and are in pull requests
#309 (Blocks 1 and 3, merged), #311 (Blocks 2 and 4, merged), #312
(Blocks 10 to 12) and #313 (Block 9). Each decision is written here as
recommended; the owner decides. How the run went is a NOTE of its own, at
its end.

- **D180. The `device_error` notification still says "reports an error"
  for both flags** (Block 3).
  - **A (recommended):** leave it, and decide it separately.
  - **B:** name "communication error" and "link error" in its texts.
- **D181. The flag alert's `failed_radiogram`** (Block 1) is the device's
  current one, not that of the line that set the flag.
  - **A (recommended):** keep the current one.
  - **B:** take the one of the line that set the flag.
- **D182. A radio line sent directly to a flagged device that is not the
  command's Target** (the room configuration write's per-device lines, the
  copy steps) gets no `r:` first (Block 2); the original software resets
  every flagged device in a command's scope.
  - **A (recommended):** reset it too, a small follow-up of Block 2.
  - **B:** leave it; `docs/testing-writes.md` keeps it as a known limit.
- **D183. A command of `r:` lines only waits behind the free-slot hold**
  (Block 4), although an `r:` takes no slot: below 5 free slots an
  automatic reset waits 5 minutes.
  - **A (recommended):** leave it as the plan says; at most once a day per
    device.
  - **B:** let `r:`-only commands pass the slot hold.
- **D184. The duty cost of a repair's preview** (Block 11) is 1.25 % per
  radiogram; `docs/cube-protocol.md` records more for some (0x10 one day
  +8 %, 0x11 +3 %).
  - **A (recommended):** keep 1.25 % until the spare-Cube run measures it.
  - **B:** per-command costs now.
- **D185. Listing a device into a room the list lacks** (Block 11) needs a
  name from the user.
  - **A (recommended):** keep that.
  - **B:** make one up, such as "Room N".
- **D186. Merging #312 before the spare-Cube run** of the re-link (Block 11
  item 4) and "unlink all and re-link" (Block 12).
  - **A (recommended):** merge; nothing in it runs by itself, and the docs
    say the two are untested on hardware.
  - **B:** hold #312 until the run has passed.
- **D187. A wall thermostat back from new batteries** (Block 9) comes back
  in auto (`wall_reset`) while the radiator thermostats keep a manual
  setpoint; the room's re-send then takes the wall thermostat's state,
  which puts the room back on its programme.
  - **A (recommended):** don't take a wall thermostat's state for the
    re-send when it had a `wall_reset` during the disagreement; only
    report it.
  - **B:** keep it as built.
- **D188. How long a setpoint of Massimo's counts as the room's intended
  state** (Block 9): 15 minutes before the disagreement began, so that a
  person's later wheel turn is never overridden.
  - **A (recommended):** keep 15 minutes.
  - **B:** longer, which catches more battery cases but can override a
    person's change.

Not built in this run: Block 8 builds on AP RoomConfirm's Block 1
(`wip/2026-11-15_20-17_AP_RoomConfirm_writes_confirmed_by_the_devices.md`);
Block 13 waits for Block 11's re-link to pass on the spare Cube and on
hardware.

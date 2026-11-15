# NOTE RadioHealth v1.2: owner decisions of the run

The owner's answers of 2026-11-15 to the open decisions of the run of AP
RadioHealth v1.2
(`wip/2026-11-15_12-58_AP_RadioHealth_communication_errors_and_resets.md`),
which `wip/2026-11-15_21-43_NOTE_RadioHealth_v1.2_open_decisions_of_the_run.md`
lists, and how each is read. D189 to D191, raised by Block 6, are listed
here too, still open.

- **D180-B:** the `device_error` notification names the flag: the
  communication error, or the link error.
- **D181:** asked again, with an example (below).
- **D182-A:** a radio line sent directly to a flagged device that is not
  the command's Target gets an `r:` first too, as the original software
  resets every flagged device in a command's scope.
- **D183-B:** a command of `r:` lines only passes the free-slot hold.
- **D184-A:** a repair's preview keeps 1.25 % per radiogram until the
  spare-Cube run measures it.
- **D185-A, with a default:** the owner: "legacy uses '<type> <n>' as
  default. I would use '<room_name> <type> <n>' as default". Read as:
  a room the list lacks is still named by the user; a device the list
  lacks gets "<room name> <type> <n>" as its name, n the next number free
  among that room's devices of the type, which the user may change.
- **D186:** moot, #312 is merged.
- **D187-B:** kept as built. The owner: "if someone changes the batteries,
  they can also verify if it's on auto or manu or whatever."
- **D188-A, refined:** the owner: "15' unless there it was on auto then
  and the programme changed in those 15 minutes. then it would be the
  programme's new value. or use the room's majority." Read as the room
  checks' re-send, in this order:
  1. Massimo's setpoint to the room within 15 minutes before the
     disagreement began, or during it; where that was auto mode, auto
     again, which takes the programme's value now, also past a switch
     point within those 15 minutes;
  2. else the state most of the room's thermostats show (more than half);
  3. else the wall thermostat's state;
  4. else only the report.

**D181, again:** the flag alert of `GET /api/v1/log/alerts` points at the
log line that set a device's communication error and stays until settled.
The device's byte 4 can change while the flag stays set: at 10:00 the
Cube's time push (0x03) fails and sets the flag; at 14:00 a setpoint
(0x40) fails too, the flag still set, byte 4 now 0x40.
- **A, as built:** the alert names the radiogram that failed last
  (ControlMode, 0x40), as the device shows it now.
- **B:** the alert names the one that set the flag (TimeInformation,
  0x03), as its log line does.
Either way, the log keeps both: the line at 10:00 names 0x03, the line at
14:00 "last radiogram that failed: ControlMode (0x40)".

**Still open, from Block 6** (each written as recommended):

- **D189. The clear's Info settles the Warn and the Error.**
  - **A (recommended):** yes, as built; as "marked seen if … it clears"
    (D157). A person can still mark them.
  - **B:** only a person's mark.
- **D190. The kind's name in the log, "error watch".**
  - **A (recommended):** keep it; it covers both flags and the three lines.
  - **B:** another name.
- **D191. A device without a valid state since the Cube's restart keeps
  its 24 hours across a `serve` restart.**
  - **A (recommended):** not for now; it needs its flag kept in the log in
    another form.
  - **B:** keep them, with that record.

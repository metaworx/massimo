# Roadmap

Owner's priorities as of 2026-11-04 evening, after a day on real
hardware. The items are in order; the first ones are the next session.

Done the same night (phase-0 up to the merge of UI round five): the
configuration API (names, moves, device and room configuration, week
programmes), the UI panels for them with the week programme editor and
copy-to-rooms, the log view, Cube names from `.env`, and the UI details
of item 3. What remains of items 1 to 3 is hardware verification of
each new write path and the feedback that follows.

1. **Room and device configuration in the API and the UI.** The
   original UI's Configuration menu per room: Temperatures (eco,
   comfort, window-open temperature, maximum), Device Settings (boost
   duration and valve opening, descaling day and time, temperature
   offset per device), Structure (room names, device names, delete
   rooms, delete devices, move devices), Holiday mode, and Copy
   configuration to other rooms. The protocol builders (0x11, 0x12,
   metadata) and the restore planner already do all of this; what is
   missing is API endpoints (PUT per room or device, metadata writes
   for names) and inline panels in the UI. Device name and offset first.
2. **Week programme per room**: edit one day, all days, or a selection
   of days, with a copy-day function, in the UI and the API (0x10 per
   day, builders and planner exist).
3. **UI details**: IS and SET temperatures at the same size in the tile
   ("IS" is the measured temperature: always from a wall thermostat,
   only occasionally from a radiator thermostat, which reports it with
   some status messages), the stepper buttons on one side as up/down
   chevrons, a HOLIDAY button next to AUTO and MANU with the end date on
   hover (suitcase as in the original), a manual refresh button, the
   "House…" button labelled MANU, expert details with firmware, IP
   address and radio address of the Cube, the time of the last poll
   with seconds, the Expert switch out of the header (it lives in the
   settings panel).
4. **Pairing** new devices through the server and the UI (`n:`/`x:`,
   the sequence in portal-protocol.md), including through the proxy.
5. **Firmware dump** of Cube 1 once the FT232H is in (docs/
   firmware-extraction.md), then the DataFlash, then the portal reply
   format and the update encryption from the code.
6. **Users and a database**: owner arbitration per Cube, settings such
   as the ignore list editable in the UI, SQLite.
7. Internationalisation of the UI with a translation table, then the
   languages of the original software: German, English, French,
   Italian, Dutch, Polish.
8. Later: Loxone, a radio-stick driver without a Cube.

Open UI feedback from 2026-11-04 is listed in handover-session.md.

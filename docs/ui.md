# Using the web UI

`massimo serve` serves the web UI at its listen address, by default
<http://127.0.0.1:8099/>. A box installed with `massimo install` serves it
on the address its `-listen` names. The page works on a phone as well
as on a desktop, and follows the system's light or dark mode.

When the server has an API token (`MASSIMO_API_TOKEN`), the page asks for
it once and keeps it in the browser. Settings → Forget token removes it
again.

## Header

- **Cube**: chooses the Cube when the server has several. Next to it you see
  whether the server is connected to that Cube.
- **The Cube's clock** and the time of the last update.
- **↻** asks the server for the current state now.
- The three page buttons switch between **Rooms**, **Log** and **Settings**.
- When the server was updated, a button offers to reload the page. The page
  reloads by itself only when nothing is open: no setting being edited, no
  panel and not the settings page.
- In expert mode the header also shows the Cube's details and its radio
  budget. The duty cycle and the free memory slots limit how many radio
  commands the Cube can send.

## Rooms

Every room has a tile:

- **IS** is the measured temperature: the wall thermostat's, else the
  first radiator thermostat's that reported one.
- **SET** is the set temperature. The up and down chevrons next to it
  change it in steps of 0.5 °C; the value is sent 2 seconds after the last click, or at once
  with **Save**; **Cancel** drops it. A temperature set this way puts the
  room into manual mode.
- **AUTO** follows the week programme. **MANU** keeps the current
  temperature. **HOLIDAY** keeps a temperature until a date and time.
- **"mixed"** next to the mode buttons means the room's thermostats are in
  different modes, for example after another program set one of them. The
  modes in use are outlined, and the note lists each device's mode.
- The **⋯** menu locks or unlocks the buttons on the room's thermostats, and deletes
  a room that has no devices.
- The room name or the **i** button opens the device list: every device with its
  values (valve, set and measured temperature, offset) and badges for
  battery, transmission errors, lock, window and mode.
- The cog button opens the room's configuration:
  - **Room**: the name.
  - **Temperatures**: comfort, eco, open window and its duration, maximum
    and minimum. For radiator thermostats also Boost, descaling, maximum
    valve and valve offset.
  - **Devices**: name, room, temperature offset, and for a wall thermostat
    whether its display shows the measured temperature.
  - **Week programme**: one bar per day.
    - A click on a day selects it together with every day that has the
      same programme.
    - With **Only {day}** on, changes apply to that day alone. Off, they
      apply to all selected days.
    - **Copy {day} to …** copies the day to the days you choose.
    - **Reset day** and **Reset all** go back to what the Cube has.
    - **Send** writes only the days that changed.
  - **Copy**: copies this room's stored settings (temperatures, week
    programme, radiator settings) to other rooms. It copies what is
    stored, not changes that haven't been sent yet.
- Each setting has a **?** button that explains it, with the factory values
  from the eQ-3 manuals.

The **House** tile sets several rooms at once: ECO, AUTO, COMFORT or MANU
with a temperature, permanently or until a date and time. Devices that
belong to no room are listed at the end.

Every change goes to the Cube as a radio command. The tile shows the result
when it is done: "ok", or what failed. While radio commands wait for the
devices' acknowledgement, the header says how many are pending.

## Log

Shows the Cube's recent events and new ones as they arrive: device states,
configuration and name changes, connections to the Cube and errors. The filter matches any part of a
line (time, kind, device, text). **Clear** empties the view only; the
server keeps its log.

## Settings

- **Poll every**: how often the page asks the server for the state. Device
  changes arrive in between as they happen.
- **Expert mode**: radio addresses, device types, the Cube's budget and the
  server version.
- **Cube clock shown in**: the browser's time zone or the Cube's.
- **Language**: from the browser, or English, German, French, Italian,
  Dutch or Polish. A text without a translation is shown in English. The
  server's error messages are in English. French, Italian, Dutch and
  Polish are machine-translated and not yet reviewed by a native
  speaker; a note under the choice says so, in the language and in
  English.
- **Forget token**: removes the stored API token.
- **Software**: the running version, the update channel and whether a newer
  release is out. On a box with the update helper installed
  (packaging/README.md, "Updating"), **Update** installs it and restarts
  the server. If the new version does not come up, the old one is put back.

The settings are kept in the browser, per browser.

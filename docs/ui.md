# Using the web UI

`massimo serve` serves the web UI at its listen address, by default
<http://127.0.0.1:8099/>. A box installed with `massimo install` serves it
on the address its `-listen` names.
The page works on a phone as well as on a desktop,
and follows the system's light or dark mode.

When the server has an API token (`MASSIMO_API_TOKEN`),
the page asks for it once and keeps it in the browser.
Settings → Forget token removes it again.
The live updates come over a stream that opens with a single-use ticket,
so the token never appears in a URL.

## Header

- **Cube**:
  chooses the Cube when the server has several.
  Next to it you see whether the server is connected to that Cube.
  The last entry, **Search for new Cubes…**,
  asks the server to look for Cubes on the LAN (about two seconds;
  the server also looks every 30 minutes by itself).
  Cubes it finds but does not serve are listed in the choice under "Found,
  not configured" and cannot be chosen;
  a line under the header counts them and opens to their address and firmware
  and the lines to put into the server's env file (`/etc/massimo/env` for a
  server installed with `massimo install`, otherwise `.env`).
  The server serves a Cube once its `MASSIMO_CUBE_<SERIAL>_ADDRESS`,
  or only a `_NAME`, is there and the server has been restarted;
  with a name alone it takes the address from discovery and follows it.
  With several Cubes each also needs a `_PROXY` address.
  Key and IV are only needed for the portal commands (README.md,
  "Cube variables").
  A Cube the server finds by itself appears without reloading the page,
  and in the log.
- **The Cube's clock** and the time of the last update.
- **↻** asks the server for the current state now.
- The three page buttons switch between **Rooms**, **Log** and **Settings**.
- When the server was updated, a button offers to reload the page.
  The page reloads by itself only when nothing is open: no setting being edited,
  no panel and not the settings page.
- In expert mode the header also shows the Cube's details and its radio budget,
  and when the server was started, with how long ago.
  The duty cycle and the free memory slots limit how many radio commands the
  Cube can send.

## Rooms

Every room has a tile:

- **IS** is the measured temperature: the wall thermostat's,
  else the first radiator thermostat's that reported one.
  Under it is how long ago the server received it ("12 min", "5 h", "3 days");
  the tooltip has the exact time.
  "≥" means the value was already there when the server connected to the Cube,
  so it may be older.
  Radiator thermostats send the measured temperature only now and then,
  mostly after the valve moved.
- **Open** in the tile's head means a window contact of the room reports an open window.
  It shows how long ago the window was opened and, as tooltip,
  which contacts are open; it goes as soon as the window is closed.
- **"Values may be old"**:
  a thermostat of the room has not been heard from for more than 24 hours,
  and the note says how long.
  It may be out of range or its batteries flat,
  but a thermostat whose valve stays shut outside the heating season can be quiet as well.
  Its values are greyed.
- **SET** is the set temperature.
  The up and down chevrons next to it change it in steps of 0.5 °C;
  the value is sent 2 seconds after the last click, or at once with **Save**;
  **Cancel** drops it.
  A temperature set this way puts the room into manual mode.
- **AUTO** follows the week programme.
  **MANU** keeps the current temperature.
  **HOLIDAY** keeps a temperature until a date and time.
- **"mixed"** next to the mode buttons means the room's thermostats are in
  different modes, for example after another program set one of them.
  The modes in use are outlined, and the note lists each device's mode.
- The **⋯** menu locks or unlocks the buttons on the room's thermostats,
  and deletes a room that has no devices.
- The room name or the **i** button opens the device list:
  every device with its values (valve, set and measured temperature,
  offset) and badges for battery, transmission errors, lock, window and mode.
  Expert mode adds each device's type and radio address.
  Valve, measured temperature, window and a low battery show how old they are;
  in expert mode also the exact time, and when the device was last heard from.
  A device that has not been heard from for more than 24 hours has a "Not seen
  for …" badge.
- The cog button opens the room's configuration:
  - **Room**:
    the name.
  - **Temperatures**:
    comfort, eco, open window and its duration, maximum and minimum.
    For radiator thermostats also Boost, descaling,
    maximum valve and valve offset.
  - **Devices**:
    name, room, temperature offset,
    and for a wall thermostat whether its display shows the measured temperature.
  - **Week programme**:
    one bar per day.
    - A click on a day selects it together with every day that has the same programme.
    - With **Only {day}** on, changes apply to that day alone.
      Off, they apply to all selected days.
    - **Copy {day} to …** copies the day to the days you choose.
    - **Reset day** and **Reset all** go back to what the Cube has.
    - **Send** writes only the days that changed.
  - **Copy**:
    copies this room's stored settings (temperatures, week programme,
    radiator settings) to other rooms.
    It copies what is stored, not changes that haven't been sent yet.
- Each setting has a **?** button that explains it, with the factory values
  from the eQ-3 manuals.

The **House** tile sets several rooms at once: ECO, AUTO,
COMFORT or MANU with a temperature, permanently or until a date and time.
Devices that belong to no room are listed at the end.

Every change goes to the Cube as a radio command.
The tile shows the result when it is done: "ok", or what failed.
While radio commands wait for the devices' acknowledgement,
the header says how many are pending.

## Log

Shows the Cube's recent events and new ones as they arrive: device states,
configuration and name changes, connections to the Cube and errors.
A device's state line says what changed ("window opened", "set 18.0 → 21.0",
"mode: manual", "reports again"), whoever changed it, also at the device;
its first line since the server started shows the whole state.
The filter matches any part of a line (time, kind, device, text).
**Clear** empties the view only; the server keeps its log.

## Settings

- **Poll every**:
  how often the page asks the server for the state.
  Device changes arrive in between as they happen.
- **Expert mode**:
  radio addresses, device types, the Cube's budget and the server version.
- **Cube clock shown in**:
  the browser's time zone or the Cube's.
- **Language**:
  from the browser, or English, German, French, Italian, Dutch or Polish.
  A text without a translation is shown in English.
  The server's error messages are in English.
  French, Italian, Dutch and Polish are machine-translated and not yet reviewed
  by a native speaker; a note under the choice says so,
  in the language and in English.
- **Cubes**:
  the Cubes the server serves, kept in its store,
  each in a panel that opens with a click: its connection,
  where its settings came from (the settings file, changed here,
  or for this run only), and the fields **Name**, **Address** (empty:
  the address the Cube answers discovery from, shown in grey),
  **Follow the address the Cube answers discovery from**,
  **Proxy address** (the address of this server other programs connect to for this Cube;
  empty: :62910 when it is the only Cube) and **Serial in discovery replies**.
  **Hidden devices** lists the devices hidden besides those of room 255,
  each with **Show again**; **Hide** takes a radio address,
  with the Cube's devices to choose from.
  **Save** sends the changes, which apply at once:
  a hidden device leaves the tiles, the API and the original software,
  and a room that holds only hidden devices goes with it.
  When a line of the server's settings file says something else than the panel,
  a notice names the line and **Take the file's value** takes it.
  **Remove** stops serving the Cube;
  it keeps its settings and history in the store and is listed under **Found on
  the LAN** once discovery sees it, with **Add**,
  which serves a found Cube at once (also in the list of found Cubes under the header).
  A Cube served for this run only (`serve -addr` or `-sn`) cannot be changed here.
- **Appearance**:
  **Mode** is light, dark, or as the system,
  which follows the device's setting and changes with it.
  **Colour scheme** picks the colours: the built-in Massimo (the default),
  High contrast and Graphite (a grey accent in place of the blue), and your own.
  Every scheme has a light and a dark set; the mode picks which is shown.
  **Customise…** opens an editor with a colour input per token for both sets,
  applied to the page as you change them,
  and a number next to each background that is the contrast of the text on it
  (from 4.5 it reads well, under 3 badly).
  **Save as** keeps the edited scheme in this browser under the name given,
  **Reset** goes back to the scheme the editor started from,
  **Delete** removes one of your own.
  **Export** downloads the chosen scheme (or the one being edited) as a JSON file,
  `{"format": "massimo-scheme/1", "name": …, "light": {…}, "dark": {…}}` with
  the token names of style.css without `--`;
  **Import…** reads such a file and keeps it under its name.
  A token the file lacks takes the default's value, unknown ones are ignored,
  and a file that is not a scheme says so under the buttons.
- **Forget token**:
  removes the stored API token.
- **Software**:
  the running version, the update channel and whether a newer release is out.
  On a box with the update helper installed (packaging/README.md, "Updating"),
  **Update** installs it and restarts the server.
  If the new version does not come up, the old one is put back.
  While the helper works, **Check for updates** and **Update** are greyed out,
  also after a reload of the page, until it reports that it is done.

The settings are kept in the browser, per browser, except the Cubes,
which are the server's.

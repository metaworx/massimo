# Using the web UI

`massimo serve` serves the web UI at its listen address, by default
<http://127.0.0.1:8099/>. A box installed with `massimo install` serves it
on the address its `-listen` names.
The page works on a phone as well as on a desktop,
and follows the system's light or dark mode.

The page asks who you are: a name and a password,
or an API key pasted into the second field,
and keeps the session in the browser for 30 days from its last use;
Settings → Log out ends it.
A server without users shows the form that creates the first owner to a
browser on the local network; until then everything is open,
and a warning under the header's rule says so and what closes it:
the first owner, or `MASSIMO_API_TOKEN` in the server's settings file
(`-token` for a server without a store).
It cannot be dismissed, and goes once the server asks who is there.
What the signed-in role may not do is not shown (docs/api-v1.md,
"Authentication").
The live updates come over a stream that opens with a single-use ticket,
so the session never appears in a URL.

## Header

- **Cube**:
  chooses the Cube when the server has several.
  Next to it you see whether the server is connected to that Cube.
  The last entry, **Search for new Cubes…**,
  asks the server to look for Cubes on the LAN (about two seconds;
  the server also looks every 30 minutes by itself).
  Cubes it finds but does not serve are listed in the choice under "Found,
  not configured" and cannot be chosen;
  a line under the header counts them and opens to their address,
  whether they take it by DHCP or have a static one ("DHCP" or "static address"),
  their firmware and the lines to put into the server's env file
  (`/etc/massimo/env` for a server installed with `massimo install`,
  otherwise `.env`).
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
  when the server was started, with how long ago,
  and the certificate the server serves (its source, names and expiry).
- With a self-signed certificate (`massimo serve -tls self-signed`)
  the browser warned once, and a line under the header shows the
  certificate's fingerprint: compare it with the server's log,
  or with `massimo tls show` on the box.
- A certificate that ends within two weeks, or one the server could not
  get from Let's Encrypt yet, is a warning in the same line.
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
    A device moved to another room gets that room's settings,
    as when it is put into a room (Settings → Cubes, **Pair a device**),
    where the role has the capability `configure`.
    **Remove from the Cube** asks in the row,
    then deletes the device from the Cube:
    the Cube resets it to its factory settings,
    and it has to be paired again to be used again.
    A device out of range or out of battery stays in the Cube's memory
    and comes back under the devices without a room.
  - **Week programme**:
    one bar per day.
    - A click on a day selects it together with every day that has the same programme.
    - With **Only {day}** on, changes apply to that day alone.
      Off, they apply to all selected days.
    - **Copy {day} to …** copies the day to the days you choose.
    - A switch point's time is typed in full, hours and minutes;
      the rows take the order of their times at the next change or at **Send**.
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

**Devices without a room** are listed at the end:
a device just paired, or one the original software paired through the proxy
and never put into a room.
**Put into a room…** opens the form of Settings → Cubes' pairing under the row,
with name, room and **Copy the room's settings**,
and **Remove from the Cube** asks in the row, then deletes it from the Cube.

Every change goes to the Cube as a radio command.
The tile shows the result when it is done: "ok", or what failed.
While radio commands wait for the devices' acknowledgement,
the header says how many are pending.

## Log

Shows the Cube's newest 200 events and new ones as they arrive: device states,
configuration and name changes, connections to the Cube, pairing,
restores and errors.
A lost connection is one line, however often the server tries again
until the Cube is back.
A device's state line says what changed ("window opened", "set 18.0 → 21.0",
"mode: manual", "reports again"), whoever changed it, also at the device;
its first line since the server started shows the whole state.
**Older events** above the lines loads the 200 before the oldest shown,
the view staying where it was,
until the server has nothing older ("No older events").
The pages fetched stay while new events come in, with room for 2000 more lines;
past that the oldest lines go, and Older fetches them again.
Only a log left open until every fetched line has gone pages on by the time of
the oldest line, where an event of that same millisecond can be missed.
A page that cannot load says so in the log, whatever is chosen;
an answer that comes after **Clear**, another choice or another Cube is dropped,
and another Cube starts the log over with every device.
The server keeps its log in its store across restarts and updates,
for the days `log.keep_days` says (30 by default; docs/api-v1.md,
"GET and PUT /api/v1/settings").
The two choices in the head show one kind of event (all kinds by default)
and one device (all devices); the server answers what matches,
and the new lines follow the choices too.
The filter matches any part of a line (time, kind, device, text).
**Clear** empties the view only; the server keeps its log.

## Settings

Signed in with a role that has the capability `preferences` (manager,
operator and viewer by default),
the settings and your own colour schemes follow you:
the server keeps them for you,
and every browser you sign in on takes them at sign-in,
the first one handing over what it had.
Once the server has your schemes, its list is the list:
a scheme deleted in one browser goes from the others when they next load the page.
Under each of **Poll every**, **Expert mode**, **Cube clock shown in**,
**Language**, **Mode** and **Colour scheme** a switch **only on this device**
keeps that setting's value in this browser instead,
for a phone that polls less often or a wall tablet that stays dark;
switched off, your value applies again.
Log out takes your settings and schemes out of the browser,
a change made just before still reaching the server,
so that the next one to sign in there starts from the defaults
and does not take yours into its account;
what is kept only on this device stays.
A guest, a server without users and the API token keep every setting in the browser,
without the switches.

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
  the LAN** once discovery sees it, with its address,
  DHCP flag and firmware and **Add**,
  which serves a found Cube at once (also in the list of found Cubes under the header).
  A Cube served for this run only (`serve -addr` or `-sn`) cannot be changed here.

  **Pair a device**, at the top of the Cube's panel,
  shows the pairing time (60 seconds, at most 600) and **Start**,
  which puts the Cube into pairing mode;
  then the device's pairing button is pressed, as its manual says.
  A countdown with **Stop** runs while the Cube pairs,
  and each device that joins is listed with its type and serial and a form:
  **Name**, **Room** (one of the Cube's rooms),
  and for a thermostat **Copy the room's settings**, on by default,
  shown where the role has `configure`, which the copy needs.
  **Put into a room** links the device with the room's other devices and lists it there;
  with the copy it also gets the temperatures,
  valve settings and week programme of the room's other thermostats,
  as the original software does, which costs up to two thirds of an hour's
  radio budget for a new thermostat (the **?** says how);
  the room's other thermostats get its temperatures too.
  The line under the form says how it went, and the end of pairing is said too.
  A role with the capability `pair` but not `cubes`, the manager's by default,
  sees the Cubes' panels with the pairing alone;
  putting a device into a room needs `structure`.
- **Backup and restore** (with `backup`):
  **Download backup** saves the Cube as it is now, as YAML or JSON (**Format**).
  The server keeps the Cube's backups in its store,
  listed newest first with why each was taken (after a change, by hand,
  before a restore, or the file a restore was sent), its size and who asked:
  one follows by itself an hour after the Cube's configuration last changed,
  at the latest two hours after the first change (the settings `backups.*`,
  docs/api-v1.md, "Backups"), and a line says when while one is pending.
  **Take a backup now** keeps one by hand.
  Each row has **Download**, **Compare with the one before**,
  which lists what changed as the restore would send it,
  and, with `restore`, **Restore…** and **Delete**, which asks first.
  **Restore from file…** (with `restore`) takes a backup file instead.
  A restore first shows its plan, the steps with the radio ones marked,
  the notes and what the radio commands cost of the hourly budget,
  and sends nothing until **Send N commands**;
  then each step is marked as the Cube took it, and the end is said.
  A Cube that is not in the server's store (a run with `-addr`) has no list;
  its download and the restore from a file work as before.
- **Appearance**:
  **Mode** is light, dark, or as the system,
  which follows the device's setting and changes with it.
  **Colour scheme** picks the colours: the built-in Massimo (the default),
  High contrast and Graphite (a grey accent in place of the blue), and your own.
  Every scheme has a light and a dark set; the mode picks which is shown.
  **Customise…** opens an editor with a colour input per token for both sets,
  applied to the page as you change them,
  and a number in front of each background's colour that is the contrast of the text on it
  (from 4.5 it reads well, under 3 badly).
  Next to each colour input is its hex value, `#f6f6f4`, in a text field:
  a value typed or pasted there, with or without the `#`,
  also as `#abc` and in any case, applies at once
  and is written as `#rrggbb` in lower case when you leave the field.
  Anything that is not a colour is refused: the field is marked,
  a message under the sets says why, the colour stays,
  and Escape puts the field back.
  The browser's own colour picker (the popup with R, G, B fields in Chromium)
  cannot be changed by the page, so the hex field is the place to paste a value.
  **Save as** keeps the edited scheme in this browser under the name given,
  **Reset** goes back to the scheme the editor started from,
  **Delete** removes one of your own.
  **Export** downloads the chosen scheme (or the one being edited) as a JSON file,
  `{"format": "massimo-scheme/1", "name": …, "light": {…}, "dark": {…}}` with
  the token names of style.css without `--`;
  **Import…** reads such a file and keeps it under its name.
  A token the file lacks takes the default's value, unknown ones are ignored,
  and a file that is not a scheme says so under the buttons.
- **Users** (with `users`, or `guests` for the guests alone):
  every user in a panel with its display name, role, whether it is disabled,
  a guest's end, and a new password, which ends the user's sessions;
  **Remove**; the user's sessions and API keys with **Revoke**,
  and **Create key**, which shows the new key once.
  **Add a user** opens the form for one.
- **Roles** (with `roles`):
  every role in a panel with its display name and a switch per capability,
  **Disable** (with `roles.disable`), **Remove** for a created role nobody has,
  and **Add a role**; the owner cannot be changed.
  **Capabilities for every role** (with `capabilities`) switches a capability
  off for every role but the owner.
- **Change password** and **Log out**:
  the own password, with the current one, and the end of the session
  (an API key is only forgotten).
- **Software**:
  the running version, the update channel and whether a newer release is out.
  On a box with the update helper installed (packaging/README.md, "Updating"),
  **Update** installs it and restarts the server.
  If the new version does not come up, the old one is put back.
  While the helper works, **Check for updates** and **Update** are greyed out,
  also after a reload of the page, until it reports that it is done.

The settings are kept in the browser, per browser, except the Cubes,
which are the server's.

## How the page is built

This section is for changing the UI;
its rules are in the project contract (`AGENTS.md`, "The web UI").
The page is plain HTML, CSS and JavaScript under `internal/server/ui/`,
embedded in the binary, with no build step.
`index.html` is the skeleton: the head, the icons, the header, the sections,
the sign-in dialog, the first-owner form and the scripts,
with two marker comments.
At start the server replaces `<!-- partials: settings -->` with the settings
panels and `<!-- partials: templates -->` with the templates,
from the files in `ui/partials/` in the order `uiPartials` in
`internal/server/ui.go` lists them, and serves the result at `/`.
A partial is a file per feature,
and its first line names the script that uses it.
What a script builds again and again, a Cube's panel, a log line, a scheme row,
is a `<template>` there, which the script clones with `fromTemplate`,
fills with `textContent` and wires with `addEventListener`.
The shared pieces (buttons, the `.field` rows, forms, confirmations in the page)
come from `components.js`; the scripts never use `innerHTML`.
The texts are in `ui/i18n/<lang>.json`, one flat table per language,
English the base: markup takes them with `data-i18n` and `data-i18n-attr`,
scripts with `t()` and `tn()`, and a change of language fills the page and
the templates again and redraws what was cloned from them.
A new panel is a partial, listed in `uiPartials`,
with its script added to the `<script>` tags of `index.html` and to the embed
list in `ui.go`; a new text is a key in all six files,
which `go test ./internal/server` checks against the markup and the scripts,
as it checks every id the scripts look up.
Every URL the page and the scripts use is relative to the page (`ui/style.css`,
`api/v1/cubes`, `healthz`),
so that a reverse proxy may serve the UI under a path prefix it strips
(docs/reverse-proxy.md); `go test ./internal/server` refuses an absolute one.

# Using the web UI

`massimo serve` serves the web UI at its listen address, by default
<http://127.0.0.1:8099/>. A box installed with `massimo install` serves it
on the address its `-listen` names.
The page works on a phone as well as on a desktop,
and follows the system's light or dark mode.

The page asks who you are: a name and a password,
or an API key pasted into the second field,
which is open from the start once the page has seen the server ask for no key
(the owner made from `MASSIMO_API_TOKEN` has no password),
and keeps the session in the browser for 30 days from its last use;
👤 → Sign out ends it.
The password stays in the browser:
the page signs in by OPAQUE (docs/api-v1.md, "The sign-in by OPAQUE"),
running argon2id itself while the button turns,
a fraction of a second on a laptop, more on an old phone.
A user whose password the server still hashes (`auth_version` 1) is signed in
with the password after the OPAQUE attempt, while the server takes that,
and the page registers the password by OPAQUE at once:
it is the last time that password goes to the server.
A server without users shows the form that creates the first owner to a
browser on the local network; until then everything is open,
and a warning under the header's rule says so and what closes it:
the first owner, or `MASSIMO_API_TOKEN` in the server's settings file
(`-token` for a server without a store).
It cannot be dismissed, and goes once the server asks who is there.
The first owner's name is checked as a new user's is (System settings → Users),
and its password is registered by OPAQUE, as every password the page sets.
What the signed-in role may not do is not shown (docs/api-v1.md,
"Authentication").
The live updates come over a stream that opens with a single-use ticket,
so the session never appears in a URL.

## Header

The header's line reads, from the left:
**Massimo** with the server's version, the time of the last update with **↻**,
the Cube's clock;
then the Cube choice with the server's connection to the Cube as a plug;
then the house, **👤** and the other page buttons.
On a screen narrower than 600 px the Cube choice takes a line of its own,
and **☰** beside the house and 👤 holds **Log** and **System settings**,
highlighted while one of them is shown.

The header stays at the top while the page scrolls.
Scrolled, it keeps its first line without the time of the last update
and the Cube's clock, and the lines under it go until the page is back
at its top, the button that reloads a new version apart;
narrower than 600 px the Cube choice then takes the line of **Massimo**.

- **Cube**:
  a menu that chooses the Cube when the server has several.
  Closed it shows the chosen Cube's name (its serial where it has none)
  and whether the server is connected to it, as a plug:
  joined and green while connected, apart and red while not;
  its tooltip adds the serial and the address.
  Open, each Cube has its plug, its name and under it its serial and
  address, the list as wide as they need; the arrow keys move between them.
  The last entry, **Search for new Cubes…**,
  asks the server to look for Cubes on the LAN (about two seconds;
  the server also looks every 30 minutes by itself).
  Cubes it finds but does not serve are listed in the choice under "Found,
  not configured" and cannot be chosen,
  as long as they answer: one that a search or the server's own look no
  longer hears leaves the list at the page's next poll;
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
- **The time of the last update**, **↻**, which asks the server for the
  current state now, and **the Cube's clock**.
- **👤** drops down who is signed in, with the role,
  or the API key and whose it is;
  **My profile** and **My settings**, which open those pages,
  and **Sign out**, which ends the session, each with its icon;
  its tooltip names who is signed in, by display name and login.
  The items of **☰** have the icons of their pages.
  An API key or the server's token is only forgotten in this browser
  (**Forget key**),
  and an open server says that it is open to everyone and has nothing to sign out of.
  Escape, a click outside and a choice close the menu,
  as they close a room's **⋯**.
- The three page buttons, a house, a sheet and a cog,
  switch between **Rooms**, **Log** and **System settings**.
- The page shown is in the address, Rooms without one:
  `#log`, `#profile` (My profile), `#prefs` (My settings) and `#system`
  (System settings),
  so that a reload opens it again, the browser's back and forward move between
  the pages, and an address can be kept as a bookmark.
  The log's range, kind and device follow its page,
  as in `#log?range=today&kind=state&device=MEQ0000101` (see Log).
  A page the role may not open shows Rooms instead.
- When the server was updated, a button offers to reload the page.
  The page reloads by itself only when nothing is open: no setting being edited,
  no panel and none of the settings pages.
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

When the Cube chosen is not connected, a notice above the rooms says so,
naming it by its name and giving its serial:
the address the server tries, when the Cube last sent data
(or last answered discovery),
and what to check, a Cube serving one client at a time;
the rooms the server still has stay under it.
Without a Cube, it says that none is configured.
For a role with `cubes` its button opens System settings → Cubes.

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
  Expert mode adds each device's type, serial number and radio address,
  as "S/N MEQ0000103 · RF 0a0103": the serial is printed on the device,
  the radio address is not.
  A device without a name is shown by its serial number, here and on every
  page, and the page asks the server for a device by its serial.
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
    as when it is put into a room (System settings → Cubes, **Pair a device**),
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
  - **Send** is off while nothing differs from what the Cube has,
    or on the Copy tab while no room is ticked.
- Each setting has a **?** button that explains it, with the factory values
  from the eQ-3 manuals.

The **House** tile sets several rooms at once: ECO, AUTO,
COMFORT or MANU with a temperature, permanently or until a date and time;
its **Send** is off while no room is ticked.

**Devices without a room** are listed at the end:
a device just paired, or one the original software paired through the proxy
and never put into a room.
**Put into a room…** opens the form of System settings → Cubes' pairing under the row,
with name, room and **Copy the room's settings**,
and **Remove from the Cube** asks in the row, then deletes it from the Cube.

Every change goes to the Cube as a radio command.
While it is sent, the tile's buttons that send are off,
the one clicked turning, so that a second click sends nothing;
a value of the chevrons stays shown, with **Save** turning, until the answer.
The tile shows the result when it is done: "ok", or what failed.
While radio commands wait for the devices' acknowledgement,
the header says how many are pending.

## Log

Shows the Cube's last 20 events, newest first, and new ones on top as they
arrive: device states,
configuration and name changes, connections to the Cube, pairing,
restores and errors.
A lost connection is one line, however often the server tries again
until the Cube is back.
A line names its device by name, else by serial number,
with the radio address after it in expert mode only;
the Cube's own configuration reads "Cube KEQ0000000".
A removed device reads "Removed radiator thermostat MEQ0000101 (Window)",
with its name as it was.
A device's state line says what changed ("window opened", "set 18.0 → 21.0",
"mode: manual", "reports again"), whoever changed it, also at the device;
its first line since the server started shows the whole state.
A view scrolled down keeps its place when a new line comes in on top.
The choice of how far back, beside the kind and the device,
shows the **last 20 events** (the default), the **last 10 minutes**,
the **last hour**, **today**, **this week** (from Monday), **this month**
or **all events**; it rolls with the time:
the last 20 keep the newest 20 as events come in,
and a time range drops a line once it falls out, checked each minute.
A line of another day shows its date.
For a time range and for all events,
**Older events** under the lines loads the 200 before the oldest shown, within the range,
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
and one device (all devices), listed by name and serial number;
the server answers what matches,
and the new lines follow the choices too.
The filter matches any part of a line (time, kind, device, text).
**Clear** sets the filter, the kind and the device back to all
and loads the log again, the range staying; it is off while none of them is set.
The range, the kind and the device are in the address after `#log`,
each left out at its default:
`range=` one of `10m`, `1h`, `today`, `week`, `month` and `all`,
`kind=` an event's kind and `device=` a device's serial number
(a link's radio address takes the serial once the devices are loaded),
so that a reload, a bookmark or a link opens the log with them,
and the log's button opens it with the choices it had.
A choice changes the address in place, so back still leaves the log.
A value the page does not know is dropped from the address,
and so is a device the Cube does not have, once its devices are loaded.
The filter's text is not in the address.

## My profile

👤 → **My profile**, for a user signed in (not on an open server,
nor with the server's API token):

- **Display name**, the name the 👤 menu and the users list show,
  with **Save**, off while it is the one stored.
- **Change password**, for a user with a password:
  the own password, with the current one,
  which a sign-in by OPAQUE proves without sending it;
  the new one is registered by OPAQUE, as every password the page sets.
  A user with keys only, such as the owner made from `MASSIMO_API_TOKEN`,
  has **Set a password** instead, without a current one, and a line saying
  that it lets the user sign in by name in any browser.
- **Sessions and keys**: the own sessions and API keys,
  each with where and when it was last used and **Revoke**,
  which signs another browser out without an administrator;
  **Create key** shows the new key once.

## My settings

👤 → **My settings**, the page's settings:

Signed in with a role that has the capability `preferences` (manager,
operator and viewer by default),
the settings and your own colour schemes follow you:
the server keeps them for you,
and every browser you sign in on takes them at sign-in,
the first one handing over what it had.
Once the server has your schemes, its list is the list:
a scheme deleted in one browser goes from the others when they next load the page.
Beside each of **Poll every**, **Expert mode**, **Cube clock shown in**,
**Language**, **Mode** and **Colour scheme**, in a column of its own at the
right headed **only on this device**, a switch keeps that setting's value in
this browser instead (its tooltip names the setting),
for a phone that polls less often or a wall tablet that stays dark;
switched off, your value applies again.
My settings and My profile are at most 640 px wide.
Sign out takes your settings and schemes out of the browser,
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

## System settings

The cog in the header, **System settings**, holds what is the server's,
each part for a role with its capability;
a role with none of them (the viewer's, a guest's) has no cog.

The page is a card per part, as wide as the page:
Software update first, then Maintenance, Cubes, Users, Roles,
**Capabilities for every role**, and Backup and restore last;
from 900 px Users, Roles and Capabilities flow in two columns,
none split, the break where their heights put it.
The title stays under the header while the page scrolls,
with **Sections** at its right end (a bulleted list, apart from ☰):
it lists the cards, marks the one in view and brings the one chosen into view.
A row of buttons under the title names the cards the role sees,
each bringing its card into view, opened.
A card folds with the button in its heading,
which also says what it holds at a glance (**up to date**, **1 served**,
**2 users**, **all on**, **3 kept**), also while it is folded;
the browser keeps which cards are folded.
Once the row of buttons is out of sight, **↑** at the bottom right goes back
to the top of the page.

- **Software update**:
  the running version, the update channel and whether a newer release is out.
  On a box with the update helper installed (packaging/README.md, "Updating"),
  **Update** (with `update`) installs it and restarts the server.
  If the new version does not come up, the old one is put back.
  While the helper works, **Check for updates** and **Update** are greyed out,
  also after a reload of the page, until it reports that it is done.
- **Maintenance** (with `restart`, `reboot` for its reboot,
  or `settings` for its last two parts):
  **Restart the server**
  asks in the section and restarts the server through its service manager
  (`docs/api-v1.md`, "POST /api/v1/restart");
  the page then asks `/healthz` every two seconds until the server reports a new start,
  reads the data again and says when it started,
  or says after two minutes that it has not come back.
  Where the server cannot be restarted from this page the button is missing
  and the server's reason is shown,
  such as a server in a terminal or an open server asked from elsewhere than the box.
  The settings of the settings file that changed since the start and apply only
  at the next one are listed under it, by name, and counted in the card's head.
  **Settings file** names the file and when the server last read it;
  **Reload now** reads it again at once, as SIGHUP does
  (`docs/api-v1.md`, "POST /api/v1/reload"), and says what changed.
  **Reboot the computer** (with `reboot`, the owner's) has a delay in seconds,
  60 to begin with, 0 to 3600, and asks in the section,
  naming the delay, before it asks the server (`docs/api-v1.md`,
  "GET /api/v1/reboot, …");
  while the reboot waits the row says when it comes,
  and **Take the reboot back** takes it back;
  from its time the page follows `/healthz` as for the restart,
  for up to five minutes,
  and says when the computer is back, that it has not come back,
  or why the reboot failed while the old server still answers.
  Where the computer cannot be rebooted from here
  the delay and the button are missing and the server's reason is shown:
  the switch is off, the helper is not installed, a server in a terminal
  (`packaging/README.md`, "Rebooting from the web UI").
  **Store** (with `settings`) opens to what `massimo store info` shows:
  the file, the schema version, the node and tenant and the rows per table,
  its head the file, the schema and the rows in all;
  **Settings in effect** to what `massimo config` shows: the version, the
  settings file, the data directory, the listen address, TLS, the log level
  and every `MASSIMO_*` variable with where it came from, a key, an IV, a
  token or a password as "set, not shown"
  (`docs/api-v1.md`, "GET /api/v1/store, GET /api/v1/config").
  From 900 px Software update and Maintenance share a row.
- **Cubes**:
  the Cubes the server serves, kept in its store,
  each in a panel whose head shows the name, the serial and the address,
  and opens with a click: its connection,
  where its settings came from (the settings file, changed here,
  or for this run only), and the fields **Name**, **Address** (empty:
  the address the Cube answers discovery from, shown in grey),
  **Follow the address the Cube answers discovery from**,
  **Proxy address** (the address of this server other programs connect to for this Cube;
  empty: :62910 when it is the only Cube) and **Serial in discovery replies**.
  **Hidden devices** lists the devices hidden besides those of room 255,
  each with its name, serial number and radio address and **Show again**;
  **Hide** takes a radio address,
  with the Cube's devices to choose from.
  **Save**, off while nothing differs from the Cube's settings,
  sends the changes, which apply at once:
  a hidden device leaves the tiles, the API and the original software,
  and a room that holds only hidden devices goes with it.
  When a line of the server's settings file says something else than the panel,
  a notice names the line and **Take the file's value** takes it.
  **Remove** stops serving the Cube;
  it keeps its settings and history in the store and is listed under **Found on
  the LAN** while discovery hears it, with its address,
  DHCP flag and firmware and **Add**,
  which serves a found Cube at once (also in the list of found Cubes under the header).
  A Cube served for this run only (`serve -addr` or `-sn`) cannot be changed here.

  From 760 px a Cube's panel has two columns: its settings at the left,
  the pairing and the hidden devices at the right.
  **Pair a device**, at the top of the Cube's panel on a phone,
  shows the pairing time (60 seconds, at most 600) and **Start**,
  which puts the Cube into pairing mode;
  then the device's pairing button is pressed, as its manual says.
  A countdown with **Stop** runs while the Cube pairs,
  and each device that joins is listed with its type and serial
  (in expert mode also its radio address) and a form:
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
- **Users** (with `users`, `passwords`, or `guests` for the guests alone):
  a user changes only users whose role has no capability its own lacks,
  so nobody but an owner changes an owner, whose panel says so,
  and with `passwords` alone (a manager's) a panel offers the password alone;
  the role choice offers no owner but to an owner.
  Every user in a panel whose head shows the display name, the login apart
  from it and, at the right edge, the badges: **you** first, the role,
  the state and **keys only**;
  in the panel the display name, role, whether it is disabled,
  a guest's end, and a new password, which ends the user's sessions and,
  as every password the page sets,
  is registered by OPAQUE in the browser and never sent;
  **Save**, off while nothing differs from the user;
  **Remove**; the user's sessions and API keys with **Revoke**,
  and **Create key**, which shows the new key once.
  **Add a user** opens the form for one;
  its name is checked 2 seconds after the last keystroke and when the field is left:
  the letters a to z, digits and `. _ - @`, starting with a letter or a digit,
  at most 64 characters, capitals taken as lower case, and not a name in use.
  **Create** sends nothing while the name is wrong,
  and turns until the browser has registered the password and the server has
  added the user.
  With `settings`, **Password sign-in** switches off the sign-in that sends a
  password to the server,
  which the page uses for users whose password the server still hashes;
  the line under it says how many those are,
  and they cannot sign in once it is off until a password is set for them here.
- **Roles** (with `roles`):
  every role in a panel with its display name and a switch per capability,
  **Save**, off while nothing differs from the role,
  **Disable** (with `roles.disable`), **Remove** for a created role nobody has,
  and **Add a role**; the owner cannot be changed.
- **Capabilities for every role** (with `capabilities`):
  a switch per capability, which switches it off for every role but the owner.
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

## In the apps

The apps for Android and iOS (docs/app.md) open on a page of their own,
`ui/connect.html`: the servers the app keeps, each with what it found
when it last looked (on the LAN, remote, not reachable, a changed certificate),
a tap opening the server, ⚙ or a long press its settings;
the servers on the network and an address typed, to add one;
and the fingerprint of a self-signed certificate, compared with the
server's log or `massimo tls show` before it is trusted.
A browser that opens that page sees a line saying it is the app's.
A server's page in the app is this page,
except that the app keeps its session in place of the browser,
behind the phone's fingerprint or PIN where the server's settings in the app ask for it.

## How the page is built

This section is for changing the UI;
its rules are in the project contract (`AGENTS.md`, "The web UI").
The page is plain HTML, CSS and JavaScript under `internal/server/ui/`,
embedded in the binary, with no build step.
`index.html` is the skeleton: the head, the icons, the header, the sections,
the sign-in dialog, the first-owner form and the scripts,
with marker comments.
At start the server replaces each marker, such as `<!-- partials: prefs -->`,
with its partials, the panels of the settings pages and at the end
`<!-- partials: templates -->` with the templates,
from the files in `ui/partials/` in the order `uiPartials` in
`internal/server/ui.go` lists them, and serves the result at `/`;
System settings' cards fill three markers,
before, in and after the cards that flow in two columns.
A partial is a file per feature,
and its first line names the script that uses it.
What a script builds again and again, a Cube's panel, a log line, a scheme row,
is a `<template>` there, which the script clones with `fromTemplate`,
fills with `textContent` and wires with `addEventListener`.
The shared pieces (buttons, the `.field` rows, forms, confirmations in the page)
come from `components.js`; the scripts never use `innerHTML`.
A panel that opens, a Cube's, a user's or a role's, is a `detailsPanel`:
its head the name, the serial, login or key, and the badges,
its body the form cloned from the feature's template.
A long page is a grid of section cards (`section.section-card`,
its heading's `button.section-fold` folding it):
`sectionCards` makes them fold and keeps the fold in the browser,
`sectionBadges` shows what a card holds in its head,
`pageContents` fills the page's row of section buttons from them,
`pageBar` the sections menu of its title's row kept at the top,
and `backToTop` wires the page's ↑;
`pickerMenu` makes a dropdown the choice of one item (the Cube choice),
its items `menuItem`s, a Cube's connection a `connIcon`;
a device's serial and radio address are a `deviceIds`;
a list of facts is a `kvList`, of counts a `countList`;
an action with what it does and its button is an `.action-row`;
`stickyHeader` keeps the header at the top and compacts it once scrolled.
A button that sends a request is busy from its click until the answer:
off, with a turning ring after its text, and `aria-busy`
(`setBusy` and `whileBusy`),
so that a second click or Enter sends nothing;
a form that knows what it last sent keeps its Send or Save off
while nothing differs (`sendWhenChanged`).
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

`ui/connect.html` is a second page, the apps' (docs/app.md),
served at `/ui/connect.html` as it is, not assembled:
its markup, its templates and `<base href="../">`,
so that its URLs resolve as those of the page at `/`;
it loads `i18n.js`, `components.js` and its own `connect.js`, not `app.js`.
`go test ./internal/server` checks the ids `connect.js` looks up against
it (`uiPageScripts` in `ui_test.go`) and its texts as the page's.

One file is not the project's: `ui/vendor/opaque.js`,
`@serenity-kit/opaque` as published, the OPAQUE client in WebAssembly,
which the sign-in needs and which the browser does not offer
(`ui/vendor/README.md` says which version, its licence and how to update it).
`login.js` imports it when a sign-in first needs it, not with the page,
and the page's `Content-Security-Policy` allows WebAssembly to compile
(`'wasm-unsafe-eval'`) for it;
`go test ./internal/server` checks the file's SHA-256.

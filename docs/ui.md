# Using the web UI

`massimo serve` serves the web UI at its listen address, by default
<http://127.0.0.1:8099/>. A box installed with `massimo install` serves it
on the address its `-listen` names.
The page works on a phone as well as on a desktop,
and follows the system's light or dark mode.

The page asks who you are on a card in the page, not over it:
a name and a password,
or an API key pasted into the field under "Or an API key", a form of its own,
which is open from the start once the page has seen the server ask for no key
(the owner made from `MASSIMO_API_TOKEN` has no password).
The password form holds the one password field, its fields named,
so that a password manager such as KeePassXC fills it.
The page keeps the session in the browser for 30 days from its last use;
**Sign out** in ☰, or on the personal settings, ends it.
The card's ☰ sets the language and the appearance (light, dark, as the system)
before anyone signs in, the same settings as My settings,
kept in this browser.
Above the form, a button per enabled OpenID Connect provider,
**Sign in with Nextcloud** for example
(docs/api-v1.md, "Sign-in through OpenID Connect"),
goes to the provider and back signed in,
the provider's answer taken from the address at once;
an account the server does not know, or a refusal, comes back to the card,
which says why under the providers' buttons:
the account not known, no e-mail sent, the e-mail not verified,
or the account in none of the groups the provider admits.
The app shows no such button yet:
the providers refuse a sign-in inside an app's page.
While it asks, the page shows its name, its version and the sign-in,
nothing else, as it does from its first moment until the server has said who is in,
so that no button shows before the sign-in:
Sign out forgets what the page loaded of the Cubes,
so that the next to sign in sees nothing of the last session.
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
After an interruption the page opens a new stream that resumes after the
last event it read, so what happened meanwhile still arrives;
where the server no longer holds it, the page reloads the rooms
(docs/api-v1.md, "GET …/events").
A room's tile whose configuration has the focus is not drawn again under a click
or the user's typing, for a state event or a reload of the rooms alike;
it is drawn once the focus has left the configuration.

## Header

The header's line reads, from the left:
Massimo's logo with the server's version and the time of the last update with **↻**,
all on one baseline where they fit beside it, the M as tall as the page buttons;
then the Cube choice with the server's connection to the Cube as a plug;
then the house, **Rooms**, and **☰**, the menu of every other page.
Where the version, the server's sign and the time do not fit beside the logo
on one line, they take a row of their own across the header,
under the logo and the Cube choice, on a phone under the logo and the page buttons,
the version on a line of its own where the row is too narrow for all three,
so that it is cut only where it is wider than the header.
On a screen narrower than 600 px the Cube choice takes a line of its own,
and the logo is the mark alone, the M with the clock in its V.

The logo lies over the word "Massimo", which a screen reader reads:
the word shows until the drawing has come (`ui/brand/brand.svg`,
made by `make ui-brand`, `tools/brand/README.md`), and stays where it never does.
The drawing has no margin: its bottom edge is the letters' baseline,
which the version and the time of the last update stand on.
Its colours follow the light or dark mode chosen, its letters the page's text.
The sign-in, the form that creates the first owner and the apps' connect page
show the logo too, the first two larger, above their titles.

The header stays at the top while the page scrolls.
Scrolled, it keeps its first line, the server's sign and the time of the last update,
without the version, and the lines under it go until the page is back
at its top, expert mode's line of the Cube's radio budget apart;
narrower than 600 px the Cube choice then takes the line of **Massimo**,
the sign and the time a short line under it.
The compacted header has **↑** left of ☰, back to the top.

- **Cube**:
  a menu that chooses the Cube when the server has several.
  Closed it shows the chosen Cube's name (its serial where it has none)
  and how the server stands with it, as the network's sign:
  green while connected, a triangle while its lost connection is held open
  or it reboots, a cross while it holds the server's old connection,
  another client has it, or it is gone (see Signs, under How the page is built);
  its tooltip adds the serial, the address and the link in words.
  Open, each Cube has its sign, its name with an icon after it for each
  kind of problem it has (the radio, a battery, a window open too long, a
  device's link and, for one who may change the settings, the cog), each in
  its worst colour, and under it its serial and address,
  the list as wide as they need; the arrow keys move between them.
  The last entry, **Search for new Cubes…**,
  asks the server to look for Cubes on the LAN (about two seconds;
  the server also looks every 30 minutes by itself),
  and reads **Searching…** meanwhile.
  What it found shows in a toast at the page's foot for eight seconds,
  as a notification does:
  the number of Cubes found and not set up, with **Show**, for a role that may add Cubes,
  which opens them in System settings;
  or that none was found;
  a failed search in red, with the server's reason, for 15 seconds.
  Cubes it finds but does not serve are listed in the choice under "Found,
  not configured",
  as long as they answer: one that a search or the server's own look no
  longer hears leaves the list at the page's next poll.
  For a role that may add Cubes, a found Cube in the choice
  opens System settings with **Cubes** in view and its found list open,
  which lists them with **Add** and the lines for the server's settings file;
  for others the choice's entries cannot be chosen.
  A Cube the server finds by itself appears without reloading the page,
  and in the log.
- **Problems**, beside the Cube choice, a green check in a pill's frame
  where nothing is wrong on any Cube; where something is,
  a pill with an icon per kind (a Cube's link, the radio, a battery, a window
  open too long, the settings), which lists every Cube with a problem as a heading,
  its own sign and its link's badge, and under it its devices with a problem
  as "Room › Device" with their badges;
  for one who may change the settings, **Settings**:
  a Cube proxy that does not listen, a settings file that differs,
  a Cube that discovery finds at another address than its fixed one.
  What discovery found and the server does not serve has the pill's last icon,
  a cube in the accent's blue with their number in a corner badge,
  as information rather than a problem,
  and its list's last group, **Found on the network**:
  "not set up", for a role that may add Cubes with a link to System settings → Cubes,
  and under it each Cube and Massimo proxy with its address,
  for a proxy the Cube it serves, and the Massimo server serving it,
  its name a link to the address its announcement on the network gives.
  A window open briefly is not listed: it is a state, not a problem.
  Its list, as every menu of the page, opens inside the window,
  no higher than the room below it, and scrolls in itself, not the page.
- **How long ago the chosen Cube was heard from** and **↻**, one button without a frame,
  which asks the server for the current state now:
  "8 sec" since the server last took a message from the Cube,
  measured on the server's clock,
  counting on by itself, each second under a minute, then each minute, each hour;
  its tooltip says the time itself, "updated 20:51:00, click to reload";
  a finger hits it a little beyond what it shows, and ↻ turns while it asks.
- **The version** is short; its tooltip is the whole version, "(server version)".
  The Cube's clock is in the Cube's facts, under System settings → **Cubes**:
  the Cube tells it when the server connects, to the minute,
  and the page moves it on from then;
  a clock that was off by 90 s or more says by how much ("3 min ahead").
- **The house** shows **Rooms**, the header's one page button.
- **☰** holds every other page, at every width,
  each for the roles that may open it and with the icon of its page:
  **Log**;
  **Cube settings**, the chosen Cube's, for a role with `cubes` or with `pair`,
  **System settings**, for a role with one of its capabilities,
  **Maintenance**, for those and a role with `update`, `restart` or `reboot`,
  and **System info**, for every role with `read`;
  **Personal settings**, its tooltip naming whose they are, by display name and login,
  and **Sign out** (Forget key for an API key or the token, none on an open server),
  in the accent and bold, as the personal page's button.
  It is highlighted while one of its pages is shown, as a page button is,
  that page's entry marked current.
  While the log holds a warning or an error not yet settled or marked seen (see Log),
  ☰ carries an amber triangle or a red cross at its corner,
  on a disc of the card's colour, so that it shows on the highlighted ☰ too,
  and the sheet of its Log entry turns amber or red, both named with the count.
- The settings pages, Cube settings, System settings, Maintenance and System info,
  each have a row kept under the header, and in it, in the title's place,
  a button for each of them the role may open, with its icon and its name,
  always in this order, so that each keeps its place;
  the page shown is filled, and a click on it opens nothing.
  Below 900 px the buttons show their icon alone but for the page shown,
  which is as wide on every page, so that the buttons after it keep their places too:
  the longest of the names, as far as the row leaves room
  beside the sections menu's place, kept on every page;
  a longer name ends in "…".
  A role that may open one of them alone sees its title there, and no button.
- The page shown is in the address, Rooms without one:
  `#log`, `#me` (Personal settings, or at a card `#profile`, `#prefs` and
  `#notify`), `#cube` (Cube settings), `#system` (System settings),
  `#maintenance` (Maintenance) and `#info` (System info),
  so that a reload opens it again, the browser's back and forward move between
  the pages, and an address can be kept as a bookmark.
  The log's range, kind and device follow its page,
  as in `#log?range=today&kind=state&device=MEQ0000101` (see Log).
  A page the role may not open shows Rooms instead.
- Each page comes back at the place it was left,
  by its button and by back and forward alike;
  a page opened for the first time starts at its top.
- **↑**, round and filled in the accent's blue at the bottom right,
  shows on every page once it is a screen down,
  and another ↑ left of ☰ while the header is compacted;
  each takes the page back to its top,
  the focus on the page's sections menu where it has one.
- **The server's sign**, before the time of the last update,
  an icon in a frame whose label says what a click does:
  the plug in green while the server answers (a click updates now);
  a blue "i" while an update is out,
  for one who may install it (a click opens Software update);
  an amber triangle, blinking, once the server runs a new version
  (a click reloads the page);
  the open plug on a filled red square, blinking, after a poll without an answer:
  the page's other signs are greyed, their state not known,
  the time is that of the last update, and the page tries again at every poll.
  After an update the page reloads by itself only when nothing is open:
  no setting being edited, no panel and none of the settings pages.
- In expert mode a line under the first shows the chosen Cube's name,
  its duty cycle, free slots and radiograms pending,
  and the commands queued while there are any.
  The server's facts are in ☰ → **System info**,
  the Cube's in its panel under System settings → **Cubes**.
- With a self-signed certificate (`massimo serve -tls self-signed`)
  the browser warned once, and a line under the header,
  and in the sign-in dialog, asks to compare the SHA-256 fingerprint in
  the browser's certificate details with the one `massimo tls show` prints on the box,
  or the server's log, before signing in.
  The page shows no fingerprint itself:
  it comes over the very connection in question,
  and a man in the middle would pass the server's along, or rewrite it.
- A certificate that ends within two weeks, or one the server could not
  get from Let's Encrypt yet, is a warning in the same line.
  The duty cycle and the free memory slots limit how many radio commands the
  Cube can send.

## Rooms

When the Cube chosen is not connected, a notice above the rooms says so,
naming it by its name and giving its serial:
first why, where the server knows (its connection held open,
or the Cube refusing the server, as in System settings → Cubes),
the address the server tries, when the Cube last sent data
(or last answered discovery),
and what to check, a Cube serving one client at a time,
or, for a Cube whose address lies outside this machine's networks
(`other_network` of `docs/api-v1.md`), that the server reaches it only through a
router, that a Cube without a DHCP lease uses 192.168.0.222,
and the `massimo netconfig` line with its serial that gives it a fixed address,
which copies;
the rooms the server still has stay under it.
Without a Cube, it says that none is configured.
For a role with `cubes` its button opens System settings → Cubes.

Every room has a tile:

- **IS** is the room's temperature, its virtual wall thermostat's
  (docs/api-v1.md, "The virtual wall thermostat"), which every client shows:
  the wall thermostat's, else the median of the radiator thermostats',
  or their mean where the room's setting says so.
  Under it is how long ago the server received it ("12 min", "5 h", "3 days");
  the tooltip has the exact time.
  "≥" means the value was already there when the server connected to the Cube,
  so it may be older.
  Radiator thermostats send the measured temperature only now and then,
  mostly after the valve moved.
- **Icons in the tile's head** say what is wrong in the room, each in its colour
  and with its mark (see Signs, below):
  the radio, a Wi-Fi fan or a tower as you choose (My settings),
  a device that lost its link (amber, a triangle, the badge **link error**,
  its tooltip the original software's sentence:
  "The radio communication with another MAX! device in this room is disturbed."),
  or the Cube could not reach (red, a cross, the badge **communication error**,
  its tooltip the radiogram that failed, where the server gives it,
  by its technical name and code:
  "Last radiogram that failed: TimeInformation (0x03)");
  a battery, one getting weak (amber) or low (red), as the server keeps its level;
  an open window, the opening sash or the paned window as you choose,
  a window contact reporting it:
  blue for its first 5 minutes, then amber with a triangle,
  red with a cross after 10
  (the server's settings `window_warn_after` and `window_alert_after`);
  an hourglass, amber, a device that has not shown a write by its deadline
  (`docs/api-v1.md`, "Writes confirmed by the devices"):
  five minutes after a setpoint, a key lock or a display mode,
  about six after a setting or a week programme;
  its badge **not confirmed**, its tooltip what the device does not show and
  what it shows, "mode (shows auto), setpoint (shows 20.0 °C)";
  nothing while the write is pending, and it goes once the device shows it
  or a newer write replaces it;
  crossed arrows, the room's thermostats in different modes;
  a suitcase, a room on holiday, which is no problem but is listed there.
  A tap on them lists the devices concerned, each once with its serial,
  its badges under its name, led by the same icons, a warning's and a failure's
  by its mark, and how long ago each came,
  for different modes each thermostat's mode,
  and for a holiday its end, "holiday until 24.12., 18:00";
  Escape or a click outside closes the list.
  An icon goes as soon as its problem does.
  The room's virtual wall thermostat is not listed:
  its flags are the wall thermostat's.
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
  **MORE** opens a menu; the arrow keys move in it, Escape closes it:
  - **Eco** and **Comfort** set the room's eco or comfort temperature,
    shown under each, and keep the room's mode:
    in Auto until the next switch point,
    in Manual held, on holiday until the holiday's end.
  - **Hold (Manual)**, a switch shown while the room is in Auto:
    on, Eco and Comfort put the room into Manual, holding the temperature.
    It stays as set for the tile until the page is loaded again.
  - **Holiday…** opens the holiday form:
    a temperature kept until a date and time, then back to Auto.
  A room on holiday has MORE pressed, saying until when:
  "Holiday until 24.12., 18:00", the year added where it is not this one.
- With the room's thermostats in different modes, for example after another
  program set one of them, the modes in use are outlined,
  and the head's crossed arrows list each thermostat's mode.
- The **⋯** menu starts a **Boost** (the valves open by the thermostats' boost
  settings for their boost time, then the room returns to its mode),
  locks or unlocks the buttons on the room's thermostats,
  deletes a room that has no devices,
  and with **Repair…** opens the room's repairs under the tile (Repairs, below).
- The room name or the **i** button opens the device list:
  every device with its type's icon before its name (a radiator, a wall
  thermostat, a window, a leaf for the eco button), its values (valve, set
  and measured temperature, offset), its problems as icons that list them,
  as in the tile's head, a write not confirmed among them,
  and badges for lock, window and mode.
  A device the Cube has not heard for some minutes keeps the values of its
  last report, greyed, with the badge "out of date" and its age in place of
  "not reported".
  Expert mode adds each device's type, serial number and radio address,
  as "S/N MEQ0000103 · RF 0a0103": the serial is printed on the device,
  the radio address is not.
  A device without a name is shown by its serial number, here and on every
  page, and the page asks the server for a device by its serial.
  Valve, measured temperature, window and a low battery show how old they are,
  a value's age on the line under its name;
  in expert mode also the exact time, under the value on the same line,
  and when the device was last heard from.
  Expert mode lists the room's virtual wall thermostat after the devices,
  a house with a thermometer its icon,
  with the devices its temperature comes from, by serial, and its two settings:
  how a room without a wall thermostat takes its temperature
  (**Median** or **Mean** of its radiator thermostats)
  and its radio address, which matters only to clients of the LAN proxy
  whose device numbers collide; **Save** writes them.
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
    A device that has not shown a write by its deadline says which values first,
    "Not confirmed by the device: eco, Monday's program".
    A device moved to another room gets that room's settings,
    as when it is put into a room (System settings → Cubes, **Pair a device**),
    where the role has the capability `configure`.
    **Remove from the Cube** asks in the row,
    then deletes the device from the Cube:
    the Cube resets it to its factory settings,
    and it has to be paired again to be used again.
    A device out of range or out of battery stays in the Cube's memory
    and comes back under the devices without a room.
    After a move, a removal or a device put into a room,
    the message adds that the room's devices take it at their next wakeup
    and that the log says when it is done:
    the log's **device change** line, within about 15 minutes,
    a warning where the Cube still holds memory slots for it
    or a device of the room was left with a communication error or a link error.
  - **Week programme**:
    one bar per day.
    - A click on a day selects it together with every day that has the same programme.
    - With **Only {day}** on, changes apply to that day alone.
      Off, they apply to all selected days.
    - **Copy {day} to …** copies the day to the days you choose.
    - A switch point's time is typed in full, hours and minutes;
      the rows take the order of their times at the next change or at **Send**.
    - **Reset day** and **Reset all** go back to what the Cube has.
    - **Send** writes only the days that changed,
      naming the programme it was loaded as (its tag, `If-Match`).
      Where the programme was changed elsewhere since,
      or a write to it is still on its way, nothing is sent:
      the edits stay, and **Load the current programme** replaces them
      with the programme as it is now.
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

### Repairs

For a role with `repair` (the owner's and the manager's),
**Repair…** opens a box of repairs:
in a room tile's **⋯** menu, under the tile;
in a device's row of the device list and of the devices without a room,
under the row;
in a Cube's panel (Cube settings, System settings → Cubes), under its actions.
Each repair is named and says what it does;
the box asks the server what each would send now, sending nothing,
and shows its cost under it:
the radiograms, about how much of the hourly radio budget they take,
the memory slots, those held until the devices' next wakeup, and the wakeups;
or that it costs no radio time, a reset or a list write.
Expert mode adds the lines themselves.
A repair with nothing to do says why, as the server words it,
and one the Cube's budget would stop halfway says that it cannot run now.
**Run…** asks in its place, naming the cost again;
**Run now** sends exactly what was shown,
and the box says how many lines the Cube accepted,
that the devices take radiograms at their next wakeup and the log says when
it is done (its **device change** line),
or for resets whether the Cube's device list still shows the error.
**Close** closes the box; it stays open while the tile is drawn again.

- In a room: **Reset the communication errors** of its devices, free;
  **Name the room's master**, where the Cube's list names no thermostat of
  the room, the wall thermostat, else the first radiator thermostat,
  in the list only;
  **Re-link the room**, each device's links and room sent again,
  adding only;
  **Unlink all and re-link**, with `structure` as well,
  every link of the room removed and then built again:
  its **Run…** first warns, naming the cost,
  that it has not yet run on real hardware,
  and that a run that stops halfway leaves the room's devices without their
  partners until it is run again;
  **Continue** asks once more, and only that question's
  **Unlink all and re-link now** sends it.
- At a device: **Re-send the configuration**, for a thermostat,
  the temperatures and week programme most of the room's other thermostats
  share, where its own differ;
  **Place in its listed room**, by radio, where the Cube has it in another
  room or none;
  **List where the Cube has it**, in the list only,
  asking first for the name of a room the list does not have.
- In a Cube's panel: **Reset the communication errors** of every device.

## Log

Shows the Cube's last 20 events, newest first, and new ones on top as they
arrive: device states,
configuration and name changes, connections to the Cube, pairing,
restores and errors.
A lost connection is one line, however often the server tries again
until the Cube is back.
A connection to the Cube is one line, "connected: 12 devices in 5 rooms,
firmware 1.1.3, duty cycle 3 %, 180 slots free", after a loss with how long the
Cube was lost; what changed meanwhile, while disconnected or while the server
was down, follows it, a line each ("Room “Bath” renamed “Bathroom” (while
disconnected)").
A list or configuration line says what changed: "MEQ0000101 “Window” moved
from Bath to Kitchen", "comfort 21.0 → 21.5", "Monday's program changed".
The server's own lines are there too: its start with its version, its stop
and why, the settings file read again with the names of what changed, a
restart, a reboot or an update asked for and its outcome, the backups, their
prune and the log's, a restore started; and a Cube's new address or its
leaving the LAN.
A line someone asked for says by whom ("backup 12 taken (by hand), by Anna").
Every change made through the web UI, the API or `massimo user` is a line
of its own, with who made it: "Cube changed KEQ0000000: name – → Upstairs, by
Anna", "setpoint Window: mode manual, temperature 21.5, by Anna",
"user added Zed, by the command line"; a change of a device shows under the
device too, and the Cube's answer follows as its own line.
A command's steps on the radio are lines of the queue
("command acknowledged by the Cube; duty cycle 3 %, 49 free memory slots"),
queued and sent at debug, a discard, a retry and a failure as warnings,
and so are the queue's holds for the duty cycle and the memory slots.
The room checks (`docs/api-v1.md`, "The room checks") have lines of their own:
**room disagrees**, a room whose thermostats differ for ten minutes, each named
with its setting ("Bedroom: its thermostats differ in setpoint for 11 min:
Window: manual set 21.0, Door: manual set 18.0"), then the room's state sent
again to them and from where ("as Massimo last set it", "as its wall thermostat
has it"), or why not, and their agreeing again;
**programme drift**, a thermostat that does not follow its week programme while
the room's others follow theirs ("does not follow its week programme: 12.0
instead of 21.0 since the switch point at 18:20, …; its clock or its programme is
suspect"), with "it shows the programme an hour off" where it does and the
radiogram that failed, and its following again.
A write its devices do not show by its deadline is a **not confirmed** line,
a warning that names the write, the room and each device with what it shows
("setpoint to Bedroom: not confirmed after 5 min: Window (mode auto, setpoint
20.0)"); a device that shows it later has a line of its own ("setpoint:
confirmed after 70 s"). The warning is settled by a mark alone.
The LAN proxy's clients have lines of their own: a client connected or gone,
turned away, its deletes, a pairing for it and the proxy's warnings;
a line a client passes on to the Cube is a command like the web UI's.
The sign-ins, failed ones and lockouts are lines for those who may see the
audit alone.
A line of a command has a button, **command 42**, which opens the command
under the line: what was asked and its state, by whom, its steps with what
the Cube made of them, and for those who may see the audit its audit row;
the button says whether it is open and closes it again.
A device's state line that follows a command sent to it names that command too.
The level choice "debug and above" also shows the debug lines, such as the
greeting's hello, room and device list and configurations, each with its
summary, which the log leaves out at "info and above", the default;
"warnings" keeps the lost connections and the failures.
A line names its device by its room and name ("Kitchen › Window"),
one without a name by its room and type ("Kitchen › Radiator thermostat");
until the rooms are loaded, by the name a state line kept, else its type
or serial number.
The serial number is the name's tooltip and, with the radio address after
it, shows in expert mode only;
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
and another Cube in the header starts the log over with every device.

A warning or an error of any Cube's log, or of the server's,
stays a problem until a later line settles it
or the person marks it seen (`docs/api-v1.md`, "GET /api/v1/log/alerts"):
a lost connection until the next one,
a failed command until the same one is acknowledged,
a failed backup until the next, a hold until it ends,
a room whose thermostats disagree until they agree again,
a thermostat off its week programme until it follows it again;
a device's communication error, link error and low battery
while the device has them, at the line that set them;
a communication error's line names the radiogram that failed,
"communication error: TimeInformation (0x03)", as the badge's tooltip does.
Its line is highlighted, a bar and its mark in amber or red,
with **Mark seen**; **Mark all as seen** beside **Clear** marks every one.
Opening the log marks nothing.
The marks follow a signed-in user with `preferences` (the preference `log.seen`);
anyone else's stay in the browser.
The page asks at every poll.
A device that reports while its communication error is set is reset by the server,
once a day at most: the log's **communication error reset** line.
Where the error is back within that day,
at the device's next report or later,
the log's **error watch** line says so at warn, once a day,
with the radiogram that failed,
and the server leaves the device alone until the error has stayed away a day;
a communication error or a link error set for 24 hours
is an **error watch** line at error,
also for a device that sends nothing any more;
each stays a problem until the same flag clears,
an **error watch** line at info.
The server keeps its log in its store across restarts and updates,
for the days `log.keep_days` says (30 by default; docs/api-v1.md,
"GET and PUT /api/v1/settings").
With more than one Cube, a **Cubes** choice comes first in the head:
**this Cube**, the header's, with the server's own lines (the default),
or **all Cubes**, every Cube's lines with the server's own once
(`GET /api/v1/log`).
Under all Cubes a line names its Cube first ("Upstairs › Kitchen › Window"),
a Cube's own line, such as its connection, the Cube alone,
and the server's own lines no Cube;
the device choice lists every Cube's devices, a group for each Cube,
and a radio address there names that address on every Cube.
The page then loads every Cube's rooms once, for the names,
and asks for the new lines every 10 seconds while the log is shown,
in place of the header's Cube's stream;
another Cube in the header leaves the log as it is.
The choices in the head show one kind of event (all kinds by default,
grouped by their source), one source or one device (all by default:
System, Configuration, the Cube, Devices, the queue, the server, the proxy,
then each device by name and serial number),
the level and above (info by default) and, for those who
may see the users, who made a change (everyone by default);
the server answers what matches,
and the new lines follow the choices too.
The filter matches any part of a line (time, kind, the device's room, name,
serial number and radio address, text).
**Clear** sets the filter, the Cubes, the kind, the source or device, the level
and who back to their defaults and loads the log again, the range staying;
it is off while none of them is set.
The choices are in the address after `#log`,
each left out at its default:
`cubes=all`, `range=` one of `10m`, `1h`, `today`, `week`, `month` and `all`,
`kind=` an event's kind, `source=` one of `system`, `config`, `cube`, `device`,
`queue`, `server` and `proxy`,
`device=` a device's serial number
(a link's radio address takes the serial once the devices are loaded),
`level=` `debug` or `warn`, and `user=` a login name,
so that a reload, a bookmark or a link opens the log with them,
and the log's button opens it with the choices it had.
A choice changes the address in place, so back still leaves the log.
A value the page does not know is dropped from the address,
and so is a device the Cube does not have, once its devices are loaded.
The filter's text is not in the address.

## Personal settings

☰ → **Personal settings** opens the personal settings, a page built as System settings is:
its title stays under the header while the page scrolls,
with who is signed in under it (with the role, or the API key and whose it is),
**Sign out** at its right end, which ends the session,
and **Sections** (a bulleted list) beside it,
which lists the page's cards with their icons and brings the one chosen into view.
An API key or the server's token is only forgotten in this browser
(**Forget key**, with a key),
and an open server says that it is open to everyone and has nothing to sign out of.
The page has three cards, **My profile**, **My settings** and **Notifications**,
each with its icon in its head, that fold as System settings' do;
from 900 px they stand two side by side, and My settings takes the whole width
while its colour editor is open.
The addresses of the pages they were, `#profile`, `#prefs` and `#notify`,
open the page at their card.

### My profile

**My profile** is for a user signed in (not on an open server,
nor with the server's API token).
Behind the last button of each group below, an ⓘ opens what its buttons do,
as the settings' ⓘ do:

- **Display name**, the name the personal page and the users list show,
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
- **Linked accounts**, where a sign-in provider is enabled or an account is linked:
  the own accounts at the providers, each with its e-mail or user ID,
  what the provider said of it at the last sign-in (name, user name, groups)
  and when, and **Remove**,
  which is refused for the last way the user signs in
  (no password and no other linked account);
  **Link Nextcloud** per enabled provider goes there to sign in
  and back to My profile, which shows the account before it is linked:
  its e-mail, name, user name, user ID and groups,
  and "This replaces your link to …" where the user has one there,
  with **Link** and **Cancel**;
  an account another user has, or one in none of the provider's groups,
  comes back refused, saying so.
  The app shows no **Link** buttons, as no provider buttons on its sign-in.

### My settings

**My settings**, the page's settings:

Signed in with a role that has the capability `preferences` (manager,
operator and viewer by default),
the settings and your own colour schemes follow you:
the server keeps them for you,
and every browser you sign in on takes them at sign-in,
the first one handing over what it had.
Once the server has your schemes, its list is the list:
a scheme deleted in one browser goes from the others when they next load the page.
Beside each of **Poll every**, **Expert mode**, **Cube clock shown in**,
**Language**, **Mode**, **Colour scheme**, **Blinking signs**,
**Window icon** and **Radio icon**,
in a column of its own at the right headed **only on this device**,
a switch keeps that setting's value in this browser instead (its tooltip names the setting),
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
  (from 4.5 it reads well, under 3 badly),
  and in front of the warning icons' colour its contrast on a tile
  (from 3 they stand out).
  A scheme saved before a colour was added takes the default's for it.
  **Blinking signs**: whether a warning's triangle and a failure's cross blink,
  so that they are not missed; as the system (blinking unless the device asks
  for reduced motion), on, at first (five times when a sign comes) or off.
  **Window icon**: an open window drawn as the opening sash (the default),
  its mark inside its frame,
  or as the paned window of the original MAX! software,
  its mark at the bottom right;
  a window contact's icon follows.
  **Radio icon**: a radio fault drawn as the Wi-Fi fan (the default)
  or as the tower of the thermostats' display, its mark at the bottom right.
  Beside each choice it is drawn as the page shows it, plain and with its marks,
  and the page is drawn again at once.
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

### Notifications

**Notifications** chooses which events notify you and where,
for a user whose role has the capability `preferences` (manager,
operator and viewer by default);
a guest, a server without users and the server's API key see a line saying why
they have none.
The server decides what to tell you, from the Cubes' events,
and sends it to your devices by Web Push (docs/api-v1.md, "Notifications"),
also while no page is open.

- **On this device**:
  whether this browser is notified,
  with **Turn on on this device** or **Turn off on this device**.
  Turning on asks the browser's permission once,
  subscribes with the server's key and tells the server,
  under the **Name of this device** given before,
  or the browser's own name when it is left empty.
  The browser must reach the page over HTTPS, or from its own address;
  without push, or with notifications blocked for the page, a line says so.
  On an iPhone or iPad the page has to be added to the Home Screen first.
- **What notifies you**:
  a switch per kind, with its delay or threshold, each with its help:
  a window open (after 30 min), frost (below 8 °C), a low battery, a battery getting weak,
  a device's error, the Cube out of reach (after 10 min) and back,
  and, off by default, a device that sends no news, a device paired,
  a new Cube on the network and a new version of Massimo.
  A change is saved a second after the last one;
  a delay is 1 to 1440 minutes, the threshold −10 to 20 °C,
  and a value outside is not saved:
  its box turns red, with the range in amber before it,
  until it holds one inside, whatever a save meanwhile says.
  One kind notifies once per device per half hour.
- **Your devices**:
  every browser and app notified for you,
  with its name, when it was added and last notified,
  and the deliveries that failed since;
  this browser is marked, and **Remove** ends one.
  Revoking a session or a key under My profile removes the devices it subscribed.

A line under the lists says what came of reading them, of a save,
and of turning this browser on or off or removing a device.
Each clears only its own outcome when it next succeeds,
so the outcome of turning on stays after the lists are read again,
and the page opened again starts with the line empty.

While the page is open,
a notification shows at its foot for ten seconds instead,
in the toast the Cube search's outcome uses (`showToast`),
in the page's language, with × to close it;
the same one from the stream and from the push shows once.
The page is at most 640 px wide, as My settings is.

**The installable page.**
`manifest.webmanifest` names the page, its icons and its colours,
so that a phone's browser offers to add it to the Home Screen,
where it opens without the browser's bar.
Its icons are Massimo's mark, from `ui/brand/` (`make ui-brand`,
`tools/brand/README.md`): the mark in white on a blue rounded square,
and for the launchers that cut an icon to a shape of their own (Android's
maskable icons, the Home Screen of iOS) the same blue as a full square,
the mark within the circle they keep.
The browser's tab shows the small mark: `favicon.svg`,
in the colours of the browser's own light or dark,
and `favicon.ico` at `/favicon.ico` for browsers without SVG icons.
Its service worker, `sw.js`, beside the page so that it covers the page,
shows a notification in the language the page last told it
(the texts of the page's tables, kept for when the server is out of reach),
or hands it to the page in front, and a tap opens the page.
It caches nothing else: the page loads from the server as before.

## Cube settings

☰ → **Cube settings** shows the panel of the Cube chosen in the header,
as System settings → Cubes draws it, for a role with `cubes` or with `pair`
(the pairing alone for `pair`).
The page's row, **Cube settings** filled among the settings pages,
stays under the header as the page scrolls, as every page's does,
with **Sections** at its right end: the Cube and Backup and restore.
Under it the panel is open and stays open: its head, the Cube's sign, name,
serial, radio budget and pill, is its first row, and a click on it folds
nothing.
The page follows the Cube choice and every poll, as the card does,
and what it saves shows in System settings.
A panel is drawn in one place at a time:
while Cube settings is shown, System settings' list does not hold it.
Under the panel the Cube's **Backup and restore** (with `backup`),
as System settings → Backup and restore shows it for each Cube.

## System info

☰ → **System info** is for every role with `read`, a viewer's too:
its row stays under the header while the page scrolls,
with the viewer's title in it, the one settings page that role has.
The facts are the server's, a Cube's in its panel under System settings → **Cubes**.
The page is a table of the server's version and what it was built with,
when it started, as the server's clock read it, to the second in its zone,
then how long ago,
the server's time in its own zone, ticking,
with a note where it differs from this browser's clock by more than 5 s,
both times in this browser's zone when the pointer rests on them,
the certificate the browser sees,
how it was installed (the Debian package, Homebrew, the Windows service,
a systemd or launchd service, or other: by hand, a container or another supervisor),
the system it runs on with its processor,
the host's full name, the host bold and the domain as it is,
the interfaces the server listens on,
a line each with the name, the MAC and the addresses,
an asterisk marking the web interface's,
the Cube proxies' listeners, a line each with the Cube's serial,
the address and "listening",
or a notice for one that does not listen: the error, since when,
and the next try when the pointer rests on it,
counted in a badge in the page's row while it fails (docs/proxy.md),
and the address the server sees this browser at
(`docs/api-v1.md`, "GET /api/v1/system").
The certificate is the server's own (its source, names and expiry; none on plain HTTP),
or, where a trusted proxy serves the page, the proxy's, "(proxied)":
its names, expiry and issuer as the server read it, "not trusted" where it is not valid,
with the address the proxy serves, a link to it in a new window and its copy icon;
"being read…" while the server reads it,
which it does within seconds of the first request:
the page asks again every 2 s while it is shown,
and says "could not be read" when 30 s have not brought it;
where the proxy names no scheme (no `X-Forwarded-Proto`), the page's own scheme
and address, and that with that header the server reads the proxy's certificate;
a page on HTTPS from a server without TLS says "(proxied)" too,
and that `-trusted-proxies` would show more.
The server's version copies on a click, or by its copy icon, whole, with
what it was built with (the header's does too, without the icon);
so do each Cube proxy's serial, the serial it answers discovery with ("as …",
where it answers discovery) and its address, which the original software connects to.
A copy says "Copied"; on a page served over plain HTTP the browser gives no
clipboard to the page, and the copy goes through its copy command instead.
At its end, for a holder of `settings`,
**Store** opens to what `massimo store info` shows:
the file, the schema version, the node and tenant and the rows per table,
its head the file, the schema and the rows in all;
**Settings in effect** to what `massimo config` shows: the version, the
settings file, the data directory, the listen address, TLS, the log level
and every `MASSIMO_*` variable with where it came from, a key, an IV, a
token or a password as "set, not shown"
(`docs/api-v1.md`, "GET /api/v1/store, GET /api/v1/config").

## System settings

☰ → **System settings** holds what is the server's,
each part for a role with its capability;
a role with none of them (the viewer's, a guest's) has no such entry in ☰.

The page is a card per part, as wide as the page:
Cubes first, then Users, Roles,
**Capabilities for every role**, and Backup and restore last;
from 900 px Users, Roles and Capabilities flow in two columns,
none split, the break where their heights put it.
The page's row stays under the header while the page scrolls,
with **Sections** at its right end (a bulleted list):
it lists the cards the role sees, marks the one in view and brings the one
chosen into view, opened.
A card folds with the button in its heading,
which also says what it holds at a glance (**up to date**, **1 served**,
**2 users**, **all on**, **3 kept**), also while it is folded;
the browser keeps which cards are folded.

- **Cubes**:
  the Cubes the server serves, kept in its store;
  the card's head counts them, at every poll whether a panel is open or not:
  how many are served, those not connected as their signs show them
  (red "not connected" for a cross, amber "connection held" or "rebooting"
  for a triangle), the proxies that do not listen,
  the Cubes found at another address and those found and not served.
  Each is in a panel whose head shows the Cube's sign, the name,
  the serial and the address without the port,
  the duty cycle, the free slots and the radiograms pending,
  a pill with every problem of the Cube (its link, its proxy, its settings file
  and its devices with theirs, as the header's), top-right whatever wraps,
  a green check where nothing is wrong, whose click opens its list
  and not the panel, and under them where its settings come from,
  all of it drawn again at every poll, and opens with a click:
  its own badges as its top line (its link where it is not connected,
  a proxy that does not listen, a settings file that differs,
  a Cube found elsewhere than its fixed address, the source),
  a table of its facts for everyone who sees the panel
  (serial, address, port, firmware, radio address, the Cube's clock,
  duty cycle, free slots, pending and queued, refreshed while it is open;
  the serial, the address and the radio address copy with a click on them
  or their copy icon),
  and above the facts a notice with since when where the server knows why it
  is not connected (`link` of `docs/api-v1.md`, "GET /api/v1/cubes"):
  its lost connection held open until a time, so that it carries on when the
  network is back, the Cube still holding the server's old connection
  (it lets go after up to about 40 minutes, or a reboot frees it),
  or the Cube busy with another client;
  for a Cube not connected whose address lies outside this machine's networks
  (`other_network`), a notice as on Rooms, with the `netconfig` line;
  and where discovery finds a Cube with a fixed address at another one
  (`found_elsewhere`), a notice with both addresses and since when,
  and **Use the found address** and **Follow discovery**, which save either
  at once (a Cube of `-addr` or `-sn` has the notice alone);
  the card's badge counts such Cubes;
  where its settings came from (the settings file, changed here,
  or for this run only), and the fields **Name**, **Address** (empty:
  the address the Cube answers discovery from, shown in grey),
  **Follow the address the Cube answers discovery from**,
  **Proxy address** (the address of this server other programs connect to for this Cube;
  empty: :62910 when it is the only Cube), **Serial in discovery replies**
  and **Show proxy clients the virtual wall thermostat of every room**
  (docs/proxy.md, "Virtual wall thermostats").
  Under the proxy address the panel says where the proxy listens,
  and for a listener that does not, a notice with the error and since when,
  the next try when the pointer rests on it:
  the server tries it again until it listens, at most 30 s apart,
  so an address the host takes on later needs no restart (docs/proxy.md);
  the panel's head and the card's badge count such listeners.
  **Hidden devices** lists the devices hidden besides those of room 255,
  each with its name, serial number and radio address and **Show again**;
  **Hide** takes a radio address,
  with the Cube's devices to choose from.
  While the panel is open, a field not touched follows a value stored meanwhile
  (by another browser or the settings file);
  a field being edited keeps what is typed and says under it that the stored value
  changed, with the new value and **Take theirs**.
  **Save**, off while nothing differs from the Cube's settings,
  sends the changes, which apply at once:
  a hidden device leaves the tiles, the API and the original software,
  and a room that holds only hidden devices goes with it.
  **Repair…**, in the panel's actions for a role with `repair`,
  opens the Cube's repairs under them (Rooms, Repairs):
  the reset of every communication error of the Cube.
  **Reboot the Cube**, in the panel's actions for a role with `cubes.reboot`,
  restarts the Cube (`POST /api/v1/cubes/{sn}/reboot`),
  which frees one that still holds the server's old connection at once:
  in place of the button it asks first, naming the Cube's serial,
  that it is gone for about a minute and its rooms refuse commands for a while,
  the radiograms pending as the Cube last said and when, which the restart drops,
  and for a Cube busy with another client that the client may take it back;
  the head's badge then says rebooting until the Cube greets the server.
  When a line of the server's settings file says something else than the panel,
  a notice names the line and **Take the file's value** takes it.
  **Remove** stops serving the Cube;
  it keeps its settings and history in the store and is listed as found
  while discovery hears it.
  Under the panels, what discovery found and the server does not serve
  is one fold, **Found on the network, not set up**, closed at first:
  its head the found Cubes' sign and how many Cubes and Massimo proxies;
  open, each Cube with its address, DHCP flag and firmware,
  "via proxy at …" where a massimo proxy answers for it,
  in expert mode its radio and MAC address and the gateway it uses, and **Add**,
  which serves a found Cube at once;
  under a Cube whose address lies outside this machine's networks
  (`other_network`), a notice as on Rooms, with the `netconfig` line;
  each Massimo proxy with its address, the Cube it serves
  and the Massimo server serving it, its name a link to the address
  its announcement gives (`host` of `docs/api-v1.md`), without **Add**:
  it is another server's, listed so that its Cube is not added twice;
  a Cube a proxy of another server serves names that server too.
  Under them, **Lines for the settings file**, a fold of its own
  with one copy icon for all of them, serve the Cubes too
  (`/etc/massimo/env` for a server installed with `massimo install`,
  otherwise `.env`):
  the server serves a Cube once its `MASSIMO_CUBE_<SERIAL>_ADDRESS`,
  or only a `_NAME`, is there and the server has been restarted;
  with a name alone it takes the address from discovery and follows it.
  With several Cubes each also needs a `_PROXY` address (README.md,
  "Cube variables").
  A Cube served for this run only (`serve -addr` or `-sn`) cannot be changed here.

  A Cube's panel has three groups: its facts, its settings with **Hidden devices**
  and **Save**, and its actions in one row under both,
  **Pair a device**, **Reboot the Cube** and **Remove**,
  each question and the pairing's progress in a line under them;
  from 760 px the facts and the settings are side by side.
  **Pair a device**
  shows the pairing time (60 seconds, at most 255, the longest a Cube keeps)
  and **Start**,
  which puts the Cube into pairing mode;
  then the device's pairing button is pressed, as its manual says.
  A time out of range outlines the field in red as it is typed,
  the range in amber before it, and **Start** goes back to the field instead.
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
  and **Create key**, which shows the new key once;
  where a sign-in provider is enabled, the user's identities at the providers,
  each with its e-mail, or its user ID there,
  and whether it is bound or waits for its first sign-in,
  what the provider said of it at the last sign-in (name, user name, groups) and when,
  **Change** of the e-mail or user ID while no sign-in used it,
  **Remove**, and **Assign** to the user, at a provider,
  by an e-mail or by the user ID there.
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
- **Sign-in providers** (with `auth`, the owner's alone by default),
  explained by the ⓘ in its head:
  the redirect URI to register at a provider,
  the page's address and `api/v1/login/oidc/callback`, with its copy icon;
  every OpenID Connect provider in a panel, its name and key,
  **disabled** and **creates users** as badges,
  with its name on the button, issuer, client ID, client secret (written only:
  the line under it says whether one is set,
  or that it no longer opens, `opaque.key` lost, and is to be entered again),
  scopes, **Allowed groups** (comma-separated, any where empty)
  and the **Groups claim** they are read from (`groups` where empty), **Enabled**,
  and **Create users who sign in** with their role,
  which holds no administrative capability, and their e-mail domains;
  **Save**, which for an enabled provider first reads its discovery document
  and says what is wrong where the issuer is not reached or named otherwise,
  **Remove**; **Add a provider** opens the form for one, with its key.
- **Integrations** (with `integrations`, the owner's and the manager's by
  default; docs/api-v1.md, "Integrations"), explained by the ⓘ in its head:
  with `settings`, **Integrations may ask to be registered**;
  each request waiting in a panel whose head shows its code, to compare
  with the one the integration shows,
  and **seen on the LAN** or **not seen on the LAN**;
  in it where it came from, its installation id, address, version and
  until when it waits, a switch per capability of the role `integration`,
  all on, for its maximum,
  **Registered before as**, where an integration of its kind is
  registered, the same installation chosen,
  **Refuse**, which asks first, and **Approve**.
  Every integration in a panel, its name and key, **disabled** and
  **new installation** as badges:
  its key, installation id, address, version, when its key was issued and
  whether the LAN saw it;
  for a call from another installation, **Accept the new installation**;
  **Enabled**, its maximum with **Save the maximum**,
  its users (read when the panel opens), the system user first,
  each with its id's first eight characters, its level and when it acted
  last, and **Acts as**: its own user or a Massimo user, the one of the
  same name proposed, with **Link**; and **Revoke**, which asks first.
  Under them the Home Assistant installations the server saw on the LAN,
  each **registered** or **not registered**, with **Look again**;
  where the server does not look, why, naming Announce on the LAN and
  what to change: Off, Automatic on the loopback, a change waiting for
  the restart, or `-announce` or `MASSIMO_ANNOUNCE`, which decides;
  with `settings`, **Open Maintenance**, but where the flag or the
  variable decides.
  In **Users**, an integration's users carry **user of** its key (the
  integration's own **integration** and its key), and a linked one
  **acts as** the user; a user that integration users act as counts them
  as a badge and names them at the top of its panel.
- **Roles** (with `roles`):
  every role in a panel with its display name and a switch per capability,
  **Save**, off while nothing differs from the role,
  **Disable** (with `roles.disable`), **Remove** for a created role nobody has,
  and **Add a role**; the owner cannot be changed.
- **Capabilities for every role** (with `capabilities`):
  a switch per capability, which switches it off for every role but the owner.
- **Backup and restore** (with `backup`):
  a fold per Cube, its sign, name and serial and how many backups the server
  keeps of it in its head, a single Cube's open; the card's head counts them all.
  In each, and on Cube settings for the chosen Cube,
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

## Maintenance

☰ → **Maintenance** keeps the server itself going,
in two cards as wide as the page, **Software update** and **Maintenance**,
under the page's row with **Sections** at its right end, as System settings has.
It is for every role that has System settings, Software update being for all of them,
and for one with `update`, `restart` or `reboot` alone;
a viewer has no such entry in ☰.
The server's sign, when it says an update is out, opens it at Software update.

- **Software update**:
  the running version, the update channel and whether a newer release is out.
  A holder of `settings` chooses the channel there:
  the build's own flavour, or devel, nightly, alpha, beta, rc or stable,
  each taking its own releases and those of every more stable one.
  The choice is the setting `update.channel`, none for the build's flavour
  (`docs/api-v1.md`, "GET and PUT /api/v1/settings"),
  and the release check asks again at once.
  On a box apt updates, the first choice is apt's suite,
  and another channel is the one the next update switches apt's source to:
  the note says so, and nothing is downgraded.
  Where something else decides the channel, it is shown fixed, with the reason:
  brew's formula, which another channel's formula replaces
  (the rolling ones from the tap `metaworx/massimo-rolling`);
  or `MASSIMO_UPDATE_CHANNEL`, until it is removed.
  On a box with the update helper installed (packaging/README.md, "Updating"),
  **Update** (with `update`) installs it and restarts the server.
  If the new version does not come up, the old one is put back.
  On a box the Debian package installed it says "Installed by apt",
  the channel is the suite of the apt source,
  and **Update** has the package's helper run apt;
  nothing is put back there, and the last update says how to go back
  (packaging/README.md, "Debian package").
  Under Homebrew's service it says "Installed by brew",
  and **Update** runs `brew upgrade` as the service's user
  (packaging/README.md, "Homebrew").
  While the helper works, **Check for updates** and **Update** are greyed out,
  also after a reload of the page, until it reports that it is done.
  **Last updates** lists the last three under the version lines:
  the version reached,
  or the one asked for with "under way" until the next start,
  when, and who started it,
  or "outside Massimo" with apt or brew where Massimo did not start it;
  one the helper ended without its version says how it ended (`docs/api-v1.md`,
  "GET /api/v1/update?refresh=1").
- **Maintenance** (with `restart`, `reboot` for its reboot,
  or `settings` for its announcement, its Cube intervals and its last two parts),
  in this order: **Announce on the LAN**, the three Cube intervals,
  **Settings file**, **Restart** and **Reboot the computer**,
  each under a bold title:
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
  at the next one are listed under it, by name, and counted in the card's head,
  with the card's own settings that wait for it.
  **Announce on the LAN** (with `settings`) chooses **Automatic**, **On**
  or **Off** for the setting `announce`: the announcement by mDNS for
  `massimo discover`, the apps and Home Assistant, Automatic on unless the
  server listens on the loopback, as behind a reverse proxy.
  The line under it says whether the server announces itself now, or that
  a change waits for the next restart; where `-announce` or
  `MASSIMO_ANNOUNCE` decides, the choice is read-only and names it
  (`docs/api-v1.md`, "GET and PUT /api/v1/settings").
  **Poll the Cubes every**, **Refresh the Cubes every** and **Look for
  Cubes every** (with `settings`) choose the intervals `poll_interval`
  (10 s to 1 h, 10 s by default), `refresh_interval` (Off, 30 min to
  7 days, 6 h by default) and `discover_interval` (Off, 1 min to 1 day,
  30 min by default) from a list of steps, and **Re-read the radio
  budget after a hold every** the duty cycle's hold `duty_wait` (1 min to
  1 h, 10 min by default); a value set through the API
  that is none of them is listed as well, and the default is stored as
  none.
  The line under each says what it does: the device list asked for,
  which costs the Cube no radio time; a new connection that reads
  everything the Cube holds; the search for Cubes on the LAN; how long
  radio commands wait at a used-up duty cycle before the Cube's budget is
  read again.
  It then gives this run's value, or says that a change waits for the
  next restart; where `-poll`, `-refresh` or `-discover-interval`
  decides, the choice is read-only and names the flag.
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
The page is HTML, CSS and JavaScript under `internal/server/ui/`,
embedded in the binary.
Its regions move from plain scripts to parts, one region at a time,
and the parts are the one thing that is built (below).
`index.html` is the skeleton: the head, the icons, the sections of the page
and the scripts, with marker comments.
At start the server replaces each marker, such as `<!-- partials: prefs -->`,
with its partials: the header, the home notice, the first owner's form,
the panels of the settings pages, the Cube page, the House tile,
the devices without a room, the log, the sign-in dialog, the toast,
and at the end `<!-- partials: templates -->` with the templates,
from the files in `ui/partials/` in the order `uiPartials` in
`internal/server/ui.go` lists them, and serves the result at `/`;
System settings' cards fill three markers,
before, in and after the cards that flow in two columns,
and Maintenance's two cards one, `maint`.
A partial is a file per feature,
and its first line names the script that uses it.
What a script builds again and again, a Cube's panel, a log line, a scheme row,
is a `<template>` there, which the script clones with `fromTemplate`,
fills with `textContent` and wires with `addEventListener`.
The shared pieces (buttons, the `.field` rows, forms, confirmations in the page)
come from `components.js`; the scripts never use `innerHTML`.
The week programme editor is an element of its own, `<massimo-week-program>`
(`week-program.js`), so that a Home Assistant card can use the same editor:
it holds the editor's behaviour, works on a model the page keeps,
builds its controls with a kit the page gives it (`components.js`, `t()`),
and tells the page of every edit with a `change` event;
the programme tab puts it in a form with Cancel, Send and the two resets.
A panel that opens, a Cube's, a user's or a role's, is a `detailsPanel`:
its head the name, the serial, login or key, and the badges,
its body the form cloned from the feature's template.
Backup and restore is a box per Cube (`backupBox` in `backup.js`),
built once per Cube and page and kept while its page is shown again,
what a Cube has kept by its serial (`backupOf`), so that both pages show the same.
A long page is a grid of section cards (`section.section-card`,
its heading's `button.section-fold` folding it):
`sectionCards` makes them fold and keeps the fold in the browser,
`sectionBadges` shows what a card holds in its head,
`pageBar` fills the sections menu of its title's row kept at the top
(with a part without a fold marked `.page-section`, as Cube settings' panel),
`pageSwitch` puts the settings pages in its title's place (`SETTINGS_PAGES` in `pages.js`,
as ☰'s pages are `MENU_VIEWS`; `markSettingsPages` shows the title for one page alone),
`fitPageTabs` gives the page shown's button the longest name's width,
and `backToTop` wires the two ↑, the floating one and the compacted header's;
`showToast` shows the page's one toast at its foot (a notification, the search's outcome),
`pickerMenu` makes a dropdown the choice of one item (the Cube choice),
its items `menuItem`s, a Cube's link its network sign (`cubeSignEl`, `LINK_SIGN`);
a device's serial and radio address are a `deviceIds`;
a list of facts is a `kvList`, of counts a `countList`;
an action with what it does and its button is an `.action-row`;
`stickyHeader` keeps the header at the top and compacts it once scrolled.
A state is a sign, a colour and a mark, so that it reads without its colour:
nothing for a state that is fine, a triangle for a warning (amber),
a cross for a failure (red).
`signEl` draws a kind of thing in a state (`signOf`: the radio and the window
as the person chooses, `ICON_CHOICES`, the Wi-Fi fan or the tower,
the opening sash with the mark inside its frame or the paned window with it
at the bottom right, as the original MAX! software has it;
`SIGNS`: the network of a Cube's link with it on top, the cog for the settings),
the drawing and the mark two parts, so that the mark alone blinks;
`badgeEl` leads a warning's or a failure's badge with its mark (`markEl`)
and sets it in bold, while a badge that names a state, such as a mode, stays regular;
`keepBlink` keeps a mark's blinking where it was across the redraws that make a new
element of it.
The icons draw in the token `warn-mark`, a brighter amber than the text's `warn`,
at a contrast of 3 on a tile, the text at 4.5 (`TestUIDefaultSchemeMatchesCSS`);
the filled amber, `fill-warn`, is the Manual mode's alone.
`problemsEl` is the list of what is wrong in a place, wherever it is put,
its entries one format at every level (`problemEntry`):
the icon in a column of its own, the name and a grey detail beside it,
the badges under the name, and the entries under it (`subs`) indented to the name,
a line down from its icon.
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

The scripts are classic scripts sharing the page's global scope,
a script for each region of the page, in the order the page loads them:

- `i18n.js`, the texts and `t()`; `components.js`, the shared builders,
  the menus and the page's chrome;
- `app.js`, the core: the state, the settings and their scopes, the API,
  the helpers, the ages of values, help, the Cube clock, loading, polling,
  the event stream, the language and `init`;
- `header.js`, the header: the radio budget,
  the last update and the server's clock, the Cube choice and the problems list,
  the version, the TLS notice and the server's sign;
  `home.js`, the home notice and the search for Cubes;
  `prefs.js`, the wiring of My settings' rows and the language choice;
- `rooms.js`, a room's facts, the drawing of Rooms, a device's state event
  and the actions behind a tile; `tiles.js`, a room's tile;
  `devices.js`, its device rows; `unlisted.js`, the devices without a room;
  `house.js`, the House tile;
- `week-program.js` and `config.js`, a room's configuration;
- `log.js`, the log and its sign for unseen problems;
  `pages.js`, the pages, the main menu, the address and the scroll;
- `update.js`, Software update; `maintenance.js`, the Maintenance card;
  `system-info.js`, System info;
- `cubes.js`, System settings → Cubes, the found Cubes and the Cube page;
  `cube-panel.js`, a Cube's panel; `repair.js`, the repairs of a room,
  a device and a Cube; `pairing.js`, the pairing;
- `login.js`, `users.js`, `providers.js`, `integrations.js`,
  `backup.js` and `roles.js`, the sign-in and the cards of System settings;
- `theme.js`, the mode and the colour schemes, applied as the script loads;
  `appearance.js`, the Appearance card, the editor, export and import;
- `notifytext.js` and `notify.js`, the notifications.

A script's top-level code runs as it loads, and the `DOMContentLoaded`
listeners in the order the scripts register them,
so the order of the tags matters where one script's top-level code needs
another's (`theme.js` after `app.js`, `config.js` after `week-program.js`).
`go test ./internal/server` refuses a top-level name that two scripts declare,
and a script embedded that the page does not load
(but those of `uiScriptsOffPage` in `ui_test.go`).

The six scripts that held several regions were cut along them by
`tools/ui-split` (its `README.md`), by moves alone,
and a branch that was open meanwhile makes the same moves with it.
The tool is there while the plan UiParts lasts, and goes with it.

A region that has moved is a part:
Preact with its signals, written in TSX under `ui/src/<region>/`,
built once and updated in place.
`ui/src/index.ts` is the entry point; it registers `window.MassimoParts`,
by region the functions the plain code calls,
and no region is a part yet.
`make ui` bundles it with esbuild, through esbuild's Go API (`tools/uibundle`,
a module of its own, its `README.md`),
into `ui/parts.js`: one classic script, minified, for ES2022,
with the vendored packages' licence comments at its end.
The page loads it after the plain scripts, whose globals a part calls.
git does not keep the bundle:
every target of the Makefile that compiles Go makes it first,
and so do the scripts of the UI suite, the LAN test and the store fixture,
CI and the Homebrew formula;
`go build` without it stops at the `go:embed` line of `ui.go`, which names it.
Preact 11.2.0 and @preact/signals 2.11.4 are vendored under
`third_party/preact/` (its `README.md`);
the bundler takes them from there and refuses any other import.
`make ui-check` type-checks `ui/src/` with `tsc --noEmit` (its `tsconfig.json`:
strict, the vendored types by `paths`),
with the TypeScript `tools/ui-check/` locks (its `README.md`),
and CI's job UI types runs it.

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
Nor is the Preact the bundle holds, which `go test ./internal/server`
checks the same way (`third_party/preact/README.md`).

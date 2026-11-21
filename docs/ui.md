# Using the web UI

`massimo serve` serves the web UI at its listen address, by default
<http://127.0.0.1:8099/>. A box installed with `massimo install` serves it
on the address its `-listen` names.
The page works on a phone as well as on a desktop,
and follows the system's light or dark mode.

The page asks who you are on a card in the page, not over it,
its row **Sign in with: Name | API key** choosing the form:
a name and a password, chosen at first,
or an API key pasted into a form of its own,
which is chosen from the start once the page has seen the server ask for no key
(the owner made from `MASSIMO_API_TOKEN` has no password).
The password form holds the one password field, its fields named,
so that a password manager such as KeePassXC fills it.
The page keeps the session in the browser for 30 days from its last use;
**Sign out** in ☰, or on the personal settings, ends it.
The card's ☰ shows the version first,
then sets the language and the appearance (light, dark, as the system)
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
In the app the card has the login by fingerprint instead (docs/app.md,
"The login by fingerprint"):
**Enable login by fingerprint**, a switch under the password, its label to
its right, where the phone has a fingerprint and keeps no login for the
server, which after the sign-in asks for the fingerprint and keeps an API key
of the user's own behind it,
or says that nothing was kept, with **Try again** and **Sign in without it**;
the same switch on the form that creates the first owner;
the fingerprint first in the row **Sign in with** where a login is kept,
named with the user's name, which signs in at once with a new session from
the key, Name staying the form chosen;
a line instead of both where the page is open over plain HTTP outside the
home network, where the app sends no key;
and **Select server**, back to the app's servers,
at the card's foot and last in the card's ☰.
While it asks, the page shows the card alone, with the logo and,
at its foot, the version, and no header;
from its first moment until the server has said who is in,
the header's logo and name alone,
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

Settings typed and not saved (Cube settings, a room's virtual thermostat)
stay while the page is loaded, across its pages and the Cube chosen;
moving to another page or choosing another Cube asks nothing,
since the disks of the problem lists say where they are.
Closing the tab, reloading it or going to another site
while such settings have changes, also those still being typed or being saved,
the browser asks with its own prompt, which says nothing of the page:
leaving drops the changes; staying, every changed form is unsaved now,
with its sign and its disk, and a second later the page shows **Settings not saved**,
a dialog that lists them, red where a save failed,
each a button that closes it and opens its settings
with the focus in them, from any page, and **Close**;
the second keeps the dialog off a page that is leaving while the next one comes.
A browser asks only once the page has had a click or a key.
**Sign out** (or **Forget key**), which drops them too,
asks first with the same dialog, with **Stay** and the button that signs out;
without such changes it signs out at once.
The dialog closes by itself once the last of its settings is saved or reset,
and a sign-out it asked about then goes on, since nothing is lost any more.
A session that expires, is revoked, or ends in another tab,
asks for a sign-in and nothing more:
the page forgets it at the first request the server refuses,
so that its reads and its stream do not go on sending it until the server
locks the address out (ten failed checks a minute);
the same user signed in again finds the changes,
another user does not.
What the page was reading when the session ended it reads again after the sign-in;
what the user did, a setpoint, a save, a deletion, any request but a read,
is never sent again, whoever signs in and however much later:
it fails at once, red where its result shows, as not sent since the user was signed out.
A setpoint stepped and still waiting for its two seconds is dropped as the sign-in opens,
and its tile says so; a settings change waiting for its second waits for the sign-in.
For the same user a form's draft stays, so that it is one click on **Save** again,
and where the page's message line said that a write was not sent,
the line says it again after the page's reload, unless the reload put something there.
Another user's sign-in forgets on the page what a sign-out forgets:
the first user's settings in this browser, which the page would otherwise hand
to the second user's account, the drafts, the settings changes still waiting,
every line that said a write was not sent, and the Cubes' data, which loads again.
The stream's ticket alone, which the page asks for by itself and which changes nothing,
waits for the sign-in as a read does.
A request sent with the old key whose 401 comes after a sign-in is done
opens the sign-in once more, though the page is signed in;
a write so answered fails as any other and is not sent again.
The page that reloads by itself when the server has a newer version
waits while there are any.

## Header

The header's row has Massimo's logo at the left, the M as tall as the page buttons;
at the right, the pile, a pill that starts with the server's sign and shows
every Cube's problems;
the Cube choice, the cube in the colour of the server's link to it
and the chosen Cube's name;
then the house, **Rooms**, and **☰**, the menu of every other page,
the Cube choice as near to them as they are to each other.
They all end on one line, the logo's baseline and the buttons' lower edge.
Where the row is short, it folds step by step, as far as it must:
first the logo becomes its M, the M with the clock in its V;
then the Cube choice becomes a button with the cube alone,
whose tooltip names the chosen Cube and which opens the same list;
then the pile goes to a row of its own under them, at the left.
The M stays at every width.
The row is measured as if ↑ were there, so that scrolling never changes how it folds.

The logo lies over the word "Massimo", which a screen reader reads:
the word shows until the drawing has come (`ui/brand/brand.svg`,
made by `make ui-brand`, `tools/brand/README.md`), and stays where it never does.
The drawing has no margin: its bottom edge is the letters' baseline,
which the pile and the buttons end on.
Its colours follow the light or dark mode chosen, its letters the page's text.
The sign-in, the form that creates the first owner and the apps' connect page
show the logo too, the first two larger, above their titles.

The header stays at the top while the page scrolls.
Scrolled, it keeps its rows as they are, the logo too,
and the lines under them go until the page is back
at its top, expert mode's line of the Cube's radio budget apart.
The compacted header has **↑**, back to the top, first of the page buttons: ↑ ⌂ ☰.

- **Cube**:
  a menu that chooses the Cube when the server has several.
  Closed it shows the cube in the colour of how the server stands with the Cube,
  then the chosen Cube's name (its serial where it has none):
  green while connected, amber while its lost connection is held open
  or it reboots, red while it holds the server's old connection,
  another client has it, or it is gone;
  its tooltip adds the serial, the address and the link in words,
  and the pile shows the link's sign with its mark where it is not connected
  (see Signs, under How the page is built).
  Open, each Cube has the cube, its name, its sign after it, always,
  then an icon for each kind of problem it has (the radio, a battery,
  a window open too long, a device's link and, for one who may change the settings,
  the cog, and the disk while the Cube's settings or the settings of one of its
  rooms' virtual wall thermostats are unsaved or a save of them failed),
  each in its worst colour, and under it its serial and address,
  the list as wide as they need; the arrow keys move between them.
  Wherever a Cube is named with its sign, the cube is before its name,
  in the text's colour, and the sign after it;
  a blue cube is the sign of the Cubes found and not served.
  Open, it stays open when data comes in, the focus where it was,
  and shows what came: a Cube's new name, a problem, a Cube found.
  In the app the list ends with **Select server**, which shows the app's
  servers (docs/app.md).
  The search's entry, **Search for new Cubes…**,
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
- **The pile**, before the Cube choice:
  a pill that starts with the server's sign (below), then
  a green check where nothing is wrong on any Cube; where something is,
  an icon per kind (a Cube's link, the radio, a battery, a window
  open too long, the settings).
  The pill opens a list, whatever is wrong or not.
  Its first item is **Refresh now**, which asks the server for the current state now,
  turns while it asks and leaves the list open;
  under it, a row each, a label and its time, the times right-aligned:
  **Cube read**, how long ago the chosen Cube was read
  (the last message the server took from it, measured on the server's clock),
  **Server replied**, how long ago the server last answered the page,
  each "14 sec ago" in the page's language with its time of day as its tooltip,
  and **Current time** in the browser's time zone; they move on each second.
  While the server's sign offers something to do,
  that is the list's second item: **Software update** with the version,
  or the reload for the server's new version.
  Then the list says that nothing is wrong,
  or it lists every Cube with a problem as a heading,
  the cube before its name and its sign after it, its link's badge,
  and under it its devices with a problem
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
  Its list is as wide as its entries need, up to the page's width,
  and, as every menu of the page, has a border a little stronger than a
  card's and a shadow, in dark mode a faint light glow, and opens inside the window,
  no higher than the room below it, and scrolls in itself, not the page;
  open, it stays inside the window as its pill moves,
  when the pill grows with the problems that come or the row folds.
  An open list stays open when data comes in, and follows it:
  a problem that comes is listed, one that goes leaves the list,
  until nothing is left to list and the list says so, the green check in the pill.
- **The version** is ☰'s first row, short and small, after Massimo's icon,
  a rule under it; its tooltip is the whole version,
  "(server version)", and a click copies it.
  While the sign-in's card is open there is no header:
  the version is at the card's foot and first in its ☰.
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
  in the app **Select server**, back to the app's servers;
  and **Sign out** (Forget key for an API key or the token, none on an open server),
  in the accent and bold, as the personal page's button.
  It is highlighted while one of its pages is shown, as a page button is,
  that page's entry marked current.
  While the log holds a warning or an error not yet settled or marked seen (see Log),
  ☰ carries an amber triangle or a red cross at its corner,
  on a disc of the card's colour, so that it shows on the highlighted ☰ too,
  and the sheet of its Log entry turns amber or red, both named with the count.
  Open, it stays open when data comes in and when who is in is read again,
  the focus where it was.
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
  and another ↑ in the compacted header, first of the page buttons;
  each takes the page back to its top,
  the focus on the page's sections menu where it has one.
- **The server's sign**, first in the pile's pill,
  its state named first in the pill's label:
  the plug in green while the server answers;
  a blue "i" while an update is out,
  for one who may install it (the list's second item opens Software update);
  an amber triangle, blinking, once the server runs a new version
  (the list's second item reloads the page);
  the open plug, blinking, the pill filled red, after a poll without an answer:
  the page's other signs are greyed, their state not known,
  Cube read says how long ago the last update came,
  and the page tries again at every poll.
  After an update the page reloads by itself only when nothing is open:
  no setting being edited, no panel and none of the settings pages.
- In expert mode a line under the first shows the chosen Cube's name,
  how long ago it was read and the server replied, as under Refresh now,
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
  In the apps neither line asks for the comparison:
  the app pinned the fingerprint when the server was added (`docs/app.md`).
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
  a pause sign, amber, a room whose commands are discarded until every device
  has reported since the Cube started, its devices that have not with the badge
  **not reported**, or **out of date** with its age as the device's row says it;
  the clock, amber, values not seen for long (below);
  the sliders, amber, where the Cube and its room list differ (below);
  a suitcase, a room on holiday, which is no problem but is listed there.
  A tap on them lists the devices concerned, each once with its serial,
  its badges under its name, led by the same icons, a warning's and a failure's
  by its mark, and how long ago each came,
  for different modes each thermostat's mode,
  and for a holiday its end, "holiday until 24.12., 18:00";
  what concerns the room itself, its commands discarded and a finding that names
  no device of the room in its whole sentence, under the room's own name first;
  Escape or a click outside closes the list.
  An icon goes as soon as its problem does.
  The room's virtual wall thermostat is not listed:
  its flags are the wall thermostat's.
- **One line under the tile's head** while the room's commands are refused:
  "Room commands are discarded until every device has reported",
  as that is why a change does nothing;
  **Show** opens the list of the tile's icons, which names the devices that have not.
  The tile's other notices are in that list alone.
- **Values may be old**:
  a thermostat of the room has not been heard from for more than 24 hours,
  and its entry says how long, "Not seen for 30 hours".
  It may be out of range or its batteries flat,
  but a thermostat whose valve stays shut outside the heating season can be quiet as well.
  Its values are greyed.
- **Where the Cube and its room list differ** (`mismatches` of `docs/api-v1.md`,
  "The Cube's lines against its room list"), a badge each in the device's entry,
  with since when, "in Office for the Cube since 16.11., 00:41",
  its tooltip the whole sentence naming the device and both rooms:
  "Radiator: in Office by its configuration on the Cube, in Living room in the
  room list"; a device the room list does not have whose configuration puts it
  in the room, which can block the room's commands;
  a device the room list has and the Cube does not;
  another type on each side;
  the room's master in the room list that is no device of the Cube,
  in another room, or no thermostat;
  and a thermostat whose week programme or temperatures differ from the rest
  of the room.
  The device's row in the device list has a badge for each,
  "in Office for the Cube", "own week programme",
  the whole sentence its tooltip;
  the log has its **room list** line, a warning until the two agree again.
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
  What is typed there stays until it is saved or reset:
  when the device list closes and opens, when the rooms load again,
  when a device reports and when another Cube is chosen, each with its Cube.
  It is forgotten at a sign-out, when its Cube is no longer served,
  and when a load of the rooms from the Cube, connected, no longer gives the room
  or its virtual wall thermostat: deleted, or hidden in Cube settings;
  a Cube not connected forgets nothing.
  Opened again, the settings show what was typed at once;
  opened for the first time, they are off until they have come from the server,
  or say in their result line that they could not be read.
  After a save the settings show what the server has,
  or what was sent where they cannot be read again.
  **Save** and **Reset** say how they stand, as in Cube settings:
  off and plain while nothing is changed, in the accent colour once something is,
  with the warning mark once the focus has left them,
  the list was closed or a minute has passed,
  with the cross after a save that failed, and the check after one that went through.
  While they are unsaved, or a save failed,
  the virtual wall thermostat's problem list in its row,
  the tile's problem list in its head
  and the Cube's problem lists,
  in the header's list of every Cube and in its panel's head,
  show the disk with the corner mark,
  "Living room, virtual thermostat: not saved", amber, red after a failed save,
  outside expert mode too, where the virtual wall thermostat's row then shows
  as long as its settings have changes, and goes again once they are saved or reset.
  A click on it, from any page, chooses the Cube where another is chosen,
  shows the rooms, opens the room's device list and puts the focus into the settings.
  A device that has not been heard from for more than 24 hours has a "Not seen
  for …" badge.
- The cog button opens the room's configuration:
  - **Room**:
    the name.
  - **Temperatures**:
    comfort, eco, open window and its duration, maximum and minimum,
    and which thermostat the values come from.
  - **Devices**:
    for a room with radiator thermostats first their settings,
    Boost and its valve, descaling, maximum valve and valve offset,
    with a **Send** of their own;
    then per device its name, room, temperature offset,
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
    It copies what is stored, not changes that haven't been sent yet,
    and the week programme as the last **Send** of the Week programme tab left it.
    It is a command, as the House tile's panel is, and no settings form:
    what is chosen stays while the configuration is open and when it closes,
    marks nothing unsaved, and a copy that went through clears the rooms chosen,
    what is copied staying as it was set.
  - **Send** is off while nothing differs from what the Cube has,
    or on the Copy tab while no room is ticked.
    While one of the configuration's writes runs, its **Send** turns
    and every other **Send** of the configuration is off.
  - A field that shows what is stored keeps what is typed,
    where the stored value changed meanwhile, as another client renames the room,
    it says so under the field, with **Take theirs**.
  - Room, Temperatures and Week programme are settings forms, each of its own,
    and on the Devices tab the radiator settings and each device:
    what is typed stays until it is sent, reset or cancelled,
    from one tab to another and when the configuration closes;
    opened again, the configuration shows the tab it was on, with what was typed,
    and the week programme's edits, its days marked changed.
    It is forgotten as the virtual thermostat's settings are,
    a device's form also when the device has left the room.
    **Send** and **Reset** say how a form stands, as in Cube settings:
    off and plain while nothing is changed, in the accent colour once something is,
    with the warning mark once the focus has left the form,
    its tab or the configuration was closed or a minute has passed since the last change,
    a key typed or a change of the week programme making it changed again,
    with the cross after a write that failed, the Cube discarded or a device did not take,
    the form then still changed, and the check after one that went through;
    **Cancel** resets the form and closes the configuration.
    The week programme keeps its own **Reset day** and **Reset all**.
    While a form is unsaved, or its write failed,
    the room's entry in the tile's problem list,
    and the Cube's problem lists, in the header and in its panel's head,
    show the disk, "Living room, Temperatures: not saved",
    "Living room, Devices: Radiator: not saved" for a device;
    a click on it, from any page, chooses the Cube where another is chosen,
    shows the rooms, opens the configuration on the form's tab
    and puts the focus into the form.
- Each setting has a **?** button that explains it, with the factory values
  from the eQ-3 manuals; an explanation shown stays shown when its tab is drawn again.

The **House** tile, named after the chosen Cube with the cube before its name
("All rooms of …" its tooltip), sets several rooms at once: ECO, AUTO,
COMFORT or MANU with a temperature, permanently or until a date and time;
its **Send** is off while no room is ticked,
the rooms a choice of checkboxes with **All rooms** first.
What its panel holds stays when the rooms load again and when it closes:
the rooms ticked, the mode, the end typed.

**Devices without a room** are listed at the end:
a device just paired, or one the original software paired through the proxy
and never put into a room.
**Put into a room…** opens the form of System settings → Cubes' pairing under the row,
with name (offered as there), room and **Copy the room's settings**,
and **Remove from the Cube** asks in the row, then deletes it from the Cube.

Every change goes to the Cube as a radio command.
While it is sent, the tile's buttons that send are off,
the one clicked turning, so that a second click sends nothing;
a value of the chevrons stays shown, with **Save** turning, until the answer.
The tile shows the result when it is done: "ok", or what failed.
While radio commands wait for the devices' acknowledgement,
the header says how many are pending.

A tile is drawn once and follows the data in place:
a menu open, the focus, a click under way, a text selected, the result line,
the holiday form and what is typed in it stay
when a device reports or the rooms are loaded again.
So does the configuration, with the focus in it or not:
its tab, what is typed, an explanation shown, the week programme's edits
and a click on its buttons.

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
**Close** closes the box.
A box follows the data in place: while the tile, the device's row or the panel
follows a device's report, a reload of the rooms or a poll,
a rename of the device or the Cube among them,
it stays open, and a name typed in it, a question open and the focus stay.
A question never outlives the preview it asked about:
where a repair's preview is read again,
after a run in the same box or a name given,
its question is gone and **Run…** asks again.
At a sign-out, and when another user signs in after an expired session,
every box is closed and forgotten with what it held.

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
  asking first for the name of a room the list does not have;
  a device the list does not have is listed under the name its field
  offers, which can be changed before Run…:
  the room's name, the device's type in the page's language,
  and the lowest number from 1 that no device of the room takes
  in a name of that form ("Bath radiator thermostat 2") (D185).
  One longer than the 32 bytes the Cube keeps
  ("Wohnzimmer Heizkörperthermostat 1" is 34) stays in its field as it is,
  outlined in red with the limit in amber above it, as it is typed too,
  and the listing shows no cost and no **Run…** until it is shortened (D201).
  **Show what it sends** reads the preview again with a name changed.
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
again to them and from where ("as Massimo last set it", "as most of its
thermostats have it", "as its wall thermostat has it"), or why not,
and their agreeing again;
**programme drift**, a thermostat that does not follow its week programme while
the room's others follow theirs ("does not follow its week programme: 12.0
instead of 21.0 since the switch point at 18:20, …; its clock or its programme is
suspect"), with "it shows the programme an hour off" where it does and the
radiogram that failed, and its following again.
A write its devices do not show by its deadline is a **not confirmed** line,
a warning that names the write, the room and each device with what it shows
("setpoint to Bedroom: not confirmed after 5 min: Window (mode auto, setpoint
20.0)"); a device that shows it later has a line of its own ("setpoint:
confirmed after 70 s").
A room's setpoint is sent once more then (`docs/api-v1.md`, "A room setpoint
sent once more"), with lines of the same kind: "setpoint to Bedroom: sent once
more to its thermostats", or why not ("not sent once more: once a day at most,
and it was today"), then "sent once more, confirmed by every thermostat after
25 s", which settles the warning, or a warning "sent once more, still not
confirmed after 5 min: Window (mode auto, setpoint 20.0)".
Any other write's warning is settled by a mark alone.
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
Older first asks the server for what follows the newest line shown,
by its id (`after=`, `docs/api-v1.md`, "GET /api/v1/cubes/{sn}/log");
where the server's log ends before that line, the log loads again
in place of the page before the oldest:
so for a Cube served for this run whose lines the server kept in memory,
once it gains its row in the store and those lines leave its log,
for a store restored from an older copy or emptied by the prune,
and for a server without a store restarted with its clock set back.
A line the prune took while newer ones stay is not noticed.
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
a thermostat off its week programme until it follows it again,
a room's setpoint not confirmed until every thermostat shows its re-send;
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
in place of the header's Cube's stream:
those after the newest line it has, by its id (`after=`),
page after page while a page is full, ten at most,
so that each line comes once, also where a burst brought more than a page,
and none is missed that the server stamped before the newest but kept after it;
past ten pages, as after a computer's sleep with the log open,
and where the server's log ends before that line (as Older finds it, above),
the log loads again.
Another Cube in the header leaves the log as it is.
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
  with **Reset** and **Save**, off while it is the one stored.
  What is typed stays when the personal page is shown again;
  left unsaved, the card's head shows the disk, which opens the form,
  and ☰ and its Personal settings carry the mark, as System settings' forms do.
- **Change password**, for a user with a password:
  the own password, with the current one,
  which a sign-in by OPAQUE proves without sending it;
  the new one is registered by OPAQUE, as every password the page sets.
  A user with keys only, such as the owner made from `MASSIMO_API_TOKEN`,
  has **Set a password** instead, without a current one, and a line saying
  that it lets the user sign in by name in any browser.
  **Reset** empties the fields; a new password typed is marked as the display name is,
  but kept nowhere but in its field, and gone at a sign-out;
  the current one alone, as a password manager fills it in, is no change.
- **Sessions and keys**: the own sessions and API keys,
  each with where and when it was last used and **Revoke**,
  which signs another browser out without an administrator;
  a session made from a key, as the app's login by fingerprint makes one,
  names the key by its label, and revoking the key ends it;
  the session or key the page is signed in with says **this device**,
  the key that session was made from **its key**,
  and Revoke on this device signs out, as **Sign out** does;
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
A setting is saved as it is chosen, Poll every as its field is left,
and a poll being typed stays when your settings arrive from the server.
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
  off while nothing differs from the scheme the editor started from,
  as Enter in the name is,
  **Reset** goes back to that scheme, its colours and its name,
  **Delete** removes one of your own.
  What is edited stays when your settings arrive from the server;
  left with changes, My settings' head shows the disk, which opens the editor,
  and ☰ and its Personal settings carry the mark;
  Close, choosing another scheme, an import and a sign-out drop the edit.
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
  a device's communication error or link error,
  the Cube out of reach (after 10 min) and back,
  and, off by default, a device that sends no news, a device paired,
  a new Cube on the network and a new version of Massimo.
  A change is saved a second after the last one,
  at once where you sign out within that second;
  a delay is 1 to 1440 minutes, the threshold −10 to 20 °C,
  and a value outside is not saved:
  its box turns red, with the range in amber before it,
  until it holds one inside, whatever a save meanwhile says,
  and what is typed stays when the page is shown again.
  One kind notifies once per device per half hour,
  a device's error once per flag:
  a link error set while the communication error lasts notifies too.
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
A device's error names its flag, here as in the push:
the communication error, with the radiogram that failed where the device
gives it ("The radiogram that failed: ControlMode (0x40)."),
the link error, or both (`notifytext.js`).
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
The panel is built once and follows every poll in place:
its problem list stays open and follows the data, a focused button keeps the focus,
and what is typed stays, each field with the "changed meanwhile" of System settings;
Reboot's question names the radiograms pending as the Cube says now,
and a line of the settings file that comes to differ while the panel is shown
gets its notice with **Take the file's value**.
The pairing is part of the panel and follows in place too:
the countdown counts down in its line, a device that joins is added under it,
and the time typed and a joined device's name and room stay, with the focus,
through the countdown, a pairing event, a device that joins and a poll;
what a joined device's form holds stays while the panel moves
between System settings and Cube settings,
and is forgotten at a sign-out, as the Cubes' rooms are,
and when another user signs in after an expired session.
So does Repair… with its box (Rooms, Repairs).
What is typed in a Cube's settings stays until it is saved or reset:
when the panel closes, from System settings to Cube settings and back,
and when another Cube is chosen, each Cube's with its Cube;
it is forgotten at a sign-out and when the Cube is no longer served.
**Save** says how it stands: off and plain while nothing is changed,
in the accent colour once something is,
with the warning mark before its text once the focus has left the form,
the form was closed or left, or a minute has passed since the last change,
turning while it saves,
with the cross after a save that failed, the server not reached or the value refused
(the message line says why, no field is outlined), until a save goes through or Reset,
and with the check for five seconds after a save.
**Reset**, beside it and off while nothing is changed, takes the values stored back.
While the settings are unsaved, or a save failed,
the Cube's problem list, in its panel's head and in the header's list of every Cube,
shows the disk with the corner mark, amber, red after a failed save,
"Cube settings: not saved", as it shows a disk for each of its rooms' unsaved settings
(Rooms, the device list); a click on it opens the settings,
the Cube's panel in System settings, else Cube settings with the Cube chosen,
and puts the focus into them.
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
The page is drawn in place as its facts are read again,
while a certificate is being read and each time the page is shown,
and as its clock ticks:
the folds stay open, a text selected stays selected
and a copy says "Copied" until it fades.
A sign-out forgets the facts, the store and the settings in effect,
and ends the readings of a certificate;
the next user who opens the page reads them again.
While it is shown, the page follows the server's events:
a `resync` of the stream, after the server's restart, reads the facts,
the store and the settings in effect;
`settings_read`, the settings file read again, the store and the settings in effect;
a `change` that adds, changes or takes away a Cube
(`cube.add`, `cube.update`, `cube.remove`) the facts,
since the proxies' listeners and the interfaces listened on change with it.
No other event changes a fact it shows:
the proxies' events are about their clients, not their listeners,
and the store's rows, which every event adds to, are read when the page is shown.
The certificate's readings keep their count and their pace through an event's read;
where an event's read finds the certificate being read and nothing reading it,
as after a restart, the readings start anew, their count at none.

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
A settings form of Users, Roles, Sign-in providers and Integrations
(a user's, a role's, an identity's, a provider's, an integration's name
and its maximum) keeps what is typed in it until it is saved or reset:
through a change saved in another panel, the list read again,
the panel closed and opened again and another page shown;
its **Save** says its state, with **Reset** beside it, as Cube settings' does
(AP UnsavedMarks).
While one is unsaved, the card's head shows a pill with the disk, amber,
or red after a save that failed, each of its items naming the form,
which opens it, its panel opened and the focus in it, from any page;
and ☰ carries the mark, the worse of it and the log's,
its **System settings** the disk; so do ☰ and its **Cube settings** for a Cube's
settings unsaved (UnsavedMarks D24), which also have their panel's pill and the
header's entry of the Cube.
An item opens its card first where the card is folded.
A switch saved at once (the password sign-in, a capability for every role,
whether integrations may ask, an integration's **Enabled**) shows the position
chosen while its request runs and stays where the focus is; a second change
meanwhile sends nothing; refused, it shows what is stored again.
A form that adds an item, the switches saved at once and a restore are no
settings forms and have none of it.

- **Cubes**:
  the Cubes the server serves, kept in its store;
  the card's head counts them, at every poll whether a panel is open or not:
  how many are served, those not connected as their signs show them
  (red "not connected" for a cross, amber "connection held" or "rebooting"
  for a triangle), the proxies that do not listen,
  the Cubes found at another address and those found and not served.
  Each is in a panel whose head shows the cube, the Cube's name,
  the serial and the address without the port,
  the duty cycle, the free slots and the radiograms pending,
  a pill with every problem of the Cube (its proxy, its settings file
  and its devices with theirs, as the header's), top-right whatever wraps,
  led by the Cube's network sign,
  which stands for the green check where nothing else is wrong;
  a click on it opens its list and not the panel,
  the Cube's link in words its first row, "connected" too,
  and under them where its settings come from,
  all of it following every poll in place, whether the panel is open or not,
  so that an open problem list stays open, and opens with a click:
  its own badges as its top line (its link where it is not connected,
  a proxy that does not listen, a settings file that differs,
  a Cube found elsewhere than its fixed address, the source),
  a table of its facts for everyone who sees the panel
  (serial, address, port, firmware, radio address, the Cube's clock,
  duty cycle, free slots, pending and queued, following every poll;
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
  where few of the Cube's memory slots stayed free for an hour with nothing of
  the server's on its way (`slots_low`), a badge, amber **few memory slots
  free** below 5, red **memory slots full** below 2, and a notice with the
  free slots and since when, that they hold radiograms for devices that do not
  answer and that radio commands wait for free slots, pointing to
  **Reboot the Cube** below, the owner's, which drops them;
  the log has its **memory slots** line, a warning below 5, an error below 2,
  until the slots are free;
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
  A field not touched follows a value stored meanwhile
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
  its settings and history stay in the store with its removed row,
  and it is listed as found while discovery hears it.
  Under the panels, what discovery found and the server does not serve
  is one fold, **Found on the network, not set up**, closed at first:
  its head the found Cubes' sign and how many Cubes and Massimo proxies;
  open, each Cube with its address, DHCP flag and firmware,
  "via proxy at …" where a massimo proxy answers for it,
  in expert mode its radio and MAC address and the gateway it uses, and **Add**,
  which serves a found Cube at once;
  a Cube that was removed has **Add as new**, a row of its own without the
  old history, and a **Restore** with the date for each removed row of its serial,
  newest first, its name in the label and the tooltip,
  which serves that row again with its settings and history (`removed`, `restore`);
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
  the name offered as a repair's listing offers one (Rooms, Repairs; D200-A),
  the room's name, the device's type in the page's language and the lowest
  free number ("Bath radiator thermostat 2"), following the room chosen until
  a name is typed;
  a name longer than the 32 bytes the Cube keeps, the one offered included,
  stays as it is, outlined in red with the limit in amber above it,
  and **Put into a room** goes back to the field until it is shortened (D201);
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
  **Save**, off while nothing differs from the user, and **Reset**;
  a new password typed stays in its field alone, in no draft, and goes
  with a Save, a Reset or a sign-out;
  **Remove**, which asks in place of the panel's actions;
  the user's sessions and API keys with **Revoke**,
  read when the panel opens and after every change of the users while it is open;
  in one's own panel the session or key the page is signed in with says **this device**,
  the key that session was made from **its key**,
  and Revoke on this device signs out, as **Sign out** does;
  and **Create key**, which shows the new key once: from the read that follows it
  until the next, in that panel alone, kept nowhere;
  where a sign-in provider is enabled, the user's identities at the providers,
  each with its e-mail, or its user ID there,
  and whether it is bound or waits for its first sign-in,
  what the provider said of it at the last sign-in (name, user name, groups) and when,
  **Change** of the e-mail or user ID while no sign-in used it,
  a settings form of its own, with **Cancel**,
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
  **Reset**, **Remove**, which asks in place of the actions;
  a client secret typed stays in its field alone, in no draft;
  **Add a provider** opens the form for one, with its key.
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
  Approved, the integration's panel says until when Home Assistant may
  collect its key, and the page reads that integration again, every 3 s
  in the first minute, then every 15 s, until it has: then it says so;
  once that time has passed, **approval expired** as a badge, why, and
  **Register**.
  Every integration in a panel, its name, **disabled**,
  **new installation** and **approval expired** as badges:
  its **Name** with **Save the name**, off while the name is as stored, and **Reset**,
  the name its users are shown under (the integration's own user's
  display name);
  its **Name in Home Assistant** where Home Assistant reports another,
  its id, installation id, address, version, when its key was issued and
  whether the LAN saw it;
  for a call from another installation, **Accept the new installation**;
  **Enabled**, saved at once, its maximum with **Save the maximum** and **Reset**,
  its users (read when the panel opens), the system user first,
  each with its id's first eight characters, its level and when it acted
  last, and **Acts as**: its own user or a Massimo user, the one of the
  same name proposed, with **Link**, the choice following what is stored
  until it is changed, without the marks of a settings form (UnsavedMarks D25):
  Link also takes a proposal the select already shows, so it cannot be off
  while nothing is changed, and it is a command; and **Revoke**, which asks first.
  A request's maximum and what it was registered as before stay as chosen
  while the integrations are read again, until it is approved or refused.
  Under them the Home Assistant installations the server saw on the LAN,
  each with its address as a link, or as text where it announced none,
  either with a copy icon, and **registered** or **not registered**;
  one not registered has **Register**, which opens Home Assistant's page
  that adds the Massimo integration
  (`<address>/_my_redirect/config_flow_start?domain=massimo`), and where
  Home Assistant announced no address first asks for it, filled in with
  `http://<its IP address>:8123`; with **Look again**;
  where the server does not look, why, naming Announce on the LAN and
  what to change: Off, Automatic on the loopback, a change waiting for
  the restart, or `-announce` or `MASSIMO_ANNOUNCE`, which decides;
  with `settings`, **Open Maintenance**, but where the flag or the
  variable decides.
  In **Users**, an integration's user is shown by its chain,
  "<the integration's name> › <its display name>", with
  ", as <login name>" where it is linked (AP StoreRaise v1.2), without
  its login name, and carries **integration user** (the integration's own,
  shown by the integration's name, **integration**); a linked one
  **acts as** the user; a user that integration users act as counts them
  as a badge and names them, by their chains, at the top of its panel.
  The log's lines, a command's detail and its audit row name who
  started them the same way ("…, by Home › Anna, as anna"), and the
  log's user filter lists the users so.
- **Roles** (with `roles`):
  every role in a panel with its display name and a switch per capability,
  the owner's all on and off,
  **Save**, off while nothing differs from the role, and **Reset**,
  **Disable** (with `roles.disable`), **Remove** for a created role nobody has,
  and **Add a role**; the owner cannot be changed.
- **Capabilities for every role** (with `capabilities`):
  a switch per capability, which switches it off for every role but the owner,
  saved at once and off while its request runs;
  refused, it shows the capability as stored again and the roles' line says why.
- **Backup and restore** (with `backup`):
  a fold per Cube, the cube, its name and serial, its sign,
  and how many backups the server keeps of it in its head, a single Cube's open;
  the card's head counts them all.
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
  A Cube's boxes on both pages show the same restore,
  and a fold open, a deletion asking, the format chosen and a restore shown
  stay while the lists are read again;
  while a restore runs, its Cube's **Restore…** and **Restore from file…** are off
  in both places, so that it stays in sight, and a stored backup's plan closes
  once the list no longer has the backup;
  where the page's connection to the server was cut so long that the server
  answers a resync, a restore still running is shown as one whose end the page
  did not see, the log saying how it ended, and the buttons are back.
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
  The section is drawn in place as a check comes in,
  the header's every half hour, Check for updates, the reading while the helper works
  and Maintenance shown again:
  a line selected stays selected, the channel's choice and Update's question stay;
  the question goes where a check finds another release than the one it named,
  the focus back on Update,
  so that it never names a release the page knows to be gone,
  and Update then names the newer one.
  Yes asks the server to install its newest, which it checks again,
  and the line under the versions names the release the server says it asked for.
  The channel is saved as it is chosen and stays on while its request runs,
  so that the focus stays on it and another choice goes out at once;
  the release check on the channel follows outside it,
  and a later choice's check supersedes an earlier one's.
  A sign-out ends the reading while the helper works and takes the question back;
  the next user who opens Maintenance reads it again.
  While Maintenance is shown, the section follows the server's events
  (`docs/api-v1.md`, "Events"), each read drawn in place:

  | Event | When the server sends it | What Software update reads |
  |---|---|---|
  | `resync` | the stream opened again with no replay: after the server's restart, an update's among them | the versions, the helper's state and the last updates (`GET /api/v1/update`), and the setting |
  | `change` of `update.channel` | the channel set, from any browser | the same |
  | `update` | an update asked for from another browser | nothing at once: the page takes Update's question back and follows it as its own, the reading every 3 s with its 30 s of grace; an event more than a minute old by the server's clock, replayed after a gap of the stream, reads the versions once |

  The server's start and an update's end are written before any stream opens,
  and a stream opened again gets a `resync` with no replay:
  the page learns of them by the `resync`.
  An update this page follows, from its request on and while it reads every 3 s,
  reads at its own pace: an event then reads the versions not at all.
  An event's read of the channel waits while this page's own change of it is under way
  and is made once when it has ended,
  and its answer shows nothing where a choice was made meanwhile.
  Whoever started an update, a read that finds the helper at work greys the buttons out,
  and a refused request of this page leaves them so while the reading goes on.
  One `resync` reads the settings twice,
  once for the channel and once for Maintenance's card,
  as showing Maintenance does: two regions do not share their reads.
  A page not shown reads nothing; showing it reads everything, as before.
  Events that come together make one read, and one more after it for those
  that came while it ran.
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
  The card is drawn in place each time Maintenance is shown and after each of its changes:
  the delay typed, a question and the focus stay.
  The announcement and the intervals are saved as they are chosen,
  each staying on with the choice in it while its request, the PUT alone, runs,
  so that the focus stays on it, and showing what is stored once it has answered.
  The restart's question goes where the restart is no longer offered,
  the reboot's where the reboot is not, or a reboot waits, asked for elsewhere.
  A sign-out ends the following of a restart or a reboot,
  takes the questions back and forgets the delay typed.
  While Maintenance is shown, the card follows the server's events too:

  | Event | When the server sends it | What Maintenance reads |
  |---|---|---|
  | `change` of `announce` or an interval | the setting set, from any browser | the settings (`GET /api/v1/settings`) |
  | `settings_read` | the settings file read again: changed on disk, SIGHUP, Reload now | nothing: the event names what waits for the next start, and its time is when the file was read |
  | `reboot` | a reboot asked for, begun or failed | the reboot's state (`GET /api/v1/reboot`) |
  | `reboot_cancelled` | a reboot taken back, or the owner's switch turned off | the same |
  | `resync` | the stream opened again with no replay: after the server's restart | the settings, the restart's row and the reboot's |

  `restart` reads nothing: the server is going away,
  and its start says what it is like when it is back.
  A restart or a reboot this page follows reads at its own pace,
  `/healthz` every 2 s, and an event reads its row not at all meanwhile;
  a reboot that waits keeps its timer through a read that finds the same reboot,
  and ends it where the read finds it taken back.
  An event's read of the settings or of the reboot waits
  while this page's own request of them is under way and is made once when it has ended,
  and its answer shows nothing where one was made meanwhile.

## In the apps

The apps for Android and iOS (docs/app.md) open on a page of their own,
`ui/connect.html`, or on the server marked **Open at start**:
the servers the app keeps, each with what it found
when it last looked (on the LAN, remote, not reachable, a changed certificate)
and, as a green badge, the user the app is signed in as there,
checked at each look,
a tap opening the server, ⚙ or a long press its settings;
the Servers card's ☰, with **Add server…**, **Sign out of all**,
**Forget all stored credentials** where a login is kept (asked first, under
the list) and **App settings…**;
the card that adds a server, shown while the list is empty or from the ☰,
with × and closed once a server is added:
the servers on the network, searched for as it opens, and an address typed;
and the fingerprint of a self-signed certificate, compared with the
server's log or `massimo tls show` before it is trusted.
A server's settings take the focus themselves, not a field,
so that a phone's keyboard stays closed:
the name and the addresses, **Open at start** (one server at most),
when the fingerprint is asked, the session with **Sign out**,
the certificate's subject and end with **Certificate…**,
a dialog with its issuer, names, validity, fingerprint and since when it
is trusted, and the login by fingerprint with **Turn off**,
which revokes its key and signs out.
**App settings** hold the **Appearance**, as the phone, light or dark,
for that page and for every server's page set to "as the system".
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
What a script builds again and again, a log line, a scheme row,
is a `<template>` there, which the script clones with `fromTemplate`,
fills with `textContent` and wires with `addEventListener`.
The shared pieces (buttons, the `.field` rows, forms, confirmations in the page)
come from `components.js`; the scripts never use `innerHTML`.
The week programme editor is an element of its own, `<massimo-week-program>`
(`week-program.js`), so that a Home Assistant card can use the same editor:
it holds the editor's behaviour, works on a model the page keeps,
builds its controls with a kit the page gives it (`components.js`, `t()`,
`weekKit` of `config.js`),
and tells the page of every edit with a `change` event;
the programme tab, a part (`ui/src/rooms/Config.tsx`), makes it once in an element
it leaves to it and puts its two resets beside Cancel and Send.
A plain panel that opens is a `detailsPanel`:
its head the name, the serial, login or key, and the badges,
its body the form cloned from the feature's template.
A long page is a grid of section cards (`section.section-card`,
its heading's `button.section-fold` folding it), the page's skeleton:
the parts fold them and keep the fold in the browser,
and draw what a card holds in its head
(`ui/src/shared/card.ts`, `ui/src/chrome/`, below);
a page's title's row kept at the top is the chrome's
(`ui/src/chrome/Bar.tsx`, drawn into its `[data-bar]` place):
its sections menu, an item per card
(with a part without a fold marked `.page-section`, as Cube settings' panel),
the settings pages in its title's place
(`SETTINGS_PAGES` in `chrome/pages.ts`, ☰'s middle four;
the title shown for one page alone),
the page shown's button the longest name's width,
System info's badges,
and the personal page's who is in and Sign out (`whoOf` of `shared/me.ts`),
which hides My profile from all but a user;
and the two ↑, the floating one and the compacted header's, are the chrome's
(`ui/src/chrome/ToTop.tsx`, `scroll.ts` `toTop`);
`showToast` shows the page's one toast at its foot (a notification, the search's outcome),
a Cube's link is its network sign (`cubeSignEl`, `LINK_SIGN`), which no plain region calls
any more since Backup and restore is a part, and which waits for Block 15 (D181-A);
a device's serial and radio address are a `deviceIds`;
a list of facts is a `kvList`, of counts a `countList`
(`KvList` and `CountList` for a part);
an action with what it does and its button is an `.action-row`;
the chrome keeps the header at the top and compacts it once scrolled
(`ui/src/chrome/scroll.ts` `stickyHeader`, drawing nothing on a scroll's path).
A state is a sign, a colour and a mark, so that it reads without its colour:
nothing for a state that is fine, a triangle for a warning (amber),
a cross for a failure (red).
`signEl` draws a kind of thing in a state (`signOf`: the radio and the window
as the person chooses, `ICON_CHOICES`, the Wi-Fi fan or the tower,
the opening sash with the mark inside its frame or the paned window with it
at the bottom right, as the original MAX! software has it;
`SIGNS`: the network of a Cube's link with it on top, the cog for the settings,
the disk for a form's unsaved changes),
the drawing and the mark two parts, so that the mark alone blinks;
`badgeEl` leads a warning's or a failure's badge with its mark (`markEl`)
and sets it in bold, while a badge that names a state, such as a mode, stays regular;
`keepBlink` keeps a mark's blinking where it was across the redraws that make a new
element of it.
The icons draw in the token `warn-mark`, a brighter amber than the text's `warn`,
at a contrast of 3 on a tile, the text at 4.5 (`TestUIDefaultSchemeMatchesCSS`);
the filled amber, `fill-warn`, is the Manual mode's alone.
The list of what is wrong in a place is the shared part `Problems`, wherever it is put,
its entries one format at every level:
the icon in a column of its own, the name and a grey detail beside it,
the badges under the name, and the entries under it (`subs`) indented to the name,
a line down from its icon;
its plain twins in `components.js`
(`problemsEl`, `problemEntry`, `kindIcons`, `entryItems`)
have no caller left and wait for Block 15,
and so do `noticeEl`, `linkEl`, `kvList` and `countList` since System info is a part.
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
- `header.js`, the plain names of the header, which call its parts (`ui/src/header/`),
  and what stays plain there: the radio budget, the server's clock and its last answer,
  the version in ☰ and in the sign-in card, the server's state;
  `home.js`, the home notice's place shown or hidden, the Cubes a `cube_found` event adds
  and the way to them;
  `prefs.js`, the plain names of My settings, which call their part (`ui/src/prefs/`),
  the sign-in's language choice and the copy of the version in ☰;
- `rooms.js`, a room's facts, the plain names of the rooms' drawing,
  which call their parts (`ui/src/rooms/`), a device's state event
  and the actions behind a tile; `tiles.js`, the tile's actions that stay plain
  and a finding of the Cube's lines in words;
  `unlisted.js`, a device removed from the Cube;
  `house.js`, the House tile's plain names, which call its part;
- `week-program.js`, the week programme editor; `config.js`, the editor's kit;
- `log.js`, the log and its sign for unseen problems;
  `pages.js`, the plain names of the pages' store (`ui/src/shared/pages.ts`)
  and the log's choices in the address;
- `update.js`, the plain names of Software update,
  which call its part (`ui/src/software/`),
  for the header's check and the server's sign;
  the Maintenance card has none, a part alone (`ui/src/maintenance/`);
  System info has none, a part alone (`ui/src/sysinfo/`);
- `cubes.js`, the plain names of System settings → Cubes and Cube settings,
  the pairing's among them, which call their part (`ui/src/cubes/`),
  and the Cubes card's badges;
- `login.js`, the sign-in; `users.js`, the plain names of System settings → Users,
  which call its part (`ui/src/admin/`), and the check of a new user's name,
  which the first owner's form shares;
  `providers.js`, the enabled sign-in providers and the plain name of
  Sign-in providers; My profile has none, a part alone (`ui/src/profile/`);
  `backup.js`, the plain names of Backup and restore (`ui/src/backups/`);
  `roles.js`, the plain name of Roles and Capabilities (`ui/src/admin/`);
  Integrations has none, a part alone (`ui/src/integrations/`);
- `theme.js`, the mode and the colour schemes, applied as the script loads;
  `appearance.js`, the plain name of the Appearance card, which applies the appearance
  and calls its part (`ui/src/prefs/`), and the system's changes it follows;
- `notifytext.js`, a notification's text; `notify.js`, the plain names of the
  notifications' card, which call its part (`ui/src/notify/`), the service worker's
  language and the toast of a notification.

A script's top-level code runs as it loads, and the `DOMContentLoaded`
listeners in the order the scripts register them,
so the order of the tags matters where one script's top-level code needs
another's (`theme.js` after `app.js`, `config.js` after `week-program.js`).
`go test ./internal/server` refuses a top-level name that two scripts declare,
and a script embedded that the page does not load
(but those of `uiScriptsOffPage` in `ui_test.go`).

The six scripts that held several regions were cut along them by
`tools/ui-split` (its `README.md`), by moves alone,
and so were `users.js` and `providers.js` with AP UiParts' Block 10,
My profile going to `profile.js` and `linked.js`, which went with AP UiParts' Block 13,
and a branch that was open meanwhile makes the same moves with it.
The tool is there while the plan UiParts lasts, and goes with it.

A region that has moved is a part:
Preact with its signals, written in TSX under `ui/src/<region>/`,
built once and updated in place.
`ui/src/index.ts` is the entry point; it registers `window.MassimoParts`,
by region the functions the plain code calls;
the first region that is a part is the Cubes (`ui/src/cubes/`, below).
What the parts share is under `ui/src/shared/`; read it before writing a part.
A region uses these and writes no copy of their markup.
A shared part knows no region:
its data come as properties or signals, its texts as text or as its own keys.
A part calls the plain scripts' globals,
each declared to TypeScript in `ui/src/plain/<script>.d.ts`,
with what the parts call alone.
Two regions never import from each other (`TestRegionsKnowNoOtherRegion`:
every directory of `ui/src/` but `shared`, `model` and `plain` is a region,
and imports from itself, `shared`, `model`, Preact and the API's types alone).
What several regions read of the API's data beside its fields is under `ui/src/model/`
(`capLabel` of `cap.ts`, what a capability allows,
for the roles and the integrations, and `event.ts`, a server's event of the stream
and the settings a change event set, for the parts that follow the server, among it):
it knows the API's data and no region, holds no state and no part with hooks of its own,
and may put shared parts together into a small piece of that data;
it imports from itself, the shared parts, Preact and the API's types alone
(`TestModelKnowsNoRegion`).
What one region alone reads stays in the region, anything with state too,
and what is not about the API's data is a shared part.
A part with state that two regions draw is a shared part too (UiParts D193),
since `model/` holds no state and no region imports another:
the form that puts a device into a room (`put.tsx`) and the repairs (`repair.tsx`),
with how long a name may be and the name a device is offered (`name.ts`),
which they read.
Such a part knows the API's data and sends its own requests,
and still knows no region:
it imports none, its texts are keys of its own (`put.…`, `repair.…`, `name.…`),
and it reads no id or class of a region's markup it does not draw itself.
`cube.tsx` holds the Cube chosen in the header (`chosenCube`),
a Cube's address split into host and port (`hostPort`),
the cube before its name (`CubeIcon`),
its network sign (`CubeSign`), its link's row in its own pill's list (`CubeLink`)
and its problems as an entry of `Problems` (`cubeEntry`, `cubeSettingItems`)
with the label naming their kinds and the Cubes found (`problemsLabelOf`);
`found.tsx` what a Cube or a Massimo proxy found is and who serves it (`foundWhat`);
`device.ts` a device's type as its icon (`deviceIcon`), an open window's state
by the page's clock (`windowState`, `WINDOW_TONE`) and what is wrong with a device
as items of `Problems` (`deviceProblems`),
and a device as an entry of `Problems` (`deviceEntry`),
which the rooms list in a tile and a device's row,
and a Cube's entry under the Cube;
a value's age it takes from `received` of `rooms.js`, as the rooms do.

| Part or helper | File | For | Properties or arguments |
|---|---|---|---|
| `counter` | `counter.ts` | a region's load counter: "draw the region again" while its data stay plain objects | gives `bump()`, after a load, and `useLoad()`, in every part of the region, which reads the counter, the language and the icons chosen |
| `register`, `mounter` | `region.ts` | a region's functions as `MassimoParts.<name>`; its root drawn once into the page's container | `name`, `fns`; `id`, `root` |
| `live`, `liveMap` | `live.ts` | objects whose properties are signals, for what a region keeps per place; the plain code reads and writes them as before | `init`; `make`, then `of(key)`, `forget(key)` (what it was kept for is gone), `clear()` |
| `lang`, `useLang`, `tParts`, `joinParts` | `lang.ts` | the page's language as a signal, raised by `applyLanguage` (`MassimoParts.language`); a text with parts in it (`tNodes`); parts joined with " · ", the empty ones left out | `key`, `texts`, `parts`; `parts`, `lead` (" · " before the first too) |
| `useIcons` | `look.ts` | the icons chosen for the window and the radio as a signal, raised by `redrawSigns` (`MassimoParts.icons`); `Sign` reads it | – |
| `Island` | `island.tsx` | a plain piece inside a part: an element the part draws empty and plain code fills, again where a key or the language changed | `tag` (div, p, span), `cls`, `keys`, `fill(el)` |
| `Fold`, `FoldHead` | `fold.tsx` | a details that opens and closes (`detailsPanel`), the toggle of its making not passed on; a panel's head | `id` (the plain markup's id, which a region keeps: System info's two folds), `cls`, `open`, `onToggle(open)`, `summary`, `summaryProps`, `data`, the body as children; `before`, `name`, `mono`, `after` (after the monospaced part, a sign of the thing's state), `badges` |
| `Dropdown` | `menu.tsx` | a menu that drops down (`dropdown`), with `arrows` its arrow keys (`menuArrows`) and with `focusCurrent` the focus on its current item when it opens (among `arrows`' items, else its list's buttons that are on); a click on any button of its list closes it, as `dropdown()` of `components.js` does for every menu of the page, but on one marked `data-keep-open` (`MenuItem`'s `keepOpen`) | `id`, `cls`, `summaryId`, `summaryCls`, `label` (without it the summary is named by its text), `title` (where the tooltip is not the label), `summary`, `listCls`, `arrows` (a selector of its items), `focusCurrent`, `busy` (the summary turning while a request sent from its list runs), `onClick`, `onOpen` (told each time it opens), `onClose` (told each time it closes), the items as children |
| `MenuItem` | `menu.tsx` | a menu's button, its icon, label, the icons after it and a grey second line (`menuItem`) | `icon`, `blank`, `label`, `icons`, `sub` (the second line, a text or elements, there whenever given, empty too), `subCls` (its class), `cls`, `value` (its `data-value`), `current`, `disabled`, `busy`, `keepOpen` (a click on it leaves the menu open, for an item whose result shows in the list), `onClick`; `current` "page" for the entry of the page shown (☰'s), `id`, `title`, `aria` (its name for a screen reader), `labelId` and `after` (a mark at its end) where the page names them |
| `Picker` | `menu.tsx` | a menu that chooses one item, its summary the chosen one (`pickerMenu`) | `id`, `cls`, `summaryId`, `summaryCls`, `entries` (an item `{ value, label, sub, icon, icons, disabled }`, `{ group }` or `{ separator: true }`), `value`, `onPick(value)`, `shown` (`{ icon, label, title, aria }`) |
| `actionRunner`, `fader` | `action.ts` | a request, a reload, and its outcome (`Outcome`, `{ text, ok }`) in the region's message line (`showResult`) or in a part's state that a `ResultLine` draws, a success fading where a fade is given; `fader` is that fade alone, for a result set by other means | `where()`, `set(res)`, `fade` (ms), `reload()`, `bound` (an answer after a sign-out reloads and says nothing, `session.ts`); gives `run(fn, ok, fail)`, `say(text, ok)`; `set`, `ms` |
| `Icon` | `icon.tsx` | a symbol of the page (`iconEl`) | `symbol`, `cls`, `mark` (the symbol is a mark, which blinks where the style makes it), `useId` (its `use` element's id, where the page names it) |
| `Sign` | `icon.tsx` | a kind of `SIGNS` in a state, its mark blinking (`signEl`) | `kind`, `state` ("", ok, warn, bad, open), `cls`, `blink` (the mark's key), `label` (named for a screen reader, else hidden) |
| `Mark` | `icon.tsx` | a mark alone (`markEl`) | `state` (warn, bad), `blink` |
| `CountIcon` | `icon.tsx` | an icon with a number in its corner (`foundCountEl`) | `symbol`, `n`, `cls` |
| `IconButton` | `icon.tsx` | a button that is an icon alone, named for a screen reader (`iconButton`): a tile's info and cog, the copy icon, help's question mark | `symbol`, `label` (its name, and its tooltip without `title`), `title`, `cls`, `iconCls` (the icon's class, `ni` by default, `hi` for help's line drawing), `mark`, and the button's own attributes (`id`, `aria-expanded`, `aria-controls`, `data-…`, the handlers) |
| `Badge` | `badge.tsx` | a `.flag` led by its mark for a warning or a failure (`badgeEl`) | `kind`, `text`, `blink`, `title`, children after the text |
| `Notice` | `badge.tsx` | a lasting warning with since when (`noticeEl`) | `text`, `since`, `next`, `cls`, `id`, `role`, `hidden` (the class `hidden`, the line kept in its place), children after |
| `Age` | `badge.tsx` | how old a value is, filled by `fillAge` (`ageEl`) | `at`, `stale`, `plain` |
| `Since` | `badge.tsx` | from when a badge's state holds, after its text (a problem list's item) | `at` |
| `ResultLine` | `badge.tsx` | the line that says how an action went, green or red, hidden while there is none (`showResult`), as an `actionRunner`'s `set` gives it | `res` (an `Outcome` or null), `cls`, `role` (`status`, a screen reader told of each), `id` (a section's message line the page names, a field's message it describes) |
| `Problems` | `problems.tsx` | the problem list, entries under entries (`problemsEl`), an item with an action a button | `entries`, `label`, `ok` (the green check's label where nothing is wrong), `always` (the list there where nothing is wrong too; where `ok` is given, its summary the green check and `ok` a line of its list), `lead` (drawn first in the summary, a sign of the place's own state), `head` (drawn first in the list, items that act on the place or a row of its own state), `onClick`, `onOpen`, `onClose` (as `Dropdown`'s); an entry's `tail` is drawn after its name (a sign of its state where its icon says what it is) |
| `KindIcons`, `problemKinds`, `kindsLabel` | `problems.tsx` | the kinds' icons of entries without the list (`kindIcons`); the kinds themselves, to know whether there are any; the label naming the kinds among items in an order of kinds, "" for none | `entries`, `skip`; `entries`, `skip`; `items`, `kinds` |
| `OkPill` | `problems.tsx` | the green check in a list's place (`okPill`) | `label` |
| `useHelp` | `help.tsx` | a setting's or a group's help as a pair, the button and its explanation of an id of their own (`help-<key>-…`), which a field's row, a switch and a group of My profile draw where each belongs | `help` (the key), `label` (the name the button is named after); gives `button`, `text` |
| `HelpButton`, `HelpText` | `help.tsx` | a setting's help (`helpParts`): the question mark that shows and hides its explanation, `help.<key>` of the texts, shown by its key wherever it is drawn again, in the page's one set of keys shown (`helpOpen` of `app.js`), the parts drawn again by a signal of their own, so a key is drawn by one side only; `Field` and `Switch` draw both from their `help` | `help` (the key), `id` (the explanation's, which the button controls), `label` (the setting's name); `help`, `id` |
| `Field` | `field.tsx` | a label and its control, `.field` or `.field.wide` (`fieldRow`); with `limit`, what the field takes, said in amber just before the control while a value is refused, the box outlined in red and the limit its description (`markRange`) (`TextField` and `NumberField` mark their input for it; a control of the caller's inside a `Field` with a limit has no marking of its own yet) | `label`, `id`, `wide`, `unit`, `hint`, `hintId` (the hint's id, which a `Select` names as its description), `help` (the key of the setting's help: its button after the label, its explanation under the control), `cls` (classes the row adds, a capability's `needs-…`), `labelCls` (the label's class, `action-title` for a row's title in bold: Maintenance's settings; `Select` passes it on), `limit` (`{ text, over, cls }`: the text, whether the value is refused, the line's class beside `field-limit`; in the unit's line where there is one, else on a line of its own above the box), `draft`, `show`, the control as children |
| `TextField` | `field.tsx` | a row with a text input that shows a stored text, from its draft, or a free text (a name offered) by `value` and `set` | `label`, `id`, `name`, `draft` and `show`, or `value` and `set`; `wide` (on by default), `unit`, `hint`, `help`, `cls`, `limit`, `placeholder`, `type` (`password` too, by `value` and `set` alone, never a draft), `autocomplete`, `inputCls` (the input's class, `mono` for a key), `maxLength`, `pattern`, `required`, `disabled`, `inputRef` |
| `NumberField`, `numberNorm` | `field.tsx` | a number input that shows a stored number (`numberInput`), its draft the input's text, "" for none, or a free number (a time) by `value` and `set`; with `row={false}` the input alone, in a row of the caller's, which draws its notice (`Meanwhile`); how such a draft compares, as a number | `label`, `id`, `name`, `min`, `max`, `step`, `draft` and `show`, or `value` and `set`; `aria` (its name where no label names it), `disabled`, `inputMode` (the keyboard a phone offers: `numeric` for the reboot's delay), `inputRef`, `onChange` (told of the value as the field is left changed: My settings' poll, saved then), `row`, and a row's `wide`, `unit`, `hint`, `help`, `cls`, `limit`; `v` |
| `Select` | `field.tsx` | a row with a select (`selectInput`), its options as `fillOptions` takes them; with `row={false}` the select alone, in a row of the caller's, which draws its notice | `label`, `id`, `name`, `options` (`[value, label, disabled]` or `{ group, options }`), `value` and `onChange`, or `draft` and `show`; `row`, `wide`, `unit`, `hint`, `hintId` (the hint's id, which the select names as its description: the update channel's note, which changes with the choice), `help`, `cls`, `disabled`, `before` (between the label with its help and the select: a drawing's preview), `after` (after the select in its row: the language's note of a machine translation) |
| `DateTime` | `field.tsx` | a date and time field (`datetime-local`), its value as the input takes it | `value`, `set`, `id`, `cls`, `step`, `required`, `disabled`, `inputRef` |
| `localOf`, `isoOf` | `time.ts` | a `DateTime`'s value and the API's time as each other, for a field that shows a stored end and sends it (a user's end); the draft compares the field's minute, all it holds | `iso`; `local` |
| `fmtWhen` | `time.ts` | an API time as date and time in the page's language, the hour of 24 (a backup's time, a device's in Notifications) | `iso` |
| `Switch` | `field.tsx` | a checkbox drawn as a switch (`switchEl`), in its own row with its help where asked (`fieldRow` without a label) | `name`, `label`, `checked` and `onChange`, or `draft` and `show`; `row`, `help`, `rowCls` (classes its own row adds, a capability's `needs-…`), `cls` (the switch's classes), `id`, `disabled`, `title`, `aria` (its name where `label` is empty, which draws no text: My settings' "only on this device"), `data` (the checkbox's `data-` attributes), `badge` (inside the switch after its label, a capability's state), children after the switch |
| `Choices` | `choices.tsx` | a choice of several among a list: a group of switches under its heading, All and None as buttons where asked (the configuration's Copy tab), under a legend (an integration's maximum) or without a heading (a role's capabilities, Capabilities for every role), or of checkboxes, All a checkbox first (`look` `check`, the House panel); an option locked shows whether it is chosen and is off (the owner's set, a capability while its request runs) | `title`, `legend` (a fieldset under it), `label` (the group's name for a screen reader), `id`, `cls` (a class the group adds, `cap-grid`, `integration-caps`), `options` (`[value, label, disabled, { title, badge, lock }]`, the last for switches alone), `chosen`, `set(chosen)`, `all` (`{ all, none }`, their labels, `none` left out for All alone), `empty` (said where there is no option), `look` (`switch`, the default, or `check`) |
| `ChoiceGroup` | `choices.tsx` | the frame of a choice of several drawn as switches, which `Choices` is drawn with and a region uses where each switch is a part of its own (Capabilities for every role, each switch saved at once) | `id`, `cls`, `label` (the group's name for a screen reader), `legend` (a fieldset under it), `head`, the switches as children |
| `Tabs` | `tabs.tsx` | a row of tabs: a tab list of buttons, each with its key in `data-tab`, the one shown active and selected, and what follows the list (a close button) | `cls` (the row's class), `listCls`, `label` (the list's name), `tabs` (`{ key, label, cls }`), `current`, `onPick(key)`, children after the list |
| `Stepper` | `field.tsx` | a number field between − and + (`stepper`), the step within its range; its buttons are named for a temperature, half a degree less and more, and the first user of another unit adds their names | `value`, `set`, `min`, `max`, `step`, `id` and `cls` of the field, `hidden` (the whole kept in its place unseen), children after the + (a unit) |
| `useDraft`, `Meanwhile` | `field.tsx` | a field that shows stored data keeps a draft: untouched it follows, changed it says "changed meanwhile" with Take theirs (D59-A); with a place, a form's (`useForm`), the draft is kept there | `stored`, `norm`, `place`; gives `value`, `stored`, `set`, `changed`, `meanwhile`, `take`; `draft`, `show` |
| `SaveButton` | `field.tsx` | a form's Save by its state, as `FormFooter` draws it, for a Save that stands elsewhere (the scheme editor's Save as, inline with the name) | `label`, `look`, `busy`, `off`, `type` (submit unless given), `onClick` |
| `FormFooter` | `field.tsx` | a form's row of actions (`formFooter`, `sendWhenChanged`); with `look`, Save shows the form's state | `submit`, `busy`, `off`, `cancel` (its button of the class `form-cancel`, as the plain footer's), `cancelLabel`, `reset` (Reset, of the class `form-reset`, one fixed class per footer button as Cancel's `form-cancel`, off while clean), `look` (a `FormLook` of `useForm`), `extra`, `after` (after the submit button: My profile's ⓘ) |
| `useForm` | `forms.ts` | a settings form that keeps its changes until they are saved or reset, kept by a key the region chooses beyond the part, and its state: clean, changed, unsaved (the focus left it, it was taken away, or a minute passed since the last change), saving, failed, saved | `key`, `name` (a text key and its arguments, naming it in the problem list), `open` (shows it and calls `focusForm`); gives `look`, `props` (the form element's), `place(field)` (for `useDraft`; its `known()` the stored value its draft holds, undefined before it has one), `track(changed, drafts)` (after the fields, at each drawing), `save(fn)`, `reset()` |
| `hasChanges` | `forms.ts` | whether a form, or one under it, has changes not saved: changed, saving, unsaved or failed (a row shown while it holds such a form, otherwise for expert mode alone) | `key` |
| `unsavedItems`, `focusForm`, `forgetForms` | `forms.ts` | the unsaved and the failed forms as items of `Problems`, the disk naming the form, a click opening it (the registry they come from, a signal inside `forms.ts`, holds the forms merely changed too, for the question on leaving the page); the focus put into a form once it is shown; a form's drafts forgotten, what it belongs to gone, or all at a sign-out (`MassimoParts.signedOut`, `logout` of `login.js`) | `key` (that form's and those under it, `key:…`); `key`; `key` ("" for all) |
| `unsavedTone` | `forms.ts` | the worse state of the unsaved and the failed forms under keys, as their disks show it: `bad` after a failed save, else `warn`, else ""; a section's pill and ☰'s mark | `keys` |
| `changedItems`, `leaveForms` | `forms.ts` | every form with changes not saved, the merely changed and the saving ones too, as `unsavedItems` draws them: what leaving the page drops; every changed form counted as left, unsaved, at an attempt to leave the page | – |
| `FormsDialog` | `dialog.tsx` | a modal `<dialog>` that lists forms with changes, each a button that closes it and opens its form, with a button that stays (Escape too, and any close without a button of its own) and one that goes on, of the classes `dialog-stay` and `dialog-go` in every use; the focus on the one that stays, given back where it was when it closes | `shown`, `title`, `text`, `forms` (items, as `changedItems` gives them), `stay` (its label), `onStay`, `go` (`{ label, onGo }`, none without) |
| `Question` | `question.tsx` | a question in the page, No and Yes, Escape for No, the focus on No as it is drawn (`confirmBox`) | `text`, `yes`, `no`, `onYes` (its request's promise), `onNo`, `boxRef` (`useAsk`'s `box`) |
| `useAsk` | `question.tsx` | a row that asks in its own place, the focus back on its button in the drawing's own turn, as `confirmInPlace` gave it at once (a layout effect); with `held`, whether it asks is a store's live state, which the store takes back where what it asked about changed and at a sign-out (a part drawn once for the page's life: Software update, Maintenance); a question the store takes back while the focus is in its box (`box`, handed to the `Question`'s `boxRef`, judged as the drawing that takes it away begins) gives it back to the button, or to the row where the button cannot take it, in the same turn; `done(false)` gives it to nothing, as before | `held` (`{ asking }`); gives `asking`, `ask`, `done`, `ref`, `row` (the row of the caller's, which holds the focus with `tabindex` -1 until it leaves, where the button cannot), `box` |
| `useBusy`, `busyProps` | `question.tsx` | a button's request under way: busy, off, `aria-busy` (`whileBusy`, `setBusy`) | `run(fn)`; `busyProps(cls, busy, off)` |
| `useSetAtOnce` | `question.tsx` | a setting saved at once, a switch without a Save: the user's choice shown while its request runs, the control on so that the focus stays on it, a change meanwhile sending nothing and drawn back, then what is stored, the old value where it was refused (the password sign-in, a capability for every role, whether integrations may ask, an integration's Enabled) | `stored`; gives `value`, `busy`, `set(v, fn)` |
| `BusyButton` | `question.tsx` | a button that sends a request and turns until its answer (`useBusy`, `busyProps`), a click meanwhile sending nothing | `id`, `cls`, `label`, `run`, `aria` (its name where the text is not enough), `off` (kept off: a request that may not be sent now), `data` (its `data-` attributes: a Link button's provider) |
| `burstReader` | `burst.ts` | one read for a burst of the stream's events: a read in flight and at most one more after it, which reads what the events meanwhile asked for, none asked for in the session before (the parts that follow the server while shown) | `read(what)`, the reads by name; gives `ask(...what)` |
| `session`, `stillSession`, `newSession` | `session.ts` | the session's number, raised by `MassimoParts.signedOut` before anything is forgotten: a store takes it before an await and drops the answer, its message and its next timer where it changed; `actionRunner`'s `bound` does so for a runner | –; `n`; – |
| `me`, `own`, `meChanged` | `me.ts` | who the page is signed in as, `state.me` as `applyCaps` of `login.js` hands it over at every read of it (`MassimoParts.me`), null from a sign-out on; the token it is signed in with, by which a user's sessions say "this device" and "its key" | `m` |
| `whoOf`, `Who` | `me.ts` | who is in in the page's words, for the personal page's row, ☰'s Sign out and Personal settings' tooltip and the question before a sign-out alike: the row's line, Sign out or Forget key (none on an open server) with its icon, the name for the tooltip, and whether My profile is there | `me` |
| `cardBadges`, `badgesOf`, `BadgeSpec` | `card.ts` | what a section card's head shows at a glance, also while it is folded: a region gives a card's badges by its id from its drawing's effect, `[]` for none, and the page's chrome draws them (`ui/src/chrome/Cards.tsx`) | `id`, `list` of `{ text, kind }` |
| `showCard`, `wireCards` | `card.ts` | a section card brought into view, unfolded, the focus on its heading's button, kept there for two seconds where its page has just been opened (`instant`); the cards' folds wired once, each kept in this browser (`massimo.fold.<card id>`), set on the skeleton at once | `id`, `instant` |
| `Brand` | `brand.tsx` | Massimo's logo and small mark from the sprite (`brandMarks` of `components.js`, which draws them over the skeleton's `data-brand`), of which the place's style shows one, hidden from screen readers: ☰'s first row | |
| `page`, `isShown`, `showView`, `onView`, `mayShow` | `pages.ts` | the page of the main area shown, a signal: a drawing that reads `page.value` follows it, `isShown` asks at that moment and follows nothing (a handler, a reader, a timer), so that no request is sent again at a change of page; showing a page puts it into the address, as a new entry of the history or in place of the current one, and brings each page back at the scroll it was left at; `onView` runs at each showing, on the `viewshown` event; `mayShow` says who may open which, reading who is in; a page with an address of its own (the log's, `pages.js`) hands it over with `ownAddress`; `pages.js` keeps the plain names, and `currentView` | `view`, `address`, `card` |
| `userPath`, `Run`, `loadTokens`, `revokeToken`, `revokeOwn`, `createKey`, `loadIdentities`, `addIdentity`, `changeIdentity`, `removeIdentity` | `account.ts` | what a user's sessions, keys and identities read and send (`…/users/{name}/tokens`, `…/identities`), for System settings → Users and My profile alike; a change through the caller's runner (`Run`), which reloads what it shows and says how it went in its own line; Revoke of the token in use signs out | `name`, `rest`; `run`, `name`, and what each sends |
| `Tokens` | `tokens.tsx` | a user's sessions and keys (a user's panel, My profile): each with Revoke, "this device" and "its key" by `own`, Create key and the key just made, shown once by the read that follows and kept in the part's own state alone; read when its place opens and again at each change of the caller's counter while open; an answer after a sign-out dropped | `u`, `off`, `open`, `reads` (the caller's counter), `run`, `say` (the caller's message line), `labelId` (the new key's field's id), `help` (the group's ⓘ after Create key, My profile's) |
| `Identities`, `Changes` | `identities.tsx` | a user's identities at the sign-in providers (a user's panel, My profile): each with what the provider said and Remove; with `changes`, Change of one no sign-in used, a settings form kept by its key, the forms of identities gone or made unchangeable forgotten, and all of them where the box goes off; with `assign`, Assign; `after` what follows the list once read (My profile's Link buttons); `quiet` draws nothing while the list is empty and the caller offers nothing after it (`offer`: My profile's Link buttons, none in the app, where the rows are still named from every provider) | `u`, `off`, `open`, `reads`, `providers` (the enabled ones), `run`, `say`, `title`, `none`, `cls`, `changes` (`key(id)`, `under`, `ui(key)`, `forget(key)`, `show(key)`: the caller's forms and how one is brought into sight), `assign`, `after(list)`, `quiet`, `offer` |
| `FileButton` | `file.tsx` | a button that opens the browser's choice of a file of the types it accepts (`filePicker`), the same file again too | `id` (the button's), `cls`, `label`, `accept`, `off`, `inputId` and `inputLabel` (the input's id and its name in the page's language: the Appearance card's Import…), `onFile(file)` |
| `Adder` | `adder.tsx` | the end of a list that adds an item: the new item's form in its place while shown, the button that shows it hidden meanwhile, the focus in the form's first field when it comes and back on the button when it goes (Users, Roles, Sign-in providers) | `id` (the button's), `formId` (the form's place), `label`, `shown`, `set(shown)`, the form as children |
| `KvList`, `CountList` | `kv.tsx` | a list of facts, a name and its value each (`kvList`), every fact given, the caller leaving out those it does not show; a list of counts by name, sorted, in columns (`countList`) | `facts` (`[name, value, mono]`); `counts` |
| `Copy`, `CopyIcon`, `CopyPair` | `copy.tsx` | a text that copies itself, a copy icon (`copyEl`, `copyIcon`); something with its copy icon after it, a text or a link (an address served: an integration's, System info's) | `text`, `value` (what it copies, where it is not the text), `label` (named "Copy …"), `name` (its whole name in its place: the server's version), `icon`, `cls`, `id`; `get`, `label`; `get`, `label`, what it follows as children |
| `Link` | `link.tsx` | a link in a tab of its own, the text alone for a url that is not http(s) (`linkEl`) | `url`, `text`, `cls` |
| `DeviceIds` | `device.tsx` | a device's serial and radio address (`deviceIds`) | `serial`, `rf`, `always` |
| `cubePath` | `path.ts` | a Cube's path in the API, which every part's request about a Cube starts from, Backup and restore's too; `base()` of `app.js` is its next user, with its block | `serial` |
| `hid` | `cls.ts` | the class `hidden`, with its space, for an element kept in its place and hidden | `off` |
| `PutForm`, `newPut` | `put.tsx` | the form that puts a device in no room into one (with `structure`), wherever such a device is shown, a device that joined while its Cube pairs or one of the devices without a room: its name (with `rename`), offered for the room chosen until one is typed (D200-A) and marked where it is too long for the Cube (D201), the room, for a thermostat the copy of the room's settings (with `configure`), on by default, then `PUT …/devices/{rf}` and its outcome; what the caller keeps of it, live; the room chosen is pinned at its first drawing, the first of the rooms, and moves only where it is gone; a part with state that two regions draw, its texts its own (`put.…`, `help.putCopy`, and `name.long` for the name's limit) | `id` (its fields' ids' beginning), `serial`, `dev`, `rooms`, `put` (what `newPut` makes: the name typed, the room and the copy, the request under way, the outcome, done), `reload` (what follows the request, before `onDone` or `onFail`), `onDone`, `onFail`, `onCancel` (its Cancel, of the class `form-cancel`, none without), `focus` (its first field takes the focus when it comes) |
| `RepairButton`, `RepairBox`, `toggleRepair`, `repairScope`, `forgetRepairs` | `repair.tsx` | the repairs of a Cube, a room or a device (with `repair`), wherever it is shown (Rooms, Repairs): the button that opens and closes the box of a scope, and the box while it is open, each repair with its preview (`GET`, which sends nothing), its cost, its lines in expert mode, a name it asks for, and Run… that asks in its place before it sends (`POST` with the preview's plan), "unlink all and re-link" warning first and asking twice; a box's state kept by its scope, live, across its being drawn again, a question gone wherever its preview is read again, nothing read for a box closed meanwhile; the box opened from elsewhere (a tile's ⋯); a box's scope; every box forgotten at a sign-out and for another user after a 401 (`MassimoParts.signedOut`); a part with state that two regions draw, its texts its own (`repair.…`, and `name.long` for the name's limit) | `scope`, `kind` (`cube`, `room`, `device`), `path` (the API path its routes hang off), `label` (what it repairs, in its accessible name), `skip` (the repairs it leaves out), `names(room)` (the names of a room's devices, for the name a listing offers); `scope`; `scope`, `kind`, `path`, `{ skip, names }`; `serial`, `kind`, `id`; – |
| `NAME_BYTES`, `byteLength`, `nameFits`, `defaultDeviceName` | `name.ts` | a name as the Cube keeps it, at most 32 bytes of UTF-8; the name a device is offered in a room, "<room> <type> <n>" with the lowest free number whatever the case (D185, D200-A), as the server's default; the text of the limit, `name.long`, is the shared parts' | –; `s`; `s`; `room`, `word`, `taken` |

What the table cannot say:

- An entry of `Problems` is `{ icon, sign, node, title, sub, items, subs, info, summary }`:
  its icon, or `sign` (`{ kind, state }`) where its own state shows,
  or `node`, a part of its own with the class `problem-icon`;
  its name, a grey detail (a text, monospaced, or parts that are not),
  its items, the entries under it, `info` to list it and those under it
  without an item, and `summary`, shown in the list's summary after the icons.
  An item is `{ kind, icon, sign, tone, mark, text, title, at, stale, since, action }`,
  one badge:
  `kind` groups the summary's icons (`sign` draws it as a sign), an item
  without one adds no icon; `tone` is the badge's kind, `mark` how its icon is
  coloured where that differs; `title` its tooltip, `at` and `stale` its age,
  `since` from when its state holds, said after its text;
  `action`, where given, makes the badge a button that runs it
  (an unsaved form's item, which opens the form).
- `Field` draws the row and the label for `id`; its control comes as children
  and carries that id and, without a unit, the class `field-control`.
  `TextField` makes the input itself.
- `Switch` draws its own `.field` row; with `row={false}` it is the switch alone,
  inside a row of the caller's.
- `useDraft` and `useBusy` are the component's state and go with it;
  what must outlive the component (a request a tile keeps across its being
  drawn again) is live state (`live`), and a settings form's drafts are kept
  by its `useForm`, at the place each field is given.
- A settings form with `useForm`: its fields' `useDraft` at `form.place(field)`,
  then `form.track(changed, drafts)`, `form.props` on the form element,
  `form.save(fn)` in its submit, and `look={form.look}` and `reset={form.reset}`
  on its `FormFooter`; its region lists `unsavedItems(key)` among the items of
  the problem list nearest the form, and forgets the form's key (`forgetForms`)
  when what it belongs to is gone.
  Save's states: clean, plain and off; changed, primary; unsaved, primary
  with the warning mark before its text; saving, turning as every busy button,
  its ring after the text; failed, the cross, until a save goes through, Reset,
  or nothing is left to send; saved, the check, plain and off, for as long as
  a result line's success stays (`RESULT_FADE_MS`, by `fader`), then clean.
  A key in an unsaved form makes it changed
  again; a failed one stays red. The minute is one constant of `forms.ts`,
  which the UI suite moves on with Playwright's clock (the marker `clock`).
  A form without `look` keeps Save's one look, primary.
  One `useForm` per form element; a form of tabs takes a key per tab
  under its owner's key (`<owner>:<tab>`),
  which also gives a section its prefix.
  A form whose stored values load later (the virtual thermostat's settings)
  draws its fields from their places at once where `known()` gives their
  stored values (a form drawn again), and otherwise takes no place,
  calls no `track` and keeps its fields off (`disabled`) until they have come,
  so that its drafts start from them and nothing typed meanwhile is lost.

These builders of `components.js` and `app.js` have no part yet, since no region
that has moved uses them: `countList`.
A part that counts what a card holds gives it from an effect, by the card's id
(`cardBadges`).
Known gaps, for the blocks that meet them:
a device's type and identifiers, drawn twice:
in a device row's line of expert mode (`rooms/Devices.tsx`, `.device-sub`,
grey, with when it was last seen and a virtual thermostat's sources)
and in a joined device's head (`cubes/Pairing.tsx`,
the type bold, the serial always),
each `typeName` and the shared `DeviceIds`, so what is left to make one piece
is the two spans' markup, which differs in its classes and what follows;
a Cube's rooms read by a part of their own beside the page's,
the pairing's `cubeRooms` (`cubes/flow.ts`), the log's `logState.cubeRooms` and
`fetchRooms` of `app.js`: one store of them for the log's block, and signals of
the data with Block 15;
the stepped setpoint's end, `apiUntil` of `rooms.js`, which stays with its users
beside `time.ts`;
a room's write's outcome in words, built in three places
(`House.tsx` `houseRequests`, `rooms.js` `resultText`, `configuration.ts` `sendCopy`);
a failed save's error with its unsaved form's item (its tooltip),
which `actionRunner`'s `run` does not hand back:
it says only whether the request was taken,
and the error is in the message line;
the page's modal dialog: `FormsDialog` is the list of forms with a title,
and a later modal takes its shell out of it
(its opening by a property, the focus, its closing);
a sign-in through a provider after a 401 leaves the page while drafts are kept,
so the browser's prompt asks there too, and staying shows the dialog above the sign-in,
whose forms cannot be shown while signed out.
The block that first needs one writes it under `ui/src/shared/`,
knowing no region, and adds its row to the table above; no region writes its own.
A section card's badges are the page's chrome's (`ui/src/chrome/Cards.tsx`):
a region gives them by the card's id, a part with `cardBadges`,
`cubes.js` the Cubes card's with `MassimoParts.chrome.badges`.

A region that moves calls these, in this order:

- `counter()` for its load counter, bumped by its plain names,
  which other scripts call and which call the part,
  and after each of its requests;
  `useLoad()` in each of its parts, at the top of its drawing;
- `register` for `MassimoParts.<region>`, whose functions its plain names call,
  and `mounter` for each root, drawn into the container the page has for it;
- `liveMap` for what it keeps per item and place (a panel open, by its key);
- `Fold` for a panel or a fold, `Island` for a plain piece it keeps inside;
- `actionRunner` for its requests and their message line, `useBusy` for the button;
- `useDraft` with `TextField`, `Switch` or `Field` for a field that shows stored data,
  `FormFooter` for its Save, `useForm` for a settings form that keeps its changes
  and marks them unsaved, `useAsk` and `Question` for an action that asks first.

The Cubes (`ui/src/cubes/`) are the first such region:
`index.tsx` mounts System settings' list in `#cubes-list` and Cube settings' panel
in `#cube-page-body`, a panel in one place at a time,
and registers `render`, `polled`, `openFound`,
`pairingEvent` and `pairingJoined`,
which `cubes.js` calls as `renderCubes`, `cubesPolled`, `showFoundFold`,
`pairingEvent` and `pairingJoined`;
`store.ts` holds its counter, each panel's `open` by serial, the folds
and the devices loaded for the hide choice;
`send.ts` holds its requests, run by its action runner;
`flow.ts` a Cube's pairing (AP UiParts Block 9), live by its serial:
the form shown, the time typed, the server's state, the devices that joined,
the last outcome, Start's or Stop's request under way,
the seconds left, which its timer moves on every second,
asking for the state every third second where the stream does not report it,
and what each joined device's form keeps, made where the device is listed;
the Cube's rooms its forms offer, read from `GET …/rooms` for every Cube,
the shown one too;
and what it sends, as `pairing.js` sent it;
at a sign-out,
and for another user's sign-in after a 401 (`MassimoParts.signedOut`,
by `forgetSession` of `login.js`),
`index.tsx` forgets them all, the countdowns stopped;
`Pairing.tsx` draws it in the panel's `.cube-pairing`, its time a `NumberField`
with its `limit`, Start and Cancel a `FormFooter`,
the outcome a `ResultLine`, each joined device its type, a `DeviceIds`
and the shared `PutForm`;
`Actions.tsx` the row of a Cube's actions, Repair… with its box
the shared `RepairButton` and `RepairBox`;
`Form.tsx` a Cube's settings as a `useForm` of the key `cubeFormKey(serial)`
(`model/cube.tsx`, which lists its `unsavedItems` in `cubeEntry`,
so in the panel head's pill and in the header's entry of the Cube),
and `index.tsx` forgets a Cube's form when the list no longer gives the Cube.
The header (`ui/src/header/`) is the second:
`index.tsx` draws the pile in `#header-pile` (`Pile.tsx`: every Cube's problems,
`CubeProblems.tsx`, led by the server's sign, `ServerSign.tsx`,
with Refresh now and the reads' times, `Times.tsx`),
the Cube choice in `#header-cube`, the reads' times of expert mode's line
in `#header-times`, the certificate's line in `#tls-slot`
and the home notice inside `#home-notice`, and registers `render` and `tls`,
which `header.js` calls as `renderCubeSelect`, `renderUpdated`, `renderServerSign`
and `renderTLS`, and `home.js` as `renderHomeNotice`;
`store.ts` holds its counter, the search under way and the certificate,
`send.ts` the search,
and `clock.ts` the header's one clock, a signal that moves on each second,
which every time the header shows reads,
and `fmtAgo`, how long ago in the page's language;
`fold.ts` measures the row and folds it where it is short (`data-fold` on `.top-line`),
and after each measure places an open list of the row in the window again (`placeMenus` of `components.js`).
The choice is a `Picker`, the problems a `Problems` with `lead`, `always` and `head`,
Refresh now a `MenuItem` with `keepOpen` and its times as `sub`,
the certificate's line a `Notice`.
The logo and the sign-in card's version stay plain,
around the parts' places and not in them;
☰ with its version, and ↑, are the page's chrome's (`ui/src/chrome/`),
and the home notice's place is shown and hidden by the plain code (and by the pages' store, `ui/src/shared/pages.ts`),
the part drawing inside it.
The rooms (`ui/src/rooms/`) are the third:
`index.tsx` draws the House tile's modes and panel in `#house-slot`,
a tile per room of the chosen Cube in `#rooms`,
kept by the Cube and the room's id, and the devices without a room in `#unlisted-devices`,
and registers `render`, `data`, `unlisted`, `result`, `ui`, `loaded`,
`houseOpen` and `closeHouse`,
which `rooms.js` calls as `renderRooms`, `updateDevice` (`data` for a room's device,
`unlisted` for one in no room), `showTileResult`, `roomUI`
and `roomsLoaded`,
and `house.js` by their own names;
`store.ts` holds its counter, a counter per room that a state event of one of its
devices raises, what a tile keeps (`newUI`,
the live object `roomUI` gives the plain actions, which set it as before)
and what the House tile's panel keeps (`house`: the mode, the duration,
the temperature and the end typed, the rooms unticked, the last result),
across its close too,
and the timer of an open window's next state by the clock;
`facts.ts` what the rooms alone read of a room and its devices
(a mode's badge, the problems of a tile's head and of a device's row with their label,
the room's notices among them, a device's badges in words);
`Tile.tsx` the tile, `Devices.tsx` a device's row and a tile's device list,
`Config.tsx` a tile's configuration with its tabs, `ConfigDevices.tsx` its Devices tab,
`configuration.ts` what the configuration keeps and sends,
`House.tsx` the House tile, its temperature a `Stepper`, its end a `DateTime`,
its mode and its duration a choice of radio buttons side by side of its own (`Segments`),
its rooms a `Choices` of checkboxes with All,
its Send and Cancel a `FormFooter`;
`Unlisted.tsx` the devices without a room, each row a device's row with
Put into a room… and Remove from the Cube;
the section of the devices without a room is shown and hidden by `rooms.js`,
its message line, `#unlisted-msg` of the partial, an `actionRunner`'s, a success fading,
and the House tile's section is shown and hidden by `rooms.js` too, its title the part's.
The tile's actions stay plain (`rooms.js`, `tiles.js`) and set `roomUI`,
which the tile follows: a write running turns its button and keeps the others off,
a stepper value waiting to be sent shows with Save and Cancel.
The configuration (AP UiParts Block 8) is a part of the tile, open while `roomUI`'s
`config` says so: `configuration.ts` keeps per room of its Cube, live,
the tab shown and the Copy tab's choice, kept across a close, the configuration
and the week programme as loaded with the programme's tag, which a Send of the programme
makes the one it sent, the result line and the write under way,
and sends what each tab sends, as `config.js` sent it;
its tabs are `Tabs`, its fields `TextField`s, `NumberField`s, `Select`s and `Switch`es
with their drafts and their help, the Copy tab's rooms and settings `Choices`,
a move and a removal `Question`s, each footer a `FormFooter`,
the result line a `ResultLine`,
followed by Load the current programme where the programme changed elsewhere;
the week programme editor stays the element it is (`week-program.js`), made once
in an `Island`, its two resets in another.
Each tab but Copy, a command, is a `useForm` (`useConfigForm` of `configuration.ts`)
of the key `cfgFormKey(serial, id, form)` of `store.ts`,
`<room>:cfg:<form>` under the room's `roomFormKey` beside the virtual thermostat's `:vwt`,
the form `room`, `temps` or `program`,
and on the Devices tab `valves` and `device:<radio address>`;
a write the Cube discarded or a device did not take leaves the stored values as they were,
so the form stays changed and failed;
its Cancel resets the form and closes (`cancelOf`), and the programme's draft is the
programme kept in its form's place, a later load taking its place only where
nothing of it differs or the user loaded it again.
The room's own entry in the tile's problem list lists their `unsavedItems`
(`cfgFormKey(serial, id)`, `facts.ts`), so the virtual thermostat's are listed
once, under its own entry;
a click opens the tab from anywhere (`openForm`), by `openRoom` of `store.ts`,
which `openSettings` uses too and which keeps the request until the Cube's rooms
have loaded; `loaded` forgets a device's form when the room no longer has the device.
A room's repairs and a device's Repair… are the shared `RepairButton` and `RepairBox`,
the room's opened from ⋯ by `toggleRepair`, a device's box under its row,
its listing offering the name its room's devices leave free;
the form that puts a device without a room into one is the shared `PutForm`,
over the chosen Cube's rooms, what it holds kept while it is open.
The menus are `Dropdown`s, their items `MenuItem`s, the problems a `Problems`,
the info and the cog `IconButton`s, the holiday form's temperature a `Stepper`
and its end a `DateTime`,
the deletion's question a `Question`, the line of a refused room a `Notice`,
the result lines `ResultLine`s,
a device's badges `Badge`s with their `Age`, its identifiers a `DeviceIds`,
the virtual thermostat's two settings a `Select` and a `TextField` with their drafts (`useDraft`),
a `FormFooter` and a `ResultLine` that its `actionRunner` sets,
a `useForm` of the key `vwtFormKey(id)` (`store.ts`), under the room's
`roomFormKey(serial, id)` of `model/cube.tsx`, which the room's configuration's
tabs take keys under too: `facts.ts` lists its `unsavedItems` in the row's
and the tile's problem list (`vwtItems`), `cubeEntry` those of every room of the Cube
(`unsavedItems(roomFormKey(serial))`), the row shows outside expert mode while
`hasChanges` says so, and `openSettings` (`store.ts`) chooses the Cube, shows the rooms
and keeps the request until that Cube's rooms have loaded;
`index.tsx` registers `loaded`, which `loadRooms` calls as `roomsLoaded`
after a load that went through and before the rooms are drawn:
it forgets a room's forms when the load, from a Cube connected with its greeting,
no longer gives the room or its virtual thermostat
(a Cube the server has not greeted answers no rooms at all),
and opens the settings asked for;
the Cubes' `index.tsx` forgets `roomFormKey(serial)` with the Cube;
the tile's result line is a `ResultLine` too, its success fading by `fader`.
System settings' lists of users, roles and sign-in providers (`ui/src/admin/`,
AP UiParts Block 10) are one directory for four regions of the guard,
Users, Roles, Capabilities for every role and Sign-in providers,
since they read each other's data and no region imports another:
the role choice reads the roles, a user's identities the enabled providers,
a provider's role the roles.
`index.tsx` draws each into its card's body, `#sys-users-body`, `#sys-roles-body`,
`#sys-caps-body` and `#sys-providers-body`, reads it each time System settings is shown,
and registers `users` (`load`, `render`), `roles` (`render`)
and `providers` (`render`),
which `users.js` calls as `loadUsers` and `renderUsers`, `roles.js` as `renderRoles`
and `providers.js` as `renderProviders`;
`store.ts` holds the one counter, the lists as the last load that went through gave
them, each panel's `open` by its item's key, each new item's form shown, the message
lines' outcomes and `reads`, the users' change counter, at which an open user's panel
reads its sessions, keys and identities again, as every redraw of the plain list did;
`sendUsers.ts`, `sendRoles.ts` and `sendProviders.ts` what each reads and sends,
line for line as the plain scripts did, through an `actionRunner` per list;
`Users.tsx`, `Roles.tsx` and `Providers.tsx` the parts,
a user's sessions and keys and its identities the shared `Tokens` and `Identities`
(`shared/tokens.tsx`, `shared/identities.tsx`, AP UiParts Block 13), drawn with this
list's runner, `reads` and its forms of an identity's change:
a panel a `Fold` with its `FoldHead`, its settings form a `useForm` of the key
`users:<name>` (an identity's change `users:<name>:identity:<id>` under it),
`roles:<key>` or `providers:<key>`, its footer a `FormFooter` with Reset,
its Remove a `useAsk` with a `Question`, the new item's form under an `Adder`,
a role's set and the capabilities a `Choices` of the class `cap-grid`.
A user's new password, one's current one, a provider's client secret and a key just
made are a part's own state, never a draft of the forms' store;
a load that went through forgets the forms of the users, roles and providers it no
longer gives;
at a sign-out, and for another user after a 401 (`MassimoParts.signedOut`),
every list, panel, new item's form and message is forgotten.
The page's chrome says where they are unsaved (`ui/src/chrome/`, AP UnsavedMarks
Block 5), a region of its own drawn once as `parts.js` loads:
a section's pill in its `span.section-pill` beside the badges (`Cards.tsx`),
the forms it shows the beginnings of their keys in `data-forms`, its entry's name
the text key in `data-title`;
a ☰ entry's disk after its name (`Menu.tsx`, the forms of each page in `chrome/pages.ts`:
System settings' `users roles providers integrations`; Cube settings' `cube`;
Personal settings' `profile scheme`);
☰'s mark at its corner, the worse of the log's, which `renderLogSign` of `log.js`
hands it as `MassimoParts.chrome.log(count, tone)`,
and the forms' of every entry (`unsavedTone`), with ☰'s name and tooltip.
☰ and the header's page buttons are the chrome's too (AP UiParts Block 14,
`ui/src/chrome/Menu.tsx`), drawn into the header's `nav#nav`:
↑, Rooms and ☰, its first row Massimo's mark (the shared `Brand`) and the server's
version, which `header.js` hands it as `MassimoParts.chrome.version(v)`,
and its entries from the pages' store (`ui/src/shared/pages.ts`) and who is in
(`ui/src/shared/me.ts`), worded by its `whoOf`,
as the personal page's row words it:
Sign out or Forget key, and whose Personal settings they are;
in the app Select server, by `appConnect` of `login.js`, and Sign out by its `logout`.
My settings (`ui/src/prefs/`, AP UiParts Block 13) draws their rows into
`#prefs-rows` as `parts.js` loads, before `init` applies the settings, and
registers `prefs` (`render`), which `prefs.js` calls as `renderDeviceSwitches`
and `renderLangSelect`, where the plain code applied the settings
(`applySettings`, a sign-in's `loadPreferences`) or changed the language;
the settings stay `app.js`' plain `state.settings`, saved by `setSetting` as
before, and `store.ts` holds the counter that draws them again;
`rows.tsx` a row with its switch that keeps the setting on this device alone
(`DevRow`), `Settings.tsx` the rows: each saved as it is changed, the poll as it
is left, a poll being typed kept as a draft through the settings applied again
(without the "changed meanwhile" notice, which the plain field had not: leaving
the field for Take theirs would save it).
The Appearance card is drawn into `#appearance` the same way, `appearance.js`
keeping `applyAppearance`, which `app.js` and `theme.js` call, as a call of it,
and the system's changes it follows: `Appearance.tsx` its rows, its buttons,
their line and the editor while it is open, and beside the window's and the
radio's choice their drawing as the page shows it (`Icon`, `Sign`), drawn again
by the icons' signal; `theme.js` applies the choices to the page and writes into
none of it;
`scheme.ts` the editor's state beside `theme.js`' `themeState.draft` and `base`,
which the page applies as the draft changes, and what Save as, Delete, Export
and Import do, as `appearance.js` did;
`Editor.tsx` the editor, a `useForm` of the key `scheme`, its changes the
draft's colours and the name typed against the scheme it started from, Save as a
`SaveButton` beside the name, off while the editor holds no change, Enter in the
name saving only where it would (UiParts D204), Reset putting both back,
a hex field's text typed
or refused kept by the field; closing the editor drops the draft and forgets its
form, and so does a sign-out.
Notifications (`ui/src/notify/`, AP UiParts Block 13) is a part alone, drawn
into `#notify-sec` as `parts.js` loads and read each time the personal page is
shown, the line emptied first, as `notify.js` read it; it registers `notify`
(`render`, `save`, `takeSave`), which `notify.js` calls as `renderNotify` at a
change of language and as `saveNotifyRules` and `takeNotifySave`, which
`login.js` `holdPushes` and `resumePushes` hold and send again around a sign-in,
as before;
`store.ts` holds the kinds with their defaults, the rules once read, the
devices, this browser's endpoint, the name of this device typed and the line
with whose outcome it shows;
`send.ts` what it reads and sends, line for line as `notify.js` did, a read that
finds a change of the rules waiting keeping the rules shown, which that change
sends;
`Notify.tsx` the card: a rule's switch, saved a second after the last change,
its delay or threshold a draft of the rule shown with its limit (`markRange`),
taken as the field is left;
a rule has no Save, so the card has no marks (UnsavedMarks D6-A).
The rules are off until this session's first read has answered, so that a
change does not send the defaults over the rules the user has.
Sign out sends a change still waiting at once, under the user's own token, as it
sends the waiting settings (`sendNotifySave`, UiParts D205); for another user
after a 401 `holdPushes` takes it and drops it, for the same user sends it again.
At a sign-out, and for another user after a 401, the rules, the devices, the
name typed and the line are forgotten; an answer after it is dropped.
The toast and the service worker's language stay plain in `notify.js`.
My profile (`ui/src/profile/`, AP UiParts Block 13) is a part alone, drawn into
`#profile-sec` as `parts.js` loads, for the user `MassimoParts.me` names
(`shared/me.ts`, which `applyCaps` of `login.js` calls at every read of who is
in), and read each time the personal page is shown, as `profile.js` read it: the
user (`GET …/users/{name}`), the enabled providers where none are known yet,
then its sessions and keys and its identities, and after every change made there
the users again where the signed-in user may read them, then My profile;
`store.ts` holds the user as that read gave it, the providers, `reads`, the link
come back shown to confirm, Change password open and the two message lines'
outcomes;
`send.ts` what it reads and sends through its runner, the requests of
`profile.js`, `linked.js` and `login.js`' `changeOwnPassword`, in the order: the
user, the providers where none are known yet, then the sessions and keys and the
identities together;
`Profile.tsx` the parts: the display name a `useForm` of the key `profile:name`
and the own password one of `profile:password`, each with Reset and Save by its
state, the password's three fields its part's own state, never a draft and kept
by no key, the form's marks knowing only that a new password is typed, the
current one alone, as a password manager fills it in, being none (UnsavedMarks
D28);
the shared `Tokens` and `Identities`, the latter `quiet` and with the Link
buttons after it, and the link come back above it, its title taking the focus as
it comes.
Another user signed in draws all of it from nothing:
at a sign-out, and for another user after a 401, `MassimoParts.signedOut` sets
who is in to null, which takes the parts and what is typed in them away, and
forgets the user read, the link shown and the messages; an answer after it is
dropped (`session.ts`).
No plain script calls it.
Integrations (`ui/src/integrations/`) is a part alone, drawn into `#sys-integrations-body`
and read each time System settings is shown; no plain script calls it:
`store.ts` holds its counter, what it shows as the last loads gave it,
each panel open, each request closed, the message line and the readings of an approval
again, every 3 s in its first minute and every 15 s after, stopped while the list is not
visible (another page, the card folded) and at a sign-out, and started again by every load
of the list that went through, which says a key collected meanwhile, and what it sends;
`Integrations.tsx` its parts, the facts a `KvList`, a request's maximum and an
integration's a `Choices` under a legend, of the class `integration-caps`,
an integration's name and its maximum each a `useForm` of the key
`integrations:<uuid>:name` and `:max`, Acts as a `Select` with its draft beside Link,
Refuse and Revoke a `useAsk` with a `Question`.
Backup and restore (`ui/src/backups/`) draws System settings' folds into `#backups-cubes`
and Cube settings' box into `#cube-backup-body`, a box per Cube and place,
and registers `render`, `cubeChanged`, `restoreEvent` and `resynced`, which `backup.js`
calls as `renderBackups`, `backupCubeChanged`, `restoreEvent` and `restoreResynced`;
`store.ts` keeps what a Cube has by its serial, live, so that its boxes on both pages
show the same: its list, whether the server keeps it, the comparisons, the restore
shown and the one sent, a running one shown as unseen at a resync
(`app.js`'s resync handler calls `restoreResynced`);
each box's message line and format by its place and serial;
each fold open, a single Cube's at first; and what it reads and sends, as `backup.js`
did; `Backups.tsx` the box, a fold a `Fold` whose `FoldHead` has the Cube's sign
`after` its serial, a deletion a `useAsk` with a `Question` in its row.
Leaving the page (`ui/src/leaving/`) is a region of its own,
a dialog without a place in any page's region:
the page's chrome stays plain until AP UiParts' Block 14, and no region imports another,
so it knows the forms only through `forms.ts`.
`index.tsx` draws a `FormsDialog` into `#leaving` at the end of the page
as soon as `parts.js` loads, and adds its `beforeunload` and `pagehide` listeners
only while `changedItems` has any, removed once none has,
since a page with a `beforeunload` listener can lose the browser's back and forward cache:
at the prompt it counts the forms left (`leaveForms`), and the dialog with Close
comes `STAY_AFTER_MS` (a second) later where the page is still there:
a browser does not tell the page the user's answer,
and a page that is left runs on until the next one comes (a Pi 1 is slow to send it),
so the dialog shown at once would come on a page that is leaving;
`pagehide` takes back the dialog of a stay, and one that waits to come,
so that a page kept for the back button comes back without it;
once the last form is saved, reset or forgotten, the dialog closes
and a sign-out's question is answered as a sign-out;
it registers `signOut(label)`, which `logout` of `login.js` awaits first,
the dialog with Stay and the label of the page's own button,
resolving true at once where no form has changes,
and `changes()`, which `somethingOpen` of `header.js` asks,
so that a newer server's reload waits.
After a 401, the sign-in of `askLogin` forgets every form (`settleSignIn` of `login.js`,
which calls `forgetSession`, the page's part of `logout`,
and with it `MassimoParts.signedOut`)
where the user who signs in is not the one who was,
by `/me`'s user or mode, before the reads that waited for it are sent again.
Moving inside the page (the pages' store, the Cube choice) unloads nothing,
so nothing asks there.
A part's place among plain siblings is an element of the class `part-slot`,
which takes no box of its own (`display: contents`),
so that the part's element is laid out as if it stood there itself.
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
then checks its hooks with Biome's two rules (`tools/ui-check/biome.jsonc`):
a hook at the top of a part's drawing, never after an early return or in a branch,
and an effect that lists what it reads,
or says why not in a `biome-ignore` comment with its reason;
with the TypeScript and the Biome `tools/ui-check/` locks (its `README.md`),
and CI's job UI types runs it.
What a part reads from the API is typed by `ui/src/api.d.ts`,
one declaration per schema of `docs/api-v1-openapi.yaml`,
`Cube`, `Room` or `Device`, which a part imports with `import type`:
`make api-types` writes it (`tools/apitypes`, its `README.md`),
git keeps it, and `go test` fails while it does not follow the OpenAPI file.

Two tests hold the files of both kinds of region to the contract.
`TestUIFileHeaders`: every `ui/*.js` and `ui/*.css` opens with the SPDX line
and the copyright line, and every file under `ui/src/` with both as `//` lines,
then its `@file` header, which names its own path from the repository root
and says what the file is for.
`TestUINoHTMLInjection`: no script and no part puts markup into the page from a
string (`innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`,
`dangerouslySetInnerHTML`) or sets an inline handler (`setAttribute("on…")`,
an `on…=` attribute in `ui/*.html` or a partial),
and no part carries a `data-i18n`, which `applyI18n` would write over;
comments are left out, so that prose may name them.

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

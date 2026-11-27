# Massimo HTTP API, version 1

The API is served by `massimo serve` (default `127.0.0.1:8099`),
plain HTTP, or TLS with `-tls` (README.md, "Server");
with TLS every response carries `Strict-Transport-Security`,
a year by default (`-hsts <days>`, `0` clears it).
Every response is JSON unless stated otherwise.
The machine-readable description is in [api-v1-openapi.yaml](api-v1-openapi.yaml).
The web UI's types are generated from its schemas (`make api-types`,
`tools/apitypes`), so a change to a schema writes them again in the same change.

The server keeps one persistent connection and one command queue per Cube.
Reads answer from the server's copy of the Cube's state,
which the Cube refreshes on every change and which the server polls every 10 seconds.
Writes go through the queue, which serialises commands, respects the
radio duty cycle and the Cube's memory slots
(a command of `r:` lines alone, which take neither, does not wait for them),
and resets a device's communication error (`error`) before a command that reaches it:
one `r:`, which costs no radio time, for each device whose flag is set
that a command names or sends a radiogram to directly,
and for every member of the room a room command names whose flag is set.

## Web UI

The server also serves a web page at `/` (files under `/ui/`) built on this API,
plain HTML and JavaScript embedded in the binary, no setup.
It shows one tile per room in the manner of the original software:
the measured temperature (IS, from a wall thermostat,
or from a radiator thermostat when it reported one;
"–" otherwise) and the setpoint (SET) side by side at the same size,
each with a small icon above its label (a thermometer for IS, a gauge for SET),
with up and down chevrons next to SET (shown on hover,
always on touch screens) that change it in 0.5 °C steps and send the value as a
manual setpoint two seconds after the last click,
with Save and Cancel while it waits; AUTO, MANU and HOLIDAY buttons,
HOLIDAY opening the vacation panel and, while the room is in vacation mode,
highlighted with the end date next to it;
AUTO and MANU close an open vacation panel.
Right of the mode buttons an info button opens the device list and a cog the
configuration, one of the two at a time,
a click on the highlighted one closes it.
The ⋯ menu keeps lock and unlock of the room's thermostats and,
for an empty room, deletion.
A House tile sets every room to ECO or COMFORT (manual at the room's eco
or comfort temperature from its thermostat's configuration), AUTO or MANU;
each of its buttons opens an inline panel with everything on one screen,
no wizard: the mode (ECO, AUTO, COMFORT, or MANU with a temperature),
"Permanently" or "Until" a date and time (tomorrow 06:00 by default;
a temperature with "Until" is sent as vacation mode),
the rooms to apply it to with an "All rooms" box, and Send,
which runs the room commands one after another and lists the result per room.
Enter in any field sends.
Vacation for one room is an inline panel in its tile in the same way,
and deleting an empty room asks inline.
The device list (room name or info button) shows each device with its mode
or window state as a filled badge at the right (green auto and closed,
amber manual, blue holiday, red open), warnings outlined before it,
type and address on a second line,
and the values in two columns with the decimals aligned: valve and offset left,
set and actual right in bold with the gauge and thermometer icons,
and the per-thermostat lock.
Setpoints are set per room only, as in the original software.
The cog of a tile expands the tile into the room's configuration,
one screen per section with Send and Cancel, Enter sends: Room (rename),
Temperatures (comfort, eco, open-window temperature and minutes,
maximum and minimum; only the changed fields are written,
to every thermostat of the room, with the result per device),
Devices (boost duration and valve opening, descaling day and hour,
valve limits for the room, then per device its name, temperature offset,
the display mode of a wall thermostat,
and a move to another room behind an inline confirm),
Week programme (day buttons:
a click edits that day and selects every day with the same programme;
the "Only {day}" switch, off by default, sends an edit to all selected days,
on to the edited day alone, which then leaves the group;
the day's switch points as rows "until … at … °C" with add and remove,
the last fixed at 24:00, a bar of the day above them;
a "Copy {day} to …" section with every day as a toggle,
the days already sharing the programme greyed out, "All days" and Copy,
after which the copied days join the group while the switch is off;
Reset day and Reset all left of Cancel and Send;
Send writes the days that differ from the loaded programme),
and Copy (a Rooms section with a switch per room, "All rooms" and "None",
and a Settings section with Temperatures,
Week programme and Radiator settings as switches;
what is stored for this room goes to each selected room, one after another,
through `PUT …/rooms/{id}/config` with the chosen fields and `PUT …/rooms/{id}/program`,
with a result per room).
A help button next to each setting shows a short explanation of it.
All texts come from one translation table (`ui/i18n.js`),
English and German so far;
the language follows the browser or the Language setting.
Icon buttons carry labels for screen readers,
the mode buttons are a group with their pressed state,
the page icons a navigation with the current page marked,
every control has a visible focus ring, and Escape closes an open menu or panel;
the pages pass an axe-core check without violations.
Three icon buttons at the right end of the header switch between the pages Rooms,
Log and Settings:
the log lists the Cube's recent events from `GET …/log` and then the live ones,
one line each with the local time, kind, device and a short state text,
with a filter box and a clear that only empties the view;
the tiles keep their state meanwhile.
The short server version stands next to the brand;
the Cube selector shows the Cube's name when one is configured.
My settings, on the personal page the header's 👤 opens,
has the poll interval (10 s by default;
the header is refreshed on it,
and the rooms as well while the event stream is down;
the ↻ button in the header runs the same refresh at once, rooms included,
and the header shows the time of the last successful poll with seconds),
the Expert switch (off by default; on,
a second header line shows the server version, the Cube's firmware,
IP address and radio address, the duty cycle, free slots,
queued and pending commands and the Cube clock,
and the tiles show radio addresses and device types),
and whether the Cube clock is shown in the browser's time zone or the Cube's own;
that page says who is signed in and signs out.
In the tiles a thermometer marks the measured temperature (IS) and a target the
setpoint (SET);
AUTO and MANU close an open holiday panel and drop a pending stepper value.
The device list shows per device its name with the mode or window state as a
badge and the warnings next to it, the type and address beneath, and valve,
offset, set and actual in a grid with the decimal points aligned.
On every poll the page compares the server's version from `/healthz` with the
one it was loaded with and reloads itself when they differ,
immediately when nothing is open,
otherwise it offers a "New version on the server, reload" button.
The Cube clock is converted from the Cube's time zone rules (`config.tz_winter`
and `tz_summer` of the cube detail); hovering it shows the same instant as UTC.
The selected Cube and the settings are kept in the browser.
Results of every action appear in the tile,
including the devices to nudge when the Cube discarded a room command.
The page follows the event stream.
With users, the page asks for a name and a password (or an API key) once and
keeps the session in the browser, hides what the role may not do, and offers
System settings → Users and Roles to those who may;
every route needs the header except the event stream,
which browsers open without headers and which takes a ticket (see Authentication):
the page gets a new one for every connection and, after an error,
closes the stream and opens it again with a fresh ticket,
waiting longer after each failure.
The tiles and the device list show how old the values are from `received` and `last_seen`,
and warn about stale devices.
The page and its files are public, the API behind them is not.

## Authentication

The server authenticates in one of three ways, by what it has.
In this document a server's users are its regular users,
those that are not an integration's, disabled ones included:
"a server without users" is one whose only users, if any, are an integration's.

- **Users**, a store with at least one regular user: every `/api` request carries
  `Authorization: Bearer <token>`, a session from `POST /api/v1/login`
  (or made from a key by `POST /api/v1/login/key`) or
  an API key from `POST /api/v1/users/{name}/tokens`; otherwise `401`.
  The token names a user, the user's role grants capabilities,
  and every route needs one of them (the table below);
  a request without it is `403` with the message `capability "pair" required`.
  A disabled user, a guest past its end date or a disabled role is `403` too,
  with the reason.
- **Open**, a store without regular users, the state before the first owner:
  every request from the local network may do everything,
  as the original software can on the LAN.
  A regular user is every user that is not an integration's,
  a disabled one included;
  an integration's users do not count, so a server with an integration
  approved and no owner stays open.
  Its calls are then anyone's from the local network, as every other:
  not attributed to its users,
  `X-MassimoHA-On-Behalf-Of` not read, its maximum not applied,
  and `GET /api/v1/me` with its key says `"mode": "open"`;
  an installation beyond the local network is refused.
  Its chain of rights begins with the first owner.
  The start of a server whose only users are an integration's warns
  `the server is open: its only users are an integration's; create the owner`,
  unless it is kept open on purpose (`auth.keep_open`);
  the removal of the last regular user warns
  `the server is open: its last regular user was removed; create the owner`,
  naming whom and by whom.
  `POST /api/v1/users` from an address on the local network creates the first
  owner (see that route), and `massimo user add` does the same on the box.
  The first regular user, made either way or from `MASSIMO_API_TOKEN`
  at a start or a reload of the settings file,
  is told once what was set up before it and still works, where anything was,
  as a rule while the server was open, else left by an owner since removed
  (AP AccessEnds, Block 3):
  the integrations active with a key, or one still to be collected,
  the sign-in providers enabled, the roles added or changed,
  and the capabilities turned off, by name, never a key or a secret.
  The log's `open_setup` line at `warn` says it,
  an alert until a person marks it seen, with its line in the server's log,
  `the first regular user was made; what was set up before it still works: check it`,
  and the first owner's answer of `POST /api/v1/users` holds it as `open_setup`.
  The setting `auth.keep_open` says the server is kept open on purpose,
  so that the web UI does not ask for the first owner;
  it changes nothing about who is let in,
  and goes with the first regular user (GET and PUT /api/v1/settings).
- **Token**, no store (`serve -addr`):
  `MASSIMO_API_TOKEN` (or `-token`) is the one token, as before the store;
  without it the API is open as in open mode.

An open server answers its local network only:
the box itself, the private ranges (RFC 1918),
IPv6 unique local and link-local addresses.
A client beyond them gets `403` with
`this server is open and answers only its local network; …`,
and so does a request whose forwarding header names a client,
`X-Forwarded-For`, `X-Real-IP` or `Forwarded`,
unless it comes from a proxy of `-trusted-proxies`, whose client then decides:
a reverse proxy on the box would otherwise pass the internet through as itself.
`/healthz` stays open.

`/healthz` is always open.
The errors say which: `401` is a missing, wrong, revoked or expired token,
`403` a token whose user may not do this.

A write that another site's page sent is refused in every mode,
before the token is looked at,
with `403` and `{"error": "cross-origin request refused"}`:
a request whose `Sec-Fetch-Site` is `cross-site` or `same-site`,
or, without that header, whose `Origin` names another host than its `Host`.
A browser sends a `POST` with a plain-text body from any page without asking
the server first, and an open server would serve it.
`GET`, `HEAD` and `OPTIONS` pass,
and so does a client that sends neither header, such as curl or a script.
Behind a reverse proxy see [reverse-proxy.md](reverse-proxy.md),
"The `Host` header".

`MASSIMO_API_TOKEN` and the store:
at the first start with the variable set and no user,
the server creates the user `owner` with the role `owner`,
no password and the token as its API key (label `MASSIMO_API_TOKEN`),
so every client that sends the token keeps working with full rights.
At every later start, and whenever the server reads the file again while it runs
(it looks every five seconds, and on SIGHUP; see README.md, "Server"),
a token in the file that the store does not know is added to an enabled owner's keys,
and a key with that label whose secret is not the file's any more is revoked.
The file's key that an older start gave to an integration's user, live or revoked since,
moves to that owner, or to the user `owner` the start makes where there is no other,
since an integration's user holds no key of its own.
The file stays the way to recover an owner who lost the password:
put a token into it, log in with it, set a password; no restart is needed.

The capabilities, what each allows and the routes it guards,
`…` standing for any part of a path;
a test holds each row to the routes the server registers with its capability,
and the OpenAPI file carries each operation's as `x-capability`:

| Capability | Allows | Routes |
|---|---|---|
| `read` | see everything of the Cubes and one's own user: rooms, devices, configuration, programmes, the log and the stream, the pairing and restore state, a command's outcome, the server's log and its alerts, the system's facts, the update state | `GET /api/v1/cubes`, `GET /api/v1/cubes/{sn}`, `GET …/rooms`, `GET …/devices`, `GET …/devices/{rf}`, `GET …/rooms/{id}/config`, `GET …/rooms/{id}/program`, `GET …/devices/{rf}/program`, `GET …/log`, `GET …/events`, `POST …/events/ticket`, `GET …/pairing`, `GET …/restore`, `GET /api/v1/commands/{id}`, `GET /api/v1/log`, `GET /api/v1/log/alerts`, `GET /api/v1/system`, `GET /api/v1/push/vapid`, `GET /api/v1/update`; `GET /api/v1/me` and one's own preferences, `GET /api/v1/me/preferences…`, for any signed-in user |
| `preferences` | keep one's own settings and colour schemes on the server, so that they follow the user across browsers, and be notified; without it they stay in the browser | `PUT`/`DELETE /api/v1/me/preferences/{key}`, `GET`/`POST /api/v1/me/push`, `DELETE /api/v1/me/push/{id}` |
| `setpoint` | set temperatures and modes | `PUT …/devices/{rf}/setpoint`, `PUT …/rooms/{id}/setpoint` |
| `lock` | lock and unlock the thermostats' buttons | `PUT …/devices/{rf}/lock` |
| `configure` | temperatures, device settings, week programmes, and new batteries marked | `PUT …/rooms/{id}/config`, `PUT …/devices/{rf}/config`, `PUT …/rooms/{id}/program`, `PUT …/devices/{rf}/program`, `POST …/devices/{rf}/battery`; the copy of a room's settings in `PUT …/devices/{rf}` |
| `rename` | room and device names | `PUT …/rooms/{id}`, `PUT …/devices/{rf}` with `name` |
| `structure` | move and remove devices, delete rooms | `PUT …/devices/{rf}` with `room_id`, `DELETE …/devices/{rf}`, `DELETE …/rooms/{id}`; beside `repair`, `GET`/`POST …/repair/unlink-relink` |
| `pair` | put the Cube into pairing mode and stop it | `POST`/`DELETE …/pairing` |
| `discover` | search for Cubes on the LAN | `POST /api/v1/cubes/discover` |
| `backup` | download a backup, and the backups the server keeps | `GET …/backup`, `GET`/`POST …/backups`, `GET …/backups/{id}`, `GET …/backups/{id}/diff`, `GET /api/v1/backups/unserved` |
| `restore` | plan and run a restore, delete a kept backup | `POST …/restore/plan`, `POST …/restore`, `DELETE …/backups/{id}` |
| `repair` | the repairs of a Cube's rooms and devices: their communication errors reset, a device's configuration re-sent from its room, a device placed or listed where the Cube and its list disagree, a room's master named, a room re-linked; "unlink all and re-link" needs `structure` as well ("Repairs", below) | `GET`/`POST …/repair/…`; `GET`/`POST …/repair/unlink-relink` with `structure` as well |
| `guests` | create, invite, change, disable and remove users whose role is `guest`, and give them an end date: regular ones, or for a call through an integration that integration's own, which makes no user ("Users", below) | the users routes, `GET`/`POST`/`PUT`/`DELETE /api/v1/users…`, for those users |
| `users` | every user: create, role, disable, password, tokens, the identities they sign in with | `GET`/`POST /api/v1/users`, `GET`/`PUT`/`DELETE …/users/{name}`, `GET`/`POST …/users/{name}/tokens`, `DELETE …/users/{name}/tokens/{id}`, `POST …/users/{name}/register`, `GET`/`POST`/`PUT`/`DELETE …/users/{name}/identities…` |
| `passwords` | set the password of a user (the manager's by default) | `GET /api/v1/users`, `GET …/users/{name}`, `PUT …/users/{name}` with a password, `POST …/users/{name}/register` |
| `roles` | a role's capabilities and name, create and delete roles | `GET`/`POST /api/v1/roles`, `PUT`/`DELETE /api/v1/roles/{key}` |
| `roles.disable` | disable and enable a role | `PUT /api/v1/roles/{key}/disabled` |
| `capabilities` | disable and enable a capability for every role but `owner` | `GET /api/v1/capabilities`, `PUT /api/v1/capabilities/{key}` |
| `cubes` | add, configure and remove Cubes | `POST /api/v1/cubes`, `PUT`/`DELETE /api/v1/cubes/{sn}` |
| `cubes.reboot` | reboot a Cube, which drops the radiograms it has not delivered; seeded for no role | `POST /api/v1/cubes/{sn}/reboot` |
| `settings` | the server's settings, its store and the settings in effect | `GET`/`PUT /api/v1/settings`, `GET /api/v1/store`, `GET /api/v1/config` |
| `update` | start a software update and answer the automatic update's question; includes `restart` | `POST /api/v1/update`, `PUT /api/v1/update/answer` |
| `restart` | restart the server through the service manager that runs it, read its settings file again, and read a Cube again at once | `GET`/`POST /api/v1/restart`, `POST /api/v1/reload`, `POST /api/v1/cubes/{sn}/refresh` |
| `reboot` | reboot the computer the server runs on, where its owner switched that on; includes `restart`; seeded for no role | `GET`/`POST`/`DELETE /api/v1/reboot` |
| `audit` | read the audit | `GET /api/v1/audit` |
| `auth` | the OpenID Connect providers users sign in through; seeded for no role | `GET`/`POST /api/v1/providers`, `PUT`/`DELETE /api/v1/providers/{key}` |
| `integrations` | the registered integrations: requests, approval, their maximum, their users and links ("Integrations", below); an integration's users but those stored as `owner`: their name, state, end date, keys and removal, and a role within their integration's maximum ("Users", below) | `GET /api/v1/integrations`, `GET …/integrations/found`, `GET`/`PUT`/`DELETE …/integrations/{uuid}`, `POST …/integrations/{uuid}/approve`, `POST …/integrations/{uuid}/refuse`, `POST …/integrations/{uuid}/accept-instance`, `PUT …/integrations/{uuid}/users/{id}`; the users routes for an integration's users |
| `backup.server` | the server's own backups: make one now, pin, check, download and upload one, push and pull a copy, delete one, and the targets with what each takes and its retention; the keys' list; includes `backup.retry`; seeded for no role, never an integration's ("Server backups", below) | `POST /api/v1/server-backups`, `POST …/server-backups/upload`, `PUT …/server-backups/{id}/pin`, `POST …/server-backups/{id}/check`, `GET …/server-backups/{id}/file`, `PUT`/`DELETE …/server-backups/{id}/copies/{target}`, `POST …/copies/{target}/pull`, `DELETE …/server-backups/{id}`, `GET`/`POST /api/v1/server-backup-targets`, `GET …/server-backup-targets/kinds`, `PUT`/`DELETE …/server-backup-targets/{uuid}`, `POST …/server-backup-targets/{uuid}/test`, `GET /api/v1/server-backup-keys` |
| `backup.retry` | see the state of the server's backups and retry a failed backup or copy, nothing more (the manager's by default) | `GET /api/v1/server-backups`, `POST …/server-backups/{id}/retry` |

<!-- BEGIN GENERATED PARTIAL docs/fragments/_capability-rules.md - do not edit; run "GUIDELINES/shared/tools/sync-docs.sh" -->
`update` and `reboot` each include `restart`, since each restarts the server:
their holder passes every check of `restart` as a holder of `restart` does,
and `GET /api/v1/me` lists `restart` beside them.
`backup.server` includes `backup.retry` alike: who makes,
pushes and pulls the server's backups sees their state and retries one.
The include is taken at the check, on the set the bounds leave
(the role's set less the capabilities disabled for every role,
an integration's ceiling and maximum, a call's level):
an integration whose maximum holds `update` and not `restart` restarts the server,
a call of it at `control`, which leaves no `update`, does not,
and `restart` disabled for every role is disabled for their holders too.
Nothing stored changes, and no migration runs:
`GET /api/v1/roles` shows each role's set as saved,
and the rule below compares the stored sets.
`reboot` does not include `update`, nor `update` `reboot`.

A user changes only users whose role has no capability its own lacks,
and gives only such roles: an owner changes everyone,
nobody but an owner changes an owner or gives the role `owner`,
and a role made with `users` but without, say, `restore` changes no manager.
A holder of `integrations` changes an integration's user whatever its role,
but one stored as `owner`, which stays an owner's alone,
and gives it only a role it outranks
that lies within the integration's maximum (AP AccessEnds, D47-A, D48-A):
so the manager, which does not outrank `integration`,
moves such a user off it, and an owner gives it back.
<!-- END GENERATED PARTIAL docs/fragments/_capability-rules.md -->

The users routes say what the caller may do with each user they list
(`"may": {"manage": true, "password": true, "roles": ["guest", "viewer"], "beyond": false}`):
change it, set its password,
and give it the roles of `roles`, in the roles' order,
empty where it may give none;
of those, for an integration's user only the roles within the role `integration`
and its integration's maximum, and `integration` itself ("Integrations"),
never `owner`.
`beyond` says that an integration's user's role, as stored, lies beyond those,
`false` for every other user:
a role `roles` leaves out may lie within them and be one the caller does not outrank.
So a holder of `users` without `roles` or `integrations` learns from it
the keys of the roles it outranks and, for each integration's user,
which of them lie within that integration's maximum.
A holder of `integrations` may manage an integration's user but one stored as `owner`,
and set no password of it ("Users", below).

The default roles and their sets are those of docs/store.md section 6:
`owner` has every capability and cannot be changed,
`manager` is `operator` plus guests, pairing, names, structure,
restore, the repairs, the restart and the retry of the server's backups,
`operator` runs the heating, locks, configures, discovers and takes backups,
`guest` is `operator` without the lock, backups, discovery and preferences,
`viewer` reads, and `integration`, fixed like `owner`, is the role of the
registered integrations and their users and the ceiling of their calls
(docs/store.md section 6); a regular user is never given it,
and an integration's user given another role may be given it back.
A role's set can be changed, a role renamed, created and disabled,
and a capability disabled for every role but `owner`,
through the routes of `/api/v1/roles` and `/api/v1/capabilities`.
A user's own user and the routes of `/api/v1/users/{name}` with the user's
own name are open to it for its display name and its password.
The keys of the server's backups and their restore are the owner's alone,
a user of the role `owner` and never a call through an integration:
no capability gives them
(`x-capability: owner` in the OpenAPI file; "Server backups", below).

Ten failed token checks from one address within a minute lock that
address out for five minutes: every `/api` request from it is then
answered `429` with a `Retry-After` header, the right token included.
A failed check is a wrong token, an empty `Bearer` header among them,
or a wrong ticket;
a request that offers neither (no `Authorization: Bearer`,
no `ticket` or `token` on the stream) is answered `401` and counts as none,
since it guesses nothing:
a page before its sign-in, or a scanner behind a proxy not in
`-trusted-proxies`, does not lock the address out.
Nor does a token the server issued and that has ended:
a session revoked, logged out or past its end, a revoked key,
a session whose key was revoked, or the token of a removed user
is answered `401` with the words of a wrong token, and counts as none,
since a guessed token never matches one that was issued:
a client that sends again a session that ended,
as a page does after a link that stalled, does not lock its address out
(`POST /api/v1/login/key`'s key included).
A wrong Bearer token counts beside any token and is answered `401` as it is alone:
the stream's `ticket` or `token` beside it is not read;
a wrong ticket counts beside the Bearer token of a disabled user,
of a guest past its end or of a disabled role,
though the request is answered that token's `403`.
A failed login counts against the address and against the user name,
so that password guessing from many addresses locks the name.
With a store the counters survive a restart.
The address is the TCP peer's, or,
with `-trusted-proxies` naming the proxy's address,
the client the proxy passes in `X-Forwarded-For`
(the rightmost entry that is not a trusted proxy;
an entry that is not an address makes it the proxy),
or `X-Real-IP` when there is no `X-Forwarded-For`;
without it the headers are ignored, since anyone can set them,
and behind a reverse proxy the limit then applies to the proxy as a whole
(docs/reverse-proxy.md).

The event stream is opened by browsers without headers,
so it takes a ticket instead:
`POST /api/v1/cubes/{sn}/events/ticket` with the token returns one,
and `GET /api/v1/cubes/{sn}/events?ticket=<ticket>` opens the stream with it.
A ticket is good for one connection to that Cube's stream within a minute
and carries the user that asked for it;
a client gets a new one for every connection,
also after the stream broke off (an `EventSource` stops retrying after a `401`,
so it reports the error and the client opens a new one).
`?token=<token>` on the stream is still accepted but deprecated since 2026-11-05,
and the response says so with the header `Deprecation: @1793836800` (RFC 9745):
it puts the token into URLs, browser history and proxy logs,
and it will be removed in a later release.

## Identifiers

- `{sn}` is a Cube's serial number, as printed on it (`KEQ0000100`);
  case does not matter.
- `{rf}` names a device: by its serial number, as printed on it (`MEQ0000101`),
  case not mattering, or by its radio address, six hex digits (`0a0102`);
  the two cannot be confused.
  A serial reaches a device while the Cube has it, and a serial the Cube
  does not have is `404`.
  Radio address 000000 is no device's and is `400`
  ("radio address 000000 is no device"):
  a Cube can list an entry there, which no list of the API shows,
  and nothing is sent to it (docs/cube-protocol.md).
  The answers carry both, `serial` and `rf_address`,
  and the audit names a device by its serial (`device:MEQ0000101`).
- `{id}` is a room number as the Cube assigns it (1..255).
- A room's virtual wall thermostat is also `{rf}`:
  by its radio address, its serial (`VWT0000005`) or its room, `room-5`
  (see "The virtual wall thermostat").

## Errors

Errors are `{"error": "<message>"}` with a matching status: `400` bad input,
`401` missing or wrong token, `403` a user who may not do this,
`429` locked out after repeated wrong tokens, tickets or passwords,
`404` unknown Cube, device or room,
`409` the command was replaced by a newer one for the same target before it was sent (or,
for a restore, a guard rail: see POST /restore),
`413` a body over the limit (a backup of more than 1 MB),
`502` the Cube discarded the command or the connection failed,
`503` the Cube has not sent its greeting yet.

A room command is discarded by the Cube while it holds no state for a device in that room:
one that never reported,
or one that has not reported since the Cube restarted (window contacts only
transmit on a window change).
The error message then names the devices and the response carries their
addresses as `no_state`.
Make them report (open and close the window, turn the thermostat's wheel),
replace a dead battery, or remove the device from the Cube:

```json
{"error": "engine: command discarded by the cube: the cube holds no state for room 4 members 005447, 1570d3 …", "no_state": ["005447", "1570d3"]}
```

## Routes

### GET /healthz

`{"ok": true, "version": "massimo v0.1.0 …", "started_at": "2026-11-05T18:20:00+01:00"}`

`started_at` is when the server process started;
a new time with the same version means the server was restarted.

`features` is what this server can do, by name and version `"MAJOR.MINOR"`:
`{"event_ids": "1.0", "vwt": "1.0", …}`.
A minor rises when a feature gains something compatible (a field, a route),
so a client written for 1.0 works with 1.3,
and one that needs 1.2 asks for a minor of at least 2;
under `/api/v1` the major stays 1.
A name never changes meaning: a feature that goes is taken out of the map.
A server without the field, older than these names, has none of them.
The map is the running server's, not its build's:
without a store (`serve -addr`) the features that need one are left out.

| Name | What it says the server has | Needs the store | First in |
|------|-----------------------------|-----------------|----------|
| `event_ids` | the stream's ids, resuming by `Last-Event-ID` or `last_event_id`, `resync` | no | 0.13.0-alpha.1 |
| `idempotency` | `Idempotency-Key` on writes | yes | 0.8.0-alpha.1 |
| `commands` | `GET /api/v1/commands/{id}`, and command ids in answers | yes | 0.8.0-alpha.1 |
| `vwt` | virtual wall thermostats, `devices=`, `room_uid`, `temp_method` | no | the release after 0.15.0-alpha.1 |
| `program_etag` | the programme's tag, `If-Match`, `412`, `pending`, the config event's `changed` and `etag`, `GET …/devices/{rf}/program` | no | the release after 0.15.0-alpha.1 |
| `boost` | `{"mode": "boost"}` on the setpoint routes | no | the release after 0.15.0-alpha.1 |
| `proxy_virtual` | the Cube option `proxy_virtual` | yes | the release after 0.15.0-alpha.1 |
| `integration` | registered integrations (`/api/v1/integrations`) acting for their users (`X-MassimoHA-On-Behalf-Of`), `integration` and `acts_for` in `/api/v1/me` | yes | the release after 0.15.0-alpha.1 |
| `log_after` | `after=<id>` on the log's routes, the forward cursor; an `id` on every line of the log kept in memory; `409` for a cursor beyond the newest line | no | the release after 0.16.0-alpha.1 |

What a key may do is not here but in `GET /api/v1/me`.
`node_id` is the box's identifier from its store (`node.id`,
a UUID made at the first start, see docs/store.md);
it stays the same across restarts and updates.
`password_reveal` is the setting `auth.password_reveal` (below), `true`
without a store: whether the web UI's password fields offer their eye;
the page reads it here, before anyone signs in, so that the sign-in card
shows the eye or not from its first drawing.
`tls`, with TLS, describes the certificate served:
`{"source": "files", "names": ["massimo.example.net"], "not_after": "2027-02-01T00:00:00Z", "fingerprint": "SHA256:…", "self_signed": false}`;
`source` is `files` (the owner's files, re-read when they change),
`self-signed` (the certificate the server made, `self_signed` true)
or `acme` (Let's Encrypt),
the fingerprint is the SHA-256 of the certificate as browsers show it,
and `warning` is set while something needs the owner:
a certificate within two weeks of its end,
or an order from Let's Encrypt that has not succeeded yet.

### POST /api/v1/login, POST /api/v1/logout, DELETE /api/v1/me/token, GET /api/v1/me

`POST /api/v1/login` with `{"user": "anna", "password": "…"}`,
and an optional `label` that names the session under the user's tokens
("Firefox on the laptop"; the browser's `User-Agent` when absent),
answers a session:

```json
{
  "token": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
  "expires_at": "2026-12-06T10:42:00Z",
  "user": {"name": "anna", "display_name": "Anna", "role": "operator", "disabled": false, "has_password": true,
           "auth_version": 1, "created_at": "…", "updated_at": "…", "last_login_at": "…"},
  "role": {"key": "operator", "name": "", "disabled": false},
  "capabilities": ["backup", "configure", "discover", "lock", "read", "setpoint"]
}
```

The token is shown once; the client sends it as the bearer header.
A session lasts 30 days from its last use;
its end and its last use are written at most once an hour.
Case does not matter in the name.
A wrong name or password is `401` and counts against the address and the name
(ten in a minute lock either for five minutes, `429`);
an address signs in once at a time,
so a login while another from the same address is being checked
is `429` with `Retry-After: 1`, and counts as no failure;
a disabled user or a guest past its end date is `403`,
and so is every password sign-in while `auth.password_login` is off
(`GET and PUT /api/v1/settings`);
a user whose role is disabled logs in and is then refused every route.
The server hashes at most one password a core at once, four at the most,
with eight more waiting;
a login beyond them is `503` with `Retry-After`, and so is a user add or a
password change (`docs/store.md` section 6, "The lockout").
`409` on a server without users, where the first owner is created with
`POST /api/v1/users` or `massimo user add`,
and on one without a store (`serve -addr`), which takes the API token.
`login` names a sign-in by OPAQUE for the name, started from the same address,
that the client could not finish (below):
the password sign-in takes it over, so the attempt, counted at its start,
counts once, and the address it holds is this request's;
for another name or address it is `400`.
For a user still of `auth_version` 1, the answer then adds `upgrade`,
a proof good once for five minutes,
with which the client sets the user's own record (`PUT /api/v1/users/{name}`
with `record`, `ksf` and `proof`) without the password.

`POST /api/v1/logout` revokes the session that makes the call, `{"ok": true}`.
An API key answers `400`; a key is revoked under the user's tokens,
or by itself (below).

`DELETE /api/v1/me/token` revokes the token that makes the call,
whatever it is: a session, or an API key and the sessions made from it,
`{"ok": true, "id": 4}` with the token's id (AP MobileApp, D80-B).
It is one call for any token, without the user's name or the token's id:
the apps revoke a kept key with the key alone so,
and an integration may revoke its own key.
It is audited as `token.revoke`, as a revoke under the user's tokens is.
A caller without a token of a user,
on an open server or with the one token of a server without users,
is `400`, and so is an integration's call on a user's behalf
(`X-MassimoHA-On-Behalf-Of`), which would revoke the integration's key in that
user's name: the integration revokes its key without the header.
The settings file's token, `MASSIMO_API_TOKEN`, is `409`:
the next start would not add the owner's key back, its hash being taken,
so it is changed or removed in the settings file;
a revoke under the user's tokens is `409` too (below).

`GET /api/v1/me` says who the caller is:

```json
{
  "mode": "users",
  "user": {"name": "anna", "role": "operator", …},
  "role": {"key": "operator", "name": "", "disabled": false},
  "capabilities": ["backup", "configure", "discover", "lock", "read", "setpoint"],
  "token": {"id": 3, "kind": "session", "label": "Firefox on the laptop", "created_at": "…", "expires_at": "…", "last_used_at": "…", "last_used_from": "192.168.1.20"}
}
```

`mode` is how the server authenticates (Authentication above).
With users, `user`, `role`, `capabilities` (the effective set:
the role's minus the disabled capabilities, every one for `owner`,
none for a disabled role, and what those include, `restart` beside
`update` or `reboot` ("Authentication")) and `token`, the one presented;
outside users mode `user` and `role` are null and `capabilities` is every one.
`setup` is true on an open server for a caller on the local network,
the one that may create the first owner,
an integration's users beside it or not.
`kept_open` is true on an open server while the setting `auth.keep_open` is on,
and absent otherwise: the web UI then does not ask for the first owner.
`auth_version` 1 says the server hashes the password;
2, that the user signs in by OPAQUE (below) and the server never sees it.

### POST /api/v1/login/key

A session from an API key, for the apps' login by fingerprint
([app.md](app.md)): the phone keeps a key of the user's own, made by
`POST /api/v1/users/{name}/tokens`, and turns it into a session when the
user passes the fingerprint.
The bearer is the key; the optional body `{"label": "…"}` names the session
as `POST /api/v1/login`'s does (the `User-Agent` when absent),
and the answer is that route's: `token`, `expires_at`, `user`, `role`,
`capabilities`.

The session names its key: `key_id` in the token list and in `GET /api/v1/me`.
The key's next session ends the earlier one, so a phone holds one live
session per key,
and revoking the key (`DELETE /api/v1/users/{name}/tokens/{id}`) ends the
session at its next request.
A password set by someone else ends the sessions and leaves the keys,
as it does for every key.

A wrong key is `401` and counts toward the address's lockout,
as every failed token check does;
a revoked one is `401` and does not,
as no token that ended does (Authentication above);
a session as the bearer is `400`;
a disabled user, a guest past its end date, a disabled role
and a registered integration's key are `403`;
`409` on a server without users or without a store.
It sends no password, so it works while `auth.password_login` is off.
Its audit action is `login.key`.

### The sign-in by OPAQUE: POST /api/v1/login/start, POST /api/v1/login/finish

A user of `auth_version` 2 signs in without sending the password,
a hash of it or anything that could be replayed: OPAQUE (RFC 9807;
suite ristretto255-SHA512 with 3DH, argon2id run by the client,
the empty context, the public keys as identities),
as `@serenity-kit/opaque` and `github.com/bytemare/opaque` implement it.
Every message is base64url without padding.

`POST /api/v1/login/start` with `{"user": "anna", "ke1": "…"}`,
the client's KE1 (96 bytes), answers

```json
{"login": "…", "ke2": "…", "ksf": "m=19456,t=2,p=1", "v1": true, "expires_at": "…"}
```

KE2 (320 bytes), the argon2id parameters the client is to run for the record,
and the login to finish under, good for a minute.
A name without a record of version 2, unknown or of version 1,
is answered from a fake record of the same shape,
so the start does not say which names exist; its finish fails.
A start is a guess, since the client can tell from KE2 alone whether its
password was right:
it counts as a failed login against the address and the name at once,
and a finished sign-in clears both again,
the lockout its own start set included.
The address is held from the start to the finish, at most the minute:
a start or a `POST /api/v1/login` from it meanwhile is `429` with `Retry-After: 1`.
`v1` says whether the box still takes `POST /api/v1/login`,
for a client whose finish fails:
the web UI then signs in with the password and `login`,
and upgrades a user of version 1 at once.
A bad KE1 is `400`; the name's lockout `429`, as for the login.

`POST /api/v1/login/finish` with `{"login": "…", "ke3": "…"}`
and an optional `label` answers the session of `POST /api/v1/login`.
A wrong KE3 is `401` "wrong user or password" and counts nothing more;
an unknown login, one finished already or one past its minute, `401` too.
`"purpose": "proof"` at the start makes the finish answer
`{"proof": "…", "expires_at": "…"}` in place of a session:
a proof of the password, good once for five minutes,
for one's own password change.

### Sign-in through OpenID Connect: /api/v1/login/providers, /api/v1/login/oidc/…

A user may sign in through an OpenID Connect provider beside the password:
Nextcloud, Google, Microsoft, Authentik, Keycloak,
each found by its issuer's discovery document (docs/store.md section 6,
"Sign-in through OpenID Connect").
None of the four routes needs a token.

`GET /api/v1/login/providers` lists the enabled providers for the sign-in's buttons,
`[{"key": "cloud", "name": "Nextcloud"}]`, empty on a server without users.

`POST /api/v1/login/oidc/{key}` with `{"redirect": "https://massimo.example.com/"}`,
the page's own address, starts a sign-in:
the authorization code flow with PKCE (S256), a `state` and a `nonce`,
the server a confidential client.
It answers `{"url": …, "expires_at": …}`, the provider's page,
which the browser opens; the sign-in waits ten minutes.
The redirect URI is the page's address plus `api/v1/login/oidc/callback`,
which must be registered at the provider:
behind a reverse proxy the server cannot know its own address, the page does,
and a provider sends nobody to a redirect URI it does not know.
An address locked out is `429`, as for a password;
an unknown or disabled provider `404`,
a discovery document that cannot be read or does not do `502`.

`GET /api/v1/login/oidc/callback?code=…&state=…` is where the provider sends the browser.
The server trades the code at the token endpoint (`client_secret_basic`,
or `client_secret_post` where the provider names only that) and reads the `id_token`,
which comes straight from the provider over TLS,
so OpenID Connect Core 1.0, 3.1.3.7,
lets that stand in for checking its signature;
its `iss`, `aud` (and `azp`), `exp` and `nonce` are checked.
Where the discovery document names a `userinfo_endpoint`,
the server reads it with the access token and fills in what the `id_token` lacks:
the e-mail with `email_verified`, `name` and `preferred_username`;
Nextcloud's OIDC app sends them there only, since its 2.0.
An answer for another `sub` fails the sign-in;
one that cannot be read is logged, and the `id_token` goes on alone.
The user is found by the provider's subject (`sub`);
else by an e-mail an owner assigned to a user,
which the provider says it verified (`email_verified`),
and which binds the subject to the user from then on;
else, where the provider creates users, a new user for an e-mail of its domains,
with its role.
The browser goes back to the page with `#oidc=<code>`,
a one-time code good for a minute,
or with `#oidc-error=<reason>`:
`denied` (at the provider), `failed` (its answer did not do),
`refused` (no user here for the account),
`noemail` (none found, and the provider sent no e-mail),
`unverified` (none found, and the e-mail, which an assignment waits for
or a user would have been created for, is not marked verified),
`group` (the account in none of the groups the provider admits)
or `disabled` (the user is disabled or has expired).
A refusal but `denied` counts as a failed sign-in of the address,
and is a `login_failed` line of the log,
beside a warning with the provider, the account and the reason.
A state is good once.

`POST /api/v1/login/oidc/finish` with `{"code": …}` trades the code for the session,
as `POST /api/v1/login` answers it, with `"provider"` its key;
from the address the callback came from, once.
The session's token so never sits in a URL, a history or a proxy's log.

### Users: /api/v1/users and /api/v1/users/{name}

`GET` and `POST /api/v1/users`; `GET`, `PUT` and `DELETE /api/v1/users/{name}`.
The users of the store (docs/store.md section 6), each with one role.
The routes need `users`; a holder of `guests` reaches them for users whose role
is `guest`, regular ones,
or for a call through an integration that integration's own;
a holder of `integrations` for an integration's users, whatever their role but `owner`;
and a user reaches its own name (see below),
but never through an integration:
a call for an integration's user linked to a Massimo user
has none of the rights of that user's own name.
A call through an integration is one with its key
or with a key one of its users holds from before ("Acting for an integration's users",
below);
another integration's key reaches none of its users,
and a holder of `guests` that comes through no integration none of an integration's
(AP AccessEnds, D41-A4 and A5).
Every one answers `503` on a server without a store.

`GET /api/v1/users` lists the users by name, as `GET /api/v1/me` shows a user;
a holder of `users` or `passwords` sees every user,
any other caller the users it may change, as above, and itself
(AP AccessEnds, D45-A).
Each user carries `display`, how it is shown:
its `display_name`, else its `name`;
an integration's user ("Integrations", below) by the chain
`"<the integration's name> › <its display name>"`,
with `", as <login name>"` where it is linked to a Massimo user
(`"Home › Anna, as anna"`),
and `integration`, with the integration's `uuid`.
`display_name` stays as it is.

`POST /api/v1/users` creates one:

```json
{"name": "anna", "display_name": "Anna", "role": "operator", "password": "…"}
```

`name` is the login name, 1 to 64 of letters, digits, `.`, `_`, `-` and `@`,
kept in lower case; `role` is a role's key, `viewer` when absent;
`password` may be absent for a user that logs in with keys only,
or replaced by `record` and `ksf`, a password of version 2 (below);
`disabled` and, for a guest, `expires_at` (RFC 3339) as well.
A new `password` has 8 to 64 characters, counted in Unicode code points as sent,
not normalised;
outside that it is `400` with `password: at least 8 characters`
or `password: at most 64 characters`, and nothing is written.
Nor does it contain a name, whatever the case of either:
the user's login name and display name (in `PUT` the new one too),
each and each run of letters in them,
the server's name (the first label of its host's name),
the first label of the request's `Host` where it is no IP address and not `localhost`,
`massimo` and `maxcube`;
a name or a run of fewer than 4 letters is none.
Such a password is `400` with `password: contains the user's or the server's name`,
which names none of them, and nothing is written.
A `Host` that a reverse proxy rewrites, or a client sets,
only adds a word to the request's own check or takes one away:
it decides nothing else.
`massimo user` checks against the user's names and the host's name;
the web UI, which sends a `record`,
checks against the host it was opened by and, in the app,
the name the app gives the server, not the host's own name (docs/ui.md).
A `record` is not checked: the server never sees the password in it,
so a client that registers by OPAQUE holds its users to the rule itself.
A password set before the rule signs in as it is.
A `guests` holder creates guests only (`403` otherwise),
and a call through an integration none:
every user made here is a regular one.
The answer is `201` with the user; a name in use is `409`, a bad name,
an unknown role or an unknown field `400`,
too many password checks at once `503` with `Retry-After`, as for the login.
The name of a removed user can be used again,
for a new user in a row of its own,
which starts without the old one's preferences or history.

On a server without regular users (an integration's do not count),
`POST /api/v1/users` creates the first owner:
open to a caller on the local network (loopback, a private range, link-local),
`role` `owner` (absent, or that) and a password or a record required,
`403` from anywhere else; `massimo user add` on the box is the other way.
The first owner is created once:
where two requests come at the same moment,
the server counts the regular users again as it writes the owner,
and the second is `409` with `the first owner exists already: sign in`;
its registration (below) kept nothing, so nothing is left of it.
Its answer holds `open_setup` where anything set up before it still works
(Authentication):
`integrations` (`uuid`, `name`), `providers` (`key`, `name`, `auto_create`),
`roles` (`key`, `name`) and `capabilities_off`, each where it lists any.
From then on the server is in users mode (Authentication).

`GET /api/v1/users/{name}` is one user.
`PUT /api/v1/users/{name}` changes any of `display_name`, `role`, `disabled`,
`expires_at` (`null` clears it) and `password`;
absent fields stay, an unknown one is `400`.
A user changes its own `display_name` and, with `current_password`
or a `proof` from the sign-in by OPAQUE, its own `password` or `record`;
a user with keys only (`has_password` false) sets its first one without,
as its key could as well make another key;
the rest needs `users` (`guests` for a guest it reaches, above),
a password `passwords` too,
each for a user the caller outranks ("Authentication");
a holder of `integrations` changes an integration's user's `display_name`,
`disabled` and `expires_at`, and removes it,
whatever its role but `owner`,
and gives it a `role` it outranks within the integration's maximum,
any beyond it `403` (AP AccessEnds, D47-A, D48-A);
the answer is the user with its `may`, as the list gives it.
A refusal of an integration's rule says why:
`a call through an integration changes its own integration's users alone`,
`the user <name> is an integration's: its own integration, a holder of integrations or of users changes it`
to a holder of `guests` of no integration,
and `the role <key> lies beyond the maximum of its integration`.
A `password` is held to the rule of `POST`,
before anything of the request is written,
and for an integration's user it is `403` before that, whatever its length;
a password set by someone else ends the user's sessions;
an empty password removes it.

A password of version 2 is set in two steps.
`POST /api/v1/users/{name}/register` with `{"request": "…"}`,
the client's registration request (32 bytes),
answers `{"response": "…", "ksf": "m=19456,t=2,p=1"}`;
nothing is kept.
The client finishes with the password and sends the record (192 bytes) as `record`,
with that `ksf`, in `POST /api/v1/users` or `PUT /api/v1/users/{name}`,
in place of `password`.
The registration is open to whoever may set the name's password,
or add a user of it:
the first owner from the local network on a server without users.
The record replaces a hash of version 1 and the user is of version 2;
a `password` set later makes it version 1 again.
A record that does not decode, or a `ksf` below the cost of
`m=19456,t=2,p=1` or above 256 MiB, 16 passes or 16 lanes, is `400`.
The last enabled owner without an end date cannot be disabled, demoted
or given an end date (`409`): an owner with one does not count,
nor does an integration's user, whatever its role.

`DELETE /api/v1/users/{name}` removes a user: its sessions and keys are revoked,
its preferences removed,
and its row stays for the audit under the name `<name>-del-YYYYMMDD-HHNN`
of the removal in UTC (`-<id>` added where that is taken),
which frees the name; `{"ok": true, "name": "anna"}`.
The last enabled owner without an end date stays (`409`);
a user cannot remove itself.
An integration's own user, `<uuid>:`, which holds its key,
and its user `<uuid>:system` stay (`403`), for every caller:
`DELETE /api/v1/integrations/{uuid}` removes the integration itself
and keeps both, so that a request from the same installation can be attached to it
("Integrations", below).
The user that holds the settings file's key, `MASSIMO_API_TOKEN`,
stays too (`409`), as the key's revoke is refused (below, "Tokens"):
the removal would revoke the key with the user's tokens,
and the next start would not add it back, its hash being taken;
the key is changed or removed in the settings file first
(AP MobileApp v1.4, D89-A).

### Tokens: /api/v1/users/{name}/tokens

`GET` and `POST /api/v1/users/{name}/tokens`;
`DELETE /api/v1/users/{name}/tokens/{id}`.
A user's sessions and API keys, as `GET /api/v1/me` shows the one presented;
the same rules of access as the user.

`GET` lists the live ones, oldest first, without their secrets;
the key that is the settings file's `MASSIMO_API_TOKEN` carries `"from_settings": true`,
found by the hash of the file's token, not by its label
(AP MobileApp v1.4, D86-A).
`POST` with `{"label": "Loxone"}` creates an API key, which never expires,
and answers `201` with the key once;
an integration's user gets none (`403`, "Acting for an integration's users"):

```json
{"token": "…", "id": 4, "kind": "api", "label": "Loxone", "created_at": "…"}
```

The key is kept nowhere but in this answer:
the command of the request keeps the answer without `token`,
and a retry with its `Idempotency-Key` answers `201` without it too.

`DELETE …/tokens/{id}` revokes a session or a key by the `id` of the list;
a token of another user is `404`.
The settings file's key is `409`, as `DELETE /api/v1/me/token` answers it:
it is changed or removed in the settings file,
since the next start would not add it back, its hash being taken.
A session made from a key (`POST /api/v1/login/key`) carries the key's id
as `key_id`; revoking the key revokes that session with it.
A token revokes itself the same way, as its own bearer:
a key with `DELETE …/tokens/{id}` and its own id,
or any token with `DELETE /api/v1/me/token`, which needs neither the name nor the id;
a session ends itself with `POST /api/v1/logout`.

### Identities: /api/v1/users/{name}/identities

`GET` and `POST /api/v1/users/{name}/identities`;
`DELETE /api/v1/users/{name}/identities/{id}`.
A user's identities at the OpenID Connect providers
("Sign-in through OpenID Connect" above),
with the rules of access of the users routes:
`users`, `guests` for a guest's it reaches,
or `integrations` for an integration's user's, to list and remove alone
("Acting for an integration's users", "No way in of its own");
a user lists its own, removes its own and links its own,
but assigns and changes none.

`GET` lists them by provider:
`[{"id": 3, "provider": "cloud", "email": "anna@example.com", "subject": "anna",
"bound": true, "created_at": …, "bound_at": …, "last_used_at": …}]`;
`subject` is the user id at the provider, empty until bound.
`POST` with `{"provider": "cloud", "email": "anna@example.com"}` assigns the e-mail,
a pre-link that the first sign-in with it binds, `201` with the identity;
with `{"provider": "cloud", "subject": "anna"}` instead it assigns the user id there,
bound at once, which needs no e-mail from the provider
(Nextcloud's user id is the login name;
Google's a number, which its sign-in shows nowhere);
an e-mail that already waits at the provider,
or a user id another identity there has, is `409`,
an unknown provider `404`, a bad e-mail or both or neither of the two `400`.
`GET` answers too what the provider said of the account at the last sign-in or link:
`name`, `username` (`preferred_username`) and `groups`.
`PUT …/identities/{id}` with `{"email": …}` or `{"subject": …}` changes one no sign-in used,
the e-mail making it a waiting one, the user id a bound one;
one a sign-in used is `409`: it is removed and assigned again.
`DELETE` removes one: no sign-in finds the user by it any more.
A user removing its own last way in,
a bound identity where it has no password and no other bound one, is `409`.

`POST /api/v1/users/{name}/identities/link/{key}` with `{"redirect": …}`,
the user's own only (`403` for another's),
starts a sign-in at the provider as `POST /api/v1/login/oidc/{key}` does,
whose callback keeps the account as a pending link instead of signing in,
and sends the browser back with `#oidc-link-confirm=<code>`;
`#oidc-link-error=taken` where another user has the account,
`#oidc-link-error=group` where it is in none of the groups the provider admits,
`#oidc-link-error=failed` where the provider's answer did not do.
No session is made, and nothing counts against the address.

`GET /api/v1/users/{name}/identities/link/pending/{code}` shows the pending link
before anything is stored: `provider`, `provider_name`, the account's `subject`,
`email`, `name`, `username` and `groups`,
and `replaces`, the user's identities at the provider the link would replace.
`POST` on it stores the link, replacing those, and answers the identity;
`DELETE` drops it.
A pending link is the user's own (`403` for another's),
from the address that started it, for ten minutes and once;
an unknown, used or old code is `404`, and a restart drops it.

### Roles: /api/v1/roles and /api/v1/roles/{key}

`GET` and `POST /api/v1/roles`; `PUT` and `DELETE /api/v1/roles/{key}`;
`PUT /api/v1/roles/{key}/disabled`.
A role is a set of capabilities (Authentication).
The six default roles keep their keys; each can be renamed (`name`,
empty shows the key's translated default) and given another set,
except `owner`, which has every capability and cannot be changed,
disabled or removed.
The routes need `roles`, disabling `roles.disable`.

`GET /api/v1/roles` answers the roles, the default ones first,
and every capability with whether it is disabled, for a roles editor:

```json
{
  "roles": [
    {"key": "owner", "builtin": true, "disabled": false, "capabilities": ["read", "setpoint", …], "users": 1, "created_at": "…", "updated_at": "…"},
    {"key": "operator", "name": "", "builtin": true, "disabled": false, "capabilities": ["backup", "configure", "discover", "lock", "read", "setpoint"], "users": 2, …},
    {"key": "night", "name": "Night shift", "builtin": false, "disabled": false, "capabilities": ["read", "setpoint"], "users": 0, …}
  ],
  "capabilities": [{"key": "read", "disabled": false}, {"key": "pair", "disabled": true}, …]
}
```

`POST /api/v1/roles` with `{"key": "night", "name": "Night shift",
"capabilities": ["read", "setpoint"]}` creates a role:
the key is 1 to 32 of letters, digits, `_` and `-`,
starting with a letter, kept in lower case; `201` with the role,
`409` for a key in use, `400` for a bad key or an unknown capability.
`PUT /api/v1/roles/{key}` changes `name` and `capabilities` (the whole set);
`DELETE /api/v1/roles/{key}` removes a created role that no user has
(`409` while one does, `403` for a default role).
`PUT /api/v1/roles/{key}/disabled` with `{"disabled": true}` disables a role:
its users keep their login and are refused every route, with the reason,
until it is enabled again.

### Capabilities: /api/v1/capabilities

`GET /api/v1/capabilities`; `PUT /api/v1/capabilities/{key}`.
`GET` lists every capability in the order of the table under Authentication,
with whether it is disabled; `PUT …/{key}` with `{"disabled": true}` disables
one for every role but `owner`, whatever the roles' sets say
(for example `pair` while nobody should pair,
or `update` on a box the owner updates by hand), and `false` enables it again.
Both need `capabilities`, which only `owner` has by default.

### Integrations: /api/v1/integrations

An integration, Home Assistant first, is a program that acts for users of
its own: each of its calls can name the user it acts for and the level
the user has for that call, and Massimo allows only what every link of
that chain allows ("Acting for an integration's users", below).
On an open server, one without regular users (Authentication),
the key is not looked at:
every request from the local network is anyone's,
`GET /api/v1/me` says `"mode": "open"` without `integration`,
nothing is attributed to the integration or its users,
and a request from beyond the local network is `403`;
the integration's chain begins with the first owner (AP LooseEnds, D40-A).
An integration is registered by the owner, without a key typed anywhere:

1. The integration asks: `POST /api/v1/integrations/requests`, without a key,

   ```json
   {"kind": "homeassistant", "name": "Casa Leone", "instance_id": "<32 hex>",
    "url": "http://homeassistant.local:8123", "version": "2026.11.1"}
   ```

   `instance_id` is the installation's id (Home Assistant's `core.uuid`).
   The answer is `201` with `{"id": "<uuid>", "code": "K7Q2PX", "expires_at": "…"}`;
   the integration shows the code.
   A request waits ten minutes; at most five wait at once,
   an address asks at most ten times in ten minutes (`429`),
   and the setting `integrations.requests` off refuses requests (`403`).
2. The owner compares the codes and approves (capability `integrations`):
   `POST /api/v1/integrations/{uuid}/approve` with `{"max_caps": [...]}`,
   the integration's maximum, within the role `integration` and all of it
   by default; `"attach": "<uuid>"` makes the request an integration of the
   same kind that was registered before, with its users, links and maximum,
   as after a key lost or revoked.
   `POST …/refuse` refuses it.
3. The integration collects its key:
   `GET /api/v1/integrations/requests/{id}?code=K7Q2PX`,
   `202` while the request waits,
   `200` with `{"key": "…", "integration": {…}}` once approved,
   `410` once refused, expired, or its key collected and used.
   An answer lost on the way is not lost:
   the same call answers a new key, the one before revoked,
   as long as that key was never used and ten minutes since the approval
   have not passed.
   Until then the approved integration carries `request_expires`,
   the end of those ten minutes; once they are over, Home Assistant
   registers again and the owner approves the new request.

The key belongs to the integration's own user, named `<uuid>:`
after the integration's `uuid`, with the role `integration`;
the user `<uuid>:system` stands for its calls that no user started.
An integration's `name` is its own user's `display_name`,
the name the owner gives it,
and `reported_name` the one the integration reports,
when it asks and when a request is attached to it;
`name` follows `reported_name` as long as it is the name reported before,
so that a name the owner gave stays.
The chain each of its users is shown by begins with `name`
("Acting for an integration's users", below).
`GET /api/v1/integrations` lists the integrations and the requests
waiting, a request with its code and the address it came from;
`PUT /api/v1/integrations/{uuid}` changes `name`, `max_caps` and `state`
(`active`, `disabled`):

```json
{"name": "Home"}
```

answers the integration with `"name": "Home"` and `reported_name` as it was;
`name` is 1 to 64 bytes, its spaces at either end dropped;
`DELETE` revokes it: its key revoked and the integration disabled,
its users, links and maximum kept for a request from the same installation.
A call that came with another installation id than the accepted one is
refused and the id recorded as `pending_instance`, which
`POST …/accept-instance` accepts.

**On the LAN.**
A server that announces itself (README.md, "Server") also looks for
Home Assistant's zeroconf records, `_home-assistant._tcp`,
at the start and every 30 minutes.
`GET /api/v1/integrations/found` answers what the last look saw,
`?refresh=1` looking now:
`{"found": [...], "at": "…", "looks": true}`,
each installation with `kind`, `name`, `instance_id`, `location`, `url`,
`version`, `address`, and `integration`, the uuid of the integration or
the request of that installation id, where there is one.
A server that does not look answers `"looks": false` and why, in
`announce`: the setting `announce` as `GET /api/v1/settings` shows it
(`value`, the stored one or `auto`; `in_effect`, `off`; `fixed_by`),
and `why`, the first that holds of
`fixed` (`-announce` or `MASSIMO_ANNOUNCE` decides, named in `fixed_by`),
`restart` (the stored setting announces from the next start: `on`, or
`auto` where the server does not listen on the loopback),
`off` (the setting is `off`) and
`loopback` (the setting is `auto` and the server listens on a loopback
address), so that a holder of `integrations` alone reads it too.
An integration or request whose installation id the last look saw is
marked `"seen": true`: a request's id confirmed by the LAN.
Home Assistant finds the server the same way:
the TXT record of `_massimo._tcp` carries `url`, where the API is
reached (README.md, "Server"), `loc`, the installation's name, `reg=1` while requests are
taken, and `api=1`, `GET /healthz` listing the features.

### Acting for an integration's users

A registered integration's key with the header

```
X-MassimoHA-On-Behalf-Of: id=<the user's id, 32 hex>; level=control; name*=UTF-8''J%C3%BCrg; instance=<installation id, 32 hex>
```

acts for one of the integration's users,
on a server with regular users: on an open server the header is not read
("Integrations", above).

- **The header's parameters:**
  - `id` is the user's id at the integration (Home Assistant's `user.id`),
    or `system` for a call no user started (an automation);
  - `level` is the user's level for this call: `view`, `control` or `admin`;
  - `instance` is the installation id;
  - `name` (quoted or not) or `name*` (RFC 8187, for any character)
    is the user's name there,
    shown in the users list and updated when it changes;
  - `system_generated`, alone,
    marks one of the integration's own system users
    (Home Assistant's Supervisor, Cloud, …).
- **The user** is made at its first call,
  with the role `integration` and no password,
  named `<the integration's uuid>:<the id>`,
  its `display_name` its name there, which follows the name
  while it is the one reported before.
  It is shown by the chain `"<the integration's name> › <its display name>"`
  (`"Home › Anna"`), and where it is linked to a Massimo user,
  `", as <that user's login name>"` after it (`"Home › Anna, as anna"`):
  the page, `massimo user list` and the server's own log lines alike.
  Several of the integration's users may be linked to one Massimo user.
- **What the call may do** is what every link of the chain allows:
  1. the integration user's role, `integration` at first,
     narrowed by the owner where wanted;
  2. the role `integration`, the ceiling of every call through an integration;
  3. the Massimo user it is linked to, where it is;
  4. the integration's maximum, `max_caps`;
  5. the level: `view` reads, `control` also sets, locks and configures,
     and `admin` adds nothing of its own.

  The include is taken on what they leave ("Authentication"):
  a call left with `update` or `reboot` passes `restart`'s checks too.
- **Its role:** of the roles the caller may give,
  the users routes offer such a user (`may.roles`) only those
  whose capabilities lie within the role `integration` and the integration's maximum,
  and `integration` itself,
  which a user moved off it is given back, by the rank rule as any role;
  never `owner`, which holds every capability.
  A role beyond them is taken all the same from a holder of `users`,
  and acts within them,
  and refused, `403`, to a holder of `integrations` (AP AccessEnds, D48-A);
  one stored so, or left beyond a maximum lowered later,
  is offered no more once another is given.
- **Who manages them:** an owner;
  a holder of `users`, by the rank rule;
  a holder of `integrations`, every integration's user
  but one stored as `owner`, which stays an owner's alone,
  giving a role it outranks within the maximum;
  and the integration's own calls, where their chain holds `guests`,
  its users stored as `guest`, by the rank rule.
  No other integration's key changes them,
  nor a holder of `guests` that comes through no integration,
  and no call through an integration changes or makes a regular user,
  nor has the rights of the own name of the user it is linked to:
  no key, password, record, identity, link, name or field of that user's
  (AP AccessEnds, D41-A4 and A5);
  that user changes them signed in itself.
  Disabling one stops its calls, a linked one's too;
  removing one does not, where it is linked: disable it, or unlink it.
  Its own user and its system user are not removed ("Users", above).
- **Without the header** the key acts as the integration itself:
  its role and its maximum (an integration's own polls and stream),
  within the role `integration` as every call through it.
- **No way in of its own:** an integration's user acts through its integration
  alone, whatever role the owner gives it.
  The users' routes refuse it a key (`POST /api/v1/users/{name}/tokens`),
  a password, plain or by a record (`PUT /api/v1/users/{name}`,
  `POST /api/v1/users/{name}/register`),
  and an identity at a provider (`POST` and `PUT /api/v1/users/{name}/identities`,
  the link's start), with `403`, before any other change of the request;
  no sign-in makes it a session, the integration's key at
  `POST /api/v1/login/key` included.
  An empty `password` that removes one is taken.
  A key it was given before this version keeps working
  while its integration is active (else `403` with `integration_disabled`),
  within the role `integration` and the integration's maximum,
  as every call of such a user, whatever its role;
  `GET /api/v1/me` with it says what is left.
  A password or a provider's identity it holds signs in nowhere.
- **The audit and the log** name the user the call acted as,
  `effective_user`, and the integration's user, `real_user`
  (GET /api/v1/audit, "Who did it");
  the audit's `detail` adds `"via"`, the integration's uuid,
  and `"for"`, the integration's user (id, name, level).

A refusal says which link refused, in `code`:

| Status | `code` | Why |
|--------|--------|-----|
| 400 | `bad_header` | the header says something it may not |
| 403 | `not_integration` | the header with a key that is no registered integration's |
| 403 | `integration_disabled` | the integration is disabled or revoked |
| 403 | `instance_pending` | another installation id than the accepted one; recorded as `pending_instance` for the owner to accept |
| 403 | `user_disabled` | the integration's user, its linked user or one of their roles is disabled |
| 403 | `forbidden` | a capability the chain did not leave |

`GET /api/v1/me` answers, for an integration's key, `integration`
(`uuid`, `name`, `reported_name`, `state`, `instance_id`, null for any other key),
with the header `acts_for` (`id`, `name`, `level`, `linked`,
`system_generated`), and `capabilities`, what the chain leaves.
So an integration knows before naming a user whether its key is a
registered integration's, and a key typed by hand keeps working without
the header.

**Links.**
`GET /api/v1/integrations/{uuid}` answers the integration with `users`,
its users, the system user first:
`id`, `name`, `display` (its chain), the `user` its calls act as,
its `own_user`, `linked`, `system_generated`, `last_level`, `last_used_at`,
and for one not linked a `proposal`, a Massimo user whose name or display
name is its name at the integration.
The owner confirms a link (capability `integrations`):
`PUT /api/v1/integrations/{uuid}/users/{id}` with `{"user": "anna"}`;
its calls then act as `anna`, within `anna`'s role (the chain's third
link), and `{"user": null}` unlinks it.
Several of an integration's users may be linked to one user.
The system user stays its own, a user of an integration is no user to
link to (`400`), and the caller must outrank the user linked to (`403`).
`GET /api/v1/users` marks an integration's user with `integration`
(`uuid`, and for one of its users `id`, `name`, `linked_to`)
and lists under `links` the integrations' users linked to a user
(`integration`, its uuid, `id`, `name` and `display`, its chain).

### Providers: /api/v1/providers

`GET` and `POST /api/v1/providers`; `PUT` and `DELETE /api/v1/providers/{key}`.
The OpenID Connect providers users sign in through
("Sign-in through OpenID Connect" above).
They need `auth`, which only `owner` has by default.

```json
{"key": "cloud", "name": "Nextcloud", "issuer": "https://cloud.example.com",
 "client_id": "massimo", "client_secret": "…", "scopes": "openid email profile",
 "auto_create": false, "auto_role": "viewer", "auto_domains": [],
 "groups_claim": "groups", "allowed_groups": ["family"], "enabled": true}
```

`POST` adds one: `key`, `issuer` and `client_id` are needed.
The key is that of a role (lower-case letters, digits, `_` and `-`);
the issuer an `https` URL (`http` only on loopback),
whose discovery document is under `/.well-known/openid-configuration`;
the scopes include `openid`.
An enabled provider is saved,
by `POST` or by a `PUT` that changes its issuer or enables it,
only where that document is reached and names the issuer:
one that differs only by a trailing slash is kept in the document's form,
which the sign-in compares exactly;
any other answer is `400`, saying what the document names or why it was not reached.
A disabled provider is saved unchecked.
`auto_role`, the role of the users a sign-in creates where `auto_create` is true,
may not be `owner` nor hold an administrative capability (`guests`,
`users`, `passwords`, `roles`, `roles.disable`, `capabilities`, `cubes`,
`cubes.reboot`, `settings`, `update`, `restart`, `reboot`, `audit`, `auth`);
`viewer` where absent.
`auto_domains` limits it to e-mails of those domains, any where empty.
`allowed_groups` are the groups the provider admits, read from the claim `groups_claim`
(`groups` where empty) of the `id_token` or the userinfo,
a list of strings or one string:
an account in none of them neither signs in (`#oidc-error=group`)
nor links (`#oidc-link-error=group`); any where empty.
`PUT` changes the fields given; the key stays,
`client_secret` left out stays and `""` clears it.
The answers never hold the secret:
`has_secret` says whether there is one,
`secret_lost` that one is set and sealed and the key of `opaque.key` no longer opens it,
so that it is to be entered again,
and `callback` is the redirect URI's path under the page's address,
`api/v1/login/oidc/callback`, to register at the provider.
`DELETE` removes a provider and every identity at it.
A key in use is `409`, an unknown one `404`.

### Preferences: /api/v1/me/preferences

`GET /api/v1/me/preferences`; `GET`,
`PUT` and `DELETE /api/v1/me/preferences/{key}`.

The signed-in user's preferences (docs/store.md section 9 (c)):
a JSON document per key, which the web UI uses so that its settings follow
the user across browsers.
The page keeps these keys, and `notify` (below):

```json
{
  "ui": {"poll": 10, "expert": false, "clock": "browser", "lang": "auto", "theme": "system", "scheme": "custom:Mine"},
  "schemes": {"format": "massimo-schemes/1", "schemes": [{"format": "massimo-scheme/1", "name": "Mine", "light": {…}, "dark": {…}}]},
  "log.seen": {"all": 4711, "lines": [4720]}
}
```

`log.seen` is the user's marks of the log's alerts (`GET /api/v1/log/alerts`):
`all` marks every line up to that id seen, **Mark all as seen**,
and `lines` up to 500 lines marked one by one, **Mark seen**;
a document of another form is `400`.

`GET /api/v1/me/preferences` is every key the user has, `{}` for none;
`GET …/{key}` the document, `404` when the key is not set.
`PUT …/{key}` with any JSON document of at most 64 KiB as the body replaces
the key's and answers how it was stored:

```json
{"key": "ui", "value": {"poll": 10, …}, "updated_at": "2026-11-06T18:46:40.233Z", "version": 3}
```

`version` counts the writes of the key;
between two browsers the later write wins.
`DELETE …/{key}` removes it,
`{"ok": true, "key": "ui"}` also when it was not set.
A key is 1 to 32 of lower-case letters, digits, `_`, `.` and `-`,
starting with a letter; a bad key or a body that is not JSON is `400`,
a body over 64 KiB `413`.

Reading needs a signed-in user only; writing needs `preferences`,
which manager, operator and viewer have and guest and integration not:
a guest's settings stay in its browser.
Without a signed-in user (an open server,
or the API token) the routes answer `409`, and without a store `503`;
the page then keeps its settings in the browser as before.
A write is an audit row (`preference.set`, `preference.delete`,
the target `preference:<key>`) but no command, since nothing goes to a Cube,
and takes no `Idempotency-Key`.

### Notifications: /api/v1/me/push, GET /api/v1/push/vapid

`GET /api/v1/push/vapid`; `GET` and `POST /api/v1/me/push`;
`DELETE /api/v1/me/push/{id}`.

The server decides from the Cubes' events what to tell each user with a push subscription,
by the rules the user chose in the preference `notify`.
A notification is data, no text: its kind and values,
which the page or the app renders in the user's language.
Each one is written to the server's log as `notification`,
with the user, the kind, the Cube, the device, its S/N and its room,
pushed to the user's subscriptions by Web Push (see Delivery below),
and sent on the user's own event streams as a `notification` event.

**The preference `notify`**, through the preference routes above,
a rule per kind; a kind left out, and a delay or threshold left out,
keep the default:

```json
{"cube_offline": {"on": true, "after_min": 10}, "window_open": {"on": true, "after_min": 30},
 "frost": {"on": true, "below_c": 8}, "battery_low": {"on": true}, "battery_weak": {"on": true}, "device_error": {"on": true},
 "device_stale": {"on": false}, "paired": {"on": false}, "cube_found": {"on": false}, "update": {"on": false},
 "server_backup_failed": {"on": true}}
```

| Kind | Told when | Default |
|---|---|---|
| `cube_offline` | the Cube disconnected and is not back within `after_min` | on, 10 min |
| `cube_online` | the Cube is back after a `cube_offline` was told | on |
| `window_open` | a window opened and is still open after `after_min` | on, 30 min |
| `battery_low` | a device's battery flag held for 15 minutes: change the batteries (its `battery` `low`) | on |
| `battery_weak` | a device's battery flag seen, as under the valve motor's load, and still weak after 15 minutes (its `battery` `weak`); once until a long valve move goes by without it | on |
| `device_error` | a device reports a communication error (its state's `error`) or a link error (`link_error`), when each of them is set, a second while the first stays set as well | on |
| `frost` | a device's measured temperature fell under `below_c`; again once it rose half a degree above | on, 8 °C |
| `device_stale` | a thermostat went past the stale threshold (`stale_after`, `stale_after_wall`) | off |
| `paired` | a device joined by pairing | off |
| `cube_found` | discovery found a Cube that is not configured | off |
| `update` | the server's own check, after its start and every six hours, or `GET /api/v1/update` saw a newer release; once per release | off |
| `server_backup_failed` | a backup of the server, or its copy to a target, still failed after its three retries ("Server backups"); told only to a user who holds `backup.retry` | on |

`after_min` is 1 to 1440, `below_c` −10 to 20.
A `PUT` of a document the rules cannot read (an unknown kind,
a value out of bounds, `after_min` on a kind without a delay) is `400`.
One kind is told once per device per half hour,
a `device_error` once per flag,
so that the other flag's start is told while the first's half hour runs;
a delay ends when its condition does.
The rules keep their state in memory:
a restart costs at most one repeated notification.
A user without a subscription costs nothing.

**The notification:**

```json
{"kind": "window_open", "cube": "KEQ0000000", "device": "0a0102", "serial": "MEQ0000001",
 "device_name": "Window", "room": 3, "room_name": "Bath", "since": "2026-11-09T06:12:00Z",
 "values": {"minutes": 30}, "time": "2026-11-09T06:42:00Z", "url": "/"}
```

`since` is when the condition began,
for `cube_offline`, `cube_online` and `window_open`; `values` are the kind's:
`minutes` (the delay), `down_minutes` (`cube_online`),
`error` and `link_error` (`device_error`), `actual_c` and `below_c` (`frost`),
`type` (`paired`), `address` (`cube_found`, whose `cube` is the one found),
`version` (`update`), `step`, `target` and `backup` (`server_backup_failed`:
`make` or `copy`, the target's name, the backup's uuid).
A `device_error` names the flags set then:
`error` the communication error, with `failed_radiogram`,
the type of the radiogram that failed, where the device gives it
(`{"code": 64, "name": "ControlMode"}`, as a device's state gives it),
and `link_error` the link error;
the web UI's text names the one set, or both, and the radiogram:

```json
{"kind": "device_error", "cube": "KEQ0000000", "device": "0a0103", "serial": "MEQ0000003",
 "device_name": "Radiator", "room": 3, "room_name": "Bath",
 "values": {"error": true, "link_error": false, "failed_radiogram": {"code": 64, "name": "ControlMode"}},
 "time": "2026-11-09T07:00:00Z", "url": "/"}
```

**Delivery.**
A notification goes to every subscription of its user that may be sent to,
by Web Push (RFC 8030), which UnifiedPush takes as well:
the notification above as JSON, encrypted for the subscription (RFC 8291,
`aes128gcm`, one record), so that the push service reads none of it,
with a VAPID token (RFC 8292) signed by the server's key for the push service's origin,
valid for 12 hours, its subject the box's node, `urn:uuid:<node_id>`.
The request asks the push service to keep it a day (`TTL: 86400`),
says `Urgency: high` for `frost` and `cube_offline`, `normal` for the rest,
and `Topic: <kind>-<device or Cube>`,
so that a newer one replaces one not delivered yet.
A `2xx` is delivered and sets the subscription's `last_ok_at`;
`404` and `410` remove the subscription;
a `429` or a `5xx` is tried again after 5, 15 and 40 seconds;
any other answer, or none, counts a failure,
and a subscription failing for seven days,
since its last delivery or its creation, is removed.
A removal is a line in the server's log, which names the push service's host,
never the endpoint's path.
The server follows no redirect:
an endpoint is not a way to send its requests elsewhere.
The user's own event streams get the same notification at once,
as a `notification` event (see GET /api/v1/cubes/{sn}/events),
so that an open page shows it without a push;
another user's streams never do.
An app that embeds the server can take every notification itself as well
(`internal/serve`, `Config.Deliverers`).

**The subscription.**
`GET /api/v1/push/vapid` is the server's VAPID public key (RFC 8292),
`{"key": "<base64url>"}`, which the page gives `pushManager.subscribe`
as `applicationServerKey`;
made at the first request and kept sealed in the store (docs/store.md section 5),
for `read`.
Where the store's key no longer opens it, a new pair is made,
and the page, finding its subscription made for another key, subscribes again.
`POST /api/v1/me/push` takes the browser's `PushSubscription.toJSON()` and a label:

```json
{"endpoint": "https://push.example.net/send/…", "expirationTime": null,
 "keys": {"p256dh": "<base64url>", "auth": "<base64url>"}, "label": "Firefox on the phone"}
```

and answers `201` with the row, or `200` when the endpoint was known,
whose row then belongs to the caller:

```json
{"id": 4, "endpoint": "https://push.example.net/send/…", "label": "Firefox on the phone",
 "created_at": "2026-11-09T06:00:00Z", "failures": 0, "current": true}
```

The endpoint is the push service's `http` or `https` URL
(a UnifiedPush distributor on the LAN may be plain HTTP),
`p256dh` an uncompressed P-256 point and `auth` 16 bytes, base64url;
a label left out is the request's `User-Agent`, at most 100 characters.
A subscription belongs to the user and to the session or API key that made it:
revoking that, or removing the user, removes the subscription,
and one whose session ended or whose user is disabled is not sent to.
`GET /api/v1/me/push` lists the caller's, without their keys,
`current` marking the one made with the caller's session or key;
`DELETE …/{id}` removes one of them, `404` for another's.
The three need `preferences`, which manager, operator and viewer have:
a guest and an integration have no notifications.
Without a signed-in user they answer `409`, without a store `503`.
`POST` and `DELETE` are audit rows (`push.subscribe`, `push.unsubscribe`,
the target `push:<id>`), no command.

### GET /api/v1/audit?before=&limit=&user=

The audit (docs/store.md section 5): every request that changed something,
or tried to, newest first.
Needs `audit`, which only `owner` has by default; `503` without a store.

```json
[
  {
    "id": 42,
    "time": "2026-11-06T10:42:00.123Z",
    "user": "anna",
    "token_id": 3,
    "remote": "192.168.1.20",
    "action": "setpoint",
    "cube": "KEQ0000000",
    "target": "room:6",
    "detail": {"method": "PUT", "path": "/api/v1/cubes/KEQ0000000/rooms/6/setpoint", "status": 200, "ms": 812, "bytes": 15, "body": {"mode": "auto"}},
    "result": "ok"
  },
  {
    "id": 41,
    "time": "…",
    "remote": "192.168.1.20",
    "action": "login.failed",
    "target": "user:anna",
    "detail": {"method": "POST", "path": "/api/v1/login", "status": 401, "ms": 310, "bytes": 38, "body": {"user": "anna", "password": "…"}, "error": "wrong user or password"},
    "result": "refused"
  }
]
```

Every route but the `GET`s and the stream ticket leaves a row:
`action` names what was asked (`setpoint`, `device.lock`, `device.update`,
`room.rename`, `room.config`, `room.program`, `room.delete`, `update.answer`,
`pairing.start`, `restore.start`, `cube.add`, `settings.update`, `login`,
`login.failed`, `logout`, `user.add`, `token.add`, `role.update`,
`capability.disable`, …), `cube` the Cube of the route, `target` the device,
room, user, role, token or Cube concerned, `user` and `token_id` who asked
(absent without authentication, for a failed login,
and for `massimo user` on the box, whose `remote` is `cli`),
`effective_user` and `real_user` the users of the row
("Who did it", below),
and `result` how it went: `ok` for a 2xx, `refused` for a `401`,
`403` or `429`, `failed` otherwise.
`detail` keeps the method, the path, the status, the milliseconds,
the body's size and, when it is JSON of at most 4 KB,
the body with `password`, `current_password` and `token` replaced by `…`,
and the error of a failed answer.
A request refused for a known user (a capability it lacks,
a disabled user or role) is a row with who was refused, without its body.
Requests refused before anyone is known, without a token,
with a wrong one or from a locked-out address,
are one row per address and minute, written when the minute ends,
with their bodies never read:
the first request's action, target, method, path and status,
and in `detail` the `count` of requests within the minute,
the count of each status in `statuses`, and the time of the `last`:

```json
{"id": 43, "time": "…", "remote": "192.0.2.10", "action": "setpoint", "cube": "KEQ0000000", "target": "room:6",
 "detail": {"method": "PUT", "path": "/api/v1/cubes/KEQ0000000/rooms/6/setpoint", "status": 401, "count": 20, "statuses": {"401": 10, "429": 10}, "last": "…"},
 "result": "refused"}
```

Nothing in a row is a secret.

`before`
(RFC 3339) pages: the rows before that time, a time only, unlike the log's;
`limit` is 100 by default and at most 1000; `user` filters by login name,
the rows made with that user's rights or by it.
A row's `time` is when its request came,
and the row is written once the request is answered,
a row of refused requests once its minute has ended:
after rows of requests that came later, so that an `id` is no measure of time.
A page is therefore ordered by `time`, then by `id`, both newest first,
so that the next page, `before` the `time` of a page's last row,
goes on where it ended.
Rows of the same millisecond that a page's end divides stay divided:
the next page, before that time, has none of them.
A row written while a client pages, with a time on a page it has already read,
is on none of the later pages; a new read from the newest finds it.
The rows are append-only,
and kept for `audit.keep_days` (`GET`/`PUT /api/v1/settings`).

**Who did it.**
A row of the audit or the log names two users,
each as it is named now:
`effective_user`, whose rights the request had,
and `real_user`, who made it.
They are the same user but for a call of an integration's user
linked to a Massimo user ("Integrations", below):
its rights are that user's, `effective_user`,
and `real_user` is the integration's user.
Each carries `name`, `display_name`, `integration` (an integration's user's)
and `display`, how it is shown;
`real_user`'s says `", as <the effective user's login name>"`
where the two differ:

```json
"user": "anna",
"effective_user": {"name": "anna", "display_name": "Anna", "display": "Anna"},
"real_user": {"name": "<uuid>:<id>", "display_name": "Anna", "integration": "<uuid>", "display": "Home › Anna, as anna"}
```

`user`, the effective user's login name, stays as before.

### Commands and Idempotency-Key: GET /api/v1/commands/{id}

Every request that changes something is a **command** in the store
(docs/store.md section 5; docs/server.md, "What version 1 prepares", item 7):
a row with who asked, the Cube,
the action and the target as the audit names them, the request, and a state;
and every protocol command the request puts on the Cube's queue is a **step** beneath it,
written by the queue as it goes.
The audit row of the request names the command in `command_id`.
A line a client of the LAN proxy passes on to the Cube is a command as well,
with no user and no audit row: its `action` is `proxy.radio` (an `s:`),
`proxy.delete` (a `t:`), `proxy.metadata` (an `m:`), … and its `request` the
client's address (`remote`) and the line's letter; it is `dropped` where the
Cube discarded it.
The log's lines of a command name it (`GET …/log?command=`).

The states are those of docs/design.md section 5:
`queued` while the request runs and nothing has gone to the Cube,
`sent` once a step went to the Cube, or the request was answered `202`
and its run goes on (a restore),
`acknowledged` when the request was answered `2xx` (a step:
the Cube accepted it),
`failed` for a `5xx` (a step: the send failed),
`dropped` for a `4xx` or `503`, nothing sent (a step: superseded by a newer
command with its key, discarded by the Cube, or the server stopped);
at a start of the server every command still `queued` or `sent` is `dropped`,
since the queue it was in is gone.
A restore's command is `sent` with its `202` and takes `acknowledged` or
`failed` from its run, with the run's id, state and counts as its result,
and the `202`'s answer under `answer`, which a replay of its key answers.
An update's `202` hands the work to the helper,
which reports in `GET /api/v1/update`:
its command is `acknowledged` with the answer.

`GET /api/v1/commands/{id}` is one command with its steps,
by its id or by its number, as the log's `command` and the audit's
`command_id` name it, and for a caller with `audit` with its audit row
under `audit`;
the command names a device by its serial number, as the audit does,
and a step by its radio address, the radio command's:

```json
{
  "id": "01a1b0c2-5a3e-7b1c-9d4e-0f1a2b3c4d5e",
  "action": "device.update",
  "target": "device:MEQ0000106",
  "cube": "KEQ0000000",
  "user": "anna",
  "state": "acknowledged",
  "status": 200,
  "request": {"method": "PUT", "path": "/api/v1/cubes/KEQ0000000/devices/0a010d", "query": "", "hash": "…"},
  "result": {"ok": true, "budget": {…}},
  "created_at": "…", "updated_at": "…", "finished_at": "…",
  "steps": [
    {"id": "…", "target": "device:0a010d", "state": "acknowledged", "request": {"key": "", "priority": 1, "lines": 2, "radio": 2, "wakeup": true, "room": 6, "reconnect": false}, "result": {"discarded": false, "budget": {…}}, "created_at": "…", "finished_at": "…"},
    {"id": "…", "target": "device:0a010d", "state": "acknowledged", "request": {…}, "result": {…}, "created_at": "…", "finished_at": "…"},
    {"id": "…", "state": "acknowledged", "request": {"key": "metadata", "priority": 2, "lines": 1, "radio": 0, …}, "result": {…}, "created_at": "…", "finished_at": "…"}
  ]
}
```

A step's `result` also gives `answers`, the Cube's answer to each of its
lines in order up to the first failure (`S:` for a radio line, `A:` for
`t:`), and `discarded_line`, the line the Cube discarded, where it
discarded one.

Needs `read`; a user without `audit` sees its own commands only,
and another's is `404`.

**`Idempotency-Key`**:
a write route takes the header with a key of the client's choice,
at most 128 characters, kept per user for 24 hours.
A retry with the same key answers what the first request answered,
status and body, with the header `Idempotent-Replayed: true`,
and sends nothing to the Cube; the same key with another request (the method,
the path, the query or the body differ) is `422`,
and one whose request is still running `409`.
A key whose request was running when the server stopped is `503`
with the row's `{"error": "server restarted"}`, since it got no answer.
A page that lost the connection while a setpoint was on its way retries
with the key and learns the outcome without a second radio command.
A command keeps its answer without `password`, `current_password` and `token`,
so the replay of a new API key's creation answers without the key.
The login, the logout, the stream ticket and the preferences take no key
and are no commands.

Finished commands are kept for `commands.keep_days` (`GET`/`PUT /api/v1/settings`),
0 for ever, the default;
the key is forgotten after 24 hours whatever the setting.

### GET /api/v1/cubes

Every configured Cube,
then every Cube discovery found on the LAN that is not configured:

```json
[
  {
    "serial": "KEQ0000200",
    "name": "Ground floor",
    "address": "192.168.1.9",
    "configured": true,
    "source": "env",
    "file_differs": [
      {"field": "name", "variable": "MASSIMO_CUBE_KEQ0000200_NAME", "file": "Downstairs", "store": "Ground floor"}
    ],
    "backups": 10,
    "mac": "00:1a:22:0a:0b:0c",
    "last_seen": "2026-11-04T15:00:02+01:00",
    "connected": true,
    "link": {"state": "connected"},
    "firmware": "1.1.3",
    "rf_address": "0a0104",
    "budget": {"duty_cycle_pct": 4, "free_memory_slots": 50, "updated_at": "2026-11-04T15:28:11+01:00"},
    "pending_radiograms": 0,
    "updated": "2026-11-04T15:28:11+01:00",
    "rooms": 5,
    "devices": 16,
    "queued": 0,
    "cube_time": "2026-11-04 15:28",
    "cube_time_at": "2026-11-04T15:28:09+01:00"
  },
  {
    "serial": "KEQ0000000",
    "kind": "cube",
    "configured": false,
    "address": "192.168.1.23",
    "mac": "00:1a:22:01:02:03",
    "rf_address": "0a0b0c",
    "firmware": "1.1.3",
    "dhcp": true,
    "gateway": "192.168.1.1",
    "netmask": "255.255.255.0",
    "dns": ["192.168.1.1"],
    "proxies": ["192.168.1.40"],
    "host": {"name": "box-a", "url": "https://box-a.example:8443"},
    "backups": 2,
    "found_at": "2026-11-04T14:30:01+01:00",
    "last_seen": "2026-11-04T15:00:02+01:00"
  }
]
```

`configured` tells the two apart.
Both have `backups`, the number of backups the store keeps of the Cube's row
(0 without one, and without a store),
which `GET …/backups` lists also for a found Cube (see Backups);
the answer of a discovery has no such field.
A configured Cube has the fields below;
`mac`, `last_seen` (when it last answered discovery) and `found_address` only once
discovery has seen it:
`found_address` is the host it answered from, which differs from `address`
where the server connects through a proxy or to a fixed address the Cube has left.
`found_elsewhere` says the latter:
a Cube with a fixed address (`follow_discovery` false) that answered the latest
discovery at another address, and neither it nor a proxy at its own,
has `address` (where it answers, with the configured address's port,
ready for a `PUT` of `address`), `configured` and `since`;
it is absent otherwise, and goes at once with a `PUT` of that address or of
`"follow_discovery": true`:

```json
"found_elsewhere": {"address": "192.0.2.24:62910", "configured": "192.0.2.23:62910", "since": "2026-11-04T15:00:02+01:00"}
```

`other_network` is `true` where the address the server connects to lies in none
of this machine's networks, as its interfaces' addresses and netmasks give them,
and absent otherwise, as it is for a host name:
the server reaches it only through a router, if at all.
A Cube with DHCP on and no lease uses its fallback address,
192.168.0.222/16 from the factory (docs/cube-protocol.md),
and refuses a connection from another network;
`massimo netconfig -sn <serial> -broadcast -static …` gives it a fixed address (README.md).
The networks are read again at most every 30 seconds.

`slots_low` says that the Cube's memory slots stay held:
a radiogram for a device that does not answer holds its slot until the device
answers or the Cube restarts, and one Cube held 49 of its 50 for months.
Each `S:` reply to a radio line and each greeting's `H` line is a reading of the
free slots;
a reading counts while nothing of the server's is in flight to the Cube,
no `s:` line (the server's own or a proxy client's) accepted in the 15 minutes
before it, since its radiogram may still wait for its device's wakeup
(an `r:` holds no slot).
Two counting readings an hour apart with fewer than 5 free,
and none with 5 or more in between,
give the Cube's log event `slots` at `warn` (see the log, below)
and `slots_low` with `level` `warn`;
two with fewer than 2, and none with 2 or more in between,
give it at `error` and `level` `error`, once each in a run,
the error without the warning where both come due at one reading.
A reading with 2 or more ends the error, which is logged as the step back to
the warning;
one with 5 or more gives `slots` at `info` and drops `slots_low`.
`slots_low` has `since`, the first reading below 5,
`full_since`, the first below 2, for an error,
`free_slots` at the last reading, and `logged`, when its level was logged.
Below 5 free the server holds each radio command for a while before it sends
it all the same (the log's `hold` with `memory_slots`).
A reboot of the Cube (`POST …/reboot`, below) drops the radiograms it holds.
The server reads no budget of its own for it:
the readings are those its writes, its proxy's clients and its reconnects bring,
the periodic refresh (`serve -refresh`, or `refresh_interval` in the settings)
among them.

```json
"slots_low": {"level": "error", "since": "2026-11-14T07:10:00+01:00", "full_since": "2026-11-14T08:00:00+01:00",
  "free_slots": 1, "logged": "2026-11-14T09:02:00+01:00"}
```

`problems` lists, where there are any, the Cube's visible devices with a problem,
as `GET …/devices` shows them, each with its room's name (`room`):
a communication or link error, a low battery, a window open (with its `window` times);
a client that shows every Cube's problems, such as the web UI's header,
needs no other request.
It is the server's last state, also while the Cube is not connected.
A found Cube has only the fields shown: `address` is the IP it answered from,
its own where a massimo proxy answers for it too (docs/proxy.md),
and `proxies` the other addresses that answered for its serial in the latest run;
`kind` is what answered: `cube`, a Cube
(it answered the `N` or `c` request, or has eQ-3's MAC),
`proxy`, a massimo proxy (it answered the identify request alone, from a MAC
that is not eQ-3's, or from the address of a massimo server announced on the LAN),
or `unknown`, neither, as on the loopback;
a proxy has `serves`, the serial of the Cube with its radio address,
served here or found, where one is known.
A found Cube whose `address` lies in none of this machine's networks has
`other_network`, as a configured one has (above), in the `cube_found` event too;
a proxy never has it.
`host` names the massimo server that serves the entry:
for a proxy the one at its address, for a Cube the one a proxy of it runs on,
with its instance name (`name`) and the `url` its announcement gives
(`_massimo._tcp`, README.md); it is absent where no announced server matched.
The servers are asked by mDNS beside each run of discovery, for as long,
by multicast, or at the addresses of `-discover-target` where it is set;
the answer of a run that heard none keeps the last.
`found_at` when the server first saw it, `last_seen` its latest answer;
`mac` comes from the host's ARP cache and may be missing.
`gateway`, `netmask` and `dns` are the addresses the Cube uses,
from its answer to the `N` request of this server run's latest discovery that had one,
kept in memory only as `dhcp` is.
`dhcp` says whether the Cube takes its address by DHCP (`true`) or has a static
one (`false`):
discovery asks each Cube that answers for its network configuration (the `c` request,
a read), and the flag is that of its latest answer in this server run;
it is kept in memory only, so it is missing until the Cube has answered `c`,
after a restart too.
To use it, add it with `POST /api/v1/cubes` (below),
or configure it in the settings file (its address,
or its serial with `-sn` or a `_NAME`, see the README) and restart serve.
The answers of serve's own proxies (docs/proxy.md) are not listed.
A found Cube is listed while it answers:
from its first answer until a complete run of discovery does not hear it,
a search or the periodic one alike;
a run that fails, such as one without the discovery port, changes nothing.
Its row stays in the server's store with its first and latest sighting
(the latest is written once per server run, or when an answer changes the row),
so that a restart does not announce it again;
after a restart it is listed once a run has heard it.
A Cube that was removed (`DELETE`, below) gets no such row:
it is found in memory only, and after a restart once a run has heard it.
Its entry lists the removed rows of its serial, newest first,
as `removed`, each `{"uuid", "name", "removed_at"}`;
`POST /api/v1/cubes` adds it as new, or with `restore` one of them.

`link` says how the server stands with a configured Cube,
beyond `connected`,
with `since` and for a hold `until` where they apply:

- `connected`:
  a connection is up and its greeting read;
- `held`:
  the Cube is lost, its connection held open since the loss,
  until `until` at the latest,
  so that it resumes once the network is back (`serve -cube-hold`, README.md);
- `stale`:
  the Cube answers discovery from its address,
  but the server's connections time out, since `since`,
  after the server's side ended the last one;
  the Cube still holds that one, as after an outage of the server's network,
  and lets go after up to about 40 minutes;
  `POST /api/v1/cubes/{sn}/reboot` frees it at once (below);
- `busy`:
  the same signs, the server having left no connection behind;
  another client holds the Cube, such as the MAX! software,
  another server or an integration;
- `rebooting`:
  a reboot was sent (`POST /api/v1/cubes/{sn}/reboot`) since `since`,
  and the Cube has not greeted the server since, until `until` at the latest;
- `offline`:
  none of these, such as a Cube that answers nothing.

`source` says where a configured Cube's configuration comes from:
`env` (taken over from the settings file into the store),
`ui` (changed through the API), `flag` (`-addr` or `-sn`, for this run).
The store wins over the settings file;
`file_differs` lists the file's lines for the Cube whose values differ from the store's,
with the field as this object names it (`name`, `address`, `proxy_address`,
`proxy_serial`, `hidden`), the variable, and both values (lists for `hidden`);
it is absent when the file agrees or has no lines for the Cube.

The configuration the routes below change is in the object too:
`configured_address` (the stored address; empty when the Cube follows discovery,
while `address` is where it connects now),
`follow_discovery` (the address follows the Cube's discovery answers;
always true without an address),
`proxy_address` and `proxy_serial` as stored (empty: the default),
`proxy_listening` (the addresses its proxy listens on now),
`proxy_failing` (its proxy listeners that do not listen, absent where none)
and `hidden` (the radio addresses hidden besides room 255).

A proxy listener that cannot listen,
on an address the host does not carry yet or a port in use,
is tried again until it does ([docs/proxy.md](proxy.md)):
after a second, the wait doubling up to 30 seconds.
Until then it is in `proxy_failing`, not in `proxy_listening`:

```json
"proxy_failing": [{"addr": "192.0.2.10:62910",
  "error": "listen tcp 192.0.2.10:62910: bind: cannot assign requested address",
  "since": "2026-11-13T08:00:00+01:00", "next_try": "2026-11-13T08:02:30+01:00", "tries": 9}]
```

`error` is the last try's, `since` the first failure's time,
`next_try` when it is tried again, `tries` the tries so far.

`budget` is what the Cube last reported:
the share of the hourly radio budget already used,
and the free memory slots (each unacknowledged radiogram occupies one).
`pending_radiograms` is the number of slots in use,
that is commands the Cube has sent and no device has acknowledged yet,
all 50 when no slot is free, and 0 until the Cube has reported its budget;
a setpoint shows in a device's state once that device has acknowledged it,
and the mode follows only with its next report,
so a non-zero value says the states may still change.
`queued` is the number of commands waiting in the server.
`observed_since` is when the server first heard from the Cube (see the
ages of device values below).
`cube_time` is the Cube's clock, in its own time zone and to the minute,
as its greeting told it at the last connect, and `cube_time_at` when the
server read it: the clock moves on by the time since, and the two say how
far it is off.

The Cube reports its budget only in the answer to a radio command and when a
connection starts,
so `budget` and `pending_radiograms` do not change on their own while devices acknowledge.
After radio writes through the API,
once the devices had two minutes and the re-reads below are done,
the server reconnects to the Cube for a fresh report if radiograms were still pending,
at most every 15 minutes and only while the periodic refresh (`serve -refresh`) is on;
the stream then sends `hello` and `connected`,
and `connected` is false for the moment of the reconnect.
Likewise the Cube changes a device's configuration only once the device has
acknowledged a write, which took one to four minutes on hardware:
after a configuration,
programme or room write the server reads the device's configuration again after 15 s,
30 s, 1, 2 and 2 minutes, until it changed,
and the stream sends a `config` event when it does.
Until then `GET …/devices/{rf}` and the room configuration show the old values.

### POST /api/v1/cubes, PUT and DELETE /api/v1/cubes/{sn}

The Cubes the server serves are rows of its store (docs/store.md), and
these routes change them while the server runs; each needs the store
and answers 503 without one.

`POST /api/v1/cubes` with `{"serial": "KEQ0000000"}` serves a Cube from now on:
one discovery found, or one given by serial.
Any field of `PUT` can come with it.
Without an address the Cube follows discovery,
starting at the address discovery saw last, else after the next search.
The answer is `201` with the Cube object; a Cube served already is `409`.
A Cube that was removed is added as new,
a row of its own without the removed rows' history,
unless `"restore": "<uuid>"` names one of the removed rows its found entry lists:
that row is served again with its settings, log, backups, rooms, commands and audit,
the body's other fields changing its settings (D11-A).
A `restore` that names no removed row of the serial is `404`,
one for a serial with a row that is not removed `409`,
and in a `PUT` it is `400`.

`PUT /api/v1/cubes/{sn}` with any of

```json
{"name": "Ground floor", "address": "192.168.1.9", "follow_discovery": false,
 "proxy_address": "192.168.1.49", "proxy_serial": "LEQ0000000", "hidden": ["0abc12"]}
```

changes those fields and leaves the others; an unknown field is `400`,
so a typo is not taken for "no change".
`"proxy_virtual": true` makes the Cube's proxy show its clients the rooms'
virtual wall thermostats (docs/proxy.md, "Virtual wall thermostats"),
kept apart from the Cube's row and off by default;
the Cube object says it as `proxy_virtual`.
The row's `source` becomes `ui`, and the change applies at once:
a new address reconnects,
a new proxy address or serial restarts that Cube's proxy listener
(one that cannot listen yet answers with `proxy_failing`, and is tried again),
a new hidden list hides (or shows) the devices in the API, the UI and the proxy.
An empty `address` makes the Cube follow discovery.
An empty `proxy_address` means `:62910` when the Cube is the only one
and nothing else listens, otherwise no proxy.
With `-proxy` on serve's command line the listeners stay as given there,
and a changed proxy address applies at the next start without `-proxy`.
Taking the settings file's value of a `file_differs` entry is a `PUT` of that value.
The answer is the Cube object.

`DELETE /api/v1/cubes/{sn}` stops serving the Cube:
its connection and its proxy stop,
and its row is removed, kept in the store with its history.
Discovery lists the Cube as found while it answers, with that row in `removed`;
`POST` adds it again, as new or, with `restore`, as the removed row;
a line in the settings file does not bring it back by itself.

A Cube served by serial for this run only (`-sn` without a row)
is not in the store and answers `409` to `PUT` and `DELETE`; `serve -addr`
runs without a store, so these routes answer `503` there.

### GET and PUT /api/v1/settings

The server's settings in the store:

```json
{
  "stale_after":       {"value": "50h", "default": "50h", "set": false, "in_effect": "50h", "applies": "now"},
  "stale_after_wall":  {"value": "24h", "default": "24h", "set": false, "in_effect": "24h", "applies": "now"},
  "poll_interval":     {"value": "10s", "default": "10s", "set": false, "in_effect": "10s", "applies": "restart"},
  "refresh_interval":  {"value": "6h",  "default": "6h",  "set": false, "in_effect": "6h",  "applies": "restart"},
  "discover_interval": {"value": "30m", "default": "30m", "set": false, "in_effect": "30m", "applies": "restart"},
  "duty_wait":         {"value": "10m", "default": "10m", "set": false, "in_effect": "10m", "applies": "restart"},
  "window_warn_after": {"value": "5m",  "default": "5m",  "set": false, "in_effect": "5m",  "applies": "now"},
  "window_alert_after": {"value": "10m", "default": "10m", "set": false, "in_effect": "10m", "applies": "now"},
  "update.channel":    {"value": "alpha", "default": "alpha", "set": false, "in_effect": "alpha", "applies": "now"},
  "update.auto":       {"value": "off", "default": "off", "set": false, "in_effect": "off", "applies": "now"},
  "update.auto_window": {"value": "03:00-05:00", "default": "03:00-05:00", "set": false, "in_effect": "03:00-05:00", "applies": "now"},
  "serverbackup.time": {"value": "03:30", "default": "03:30", "set": false, "in_effect": "03:30", "applies": "now"},
  "node.id":           {"value": "01a1ae5a-6bee-7ed4-bb4e-3baa2c43fe55", "set": true, "read_only": true},
  "tenant.id":         {"value": "01a1ae5a-6bee-7edb-b3d5-390730e9e1c5", "set": true, "read_only": true}
}
```

`value` is the store's, or the default when `set` is false;
`in_effect` is what this run uses, which a serve flag (`-poll`, `-refresh`,
`-discover-interval`, `-stale-after`) overrides for one run;
`fixed_by` then names the flag.
`poll_interval` is how often the server asks each Cube for its device list,
10 seconds by default, which costs no radio time;
`refresh_interval` how often it connects to each Cube anew and reads
everything the Cube holds, 6 hours by default;
`discover_interval` how often it looks for Cubes on the LAN, 30 minutes by default;
`duty_wait` how long radio commands wait while a Cube's duty cycle is used up
before the server reads the Cube's budget again by a new connection,
which costs no radio time, 10 minutes by default:
the duty cycle falls within the hour (docs/cube-protocol.md), so the server
does not wait for the next full hour, and holds again while it is still used up.
`stale_after` is when a radiator thermostat that has not changed counts as stale,
50 hours by default, since an idle one changes its entry once a day and not on
its decalcification day (docs/cube-protocol.md, "How devices report");
`stale_after_wall` the same for a wall thermostat, 24 hours by default;
both apply at once, the intervals at the next start.
`window_warn_after` and `window_alert_after` are how long a window may stay open
before it counts as open too long, a warning and then an alert
(5 and 10 minutes, 1m to 24h, the alert not under the warning, at once;
a device's `window`, above).
`commands.keep_days` is how many days finished commands are kept (see Commands),
a number, 0 for ever, and applies at once;
`log.keep_days` how many days the Cubes' events are kept (see `GET …/log`),
30 by default, 0 for ever, at most 3650, and applies at once:
a lower value removes the older events within moments;
`audit.keep_days` how many days the audit's rows are kept (see
`GET /api/v1/audit`), 365 by default, 0 for ever, at most 3650, at once too,
an `audit_pruned` line in the log saying how many went.
The backups' settings are whole numbers that apply at once (see Backups):
`backups.auto_min`,
the minutes without a change before the automatic backup (60, 1 to 10080),
`backups.auto_max`, the minutes after the first change at the latest (120,
1 to 10080),
a request that leaves `backups.auto_min` above `backups.auto_max` answering
`400` with nothing written; `backups.keep_days_auto`,
the days an automatic backup is kept (90), `backups.keep_days_manual`,
the days the others are kept (365), 0 for ever and at most 3650,
and `backups.keep_newest`,
how many of a Cube's newest backups are kept whatever their age (10, 0 to 1000);
a change of the last three prunes at once.
`auth.password_login`, true by default, says whether `POST /api/v1/login`
takes a password, and applies at once:
off, it answers `403`, the start of a sign-in by OPAQUE says `"v1": false`,
and the users sign in by OPAQUE alone, so that no password crosses;
the API keys are not touched.
A user still of `auth_version` 1 cannot sign in then,
until someone with `users` sets a password for it.
`integrations.requests`, true by default, says whether integrations may
ask to be registered (`POST /api/v1/integrations/requests`), and applies
at once; the announcement's `reg` says it on the LAN.
`auth.keep_open`, false by default, says that a server without regular users
is kept open on purpose, and applies at once:
`GET /api/v1/me` then says `kept_open`, and the web UI does not ask for the
first owner.
The web UI sets it from the sign-in card once the page's own address is entered,
and System settings → Users turns it off again (docs/ui.md).
It changes nothing about who is let in:
the local network alone, as on any open server (Authentication).
It counts only while the server has no regular users:
the first regular user removes it, however it is made
(the API, `massimo user add`, `MASSIMO_API_TOKEN`, a sign-in through a provider),
and so do the server's start and a reload of the settings file
where they find regular users beside it;
the log says `keep-open setting removed`.
An integration approved and its users leave it as it is.
With regular users, `in_effect` is `false`, and `true` is `409`;
`false` and `null` are taken.
`auth.password_reveal`, true by default, says whether the web UI's password
fields offer their eye, a button at a field's end that shows the password as
it is typed, and applies at once (AP MobileApp, D84-A):
off, no field shows one, the browser's own eye hidden either way;
`GET /healthz` says it as `password_reveal`, since the sign-in reads it
before anyone signs in, and System settings → Users has its switch (docs/ui.md).
`announce`, `auto` by default, says whether the server announces itself
on the LAN by mDNS (README.md, "Server"): `on`, `off`,
or `auto`, on unless it listens on a loopback address;
it applies at the next start, `in_effect` saying `on` or `off` for this
run, and where `-announce` or `MASSIMO_ANNOUNCE` decides instead,
`fixed_by` names it.
`update.channel` is the update channel (see `GET /api/v1/update`),
one of `devel`, `nightly`, `alpha`, `beta` and `stable`,
the build's flavour by default, and applies at once;
`MASSIMO_UPDATE_CHANNEL`, where set, is used instead, as `in_effect` shows.
`update.auto` says whether the box installs a release by itself:
`off`, the default, or `night`,
inside the window `update.auto_window`, `HH:MM-HH:MM` in the server's time,
`03:00-05:00` by default, at least 30 minutes, its end past midnight allowed
(`23:30-01:30`); both apply at once (see `GET /api/v1/update`, `auto`).
`serverbackup.time` is the daily time of the server's own backups, `HH:MM`
in the server's time, `03:30` by default; it applies at once ("Server backups").
`PUT` takes an object of keys and durations,
`{"stale_after": "36h", "refresh_interval": null}`,
with `null` for the default and `"0"` to switch the refresh or the periodic discovery off.
A number setting takes a JSON number, or the number as a string,
`auth.password_login`, `integrations.requests`, `auth.keep_open`
and `auth.password_reveal` `true` or `false`,
`announce` `auto`, `on` or `off`, `update.channel` a name,
`update.auto` `off` or `night`, `update.auto_window` a window as above,
and `serverbackup.time` a time `HH:MM`.
The ranges: `stale_after` and `stale_after_wall` 1h to 720h, `poll_interval` 10s to 1h,
`refresh_interval` 10m to 168h, `discover_interval` 1m to 24h, `duty_wait` 1m to 1h,
`window_warn_after` and `window_alert_after` 1m to 24h.
Every value is checked before any is written.
The answer is that of `GET`.

### POST /api/v1/cubes/{sn}/reboot

Reboots the Cube: the server sends it `R` on UDP port 23272
(`docs/cube-protocol.md`, "Cube UDP protocol"),
from its discovery socket, where the Cube's answer arrives.
The Cube is gone for about a minute and then takes a connection again;
one that still holds an old connection (`link` `stale`, above)
does so otherwise only after up to about 40 minutes.
A restart drops the radiograms the Cube has not delivered,
and its rooms refuse commands for a while after it,
until their devices have reported:
the server never reboots a Cube by itself,
and while radiograms are pending as the Cube last said (`pending_radiograms`),
the request names the count it accepts to drop:

```json
{"pending": 3}
```

A request without it, or with fewer, is `409`, naming the count and when
the Cube said it.
A Cube that another client holds (`link` `busy`) restarts too,
and that client may take it again before the server does.
The answer is `202`:

```json
{"sent": true, "answered": true, "status": 2}
```

`answered` says whether the Cube answered within a second,
`status` its answer's status, 2 for restarting (firmware 1.1.3), null without one.
The client's `link` is `rebooting` until the Cube's greeting, three minutes at most,
and the server's log has no loss for it.
`409` also comes while the Cube's serial or address is not known yet,
`502` when `R` could not be sent.
`massimo netconfig -reboot` sends the same request without the server (README.md).

### POST /api/v1/cubes/{sn}/refresh

Reads the Cube again at once,
as the periodic refresh does every `refresh_interval` (the settings, above):
the server ends its connection to the Cube and connects again,
and the Cube's greeting replaces what the server holds of it,
its rooms, its devices with their state and battery flags, and its budget.
The reconnect goes through the Cube's queue, after the commands before it,
so that it cuts none off;
it sends nothing by radio and takes no memory slot.
It needs no body, and the capability `restart`.
The request waits for the new greeting, a minute at most once its turn has come,
and answers `200`:

```json
{"ok": true, "budget": {"duty_cycle_pct": 5, "free_memory_slots": 49, "updated_at": "…"}}
```

`budget` is the one the new greeting brought.
The server's log says `snapshot refreshed` with `asked` and the user,
and the Cube's log has the request as a change, with who made it.
`409` comes while the Cube reboots (`link` `rebooting`),
whose greeting after the reboot reads it again,
while the Cube's serial or address is not known yet,
and when a newer refresh took the request's place before its turn;
`502` when the reconnect failed:
the Cube did not greet again within the minute, as when it is not reachable
or another client holds it (`link` `busy`),
or the server still holds a lost connection to it (`link` `held`).
The error names the reconnect's error and the `link` after it.

### POST /api/v1/cubes/discover

Looks for Cubes on the LAN now (UDP discovery, about three seconds) and
answers with every Cube that answered, configured or not, once each,
at its own address where a proxy answered for it too, as the
objects of the found Cubes above:

```json
{
  "ran_at": "2026-11-04T15:00:00+01:00",
  "found": [
    {"serial": "KEQ0000200", "configured": true, "address": "192.168.1.9", "mac": "00:1a:22:0a:0b:0c",
     "rf_address": "0a0104", "firmware": "1.1.3", "found_at": "2026-11-04T14:30:01+01:00", "last_seen": "2026-11-04T15:00:02+01:00"},
    {"serial": "KEQ0000000", "configured": false, "address": "192.168.1.23", "mac": "00:1a:22:01:02:03",
     "rf_address": "0a0b0c", "firmware": "1.1.3", "dhcp": true, "found_at": "2026-11-04T14:30:01+01:00", "last_seen": "2026-11-04T15:00:02+01:00"}
  ]
}
```

The server's own proxies are never among them:
an answer from this host with a configured Cube's serial is left out,
and so is one from any address with the serial a proxy of this server answers
with (`proxy_serial`), unless a Cube gave it.

No body.
A call while a run is going waits for the run after it,
which starts as that one ends and which every call that came meanwhile shares,
the same `ran_at`: a run sends its requests as it starts,
so the answer is never that of a run begun before the call,
and such a call takes up to twice the wait, about six seconds.
While another program on the server's host searches for Cubes on UDP 23272,
such as the original software, a run waits for it,
up to 30 seconds (docs/proxy.md, "Discovery"),
and a run it interrupts lists only what it heard before.
`503` when the server cannot have the discovery port (UDP 23272,
the only one the Cubes answer to, or the address of `serve -discover-listen`),
for example because another program holds it.
`serve` also runs discovery at the start and every 30 minutes (`-discover-interval`,
`0` for on request only),
and every minute while a Cube configured by serial has no address yet.
A Cube whose connection is lost is searched for at the loss and every minute
until it is back, since it may have taken another address:
one following discovery always, one with a fixed address
where `-discover-interval` is not `0`,
which then logs that it answers from another address.
The first time a Cube that is not configured answers,
and again when it answers after a complete run without it,
every Cube's event stream sends `cube_found`; a Cube found before a restart
is not announced again after it:

```
event: cube_found
data: {"kind":"cube_found","cube":{"serial":"KEQ0000000","configured":false,"address":"192.168.1.23",…},"time":"…"}
```

A found Cube that does not answer a complete run leaves the list of `GET /api/v1/cubes`,
and serve's log says "cube gone from the LAN";
the stream sends nothing for it.

A configured Cube that answers from another address than configured is logged,
at Warn when that is new or the address it answers from changes,
at Debug for the runs that repeat it,
and shows as `found_elsewhere` (above);
one configured by serial only, without an address, follows discovery:
it keeps its address while that answers,
else takes the address of a proxy that answers for it
(a proxy answers only while its serve holds the Cube,
whose one LAN connection is then taken),
else the Cube's own, and reconnects when that changes, at most once a run.
An answer that is neither a Cube's nor a proxy's for sure
(no answer to `N` or `c` and no MAC, README.md "Command line")
is taken only by a Cube without an address yet.

### GET /api/v1/cubes/{sn}

The same object with `config` (portal URL and flag, push button settings,
time zone rules) and `room_list`, the rooms as below.

### GET /api/v1/cubes/{sn}/rooms

Rooms with their devices and live states,
each with its `room_uid` and the devices the view (`devices=`, see
"The virtual wall thermostat") shows:

```json
[
  {
    "id": 5,
    "name": "Bad",
    "ready": true,
    "thermostat": "0a0102",
    "comfort_temp": 21,
    "eco_temp": 17,
    "min_setpoint": 4.5,
    "max_setpoint": 30.5,
    "devices": [
      {
        "rf_address": "0a0102",
        "type": "radiator_thermostat",
        "name": "Thermostat Bad",
        "room_id": 5,
        "serial": "JEQ0000502",
        "reported": true,
        "state": {"mode": "auto", "setpoint": 22, "valve_pct": 0, "battery_low": false, "error": false, "link_error": false},
        "comfort_temp": 21,
        "eco_temp": 17
      }
    ]
  }
]
```

Device types: `radiator_thermostat`, `radiator_thermostat_plus`,
`wall_thermostat`, `shutter_contact`, `eco_switch`.
`reported` says whether the Cube holds state for the device;
it is false for every device after a Cube restart until the device transmits
again (window contacts only do so on a window change),
and `state` is then absent.
It is false too for a device the Cube has not heard for some minutes, whose
entry the Cube marks out of date (a wall thermostat after about five minutes of
silence, docs/cube-protocol.md): such a device has `out_of_date: true` and
`last_known`, the values of its last report, as `state` holds them,
with `received` and `last_seen` as old as they are.
`ready` on a room is true when all its devices have reported,
which is the condition for the Cube to accept a room command.
`thermostat` on a room is the radio address of the thermostat whose state is the room's,
the one its virtual wall thermostat takes its setpoint and until from
(the first of its `sources.setpoint`, below; RoomMajority.D9, D19-A):
one of those whose state more than half of the room's thermostats show,
else the one whose mode or setpoint changed last,
else the first of the room's one order,
its master where that is a thermostat of the room, else its wall thermostat,
else its first radiator thermostat;
during a boost the same over the thermostats that do not boost,
so that the room's tile and its virtual wall thermostat show one setpoint.
Where no thermostat of the room has a state,
it is the first of that order, as the room's commands name it;
absent for a room without a thermostat.
The web UI shows the room's set temperature and mode by it,
and between two reads of the rooms by its virtual wall thermostat's `sources.setpoint`,
which that one's `state` events bring;
the room's commands keep naming the first of the order,
the dead passed over, whichever thermostat `thermostat` names.
`comfort_temp` and `eco_temp` on a room are the room's comfort and eco temperatures,
as `GET …/rooms/{id}/config` gives them:
what more than half of its thermostats store, else the first of that order's;
the web UI marks the room's setpoint by them, and each device's by its own.
`min_setpoint` and `max_setpoint` on a room are the room's lowest and highest setpoint,
from the same configuration, absent for a room without a thermostat:
a client offers the room's setpoints within them,
4.5 being OFF and 30.5 ON there as in any setpoint.
`confirmation`, on a device and on a room, is present while a write to it is
not confirmed yet (see "Writes confirmed by the devices"):
its `state` (`pending`, `unconfirmed`), `since` the Cube took the oldest write
still open, the `fields` not shown yet (the state's names, the configuration's,
and `program.<day>` for a day of the week programme), and on a room the
`devices` that do not show them; a virtual wall thermostat has its room's.
Thermostats carry `mode` (`auto`, `manual`, `vacation`, `boost`), `setpoint`,
`actual_temp` once the device reported one (the last it reported, kept while
its entries carry none),
`valve_pct` for radiator thermostats, `until` for vacation mode;
shutter contacts carry `window_open`.
A radiator or wall thermostat's entry, beside `state`, carries `comfort_temp`
and `eco_temp`, its comfort and eco temperatures from its configuration
(its C line, as `GET …/devices/{rf}` gives them under `thermostat` or
`wall_thermostat`), absent where the Cube sent none:
the web UI marks a device's setpoint that is one of them,
as the device's display does, and a room's by the room's (below).
`error` is the device's communication error,
as the original software's UI calls it ("Communication error!"):
the Cube's last radiogram to the device failed,
and the flag stays until it is reset (`r:`) or the Cube restarts,
whatever the device reports meanwhile;
`failed_radiogram`, present with it alone, is
the type of the Cube's last radiogram to the device that failed,
`{"code": 3, "name": "TimeInformation"}`:
the code is byte 4 of the device's entry,
the name the original software's (docs/cube-protocol.md, its `FrameType` list),
absent for a code the list lacks, such as `{"code": 246}`;
nearly always the Cube's own time push (`TimeInformation`),
also a client's radiograms, such as a removal's `RemoveLinkPartner`.
`link_error` is the device's link error, its own report,
in the original software's words
"The radio communication with another MAX! device in this room is disturbed";
it clears with a later report of the device.
The original software names it `transmitError`,
which this API's names do not follow:
they stay `error` and `link_error`
(docs/cube-protocol.md, "Lessons from the hardware days").
`panel_locked` (present when true) is the key lock on the device.
`battery_low` is the device's battery flag as the Cube shows it now;
`battery`, beside `state`, is the level the server keeps from its history:
`ok`; `weak` once the flag has been seen, as a radiator thermostat shows it for
a few minutes under the load of its valve motor days before it stays;
`low` once the flag has stayed for 15 minutes.
A weak device is `ok` again once a long valve move has gone by without the flag:
its next decalcification, a valve move of 50 points or more,
or for a device without a valve a day;
a low one an hour after the flag went, with new batteries.
A low or weak device is also `ok` at once when someone marks its new batteries
once its flag is off (`POST …/devices/{rf}/battery`, below).
The levels outlive a restart: the server reads them back from its log
(docs/cube-protocol.md, "How devices report").
A change of a device's level is a `battery` event in the log,
its `summary` the `level` and the one before (`from`); `low` at warn.
Where someone marked new batteries, its `summary` has `cause` `replaced` too,
and `by` names who marked them.

How old the values are: a device with state also has `received`,
the time per value of `state` (same names) at which the server saw its current
value appear, `last_seen`,
when the device's entry in the Cube's device list last changed in any way, and,
for radiator and wall thermostats,
`stale: true` once that is more than 24 hours ago;
a room with a stale device is `stale` too.
For example:

```json
"received": {"mode": "2026-11-05T08:12:40+01:00", "setpoint": "2026-11-05T08:12:40+01:00",
             "actual_temp": "2026-11-05T06:01:10+01:00", "valve_pct": "2026-11-05T09:40:02+01:00", …},
"last_seen": "2026-11-05T09:40:02+01:00"
```

The Cube keeps no times and repeats every value in each device list,
so these are the times the server saw a value change,
which is as close as a client gets to when the device sent it.
A device that reports the same value again does not move its time,
and the Cube changes a setpoint in the list after a set-temperature command
as each device acknowledges it, within seconds for a device that answers;
a device that does not answer keeps its old setpoint.
Values that were already there when the server first connected carry that time,
the Cube's `observed_since`, and may be older;
a reconnect keeps the times of values that did not change.
Radiator thermostats report the measured temperature once a day in manual mode,
at 12:00 with the default decalcification time (Saturday 12:00) and not on
the decalcification day, at the programme's switch points in auto,
and not at all on vacation, when those bytes carry the end:
the server keeps the last one until a new one comes,
and `received.actual_temp` says how old it is.
`stale` is a hint, after `stale_after` for a radiator thermostat (50 hours by
default) and `stale_after_wall` for a wall thermostat (24 hours):
a thermostat out of range or with a flat battery goes quiet,
but so may one whose valve stays shut outside the heating season.
Shutter contacts and eco buttons transmit only on events and are never stale.

**The Cube's lines against its room list.**
A device's room for the Cube is the room byte of its C line, not the room list's
(the metadata, `M`):
a previous owner's unlisted devices, their C lines in two rooms,
made the Cube discard those rooms' commands.
So at every complete greeting, every C line and every change of the room list,
and when the Cube gains or loses a device's state,
the server compares the two, which costs no radio time,
and gives a device, and each room a finding concerns, `mismatches`,
a list of what it found, absent where nothing:

| `finding` | what differs |
|-----------|--------------|
| `room` | the device's C line names another room (`cube_room`) than the room list (`list_room`) |
| `unlisted` | a C line in a room (`cube_room`) for a device the room list does not have, which can block the room's commands |
| `no_config` | a device the room list has in `list_room`, that the Cube has no C line for, whatever its device list says: one it no longer holds, as a Cube sends a C line for every device it has |
| `type` | the C line's type (`cube_type`) and the room list's (`list_type`) differ |
| `master` | the room's master in the room list, the address of its entry (`device`), is no device of the Cube (`why` `no_device`), a device of another room (`other_room`, that room in `cube_room`) or no thermostat (`not_thermostat`); a room without a thermostat is not looked at |
| `programme`, `temperatures` | a thermostat of the room, by its C line, with a day of its week programme, or a comfort, eco, maximum, minimum or window-open temperature, that differs from the room's, as the copy (`PUT …/devices/{rf}`) and the configuration's repair decide it (RoomMajority.D9): each day and the temperatures what more than half of the room's thermostats share, else what the first of the room's order has (the master where it is a thermostat of the room, else the wall thermostat, else the first radiator thermostat), one the Cube holds no valid state for passed over; so a thermostat fresh from its copy is none. While a thermostat of the room has not reported since the Cube's restart, its entry neither valid nor initialized (`reported` false and not `out_of_date`), every finding of the room's thermostats stays as it was, neither found nor cleared, so that a restart of the Cube clears nothing and warns of nothing again as they report one by one; then one the Cube marks out of date (`out_of_date`) is passed over and its finding stays as it was, as do those of a room with fewer than two thermostats with a valid state, which has no reference; the others are judged. The window-open minutes, which a wall thermostat does not keep, are not compared |

Each finding has its `device` (the device concerned, a room's master for
`master`) and its `name` in the room list, its `room` (the room list's where it
has the device) with `room_name`, the rooms of both sides with their names
(`cube_room_name`, `list_room_name`) where the room list has them,
and `since`, when the server first found it.
A device's `mismatches` are those of its `device`, a room's those of its `room`
or of either side.
Left out are the hidden devices and those parked in room 255, and the rooms
they alone make up;
the device of a removal, placement or move through the API, and the rooms it
concerns, from the change's start until the device's C line follows,
5 minutes at most after its end
(the Cube changes a C line only once the device acknowledged);
for `programme` and `temperatures`, a room with a member whose configuration
write the server still follows (see the re-reads after writes, above);
and the whole Cube while a restore runs.

```json
"mismatches": [{"finding": "room", "device": "0a010a", "name": "Living room radiator", "room": 1, "room_name": "Living room",
  "cube_room": 2, "cube_room_name": "Office", "list_room": 1, "list_room_name": "Living room", "since": "2026-11-15T08:00:00+01:00"}]
```

Each finding is the Cube's log event `mismatch` at `warn` (see the log, below),
once, until a check finds the two agree again:
then `mismatch` at `info`, which settles the warning.
A room master's warning is given again while it holds once someone has marked
the last one seen (`log.seen`), at most once a minute:
warned only while no unread warning for it exists.
After a restart the server takes the findings its log holds unsettled,
so that it does not warn of them again.

A device whose state says a window is open (`window_open: true`,
a shutter contact, or a virtual wall thermostat of a room with one)
also has `window`, when the server saw it open and when it counts as open
too long, first as a warning, then as an alert,
by the settings `window_warn_after` and `window_alert_after`
(5 and 10 minutes by default, see `GET /api/v1/settings`):

```json
"window": {"open_since": "2026-11-05T09:40:02+01:00", "warn_at": "2026-11-05T09:45:02+01:00",
           "alert_at": "2026-11-05T09:50:02+01:00"}
```

A client tells the window's state from its clock, between two answers too;
the web UI shows it blue, then amber, then red.

### The virtual wall thermostat

Every visible room has a virtual wall thermostat:
a wall thermostat the server computes from the room's devices,
so that every client shows a room by the same numbers.
The original software allows one wall thermostat per room,
whose measured temperature the room's radiator thermostats regulate by;
a room's temperature is its virtual wall thermostat's `actual_temp`,
whichever client shows it.

```json
{"rf_address": "000005", "serial": "VWT0000005", "type": "wall_thermostat", "virtual": true,
 "name": "Bad", "room_id": 5, "room_uid": "0196…", "reported": true,
 "state": {"mode": "auto", "setpoint": 21, "actual_temp": 20.5, "valve_pct": 30,
           "window_open": false, "battery_low": false, "error": false, "link_error": false,
           "consistent": true},
 "received": {"actual_temp": "…", "mode": "…", …}, "last_seen": "…",
 "sources": {"actual_temp": ["0a0102", "0a0103"], "mode": ["0a0104"], "setpoint": ["0a0104"]}}
```

- It belongs to its room: its name is the room's,
  it comes with the room and goes with it (`device_forgotten`),
  and `room_uid` is the room's uid, which no later room of the same number takes.
  A room deleted and another made with its number while the server was not running
  cannot be told apart and keeps the uid.
- Its address is the room number (`000005` for room 5),
  unless a real device or the Cube has that one:
  then the first free one from `000100` to `0001ff`.
  Its serial is `VWT` and the room number in seven digits;
  no eQ-3 serial starts with V.
- `actual_temp` is the room's wall thermostat's
  (of two, set up by another tool, their median),
  else the median of the room's radiator thermostats',
  or their mean where the room's setting says so (`temp_method`);
  a median of two values is their mean, rounded to 0.1 °C.
  A stale value is left out while a fresh one exists;
  where all are stale they are used, and the virtual wall thermostat is `stale`.
  `received.actual_temp` is the oldest of the values used, so that the age never
  looks better than the data, and `sources.actual_temp` lists them.
- `mode` is `boost` while any thermostat of the room boosts;
  else the room's state by its one order (RoomMajority.D9, D19-A),
  over its thermostats with a state that do not boost, in the Cube's order:
  what more than half of them show
  (those in auto that each follow their own week programme counting as one state);
  else that of the one whose mode or setpoint changed last, by someone's hand,
  as the room checks' `newest` counts it (below);
  else that of the room's master,
  its wall thermostat, its first radiator thermostat,
  the first of them with a state.
  `setpoint` and `until` come from the same one,
  and `sources.setpoint` names those they came from, that one first:
  the thermostats that show its state, the one of the newest change,
  or the one of the order;
  `sources.mode` names the same.
  During a boost, `setpoint` and `until` are still those of that rule
  over the thermostats that do not boost (over those that boost, where every one does),
  which `sources.setpoint` names,
  and `sources.mode` names the boosting ones.
  It is the room checks' rule for the state they send again (below),
  where Massimo's own command and a state held from before a newcomer come first.
  `consistent` is false while the room's thermostats disagree on mode,
  setpoint or `until`.
- `valve_pct` is the highest of the radiator thermostats',
  `window_open` true while any shutter contact of the room is open;
  both are given so that a client need not compute them.
  `battery_low`, `error`, `failed_radiogram`, `link_error` and `panel_locked`
  are the real wall thermostat's; the radiator thermostats show their own.
- `reported` is true once a thermostat of the room has reported;
  a room without one has a virtual wall thermostat with `window_open` alone.
- It has a `state` event whenever its computed state changes,
  right after the event of the member that changed it, with the member's `command`.
  It is never in the device counts, the log, the notifications,
  a backup or a room's `ready`.

Which devices the lists show is the parameter `devices`
of `GET …/rooms`, `GET …/devices`, `GET /api/v1/cubes/{sn}` and `GET …/events`:

| `devices=` | real wall thermostats | virtual wall thermostats |
|------------|-----------------------|--------------------------|
| `default` (or none) | shown | in each room without a real one |
| `real` | shown | none: the answer before virtual wall thermostats |
| `virtual` | left out | in every room |
| `all` | shown | in every room |

Radiator thermostats, shutter contacts and eco switches are in every view.
In `default` and `virtual` every room has one wall thermostat,
in `virtual` always the same;
`default` only adds to what a client saw before.
Any other value is `400`.
Its commands are the device routes', under any of its names:

| Route | Does |
|-------|------|
| `PUT …/setpoint` | the room's setpoint (`PUT …/rooms/{id}/setpoint`) |
| `PUT …/program` | the room's programme (`PUT …/rooms/{id}/program`) |
| `PUT …/config` | `temp_method` and `rf_address` to its own settings; `temp_offset` and `display_actual` to the room's real wall thermostat, `400` without one; every other field to the room (`PUT …/rooms/{id}/config`) |
| `PUT …/lock` | the room's real wall thermostat's key lock; `409` without one, whose radiator thermostats are locked one by one |
| `PUT …/devices/{rf}`, `DELETE` | `409`: it is renamed with its room and goes with it |

A configuration of one part answers as that part's route does;
one of several answers each by name,
`{"ok": true, "room": {…}, "wall_thermostat": {…}, "settings": {…}}`.
`temp_method` is `median` or `mean` and needs `configure`, as the room's
settings do; `rf_address` is six hex digits, or `""` for the room number,
and one that a real device, the Cube or another room's virtual wall thermostat has
is `409`.
A change of its settings is a `config` event of it,
and a `state` event where its state changes.
Each command needs the capability of the route it goes to,
and the audit names the room as its target.

A room takes one wall thermostat, as in the original software:
moving or adding a second one into a room (`PUT …/devices/{rf}`) is `409`
before anything is sent.
A Cube that has two in a room, set up by another tool, is shown as it is.

`GET …/devices/{rf}` answers for a virtual wall thermostat whatever the view,
with the room's settings and week programme in a wall thermostat's shape
(`wall_thermostat`) and its own `settings`:
`{"temp_method": "median"}`, and `rf_address` where the owner set one.
The settings are the room's by its one order (RoomMajority.D9, D19-A),
as `GET …/rooms/{id}/config` decides them:
the five temperatures what more than half of the room's thermostats share,
else what the first of its order has,
the boost settings so among its radiator thermostats,
the wall thermostat's in a room without one,
a thermostat out of date passed over as there;
the week programme as `GET …/rooms/{id}/program` gives it,
under the `etag` beside it.
`temp_offset` is no room's but a device's own:
the room's real wall thermostat's, which a write of it reaches,
else that of the first of the room's order.
The proxy's virtual C line carries the same
(docs/proxy.md, "Virtual wall thermostats").

### The room checks

Every minute the server compares the thermostats of each room,
those whose C line names it, as the Cube goes by,
and writes what it finds to the Cube's log (`disagree` and `drift`, see the log, below).
Left out are the hidden and parked devices, window contacts,
a thermostat in boost and one whose entry is out of date;
a room while one of its windows is open
(its thermostats then hold their own window-open temperature),
while a structure change of Massimo's concerns it
(see "Structure changes read back"),
while a command to it waits or runs in the queue
and while a write to it is open or not confirmed, its `confirmation` set
(see "Writes confirmed by the devices": a room setpoint not confirmed is
sent once more there);
a difference not yet reported then starts again.

- **Thermostats that disagree:** they differ in mode, setpoint or `until`,
  as the virtual wall thermostat's `consistent` says,
  but two in auto that each follow their own week programme agree,
  since a difference of their programmes is their configuration's.
  No person can mean a room's thermostats to differ:
  the original software and Massimo set them only together.
  Differing for 10 minutes, the room is a `disagree` warning;
  still differing 10 minutes later, its state is sent once more
  as one room command (`0x40`) through the queue at low priority,
  the first of these, which the log's line names (`from`;
  RadioHealth.D188 as RoomMajority.D3-A, D4-A, D7-C and D9 change it):
  1. `massimo`: Massimo's last setpoint to the room (`PUT …/rooms/{id}/setpoint`,
     a member's or the virtual wall thermostat's setpoint),
     where it was sent at most 15 minutes before they began to differ,
     or since, and a thermostat still shows it.
     Where it was auto, auto again:
     auto without a temperature, the week programme, takes the programme's value now;
     auto with a temperature, which lasts until the next switch point,
     is sent as it was while no switch point of a thermostat's programme
     has passed since, at the Cube's time, and else as auto on the programme,
     which a thermostat on its programme shows too;
  2. `majority`: else the state more than half of the compared thermostats show,
     their mode, setpoint and `until`, those on their own programmes counting
     as one state, auto on the programme;
     in its place, where the server holds one for the room,
     the state its thermostats showed before a structure change brought a device in
     (`before_change`: true, with the `from` of the rule that took it then):
     taken by this order (steps 1 to 5) over the room's thermostats,
     the device brought in left out where its C line names the room already,
     under the structure lock, before the device is linked,
     by `PUT …/devices/{rf}` that adds a thermostat to the room or moves one into it,
     whose copy gives it to the device too (RoomMajority.D8-A),
     though not with `"copy": false`, where the device keeps its own,
     and by the repair `…/devices/{rf}/repair/place` of a thermostat (RoomMajority.D15-A);
     held once the Cube accepted a radiogram of the change's link half,
     or one was sent whose answer did not come,
     or the request's time ran out while the link half was under way,
     since the room's thermostats may then take the newcomer's state;
     not held where the room has no state,
     nor where no radiogram of the link half went through,
     which leaves a hold of an earlier change as it was;
     nor where Massimo set the room another state since it was read;
     auto with a temperature as for `massimo`,
     auto on the programme once a switch point has passed since it was held;
     held until the room's first check after the change that finds its thermostats
     agreeing, on any state, two at least or every thermostat of the room;
     until a compared thermostat is changed by hand to another state
     after the first check that judged the room
     (one to the held state, as by its own room command, keeps it);
     until a newer setpoint of Massimo's to the room, but one of the held state;
     until a held holiday's end has passed at the Cube's time;
     for a day at most;
     and until a later change holds its own in its place,
     but one made before any check has judged the room keeps the earlier hold
     and its time;
  3. `newest`: else the state of the thermostat whose mode or setpoint changed last,
     by when the server saw each value change from an earlier one, as `received` dates it:
     a change by hand at a device is the newest wish (RoomMajority.D3-A).
     A value's first sighting has no time here, though `received` dates it:
     one there when the server first connected to the Cube,
     a new device's, an entry's that got its values late;
     nor has the mode with which a boost ends,
     nor its setpoint where it is the one from before the boost
     (a new one, turned at the wheel to end the boost, is a change);
     nor have the mode and setpoint with which a holiday ends at its end,
     read on the Cube's clock, which the device changes by itself.
     A thermostat's time is the later of its mode's and its setpoint's,
     but where one of them is in manual or on holiday, one that follows its week
     programme counts its mode's time alone,
     since a switch point changes nothing in manual or on holiday (D7-C).
     None where no thermostat has a time,
     or where thermostats that do not agree share the newest;
  4. else the state of the first of the room's order (RoomMajority.D9),
     its week programme where it follows it:
     `master`, the room's master in the Cube's list,
     where it is a thermostat of the room;
     `wall_thermostat`, its wall thermostat;
     `radiator_thermostat`, its first radiator thermostat, then each other one
     in the Cube's order;
  5. else nothing, the warning alone (`not_resent`, `no_state`),
     which a room that disagrees no longer reaches:
     `no_state` is found only in lines written before RoomMajority.

  A thermostat the Cube holds no valid state for,
  `reported` false or `out_of_date`, is passed over at every step,
  as the comparison leaves it out;
  the communication error and the link error do not make it so.
  The state is sent at most once a day per room,
  a day shared with the re-send of a room setpoint not confirmed
  (one lost room command, one extra write):
  a room sent its state once more by either is not by the other that day;
  but a held state's one re-send goes out whatever the day had,
  the one radiogram its structure change costs;
  the day's re-sends of both are read back from the log after a restart.
  It is never sent while the queue holds the radio for the duty
  cycle or the memory slots: it waits for a later check then.
  Once they agree, an `agree` line settles the warning.
- **A thermostat off its week programme:** in a room in auto,
  one whose setpoint has not been its own programme's temperature
  at the Cube's time for 5 minutes, while every other thermostat
  of the room follows its own: a `drift` warning naming its clock or its
  programme as suspect, with the radiogram that failed where it has the
  communication error, and `hour_off` where its setpoint is the programme's
  an hour before or after, as a clock on the other side of a change of summer
  time would show it; its `auto_dst` says whether it switches summer time itself.
  Nothing is sent for it.
  A temperature the programme has within 5 minutes of the Cube's time counts,
  so that a switch point just past, or a clock a few minutes off, is no drift.
  The Cube's time is its last greeting's, moved on by this host's local clock.
  Following again, a `follows` line settles the warning;
  the next warning for the thermostat comes a day later at the earliest.
  Such a thermostat is left out of the comparison above.

### GET /api/v1/cubes/{sn}/devices

Every device the Cube knows, in the Cube's order,
then the virtual wall thermostats the view (`devices=`) shows.
Devices that are paired but not assigned to a room have `"unlisted": true` and no name;
they do not appear under rooms.

Hidden devices, those in room 255 and those listed in `MASSIMO_CUBE_<SN>_IGNORE`,
are invisible to the API unless the server runs with `-show-hidden`:
they are absent from every list and count, their addresses answer `404`,
their events are not streamed,
and a room that holds only such devices is absent as well,
including from room commands.
The use case is a device that is paired but never reports,
which blocks every room command for the room the Cube counts it in and which the
Cube never drops;
it is parked in room 255 on the Cube and hidden here (see
[cube-protocol.md](cube-protocol.md)).
A device that answers is removed from the Cube instead,
with `DELETE …/devices/{rf}` (below).

### GET /api/v1/cubes/{sn}/devices/{rf}

One device with its configuration: `firmware`,
and for thermostats `thermostat` or `wall_thermostat` with the preset temperatures,
valve or boost settings and the week program keyed by weekday (`monday` … `sunday`,
each a list of `{"temp": 17, "until": "07:00"}`).
A wall thermostat also reports `display_actual`.

### PUT /api/v1/cubes/{sn}/devices/{rf}/setpoint

Set mode and setpoint of one thermostat.
Body:

| mode       | fields                                   | effect                                        |
|------------|------------------------------------------|-----------------------------------------------|
| `manual`   | `temp` (4.5 … 30.5, steps of 0.5)        | hold `temp`                                   |
| `auto`     | optional `temp`                          | follow the week program (`temp` until the next switch point) |
| `vacation` | `temp`, `until` (`"YYYY-MM-DD HH:MM"`, half hours) | hold `temp` until `until`, then auto |
| `boost`    | none (`temp` is ignored by the devices)  | open the valves by the thermostats' boost settings (`boost_minutes`, `boost_valve_pct`) for their boost time, then back to the mode they had |

```json
{"mode": "manual", "temp": 21}
```

The request waits for the Cube's answer (up to two minutes when the
radio budget is exhausted) and returns

```json
{"ok": true, "discarded": false, "budget": {"duty_cycle_pct": 5, "free_memory_slots": 49, "updated_at": "…"},
 "confirmation": "pending"}
```

`ok` is the Cube's answer: it has taken the radiogram.
`confirmation` is `pending` until the device shows the setpoint and mode,
or `confirmed` where it shows them already
(see "Writes confirmed by the devices").
A device that belongs to a room is addressed as its room,
because the Cube discards a direct command to a room member;
every thermostat of that room therefore takes the new setpoint,
which is how MAX! treats setpoints.
A device outside any room is addressed directly.
A boost is the set-temperature command with mode 3, as a button at a device starts it;
the command carries the setpoint the device has, which the devices ignore.
Seen on hardware on 2026-11-13: both thermostats of a room showed `boost` within a minute,
and returned by themselves to the mode they had after their boost time
(docs/cube-protocol.md).

### PUT /api/v1/cubes/{sn}/rooms/{id}/setpoint

Same body and answer, addressed to every device of the room, which is
what the original software does:
one set-temperature radiogram with the room flag, which every member takes.
It names one thermostat of the room,
the first of the room's order (RoomMajority.D9, in place of DeviceUnPairingFix.D167):
the room's master from the Cube's metadata where that is one,
else the room's wall thermostat, else its first radiator thermostat,
each passed over while the Cube holds no valid state for it
(`reported` false or `out_of_date`) and another thermostat of the room is not,
never a device of another room, which would take the command as well
(docs/cube-protocol.md, "Set temperature (0x40) addressing").
A room without a thermostat is refused with `409`.
Before the radiogram, every member of the room whose communication error
(`error`) is set gets one `r:`, the room taken from each device's C line,
as the Cube takes it, so that the Cube does not hold the command back
for a flagged member, as the original software does;
the link error alone (`link_error`) gets none.
A device's setpoint, a room command too, and a room's programme do the same.
Where the Cube discards the radiogram, the same members are reset once more
and it is sent once more.
`confirmation` is `pending` until every visible thermostat of the room shows
the setpoint and mode.

### Writes confirmed by the devices

The Cube answers a radio write once it has taken it;
a device's state takes the new values only when the device has acknowledged the
radiogram, at its next wakeup, and a room's members take a room command each at
its own (docs/cube-protocol.md).
So a setpoint, a key lock and a wall thermostat's display mode are confirmed
by the devices' states, at each of their reports and at a deadline five
minutes after the Cube's answer:

| Write | Confirmed by `state`'s | Devices |
|-------|------------------------|---------|
| setpoint `manual` | `mode`, `setpoint` | a room's visible thermostats, or the device |
| setpoint `vacation` | `mode`, `setpoint`, `until` | the same |
| setpoint `auto`, `boost` | `mode` | the same |
| key lock | `locked` (`panel_locked`) | the device |
| display | `display_actual` | the wall thermostat |

A configuration write is confirmed by each device's configuration instead,
which the Cube changes once the device took the write, and which the server
re-reads after it, about six minutes in all:
it compares what was written with what it reads, value by value.

| Write | Confirmed by the configuration's | Devices |
|-------|----------------------------------|---------|
| presets (0x11) | `comfort_temp`, `eco_temp`, `max_setpoint`, `min_setpoint`, `temp_offset`, `window_open_temp`, `window_open_minutes` | each device written |
| valve (0x12) | `boost_minutes`, `boost_valve_pct`, `decalc_day`, `decalc_hour`, `max_valve_pct`, `valve_offset_pct` | the same |
| programme (0x10) | the day, `program.<day>`; a long day's second radiogram `program.<day>.2` | the device, or each thermostat of the room |

A value the device already had is shown at once,
so only what the write changes stays open;
a wall thermostat is not compared by what it does not store
(the window-open time, descaling and valve limits).
Each device written is a write of its own:
not confirmed after the last re-read, its line names the device and the values
that differ.

The write's answer says `"confirmation": "pending"`,
or `confirmed` where each device shows it already.
While it is open, the devices that do not show it yet and their room have
`confirmation` with `state` `pending`;
after the deadline `unconfirmed`, until they show it or a newer write of the
same kind to them replaces it.
A device whose entry is out of date (`out_of_date`) shows nothing until the
Cube hears it again.
A newer configuration write replaces the values it sets of an older one.
At the deadline the log has one `unconfirmed` line at `warn` for the write,
naming each device that does not show it, what it shows, and its `error`,
`link_error` and `out_of_date`;
a device that shows it later has a line at `info` then.
There is no notification.
The writes of the LAN proxy's clients are confirmed the same way where their
radiogram decodes (docs/proxy.md).
It costs no radio time: the states come with the server's poll of the Cube.
The confirmations are kept in memory: a restart forgets them.

**A room setpoint sent once more.**
A room's setpoint of the API's (`PUT …/rooms/{id}/setpoint`)
that a thermostat of the room does not show at the deadline is sent once more,
the same room command (`0x40`), through the queue at low priority,
behind its holds for the duty cycle and the memory slots,
the room's thermostats with a communication error reset first,
as before every room command.
Auto with a temperature is sent as auto on the week programme
where a switch point of a thermostat's programme has passed since,
at the Cube's time.
While the re-send is open, the room's `confirmation` is `pending` again,
and the re-send is confirmed as the write was:
shown by every thermostat by its deadline,
a line at `info` says so and settles the write's warning;
else a warning names each thermostat that still does not show it,
and what it shows.
Nothing more is sent for it.
It is not sent where a newer setpoint to the room or to one of its devices
has been asked for since (`superseded`);
where a thermostat that showed it shows something else now,
a person's change passed on in the room or its window open (`changed`);
while a command of Massimo's to the room waits or runs in the queue (`busy`);
for a boost, which ends by itself (`boost`);
and where the room was sent a state once more within the day,
by this or by the room checks (`today`, see "The room checks").
`superseded` and `changed` are asked again when the queue's turn comes:
dropped then, nothing is sent and the room's day stays free.
A device's setpoint and a LAN proxy client's write are only reported,
as the proxy's clients' writes are left to them.
It costs one room command, about 3 % of the duty cycle and a memory slot
(docs/cube-protocol.md), once a day per room at most.

### DELETE /api/v1/cubes/{sn}/rooms/{id}

Removes an empty room by writing the metadata without it;
answers `{"ok": true, "rooms": 5}`.
A room that still has devices, hidden ones included, is refused with `409`,
and room 255 is not found.

### PUT /api/v1/cubes/{sn}/rooms/{id}

Rename a room: `{"name": "Küche"}`, up to 32 bytes.
A metadata write; answers `{"ok": true}`.

### PUT /api/v1/cubes/{sn}/devices/{rf}

Rename a device and/or move it to another room:
`{"name": "Thermostat Küche", "room_id": 2}`, either field alone is fine.
A move is done as the original software moved one in its capture of 2026-11-04:
after a wakeup of the old room, one `r:` for each other member whose error flag is set,
0x23 remove group,
one 0x21 remove link partner radiogram with the room flag naming the device,
which every member of the room takes,
addressed to the first of the room's order, as a room command is
(see `PUT …/rooms/{id}/setpoint`):
the room's master where it is a thermostat of the room,
else its wall thermostat, else its first radiator thermostat,
one the Cube holds no valid state for passed over,
one to each window contact among its partners,
and one to the device for each partner;
then, after a wakeup of the new room, the same with 0x20 add link partner
and 0x22 set group; then the metadata is written.
`room_id` must name an existing room; 0 is refused,
a device outside every room would be a device that blocks no room but cannot be
addressed by one either.

A device that is paired but in no room yet (`"unlisted": true`,
such as one just paired, see Pairing) needs `room_id`;
`name` alone answers `409`.
It is added to the room as the original software adds a new device:
after a wakeup of the room (`z:1e,G,<room>`)
and one `r:` for each member whose error flag is set,
one 0x20 add link partner radiogram with the room flag naming the device,
addressed as in a move, which every member of the room takes,
one to each window contact among its partners,
and one to the new device for each partner
(two shutter contacts are not linked with each other), then 0x22 set group,
then the metadata with the device listed under its name in the room,
then a re-read of its C line.
Without `name` it is listed under the default a repair's listing gives (D200-A):
`<room name> <type> <n>`, such as `Bath radiator thermostat 2`
beside `Bath radiator thermostat 1`,
and `<type> <n>` where that would pass 32 bytes ("A listed device's name",
under Repairs).
A device whose C line names another room (paired long ago and never listed) is
first unlinked from that room with 0x23 remove group and 0x21 remove link partner,
as in a move.

A thermostat put into a room, by an add or a move,
then gets the room's state and settings, in the order of the copy
the original software gives a device it pairs or moves (RoomMajority, Block 2):
0x40 with the room's state from before the change, to the device directly,
with an error reset first, and one more and a retry after a discard,
only where the device's flag is set:
its mode, setpoint and `until`, the state the room checks would send the room
again (RoomMajority.D8-A; "The room checks", below: Massimo's own setpoint,
what most of the room's thermostats show, the newest change,
the master's, the wall thermostat's, a radiator thermostat's),
read under the structure lock before the device is linked,
so that the new device's own state,
which may reach the room's thermostats first, does not count,
and held for the room checks;
where Massimo set the room another state since it was read,
as by `PUT …/rooms/{id}/setpoint` while the links were sent,
that setpoint, Massimo's own command coming first;
auto with temperature 0, the week programme, where the room's state is auto on
its programme, where it has none, or no other thermostat,
as the original software sends it always;
after a wakeup of the room,
0x11 to the device and to every thermostat of the room the Cube holds a valid state for,
with the room's comfort, eco, maximum, minimum and window-open temperatures
and window-open minutes, each keeping its own temperature offset:
one it holds none for, whose settings did not count, is not given the others';
after a wakeup,
0x12 with the room's boost, descaling and valve settings to the device;
after a wakeup, one 0x10 per day of the room's week programme that differs from
the device's own, then 0x11 to the device once more.
The room's settings are decided by the one rule that the report of members
that differ and the configuration's repair take too (RoomMajority.D9, D5-A),
each group on its own:
the temperatures, the valve settings and each day of the week programme
are what more than half of the room's other thermostats share,
else those of the first of the room's order that has them:
its master where it is a thermostat of the room, its wall thermostat,
its radiator thermostats in the Cube's order;
a thermostat the Cube holds no valid state for
(`reported` false or `out_of_date`) is passed over.
The window-open minutes and the valve settings,
which only radiator thermostats store in full, are decided among them;
without one, they are the wall thermostat's boost settings
and the original software's window-open minutes, descaling and valve limits.
The device keeps its own temperature offset and display mode.
A hidden device is neither linked with the device nor given anything,
nor taken for the room's state or settings.
A room without another thermostat the Cube holds a valid state for
has no settings to copy, and the device gets 0x40 alone;
a shutter contact or an eco switch gets nothing.
With `"copy": false` the device keeps its own state and settings.
The copy needs the capability `configure` as well as `structure`,
since its 0x11 changes the room's other thermostats too:
without it the request is refused with `403` before anything is sent,
unless the body says `"copy": false`, which skips the copy;
`name` alone never starts it.
The copy costs radio budget,
0x11 and 0x12 about 3 % each and 0x10 about 8 % per day,
so a factory-fresh thermostat takes about 65 % of an hour's budget with every day sent;
the queue waits for the budget as for every write.

The answer is

```json
{"ok": true, "copied": {"state": {"mode": "manual", "setpoint": 5}, "temperatures": ["0a010a", "0a0b0c"], "valve": true, "days": 5}, "budget": {…}}
```

with `copied` where the copy ran,
absent without it (`"copy": false`, a device that is no thermostat, a rename):
`state`, the state its 0x40 carried,
`mode` with `setpoint` (none for auto on the week programme)
and `until` (a holiday's end, as `PUT …/rooms/{id}/setpoint` takes it),
and `state_from`, `room` for the room's or Massimo's setpoint to it,
`default` where the room had none and the 0x40 carried auto on the programme;
`temperatures`, `valve` and `days`, what of the room's settings was sent;
`config_skipped` where none was and why:
`no_other_thermostat`, the room has no other thermostat,
or `no_valid_state`,
the Cube holds no valid state for any other thermostat of it,
as after its restart until they report;
`"confirmation": "pending"` where the device went into a room
(see "Structure changes read back", below),
and `"mode_not_set": true` in `copied` when the Cube discarded the 0x40,
which leaves the device's mode as it was and ends nothing:
on 2026-11-04 the Cube discarded a direct 0x40 to a device in a room.
The room's state then goes once more as a room command,
to the room's addressee after the programme (RoomMajority.D16-A),
with `"room_mode": true` in `copied` where the Cube took it,
which counts as Massimo's own setpoint to the room ("The room checks");
its discard ends nothing either,
and nothing is sent where the room has no state.
Any other command of the copy that fails ends it with its error (`502`,
`409` when superseded), which says that the device is in the room:
what was sent until then stays sent,
and `PUT …/rooms/{id}/config` and `PUT …/rooms/{id}/program` complete it.
The links and the metadata of one change of rooms and devices at a time
are written per Cube (this route, `DELETE …/devices/{rf}`,
`DELETE …/rooms/{id}`, `PUT …/rooms/{id}`):
a second request waits for the first's metadata, up to its time limit,
and then answers `503`; the copies run side by side afterwards.

### DELETE /api/v1/cubes/{sn}/devices/{rf}

Removes a device from the Cube, with the capability `structure`,
as the original software removed one on 2026-11-15.
A device in a room, the metadata's or else its C line's,
is unlinked from that room first:
after a wakeup of the room (`z:1e,G,<room>`),
one `r:` for each other member of the room whose error flag is set
(the queue resets the device itself where it is flagged),
then one 0x21 remove link partner radiogram with the room flag naming the device,
which every member of the room takes,
addressed to the first of the room's order, as a room command is
(see `PUT …/rooms/{id}/setpoint`):
the room's master where it is a thermostat of the room,
else its wall thermostat, else its first radiator thermostat,
one the Cube holds no valid state for passed over;
where the room's other members are window contacts only, each gets its own 0x21.
No 0x23 and no 0x21 go to the device itself: `t:` resets it.
Hidden devices are no partners,
and a device whose C line names room 255 is not unlinked at all.
Then `t:01,1,<rf>`, delete with the force flag,
which the Cube answers `A:` and which makes it send the device a factory reset;
then, for a device the metadata lists, the metadata without it.
The server's copy forgets the device at once:
it is gone from the devices, the rooms and the counts,
and the stream reports `metadata` (for a listed device) and `device_forgotten`.
The answer is `{"ok": true, "budget": {…}, "confirmation": "pending"}`.

**Structure changes read back.**
The Cube accepts a removal's, a placement's or a move's radiograms at once,
but the room's members take them at their next wakeup, about ten minutes on,
and the Cube holds a memory slot for each until then;
it reports its free slots only in `S:` replies and in the `H` line of a new
connection.
So such a change answers `"confirmation": "pending"`,
and the server reads it back 2, 6 and 12 minutes after it:
done once a budget the Cube reported after the change shows the free slots
back at their count before it;
at the last check, where the Cube reported none since,
the server reconnects once for a fresh `H` line
(only with `RefreshInterval` set, as the budget refresh after writes).
The outcome is the Cube's log event `structure` (see the log, below):
at `info` when done, at `warn` when not confirmed,
or when the change left a member of the rooms concerned with the error flag
or a link error it did not have before.

A device the metadata lists without a C line, a leftover,
loses its entry in the metadata, and nothing is sent by radio.

The answer is `{"ok": true, "budget": {…}}`.
A hidden device and an unknown address are `404`, the Cube itself `400`,
a Cube that is not connected `503`;
a write that fails is `502` with the Cube's answer, and stops the removal there:
one that stops after the unlink leaves the device in its room,
without links or group, and a second `DELETE` finishes it.
The device is paired again like a new one (Pairing).

The Cube drops a device only when it answers the reset:
a device out of range or out of battery stays in the Cube's memory,
and comes back, unlisted, with the server's next full read of the Cube
(a reconnect or the periodic refresh);
the Cube's device lists in between do not bring it back.
Seen on hardware for devices that never report (cube-protocol.md);
that one which answers is dropped is expected, not yet seen.
Such a device is parked in room 255 instead, and hidden (above).

### PUT /api/v1/cubes/{sn}/devices/{rf}/config

Write a thermostat's settings.
Every field is optional and merged into the device's current configuration:

```json
{"comfort_temp": 21, "eco_temp": 17, "max_setpoint": 30.5, "min_setpoint": 4.5,
 "temp_offset": -0.5, "window_open_temp": 12, "window_open_minutes": 15,
 "boost_minutes": 5, "boost_valve_pct": 80, "decalc_day": "saturday", "decalc_hour": 12,
 "max_valve_pct": 100, "valve_offset_pct": 0, "display_actual": true}
```

Temperatures 4.5 to 30.5 in steps of 0.5, the offset −3.5 to 3.5,
window minutes 0 to 60 in steps of 5, boost 0 to 30 in steps of 5 or 60,
boost valve 0 to 100 in steps of 5, hours 0 to 23, valve percentages 0 to 100;
`display_actual` is for wall thermostats only.
The temperature fields go as radiogram 0x11, the boost,
descaling and valve fields as 0x12, the display as 0x82,
each directly to the device after a wakeup of its room.
Wall thermostats take the same radiograms;
the values they do not store are filled in from a radiator thermostat of the room.
Shutter contacts and eco switches answer `400`.
The answer is the same as for a setpoint;
afterwards the server re-reads the device's configuration from the Cube
for the temperature, boost, descaling and valve fields.
The answer's `confirmation` says whether the device's configuration shows
what was written (see "Writes confirmed by the devices").
The display mode is no part of the configuration the Cube reports:
it is confirmed by the wall thermostat's state, `display_actual`.

### GET /api/v1/cubes/{sn}/rooms/{id}/config

The room's settings as the UI edits them:
the common fields above (without `temp_offset` and `display_actual`),
decided over the thermostats whose C line names the room by the one order
(RoomMajority.D9, D19-A),
as the copy of `PUT …/devices/{rf}`, the configuration's repair
and the room's differences (`mismatches`, `temperatures`) decide them,
each group on its own:
the five temperatures what more than half of the thermostats share,
else what the first of the room's order has
(its master where that is a thermostat of the room, else its wall thermostat,
else its first radiator thermostat, then each other one).
The boost, descaling and valve settings are one group,
decided so among the radiator thermostats alone, which alone store it all,
as are the window-open minutes;
a wall thermostat's boost settings count only in a room of wall thermostats alone,
with the original software's defaults for the rest.
A thermostat the Cube marks out of date is passed over while another is not;
while one of the room's thermostats has not reported since the Cube's restart
(`reported` false and not `out_of_date`),
as the room's differences wait for them all,
and where every one is out of date,
they are decided over them all,
so that the room's settings stay as they were while its thermostats report one by one.
`source` names the first thermostat of that order that stores the room's temperatures,
else the first of the order.
A `devices` list gives each thermostat's `rf_address`, `type`, `name`,
`temp_offset` and, for wall thermostats, `display_actual`;
a room without a thermostat has no `source` and an empty list.

### PUT /api/v1/cubes/{sn}/rooms/{id}/config

The same body as for a device, without `temp_offset` and `display_actual`,
applied to every thermostat of the room after one wakeup:
temperatures to radiator and wall thermostats, boost to both,
descaling and valve limits to radiator thermostats only.
The answer lists every device:

```json
{"ok": true, "devices": [{"rf_address": "0a0102", "name": "…", "ok": true, "discarded": false}], "budget": {…},
 "confirmation": "pending"}
```

`ok` is true when every device took the write; `502` only when none did.
`confirmation` is `pending` until each device that took it shows it in its
configuration (see "Writes confirmed by the devices").

### GET /api/v1/cubes/{sn}/rooms/{id}/program

The room's week programme, keyed by weekday as in a backup,
each day decided on its own as the configuration's groups are (above):
what more than half of the room's thermostats have,
else the first of the room's order's,
a thermostat out of date passed over as there (RoomMajority.D9, D19-A);
a programme written and not yet in a thermostat's configuration counts as written.
Plus `source`, the first thermostat of that order whose whole week is the room's,
else the first of the order;
its tag `etag` (also the `ETag` header), `pending` and `program_consistent`;
`404` for a room without a thermostat.
`GET …/devices/{rf}/program` gives one thermostat's own the same way,
under its room's tag;
a virtual wall thermostat's is its room's.

**The programme's tag.**
An editor that read a programme sends its tag back with its write,
in `If-Match` (quoted, `W/` taken, `*` for any),
and a programme changed since, in another client or at a device,
is refused with `412`, whose body and `ETag` carry the current tag,
instead of being overwritten unseen; nothing is sent then.
A write without `If-Match` is unconditional, as before.
The tag covers every thermostat's programme of the room,
since a write to the room or to one member changes the room:
it changes where a member's programme changes, though the room's stays,
and not where only who decides the room's programme changes
(a thermostat passed over or back, another master),
which no one made to a programme and which refuses no write.
A member's tag is its room's,
and `program_consistent` is false while the members' programmes differ.
A programme written and not yet in the devices' configurations,
which the server re-reads within minutes after a write,
counts as written: the GET shows it, `pending: true`, under its own tag,
so a write based on the copy from before it is `412` too.
It is pending until the configurations show it, or for ten minutes at most.
A write's answer carries the new tag, `"etag"`.
A device's `config` event says what its new configuration changed,
`"changed": ["program", "temps"]` (the presets, boost and valve settings),
and `etag` where the programme changed;
its room's virtual wall thermostat has the same `config` event,
and one with `"changed": ["settings"]` when its own settings change.

### PUT /api/v1/cubes/{sn}/rooms/{id}/program

Write the week programme of a whole room, any subset of days:

```json
{"monday": [{"temp": 17, "until": "07:00"}, {"temp": 21, "until": "20:30"}, {"temp": 17, "until": "24:00"}],
 "saturday": [{"temp": 19, "until": "24:00"}]}
```

Each day has 1 to 13 points with increasing times,
the last at 24:00 (`"0700"` and `"until_min": 420` are accepted as well).
One radiogram 0x10 per day (two for a day with more than 7 points) goes to the
room as a group command, which every thermostat of the room takes,
as the original software sends it.
`PUT …/devices/{rf}/program` writes the same body to one thermostat directly.
Both answer like a setpoint, with the programme's new `etag`;
`confirmation` is `pending` until each thermostat's configuration shows the days
(see "Writes confirmed by the devices").

### PUT /api/v1/cubes/{sn}/devices/{rf}/lock

Lock or unlock the buttons of one thermostat (the key lock,
"LoC" on the display, `panel_locked` in its state).
Body `{"locked": false}`; the answer is the same as for a setpoint,
its `confirmation` the device's `panel_locked`
(see "Writes confirmed by the devices").
The command is radiogram 0x80,
which the original software lists but never sends;
it is sent directly to the device after a wakeup of its room and takes effect
immediately (both directions verified on radiator thermostats).
On the device itself the lock is toggled by pressing Auto/Manu and the second
button (Boost or the moon/sun key,
Mode/Menu and Boost/OK on a wall thermostat) together.

### POST /api/v1/cubes/{sn}/devices/{rf}/battery

Marks new batteries in a device:
a device whose `battery` is `weak` or `low` and whose flag is off,
`battery_low` as the Cube shows it now, is `ok` at once,
rather than when the server's rules say so,
a low one an hour after its flag went (see the devices' `battery` above).
The body is `{"replaced": true}` and nothing else;
the route needs the capability `configure`, and sends nothing to the Cube.
The answer is the level, the one before, and whether it changed:

```json
{"battery": "ok", "from": "low", "changed": true}
```

A device that is `ok` already answers `{"battery": "ok", "from": "ok", "changed": false}`.
The change is a `battery` event in the log, as every change of a level is,
its `summary` with `cause` `replaced`, and `by` naming who marked the batteries,
beside the request's `change` line;
the server reads it back at a start with the device's states,
so that the level stays `ok` across a restart.
Later reports follow the rules of an `ok` device:
the flag seen again makes it `weak`, held for 15 minutes `low`.
`409` while the device's flag is on, since the device has not reported its new
batteries yet, and while the Cube holds no current report of the device,
since the Cube's restart or for some minutes (`reported` false):
the error says to wait for the device's next report.
`404` for a device the Cube does not have;
`400` for another body, a virtual wall thermostat and the Cube itself,
which have no batteries.

### Pairing: POST, GET, DELETE /api/v1/cubes/{sn}/pairing

`POST` with `{"seconds": 60}` (optional; 1 to 255,
default 60) puts the Cube into inclusion mode with `n:<seconds>`;
a device whose pairing button is pressed then joins.
255 seconds is the longest a Cube keeps the mode (docs/cube-protocol.md,
"Pairing a device, as the original software does it").
The Cube may end the mode earlier on its own.
The answer is the pairing state:

```json
{
  "active": true,
  "started": "2026-11-05T16:02:10+01:00",
  "until": "2026-11-05T16:03:10+01:00",
  "paired": []
}
```

`409` while pairing already runs, `503` while the Cube is not connected,
`400` for a bad `seconds`.
`GET` returns the same state at any time; after the end `active` is false,
`ended` says when,
and `paired` keeps the devices of the last pairing until the next one starts:

```json
{
  "active": false,
  "started": "2026-11-05T16:02:10+01:00",
  "until": "2026-11-05T16:03:10+01:00",
  "ended": "2026-11-05T16:02:41+01:00",
  "paired": [{"type": "radiator_thermostat", "rf_address": "0a0b0c", "serial": "TEQ0000001"}]
}
```

`DELETE` ends pairing with `x:` (also when the server counts it as over already,
so that a Cube still in the mode stops) and returns the state.
Pairing ends when the Cube says so (an empty `N` line), on `DELETE`,
or ten seconds after `until`, whichever is first.

The event stream reports `pairing_started`,
then a `paired` event per device that joins,
with `"device": "<rf>"` and `"paired": {"type": …, "rf_address": …, "serial": …}`,
and `pairing_ended`.
A joined device is in the device list right after (`GET …/devices`,
with `"unlisted": true` and no name;
the server fetches its configuration with `c:`) but in no room and not under `rooms`.
Put it into a room with `PUT …/devices/{rf}` and `{"name": "…", "room_id": N}` (above),
which also gives it the room's settings.
A client pairing a device therefore: `POST …/pairing`,
waits for the `paired` event (or polls `GET …/pairing`),
asks the user for a room and a name, `PUT …/devices/{rf}`,
and `DELETE …/pairing` if more time is left and no further device is to come.
Pairing through the LAN proxy, as the original software does it,
uses the same mode (proxy.md).

### GET /api/v1/cubes/{sn}/backup?format=yaml|json

A backup file as `massimo backup` writes it: rooms,
devices with configuration and state, Cube settings, NTP servers,
plus the raw protocol lines.
YAML comes as an attachment named `massimo-backup-<serial>-<time>.yaml`.

### Backups

The server keeps a history of each Cube's backups in its store
(docs/store.md section 9 (e)), gzip, a few kilobytes each.
It takes one by itself after a change of the Cube's configuration (`auto`):
the rooms, names, device settings and week programmes,
not the live state and not the NTP servers,
compared with the newest backup's by a hash;
the backup follows once `backups.auto_min` minutes passed without a further change,
at the latest `backups.auto_max` minutes after the first,
and a Cube that has none gets one at once:
at its first connection, or once its last backup is deleted or pruned.
The windows hold across a start of the server, `changed` and `due` with them;
a clean stop takes the backup of a change pending before the Cubes close.
Changes made outside Massimo count as well, once the Cube reports them,
at the latest at the six-hourly refresh.
A restore keeps the Cube's configuration before its first step (`before_restore`)
and the file it was sent (`upload`);
a backup taken by hand is `manual`.
The settings of the section Settings decide how long they are kept:
a Cube's newest `backups.keep_newest` whatever their age,
of the others an automatic one for `backups.keep_days_auto` days and the rest for
`backups.keep_days_manual`.
A Cube without a row in the store (`serve -addr`) has no history:
these routes answer `503`, and `GET …/backup` works as before.

A Cube the store keeps a row of and no runtime serves keeps its backups in reach:
one taken out of the configuration, or one only found by discovery.
`GET …/backups` lists them, without `changed` and `due`;
`GET …/backups/{id}`, its `diff` and `DELETE` work as for a served Cube,
the comparison leaving out the devices its row hides.
It takes no backup and no restore:
`POST …/backups` and the restore's plan and restore answer `409`,
and `GET …/backup` stays `404`.
Such a backup goes onto another Cube as a file:
downloaded, then sent to that Cube's `POST …/restore/plan` and `POST …/restore`.
A serial with neither a runtime nor a row is `404`.
`GET /api/v1/backups/unserved` lists such Cubes (below),
also one that discovery does not hear;
`GET /api/v1/cubes` counts each Cube's backups kept,
served or found (`backups`).

#### GET /api/v1/cubes/{sn}/backups

The history, newest first, and,
while the configuration differs from the newest backup's,
when it last changed and when the automatic backup follows:

```json
{
  "backups": [
    {"id": 12, "taken_at": "2026-11-07T09:14:03.120Z", "reason": "manual", "format": "json",
     "tool": "v0.10.0", "size": 23994, "sha256": "4c1f…", "user": "owner", "note": "before the holidays"},
    {"id": 11, "taken_at": "2026-11-06T18:02:44.871Z", "reason": "auto", "format": "json",
     "tool": "v0.10.0", "size": 23990, "sha256": "9ab0…"}
  ],
  "changed": "2026-11-07T09:30:12Z",
  "due": "2026-11-07T10:30:12Z"
}
```

`size` is the file's, before the store compresses it; `user` is absent for the
server's own backups.
A Cube that is not served has no `changed` or `due` (above).
`id` names one backup for good:
the store never gives it to another, also once the backup is deleted or pruned.
Capability `backup`.

#### POST /api/v1/cubes/{sn}/backups

Take a backup now, as `GET …/backup` takes it,
and keep it as `manual` with the user and an optional note,
`{"note": "before the holidays"}` (at most 200 characters;
another field answers `400`).
`201` with the backup as the list shows it;
`503` while the Cube is not connected,
`409` for a Cube the store keeps a row of and no runtime serves.
Capability `backup`, audited as `backup.take`.

#### GET /api/v1/cubes/{sn}/backups/{id}?format=json|yaml

The backup's file as an attachment named `massimo-backup-<serial>-<time>.<format>`,
as it was stored, or converted to the format asked for.
`404` for an id the Cube does not have.
Capability `backup`.

#### DELETE /api/v1/cubes/{sn}/backups/{id}

Remove a backup from the history (`204`);
the store keeps a tombstone without the file until the retention removes it.
Deleting the newest makes the one now newest the reference of the automatic backup:
from the next look, within a minute,
`changed` and `due` show a configuration that differs from it.
Deleting the last has the server take a new one at once.
Capability `restore`, so that an operator cannot erase the history;
audited as `backup.delete`.

#### GET /api/v1/cubes/{sn}/backups/{id}/diff?against={id}

What differs between two stored backups, as the restore planner sees it:
the steps a restore of `{id}` would send to the Cube as `against` recorded it
(its raw lines and NTP servers), read-only:

```json
{
  "steps": [
    {"what": "write room and device names (room 6: \"Room 6\"→\"Bedroom\")", "radio": false},
    {"what": "0abcde \"Radiator 1\": temperatures comfort 21.5 eco 17.5 …, program saturday", "radio": true, "device": "0abcde", "room": 6}
  ],
  "notes": [],
  "radio_commands": 2
}
```

No steps when the two hold the same configuration.
The restore's options apply (`skip=clock`, `state`, `portal`),
and hidden devices are not compared.
`400` without `against`, `404` for an id the Cube does not have.
Capability `backup`.

#### GET /api/v1/backups/unserved

The Cubes the store keeps backups of and no runtime serves, by serial:
each row that is not removed and keeps at least one backup,
one taken out of the configuration or one only found by discovery,
also while discovery does not hear it and `GET /api/v1/cubes` does not list it:

```json
[
  {"serial": "KEQ0000000", "name": "Old hall", "backups": 3, "newest": "2026-11-09T10:00:00Z"}
]
```

`name` is the row's, absent where it has none;
`backups` counts the backups kept, `newest` is when the newest was taken.
Their backups are listed, downloaded, compared and deleted through
`GET …/backups` and its routes (above).
The served Cubes are not in it, and a server without a store lists none (`[]`).
Web UI: Maintenance → Cube backups draws a fold for each.
Capability `backup`.

### POST /api/v1/cubes/{sn}/restore/plan

What restoring a backup would send, the dry run of `massimo restore`.
The body is the backup file, which may be edited by hand:
JSON with `Content-Type: application/json`,
YAML with `application/yaml` or `text/yaml`;
a body that does not start with `{` is read as YAML whatever the header says.
At most 1 MB.
`backup=<id>` takes a stored backup (see Backups) in place of the body,
which is then left empty; `404` for an id the Cube does not have.
The options are query parameters, the flags of the command:
`portal` also restores the portal URL and whether the Cube polls it,
`state` also re-applies each thermostat's mode and setpoint from the backup,
both off unless given (`?portal`, `?portal=true`);
`skip=names,settings,programs,clock` leaves those groups alone.
Nothing is written.

```json
{
  "plan": "558237da9e11cf498b34896fa99b7a4ebe3d05ff163a197e37a76f19b79fd804",
  "steps": [
    {"what": "write room and device names (room 6: \"Bedroom\"→\"Guest room\")", "radio": false},
    {"what": "0a010d \"Radiator Bedroom\": temperatures comfort 21.5 eco 17.0 max 30.5 min 4.5 offset +0.0 window 12.0/15min",
     "radio": true, "device": "0a010d", "room": 6}
  ],
  "notes": ["device 123456 \"New\" (radiator_thermostat, MEQ0000000) is not paired with this cube; pair it and restore again"],
  "radio_commands": 1,
  "duty_cost_pct": 1.25,
  "budget": {"duty_cycle_pct": 4, "free_memory_slots": 50, "updated_at": "…"}
}
```

`steps` are the commands in the order they would be sent,
each with `what` as the command prints it,
`radio` when it sends radio commands (which cost budget;
a step without is a Cube command such as names, NTP servers or time zone rules),
and the device it addresses and the room woken up before it where the command has them.
`notes` are the differences a restore cannot write:
devices in the backup that are not paired (pairing cannot be restored over the LAN),
devices on the Cube that are not in the backup (left alone),
a configuration the Cube has not sent yet.
`radio_commands` counts the radiograms of all steps and `duty_cost_pct` is about
what they will cost of the hourly radio budget,
1.25 % each as measured on hardware (a move of a device to another room is several);
`budget` is what the Cube last reported,
so a client can show the cost against it.
`plan` is a SHA-256 over the step list (the texts and the command lines);
`POST …/restore` takes it and refuses to send when the plan it computes then differs,
so what is sent is what was shown.
An empty `steps` means the Cube matches the backup.

The clock group reads the Cube's NTP servers first (an `f:` through the queue,
ahead of queued work); when that fails,
`notes` says so and the servers are not compared.
Hidden devices (room 255 and `MASSIMO_CUBE_<SN>_IGNORE`) are neither compared nor written,
and the names step keeps them, and the rooms they are in, as the Cube has them,
unless the server runs with `-show-hidden`.

`400` for a backup the server cannot read, one from another Cube or
without a serial (`cube.serial` must be this Cube's), a bad option or
an unknown `skip` group; `413` for a body over 1 MB; `503` while the
Cube is not connected;
`409` for a Cube the store keeps a row of and no runtime serves (see Backups).

### POST /api/v1/cubes/{sn}/restore?plan=<hash>

Send the plan: the same body and options as for the plan,
plus the hash it answered.
The server computes the plan again and, when the hash is the same,
answers `202` at once and sends the steps one after the other through the Cube's
queue at low priority, so that setpoints and other user commands go first:

```json
{"id": "f0eb2372c18a88c5", "steps": 2}
```

Each step is reported on the event stream as `restore_step` and the end as
`restore_done` (below), and `GET …/restore` shows the state at any time.
As the command, the restore stops at the first step the Cube refused or that failed;
the steps after it are not sent and the state is `failed`.
After radio writes the server follows up the devices' configurations as after
`PUT …/devices/{rf}/config`.
A Cube with a row in the store first keeps a backup of its configuration
(`before_restore`; the restore is refused with `503` when it cannot be stored)
and the file of the body (`upload`), both with the user;
a stored backup restored by `backup=<id>` is not stored again.
After the restore the file restored is what the next automatic backup compares
with, so a restore that wrote everything leaves nothing to back up.

Refused with `409` while a restore is running on this Cube,
when the plan has changed since it was made (another client wrote to the Cube,
or the backup or the options differ from the plan request;
the message has both hashes, plan again),
and when the plan has radio commands while the Cube's duty cycle is above 80 %,
the message saying the percentage (the write test on hardware ran up to there;
a restore without radio commands, names and clock only,
is sent whatever the duty cycle);
`400` without `plan` and for everything the plan request refuses with `400`;
`409`, `413` and `503` as for the plan.
The Cube changes a device's configuration only once the device acknowledged the
write (one to four minutes),
so a plan made right after a restore still lists the radio steps until the
follow-up re-read saw the change.

### GET /api/v1/cubes/{sn}/restore

The last restore of the Cube since the server started:

```json
{
  "id": "f0eb2372c18a88c5",
  "state": "running",
  "plan": "558237da9e11cf498b34896fa99b7a4ebe3d05ff163a197e37a76f19b79fd804",
  "started": "2026-11-05T21:19:32+01:00",
  "steps": [
    {"what": "write room and device names (…)", "radio": false,
     "result": {"ok": true, "budget": {…}, "time": "2026-11-05T21:19:32+01:00"}},
    {"what": "0a010d \"Radiator Bedroom\": temperatures …", "radio": true, "device": "0a010d", "room": 6}
  ],
  "sent": 1,
  "radio_commands": 1
}
```

`state` is `running`, `done` or `failed`;
`{"state": "none"}` before the first restore.
`steps` are the plan's, each with `result` once it ran: `ok`,
`discarded` when the Cube refused the radiogram, `error`,
the `budget` the Cube reported after it and the `time`.
`sent` counts the steps the Cube accepted; `finished` and, for a failed restore,
`error` (which step and why) come with the end.

The event stream reports each step and the end:

```
event: restore_step
data: {"kind":"restore_step","device":"0a010d","restore":{"id":"f0eb2372c18a88c5","step":2,"steps":2,"what":"0a010d \"Radiator Bedroom\": temperatures …","ok":true},"time":"…"}

event: restore_done
data: {"kind":"restore_done","restore":{"id":"f0eb2372c18a88c5","steps":2,"ok":true,"state":"done","sent":2},"time":"…"}
```

`restore.step` counts from 1 in the order of `steps`;
a failed step has `ok` false, `discarded` when the Cube refused it, and `error`,
also as the event's `error`.
`restore_done` carries `state` (`done` or `failed`), `sent` and, when failed,
`error`.
Both are in `GET …/log`.

### Repairs: GET and POST …/repair/…

The repairs of a Cube's rooms and devices, with the capability `repair`
(the owner's and the manager's; AP RadioHealth, D163-A).
Each has a route whose `GET` is its preview and whose `POST ?plan=<hash>` runs it.
The preview sends nothing:
it answers the lines a run would send, command by command,
and what they cost:

```json
{
  "action": "relink",
  "plan": "7f3c…",
  "steps": [
    {"what": "link 0a010a with its partners in room 1", "device": "0a010a", "room": 1,
     "lines": ["s:…", "s:…", "s:…", "s:…", "s:…"]}
  ],
  "radiograms": 14, "duty_cost_pct": 17.5, "slots": 20, "held_slots": 6,
  "resets": 0, "wakeups": 3, "metadata": false,
  "budget": {"duty_cycle_pct": 4, "free_memory_slots": 50, "updated_at": "…"},
  "notes": []
}
```

`steps` are the commands in the order they would be sent:
each with the device it addresses,
whether the queue resets error flags first (`reset`, the `r:` not among
`lines`): the device's, the flagged members its room-flagged lines reach,
the flagged devices its direct lines address, as the queue's prelude does
for every command (D204-A),
the room woken up before it (`z:1e,G,<room>`, not among `lines`),
and its `lines` as they go to the Cube;
a metadata write is a step of its own.
`radiograms` counts the `s:` lines,
`duty_cost_pct` is about what they cost of the hourly radio budget (1.25 % each),
`slots` the memory slots they take, one each,
a radiogram with the room flag one per member of the room (T134),
and `held_slots` those that stay taken until the members' next wakeup,
about ten minutes: a room radiogram's but its addressee's.
`resets` counts the `r:` lines, those the queue sends before each step included,
by the device list as it is now, which cost no radio time;
`wakeups` the room wakeups; `metadata` a metadata write, no radio.
`budget` is what the Cube last reported.
`untested` marks a repair not yet run on hardware.
`refused` says why a run would be refused now:
the duty cycle with the repair's cost would reach the queue's hold (95 %),
or its held slots would leave fewer free than the queue waits for (5);
either would stop it halfway.
`details` names what the repair concerns, per repair (below).
A repair with nothing to send now has a preview without `plan` and
`steps`, and `nothing` says why, as the table names it;
its run is refused with `409` and that message.
A listing into a room the Cube's list does not have says so with
`details` `{"code": "room_name_needed", "room": 7}`, and its run's `409`
carries the same `code` and `room`.

The run takes the preview's `plan`:
the repair is built again from the Cube as it is then,
and a different one is refused with `409` (both hashes in the message;
preview it again), as is one that `refused` names.
Its answer:

```json
{"ok": true, "action": "relink", "confirmation": "pending",
 "steps": [{"what": "link 0a010a with its partners in room 1", "lines": 5, "accepted": 5}],
 "budget": {…}}
```

A run is judged by what the Cube shows:
each step's lines `accepted`;
where it sent radiograms, the structure read-back of
`DELETE …/devices/{rf}` (`"confirmation": "pending"`, the log's
**device change** line once the Cube's free slots are back at their count before
it, or naming the slots still held and the members it left with an error flag
or a link error they did not have);
for resets, the device list read about two seconds after them:
`still_flagged` names the devices whose flag it still shows.
A step the Cube discarded or that failed stops the run there:
`502` (`409` when superseded) with `error` naming the step and the same
`steps`, so that a client sees how far it got.
Room repairs and placements run under the Cube's structure lock,
as moves and removals do.
A room is its members by their C lines, the Cube's own idea of it.
`400` for a bad device or room, a virtual wall thermostat
(repair its room) and the Cube itself; `404` for a hidden or unknown one;
`503` while the Cube is not connected.

| Repair | What a run sends | Its cost |
|---|---|---|
| `…/cubes/{sn}/repair/reset` | one `r:01,<rf>` per device of the Cube whose error flag (byte 5's 0x08, the communication error) is set, the hidden ones left out; nothing where none is | none: `r:` is the Cube's bookkeeping |
| `…/cubes/{sn}/rooms/{id}/repair/reset` | the same for the devices whose C line names the room | none |
| `…/cubes/{sn}/rooms/{id}/repair/master` | the metadata, with the room's wall thermostat as its master, else its first radiator thermostat, one the Cube holds no valid state for passed over while another is not (the room's order without its master, RoomMajority.D9, in place of DeviceUnPairingFix.D167), where the metadata's master is no thermostat whose C line names the room; nothing where it is one, or the room has none | none: metadata only |
| `…/cubes/{sn}/rooms/{id}/repair/relink` | for each member that takes part in links (thermostats and window contacts), a command of its own after a wakeup of the room: one 0x20 add link partner radiogram with the room flag naming it, to the room's addressee (the first of the room's order, RoomMajority.D9: the master where it is a thermostat of the room and not the member, else its wall thermostat, else its first radiator thermostat, one the Cube holds no valid state for passed over), one to each window contact among its partners, one to the member for each partner, then 0x22, its group; the first resets the room's other flagged members. Nothing is unlinked. Nothing to send where the room's list and its C lines disagree (place or list the devices first) or fewer than two members take part in links | a few radiograms per member: a room of a radiator, a wall thermostat and a window contact is 14, about 17.5 % |
| `…/cubes/{sn}/rooms/{id}/repair/unlink-relink` | "unlink all and re-link" (D163-B), with `structure` as well as `repair`, its preview too: for each member that takes part in links, a command of its own after a wakeup of the room, the 0x21 remove link partner radiograms of a move's unlink half without its 0x23 (one with the room flag naming it to the room's addressee, one to each window contact among its partners, one to the member for each partner), the room's flagged members reset in the first; then the re-link of every member, as above, whose 0x22 sets each group again. **Not yet run on hardware**: its preview says so (`untested`, a note), and a run that stops halfway, at a discarded radiogram or a lost connection, leaves the room's devices without their partners until it is run again. Nothing to send as the re-link | about twice a re-link's: 25 radiograms, about 31 %, for the room above; a room of five members far more, which the duty-cycle refusal may stop before it starts |
| `…/cubes/{sn}/devices/{rf}/repair/config` | after a wakeup of the room, the days of the room's week programme that differ from the device's (0x10), then 0x11 with the room's temperatures (comfort, eco, maximum, minimum, window open), the device's own offset and window-open minutes kept; its C line read again. The room's are those the report of members that differ names (`programme`, `temperatures`; RoomMajority.D9, the copy's one rule), by the C lines of every thermostat of the room, the device's included: each day and the temperatures what more than half of them share, else what the first of the room's order has, one the Cube holds no valid state for passed over. Nothing to send where the room has no other thermostat the Cube holds a valid state for (`nothing` says how many it passed over), the device has the room's configuration already, or the Cube and its list have it in different rooms | one or two radiograms per day, one 0x11 |
| `…/cubes/{sn}/devices/{rf}/repair/place` | where the device's C line names another room than the metadata, or none: a move into the metadata's room (`PUT …/devices/{rf}` with `room_id`, without the copy), from none the placement's link half alone; its C line read again. The metadata stays. A thermostat's run holds the room's state from before it for the room checks, as an add does (RoomMajority.D15-A; "The room checks"). Nothing to send where the list does not list the device, has it in no room or where the Cube has it, or the room has a wall thermostat already | about 8 % for a move |
| `…/cubes/{sn}/devices/{rf}/repair/list` | the metadata, the device's entry moved to the room its C line names, or one made for a device the list does not have, named by `name` (a query parameter) or by default (below); a room the list does not have is made with `room_name` (a query parameter), its master the room's wall thermostat, else its first radiator thermostat, as the repair of a room's master names it, and asked for (above) where it is not given. Nothing to send where the Cube has the device in no room, or its list where the Cube has it | none: metadata only |

`details` holds, per repair:
`devices`, those reset;
`room`, `master` and `new_master`;
for the configuration `room`, `others` (the room's other thermostats
the Cube holds a valid state for), `thermostats` (those the room's
configuration was decided over, the device among them
where the Cube holds a valid state for it),
`temperatures_by` and `program_by` (how many of those share what is re-sent,
for the programme the fewest of any day),
`temperatures` and `days` (what is re-sent) and `agreed_temperatures`;
for a placement `from` and `to`;
for a listing `from`, `to`, `listed` and `new_room`,
and for a device the list lacks `name`, the name its entry gets,
`room_name`, the room's, and `type`, the device's,
so that a client can offer a name of its own words.

**A listed device's name** (D185):
a device the list lacks gets the name the query's `name` gives,
checked as every name is (1 to 32 bytes, else `400`),
else the default `<room name> <type> <n>`:
the room's name, the one it is listed into or the new room's `room_name`;
the device type in English (`radiator thermostat`, `radiator thermostat plus`,
`wall thermostat`, `window contact`, `eco button`);
`n` the lowest number from 1 that no device the list has in that room
takes in a name of that form, whatever its case,
such as `Bath radiator thermostat 2` beside `Bath radiator thermostat 1`.
Where the whole would pass 32 bytes, nobody being there to shorten it,
the room's name is dropped (D201):
`<type> <n>`, `n` the lowest free in that form,
such as `radiator thermostat 1` for a room `Wohnzimmer Süd`.
A device the list has keeps its name, `name` or not.
The web UI offers the same default with the type in the page's language,
one past 32 bytes whole, which its user shortens before it is sent.
The log's **device change** names a re-link `relink`, "unlink all and re-link"
`unlink_relink`, a re-sent configuration `config` and a placement `move`;
one of a room has no device and names its `rooms`.

### GET /api/v1/cubes/{sn}/log?after=&before=&since=&limit=&kind=&source=&level=&device=&user=&command=

A page of the Cube's events, oldest first,
the same objects the event stream sends (`kind`, `device`, `state`, `error`,
`changes`, `time`, …), for a log view that starts with history and continues
with the stream:

```json
[
  {"id": 4711, "kind": "connected", "time": "2026-11-06T20:00:00.120345+01:00"},
  {"id": 4712, "kind": "state", "device": "0abcde", "state": {"rf_address": "0abcde", "name": "Radiator 1", …}, "time": "…"}
]
```

The server keeps the log in its store, `massimo.db`,
so it survives a restart and an update,
for the days `log.keep_days` says (`GET`/`PUT /api/v1/settings`, 30 by default,
0 for ever); an event is in it within a second of the stream sending it,
and a read writes what is pending first.
Each event carries `id`, its place in the log, which the stream's events do not.
The prune keeps the log's newest event, whatever its age,
until the first prune after a newer one is written,
so that no `id` is given out again after a prune.
Without parameters the answer is the newest 200 events; `limit` takes 1 to 1000,
`before` an event's `id` for the page before it (`?before=4711`),
or an RFC 3339 time for the events before that time;
`after` an event's `id` for the page after it (`?after=4711`),
the first `limit` events after that one, oldest first,
and `0` for the log's start;
`since` an RFC 3339 time for the events at or after it
(`?since=2026-11-06T20:00:00Z`, not after a `before` time),
so that a view of the last hour or of today is one request,
with `before` as the cursor within it;
`since` is the start of a range, not a cursor:
its page is the newest `limit` events since the time,
a client that asks again since the newest time it has
gets that millisecond's events again,
and an event stamped before that time but kept after the client's last read
in no page at all;
`kind` keeps one kind (`state`, `config`, `connected`,
…) and `device` one device, by its serial number or its radio address,
also one the Cube no longer has.
`source` keeps one source: `device` (a device's), `cube` (the Cube's own:
its connection, configuration, rooms, pairing, a Cube found on the LAN),
`system` (the server's own jobs), `config` (a change of the configuration),
`queue` (a command's steps on the radio, its holds), `server` (the
sign-ins) or `proxy` (the LAN proxy's clients).
`level` keeps that level and above, `debug`, `info`, `warn` or `error`;
`info` by default, so that a client asks for `level=debug` to see the
debug lines (the Cube's `hello`).
`user` keeps the events a user started, by the login name,
started with its rights or by it,
a removed user's too, under the name its removal gave it;
a name the server does not know answers an empty page.
`command` keeps the events of one command, by its id
(`GET /api/v1/commands/{id}`):
its attempt, a discard and a retry, its outcome.
An event of a command names it in `command`,
until `commands.keep_days` prunes the command.
The events only a user with `audit` sees (sign-ins, lockouts)
are on such a user's pages alone.
The page holds the server's own events too, kept once with no Cube,
in every Cube's log.
Each event says its `source` and its `level`,
and `by` names who started it, as the user is named now
(`{"name": "anna", "display_name": "Anna"}`),
with `effective_user` and `real_user` as the audit names them
(GET /api/v1/audit, "Who did it"):
absent for the Cube, the server's own jobs and a server without users;
the bootstrap token is a key of the first owner, whom it names, as the audit does.
A client pages back with `before` set to the `id` of the oldest event it has,
until a page is empty: the events of a burst share a millisecond,
so a time does not page them without gaps or repeats.
A client follows the log with `after` set to the `id` of the newest event it has,
on a server with the feature `log_after` (`GET /healthz`;
an older server ignores `after` and answers the newest page):
each event comes once, in the order the log kept them,
also where a burst brings more than a page;
a full page means more may follow,
and an empty one leaves the cursor where it was.
`after` goes with every other parameter, `before` and `since` included,
as further bounds: `?after=4711&before=4800` is the events between the two.
A cursor beyond the log's newest event, one the log never gave out,
answers `409` with an error saying to reload the log;
the client then reads it again without `after`.
That is a store restored from an older copy;
a server without a store that restarted with its clock set back,
which answers `409` to a cursor from before the restart until the client reloads;
and the cursor of a Cube's events kept in memory once the Cube has its row (below).
The prune makes none, since it keeps the log's newest event.
A cursor older than what the log keeps,
by `log.keep_days` or the 500 kept in memory,
gets no sign: the page starts at the oldest event kept.
The stored copy of a `state` event leaves out `state.received`,
the per-value times the tiles' ages read (`GET …/devices/{rf}` has them);
the event's own `time` is the log's.
A Cube served for this run only, without a row in the store (`serve -addr`,
or `-sn` before discovery noted it), keeps its last 500 events in memory,
as before the store.
Each of them has an `id` too, from a counter of the server's
that starts at the server's start in microseconds,
far above the store's ids,
so that the ids grow across a restart and `after` reads the new run from its start;
`before`, `after` and `since` work there as in the store.
Such a Cube that gains its row while it runs, as discovery notes it,
answers from the store from then on,
and the events it kept in memory leave the log.
A device hidden later keeps its events in the log until `log.keep_days` removes them.
A page before a time first looks up the newest event before it,
which reads the Cube's times up to there: an `id` is the quicker cursor.
A page since a time looks up the oldest event at or after it the same way.
`400` for a bad `after`, `before`, `limit`, `source`,
`level`, `device` or `command`;
`409` for an `after` beyond the newest event:

```json
{"error": "after 4711 is beyond the log's newest line: reload the log"}
```

### GET /api/v1/log?after=&before=&since=&limit=&kind=&source=&level=&device=&user=&command=

Every Cube's log at once, for a server with more than one Cube:
the same parameters and the same events as a Cube's log,
each Cube's event with the Cube's serial number in `cube_serial`,
and the server's own once, without it (`read`, as a Cube's log):

```json
[
  {"id": 4711, "kind": "connected", "cube_serial": "KEQ0000100", "time": "…"},
  {"id": 4712, "kind": "update", "source": "system", "time": "…"},
  {"id": 4713, "kind": "state", "cube_serial": "KEQ0000200", "device": "0abcde", "serial": "MEQ0000101", …}
]
```

Pages go back by `id` across the Cubes, and forward with `after`,
as in a Cube's log, `409` included.
A serial number as `device` names one device;
a radio address names the device of that address on every Cube,
since each Cube gives out its own.
A Cube without a row in the store yet adds nothing.
Without a store, each Cube's last 500 events, merged by `id`,
which every Cube's events take from the server's one counter,
in the order the server kept them;
the server's own events are kept by each Cube with an `id` of its own,
and come once, with the first Cube's.
A Cube's own log does not carry `cube_serial`.

### GET /api/v1/log/alerts?after=

The log's warnings and errors that nothing has settled yet,
of every Cube and the server's own (`read`), newest first,
and the worst of them, for the web UI's log button:

```json
{
  "level": "error",
  "alerts": [
    {"id": 4720, "kind": "state", "level": "warn", "cube_serial": "KEQ0000100", "device": "0a0101", "serial": "MEQ0000101", "flag": "error",
     "failed_radiogram": {"code": 3, "name": "TimeInformation"}, "time": "…"},
    {"id": 4715, "kind": "restart", "level": "error", "time": "…"}
  ]
}
```

A warning or an error is settled by a later event that says it is over,
of the same Cube, or of the server for the server's own:

| Kind | Settled by |
|------|------------|
| `disconnected`, also one the link held | the next `connected` |
| `command` retried, failed or dropped | the same command key acknowledged for the same device |
| `hold` | its end: the duty cycle's reset, or the wait for free slots |
| `backup`, `restore_done`, `update_result`, `log_rotated` failed | the next one of its kind below warn |
| `settings_read` with a value the next start would refuse, a file that does not parse or is there but cannot be read, or a note on the file | the next `settings_read`, also one at `warn`, whose line holds all that its read found, so that the newest is the alert; or `server_started` (a start's own notes follow it, as a `settings_read` of their own) |
| `proxy_refused` | the Cube's `connected`, or `proxy_connected` |
| `proxy_warning`, a pairing not stopped or not started | `pairing_ended`; `pairing_started` or `proxy_pairing` |
| `disagree`, a room's thermostats differing or its state not sent again | a `disagree` of the same `room` whose `outcome` is `agree` |
| `drift`, a thermostat off its week programme | a `drift` of the same device whose `outcome` is `follows` or `left_auto` |
| `login_failed` | a `login` of the same name |
| `locked_out` | its end |
| `restart`, `reboot`, `update` not started | `server_started`; for `update`, also an `update` started |
| `error_watch` `back` or `lasting` | the same device's `error_watch` `clear` of the same `flag` |
| `unconfirmed`, a room's setpoint or its re-send not confirmed, its re-send failed | an `unconfirmed` of the same `target` whose `outcome` is `resend_confirmed` |

Any other, such as `proxy_discarded`, `proxy_delete_refused` and `open_setup`,
is settled only by a person's mark.
The alerts the signed-in user marked seen are left out:
the preference `log.seen` (see "Preferences"),
which the web UI writes and opening the log does not;
a browser without a user keeps its marks itself and asks with `after`.
A device's communication error, link error and low battery are in the log at info,
in its state's changes:
each is an alert at warn while the device has it,
at the state line that set it (`flag`),
the `error` flag's with a `failed_radiogram`:
of the radiograms that failed while the flag stayed set,
those of the device's state lines from the one that set it on
and the one its state gives now,
the most severe, and of two as severe the oldest.
The severity is the original software's category of the radiogram's type
(docs/cube-protocol.md, "FrameType"), highest first:
Assignment and Remove (0x20 to 0x23, 0xF0), which change the system's structure;
Configuration (0x10 to 0x12, 0x82);
Control (0x40, 0x41, 0x43, 0x44, 0x80);
then the rest (the time push 0x03, the wakeup 0xF1, a code the list lacks).
So a flag set by the Cube's time push (0x03) at 10:00
and still set when a setpoint (0x40) fails at 14:00 names the setpoint's,
and a removal's link (0x21) failing after that names the link's.
`after` leaves out the alerts up to that id.
The audit's lines (`login_failed`, `locked_out`)
are there for a holder of `audit` only.
A Cube without a row in the store yet adds nothing;
without a store, there are none.

### GET /api/v1/cubes/{sn}/events

A `text/event-stream`.
On connect, one `state` event per device with its current data,
the devices of the view `devices=` (see "The virtual wall thermostat"),
whose events alone the stream then carries;
afterwards an event whenever the Cube reports a change:

```
event: state
data: {"kind":"state","device":"0a0102","state":{…},"time":"2026-11-04T15:30:12+01:00"}

event: disconnected
data: {"kind":"disconnected","error":"read tcp …: i/o timeout","time":"…"}
```

Kinds: `connected`,
`disconnected` (once per loss: a connection that had its greeting ends,
or the first attempt fails after the start or after a reconnect;
the server retries by itself, without an event for each attempt,
and sends `connected` when the Cube is back),
`hello` (new budget or clock from the Cube),
`metadata` (rooms or names changed), `config` (a device configuration arrived),
`state`, `battery` (a device's battery level changed, see the devices above;
with `cause` `replaced` in its `summary` and who in `by` where someone marked
new batteries),
`wall_reset` (a wall thermostat came back after five minutes or more without a
state on another mode than it had, as after new batteries, at warn: its
`summary` has the `mode` and the `was_mode`, the setpoints and `silent_min`),
`error_reset` (the server reset a device's communication error,
its `error` flag, with one `r:`, which costs no radio time,
because the device reported while flagged:
at most once a day per device,
never while a command to the device or its room waits or runs in the queue,
at info;
its `summary` has `automatic`: `true` and the `room` of the device's C line),
`error_watch` (a device's flags watched, its `summary`'s `what` saying which:
`back`, at warn, once a day per device, where its communication error is back
within a day of its automatic `error_reset`,
in an entry the Cube sent after it answered the reset,
at the device's report or set again after it was clear,
with `reset`, the reset's time, and `minutes` since;
`next_report` where it was the device's first report after it;
`failed_radiogram`;
and `held`: `true`, the server resets the device by itself no more
until its flag has stayed clear for a day;
`lasting`, at error, once in each run of the flag,
where the communication error or the link error, its `flag`,
has been set for 24 hours without a break,
also for a device that sends no state (`silent`),
the start of the run read from the device's state lines after a restart,
with `since` and `hours`,
for the communication error also `failed_radiogram` and `held`;
`clear`, at info, where such a flag clears after a warning or an error of it,
or the communication error of a held device does
(`since` and `hours` where its start is known, `held`);
a warning or an error of it is an alert until its clear,
see GET /api/v1/log/alerts),
`pairing_started`, `paired` (with `paired`, the device that joined),
`pairing_ended` (see Pairing),
`device_forgotten` (a device deleted from the Cube, through the API or a client
of the proxy, left the server's copy;
its `state` is the device as the server knew it,
with its type, serial, name and room), `cube_found` (with `cube`,
a Cube that is not configured, see POST /cubes/discover),
`restore_step` and `restore_done` (with `restore`, the progress of a restore,
see POST /restore),
`notification` (with `notification`, what the stream's user is told, on that
user's streams alone; see Notifications).
The server's own lines, source `system`, each with its `summary` and, where a
request asked for it, `by`:
`server_started` (`version`, and `before` when it changed),
`server_stopping` (`why`: `restart`, `reboot`, `update` or `stop`),
`settings_read` (the settings file read again: `why`, the names of the
variables `changed` and of those waiting `at_next_start`, never their values;
at `warn` with its `error`
where the next start would refuse a setting of the file,
such as a log level that does not parse
or a `MASSIMO_TLS_HTTP` beside a `MASSIMO_TLS` that is not `acme`,
or the file itself, one that does not parse or is there but cannot be read,
which then changed nothing, and whose next read writes its `settings_read`
even where nothing changed:
the start's own error after "the next start refuses: ",
a secret's value, the path of `MASSIMO_TLS_KEY`, named by its variable,
and a line by its number, never quoted;
GET /api/v1/restart's `refused` says the same, each with its settings,
and a read whose settings did not change writes its `settings_read`
where that differs from the last read's,
as a DNS hook made or removed meanwhile makes it;
README.md, "Server", says which settings it checks;
at `warn` too where the read found notes on the file,
the lines it set aside, which set nothing
(a later line of a name given twice, a line without a name,
a `MASSIMO_ENV_FILE` line),
after "the settings file: ", each naming its lines by their numbers,
never a value, and after the refusals where both stand:
`the next start refuses: -hsts or MASSIMO_HSTS "-1": want a number of days, 0 or more; the settings file: MASSIMO_HSTS is given on lines 3 and 4; the first wins`,
each list joined by "; ",
one line holding all that its read found;
a read whose settings did not change writes its `settings_read`
where its notes differ from the last read's,
and GET /api/v1/restart's `refused` never names a note;
a start whose read found notes writes a `settings_read` of them,
`why` `start`, after its `server_started`,
which settles every earlier warning of the kind,
so that a note that stands stays a warning across a restart),
`restart`, `reboot`, `reboot_cancelled`, `update` (`from`, `to`, `channel`,
and `auto` for one the server started itself at night, by nobody),
`update_result` (the helper's outcome after the restart, once:
`state`, `from`, `to`; a failure at `warn` with its `error`),
`update_postponed` (the night's window closed while the radio,
or a settings file the next start refuses, held a due automatic update back:
`version`, `why`, `window`),
`backup` (`reason`, `id`, `bytes`; an automatic one that failed at `warn`),
`backup_deleted` (`id`), `backups_pruned`, `log_pruned` and `audit_pruned` (`removed`),
`log_rotated` (serve's `-log` file rotated: `why`, `size`, `format`, `sighup`
or `reload`; `previous`, the name the old file has now; `kept` and `removed`,
the rotated files kept and those deleted; at `warn` with its `error` where a
step failed),
`restore_started` (`id`, `steps`, `radio_commands`),
`open_setup` at `warn`, what was set up before the first regular user and still works
(Authentication): the `user` and its `role`, the lists of `open_setup` where it holds any,
and `via` `cli` where `massimo user add` made the user.
A Cube's own: `address_changed` (`from`, `to`, for a Cube that follows discovery)
and `cube_gone` (with `cube`, a Cube found before that no longer answers).
`change`, source `config`: every request that changed something, once it
succeeded, with `by`: its `summary` holds the audit's `action` and `target`
(a device by its serial number), what it `set` (the request's body, its
secrets removed), what `changes` from and to where the server knows both
(a Cube's `name`, `address`, proxy and hidden devices; a setting), and the
`subject` it concerns as it was then (a user's `id`, `uuid`, `name`,
`display_name` and `role`; a role's `id`, `key` and `name`; a key's `id`,
`uuid`, `name` and its user).
A change of a Cube is that Cube's line, one of a device carries its
`serial`, every other is the server's; it names its request's `command`.
The queue's, source `queue`:
`command`, a step of a command on the Cube's radio, with `command` naming the
request (`GET /api/v1/commands/{id}`) and its `summary` the `step`'s id, its
`key` and its `state`: `queued` and `sent` at `debug`, `acknowledged` at `info`
with the budget (`duty_pct`, `free_slots`), and at `warn` `retried` (the Cube
discarded it, the communication error of the devices it reaches was reset
and it was sent again),
`dropped` and `failed` with their `error`;
`hold` at `warn`, the radio commands waiting (`reason` `duty_cycle` with
`duty_pct` and `until`, or `memory_slots` with `free_slots` and `wait_s`).
`structure`, source `cube`, a removal, placement or move read back
(see "Structure changes read back"), or a repair's radiograms ("Repairs"),
with the device in `device` and in its
`summary` the `action` (`remove`, `add`, `move`; a repair's `relink`,
`unlink_relink`, `config`, `move`, a room's without `device` and with its
`rooms`),
the `outcome` (`done` at `info`, `not_confirmed` at `warn`),
the free slots `before` the change and now (`free_slots`),
the minutes since it (`minutes`), the slots still `held` or `no_reading`,
and, at `warn`, the devices the change left `flagged` or with a `link_error`;
it is settled only by a person's mark.
`slots`, source `cube`, the Cube's memory slots held for an hour with nothing
of the server's in flight (see `slots_low` under `GET /api/v1/cubes`):
at `warn` with `state` `low` (fewer than 5 free), at `error` with `state`
`full` (fewer than 2), each once in a run, a step between the two with the
state it came `from`, and at `info` with `state` `free` at the first reading
with 5 or more; each settles the one before it, so that the newest is the
alert; its `summary` has the `free_slots` of the reading, the `since` of the
first reading below 5 and, below 2, the `full_since` of the first.
`mismatch`, source `cube`, a C line and the room list, or the members of a
room, that differ (see "The Cube's lines against its room list" under the
devices): at `warn` with `state` `found`, once, a room master's again once
read, and at `info` with `state` `cleared` when they agree again, which settles
it; with the device in `device`, and in its `summary` the finding as a
device's `mismatches` has it.
The room checks', every minute (see "The room checks"):
`disagree`, source `cube`, a room whose thermostats differ, with the
`room` and the `outcome` in its `summary`:
`differ` at `warn` (the `members`, each with its `rf`, `mode`, `setpoint`
and `until`, what `differs`, the `minutes` so far),
`resent` (the room's state sent once more as a room command:
`from` `massimo`, `majority`, `newest`, `master`,
`wall_thermostat` or `radiator_thermostat`,
`before_change` true for the state from before a structure change,
its `mode`, `setpoint` and `until`,
the budget's `duty_pct` and `free_slots`), `resend_failed` at `warn` with its
`error`, `not_resent` (`why`: `today`, once a day at most, or
`no_state`, only in lines written before RoomMajority),
and `agree` once they agree again (`minutes`);
`drift`, source `device`, a thermostat in auto that does not follow its
week programme while the room's other thermostats follow theirs:
`not_following` at `warn` (its `setpoint` and the programme's
`expected` at the Cube's `cube_time`, the switch point `switch_at`, the
`others`, `hour_off` where its setpoint is the programme's an hour
before or after, its `auto_dst`, and the `failed_radiogram` of its
communication error), then `follows` (`minutes`) or `left_auto`
(`mode`).
`unconfirmed`, source `device`, a write its devices do not show in time
(see "Writes confirmed by the devices"), its `summary` the `write`
(`setpoint`, `lock`, `display`, `config`, `program`), its `target` (`room:<id>` or
`device:<rf>`), the `room`, its `source` (`api` or `proxy`), what it set
(`expected`), and an `outcome`:
`not_confirmed` at `warn`, with the `minutes` since the Cube's answer, how many
devices showed it (`confirmed`), and the `devices` that do not, each with its
`rf_address`, `serial`, the `fields` it does not show, what it `shows`, and its
`error`, `link_error` and `out_of_date`;
or `confirmed_late` at `info`, one device's, with the `seconds` it took.
A room setpoint's re-send (see "A room setpoint sent once more") has these:
`resent` at `info` (the budget's `duty_pct` and `free_slots`, and `program`
where auto with a temperature went as auto on the week programme),
`not_resent` at `info` (`why`: `superseded`, `changed`, `busy`, `boost` or
`today`), `resend_failed` at `warn` with its `error`,
then `resend_confirmed` at `info` (`confirmed`, the `seconds` it took) or
`resend_not_confirmed` at `warn`, with what `not_confirmed` has.
A device's write has the device in `device`, and so has each device's of a
configuration; a room's setpoint none.
A room setpoint's warning, its re-send's included, is settled by a later
`resend_confirmed` of the same room; any other only by a person's mark.
The server's sign-ins, source `server`, only on the pages and streams of a
user with `audit`: `login` (`user`, `remote`), `login_failed` at `warn`,
and `locked_out` at `warn` (`remote`, `for`, `after`: `logins` or `tokens`).
The LAN proxy's, source `proxy`, with the client's `remote`:
`proxy_connected`, `proxy_disconnected`,
`proxy_refused` at `warn` (the Cube is not connected),
`proxy_discarded` at `warn` (a radio command answered as discarded and not
sent, the duty cycle at `duty_pct`),
`proxy_deleted` (the `devices` a client deleted),
`proxy_delete_refused` at `warn` (`why`: `hidden`, with the `devices`, or
`not_understood`),
`proxy_pairing` (`seconds`, with `asked` where a client asked for more,
or `joined`) and `proxy_warning` at `warn` (`what`: `pairing_not_started`,
`pairing_not_stopped`, `metadata_multipart`, `metadata_not_understood` or
`metadata_not_encoded`).
A line a proxy client passes on to the Cube is a command
(`GET /api/v1/commands/{id}`) with no user, its `action` `proxy.radio`,
`proxy.delete`, `proxy.metadata`, … and its `request` the client's `remote`
and the line's letter; its step's lines name it.
A device's next `state` line after a command was sent to it, within two
minutes, names that command too.
Setpoints are changes too; a sign-in, a preference, a search, a restore's
plan and the requests whose own line is the server's (a backup, a restore,
a restart, …) write none.
`massimo user` writes the same lines, with `"via": "cli"`.
A `restart`, `reboot` or `update` the helper did not start is its kind's line
at `error` with the `error`;
any other failure the server reports is at `warn` with its `error`.
A comment line is sent every 30 seconds to keep the connection alive.

**Ids and resume.**
Every event the stream sends carries an `id:` line,
`<epoch>-<number>`: the number counts the Cube's events up,
the epoch changes when the server or the Cube's connection to it starts anew.
The connection's state events carry none,
but the last of them carries the newest id, where a reconnection resumes:
```
id: mh8k2q3x-41
event: state
data: {"kind":"state","device":"0a0102",…}
```
A client that reconnects names the last id it read,
in the header `Last-Event-ID` (the browser's own retry sends it)
or the parameter `last_event_id` (a client that opens a new stream, such as
one with a fresh ticket); the header wins where both are given.
The stream then sends the events since that id that the caller may see,
oldest first, and no state burst.
The server keeps a Cube's last 1024 events for that;
an id it no longer holds, one from before a restart, or one it never issued
gets a `resync` event first, then the state burst of a first connection:
what happened in between is lost, and the client reads what it needs again.
```
event: resync
data: {"kind":"resync","source":"system","level":"info","time":"…"}
```
A client that falls behind, its events piling up at the server,
gets what is queued for it and then the end of the stream,
rather than a stream with a gap it cannot see:
its reconnection resumes.

An event of a device carries its radio address as `device` and its serial number
as `serial`, also before the device has a state and after it is gone.
Every event says its `source` (`device`, `cube` for the Cube's own, its
configuration's `config` event under the Cube's serial; `system`, `config`)
and its `level` (`debug` for `hello`, `warn` for `disconnected`, else `info`),
as `GET …/log` does;
the server's own events, such as `cube_found`, come on every Cube's stream:

```
event: config
data: {"kind":"config","device":"0a0101","serial":"MEQ0000101","source":"device","level":"info","time":"…"}
```

A connection's greeting brings the Cube's `hello`, `metadata` and a `config`
for each device and the Cube itself every time, changed or not.
These lines come at `debug`, each with
its `summary` (a hello's `firmware`, `duty_cycle_pct` and `free_memory_slots`,
a list's `rooms` and `devices`, a configuration's main values).
`connected` comes at `info` with a `summary` of the connection:
`devices`, `rooms`, `firmware`, `duty_cycle_pct`, `free_memory_slots`,
`cause` (`start`, `loss`, or `reconnect` for one the server asked for,
which is at `debug`) and, after a loss, `down_for_s`.
What the greeting found changed, against what the server held before
(or after a start the newest backup), follows `connected` at `info`,
with `"while_disconnected": true`:
a `metadata` event with `list_changes`, a `config` event with `changes`.
Outside a greeting, `metadata` says its `list_changes`
(`room_added`, `room_renamed`, `room_deleted`, `device_added`, `device_renamed`,
`device_moved`, `device_left`, each with the room's or device's `name`,
`from`, `to`, and a device's `device` and `serial`),
a line of one device carrying that device's;
`config` says its `changes` by the configuration's names, from and to
(`comfort_temp`, `temp_offset`, `program_monday` without values, …),
and a device's first `config` says its `summary`;
one that changes nothing is at `debug`.

```
event: metadata
data: {"kind":"metadata","source":"cube","level":"info","list_changes":[{"what":"room_renamed","room":6,"name":"Bathroom","from":"Bath"}],"while_disconnected":true,"time":"…"}
```

A `state` event that follows an earlier one for the same device also says what changed,
whoever changed it, the server, another client or someone at the device:

```
event: state
data: {"kind":"state","device":"05cdc6","state":{…},
       "changes":{"window_open":{"from":false,"to":true},"setpoint":{"from":18,"to":5}},"time":"…"}
```

`changes` has one entry per value of `state` that differs (the same names:
`mode`, `setpoint`, `actual_temp`, `valve_pct`, `window_open`, `until`,
`battery_low`, `error`, `failed_radiogram`, `link_error`, `panel_locked`),
with `from` and `to`;
a value the device did not have is `null`,
so a measured temperature the thermostat stopped reporting is
`{"from": 19.9, "to": null}`,
and the line that sets `error` has the radiogram that failed,
`"failed_radiogram": {"from": null, "to": {"code": 3, "name": "TimeInformation"}}`.
`reported` appears when the Cube gained or lost the device's state
(`{"from": false, "to": true}` for a device that reported for the first time
since the Cube's restart).
Every `state` event has at least one change except a device's first one since
the server started (the connect burst,
and the state events the stream sends a new client on connect),
which has no `changes`; a reconnect compares with the state before it.
`GET …/log` keeps `changes` as well.
A change shows when the Cube's device list has it:
when the Cube sends the list on its own, or at the server's next poll,
at most 10 seconds later by default (`serve -poll`);
a thermostat reports some values only later than others,
for example its window-open temperature after the contact.

With a token configured the stream needs `?ticket=<ticket>` (below) or the bearer header;
`?token=<token>` still works, deprecated.
At each keep-alive, every 30 seconds,
the server asks whether the stream's caller would be let in now,
and ends the stream when not:
a key revoked or expired, a user or role disabled,
a server that was open when the stream opened and has a user since,
or, without a store, another token.
The client's next ticket then asks it to sign in.

### POST /api/v1/cubes/{sn}/events/ticket

A single-use ticket for that Cube's event stream, valid for a minute:

```json
{"ticket": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822c", "expires_at": "2026-11-05T15:31:12+01:00"}
```

Without a token configured the stream is open and the ticket not needed,
but the route answers all the same.

### GET /api/v1/system

What System settings → System Info shows (capability `read`):
the version, the start and the certificate, as `/healthz` has them,
and what `/healthz` does not tell anyone:
how the server was installed, the computer it runs on, its clock,
its full name, the interfaces it listens on,
the caller's address as the server sees it,
the client a trusted proxy names (`serve -trusted-proxies`),
and that proxy, where the request came through one.

```json
{
  "version": "v0.14.0-alpha.1 (go1.26.0, linux/arm64)",
  "started_at": "2026-11-13T08:00:00+01:00",
  "tls": {"source": "self-signed", "names": ["massimo.local"], "not_after": "2027-11-13T08:00:00+01:00", "fingerprint": "…", "self_signed": true},
  "install": "apt",
  "os": "linux",
  "os_name": "Debian GNU/Linux 13 (trixie)",
  "arch": "arm64",
  "hostname": "massimo",
  "client_ip": "192.168.1.20",
  "now": "2026-11-13T09:53:10.25+01:00",
  "time_zone": "Europe/Zurich",
  "fqdn": "massimo.example.net",
  "interfaces": [
    {"name": "eth0", "mac": "02:00:00:00:00:01", "addresses": [{"ip": "192.168.1.8", "api": true}, {"ip": "192.168.1.31"}]}
  ],
  "api_port": 8099,
  "cube_proxies": [
    {"serial": "KEQ0000000", "discovery_serial": "LEQ0000000", "addr": "192.168.1.31:62910", "listening": true},
    {"serial": "KEQ0000001", "discovery_serial": "KEQ0000001", "addr": "192.0.2.10:62910", "listening": false,
     "error": "listen tcp 192.0.2.10:62910: bind: cannot assign requested address",
     "since": "2026-11-13T08:00:00+01:00", "next_try": "2026-11-13T09:53:30+01:00", "tries": 230}
  ],
  "proxy": {"proto": "https", "host": "massimo.example.net", "port": 443,
    "certificate": {"source": "proxy", "names": ["massimo.example.net"], "not_after": "2027-02-10T12:00:00Z", "fingerprint": "…",
      "self_signed": false, "issuer": "R11", "trusted": true, "checked_at": "2026-11-13T09:50:02+01:00"}}
}
```

`install` is the package manager that owns the binary, `apt` or `brew`,
else what runs the server: `windows-service`, `systemd` or `launchd`,
the units of `massimo install` or others;
`other` for a terminal, a container or a supervisor massimo does not know.
`os` and `arch` are as Go names them (`linux`, `darwin`, `windows`;
`amd64`, `arm64`, `arm`);
`os_name` is the distribution's name from `/etc/os-release`, absent elsewhere.
`tls` is absent without TLS.
`now` is the server's clock with its offset, `time_zone` its zone's name:
`TZ` where set, else the zone `/etc/localtime` links to,
else the zone's abbreviation, as on Windows.
`fqdn` is the host's full name as `hostname -f` gives it, `/etc/hosts` first,
`hostname` where there is none.
`interfaces` are those the API or a Cube proxy listens on:
a listener on every address takes each interface that is up but the loopback,
one on an address that address's interface;
an interface comes with its MAC and its addresses but link-local ones,
`api` marking those the API listens on, at `api_port`.
`cube_proxies` are the Cubes' proxy listeners, listening or not,
by Cube serial and address,
the serial absent for the Cube of a plain `MASSIMO_CUBE_ADDRESS` until it reports one,
with `discovery_serial`, the serial a listener answers discovery with:
the Cube's proxy serial where one is set, else the Cube's own;
absent for a listener that answers no discovery (only one on the Cube port does,
docs/proxy.md);
one that does not listen has the fields of a Cube's `proxy_failing`
(`GET /api/v1/cubes`), and is tried again.

`proxy` is there when the request came from a trusted proxy:
the scheme, host and port the proxy serves the server at.
The scheme is read from RFC 7239's `Forwarded` (`proto=`),
`X-Forwarded-Proto`, `X-Forwarded-Scheme`, `X-Forwarded-Protocol`,
`X-Url-Scheme` or `X-Scheme`, `X-Forwarded-Ssl: on`, `Front-End-Https: on`,
or Cloudflare's `CF-Visitor`, in that order, the first value of a list;
`proto` is absent where none says.
The host comes from `Forwarded` (`host=`), `X-Forwarded-Host`,
else the request's `Host`; the port from `X-Forwarded-Port`,
else the host's own, else 443 or 80 for the scheme said;
`port` is absent where neither a scheme nor a port is.
For `https`, `certificate` is the proxy's,
read through a TLS handshake with the proxy at the request's peer address and
that port, for that host, best effort, within five seconds:
at the first proxied request after the server's start, once a day after,
and again when the proxy's address changes.
It is absent until the first check has read it; `error` says why one failed.
`trusted` says whether it is valid for the host against the system's roots,
`issuer` names who issued it.

### GET /api/v1/update?refresh=1

Whether a newer signed release is out on the server's update channel:
`MASSIMO_UPDATE_CHANNEL`, else the setting `update.channel`,
else, on a box the Debian package installed, the suite of its apt source,
else the build's `flavour`, what it was released as.
`suite` is that apt source's suite, read at each check;
where `channel` differs from it,
the next update switches the source to the channel's suite.
A channel takes the releases of its own flavour and of every more stable one,
in the order `devel`, `nightly`, `alpha`, `beta`, `rc`, `stable`
([README.md](../README.md), "Versions").
The server asks GitHub at most every ten minutes; `refresh` asks now.
It also checks by itself, whatever its settings say:
a minute after its start, then every six hours,
and ten minutes after a check that failed,
so that the notification `update` reaches people without an open page.
It does so only with a store and a valid channel;
a channel set later is checked within a minute.

```json
{
  "current": "v0.1.0-alpha.9",
  "flavour": "alpha",
  "channel": "alpha",
  "latest": "v0.1.0-alpha.10",
  "release_url": "https://github.com/metaworx/massimo/releases/tag/v0.1.0-alpha.10",
  "available": true,
  "checked_at": "2026-11-05T13:40:00+01:00",
  "can_update": true,
  "helper": {"state": "done", "from": "v0.1.0-alpha.8", "to": "v0.1.0-alpha.9", "channel": "alpha", "time": "…"},
  "auto": {"mode": "night", "window": "03:00-05:00", "next": "2026-11-06T03:00:00+01:00"},
  "history": [
    {"at": "2026-11-05T12:01:00Z", "by": {"name": "anna", "display_name": "Anna"}, "from": "v0.1.0-alpha.8", "to": "v0.1.0-alpha.9", "channel": "alpha", "reached": "v0.1.0-alpha.9", "reached_at": "2026-11-05T12:02:10Z"},
    {"at": "2026-10-30T07:15:00Z", "outside": true, "manager": "apt", "from": "v0.1.0-alpha.7", "channel": "alpha", "reached": "v0.1.0-alpha.8", "reached_at": "2026-10-30T07:15:00Z"}
  ]
}
```

`latest` is the version an update would install,
never newer than the newest release whose manifest verifies against the key
compiled into the server (see `internal/update`).
On a box `massimo install` set up it is that release.
Where apt or brew owns the binary,
a release is offered only once the box's package source has it,
since GitHub's release is out before the apt index and the tap have it:
the server reads, without root, apt's index of the suite the next update uses
(the channel's, which the helper switches the source to,
`dists/<suite>/main/binary-<arch>/Packages.gz` of the source's `URIs`,
`armhf` for an ARM 32-bit box),
or the formula of brew's tap on GitHub, the file `brew update` fetches;
`latest` is then the newest version the source has,
and `released` the signed release where the source has it not yet.
The source is read with the release check,
where the release is newer than the running version.
`source_error` says why the source could not be read,
and nothing is available then: apt could not fetch it either.
After the helper's `up-to-date` ("apt has nothing newer yet")
for the version an update asked for,
`held` names that version, which is not available, nor can it be started,
until a check made after that answer finds it in the source,
or in the signed release on a box without one;
a newer version is not held.
The hold is kept in memory: a restart is followed by a check.
`check_error` says why a check failed.
`manager` is the package manager that owns the binary, `apt`,
where the Debian package installed it, or `brew` under Homebrew's service;
absent on a box `massimo install` set up.
`can_update` is true when POST would start an update;
otherwise `blocked` says why it cannot be started from here (a package
manager owning the binary without a helper of its own, which then upgrades it;
for apt, its source missing, `MASSIMO_UPDATE_CHANNEL` naming another channel
than its suite, or the package's helper turned off;
no API token; no update helper;
on Windows and under brew, a listen address the helper cannot ask `/healthz` at,
port 0 or a service name, named by its setting,
`-listen or MASSIMO_LISTEN: the update helper cannot ask "127.0.0.1:0" for /healthz: bad port`,
under brew in brackets after the manager's text;
a development build).
`helper` is the update helper's last run: `state` is one of `checking`,
`installing`, `restarting`, `done`, `up-to-date`, `failed`, `rolled-back`,
with `message` on failure.
`auto` is the automatic update,
as the settings `update.auto` and `update.auto_window` set it (`GET /api/v1/settings`):
`mode` `off` or `night`, its `window`,
and `next`, when the box will try, absent while no update is due:
the window's next opening, or now inside it before the night's try.
At night the server installs what `POST` would, itself and by nobody,
inside the window while the radio is quiet on every Cube:
no command with a radio line waiting for the Cube or on its way,
the proxy's clients' among them, no write waiting for its devices to show it,
no restore and no pairing;
a page open in a browser holds nothing back,
nor do the Cube's own memory slots in use.
It also waits while the server's next start refuses the settings file
(`GET /api/v1/restart`, `refused`):
the update restarts the server, which would not come back,
the old version no more than the new.
Neither wait uses up the night's try, and each tick, once a minute, asks again,
so that a file fixed inside the window, which the server reads within seconds,
lets the update come that same night.
`waits` names what the next try waits for, where `next` is set:
`settings` while the next start refuses the settings file;
a busy radio sets none, since it passes within minutes.
It checks afresh at the window's opening and every half hour inside it,
so that a release built in the night is taken the same night,
and tries once a night, a start that failed included, across a restart too.
It tries nothing that `POST` would refuse:
a version held back, an update running, nothing newer,
a box that cannot start updates from here (`blocked`).
A window that closes while the radio,
or a settings file the next start refuses, held a due update back
writes `update_postponed` to the log, its `why` the last reason;
the next night tries again.
`ask` says that the page asks a holder of `update`:
Install now, Tonight or Skip this version,
automatic updates at night, an update that `POST` would start,
and no answer for it yet;
`answer` is the box's answer (`PUT /api/v1/update/answer`)
where it names the version offered or a newer one:
`version`, `answer` `tonight` or `skip`,
`by` (absent for the owner's key) and `at`.
A version skipped is not installed at night, and `next` is absent,
until a newer release is out; `Update` still installs it.
`history` is the last ten updates, newest first,
kept in the setting `update.history`:
an update started here names who started it (`by`, absent for the owner's key),
the version it ran (`from`), the one it asked for (`to`)
and the channel it asked on (`channel`);
one the server started itself at night has `auto` and no `by`;
the next start with another version adds `reached` and `reached_at`,
whichever version that is:
apt may install an older one than `to`, in between, or a newer one.
A new version that no entry waits for came from outside Massimo (`outside`):
apt, brew (`manager`) or a person on the box;
its `channel` is the one the box followed at the start that found it.
`result` is the helper's outcome where the box did not stay on a new version:
`failed` or `up-to-date`, without `reached`,
or `rolled-back`, after the version `reached` ran.
An outcome the helper reports while the server runs on,
`up-to-date` where apt has nothing newer yet or a failure before the restart,
is taken at the next read of this route or at the next update started;
one that comes with a restart, at that start.
An entry the helper reports no outcome for stays open.
The log keeps an update's lines for a month only.

### POST /api/v1/update

Asks the update helper to install the newest release on the channel:
the server writes the channel into its request file,
having stored it as `update.channel` first where the release is of another
flavour and neither the variable nor the setting names a channel,
so that a box on `beta` stays on it after installing a stable release;
and the helper (started by `massimo-update.path` as root on Linux,
by launchd's `ch.metaworx.massimo-update` on macOS,
by the server itself as a detached process on Windows;
see [packaging/README.md](../packaging/README.md)) downloads and verifies the release,
test-runs it, replaces the binary,
restarts the server and rolls back if the new version does not answer `/healthz`.
Where `manager` is `apt`, the Debian package's helper runs apt for the massimo
source instead, having pointed the source at the channel's suite where it names another
(and back where apt cannot fetch that suite), the package restarts the server,
and nothing is rolled back: the helper's last run names the way back
(packaging/README.md, "Debian package").
Where it is `brew`, the server starts a helper as its own user that runs
`brew upgrade` and restarts the job (packaging/README.md, "Homebrew").
The request names no version and no file,
so the most the API can do is install the newest signed release,
or what the box's package source has of it (`latest`).
Answers `202` with `{"ok":true,"requested":"v…", "channel":"…"}`;
follow the progress with GET.

Needs the API token: without `MASSIMO_API_TOKEN` it answers `403` even though
the other routes are open.
`403` also when no helper is configured (`serve -update-request`),
and with GET's `blocked` as the error wherever that names a reason;
`409` when already up to date,
when the channel has no release, while an update is running (on Windows also
in the moment between the start of the helper and its first status),
when the package source has the release not yet, or while it is held;
`500` on Windows when the helper could not be started (the request is withdrawn);
`502` when GitHub or the package source could not be asked.

### PUT /api/v1/update/answer

The box's answer to the automatic update's question (`auto.ask` of GET),
for a holder of `update`:

```json
{"version": "v0.1.0-alpha.10", "answer": "tonight"}
```

`version` is the one asked about, `latest` of GET; another is `409`.
`now` starts the update at once, as `POST /api/v1/update` does,
with its answers, and keeps nothing.
`tonight` ends the question for the version, which the night installs
as it would unanswered;
`skip` holds the version back from the night, and the question with it,
until a newer release is out.
Both need automatic updates at night (`409` while `update.auto` is `off`),
are kept in the setting `update.answer`, one answer for the box,
and answer `200` with what GET answers.
Audited as `update.answer`, a change line in the log:
every page reads the versions again at it, so that the question goes everywhere.

### GET /api/v1/restart, POST /api/v1/restart

`GET` says whether this caller's `POST` would restart the server now,
and who starts it again:
`{"available": true, "by": "systemd"}`,
or `{"available": false, "blocked": "<why>"}`;
`pending`, where there is any,
names the settings of the settings file that changed since the start
and apply only at the next one,
such as `["MASSIMO_TLS", "MASSIMO_LISTEN"]`: names, never values;
`file` is the settings file `serve` reads and `read_at` when it last read it,
at the start or since,
also where the file did not parse, or was there but could not be read.
`refused`, where there is any,
is what that read found the next start refuses, in the start's order,
as the log's `settings_read` says it (POST /api/v1/reload):

```json
{"available": true, "by": "systemd", "pending": ["MASSIMO_HSTS"], "refused": [
  {"settings": ["MASSIMO_HSTS"], "error": "-hsts or MASSIMO_HSTS \"-1\": want a number of days, 0 or more"}
]}
```

each with `settings`, the settings that changed since the start
and that the refusing check reads, by name,
such as `MASSIMO_TLS`, changed from acme,
for the `MASSIMO_TLS_HTTP` the start took beside it,
also one that `pending` does not list,
`MASSIMO_REBOOT`, which applies at once,
or `MASSIMO_CUBE_SN`, which it never lists,
and `error`, the start's own text,
which may quote a value,
a secret's (the path of `MASSIMO_TLS_KEY`) named by its variable;
without `settings` for the file itself,
one that does not parse or is there but cannot be read,
its error naming the file and the line by its number, never the line.
A read that finds nothing refused clears it,
and the start has none, since it would have refused it.
It blocks nothing: the restart is offered all the same,
and so are an update and a reboot.
A holder of `update` or of `reboot` reads it as one of `restart` does,
since each includes `restart` ("Authentication").
`by` is `systemd`, `launchd` (also Homebrew's job), `windows`
(the Windows service manager) or `exit` (`serve -restart exit`:
whatever runs the server, told by the exit status).

`POST` restarts the server.
It stops as on SIGTERM once this request is answered and its audit row written,
and exits with status 75,
which systemd, launchd and Homebrew answer with a start;
a Windows service runs `massimo restart` instead,
which stops and starts the service through the service manager.
Answers `202` with `{"ok": true, "by": "systemd"}`;
`GET /healthz` then reports a new `started_at` once the server is back.
Sends nothing to a Cube, and a retry with the same `Idempotency-Key`
is answered with the first `202` rather than restarting again.

An open server, with no users or without a token (see "Authentication"),
takes the `POST` only from the box itself:
a loopback peer with no `X-Forwarded-For`, `X-Real-IP` or `Forwarded` header,
so that nobody on the LAN, and no client of a reverse proxy on the box,
can restart it;
anyone else gets `403` with the reason.
`403` also without the capability `restart`;
`409` when no service manager would start the server again (`serve` in a terminal,
or `-restart off`),
while a restart is under way,
or while an update runs, which restarts the server itself;
`500` when the restart could not be started (on Windows, the helper).

### POST /api/v1/reload

Reads the settings file again at once, as SIGHUP does on Unix:
`MASSIMO_API_TOKEN` applies at once unless `-token` was given,
and so does the reboot's switch, `MASSIMO_REBOOT`,
unless `-allow-reboot` was given;
every other setting that changed applies at the next start;
the log file is rotated, where `serve` writes one (`-log`),
and the store is checkpointed, whatever the read finds.
`serve` also reads the file by itself within seconds of a change,
so this is for not waiting, and for systems without SIGHUP.
Needs the capability `restart`, and answers what it found:

```json
{"changed": ["MASSIMO_LISTEN"], "pending": ["MASSIMO_LISTEN"], "refused": [], "read_at": "2026-11-08T19:52:00Z"}
```

`changed` names the variables this read changed, `pending` those that now
differ from the start and wait for the next one; names, never values.
A setting the next start would refuse is not refused here:
`refused` says it, as GET /api/v1/restart does from then on,
each with its settings and the start's own text, `[]` for none;
the log's `settings_read` (under GET /api/v1/cubes/{sn}/events) is then at `warn`,
its `error` those texts after "the next start refuses: ", joined by "; ",
and a reboot's switch the start would refuse is off.
A note on the file, a line the read set aside, is no refusal:
`refused` does not name it,
and the log's `settings_read` is at `warn` with it after "the settings file: ",
after the refusals where both stand.
It sends nothing to a Cube, and is an audit row (`server.reload`) but no
command, so a retry is harmless.
`409` when the server reads no settings file,
`500` when the file could not be read, with the error,
after the rotation and the checkpoint:
nothing is applied, and what waits for the next start stays as it was,
and the log's `settings_read` is at `warn`
with the start's own error after "the next start refuses: "
for a file that does not parse, or is there but cannot be read,
which GET /api/v1/restart's `refused` then says, without a setting;
a missing one, which an editor may be replacing,
is a warning on the process's log alone.

### GET /api/v1/reboot, POST /api/v1/reboot, DELETE /api/v1/reboot

The reboot of the computer the server runs on, for the capability `reboot`,
which no default role but `owner` has:
restarting the server is not rebooting a computer that may run other services.
It needs the owner's switch, `MASSIMO_REBOOT=on` in the settings file
or `serve -allow-reboot` (README.md, "Server"),
and on Linux and macOS the root helper that `massimo install -reboot-helper`
installs (packaging/README.md, "Rebooting from the web UI");
the Windows service reboots by itself.
`serve` in a terminal, in Docker, as Homebrew's service
or on Linux without systemd does not reboot.

`GET` says whether this caller's `POST` would reboot the computer now,
and who does:
`{"available": true, "by": "systemd"}`,
or `{"available": false, "by": "systemd", "blocked": "<why>"}`;
`at`, while a reboot waits or has begun, is when it is asked for,
`begun` is `true` once the delay is over and the helper was asked,
and `error` says why the last one failed.
`by` is `systemd` or `launchd`, their root helper,
or `windows`, the service itself.
The reasons come in this order:
the server cannot reboot its computer at all,
the switch is off,
the helper is not installed,
a reboot has begun or waits,
a restart is under way,
an update is running.

`POST` with `{"delay": 60}` asks for the reboot after that many seconds,
0 to 3600, 60 when the body names none,
and answers `202` with `{"ok": true, "by": "systemd", "at": "2026-11-08T15:20:00Z"}`.
The server waits the delay itself:
a reboot that waits ends with the server, and `DELETE` takes it back.
Then on Linux and macOS it writes `/var/lib/massimo/reboot-request`,
which starts the root helper (`massimo-reboot.path`,
`ch.metaworx.massimo-reboot`);
the helper removes it and reboots,
and reads nothing the server's user can write.
A request the helper has not taken within 30 s is removed,
and `GET` reports it as the `error`;
so it does when the server still runs two minutes after the helper took it,
since the system stops the server within seconds of a reboot;
the next reboot may then be asked for.
The Windows service runs `shutdown.exe /r` instead.
The system stops the server as at every shutdown;
`GET /healthz` reports a new `started_at` once the computer is back.
It sends nothing to a Cube, and a retry with the same `Idempotency-Key`
is answered with the first `202`.
While a reboot waits the restart is refused,
and switching the reboot off takes it back.

`DELETE` takes back a reboot that waits: `200` with `{"ok": true}`,
`409` once it has begun or when none waits.

An open server takes the `POST` and the `DELETE` only from the box itself,
as the restart (above).
`400` for a delay outside 0 to 3600 seconds;
`403` without the capability `reboot`;
`409` with the reason `GET` gives.
Both are audit rows, `server.reboot` and `server.reboot.cancel`.

### GET /api/v1/store, GET /api/v1/config

What `massimo store info` and `massimo config` show, for the capability
`settings` (System settings → Maintenance in the web UI).
`GET /api/v1/store` answers the store's file, its schema version, the
oldest that still reads it, the version this server is built for, the node
and tenant identities and the rows per table:

```json
{"file": "/var/lib/massimo/massimo.db", "schema": 7, "min_reader": 1, "binary": 7,
 "node": "…", "tenant": "…", "rows": {"audit": 412, "users": 5}}
```

`GET /api/v1/config` answers the version, the settings file and whether it
was found, the data directory, the address `serve` listens on, its TLS mode,
the log level and every `MASSIMO_*` variable in effect with where it came
from (`environment` or the settings file, with its lines where the file
gives the name more than once: `file, lines 3 and 4: the first wins`).
A key's, an IV's, a token's or a password's value never leaves the server:
such a variable has `"secret": true` and no `value`.
A name Massimo does not know has `"unknown": true`, and `"secret": true`
with no `value`, since it may hold a secret under a name the rule misses,
such as `MASSIMO_API_TOKEN_OLD`.
`503` when the server has no store, or does not show its settings.

### Server backups

The server's own backups (docs/store.md, "Server backups"):
the whole store and the files beside it, encrypted for the owner's keys,
made once each night at `serverbackup.time` (`03:30` by default, the box's time;
`GET`/`PUT /api/v1/settings`) and copied to the targets that take them,
each target keeping them by its retention.
A backup is one file, the same on every target, its copies the targets that hold it.
Nothing is made while no key is set.
`backup.server` makes, pushes and pulls; `backup.retry` sees the state and
retries what failed, and `backup.server` includes it.
The keys are the owner's alone.
Neither capability is an integration's, nor may a provider's role hold one.

`GET /api/v1/server-backups` (`backup.retry`) lists the backups, newest first,
each with its copies, and each target's state:

```json
{"backups": [{"uuid": "0193…", "time": "2026-11-27T02:30:00Z", "trigger": "scheduled",
  "state": "made", "size": 26717, "sha256": "8468…", "label": "", "version": "v0.18.0",
  "node": "SHA256:…", "this_box": true, "manifest": {"format": 1, …}, "pinned_until": null,
  "tries": 0, "next_try": null,
  "copies": [{"target": "0193…", "target_name": "local", "path": "massimo-20261127-023000-0193….zip",
    "state": "there", "pulled": false, "pushed_at": "…", "checked_at": null, "tries": 0, "next_try": null}]}],
 "targets": [{"uuid": "0193…", "name": "local", "kind": "local", "takes": "every", "enabled": true,
   "last_success": "…", "behind": false}],
 "keys": 1, "time": "03:30", "next_run": "2026-11-28T02:30:00Z"}
```

- A backup's `trigger` is `scheduled`, `manual`, `before-update`, `before-restore`,
  or `uploaded` for a file uploaded, whose manifest keeps the trigger it was made with.
  A backup that `failed` is a row too, with its `error`, `tries` and `next_try`.
- A copy is `there`, `failed` (a copy that did not go through, or that a check
  found changed) or `missing` (gone from the target at its scan).
  `pulled` is a copy put on the local target by hand, kept until it is deleted.
- A target is `behind` where its newest copy, or without one its creation,
  is older than two of its periods: a day where it takes `every` or `daily`
  backup, a week for `weekly`, a month for `monthly`.

`POST /api/v1/server-backups` makes one now, `{"label": "…", "target": "…"}`,
the label at most 100 characters in plain in the manifest,
the target a uuid or a name; without one it goes to every target that takes it,
or where none does to the local ones; `409` without a key.
Each answer of a job is `{"backup": {…}, "steps": [{"step": "copy", "target": "local",
"ok": true}, …]}`: what it made, copied, pruned, scanned or checked.
A copy that failed is in the backup's copies, and the answer is still `201`.

The schedule:

- A run missed while the server was down is made five minutes after its start.
- Each target takes every backup, or the first of each day, week (Monday to Sunday)
  or month of which it holds none, in the box's time.
- After each copy, the target keeps what its retention keeps, restic's forget policy:
  `{"last": N, "daily": D, "weekly": W, "monthly": M, "yearly": Y}`,
  the newest of each of the last D days that hold a copy, and so on;
  a pinned backup, a pulled copy and the target's newest are kept whatever it says,
  and a policy of zeros keeps everything.
  A new target keeps 7 days, 4 weeks and 6 months.
- A scheduled backup, or a copy of one, that fails is tried again after 5 minutes,
  30 minutes and 2 hours, the backup in its row, a copy as the same file again.
  After the third retry it is a warning in the log,
  an alert there (`GET /api/v1/log/alerts`) until a later success on its target,
  and, once, a notification `server_backup_failed` to every user who holds
  `backup.retry`; then it waits for a retry by hand.
  A backup made by hand is not retried by itself: its failure is the answer,
  and a warning in the log.
  `POST …/server-backups/{id}/retry` (`backup.retry`) tries again at once
  what of a backup failed, as the schedule made it, and prunes no more than the
  schedule would; `409` where nothing of the backup failed,
  or for a backup made by hand that failed, which is made again by hand.
- A backup is a failed row until its first copy is there,
  so that a stop of the server in the middle of its copies leaves a backup to retry;
  `DELETE …/server-backups/{id}` removes one of which no target holds a copy.

The other routes of a backup, `backup.server`'s:

- `PUT …/server-backups/{id}/pin`, `{"pinned": true}`, keeps it from every
  retention until `{"pinned": false}`, or until `until`.
- `POST …/server-backups/{id}/check` reads each copy back whole, without the key:
  its size, the file's hash and the payload's against the signed manifest;
  `{"target": "…"}` checks one.
- `GET …/server-backups/{id}/file` downloads it from a target that holds it,
  the local ones first, a `Range` answered.
- `POST /api/v1/server-backups/upload`, the file as the body (at most 8 GiB),
  takes a backup made elsewhere to the local target, pulled;
  `400` for a file that is no backup,
  `422` for one of a newer format or changed since it was made.
- `PUT …/server-backups/{id}/copies/{target}` pushes it to the target,
  a uuid or a name, from a copy that is there;
  `DELETE` removes the copy, the file included,
  and the backup goes from the catalogue with its last copy;
  `409` where the target keeps the file, such as an S3 bucket's object lock,
  the copy staying.
- `POST …/server-backups/{id}/copies/{target}/pull` fetches the target's copy
  to the local target, where it stays until it is deleted.

The targets, `backup.server`'s:
`GET /api/v1/server-backup-targets/kinds` names the kinds this server knows,
with the fields of each one's form, the web UI's texts
`serverbackup.field.<kind>.<key>`:

```json
[{"name": "local", "fields": [{"key": "path", "type": "path", "required": false, "default": "backups"}]}]
```

A field's `type` is `text`, `number`, `path` (a directory of this box,
relative to the data directory), `secret`, `bool` or `choice` (with `choices`);
`secret` names the kind's secret field, given apart and kept sealed.
A field's `warn`, where it has one,
is a pattern in the syntax RE2 and JavaScript share,
of the values the card warns of beside the field,
with the text `serverbackup.warn.<kind>.<key>`; it refuses nothing.
The kind `webdav` takes `url`, `user` and the secret `password`,
its `url` warned of where it is plain `http:` (`^[Hh][Tt][Tt][Pp]:`),
since the password then travels in clear (docs/store.md, "WebDAV").
`POST /api/v1/server-backup-targets` adds one,
`{"name": "NAS", "kind": "local", "config": {"path": "/mnt/nas/massimo"},
"secret": "…", "takes": "daily", "retention": {"daily": 7, "weekly": 4},
"enabled": true}`,
a name unique without regard to case (`409`), and scans it at once:
`{"target": {…}, "found": 2, "skipped": ["notes.zip"]}`, the backups it holds
added to the catalogue by their manifests, read without the key.
`PUT …/server-backup-targets/{uuid}` changes what it is given,
`secret` replaced, `""` removing it, left out kept;
of a kind with a secret field, a change to `config` keeps no secret:
the config says where the secret goes, so the secret comes again with it,
or the change is refused with `400`;
`DELETE` removes the target and its copies from the catalogue, the files staying;
`POST …/{uuid}/test` reaches and writes to it, `{"ok": false, "error": "…"}` where not.
A target's list (`GET /api/v1/server-backup-targets`) shows its fields,
never its secret, but whether it has one (`has_secret`)
and whether that does not open (`secret_lost`), to be entered again.
Where no target exists, the first backup makes `local`, `backups/` in the data directory.

An SFTP target, `sftp` (`host`, `port` 22, `user`, `path`, `host_key`),
takes no secret: the server makes its key, an Ed25519 pair, when it is added,
and `PUT` with `"secret": ""` makes a new one.
The target shows the key's public line as `public_key`,
`ssh-ed25519 AAAA… massimo-server-backup`,
for the user's `authorized_keys` on the host.
Until `host_key` is set, and while the host shows another key,
the server stops at the host's key, offering it nothing and writing nothing:
the add's scan and `POST …/{uuid}/test` answer `"host_key": "SHA256:…"`,
the host's, and `"pinned_host_key"`, the one confirmed,
where the host's changed.
The owner compares it with the host's own
(`ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` there)
and confirms it with a `PUT` of the target's config, `host_key` set
to the fingerprint or to the host's key line.

The keys: `GET /api/v1/server-backup-keys` (`backup.server`) lists their public parts.
`POST /api/v1/server-backup-keys` (the owner's alone) takes
`{"label": "laptop", "recipient": "age1…"}`, a post-quantum `age1pq1…`,
a classic `age1…` or an `ssh-ed25519` line,
or `{"label": "safe", "make": "pq"}` (or `"classic"`), which makes a key and
answers it once, `{"key": {…, "pending": true, "expires_at": "…"},
"identity": "AGE-SECRET-KEY-…", "key_file": "# created: …"}`.
A key made encrypts nothing until `POST …/server-backup-keys/{uuid}/confirm`,
`{"last_six": "…"}`, gets the identity's last six characters,
without regard to case (`422` for others), within an hour, after which it goes.
The server keeps the public part and a hash of those six, never the identity,
and no audit row or command keeps either.
A box's keys are all post-quantum or all classic, as age requires:
another kind is `409` with `"code": "mixed_kinds"`.
`DELETE …/server-backup-keys/{uuid}` removes one;
backups made before stay encrypted for it.

The restore, the owner's alone, as the keys are
(docs/store.md, "Restore by scope"):
each route takes the key in its body, held in memory for the request alone,
never written, and kept by no audit row,

```json
{"key_file": "# created: …\nAGE-SECRET-KEY-…", "scopes": ["users", "cube:KEQ0000000"],
 "take_identity": false, "env": false, "tls": false}
```

`key_file` an age identity or an OpenSSH `ssh-ed25519` key without a passphrase;
`scopes` of `settings`, `users`, `integrations`, `cubes` (every Cube of the backup),
`cube:<serial>`, `history` and `all`;
`take_identity` the backup's `node.key` and node id, this box keeping its own without;
`env` the backup's env file where this server reads its own,
`tls` its TLS files where this server keeps its own,
each file replaced kept beside as `<name>.before-restore`.
A key that does not open the backup is `422`,
and so is a backup whose store is newer than this server's: update first.

- `POST …/server-backups/{id}/restore/plan` answers what a restore would do
  and changes nothing, every scope where none is named:
  the backup (`uuid`, `time`, `trigger`, `label`, `version`, `schema`),
  its `identity` (`this_box`, `fingerprint`, its `node` id, and for another box's
  whether that box still `answers` on the network, `null` where not looked for),
  whether its store was `migrated` from an older schema,
  the `cubes` it holds (`serial`, `name`, `live` where this server has it),
  the `files` the options would put in place (`role`, `to`),
  whether this server can `restart` itself (`available`, `by`, `blocked`),
  the `report` and, where the restore would be refused, why (`refused`):
  what is missing, a place this server cannot write (the systemd unit keeps
  `/etc/massimo` read-only to it), or users and sign-in or the identity
  on a server that cannot restart itself.
- `POST …/server-backups/{id}/restore` writes the files beside their places,
  takes a pinned local backup, replaces the scopes whole in one transaction,
  renames the files into place, and restarts the server as `POST /api/v1/restart` does,
  also where a file failed after the commit: then `500`, with the answer and its `error`.
  What the server wrote between the backup taken first and the commit is in neither:
  the answer names both times, `before_backup_time` and `restored_at`.
  Users and sign-in, or the identity, are `409` on a server that cannot restart itself.
  The answer says so first, `{"restart": {"started": true, "by": "systemd"}, …}`,
  or why not (`blocked`): the server is then to be restarted by hand,
  since it keeps `opaque.key` and its node in memory.
  It names the backup taken first (`before_backup`), the `report`, the `files`,
  and the `cubes` restored, each with the Cube backup the Cube restore could write
  onto it (`backup`; `POST /api/v1/cubes/{sn}/restore`, with its radio cost).
  `409` with `"code": "missing"` where rows of other scopes the backup's rows name
  are missing here, `"no_owner"` where users and sign-in would leave no owner,
  each with the `report`.
- `POST …/server-backups/{id}/check-key` opens the backup into a temporary directory
  and checks its store, `PRAGMA integrity_check`, changing nothing:
  `{"ok": true, "integrity": ["ok"], "this_box": true, "files": [{"name": "massimo.db",
  "role": "store"}, …], "backup": {…}}`.

The `report` gives, per scope and table, the rows `restored` from the backup,
of them those that `replaced` a live row, the live rows the backup lacks (`removed`),
of them those `kept_removed` since a row outside the scopes names them;
the rows `missing`, each with its `table`, `uuid`, `name` and the table that names it
(`by`); the references `cleared` and the rows `removed` with a row the restore removed;
and the sealed secrets that did not open (`secrets_lost`), each to be entered again.

## The LAN proxy

Besides the API, `massimo serve` answers as a Cube on TCP port 62910 and UDP discovery,
so that programs speaking the Cube protocol share the one connection the Cube allows.
See [proxy.md](proxy.md).

## Not in version 1

Removing devices, creating rooms, and ownership per Cube.
Removing needs the original software for now; `massimo restore` or
`POST …/restore` from an edited backup covers everything else in bulk.

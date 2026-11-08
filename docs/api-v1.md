# Massimo HTTP API, version 1

The API is served by `massimo serve` (default `127.0.0.1:8099`),
plain HTTP, or TLS with `-tls` (README.md, "Server");
with TLS every response carries `Strict-Transport-Security`,
a year by default (`-hsts <days>`, `0` clears it).
Every response is JSON unless stated otherwise.
The machine-readable description is in [api-v1-openapi.yaml](api-v1-openapi.yaml).

The server keeps one persistent connection and one command queue per Cube.
Reads answer from the server's copy of the Cube's state,
which the Cube refreshes on every change and which the server polls once a minute.
Writes go through the queue, which serialises commands, respects the
radio duty cycle and the Cube's memory slots, and resets a device's error
state when needed.

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
My settings, under the header's 👤 menu, has the poll interval (10 s by default;
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
the header's 👤 menu says who is signed in and signs out.
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

The server authenticates in one of three ways, by what it has:

- **Users**, a store with at least one user: every `/api` request carries
  `Authorization: Bearer <token>`, a session from `POST /api/v1/login` or
  an API key from `POST /api/v1/users/{name}/tokens`; otherwise `401`.
  The token names a user, the user's role grants capabilities,
  and every route needs one of them (the table below);
  a request without it is `403` with the message `capability "pair" required`.
  A disabled user, a guest past its end date or a disabled role is `403` too,
  with the reason.
- **Open**, a store without users, the state before the first owner:
  every request from the local network may do everything,
  as the original software can on the LAN.
  `POST /api/v1/users` from an address on the local network creates the first
  owner (see that route), and `massimo user add` does the same on the box.
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
The file stays the way to recover an owner who lost the password:
put a token into it, log in with it, set a password; no restart is needed.

The capabilities, what each allows and the routes it guards;
the OpenAPI file carries each operation's as `x-capability`:

| Capability | Allows | Routes |
|---|---|---|
| `read` | see everything of the Cubes and one's own user | every `GET` of a Cube, its rooms, devices, configuration, programmes, log, pairing, restore and backup state, the event stream and its ticket, `GET /api/v1/update`, `GET /api/v1/me` |
| `preferences` | keep one's own settings and colour schemes on the server, so that they follow the user across browsers; without it they stay in the browser | `PUT`/`DELETE /api/v1/me/preferences/{key}` |
| `setpoint` | set temperatures and modes | `PUT …/devices/{rf}/setpoint`, `PUT …/rooms/{id}/setpoint` |
| `lock` | lock and unlock the thermostats' buttons | `PUT …/devices/{rf}/lock` |
| `configure` | temperatures, device settings, week programmes | `PUT …/rooms/{id}/config`, `PUT …/devices/{rf}/config`, `PUT …/rooms/{id}/program`, `PUT …/devices/{rf}/program`, the copy of a room's settings in `PUT …/devices/{rf}` |
| `rename` | room and device names | `PUT …/rooms/{id}`, `PUT …/devices/{rf}` with `name` |
| `structure` | move and remove devices, delete rooms | `PUT …/devices/{rf}` with `room_id`, `DELETE …/devices/{rf}`, `DELETE …/rooms/{id}` |
| `pair` | put the Cube into pairing mode and stop it | `POST …/pairing`, `DELETE …/pairing` |
| `discover` | search for Cubes on the LAN | `POST /api/v1/cubes/discover` |
| `backup` | download a backup | `GET …/backup` |
| `restore` | plan and run a restore | `POST …/restore/plan`, `POST …/restore` |
| `guests` | create, change and remove users whose role is `guest` | the users routes, limited to guests |
| `users` | every user: create, role, disable, password, tokens | `GET`/`POST /api/v1/users`, `…/users/{name}`, `…/users/{name}/tokens` |
| `passwords` | set the password of a user (the manager's by default) | `GET /api/v1/users`, `PUT …/users/{name}` with a password, `POST …/users/{name}/register` |
| `roles` | a role's capabilities and name, create and delete roles | `GET`/`POST /api/v1/roles`, `PUT`/`DELETE /api/v1/roles/{key}` |
| `roles.disable` | disable and enable a role | `PUT /api/v1/roles/{key}/disabled` |
| `capabilities` | disable and enable a capability for every role but `owner` | `GET /api/v1/capabilities`, `PUT /api/v1/capabilities/{key}` |
| `cubes` | add, configure and remove Cubes | `POST /api/v1/cubes`, `PUT`/`DELETE /api/v1/cubes/{sn}` |
| `settings` | the server's settings, its store and the settings in effect | `GET`/`PUT /api/v1/settings`, `GET /api/v1/store`, `GET /api/v1/config` |

A user changes only users whose role has no capability its own lacks,
and gives only such roles: an owner changes everyone,
nobody but an owner changes an owner or gives the role `owner`,
and a role made with `users` but without, say, `restore` changes no manager.
The users routes say what the caller may do with each user they list
(`"may": {"manage": true, "password": true}`).
| `update` | start a software update | `POST /api/v1/update` |
| `restart` | restart the server through the service manager that runs it, and read its settings file again | `GET`/`POST /api/v1/restart`, `POST /api/v1/reload` |
| `audit` | read the audit | `GET /api/v1/audit` |

The default roles and their sets are those of docs/store.md section 6:
`owner` has every capability and cannot be changed,
`manager` is `operator` plus guests, pairing, names, structure,
restore and the restart,
`operator` runs the heating, locks, configures, discovers and takes backups,
`guest` is `operator` without the lock, backups, discovery and preferences,
`viewer` reads, `integration` reads, sets and locks.
A role's set can be changed, a role renamed, created and disabled,
and a capability disabled for every role but `owner`,
through the routes of `/api/v1/roles` and `/api/v1/capabilities`.
A user's own user and the routes of `/api/v1/users/{name}` with the user's
own name are open to it for its display name and its password.

Ten failed token checks from one address within a minute lock that
address out for five minutes: every `/api` request from it is then
answered `429` with a `Retry-After` header, the right token included.
A wrong ticket counts as a failed check,
and a failed login counts against the address and against the user name,
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
- `{rf}` is a device's radio address, six hex digits (`0a0102`).
- `{id}` is a room number as the Cube assigns it (1..255).

## Errors

Errors are `{"error": "<message>"}` with a matching status: `400` bad input,
`401` missing or wrong token, `403` a user who may not do this,
`429` locked out after repeated wrong tokens,
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
`node_id` is the box's identifier from its store (`node.id`,
a UUID made at the first start, see docs/store.md);
it stays the same across restarts and updates.
`tls`, with TLS, describes the certificate served:
`{"source": "files", "names": ["massimo.example.net"], "not_after": "2027-02-01T00:00:00Z", "fingerprint": "SHA256:…", "self_signed": false}`;
`source` is `files` (the owner's files, re-read when they change),
`self-signed` (the certificate the server made, `self_signed` true)
or `acme` (Let's Encrypt),
the fingerprint is the SHA-256 of the certificate as browsers show it,
and `warning` is set while something needs the owner:
a certificate within two weeks of its end,
or an order from Let's Encrypt that has not succeeded yet.

### POST /api/v1/login, POST /api/v1/logout, GET /api/v1/me

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
An API key answers `400`; a key is revoked under the user's tokens.

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
none for a disabled role) and `token`, the one presented;
outside users mode `user` and `role` are null and `capabilities` is every one.
`setup` is true on an open server for a caller on the local network,
the one that may create the first owner.
`auth_version` 1 says the server hashes the password;
2, that the user signs in by OPAQUE (below) and the server never sees it.

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

### Users: /api/v1/users and /api/v1/users/{name}

`GET` and `POST /api/v1/users`; `GET`, `PUT` and `DELETE /api/v1/users/{name}`.
The users of the store (docs/store.md section 6), each with one role.
The routes need `users`; a holder of `guests` reaches them for users whose role
is `guest`, and a user reaches its own name (see below).
Every one answers `503` on a server without a store.

`GET /api/v1/users` lists the users by name, as `GET /api/v1/me` shows a user;
a `guests` holder sees the guests and itself.

`POST /api/v1/users` creates one:

```json
{"name": "anna", "display_name": "Anna", "role": "operator", "password": "…"}
```

`name` is the login name, 1 to 64 of letters, digits, `.`, `_`, `-` and `@`,
kept in lower case; `role` is a role's key, `viewer` when absent;
`password` may be absent for a user that logs in with keys only,
or replaced by `record` and `ksf`, a password of version 2 (below);
`disabled` and, for a guest, `expires_at` (RFC 3339) as well.
A `guests` holder creates guests only (`403` otherwise).
The answer is `201` with the user; a name in use is `409`, a bad name,
an unknown role or an unknown field `400`,
too many password checks at once `503` with `Retry-After`, as for the login.
The name of a removed user can be used again;
the new user takes over the old one's row,
so the audit's history of the name reads on,
and starts without the old one's preferences.

On a server without users, `POST /api/v1/users` creates the first owner:
open to a caller on the local network (loopback, a private range, link-local),
`role` `owner` (absent, or that) and a password or a record required,
`403` from anywhere else; `massimo user add` on the box is the other way.
From then on the server is in users mode (Authentication).

`GET /api/v1/users/{name}` is one user.
`PUT /api/v1/users/{name}` changes any of `display_name`, `role`, `disabled`,
`expires_at` (`null` clears it) and `password`;
absent fields stay, an unknown one is `400`.
A user changes its own `display_name` and, with `current_password`
or a `proof` from the sign-in by OPAQUE, its own `password` or `record`;
a user with keys only (`has_password` false) sets its first one without,
as its key could as well make another key;
the rest needs `users` (`guests` for a guest), a password `passwords` too,
each for a user the caller outranks ("Authentication").
A password set by someone else ends the user's sessions;
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
or given an end date (`409`): an owner with one does not count.

`DELETE /api/v1/users/{name}` removes a user: its sessions and keys are revoked,
its preferences removed,
and its row stays for the audit; `{"ok": true, "name": "anna"}`.
The last enabled owner without an end date stays (`409`);
a user cannot remove itself.

### Tokens: /api/v1/users/{name}/tokens

`GET` and `POST /api/v1/users/{name}/tokens`;
`DELETE /api/v1/users/{name}/tokens/{id}`.
A user's sessions and API keys, as `GET /api/v1/me` shows the one presented;
the same rules of access as the user.

`GET` lists the live ones, oldest first, without their secrets.
`POST` with `{"label": "Loxone"}` creates an API key, which never expires,
and answers `201` with the key once:

```json
{"token": "…", "id": 4, "kind": "api", "label": "Loxone", "created_at": "…"}
```

The key is kept nowhere but in this answer:
the command of the request keeps the answer without `token`,
and a retry with its `Idempotency-Key` answers `201` without it too.

`DELETE …/tokens/{id}` revokes a session or a key by the `id` of the list;
a token of another user is `404`.

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

### Preferences: /api/v1/me/preferences

`GET /api/v1/me/preferences`; `GET`,
`PUT` and `DELETE /api/v1/me/preferences/{key}`.

The signed-in user's preferences (docs/store.md section 9 (c)):
a JSON document per key, which the web UI uses so that its settings follow
the user across browsers.
The page keeps two keys:

```json
{
  "ui": {"poll": 10, "expert": false, "clock": "browser", "lang": "auto", "theme": "system", "scheme": "custom:Mine"},
  "schemes": {"format": "massimo-schemes/1", "schemes": [{"format": "massimo-scheme/1", "name": "Mine", "light": {…}, "dark": {…}}]}
}
```

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
`room.rename`, `room.config`, `room.program`, `room.delete`,
`pairing.start`, `restore.start`, `cube.add`, `settings.update`, `login`,
`login.failed`, `logout`, `user.add`, `token.add`, `role.update`,
`capability.disable`, …), `cube` the Cube of the route, `target` the device,
room, user, role, token or Cube concerned, `user` and `token_id` who asked
(absent without authentication, for a failed login,
and for `massimo user` on the box, whose `remote` is `cli`),
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
`limit` is 100 by default and at most 1000; `user` filters by login name.
The rows are append-only; a later stage prunes them by age.

### Commands and Idempotency-Key: GET /api/v1/commands/{id}

Every request that changes something is a **command** in the store
(docs/store.md section 5; docs/server.md, "What version 1 prepares", item 7):
a row with who asked, the Cube,
the action and the target as the audit names them, the request, and a state;
and every protocol command the request puts on the Cube's queue is a **step** beneath it,
written by the queue as it goes.
The audit row of the request names the command in `command_id`.

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

`GET /api/v1/commands/{id}` is one command with its steps:

```json
{
  "id": "01a1b0c2-5a3e-7b1c-9d4e-0f1a2b3c4d5e",
  "action": "device.update",
  "target": "device:0a010d",
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
    "mac": "00:1a:22:0a:0b:0c",
    "last_seen": "2026-11-04T15:00:02+01:00",
    "connected": true,
    "firmware": "1.1.3",
    "rf_address": "0a0104",
    "budget": {"duty_cycle_pct": 4, "free_memory_slots": 50, "updated_at": "2026-11-04T15:28:11+01:00"},
    "pending_radiograms": 0,
    "updated": "2026-11-04T15:28:11+01:00",
    "rooms": 5,
    "devices": 16,
    "queued": 0,
    "cube_time": "2026-11-04 15:28"
  },
  {
    "serial": "KEQ0000000",
    "configured": false,
    "address": "192.168.1.23",
    "mac": "00:1a:22:01:02:03",
    "rf_address": "0a0b0c",
    "firmware": "1.1.3",
    "dhcp": true,
    "found_at": "2026-11-04T14:30:01+01:00",
    "last_seen": "2026-11-04T15:00:02+01:00"
  }
]
```

`configured` tells the two apart.
A configured Cube has the fields below;
`mac` and `last_seen` (when it last answered discovery) only once discovery has seen it.
A found Cube has only the fields shown: `address` is the IP it answered from,
`found_at` when the server first saw it, `last_seen` its latest answer;
`mac` comes from the host's ARP cache and may be missing.
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
Found Cubes are kept in the server's store,
so they are listed again after a restart, with their first and latest sighting.

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
`proxy_listening` (the addresses its proxy listens on now) and `hidden` (the
radio addresses hidden besides room 255).

`budget` is what the Cube last reported:
the share of the hourly radio budget already used,
and the free memory slots (each unacknowledged radiogram occupies one).
`pending_radiograms` is the number of slots in use,
that is commands the Cube has sent and no device has acknowledged yet;
a setpoint shows in the states at once,
but the mode follows only with the device's report,
so a non-zero value says the states may still change.
`queued` is the number of commands waiting in the server.
`observed_since` is when the server first heard from the Cube (see the
ages of device values below).

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

`PUT /api/v1/cubes/{sn}` with any of

```json
{"name": "Ground floor", "address": "192.168.1.9", "follow_discovery": false,
 "proxy_address": "192.168.1.49", "proxy_serial": "LEQ0000000", "hidden": ["0abc12"]}
```

changes those fields and leaves the others; an unknown field is `400`,
so a typo is not taken for "no change".
The row's `source` becomes `ui`, and the change applies at once:
a new address reconnects,
a new proxy address or serial restarts that Cube's proxy listener,
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
and its row stays in the store with `configured` off, with its history.
Discovery lists it as found again, `POST` adds it back,
and a line in the settings file does not bring it back by itself.

A Cube served by serial for this run only (`-sn` without a row)
is not in the store and answers `409` to `PUT` and `DELETE`; `serve -addr`
runs without a store, so these routes answer `503` there.

### GET and PUT /api/v1/settings

The server's settings in the store:

```json
{
  "stale_after":       {"value": "24h", "default": "24h", "set": false, "in_effect": "24h", "applies": "now"},
  "poll_interval":     {"value": "1m",  "default": "1m",  "set": false, "in_effect": "1m",  "applies": "restart"},
  "refresh_interval":  {"value": "6h",  "default": "6h",  "set": false, "in_effect": "6h",  "applies": "restart"},
  "discover_interval": {"value": "30m", "default": "30m", "set": false, "in_effect": "30m", "applies": "restart"},
  "node.id":           {"value": "01a1ae5a-6bee-7ed4-bb4e-3baa2c43fe55", "set": true, "read_only": true},
  "tenant.id":         {"value": "01a1ae5a-6bee-7edb-b3d5-390730e9e1c5", "set": true, "read_only": true}
}
```

`value` is the store's, or the default when `set` is false;
`in_effect` is what this run uses, which a serve flag (`-poll`, `-refresh`,
`-discover-interval`) overrides for one run.
`stale_after` is when a thermostat that has not changed counts as stale,
and applies at once; the intervals apply at the next start.
`commands.keep_days` is how many days finished commands are kept (see Commands),
a number, 0 for ever, and applies at once;
`log.keep_days` how many days the Cubes' events are kept (see `GET …/log`),
30 by default, 0 for ever, at most 3650, and applies at once:
a lower value removes the older events within moments.
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
`PUT` takes an object of keys and durations,
`{"stale_after": "36h", "refresh_interval": null}`,
with `null` for the default and `"0"` to switch the refresh or the periodic discovery off.
A number setting takes a JSON number, or the number as a string,
and `auth.password_login` `true` or `false`.
The ranges: `stale_after` 1h to 720h, `poll_interval` 10s to 1h,
`refresh_interval` 10m to 168h, `discover_interval` 1m to 24h.
Every value is checked before any is written.
The answer is that of `GET`.

### POST /api/v1/cubes/discover

Looks for Cubes on the LAN now (UDP discovery, about three seconds) and
answers with every Cube that answered, configured or not, as the
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

No body.
A call while a run is going waits for it and gets its result, the same `ran_at`.
`503` when the server cannot have the discovery port (UDP 23272,
the only one the Cubes answer to), for example because another program holds it.
`serve` also runs discovery at the start and every 30 minutes (`-discover-interval`,
`0` for on request only),
and every minute while a Cube configured by serial has no address yet.
The first time a Cube that is not configured answers,
every Cube's event stream sends `cube_found`, once per serial and server run:

```
event: cube_found
data: {"kind":"cube_found","cube":{"serial":"KEQ0000000","configured":false,"address":"192.168.1.23",…},"time":"…"}
```

A configured Cube that answers from another address than configured is logged;
one configured by serial only, without an address,
is connected at the address it answers from and reconnected when that changes.

### GET /api/v1/cubes/{sn}

The same object with `config` (portal URL and flag, push button settings,
time zone rules) and `room_list`, the rooms as below.

### GET /api/v1/cubes/{sn}/rooms

Rooms with their devices and live states:

```json
[
  {
    "id": 5,
    "name": "Bad",
    "ready": true,
    "devices": [
      {
        "rf_address": "0a0102",
        "type": "radiator_thermostat",
        "name": "Thermostat Bad",
        "room_id": 5,
        "serial": "JEQ0000502",
        "reported": true,
        "state": {"mode": "auto", "setpoint": 22, "valve_pct": 0, "battery_low": false, "error": false, "link_error": false}
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
`ready` on a room is true when all its devices have reported,
which is the condition for the Cube to accept a room command.
Thermostats carry `mode` (`auto`, `manual`, `vacation`, `boost`), `setpoint`,
`actual_temp` when the device reported one,
`valve_pct` for radiator thermostats, `until` for vacation mode;
shutter contacts carry `window_open`.
`error` is the Cube's transmit error flag,
`link_error` a failed link with a partner device,
`panel_locked` (present when true) the key lock on the device.

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
and the Cube itself changes a setpoint in the list right after a set-temperature command,
before the devices confirm it.
Values that were already there when the server first connected carry that time,
the Cube's `observed_since`, and may be older;
a reconnect keeps the times of values that did not change.
Radiator thermostats report the measured temperature only now and then
(after the valve moved), which is what `received.actual_temp` is for.
`stale` is a hint: a thermostat out of range or with a flat battery goes quiet,
but so may one whose valve stays shut outside the heating season.
Shutter contacts and eco buttons transmit only on events and are never stale.

### GET /api/v1/cubes/{sn}/devices

Every device the Cube knows, in the Cube's order.
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
[portal-protocol.md](portal-protocol.md)).
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

```json
{"mode": "manual", "temp": 21}
```

The request waits for the Cube's answer (up to two minutes when the
radio budget is exhausted) and returns

```json
{"ok": true, "discarded": false, "budget": {"duty_cycle_pct": 5, "free_memory_slots": 49, "updated_at": "…"}}
```

A device that belongs to a room is addressed as its room,
because the Cube discards a direct command to a room member;
every thermostat of that room therefore takes the new setpoint,
which is how MAX! treats setpoints.
A device outside any room is addressed directly.
Boost cannot be started over the air.

### PUT /api/v1/cubes/{sn}/rooms/{id}/setpoint

Same body and answer, addressed to every device of the room, which is
what the original software does.

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
A move is done as the original software does it:
the device is unlinked from every partner of its old room and its group removed,
then linked with every partner of the new room and the new group set,
each after a wakeup of the room concerned, then the metadata is written.
`room_id` must name an existing room; 0 is refused,
a device outside every room would be a device that blocks no room but cannot be
addressed by one either.

A device that is paired but in no room yet (`"unlisted": true`,
such as one just paired, see Pairing) needs `room_id`;
`name` alone answers `409`.
It is added to the room as the original software adds a new device:
after a wakeup of the room (`z:1e,G,<room>`),
one 0x20 add link partner radiogram per room member it links with in each
direction (to a radiator thermostat with the room flag and its room,
to every other device directly, and from the new device to each partner;
two shutter contacts are not linked with each other), then 0x22 set group,
then the metadata with the device listed under its name in the room,
then a re-read of its C line.
A device whose C line names another room (paired long ago and never listed) is
first unlinked from that room with 0x21 remove link partner and 0x23 remove group,
as in a move.

A thermostat put into a room, by an add or a move,
then gets the room's settings, as the original software gives them to a device
it pairs or moves:
0x40 auto with temperature 0, to the device directly,
without the error reset and the retry other writes get;
after a wakeup of the room,
0x11 to every thermostat of the room, the device included,
with the room's comfort, eco, maximum, minimum and window-open temperatures
and window-open minutes, each keeping its own temperature offset;
after a wakeup,
0x12 with the room's boost, descaling and valve settings to the device;
after a wakeup, one 0x10 per day of the room's week programme that differs from
the device's own, then 0x11 to the device once more.
The room's settings are those of its first other thermostat,
radiator thermostats first;
a hidden device is neither linked with the device nor given anything,
nor taken for the room's settings;
where that is a wall thermostat,
the window-open minutes, descaling and valve limits it does not store are the
original software's defaults.
A room without another thermostat has nothing to copy,
and the device gets 0x40 alone;
a shutter contact or an eco switch gets nothing.
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
{"ok": true, "copied": {"temperatures": ["0a010a", "0a0b0c"], "valve": true, "days": 5}, "budget": {…}}
```

with `copied` absent when nothing was copied,
and `"mode_not_set": true` in it when the Cube discarded the 0x40,
which leaves the device's mode as it was and ends nothing:
on 2026-11-04 the Cube discarded a direct 0x40 to a device in a room.
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
as the original software's "delete device" does,
and also removes the device's group, which the original software leaves.
A device in a room, the metadata's or else its C line's,
is unlinked from that room first:
after a wakeup of the room (`z:1e,G,<room>`),
one 0x21 remove link partner radiogram per room member it is linked with,
in each direction (addressed as 0x20 is in an add), then 0x23 remove group.
Hidden devices are no partners,
and a device whose C line names room 255 is not unlinked at all.
Then `t:01,1,<rf>`, delete with the force flag,
which the Cube answers `A:` and which makes it send the device a factory reset;
then, for a device the metadata lists, the metadata without it.
The server's copy forgets the device at once:
it is gone from the devices, the rooms and the counts,
and the stream reports `metadata` (for a listed device) and `device_forgotten`.

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
Seen on hardware for devices that never report (portal-protocol.md);
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
afterwards the server re-reads the device's configuration from the Cube.

### GET /api/v1/cubes/{sn}/rooms/{id}/config

The room's settings as the UI edits them:
the common fields above (without `temp_offset` and `display_actual`) from the
room's first radiator thermostat, else its wall thermostat, named in `source`,
and a `devices` list with each thermostat's `rf_address`, `type`, `name`,
`temp_offset` and, for wall thermostats, `display_actual`.

### PUT /api/v1/cubes/{sn}/rooms/{id}/config

The same body as for a device, without `temp_offset` and `display_actual`,
applied to every thermostat of the room after one wakeup:
temperatures to radiator and wall thermostats, boost to both,
descaling and valve limits to radiator thermostats only.
The answer lists every device:

```json
{"ok": true, "devices": [{"rf_address": "0a0102", "name": "…", "ok": true, "discarded": false}], "budget": {…}}
```

`ok` is true when every device took the write; `502` only when none did.

### GET /api/v1/cubes/{sn}/rooms/{id}/program

The week programme of the room's first radiator thermostat (else its wall thermostat),
keyed by weekday as in a backup, plus `source`.

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
Both answer like a setpoint.

### PUT /api/v1/cubes/{sn}/devices/{rf}/lock

Lock or unlock the buttons of one thermostat (the key lock,
"LoC" on the display, `panel_locked` in its state).
Body `{"locked": false}`; the answer is the same as for a setpoint.
The command is radiogram 0x80,
which the original software lists but never sends;
it is sent directly to the device after a wakeup of its room and takes effect
immediately (both directions verified on radiator thermostats).
On the device itself the lock is toggled by pressing Auto/Manu and the second
button (Boost or the moon/sun key,
Mode/Menu and Boost/OK on a wall thermostat) together.

### Pairing: POST, GET, DELETE /api/v1/cubes/{sn}/pairing

`POST` with `{"seconds": 60}` (optional; 1 to 600,
default 60) puts the Cube into inclusion mode with `n:<seconds>`;
a device whose pairing button is pressed then joins.
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
Capability `backup`.

#### POST /api/v1/cubes/{sn}/backups

Take a backup now, as `GET …/backup` takes it,
and keep it as `manual` with the user and an optional note,
`{"note": "before the holidays"}` (at most 200 characters;
another field answers `400`).
`201` with the backup as the list shows it;
`503` while the Cube is not connected.
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
Cube is not connected.

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
`413` and `503` as for the plan.
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

### GET /api/v1/cubes/{sn}/log?before=&since=&limit=&kind=&device=

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
Without parameters the answer is the newest 200 events; `limit` takes 1 to 1000,
`before` an event's `id` for the page before it (`?before=4711`),
or an RFC 3339 time for the events before that time;
`since` an RFC 3339 time for the events at or after it
(`?since=2026-11-06T20:00:00Z`, not after a `before` time),
so that a view of the last hour or of today is one request,
with `before` as the cursor within it;
`kind` keeps one kind (`state`, `config`, `connected`,
…) and `device` one radio address.
A client pages back with `before` set to the `id` of the oldest event it has,
until a page is empty: the events of a burst share a millisecond,
so a time does not page them without gaps or repeats.
The stored copy of a `state` event leaves out `state.received`,
the per-value times the tiles' ages read (`GET …/devices/{rf}` has them);
the event's own `time` is the log's.
A Cube served for this run only, without a row in the store (`serve -addr`,
or `-sn` before discovery noted it), keeps its last 500 events in memory,
without `id`, as before the store;
`before` then takes a time, and an `id` answers an empty page;
`since` works there as in the store.
Such a Cube that gains its row while it runs, as discovery notes it,
answers from the store from then on,
and the events it kept in memory leave the log.
A device hidden later keeps its events in the log until `log.keep_days` removes them.
A page before a time first looks up the newest event before it,
which reads the Cube's times up to there: an `id` is the quicker cursor.
A page since a time looks up the oldest event at or after it the same way.
`400` for a bad `before`, `limit` or `device`.

### GET /api/v1/cubes/{sn}/events

A `text/event-stream`.
On connect, one `state` event per device with its current data;
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
`state`, `pairing_started`, `paired` (with `paired`, the device that joined),
`pairing_ended` (see Pairing),
`device_forgotten` (a device deleted from the Cube, through the API or a client
of the proxy, left the server's copy), `cube_found` (with `cube`,
a Cube that is not configured, see POST /cubes/discover),
`restore_step` and `restore_done` (with `restore`, the progress of a restore,
see POST /restore).
A comment line is sent every 30 seconds to keep the connection alive.

A `state` event that follows an earlier one for the same device also says what changed,
whoever changed it, the server, another client or someone at the device:

```
event: state
data: {"kind":"state","device":"05cdc6","state":{…},
       "changes":{"window_open":{"from":false,"to":true},"setpoint":{"from":18,"to":5}},"time":"…"}
```

`changes` has one entry per value of `state` that differs (the same names:
`mode`, `setpoint`, `actual_temp`, `valve_pct`, `window_open`, `until`,
`battery_low`, `error`, `link_error`, `panel_locked`), with `from` and `to`;
a value the device did not have is `null`,
so a measured temperature the thermostat stopped reporting is
`{"from": 19.9, "to": null}`.
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
at most a minute later (`serve -poll`);
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

### GET /api/v1/update?refresh=1

Whether a newer signed release is out on the server's update channel (the branch
it was built from, or `MASSIMO_UPDATE_CHANNEL`).
The server asks GitHub at most every ten minutes; `refresh` asks now.

```json
{
  "current": "v0.1.0-alpha.9",
  "channel": "alpha",
  "latest": "v0.1.0-alpha.10",
  "release_url": "https://github.com/metaworx/massimo/releases/tag/v0.1.0-alpha.10",
  "available": true,
  "checked_at": "2026-11-05T13:40:00+01:00",
  "can_update": true,
  "helper": {"state": "done", "from": "v0.1.0-alpha.8", "to": "v0.1.0-alpha.9", "channel": "alpha", "time": "…"}
}
```

`latest` is only a release whose manifest verifies against the key compiled into
the server (see `internal/update`).
`check_error` says why a check failed.
`can_update` is true when POST would start an update;
otherwise `blocked` says why it cannot be started from here (a package
manager owning the binary, which then also upgrades it; no API token;
no update helper; a development build).
`helper` is the update helper's last run: `state` is one of `checking`,
`installing`, `restarting`, `done`, `up-to-date`, `failed`, `rolled-back`,
with `message` on failure.

### POST /api/v1/update

Asks the update helper to install the newest release on the channel:
the server writes the channel into its request file,
and the helper (started by `massimo-update.path` as root on Linux,
by launchd's `ch.metaworx.massimo-update` on macOS,
by the server itself as a detached process on Windows;
see [packaging/README.md](../packaging/README.md)) downloads and verifies the release,
test-runs it, replaces the binary,
restarts the server and rolls back if the new version does not answer `/healthz`.
The request names no version and no file,
so the most the API can do is install the newest signed release.
Answers `202` with `{"ok":true,"requested":"v…", "channel":"…"}`;
follow the progress with GET.

Needs the API token: without `MASSIMO_API_TOKEN` it answers `403` even though
the other routes are open.
`403` also when no helper is configured (`serve -update-request`);
`409` when already up to date,
when the channel has no release or while an update is running (on Windows also
in the moment between the start of the helper and its first status);
`500` on Windows when the helper could not be started (the request is withdrawn);
`502` when GitHub could not be asked.

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
at the start or since.
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
every other setting that changed applies at the next start,
and the store is checkpointed.
`serve` also reads the file by itself within seconds of a change,
so this is for not waiting, and for systems without SIGHUP.
Needs the capability `restart`, and answers what it found:

```json
{"changed": ["MASSIMO_LISTEN"], "pending": ["MASSIMO_LISTEN"], "read_at": "2026-11-08T19:52:00Z"}
```

`changed` names the variables this read changed, `pending` those that now
differ from the start and wait for the next one; names, never values.
It sends nothing to a Cube, and is an audit row (`server.reload`) but no
command, so a retry is harmless.
`409` when the server reads no settings file,
`500` when the file could not be read.

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
from (`environment` or the settings file).
A key's, an IV's, a token's or a password's value never leaves the server:
such a variable has `"secret": true` and no `value`;
a name Massimo does not know has `"unknown": true`.
`503` when the server has no store, or does not show its settings.

## The LAN proxy

Besides the API, `massimo serve` answers as a Cube on TCP port 62910 and UDP discovery,
so that programs speaking the Cube protocol share the one connection the Cube allows.
See [proxy.md](proxy.md).

## Not in version 1

Removing devices, creating rooms, and ownership per Cube.
Removing needs the original software for now; `massimo restore` or
`POST …/restore` from an edited backup covers everything else in bulk.

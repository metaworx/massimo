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
| `preferences` | keep one's own settings and colour schemes on the server, so that they follow the user across browsers, and be notified; without it they stay in the browser | `PUT`/`DELETE /api/v1/me/preferences/{key}`, `/api/v1/me/push` |
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
| `cubes.reboot` | reboot a Cube, which drops the radiograms it has not delivered; seeded for no role | `POST /api/v1/cubes/{sn}/reboot` |
| `settings` | the server's settings, its store and the settings in effect | `GET`/`PUT /api/v1/settings`, `GET /api/v1/store`, `GET /api/v1/config` |

A user changes only users whose role has no capability its own lacks,
and gives only such roles: an owner changes everyone,
nobody but an owner changes an owner or gives the role `owner`,
and a role made with `users` but without, say, `restore` changes no manager.
The users routes say what the caller may do with each user they list
(`"may": {"manage": true, "password": true}`).
| `update` | start a software update | `POST /api/v1/update` |
| `restart` | restart the server through the service manager that runs it, and read its settings file again | `GET`/`POST /api/v1/restart`, `POST /api/v1/reload` |
| `reboot` | reboot the computer the server runs on, where its owner switched that on; seeded for no role | `GET`/`POST`/`DELETE /api/v1/reboot` |
| `audit` | read the audit | `GET /api/v1/audit` |
| `auth` | the OpenID Connect providers users sign in through; seeded for no role | `GET`/`POST /api/v1/providers`, `PUT`/`DELETE /api/v1/providers/{key}` |

The default roles and their sets are those of docs/store.md section 6:
`owner` has every capability and cannot be changed,
`manager` is `operator` plus guests, pairing, names, structure,
restore and the restart,
`operator` runs the heating, locks, configures, discovers and takes backups,
`guest` is `operator` without the lock, backups, discovery and preferences,
`viewer` reads, and `integration`, fixed like `owner`, is the role of the
registered integrations and their users and the ceiling of their calls
(docs/store.md section 6); a regular user is never given it.
A role's set can be changed, a role renamed, created and disabled,
and a capability disabled for every role but `owner`,
through the routes of `/api/v1/roles` and `/api/v1/capabilities`.
A user's own user and the routes of `/api/v1/users/{name}` with the user's
own name are open to it for its display name and its password.

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
- `{rf}` names a device: by its serial number, as printed on it (`MEQ0000101`),
  case not mattering, or by its radio address, six hex digits (`0a0102`);
  the two cannot be confused.
  A serial reaches a device while the Cube has it, and a serial the Cube
  does not have is `404`.
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

What a key may do is not here but in `GET /api/v1/me`.
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
The name of a removed user can be used again,
for a new user in a row of its own,
which starts without the old one's preferences or history.

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
and its row stays for the audit under the name `<name>-del-YYYYMMDD-HHNN`
of the removal in UTC (`-<id>` added where that is taken),
which frees the name; `{"ok": true, "name": "anna"}`.
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

### Identities: /api/v1/users/{name}/identities

`GET` and `POST /api/v1/users/{name}/identities`;
`DELETE /api/v1/users/{name}/identities/{id}`.
A user's identities at the OpenID Connect providers
("Sign-in through OpenID Connect" above),
with the rules of access of the users routes:
`users`, or `guests` for a guest's;
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
that chain allows (AP IntegrationUsers; "Acting for an integration's
users", below, once that block is in).
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
   and the setting `integrations.requests` `"off"` refuses requests (`403`).
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

The key belongs to the integration's own user, named by the integration's
`key` (lower case, from its name: `casa-leone`), with the role
`integration`; the user `<key>.system` stands for its calls that no user
started.
`GET /api/v1/integrations` lists the integrations and the requests
waiting, a request with its code and the address it came from;
`PUT /api/v1/integrations/{uuid}` changes `name`, `max_caps` and `state`
(`active`, `disabled`);
`DELETE` revokes it: its key revoked and the integration disabled,
its users, links and maximum kept for a request from the same installation.
A call that came with another installation id than the accepted one is
refused and the id recorded as `pending_instance`, which
`POST …/accept-instance` accepts.

### Acting for an integration's users

A registered integration's key with the header

```
X-MassimoHA-On-Behalf-Of: id=<the user's id, 32 hex>; level=control; name*=UTF-8''J%C3%BCrg; instance=<installation id, 32 hex>
```

acts for one of the integration's users.

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
  named `<integration key>.<the id's first 8 characters>`
  (longer where two ids begin alike).
  Several of the integration's users may be linked to one Massimo user.
- **What the call may do** is what every link of the chain allows:
  1. the integration user's role, `integration` at first,
     narrowed by the owner where wanted;
  2. the role `integration`, the ceiling of every call through an integration;
  3. the Massimo user it is linked to, where it is;
  4. the integration's maximum, `max_caps`;
  5. the level: `view` reads, `control` also sets, locks and configures,
     and `admin` adds nothing of its own.
- **Without the header** the key acts as the integration itself:
  its role and its maximum (an integration's own polls and stream).
- **The audit and the log** name the user the call acted as,
  with `"via"`, the integration's key,
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
(`uuid`, `key`, `name`, `state`, `instance_id`, null for any other key),
with the header `acts_for` (`id`, `name`, `level`, `linked`,
`system_generated`), and `capabilities`, what the chain leaves.
So an integration knows before naming a user whether its key is a
registered integration's, and a key typed by hand keeps working without
the header.

**Links.**
`GET /api/v1/integrations/{uuid}` answers the integration with `users`,
its users, the system user first:
`id`, `name`, the `user` its calls act as, its `own_user`, `linked`,
`system_generated`, `last_level`, `last_used_at`,
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
(`key`, `uuid`, and for one of its users `id`, `name`, `linked_to`)
and lists under `links` the integrations' users linked to a user.

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
 "frost": {"on": true, "below_c": 8}, "battery_low": {"on": true}, "device_error": {"on": true},
 "device_stale": {"on": false}, "paired": {"on": false}, "cube_found": {"on": false}, "update": {"on": false}}
```

| Kind | Told when | Default |
|---|---|---|
| `cube_offline` | the Cube disconnected and is not back within `after_min` | on, 10 min |
| `cube_online` | the Cube is back after a `cube_offline` was told | on |
| `window_open` | a window opened and is still open after `after_min` | on, 30 min |
| `battery_low` | a device's battery went low | on |
| `device_error` | a device reports an error or a link error | on |
| `frost` | a device's measured temperature fell under `below_c`; again once it rose half a degree above | on, 8 °C |
| `device_stale` | a thermostat went past the stale threshold (`stale_after`) | off |
| `paired` | a device joined by pairing | off |
| `cube_found` | discovery found a Cube that is not configured | off |
| `update` | `GET /api/v1/update` saw a newer release | off |

`after_min` is 1 to 1440, `below_c` −10 to 20.
A `PUT` of a document the rules cannot read (an unknown kind,
a value out of bounds, `after_min` on a kind without a delay) is `400`.
One kind is told once per device per half hour;
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
`version` (`update`).

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
    "found_at": "2026-11-04T14:30:01+01:00",
    "last_seen": "2026-11-04T15:00:02+01:00"
  }
]
```

`configured` tells the two apart.
A configured Cube has the fields below;
`mac` and `last_seen` (when it last answered discovery) only once discovery has seen it.
A found Cube has only the fields shown: `address` is the IP it answered from,
its own where a massimo proxy answers for it too (docs/proxy.md),
and `proxies` the other addresses that answered for its serial in the latest run;
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
that is commands the Cube has sent and no device has acknowledged yet;
a setpoint shows in the states at once,
but the mode follows only with the device's report,
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
and its row stays in the store with `configured` off, with its history.
Discovery lists it as found while it answers, `POST` adds it back,
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
  "update.channel":    {"value": "alpha", "default": "alpha", "set": false, "in_effect": "alpha", "applies": "now"},
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
`update.channel` is the update channel (see `GET /api/v1/update`),
one of `devel`, `nightly`, `alpha`, `beta` and `stable`,
the build's flavour by default, and applies at once;
`MASSIMO_UPDATE_CHANNEL`, where set, is used instead, as `in_effect` shows.
`PUT` takes an object of keys and durations,
`{"stale_after": "36h", "refresh_interval": null}`,
with `null` for the default and `"0"` to switch the refresh or the periodic discovery off.
A number setting takes a JSON number, or the number as a string,
`auth.password_login` `true` or `false`, and `update.channel` a name.
The ranges: `stale_after` 1h to 720h, `poll_interval` 10s to 1h,
`refresh_interval` 10m to 168h, `discover_interval` 1m to 24h.
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

A configured Cube that answers from another address than configured is logged;
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
`actual_temp` once the device reported one (the last it reported, kept while
its entries carry none),
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
(after the valve moved), and not at all on vacation, when those bytes carry the end:
the server keeps the last one until a new one comes,
and `received.actual_temp` says how old it is.
`stale` is a hint: a thermostat out of range or with a flat battery goes quiet,
but so may one whose valve stays shut outside the heating season.
Shutter contacts and eco buttons transmit only on events and are never stale.

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
 "sources": {"actual_temp": ["0a0102", "0a0103"], "mode": ["0a0104"]}}
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
  else the wall thermostat's, else that of the radiator thermostat that reported last,
  with `setpoint` and `until` from the same one, named in `sources.mode`
  (the boosting ones during a boost).
  `consistent` is false while the room's thermostats disagree on mode,
  setpoint or `until`.
- `valve_pct` is the highest of the radiator thermostats',
  `window_open` true while any shutter contact of the room is open;
  both are given so that a client need not compute them.
  `battery_low`, `error`, `link_error` and `panel_locked` are the real wall
  thermostat's; the radiator thermostats show their own.
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
(`wall_thermostat`, from the room's wall thermostat, else its first radiator
thermostat) and its own `settings`:
`{"temp_method": "median"}`, and `rf_address` where the owner set one.

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
{"ok": true, "discarded": false, "budget": {"duty_cycle_pct": 5, "free_memory_slots": 49, "updated_at": "…"}}
```

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
keyed by weekday as in a backup, plus `source`, its tag `etag` (also the `ETag` header),
`pending` and `program_consistent`.
`GET …/devices/{rf}/program` gives one thermostat's the same way;
a virtual wall thermostat's is its room's.

**The programme's tag.**
An editor that read a programme sends its tag back with its write,
in `If-Match` (quoted, `W/` taken, `*` for any),
and a programme changed since, in another client or at a device,
is refused with `412`, whose body and `ETag` carry the current tag,
instead of being overwritten unseen; nothing is sent then.
A write without `If-Match` is unconditional, as before.
The tag covers every thermostat of the room,
since a write to the room or to one member changes the room:
a member's tag is its room's,
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
`id` names one backup for good:
the store never gives it to another, also once the backup is deleted or pruned.
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

### GET /api/v1/cubes/{sn}/log?before=&since=&limit=&kind=&source=&level=&device=&user=&command=

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
(`{"name": "anna", "display_name": "Anna"}`):
absent for the Cube, the server's own jobs and a server without users;
the bootstrap token is a key of the first owner, whom it names, as the audit does.
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
`400` for a bad `before`, `limit`, `source`, `level`, `device` or `command`.

### GET /api/v1/log?before=&since=&limit=&kind=&source=&level=&device=&user=&command=

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

Pages go back by `id` across the Cubes, as in a Cube's log.
A serial number as `device` names one device;
a radio address names the device of that address on every Cube,
since each Cube gives out its own.
A Cube without a row in the store yet adds nothing.
Without a store, each Cube's last 500 events, merged by time, without `id`.
A Cube's own log does not carry `cube_serial`.

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
`state`, `pairing_started`, `paired` (with `paired`, the device that joined),
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
variables `changed` and of those waiting `at_next_start`, never their values),
`restart`, `reboot`, `reboot_cancelled`, `update` (`from`, `to`, `channel`),
`update_result` (the helper's outcome after the restart, once:
`state`, `from`, `to`; a failure at `warn` with its `error`),
`backup` (`reason`, `id`, `bytes`; an automatic one that failed at `warn`),
`backup_deleted` (`id`), `backups_pruned`, `log_pruned` and `audit_pruned` (`removed`),
`log_rotated` (serve's `-log` file rotated: `why`, `size`, `format`, `sighup`
or `reload`; `previous`, the name the old file has now; `kept` and `removed`,
the rotated files kept and those deleted; at `warn` with its `error` where a
step failed),
`restore_started` (`id`, `steps`, `radio_commands`).
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
discarded it, the device's error state was reset and it was sent again),
`dropped` and `failed` with their `error`;
`hold` at `warn`, the radio commands waiting (`reason` `duty_cycle` with
`duty_pct` and `until`, or `memory_slots` with `free_slots` and `wait_s`).
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
`not_understood`), `proxy_pairing` (`seconds`, or `joined`)
and `proxy_warning` at `warn` (`what`: `pairing_not_started`,
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
A failure the server reports, such as a reboot the helper did not start,
is its kind's line at `warn` with the `error`.
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
    {"serial": "KEQ0000000", "addr": "192.168.1.31:62910", "listening": true},
    {"serial": "KEQ0000001", "addr": "192.0.2.10:62910", "listening": false,
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
by Cube serial and address;
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
  "helper": {"state": "done", "from": "v0.1.0-alpha.8", "to": "v0.1.0-alpha.9", "channel": "alpha", "time": "…"}
}
```

`latest` is only a release whose manifest verifies against the key compiled into
the server (see `internal/update`).
`check_error` says why a check failed.
`manager` is the package manager that owns the binary, `apt`,
where the Debian package installed it, or `brew` under Homebrew's service;
absent on a box `massimo install` set up.
`can_update` is true when POST would start an update;
otherwise `blocked` says why it cannot be started from here (a package
manager owning the binary without a helper of its own, which then upgrades it;
for apt, its source missing, `MASSIMO_UPDATE_CHANNEL` naming another channel
than its suite, or the package's helper turned off;
no API token; no update helper; a development build).
`helper` is the update helper's last run: `state` is one of `checking`,
`installing`, `restarting`, `done`, `up-to-date`, `failed`, `rolled-back`,
with `message` on failure.

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

### GET /api/v1/reboot, POST /api/v1/reboot, DELETE /api/v1/reboot

The reboot of the computer the server runs on, for the capability `reboot`,
which no default role but `owner` has:
restarting the server is not rebooting a computer that may run other services.
It needs the owner's switch, `MASSIMO_REBOOT=on` in the settings file
or `serve -allow-reboot` (README, "serve"),
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

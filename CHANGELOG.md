# Changelog

All notable changes to Massimo are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions
below 1.0.0 are pre-releases, tagged `vX.Y.Z-alpha.N` by
`make release-alpha` (`README.md`, "Versions"). A new feature starts
the next minor, a fix counts the suffix up.

## [Unreleased]

### Added

- The event stream gives every event an id and resumes after the last
  one a client read, or says `resync`; the web UI resumes with it
  (`docs/api-v1.md`, "GET …/events").

- `massimo -no-env` reads no settings file, as `-env /dev/null` did,
  also on Windows (`README.md`, "Cube variables").

- `packaging/acme-dns-hooks/cloudflare.sh` is the DNS-01 hook of
  `serve -acme-dns-hook` for a zone at Cloudflare, with its token in
  `/etc/massimo/cloudflare.token` (`packaging/README.md`).

- The Debian package installs the DNS-01 hooks for Infomaniak and
  Cloudflare as `/usr/share/massimo/acme-dns-hooks/*.sh`
  (`packaging/README.md`, "Debian package").

- `massimo install -acme-directory <url>` sets the CA's directory for
  `-tls acme`, such as Let's Encrypt's staging one for a first try
  (`packaging/README.md`, "Install with `massimo install`").

- `internal/store/migrations/store.sql` is the store's complete schema,
  made from the migrations beside it and checked by `go test`
  (`docs/store.md`, "Migrations").

- The store's tests migrate a store written by each release since
  v0.7.0-alpha.1, kept as text dumps that `tools/store-fixture` makes
  (`docs/store.md`, "Migrations").

- The log keeps the server's own events beside the Cube's, each with
  its source, its level and the user who started it; the store's
  migration 0009 rebuilds its events table once (`docs/store.md`).

- The store's migration 0011 indexes the log's warnings and errors,
  so that a page of them reads those rows only, not the whole log:
  11 s at 90,000 events on a Raspberry Pi 1 before (`docs/store.md`).

- Benchmarks of the log's writer and pages in `internal/store`, also on
  a copy of a box's store, and `make store-bench` builds them for the
  Raspberry Pis (`docs/store.md`, "Measuring on a box").

- The log shows the server's start, stop, restart, reload, reboot,
  update, backups and prunes, and a Cube's new address or its leaving
  the LAN (`docs/ui.md`, "Log").

- Every change made through the API or `massimo user` is in the log,
  with who made it: a Cube named, a setting, a user, a role, a key
  (`docs/ui.md`, "Log").

- The log chooses a source, System, Configuration, the Cube or one
  device, a level and who made a change, kept in the address after
  `#log` (`docs/ui.md`, "Log").

- `serve` announces itself on the LAN by mDNS as `_massimo._tcp`
  (`-announce`, `MASSIMO_ANNOUNCE`); `massimo discover` lists the servers
  and marks a proxy among the Cubes (`README.md`, "Server").

- `ui/connect.html`, the apps' page: the servers, discovery, the
  fingerprint of a self-signed certificate, the biometric and
  notification switches; a browser sees it as the app's (`docs/app.md`).

- The web UI's Update works on a box the Debian package installed: its
  helper, enabled, upgrades massimo through apt from the massimo source
  alone (`packaging/README.md`, "Debian package").

- The web UI's Update works under Homebrew's service too: it runs
  `brew upgrade` as the service's user and restarts the job
  (`packaging/README.md`, "Homebrew").

- `serve -discover-listen <addr>` puts the discovery socket on another
  address than UDP 23272, for tests with a fake Cube (`README.md`).

- Notification rules on the Cubes' events, per user in the `notify`
  preference, and push subscriptions under `/api/v1/me/push` with the
  server's VAPID key at `GET /api/v1/push/vapid` (`docs/api-v1.md`).

- A notification is pushed by Web Push (RFC 8291, 8292) to the user's
  browsers and devices, UnifiedPush's too, and sent on the event stream
  as `notification` (`docs/api-v1.md`, "Notifications").

- 👤 → Notifications chooses which events notify and turns them on for
  this browser, and the page installs as an app, with a manifest and a
  service worker (`docs/ui.md`, "Notifications").

- The log says when Let's Encrypt issued or renewed a certificate by
  TLS-ALPN-01 or HTTP-01, as it does for DNS-01; `-tls-http` redirects
  to port 443, the one a router forwards (`README.md`, "Server").

- CI runs the tests on 32-bit ARM under QEMU, as the Raspberry Pi 1's
  core, and `make test-armv6` does the same where `qemu-arm` is
  installed (`GUIDELINES/project/CI.md`).

### Changed

- The web UI's week programme editor is an element of its own,
  `<massimo-week-program>`, for a Home Assistant card to use too; it
  works as before (`docs/ui.md`).

- The role `integration` may configure: week programmes, temperatures,
  device settings; migration 0012 adds it where the role is still the
  seeded one (`docs/store.md` section 6).

- The Homebrew caveats and the settings template ask for nothing to fill
  in and name the first owner; `brew services restart` follows
  `brew upgrade` (`packaging/README.md`, "Homebrew").

- `massimo install` restarts a running service also on the template's
  settings and leaves a box that never ran for its owner, naming the web
  UI and a `massimo user add` line that runs anywhere (`README.md`).

- `MASSIMO_REBOOT` and `-allow-reboot` take on, true, yes, 1 or enabled,
  and their opposites, in any case, as `serve -help`, the settings
  file's template and the READMEs say (`README.md`, "Server").

- `massimo discover -json` prints an object with `cubes` and `servers`,
  no longer a list of Cubes; a serial heard from a Cube and from its
  proxy is listed twice (`docs/proxy.md`).

- `serve`'s wiring is the package `internal/serve`, which a program
  embedding the server calls as the command does; nothing changes for
  `massimo serve` itself.

- CI runs all four cases of the web UI suite on a pull request too, in
  four workers (`GUIDELINES/project/CI.md`, section 2).

- The log's metadata and config lines say what changed, and a reconnect
  is one line with the Cube's devices, rooms, firmware and duty cycle;
  the greeting's own lines are kept at debug (`docs/ui.md`, "Log").

- The apt repository's address is `https://massimo.metaworx.ch/apt`,
  under the project's site: the install routes, building from source,
  the first settings and the privacy policy (`packaging/README.md`).

- `make mdns-test` checks the mDNS announcement and `discover` against
  avahi in Docker, and CI against mDNSResponder and Windows' mDNS as well
  (`tools/mdns-test/README.md`).

### Fixed

- The Debian package restarts a running service on an upgrade, also on
  the template's settings, and leaves a box that never ran stopped, even
  when a release changed the template's comments (`packaging/README.md`).

- A device list or a configuration the Cube client asked for is in its
  state when the command returns, so that a read right after sees it;
  a race test found the other order now and then.

- System settings' ↑ takes the page all the way to the top, where it
  could stop a few pixels short as the header grew back (`docs/ui.md`,
  "System settings").

- A Cube removed under System settings → Cubes is listed as found and
  not configured, no longer as a configured Cube that is disconnected,
  once discovery had heard it or serve had restarted since its add.

- A search, and the periodic discovery, list only the Cubes that
  answered it: one that no longer answers leaves the Cube choice and
  System settings → Cubes (`docs/api-v1.md`, "GET /api/v1/cubes").

- The log names a device by its S/N, with the radio address in expert
  mode, and a removed one by its type, S/N and name as it was; device
  routes take the S/N too (`docs/api-v1.md`, "Identifiers").

- A build from a release tag's checkout follows the tag's update
  channel, such as `alpha`, no longer `HEAD`, which has no releases
  (`README.md`, "From source").

- A self-signed certificate is valid 825 days in all, as Apple requires
  of one it trusts; the hour it is dated back made it 825 days and an
  hour (`README.md`, "Server").

- `massimo user` and `massimo store` print their help, or refuse wrong
  arguments, without creating a store in the data directory first.

- As the service user, without `-env`, the commands read
  `/etc/massimo/env` and the store in `/var/lib/massimo` from any
  directory, not a home's `.env` (`README.md`, "Cube variables").

### Security

- On Linux the service runs as `_massimo`, as on macOS: the package and
  `massimo install` rename their own `massimo`, spare a person's, and
  name the `chown` after an older takeover (`packaging/README.md`).

- `/var/lib/massimo`, the store, stays 0750 when the service starts;
  systemd made it 0755 at every start, so that every user of the box saw
  the names of its files (`packaging/README.md`).

- The web UI shows no self-signed certificate's fingerprint, which a man
  in the middle passes along; its notice, also in the sign-in dialog,
  asks to compare the browser's with `massimo tls show` (`docs/ui.md`).

## [0.12.0-alpha.1] - 2026-11-08

### Added

- System settings → Maintenance reboots the computer, after a delay
  of 60 s or the one given, and takes the reboot back meanwhile
  (`docs/ui.md`, "System settings").

- `massimo install -reboot-helper` installs the root helper that
  reboots the computer; the Debian package ships it disabled
  (`packaging/README.md`, "Rebooting from the web UI").

- `serve -allow-reboot`, or `MASSIMO_REBOOT=on` in the settings file,
  lets the owner reboot the computer: under systemd and launchd through
  a root helper, as the Windows service by itself (README, "serve").

- `GET`/`POST`/`DELETE /api/v1/reboot` reboot the computer after a
  delay, for the capability `reboot`, which only the owner holds
  (`docs/api-v1.md`, "GET /api/v1/reboot, …").

- A user with keys only sets its first password in My profile, without
  a current one (`docs/ui.md`, "My profile").

- `GET /api/v1/store` and `GET /api/v1/config` show what `store info`
  and `config` print, secrets left out; System settings → Maintenance
  shows them (`docs/api-v1.md`, "GET /api/v1/store, GET /api/v1/config").

- `POST /api/v1/reload` reads the settings file again at once, as SIGHUP
  does; System settings → Maintenance holds it and the restart
  (`docs/api-v1.md`, "POST /api/v1/reload").

- Fuzz targets for the parsers of untrusted bytes, their seeds run by
  `go test`; `make fuzz` and a Fuzz job on pushes to `main` run them
  (`GUIDELINES/project/CI.md`, section 2).

- The log's range, kind and device are in the address after `#log`, so
  that a reload, a bookmark or a link opens the log with them
  (`docs/ui.md`, "Log").

### Changed

- My settings keeps "only on this device" in a column of its own at
  the right end of the settings' rows (`docs/ui.md`, "My settings").

- Expert mode shows a device's serial number beside its radio address,
  "S/N … · RF …", in the rooms, a room's configuration, pairing and the
  hidden devices (`docs/ui.md`, "Rooms").

- The Cube choice is a menu: closed the Cube's name and its connection
  as a plug, open each Cube's serial and address; the "connected" label
  is gone (`docs/ui.md`, "Header").

- The 👤 and ☰ menus show an icon per item, and the 👤 button's tooltip
  names who is signed in (`docs/ui.md`, "Header").

- The header stays at the top while the page scrolls, compacted, and
  System settings' title stays under it with a menu of its sections
  (`docs/ui.md`, "Header").

- System settings is a card per part at full width, Software update
  first, with a row of section buttons, cards that fold and stay folded,
  and ↑ (`docs/ui.md`, "System settings").

- The tests' fixtures and the recorded greeting carry placeholder radio
  addresses too, and `tools/identifiers` fails `go test` on a real
  serial, MAC or radio address in the tree (`AGENTS.md`).

- CI runs the race detector on `internal/server`, `store`, `cubelan` and
  `proxy` in a Race job beside Vet and test; `make race` runs it
  locally (`GUIDELINES/project/CI.md`, section 2).

- The workflows pin their actions by commit SHA; Dependabot proposes
  their updates and the Go modules' monthly, its commit messages left
  unchecked (`GUIDELINES/project/CI.md`, "Dependabot").

- Dependencies updated: actions/checkout v7.0.1, setup-go v7.0.0,
  upload-artifact v7.0.2, download-artifact v8.0.2,
  upload-pages-artifact v5.0.0 and deploy-pages v5.0.1.

- The update check and an update's download try again twice, after 2
  and 5 seconds, when GitHub answers 502, 503 or 504, gives no answer
  within 10 seconds or the connection fails before an answer.

- With `MASSIMO_GITHUB_TOKEN` set, a release's manifest, signature and
  binary come through GitHub's API as its list does, not from
  github.com's download pages (`packaging/README.md`, "Updating").

### Fixed

- A user's or role's head in System settings shows the name and the
  login or key apart, and the badges at the right edge, `you` first
  (`docs/ui.md`, "System settings").

- The backup list no longer shows a pending change next to the backup
  that has just covered it (`docs/api-v1.md`, "GET …/backups").

- Deleting a Cube's newest backup makes the one now newest the
  reference of its automatic backup, and deleting its last one takes a
  new one at once, without a restart of `serve`.

- A prune that removes a Cube's newest backup, as `backups.keep_newest`
  0 allows, does the same.

- A metadata block without bytes after its device list, as a LAN client
  may send it, gets none added when the server or the proxy writes it
  back (`internal/protocol/cube`).

- A stored password hash of another argon2 version, or with a salt under
  8 bytes, is refused as malformed, as one over the ceiling is
  (`docs/store.md` section 6).

- A Cube's duty cycle or free memory slots over `ffff` are refused
  rather than read as negative on a 32-bit system, as code scanning
  warned; the sign-in's key stretching reads its lanes at 8 bits too.

### Security

- A user changes only users whose role has no capability its own lacks,
  so nobody but an owner changes an owner; a manager sets other users'
  passwords with `passwords` (`docs/api-v1.md`, "Authentication").

## [0.11.0-alpha.1] - 2026-11-08

### Added

- The web UI's log has a choice of how far back: the last 20 events by
  default, 10 minutes, an hour, today, this week, this month or all
  (`docs/ui.md`, "Log").

- The store keeps each Cube's backups, gzip, pruned by reason and age
  as the `backups.*` settings say (migration 0005; `docs/store.md`
  section 9).

- The server backs a Cube up after a change, by hand and before every
  restore, and `…/backups` lists, downloads, compares and deletes the
  backups, which a restore takes by id (`docs/api-v1.md`, "Backups").

- System settings → Backup and restore in the web UI: the download, the
  server's backups with compare, restore and delete, and a restore from
  a file that shows its plan first (`docs/ui.md`, "System settings").

- The web UI warns under its header while the server is open, and says
  what closes it (`docs/ui.md`).

- `POST /api/v1/restart` restarts the server for the capability
  `restart` of owner and manager (migration 0006), an open server only
  from the box itself (`docs/api-v1.md`, "POST /api/v1/restart").

- System settings → Software restarts the server, follows it until it
  is back, and lists the settings that wait for a restart (`docs/ui.md`).

- `serve` reads its settings file again when it changes and on SIGHUP
  (`-sighup`), so `MASSIMO_API_TOKEN` applies without a restart
  (`README.md`, "Server").

- `serve -restart` (`MASSIMO_RESTART`) says when the server restarts
  for the API, stopping and exiting 75 for its service manager; `massimo
  restart` restarts the installed server (`README.md`, "Server").

- The web UI's header has a 👤 menu: who is signed in, My profile, My
  settings and Sign out (`docs/ui.md`, "Header").

- The web UI's settings are three pages: My profile with the own name,
  password and keys, My settings, and System settings behind ⚙
  (`docs/ui.md`, "My profile", "My settings", "System settings").

- The web UI keeps the page shown in the address, so that a reload opens
  it again and back and forward move between pages (`docs/ui.md`,
  "Header").

- The web UI's header reads the time with ↻, then the Cube choice, then
  the pages; below 600 px ☰ holds Log and System settings (`docs/ui.md`,
  "Header").

- The web UI's Rooms page says when the Cube is not connected or none is
  configured: the address tried, the last data and what to check
  (`docs/ui.md`, "Rooms").

- A sign-in by OPAQUE (RFC 9807) that sends no password, and the
  registration that sets one: `POST /api/v1/login/start` and `…/finish`
  (migration 0007, `opaque.key`; `docs/api-v1.md`).

- The web UI signs in by OPAQUE, the password kept in the browser, with
  `@serenity-kit/opaque` 1.1.0 in `ui/vendor/`; a user of version 1 is
  signed in with the password after it (`docs/ui.md`).

- `auth.password_login` in `/api/v1/settings`, a switch in System
  settings → Users: off, `POST /api/v1/login` is `403` and users sign
  in by OPAQUE alone (`docs/api-v1.md`, "GET and PUT /api/v1/settings").

- `GET /api/v1/cubes/{sn}/log` takes `since`, an RFC 3339 time, for the
  events at or after it (`docs/api-v1.md`, "GET /api/v1/cubes/{sn}/log").

- `-log-level debug|info|warn|error` or `MASSIMO_LOG_LEVEL` sets the
  level the commands log at, `info` by default; a change in `serve`'s
  settings file waits for its restart (`README.md`, "Server").

### Changed

- Passwords are hashed with argon2id at 19 MiB and two passes, about
  55 % of the time and 30 % of the memory of the 64 MiB before; an old
  hash is replaced at the next sign-in (`docs/store.md` section 6).

- A user add of a taken name or an unknown role, and a password set for
  a user that is not there, answer at once rather than after a hash
  (`docs/store.md` section 6).

- Every password the web UI or `massimo user add` and `user passwd` set
  is registered by OPAQUE, `auth_version` 2, and never sent: the server
  keeps a record and no hash (`docs/store.md` section 6, README.md).

- A user of `auth_version` 1 who signs in in the web UI is moved to 2 at
  once: the password crosses that once more and the hash goes
  (`docs/store.md` section 6).

- The web UI's log reads newest first, new events coming in on top and
  Older events at the bottom (`docs/ui.md`, "Log").

- The recorded greeting that the tests and CI's fake Cube load, and its
  UI-test copy, carry placeholder serials
  (`internal/protocol/cube/testdata/`, `tools/ui-test/`).

- CI runs the web UI's suite on a pull request only when it changes the
  UI, the suite or `ci.yml`, and then on two of its four cases; a push to
  `main` runs all of it (`GUIDELINES/project/CI.md`, section 2).

- The web UI's Log button shows a sheet, since three lines alone read as
  a menu (`docs/ui.md`, "Header").

### Fixed

- The web UI's Clear in the log sets the filter, the kind and the device
  back to all, in place of emptying the view (`docs/ui.md`, "Log").

- The web UI keeps a button that sends off, with a spinner, until the
  answer, and a Send off while nothing changed, so a second click sends
  nothing (`docs/ui.md`, "Rooms", "How the page is built").

- The web UI checks a new user's name while it is typed, and the users
  and roles routes refuse without the store's `store: ` prefix
  (`docs/ui.md`, "System settings").

- A request still being answered when `serve` stops is finished, and
  its audit row written, before the store closes (`cmd/massimo`).

- A stop of `serve` brings `massimo.db-wal` into `massimo.db`, also
  while another program has it open, and logs `store closed`
  (`docs/store.md` section 3).

- A lost Cube is logged and stored as `disconnected` once, not at
  every retry, and its return logged with how long it was down
  (`docs/api-v1.md`, "GET /api/v1/cubes/{sn}/events").

### Security

- A server that asks for no key answers only its local network, a
  reverse proxy's clients only with `-trusted-proxies`
  (`docs/api-v1.md`, "Authentication").

- An event stream ends within 30 s once its caller would not be let in
  any more, such as a revoked key or an open server's first user
  (`docs/api-v1.md`, "GET /api/v1/cubes/{sn}/events").

- The server refuses a write that another site's page sent, which an
  open server served: such a page could create the first owner
  (`docs/api-v1.md`, "Authentication").

- Password hashes run at most one a core at once, four at the most, and
  eight wait; a sign-in, user add or password change beyond them is
  `503` with `Retry-After` (`docs/store.md` section 6, "The lockout").

- An address signs in once at a time: a login while its previous one
  is checked is `429` with `Retry-After: 1`, and the web UI's dialog
  sends one sign-in at a time (`docs/api-v1.md`, "POST /api/v1/login").

## [0.10.0-alpha.1] - 2026-11-07

### Added

- The store keeps the Cubes' events (migration 0004), written in
  batches and pruned daily by `log.keep_days` in `/api/v1/settings`
  (30 days; 0 keeps them), so that the log survives a restart.

- The web UI's log pages back with "Older events" and shows one kind
  of event or one device, from the server's log, which reaches back
  across restarts (`docs/ui.md`, "Log").

- The web UI pairs a device in Settings → Cubes and puts it into a room
  with the room's settings; Devices without a room and the Devices tab
  remove one from the Cube (`docs/ui.md`).

- `massimo fakecube -pair <template rf>,<rf>,<serial>`: a device that
  joins pairing mode, and again after it was removed (README.md).

- `DELETE /api/v1/cubes/{sn}/devices/{rf}` removes a device from the Cube
  as the original software does, under `structure`; a client's `t:`
  through the proxy drops the devices from the server's copy too.

### Changed

- `GET …/log` pages back with `before` (an event's `id` or a time) and
  filters by `kind` and `device`, from the store; a `limit` above 1000
  answers 400, and `state` events lack `received` (`docs/api-v1.md`).

- `PUT …/devices/{rf}` gives a thermostat put into a room or moved the
  room's settings, as the original software does, with `configure`;
  `"copy": false` skips it (docs/api-v1.md).

### Fixed

- A stop of `serve` just after the Cube of the plain
  `MASSIMO_CUBE_ADDRESS` reported its serial no longer leaves the next
  start refusing "several cubes configured".

- A second update started on Windows while the helper reads the first
  answers 409, no longer 500 from the request file the helper holds.

- Two updates started at once, as from two tabs, write one request and
  answer 202 and 409, no longer 500 when both wrote the request file.

- A Debian package built between two tags holds the version its file
  name shows (`make packages`).

- The apt repository's keyring package is versioned by the time of the
  publish, so that its version only goes up, whatever the key's dates.

- The apt snippet's Signed-By names the keyring package's key, installed
  first, so that an extended key reaches the boxes (packaging/README.md).

- The release workflow pushes the Homebrew formula to the tap itself:
  the release it creates never started publish.yml (CI.md, section 6).

- The apt repository's setup names the `github-pages` environment's rule
  for the tags `v*`, without which every deploy is refused.

- Settings → Users shows a key just created, lists each token once, and
  one click on Create key makes one key.

- A redraw of Settings → Cubes no longer reloads an open Cube's pairing,
  which took the focus out of the pairing form being typed in.

- Hide and Show again in a Cube's panel keep what is typed in its form.

- A sign-in as the page starts, or after Log out, starts the page once,
  with one event stream instead of two.

- Log out takes the user's settings and colour schemes out of the
  browser, so that the next user neither sees nor inherits them.

- A colour scheme deleted in one browser no longer comes back from
  another that still had it (docs/ui.md, "Settings").

- A room's state arriving while its configuration is typed in no longer
  draws the tile again under the typing; it shows once the focus leaves.

- A switch point's time can be typed in full from the keyboard in the
  week programme; the editor no longer redraws after its first part.

- The log keeps the live events that come while it loads a page.

- Saving a user with an end date no longer sends the end date again,
  cut to the minute, when it did not change.

- A double click on Yes of a removal sends one request, and the page
  says it worked instead of reporting the second as failed.

- A language load that a newer choice has superseded no longer shows
  its error on the page in the newer language.

- An event that makes the page load again no longer leaves a failed
  load as an uncaught error in the browser's console.

- A new API key is no longer kept in its command's result, nor given
  out again by a replay of its `Idempotency-Key` (docs/api-v1.md).

- Behind a trusted proxy, a client's `X-Real-IP` no longer wins over an
  `X-Forwarded-For` entry that is not an address, nor a second
  `X-Forwarded-For` line over the proxy's (docs/reverse-proxy.md).

- Requests refused before anyone is known are one audit row per address
  and minute, with their count, and their bodies are not read
  (docs/api-v1.md, "GET /api/v1/audit").

- A user added under a removed user's name no longer inherits that
  user's settings and colour schemes (docs/api-v1.md).

- The Infomaniak hook hands its token to curl on standard input, not on
  the command line, and gives each call 30 seconds (packaging/README.md).

- A DNS-01 hook whose child keeps its output open no longer holds the
  order past the hook's two minutes; the child ends with the hook.

- `massimo install -no-update-helper` on Linux disables and removes the
  update helper an earlier install put there (packaging/README.md).

- An owner with an end date no longer counts as the owner left, so the
  last one without an end date keeps the box reachable (docs/api-v1.md).

- A DNS-01 order that ran out of time still clears its TXT record, and
  a clear that fails is always logged.

- An authenticated request writes the store's lockout only where its
  address or user name had failed checks, no longer on every request.

- `-tls acme` serves the first name's certificate to a client that names
  no host, so the update helper's health check no longer rolls back.

- `-tls acme`'s hourly check orders the ECDSA certificate browsers get,
  no longer a second RSA one, and the status shows the one served.

- `massimo install` with another `-tls` mode removes the variables of
  the one before, so serve no longer refuses to start after a switch.

- The update helper's health check over TLS on one of the box's own
  addresses (a specific `-listen`) no longer fails on its certificate.

- `massimo serve` stops on SIGTERM as on Ctrl-C, so a stop by systemd or
  launchd writes the pending log events and closes the store.

- A stop of the Windows service, as by an update or a reboot, waits for
  `serve` to write the pending log events and close the store.

- The update helper reports "failed" for every error after it took the
  request, so the web UI no longer shows the previous run's result.

- `massimo serve` takes the plain `MASSIMO_CUBE_ADDRESS` for the one Cube
  `MASSIMO_CUBE_SN` names, also beside a variable of its own (README.md).

- A serial in lower case in a variable's name, such as
  `MASSIMO_CUBE_keq0000000_ADDRESS`, is that Cube's again on Linux.

- The self-signed certificate's renewal keeps a name added with
  `massimo tls self-signed -names` (README.md, "Server").

- `massimo update -check` and the web UI say that a channel has no
  signed release, no longer that an old release has no manifest.

- `massimo update` downloads a binary for as long as a slow line needs,
  no longer cut off after two minutes; a stall ends it after one.

- An update check fetches no manifest of another channel's release, so
  a build from `main` no longer verifies thirty alpha releases each time.

- `massimo tls show` prints what `massimo config` prints of the TLS,
  the HSTS days and the names given included.

- `massimo user tokens <name>` lists a user's sessions and keys with the
  ids `user revoke` takes, so one key can be revoked alone (README.md).

- The TLS variables are named once for `massimo install`, `serve` and
  `massimo config`, which can no longer read different names.

- A command's step in `GET /api/v1/commands/{id}` no longer stays
  `queued` when the queue ran it at once (docs/api-v1.md).

- A radio command no longer waits for the store to record its step,
  which a busy SD card could delay by up to five seconds.

- A retry with the `Idempotency-Key` of a request the server's restart
  cut off answers 503, not a closed connection (docs/api-v1.md).

- A replay of a restore's `Idempotency-Key` answers the first `202`, no
  longer the run's outcome (docs/api-v1.md).

- `POST /api/v1/update`'s command is acknowledged with its 202, no
  longer `sent` until a restart, and its key replays (docs/api-v1.md).

- A Cube served by its address alone learns its serial without a data
  race with the handlers and discovery that read it.

- A Cube served by its address that reports the serial of a Cube added
  meanwhile stops, instead of taking its place beside it.

- A `t:` deleting several devices waits for the Cube's `A:` of each, so
  the next command no longer takes a late one as its reply
  (docs/proxy.md).

- A Cube's new address from discovery is dialled at once, with one
  connection, no longer after the minute of backoff the old address
  built up.

- A device whose greeting C line answered a re-read sent during the
  greeting no longer drops out of the server's copy.

- A pairing given up while its `n:` still waits in the queue is taken
  out of it, so the Cube no longer starts an inclusion nobody watches.

- The re-read after a configuration write is a step of its command in
  `GET /api/v1/commands/{id}`, so a failed one shows there.

- `massimo fakecube -pair`: the device no longer joins after `x:` or
  twice after two `n:`, nor once the connection is closed.

- A write no longer scans the commands table for its `Idempotency-Key`
  and for the retention: the key's index serves it, the prune is hourly.

- What counts as a thermostat is decided in one place, so the copy of a
  room's settings, the stale rule and the log can no longer disagree.

## [0.9.0-alpha.1] - 2026-11-06

### Added

- Store stage (c), the settings per user (`docs/store.md` section 9):
  kept per user and key for a role with the capability `preferences`
  (migration 0003), through `/api/v1/me/preferences` (`docs/api-v1.md`).

- The web UI's settings and colour schemes follow the signed-in user to
  every browser, each with a switch to keep it on this device only
  (`docs/ui.md`, "Settings").

- `massimo serve -tls files -tls-cert … -tls-key …`: TLS from certificate
  files, re-read when they change, with HSTS for a year (`-hsts <days>`);
  `massimo install -tls-cert … -tls-key …` installs them (README.md).

- `massimo serve -tls self-signed`: a certificate the server makes for
  the box's names, renewed 30 days before it expires; `massimo tls show`
  and the web UI's header show its fingerprint (README.md, "Server").

- `massimo serve -tls acme -tls-names <name>`: a Let's Encrypt
  certificate by TLS-ALPN-01, renewed by itself (`-tls-http :80` adds
  HTTP-01 and the redirect); the UI warns two weeks before an expiry.

- `massimo serve -tls acme -acme-dns-hook <script>`: the certificate by
  DNS-01 through a hook that sets the TXT record, wildcards included,
  no open port; `packaging/acme-dns-hooks/infomaniak.sh` for Infomaniak.

- `massimo serve -trusted-proxies <addresses or CIDRs>`: the client's
  address from `X-Forwarded-For` behind a reverse proxy, the stream
  unbuffered, the UI under a path prefix (docs/reverse-proxy.md).

- A Debian package per release (amd64, arm64, armhf for a 32-bit Pi OS),
  installed with apt, which then owns the binary: `massimo update` and
  the web UI show what is out (packaging/README.md, "Debian package").

- An apt repository on the project's GitHub Pages, one suite per update
  channel, signed with the repository key, with the keyring package;
  `apt upgrade` then follows the channel (packaging/README.md).

- A Homebrew tap, `brew install metaworx/massimo/massimo-alpha`, built
  from source and run with `brew services`; brew then owns the binary,
  as apt does (packaging/README.md, "Homebrew").

### Changed

- CI runs once per commit (a push to `main` or a pull request), and the
  Screenshots job runs in the Playwright image, without apt.

- The web UI's tests are a pytest suite (tools/ui-test/); CI's
  Screenshots job fails when they cannot run instead of skipping them,
  and keeps a failed test's trace, screenshot and video as an artifact.

### Fixed

- CI's Windows job sends the second `POST /api/v1/update`, which must
  be refused, on the first's connection: a second curl could start
  after the helper had stopped the service (GUIDELINES/project/CI.md).

## [0.8.0-alpha.1] - 2026-11-06

### Added

- Store stage (b) in `internal/store` (`docs/store.md` section 9):
  migration 0002 (`commands`, `users.auth_version`, `audit.command_id`),
  users with argon2id passwords, roles, tokens, the bootstrap, the audit,
  the lockout counters and the commands with their steps.

- `massimo serve` authenticates users: a bearer token names a user whose
  role grants what a route needs (`403` without), `MASSIMO_API_TOKEN`
  becomes the owner's key, the lockout survives a restart
  (`docs/api-v1.md`, "Authentication").

- `POST /api/v1/login`, `POST /api/v1/logout` and `GET /api/v1/me`: a
  session for a name and a password, 30 days from its last use, and who
  the caller is with the effective capabilities (`docs/api-v1.md`).

- The users, tokens, roles and capabilities routes: create, change and
  remove users and their API keys, reshape roles, disable a role or a
  capability, the first owner from the local network (`docs/api-v1.md`).

- The audit: every request that changes something, or tries to, is a row
  with who, from where, what and how it went, read with
  `GET /api/v1/audit` (`docs/api-v1.md`).

- Commands: every write request is a row with its steps on the Cube's
  queue, `Idempotency-Key` answers a retry without a second radio
  command, `GET /api/v1/commands/{id}` shows one (`docs/api-v1.md`).

- The web UI signs in with a name and a password or an API key, hides
  what the role may not do, creates the first owner from the local
  network, and manages users and roles in Settings (`docs/ui.md`).

- `massimo user add|list|passwd|disable|enable|token|revoke`: the users
  of the store from the box, beside a running `serve` (`README.md`,
  "Cube variables").

- CI runs the guidelines' checks on every push and pull request
  (generated documents, languages, revision records, headings, line
  breaks, the changelog against its tags, commit messages on a PR).

- `make ui-screenshots` takes the web UI's screenshot set; CI's
  Screenshots job runs the UI smoke test and uploads the set.

- A Cube found by the server's discovery shows whether it takes its
  address by DHCP, in the web UI and as `dhcp` in the API
  (docs/api-v1.md).

### Changed

- The colour-scheme editor takes typed or pasted hex values next to each
  colour, and its contrast badges stand before the colours, which now
  line up (docs/ui.md, "Appearance").

- The web UI loads the texts of the chosen language only
  (`ui/i18n/<lang>.json`), and the browser revalidates the UI's files
  by their ETag instead of downloading them again.

- The web UI's settings and room configuration share one row layout:
  the label at the left and the control at the right, a text field or
  a long choice on a line of its own (docs/screenshots).

- The web UI asks in the page, translated, before it removes a Cube or
  installs an update, instead of in a browser dialog; Settings → Cubes
  lists a found Cube with its firmware and age, as the header does.

- The release workflow reads the update channel from the tag's name
  (`v0.7.0-alpha.1` is on `alpha`) before the tag message's `channel:`
  line, so a message rewritten with the changelog keeps its channel.

- `make release-alpha` cuts the changelog's `[Unreleased]` into the
  release's section on `main` and tags with that section; `DRY=1`
  shows what it would cut, tag and push (README.md, "Versions").

- A GitHub release's notes are its tag's changelog section, then the
  sentence on the channel and how to install and verify a download.

### Fixed

- Writes from LAN proxy clients, such as the original software, get
  the server's follow-up re-reads and budget reconnect, instead of a
  stale copy of the device until the six-hourly refresh.

## [0.7.0-alpha.1] - 2026-11-06

### Added

- The store: `massimo serve` keeps the Cubes and its settings in
  `massimo.db` (SQLite) in the data directory (`-data`,
  `MASSIMO_DATA`), created at the first start, migrated with a
  `min_reader` rule that keeps a rollback of the binary safe, seeded
  from the settings file, with the Cubes found by discovery surviving
  a restart and a node identity (`node.id`, an Ed25519 key).

- `POST`, `PUT` and `DELETE /api/v1/cubes`, `GET` and
  `PUT /api/v1/settings`, and Settings → Cubes in the web UI: add,
  change and remove Cubes and the server's settings while it runs;
  a changed address reconnects, a changed proxy address restarts that
  listener, a hidden device applies at once.

- `massimo store backup|migrate|info` for the store, and
  `massimo config` says which values come from the store.

- `massimo netconfig` reads and writes a Cube's network configuration
  over UDP (static address or DHCP), with a broadcast recovery for a
  Cube on a wrong address; the protocol is confirmed on hardware.

- `massimo discover` shows whether a Cube takes its address by DHCP.

- `-direct` and `-no-direct` on every command: skip the configured
  proxy, or refuse the fallback to the Cube when the proxy does not
  answer; the fallback warns that it takes the Cube's only connection.

- Updates from the web UI on Windows: the service starts the update
  helper itself as a detached process; the CI Windows job exercises
  the API path and a rollback.

- `massimo legacy` shows the original software's Cube search lines.

- The designs of the store (`docs/store.md`), of what comes after it
  (`docs/server.md`) and of the Windows update path.

### Changed

- The first device list of a connection carries the greeting's time
  as `observed_since`.

- `serve -addr` runs without a store; the Cube and settings routes
  answer 503 then.

- The stale threshold is a server setting (`stale_after`) rather than
  a constant.

- A discovery answer no longer rewrites a configured Cube's row; only
  the discovery columns change, and only when they differ.

- `.gitattributes` keeps the embedded unit files, plists and scripts
  LF on every checkout; the protocol fixture keeps its CRLF.

- `update` removes `massimo.exe.failed` once a newer binary is in
  place, and a rollback tolerates a missing target.

- `massimo config` knows the update variables as names.

## [0.6.0-alpha.1] - 2026-11-05

### Added

- Colour schemes in the web UI: light, dark or system mode, three
  built-in schemes, a custom scheme with a contrast check, export and
  import as `massimo-scheme/1`.

- A restore API: a plan from an uploaded backup, the send checked
  against the plan's hash, progress on the event stream, guard rails
  for the duty cycle and the Cube's serial.

- An "Installing" section in the README per operating system, with
  the SmartScreen dialog as the Windows test saw it.

- The report of the Windows service tested on a real PC.

### Changed

- Both buttons of Settings → Software are greyed out while an update
  runs, also after a reload of the page.

- `.env.*` copies and `*.log` files are ignored by git.

## [0.5.0-alpha.1] - 2026-11-05

### Added

- State events say what changed and who changed it (the server,
  another program, someone at the device); the log view shows it as
  "window opened", "set 18.0 → 21.0", "reports again".

- `started_at` on `/healthz`; the web UI shows it in expert mode.

### Changed

- The web UI's Cube search uses the server's discovery API.

- Blue is the accent colour, red only for alerts.

- Device type and radio address show in expert mode only.

## [0.4.0-alpha.1] - 2026-11-05

### Added

- Discovery from `serve`: at the start, every 30 minutes and on
  `POST /api/v1/cubes/discover`; unconfigured Cubes in
  `GET /api/v1/cubes`, `cube_found` events, Cubes configured by serial
  follow their discovered address; "Search for new Cubes…" in the UI.

- Pairing: `POST`, `GET` and `DELETE …/pairing`, `paired` events,
  adding a paired device to a room, pairing through the proxy,
  inclusion mode in the fake Cube.

- After a write the server re-reads the device's configuration and
  reconnects once for a fresh budget when radiograms were pending;
  accepted commands and wakeups in the log.

- macOS LaunchDaemons for `install` and `update`, tested on a Mac and
  in CI; `MASSIMO_GITHUB_TOKEN` for the release check.

- Ages and stale warnings in the UI, the "Open" window badge on the
  tile, the event stream opened with tickets.

### Fixed

- The window contact's move and its link error, confirmed in the
  write test's step 7.

## [0.3.0-alpha.1] - 2026-11-05

### Added

- Timestamps per value: `received`, `last_seen`, `stale` and
  `observed_since` in the API; ages and greyed values in the UI.

- A lockout after ten wrong tokens a minute, and single-use tickets
  for the event stream (`?token=` on the stream deprecated).

- French, Italian, Dutch and Polish in the web UI, machine-translated
  and marked; `docs/ui.md`, using the web UI.

- `serve -refresh`: every Cube's snapshot re-read in full every six
  hours.

- `make ui-test`, a Playwright smoke test of the web UI.

- The Windows service: `massimo install` and `massimo update` on
  Windows, tested on a CI runner; govulncheck in CI.

- `internal/view`, `internal/cubeio` and `internal/env`: one read
  model, one place for writes that keep the snapshot in step, one
  reader of the command line's configuration.

### Changed

- "Hidden devices" is the one term; `-listen` names the listener on
  every command (the old flags are kept); the proxy refuses to delete
  hidden devices.

- `connected` means the greeting is in, and goes false on a loss.

### Fixed

- A reconnect right after `Ready` waits for the new greeting.

- A single Cube without a serial gets its proxy again.

## [0.2.0-alpha.1] - 2026-11-05

### Changed

- `install -listen` moves the API and web UI and is kept across
  installs.

- Releases: a new feature starts the next minor, a fix counts the
  suffix up.

## [0.1.0-alpha.12] - 2026-11-05

### Changed

- Development happens on `main`; `make release-alpha` promotes it to
  the branch `alpha`, the update channel of the alpha releases.

- Pre-releases are numbered past `alpha.9`.

## [0.1.0-alpha.11] - 2026-11-05

### Fixed

- `update` runs its smoke test in an empty directory.

## [0.1.0-alpha.10] - 2026-11-05

### Added

- `massimo install` sets `serve` up as a systemd service, with a root
  update helper; `massimo update` installs signed releases, with a
  health check and a rollback; an Update button in Settings.

- CI tests every push and publishes signed releases from `v*` tags;
  macOS and `linux-armv6` targets; the update channel; `make release`.

- Documentation for preparing a Raspberry Pi 1, per-Cube proxy
  addresses, moving networks, and measured resource use.

## [0.1.0-alpha.9] - 2026-11-05

### Changed

- MANU needs a known setpoint; pending radiograms count against the
  Cube's observed capacity.

- One state constructor and one metadata mirror for the API, `status`
  and backups.

- A constant-time token check and security headers on the web UI; a
  test that the OpenAPI description covers every route.

## [0.1.0-alpha.8] - 2026-11-05

### Added

- The design proposal, the background evaluation, the code review
  with the coding standard, and the session handover.

## [0.1.0-alpha.7] - 2026-11-04

### Changed

- The web UI: coloured badges, IS and SET icons, the info and cog
  buttons on the tile, icon pages in the header, accessibility.

- `massimo config` redacts the API token like the keys and IVs.

## [0.1.0-alpha.6] - 2026-11-04

### Added

- The room configuration area and the log view in the web UI; Cube
  names from `MASSIMO_CUBE_<SN>_NAME`; `GET …/log` with the last
  events.

## [0.1.0-alpha.5] - 2026-11-04

### Added

- Room and device configuration in the API: names, moves, device and
  room configuration, week programmes.

- IS and SET tiles with a chevron stepper, HOLIDAY, a refresh button.

## [0.1.0-alpha.4] - 2026-11-04

### Added

- The first web UI at `/`: room tiles with a stepper, the House tile,
  the device list, expert mode, settings with polling and the clock
  zone, a reload on a new server version.

- `serve -log` appends the log to a file.

- Documentation of the firmware extraction over JTAG.

## [0.1.0-alpha.3] - 2026-11-04

### Added

- Room 255 as the ghost room, `-show-ghosts` and `-no-ignore`,
  `DELETE …/rooms/{id}`, hidden parking rooms; the proxy hides ignored
  devices from clients and puts them back into metadata writes.

- `status` and the API count the radiograms the Cube still waits for.

- `serve -sn` serves one Cube; the proxy settings come from `.env`;
  the commands try the proxy first; `make tag`.

## [0.1.0-alpha.2] - 2026-11-04

### Added

- The server: one connection and one command queue per Cube, the JSON
  API, server-sent events, and the legacy launcher with the LAN lines
  derived from the original software's gateway calls.

- The LAN proxy: `serve` answers as a Cube on port 62910 with UDP
  discovery, so other programs and the original software connect to
  the server.

- The ignore list, the key lock (`PUT …/devices/{rf}/lock`), the
  `status` command with reported and ready flags.

- Backups record each device's live state; `restore -state` re-applies
  mode and setpoint; week programmes by weekday with HH:MM times; wall
  thermostats' display mode, offset, window-open and boost.

- Cube discovery, per-Cube variables (`MASSIMO_CUBE_<SN>_KEY`), the
  IV exchange over the LAN, `massimo config`, and the portal's probe
  mode.

- Documentation of the portal protocol, the PC software's portal
  client and the Cube's UDP protocol.

## [0.1.0-alpha.1] - 2026-11-03

### Added

- The Cube LAN protocol package, `massimo decode`, the portal crypto
  package with `portal-crypto`, the fake portal server and a LAN
  client command.

- Secrets from a `.env` file, `--version`, a Makefile with cross
  builds.

[Unreleased]: https://github.com/metaworx/massimo/compare/v0.7.0-alpha.1...HEAD
[0.7.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.6.0-alpha.1...v0.7.0-alpha.1
[0.6.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.5.0-alpha.1...v0.6.0-alpha.1
[0.5.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.4.0-alpha.1...v0.5.0-alpha.1
[0.4.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.3.0-alpha.1...v0.4.0-alpha.1
[0.3.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.2.0-alpha.1...v0.3.0-alpha.1
[0.2.0-alpha.1]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.12...v0.2.0-alpha.1
[0.1.0-alpha.12]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.11...v0.1.0-alpha.12
[0.1.0-alpha.11]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.10...v0.1.0-alpha.11
[0.1.0-alpha.10]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.9...v0.1.0-alpha.10
[0.1.0-alpha.9]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.8...v0.1.0-alpha.9
[0.1.0-alpha.8]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.7...v0.1.0-alpha.8
[0.1.0-alpha.7]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.6...v0.1.0-alpha.7
[0.1.0-alpha.6]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.5...v0.1.0-alpha.6
[0.1.0-alpha.5]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.4...v0.1.0-alpha.5
[0.1.0-alpha.4]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.3...v0.1.0-alpha.4
[0.1.0-alpha.3]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.2...v0.1.0-alpha.3
[0.1.0-alpha.2]: https://github.com/metaworx/massimo/compare/v0.1.0-alpha.1...v0.1.0-alpha.2
[0.1.0-alpha.1]: https://github.com/metaworx/massimo/releases/tag/v0.1.0-alpha.1

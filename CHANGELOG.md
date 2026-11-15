# Changelog

All notable changes to Massimo are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions
below 1.0.0 are pre-releases, tagged `vX.Y.Z-alpha.N` by
`make release-alpha` (`README.md`, "Versions"). A new feature starts
the next minor, a fix counts the suffix up.

## [Unreleased]

## [0.16.0-alpha.1] - 2026-11-15

### Added

- Every room has a virtual wall thermostat, the room's temperature for
  every client; `?devices=default|real|virtual|all` chooses the lists'
  wall thermostats, and rooms carry a `room_uid`.

- A virtual wall thermostat takes a real one's commands, sending them to
  its room or the room's wall thermostat; a room refuses a second wall
  thermostat, as the original software does.

- Week programmes carry a tag over the room's thermostats: a write with
  If-Match is refused with 412 where the programme changed since or a
  write is still on its way; config events say what changed.

- The web UI's programme editor refuses to overwrite a programme changed
  elsewhere since it was loaded, and offers the current one instead.

- Boost over the air: `{"mode": "boost"}` on the setpoint routes, and
  Boost in a room's ⋯ menu; the thermostats return to their mode by
  themselves after their boost time.

- A Cube's proxy can show its clients the virtual wall thermostat of
  every room (`proxy_virtual`, off by default), keeping them off the
  Cube: commands go to the room or its real wall thermostat.

- System Info shows the server's ticking clock, its full host name, its
  interfaces with their MAC and addresses, and behind a trusted proxy
  "(proxied)" with the proxy's address and certificate, trusted or not.

- `/healthz` names what the running server can do, each feature with
  its version (`"features": {"vwt": "1.0", …}`), for clients of builds
  without a release version.

- Users link their own accounts at the sign-in providers in My profile,
  managers change a link not used yet, and both see what the provider
  says of the account: its name, user name, groups and last sign-in.

- A sign-in provider may admit only the accounts in some of its groups,
  read from a claim of the provider's choosing, `groups` by default.

- `make ui-brand` writes the web UI's logo, favicon and app icons into
  `internal/server/ui/brand/`, from the brand tool's new `ui` scope
  (`tools/brand/README.md`).

- A link of an account at a sign-in provider is shown in My profile
  before it is stored, with the link it replaces, so that a wrong account
  signed in at the provider is never linked by mistake.

- My profile explains its controls: an ⓘ behind the display name, the
  password, the sessions and keys and the linked accounts opens what
  their buttons do.

- `GET /api/v1/log` reads every Cube's log at once, each Cube's line
  with its Cube's serial number in `cube_serial`, the server's own once.

- The log has a Cubes choice where there is more than one Cube: all
  Cubes shows every Cube's lines at once, each naming its Cube, and asks
  for new ones every 10 seconds.

- GET /api/v1/cubes names a Cube's proxy listener that does not listen
  as proxy_failing, with the error and the next try, and GET
  /api/v1/system lists every Cube proxy as cube_proxies.

- System settings show a Cube proxy that does not listen: a notice
  with the error under the Cube's proxy address, a line in System
  Info, and a warning badge on both cards.

- `GET /api/v1/cubes` says when the Cube's clock was read
  (`cube_time_at`), so that a client can move the clock on.

- massimo fakecube -single-client serves one client at a time, as the
  Cube does, and holds a session that ended while it was silent for
  -hold-dead, until POST /release or an R frees it (README.md).

- GET /api/v1/cubes says in link whether a Cube's lost connection is
  held, or the Cube refuses serve while it holds serve's old connection
  (stale) or another client's (busy); the UI says so in words.

- Reboot the Cube in System settings → Cubes, POST
  /api/v1/cubes/{sn}/reboot and massimo netconfig -reboot restart a Cube
  (capability cubes.reboot), naming the radiograms the restart drops.

- Integrations ask to be registered without a key and get one once the
  owner approves them with the code both sides show, their maximum set;
  `/api/v1/integrations` lists, changes and revokes them.

- A registered integration acts for its users: `X-MassimoHA-On-Behalf-Of`
  names the user and the call's level, and the call may do what its role,
  its link, the integration's maximum and the level all allow.

- An integration's users are linked to Massimo users by the owner, a
  link proposed by name; the users list marks them (AP IntegrationUsers).

- The announcement on the LAN tells integrations where the API is and
  whether they may register; the server lists Home Assistant
  installations it sees there (AP IntegrationUsers).

- Behind a trusted reverse proxy, the announcement's url is the address
  the proxy serves the server at; System Info leaves out the scheme's
  own port (AP IntegrationUsers, D12-A).

- Each page of the web UI comes back at the place it was left, Rooms
  where it was scrolled to while System settings was open.

- System settings → Integrations approves an integration's request by
  its code, sets its maximum and links its users; the users list marks
  integration users and their links (AP IntegrationUsers).

- A warning's triangle and a failure's cross blink, so that they are
  not missed; My settings → "Blinking signs" chooses as the system, on,
  at first (5 s) or off.

- A test fails when two scripts of the same web UI page declare the
  same top-level name, which in their shared scope silently replaced a
  function (the found Cubes under the header) or stopped a script.

- An open window turns from blue to amber after 5 minutes and to red
  after 10 in the web UI, by the settings `window_warn_after` and
  `window_alert_after`; the API gives each open window these times.

- The web UI's header shows the server's sign before the time of the
  last update: connected, an update out, a new version (a click
  reloads, in place of the reload button) or gone, blinking.

- `GET /api/v1/cubes` gives each Cube the address discovery last found
  it at (`found_address`), beside the one the server connects to.

- `GET /api/v1/cubes` lists each Cube's devices with a problem, a radio
  fault, a low battery or a window open, with their rooms' names.

- The web UI's header shows each Cube's link as a network sign in the
  Cube choice, and beside it every Cube's problems in one list, the
  settings that need a look last.

- An open Cube's form in System settings keeps what is being typed while
  its values change on the server: a field not touched follows, an
  edited one says so under it, with "Take theirs".

- The announcement on the LAN is the setting announce (auto, on, off),
  applied at the next start; -announce and MASSIMO_ANNOUNCE still win
  (AP AnnounceSetting).

- System settings → Maintenance chooses the announcement on the LAN:
  automatic, on or off, at the next restart (AP AnnounceSetting).

- A Cube with a fixed address that answers discovery at another one is
  `found_elsewhere` in the API, and logged at Warn once rather than at
  every run (AP AddressCheck).

- System settings → Cubes and the header's problems show a Cube that
  discovery finds at another address than its fixed one, with "Use the
  found address" and "Follow discovery" (AP AddressCheck).

- The web UI copies a fact with a click, or with the copy icon beside it,
  also over plain HTTP: the server's version, whole, and System Info's
  Cube proxies; Store and Settings in effect moved to System Info.

- A room tile's Holiday button becomes More: Eco and Comfort set the
  room's eco or comfort temperature in its mode, Hold (Manual) while it
  is in Auto, and Holiday… (AP RoomTile).

- A room on holiday says until when on its More button and in its
  problem pill, with a suitcase, and its devices' end in the same form
  (AP RoomTile).

- ☰ (its mark on a disc) and its Log entry turn amber or red while a log
  warning or error is not settled or seen; a line has Mark seen, the log
  Mark all as seen (`GET /api/v1/log/alerts`, the preference `log.seen`).

- My settings → Appearance chooses the window icon (the opening sash or
  the paned window) and the radio icon (the Wi-Fi fan or the tower),
  for the person or this device (AP IconChoice).

- The found Cubes in the API say whether each is a Cube or a Massimo
  proxy (`kind`), the Cube a proxy serves (`serves`) and the Massimo
  server serving it (`host`), from the servers' announcements.

- Software update lists the last three updates: the version, when, and
  who started it, or "outside Massimo" with apt or brew; the server keeps
  ten, in `GET /api/v1/update`.

- `GET /api/v1/system` names the serial each Cube proxy answers
  discovery with (`discovery_serial`); System Info shows it beside the
  Cube's, both copyable.

- System settings → Maintenance sets how often the server polls,
  refreshes and looks for the Cubes, each with a line on what it does,
  applied at the next restart and counted in the card's head till then.

- Cube settings in ☰: the chosen Cube's panel on a page of its own,
  open, under the page's row with its sections menu, following the Cube
  choice.

- Each settings page's row holds the settings pages' buttons in its
  title's place, in one order, the page shown filled and as wide on every
  page, the others icons alone below 900 px; one page alone: its title.

- The Cube choice's list shows each Cube's problems as icons after its
  name, as the pill beside it does.

- A device's battery level, ok, weak or low, kept from its battery flag's
  history (`battery` in the API, amber or red on the tile), with the
  notification `battery_weak`, on by default.

- A wall thermostat back from a silence on another mode, as after new
  batteries, is a warning in the log (`wall_reset`, docs/api-v1.md).

- A move, a removal or a device put into a room answers pending, and the
  log says when the Cube's free memory slots show it done, or warns where
  they do not within about 15 minutes.

- A Cube outside this machine's networks, as one without a DHCP lease on
  192.168.0.222, says so with the `netconfig` line that gives it an
  address here (`other_network`, docs/api-v1.md).

### Changed

- A thermostat keeps its last measured temperature and its time while
  its reports carry none, on vacation too, instead of losing it.

- A room tile's temperature is its virtual wall thermostat's, the wall
  thermostat's else the radiator thermostats' median; expert mode lists
  the virtual wall thermostat with its settings.

- The site's documentation pages name their channel, version and
  source right under the header, no longer at the end of the page.

- The sign-in providers' client secrets are sealed with a key derived
  from `opaque.key`, so that a copy of `massimo.db` alone does not hold
  them; those kept plain are sealed at the next start.

- A change to the site's own pages under `packaging/site/` alone runs
  Guidelines and Site and deploys the site, as one to `docs/` does; they
  are no longer shipped (`GUIDELINES/project/CI.md`, section 2).

- The Web Push key is kept sealed in the store, as the sign-in
  providers' client secrets are.

- The web UI's favicon and the installable app's icons are Massimo's
  mark: an SVG favicon in the browser's light or dark, `/favicon.ico`,
  and a full-square icon for Android's and Apple's home screens.

- The web UI shows Massimo's logo in the header, the sign-in, the first
  owner's form and the connect page, the mark alone on a narrow screen;
  the word stays in view until the drawing has come.

- A search for Cubes that comes while one is going gets the run after
  it, so that it hears a Cube plugged in or moved just before the call;
  the calls that came meanwhile share that run (`docs/api-v1.md`).

- System Info shows the server's start to the second in its zone with
  the uptime, both times in the browser's zone on hover, each interface
  named, and behind a proxy without a scheme header the page's own.

- A Cube whose connection is lost is searched for on the LAN at once
  and every minute until it is back, so that one that took another
  address is followed without waiting for the periodic search.

- The log names a device by its room and name, "Kitchen › Window", and
  one without a name by its room and type; its serial number is the
  tooltip and shows, with the radio address, in expert mode.

- The web UI draws Massimo's logo and mark without a margin, from the
  M's top to the letters' baseline; the brand tool also writes each
  drawing so (`-tight.svg`).

- The header's logo, version and time of the last update stand on one
  baseline, the M as tall as the page buttons; the time and ↻ are one
  button, and the Cube's clock moved from the header to its facts.

- System settings lists its sections in ☰ alone, without the row of
  buttons under its title; System Info's "This browser's address" is
  "Browser address".

- 👤 opens the personal settings, one page like System settings: My
  profile, My settings and Notifications as its cards, with Sign out and
  its sections menu at its top; the 👤 menu is gone.

- A room tile's head shows what is wrong as icons that list the devices
  (radio, battery, open window, mixed modes); a device's head shows its
  type's icon and its own problems so.

- serve holds a Cube's lost connection open for 45 minutes (-cube-hold),
  so that it resumes after a short outage of serve's network instead of
  meeting a Cube that refuses it while it keeps the old session.

- **Breaking** for stores of 0.15.0-alpha*: identities name a sign-in
  provider or an integration, and those releases cannot open a migrated
  store again; the role integration is fixed (its keys keep theirs).

- The web UI's amber comes in two strengths, a brighter one for icons
  and a deeper one for text, and its red is darker in light mode and
  stronger in dark mode, so that amber and red are told apart better.

- A state in the web UI is a colour and a mark: a warning's badge is
  bold and led by a triangle, a failure's by a cross; the radio is drawn
  as a Wi-Fi fan and the window as a paned window, each with its mark.

- A problem list of the web UI has one entry format at every level: the
  icon in a column of its own, the badges under the name, the entries
  under an entry indented to its name with a line from its icon.

- System settings → Cubes shows each Cube's network sign before its
  name and every problem in one pill, whose click opens its list; an
  open Cube's own badges are its top line, without "connected".

- CI's All tests passed names each test job with its result, a job
  no runner took told apart, and on a pull request keeps one comment
  saying which jobs did not pass.

- A Cube's panel in System settings shows its facts, with the serial, the
  address and the radio address to copy, beside its settings, and its
  actions in one row last: Pair a device, Reboot the Cube, Remove.

- An open window is drawn as the opening sash again, its mark inside
  its frame; the paned window and the radio's tower stay as drawings to
  choose (AP IconChoice).

- CI runs the web UI's suite in a job per width, "UI test (1100 px,
  screenshots)" and "UI test (390 px)", each with both languages, so a
  run takes about half the time and a failure names its width.

- The Cubes found on the network have a sign of their own in the header's
  problem pill, and a group last in its list, for the line under the
  header; System settings lists them in one fold, proxies apart.

- CI's What to test shows its answers in the run's summary, as a table of
  what each means for the run, with the reasons it found for them.

- A Cube's problem pill stays top-right in its head, the source badge a
  line under it; a green check takes a pill's place where nothing is
  wrong, in a Cube's head and beside the header's Cube choice.

- System settings: the sign-in redirect URI copies by its icon, Sign-in
  providers and Integrations explain themselves in their heads, and
  Maintenance reads Announce, Settings file, Restart, Reboot.

- `serve` polls each Cube every 10 s instead of every minute, as the
  original software does; a lost Cube is marked within 30 s, and the
  settings name the flag that decides an interval (`fixed_by`).

- CI's LAN test runs on a pull request that changes the server's
  own Go files, where discovery's runs are decided.

- ☰, at every width, holds Log, Cube settings, System settings, System
  info, Personal settings and Sign out, in the accent, each for the roles
  that may open it; the header keeps Rooms and ☰; pages `#cube`, `#info`.

- System info is a page of its own in ☰, for every role with `read`,
  a viewer's too.

- Back to top on every page, filled in the accent's blue, a screen down,
  and in the compacted header left of ☰.

- The protocol and API docs say a setpoint shows once each device has
  acknowledged it, and that idle radiator thermostats report once a
  day; the protocol docs describe the Cube's H line.

- The merge of a pull request whose changes cannot reach discovery no
  longer runs LAN against avahi on `main`; a pull request changing the
  shared UDP port or the Go module now runs it.

- What to test's summary says for the UI test and LAN jobs whether they
  run, and if not, why.

- The pairing time outlines a time out of range in red as it is typed,
  the range in amber before it, and Start goes back to the field instead
  of the browser's message, which came in the browser's language.

- Software update and Maintenance are a page of their own, ☰ →
  Maintenance at `#maintenance`; System settings starts with Cubes.

- Backup and restore per Cube: the chosen Cube's on Cube settings, and
  in System settings a fold for each Cube with how many are kept.

- The header shows how long ago the chosen Cube was last heard from,
  "8 sec", counting on; its tooltip says the time, click to reload.

- `massimo decode` names the L flags as the original software reads them:
  `auto_dst` for `dst`, `flag_04` for the inverted `is_answer`; the H
  line adds `state_cube_time` and `ntp_counter`.

- At a used-up duty cycle, radio commands wait 10 minutes, then the
  Cube's budget is read again, not until the next full hour; the wait is
  `duty_wait`, in System settings → Maintenance (docs/api-v1.md).

- A radiator thermostat counts as stale after 50 h, not 24 h, as it
  reports once a day and not on its decalcification day; wall
  thermostats keep 24 h, settable apart as `stale_after_wall`.

- An open window drawn as the paned window (My settings, Window icon)
  has no middle bar: its frame is one opening beside the swung-out sash
  (AP IconChoice).

- `battery_low` is notified once the flag has stayed 15 minutes, not at
  every flicker of it under a valve motor's load.

- A device the Cube has not heard for some minutes keeps its last values,
  as `last_known` with `out_of_date` in the API and greyed on its tile,
  in place of none.

- The Integrations card says why the server does not look for Home
  Assistant on the LAN, naming Announce on the LAN and what to change,
  with Open Maintenance; GET /api/v1/integrations/found gives the reason.

- The Cube search's outcome shows in a toast at the page's foot, with
  Show for the Cubes found, instead of a line beside the Cube choice; a
  failed search in red.

### Removed

- The portal commands, `massimo portal` and `portal-crypto`, and the Cube
  keys and IVs they read are gone, kept on the branch `archive/portal`;
  a key line left in `.env` is named as ignored.

### Fixed

- A setpoint or programme write sent while the Cube answered the
  minute's poll waits for that reply instead of failing with a 502
  ("another command is waiting for its reply").

- A sign-in through an OpenID Connect provider reads its userinfo for the
  e-mail Nextcloud leaves out, a user ID may be assigned instead, the
  issuer is checked at saving, and a refusal says why.

- The web UI's sign-in page no longer flickers: the header's buttons stay
  hidden until the server has said who is signed in.

- The log names its devices again after a reload on the log or a
  switch of Cube there, where it showed their serial numbers until
  another page was opened and closed.

- A Cube's proxy listener that cannot listen, on an address the host
  does not carry yet or a port in use, is tried again every 30 s at
  most, instead of staying down until serve restarts.

- A wall thermostat that has not measured yet, as after new batteries,
  no longer gives its room 0.0 °C: its 0 counts as not reported.

- A device's values show their age under the value's name and their time
  under the value, so each stands under its own value; the virtual
  thermostat names its temperature's sources by serial.

- An apt box switches to `devel` or `nightly` from the web UI: the
  update helper upgrades `massimo-archive-keyring` with `massimo`, first
  from the current suite where the rolling key is missing.

- The LAN proxy no longer pushes an `L` line to its clients after every
  change: the original software took it for the reply to its next
  command ("S: expected"); a change shows at the client's next `l:`.

- The original MAX! software on a PC running serve binds the discovery
  port again and finds serve's proxies: serve shares UDP 23272, and
  answers a search from its own host by broadcast too.

- The web UI's refresh icon stands on the digits' middle, the logo's
  round letters are no longer cut at the baseline, and a narrow header
  moves the version and the update time under the logo together.

- The proxy and backups keep a C line the Cube sends that `serve`
  cannot read, so the original software gets the greeting as sent.

- An entry a Cube lists at radio address 000000 is no device: the web
  UI and the API no longer show it, and nothing is sent to it, so its
  Remove can no longer make the Cube send a factory reset to 000000.

- The original MAX! software on a PC running serve finds the Cubes:
  serve's discovery socket is dual-stack, so the software gets their
  answers; serve waits for its search, and warns of an answer it missed.

- System settings → Cubes' badges follow every poll, also while a Cube's
  panel is open, and count a held connection or a reboot in amber, a
  lost Cube in red, as the Cubes' signs show them.

- A click in a room's configuration, such as Reset all, is no longer lost
  when the rooms are reloaded in the middle of it: the tile with the
  focus stays, as for a state event, and is drawn once the focus leaves.

- Discovery keeps hearing a served Cube on the server's own host, such as
  the fake Cube: only an answer to the identify request alone from there
  is the server's own proxy's, so the Cube is found again once removed.

- A change to documents alone on code that failed no longer passes CI:
  the job Code result carries the verdict of the last run that tested
  the code, so the nightly build cannot release it.

- The web UI's menus open inside the window on a phone too, no higher
  than the room below them, and scroll in themselves, not the page; a
  long badge in a problem list wraps instead of running out of it.

- System Info follows a proxy's certificate the server is still reading,
  "being read…", until it is read or "could not be read"; the address
  the proxy serves is a link, and copies by its icon.

- The header's version is no longer cut away beside the Cube choice or
  on a narrow phone: it takes a row of its own where needed; the time of
  the last update stays in the compacted header.

- A SIGHUP while `serve` starts no longer ends it, which systemd took
  for a clean stop and did not restart: with `-sighup reload` it is
  logged as a warning and ignored until `serve` is ready.

- Pairing lasts the time asked for, at most 255 seconds: a Cube reads
  only its low byte, so 300 s paired for 44. The API refuses more, the
  proxy caps a client's longer time, the web UI's field ends at 255.

- A jump down the page, to a control scrolled into view, no longer moves
  back by the header's height on a phone as the header compacts: the
  browser's scroll anchoring is off then, as it was meant to be.

- A change to documents alone after a merge whose run on `main` was
  replaced while it waited no longer fails CI: Code result takes the
  pull request's green Tested record for the same code.

- A day of more than 7 switch points is written as the original
  software writes it: the second telegram marked 0x10 and padded to 6
  points, as verified on a Cube (docs/cube-protocol.md).

- Moving a device out of a room, or removing it, sends 0x23 (remove
  group) before the 0x21s that unlink it, as the original software does.

- A room's setpoint and week programme name a thermostat of the room:
  the metadata's master where it is one, else the wall thermostat, else
  a radiator thermostat; a room without one gets 409 (docs/api-v1.md).

- A metadata write of more than ten blocks numbers its `m:` lines in
  decimal, as the original software does (docs/cube-protocol.md).

- `pending_radiograms` reads 50, not 0, when the Cube has no memory slot
  left, as a Cube whose radiograms wait for silent devices can have.

- Removing a device from its room resets the room's flagged members and
  sends one unlink radiogram to the room as a group, as the original
  software did; a step's result names the line the Cube discarded.

- Moving or placing a device sends one link radiogram to the room, one to
  each window contact and one to the device per partner, as the original
  software does, room commands naming the same member.

- A change to documents alone, pushed after a merge whose own run never
  started, tests the merged code instead of failing on a verdict no run
  gave.

## [0.15.0-alpha.1] - 2026-11-13

### Added

- `GET …/log?command=<id>` keeps the events of one command, and each
  event of a command names it in `command`.

- The log shows a command's steps on the radio under its command, the
  queue's holds for the duty cycle and the memory slots, and to a user
  with `audit` alone the sign-ins, failed ones and lockouts.

- A line a client of the LAN proxy passes on to the Cube is a command of
  the store, found in the log with its step; the proxy's clients and its
  warnings are lines of the log, source `proxy`.

- The web UI's log opens a command under its line, with its steps and,
  for those who may see the audit, its audit row; its source choice knows
  the queue, the server and the proxy.

- `massimo user rename <current> <new>` renames a user, asking for the
  password under the new name; its API keys and settings stay.

- System settings → Software chooses the update channel, for a holder of
  `settings`; where apt, brew or `MASSIMO_UPDATE_CHANNEL` decides it, the
  channel is shown fixed with the way to change it.

- The sign-in card's ☰ menu sets the language and the appearance (light,
  dark or as the system) before anyone signs in.

- The apt repository has the rolling suites `devel` and `nightly`, signed
  with a key of their own that the keyring package carries beside the
  numbered suites' key; each rolling build signs its suite.

- The store keeps OpenID Connect providers and each user's identities
  at them, an e-mail assigned bound at its first sign-in (migration
  0016, `docs/store.md` section 6).

- Users sign in through an OpenID Connect provider such as Nextcloud,
  Google or Microsoft, found by subject or an e-mail assigned to them
  (`docs/api-v1.md`, "Sign-in through OpenID Connect").

- The capability `auth` manages the OpenID Connect providers, owner's
  alone by default, and a user's identities are assigned by e-mail
  (`docs/api-v1.md`, "Providers" and "Identities").

- The web UI signs in with a provider's button on the sign-in card, and
  System settings manages the sign-in providers and assigns users'
  identities at them by e-mail.

- `make release <flavour>` cuts an `alpha`, `beta`, `rc` or `stable`
  release, with `BUMP=minor`, `patch` or `major`; `rc`, release
  candidates, ranks between `beta` and `stable` as an update channel.

- On a box apt updates, the update channel chosen in the web UI switches
  apt's suite at the next update, the source put back where apt cannot
  fetch the new one; nothing is downgraded.

- The rolling channels have Homebrew formulae, `massimo-devel` and
  `massimo-nightly` in the tap `metaworx/massimo-rolling`, pushed by each
  rolling build, or following a newer numbered release.

- `massimo fakecube` carries out what it acknowledges (setpoints, boosts,
  programmes, locks), serves a built-in house (`-fixture rooms`), takes
  a test's control (`-control`); `serve -stale-after` for tests.

- System settings → System Info shows the server's version, start and
  certificate, how it was installed, the computer it runs on and the
  browser's address as the server sees it, from `GET /api/v1/system`.

### Changed

- `serve -log` writes JSON lines, made 0640 in `/var/log/massimo`, now
  `_massimo:adm` 2750; stderr and the journal stay plain; `-log-format`,
  `-log-file-format` and `-log-process-format` choose (`README.md`).

- A release asks for the owner's approval once: the apt repository and
  the Homebrew formula start beside the signing and wait for it
  (`GUIDELINES/project/CI.md`, section 6).

- `serve` rotates its `-log` file itself, on every platform: at 10M, at a
  change of its format and on a reload; logrotate's file is gone, and the
  Mac's plist passes `-log`, stderr to `serve.stderr.log`.

- Every line of `serve`'s log names its `source` (`system`, `server`,
  `cube`, `queue`, `proxy`, `store`, `tls`, `announce`, `push`), and
  `cube=` where it concerns one; `serving cube` says `from=`.

- Homebrew's service passes `-log`, so `serve` writes and rotates
  `var/log/massimo/serve.log` in brew's prefix; `var/log/massimo.log`
  keeps only what it prints before its log.

- The store's `events` are its `log` (migration 0014), whose rows may
  name their command and be for an auditor's eyes alone; older releases
  read and write it through a view `events`, so a rollback still works.

- Every event of the web UI's log is one line of serve's log, written
  from the same record (`kind=`, `command=`, `by=`), and the separate
  lines that said the same again (`device state`, `backup taken`, …) go.

- The site's landing page follows the channels: each channel's docs
  index shows its own build's install routes, and the root shows the
  most stable channel's, with the selector (`packaging/README.md`).

- A removed user's row is renamed `<name>-del-<date>`, so a new user
  under the name starts with a row of its own rather than the old one's
  history.

- The workflows are linted by actionlint, pinned in a module of its own
  that Dependabot updates, in `make actionlint` and in CI's job
  "actionlint" (`tools/actionlint/README.md`).

- An apt suite and a Homebrew formula take every more stable channel's
  releases too, as `massimo update` does: `alpha` is offered a newer
  `beta` or `stable` release, and a rolling suite the numbered releases.

- The build of a release is `make build-release`; `make release` stops
  and names it, since it is to cut a release.

- Each channel's page on the site names its own apt suite, and for
  `devel` and `nightly` the rolling key's file; the root keeps the most
  stable channel's.

- `make release` dumps the store the release writes into
  `internal/store/testdata/releases/` before the release commit, and
  the release refuses a tag without it.

- A change to the site's documents alone, `docs/` among them, runs
  Guidelines and Site, and pushed to `main` it deploys the site without
  the tests and the devel build (`GUIDELINES/project/CI.md`, section 2).

- The web UI counts the Cubes found on the LAN on one line under the
  header, a link to System settings → Cubes, which lists them with Add
  and their settings file lines; a found Cube in the choice opens it.

- Each Cube's panel in System settings → Cubes shows its duty cycle, free
  slots and pending radiograms in its head, and opens to a table of its
  address, port, firmware, radio address, clock and radio budget.

- In expert mode the web UI's header shows the chosen Cube's name, duty
  cycle, free slots and pending radiograms on one line, also scrolled;
  the server's facts are in System Info, the Cube's in its panel.

### Fixed

- The nightly build no longer deploys the site at a release commit,
  whose own deploy GitHub Pages then ignored: apt did not offer
  v0.14.0-alpha.1.

- The queue's lines (`room woken up`, `proxy client's command sent`, the
  duty cycle's waits) reach `serve`'s log, its file and `-log-level`,
  rather than Go's default logger on stderr.

- A DNS-01 hook on a box without `curl` or `jq` says which one is
  missing, where it said that no zone was found; the Debian package
  suggests both (`packaging/README.md`).

- A pruned command's id is never given to a later one: `commands` has
  `AUTOINCREMENT` (migration 0014), as the backups have since 0013.

- The audit is pruned, as docs/store.md said it was: `audit.keep_days`
  keeps a year by default, 0 keeps every row.

- `massimo user` no longer creates a store of its own in the current
  directory: it asks for `-data` or names the service's command, says
  which store it wrote, and warns of one that is not the service's.

- Only a version tag of the full shape, `vX.Y.Z` or `vX.Y.Z-<label>.N`,
  counts as a release: a tag such as `v1.2` or `v1..2-alpha.1` no longer
  sets the build's version, the apt pool, the site's index or a flavour.

- Signed out, the web UI shows only its name, its version and the
  sign-in, where Sign out left the last user's rooms, Cube and clock
  drawn behind it.

- The sign-in is a card in the page instead of a modal dialog, with one
  password field, so that password managers such as KeePassXC fill it.

- A server no longer lists its own proxies as Cubes found on the LAN when
  their Cube was added or changed after its start, or when their answer
  comes from another address, as through NAT.

- The web UI's page scrolls down from the top in small steps again with
  expert mode on or the found Cubes open: the browser took it back to
  the top each time the header compacted, and the header flickered.

### Security

- The Homebrew tap's deploy key is held by the `signing` environment,
  as the release keys are, no longer by any run of the repository.

## [0.14.0-alpha.1] - 2026-11-11

### Added

- A found Cube lists the proxies that answer for it and the addresses it
  uses, gateway, netmask and DNS; the web UI shows "via proxy at …"
  (`docs/api-v1.md`, "GET /api/v1/cubes").

- The files massimo writes go through `internal/safefile`: whole, synced
  and renamed into place, keeping the replaced file's link, owner, mode
  and attributes, or in place where only the file is writable.

### Changed

- CI builds a release beside the tests and signs it once they have
  passed, about five minutes sooner; the nightly build has a workflow of
  its own (`GUIDELINES/project/CI.md`, section 2).

- A request to the API without a token, ticket or `?token=` is still
  refused with 401 but no longer counts toward the address's lockout:
  only wrong tokens, tickets and passwords do (`docs/api-v1.md`).

- serve's search asks each Cube for the addresses it uses as well, and
  massimo discover asks all Cubes at once; a proxy is told by its MAC or
  by its silence to N and c (`README.md`, "Command line").

- A change to the plans, the guidelines' documents or the changelog
  alone runs the Guidelines checks and no tests, and makes no devel
  build (`GUIDELINES/project/CI.md`, section 2).

- CI does not test again code that has passed: a branch's retirement
  after its green run, and its merge into `main`, run only what had not
  run (`GUIDELINES/project/CI.md`, section 2).

### Fixed

- A package built between two releases sorts after the one it follows,
  `0.13.0~alpha.2+3.gabc1234`, so that apt no longer takes it back to
  that release (`packaging/README.md`, "Debian package").

- The Debian packages are named as GitHub names their assets, the `~`
  and `+` of the version written as dots: `sha256sum -c SHA256SUMS`
  checks them, and the `devel` and `nightly` builds keep theirs.

- A release fails where GitHub renames a file it uploads, before a
  `devel` or `nightly` build moves its tag and deletes the files it does
  not name (`packaging/release-assets.sh`).

- A Cube heard from its own address and from a proxy is one found Cube,
  and serve following it by discovery stays at one of them: it moved to
  each answer in turn, every run (`docs/api-v1.md`).

- The settings file watch's test writes each content whole, renamed into
  place: a busy runner paused between truncating the file and writing it,
  and the watch saw two changes where the test made one.

- A `massimo backup` file and a store copy cut short by a kill or a full
  disk no longer replace a good backup or leave a partial database under
  the copy's name (`docs/store.md`).

- A pruned backup's id is never given to a later backup, so a list
  loaded before the prune can no longer delete or download another
  backup by its id (migration 0013; `docs/store.md` section 9).

### Security

- `massimo install` hands the files it wrote to the service user with
  `chown -h`: a link the service put in their place no longer has root
  give it the link's target.

## [0.13.0-alpha.2] - 2026-11-10

### Fixed

- `massimo install` over a running Windows service waits until the
  service's process has exited before it replaces `massimo.exe`, in
  place of failing now and then with "Access is denied".

- `make release-alpha` fetches the version tags alone, so a rolling
  tag that CI moved since the checkout last fetched it no longer
  stops it ("would clobber existing tag").

- A release commit is tested once, by its tag's run: CI's run of `main`
  skips it and makes no `devel` build under the release's own version.

- CI's macOS job no longer fails now and then with exit code 141: a pipe
  into `grep` reads to the end, so no writer is cut off.

## [0.13.0-alpha.1] - 2026-11-10

### Added

- Every release carries its build record, `massimo-build.txt`, listed in
  the signed manifest, and GitHub's build provenance of every file it
  publishes (`README.md`, "Installing").

- The site is deployed by `.github/workflows/site.yml` at every run of
  the release workflow, so that a push to `main` updates its docs; CI's
  `Site` job builds the docs of every pull request.

- `packaging/site/build.sh` builds the site without a key: the apt
  repository from a release's index asset, the pages, and the docs of
  every channel under `/docs/`, linked from the home; `make site`.

- A release has a flavour, and a box takes its channel's releases and
  every more stable flavour's; the rolling `devel` and `nightly` builds
  verify with a key of their own (`README.md`, "Versions").

- The setting `update.channel` chooses the channel a box follows, its
  build's flavour by default, and an update to another flavour keeps it
  (`docs/api-v1.md`, "GET /api/v1/update").

- Every push to `main` whose tests pass is published as the `devel`
  build, one release replaced in place (`GUIDELINES/project/CI.md`,
  "Releases").

- A `nightly` build, once a day, of the newest commit of `main` whose
  tests passed, if newer than the last one (`GUIDELINES/project/CI.md`,
  "Releases").

- A release carries its signed apt index, `massimo-apt-index.tar.gz`, for
  a site rebuilt without the key (`packaging/README.md`, "The apt
  repository").

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

- `make deb-systemd-test` checks the package under a real systemd, the
  upgrade from v0.12.0-alpha.1 with the service running among its cases,
  and CI runs it on trixie and bookworm (`GUIDELINES/project/CI.md`).

- CI's macOS job installs on the template's settings first, the owner
  line run as printed, a loaded job restarted and a booted-out one left
  stopped (`packaging/launchd/template-test.sh`).

### Changed

- A release waits for CI's whole suite on its tag and takes its flavour
  from the tag's label, `stable` without one; the `channel:` line is no
  longer read (`GUIDELINES/project/CI.md`, "Releases").

- The release branch is `release/alpha`, moved by CI once the release is
  out; `make release-alpha` pushes only `main` and the tag (`README.md`,
  "Versions").

- The session handover and the 2026-11-05 review left `docs/` for
  `GUIDELINES/wip/`, its coding standard for the contract; the write
  test's plan is `docs/testing-writes.md`, its reports `docs/reports/`.

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

- `make lan-test` checks discovery, the mDNS announcement and
  `discover` in Docker, against avahi, with a Cube's cable pulled and a
  second server's proxy (`tools/lan-test/README.md`).

- CI checks the announcement beside mDNSResponder and Windows' mDNS,
  and runs discovery's tests on Windows as well
  (`GUIDELINES/project/CI.md`).

- serve takes a silent Cube for lost and reconnects: after about 30 s
  without traffic, or two polls in a row unanswered; it waited three
  polls and 30 s (`README.md`, "Server").

### Fixed

- A client of the proxy that asks for the pairing state as soon as the
  Cube's end of pairing is relayed sees pairing ended, not still active
  (`docs/proxy.md`, "Pairing").

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

- serve's proxy no longer answers discovery for a Cube it has lost,
  so other servers and the original software see the Cube go; serve
  stays findable by mDNS (`docs/proxy.md`, "Discovery").

- A device that joins during pairing is in the server's pairing state
  before a proxy client receives its N line; a race test found the
  other order now and then.

- The queue's observer test holds its first command until the next two
  are queued, as it assumed; under QEMU the armv6 job found the queue
  starting it first.

- `make fuzz` no longer fails a target on Go 1.26's own `-fuzztime`
  deadline, "context deadline exceeded" with no input written
  (golang/go#75804; `GUIDELINES/project/CI.md`, section 3).

- A finished restore no longer gets an automatic backup of the state it
  restored, which a backup check could take before the restored file
  became the reference; under QEMU the armv6 job found it.

- A Cube's loss and return are logged before their events, the loss
  also before the client reports the Cube lost, as `docs/proxy.md` says;
  under QEMU the armv6 job found the other order.

### Security

- The release build and the job holding the apt key restore no Actions
  cache, so that nothing another run wrote reaches a release
  (`GUIDELINES/project/CI.md`, "Releases").

- On Linux the service runs as `_massimo`, as on macOS: the package and
  `massimo install` rename their own `massimo`, spare a person's, and
  name the `chown` after an older takeover (`packaging/README.md`).

- `/var/lib/massimo`, the store, stays 0750 when the service starts;
  systemd made it 0755 at every start, so that every user of the box saw
  the names of its files (`packaging/README.md`).

- The web UI shows no self-signed certificate's fingerprint, which a man
  in the middle passes along; its notice, also in the sign-in dialog,
  asks to compare the browser's with `massimo tls show` (`docs/ui.md`).

- `serve -allow-reboot off` no longer allows the reboot: `serve` refuses
  an argument it does not take, and a switch takes its value after `=`,
  as `-allow-reboot=off` (`README.md`, "Server").

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

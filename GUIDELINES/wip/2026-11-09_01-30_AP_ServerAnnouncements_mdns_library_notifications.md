# AP ServerAnnouncements v1.0: mDNS, library, notifications

## TL;DR

What the server must offer before a phone app can find it,
embed it and be told by it: an announcement on the LAN, its wiring as a package,
and notifications pushed to a user's devices.
Each is of use without the app: `discover` lists the servers, the page
becomes installable with Web Push, and `serve` stays as it is.

- `serve` on the LAN → found by typing its address → announced by mDNS
  `_massimo._tcp` with node, version, TLS and fingerprint; `discover` lists
  servers and marks the proxy among the Cubes
- `cmd/massimo/serve.go` → 800 lines of wiring in the command → `internal/serve`: `Open`,
  `Run`, the command a thin caller
- events → the stream and the log only → rules per user (offline, window open,
  battery, frost, …) → Web Push (RFC 8030, 8291, 8292) to browsers and, later,
  the app through UnifiedPush; a local deliverer for an embedded server
- the web UI → a page → a service worker, a manifest, 👤 → Notifications

### Blocks

1. Library: `internal/serve` out of the command, behaviour unchanged
2. Announce: the mDNS responder, `serve -announce`, `discover` listing servers,
   the firewall notes
3. Rules: `internal/notify`, the `notify` preference,
   push subscriptions and their routes, the VAPID key
4. Deliver: `internal/push`, Web Push encryption and VAPID, the local deliverer,
   the `notification` stream event
5. Page: the service worker, the manifest, 👤 → Notifications in six languages,
   the UI tests

## Discussion

**Where it comes from.**
Roadmap items 16 and 17, an Android and an iOS app,
were investigated on 2026-11-08 and 2026-11-09 and decided at their TL;DR gate:
the app shows the existing web UI in a thin native shell,
connected to a `serve` on the LAN or elsewhere,
or over a server embedded in the app itself; several servers,
remembered with a LAN and a remote address; biometric unlock;
push notifications on events.
The owner split the work: what the server must offer is this plan,
so that a session can start on it at once;
the app itself, from the gomobile binding on, is AP MobileApp,
written separately.
This plan restates what it needs and cites no analysis.

**What the app needs from the server, and gets here:**

- a way to be found on the LAN other than typing an address (Block 2);
- the wiring of `serve` callable from a program, not only from the command,
  so that the app's core can start it in-process (Block 1);
- notifications decided and sent by the server, since the app cannot hold a
  connection open on a phone in the background, and the same rules firing
  locally when the server runs inside the app (Blocks 3 and 4).

**What each is worth without the app:**
`massimo discover` and any mDNS browser show where a `serve` is;
the page gets a manifest, a service worker and Web Push, so that a phone's
browser installs it and is told when a window stays open or the Cube goes
offline (design.md section 8, "an installable web app stays the free route
with Web Push"); `serve` itself behaves as before.

**Open at the plan gate:**

- **D1.
  The mDNS responder.**
  A (recommended): a small responder of our own in `internal/announce` over
  `golang.org/x/net/dns/dnsmessage` and `x/net/ipv4`, `ipv6`,
  which the module already requires indirectly: about 400 lines,
  answers for its own records only, no new dependency.
  B: `github.com/brutella/dnssd` (MIT):
  probing and conflict handling we do not need,
  a new module with its own dependencies.
- **D2.
  Where a user's notification choices live.**
  A (recommended): the preference key `notify` through the existing
  `/api/v1/me/preferences/{key}` routes, a documented schema, no migration;
  a guest, who has no `preferences`, gets none.
  B: a `notify_rules` table with routes of its own: more API, more roles work,
  the same result.
- **D3.
  The capability of a push subscription.**
  A (recommended): `preferences`, which is "keep one's own settings on the
  server" already; manager, operator and viewer have it.
  B: a new capability `notify`: a migration seeding it into the roles,
  the roles UI, the docs; it buys only the ability to let a guest subscribe.
- **D4.
  The announcement's switch.**
  A (recommended): `serve -announce on|off` and `MASSIMO_ANNOUNCE` in the settings file,
  on by default when the listener is reachable from the LAN (not `127.0.0.1`),
  off otherwise,
  so that an embedded server on a phone announces nothing unless asked.
  B: a store setting as the intervals are, so that Settings → Server
  (roadmap item 6) can show it; it can move there when that item runs.

**Names chosen here**, routine and not decisions of the owner's:
the packages `internal/serve`, `internal/announce`,
`internal/notify` and `internal/push`;
the service type `_massimo._tcp` and its TXT keys `node`, `v`, `tls`, `fp`,
`name`; the preference key `notify`;
the routes `/api/v1/me/push` and `/api/v1/push/vapid`;
the stream event `notification`; the table `push_subscriptions`;
the branch `claude/server-announcements`.

**Not in this plan:**

- the gomobile binding of `internal/serve` and the Makefile targets for the
  `.aar` and the XCFramework: Block 1 of AP MobileApp,
  which needs Block 1 here first;
- notification texts on the server: a notification carries its kind and values,
  and the service worker or the app renders the text in the user's language from
  the UI's translation tables;
- e-mail, webhooks and MQTT as delivery (design.md section 5): a later plan,
  on the `Deliverer` interface of Block 4;
- an APNs relay for iOS: AP MobileApp, with item 17;
- telling a proxy from a Cube by its MAC prefix (roadmap item 18): here only
  the address match with an announced server;
- a `-announce` store setting and its row in Settings → Server (D4-B).

**Who runs it:**
a session of its own, after the plan's registration on `main` by the
coordinating session or the owner.
The work branch `claude/server-announcements` starts from `origin/main` after it;
one pull request per block,
merged before the next block that depends on it starts,
so that AP MobileApp can begin on Block 1's merge.
Blocks 1 and 2 are independent of each other; 3, 4 and 5 follow in order.

## Analysis

### The server as a library (Block 1)

`cmd/massimo/serve.go` (644 lines) and `servetls.go` (169) assemble everything:
the log, the TLS source, the store, the seed from the settings file,
the Cubes and proxy listeners, the server's options, the listener,
the ACME helper, the settings reloader, SIGHUP, the restart and update helpers,
the reboot.
A program that wants a running server has nothing to call.

What moves into `internal/serve`, as `Open(ctx, Config) (*Instance, error)`
and `(*Instance).Run(ctx) error`:

| Today in the command | In the package |
|---|---|
| `openServeStore`, `closeStore` | `Open` opens the store of `Config.DataDir`, or none with `Config.Addr`; `Run` closes it after the server stopped, within the 10 s grace |
| the store's intervals winning unless a flag was given | `Config` values that are zero take the store's |
| `serveSetup`, `rowTarget`, `fileCubes`, `fileCube`, `parseHidden` | unchanged, in the package; the settings file's Cubes read through `internal/env` as now |
| `tlsSource` (`servetls.go`) | `Config.TLS` with the mode, files, names, ACME values; the source made in `Open` |
| `server.New(...)`, `net.Listen`, the first log line, the ACME HTTP helper, `srv.Run`, `serveHTTP` | `Run`; `Instance.Addr()` the bound address, `Instance.Server()` the `*server.Server`, `Instance.Store()` |
| `serveReboot`, `serveUpdateHelper`, the restarter | stay optional: `Config.Restart`, `Config.Reboot`, `Config.Update` as the `server` options they are, filled by the command; empty for an embedder |

What stays in the command: the flags and their help, the log file and level,
the signal context, the service context, the Windows restart helper,
the settings reloader and its watcher, SIGHUP, `SetConfigView`, the exit codes.
`runServe` becomes: parse, build `Config`, `serve.Open`, wire the
command-only hooks on `inst.Server()`, `inst.Run(ctx)`.

**Behaviour does not change**,
which the existing tests of `cmd/massimo/serve_test.go`, `serve_seed_test.go`,
`serve_stop_test.go`, `servetls_test.go` and `cubecmd_test.go` guard.
A new `internal/serve/serve_test.go` starts an instance over a temporary
data directory against `massimo fakecube`'s greeting, reads `/healthz`,
cancels the context and checks the store is closed and the data directory
holds `massimo.db`: the call an embedder makes.

**Risk:**
`selectedSN` and `snExplicit` are package-level variables of the command (`snFlag`);
the package takes them as `Config.Serial` and `Config.SerialGiven`.
`dataDir()`, `versionString()`, `env.File()` likewise become `Config` fields,
filled by the command as now.

### The announcement (Block 2)

**Why mDNS.**
The Cube's UDP discovery (port 23272) is answered by `serve`'s proxy as a Cube:
a client cannot tell the two apart, and the answer carries no API port.
Phones browse DNS-SD by nature: Android's `NsdManager`, iOS's `NWBrowser`;
so do `avahi-browse` and `dns-sd`.
design.md section 8 names `_massimo._tcp`.

**The records.**
For a `serve` whose listener is reachable from the LAN:

| Record | Value |
|---|---|
| PTR `_massimo._tcp.local` | `<name>._massimo._tcp.local`, `<name>` the box's hostname, `massimo` without one |
| SRV | port: the listener's; target: `massimo-<node8>.local`, `<node8>` the first eight hex digits of `node_id`, so that the OS's own `<hostname>.local` record is never contested |
| TXT | `node=<node_id>` `v=<version>` `tls=0|1` `fp=<SHA-256 of the certificate, with TLS>` `name=<hostname>` |
| A, AAAA for the target | the host's unicast addresses that are not loopback |

A server without a store (`-addr`) has no `node_id` and announces nothing;
a listener on `127.0.0.1` announces nothing (D4-A).
The announcement is sent unsolicited at the start and every two minutes
(the TTL is 120 s for the PTR and SRV, 4500 s for TXT, as RFC 6762 suggests
for shared and unique records), and a goodbye with TTL 0 at the stop.

**The responder** (`internal/announce`, D1-A):
one UDP socket on `224.0.0.251:5353` and one on `[ff02::fb]:5353`,
bound with `SO_REUSEADDR` and, where it exists, `SO_REUSEPORT`,
since avahi on the Pi,
mDNSResponder on macOS and Windows' own mDNS hold the port too and every
responder on a host receives every query;
it answers queries for its own names only: PTR for the service type, SRV,
TXT and A/AAAA for its instance and target,
with the known-answer suppression of RFC 6762 section 7.1 ignored at first (a
query answered twice costs nothing).
A legacy unicast query (source port not 5353) is answered by unicast to its sender,
which is how the tests ask:
`TestAnnounceAnswersQuery` sends a PTR question from an ephemeral port on
loopback and checks PTR, SRV, TXT and A in the answer;
`TestAnnounceGoodbye` sees TTL 0 at the stop.
Multicast on loopback is not needed in CI.
A socket that cannot be bound or joined (WSL 2 in NAT mode,
a container without multicast) is a Warn line and no announcement,
never a failed start.
The wire format is `golang.org/x/net/dns/dnsmessage`, the multicast join
`x/net/ipv4` and `ipv6`; both packages are in `golang.org/x/net` v0.60.0,
which `go.mod` carries indirectly (checked 2026-11-09), so the change is a
direct requirement, not a new module.

**`massimo discover`** browses `_massimo._tcp` for the same wait as the Cube discovery,
by a multicast PTR query and by a unicast one to the addresses of `-addr`,
and lists the servers in a second table: NAME, ADDRESS (host:port), TLS,
VERSION, NODE.
A found Cube whose address is an announced server's is marked `proxy` in the Cube table,
the half of roadmap item 18 that costs two lines.
`-json` carries both lists.
`serve`'s own `POST /api/v1/cubes/discover` is not changed: the server does
not list its peers.

**The firewall.**
The Windows rule of `massimo install` is per program and covers UDP 5353 already
(`internal/install/windows.go`); its comment and the READMEs say so.
The Debian and ufw hints gain `5353/udp`; macOS's rule is per program.
The settings file's template gains `MASSIMO_ANNOUNCE`.

### Notifications (Blocks 3 and 4)

**What exists.**
Every change the Cube reports is an `event` in `internal/server` (`server.go`):
`connected`, `disconnected`, `hello`, `metadata`, `config`,
`state` with `changes` per value (`window_open`, `battery_low`, `error`,
`link_error`, `setpoint`, `actual_temp`, …), `pairing_started`, `paired`,
`pairing_ended`, `device_forgotten`, `cube_found`, `restore_step`,
`restore_done`.
`runtime.emit` hands each to the stream's subscribers and to the log;
`Server.broadcast` sends one to every Cube's stream.
Nothing acts on them.

**Why the server decides and sends.**
A phone app cannot keep the event stream open in the background:
Android's Doze suspends network access of an app that is not visible,
and iOS suspends the app itself within about half a minute;
both platforms deliver pushes through a service the OS keeps open.
On Android the free route is UnifiedPush,
whose messages are Web Push messages (RFC 8030 transport, RFC 8291 encryption;
F-Droid's post of 2026-01-08 on its fifth year says it "has always been
compatible" with them),
and whose Google Play fallback is an embedded FCM distributor in the app,
not in the server.
Firebase itself is forbidden on F-Droid (its inclusion policy, read 2026-11-08).
Browsers take the same Web Push with VAPID (RFC 8292): Chrome, Firefox,
and Safari for a page added to the Home Screen (as of 2026-11).
So one module in the server serves the installable web app now and the app later;
iOS's APNs needs a relay, which is AP MobileApp's.

**The rules** (`internal/notify`, Block 3).
A `Sink` the server calls from `emit` and `broadcast` with the Cube's serial
and the event, off the emitting goroutine (a channel, dropping nothing:
a slow deliverer must not slow the Cube).
The kinds, each with a default and what sets it off:

| Kind | Fires when | Default |
|---|---|---|
| `cube_offline` | `disconnected`, and no `connected` within the delay | on, 10 min |
| `cube_online` | `connected` after a `cube_offline` was sent | on |
| `window_open` | a `window_open` change to true, still open after the delay | on, 30 min |
| `battery_low` | a `battery_low` change to true | on |
| `device_error` | an `error` or `link_error` change to true | on |
| `frost` | `actual_temp` under the threshold | on, 8 °C |
| `device_stale` | a device past the stale threshold (`view.StaleAfter`) | off |
| `paired` | `paired` | off |
| `cube_found` | `cube_found` | off |
| `update` | a newer release seen by `GET /api/v1/update`'s check | off |

A rule holds its state in memory (window open since, offline since, sent at);
a restart forgets it, which costs at most one repeated notification.
Coalescing: one notification per kind and device per 30 minutes, and the
delayed kinds (`cube_offline`, `window_open`) cancel when the condition
clears before the delay.
The frost threshold and the delays are the user's (D2-A),
in the `notify` preference:

```json
{"cube_offline": {"on": true, "after_min": 10}, "window_open": {"on": true, "after_min": 30},
 "frost": {"on": true, "below_c": 8}, "battery_low": {"on": true}, "device_error": {"on": true},
 "device_stale": {"on": false}, "paired": {"on": false}, "cube_found": {"on": false}, "update": {"on": false}}
```

A missing key takes the default.
The rules run once per Cube for every user that has a subscription, with
that user's preference; a user without one costs nothing.
The notification is data, no text:

```json
{"kind": "window_open", "cube": "KEQ0000000", "device": "0a0102", "room": 3,
 "room_name": "Bath", "device_name": "Window", "since": "2026-11-09T06:12:00Z",
 "values": {"minutes": 30}, "time": "2026-11-09T06:42:00Z", "url": "/"}
```

**Subscriptions**
(Block 3).
Migration `0009_push_subscriptions.sql`: `id`, `user_id`,
`token_id` (the session or key that subscribed,
so that revoking it under My profile removes the subscription), `endpoint`,
`p256dh`, `auth`, `label`, `created_at`, `last_ok_at`, `failures`.
Routes, capability `preferences` (D3-A), `x-capability` in the OpenAPI file,
`TestOpenAPICoversRoutes`:

- `GET /api/v1/push/vapid`:
  `{"key": "<base64url P-256 public key>"}`,
  the key the page gives `pushManager.subscribe`;
  generated at first use and kept in the settings table as `push.vapid` (the
  private key stays in the store, mode 0600, like the OPAQUE server key;
  `GET /api/v1/settings` does not show it); capability `read`.
- `GET /api/v1/me/push`:
  the caller's subscriptions, without the keys.
- `POST /api/v1/me/push` with a browser's `PushSubscription.toJSON()` (`endpoint`,
  `keys.p256dh`, `keys.auth`) and a `label`: `201` with the row;
  the same endpoint again updates the row.
- `DELETE /api/v1/me/push/{id}`:
  `{"ok": true}`.

Audit kinds `push.subscribe` and `push.unsubscribe`, no command.
Without a store the routes answer `503`, without a signed-in user `409`,
as the preferences do.

**Delivery** (`internal/push`, Block 4).
The `Deliverer` interface: `Deliver(ctx, Subscription, Notification) error`,
with two implementations:

- `WebPush`:
  RFC 8291's `aes128gcm` (an ephemeral P-256 key from `crypto/ecdh`,
  HKDF-SHA256 from `golang.org/x/crypto/hkdf`, already a dependency,
  AES-128-GCM, record size 4096),
  a VAPID JWT signed ES256 with the server's key (RFC 8292,
  `aud` the endpoint's origin, `exp` 12 h,
  `sub` `mailto:` the owner's address if set, else the box's `node_id` URN),
  the POST with `TTL: 86400`,
  `Urgency: normal` (`high` for `frost` and `cube_offline`),
  `Topic: <kind>-<device>` so that a push service replaces an undelivered one of
  the same kind.
  `201` is delivered; `404` and `410` remove the subscription; `429` and
  `5xx` retry three times over a minute; any other error counts a failure,
  and a subscription failing for seven days is removed with a log line.
  Tested byte for byte against RFC 8291's Appendix A example (the keys,
  salt and plaintext there give the ciphertext there), and end to end
  against an `httptest` push service that decrypts with the test
  subscription's keys and checks the VAPID signature.
- `Local`:
  a Go callback `func(Notification)`,
  for the server embedded in an app (AP MobileApp), and for tests.

Both run behind the `Sink` of Block 3;
a notification also goes to the Cube's stream as an event of kind `notification`,
so that an open page shows it at once, in the user's language, without a push.

### The page (Block 5)

The web UI (`internal/server/ui/`, plain HTML, CSS and JavaScript,
texts through `t()`, `docs/ui.md`):

- `ui/sw.js`, served at `/sw.js` with scope `/` (the page lives at `/`;
  files under `/ui/` would scope the worker to `/ui/`): on `push`,
  parses the notification,
  loads `/ui/i18n/<lang>.json` for the user's language (cached in the worker;
  the language from the `ui` preference the page writes into the worker's
  storage at sign-in), renders title and body (`notify.window_open.title`,
  `.body` with the values), shows it with the page's icon;
  on `notificationclick`, focuses or opens the page at `url`.
  A `notification` stream event does the same inline while the page is open.
- `ui/manifest.webmanifest` and icons (192 and 512 px, maskable), linked
  from `index.html`: the page becomes installable; `display: standalone`,
  the colours from the scheme.
- 👤 → **Notifications** (`partials/notify.html`, `notify.js`):
  a switch per kind with its delay or threshold where it has one,
  saved to the `notify` preference; **On this device**:
  asks the browser's permission, subscribes with the VAPID key,
  posts the subscription with a label (the browser's `User-Agent`, editable);
  the list of the user's devices with **Remove**; a line that a guest,
  an open server and the API token have no notifications, and why.
  At most 640 px wide, as My settings is.
- Texts in all six `ui/i18n/<lang>.json`, FR, IT, NL and PL machine-
  translated and marked; the help button per setting.
- `securityHeaders` in `ui.go`: `worker-src 'self'`,
  `manifest-src 'self'` in the CSP.
- `tools/ui-test/`:
  the Notifications page at 1100 and 390 px,
  the switches and the subscription against a fake `pushManager`,
  axe without violations; the screenshot set gains the page.

**Risks:**

- Web Push in Safari needs the page installed to the Home Screen, and
  Firefox for Android has had gaps; the page says "not available in this
  browser" where `PushManager` is missing rather than failing.
- A push service may refuse a payload over 4 KB;
  the notification is a few hundred bytes.
- The VAPID private key is a secret in the store; a `store backup` carries it,
  as it carries the OPAQUE key today.

What changes, by file:

| File | Block | Change |
|---|---|---|
| `internal/serve/*.go` | 1 | `Config`, `Open`, `Instance`, `Run`; the moved setup functions; `serve_test.go` |
| `cmd/massimo/serve.go`, `servetls.go` | 1 | the thin caller; the command-only hooks |
| `internal/announce/*.go` | 2 | the responder, the browser used by `discover`, the tests |
| `cmd/massimo/serve.go`, `discover.go` | 2 | `-announce`, the servers table, the `proxy` mark |
| `internal/env/env.go`, `packaging/*/env*`, `internal/install/windows.go` | 2 | `MASSIMO_ANNOUNCE`; the firewall comment |
| `README.md`, `packaging/README.md`, `docs/proxy.md` | 2 | the announcement, 5353/udp, `discover`'s servers |
| `internal/notify/*.go` | 3 | the rules, the `Sink`, the preference schema, the tests |
| `internal/store/migrations/0009_push_subscriptions.sql`, `internal/store/push.go` | 3 | the table and its accessors |
| `internal/server/push_api.go`, `server.go`, `audit.go` | 3 | the routes, the `Sink` calls, the audit kinds |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml`, `docs/store.md` | 3 | the routes, the `notify` schema, the table |
| `internal/push/*.go` | 4 | `WebPush`, `Local`, the RFC 8291 vector test, the `httptest` service |
| `internal/server/server.go` | 4 | the `notification` stream event |
| `docs/api-v1.md` | 4 | the event, the notification object |
| `internal/server/ui/sw.js`, `manifest.webmanifest`, `icons/`, `index.html`, `partials/notify.html`, `notify.js`, `i18n/*.json`, `style.css` | 5 | the page |
| `internal/server/ui.go` | 5 | `/sw.js`, the manifest, the CSP, `uiPartials` |
| `tools/ui-test/`, `docs/ui.md`, `docs/screenshots/` | 5 | the tests, the page's section, the set |
| `CHANGELOG.md` | all | a bullet per block under Unreleased |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`, `make race`,
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD` from
the main checkout; Block 5 adds `make ui-test` at 1100 and 390 px.
Nothing reaches a real Cube, a box or the network: the fake Cube answers,
the responder's tests ask by unicast on loopback,
and the push service is `httptest`.
One functional commit per block,
citing `AP ServerAnnouncements v1.0, Block <n>`,
with its CHANGELOG bullet in the wrapped style of its neighbours;
a pull request per block.

### Block 1: Library

1. `internal/serve/serve.go`:
   `Config` with the fields of the Analysis, `Open`, `Instance` (`Addr`,
   `Server`, `Store`, `Run`), the moved functions with their comments;
   the package comment says what an embedder calls and what the command adds.
2. `internal/serve/serve_test.go`:
   `TestOpenRunsAgainstFakeCube` as the Analysis describes;
   `TestOpenWithoutStore` for `Config.Addr`.
3. `cmd/massimo/serve.go`, `servetls.go`: `runServe` builds the `Config`
   and keeps the command-only parts; `tlsSource` moves, `tlsMode` stays for
   the config view.
   The existing tests pass unchanged, or with the function they call
   renamed where it moved.
4. `CHANGELOG.md`, Unreleased, Changed:

   ```
   - `serve`'s wiring is the package `internal/serve`, which a program
     embedding the server calls as the command does; nothing changes for
     `massimo serve` itself.
   ```

**Verification:**
the test gate; `massimo serve` against the fake Cube from the scratchpad with
`MASSIMO_ENV_FILE=/dev/null` logs the same first line as before (addr, auth,
restart, reboot, sighup, tls, data, node);
`cmd/massimo/serve.go` is shorter by what moved and holds no setup logic.

### Block 2: Announce

1. `internal/announce/responder.go`:
   `Service` (name, port, TXT, node),
   `Start(ctx, Service, *slog.Logger) (*Responder, error)`, the sockets,
   the records, the periodic announcement, the goodbye at `Stop`; `browser.go`:
   `Browse(ctx, wait, unicast ...net.IP) ([]Found, error)` for `discover`.
2. `internal/announce/responder_test.go`:
   `TestAnnounceAnswersQuery`, `TestAnnounceGoodbye`,
   `TestBrowseFindsResponder` (unicast on loopback),
   `TestAnnounceWithoutMulticastWarns` (a bind that fails).
3. `cmd/massimo/serve.go`:
   `-announce on|off` (default `MASSIMO_ANNOUNCE`,
   else on when the listener is not loopback),
   the `Service` from the store's `node_id`, the version, the TLS status;
   started after the listener, stopped with the server; `internal/env/env.go`:
   the variable.
4. `cmd/massimo/discover.go`:
   the servers table and the `proxy` mark; `-json` with `cubes` and `servers`.
5. `README.md`
   ("Server", the firewall lines: 5353/udp),
   `packaging/README.md` (the Debian and ufw hints, the Windows rule's comment),
   the settings file templates under `packaging/` (`MASSIMO_ANNOUNCE`),
   `docs/proxy.md` ("Clients find the proxy by themselves":
   the servers in `discover`).
6. `CHANGELOG.md`, Unreleased, Added:

   ```
   - `serve` announces itself on the LAN by mDNS as `_massimo._tcp`, with
     its node, version and certificate, unless `-announce off` or
     `MASSIMO_ANNOUNCE=off`; `massimo discover` lists the servers it hears
     and marks a proxy among the Cubes (`README.md`, "Server").
   ```

**Verification:**
the test gate; on a machine with multicast (not WSL 2), `avahi-browse -r
_massimo._tcp` or `dns-sd -B _massimo._tcp` shows the running `serve` and
`massimo discover` lists it, with the fake Cube's proxy marked;
in WSL 2 `serve` logs the Warn line and runs.

### Block 3: Rules

1. `internal/notify/rules.go`:
   `Notification`, `Prefs` with its defaults and `ParsePrefs`,
   the `Sink` with its channel,
   the rules of the table with their delays and coalescing;
   `rules_test.go` with a clock the tests drive: each kind fires and does not,
   a cancelled delay, coalescing, a preference that turns a kind off.
2. `internal/store/migrations/0009_push_subscriptions.sql`,
   `internal/store/push.go`: `AddPushSubscription`,
   `PushSubscriptions` (per user, and every one for the rules),
   `DeletePushSubscription`, `NotePushResult`, the removal with a revoked token;
   `push_test.go`.
3. `internal/server/push_api.go`:
   the four routes, the VAPID key's creation and storage; `server.go`:
   `emit` and `broadcast` feed the `Sink`; `audit.go`: the kinds;
   `push_api_test.go`: each route, the capability, `409` and `503`,
   the key stable across restarts.
4. `docs/api-v1.md` (the routes, the `notify` preference,
   the notification object),
   `docs/api-v1-openapi.yaml` (the routes with `x-capability`),
   `docs/store.md` (the table, the setting `push.vapid`).
5. `CHANGELOG.md`, Unreleased, Added:

   ```
   - Notification rules on the Cubes' events, per user in the `notify`
     preference, and push subscriptions under `/api/v1/me/push` with the
     server's VAPID key at `GET /api/v1/push/vapid` (`docs/api-v1.md`).
   ```

**Verification:**
the test gate; `TestOpenAPICoversRoutes` passes with the new routes;
a run against the fake Cube with a subscription posted by `curl` and the
fake Cube's `window_open` toggled logs the rule firing after the delay.

### Block 4: Deliver

1. `internal/push/webpush.go`:
   the encryption, the VAPID token, the POST and its result handling;
   `local.go`: the callback deliverer; `webpush_test.go`:
   `TestRFC8291Vector` (Appendix A, byte for byte), `TestVAPIDToken`,
   `TestDeliverRemovesGoneSubscription` (`410`),
   `TestDeliverRetries` (`503` then `201`), the `httptest` service.
2. `internal/notify/sink.go`:
   the deliverers behind the `Sink`,
   a notification to every subscription of the user,
   the results noted in the store; `internal/server/server.go`:
   the `notification` stream event to the Cube's subscribers of that user (the
   stream knows its user from the ticket).
3. `docs/api-v1.md`:
   the `notification` event and the delivery rules (TTL, urgency, topic,
   removal after `410` or seven days).
4. `CHANGELOG.md`, Unreleased, Added:

   ```
   - A notification is pushed to the user's subscribed browsers and
     devices by Web Push (RFC 8030, 8291 and 8292), which UnifiedPush
     takes as well, and sent on the event stream as `notification`
     (`docs/api-v1.md`, "GET /api/v1/cubes/{sn}/events").
   ```

**Verification:**
the test gate; the RFC vector test passes; against the fake Cube, a
subscription whose endpoint is the `httptest` service of the test, run as
a small program from the scratchpad, receives a decryptable message.

### Block 5: Page

1. `internal/server/ui/sw.js`, `manifest.webmanifest`, the icons;
   `ui.go`: the routes `/sw.js` and `/manifest.webmanifest`, the CSP,
   the partial in `uiPartials`; `index.html`: the `<link>`s.
2. `partials/notify.html`, `notify.js`: the page as the Analysis describes,
   with `components.js`'s `.field` rows and the help buttons; `app.js`:
   the 👤 menu entry and the `notification` stream event's inline toast;
   `style.css`: what the page needs.
3. `i18n/*.json`:
   every text in the six languages, the four machine- translated ones marked.
4. `tools/ui-test/`:
   `test_notifications.py` (the page, the switches,
   the subscription with a fake `pushManager`, the toast from a stream event),
   at 1100 and 390 px; the screenshot set.
5. `docs/ui.md`:
   "Notifications" under the 👤 menu; the installable page.
6. `CHANGELOG.md`, Unreleased, Added:

   ```
   - 👤 → Notifications chooses which events notify, with their delays and
     the frost threshold, and turns notifications on for this browser;
     the page is installable, with a manifest and a service worker
     (`docs/ui.md`, "Notifications").
   ```

**Verification:**
the test gate and `make ui-test` at 1100 and 390 px (`UI_TEST_PYTHON` to the
main checkout's venv, free ports); axe without violations;
on a phone's browser against a `serve` on the LAN, the page installs,
the permission is asked once,
and a window opened at the fake Cube notifies after the delay set.

## Proposed commit message

The plan's registration,
a commit of its own on `main` before the first project write:

```
chore(aps): +ServerAnnouncements v1.0

What the server must offer before a phone app can find it, embed it and be told by it, each of use without the app: an mDNS announcement, the wiring of serve as a package, and notification rules with Web Push delivery and an installable page.

Key changes:
- GUIDELINES/wip/2026-11-09_01-30_AP_ServerAnnouncements_mdns_library_notifications.md: the plan, five blocks, the server side of roadmap items 16 and 17.

File-revision-v1: GUIDELINES/wip/2026-11-09_01-30_AP_ServerAnnouncements_mdns_library_notifications.md::v1.0::2026-11-09::All::the first revision, by PLAN+
Assisted-by: claude-code:claude-fable-5-1
```

The blocks' commits:

1. `refactor(serve): The server's wiring as the package internal/serve`
2. `feat(announce): serve announces itself by mDNS, and discover lists the servers`
3. `feat(notify): Notification rules on the Cubes' events, and push subscriptions`
4. `feat(push): Web Push delivery with RFC 8291 and VAPID, and a local deliverer`
5. `feat(ui): Notifications in the web UI, a service worker and an installable page`

## Change History

- v1.0 (2026-11-09): the first revision, the server side of roadmap items
  16 and 17 split off at their TL;DR gate; D1 to D4 open at its gate.

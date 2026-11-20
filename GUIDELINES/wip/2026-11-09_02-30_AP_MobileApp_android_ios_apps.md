# AP MobileApp v1.1: the Android and iOS apps

## TL;DR

Roadmap items 16 and 17: an app for Android, on F-Droid and Google Play,
and one for iOS from the same source: the web UI in a thin native shell,
connected to a `serve` or with one embedded.
The connect page, the Android shell and the release's jobs are on `main`;
v1.1 adds the login by fingerprint the owner designed at the phone test.

- the token behind a switch → a key of the user's own,
  behind the phone's fingerprint, makes a new session; the page never sees it
- the server → `POST /api/v1/login/key`; `tokens.parent_id`:
  a revoked key ends its sessions
- asked → the user's choice per server: each return, each start, after a logout
- in the app → "Enable login by fingerprint", "Login by fingerprint",
  "Select server", also in the Cube choice
- Push → Block 11; the phone test's findings → Block 12
- first release → the client with the login; the core second

### Blocks

1. Core: `app/core` bound by gomobile, the Makefile targets, a test
2. Connect: the page `ui/connect.html` and the bridge contract (done)
3. Shell: the Android app in client mode, Kotlin, `foss` and `play` (done)
4. Release: the Android CI job, reproducibility, signing, the version file,
   the APK and AAB on the release (done)
5. F-Droid: the recipe, the submission, our signature;
   the owner's developer verification
6. Play: the listing, Data safety, the signing key, the closed test
7. Local: the stand-alone mode on Android, the foreground service
8. Login: the login by fingerprint: the server's route and a session's key,
   the bridge's calls, the Keystore key bound to the fingerprint,
   the page's buttons, "Select server" in the Cube choice
9. iOS: the Swift shell, the same bridge, the App Store (item 17)
10. Documents: `docs/app.md`, `app/README.md`, the README, the roadmap
11. Push: UnifiedPush and the embedded FCM distributor
12. Polish: the connect page's toasts and names,
    the self-signed notice in the app, the README's notes from the phone test,
    Dependabot for Gradle

## Discussion

**Where it comes from.**
The owner's words for item 16: run as a stand-alone service,
or connected to an existing Massimo; published on F-Droid and on Google Play;
item 17: the same for iOS, on the App Store.
Added on 2026-11-08 and 2026-11-09:
a route that builds an iOS package from the same source;
run the full service locally or connect to an existing one;
remember the address and choose a service's local or remote address by the
network the phone is on; several services to choose from; biometric login;
push notifications on certain events.

**What rests on the analysis.**
The investigation of 2026-11-08 and 2026-11-09 is beside this plan as
`2026-11-09_00-10_ANALYSIS_MobileApp_modes_stack_stores.md`;
its own title still says AndroidApp,
the plan's name before the owner chose MobileApp.
It holds the facts outside the repository with the date each was read:
F-Droid's inclusion policy and reproducible builds,
Android developer verification, Google Play's requirements,
Apple's guidelines and programme, gomobile's state, the platforms' stability,
UnifiedPush.

**Decided so far**, each recorded where it was taken:

- at the TL;DR gate of 2026-11-09: both modes, the client first;
  the web UI in a thin shell, a native adapter per platform,
  the Go core by gomobile; reproducible builds and our own signature on F-Droid,
  flavours `foss` and `play`;
  a personal account on Google Play with our own signing key,
  which runs the closed test of 12 testers for 14 days;
  `app/` in this repository; Web Push from the server as the one push transport;
  the toolchain in WSL;
- **D1-A** the application id `ch.metaworx.massimo`, **D2-A** `minSdk 26`
  with 24 in reserve, **D3-A** the app released with the server's tag
  (the NOTE of the plan gate);
- **D4-A** on a server's page the bridge has `token()` and `setToken(t)` without an id,
  checked by origin,
  **D5-A** the axe check in the UI suite (the NOTE of Block 2);
- **D6-A** the bridge is messages, not `@JavascriptInterface`,
  **D7-A** the Gradle wrapper is committed, **D8-A** `languages` gains `xml`,
  **D9-A** the icon is Massimo's mark,
  made by `make ui-brand` (the two NOTEs of Block 3);
- **D10-B** the phone test's findings in the page wait for this revision,
  **D11-A** this revision is the planning session's, and Block 8 becomes the
  login by fingerprint (the phone test of 2026-11-17);
- **D12 to D21** at Block 4's gate of 2026-11-17: the version file,
  the builds that carry the app, where it is signed,
  the sums and the attestation, the keys, the nightly app, the versionCode
  (`2026-11-17_18-27_NOTE_MobileApp_v1.0_block_4_refit.md`);
- **D22 to D31** at this revision's gate of 2026-11-18, below.

**Revision v1.1 (2026-11-17 and 2026-11-18).**
Blocks 2 and 3 are on `main`: the connect page with the bridge's contract,
`docs/app.md`, and the Android shell in client mode.
On 2026-11-17 the app passed the plan's check on the owner's phone,
once one fault was fixed: discovery, a pinned self-signed certificate,
the sign-in by OPAQUE, the token across a restart
(`2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`).
At that test the owner stated how the login is to work,
in place of the switch that guards the session token:
ten points, the NOTE's last section, which this revision turns into Block 8.
Push, which shared Block 8, becomes Block 11,
so that Block 8 is the login alone;
the test's findings in the page and for the README become Block 12.
Block 4 ran under a gate of its own while this revision was written:
it is on `main`, its decisions are D12 to D21,
and this revision's therefore start at D22.
Two slips of v1.0 are corrected: the fees read "5 once" and "9 a year"
where $25 and $99 were meant, and "D9-A" named the personal account,
a decision of the TL;DR gate without a number in this plan.

**Decided at this revision's gate (2026-11-18):**

- **D22-A, the credential the phone keeps**:
  an API key of the user's own, labelled with the phone,
  made by the existing `POST /api/v1/users/{name}/tokens`;
  a new route turns it into a session;
  it is listed and revoked under My profile → Sessions and keys.
- **D23-A, who makes and holds it**:
  the adapter.
  The sign-in form learns from it that the login by fingerprint is there;
  where the user chooses it, the page asks the adapter,
  which makes the session at the server and hands the page that alone.
  The key never enters the page.
- **D24-C, when the fingerprint is asked** (the owner's):
  the user's choice, of three: each time the app comes back from being hidden;
  when the app starts; only after a logout.
- **D25-A, the PIN**:
  a fingerprint or face of the strong class,
  with the screen lock's PIN as the prompt's fallback on Android 11 and later;
  on Android 8 to 10 the fingerprint alone,
  since a Keystore key bound to the prompt takes no PIN there.
- **D26-A, "Select server" once signed in**:
  on the sign-in form, and in the app in the Cube choice's list,
  as the owner asked;
  the test had found no way back from a signed-in page but Android's Back.
- **D27-A, the phone test's two findings outside the app**:
  two lines of the roadmap's "Small ones", the coordinating session's to write;
  what they are is under "Not in this plan".
- **D28-A, the first release**:
  the client with the login: Blocks 12 and 8 after 4, then 5 and 6;
  the core and the stand-alone mode, Blocks 1 and 7, second.
  F-Droid's first recipe is then Gradle alone.
- **D29-A, Dependabot for Gradle**:
  a step of Block 12.
- **D30-A, where D24's choice is made**:
  on the phone, per server, in its entry on the connect page,
  where the switch was; "when the app starts" until the user chooses.
- **D31-A, a key and its sessions** (the owner's point):
  a session names the key that made it, in a column of its own,
  `tokens.parent_id`; revoking a key ends its sessions,
  and a new session by a key ends that key's earlier one.
  `tokens.origin`, which the owner asked about, is taken:
  every table has it, the id of the node that last changed the row,
  for the sync.

**Open, for Block 8's gate:**

- **D32, the stored servers in the Cube choice**
  (the owner's thought of 2026-11-18):
  "Select server" heads a group of the list,
  and the stored servers are its entries.
  A server's page then learns the names the user gave the other servers.
  - A (recommended): their names and ids, nothing else: no address,
    no fingerprint, no user; choosing one is the adapter's to carry out.
  - B: the one entry "Select server", which shows the connect page;
    a server's page learns nothing of the others.

**Names chosen here**, routine and not decisions of the owner's:
the directories `app/core`, `app/android`, `app/ios`;
the Makefile targets `app-core-android` and `app-core-ios`;
the route `POST /api/v1/login/key` and its audit kind `login.key`;
the column `tokens.parent_id` and its migration, 0025;
the bridge's calls `login.status`, `login.keep`, `login.fingerprint`,
`login.forget`, `connect`, `forget` and `forgetAll`,
and with D32-A `serverNames` and `switchTo`;
a server's fields `login` and `ask`,
the latter one of `return`, `start` and `logout`;
the classes `LoginKey` and `LoginStore`; the branches `claude/mobile-core`,
`claude/mobile-login`, `claude/mobile-push` and `claude/mobile-polish`,
one per block.

**Not in this plan:**

- pairing by QR code (a code shown by the web UI with the URL, the
  fingerprint and a one-time token): a later revision;
- an APNs relay for iOS push, and iOS push at all in the first iOS release:
  decided when Block 9 starts;
- the sign-in through OpenID Connect inside the app's WebView: the login by
  fingerprint follows whichever sign-in succeeded, but a provider's page in
  the WebView is untested;
- one kept credential for both addresses of a server: the owner's design
  matches the address and port, so a server reached at its LAN and at its
  remote address keeps one for each;
- saving a file the page offers (a backup's download): the app says it
  cannot yet (the NOTE of Block 3); a later call of the bridge;
- the phone test's two findings outside the app (D27-A),
  for the roadmap's "Small ones".
  The installer's Windows Firewall rule covers the Domain and Private profiles,
  and the owner's Wi-Fi was Public.
  It stays so by default (the owner, 2026-11-18);
  the Public profile is opened on request,
  by an option of the install command or, the owner's preference,
  from the interface, the Maintenance page perhaps,
  behind a UAC prompt and not the elevated installer.
  A service has no desktop to raise that prompt on,
  which the line's own planning has to settle.
  And `massimo tls show` reads the settings, not the running server;
- the Cubes of every signed-in server in the Cube choice,
  with "Log in…" for a server that is not signed in,
  the owner's thought of 2026-11-18, for later:
  a server's page cannot read another server,
  so the adapter would fetch each one's Cubes; a later revision;
- widgets, a wear app, Android Auto;
  e-mail or webhook delivery of notifications.

**Who runs it:**
sessions of their own, one per block, after this revision is on `main`.
Blocks 12 and 8 first (D28-A), Block 8 with a gate of its own for D32;
both change the sign-in form and the connect page,
so the later one merges `main` with the other's change before its pull request.
Then 5 and 6; Block 1 at any time and 7 after it, for the second release;
11 after 8, since both change the bridge; 9 after 8, on a Mac;
10 spread over the others.

## Analysis

### What the app is

One page of our own, `connect.html`, bundled in the app from the same
`ui/` sources the server embeds, and the server's page itself, loaded
from the server the user chose or from the embedded core.
The adapter is native, small, and does what a page cannot.
The two talk through one object, `window.massimo`,
whose contract is `docs/app.md` (Block 2):
every call a Promise over the platform's message channel,
the adapter checking each message's origin,
so that the connect page gets the server list and a server's page only what is its own.

| What the adapter does | Calls and events | Block |
|---|---|---|
| the saved servers | `servers`, `save`, `remove` | 3 |
| discovery of `_massimo._tcp` | `discover`, the event `found` | 3 |
| reaching an address, the pinned certificate | `probe`, `trust`, `open`, the event `network` | 3 |
| a server's session, kept on the phone | `token`, `setToken` | 3 |
| the login by fingerprint, the way back to the servers | `login.*`, `connect`, `forget`, `forgetAll`; with D32-A `serverNames`, `switchTo` | 8 |
| the server on the phone | `core.*`, the event `core` | 7 |
| push to this phone | `push.*`, the event `notification` | 11 |

The page keeps a server as `{id, name, node, lan, remote, fp, …}`:
`node` is the server's `node_id` from `/healthz`, so that a LAN address that
changed is found again by discovery, and the page takes the address whose
`node_id` matches, the LAN one first.
No Wi-Fi name is read.

**The server's page in the shell.**
The page asks `token()` once, at its start, hands a new session to `setToken`,
and keeps none in `localStorage`.
OPAQUE runs in the page as in a browser.
The event stream, the tickets, the preferences and the "only on this
device" settings work unchanged, the WebView's storage being per origin.
The `User-Agent` begins with the app and the phone,
so that the session's label under My profile names them.

**TLS and plain HTTP.**
A certificate from Let's Encrypt verifies by itself.
A self-signed one is pinned on first use: the page shows the fingerprint,
the user compares it with the server's log or `massimo tls show`,
`trust` stores it,
and a changed fingerprint blocks until it is trusted the same way.
Plain HTTP is allowed,
and the page warns for an address outside the private ranges.

### The core (Block 1)

`app/core` is a Go package in the root module (it imports `internal/serve`,
which a module of its own could not),
bound by `gomobile bind` (golang.org/x/mobile, BSD-3-Clause,
newest of 2026-10-11, `go 1.26.0`; the analysis, section 2):

- `Start(dataDir, listen, optionsJSON string) error`:
  opens the store in `dataDir`,
  starts the server on `listen` (`127.0.0.1:8099` by default,
  so that an open server is reachable by the page alone),
  no proxy and no announcement unless the options ask;
- `Stop()`, `Status() string` (JSON: running, the address, the Cubes,
  the version);
- an `Events` interface the adapter implements: `OnStatus(json)`,
  `OnNotification(json)`, the latter AP ServerAnnouncements' local deliverer.

Bound types stay within gomobile's: strings, ints, bools,
an interface of such methods; no slices but `[]byte`, no maps;
a panic across the boundary ends the process,
so every entry point recovers and returns an error.
`go.mod` keeps `golang.org/x/mobile` through a `tool` directive,
since the bound package does not import it but `gomobile bind` needs it in the graph.
The Makefile targets: `app-core-android` (`-target android -androidapi 26`,
the `.aar` into `app/android/core/`, which is gitignored),
`app-core-ios` (`-target ios,iossimulator`,
the XCFramework into `app/ios/Frameworks/`), both with `-trimpath`,
`-buildvcs=false` and `-ldflags=-buildid=` for reproducibility,
both needing the NDK or Xcode, which the targets check for and name.
A test, `app/core/core_test.go`, starts the core over a temporary data
directory against the fake Cube's greeting, reads `/healthz`, stops it:
plain `go test`, no gomobile.
Both targets depend on `ui`, as every target that compiles Go does:
the server embeds the web UI's bundle, which `make ui` builds.

### The connect page (Block 2)

Done: `ui/connect.html` and `connect.js`,
in the server's embed and bundled by the shell from the same sources,
with the contract in `docs/app.md` and the UI suite's `test_connect.py` over a
fake bridge that Playwright puts into the page.
What this revision changes on the page is Block 8's (the kept credentials)
and Block 12's (the toasts and the names).

### The Android shell (Blocks 3, 7 and 11)

`app/android`:
a Gradle project (Kotlin, `compileSdk` and `targetSdk` 36, `minSdk 26`),
one module `app`, flavours `foss` and `play`,
the `ui/` files copied into its assets by a Gradle task,
never a second copy in git.

- **Client mode (Block 3), done**:
  `MainActivity` with the WebView and `WebViewAssetLoader` for the bundled page;
  `Bridge` over `addWebMessageListener` with `res/raw/bridge.js` injected at the
  document's start, and `Origins` checking every message; `ServerStore`;
  `Discovery` over `NsdManager`; `Prober` with the pinned trust manager;
  `TokenStore` under `KeystoreSealer`, an AES-GCM key of the Android Keystore;
  `Biometric` over `androidx.biometric`; the icon from Massimo's mark.
  The phone test of 2026-11-17 passed it;
  its one fault, the asset path, is fixed.
- **Stand-alone mode (Block 7)**:
  `CoreService`, a foreground service of type `connectedDevice` (for
  "interactions with external devices that require a … network connection";
  no six-hour limit, may start from `BOOT_COMPLETED`;
  its prerequisite met by declaring `CHANGE_WIFI_MULTICAST_STATE`) with a
  persistent notification; the core's data directory under the app's files;
  a `WifiLock`;
  the battery-optimization exemption asked for only when the user turns the server on,
  with the reason shown; an optional start at boot;
  "this phone is the server now: the Cube accepts one client" said in words;
  the LAN listener and the proxy off, a switch to let other devices connect,
  which requires a first owner as the server does on the LAN.
  The analysis, section 2, has Android's rules with their sources.
- **Push (Block 11)**:
  `org.unifiedpush.android:connector` in both flavours,
  the `embedded_fcm_distributor` in `play` only (a Firebase project and its
  `google-services.json` in the `play` source set, kept out of `foss`);
  the endpoint and keys posted to the server through the bridge (`POST /api/v1/me/push`);
  a received message rendered from the app's strings per kind into a notification channel,
  a tap opening the server's page;
  the embedded core's `OnNotification` into the same channel.

### The login by fingerprint (Block 8)

**What is there.**
A server's page hands its session to the adapter (`setToken`), which keeps it
sealed under a Keystore key and gives it back at the next start (`token`).
A server's switch "Ask for my fingerprint or PIN" puts a prompt before that:
the app's logic asks, the key itself is not bound to it.
The session lasts 30 days from its last use.

**The owner's design** (`2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`,
its last section), in short:
the sign-in form offers to enable the login by fingerprint;
after a sign-in that succeeded the app asks for the fingerprint,
makes a new token on the server and keeps it behind the fingerprint,
or offers to repeat, or to sign in without keeping anything;
a logout ends the current session alone;
the sign-in form then offers the password or a button "Login by fingerprint",
which makes a new session from the kept token;
"Select server" leads back to the connect page;
a server's settings turn it off and delete what is kept,
removing the server does too, and "Forget all stored credentials" deletes all;
the server list marks a server with kept credentials,
and a fingerprint button on its row skips the sign-in form;
kept and matched are the server's address and port,
the user's name and the token, all three.

**The kept token is an API key of the user's own**
(D22-A).
Every user may make one for itself today (`POST /api/v1/users/{name}/tokens`,
"the same rules of access as the user"); it never expires,
its label is the phone's model and "login by fingerprint",
and My profile → Sessions and keys lists and revokes it.

**The server gains one route**, `POST /api/v1/login/key`:
the bearer is an API key, the optional body a `label`,
and the answer the session `POST /api/v1/login` answers:
`token`, `expires_at`, the user, the role, the capabilities.
A session as the bearer is `400`; a wrong or revoked key `401`,
counted as every failed token check is; a disabled user or role `403`.
It works where `auth.password_login` is off, since it sends no password.
Its audit kind is `login.key`; it makes no command.

**A session names its key**
(D31-A).
A session is extended at each use, 30 days from the last:
without a tie, a revoked key's session would live as long as the phone uses it.
`tokens` gains `parent_id`, the row of the key a session was made by,
empty for a session of the sign-in form,
as `commands.parent_id` names a command's parent;
migration 0025 adds the column, and nothing is rebuilt.
`origin` could not carry it: it is in every table,
the id of the node that made the last change, for the sync.
`RevokeToken` of a key revokes its live sessions with it,
and what they subscribed to push;
the new route revokes the key's earlier session when it makes the next,
so a phone holds one live session per kept key.
The list under My profile shows a session's key by its label.
A password set by someone else ends a user's sessions and leaves the keys,
the phone's among them, as it does for every API key today.

**The adapter holds the key, the page never does**
(D23-A).
On a server's page the bridge gains, in version 2 of the contract:

| Call | What the adapter does |
|---|---|
| `login.status()` | whether the phone can (a strong biometric enrolled), whether credentials are kept for this address, with the user's name, and when the fingerprint is asked |
| `login.keep(user)` | asks for the fingerprint; makes the key at the server with the page's session and seals it; resolves `false` where the prompt was not passed |
| `login.fingerprint()` | asks for the fingerprint; opens the key, makes a session at the server with it and resolves to that session; a key the server refuses is forgotten |
| `login.forget()` | forgets the credentials kept for this address |
| `connect()` | shows the connect page: "Select server" |
| `serverNames()`, `switchTo(id)` | with D32-A: the stored servers' names and ids, the one shown marked; and the adapter opening another of them |

The sign-in form asks `login.status()` when it is drawn,
so it knows whether the login by fingerprint is there before the user chooses.
On the connect page a server carries `login`,
the user whose key is kept or `""`, in place of `biometric`, and `ask`,
one of `return`, `start` and `logout`;
`forget(id)` and `forgetAll()` delete;
`open(id, url, {login: true})` opens the page to sign in at once.
The adapter makes its two requests itself, over the connection `Prober` pins:
the key's creation and the session's.

**The Keystore key requires the prompt for every use**
(D25-A): `setUserAuthenticationRequired`, a class 3 biometric,
with the screen lock as the prompt's fallback from Android 11 (a prompt with a
crypto object takes no device credential before it,
and no class 2 biometric at all; androidx's reference, read 2026-11-17).
Sealing needs the prompt as opening does,
so "ask for the fingerprint, then keep" is one step.
A fingerprint enrolled later invalidates the key:
what was kept is dropped, and the form offers the password.
What is sealed is bound to the address and port, the user's name and the
server's pinned fingerprint, so that it opens for nothing else.

**The flows.**

- *Enabling*:
  the form's switch "Enable login by fingerprint",
  shown where `login.status()` says the phone can; after the sign-in,
  `login.keep`; where it fails, "Try again" or "Sign in without it".
- *When the fingerprint is asked*
  (D24-C, D30-A): the user's choice per server,
  in its entry on the connect page;
  a server without kept credentials keeps its session as today.
  - `return`, each time the app comes back from being hidden:
    the session is in memory alone;
    the adapter covers the page when the app is hidden,
    and the prompt uncovers it, the session and the page as they were,
    so nothing is loaded again and no draft is lost.
    The app's own dialogs, the file chooser and the prompt itself,
    do not count as hidden.
    Android may end a hidden app: the return is then a start.
  - `start`, when the app starts, the choice until the user makes one:
    the session is in memory alone,
    so a start shows the form with "Login by fingerprint",
    or signs in at once from the row's button, and makes a new session.
  - `logout`, only after a logout:
    the session is kept sealed on the phone as today,
    and the fingerprint is asked on the sign-in form alone,
    after a logout or once the session ended.
- *Logout*:
  `POST /api/v1/logout` as today, the session's end; the key stays,
  and the form offers both ways.
- *Select server*
  (D26-A): the sign-in form's button, and in the app the Cube choice's list,
  the `Picker` of `src/header/Choice.tsx`,
  which the part draws for one Cube too: after the Cubes and the search,
  "Select server", which shows the connect page.
  With D32-A it heads a group: the stored servers by name,
  the one shown marked, and "All servers…" for the connect page.
  A browser's page has no bridge and shows none of it.
- *Turning it off* in a server's settings asks for the fingerprint once,
  revokes the key at the server and forgets it;
  where the prompt or the server fails, the phone forgets it alone.
  "Forget all stored credentials" and Remove forget without a prompt.
  A key only forgotten stays listed at its server under the phone's name,
  to revoke there.
- *The old switch* goes: a server that had it on starts without kept
  credentials and with its session forgotten.

### Release and the stores (Blocks 4, 5, 6)

**The pipeline as it is**
(AP Channels, after v1.0): a release is cut with `make release <flavour>`,
which commits the store's dump,
has `changelog.sh cut` make the release commit and the signed tag,
and pushes `main` and the tag; CI tests the tag,
`build.yml` builds every file without a secret and attests it,
and `release.yml` signs and publishes in the environment `signing`;
`release/devel` and `release/nightly` are rolling builds of `main`.
The ruleset requires the checks "All tests passed" and Guidelines.

**CI, as built**
(Block 4): the job "Android app" of `ci.yml`,
run where "What to test" answers `app`, and reported by "All tests passed":
the version file checked, the debug build with its unit tests and lint,
the release APK built at two paths and compared byte for byte,
then the AAB and the nightly APK,
all three signed with a key made for the run and checked against its certificate.
With Block 1 the job gains `make ui` and `make app-core-android` first.

**The release, as built**
(Block 4): `build.yml` builds a version tag's `foss` APK and `play` AAB,
and a nightly build's APK, unsigned and attested;
`release.yml` signs them in `signing` ("Sign the Android app"),
adds them to the release's `SHA256SUMS` with its `SHA256SUMS.sig`
("Publish the Android app") and attests the signed files.
The nightly app, `ch.metaworx.massimo.nightly`,
installs beside the release's and is signed inside `rolling`,
with a key of its own.
Each signing compares the signer with the certificates under `app/android/signing/`.
The signed APK is the `Binaries` URL F-Droid compares its rebuild with,
and `build.yml`'s unsigned one what a rebuild is compared with byte for byte.
The version is the tag's (D3-A): `app/version.json`,
the changelog's manifest, its `version` written by `changelog.sh cut`
and its `versionCode` by `make release`,
`MMmmPPbbbb` (D21-A, `packaging/app-version.sh`);
Gradle reads it, and F-Droid's update check reads it at the tag.
Not seen yet: a version tag's signing,
which runs at the owner's next `make release`.
Dependabot has no entry for Gradle (D29-A, Block 12).

**F-Droid** (the analysis, section 4): the recipe in `fdroiddata`,
`metadata/ch.metaworx.massimo.yml` under D1-A: AGPL-3.0-or-later,
the repository, `Binaries` and `AllowedAPKSigningKeys` (our key's SHA-256),
`AutoUpdateMode: Version`,
`UpdateCheckMode: Tags` with `UpdateCheckData` on `app/version.json`;
the build entry with `subdir`, `sudo` for a bootstrap Go,
`srclibs` building the pinned Go from source, `sdkmanager` for the pinned NDK,
gomobile installed from the module cache, the core built in `prebuild`,
`gradle: [foss]`, the module cache `scanignore`d:
the shape of sing-box's and syncthing-android's recipes.
Submitted as a Request For Packaging once the first tagged release carries the APK;
the review takes weeks.
Until Block 7 builds the core into the app the recipe is Gradle alone;
with the core, `make ui` runs before `gomobile bind`.
The owner's steps: a Google Play Console account also registers the app for
Android developer verification (from 2027 every certified Android phone installs
only apps of a verified developer, F-Droid's included; $25 once;
a personal account); the package name registered with the key's fingerprint;
a lost key cannot be re-registered.

**Google Play** (the analysis, section 5): the owner's personal account,
identity verified; the app created under D1-A;
Play App Signing with our own key uploaded before the first open-testing
or production release; the listing in English and German at least,
screenshots from the UI suite's set; the Data safety form,
"no data collected" (every byte goes to a server the user chose);
the foreground-service declaration for Block 7; target API 36;
internal testing at once for the owner's phones,
then the closed test with 12 testers opted in for 14 days,
which a personal account created after 2023-11-13 must run, then production.
The tracks, as Block 4's refit NOTE proposes for Block 6's gate:
an alpha to the internal test,
a beta and an rc to the closed test and later the open one,
a stable release to production, the 14-day closed test run on an alpha.
The AGPL is no issue on Play; the listing names the licence and the source.

### The iOS shell (Block 9)

`app/ios`:
an Xcode project with Swift Package Manager, no CocoaPods;
`WKWebView` with the bridge of the same contract,
a `WKUserScript` over a `WKScriptMessageHandlerWithReply` (`docs/app.md`);
the bundled `ui/` served by a `WKURLSchemeHandler`; `NWBrowser` for discovery;
`URLSession` probes with pinning in the authentication challenge;
the Keychain and `LocalAuthentication`, the kept key of Block 8 under an
access control that asks for Face ID or Touch ID;
the ATS keys `NSAllowsLocalNetworking` and `NSAllowsArbitraryLoadsInWebContent`;
the local-network usage description and `NSBonjourServices`;
the XCFramework from `make app-core-ios`;
the stand-alone mode while the app is open (iOS suspends an app about 30 s after
it leaves the foreground, so no 24/7 and no push from it);
no push in the first release unless a relay is decided.
The App Store: the owner enrolls as an individual ($99 a year;
a sole proprietor must), the seller name the owner's legal name;
an additional permission to the AGPL for App Store distribution,
which the owner can grant as the copyright holder (the analysis, section 1),
in `app/ios/LICENSE-EXCEPTION.md` and the listing;
guideline 4.2 is met by what the shell adds over a web page (discovery,
biometrics, the embedded server, pinning).
CI: a job on the macOS runner, `xcodebuild` for the simulator, no signing.

### The phone test's findings (Block 12)

From `2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`,
what is neither the login nor the asset path fixed in the test:

- the connect page's messages ("added", "removed", "does not answer at its
  addresses") stay, the last even after a probe finds the server;
  the owner wants toasts, `showToast` of `components.js`,
  which the page loads, with the markup of `partials/toast.html`;
- a server added by its address is named by it,
  so its row shows `host:port` twice;
- the web UI's notice for a self-signed certificate ("compare the SHA-256
  fingerprint in your browser's certificate details") shows in the app,
  on the sign-in form (`login-tls`) and in the header (`src/header/Tls.tsx`,
  `header.js`), although the app pinned the fingerprint the user compared;
  the warnings of a certificate near its end stay;
- for `app/README.md`: the test's rig (a `serve -tls self-signed` against the
  fake Cube in WSL, `adb reverse`,
  the WebView's DevTools port of a debug build),
  a phone's VPN that does not allow LAN connections and so reaches no LAN server,
  and `adb reverse` stalling on one port.

### Risks

- gomobile is experimental by its own README; the core's surface is small,
  and `-buildmode=c-shared` with a hand-written JNI layer is the fallback behind
  the same `app/core` API.
- Reproducibility: gomobile has leaked the checkout path into the library's build info;
  the CI check of Block 4 catches it before F-Droid does.
- The 14-day closed test delays production on Play; F-Droid and the
  GitHub release do not wait for it.
- A kept key is as strong as the phone's Keystore and its fingerprint;
  its revocation is one click under My profile, by the phone's name.
- Android ends a hidden app often,
  so `return` and `start` ask for the fingerprint often;
  `logout` is the price's other side, and the choice is the user's (D24-C).
- The cover of `return` is the app's own, as the old switch's prompt was:
  it stops a hand on the unlocked phone, not a debugger.
  The picture Android keeps of a hidden app shows the page as it was,
  unless the window is marked secure, which also forbids screenshots:
  Block 8's gate says which.
- With D32-A a server's page learns the names of the user's other servers;
  with D32-B nothing.
- A phone without a strong biometric,
  or on Android 8 to 10 without a fingerprint, gets no login by fingerprint,
  and keeps its session as a browser does.

What changes, by file, in the blocks still to run:

| File | Block | Change |
|---|---|---|
| `app/core/*.go`, `go.mod`, `Makefile` | 1 | the bound package, the `tool` directive, the targets |
| a merge request to `fdroiddata` | 5 | the recipe; here `app/README.md`'s section |
| the Play Console | 6 | the owner's; `app/README.md` records the steps |
| `app/android/**` | 7, 8, 11 | the service; the login; push |
| `internal/store/migrations/`, `internal/store/tokens.go`, `docs/store.md`, their tests | 8 | `tokens.parent_id`, a key's sessions revoked with it |
| `internal/server/login_api.go`, `users_api.go`, `audit.go`, their tests | 8 | `POST /api/v1/login/key`, a session's key in the list |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 8 | the route |
| `internal/server/ui/partials/login.html`, `login.js`, `app.js`, `connect.html`, `connect.js`, `src/header/Choice.tsx`, `i18n/*.json` | 8 | the form's switch and buttons, the list's marks and the choice, "Select server" |
| `docs/app.md`, `docs/ui.md`, `tools/ui-test/` | 8, 11, 12 | the contract's version 2, the sign-in in the app, the tests |
| `app/ios/**`, `.github/workflows/ci.yml` | 9 | the Xcode project, the macOS job |
| `internal/server/ui/connect.html`, `connect.js`, `header.js`, `src/header/Tls.tsx`, `partials/login.html` | 12 | the toasts, the names, the notice |
| `.github/dependabot.yml` | 12 | the entry for Gradle |
| `app/README.md`, `README.md`, `docs/roadmap.md`, `GUIDELINES/wip/HANDOVER.md` | 10, 12 | the documents; the roadmap and the handover by the coordinating session |
| `CHANGELOG.d/` | 1, 7 to 9, 11, 12 | a fragment per block |

## Implementation Plan

Each block ends with what the contract asks of a change:
`make ui` before anything compiles Go; locally the tests of what the block
touches (`go test` of the touched packages with `MASSIMO_ENV_FILE=/dev/null`,
the touched files of the UI suite in its four cases,
the Gradle tasks `:app:assembleFossDebug`, `:app:testFossDebugUnitTest` and
`:app:lintFossDebug`, `make actionlint` for a workflow),
`gofmt -l .` empty, `go vet` for linux, windows and darwin,
and `GUIDELINES/shared/tools/check` before every push;
the whole suite, the race run and the cross builds are CI's.
Nothing reaches a real Cube or a box: the fake Cube answers;
a phone is the owner's, used on the owner's word.
Every commit cites `AP MobileApp v1.1, Block <n>`;
a block's bullet is a fragment, `CHANGELOG.d/<Heading>/<branch>.md`;
a pull request per block.

### Block 1: Core

1. `app/core/core.go`:
   `Start`, `Stop`, `Status`, the `Events` interface,
   the recover in every entry point; `core_test.go`.
2. `go.mod`:
   `golang.org/x/mobile` as a `tool`; `Makefile`: `app-core-android`,
   `app-core-ios`, depending on `ui`,
   with the checks for the NDK and Xcode and the reproducibility flags;
   `.gitignore` for the outputs.
3. The fragment, Added:

   ```
   - `app/core`, the server as a library for the apps, bound by gomobile
     into an `.aar` and an XCFramework (`make app-core-android`,
     `make app-core-ios`; `docs/app.md`).
   ```

**Verification:**
`go test ./app/core/`;
`make app-core-android` on a machine with the NDK produces the `.aar`,
and `gomobile bind` lists the bound symbols without a type complaint.

### Block 2: Connect

Done on 2026-11-09, pull request 125: the page,
the bridge's contract in `docs/app.md`, the texts in six languages,
`test_connect.py`.
Its NOTE is `2026-11-09_04-30_NOTE_MobileApp_v1.0_block_2_connect.md`.

### Block 3: Shell

Done on 2026-11-16, pull request 341, and checked on the owner's phone on
2026-11-17 with one fix, pull request 344.
Its NOTEs are `2026-11-09_04-50_NOTE_MobileApp_v1.0_block_3_shell.md`,
`2026-11-16_17-38_NOTE_MobileApp_v1.0_block_3_on_main.md` and
`2026-11-17_14-17_NOTE_MobileApp_v1.0_phone_test.md`.

### Block 4: Release

Done on 2026-11-17, pull requests 349 and 352,
under a gate of its own (D12 to D21):
the job "Android app" in `ci.yml` and in `build.yml`,
the three jobs of `release.yml`, the nightly app, `app/version.json`,
`packaging/app-version.sh` and `packaging/app-sign.sh`.
The owner's three keys are in `signing` and `signing-rolling`,
their certificates under `app/android/signing/`.
Its NOTEs are `2026-11-17_18-27_NOTE_MobileApp_v1.0_block_4_refit.md`,
`2026-11-17_18-53_NOTE_MobileApp_v1.0_how_block_4_went.md` and
`2026-11-18_12-43_NOTE_MobileApp_v1.0_first_signed_nightly.md`:
the first nightly APK's signature and sums were checked by hand on 2026-11-18.
Still to see: a version tag's signing of the APK and the AAB,
at the owner's next `make release`.

### Block 5: F-Droid

1. The recipe written and tried with `fdroid build` in the F-Droid buildserver
   image where possible, else reviewed against the recipes of the analysis;
   `app/README.md`: the F-Droid section.
2. The owner: the Play Console account, developer verification,
   the package name registered with the key's SHA-256.
3. The Request For Packaging, or the merge request, to `fdroiddata`,
   once the first release with the APK is out.

**Verification:**
F-Droid's rebuild matches the release APK (their verification page says so),
which Block 4's two paths on one machine could not show;
the app appears on F-Droid under our signature.

### Block 6: Play

1. The owner: the app in the Play Console, Play App Signing with our key,
   the listing, the Data safety form, the foreground-service declaration
   once Block 7 is in, the internal testing track.
2. The tracks decided at the block's gate,
   from the proposal in Block 4's refit NOTE;
   the closed test with 12 testers for 14 days; then production.
3. `app/README.md`:
   the Play section, the steps recorded.

**Verification:**
the internal track installs on the owner's phone and updates from the release;
production approved.

### Block 7: Local

1. `CoreService`, the notification channel, the boot receiver, the `WifiLock`,
   the battery exemption request, the data directory;
   the connect page's card wired through `core.*`;
   the notice about the Cube's one client; the switch for other devices.
2. Tests: the service's lifecycle in a unit test with a fake core; by hand
   on a phone with the fake Cube on the LAN.
3. `docs/app.md`:
   the stand-alone mode and its limits.
4. The fragment, Added:

   ```
   - The Android app runs the server itself: the Cube on the LAN, the
     store on the phone, a foreground service that survives the screen
     going off; the phone then holds the Cube's one client (`docs/app.md`).
   ```

**Verification:**
the Gradle tasks; on a phone,
the embedded server serves the page from `127.0.0.1`,
holds the fake Cube over an hour with the screen off, and frees it when stopped.

### Block 8: Login

The block's gate first: D32,
and whether the window is marked secure while `return` covers the page.
Two pull requests, the server's first.

1. The store: migration 0025, `tokens.parent_id`, with `store.sql`,
   the way down and `docs/store.md` as the store's rules ask;
   `RevokeToken` of a key revokes its live sessions and their push subscriptions;
   a session made by a key revokes that key's earlier one; tests for each.
2. The server: `POST /api/v1/login/key` in `internal/server/login_api.go`
   as the Analysis describes, its audit kind in `audit.go`;
   a session's key in the token list (`users_api.go`) and under My profile;
   tests for a key, a session as the bearer, a revoked key, a disabled user,
   the lockout's count, a revoked key's session refused at its next request,
   and the earlier session ended by the next; `docs/api-v1.md`,
   `docs/api-v1-openapi.yaml` (`TestOpenAPICoversRoutes`).
3. The contract, `docs/app.md`, version 2: the calls of the Analysis,
   a server's `login` and `ask` in place of `biometric`.
4. The sign-in form, `partials/login.html` and `login.js`,
   where the bridge has `login`: `login.status()` asked when the form is drawn;
   the switch "Enable login by fingerprint";
   after a sign-in with it on, `login.keep`,
   with "Try again" and "Sign in without it" where it fails;
   the button "Login by fingerprint" with the fingerprint icon where credentials are kept;
   "Select server".
   The texts in six languages.
5. The Cube choice, `src/header/Choice.tsx`, where the page runs in the app:
   "Select server" after the Cubes and the search, as D32 is answered.
6. The connect page: the mark and the fingerprint button on a server with
   kept credentials; in its entry the user's name with Turn off,
   and the choice of when the fingerprint is asked (D30-A);
   "Forget all stored credentials" with a confirm; the switch
   "Ask for my fingerprint or PIN" removed.
7. The Android adapter: `LoginKey`, `LoginStore`,
   `Biometric` asking with a crypto object,
   the two requests over `Prober`'s connection; the three choices:
   the cover and its prompt for `return`,
   the session in memory alone for `return` and `start`, sealed for `logout`;
   the old switch's servers reset; strings in six languages.
8. Tests: the UI suite with the fake bridge for the form,
   the choice and the list; unit tests of `LoginStore` with a fake sealer,
   of the two requests against a fake server,
   and of what each of the three choices keeps.
9. `docs/ui.md` (the sign-in in the app, the Cube choice's entry),
   `app/README.md`.
10. The fragments, Added:

    ```
    - `POST /api/v1/login/key` makes a session from an API key; a session
      names the key that made it, and revoking the key ends it
      (`docs/api-v1.md`).
    - The Android app logs in by fingerprint: a key kept behind the phone's
      fingerprint makes a new session, asked for at each return to the
      app, at its start or after a logout, as the user chooses
      (`docs/app.md`).
    ```

**Verification:**
the touched Go tests and UI test files, the Gradle tasks; on the owner's phone:
enabling at the sign-in, a logout and the login by fingerprint,
each of the three choices, the row's button,
"Select server" from the Cube choice,
Turn off revoking the key at the server,
a key revoked under My profile ending its session at the next request,
a newly enrolled fingerprint ending it.

### Block 9: iOS

1. `app/ios`:
   the Xcode project and the adapter of the Analysis,
   the login by fingerprint through the Keychain;
   the XCFramework from `make app-core-ios`; the licence exception text.
2. `.github/workflows/ci.yml`:
   the macOS job.
3. The owner: the Apple Developer Program as an individual, TestFlight,
   the review.
4. `docs/app.md`.
5. The fragment, Added:

   ```
   - The iOS app (`app/ios`): the same page and bridge as Android's, with
     Bonjour discovery, the Keychain and Face ID or Touch ID; the server
     runs in it while the app is open (`docs/app.md`).
   ```

**Verification:**
the simulator build in CI; on an iPhone,
the checks of the phone test and of Block 8.

### Block 10: Documents

1. `docs/app.md` complete: the two modes, the bridge, the stores, the limits;
   `app/README.md`: the developer's setup; `README.md`:
   a section "The apps" with the store links.
2. The roadmap (items 16 and 17) and the handover: the coordinating session;
   this plan's lines pruned at its retirement by the session that retires it.

**Verification:**
`GUIDELINES/shared/tools/check` clean;
every bridge call in `docs/app.md` exists in both adapters.

### Block 11: Push

1. The UnifiedPush connector in both flavours,
   the embedded FCM distributor in `play`; the registration,
   the endpoint through the bridge to the server;
   the notification channels per kind, the texts in six languages;
   the embedded core's notifications into the same channels;
   the connect page's "Notify me on this phone".
2. Tests: the message rendering per kind; by hand with ntfy as the
   distributor against a `serve` with a subscription.
3. `docs/app.md`.
4. The fragment, Added:

   ```
   - The Android app is notified by the server through UnifiedPush, or
     through Google's service in the Play build (`docs/app.md`).
   ```

**Verification:**
the Gradle tasks; a window opened at the fake Cube notifies the phone after the delay,
with the app closed.

### Block 12: Polish

1. `connect.html`, `connect.js`: the toast's markup in the page and
   `showToast` for "added", "known", "removed" and "does not answer";
   a probe that finds the server takes its "does not answer" back;
   a server added by its address named by the host, and by discovery's
   name once its node is heard; a row shows an address only where it
   differs from the name.
2. The self-signed notice left out where the page runs in the app (`appBridge()`
   of `app.js`): `partials/login.html` with `login.js`, `header.js`,
   `src/header/Tls.tsx`; the warnings of a certificate near its end stay.
3. `app/README.md`:
   the phone test's rig, the VPN, `adb reverse`.
4. `.github/dependabot.yml`
   (D29-A): the `gradle` ecosystem for `app/android`,
   grouped and scheduled as the file's other entries are.
5. Tests: `test_connect.py` for the toasts and the names, the sign-in's
   test with the fake bridge for the notice.
6. The fragments, each under the heading COMMIT.md 5.2 gives it:

   ```
   - The apps' connect page says what happened in a toast and names a
     server once; in the app the sign-in no longer asks to compare a
     certificate the app has pinned (`docs/app.md`).
   - Dependabot proposes the updates of the Android app's Gradle
     dependencies.
   ```

**Verification:**
the touched UI test files in all four cases;
`make ui` and `make ui-check` for the part's change;
on the phone at the next test.

## Proposed commit message

The revision's registration, a commit of its own on `main`:

```
chore(aps): +MobileApp v1.1

The plan gains the login by fingerprint the owner designed at the phone test, as Block 8: a key of the user's own behind the phone's fingerprint, turned into a session by a new route. A session names its key, so that revoking the key ends it, and the user chooses when the fingerprint is asked. Push becomes Block 11, the test's findings Block 12, and Blocks 2, 3 and 4 are marked done.

Key changes:
- GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md: v1.1, rewritten for the plan as it stands; D22 to D31 decided at its gate, D32 open for Block 8's.

File-revision-v1: GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md::v1.1::2026-11-18::TL;DR, Discussion, Analysis, Implementation Plan, Proposed commit message, Change History::the login by fingerprint as Block 8; Push as Block 11; the phone test's findings as Block 12; Blocks 2 to 4 done
Assisted-by: claude-code:claude-fable-5-2
```

The blocks' commits still to come:

1. `feat(app): The server as app/core, bound by gomobile`
7. `feat(app): The Android app runs the server itself`
8. `feat(api): A session from an API key, POST /api/v1/login/key`, and
   `feat(app): The login by fingerprint`
9. `feat(app): The iOS app`
10. `docs(app): The apps' documents`
11. `feat(app): Push through UnifiedPush`
12. `fix(app): The connect page's toasts and names, the notice left out`,
    and `chore(ci): Dependabot reads the app's Gradle files`

Blocks 5 and 6 commit nothing but `app/README.md`'s sections.

## Change History

- v1.0 (2026-11-09): the first revision, from the TL;DR gate of items 16
  and 17 and the owner's additions; D1 to D3 open at its gate.
- v1.1 (2026-11-18): the login by fingerprint of the owner's design at the phone test,
  as Block 8, with a session naming its key and the user's choice of when
  the fingerprint is asked; Push moved to Block 11;
  the phone test's findings as Block 12, with Dependabot for Gradle;
  Blocks 2, 3 and 4 marked done, D1 to D21 restated as decided;
  two slips of v1.0 corrected;
  D22 to D31 decided at its gate, D32 open for Block 8's.

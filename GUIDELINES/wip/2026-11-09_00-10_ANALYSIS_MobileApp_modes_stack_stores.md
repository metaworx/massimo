# Analysis AndroidApp: an Android app, with iOS in view

Written 2026-11-09 for roadmap items 16 and 17 (docs/roadmap.md), under `PLAN`:
nothing in the project was changed.
Facts from outside the repository carry the date they were read;
facts from the repository were checked on this worktree (`main` at cc37df3).

The owner's words for item 16: run as a stand-alone service,
or connected to an existing Massimo; published on F-Droid and on Google Play.
Added during the investigation (2026-11-08/09):
a route that builds an iOS package from the same source;
run the full service locally or connect to an existing one;
remember the address, choosing the local or the remote address of a service by
the network the phone is on; several services to choose from;
biometric login; push notifications on certain events.

## 1. What the repository gives an app

- **The web UI is already a phone UI.**
  Plain HTML, CSS and JavaScript under `internal/server/ui/`,
  embedded with `go:embed`, 390 px first, light and dark,
  six languages through `t()` (809 keys in `ui/i18n/en.json`),
  about 8,200 lines of JavaScript in all, one vendored file (the OPAQUE client,
  WebAssembly inside, 435 KB, MIT).
  Every text, panel and setting the app needs for the heating exists there,
  and `docs/ui.md` describes it.
- **The API is complete for a client**:
  OPAQUE sign-in (the password never leaves the device) and API keys,
  sessions with labels (`User-Agent` by default), capabilities per role,
  the event stream with single-use tickets,
  `GET /healthz` with `node_id` (stable across restarts and updates) and the
  certificate's fingerprint, preferences that follow the user,
  `POST /api/v1/cubes/discover`.
  The settings marked "only on this device" live in the browser's storage.
- **No announcement of `serve` on the LAN yet.**
  `serve` answers the Cube's UDP discovery as a Cube (port 23272,
  `docs/proxy.md`), which an app cannot tell from a Cube (roadmap item 18),
  and the answer carries no API port. docs/design.md section 8 names mDNS
  `_massimo._tcp` for exactly this.
- **The core is portable.**
  `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build`
  and `GOOS=ios GOARCH=arm64 CGO_ENABLED=0 go build` compile `internal/protocol`,
  `cubelan`, `engine`, `proxy`, `store` (modernc SQLite, pure Go), `backup`,
  `view`, `cubeio` and `internal/server` without a change (checked 2026-11-08).
  The platform-specific files are in `internal/certs` (the DNS hook) and
  `internal/cubelan/arp.go`, both of which build for both targets.
- **The wiring lives in the command**:
  `cmd/massimo/serve.go` (644 lines) and `servetls.go` (169) assemble the store,
  the Cubes, the proxy listeners, TLS and the HTTP server.
  An embedded core needs that wiring as a package.
- **The Cube's constraints** (docs/portal-protocol.md):
  one LAN client at a time on TCP 62910;
  a radio write costs about 1.25 % duty cycle of the hour's 1 % budget (36 s of air time);
  the Cube has been seen to lose its configuration when clients connect
  and disconnect too often, hence one persistent connection.
  A phone that holds the Cube's slot is the server, and nothing else can
  connect directly while it does.
- **Events the server emits** (docs/api-v1.md, "events"): `connected`,
  `disconnected`, `hello`, `metadata`, `config`,
  `state` with `changes` (`window_open`, `battery_low`, `error`, `link_error`,
  `setpoint`, …), `pairing_*`, `paired`, `device_forgotten`, `cube_found`,
  `restore_*`.
  Everything a notification rule would trigger on is there.
- **Who holds the copyright**:
  `git shortlog` shows the owner (730 commits), the agent's commits (116,
  "Claude", no copyright claim) and dependabot (1).
  The dependencies are MIT and BSD (`go.mod`, `third_party/bytemare-ksf` MIT).
  An additional permission for the App Store (GPLv3 section 7) is therefore
  the owner's to grant.

## 2. The two modes (decision 1)

### Client of an existing `serve`

What it needs: a list of servers (name, `node_id`, a LAN address,
an optional remote address, the certificate fingerprint, the token),
finding a `serve` on the LAN, sign-in, the page, the stream.

- **Finding a `serve`.**
  mDNS/DNS-SD is what phones do: Android `NsdManager`, iOS `NWBrowser`/Bonjour,
  no permission needed on Android,
  a local-network prompt on iOS (`NSLocalNetworkUsageDescription`,
  the service type listed in `NSBonjourServices`).
  `serve` would announce `_massimo._tcp` with TXT `node=<node_id>`,
  `v=<version>`, `tls=0|1`, `fp=<fingerprint>`.
  Go's standard library has no mDNS;
  a small pure-Go responder is a dependency (candidates:
  `github.com/brutella/dnssd`, `github.com/grandcat/zeroconf`,
  `github.com/hashicorp/mdns`, all MIT or MPL),
  or about 300 lines of our own over `net`,
  since the announcement is a fixed answer to a fixed question.
  Fallbacks: the address typed by hand; a QR code shown by the web UI with URL,
  fingerprint and a one-time token (design.md section 8).
- **Local or remote address.**
  Each server entry holds up to two addresses.
  On opening, and on a network change (`ConnectivityManager` callback,
  `NWPathMonitor`), the app asks both for `/healthz` in parallel and takes the
  one whose `node_id` matches, the LAN one first.
  No Wi-Fi name is read: that needs the location permission on Android and
  tells nothing a probe does not.
  A LAN address that changed is found again by mDNS through the `node_id`.
- **Sign-in.**
  OPAQUE as the page does it; the session token stored by the shell (section 7),
  handed to the page at start.
  The session's label is set from the app's `User-Agent`
  (`Massimo/<version> (Android 16; Pixel)`), so the sessions list in
  My profile names the phone.
- **Cost.**
  No Go binding, no service, no battery rules: the shell,
  the connect page and the mDNS announcement.

### The stand-alone service

What it needs: the Go core inside the app, listening on `127.0.0.1:<fixed port>`
for the page (and optionally on the Wi-Fi address for other devices,
and the LAN proxy on 62910, both off by default), the store in the app's files
directory (`MASSIMO_DATA`), a settings screen in place of the settings file
(the store already takes every setting through the API: `PUT /api/v1/settings`,
`POST /api/v1/cubes`; the server starts in open mode, which on `127.0.0.1`
only the page can reach).

- **The binding.**
  `gomobile bind` (golang.org/x/mobile, BSD-3-Clause;
  newest v0.0.0-20261011034528-197770444867 of 2026-10-11, `go 1.26.0`,
  the module's README still calls it experimental;
  read 2026-11-08) builds an `.aar` for Android and an XCFramework for iOS from
  one Go package.
  Bound symbols are limited to ints, floats, strings, bools, `[]byte`, structs,
  interfaces, functions with at most two results, the second an `error`;
  panics across the boundary end the process.
  A package `app/core` with `Start(dataDir, listen string) error`, `Stop()`,
  `Status() string` (JSON) and an `Events` callback interface stays well inside that.
  It must live in the root module (it imports `internal/…`), so the root
  `go.mod` gains `golang.org/x/mobile/bind` (a `tool` directive keeps it).
  Android needs the NDK;
  this machine's WSL has neither SDK nor NDK (checked 2026-11-08),
  the CI runner can install both.
  The alternative without x/mobile,
  `-buildmode=c-shared` with a hand-written JNI layer, is what WireGuard does;
  more work, no experimental dependency. gomobile is the pragmatic first choice;
  the package boundary lets the plan switch later.
- **The one-client constraint.**
  The app's core takes the Cube's slot like any `serve`;
  the app says so in words ("this phone is the server now"),
  and the Cube's slot frees itself when the phone leaves and the TCP connection times out.
  The thermostats run their programmes meanwhile; only the gateway is gone.
  The LAN proxy on the phone is possible (port 62910 is unprivileged) but a
  phone as the house's Cube gateway is a poor fit; off by default.
- **Android's rules** (developer.android.com, read 2026-11-08):
  - A long-running connection needs a **foreground service**; since
    Android 14 every one declares a type.
    `connectedDevice` is for "interactions with external devices that require a Bluetooth,
    NFC, IR, USB, or network connection";
    it has **no time limit** (the 6 h/24 h limit is for `dataSync`
    and `mediaProcessing` only),
    may be **started from `BOOT_COMPLETED`** (not in Android 15's restricted
    list) and its prerequisite is met by declaring `CHANGE_WIFI_MULTICAST_STATE`,
    which discovery wants anyway.
    `specialUse` is the fallback, with a free-text justification Play reviews.
    Google Play requires every foreground service type to be declared in the
    Play Console's app content page.
  - **Doze** suspends network access when the phone lies unused with the screen off,
    in growing intervals, allowlisted apps excepted;
    a foreground service does not lift that by itself.
    The documented acceptable case for asking the exemption
    (`ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS`) is a "peripheral device
    companion app" whose "core function is maintaining a persistent connection
    with the peripheral device"; Play's policy allows the request only where
    "the core function of the app is adversely affected", which holds for the
    stand-alone mode and for nothing else in the app.
    So: the request is made only when the user turns the local service on,
    with the reason shown; Wi-Fi kept awake with a `WifiLock`.
  - **Battery**:
    one TCP connection, a poll a minute and the SSE stream to the page; small,
    but a phone is not an always-on box, and the app should say so.
- **iOS** has no equivalent:
  an app gets about 30 seconds after it leaves the foreground,
  longer only for audio, location, VoIP, accessories and the like.
  The stand-alone mode on iOS is **"while the app is open"**:
  the core starts with the app, the Cube connection drops when iOS suspends it,
  the greeting re-reads everything when it resumes.
  Fine for "a Cube and a phone, no box"; no 24/7, no push from it.

### Shared, and the first release

Both modes show the same page from the same `ui/` sources;
the difference is the origin the WebView loads
(the remote server or `http://127.0.0.1:<port>/`), and the local mode's
settings screen is the server's own Settings page.
The first release is the client alone: it serves every owner of a box today,
needs no binding and no battery exemption, and the review queues of F-Droid
and Play (weeks) start earlier.
The stand-alone mode follows in the second release.

## 3. The stack (decision 2)

The criterion the owner added: an iOS package from the same source.
Weighed with F-Droid buildability, the six languages,
the project's conventions (no framework, no build step, no `innerHTML`,
texts through `t()`), and what each costs.

| Route | Shared with iOS | New UI code | F-Droid | Conventions |
|---|---|---|---|---|
| **A. The web UI in a thin shell** (the page from the server or from the embedded core; the app's own screens as HTML/JS in `ui/`; a native adapter per platform: Kotlin ~2–3 k lines, Swift ~2 k lines; the core by gomobile) | the UI, the connect page, the core: all but the adapter | one page (`connect`) | Gradle + Go, as SFA and syncthing-android build; Kotlin-only apps reproduce best | fits; design.md section 8 names it ("the same web app in a … shell") |
| B. Compose Multiplatform (Kotlin) + Go core | the whole UI | the whole UI again: 8,200 lines, 809 strings, every panel of docs/ui.md | Gradle; JetBrains artefacts on Maven Central (Apache-2.0); Xcode for iOS | a second UI to keep equal to the web UI forever |
| C. Flutter + Go core | the whole UI | as B, in Dart | the Flutter SDK is an allowed prebuilt, but F-Droid names Flutter first among the path-embedding, non-reproducible toolchains (2026-11-08) | a third language |
| D. Capacitor shell | the shell's JS | little | Node.js is an allowed source; hundreds of npm packages to vet; Capacitor's model is a bundled app, loading a remote server's page is against its grain | npm in a project that has none |
| E. Kotlin + Jetpack Compose (Android), Swift later | nothing | the whole UI, twice | best | out by the owner's criterion |

Facts (read 2026-11-08):
Compose Multiplatform for iOS is **Stable** on JetBrains' platform table (page
dated 2025-09-10), stable since 1.8.0 in May 2025;
Kotlin Multiplatform iOS Stable.
Apple's guideline 4.2: "Your app should include features, content, and UI that
elevate it beyond a repackaged website"; 2.5.6 requires WebKit for web content.
Home Assistant's companion apps (a WebView around the server's frontend, with
native discovery, biometrics and notifications) are on both stores, the
precedent for route A; the native parts of A (discovery, biometric unlock,
the embedded server, push) are what lifts it over 4.2.

**Recommendation: A.**
The product's UI exists once, in `ui/`, and the server serves it;
an app that shows it is in step with any server version without an
API-compatibility matrix, because the page comes from the server it talks to.
The adapter is the only per-platform code,
and its surface is a dozen bridge calls: `servers()`, `save()`, `discover()`,
`probe()`, `trust(fp)`, `token(node)` behind the biometric, `startCore()`,
`stopCore()`, `subscribePush()`, `notify()`.
A Kotlin Multiplatform adapter (one Kotlin codebase for both) was weighed:
its parts are platform APIs anyway (WebView vs WKWebView, Keystore vs Keychain,
`NsdManager` vs Bonjour, a foreground service vs none), so it would be
expect/actual pairs for little gain; two small native adapters are simpler
and each is a first-class citizen of its store.

## 4. F-Droid (decision 3)

Inclusion policy (f-droid.org/en/docs/Inclusion_Policy, read 2026-11-08,
the page carries no revision date):

- "All applications in the repository must be Free, Libre and Open Source
  Software (FLOSS)"; AGPL-3.0-or-later qualifies.
- "The complete application building process requires a 100% FLOSS toolchain
  including Debian-packaged tools";
  prebuilt dependencies only from trusted sources: Debian main,
  the listed Maven repositories, the Android SDK, the Flutter SDK, Node.js.
  Google Play Services, Firebase and Crashlytics are "strictly forbidden";
  a build flavour without them is the accepted way (`foss` and `play`).
- Anti-features must be declared; the app has none (no tracking, no non-free
  network service: the server is AGPL).
- The app must have been "actively developed for at least three months", have a
  unique application id, and F-Droid "does not sign up for any API keys".
- The build server runs the recipe's `sudo:` (apt, downloads),
  `srclibs` (Go is built from source: `go@go1.25.0` with `make.bash`,
  as SFA's recipe does), `sdkmanager "ndk;<version>"`, then Gradle;
  nothing prebuilt may be committed (the `.aar` of the core is built in the recipe,
  as SFA builds `libbox.aar`).
  Recipes read: `io.nekohasekai.sfa` (sing-box, gomobile,
  reproducible with `Binaries` and `AllowedAPKSigningKeys`),
  `com.github.catfriend1.syncthingandroid` (Go built from source,
  NDK by `sdkmanager`), `org.jellyfin.mobile` (a client of a self-hosted server,
  `libre` flavour).
- Reproducible builds are **not required** (Submitting to F-Droid Quick Start Guide;
  the Reproducible Builds page calls them a goal, "can be done incrementally",
  read 2026-11-08).
  With them, F-Droid publishes **our** signature (`Binaries:` the release APK on GitHub,
  `AllowedAPKSigningKeys:` our fingerprint,
  "F-Droid will never attempt to publish APKs signed by F-Droid");
  without them F-Droid signs with its key.
  What breaks reproducibility, per that page: embedded paths (Go: `-trimpath`,
  `-buildvcs=false`, `-ldflags=-buildid=`; gomobile's own `go.mod` path leaks,
  reported by one project), the exact NDK version, JDK differences, R8;
  "apps completely written in Java and/or Kotlin tend to be unaffected".
- Updates: `UpdateCheckMode: Tags` with `UpdateCheckData` reading a
  `version.properties` in the repository (SFA), so the release must write
  `VERSION_CODE` and `VERSION_NAME` to a file; `AutoUpdateMode: Version`.
- Inclusion: a Request For Packaging issue or a merge request to `fdroiddata`
  with the recipe; the review takes weeks.

**Why reproducibility matters now:
Android developer verification** (developer.android.com/developer-verification
and its FAQ, entries dated up to 2026-09-30, read 2026-11-08):

- "Effective September 30, 2026, protections begin for users installing apps
  from participating stores in select regions" (Brazil, Indonesia, Singapore,
  Thailand); "Verification capability will be expanded globally for all apps on
  certified Android devices in 2027", Android 7 and up, through Play services.
  F-Droid is not among the first participating stores, and users keep an
  "advanced flow" (developer mode, a 24-hour wait, biometric confirmation)
  to install unverified apps; ADB installs are unaffected.
- A full-distribution account in the **Android Developer Console** costs $25 once,
  needs a government ID and, for an organization,
  a D-U-N-S number ("can take up to 28 days");
  a Play Console account covers both.
  Each package name is registered with the SHA-256 of its signing key;
  "the Android Developer Console lets you add and verify multiple signing keys
  for a single package".
- The open-source registration guide: register the package and our key;
  "if the open-source repository re-signs the app with their key, download the
  store-signed APK after you publish" and add that fingerprint too.
  F-Droid's open letter (2026-02-24) says F-Droid will not register on
  developers' behalf and calls the programme a threat to alternative stores;
  the Console API's OAuth delegation would let a store do it,
  which F-Droid refuses.

So: one signature everywhere (ours, through reproducible builds) means one key
registered and the F-Droid install path intact in 2027; F-Droid's own key would
have to be registered as a second key after F-Droid's first publish.

**Recommendation: reproducible from the first tag**, our key in F-Droid,
flavours `foss` and `play`,
pinned Go and NDK versions in one place the recipe reads, no committed binaries,
the release APK built in CI from the tag.

## 5. Google Play (decision 4)

Read 2026-11-08 on support.google.com and developer.android.com:

- **Account**:
  $25 once; an organization account needs a D-U-N-S number (the same one serves
  developer verification).
  Personal accounts created after 2023-11-13 must run a closed test with
  "a minimum of 12 testers who have been opted in continuously for at least
  14 days" before production; organization accounts are not subject to it.
- **Signing**:
  Play App Signing is the default for a new app ("automatically enrolled … with
  Google-generated keys");
  the developer may **provide a copy of their own key**,
  and "signing keys can only be changed before the app is released to open
  testing or production tracks".
  Our own key uploaded makes Play's APKs and F-Droid's carry the same signature;
  Google-generated keys would mean two keys to register and no switching between
  stores without a reinstall.
  Apps using Play App Signing are registered for developer verification automatically.
- **Target SDK**:
  since 2026-08-31 "new apps and app updates must target Android 16 (API
  level 36) or higher".
  An AAB is uploaded.
- **Data safety**:
  "'Collect' means transmitting data from your app off a user's device";
  data sent to a server the user chose and operates is not collected by the developer,
  so the form declares **no data collected**;
  the foreground service type is declared in Policy → App content.
- **Account deletion**
  (Play's and Apple's 5.1.1 rule):
  applies to apps that let users create accounts.
  The app creates no account of its own;
  the first owner of a server is the server's,
  and the server's users API deletes users.
  Worth a sentence in the listing.
- **Release tracks**:
  internal testing (instant, up to 100 testers), closed, open,
  production with staged rollout.
- **AGPL**:
  Play has no rule against it;
  the Developer Distribution Agreement imposes no use restrictions on the user
  of the kind the FSF objects to in Apple's terms; GPL apps are common there.
  The listing names the licence and the source.
- Play forbids requesting the battery-optimization exemption unless the app's
  core function needs it (section 2);
  the request is tied to the stand-alone mode only.

## 6. The repository (decision 5)

- **A: a directory in this repository**, `app/`:
  `app/core` (the Go package gomobile binds;
  in the root module because of `internal/`), `app/android` (the Gradle project;
  the `ui/` files copied into its assets by a Gradle task, never duplicated),
  `app/ios` later, `app/README.md`.
  One version: the app's `versionName` is `git describe`, its `versionCode`
  derived from the tag (e.g. `major*10^6 + minor*10^4 + patch*100 + pre`,
  a stable release taking 99), written to `app/version.properties` by
  `make release-alpha` so that F-Droid's update check can read it;
  the release workflow builds the signed `foss` APK (the `Binaries` URL for
  F-Droid) and the `play` AAB.
  CI: an "Android app" job (Ubuntu runner: JDK 17, Android SDK, NDK, Go,
  gomobile; `assembleFossDebug`, unit tests, lint; about 8 minutes), gated by
  changed paths like Screenshots; an iOS job on the macOS runner once
  `app/ios` exists (simulator build, no signing).
  Guidelines: `project.shipped` gains `app/**`; `project.languages` gains
  `kotlin` (and `swift`), for which the shared guidelines have no `lang/`
  directory yet (`lang/` has css, go, html, js, json, php, powershell, python,
  shell, ts, xml; checked 2026-11-08): either a `lang/kotlin/` baseline in the
  shared repository or `[languages] ignore = kotlin` until there is one.
- **B: a repository of its own** (`metaworx/massimo-app`): a clean F-Droid
  recipe (`Repo:` the app's), its own release cadence, no new languages here.
  Against: the UI sources and the Go core come from this repository, so every
  app build pins a server commit (submodule or module version) and the two
  release flows must be kept in step by hand; two CIs, two changelogs,
  an API-compatibility statement per release.
  With route A of decision 2 the app is the server's UI in a shell: it belongs
  where the UI is.

**Recommendation: A.**

## 7. Sign-in and security on a phone (decision 6)

- **Token storage.**
  The page keeps its session in `localStorage` (`ui/app.js`);
  in the app that would be the WebView's storage in the app's private directory,
  encrypted at rest by Android's file-based encryption, the same as a browser.
  Better, and what biometric login needs: the shell keeps the token per server
  in Keystore-backed storage (an AES-GCM key in the Android Keystore,
  the iOS Keychain), and hands it to the page through the bridge after the
  biometric prompt (`BiometricPrompt` with the device credential as fallback;
  `LocalAuthentication` on iOS), per server, off by default.
  The page's login code gets one hook:
  ask the bridge for a token before showing the form;
  store the new session through it.
- **OPAQUE stays in the page**:
  the vendored client is WebAssembly,
  which the Android System WebView (Chromium) and WKWebView run.
  The password never leaves the phone; the server never sees it.
- **TLS.**
  A Let's Encrypt certificate just works.
  A self-signed one (`serve -tls self-signed`) is pinned on first use:
  the shell shows the fingerprint, as the web UI's notice does, the user
  compares it with the server's log or `massimo tls show`, and a later change
  of fingerprint blocks with a warning (Android `onReceivedSslError` with the
  pinned SHA-256; iOS `URLAuthenticationChallenge`).
  Android 7+ ignores user-added CAs for apps, so pinning is the way, and it
  needs no CA at all.
  Plain HTTP: Android blocks cleartext by default since 9,
  the network security configuration allows it (the app allows it and warns for
  an address outside the private ranges); iOS ATS:
  `NSAllowsLocalNetworking` for LAN addresses and `.local`,
  `NSAllowsArbitraryLoadsInWebContent` for a remote plain-HTTP server,
  both accepted by Apple without justification.
- **Device-only settings** stay as they are:
  the WebView's storage is per origin,
  and each server is an origin of its own (the embedded core on a fixed port,
  so its origin stays stable).
- **The stream** uses tickets as in a browser; nothing to add.
- **Session labels**:
  the app's `User-Agent` names the phone,
  so a session is recognisable and revocable from My profile.

## 8. Push notifications (decision 8, the owner's addition)

- **F-Droid excludes Firebase Cloud Messaging.**
  The free alternative is **UnifiedPush** (unifiedpush.org, read 2026-11-08):
  the app embeds `android-connector` (Apache-2.0) and receives through a
  distributor app the user installs (ntfy, Sunup on Mozilla's push server,
  NextPush, …);
  for Google Play the `embedded_fcm_distributor` module makes FCM the fallback
  when no distributor is installed and Play services are present (`play` flavour only;
  it needs a Firebase project).
  UnifiedPush "has always been compatible with the RFC 8030 and RFC 8291 web
  push standards" (F-Droid's "5 years of UnifiedPush", 2026-01-08), so the
  server sends a **Web Push** message: an HTTP POST to the endpoint URL with
  the payload encrypted per RFC 8291 (ECDH P-256, HKDF, AES-128-GCM, all in
  Go's standard library; a pure-Go library exists, e.g.
  `github.com/SherClockHolmes/webpush-go`, MIT) and VAPID (RFC 8292) for
  browser push services.
- **The same module serves the installable web app** in a browser (Chrome,
  Firefox, Safari 16.4+):
  design.md section 8's "an installable web app stays the free route with Web Push".
- **iOS** has APNs only, which needs an Apple key;
  an AGPL server cannot carry every operator's key,
  so iOS push needs a relay run by metaworx (the server posts to the relay,
  the relay to APNs; design.md section 8 says the same),
  or iOS goes without push at first.
- **The rules belong in the server**
  (Go): triggers from the events of section 1 (Cube offline longer than N minutes,
  window open longer than N minutes, battery low, link error, device stale,
  temperature under a threshold, pairing results),
  subscriptions per user and device (`POST /api/v1/me/push` with endpoint and keys),
  a delivery log,
  and a local delivery hook so that the **stand-alone mode fires the same
  notifications** on the phone itself through the bridge.
  Settings and the UI for the rules; OpenAPI; docs.
  This is design.md section 5's "rules" and "webhooks" in a first form,
  a plan-sized block that serves browsers too.
- Alternatives: the app holding the event stream open in a foreground service
  (no server work; Doze and battery make it unreliable; Play frowns on it);
  FCM only (excluded from F-Droid).

**Recommendation**:
Web Push in the server as the one transport, UnifiedPush on Android (`foss`),
the embedded FCM distributor as the `play` fallback,
iOS push through a relay decided with item 17.

## 9. The blocks a `PLAN+` would write (decision 7)

Stack-independent blocks can start before decision 2 is settled.
Sizes are working days of one session, without review queues.

| # | Block | Size | Depends on |
|---|---|---|---|
| 1 | **mDNS announcement** of `serve` (`_massimo._tcp`, TXT `node`, `v`, `tls`, `fp`), `massimo discover` listing servers, README and API docs | 1–2 d | nothing |
| 2 | **The server as a library**: `internal/serve` out of `cmd/massimo/serve.go`; `app/core` with `Start`, `Stop`, `Status`, `Events`; `gomobile bind` targets in the Makefile; `go.mod` gains x/mobile; a test that the core starts against the fake Cube | 3–5 d | nothing (NDK/SDK in CI or on a machine) |
| 3 | **Notifications in the server**: rules, subscriptions, Web Push (RFC 8030/8291/8292), the local hook, settings UI, OpenAPI, docs; browsers get Web Push | 1–2 w | nothing; a plan of its own if preferred |
| 4 | **The connect page** in `ui/` (server list, LAN and remote address, add by address or QR or discovery, fingerprint confirmation, biometric switch, notification choices), the bridge contract, a fake bridge so the pytest suite covers it | 3–5 d | decision 2 |
| 5 | **The Android shell, client mode**: Kotlin, WebView, the bridge, `NsdManager`, probing and network change, TLS pinning, cleartext configuration, Keystore storage, `BiometricPrompt`, icon and six-language strings, `foss` and `play` flavours | 2–3 w | 4 |
| 6 | **The Android shell, stand-alone mode**: the `connectedDevice` foreground service, boot start, the battery exemption request, `WifiLock`, the data directory, the "this phone is the server" notice | 1 w | 2, 5 |
| 7 | **The Android shell, push**: `android-connector`, the `embedded_fcm_distributor` in `play`, notification channels, tap-to-open on the server | 1 w | 3, 5 |
| 8 | **CI and release**: the Android job, reproducibility check (build twice, compare), signing in `release.yml`, `app/version.properties` written by the release, the APK and AAB attached to the release, `project.shipped` and languages | 2–3 d | 5 |
| 9 | **F-Droid**: the recipe, the RFP or merge request, `Binaries` and `AllowedAPKSigningKeys`; the owner: Android Developer Console or Play Console verification, package registration | 2 d + weeks | 8 |
| 10 | **Google Play**: the owner's organization account, the listing, Data safety, the foreground-service declaration, internal → closed → production | 1–2 d + owner | 8 |
| 11 | **The iOS shell** (item 17): Swift, WKWebView, the same bridge, Bonjour, Keychain and `LocalAuthentication`, ATS, pinning; the stand-alone mode while open; the App Store additional permission; APNs relay decision; a Mac and the $99/year programme | 2–3 w + owner | 4, decision 2 |
| 12 | **Documents**: `docs/app.md`, README, the roadmap and the handover | spread | each block |

First release: Blocks 1, 4, 5, 8, 9, 10 (the client).
Second: 2, 6 (stand-alone).
Third: 3, 7 (push).
Then 11.

## 10. Open points for the owner beyond the decisions

1. The plan's name: `AndroidApp`, or `MobileApp` since the blocks carry iOS.
2. Whether Block 3 (notifications) is a plan of its own (it serves browsers
   and the server regardless of the app).
3. The application id (`ch.metaworx.massimo`?) and the app's name on the stores;
   "MAX!" and eQ-3 stay out of the name (design.md section 6).
4. The Android Developer Console or Play Console account: who holds it,
   the D-U-N-S number of the organization, and the signing key's custody
   (a lost key cannot be re-registered).
5. A development machine with the Android SDK and NDK (Android Studio on the
   Windows side of jaguar, or the SDK in WSL, about 3 GB); a Mac for item 17.

## Sources

Repository: docs/design.md, docs/roadmap.md, docs/api-v1.md,
docs/api-v1-openapi.yaml, docs/ui.md, docs/proxy.md, docs/portal-protocol.md,
docs/background.md, docs/handover-session.md, GUIDELINES/project/CI.md,
`internal/*`, `cmd/massimo/serve.go`, `go.mod`.

Outside, with the date read:

- F-Droid Inclusion Policy,
  https://f-droid.org/en/docs/Inclusion_Policy/ (2026-11-08)
- F-Droid Reproducible Builds,
  https://f-droid.org/docs/Reproducible_Builds/ (2026-11-08)
- F-Droid Submitting Quick Start Guide,
  https://f-droid.org/docs/Inclusion_How-To/ (2026-11-08)
- fdroiddata recipes `io.nekohasekai.sfa`,
  `com.github.catfriend1.syncthingandroid`, `org.jellyfin.mobile` (2026-11-08)
- F-Droid, "An Open Letter Opposing Android Developer Verification",
  https://f-droid.org/2026/02/24/open-letter-opposing-developer-verification.html
  (2026-11-08)
- F-Droid, "5 years of UnifiedPush",
  https://f-droid.org/fr/2026/01/08/unifiedpush-5-years.html (2026-11-08)
- UnifiedPush implementations,
  https://unifiedpush.org/developers/implementations/ (2026-11-08)
- Android developer verification: overview, FAQ,
  Android Developer Console guide, open-source app registration,
  https://developer.android.com/developer-verification (2026-11-08)
- Foreground service types,
  https://developer.android.com/develop/background-work/services/fg-service-types
  (2026-11-08)
- Doze and App Standby,
  https://developer.android.com/training/monitoring-device-state/doze-standby (2026-11-08)
- Android 15 behaviour changes,
  https://developer.android.com/about/versions/15/behavior-changes-15 (2026-11-08)
- Play target API level,
  https://support.google.com/googleplay/android-developer/answer/11926878 (2026-11-08)
- Play App Signing,
  https://support.google.com/googleplay/android-developer/answer/9842756 (2026-11-08)
- Play Data safety,
  https://support.google.com/googleplay/android-developer/answer/10787469 (2026-11-08)
- Play closed-testing requirement for personal accounts,
  https://support.google.com/googleplay/android-developer/answer/14151465 (2026-11-08)
- Apple App Store Review Guidelines (4.2, 2.5.6, 5.1.1),
  https://developer.apple.com/app-store/review/guidelines/ (2026-11-08)
- Apple Developer Program,
  https://developer.apple.com/programs/whats-included/ (2026-11-08)
- golang.org/x/mobile, https://pkg.go.dev/golang.org/x/mobile
  and proxy.golang.org `@latest` (2026-11-08); gobind type rules,
  https://pkg.go.dev/golang.org/x/mobile/cmd/gobind (2026-11-08)
- Kotlin Multiplatform stability of supported platforms,
  https://kotlinlang.org/docs/multiplatform/supported-platforms.html (2026-11-08)

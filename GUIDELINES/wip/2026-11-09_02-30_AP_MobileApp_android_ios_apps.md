# AP MobileApp v1.0: the Android and iOS apps

## TL;DR

Roadmap items 16 and 17: an app for Android, on F-Droid and Google Play,
and one for iOS from the same source.
The app is the existing web UI in a thin native shell, connected to a
`serve` on the LAN or elsewhere, or over a server embedded in the app;
the shell is a dozen bridge calls behind one HTML page of our own.

- a phone → the page in a browser → an app with a server list, discovery,
  pinning, biometrics, push, and the server itself when wanted
- the UI → written once in `ui/` → shown by the shell, from the server
  or from the embedded core; iOS shares all but the adapter
- the Go core → `internal/serve` → `app/core` bound by gomobile:
  an `.aar` and an XCFramework from one package
- the stores → nothing → F-Droid with our own signature (reproducible),
  Google Play with our own signing key, the App Store with an additional
  permission to the AGPL
- releases → the server's tag → the app's APK and AAB on the same release

### Blocks

1. Core: `app/core` bound by gomobile, the Makefile targets, a test
2. Connect: the page `ui/connect.html` and the bridge contract
3. Shell: the Android app in client mode, Kotlin, `foss` and `play`
4. Release: the Android CI job, reproducibility, signing, the version file,
   the APK and AAB on the release
5. F-Droid: the recipe, the submission, our signature;
   the owner's developer verification
6. Play: the listing, Data safety, the signing key, the closed test
7. Local: the stand-alone mode on Android, the foreground service
8. Push: UnifiedPush and the embedded FCM distributor,
   biometrics behind the token
9. iOS: the Swift shell, the same bridge, the App Store (item 17)
10. Documents: `docs/app.md`, `app/README.md`, the README, the roadmap

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
`2026-11-09_00-10_ANALYSIS_MobileApp_modes_stack_stores.md`,
a copy of the owner's analysis with its timestamp kept,
as AGENTS.md section 4 asks; its own title still says AndroidApp,
the plan's name before the owner chose MobileApp.
It holds the facts outside the repository with the date each was read:
F-Droid's inclusion policy and reproducible builds,
Android developer verification, Google Play's requirements,
Apple's guidelines and programme, gomobile's state, the platforms' stability,
UnifiedPush.
This plan restates what it builds on and cites the analysis for the rest.

**Decided before this plan, at the TL;DR gate of 2026-11-09**
(the owner's "plan decisions as suggested", then D9 to D11):

- **Modes**:
  both; the client first, the stand-alone service second.
- **Stack**:
  the web UI in a thin shell; a native adapter per platform, Kotlin on Android,
  Swift on iOS; the Go core bound by gomobile.
  Not Compose Multiplatform, Flutter or Capacitor.
- **F-Droid**:
  reproducible builds from the first tag, our signature published;
  flavours `foss` and `play`; no committed binaries.
- **Google Play**:
  a personal account (none exists yet, D9-A),
  Play App Signing with our own key uploaded, an AAB, target API 36,
  Data safety "no data collected";
  a new personal account runs the closed test of 12 testers for 14 days before production.
- **Repository**:
  `app/` in this repository; one version with the server's tags;
  the release workflow builds the app.
- **Security**:
  the token per server in Keystore-backed storage behind a biometric prompt;
  OPAQUE stays in the page;
  a self-signed certificate pinned on first use with its fingerprint shown;
  plain HTTP allowed, with a warning outside the private ranges;
  device-only settings unchanged.
- **Push**:
  Web Push from the server (AP ServerAnnouncements) as the one transport;
  UnifiedPush in `foss`, the embedded FCM distributor as the `play` fallback;
  iOS through a relay, decided when Block 9 starts.
- **Development**:
  the toolchain in WSL (the SDK, the NDK, Go, gomobile, Gradle),
  the same Linux build CI and F-Droid make;
  Android Studio under WSLg or IntelliJ IDEA Ultimate through JetBrains Gateway;
  the phone over wireless debugging; the emulator optional (D10-A).
- **The server side** is AP ServerAnnouncements: `internal/serve`,
  the mDNS announcement, the notification rules and Web Push.
  Block 1 here waits for its Block 1; Block 8 for its Blocks 3 and 4.

**Open at the plan gate:**

- **D1.
  The application id.**
  A (recommended): `ch.metaworx.massimo`,
  as the module path `go.metaworx.ch/massimo` is;
  neither store verifies the domain behind an id,
  only App Links and associated domains do,
  and the owner controls `metaworx.ch`;
  it can never change after the first publish on Play.
  B: another namespace of the owner's.
- **D2.
  The lowest Android version.**
  A (recommended): `minSdk 26`, Android 8.0 of 2017:
  every API the shell uses is there,
  and the WebView updates through Play on every such device.
  B: `minSdk 24`, Android 7.0: a few more old phones, every API still there,
  more to test.
- **D3.
  The app's release.**
  A (recommended): the server's tag releases the app too: `make
  release-alpha` writes `app/version.json` (the version and a
  `versionCode` derived from it) into the release commit, and
  `release.yml` builds the signed APK and the AAB and attaches them to
  the same GitHub release, which F-Droid reads; one version everywhere,
  the app's and the embedded server's the same.
  B: tags of the app's own (`app/v0.1.0`) with a workflow of their own:
  the app released without a server release, at the price of a second
  release flow and a version matrix.

**Names chosen here**, routine and not decisions of the owner's:
the directories `app/core`, `app/android`, `app/ios`;
the Makefile targets `app-core-android` and `app-core-ios`;
the page `connect.html` with `connect.js`; the bridge object `window.massimo`;
the flavours `foss` and `play`; the service `CoreService`;
the branches `claude/mobile-core`, `claude/mobile-connect`,
`claude/mobile-android` and so on, one per block.

**Not in this plan:**

- pairing by QR code (a code shown by the web UI with the URL, the
  fingerprint and a one-time token): a later revision, once the client
  works by address and discovery;
- an APNs relay for iOS push, and iOS push at all in the first iOS release:
  decided when Block 9 starts;
- the server's own work: AP ServerAnnouncements;
- widgets, a wear app, Android Auto;
- e-mail or webhook delivery of notifications.

**Who runs it:**
sessions of their own, one per block or pair of blocks,
after the plan's registration on `main` by the coordinating session or the owner.
Blocks 2 and 3 can start at once;
Block 1 once AP ServerAnnouncements Block 1 has merged; 4 after 3;
5 and 6 after 4; 7 after 1 and 3;
8 after 3 and AP ServerAnnouncements Blocks 3 and 4; 9 after 2 and 3, on a Mac;
10 spread over the others.
The first release is Blocks 1 to 6 (the client, both stores), the second 7,
the third 8, then 9.

## Analysis

### What the app is

One page of our own, `connect.html`, bundled in the app from the same
`ui/` sources the server embeds, and the server's page itself, loaded
from the server the user chose or from the embedded core.
The adapter is native, small, and does what a page cannot:

| Bridge call (`window.massimo`) | Android | iOS |
|---|---|---|
| `servers()`, `save(server)`, `remove(id)` | a JSON file in the app's files | the same |
| `discover(ms)` | `NsdManager` browse and resolve of `_massimo._tcp` | `NWBrowser` |
| `probe(urls)` | `GET /healthz` of each in parallel, with the pinned certificate | `URLSession` |
| `trust(id, fp)` | the fingerprint pinned for the server | the same |
| `token(id)`, `setToken(id, t)` | Keystore-wrapped AES-GCM storage; `BiometricPrompt` first where the server asks for it | Keychain; `LocalAuthentication` |
| `open(id)` | the WebView loads the server's page | `WKWebView` |
| `core.start()`, `core.stop()`, `core.status()` | the foreground service with the bound core | the core while the app is open |
| `push.subscribe()`, `push.unsubscribe()` | UnifiedPush registration; the endpoint back to the page | later, through a relay |
| events (`massimo` custom events) | discovery results, probe results, core status, a notification | the same |

The page keeps a server as `{id, name, node, lan, remote, fp, biometric, push}`:
`node` is the server's `node_id` from `/healthz`,
so that a LAN address that changed is found again by discovery,
and `probe` takes the address whose `node_id` matches, the LAN one first.
No Wi-Fi name is read.

**The server's page in the shell.**
The session token the page keeps in `localStorage` (`readStored` and `store` in
`ui/app.js`) comes from the bridge when `window.massimo` exists:
the page asks `token(id)` before it shows the sign-in form and hands a new
session to `setToken`; the biometric prompt is the adapter's.
OPAQUE runs in the page as in a browser: the vendored client is WebAssembly,
which the Android System WebView and WKWebView run.
The event stream, the tickets, the preferences and the "only on this
device" settings work unchanged, the WebView's storage being per origin
and each server an origin of its own.
The `User-Agent` names the app and the phone, so that the session's label
under My profile does.

**TLS and plain HTTP.**
A certificate from Let's Encrypt verifies by itself.
A self-signed one is pinned on first use:
the page shows the fingerprint as the web UI's notice does,
the user compares it with the server's log or `massimo tls show`,
`trust` stores it,
and a changed fingerprint blocks with a warning
(`WebViewClient.onReceivedSslError` compares the SHA-256;
the probe's `TrustManager` likewise).
Plain HTTP is allowed by the network security configuration, and the page
warns for an address outside the private ranges; iOS allows local
networking and web content over HTTP through its ATS keys.

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

### The connect page (Block 2)

`ui/connect.html` and `connect.js`, in the server's embed as every other file,
served at `/ui/connect.html` (a browser without the bridge sees a line saying
the page is the app's), loaded by the shell from its bundled copy of `ui/`:

- the server list, each with its name, its probe result (LAN, remote,
  offline) and a tap that opens it; a long press for its settings;
- add a server: the discovered servers as they arrive,
  or an address typed (host, host:port, or a URL); the probe;
  the fingerprint sheet for a self-signed certificate;
- a server's settings: name, LAN and remote address, "Ask for my
  fingerprint or PIN" (biometric), "Notify me on this phone" (push,
  Block 8), remove;
- "Run the server on this phone": a card with start,
  stop and the status (Block 7); off until the core is built in;
- texts through `t()` in all six `ui/i18n/<lang>.json`, FR, IT,
  NL and PL machine-translated and marked; `components.js`'s `.field` rows;
  phone width first, verified at 390 and 1100 px;
- the bridge contract written down in `docs/app.md`, with the events.

The pytest suite covers it with a fake bridge injected by Playwright's `add_init_script`,
as `conftest.py` injects the token today: no test-only code in the page.

### The Android shell (Blocks 3, 7 and 8)

`app/android`:
a Gradle project (Kotlin, the Android Gradle Plugin,
`compileSdk` and `targetSdk` 36, `minSdk` per D2), one module `app`,
flavours `foss` and `play`,
the `ui/` files copied into its assets by a Gradle task from `internal/server/ui/`,
never a second copy in git.

- **Client mode (Block 3)**:
  `MainActivity` with the WebView and `WebViewAssetLoader` for the bundled page;
  `Bridge` with `@JavascriptInterface`; `ServerStore`;
  `Discovery` over `NsdManager`;
  `Prober` over `HttpURLConnection` with the pinned trust manager;
  `Secrets` over an AES-GCM key in the Android Keystore;
  `Biometric` over `androidx.biometric`; the network security configuration;
  the icon (adaptive, from the web UI's);
  strings in six languages for the adapter's own few texts;
  `dependenciesInfo.includeInApk = false` and no build timestamps,
  for reproducibility.
  Dependencies: AndroidX (Apache-2.0) only.
- **Stand-alone mode (Block 7)**:
  `CoreService`, a foreground service of type `connectedDevice` (for
  "interactions with external devices that require a … network connection";
  no six-hour limit, may start from `BOOT_COMPLETED`;
  its prerequisite met by declaring `CHANGE_WIFI_MULTICAST_STATE`,
  which discovery wants anyway) with a persistent notification;
  the core's data directory under the app's files; a `WifiLock`;
  the battery-optimization exemption asked for only when the user turns the server on,
  with the reason shown (the documented acceptable case:
  a companion app whose core function is a persistent connection to a device);
  an optional start at boot; "this phone is the server now:
  the Cube accepts one client" said in words;
  the LAN listener and the proxy off, a switch to let other devices connect,
  which requires a first owner as the server does on the LAN.
  The analysis, section 2, has Android's rules with their sources.
- **Push (Block 8)**:
  `org.unifiedpush.android:connector` in both flavours,
  the `embedded_fcm_distributor` in `play` only (a Firebase project and its
  `google-services.json` in the `play` source set, kept out of `foss`);
  the endpoint and keys posted to the server through the bridge (`POST /api/v1/me/push`);
  a received message rendered from the app's strings per kind into a notification channel,
  a tap opening the server's page;
  the embedded core's `OnNotification` into the same channel.
  Biometrics gate the token, not the app: the page asks `token(id)` and
  the prompt comes up for a server that asks for it.

### Release and the stores (Blocks 4, 5, 6)

**CI** (`.github/workflows/ci.yml`): a job "Android app" on the Ubuntu runner:
JDK 17, the Android SDK and the pinned NDK, Go, gomobile at the pinned version;
`make app-core-android`,
then `./gradlew :app:assembleFossDebug :app:testFossDebugUnitTest :app:lintFossDebug`;
gated by changed paths (`app/**`, `internal/server/ui/**`, `internal/serve/**`,
`go.mod`, `.github/workflows/ci.yml`) as the Screenshots job is,
since it takes several minutes;
the reproducibility check builds the unsigned release APK twice in two
directories and fails on a difference.
`GUIDELINES/project/CI.md` says what runs and why.

**Release** (`release.yml`): the `foss` release APK signed by `apksigner`
with the app signing key from the repository's secrets, and the `play`
AAB signed with the upload key, both attached to the GitHub release; the
APK is the `Binaries` URL F-Droid compares its rebuild with.
Under D3-A the version is the tag's and `app/version.json` carries it with its
`versionCode` (`major*10^6 + minor*10^4 + patch*100 + pre`,
a stable release taking 99 in place of `pre`), written by the release step;
Gradle reads it, and so does F-Droid's update check.

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
The owner's steps: a Google Play Console account also registers the app
for Android developer verification (from 2027 every certified Android
phone installs only apps of a verified developer, F-Droid's included;
5 once; a personal account per D9-A); the package name registered with
the key's fingerprint; a lost key cannot be re-registered.

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
The AGPL is no issue on Play; the listing names the licence and the source.

### The iOS shell (Block 9)

`app/ios`:
an Xcode project with Swift Package Manager, no CocoaPods;
`WKWebView` with a `WKScriptMessageHandler` bridge of the same contract;
the bundled `ui/` served by a `WKURLSchemeHandler`; `NWBrowser` for discovery;
`URLSession` probes with pinning in the authentication challenge;
the Keychain and `LocalAuthentication`;
the ATS keys `NSAllowsLocalNetworking` and `NSAllowsArbitraryLoadsInWebContent`;
the local-network usage description and `NSBonjourServices`;
the XCFramework from `make app-core-ios`;
the stand-alone mode while the app is open (iOS suspends an app about 30 s after
it leaves the foreground, so no 24/7 and no push from it);
no push in the first release unless a relay is decided.
The App Store: the owner enrolls as an individual (9 a year;
a sole proprietor must), the seller name the owner's legal name;
an additional permission to the AGPL for App Store distribution,
which the owner can grant as the copyright holder (the analysis, section 1),
in `app/ios/LICENSE-EXCEPTION.md` and the listing;
guideline 4.2 is met by what the shell adds over a web page (discovery,
biometrics, the embedded server, pinning).
CI: a job on the macOS runner, `xcodebuild` for the simulator, no signing.

### Risks

- gomobile is experimental by its own README; the core's surface is small,
  and `-buildmode=c-shared` with a hand-written JNI layer is the fallback behind
  the same `app/core` API.
- Reproducibility: gomobile has leaked the checkout path into the library's build info;
  the CI check of Block 4 catches it before F-Droid does.
- The 14-day closed test delays production on Play; F-Droid and the
  GitHub release do not wait for it.
- A WebView on an old phone may lack WebAssembly or `PushManager`-level features;
  the page already copes with a missing feature by saying so.
- The shared guidelines have no `lang/kotlin` or `lang/swift` baseline;
  `detect-languages.sh --check` will ask for them once `app/` exists:
  Block 3 adds `[languages] ignore = kotlin` to `GUIDELINES/config.ini`
  until a baseline exists, and says so in the commit.

What changes, by file:

| File | Block | Change |
|---|---|---|
| `app/core/*.go`, `go.mod`, `Makefile` | 1 | the bound package, the `tool` directive, the targets |
| `internal/server/ui/connect.html`, `connect.js`, `app.js`, `login.js`, `i18n/*.json`, `style.css`, `ui.go` | 2 | the page, the bridge hook, the texts |
| `tools/ui-test/test_connect.py`, `docs/app.md`, `docs/ui.md` | 2 | the tests with a fake bridge, the contract |
| `app/android/**` | 3, 7, 8 | the Gradle project, the adapter, the service, push |
| `GUIDELINES/config.ini` | 3 | `app/**` in `shipped`, `ignore = kotlin` |
| `.github/workflows/ci.yml`, `release.yml`, `GUIDELINES/project/CI.md` | 4 | the Android job, the artefacts |
| `Makefile`, `app/version.json` | 4 | the version file written by the release |
| a merge request to `fdroiddata` | 5 | the recipe; nothing in this repository but `app/README.md`'s section |
| the Play Console | 6 | the owner's; `app/README.md` records the steps |
| `app/ios/**`, `.github/workflows/ci.yml` | 9 | the Xcode project, the macOS job |
| `docs/app.md`, `app/README.md`, `README.md`, `docs/roadmap.md`, `docs/handover-session.md` | 10 | the documents; the roadmap and the handover by the coordinating session |
| `CHANGELOG.md` | 1 to 4, 7 to 9 | a bullet per block under Unreleased |

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`, `make race`,
and `GUIDELINES/shared/tools/check --commit-range origin/main..HEAD` from the
main checkout; Blocks 2 and 3 add `make ui-test` at 1100 and 390 px; Blocks 3,
7 and 8 add the Gradle build, unit tests and lint; Block 9 the Xcode build.
Nothing reaches a real Cube or a box: the fake Cube answers; a phone on
the LAN is the owner's, used on the owner's word.
One functional commit per block, citing `AP MobileApp v1.0, Block <n>`,
with its CHANGELOG bullet in the wrapped style of its neighbours; a pull
request per block.

### Block 1: Core

1. `app/core/core.go`:
   `Start`, `Stop`, `Status`, the `Events` interface,
   the recover in every entry point; `core_test.go`.
2. `go.mod`:
   `golang.org/x/mobile` as a `tool`; `Makefile`: `app-core-android`,
   `app-core-ios`,
   with the checks for the NDK and Xcode and the reproducibility flags;
   `.gitignore` for the outputs.
3. `CHANGELOG.md`, Unreleased, Added:

   ```
   - `app/core`, the server as a library for the apps, bound by gomobile
     into an `.aar` and an XCFramework (`make app-core-android`,
     `make app-core-ios`; `docs/app.md`).
   ```

**Verification:**
the test gate; `make app-core-android` on a machine with the NDK produces the `.aar`,
and `gomobile bind` lists the bound symbols without a type complaint.

### Block 2: Connect

1. `ui/connect.html`, `connect.js`: the page as the Analysis describes;
   `app.js`, `login.js`: the bridge hook for the token; `ui.go`:
   the file in the embed; the texts in six languages.
2. `docs/app.md`:
   the bridge contract and the events; `docs/ui.md`: the page.
3. `tools/ui-test/test_connect.py`:
   the list, add by address, the fingerprint sheet, the biometric switch,
   with a fake `window.massimo` from `add_init_script`, at 1100 and 390 px;
   axe without violations.
4. `CHANGELOG.md`, Unreleased, Added:

   ```
   - `ui/connect.html`, the apps' page: the servers, discovery, the
     fingerprint of a self-signed certificate, the biometric and
     notification switches; a browser sees it as the app's (`docs/app.md`).
   ```

**Verification:**
the test gate with `make ui-test`;
the page in a browser with the fake bridge behaves as the tests say.

### Block 3: Shell

1. `app/android`:
   the Gradle project, the flavours, the asset copy task,
   the classes of the Analysis, the network security configuration, the icon,
   the strings; `GUIDELINES/config.ini`: `shipped = app/**`, `ignore = kotlin`.
2. Unit tests for `ServerStore`, `Prober`'s choice of address and the
   fingerprint comparison; an instrumented smoke test of the WebView
   loading the connect page, run by hand on a phone.
3. `app/README.md`:
   the toolchain in WSL (D10-A),
   how to build and install on a phone over wireless debugging.
4. `CHANGELOG.md`, Unreleased, Added:

   ```
   - The Android app (`app/android`): the web UI in a shell with a server
     list, mDNS discovery, a pinned self-signed certificate, the token
     behind a fingerprint or PIN (`docs/app.md`).
   ```

**Verification:**
the test gate; `./gradlew assembleFossDebug test lint` clean;
on a phone on the LAN, a `serve` of AP ServerAnnouncements Block 2 is found,
a self-signed one pinned after its fingerprint was compared,
the page signs in by OPAQUE, the session is labelled with the phone.

### Block 4: Release

1. `.github/workflows/ci.yml`:
   the job "Android app" and its path gate, the reproducibility check;
   `GUIDELINES/project/CI.md`.
2. `.github/workflows/release.yml`:
   the APK and the AAB, signed from the secrets, attached to the release;
   the secrets named in `GUIDELINES/project/CI.md` section 5;
   the owner creates the keystore and the upload key and stores them.
3. `Makefile`:
   `release-alpha` writes `app/version.json` into the release commit (D3-A),
   through `changelog.sh cut`'s manifest where it serves,
   else a step of its own before the cut; Gradle reads the file.
4. `CHANGELOG.md`, Unreleased, Added:

   ```
   - A release carries the Android app, the `foss` APK and the `play`
     AAB, built and signed by the release workflow from the same tag
     (`GUIDELINES/project/CI.md`).
   ```

**Verification:**
the test gate; the CI job green on the pull request;
two builds in two directories give the same unsigned APK;
a dry run of `make release-alpha DRY=1` shows the version file.

### Block 5: F-Droid

1. The recipe written and tried with `fdroid build` in the F-Droid buildserver
   image where possible, else reviewed against the recipes of the Analysis;
   `app/README.md`: the F-Droid section.
2. The owner: the Play Console account (D9-A), developer verification,
   the package name registered with the key's SHA-256.
3. The Request For Packaging, or the merge request, to `fdroiddata`,
   once the first release with the APK is out.

**Verification:**
F-Droid's rebuild matches the release APK (their verification page says so);
the app appears on F-Droid under our signature.

### Block 6: Play

1. The owner: the app in the Play Console, Play App Signing with our key,
   the listing, the Data safety form, the foreground-service declaration,
   the internal testing track.
2. The closed test with 12 testers for 14 days; then production.
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
   the stand-alone mode and its limits; `CHANGELOG.md`, Unreleased, Added:

   ```
   - The Android app runs the server itself: the Cube on the LAN, the
     store on the phone, a foreground service that survives the screen
     going off; the phone then holds the Cube's one client (`docs/app.md`).
   ```

**Verification:**
the test gate; on a phone, the embedded server serves the page from `127.0.0.1`,
holds the fake Cube over an hour with the screen off, and frees it when stopped.

### Block 8: Push

1. The UnifiedPush connector in both flavours,
   the embedded FCM distributor in `play`; the registration,
   the endpoint through the bridge to the server;
   the notification channels per kind, the texts in six languages;
   the embedded core's notifications into the same channels;
   the biometric prompt before `token(id)` where set.
2. Tests: the message rendering per kind; by hand with ntfy as the
   distributor against a `serve` of AP ServerAnnouncements Block 4.
3. `docs/app.md`; `CHANGELOG.md`, Unreleased, Added:

   ```
   - The Android app is notified by the server through UnifiedPush, or
     through Google's service in the Play build, and asks for a
     fingerprint or PIN before it unlocks a server's token (`docs/app.md`).
   ```

**Verification:**
the test gate; a window opened at the fake Cube notifies the phone after the delay,
with the app closed.

### Block 9: iOS

1. `app/ios`:
   the Xcode project and the adapter of the Analysis;
   the XCFramework from `make app-core-ios`; the licence exception text.
2. `.github/workflows/ci.yml`:
   the macOS job.
3. The owner: the Apple Developer Program as an individual, TestFlight,
   the review.
4. `docs/app.md`; `CHANGELOG.md`, Unreleased, Added:

   ```
   - The iOS app (`app/ios`): the same page and bridge as Android's, with
     Bonjour discovery, the Keychain and Face ID or Touch ID; the server
     runs in it while the app is open (`docs/app.md`).
   ```

**Verification:**
the test gate; the simulator build in CI; on an iPhone,
the same checks as Block 3's.

### Block 10: Documents

1. `docs/app.md` complete: the two modes, the bridge, the stores, the limits;
   `app/README.md`: the developer's setup; `README.md`:
   a section "The apps" with the store links.
2. The roadmap (items 16 and 17) and the handover: the coordinating session,
   at the end of each block's session.

**Verification:**
`GUIDELINES/shared/tools/check` clean;
every bridge call in `docs/app.md` exists in both adapters.

## Proposed commit message

The plan's registration,
a commit of its own on `main` before the first project write:

```
chore(aps): +MobileApp v1.0, +ANALYSIS MobileApp

The Android and iOS apps of roadmap items 16 and 17: the existing web UI in a thin native shell, connected to a serve or over the server embedded through gomobile, with discovery, pinning, biometrics and push; published on F-Droid with our own signature, on Google Play and on the App Store.

Key changes:
- GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md: the plan, ten blocks in three releases, the decisions of the TL;DR gate restated.
- GUIDELINES/wip/2026-11-09_00-10_ANALYSIS_MobileApp_modes_stack_stores.md: the analysis the plan rests on, with the dated facts from outside the repository.

File-revision-v1: GUIDELINES/wip/2026-11-09_02-30_AP_MobileApp_android_ios_apps.md::v1.0::2026-11-09::All::the first revision, by PLAN+
File-revision-v1: GUIDELINES/wip/2026-11-09_00-10_ANALYSIS_MobileApp_modes_stack_stores.md::v1::2026-11-09::All::the analysis of 2026-11-08 and 2026-11-09, copied from messages/
Assisted-by: claude-code:claude-fable-5-1
```

The blocks' commits:

1. `feat(app): The server as app/core, bound by gomobile`
2. `feat(ui): The apps' connect page and the bridge contract`
3. `feat(app): The Android app in client mode`
4. `ci(app): The Android job, the signed APK and AAB on the release`
7. `feat(app): The Android app runs the server itself`
8. `feat(app): Push through UnifiedPush, biometrics before the token`
9. `feat(app): The iOS app`
10. `docs(app): The apps' documents`

Blocks 5 and 6 commit nothing but `app/README.md`'s sections.

## Change History

- v1.0 (2026-11-09): the first revision, from the TL;DR gate of items 16
  and 17 and the owner's additions; D1 to D3 open at its gate.

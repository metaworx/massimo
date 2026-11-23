# The apps

The Android app, and later the iOS one,
are the web UI in a thin shell (AP MobileApp, roadmap items 16 and 17).
`docs/app.md` is the contract between the page and the shell, `window.massimo`;
this file is the developer's: how the Android app is built,
tested and put on a phone.

| Directory | What |
|---|---|
| `android/` | the Gradle project of the Android app, the module `app` |
| `version.json` | the newest release's version and versionCode, which `make release` writes (below, "Versions") |
| `android/signing/` | the public certificates of the app's signing keys (below, "The signing keys") |

## The Android app

Kotlin, the Android Gradle Plugin, `compileSdk` and `targetSdk` 36,
`minSdk` 26 (Android 8.0).
Three flavours: `foss`, the F-Droid build, and `play`, the Google Play build,
the same app until Block 8 gives `play` Google's push distributor,
both with the application id `ch.metaworx.massimo`;
and `nightly`, `foss` as the nightly build publishes it on GitHub,
`ch.metaworx.massimo.nightly`, "Massimo nightly",
an app of its own that installs beside the release
(AP MobileApp, D18-A).
Its icon is the mark's one-colour layer, white on a dark blue
(`src/nightly/res/`);
a themed icon looks the same for both, and the label tells them apart.
Dependencies are AndroidX (Apache-2.0) alone, from Google's Maven
repository and Maven Central; the JVM tests add JUnit and org.json.

The web UI's files are copied into the app's assets under `ui/` at build
time from `internal/server/ui/` (the task `bundleUi<Variant>`), never a
second copy in git:
everything but `index.html`, `partials/`, the parts (`src/` and the
`parts.js` that `make ui` bundles from it) and `vendor/`,
which are the server's page and served by the server.
The connect page is plain HTML and JavaScript and needs no `make ui`.
`-Pmassimo.ui=<dir>` takes them from another checkout,
for a page that is not merged yet.

What the shell is, by class (`android/app/src/main/kotlin/ch/metaworx/massimo/`):

| Class | What it does |
|---|---|
| `MainActivity` | an `AppCompatActivity` with the one WebView: the connect page from `https://appassets.androidplatform.net/ui/connect.html` (`WebViewAssetLoader`), then the server's page; at a start, the server marked Open at start in place of the list, where it answers (`Addresses`); the pinned certificate in `onReceivedSslError`; links to another origin open in the browser; for a server whose login is asked at each return, the cover while the app is hidden and no picture in Recents (`setRecentsScreenshotEnabled`, before Android 13 `FLAG_SECURE`); the app's appearance as AppCompat's night mode, set before the theme is applied |
| `Bridge`, `res/raw/bridge.js` | `window.massimo` (docs/app.md, contract version 4): the script, injected at the document's start by `addDocumentStartJavaScript`, makes each call a message over `addWebMessageListener`'s channel; `Bridge` answers it after checking its origin (`Origins`): the connect page gets the server list, the sessions the app holds and the app's settings, a server's page its own session, its login by fingerprint and `connect`, any other page nothing |
| `ServerStore` | the servers, in a JSON file of the app's own, each with when its fingerprint is asked (`ask`), whether it opens at start (`start`, one at most, the first added until the user chooses), when its fingerprint was trusted (`trusted_at`) and the keys of the phone's left valid there (`unrevoked`, user and id, no secret); version 1's switch is reset once, its session forgotten |
| `TokenStore`, `KeystoreSealer` | a session per server, sealed with AES-256-GCM under a key of the Android Keystore, the server's id bound into the seal |
| `SessionKeeper` | where a server's session is kept: in memory alone for a kept login asked at each return or start, else in `TokenStore` |
| `Sessions` | the sessions the app holds, as the connect page sees them: checked at the address a probe found (`GET /api/v1/me`) and forgotten at a `401`, the check revoking the keys left there for the session's user and giving a login kept without its key's id the session's key; signed out of one server or of all (`POST /api/v1/logout`), and Turn off's revoke, with the session or with the key; a failure in the phone's log (`Log.w`, tag `Massimo`), without any secret |
| `Revoker` | Turn off, Remove with a kept login and Forget all stored credentials: each kept login's key revoked with the session held, else with the key after one prompt for the run (one more for each login kept before the app key), one server after the other; a key not revoked left in `KeysLeft` where its id is known, then the session signed out before the login is forgotten |
| `KeysLeft` | the keys of the phone's left valid, kept in `ServerStore`, revoked with the next session of their user there (`DELETE /api/v1/users/{name}/tokens/{id}`) where the user's list has the id as an API key and it is not the token in use: a `404` drops one too, a `403` or a failure keeps it |
| `Addresses` | the address a server opens at without the page, the LAN one first: the server opened at start, and Sign out of all |
| `AppSettings` | the app's own settings, the appearance, in a JSON file of the app's own; `res/values/themes.xml` and `res/values-night/themes.xml` set `android:isLightTheme` for day and night, which the WebView's `prefers-color-scheme` follows |
| `LoginStore`, `LoginKey`, `AppKey` | the login by fingerprint: an API key of the user's per server, sealed in software under the app key, 256 random bits, which one Keystore key that requires the prompt for every use seals (from Android 11 a strong biometric or the screen lock, before it the strong biometric alone), the server's id, node, pinned fingerprint and the user bound into the seal, and the key's id in clear beside it, for Turn off with a session; a prompt opens the app key for one step, Forget all's included, and the step drops it; a login kept before the app key under a Keystore key of its own |
| `ServerApi` | the adapter's own requests at an address of the server's, over the probe's trust: the user (`GET /api/v1/me`), the key (`POST /api/v1/users/{name}/tokens`), the session from it (`POST /api/v1/login/key`), the revoke, the logout; the kept key never over plain HTTP outside the private ranges |
| `Biometric` | the prompt, with the key's cipher for a login, without one for the cover; one the stopped activity cannot show is answered at once as dismissed |
| `Prober` | `GET <url>healthz` of each address in parallel, within four seconds; a certificate the system does not trust for the host counts only where its fingerprint is the one given, and is otherwise reported with its fingerprint, nothing read; the certificate's subject, issuer, names and validity with it (`CertDetails`) |
| `Discovery` | `NsdManager` browsing `_massimo._tcp`, each server resolved into a `found` event |

The icon is Massimo's mark as an adaptive icon,
with a monochrome layer for themed icons:
`android/app/src/main/res-brand/`, written by `make ui-brand`
with the web UI's brand files (`tools/brand/README.md`), never by hand.

The network security configuration allows plain HTTP (the page warns outside the
private ranges) and trusts the system's certificate authorities alone.
Nothing of the app is backed up or moved to a new phone:
the tokens could not be opened there anyway.
The WebView's metrics and Safe Browsing are off.
The User-Agent begins with `Massimo/<version> (Android <release>; <model>)`,
so that the session the server labels with it names the phone.

A server's page cannot save a file yet: its downloads are `blob:` URLs,
which a WebView hands to no one; the app says so and points to the browser.
Restoring a backup from a file works, through the system's file picker.

### Versions

`version.json` holds the newest release's `version` and `versionCode`;
`make release` writes the next one's just before the cut,
which commits it in the release commit as the changelog's manifest
(`GUIDELINES/config.ini`), so a tag's source builds as the tag,
and F-Droid's update check reads both at the tag.
`packaging/app-version.sh` is the formula's one place (D21-A):
the versionCode is `MMmmPPbbbb`,
the major (at most 20, as Play's largest code allows),
the minor and the patch (at most 99 each),
and the commits since the version's first tag (at most 9999).
The labels alpha, beta and rc have no digits of their own,
so every build of a version counts on from its first tag:

| Build | versionName | versionCode |
|---|---|---|
| `v0.16.0-alpha.1` | `0.16.0-alpha.1` | 16000000 |
| the nightly build 227 commits on | `0.16.0-alpha.1-227-g<commit>` | 16000227 |
| `v0.16.0` cut two commits later | `0.16.0` | 16000229 |
| `v0.17.0-alpha.1` | `0.17.0-alpha.1` | 17000000 |

The nightly build gives its version to Gradle as `-Pmassimo.version` and
`-Pmassimo.versionCode`; a debug build's versionName ends in `-dev`.
`packaging/app-version.sh check` says whether the file is its tag's,
and the CI job Android app runs it.

### The signing keys

The app is signed in the release workflow alone
(`GUIDELINES/project/CI.md`, "Releases"), with three keys the owner makes and keeps:

| Key | Signs | Secrets, environment | Certificate |
|---|---|---|---|
| the app signing key, alias `massimo` | the released APK, which every phone and F-Droid's `AllowedAPKSigningKeys` know it by; it cannot be replaced | `MASSIMO_APP_KEYSTORE`, `MASSIMO_APP_KEYSTORE_PASSWORD`, `signing` | `android/signing/app-cert.pem` |
| Google Play's upload key, alias `upload` | the AAB for Play; Play can reset it | `MASSIMO_APP_UPLOAD_KEYSTORE`, `MASSIMO_APP_UPLOAD_KEYSTORE_PASSWORD`, `signing` | `android/signing/upload-cert.pem` |
| the rolling key, alias `rolling` | the nightly APK | `MASSIMO_APP_ROLLING_KEYSTORE`, `MASSIMO_APP_ROLLING_KEYSTORE_PASSWORD`, `signing-rolling` | `android/signing/rolling-cert.pem` |

Each is a PKCS#12 keystore with an RSA key of 4096 bits,
in base64 as its secret, with its password beside it.
`packaging/app-sign.sh` signs with one and checks the signer against its
certificate before a file is handed on;
without a key's secrets or its certificate the release warns and carries no app.
The keys never enter the repository, a log or a chat;
the certificates are public and committed.
A lost app signing key cannot be registered again for Android's
developer verification, nor for Play App Signing with our own key,
so the keystore and its password are kept in two places of the owner's.
Made with `keytool` (here the app signing key; the others alike,
with their alias and name):

```bash
keytool -genkeypair -alias massimo -keyalg RSA -keysize 4096 -sigalg SHA256withRSA -validity 20000 -dname "CN=Massimo, O=The Massimo Authors" -storetype PKCS12 -keystore massimo-app.p12
```

```bash
keytool -exportcert -rfc -alias massimo -keystore massimo-app.p12 -file app-cert.pem
```

```bash
base64 -w0 massimo-app.p12 | gh secret set MASSIMO_APP_KEYSTORE --env signing
```

```bash
gh secret set MASSIMO_APP_KEYSTORE_PASSWORD --env signing
```

The last asks for the password without showing it.

A downloaded APK is checked against the committed certificate with the
build-tools' `apksigner`; the digest it prints must be the certificate's:

```bash
apksigner verify --print-certs massimo-android.apk
```

```bash
openssl x509 -in app/android/signing/app-cert.pem -outform DER | sha256sum
```

### The toolchain in WSL

All of it lives in WSL (D10-A of the plan); nothing needs Windows.

- JDK 17 (`openjdk-17-jdk-headless`), the one CI and F-Droid build with.
  Where another JDK is the default, run Gradle with
  `JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64`.
- The Android command-line tools in `~/Android/Sdk`, with `ANDROID_HOME`
  pointing there and its `cmdline-tools/latest/bin` and `platform-tools`
  on the `PATH` (in `~/.profile`); then
  `sdkmanager "platforms;android-36" "build-tools;36.0.0" platform-tools`.
  The NDK comes with Block 1's `make app-core-android`, not before.
- Gradle comes through the wrapper, `android/gradlew`, pinned by its
  SHA-256 in `gradle/wrapper/gradle-wrapper.properties`;
  the first build downloads it and the dependencies into `~/.gradle`,
  about 1.5 GB.

Android Studio under WSLg, or IntelliJ IDEA through JetBrains Gateway,
opens `app/android` as it is.

### Building and testing

```bash
cd app/android
./gradlew :app:assembleFossDebug :app:testFossDebugUnitTest :app:lintFossDebug
```

The debug APK is `app/build/outputs/apk/foss/debug/app-foss-debug.apk`.
Lint runs with every warning an error.
The unit tests run on the JVM (`app/src/test/`): the server store,
the token store with a JVM key, the probe against a local HTTP server,
the trust manager with a self-signed certificate, fingerprints,
origins and who may call what, the discovery event;
the login store with JVM keys, the login's requests against a local HTTP
server, and where each choice of when the fingerprint is asked keeps a session;
the sessions' check, sign-out and Turn off's revoke against a local HTTP
server, a failure logged without any secret,
Turn off and Forget all on every path and the keys they leave,
revoked at the next session (`FakeServer`), the certificate's details
(`src/test/resources/san.pem`, made for the test, its key thrown away),
the address the server opened at start is taken at, and the app's settings.
The prompt and the Keystore's keys bound to it are a phone's alone:
they are checked on one (the plan's Verification of Block 8).
The instrumented test (`app/src/androidTest/`) loads the connect page on a
phone and checks that the bridge answers it and refuses it a token:

```bash
./gradlew :app:connectedFossDebugAndroidTest
```

### On a phone, over wireless debugging

On the phone: Settings → System → Developer options → Wireless debugging,
then "Pair device with pairing code".
WSL reaches the phone, which is all `adb` needs; the phone cannot reach WSL.

```bash
adb pair 192.168.1.30:37000
```

```bash
adb connect 192.168.1.30:41000
```

```bash
./gradlew :app:installFossDebug
```

The two ports are the ones the phone shows, the first with the pairing code.
A server for the phone to try is a `serve` against the fake Cube in WSL,
as `tools/ui-test/run.sh` starts one (never with the checkout's `.env`),
reached through `adb reverse` as `http://127.0.0.1:8099/` on the phone:

```bash
adb reverse tcp:8099 tcp:8099
```

The phone test of 2026-11-17 ran such a `serve` with `-tls self-signed`
and `-listen 127.0.0.1:9443`, in an empty directory of its own,
so that adding it shows the fingerprint sheet;
on the phone it is then `https://127.0.0.1:9443/`:

```bash
adb reverse tcp:9443 tcp:9443
```

`adb reverse` can stall on one port:
on that test the phone's connections to port 8443 never reached WSL,
while 9443 to the same server worked.
A port that stalls is left for another, on both sides.
Type the whole URL, with `https://`, which the page probes over one connection;
`host:port` probes two, by HTTPS and by HTTP (`docs/app.md`).

A debug build lets Chrome's DevTools inspect the WebView.
Its socket is `webview_devtools_remote_<pid>`, forwarded to a port in WSL:

```bash
adb forward tcp:9222 localabstract:webview_devtools_remote_$(adb shell pidof ch.metaworx.massimo)
```

Chrome then finds the page at `localhost:9222`
(`chrome://inspect`, "Configure…"), and `http://localhost:9222/json` lists it.

A VPN on the phone that does not let LAN connections through
(its setting is often called "Allow LAN connections")
routes the LAN into its tunnel:
the phone reaches no server on the LAN, and adding one times out,
while the LAN still reaches the phone, `adb` among it.
Allow LAN connections in the VPN, or turn it off, for a server on the LAN.

Discovery needs a `serve` on the phone's LAN that announces itself
(AP ServerAnnouncements); a `serve` in WSL is not on it.

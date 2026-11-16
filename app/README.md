# The apps

The Android app, and later the iOS one,
are the web UI in a thin shell (AP MobileApp, roadmap items 16 and 17).
`docs/app.md` is the contract between the page and the shell, `window.massimo`;
this file is the developer's: how the Android app is built,
tested and put on a phone.

| Directory | What |
|---|---|
| `android/` | the Gradle project of the Android app, the module `app` |
| `version.json` | the version the release writes (AP MobileApp Block 4); a build without it is `0.0.0-dev` |

## The Android app

Kotlin, the Android Gradle Plugin, `compileSdk` and `targetSdk` 36,
`minSdk` 26 (Android 8.0).
Two flavours: `foss`, the F-Droid build, and `play`, the Google Play build,
the same app until Block 8 gives `play` Google's push distributor.
The application id is `ch.metaworx.massimo` in both.
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
| `MainActivity` | the one WebView: the connect page from `https://appassets.androidplatform.net/ui/connect.html` (`WebViewAssetLoader`), then the server's page; the pinned certificate in `onReceivedSslError`; links to another origin open in the browser |
| `Bridge`, `res/raw/bridge.js` | `window.massimo` (docs/app.md): the script, injected at the document's start by `addDocumentStartJavaScript`, makes each call a message over `addWebMessageListener`'s channel; `Bridge` answers it after checking its origin (`Origins`): the connect page gets the server list, a server's page its own token, any other page nothing |
| `ServerStore` | the servers, in a JSON file of the app's own |
| `TokenStore`, `KeystoreSealer` | a token per server, sealed with AES-256-GCM under a key of the Android Keystore, the server's id bound into the seal |
| `Biometric` | the prompt before a token is handed over, for a server whose "Ask for my fingerprint or PIN" is on, and before that switch is turned off |
| `Prober` | `GET <url>healthz` of each address in parallel, within four seconds; a certificate the system does not trust for the host counts only where its fingerprint is the one given, and is otherwise reported with its fingerprint, nothing read |
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
origins and who may call what, the discovery event.
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

Discovery needs a `serve` on the phone's LAN that announces itself
(AP ServerAnnouncements); a `serve` in WSL is not on it.

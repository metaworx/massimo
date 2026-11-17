# NOTE MobileApp v1.0: block 3 shell

How Block 3, the Android shell in client mode,
went where it differs from the plan,
with the owner's answers of 2026-11-09 to its gate (D4-A, D5-A, D6-A,
by `EXEC+`).

## Decisions

- **D4-A, the bridge is messages, not `@JavascriptInterface`.**
  Block 2's contract, `docs/app.md`, was written first and took the same shape:
  `window.massimo` is a script injected by
  `WebViewCompat.addDocumentStartJavaScript` (`res/raw/bridge.js`) over the
  channel of `WebViewCompat.addWebMessageListener`, each call a Promise.
  The shell injects both into the connect page's origin and the opened server's
  alone, and checks every message's origin and main frame (`Origins`):
  the connect page gets the server list, a server's page its own token,
  any other page nothing.
  The plan's Analysis still says `@JavascriptInterface`.
- **D5-A, the Gradle wrapper is committed**:
  `gradlew`, the 48 KB `gradle-wrapper.jar` (its SHA-256 the one Gradle
  publishes for 9.8.2) and the distribution pinned by its SHA-256;
  the one binary file in `app/`.
  No `gradlew.bat`: the toolchain is WSL's.
- **D6-A, `languages` gains `xml`**, and `[languages]` has no `ignore = kotlin`:
  `detect-languages.sh` detects no Kotlin at all (it would only warn of the
  ignore) and detects the Android resources as XML.
  The plan's Risks expected the opposite.

## Where the work went differently

- **The choice of address is the page's.**
  The contract has the page probe both addresses and take the one whose `node`
  matches; `probe` answers per URL.
  So the unit tests cover the probe's answers and the trust decision, not
  a choice the native side does not make.
- **`androidx.core` 1.18.0**, not the newest 1.19.1,
  which wants `compileSdk 37`; `compileSdk` and `targetSdk` stay 36 as decided.
- **The icon** is the web UI's thermometer, its symbol `icon-is`, white on
  the UI's accent: the web UI has no icon of its own to take.
- **The theme is AppCompat's DayNight**, since `androidx.biometric`'s
  dialog before Android 9 needs it; `androidx.appcompat` was in the
  dependencies through `androidx.biometric` already.
- **The biometric prompt guards the token in the app's logic**; the
  Keystore key that seals the tokens is not bound to an authentication.
  A key so bound gains little where the page's own session is in the WebView,
  and before Android 11 the library cannot take the screen lock with a crypto
  object.
  Turning "Ask for my fingerprint or PIN" off takes the prompt too, so that
  whoever holds the unlocked phone cannot switch it off and open the server.
- **Files**:
  a server's page restores a backup through the system's file picker
  (`onShowFileChooser`); its downloads are `blob:` URLs,
  which a WebView hands to no one,
  so the app says it cannot save files yet and points to the browser.
  Saving them takes a call of the bridge's,
  a later version of the contract (Block 7 or 8, or a revision).
- **The User-Agent begins with `Massimo/<version> (Android <release>;
  <model>)`**, the WebView's own after it: the server labels a session with
  the User-Agent's first 64 characters.
- **`-Pmassimo.ui=<dir>`** takes the UI from another checkout; the app was
  built and linted against Block 2's worktree with it, since
  `connect.html` is not on `main` yet.
- **Not run: the instrumented test and the plan's check on a phone.**
  Both need the owner's phone and Block 2's page on `main`; the discovery
  part also needs AP ServerAnnouncements Block 2.
  `app/README.md` has the recipe, with `adb reverse` to a `serve` against
  the fake Cube in WSL.
- **`AGENTS.md` and `GUIDELINES/README.md`**:
  `changelog.sh update-contract` rendered `app/**` into
  `GUIDELINES/project/_CONTRACT.md`; `sync-docs.sh` cannot run in a worktree,
  whose `GUIDELINES/shared` is empty,
  so the same line was put into its two renderings by hand.

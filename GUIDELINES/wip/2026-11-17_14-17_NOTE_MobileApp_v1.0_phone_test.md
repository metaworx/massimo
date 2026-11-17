# NOTE MobileApp v1.0: phone test

Block 3's check on a phone, run with the owner on 2026-11-17,
after the block had merged:
the plan's Verification of Block 3 and the instrumented test, what they found,
and the owner's design for the login by fingerprint that the test prompted.

## How it ran

- **The phone**:
  a Samsung Galaxy A56 (`SM-A566B`), Android 17 (API 37),
  Android System WebView 157, over wireless debugging from WSL's own adb.
- **The app**:
  the `foss` debug build of `main` of that day.
- **A server in WSL**:
  `serve -tls self-signed` against the fake Cube, a fresh store without users,
  reached by the phone as `https://127.0.0.1:9443/` through `adb reverse`.
- **A server on the LAN**:
  the owner's Windows service, `.210:8099`, plain HTTP,
  announcing `_massimo._tcp`.

## Results

| Check | Result |
|---|---|
| The instrumented test, `ConnectPageTest` | 2 of 2 passed, once the asset path below was fixed |
| Discovery | the Windows service found by its announcement, with its node, version and `tls` |
| Adding a self-signed server | the probe reports the fingerprint, the sheet shows it, Trust pins it |
| The first owner | registered by OPAQUE from the phone; the store keeps a record, no hash |
| Sign out, sign in | both by OPAQUE |
| The session's label | `Massimo/0.0.0-dev (Android 17; SM-A566B) Mozilla/5.0 (Linux; And…`, the phone named first |
| The token across a restart | the server opens signed in, the token from the app's Keystore-sealed store |
| "Ask for my fingerprint or PIN" | the prompt before the token; turning it off asks too |
| A changed certificate | the list says so; the sheet shows the old and the new fingerprint; Trust re-pins it, the session kept |
| A plain-HTTP server on the LAN | added and opened, once the phone's VPN let the LAN through (below) |

## Findings in the app and the page

1. **The connect page did not load: `net::ERR_INVALID_RESPONSE`.**
   `WebViewAssetLoader` hands a path handler the path after the prefix it is
   registered at,
   so `/ui/connect.html` reached `AssetsPathHandler` as `connect.html`,
   an asset the app does not have, whose empty answer the WebView calls invalid.
   Fixed in the test: the handler is given `ui/` and the path, and a file the
   app does not have is answered 404.
   The JVM tests could not see it; only the instrumented test could.
2. **The web UI's self-signed notice shows in the app** ("compare the SHA-256
   fingerprint in your browser's certificate details"),
   on the sign-in form and in the header,
   although the app has pinned the fingerprint the user compared,
   and the app has no certificate details to look at.
3. **The connect page's messages stay**:
   "added", "removed", and "does not answer at its addresses" even after a later
   probe finds the server.
   The owner: they should be toasts (`showToast` of `components.js`).
4. **No way back to the server list on the page**:
   only Android's Back, through the WebView's history,
   which may take more than one.
5. **A server added by its address is named by it**,
   so the list shows `host:port` twice; discovery's name is the server's.

## Findings outside the app

- **The phone's VPN**
  (Proton VPN, LAN connections not allowed) routed the LAN into its tunnel:
  the phone reached no server on the LAN, and adding one timed out,
  while the LAN reached the phone.
  `app/README.md` should say so.
- **`adb reverse` stalled** for phone port 8443:
  the probe's connection never reached WSL,
  while port 9443 to the same server worked;
  a URL with `https://` probes one connection where `host:port` probes two.
- **Windows Firewall**:
  the installer's rule for `massimo.exe` covers the Domain and Private profiles;
  the owner's Wi-Fi was Public.
- **`massimo tls show`** with the server's data directory says TLS is off for a
  server run with `-tls self-signed` on its command line:
  it reads the settings, not the running server.

## The owner's design: login by fingerprint

The owner's expectation, as stated during the test, in place of the
switch that guards the session token:

1. Open the app, connect to the server.
2. The login screen, **with the option to enable the login by fingerprint**.
3. The login itself:
   - if it succeeds, ask for the fingerprint;
     - if that succeeds, create a new token on the server and keep it,
       with the user name, in the phone's credential store,
       behind the fingerprint;
     - if not, offer to repeat, or to log in without keeping anything;
   - if it fails, back to the login screen with the usual message.
4. Logging out in the app ends the current session on the server, and only that.
5. After logging out, on the login screen:
   - log in with the user name and password, as ever; or
   - a button **"Login by fingerprint"** (or "with saved credentials"),
     with the fingerprint or PIN icon,
     which takes the user name and the kept token from the credential store,
     makes a new session on the server with it,
     and signs the page in with that session.
6. The login screen has a button **"Select server"**, back to the connect page.
7. The server's settings can turn the login by fingerprint off.
8. The connect page's server row may show the fingerprint icon as a button
   of its own where credentials are kept, so that the login screen is skipped.
9. What is kept and matched: the server's **address and port**, the
   **user name** and the **token**, all three.
10. The server list shows, for each server, whether it holds kept credentials;
    a server's settings delete them, and so does removing the server;
    a **"Forget all stored credentials"** button deletes all of them.

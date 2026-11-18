# The apps

The apps for Android and iOS are the web UI in a thin native shell
(roadmap items 16 and 17).
The shell shows one page of the project's own, the connect page,
and then the page of the server the user chose, as a browser would.
What a page cannot do itself, the shell's adapter does:
the list of servers, discovery on the LAN, the probe of an address,
the pinned certificate of a self-signed server, the session kept for a
server, the login by fingerprint, and, later, the server running on the
phone and push.
The page and the adapter talk through one object, `window.massimo`,
whose contract is this document.

## The connect page

`internal/server/ui/connect.html` with `connect.js`,
in the server's embed as every other file of the UI,
served at `/ui/connect.html`.
A browser sees it without the bridge and says so:
a line that the page is the app's, and a link to the web UI.

The shell loads its own copy, bundled from the same `ui/` sources,
never a second copy in git.
The copy keeps the server's layout:
the page at `<origin>/ui/connect.html` and the files beside it under
`<origin>/ui/`, so that `<base href="../">` makes every URL of the page
resolve as the server's page resolves its own (`ui/style.css`,
`ui/i18n/de.json`).
The page carries its Content-Security-Policy in a `<meta>` element as well,
since the shell's copy is served without the server's header.

What the page shows:

- **the servers**, in the order saved,
  each with its name, its address and what the probe found:
  on the LAN, remote, not reachable, or a changed certificate;
  a tap opens the server, a long press (or its ⚙) its settings;
  a server with a kept login has a fingerprint button on its row,
  which opens it signed in at once;
- **add a server**: the servers discovery finds as they arrive,
  or an address typed (`host`, `host:port` or a URL);
  the probe, then the fingerprint sheet for a self-signed certificate;
- **a server's settings**: the name, the LAN and the remote address,
  "Ask for the fingerprint" (`ask`: each time the app comes back, when the
  app starts, or only after a logout),
  the login by fingerprint with the user's name and **Turn off** where one
  is kept (`login`),
  "Notify me on this phone" (`push`, where the app has push), Remove;
- **"Forget all stored credentials"**, under the list where a server has
  a kept login, which asks first;
- **"Run the server on this phone"**, a card with Start, Stop and the status,
  where the app has the server built in (`core`).

The texts are the web UI's, `connect.*` in `ui/i18n/<lang>.json`;
the language is the phone's, as `auto` is in a browser.

## The bridge, `window.massimo`

The adapter defines `window.massimo` before the page's own scripts run:
a script injected at the document's start (Android
`WebViewCompat.addDocumentStartJavaScript`, iOS a `WKUserScript` at
`atDocumentStart`), over the adapter's message channel (Android
`WebViewCompat.addWebMessageListener`, iOS a
`WKScriptMessageHandlerWithReply`).
How the script reaches the native side is the adapter's;
the page sees only what follows.

- **Every call returns a Promise.**
  A biometric prompt, a probe and a search take their time,
  and neither platform's channel answers synchronously.
  A failed call rejects with an `Error` whose message the page shows as it is.
- **Values are JSON**: objects, arrays, strings, numbers, booleans and `null`.
- **`massimo.version`** is the contract's version, the number `2`.
  A later version adds calls and fields; the page tests for what it uses
  (`massimo.login`, `massimo.core`, `massimo.push`) rather than for a version.
  Version 2 (AP MobileApp v1.1, Block 8) added the login by fingerprint:
  `login.*` and `connect` on a server's page, `forget` and `forgetAll` and
  `open`'s options on the connect page, a server's `login` and `ask`,
  which take the place of version 1's `biometric`.
- **Where the bridge exists, and with what:**
  - on the connect page, the shell's own origin: every call below
    but `token` and `setToken`;
  - on a server's page, the origin of the address `open` loaded:
    `version`, `token`, `setToken`, `login.status`, `login.keep`,
    `login.fingerprint` and `connect` alone, for that server alone;
  - on any other origin, a page the server's page links to: nothing.

  The adapter checks the origin of every message against these,
  not the page: a server's page is the server's,
  and it must never learn another server's token or address.

### The calls of the connect page

| Call | Resolves to | What the adapter does |
|---|---|---|
| `servers()` | an array of servers | reads the saved servers, in their order |
| `save(server)` | the server as saved | adds a server without an `id`, giving it one, or replaces the one with its `id`; keeps `fp` as `trust` set it |
| `remove(id)` | `null` | forgets the server, its pinned certificate, its session and its kept login, without a prompt |
| `discover(ms)` | `null`, when the search ends | browses `_massimo._tcp` for `ms` milliseconds (Android `NsdManager`, iOS `NWBrowser`) and sends a `found` event for each server resolved |
| `probe(urls, fp)` | an array of probe results, in the order of `urls` | `GET <url>healthz` of each in parallel, four seconds at most; `fp`, optional, is a certificate fingerprint it accepts beside those the system trusts |
| `trust(id, fp)` | `null` | pins the fingerprint for the server: its certificate is accepted in `probe` and in the WebView, and a self-signed one with another fingerprint is refused |
| `open(id, url, opts)` | `true`, or `false` | loads `url`, the server's `lan` or `remote`, in the WebView; another URL is refused. With `opts` `{login: true}` it first logs in by fingerprint (below): `false`, nothing opened, where the prompt was dismissed; a kept key the server refuses is forgotten and rejects |
| `forget(id, url)` | `{revoked}` | turns the server's login by fingerprint off: asks for the fingerprint once, revokes the kept key at `url`, an address of the server that answered (`""` for none), and forgets it; where the prompt or the server fails it forgets it all the same, `revoked` `false` |
| `forgetAll()` | `null` | forgets every kept login, without a prompt; the keys stay listed at their servers, to revoke there |
| `core.start()`, `core.stop()`, `core.status()` | the core's status | the server on the phone (Block 7); absent where it is not built in |
| `push` | | push to this phone (Block 8); absent where the app has none |

**A server** is an object:

| Field | |
|---|---|
| `id` | the adapter's, a string; absent before the first `save` |
| `name` | the name the list shows |
| `node` | the server's `node_id` from `/healthz` (docs/api-v1.md), `""` for a server without a store |
| `lan` | the address on the LAN, a URL ending in `/`, or `""` |
| `remote` | the address from elsewhere, a URL ending in `/`, or `""` |
| `fp` | the pinned fingerprint, `""` for none; read only for `save` |
| `login` | the user whose key the phone keeps for the login by fingerprint, `""` for none; read only for `save` |
| `ask` | when the fingerprint is asked once a login is kept: `return`, each time the app comes back from being hidden; `start` (the default), when the app starts; `logout`, only after a logout |
| `push` | `true`: the user asked for notifications from this server (Block 11) |

**A probe result** is an object:

| Field | |
|---|---|
| `url` | the URL probed, as given |
| `ok` | `true`: `/healthz` answered `ok` over a connection the adapter trusts, plain HTTP, a certificate the system trusts, or the fingerprint given |
| `node` | `node_id` of the answer, `""` without one |
| `version` | `version` of the answer |
| `fingerprint` | for HTTPS, the SHA-256 of the server's certificate as the server writes it (`SHA256:` and upper-case hex pairs separated by colons, `massimo tls show`) |
| `self_signed` | for HTTPS, `true` where the certificate is its own issuer |
| `trusted` | for HTTPS, `false` where the certificate is neither one the system trusts nor the fingerprint given; the adapter then reads nothing from the server, and `ok` is `false` |
| `error` | why it failed, in words, where `ok` is `false` and `trusted` is not |

**The core's status** (Block 7) is an object with `running`, a boolean,
`address`, the URL the page opens it at,
`version`, and `error` where it stopped by itself;
Block 7 adds what else Block 1's `Status()` carries.

### The calls of a server's page

| Call | Resolves to | What the adapter does |
|---|---|---|
| `token()` | a string, or `null` | the session or API key kept for the server it opened, `null` for none |
| `setToken(t)` | `null` | keeps `t` for that server, `null` forgets it: sealed by the Keystore (Android) or the Keychain (iOS), or, for a server with a kept login whose `ask` is `return` or `start`, in the app's memory alone |
| `login.status()` | `{can, user, ask, plain}` | `can`: the phone has a strong biometric enrolled; `user`: the user whose key is kept for the server, `""` for none; `ask`, as the server's; `plain`: the page is open over plain HTTP outside the private ranges, where no key is sent |
| `login.keep()` | `true`, or `false` | for a page just signed in: makes an API key of the signed-in user's at the server with the page's session (`POST /api/v1/users/{name}/tokens`, labelled with the phone), asks for the fingerprint and keeps the key sealed behind it; `false`, nothing kept and the key revoked again, where the prompt was not passed |
| `login.fingerprint()` | a string, or `null` | asks for the fingerprint, opens the kept key, makes a session with it (`POST /api/v1/login/key`) and resolves to that session, which it keeps as `setToken` would; `null` where the prompt was dismissed; a key the server refuses is forgotten and rejects |
| `connect()` | `null` | shows the connect page, "Select server"; the server's session stays as it was |

The page asks `token()` once, at its start, before it asks the server who is in;
without a token, or with one the server refuses, it shows its sign-in as a browser does,
with the login by fingerprint where `login.status()` offers it (below).
A sign-in, a key pasted and the first owner hand the new token to `setToken`,
and Sign out and Forget key hand it `null`.
The page then keeps no token in `localStorage`
(in a browser it keeps it there, as before).
OPAQUE runs in the page as in a browser;
the password never reaches the adapter.

### The events

The adapter sends an event as
`window.dispatchEvent(new CustomEvent("massimo", { detail }))`
on the page the event is for, `detail.type` naming it:

| `type` | The rest of `detail` | When |
|---|---|---|
| `found` | `service`: `name`, `host` (an address, IPv6 without brackets, or a name), `port`, `node`, `version`, `tls` (a boolean), `fp` | discovery resolved a server: `host` and `port` from its SRV and address records, the rest from its TXT record (`name`, else the instance's name; `node`; `v`; `tls=1`; `fp`, with TLS) |
| `network` | | the phone's network changed (Android `ConnectivityManager`, iOS `NWPathMonitor`); the page probes again |
| `core` | `status`, as `core.status()` | the server on the phone started or stopped (Block 7) |
| `notification` | | Block 11 |

An unknown `type` is ignored by the page.

## How the page uses it

- **Adding by address.**
  A URL is probed as it is (with `/` added);
  `host:port` as `https://` and then `http://`;
  `host` alone as `https://host/`, `http://host/`, `https://host:8099/`
  and `http://host:8099/`.
  The first answer in that order wins, an untrusted certificate being an answer.
  An address in the private ranges (RFC 1918, loopback, link-local,
  IPv6 unique local, a `.local` name) becomes the server's `lan`,
  any other its `remote`.
- **A self-signed certificate** comes back `trusted: false` with its fingerprint:
  the sheet shows it, the user compares it with the server's log or
  `massimo tls show`, and Trust probes again with that fingerprint,
  saves the server and `trust`s it.
  A saved server whose certificate changed shows the old and the new
  fingerprint and opens only once the new one is trusted the same way.
- **Plain HTTP** is allowed; the page warns for an `http://` address outside
  the private ranges, where anyone on the way can read the token and
  change the page.
- **Which address.**
  The page probes both addresses of a server with its `fp`, and takes
  the one that answers with the server's `node`, the LAN one first;
  it probes again on a `network` event.
  A `found` service whose `node` is a saved server's,
  at an address that is not its `lan`, becomes its `lan`:
  a LAN address that changed is found again.
  No Wi-Fi name is read.
- **The login by fingerprint** is the next section's:
  the row's button opens a server with `open(id, url, {login: true})`,
  **Turn off** calls `forget(id, url)` with the address the probe chose,
  and "Forget all stored credentials" `forgetAll()`.

## The login by fingerprint

The owner's design of the phone test of 2026-11-17 (AP MobileApp v1.1, Block 8).
The phone keeps an API key of the user's own, and the adapter holds it:
it never enters the page.
The server turns the key into a session (`POST /api/v1/login/key`,
docs/api-v1.md), which names the key, so that revoking the key,
under My profile → Sessions and keys by the phone's name, ends the session.

- **Enabling.**
  The sign-in form asks `login.status()` when it is drawn,
  and where the phone `can` shows the switch "Enable login by fingerprint".
  After a sign-in with it on, the page hands the session to `setToken`
  and calls `login.keep()`;
  where that resolves `false` or fails, the form offers "Try again" and
  "Sign in without it".
- **The login.**
  Where a key is kept (`user`), the form shows "Login by fingerprint",
  which calls `login.fingerprint()` and is signed in with the session it
  resolves to; the password stays the other way.
  "Select server" calls `connect()`, and in the app the Cube choice ends
  with the same entry.
- **When the fingerprint is asked** (`ask`, chosen per server on the
  connect page; a server without a kept login keeps its session sealed):
  - `return`: the session is in memory alone;
    the adapter covers the page when the app is hidden,
    and the prompt uncovers it, the session and the page as they were.
    The app's own file chooser does not count as hidden;
    Android may end a hidden app, and the return is then a start.
    While such a page is shown the system keeps no picture of it for
    its recent apps: on Android 13 and later by
    `setRecentsScreenshotEnabled(false)`, which leaves screenshots allowed,
    before it by `FLAG_SECURE`, which forbids them there too.
  - `start`: the session is in memory alone,
    so a start shows the form with "Login by fingerprint",
    or the row's button signs in at once.
  - `logout`: the session is sealed as without a kept login,
    and the fingerprint is asked on the sign-in form alone.
- **Logout** ends the session alone; the key stays,
  and the form offers both ways.
- **Kept and matched**: one key per server, used at whichever of its
  two addresses the page opened (D34-B); the record is bound to the
  server's id, its `node`, its pinned fingerprint and the user's name,
  so that it opens for nothing else.
  A certificate trusted anew forgets it.
- **Plain HTTP**: the key is never sent over plain HTTP to an address
  outside the private ranges (`plain`, D36-A);
  there the form offers the password alone and says why.
- **The prompt**: a strong biometric, with the screen lock's PIN as the
  prompt's fallback on Android 11 and later;
  on Android 8 to 10 the fingerprint alone.
  The key is a Keystore key that requires the prompt for every use,
  sealing included, so "ask for the fingerprint, then keep" is one step.
  On Android 8 to 10 a fingerprint enrolled later invalidates it;
  from Android 11, where the PIN opens it as well, it does not (D35-A).
  Turning the screen lock off invalidates it on every version.
  An invalidated key is forgotten, and the form offers the password.
- **Version 1's switch**, "Ask for my fingerprint or PIN", is gone:
  a server that had it on starts without a kept login and with its
  session forgotten.

## Testing

The pytest suite of `tools/ui-test/` covers the connect page with a fake
bridge in `test_connect.py`, put into the page by Playwright's
`add_init_script` as the token is (`tools/ui-test/README.md`):
the page holds no code for tests.
The same fake bridge checks the server's page's `token` and `setToken`,
the sign-in form's login by fingerprint and the Cube choice's "Select server".
Each adapter tests its own side: Block 3 for Android, Block 9 for iOS.

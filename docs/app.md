# The apps

The apps for Android and iOS are the web UI in a thin native shell
(roadmap items 16 and 17).
The shell shows one page of the project's own, the connect page,
and then the page of the server the user chose, as a browser would.
What a page cannot do itself, the shell's adapter does:
the list of servers, discovery on the LAN, the probe of an address,
the pinned certificate of a self-signed server, the token behind a
fingerprint or PIN, and, later, the server running on the phone and push.
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
- **add a server**: the servers discovery finds as they arrive,
  or an address typed (`host`, `host:port` or a URL);
  the probe, then the fingerprint sheet for a self-signed certificate;
- **a server's settings**: the name, the LAN and the remote address,
  "Ask for my fingerprint or PIN" (`biometric`),
  "Notify me on this phone" (`push`, where the app has push), Remove;
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
- **`massimo.version`** is the contract's version, the number `1`.
  A later version adds calls and fields; the page tests for what it uses
  (`massimo.core`, `massimo.push`) rather than for a version.
- **Where the bridge exists, and with what:**
  - on the connect page, the shell's own origin: every call below
    but `token` and `setToken`;
  - on a server's page, the origin of the address `open` loaded:
    `version`, `token` and `setToken` alone, for that server alone;
  - on any other origin, a page the server's page links to: nothing.

  The adapter checks the origin of every message against these,
  not the page: a server's page is the server's,
  and it must never learn another server's token or address.

### The calls of the connect page

| Call | Resolves to | What the adapter does |
|---|---|---|
| `servers()` | an array of servers | reads the saved servers, in their order |
| `save(server)` | the server as saved | adds a server without an `id`, giving it one, or replaces the one with its `id`; keeps `fp` as `trust` set it |
| `remove(id)` | `null` | forgets the server, its pinned certificate and its token |
| `discover(ms)` | `null`, when the search ends | browses `_massimo._tcp` for `ms` milliseconds (Android `NsdManager`, iOS `NWBrowser`) and sends a `found` event for each server resolved |
| `probe(urls, fp)` | an array of probe results, in the order of `urls` | `GET <url>healthz` of each in parallel, four seconds at most; `fp`, optional, is a certificate fingerprint it accepts beside those the system trusts |
| `trust(id, fp)` | `null` | pins the fingerprint for the server: its certificate is accepted in `probe` and in the WebView, and a self-signed one with another fingerprint is refused |
| `open(id, url)` | `null` | loads `url`, the server's `lan` or `remote`, in the WebView; another URL is refused |
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
| `biometric` | `true`: `token` asks for the fingerprint or PIN first |
| `push` | `true`: the user asked for notifications from this server (Block 8) |

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
| `token()` | a string, or `null` | the session or API key kept for the server it opened; where its `biometric` is on, after the fingerprint or PIN; `null` when there is none or the prompt was dismissed |
| `setToken(t)` | `null` | keeps `t` for that server, behind the Keystore (Android) or the Keychain (iOS); `null` forgets it |

The page asks `token()` once, at its start, before it asks the server who is in;
without a token, or with one the server refuses, it shows its sign-in as a browser does.
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
| `notification` | | Block 8 |

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
- **The biometric switch** saves `biometric`; the adapter then wraps the
  server's token behind the prompt, or unwraps it.

## Testing

The pytest suite of `tools/ui-test/` covers the connect page with a fake
bridge in `test_connect.py`, put into the page by Playwright's
`add_init_script` as the token is (`tools/ui-test/README.md`):
the page holds no code for tests.
The same fake bridge checks the server's page's `token` and `setToken`.
Each adapter tests its own side: Block 3 for Android, Block 9 for iOS.

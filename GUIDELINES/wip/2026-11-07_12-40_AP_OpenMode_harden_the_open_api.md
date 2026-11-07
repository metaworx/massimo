# AP OpenMode v1.0: Harden the open API

## TL;DR

An open server, meaning a store without users or `serve -addr` without a token,
answers every request from anyone who reaches it,
and any website the owner opens can also send it writes through the
owner's browser.
Cross-origin writes are refused in every mode,
the web UI says when the server is open,
and open mode answers only the local network.

- cross-site POST with a plain-text body → no CORS preflight → served;
  verified: a first owner created that way, everyone else `401`
  → `http.CrossOriginProtection` (Go 1.25+) around the handler: `403`
- open server in the UI → a warning banner under the header's rule:
  open, anyone who reaches it may change everything, how to close it
- open server from any address → only from the local network (D10);
  elsewhere `403` with the reason
- users mode: unchanged; its key travels in a header a cross-site request
  cannot set

### Blocks

1. Cross-origin writes refused: `http.CrossOriginProtection`, a JSON `403`,
   tests, `docs/api-v1.md`, `docs/reverse-proxy.md`, the CHANGELOG bullet
2. The open-server banner in the UI (D11): markup, CSS, `t()` in six
   languages, Playwright at 1100 and 390 px
3. Open mode only from the local network (D10): the check, tests, docs

## Discussion

### D9. A plan of its own, first: A

Settled at the Restart TL;DR gate: a security fix does not wait for a
feature, and AP Restart's D3-B (a restart from the box itself on an open
server) is only safe once Block 1 is in.
The plan was announced there as `CrossSite`;
it is `OpenMode` because the owner's answer added the banner and the limit,
which harden the same mode.

### D10. What "only from link-local" covers: open

The owner asked that an open server allow connections "only from link-local",
comparing open mode with the native app, which works across the LAN.

- **A (recommended).** The local network, as the first owner is already
  created from it (`localPeer`: loopback, RFC 1918, IPv6 unique local,
  link-local): a LAN client on `192.168.x.x` keeps working, as with the
  native app; a client from the internet (a port forward, a public IPv6
  address) gets `403`.
- **B.** On-link: loopback, or an address inside a subnet of one of the
  server's own interfaces. Closer to "the link"; refuses other private
  subnets and VPN clients; the interfaces are read for each check.
- **C.** Link-local literally: loopback, `169.254.0.0/16`, `fe80::/10`.
  Refuses every LAN client on a private address: only the box itself and
  self-configured links remain.

Behind a reverse proxy, with `-trusted-proxies`, the client the proxy names
is checked; without it, a request carrying `X-Forwarded-For`, `X-Real-IP` or
`Forwarded` counts as coming from beyond, as AP Restart's D3-B has it.

### D11. The banner: open

- **A (recommended).** Shown on every view while the server is open
  (`/api/v1/me` mode `open`, or `none` for `-addr` without a token),
  not dismissible, in the warning colours of both themes, under the header's
  rule: "This server is open: anyone who reaches it can change everything.
  Create the first owner under Settings → Users, or set MASSIMO_API_TOKEN in
  the server's settings file."
- **B.** The same, dismissible for the browser: quieter, and easy to forget.

## Analysis

### 1. The hole

Verified on 2026-11-07 against `origin/main` (after v0.10.0-alpha.1),
with a server on `127.0.0.1` and a throwaway data directory:

```text
POST /api/v1/users
Content-Type: text/plain;charset=UTF-8
Origin: https://attacker.example
Sec-Fetch-Site: cross-site

{"name":"intruder","role":"owner","password":"placeholder-pass-1234"}
```

was answered `201 Created`, the log said `first owner created user=intruder
remote=127.0.0.1`, and `GET /api/v1/cubes` was `401` from then on.

- A browser sends such a request from any page, with
  `fetch(url, {method: "POST", mode: "no-cors", body: "…"})`:
  `text/plain` is a CORS "simple" content type, so no preflight asks first.
  The page cannot read the answer; it does not need to.
- The server decodes the body as JSON whatever the `Content-Type`
  (`users_api.go`, `json.NewDecoder(r.Body)`), sends no CORS headers,
  and checks neither `Origin` nor `Sec-Fetch-Site`.
- The first owner is created "from the local network", and the owner's
  browser is on it.
- Every `POST` route is open to it on an open server: users, pairing,
  discovery, backups, the restore plan.
  `PUT` and `DELETE` are not simple methods and are refused by the missing
  CORS answer to their preflight.
- The address must be guessed: `127.0.0.1:8099` is the default, and a page
  can try a LAN range blind. Some browsers now ask before a public page reaches
  a private address; not all do.
- Users mode is safe: the session or key is sent as `Authorization: Bearer`,
  which a cross-site request cannot set without a preflight, and the browser
  keeps it in `localStorage`, not in a cookie.

### 2. The fix

`http.CrossOriginProtection` (`net/http`, Go 1.25; `go.mod` says 1.26):

- `GET`, `HEAD` and `OPTIONS` pass;
- otherwise `Sec-Fetch-Site: same-origin` or `none` passes,
  `cross-site` and `same-site` are refused;
- without `Sec-Fetch-Site` (older browsers), an `Origin` whose host is not
  the request's `Host` is refused;
- without either (curl, Home Assistant, `massimo` itself) the request passes.

It wraps `Server.Handler()`: the web UI's routes are `GET`s, and the Cube
proxy is not HTTP.
Its deny handler answers like every other refusal,
`{"error": "cross-origin request refused"}` with `403`.
Every browser since 2023 sends `Sec-Fetch-Site`,
so a reverse proxy that rewrites `Host` stays served;
`docs/reverse-proxy.md` says to pass `Host` for older ones.

### 3. The banner and the limit

The UI reads `/api/v1/me` at every start and after every sign-in
(`login.js`, `loadMe`): its `mode` says `open`, `users`, `token` or `none`.
The banner follows it; it goes away when the first owner is created, since
`afterLogin` reads `/me` again.
The limit is one check in `authorize` (`auth.go`), before `modeOpen` lets
the request through, and the same for `modeLegacy` without a token.

## Implementation Plan

### Block 1: Cross-origin writes refused

1. `internal/server/server.go`, `Handler()`: the handler wrapped in an
   `http.CrossOriginProtection` whose deny handler writes the JSON error.
2. `internal/server/crossorigin_test.go`: on an open server, a `POST` with
   `Sec-Fetch-Site: cross-site` is `403` and creates no user;
   `same-site` too; an `Origin` of another host without `Sec-Fetch-Site`
   is `403`; `same-origin`, `none` and a request with neither header pass;
   a cross-site `GET` passes; in users mode a cross-site `POST` with the
   right key is `403` too.
3. `docs/api-v1.md` "Authentication": a paragraph on cross-origin writes;
   `docs/reverse-proxy.md`: pass `Host`.
4. `CHANGELOG.md`: a bullet under `### Security`.

**Verification:**
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `-race` for `internal/server`,
`make cross`, `gofmt -l .`, `go vet` for linux, windows and darwin,
`make ui-test` (the page's own writes are same-origin and must keep working).

### Block 2: The banner (after D11)

1. `index.html`: the banner element under the header's `<hr>`;
   `style.css`: the warning colours as variables for light and dark.
2. `login.js`, `applyCaps`: shown while `me.mode` is `open` or `none`.
3. `ui/i18n/<lang>.json`: its text in all six, FR, IT, NL and PL marked.
4. `tools/ui-test/`: shown on an open server, gone after the first owner,
   at 1100 and 390 px.
5. `docs/ui.md`; the CHANGELOG bullet.

**Verification:** as Block 1, with `make ui-test`.

### Block 3: Open mode from the local network only (after D10)

1. `auth.go`, `authorize`: in open mode, and in legacy mode without a token,
   a client outside the range D10 settles is `403` with
   "this server is open and answers only its local network; create a user
   or set MASSIMO_API_TOKEN"; forwarding headers without `-trusted-proxies`
   count as outside.
2. Tests: loopback, a private and a public address, with and without
   forwarding headers and trusted proxies.
3. `docs/api-v1.md` "Authentication" ("only acceptable on the default
   localhost listener" becomes the rule as enforced); the CHANGELOG bullet.

**Verification:** as Block 1.

## Proposed commit message

Plan registration:

```
chore(aps): +OpenMode v1.0

An open server takes writes from any website the owner's browser opens; the plan refuses cross-origin writes, shows a banner while the server is open and limits open mode to the local network.

File-revision-v1: GUIDELINES/wip/2026-11-07_12-40_AP_OpenMode_harden_the_open_api.md::v1.0::2026-11-07::All::the first revision, from the Restart TL;DR gate's D9 and the owner's answer to it
Assisted-by: claude-code:claude-opus-5-5
```

Block 1:

```
fix(security:api): Refuse cross-origin writes

Any website the owner's browser opened could send an open server POSTs with a plain-text body, which needs no CORS preflight, and the server served them: such a request created a first owner and locked everyone else out. Every write now passes Go's cross-origin protection first (AP OpenMode v1.0, Block 1).

Key changes:
- internal/server: Handler wraps the routes in http.CrossOriginProtection; a write whose Sec-Fetch-Site is cross-site or same-site, or whose Origin names another host, is refused with 403 and a JSON error.
- Tests on an open server and with users: the refused and the admitted headers, and no user created by a refused request.
- docs/api-v1.md, "Authentication", and docs/reverse-proxy.md: what is refused, and the Host header a proxy passes.
- CHANGELOG.md: the bullet under Security.

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-07): the first revision, from AP Restart's TL;DR gate,
  where the hole was found and D9-A settled, and the owner's answer to it,
  which added the banner and the limit; D10 and D11 open.

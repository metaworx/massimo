# ANALYSIS Tls: TLS for massimo serve

How `massimo serve` should support TLS: self-signed, Let's Encrypt (HTTP-01,
TLS-ALPN-01, DNS-01), certificate files by flags, a reverse proxy,
and UPnP for the router port (added by the owner).
Nothing here is implemented; no project file changes with this document.
Figures were measured on 2026-11-06 in the main checkout at `f2a125f`,
Go 1.26.8, linux/amd64 (Xeon W-10885M, 2.4 GHz),
or in scratch modules outside the checkout; estimates are marked.
Nothing reached a Cube, the service or the network beyond module downloads.
Efforts in days and line counts are estimates.
Names and serials are placeholders (`KEQ0000000`, `example.net`).

## 1. What is there today (measured)

| Where | What |
|---|---|
| `cmd/massimo/serve.go` | `-listen` (default `127.0.0.1:8099`), one `net.Listen`, one `http.Server`; no `tls` in `serve`. README: "put a reverse proxy with TLS in front before exposing it" |
| The binary | `crypto/tls` (574 symbols by `go tool nm`, unstripped), `crypto/x509` (244), `crypto/ecdsa` (159) are linked already for the updater's HTTPS client; the server side of the handshake is not (0 `serverHandshake` symbols). Serving TLS adds no module |
| Size | 13,852,834 bytes amd64 with the Makefile's flags (`CGO_ENABLED=0 -trimpath -ldflags "-s -w"`); 19,962,912 unstripped |
| Dependencies | `go.mod` 4 direct + 7 indirect; `go list -m all` 27 modules besides massimo; `go list -deps ./cmd/massimo` 231 packages, 30 outside std and the module |
| `auth.go` | bearer token, constant-time compare; limiter per `clientIP` = host of `r.RemoteAddr`; `X-Forwarded-For` not trusted. `cubes_api.go`, `settings_api.go` log `r.RemoteAddr` directly |
| The UI | token in `localStorage` (`massimo.token`); 20 absolute paths `/ui/…`, `/api/v1/…`, `/healthz` (index.html 9, app.js 5, cubes.js 4, update.js 2); CSP `default-src 'self'`, no HSTS (`ui.go`) |
| `/events` | `Cache-Control: no-cache`, a `: keep-alive` comment every 30 s, no `X-Accel-Buffering` |
| `/healthz` | open; `install.healthURL` hard-codes `http://` into the systemd drop-in (`MASSIMO_HEALTH`), the plist (`@HEALTH@`) and the Windows helper's `-health`; `waitHealthy` uses a default `http.Client`; CI's service jobs `curl -fsS http://127.0.0.1:18099/healthz` |
| Linux unit | user `massimo`, `ProtectSystem=strict`, `StateDirectory=massimo`: only `/var/lib/massimo` writable, `/etc/massimo/env` 0600 read-only; `install -listen` writes `AmbientCapabilities=CAP_NET_BIND_SERVICE` for a port below 1024 |
| macOS, Windows | `_massimo` under launchd, a port below 1024 only on every address; LocalSystem, `C:\ProgramData\Massimo` SYSTEM and Administrators only, one firewall rule per program (all ports) |
| `-remove -purge` | removes `/var/lib/massimo`, `/etc/massimo`, the Windows data folder |
| docs/store.md | §4: `MASSIMO_LISTEN` stays in the file (needed before the store opens); §10 item 4: the first-owner form only for loopback, RFC 1918 and link-local peers, `X-Forwarded-For` not trusted; item 8: sessions in `localStorage` |
| docs/server.md | the trust anchor: the page from the edge over its own TLS, fingerprint confirmed once; "a public certificate for a name that resolves to the LAN address avoids the dialog" |

### 1.1 The cryptography's cost

A scratch program with a `crypto/tls` server and client over loopback:

| Operation | This machine | Pi 1, estimate ×50–100 |
|---|---:|---:|
| ECDSA P-256 key and self-signed certificate | 0.47 ms | 25–50 ms |
| RSA-2048 key generation | 54 ms | 3–10 s |
| Full TLS 1.3 handshake, client and server, mean of 200 | 1.44 ms | 70–150 ms |
| Resumed handshake (PSK with ECDHE) | 1.41 ms | 70–140 ms |
| ECDSA P-256 sign / verify | 47 µs / 102 µs | 2–5 ms / 5–10 ms |

The Pi factor is an estimate (ARM1176 at 700 MHz, in order, no SIMD,
Go's generic 32-bit field arithmetic);
the Pi 1 serves today at 7–9 MB RSS and under 1 % CPU (packaging/README.md).
A browser keeps one to six connections per page and the UI polls over them,
so a page load costs a few handshakes: under a second at the top of the range.
RSA signing would be 100–200 ms per handshake (estimate),
which is why RSA is out.

### 1.2 Library deltas

Nine scratch modules with the Makefile's flags,
each on the same baseline that links what massimo links already (`http.Server` over TLS,
`http.Client`, `encoding/xml`, `encoding/json`): 6,979,746 bytes amd64,
6,750,370 armv6.

| Added | Δ amd64 | Δ armv6 | go.mod requires | modules in graph | packages outside std |
|---|---:|---:|---:|---:|---:|
| `golang.org/x/crypto/argon2` (comes anyway, docs/store.md §6) | +88 KiB | +64 KiB | 2 | 5 | 4 |
| `golang.org/x/crypto/acme` (low-level client, for DNS-01) | +196 KiB | +256 KiB | 1 | 5 | 2 |
| `golang.org/x/crypto/acme/autocert` | +584 KiB | +512 KiB | 3 | 8 | 8 |
| `github.com/go-acme/lego/v4` core, DNS-01 with a hook provider | +1.5 MiB | +1.5 MiB | 11 | 231 | 34 |
| lego with all 196 DNS providers | +51.8 MiB | +47.4 MiB | 213 | 687 | 1,221 |
| `github.com/huin/goupnp` v1.3.1 (IGD client) | +368 KiB | +384 KiB | 2 | 2 | 8 |
| `gitlab.com/NebulousLabs/go-upnp` | +1.1 MiB | +1.1 MiB | 6 | 8 | 31 |
| `github.com/jackpal/gateway` + `go-nat-pmp` (NAT-PMP only) | +32 KiB | +64 KiB | 3 | 12 | 3 |

autocert's delta is mostly `x/net/idna` and the `x/text` tables.
Over the argon2 baseline ACME costs about +500 KiB, 3.6 % of today's binary.
The core of lego alone puts 231 modules into the graph;
with its providers it is four times massimo.

## 2. Self-signed

**Generation.**
By the server at the first start with `-tls self-signed` (`MASSIMO_TLS=self-signed`),
into `<data>/tls/self-signed.pem` and `.key` (0600).
The data directory is the one place every service may write;
it survives `update` and `install`, and `-purge` takes it with the store.
`massimo tls show | self-signed [-names …]` prints the fingerprint or regenerates,
run as the service user like `store info`.
At 25–50 ms on the Pi (estimate) nothing has to happen at install time.

**Key, validity, names.**
ECDSA P-256 (§1.1) and 825 days:
Apple refuses to trust a longer TLS server certificate even after a manual
install (mkcert's reason for the same number, from its documentation,
not tested here).
The server regenerates 30 days before expiry and logs the new fingerprint.
Names: the listen host when it is an address, else the hostname,
`<hostname>.local`, `localhost`, the box's addresses at generation time,
and `-tls-names` for a name the owner gave the box.
A new name set is a new certificate and a new exception on every device,
so the server never regenerates on its own when addresses move:
it logs `tls: 192.168.1.20 is not in the certificate` and names the command.

**The warning.**
Every browser warns once per device and remembers the exception per certificate
(iOS Safari seems to keep it for a session only; to verify on a phone).
Before the exception the page cannot say anything.
After it, a header notice "self-signed certificate,
fingerprint …" from a `tls` field of `/healthz` gives the owner the string to
compare with the log.
The fingerprint (SHA-256 of the DER) goes to the log at start, `massimo config`,
`massimo tls show`, `/healthz` and Settings → Server under Expert.

**What it does not give.**
Chrome and Firefox refuse to register a service worker on an origin whose
certificate they were told to ignore ("An SSL certificate error occurred when
fetching the script"), though `crypto.subtle` works.
The trust anchor of docs/server.md therefore needs a certificate the device
trusts without a click: a local CA root installed once, or a public one (§3.3).
This finding shapes the recommendation.

**A local CA (mkcert pattern).**
The server makes a root (10 years,
name-constrained to its names and the private ranges,
so a leaked box key cannot mint `bank.example.net`),
serves it at `/ui/massimo-root.crt`, and signs leaves that rotate silently.
The owner installs the root once per device (iOS profile plus "Enable full trust",
Android user CA, which Chrome trusts and apps do not, the Keychain,
the Windows root store, Firefox's `security.enterprise_roots.enabled`):
1.5 days plus the clicks that are the reason mkcert exists,
worth it only for an owner without a DNS name.

## 3. Let's Encrypt

Shared by all three paths:
a DNS name (no plain-IP certificates through autocert), an account key,
and a 90-day certificate renewed every 60 days with nobody watching (Let's
Encrypt stopped expiry mails in 2025).
Rate limits bite during experiments:
50 certificates per registered domain per week, 5 duplicates per week,
5 failed validations per account and hostname per hour (as published in 2025;
to re-read when the block starts, and the staging directory is for the tests).
`golang.org/x/crypto/acme/autocert`'s `Manager` does TLS-ALPN-01 always
and HTTP-01 once `HTTPHandler` was called.
It keeps `acme_account+key` and the certificates in a `DirCache` (directory 0700,
files 0600),
renews from 30 days before expiry (`RenewBefore` is capped there) with jitter,
and takes `Prompt: AcceptTOS` and an optional `Email`.
`HostPolicy` must be a whitelist, or every SNI a scanner sends becomes an order.
It logs nothing: errors reach only the `GetCertificate` caller,
so the server watches the certificate it serves (hourly;
under 14 days left → a log warning and the UI header notice through `/healthz`).

### 3.1 HTTP-01 on port 80

Let's Encrypt connects to port 80 of the name's address (it follows redirects to
80 and 443 only).
`HTTPHandler(nil)` answers `/.well-known/acme-challenge/<token>` for whitelisted
hosts (403 otherwise,
404 for an unknown token) and redirects the rest to `https://` with 302 (GET and HEAD,
400 for other methods; a fallback handler gives 404 instead).
The manager validates in the background on its own schedule,
so with autocert the listener stays open.
Opening it only during a challenge needs the low-level flow of §3.3 with a
listener around `Accept` and `WaitAuthorization`, about 40 lines more.
Binding 80: on Linux the capability drop-in `install -listen` writes today,
extended to this port; on macOS `:80` on every address; on Windows LocalSystem,
unless `http.sys` holds it (IIS).
Behind a home router the box need not bind 80 at all:
the router forwards public 80 to the box's 8080 and the validation never sees
the inner port.

### 3.2 TLS-ALPN-01 on 443 alone

The validation connects to port 443, offers the ALPN protocol `acme-tls/1`,
and `GetCertificate` answers with a challenge certificate: no port 80,
no second listener, no redirect.
The public port must be 443; the router may forward it to any port of the box.
So the owner's "temporary port 80" is unnecessary when 443 is forwarded.
HTTP-01 stays for an owner whose 443 is taken (a NAS,
the router's interface) and whose 80 is free.

### 3.3 DNS-01 "via a DNS plugin"

autocert has no DNS-01.
The low-level `acme` package has all of it (`AuthorizeOrder`,
`DNS01ChallengeRecord`, `Accept`, `WaitAuthorization`, `CreateOrderCert`):
150–250 lines of flow, a hook,
and a wait for the TXT record at the zone's name servers; +196 KiB measured,
no module beyond `x/crypto`.

| Way | The owner runs | Cost |
|---|---|---|
| `-acme-dns-hook <script>`, called `set <name> <txt>` and `clear <name> <txt>` as dehydrated and acme.sh do | a 10-line script against the provider's API, or an acme.sh `dnsapi/` script wrapped | 1.5–2 days; the binary knows no provider |
| lego's providers in the binary | nothing | +1.5 MiB and 231 modules for the core, +52 MiB and 687 with providers: out |
| an external client (certbot, acme.sh, lego CLI) with §4's files and a deploy hook | the client and its plugin (certbot some 20, acme.sh some 150, lego 196) | 0 days beyond §4 |

DNS-01 needs no inbound port: the A record may point at the box's LAN address,
so the owner gets a trusted certificate on the LAN,
the padlock and the service worker with nothing exposed.
That is what docs/server.md asks for and what self-signed cannot give,
the reason to prefer it over a local CA;
the external client covers it from the day §4 exists.

### 3.4 Realism per environment

| Environment | HTTP-01, TLS-ALPN-01 | DNS-01 |
|---|---|---|
| Pi, Linux box, NAS, Windows PC, Mac at home behind NAT | a port forward (by hand, or §6) and a dynamic-DNS name; impossible behind CGNAT or DS-Lite (cable and mobile uplinks); 80 and 443 often taken on a NAS, a LoxBerry or a PC | a name and the provider's API; the hook in a shell or PowerShell |
| A box with a public address (VPS, a line with the firewall opened) | straightforward | straightforward |

The account and the certificates in `<data>/tls/acme/` survive updates and `install`;
`-purge` drops the account, which is harmless (a new one is registered,
the per-domain limits stay).

## 4. Existing certificates by flags

`-tls-cert <pem>` and `-tls-key <pem>` (`MASSIMO_TLS_CERT`, `MASSIMO_TLS_KEY`),
the certificate file carrying the chain as every ACME client writes it.
`install -tls-cert … -tls-key …` copies them next to the settings (`/etc/massimo/`,
`C:\ProgramData\Massimo\`), owned by the service user and 0600,
and writes the variables into the settings file:
a box has TLS at install time and later installs keep it as they keep `-listen`.
Files kept elsewhere must be readable by the service user,
which certbot's `/etc/letsencrypt/live` (root, 0700) is not:
a deploy hook copies them, the document shows it.

**Reload.**
`tls.Config.GetCertificate` runs per handshake: stat both files (microseconds),
re-read when size or mtime changed,
keep the old pair and log once when the new one does not parse.
That covers certbot, acme.sh and win-acme renewals with no signal and no restart.
SIGHUP (the roadmap's log rotation) can re-read too when it comes,
but is not needed.

**The one mechanism.**
Yes: every source ends in a `GetCertificate`, files by stat,
self-signed as files the server wrote, autocert as its own.
`-tls off | files | self-signed | acme` (`files` implied by `-tls-cert`),
one `tls.Config` (TLS 1.2 minimum, Go's suites, `NextProtos` h2,
http/1.1 and `acme-tls/1` with autocert), one `tls.NewListener`.

## 5. Behind a reverse proxy

Plain HTTP on localhost stays the way wherever a proxy exists (nginx, Caddy,
Traefik, the Synology and QNAP proxies): it terminates TLS,
renews with its own client and sets HSTS.
What the server must learn:

- **The client's address.**
  `-trusted-proxies 127.0.0.1,::1,192.168.1.5` (CIDRs allowed,
  `MASSIMO_TRUSTED_PROXIES`, default empty = today's rule):
  a `RemoteAddr` in the list makes the client the rightmost `X-Forwarded-For`
  entry not in the list, else `X-Real-IP`.
  Without it the limiter counts every client as `127.0.0.1`,
  so ten wrong tokens from anyone lock everybody out for five minutes,
  and the planned first-owner form sees only loopback.
  With it, the form is offered to real LAN peers and refused to the rest;
  one `clientIP` serves the limiter, the local-network check,
  the audit and the log lines.
- **`X-Forwarded-Proto`.**
  Unused: no cookies, no absolute URLs, HSTS from the proxy.
- **Server-sent events.**
  The 30-second keep-alive already satisfies nginx's `proxy_read_timeout 60s`
  and Synology's; buffering is what breaks the stream,
  and `X-Accel-Buffering: no` on `/events` (one line,
  nginx honours it) switches it off,
  so the sample needs only HTTP/1.1 and an empty `Connection` header.
  Caddy streams `text/event-stream` by itself.
- **A path prefix.**
  `/massimo/` does not work today (20 absolute paths, §1).
  Relative ones (`ui/style.css`, `api/v1/cubes`, `healthz`,
  the `EventSource` URL) work at `/` and at `/massimo/` alike with the proxy
  stripping the prefix; the server never learns it,
  and the CSP's `'self'` is unaffected.
- **A document** (`docs/proxy-tls.md`) with the samples and the Synology clicks:

```nginx
server {
    listen 443 ssl; server_name massimo.example.net;
    ssl_certificate /etc/letsencrypt/live/massimo.example.net/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/massimo.example.net/privkey.pem;
    location /massimo/ {            # or location / { … } on a name of its own
        proxy_pass http://127.0.0.1:8099/;
        proxy_http_version 1.1; proxy_set_header Connection "";
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

```caddyfile
massimo.example.net {
    redir /massimo /massimo/
    handle_path /massimo/* { reverse_proxy 127.0.0.1:8099 }
}
```

On the box: `serve -trusted-proxies 127.0.0.1`;
a proxy on another host gets its address,
with `-listen` on a LAN address the firewall limits to it.

## 6. UPnP

**The protocol.**
SSDP `M-SEARCH` to `239.255.255.250:1900` for `WANIPConnection` (or `WANPPPConnection`),
`GET` of the description at the reply's `LOCATION`, the service's `controlURL`,
then SOAP `AddPortMapping` (external port, `TCP`, internal port and address,
description `massimo`, a lease), `GetExternalIPAddress`,
`DeletePortMapping` at shutdown.
NAT-PMP (RFC 6886) and PCP, the Apple-style siblings,
need the gateway's address from the routing table, a per-OS routine;
AirPorts are gone since 2018 and miniupnpd answers IGD too,
so IGD alone covers the realistic set.

**Libraries (§1.2).**
goupnp +368 KiB, 2 modules, typed clients for IGD1 and IGD2;
NebulousLabs +1.1 MiB, 8 modules; jackpal +32 KiB, NAT-PMP only.
A client of the project's own, as `internal/protocol/cube` builds its frames:
one SSDP search, one `encoding/xml` parse,
one SOAP envelope with three actions and their errors (718 conflict,
725 permanent leases only, 606 not authorised), the lease loop.
About 300 lines plus a fake IGD, +30–60 KiB (estimate,
everything it needs is linked), no module:
small enough to prefer over goupnp if UPnP is done at all.

**Leases and mapping.**
Ask 3600 s and re-add every 30 minutes;
on 725 re-add with lease 0 and delete at shutdown;
a router that forgets at reboot gets it back at the next cycle;
a refusal is logged once an hour.
Public 443 → the server's HTTPS port; 80 → the ACME helper only with HTTP-01.

**How often it works (estimate, not measured).**
Roughly one home router in two accepts an `AddPortMapping` out of the box (retail Netgear,
TP-Link, Asus, Google, eero, many cable ISP boxes).
The Fritz!Box family, common where the owner lives,
ships UPnP readable but port changes off and needs the per-device permission;
UniFi, OpenWrt and business gear have it off; BSI and CISA say keep it off.
Behind CGNAT or DS-Lite the mapping succeeds and reaches nothing,
which `GetExternalIPAddress` reveals (100.64.0.0/10 or a private range);
IPv6 would need `WANIPv6FirewallControl`, rarely implemented,
so this is IPv4 only.
A refusal or a disabled UPnP is logged with the manual steps (forward 443 to
this box's port) and the server carries on.

**Security.**
A mapping puts the whole HTTP server on the internet:
the UI files and `/healthz` public,
`/api` behind the token and the limiter (ten failures lock an address,
at most 4,096 tracked),
so scanners from many addresses are log noise rather than a lockout.
The first-owner form's local-network check sees the true peer without a proxy
and refuses the internet correctly.
What remains is a young Go server on a Pi exposed to everyone,
every future bug reachable:
the reason design.md §6 and docs/server.md make the public server a relay with
an outbound connection from the edge.
UPnP is the stopgap before the relay.

**Address and name.**
The mapping gives reachability, not a name; the next ask would be dynamic DNS,
which most routers already do (Fritz!Box, Swisscom,
Sunrise) and the document points there.
`GetExternalIPAddress` is worth a log line.

**Environments, WSL 2, the flag, the test.**
For the Pi, the Linux box, the NAS, the PC and LoxBerry behind a home router;
not the VPS.
WSL 2 has no multicast and no inbound connections,
so nothing here is testable from this checkout:
the real router is reached from the Pi's `serve` or the Windows side,
with the owner's go.
`-upnp` (default off, `MASSIMO_UPNP=1`); log lines `upnp: gateway <url>`,
`upnp: external address 203.0.113.7`, `upnp: mapped 443 → 8443`,
`upnp: refused: <reason>`, `upnp: no gateway answered`, `upnp: mapping removed`.
A Go test runs a fake IGD on loopback like `fakecube.StartDiscovery`:
an SSDP responder on a unicast port the test passes (`-upnp-target 127.0.0.1:1900`,
like `-discover-target`),
an `httptest` server for the description and the SOAP control URL,
assertions on the exact SOAP bodies and on 718 and 725.
Effort 3 days, the Pi day included.

## 7. What TLS changes elsewhere

- **Not HTTP.**
  The Cube proxy on 62910 and discovery on 23272 stay plain;
  the Cube protocol has no TLS.
- **The secure context.**
  TLS on the edge is the prerequisite for the service-worker trust anchor
  and `crypto.subtle` of docs/server.md;
  a self-signed leaf is not enough for the service worker (§2),
  a trusted certificate is, so §3.3 or a local CA comes before that design.
  `http://localhost` is secure already, which helps nobody on a phone.
- **The token in `localStorage`** is unchanged and stops crossing the LAN in the clear.
- **HSTS** only when the server terminates TLS,
  opt-in `-hsts` (`max-age=31536000`): on by default it would lock out,
  for a year, an owner who turns TLS off again; browsers ignore it for IP hosts.
- **Two schemes.**
  One `-listen`, plain or TLS,
  plus the plain helper `-tls-http :80` for HTTP-01 and the redirect.
  A second full listener (LAN plain,
  forwarded port TLS) doubles the surface and keeps the token in the clear on the LAN,
  which self-signed or DNS-01 makes unnecessary.
- **`/healthz` over HTTPS.**
  `healthURL` emits `https://` when the service has TLS;
  `waitHealthy` skips verification for a loopback host (a version from the same box;
  anything else answering there is root already);
  the Windows helper gets the same URL.
  `install` writes the scheme where it writes the port.
  The updater touches the binary only; `<data>/tls/` is untouched by an update.
- **CI.**
  The macOS job installs with `-tls self-signed` and uses `curl -fsSk https://…`,
  Windows stays plain, so both paths run on every push.
  A Pebble job (Let's Encrypt's test CA,
  which `x/crypto/acme`'s own tests use) is optional and later.
- **Where the configuration lives.**
  Flags and `MASSIMO_TLS*` variables in the settings file by docs/store.md §4's
  rule (needed before the store, a restart to change, like `MASSIMO_LISTEN`);
  `<data>/tls/` for what the server writes (the self-signed pair,
  the ACME cache); the settings directory for the owner's files;
  nothing in the store; Settings → Server shows the state read-only (scheme,
  source, names, expiry, fingerprint).
  `install -tls self-signed | -tls-cert … -tls-key … | -tls acme -tls-names …`
  writes the variables and, below port 1024, the capability.
- **Tests.**
  Go: the generator (names, validity, key), the stat reload by rewriting files,
  `-trusted-proxies` parsing and `clientIP`, the redirect,
  the ACME path filter (only `/.well-known/acme-challenge/` reaches the manager,
  a fake token store behind a seam), the expiry watch, the fake IGD;
  `httptest.NewServer(s.Handler())` as today behind a `tls.NewListener`.
  Only a box: a real certificate from Let's Encrypt staging on the Pi with a name,
  a browser's dialog, a router.

## 8. The options compared

| Option | The owner runs | Internet reach needed | Binary delta | Effort | Fits |
|---|---|---|---|---|---|
| Files by flags, stat reload | an external ACME client, or any certificate | no | 0 | 2 d | everything; the base of the rest |
| Self-signed at first start | nothing; one warning per device | no | 0 | 1.5 d | LAN only; no service worker |
| Local CA, mkcert pattern | a root installed once per device | no | 0 | 1.5 d | an owner without a name who wants the padlock |
| autocert TLS-ALPN-01 | a name, 443 forwarded | 443 | +500 KiB over argon2 | 2 d + Pi | a forward or a public address |
| autocert HTTP-01 | a name, 80 forwarded, the helper listener | 80 | same package | +0.5 d | 443 taken, 80 free |
| DNS-01 hook over `x/crypto/acme` | a name, a hook script | no | +196 KiB | 2 d | everything; trusted on the LAN |
| DNS-01 with lego | a name, provider credentials | no | +1.5 MiB core, +52 MiB providers | 2 d | out on size and modules |
| Reverse proxy | nginx, Caddy, Traefik, the NAS's | the proxy's business | 0 | 1.5 d | NAS, LoxBerry, anyone with a proxy |
| UPnP, own client | a router that allows it (one in two, estimate) | it creates the reach | +30–60 KiB (estimate) | 3 d | home routers; a stopgap before the relay |

## 9. Recommendation

Build the file path first and let every other source produce files or a `GetCertificate`;
make DNS-01 the way to a trusted certificate on the LAN, since that,
not a self-signed leaf, is what the service-worker design needs;
keep provider code out of the binary; leave UPnP out until the relay exists.
Blocks a plan could take, one pull request each:

1. **TlsFiles** (2 d): `-tls`, `-tls-cert`, `-tls-key`, the variables,
   one `tls.Config` with the stat-reloading `GetCertificate`, `-hsts`,
   `healthURL` with the scheme and the loopback exception in `waitHealthy`,
   `install -tls-cert/-tls-key`, the Windows helper, docs.
   Tests: a generated pair, a rewrite, the URL.
2. **TlsSelfSigned** (1.5 d, after 1): generation into `<data>/tls/`,
   `-tls-names`, the 30-day regeneration, the fingerprint in the log,
   `massimo config`, `massimo tls show`, `/healthz` and the header notice;
   CI's macOS job on it.
3. **TlsProxy** (1.5 d, independent):
   `-trusted-proxies` and one `clientIP` everywhere, `X-Accel-Buffering: no`,
   relative paths in the UI (checked with `make ui-test` at both widths),
   the proxy document.
   It also unblocks the first-owner form behind a proxy.
4. **TlsAcme** (2 d plus a Pi day with a name and staging, after 1):
   autocert with TLS-ALPN-01, `-tls-names`, `-acme-email`,
   the cache in `<data>/tls/acme/`, the expiry watch and notice,
   `-tls-http :80` for HTTP-01 with the redirect.
5. **TlsDns** (2 d, after 1, can wait): `-acme-dns-hook` over `x/crypto/acme`;
   until then the document shows certbot and acme.sh with a deploy hook into block 1.
6. **Upnp** (3 d, can wait; the recommendation is not now, D13).
7. A local CA (1.5 d), only if real owners without a name hit the warning.

Blocks 1 to 4 are about a week; 1 to 3 make a release before 4.

## 10. Open decisions

- **D1**
  Does every certificate source end in one `GetCertificate`?
  - A (recommended) yes, because one path is tested once and an external client
    works from day one.
  - B a path per source, which costs three tests and three documents for one listener.
- **D2**
  How is self-signed turned on?
  - A (recommended) `-tls self-signed`,
    generated at the first start into `<data>/tls/`,
    because it needs no step and no privilege.
  - B only by `massimo tls self-signed` as the service user,
    which costs the installer a command.
  - C on whenever `-listen` is not loopback,
    which breaks every `http://` client at the first update.
- **D3**
  Key and validity?
  - A (recommended) ECDSA P-256, 825 days, the names of §2,
    because the handshake is cheapest and Apple trusts nothing longer.
  - B RSA-2048, which costs seconds to generate and 100–200 ms per handshake on
    the Pi 1 (estimate).
  - C ten years, which Apple devices refuse even after a manual install.
- **D4**
  A local CA?
  - A (recommended) not now,
    because DNS-01 gives the trusted certificate without a step on any device.
  - B now, name-constrained, the root at `/ui/massimo-root.crt`,
    which costs 1.5 days and five to ten clicks per device and OS.
- **D5**
  Which ACME challenge first?
  - A (recommended) TLS-ALPN-01, HTTP-01 only with `-tls-http :80`,
    because no port 80 and no second listener are needed.
  - B HTTP-01 with a permanent port-80 listener,
    which costs the capability on three systems and a second listener.
  - C port 80 opened only during a challenge,
    which autocert cannot do and costs block 5's flow plus 40 lines.
- **D6**
  DNS-01 how?
  - A (recommended) an external client into block 1 now and the hook block later,
    because the first is zero code and the second keeps providers out.
  - B the hook block at once, which costs 2 days before anyone asked.
  - C lego, which costs +1.5 MiB and 231 modules for the core,
    +52 MiB and 687 with providers (measured).
- **D7**
  The client's address behind a proxy?
  - A (recommended) `-trusted-proxies` with CIDRs, default empty,
    because nothing changes without a proxy and the limiter and the first-owner
    form become right behind one.
  - B trust loopback by default,
    which lets any local process spoof the address the form checks.
  - C never trust the headers,
    which locks every client out together behind a proxy.
- **D8**
  A path prefix?
  - A (recommended) relative URLs in the UI and the proxy strips the prefix,
    because the server needs no flag and a subdomain keeps working.
  - B a `-base-path` flag,
    which costs a rewrite and a second thing to keep in step with the proxy.
  - C unsupported, which costs NAS owners a second name and certificate.
- **D9**
  One listener or two?
  - A (recommended) one `-listen` plus the helper `-tls-http`,
    because one scheme per box is one set of tests and no token in the clear.
  - B a second full `-listen-tls`,
    which doubles the surface and keeps the token in the clear on the LAN.
- **D10**
  HSTS?
  - A (recommended) off by default and `-hsts` sends a year,
    because an owner who turns TLS off is otherwise locked out of every device
    for that long.
  - B automatic with TLS, which costs exactly that lockout.
- **D11**
  The health check over TLS?
  - A (recommended) `https://` and no verification for a loopback host,
    because the version is no secret and the peer is the same box.
  - B a plain loopback listener for `/healthz`,
    which costs a listener and a port on every TLS box.
  - C the helper pins the fingerprint from the data directory,
    which costs it read access it lacks on macOS and Windows.
- **D12**
  Where do the files live?
  - A (recommended) the owner's files next to the settings,
    the server's in `<data>/tls/`, the variables in the settings file,
    nothing in the store, because that is docs/store.md §4's split.
  - B everything in the data directory,
    which costs `-purge` the owner's certificate and the installer a second place.
- **D13**
  UPnP?
  - A (recommended) not now,
    because an open port exposes the whole server and works on about one router
    in two (estimate); the document explains the router's forward,
    DNS-01 gives the LAN certificate, the relay is the way outside.
  - B `-upnp` opt-in with the project's own client and a fake IGD test,
    which costs 3 days and a warning the owner must read.
  - C goupnp, which costs +368 KiB and 2 modules for what 300 lines do.

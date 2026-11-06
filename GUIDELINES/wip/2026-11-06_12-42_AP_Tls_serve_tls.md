# AP Tls v1.1: TLS for massimo serve

## TL;DR

`massimo serve` gets TLS in five blocks,
every certificate source ending in one `GetCertificate` on one `tls.Config`:
certificate files re-read when they change,
a self-signed certificate the server makes, a reverse proxy in front,
and Let's Encrypt by TLS-ALPN-01, HTTP-01 and DNS-01.

- `-tls off | files | self-signed | acme`, `-tls-cert`, `-tls-key`,
  `-tls-names`; `MASSIMO_TLS*` in the settings file, nothing in the store
- files → stat per handshake, re-read on change, old pair kept on a bad one
- HSTS on by default, one year; `-hsts <days>`, `0` off;
  only when the server terminates TLS
- `/healthz` → `https://` with TLS; the helper skips verification on loopback
- self-signed → ECDSA P-256, 825 days, `<data>/tls/`,
  renewed 30 days before expiry; fingerprint in the log, `massimo config`,
  `massimo tls show`, `/healthz`, the UI header
- `-trusted-proxies` → one `clientIP` everywhere; relative URLs in the UI
- acme → autocert, TLS-ALPN-01; `-tls-http :80` for HTTP-01 and the redirect
- DNS-01 → `-acme-dns-hook <script>` over `x/crypto/acme`, wildcards;
  a sample hook for Infomaniak's API; certbot and lego into the files source
- later: UPnP, a local CA

Rests on
[2026-11-06_11-46_ANALYSIS_Tls_serve_tls_support.md](2026-11-06_11-46_ANALYSIS_Tls_serve_tls_support.md);
the owner's decisions D1 to D13 are in the Discussion.
Run order 1, 2, 4, 5, then 3, once store stage (b) is on `main`.

### Blocks

1. TlsFiles: `-tls files`, the stat-reloading source, HSTS,
   the health URL's scheme, `install -tls-cert/-tls-key`, the documents
2. TlsSelfSigned: generation into `<data>/tls/`, `-tls-names`, the renewal,
   `massimo tls`, the fingerprint everywhere, CI's macOS job on it
3. TlsProxy: `-trusted-proxies` and `clientIP`, `X-Accel-Buffering: no`,
   relative URLs in the UI, `docs/reverse-proxy.md`
4. TlsAcme: autocert with TLS-ALPN-01, `-acme-email`, the cache,
   the expiry watch and notice, `-tls-http :80` for HTTP-01
5. TlsDns: `-acme-dns-hook` over `x/crypto/acme`, wildcards,
   the Infomaniak sample hook, certbot and lego recipes

## Discussion

The analysis measured what is there (section 1), compared self-signed,
a local CA, the three Let's Encrypt challenges, certificate files by flags,
a reverse proxy and UPnP (sections 2 to 8),
and recommended the file path first with every other source producing files or a
`GetCertificate` (section 9).
The owner answered its section 10 on 2026-11-06, option A of each,
except D10:

- **D1.**
  Every source ends in one `GetCertificate`: files by stat,
  self-signed as files the server wrote, autocert as its own.
  One `tls.Config`, one listener, one test of the path.
- **D2.**
  `-tls self-signed` generates at the first start into `<data>/tls/`,
  the one place every service may write; no step, no privilege.
- **D3.**
  ECDSA P-256, 825 days (Apple trusts nothing longer),
  the names of the analysis's section 2: the listen host when it is an address,
  else the hostname, `<hostname>.local`, `localhost`,
  the box's addresses at generation time, and `-tls-names`.
- **D4.**
  No local CA now; DNS-01 gives the trusted LAN certificate.
- **D5.**
  TLS-ALPN-01 first; HTTP-01 only with `-tls-http :80`.
- **D6.**
  DNS-01 through an external ACME client into the files source,
  and the `-acme-dns-hook` over `x/crypto/acme`,
  which the owner moved from a later plan into this one as Block 5 on 2026-11-06,
  to run before Block 3; lego stays out (+1.5 MiB and 231 modules for its core,
  measured).
  The owner's DNS is at Infomaniak, whose API has a certbot plugin
  (`certbot-dns-infomaniak`) and a lego provider, wildcards included:
  the document shows both into the files source,
  and the hook block ships a sample hook against that API.
- **D7.**
  `-trusted-proxies` with addresses and CIDRs, empty by default,
  one `clientIP` for the limiter, the log lines and the first-owner form.
- **D8.**
  Relative URLs in the UI; the proxy strips any prefix;
  the server learns no base path.
- **D9.**
  One `-listen`, plain or TLS, plus the plain helper `-tls-http`.
- **D10, the owner's, against the recommendation.**
  HSTS is on by default with one year;
  `-hsts <days>` changes it and `0` switches it off;
  the header goes out only when the server itself terminates TLS
  (behind a reverse proxy the proxy sets it).
  The consequence, documented in the README and `packaging/README.md`:
  a browser that saw the header refuses `http://` to that host for the remaining days,
  so turning TLS off on a box means running it once with `-hsts 0` over TLS first,
  which sends `max-age=0` and clears it.
- **D11.**
  The health check goes to `https://` without verification for a loopback host:
  the version is no secret and the peer is the same box.
- **D12.**
  The owner's files next to the settings (`/etc/massimo/`,
  `C:\ProgramData\Massimo\`), the server's in `<data>/tls/`,
  the variables in the settings file, nothing in the store.
- **D13.**
  No UPnP now; the reverse-proxy document explains the router's forward,
  and the relay of docs/server.md is the way outside.

**No dynamic loading.**
Go's plugin package needs cgo, has no Windows,
and locks a plugin to the exact toolchain;
the project builds with cgo off and cross-compiles every target.
If binary size ever matters, the way out is a separate signed helper
executable started by the server, as the Windows and macOS update helpers are,
handing certificate files to the server's file reloading of Block 1.
Noted here as the way, not as a block.

**Not in this plan:**
Upnp, a local CA, and a Pebble job in CI;
each a plan of its own if it comes.
Settings → Server as a panel is the plan the UiStructure work left for it;
Block 2 puts the certificate's details into the header's expert list instead.

**Branch and pull requests:**
`claude/tls` from `origin/main` (pull request #50 in it),
one pull request per block,
`origin/main` merged into the branch before each, never rebased.
Nothing is pushed, opened or merged without the owner's signal at a gate,
and auto-merge is never armed.

**Order and the store work:**
another session rewrites `internal/server/auth.go` on `claude/store-b` (users,
tokens, the limiter on the store).
Block 3 touches the same file, so it runs only once that work is on `main`,
after a merge of `origin/main`;
it starts from a `main` that also has #50,
whose partials hold the UI's absolute paths.
The run order is 1, 2, 4, 5 and then 3,
Block 5 before Block 3 by the owner's instruction of 2026-11-06;
a run of several blocks stops at a gate before Block 3 while store (b) is not in.
Block 4's hardware day on the Raspberry Pi, with a real name and
Let's Encrypt's staging directory, is the owner's.

**Who runs it:**
this session, under the owner's signals.

**Names chosen here**, routine and not decisions of the owner's:
the package `internal/certs`;
the owner's files installed as `cert.pem` and `key.pem` beside the settings;
`massimo tls show | self-signed`;
the reverse-proxy document `docs/reverse-proxy.md`,
since `docs/proxy.md` is the Cube proxy and `proxy-tls.md` would read as a part of it.

## Analysis

What changes, by file:

| File | Block | Change |
|------|------:|--------|
| `internal/certs/certs.go` | 1, 2, 4 | new: the `Source` interface (`GetCertificate`, `Status`), `Status` (source, names, expiry, fingerprint, warning), `Config(src)` with TLS 1.2, h2 and http/1.1; the hourly watch |
| `internal/certs/files.go`, `files_test.go` | 1 | new: the files source, stat per handshake, re-read on change, the old pair kept and one log line on a bad one |
| `internal/certs/selfsigned.go`, `selfsigned_test.go` | 2 | new: the generator, the names, the renewal 30 days before expiry, over the files source |
| `internal/certs/acme.go`, `acme_test.go` | 4 | new: autocert with the whitelist, the cache, `acme-tls/1`, the helper handler, the errors logged |
| `internal/certs/dns01.go`, `dns01_test.go` | 5 | new: the DNS-01 flow over `x/crypto/acme` (order, TXT record through the hook, the wait, the certificate into files), the renewal by the watch |
| `packaging/acme-dns-hooks/infomaniak.sh` | 5 | new: the sample hook against Infomaniak's API, `set` and `clear` |
| `cmd/massimo/serve.go`, `serve_test.go` | 1, 2, 3, 4, 5 | `-tls`, `-tls-cert`, `-tls-key`, `-hsts`, `-tls-names`, `-trusted-proxies`, `-acme-email`, `-acme-directory`, `-tls-http`, `-acme-dns-hook`; the source by `-tls`; `ServeTLS`; the helper listener |
| `cmd/massimo/tls.go`, `tls_test.go`, `main.go` | 2 | new: `massimo tls show \| self-signed [-names …]` |
| `cmd/massimo/config.go` | 1, 2 | the TLS lines: mode, files, names, expiry, fingerprint |
| `cmd/massimo/update.go`, `update_test.go` | 1 | `waitHealthy`'s client skips verification for a loopback `https://` host |
| `internal/install/install.go`, `listen.go`, `darwin.go`, `windows.go`, `windows_args.go`, tests | 1, 2, 4 | `-tls`, `-tls-cert`, `-tls-key`, `-tls-names`, `-acme-email`, `-tls-http` on `install`; the files copied; the variables set in the settings file; the health URL's scheme from the file; the capability for `-tls-http` below 1024 |
| `internal/env/env.go` | 1, 2, 3, 4, 5 | the known names: `MASSIMO_TLS`, `_TLS_CERT`, `_TLS_KEY`, `_HSTS`, `_TLS_NAMES`, `_TRUSTED_PROXIES`, `_ACME_EMAIL`, `_ACME_DIRECTORY`, `_TLS_HTTP`, `_ACME_DNS_HOOK` |
| `internal/server/server.go`, `server_test.go` | 1, 3, 4 | `Options.TLS`, `HSTSDays`, `TrustedProxies`; the HSTS header on every response; `/healthz`'s `tls`; `X-Accel-Buffering: no` on the stream |
| `internal/server/auth.go`, `auth_test.go` | 3 | `clientIP` with the trusted proxies, as a method of the server |
| `internal/server/cubes_api.go`, `settings_api.go` | 3 | the log lines on `clientIP` instead of `r.RemoteAddr` |
| `internal/server/ui_test.go` | 3 | `TestUIRelativeURLs`: no embedded UI file holds an absolute path of its own |
| `internal/server/ui/index.html`, `i18n.js`, `app.js`, `cubes.js`, `update.js` | 3 | the 20 absolute paths relative |
| `internal/server/ui/index.html`, `app.js`, `i18n/*.json` | 2, 4 | the header notice (self-signed with the fingerprint; expiring soon), the expert list's certificate row |
| `tools/ui-test/smoke.py` | 2, 4 | the notice checked with a mocked `/healthz` |
| `.github/workflows/ci.yml`, `GUIDELINES/project/CI.md` | 2 | the macOS job installs with `-tls self-signed` and curls `https://` with `-k`; Windows stays plain |
| `packaging/systemd/massimo-update.service`, `packaging/README.md` | 1, 2, 4, 5 | the health URL's scheme; `install -tls …`; the HSTS consequence; the hook and the external clients |
| `README.md` | 1, 2, 3, 4, 5 | "Server": the TLS modes, HSTS, the proxy, DNS-01; "Command line": `serve`'s flags, `tls` |
| `docs/api-v1.md`, `docs/api-v1-openapi.yaml` | 1, 3, 4 | `/healthz`'s `tls` object; the client's address behind a proxy; HSTS |
| `docs/store.md` | 1, 3, 4 | section 4's table: the `MASSIMO_TLS*` and `MASSIMO_ACME_*` variables stay in the file; section 10 item 4: `-trusted-proxies` |
| `docs/ui.md` | 2, 3 | the header notice; the page works under a path prefix |
| `docs/reverse-proxy.md` | 3 | new: nginx and Caddy samples, the Synology clicks, `-trusted-proxies`, the stream, HSTS from the proxy |
| `go.mod`, `go.sum` | 4 | `golang.org/x/crypto` (autocert; Block 5 uses its `acme` package) |
| `CHANGELOG.md` | 1, 2, 3, 4, 5 | a bullet per block under Unreleased |

**What stays:**
plain HTTP by default (`-tls off`), `-listen`'s meaning and default,
the Cube proxy on 62910 and discovery on 23272 plain,
the token in `localStorage`, the CSP and the other security headers,
the stream's 30-second keep-alive, the smoke test's DOM and selectors,
`TestOpenAPICoversRoutes` (no route is added; `massimo tls` is a command),
the update helper's protocol (only its health URL's scheme changes),
and the Windows service's command line (TLS comes from the settings file).

**How the pieces fit:**

- `certs.Source` is what `serve` hands to `server.Options.TLS` and to
  `http.Server.TLSConfig` through `certs.Config`;
  `hs.ServeTLS(ln, "", "")` wraps the listener and sets up HTTP/2.
  `Options.TLS == nil` is the plain server of today.
- The files source holds the pair behind a mutex,
  stats both files in `GetCertificate`,
  re-reads when size or mtime changed, keeps the old pair and logs once
  when the new one does not parse;
  a file missing at the start is an error that stops `serve`.
  The self-signed source is the files source over `<data>/tls/`,
  so `massimo tls self-signed` beside a running server takes effect without a restart.
- `/healthz` gains `tls` (`source`, `names`, `not_after`, `fingerprint`,
  `self_signed`, `warning`) from `Status()`, absent without TLS.
- HSTS is a wrapper around the mux:
  `Strict-Transport-Security: max-age=<days × 86400>` on every response
  while `Options.TLS` is set, `max-age=0` with `-hsts 0`, nothing without TLS.
- The installer learns the scheme from the settings file it keeps (`MASSIMO_TLS`,
  `MASSIMO_TLS_CERT`), the way it keeps `-listen`;
  on Linux the helper's health drop-in is written whenever its content would change,
  on macOS the plist's `@HEALTH@` follows,
  on Windows the server computes the helper's URL itself.
  `install` sets a variable in an existing settings file by replacing its
  line or appending one, atomically, keeping mode and owner;
  that is new behaviour and the dry run says what it would write.
- Behind a proxy, `clientIP` takes the rightmost `X-Forwarded-For` entry
  not in the trusted list when `RemoteAddr` is in it, else `X-Real-IP`,
  else `RemoteAddr`.

**Risks:**

- HSTS: a box that turns TLS off locks browsers out until the days are up;
  the documents say `-hsts 0` over TLS first (D10).
- CI's macOS job updates to the newest alpha release,
  which knows no `-tls` until one with it is cut:
  the helper's `https://` check then fails and the job takes its rollback branch,
  which it accepts; `GUIDELINES/project/CI.md` says so.
- A box whose address changes serves a self-signed certificate without it;
  the server logs the command instead of regenerating (D3).
- Block 3 and store (b) edit `auth.go`: Block 3 waits for the merge.
- Let's Encrypt's rate limits bite in experiments:
  `-acme-directory` points every test at the staging directory,
  and the Pi day is the owner's.
- `x/crypto` adds about 500 KiB and three modules (measured, section 1.2).
- Two files of the UI test and `GUIDELINES/project/CI.md` carry reflow
  warnings that predate this work; the checks are run as they are.

**Measured on 2026-11-06 at `f2a125f`** (the analysis's section 1):
`crypto/tls` is linked already for the updater's client,
serving TLS adds no module;
an ECDSA P-256 key and certificate take 0.47 ms here,
25 to 50 ms on a Pi 1 (estimate); a full TLS 1.3 handshake 1.44 ms,
70 to 150 ms on the Pi; 20 absolute paths in the UI;
`healthURL` hard-codes `http://`; `clientIP` is the host of `RemoteAddr`.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`),
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin, `make cross`,
`make ui-test` where a file under `internal/server/ui/` changed,
and `GUIDELINES/shared/tools/check`.
Test certificates are generated in the tests, never committed;
names and addresses in tests and documents are placeholders (`massimo.example.net`,
`192.0.2.10`).
Nothing reaches a Cube, the network beyond module downloads,
or the owner's Windows service.
Each block is one or more functional commits citing `AP Tls v1.0, Block <n>`,
each with its CHANGELOG bullet, and one pull request,
with `origin/main` merged into the branch first.

### Block 1: TlsFiles

1. `internal/certs/certs.go`:
   `type Source interface { GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error); Status() Status }`;
   `Status{Source, Names []string, NotAfter time.Time, Fingerprint, SelfSigned bool, Warning string}`;
   `Config(src Source) *tls.Config` with `MinVersion: tls.VersionTLS12`,
   `NextProtos: h2, http/1.1`, `GetCertificate: src.GetCertificate`;
   `Fingerprint(der []byte) string` as `SHA256:` and colon-separated hex,
   the form browsers show;
   `Leaf(cert *tls.Certificate)` parses the leaf once for the status.
2. `internal/certs/files.go`:
   `Files(certPath, keyPath string, log *slog.Logger) (*FilesSource, error)`
   loads the pair with `tls.X509KeyPair` (the certificate file carrying the
   chain as ACME clients write it), remembers size and mtime of both,
   and in `GetCertificate` stats both, re-reads when either changed,
   keeps the old pair and logs once when the new one does not parse
   (again after it changes again); `Status()` from the leaf.
3. `cmd/massimo/serve.go`:
   `-tls` (`MASSIMO_TLS`; `off`, `files`; `files` implied by `-tls-cert`;
   the later modes are refused with the block that adds them named),
   `-tls-cert` and `-tls-key` (`MASSIMO_TLS_CERT`, `MASSIMO_TLS_KEY`,
   both required for `files`), `-hsts <days>` (`MASSIMO_HSTS`, default 365);
   the source built from the flags, `server.Options.TLS` and `HSTSDays`,
   `hs.TLSConfig = certs.Config(src)` and `hs.ServeTLS(ln, "", "")`;
   the start line logs `tls`, `names`, `expires` and `fingerprint`;
   the usage text names the modes.
4. `internal/server/server.go`:
   `Options.TLS certs.Source` and `HSTSDays int`;
   `Handler()` returns the mux wrapped so that every response carries
   `Strict-Transport-Security: max-age=<days×86400>` while `TLS` is set,
   `max-age=0` for zero days, nothing without TLS;
   `/healthz` adds `tls` from `Status()`.
5. The health URL: `install.healthURL(host, port string, tls bool)` emits the scheme;
   `install.HealthURL(listen string, tls bool)`;
   `listenDropIns(addr string, tls bool)`;
   `launchdPlists(listen string, tls, noHelper bool)`;
   the installer reads `MASSIMO_TLS` and `MASSIMO_TLS_CERT` from the settings
   file it keeps (or copies) to know the scheme,
   and the Linux health drop-in is written whenever `-listen` or TLS is in
   effect and its content differs, from the effective address (the flag,
   else the kept drop-in's `MASSIMO_LISTEN`, else `:8099`);
   `serveUpdateHelper` on Windows passes the server's own TLS state.
   `packaging/systemd/massimo-update.service`'s comment says the drop-in
   carries the scheme.
6. `cmd/massimo/update.go`:
   `waitHealthy` builds its client with `InsecureSkipVerify` when the URL is
   `https://` on a loopback host (`127.0.0.1`, `::1`, `localhost`),
   verifying any other host as today.
7. `install -tls <mode> -tls-cert <pem> -tls-key <pem>`:
   the files copied as `cert.pem` and `key.pem` beside the settings (`/etc/massimo/`,
   `C:\ProgramData\Massimo\`), mode 0600,
   owned by the service user (the directory's ACL on Windows);
   `MASSIMO_TLS=files`,
   `MASSIMO_TLS_CERT` and `MASSIMO_TLS_KEY` set in the settings file (`setEnvLines`:
   a line replaced or appended, written atomically, mode and owner kept);
   `-tls off` sets `MASSIMO_TLS=off` and leaves the files;
   a later `install` without the flags keeps them, as it keeps `-listen`;
   `checkSettings` warns when the files the settings name are not readable;
   the dry run says what it would copy and write.
8. `internal/env/env.go`:
   the four names in `Known`; `cmd/massimo/config.go` prints the TLS mode,
   the files, and the certificate's names,
   expiry and fingerprint when it can read it.
9. Tests: `files_test.go` generates an ECDSA pair in the test, serves it,
   rewrites it with a new pair (`os.Chtimes` for the mtime) and sees the new fingerprint,
   writes garbage and keeps the old pair with one log line,
   refuses a missing file at the start; `server_test.go`:
   the HSTS header with 365 days, with 0, and absent without TLS;
   `/healthz`'s `tls` object; `cmd/massimo`:
   a `TestServeTLS` that starts the handler on the listener `serve` builds
   and reads `/healthz` over `https://` with a client that skips verification;
   `TestWaitHealthyLoopback` against `httptest.NewTLSServer`,
   and a non-loopback `https://` host still verified; `internal/install`:
   `TestListenDropIns` and `TestLaunchdPlists` with `https://` cases,
   `TestInstallTLSFiles` (the copies, the mode, the variables,
   the health drop-in's scheme, kept across a second install),
   `TestWindowsLayout` with the scheme.
10. The documents:
    README.md "Server" (the TLS modes, HSTS and its consequence,
    the reverse proxy sentence) and "Command line" (`serve`'s flags);
    `packaging/README.md` (`install -tls-cert …`, the helper's health URL,
    a deploy hook copying certbot's files, `-hsts 0` before turning TLS off);
    `docs/api-v1.md` and the OpenAPI file (`/healthz`'s `tls`, HSTS);
    `docs/store.md` section 4's table (`MASSIMO_TLS*` stay in the file).
11. The CHANGELOG bullet under Added:
    `massimo serve -tls files -tls-cert … -tls-key …` serves the API and
    web UI over TLS from certificate files, re-read when they change,
    with HSTS for a year (`-hsts <days>`, `0` off);
    `massimo install -tls-cert … -tls-key …` installs them.

**Verification:**
the test gate; `curl -ksI https://127.0.0.1:8099/` against a `serve -tls-cert`
with a test pair shows `Strict-Transport-Security: max-age=31536000`
and `curl -ks https://127.0.0.1:8099/healthz` the `tls` object;
a rewritten pair is served at the next handshake without a restart;
`install -root <dir> -tls-cert … -tls-key …` leaves `cert.pem`,
`key.pem` 0600 and the three lines in `env`,
and the health drop-in says `https://`; CI's Windows and macOS jobs stay green,
plain.

### Block 2: TlsSelfSigned

1. `internal/certs/selfsigned.go`:
   `SelfSigned(dir string, names []string, log) (*FilesSource, error)`
   over `<data>/tls/self-signed.pem` and `self-signed.key` (directory 0700,
   files 0600): generated when missing or when `NotAfter` is within 30 days,
   ECDSA P-256, 825 days, `NotBefore` an hour back for clock skew,
   a random serial, `CN` the hostname, key usage digital signature,
   extended key usage server auth, the SANs from `Names(listenHost, extra)`:
   the listen host when it is a specific address, the hostname and its
   first label with `.local`, `localhost`, `127.0.0.1`, `::1`,
   the box's unicast addresses that are neither loopback nor link-local,
   and `-tls-names` (`MASSIMO_TLS_NAMES`, comma-separated, names or addresses);
   the new fingerprint logged at generation.
   `Status().SelfSigned` is true.
   A listen host or a box address missing from an existing certificate is
   logged once at the start with the command to run, never regenerated.
2. `internal/certs/certs.go`:
   `Watch(ctx, src, every, log)` ticks hourly and calls `src.(Checker).Check(now)`;
   the self-signed source regenerates 30 days before expiry and logs the new fingerprint;
   `serve` starts it beside `srv.Run`.
3. `cmd/massimo/tls.go`, `main.go`:
   `massimo [-data <dir>] tls show` prints source, files, names, expiry and
   fingerprint for the mode the settings name (`off`: "serve has no TLS");
   `massimo tls self-signed [-names <list>]` writes a new pair into
   `<data>/tls/` (run as the service user, like `store`;
   a running server takes it up through the stat reload);
   the usage of `massimo` lists `tls`.
4. `cmd/massimo/serve.go` and `install.go`: `-tls self-signed`,
   `-tls-names`; `install -tls self-signed [-tls-names …]` sets
   `MASSIMO_TLS=self-signed` and `MASSIMO_TLS_NAMES` in the settings file;
   the server generates at its first start, as the service user,
   into the state directory it may write.
5. The UI: `index.html` gains a line under the header, hidden by default,
   `data-i18n` key `header.selfSigned` with the fingerprint in `mono`;
   `app.js`'s `checkVersion` shows it when `/healthz`'s `tls.self_signed` is true;
   the expert list gains a row "Certificate" (`budget.certificate`) with source,
   names and expiry;
   the keys in all six `i18n/*.json` (the machine-translated ones marked as the
   others are).
   `smoke.py` mocks `/healthz` with a `tls` object through `page.route`
   and checks the notice's text and that it is hidden without one;
   no pixel changes without TLS, so the screenshot set stays.
6. CI's macOS job: `install … -tls self-signed`, `health=https://…`,
   `curl -fsSk`, the `POST /api/v1/update` over `https://` with `-k`;
   it checks the helper's plist carries `https://`, that
   `/var/lib/massimo/tls/self-signed.key` is `_massimo:600`,
   and accepts the rollback branch as today;
   `GUIDELINES/project/CI.md` section 2 says the job runs on a self-signed
   certificate and section 7 why the update rolls back until a release
   with `-tls` is on the alpha channel;
   its version raised with its `File-revision-v1` trailer.
7. `cmd/massimo/config.go` prints the self-signed fingerprint; the documents:
   README.md "Server", `packaging/README.md` (the fingerprint to compare,
   the warning once per device, `massimo tls self-signed` after a move),
   `docs/ui.md` "Header", `docs/api-v1.md` (`self_signed`).
8. Tests: the generator (key type, validity,
   the SANs including the given names and addresses, the files' modes),
   a second start reuses the pair (same fingerprint),
   a pair expiring within 30 days is regenerated with an injected clock,
   `massimo tls show` and `self-signed` on a temporary data directory,
   a server reading a regenerated pair without a restart,
   `/healthz`'s `self_signed`, the smoke test's notice check,
   `TestUITranslations` for the new keys.
9. The CHANGELOG bullet under Added:
   `massimo serve -tls self-signed` makes a certificate for the box's names
   into the data directory's `tls/` at the first start and renews it 30 days
   before it expires; its fingerprint is in the log, `massimo config`,
   `massimo tls show`, `/healthz` and the web UI's header.

**Verification:**
the test gate and `make ui-test` at 1100 and 390 px;
`serve -tls self-signed` against the fake Cube on a temporary data
directory logs the fingerprint, `massimo tls show` prints the same one,
and a browser on this machine shows the page after its one warning with
the notice carrying it (the owner's check on a phone is noted, not required);
CI's macOS job green on `https://`.

### Block 3: TlsProxy

Starts from a `main` that holds store stage (b) and #50,
merged into the branch first; whatever store (b) made of `auth.go`
(the limiter on the store, the first-owner form's local-network check)
is moved onto the one `clientIP`.

1. `cmd/massimo/serve.go`:
   `-trusted-proxies` (`MASSIMO_TRUSTED_PROXIES`),
   addresses and CIDRs separated by commas, parsed with `netip`;
   `server.Options.TrustedProxies []netip.Prefix`.
2. `internal/server/auth.go`:
   `clientIP` becomes `(s *Server) clientIP(r)`:
   `RemoteAddr` not in the list → `RemoteAddr`'s host;
   in it → the rightmost `X-Forwarded-For` entry not in the list,
   else `X-Real-IP`, else `RemoteAddr`;
   an entry that is not an address is skipped; the limiter,
   the lockout log line, the log lines of `cubes_api.go` and `settings_api.go`,
   and store (b)'s local-network check use it.
3. `internal/server/server.go`:
   `X-Accel-Buffering: no` on the event stream.
4. The UI's 20 absolute paths relative: `index.html` (`ui/style.css`,
   the nine scripts), `i18n.js` (`ui/i18n/<lang>.json`),
   `app.js` (`api/v1/cubes…`, `healthz`, the stream's URL), `cubes.js`,
   `update.js`;
   `TestUIRelativeURLs` reads every embedded UI file and fails on a quoted
   or attribute path starting with `/ui/`, `/api/` or `/healthz`.
   `smoke.py`'s `**/api/v1/…` globs match either form and stay.
5. `docs/reverse-proxy.md`:
   why a proxy, the nginx and Caddy samples of the analysis's section 5 (HTTP/1.1,
   an empty `Connection`, `X-Forwarded-For`, `X-Real-IP`, the prefix stripped),
   the Synology and QNAP clicks,
   `-trusted-proxies 127.0.0.1` on the box and a proxy on another host with
   `-listen` on a LAN address, HSTS set by the proxy, the stream;
   `docs/api-v1.md` "Authentication" (the client's address behind a proxy),
   `docs/store.md` section 10 item 4 (the form offered to LAN clients behind a
   trusted proxy), `docs/ui.md` "How the page is built" (every URL is relative;
   the page works under a prefix), README.md "Server" (the document linked).
6. The CHANGELOG bullet under Added:
   `massimo serve -trusted-proxies <addresses or CIDRs>` takes the client's
   address from `X-Forwarded-For` or `X-Real-IP` behind a reverse proxy,
   for the lockout and the log; the event stream asks nginx not to buffer;
   the web UI's URLs are relative, so it works under a path prefix.

**Verification:**
the test gate and `make ui-test` at 1100 and 390 px in English and German;
`TestClientIP` (no trust, a trusted peer with a chain, every hop trusted,
`X-Real-IP`, a bad entry),
the limiter locking one forwarded client and not another,
`TestEventsNoBuffering`, `TestUIRelativeURLs`;
`grep -n '"/ui/\|"/api/\|"/healthz\|="/' internal/server/ui/*` finds nothing;
the page opened through a Caddy on this machine at `/massimo/` (the sample,
Caddy downloaded for the purpose, not committed) loads, polls and streams.

### Block 4: TlsAcme

1. `go.mod`:
   `golang.org/x/crypto` (autocert); `go mod tidy`;
   the size delta per platform measured by `make cross` and noted in the commit message.
2. `internal/certs/acme.go`:
   `ACME(dir, names, email, directory string, log) (*ACMESource, error)` with
   `autocert.Manager{Prompt: autocert.AcceptTOS, Cache: autocert.DirCache(<data>/tls/acme), HostPolicy: autocert.HostWhitelist(names…), Email, Client: &acme.Client{DirectoryURL: directory}}`; `GetCertificate` wraps the manager's, logs an error once per name per ten minutes (autocert logs nothing) and remembers the certificate served for `Status()`; `Config` adds `acme-tls/1` to `NextProtos` for this source; `HTTPHandler()` is the manager's `HTTPHandler(nil)` (the challenge path and the redirect to `https://`); `-tls-names` must be DNS names here, an address is refused.
3. The watch: `Check(now)` of the files, self-signed and ACME sources sets
   `Status().Warning` ("expires in N days") under 14 days and logs it hourly;
   `/healthz` carries it; the header's notice shows it (`header.tlsWarning`,
   the six JSON files, `smoke.py`'s mocked `/healthz` covers it).
4. `cmd/massimo/serve.go`:
   `-tls acme`, `-acme-email` (`MASSIMO_ACME_EMAIL`),
   `-acme-directory` (`MASSIMO_ACME_DIRECTORY`,
   default Let's Encrypt's production directory;
   the staging one for every experiment),
   `-tls-http <addr>` (`MASSIMO_TLS_HTTP`):
   a plain `http.Server` on that address with the ACME `HTTPHandler`,
   started beside the TLS server and shut down with it;
   without it TLS-ALPN-01 alone.
   `install -tls acme -tls-names … [-acme-email …] [-tls-http :80]` sets the variables;
   `listenDropIns` writes `CAP_NET_BIND_SERVICE` when either port is below 1024;
   `checkDarwinListen` covers `-tls-http` too.
5. Tests: the `tls.Config`'s `NextProtos` holds `acme-tls/1`;
   the helper handler answers `/.well-known/acme-challenge/<token>` from the
   manager (404 for an unknown token) and redirects everything else to
   `https://` with host and path kept (302 for GET and HEAD);
   an address in `-tls-names` is refused;
   the warning under 14 days with an injected clock,
   in `/healthz` and the smoke test; `TestListenDropIns` with `-tls-http :80`;
   the cache directory's mode.
   No test reaches Let's Encrypt; a Pebble job is later.
6. The documents: README.md "Server" (a name, the forwarded 443, the rate
   limits in a sentence, the staging directory), `packaging/README.md`
   (`install -tls acme …`, the router's forward, dynamic DNS pointed at,
   HTTP-01 with `-tls-http :80`), `docs/api-v1.md` (`warning`),
   `docs/store.md` section 4's table, `docs/ui.md` "Header".
7. The CHANGELOG bullet under Added:
   `massimo serve -tls acme -tls-names <name> [-acme-email …]` gets and
   renews a Let's Encrypt certificate (TLS-ALPN-01 on the forwarded port 443;
   `-tls-http :80` for HTTP-01 and the redirect from `http://`),
   kept under the data directory's `tls/acme/`;
   the web UI warns two weeks before a certificate expires.

**Verification:**
the test gate, `make cross` with the sizes noted, `make ui-test`;
`serve -tls acme -tls-names massimo.example.net -acme-directory <staging>` on
this machine starts,
logs the order's failure from the unreachable name once and keeps serving a
challenge-less handshake failure rather than crashing;
the real certificate from the staging directory on the Pi with a name is the owner's day,
and its outcome goes into the completion gate's NOTE.

### Block 5: TlsDns

After Block 4, whose module, cache directory, watch and notice it shares;
before Block 3.

1. `internal/certs/dns01.go`:
   `DNS01(dir string, names []string, email, directory, hook string, log)` over
   `<data>/tls/acme/<first name>.pem` and `.key`,
   so that the certificate is served through the files source of Block 1:
   the account key `<data>/tls/acme/acme_account+key` as autocert names it,
   shared with Block 4, made when missing;
   when the files are missing or expire within 30 days,
   the flow of `x/crypto/acme`:
   `AuthorizeOrder` for every name (a wildcard `*.example.net` and `example.net`
   in one order), for each authorization the `dns-01` challenge,
   `DNS01ChallengeRecord`,
   the hook run as `<hook> set _acme-challenge.<name> <txt>`,
   a wait until the system resolver returns the record (`net.LookupTXT`,
   every 10 s, 10 minutes at most), `Accept`, `WaitAuthorization`,
   `clear` through the hook whatever the outcome, then a CSR with every name,
   `CreateOrderCert`, the chain and the key written atomically,
   and the files source told to reload;
   a failure is logged and tried again by the watch an hour later,
   the previous certificate kept.
   The hook gets the environment of the server without the `MASSIMO_` variables,
   as the Windows update helper does, and a 2-minute timeout;
   its output goes to the log.
2. `cmd/massimo/serve.go`:
   `-acme-dns-hook <path>` (`MASSIMO_ACME_DNS_HOOK`) with `-tls acme` selects
   DNS-01 instead of TLS-ALPN-01, without the helper listener;
   `-tls-names` may hold wildcards here;
   `install -tls acme -acme-dns-hook …` sets the variable;
   the hook file must be executable by the service user, which `install` checks.
3. `packaging/acme-dns-hooks/infomaniak.sh`:
   the sample hook, POSIX shell with `curl`,
   against Infomaniak's public API with the token in `INFOMANIAK_API_TOKEN` read
   from a file the hook names (`/etc/massimo/infomaniak.token`, mode 0600):
   `set` finds the zone of the name, adds the TXT record, prints its id;
   `clear` removes the records of that name and value.
   The calls are those `certbot-dns-infomaniak` makes;
   the script is run against a fake API in a Go test and marked untested
   against the real one until the owner's day.
4. The watch renews 30 days before expiry through the same flow;
   the warning under 14 days of Block 4 covers it.
5. Tests: the flow against a fake ACME directory over `httptest`
   (the `x/crypto/acme` package's own test server pattern:
   a directory, a new account, an order with a `dns-01` challenge,
   a finalize and a certificate), with a hook script that records its
   calls in a file and a resolver seam that answers the TXT lookup;
   the hook's `clear` on a failed authorization;
   the certificate served after the flow without a restart;
   `install` refusing a hook that is not executable;
   the Infomaniak script against a fake API with the plugin's responses.
   No test reaches Let's Encrypt or Infomaniak.
6. The documents: README.md "Server" (DNS-01: a name, no open port,
   the trusted certificate on the LAN);
   `packaging/README.md` (the hook and the sample,
   Infomaniak's token with the Domain scope,
   the zone must be hosted at Infomaniak's name servers; the external route:
   `certbot certonly --authenticator dns-infomaniak` with the credentials file
   and a deploy hook copying into `-tls files`,
   the renewal service needing the token, and `lego --dns infomaniak`);
   `docs/store.md` section 4's table.
7. The CHANGELOG bullet under Added:
   `massimo serve -tls acme -acme-dns-hook <script>` gets and renews a Let's
   Encrypt certificate by DNS-01 through a hook that sets the TXT record,
   wildcards included, with no open port;
   `packaging/acme-dns-hooks/infomaniak.sh` is the hook for Infomaniak's DNS.

**Verification:**
the test gate; the fake-directory test issues a certificate with a
wildcard and the server serves it;
the real certificate from Let's Encrypt's staging directory through the
owner's Infomaniak token is the owner's day, with Block 4's, and its outcome
goes into the completion gate's NOTE.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +Tls v1.0, +ANALYSIS Tls

TLS for massimo serve (certificate files re-read when they change, a self-signed certificate the server makes, the reverse-proxy case, Let's Encrypt through autocert) needs its plan registered before the first project write, and the analysis it rests on comes with it.

Key changes:
- GUIDELINES/wip/2026-11-06_12-42_AP_Tls_serve_tls.md: the plan, four blocks, the owner's decisions D1 to D13 in its Discussion, D10 the owner's.
- GUIDELINES/wip/2026-11-06_11-46_ANALYSIS_Tls_serve_tls_support.md: the analysis, copied from messages/ with its timestamp.

File-revision-v1: GUIDELINES/wip/2026-11-06_12-42_AP_Tls_serve_tls.md::v1.0::2026-11-06::All::the first revision
File-revision-v1: GUIDELINES/wip/2026-11-06_11-46_ANALYSIS_Tls_serve_tls_support.md::v1::2026-11-06::All::the analysis the plan rests on
Assisted-by: claude-code:claude-fable-5-1
```

The blocks' commits, each written at its gate:

1. `feat(serve): TLS from certificate files, re-read when they change`
2. `feat(serve): a self-signed certificate made at the first start`
3. `feat(server): the client's address behind a trusted proxy`
4. `feat(serve): Let's Encrypt through autocert`
5. `feat(serve): Let's Encrypt by DNS-01 through a hook`

## Change History

- v1.1 (2026-11-06): Block 5 TlsDns added on the owner's instruction,
  the `-acme-dns-hook` flow over `x/crypto/acme` with an Infomaniak sample
  hook and the certbot and lego recipes, to run before Block 3;
  D6, the run order and the file table updated.
- v1.0 (2026-11-06): the first revision, from the analysis's section 9 and
  the owner's decisions D1 to D13 of the same day.

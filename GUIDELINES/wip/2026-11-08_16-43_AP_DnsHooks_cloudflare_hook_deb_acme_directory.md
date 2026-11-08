# AP DnsHooks v1.0: Cloudflare hook, hooks in the deb

## TL;DR

A DNS-01 hook for zones at Cloudflare beside Infomaniak's,
both hooks in the Debian package, and `massimo install -acme-directory`,
so that a box installed from apt gets a Let's Encrypt certificate by DNS-01
with the packaged files alone, from the staging directory first.

- `packaging/acme-dns-hooks/cloudflare.sh` → POSIX shell, `curl`, `jq`, API v4;
  token from `CLOUDFLARE_TOKEN_FILE`; `CLOUDFLARE_ZONE_ID` skips the zone lookup
- its fake-API test beside Infomaniak's: the token never on curl's command line,
  a time limit on every call
- `infomaniak.sh` → "tested against the real API on 2026-11-08", not "untested"
- deb → both hooks in `/usr/share/massimo/acme-dns-hooks/`, 0755;
  the container test checks them
- `install -acme-directory <url>` → `MASSIMO_ACME_DIRECTORY`,
  removed by a switch to another mode as the other ACME variables are
- README.md "Server", packaging/README.md; the roadmap's wording is the lead's

### Blocks

1. CloudflareHook: the script, its test, the Infomaniak comment, the documents
2. DebHooks: the hooks in the deb, the container test, the documents
3. InstallDirectory: `install -acme-directory`, its tests, the documents

## Discussion

The owner's checks of AP Tls and AP Packaging on 2026-11-08 found two gaps,
and the owner decided at that run's gates:

- **D6-A.**
  The two findings go into one small plan of their own:
  `massimo install` takes no `-acme-directory`,
  so a first try against the staging directory through `install` means editing
  the settings file by hand (the server orders at its start,
  so a forgotten line orders from the production directory),
  and the deb carries no DNS-01 hook,
  so a box installed from apt has none to name in `-acme-dns-hook`.
- **D7-B.**
  The test of Let's Encrypt's staging directory by DNS-01 runs on a box whose
  zone is at Cloudflare.
  Its hook is a sample shipped beside Infomaniak's,
  with a test against a fake API, made in this plan;
  that check waits for this plan's merge,
  the file copied from `main` onto the box.
- **The run's order**, given with `PLAN++`:
  the plan committed on top of `main`, the work branch from there,
  the retirement on the branch after the completion gate,
  then the push and the pull request.

**Measured on 2026-11-08:**
`infomaniak.sh set` and `clear` against Infomaniak's real API,
on a box whose acme.sh holds the token,
the token handed over in a root-only file and never read:
the record was served by both of the zone's name servers 10 s after `set`,
and gone from both 10 s after `clear`.
The box had to be given `jq`.

**Names chosen here**, routine and not decisions of the owner's:
`CLOUDFLARE_TOKEN_FILE` (default `/etc/massimo/cloudflare.token`),
`CLOUDFLARE_ENDPOINT` (default `https://api.cloudflare.com/client/v4`),
`CLOUDFLARE_ZONE_ID`, `CLOUDFLARE_TTL` (default 120),
after the `INFOMANIAK_*` names of the other hook;
`/usr/share/massimo/acme-dns-hooks/`,
as Debian's acme.sh keeps its scripts under `/usr/share/acme.sh/`.

**Not in this plan:**
the hooks in the Homebrew formula, further providers,
and the roadmap's text for TLS (1.2), which the coordinating session keeps;
the completion gate hands it the wording.

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-08.

## Analysis

What changes, by file:

| File | Block | Change |
|------|------:|--------|
| `packaging/acme-dns-hooks/cloudflare.sh` | 1 | new: `set` and `clear` against Cloudflare's API v4 |
| `internal/certs/hooks_test.go` | 1 | `TestCloudflareHook` with a fake of the zones and DNS records routes |
| `packaging/acme-dns-hooks/infomaniak.sh` | 1 | its comment: run against the real API on 2026-11-08 |
| `packaging/nfpm.yaml` | 2 | the two hooks into `/usr/share/massimo/acme-dns-hooks/`, mode 0755 |
| `packaging/deb/test.sh` | 2 | the hooks installed and executable |
| `cmd/massimo/install.go` | 3 | `-acme-directory`, the usage line |
| `internal/install/install.go`, `tls.go`, `install_test.go` | 3 | `Options.ACMEDirectory`, the variable written, in `modeVars`, the URL checked |
| `README.md` | 1, 3 | "Server": the Cloudflare hook; `install -acme-directory` |
| `packaging/README.md` | 1, 2, 3 | the hooks, their tokens and the packaged path; the staging directory through `install` |
| `CHANGELOG.md` | 1, 2, 3 | a bullet per block under Unreleased, Added |

**The Cloudflare hook**, the calls lego's and certbot's Cloudflare plugins make:

- the token as `Authorization: Bearer`,
  handed to `curl` on its standard input (`-K -`), as `infomaniak.sh` does;
- the zone: `CLOUDFLARE_ZONE_ID`,
  else `GET /zones?name=<candidate>` walking up the record's labels,
  the first with a result;
  a token limited to Zone → DNS → Edit of one zone may not list it,
  and then `CLOUDFLARE_ZONE_ID` names it;
- `set`:
  `POST /zones/<id>/dns_records` with `type` TXT, the full `name`,
  `content` and `ttl`, printing the record's id;
- `clear`:
  `GET /zones/<id>/dns_records?type=TXT&name=<record>`,
  then `DELETE /zones/<id>/dns_records/<rid>` for each whose content is the
  value,
  quoted or not (Cloudflare has asked for quoted TXT content since 2024
  and keeps what it is given);
- an answer with `success` false: its `errors[].message` on standard error,
  exit 1.

**What stays:**
the server's DNS-01 flow and the hook's calling convention
(`<hook> set|clear <record> <value>`, the server's environment less the
`MASSIMO_` variables, two minutes per run),
`infomaniak.sh`'s calls, the deb's other files and scripts,
and every other `install` flag.

**Risks:**

- No test reaches Cloudflare: the fake answers as the API's documentation says;
  the check of AP Tls on a real zone is the first real run.
- A file under `/usr/share` that is executable could draw a lintian tag;
  the package's two known errors are documented (packaging/README.md),
  and a new one is noted in the completion gate.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
and `GUIDELINES/shared/tools/check`.
Names in tests and documents are placeholders (`massimo.example.net`,
`example.net`).
Nothing reaches a Cube, a box, Cloudflare or Let's Encrypt.
Each block is one functional commit citing `AP DnsHooks v1.0, Block <n>`,
with its CHANGELOG bullet.

### Block 1: CloudflareHook

1. `packaging/acme-dns-hooks/cloudflare.sh`:
   the header comment as `infomaniak.sh`'s (usage, the calls, the variables,
   the token on standard input, 30 seconds per call), `set -eu`,
   the `api` and `ok` functions over Cloudflare's envelope (`success`, `result`,
   `errors`), the zone, `set` and `clear` as above.
2. `internal/certs/hooks_test.go`:
   `fakeCloudflare` answering `GET /zones?name=`, `POST` and `GET`
   `/zones/<id>/dns_records` and `DELETE …/<rid>` for one zone,
   `example.net`, refusing another token;
   `TestCloudflareHook` runs the script as `TestInfomaniakHook` runs its own,
   through the recording curl:
   `set` below the zone and at its apex (the full name sent),
   `clear` of one name and value leaving the other,
   `CLOUDFLARE_ZONE_ID` without a lookup, a name in no zone, a wrong token;
   no call with the token on its command line or without `--max-time`.
3. `infomaniak.sh`'s comment: run against the real API on 2026-11-08,
   `set` and `clear`, beside the fake of the test.
4. The documents: README.md "Server" names both hooks; packaging/README.md,
   beside the Infomaniak paragraph: the Cloudflare hook,
   its token (Zone → DNS → Edit,
   the zone named in `CLOUDFLARE_ZONE_ID` where the token cannot list it),
   the same three-line install;
   the "untested" sentence becomes the date of the run.
5. The CHANGELOG bullet under Added:
   `packaging/acme-dns-hooks/cloudflare.sh` is the DNS-01 hook of
   `serve -acme-dns-hook` for a zone at Cloudflare,
   with a token for the zone's DNS in `/etc/massimo/cloudflare.token`.

**Verification:**
the test gate; `TestCloudflareHook` and `TestInfomaniakHook` pass,
and skip only where `sh`, `curl` or `jq` is missing;
`shellcheck` and `sh -n` on the script.

### Block 2: DebHooks

1. `packaging/nfpm.yaml`:
   `packaging/acme-dns-hooks/infomaniak.sh` and `cloudflare.sh` into
   `/usr/share/massimo/acme-dns-hooks/`, mode 0755.
2. `packaging/deb/test.sh`:
   both installed, executable, and run by `sh -n`;
   gone after the purge with the rest of `/usr/share/massimo`.
3. The documents: packaging/README.md "Debian package" lists them,
   and the DNS-01 paragraph names
   `/usr/share/massimo/acme-dns-hooks/<provider>.sh` on a box installed from
   apt, the copy into `/usr/local/lib/massimo/` elsewhere.
4. The CHANGELOG bullet under Added:
   the Debian package installs the DNS-01 hooks for Infomaniak and Cloudflare
   in `/usr/share/massimo/acme-dns-hooks/`.

**Verification:**
the test gate; `make packages` builds the debs and `dpkg-deb -c` lists the hooks
with mode 0755;
`make deb-test` (or the container test as CI runs it) passes where the container
runtime is at hand, else CI's Debian jobs are the proof.

### Block 3: InstallDirectory

1. `internal/install/install.go`:
   `Options.ACMEDirectory`, with `ACMEEmail`'s comment.
2. `internal/install/tls.go`:
   `acmeDirectory = env.ACMEDirectory`, in `modeVars`;
   written with `-tls acme` beside the e-mail;
   `checkTLS` refuses a value that is not an absolute `http` or `https` URL.
3. `cmd/massimo/install.go`:
   `-acme-directory <url>` with its help text (Let's Encrypt's staging directory
   for a first try), the usage line.
4. Tests: `MASSIMO_ACME_DIRECTORY` written with `-tls acme`,
   removed by `-tls off`, kept by an install without `-tls`,
   a value that is not a URL refused.
5. The documents: packaging/README.md's `-tls acme` sentence and its example
   with the staging directory; README.md "Server" says `install` takes it too.
6. The CHANGELOG bullet under Added:
   `massimo install -acme-directory <url>` sets the ACME directory,
   such as Let's Encrypt's staging one for a first try.

**Verification:**
the test gate; `massimo install -root <dir> -tls acme -tls-names massimo.example.net
-acme-directory <staging URL> -dry-run` says it would set the variable.

## Proposed commit message

The plan's registration, a commit of its own before the first project write:

```
chore(aps): +DnsHooks v1.0

A box installed from apt cannot get a certificate by DNS-01 with what the package ships, nor point massimo install at Let's Encrypt's staging directory, and the owner's test on a zone at Cloudflare needs a hook for it; the plan is registered on top of main before the work branches off.

Key changes:
- GUIDELINES/wip/2026-11-08_16-43_AP_DnsHooks_cloudflare_hook_deb_acme_directory.md: the plan, three blocks, the owner's D6-A and D7-B in its Discussion.

File-revision-v1: GUIDELINES/wip/2026-11-08_16-43_AP_DnsHooks_cloudflare_hook_deb_acme_directory.md::v1.0::2026-11-08::All::the first revision, by PLAN++
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:

1. `feat(packaging): A DNS-01 hook for zones at Cloudflare`
2. `feat(packaging): The Debian package ships the DNS-01 hooks`
3. `feat(install): -acme-directory names the ACME directory`

## Change History

- v1.0 (2026-11-08): the first revision, from the findings of the owner's
  checks of AP Tls and AP Packaging and the decisions D6-A and D7-B of the same day.

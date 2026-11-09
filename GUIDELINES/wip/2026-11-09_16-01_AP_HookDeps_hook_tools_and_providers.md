# AP HookDeps v1.0: hook tools and providers

## TL;DR

The two DNS-01 hooks say when `curl` or `jq` is missing,
and the Debian package suggests both;
for every other DNS provider the documents point at an ACME client that knows it,
writing into `-tls files`, instead of a script per provider.

- a hook without `jq` → "no zone found" or "not an answer of the API";
  → "jq is not installed", before any call
- the package → `Suggests: curl, jq`; the deb test checks the field
  and runs each hook in the slim image, which has neither
- README's "any other provider's API fits in a dozen lines" → acme.sh,
  lego or certbot into `-tls-cert` and `-tls-key`,
  with acme.sh's `--install-cert` into Massimo's files
- no start check: `-tls acme` without a hook needs neither tool

### Blocks

1. Tools: the check in both hooks, its test, `Suggests:`,
   the deb test, the documents, the CHANGELOG
2. Providers: README and `packaging/README.md` on other DNS providers
   through an ACME client into `-tls files`

## Discussion

The owner's checks of AP Tls on 2026-11-08 and 2026-11-09 found `jq` missing on
the Raspberry Pi 5 that ran DNS-01 through the Cloudflare hook:
the package ships both hooks under `/usr/share/massimo/acme-dns-hooks/`
and names neither `curl` nor `jq`,
and without them a hook fails with an error that names neither.

The owner asked whether `jq` is required for TLS, and whether the start could
fail under `-tls acme` without it.
It is not: only the two shipped hooks use it,
`-tls acme` by a port needs no tool,
and a hook of the owner's own may need others.
So the hook says it, where the need is.

The owner then asked whether a script is needed for every provider,
since ACME clients already ship them.
They do: acme.sh has DNS API scripts for well over a hundred providers,
lego and certbot's plugins as many;
any of them can write the files that `-tls files` reads,
and the server re-reads them when they change.

**The owner's decisions**, 2026-11-09, `EXEC D14-A D15-A`:

- **D14-A.**
  The hooks check for `curl` and `jq`, and the package suggests them;
  the start does not check.
- **D15-A.**
  The two hooks stay as they are;
  the documents name the external clients for every other provider,
  rather than an adapter over acme.sh's scripts (B)
  or lego's providers built into the binary (C).

D15-A is a block of this plan rather than one of its own:
it rewrites the paragraphs Block 1 touches.

**Names chosen here**, routine and not decisions of the owner's:
the test `TestHooksNeedCurlAndJq`; the branch `claude/hook-deps`.

**Not in this plan:**
a hook for another provider; the install warning of an open API while users
sign in (AuthWarning, a plan of its own).

**Who runs it:**
this session, under the owner's `EXEC D14-A D15-A` of 2026-11-09:
the plan and its blocks, each block's commit after a commit gate.

## Analysis

What changes, by file:

| File | Block | Change |
|------|-------|--------|
| `packaging/acme-dns-hooks/infomaniak.sh`, `cloudflare.sh` | 1 | the check for `curl` and `jq` after the arguments, before the token is read; the header says so |
| `internal/certs/hooks_test.go` | 1 | `TestHooksNeedCurlAndJq` |
| `packaging/nfpm.yaml` | 1 | `suggests: [curl, jq]` |
| `packaging/deb/test.sh` | 1 | the `Suggests` field; each installed hook without the tools |
| `packaging/README.md` | 1, 2 | the hooks' tools and the package's `Suggests`; other providers through an ACME client |
| `README.md` | 1, 2 | the same, in short |
| `CHANGELOG.md` | 1 | the bullet under Unreleased, Fixed |

**What a hook does today without its tools**, under `set -eu`:
`cloudflare.sh` calls `jq` inside a command substitution in an argument,
which `set -e` does not stop,
so the zone lookup sees an empty answer
and says "not an answer of the API" or "no zone of … that this token sees";
`infomaniak.sh` says "no zone of … at Infomaniak".
Without `curl` the same, with curl's "not found" above it.
The server logs the hook's output and the failed order,
so the box's owner reads a wrong cause.

**The check** is POSIX `command -v` for each tool,
the first missing one named on stderr with the Debian command,
exit 1, before the token file is read and before any call.

**Suggests, not Recommends:**
apt installs a package's Recommends by default,
and most boxes never use a hook;
Suggests shows in `apt show massimo` and installs nothing.

**The external clients, Block 2:**
`acme.sh --install-cert` copies the certificate and the key to the paths it is
given and runs the reload command after every renewal;
`chown massimo:massimo` there leaves the files to the service user,
and `massimo install -tls-cert … -tls-key …` reads the files fully before it
writes them back, so naming the same paths is safe.
docs/reverse-proxy.md, step 3, already has acme.sh's directories, its token and
the timer Debian's package lacks; the new paragraph points there.

**Risks:**

- A box whose `PATH` lacks `/usr/bin` for the service:
  the check then reports a tool that is installed;
  the unit's environment has the default `PATH`,
  so this is only a hook run by hand.
- The deb test's run without the tools relies on the slim images lacking both;
  it checks only where `curl` is absent.

## Implementation Plan

Each block ends with the contract's test gate:
`MASSIMO_ENV_FILE=/dev/null go test ./...`, `gofmt -l .` empty,
`go vet ./...` for linux, windows and darwin, `make cross`,
and `GUIDELINES/shared/tools/check`.
Nothing reaches a Cube, a box or a DNS provider; the fakes answer.
One functional commit per block, citing `AP HookDeps v1.0, Block <n>`.

### Block 1: Tools

1. `internal/certs/hooks_test.go`, `TestHooksNeedCurlAndJq`:
   each hook with a `PATH` of an empty directory says "curl is not installed",
   and with one of only a `curl` that notes being run says "jq is not installed";
   both fail, and that `curl` never ran.
   Run first on the old hooks, where it must fail.
2. Both hooks:
   the check after the arguments;
   the header's "It needs curl and jq" says that the script names the missing one.
3. `packaging/nfpm.yaml`:
   `suggests: [curl, jq]` with a comment.
4. `packaging/deb/test.sh`:
   `dpkg-deb -f "$deb" Suggests` is `curl, jq`;
   where `curl` is absent,
   each installed hook fails with "curl is not installed".
5. `packaging/README.md`:
   the Infomaniak paragraph and the package's paragraph name the tools
   and `sudo apt install curl jq`;
   the package's contents name the `Suggests`.
   `README.md`: the hooks' sentence names the tools.
6. `CHANGELOG.md`, Unreleased, Fixed:

   ```
   - A DNS-01 hook on a box without `curl` or `jq` says which one is
     missing, where it said that no zone was found; the Debian package
     suggests both (`packaging/README.md`).
   ```

**Verification:**
the test gate; the new test fails on the old hooks and passes on the new;
`shellcheck` on both hooks and `packaging/deb/test.sh`;
`make packages`, then `make deb-test` with `DEB_IMAGE=debian:trixie-slim`
and `debian:bookworm-slim`.

### Block 2: Providers

1. `packaging/README.md`:
   the paragraph "Without the hook, an external client …" for a provider
   without a script here as well:
   acme.sh's DNS API scripts (`--dns dns_<provider>`), lego, certbot's plugins;
   the acme.sh lines into `/etc/massimo/cert.pem` and `key.pem`
   with the `chown` as reload command,
   then `massimo install -tls-cert … -tls-key …`,
   pointing at docs/reverse-proxy.md, step 3, for the rest;
   the certbot and lego lines for Infomaniak stay, lego with the note of
   docs/reverse-proxy.md about Debian 13's version.
2. `README.md`:
   "any other provider's API fits in a dozen lines"
   → an ACME client that knows the provider, into `-tls files`.

**Verification:**
`reflow.sh --check` on both files shows no new warning;
the commands match acme.sh's options as docs/reverse-proxy.md uses them.
No CHANGELOG bullet: documents only.

## Proposed commit messages

The plan's registration, a commit of its own before the first project write,
on the work branch:

```
chore(aps): +HookDeps v1.0

The DNS-01 hooks fail without curl or jq with an error that names neither, and the Debian package names neither; README promises a hook per provider in a dozen lines where ACME clients already know them. The owner's D14-A and D15-A of 2026-11-09: the hooks check for their tools, the package suggests them, and the documents send every other provider through an ACME client into -tls files.

Key changes:
- GUIDELINES/wip/2026-11-09_16-01_AP_HookDeps_hook_tools_and_providers.md: the plan, two blocks, D14-A and D15-A in its Discussion.

File-revision-v1: GUIDELINES/wip/2026-11-09_16-01_AP_HookDeps_hook_tools_and_providers.md::v1.0::2026-11-09::All::the first revision, by EXEC D14-A D15-A
Assisted-by: claude-code:claude-opus-5-5
```

The blocks' commits:
Block 1,
`fix(packaging): A DNS-01 hook says when curl or jq is missing, and the package suggests both`;
Block 2,
`docs(tls): Every other DNS provider through an ACME client into the TLS files`.

## Change History

- v1.0 (2026-11-09): the first revision,
  from the owner's checks of AP Tls and the owner's D14-A and D15-A of the same day.

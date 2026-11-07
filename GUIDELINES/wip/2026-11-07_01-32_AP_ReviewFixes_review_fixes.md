# AP ReviewFixes v1.1: the review's defects fixed

## TL;DR

The 71 findings of the review of main since 2026-11-05 become fixes in five blocks,
the five highs and thirty mediums every one,
the lows where a block has their file open;
every fix starts with a test that shows the defect.
Two stacks run in parallel: the Go blocks and the non-Go blocks.

- Block 1 → secrets and access: the API key out of command rows,
  X-Forwarded-For, audit after authorization, the hook's token and timeout,
  the helper units, a removed user's preferences
- Block 2 → updates and TLS: SNI for the health check, the mode switch,
  the health URL, SIGTERM and the Windows stop, the hourly check's order
- Block 3 → the queue and the client: the idempotent replay,
  the observer's order, the 202, the serial, `t:` replies, the reconnect,
  the pairing ticket, the greeting
- Block 4 → publishing: Homebrew from release.yml, the apt key path,
  the keyring version, the Pages environment, the deb version
- Block 5 → the UI: one loader per panel, typing kept across redraws,
  schemes, the tile and the time input, Log out, the UI lows
- every finding reproduced first; one not reproduced is recorded,
  not "fixed"

Rests on
[2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md](2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md),
whose finding numbers the blocks cite.
The decisions D1 to D6 are in the Discussion, each with a recommendation.
Run order: 1, 2, 3 in one session; 4, 5 in another.

### Blocks

1. ReviewSecurity: findings 1, 11, 12, 16, 21, 22, 25; lows 37, 46, 54
2. ReviewUpdates: findings 2, 3, 18, 19, 20, 23, 24, 26;
   lows 55 to 62
3. ReviewQueue: findings 6 to 10, 13, 14, 15, 17;
   lows 39, 41, 43, 45, 47, 48, 50, 51
4. ReviewPublish: findings 4, 31, 32, 35; low 63
5. ReviewUi: findings 5, 27, 28, 29, 30, 33, 34; lows 64 to 71

## Discussion

The review was four Opus 5.5 agents over `git diff e3b53cf..4740636` by area,
with the `code-review` skill at level high;
each verified its top findings by hand, the rest are the skill's.
The plan takes the suggested cut of the analysis's last section,
with one change:
finding 29 (Log out) moves from the security block to the UI block,
so that the Go blocks and the non-Go blocks touch no file in common and can run
in two sessions at once.

- **D1.
  The lows.**
  - A (recommended): a low is fixed in the block that has its file open (the
    ones the Blocks list name), the rest wait for a later revision;
    the open files get their small fixes for the price of the test already written,
    and nothing else is touched.
  - B: all thirty-six lows in; a third more work, much of it in files no
    block otherwise opens.
  - C: no lows; the cheap ones (a dead field,
    a duplicated rule) would wait for nothing.
- **D2.
  Sessions.**
  - A (recommended): two sessions in parallel, the Go stack (Blocks 1 to 3,
    branches `claude/review-go`, `-2`,
    `-3`) and the non-Go stack (Blocks 4 and 5, `claude/review-ui`, `-2`),
    one pull request per block stacked as AP Tls did; they share no file.
  - B: one session, five stacked pull requests;
    half the wall-clock time is not needed.
- **D3.
  The unverified findings.**
  - A (recommended): every block starts with a test per finding that fails on main;
    a finding whose failure cannot be shown is recorded in the run's NOTE as
    "not reproduced" and left as it is, not "fixed" blind.
  - B: trust the skill and fix by the description; cheaper, and a wrong
    fix in a path nobody can test is the worst outcome.
- **D4.
  Finding 11, the client's address behind a trusted proxy.**
  - A (recommended): an X-Forwarded-For entry that does not parse yields the
    peer's address,
    and X-Real-IP counts only when no X-Forwarded-For header came at all:
    what a client can set never wins over what the proxy appended.
  - B: take X-Real-IP whenever the peer is a trusted proxy; simpler,
    and wrong with a proxy that appends X-Forwarded-For and leaves a
    client's X-Real-IP in place.
- **D5.
  Finding 12, audit rows for requests without a token.**
  - A (the owner's choice): authorize first; a refused request is audited
    at most once per address and minute, the row carrying the count of
    requests within that minute, so a lockout stays visible without
    unbounded rows on the SD card, and the body is not read.
  - B: no audit of unauthenticated requests;
    the trail of a refused attempt is lost.
- **D6.
  Finding 18, the health URL for a specific listen address with TLS.**
  - A (recommended): the helper's health client skips verification for a
    host that is one of the box's own addresses at the time of the check,
    as it does for loopback; the check reads a version from the box itself.
  - B: pin the server's certificate fingerprint in the health URL's drop-in at install;
    exact, and stale after a renewal.
  - C: document that a specific listen address with TLS needs `-no-update-helper`;
    the owner loses updates from the UI on such a box.

Two facts shape the blocks:

- **CHANGELOG.**
  Every commit under cmd/,
  internal/ or packaging/ carries a bullet under `[Unreleased]`, `### Fixed`,
  one line each;
  a block's fixes to one file are one commit where they are one change,
  else one commit per finding.
- **Hardware.**
  Nothing here needs a Cube;
  the TLS and update fixes of Block 2 are proved by the tests and by CI's
  Windows and macOS jobs,
  which install the service and update it through the helper.

## Analysis

The analysis file holds the findings with their failure scenarios and fixes.
What the blocks add to it:

### 1. Where the fixes land

| Block | Files |
|-------|-------|
| 1 | `internal/server/commands.go`, `audit.go`, `auth.go`, `server.go`, `runtimes.go`; `internal/store/users.go`, `preferences.go`; `internal/certs/dns01.go`; `internal/install/install.go`; `packaging/acme-dns-hooks/infomaniak.sh`; their tests; docs/api-v1.md, docs/reverse-proxy.md, packaging/README.md |
| 2 | `internal/certs/acme.go`, `selfsigned.go`; `internal/install/tls.go`, `listen.go`; `cmd/massimo/serve.go`, `servetls.go`, `service_windows.go`, `update.go`, `tls.go`, `config.go`, `user.go`; `internal/update/update.go`; `internal/env/env.go`; their tests; README.md, docs/update-windows.md |
| 3 | `internal/server/commands.go`, `server.go`, `login_api.go`, `config_api.go`; `internal/engine/queue.go`; `internal/cubelan/client.go`, `inclusion.go`, `snapshot.go`, `fakecube/fakecube.go`; `internal/cubeio/pairing.go`, `cubeio.go`; `internal/store/commands.go`; `internal/protocol/cube` (one method); their tests; docs/api-v1.md, docs/proxy.md |
| 4 | `.github/workflows/release.yml`, `publish.yml`; `packaging/apt/publish.sh`, `keyring.nfpm.yaml`; `Makefile`; packaging/README.md, GUIDELINES/project/CI.md |
| 5 | `internal/server/ui/users.js`, `cubes.js`, `login.js`, `theme.js`, `tiles.js`, `config.js`, `log.js`, `i18n.js`, `app.js`, `roles.js`, `components.js`; `tools/ui-test/test_*.py`; docs/ui.md |

Blocks 1 and 3 both open `internal/server/commands.go` and `server.go`,
which is why they stack in one session; Blocks 4 and 5 share nothing with them.

### 2. What is already known to be sound

The analysis's "Checked and sound" lists hold:
the update's signature and size checks, the helper's request file,
the secret file modes, the loopback-only `InsecureSkipVerify`,
the hook's stripped environment, the UI's text sinks,
the workflows' permissions.
None of the blocks weakens them; Block 2's D6 widens the loopback rule to
the box's own addresses only.

### 3. Risks

- Finding 2's fix (a server name for a handshake without SNI) must not make
  autocert order a certificate for a name it does not hold:
  the name is `names[0]`, which the manager already allows.
- Finding 13's fix (counting `A:` replies for `t:`) changes the client's reply table;
  the fake Cube already answers one per address,
  and the server's own removal sends one address,
  so the tests cover both shapes.
- Finding 19's fix (SIGTERM) changes how every box stops; the shutdown path
  then runs on every `systemctl restart`, and its own bugs would show at
  once in CI's Linux-less jobs only through the Go tests, so Block 2 adds
  a test that a cancelled serve closes the store and flushes the events.
- Block 5's redraw fixes touch the most-used pages;
  the pytest suite's four cases hold the page's behaviour,
  and the screenshot set shows a changed layout.

## Implementation Plan

Every block ends with the contract's test gate for what it touched:
`MASSIMO_ENV_FILE=/dev/null go test ./...` (`-race` for `internal/server`,
`internal/cubelan`, `internal/proxy` and `internal/store`),
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin,
`make cross`, `make ui-test` where a file under `internal/server/ui/` changed,
and `GUIDELINES/shared/tools/check --commit-range`.
Every fix starts with its test, written to fail on main (D3);
nothing reaches a Cube, the network, or the owner's boxes;
addresses, serials and tokens in tests are placeholders.
Each block is one pull request,
stacked on the previous block's branch in its stack,
with `origin/main` merged into the stack's first branch before its pull request.
Commit headlines carry the component with its part,
`fix(security:server): …` for a security fix, `fix(server.queue): …`,
`fix(install.tls): …`, `fix(ui.users): …`, `fix(workflows): …`, `fix(apt): …`,
each with its CHANGELOG bullet under `### Fixed`.

### Block 1: ReviewSecurity

1. Finding 1: `noCommand` (internal/server/audit.go) gains the token creation route,
   or `endCommand` scrubs `secretFields` out of the stored answer;
   a test creates a token and reads the command row and `GET /api/v1/commands/{id}`:
   no secret anywhere;
   an `Idempotency-Key` replay of the creation answers without the secret.
2. Finding 11 (D4-A): `clientIP` returns the peer for an entry that does not parse,
   and reads X-Real-IP only when no X-Forwarded-For header is present;
   tests for "unknown",
   "ip:port" and a client-set X-Real-IP beside an appended X-Forwarded-For;
   docs/reverse-proxy.md says which header wins.
3. Finding 12 (D5-A): `authorize` before `beginAudit`; a refused request is
   audited once per address and minute through a small in-memory map that
   the lockout's cleanup prunes, the row's detail carrying the count of
   requests within that minute (written when the minute ends or at the
   next refused request), with no body read; a test sends twenty token-less
   POSTs with bodies and finds one audit row with the count 20 and no body
   bytes read (a reader that counts).
4. Finding 16: `DeleteUser` removes the user's preferences (and `AddUser`'s
   takeover of a soft-deleted row starts with none); a test removes "anna",
   adds "anna", reads empty preferences.
5. Finding 21: `infomaniak.sh` hands the Authorization header to curl through
   `-K -` on stdin (never argv), and every curl call gets `--max-time 30`;
   packaging/README.md's hook section says so; `bash -n` and shellcheck clean;
   the hook's fake-API test in internal/certs still passes.
6. Finding 22: `runHook` sets `cmd.WaitDelay = 5 * time.Second` and, on unix,
   `Setpgid` with a process-group kill on timeout;
   a test runs a hook whose child sleeps with the pipe open and sees `runHook`
   return within the timeout plus the delay.
7. Finding 25: `installLinux` with `-no-update-helper` disables and removes
   `massimo-update.path`,
   `.service` and the `listen.conf` drop-in when they exist, as darwin does;
   a test with `-root` installs with the helper, reinstalls without it,
   and finds the units gone.
8. Low 46: the last-owner guard and `ownersLeft` count an owner with
   `expires_at` set as not left; low 54:
   the clear hook runs on `context.WithoutCancel` with its own timeout and its
   failure is always logged; low 37:
   `limiter.succeed` writes only for an address that has failures (an in-memory set).
9. CHANGELOG bullets under `### Fixed`; docs/api-v1.md where an answer's
   shape changed (the token creation's replay).

**Verification:**
the test gate; `go test -race ./internal/server/... ./internal/store/...`;
the new tests fail on main and pass on the branch (shown in the pull request);
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 2: ReviewUpdates

1. Finding 2: `ACME.GetCertificate` with an empty `ServerName` serves the
   certificate of `names[0]` (a copy of the hello with the name set); a
   test handshakes without SNI against a server with a prepared cache and
   gets the certificate; the health check's loopback URL stays as it is.
2. Finding 23: `Check` calls autocert with a hello that names `names[0]`,
   ECDSA signature schemes, the curves and a TLS 1.3 suite,
   so the ordered certificate is the one browsers get and the status shows its
   fingerprint; a test with the fake ACME directory sees one order.
3. Finding 3: `tlsFiles` removes the TLS variables the new mode does not
   set (`MASSIMO_TLS_HTTP`, `MASSIMO_ACME_DNS_HOOK`, `MASSIMO_TLS_NAMES`,
   `MASSIMO_ACME_EMAIL`, the cert and key paths) through `setEnvLines`;
   a test installs acme with `-tls-http`, reinstalls with `off`, and
   `tlsSource` of the resulting file starts; low 62: the variable names
   defined once in `internal/env` and imported by install and serve.
4. Finding 18 (D6-A): `healthClient` skips verification for a host that is
   one of the box's interface addresses (read at the time of the check),
   as for loopback; a test with a fake interface list; docs/update-windows.md
   and README.md's "Server" say when the check is verified.
5. Finding 19: `serve` stops on SIGTERM as on SIGINT
   (`signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`);
   a test cancels a running serve and finds the store closed and the pending
   events flushed.
6. Finding 20: the Windows service's `Execute` cancels serve's context on Stop
   or Shutdown and waits for `runServe` within the SCM's wait hint before returning;
   `GOOS=windows go vet` and the existing service tests;
   CI's Windows job restarts the service through the helper.
7. Finding 24: `runUpdate` in `-request` mode writes "failed" with the
   error through a deferred status for every return after `TakeRequest`;
   a test with an empty channel reads status.json "failed".
8. Finding 26: `fileCube` falls back to the plain `MASSIMO_CUBE_ADDRESS`
   and `_NAME` for a single serial equal to `MASSIMO_CUBE_SN`,
   as `env.Get` does; low 58:
   `Serials` upper-cases what it returns and `PerCube` matches case-insensitively;
   tests for both.
9. Lows 55, 56, 59, 60, 61, 57:
   the self-signed renewal keeps the loaded certificate's names;
   `Latest` returns no error when the channel has no release;
   the download gets a client without `Timeout` bounded by ctx and an idle deadline;
   candidates are dropped by channel before their manifests are fetched;
   `tls show` calls `printTLS`;
   `user tokens <name>` lists a user's tokens with ids (README.md's command line).
10. CHANGELOG bullets under `### Fixed`; README.md and
    docs/update-windows.md where the behaviour is described.

**Verification:**
the test gate; CI's Windows and macOS jobs green on the pull request, the
macOS job on its normal branch now that the alpha release knows `-tls`;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 3: ReviewQueue

1. Finding 7: the queue calls `Observer.Enqueued` before the entry becomes
   visible to `Run` (under the lock, with its Seq), or `Started` and
   `Finished` create the step they do not find; a test enqueues on an idle
   queue and reads the step as finished; low 48: the observer's store
   writes go through a buffered channel drained by one goroutine, so `Run`
   never waits on SQLite.
2. Finding 6: a replay of a row with `Status == 0` answers 503 with the
   stored result instead of `WriteHeader(0)`; a test drops an unfinished
   row and replays its key; low 39: the original answer is kept apart from
   the run's outcome and replayed as it was.
3. Finding 8: `POST /api/v1/update`'s command row reaches a final state when the
   helper reports done or failed (`finishCommand` from the status watcher),
   and a 202 with no follow-up is acknowledged;
   a test runs an update with the stub helper and reads the row finished.
4. Finding 9: `rt.serial` is read and written under the runtime's mutex through accessors;
   `go test -race ./internal/server/...` with a test that learns a serial while
   discovery runs.
5. Finding 10: the pump refuses a serial another runtime holds (stops this one and logs),
   as `register` does;
   a test adds a Cube by serial while an address-only runtime connects with the
   same serial.
6. Finding 13: the client expects `count` `A:` lines for a `t:` with `count` addresses;
   a test through the proxy sends `t:03,…` and sees the next `m:` answered by
   its own `A:`.
7. Finding 14: `SetAddress`, and `Reconnect` while disconnected,
   wake the dial loop;
   a test changes the address during the backoff and sees the dial within a second.
8. Finding 15: `Ticket` gets a cancel that removes a still-waiting entry,
   and `StartPairing` ends inclusion only when the removal succeeded; a
   test times out a pairing behind a blocked queue and sees no `n:` sent;
   `Writer.Do`'s metadata write gets the same shape.
9. Finding 17: every C line of the greeting is applied to the snapshot
   before it answers a pending `c:`, so `endGreeting` keeps it; a test
   re-reads during a greeting and finds the device still there.
10. Lows 41, 43, 45, 47, 50, 51:
    one `DeviceType.IsThermostat()` for the four copies;
    the dead `write.d` and `itoa` gone;
    the inclusion timer ends only its own generation;
    `user_id IS ?` with an index on the idempotency lookup and the prune on a timer;
    `Reread` carries the request's ID; the fake Cube's join timer stops on `x:`,
    `n:` and close.
11. CHANGELOG bullets under `### Fixed`; docs/api-v1.md for the replay answers;
    docs/proxy.md for `t:`.

**Verification:**
the test gate with `-race`; the exact lines of the proxy test;
`GUIDELINES/shared/tools/check --commit-range` clean.

### Block 4: ReviewPublish

1. Finding 4: the Homebrew job runs inside release.yml after the release is
   published (`needs:` the publish job, the formula rendered from the tag),
   gated on the deploy key as today; publish.yml goes,
   or stays for a manual `workflow_dispatch`;
   GUIDELINES/project/CI.md section 6 says why (a release made with the
   workflow's token starts no workflow).
2. Finding 31: the apt sources snippet points `signed-by` at the keyring
   package's `/usr/share/keyrings/massimo-archive-keyring.gpg`,
   and the bootstrap installs the keyring package first (the `.asc` serves that
   one download); packaging/README.md's apt section rewritten to match;
   `packaging/apt/test.sh` follows the snippet.
3. Finding 35: the keyring package's version is monotonic (the publish date and time,
   `1.0.<YYYYMMDDHHMM>`), and the comment in `keyring.nfpm.yaml` says so.
4. Finding 32: packaging/README.md and the workflow's comment say that the
   `github-pages` environment must allow `v*` tags, with the setting's path;
   the roadmap's packaging steps for the owner gain the line.
5. Low 63: `make packages` passes `DEBVER` to nfpm as the version, so the
   file name and the control version agree; the CI deb job's test checks
   `dpkg-deb -f … Version` against the file name.
6. CHANGELOG bullets under `### Fixed` for the workflow and the scripts
   (`.github/**` and `packaging/**` are shipped paths).

**Verification:**
`bash -n` and shellcheck on the scripts, a YAML parse of the workflows,
the Debian package CI job green on the pull request,
`GUIDELINES/shared/tools/check --commit-range` clean;
the Homebrew job's run is proved at the next release, which the roadmap notes.

### Block 5: ReviewUi

1. Finding 5 and 27: a panel built with `d.open` set loads its contents
   through its toggle handler only (the direct call goes, or the listener
   is added after the open), in users.js and cubes.js; the pytest suite
   creates a key and sees it once with one row, and types in the pairing
   form while a redraw happens.
2. Finding 28: Hide and Show again update the device list section alone,
   or carry the form's current values across `renderCubes`; a test types a name,
   hides a device, and sees the name still there.
3. Finding 29: Log out cancels the preferences sync timer and clears the
   synced keys from the browser (the device-only ones stay); a test signs
   out and in as another user and sees no inherited scheme.
4. Finding 30: the server's scheme list is authoritative once it exists (a local
   scheme the server lacks is dropped, not pushed), so a deletion propagates;
   a test deletes a scheme in one context and reloads another.
5. Finding 33: a state event defers the tile's re-render while the focus is
   inside its configuration area, and renders when the focus leaves;
   a test types while events stream (a `page.route` stub) and keeps its keystrokes.
6. Finding 34: the programme editor commits a time on `change` without
   re-rendering and re-sorts on blur or Save; a test types a full time.
7. Lows 64 to 71: `afterLogin` only when no `api()` retry waited;
   live events merged after a log page loads;
   the end date compared at minute precision;
   the Yes button disabled while a removal runs;
   a superseded language load resolves silently;
   the stream handlers' reloads caught;
   one `confirmInPlace` in components.js for the three remove flows;
   one low-level request helper under `api()`, `signIn` and `flushPreferences`.
8. Every new text in the six translation files;
   docs/ui.md where the behaviour is described;
   `make ui-screenshots` and the changed shots only;
   CHANGELOG bullets under `### Fixed`.

**Verification:**
the test gate; `make ui-test` at 1100 and 390 px in English and German with the new tests;
`make ui-screenshots` with the changed shots looked at;
`GUIDELINES/shared/tools/check --commit-range` clean.

## Proposed commit message

The plan's registration, a commit of its own before the first project write,
with the analysis it rests on:

```
chore(aps): +ReviewFixes v1.1, +ANALYSIS ReviewFixes

The review of main since 2026-11-05 (four agents by area, 71 findings) gets its fix plan registered before the first project write, and the analysis it rests on comes with it, copied from the review's message file with its timestamp.

Key changes:
- GUIDELINES/wip/2026-11-07_01-32_AP_ReviewFixes_review_fixes.md: the plan, five blocks in two stacks, the decisions D1 to D6 in its Discussion, D5 the owner's.
- GUIDELINES/wip/2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md: the findings, ranked, with the failure scenario and the fix of each.

File-revision-v1: GUIDELINES/wip/2026-11-07_01-32_AP_ReviewFixes_review_fixes.md::v1.1::2026-11-07::All::the first revision, with D5 settled
File-revision-v1: GUIDELINES/wip/2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md::v1::2026-11-07::All::the analysis the plan rests on
Assisted-by: claude-code:claude-fable-5-1
```

The blocks' commits, one per finding or per fix that is one change,
each written at its gate and ending with the implementing session's
`Assisted-by: claude-code:<model>`; the headline's component carries the part,
for example:

1. `fix(security:server): keep a new API key out of the command's result`
2. `fix(certs.acme): serve the first name's certificate to a handshake without SNI`
3. `fix(engine.queue): record a command's step before it can run`
4. `fix(workflows): publish the Homebrew formula from the release workflow`
5. `fix(ui.users): load a panel's tokens once, and keep the new key shown`

## Change History

- v1.1 (2026-11-07): D5 settled by the owner as A with the count of
  requests within the minute on the audit row; Block 1 step 3 says so.
- v1.0 (2026-11-07): the first revision, from the review of the same night
  and its suggested cut, finding 29 moved to the UI block so that the two
  stacks share no file.

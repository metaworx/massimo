# ANALYSIS ReviewFixes: the review of main since 2026-11-05

The analysis AP ReviewFixes rests on,
copied from the review's message file of 2026-11-07 01:03 with its timestamp;
the finding numbers below are the ones the plan cites.

Four Opus 5.5 agents reviewed `git diff e3b53cf..4740636` by area with the
`code-review` skill at level high (up to 20 findings each), on 2026-11-07;
e3b53cf is the commit that added docs/review-2026-11-05.md.
Each agent checked its top findings against the code by hand;
the rest are as the skill reported them and nobody has verified them yet.
Nothing was run against hardware; nothing was changed.

| Area | Paths | Diff |
|---|---|---|
| server | internal/server without ui/, internal/view, internal/backup | 48 files changed, 12165 insertions(+), 617 deletions(-). |
| cube/store | internal/store, engine, cubeio, cubelan, proxy, protocol/cube | 62 files changed, 10344 insertions, 134 deletions. |
| cli/platforms | cmd/massimo, internal/install, update, certs, env, portal | 72 files changed, 10978 insertions(+), 565 deletions(-). |
| ui/packaging/ci | internal/server/ui, tools/ui-test, packaging, .github, Makefile | 87 files changed, 14432 insertions(+), 781 deletions(-). |

Findings: 0 critical, 5 high, 30 medium,
36 low (71 after merging one duplicate).
Ranked most severe first, the area in brackets;
the number in parentheses is the finding's number in that area's own file.

## Themes

- **Secrets and access** (high 1; medium 11, 12, 16, 21, 22, 25, 29):
  a new API key stored in plain text as a command's result;
  a trusted proxy's unparsable X-Forwarded-For entry letting the client's
  X-Real-IP through; audit rows for unauthenticated requests;
  a re-added user inheriting a removed user's preferences;
  the Infomaniak token in curl's argv; a hook child blocking the order;
  the root update units left enabled after `-no-update-helper`;
  Log out leaving a user's settings for the next sign-in.
- **Updates that roll back, and TLS** (high 2, 3; medium 18, 23, 24, 26):
  `-tls acme` refuses the helper's health check without SNI;
  switching `-tls` modes leaves the old mode's variables and `serve` restart-loops;
  a specific `-listen` address with TLS gives a verified health URL;
  the hourly check orders an RSA certificate browsers never get;
  helper errors with no "failed" status;
  the plain `MASSIMO_CUBE_ADDRESS` ignored once a per-Cube variable exists.
- **Event loss at shutdown** (medium 19, 20): SIGTERM from systemd and launchd,
  and the Windows service Stop,
  both end `serve` without the flush of pending events.
- **The command queue and the Cube client** (medium 6 to 10, 13 to 15, 17):
  an idempotent replay that panics after a restart;
  the observer's step rows stuck at queued;
  a 202 that never reaches a final state; a serial written without a lock;
  a runtime overwriting another's key;
  a `t:` with several addresses leaving extra replies;
  a reconnect that waits out its delay after an address change;
  a pairing timeout ending inclusion while its `n:` is still queued;
  a greeting C line dropped.
- **Publishing** (high 4; medium 31, 32, 35):
  the Homebrew workflow never fires for a release made with the workflow's token;
  the apt snippet trusts a key file the keyring package never updates;
  the Pages deploy from a tag may be refused;
  the keyring version can go down on a rotation.
- **The web UI** (high 5; medium 27, 28, 30, 33, 34):
  the user panel loading its tokens twice and hiding the new key;
  the Cube panel's double load; typed values lost on Hide and Show again;
  a deleted colour scheme returning from another browser;
  a tile re-rendered while the user types;
  the programme editor's time input replaced mid-edit.

## Findings

### 1. [high] internal/server/commands.go:191

**A new API key is stored in plain text as the command's result** [server (1)] Category:
security Summary:
endCommand saves every answer that is not in noCommand (audit.go:84 lists only login,
logout and preference.*) as the command row's result,
and postToken's 201 answer (users_api.go:426) holds the new secret as `token`.
Failure scenario: An owner runs POST /api/v1/users/alice/tokens.
The commands row now holds {"token":"<secret>",…}, so GET /api/v1/commands/{id},
the SQLite file and every backup of it give out a live key.
That defeats the hashed storage and the "shown once" rule,
and a replay with the same Idempotency-Key returns the key again. secretFields
scrubs request bodies only, not answers.
Fix: Add "token.create" (and any other answer that carries a secret) to noCommand,
or scrub secretFields out of the stored result.

### 2. [high] internal/certs/acme.go:106

**ACME refuses a handshake without SNI,
so the helper's https://127.0.0.1 health check fails and every update rolls
back** [cli/platforms (1)] Category: correctness Summary:
`ACME.GetCertificate` hands an empty `hello.ServerName` straight to
`autocert.Manager.GetCertificate`, which returns "acme/autocert:
missing server name".
Failure scenario: `install -tls acme -tls-names home.example` writes
`MASSIMO_HEALTH=https://127.0.0.1:8099/healthz` (healthURL maps `:8099` to 127.0.0.1;
`serveUpdateHelper` on Windows and the darwin plist do the same).
Go sends no SNI to an IP host, so the handshake fails,
`waitHealthy` times out after 2 min and the helper rolls back every update.
LAN clients that use `https://<ip>` fail the same way.
Fix: when `hello.ServerName == ""`,
set it to `a.names[0]` on a copy of the hello before calling autocert (or put
that name in the health URL and resolve it to loopback).

### 3. [high] internal/install/tls.go:106

**switching `-tls` mode leaves the previous mode's variables,
and serve then restart-loops** [cli/platforms (2)] Category:
correctness Summary:
`tlsFiles` only sets the new mode's variables through `setEnvLines`.
It never removes `MASSIMO_TLS_HTTP` or `MASSIMO_ACME_DNS_HOOK`,
and `tlsSource` (cmd/massimo/servetls.go:75-83) refuses those outside acme,
or together.
Failure scenario: install with `-tls acme -tls-http :80`,
then `massimo install -tls off` (or `self-signed`).
The settings still have `MASSIMO_TLS_HTTP=:80`,
serve exits with "-tls-http goes with -tls acme",
and systemd or launchd keeps restarting it.
The same happens going from acme with `-acme-dns-hook` to acme with `-tls-http`.
Fix: in every mode, remove (or comment out) the TLS variables the new mode does not set.

### 4. [high] .github/workflows/publish.yml:9

**The Homebrew job never runs for releases that release.yml creates**
[ui/packaging/ci (1)] Category: correctness Summary:
publish.yml runs `on: release: published`,
but release.yml:169 creates the release with `GH_TOKEN: ${{ github.token }}`,
and GitHub starts no new workflow run for an event caused by GITHUB_TOKEN.
Failure scenario: `make release-alpha` pushes tag v0.x.y-alpha.N → release.yml
runs `gh release create` with the default token → the `release` event fires but
starts no workflow → the tap metaworx/homebrew-massimo is never updated,
and `brew upgrade` stays on the old version without any error showing.
Fix: run the Homebrew job from release.yml (`needs: sign`,
or `workflow_call` into publish.yml),
or create the release with a GitHub App token or PAT.

### 5. [high] internal/server/ui/users.js:240

**An open user panel loads its tokens twice: duplicate rows, two keys per click,
and the new key hidden** [ui/packaging/ci (2)] Category: correctness Summary:
userPanel sets `d.open = true` (line 183),
which queues a `toggle` event whose handler (line 190) calls fillTokens,
and line 240 calls fillTokens directly as well.
Failure scenario: The owner opens a user's panel and clicks Create key →
userAction → renderUsers → both fillTokens calls run.
The first shows the new key and deletes `usersUI.keys[name]`,
the second finds no key and hides `.user-key-box`,
so the key the server shows only once is lost.
Every token row appears twice, and `.create-key` has two listeners,
so the next click POSTs two API keys.
Fix: drop the direct call on line 240,
or add the toggle listener only after setting `d.open`,
so that only the toggle handler loads the tokens.

### 6. [medium] internal/server/commands.go:207

**An Idempotency-Key replay of a dropped row calls WriteHeader(0) and panics**
[server (2)] Category: correctness Summary:
DropUnfinished (store/commands.go:317) marks a row dropped but leaves Status at 0,
Final(dropped) is true,
and the replay branch then calls w.WriteHeader(row.Status) without checking it.
Failure scenario: A PUT …/setpoint with an Idempotency-Key is in flight when the
server restarts.
The client retries with the same key,
net/http panics with "invalid WriteHeader code 0", the connection is cut,
and that key never gets an answer.
Fix: When row.Status == 0,
answer 503 (or 409) with the stored result instead of replaying.

### 7. [medium] internal/server/commands.go:124

**Observer.Enqueued can run after Started and Finished,
leaving step rows stuck at queued** [server (3)] Category: concurrency Summary:
The queue calls Enqueued after the item is already runnable and the lock is released,
and Enqueued makes two store round trips (CommandByUUID,
AddStep) before it records steps[Seq].
Failure scenario: The queue is idle and a setpoint arrives.
Run picks it up at once,
Started and Finished find no steps entry and do nothing,
and then Enqueued stores it.
The step stays "queued" in GET /commands/{id} for good,
and the steps map leaks one entry per such command.
Fix: Record the step in the map before the item becomes runnable,
or let Started and Finished wait for, or create, the entry.
Same defect: the Cube-side review reports it at internal/engine/queue.go:226
(merged below as one item).

### 8. [medium] internal/server/commands.go:227

**A 202 answer that is not a restore never reaches a final state** [server (4)] Category:
correctness Summary: endCommand maps 202 to StateSent,
and only the restore path later calls finishCommand,
while POST /api/v1/update also answers 202.
Failure scenario: POST /api/v1/update with an Idempotency-Key leaves its row
"sent" until the next restart.
Every retry with that key gets 409 "still running",
and the command history shows the update as never finished.
Fix: Call finishCommand when the update completes or fails,
or mark 202 rows that have no follow-up as acknowledged.

### 9. [medium] internal/server/server.go:518

**The pump writes rt.serial with no lock while other goroutines read it** [server (5)]
Category: concurrency Summary: The pump assigns the serial it learned,
while discoveryLoop (configured(), note()) and handlers (storedCube, summary,
learnStoreID) read rt.serial at the same time.
Failure scenario: An address-only Cube connects while discovery runs.
That is a data race (go test -race reports it),
and a stale read can raise a duplicate cube_found event or a wrong own-answer filter.
Fix: Guard serial with the runtime's mutex (or an atomic.Value) and read it
through an accessor.

### 10. [medium] internal/server/server.go:520

**Learning a serial overwrites s.cubes[serial] without register's duplicate
check** [server (6)] Category: correctness Summary:
The pump sets s.cubes[serial] = rt unconditionally,
which skips the check that register makes.
Failure scenario: An address-only Cube from MASSIMO_CUBE_ADDRESS has not connected yet.
The user POSTs /cubes with the same serial, so a second runtime registers.
When the first runtime connects, it takes the key:
two LAN clients now compete for a one-client Cube,
and DELETE /cubes/{sn} stops the wrong runtime while the orphan keeps running.
Fix: When the key is already held by another runtime,
refuse it (stop this runtime and log), as register does.

### 11. [medium] internal/server/auth.go:211

**An X-Forwarded-For entry that does not parse falls through to the client's
X-Real-IP** [server (7)] Category: security Summary:
The break says "the peer it is" but then falls through to X-Real-IP,
which a client can set when the proxy appends XFF and does not overwrite X-Real-IP.
Failure scenario: A trusted proxy appends "unknown" or "ip:port" to XFF.
The client sends X-Real-IP: 127.0.0.1, and clientIP returns 127.0.0.1.
That escapes the per-address lockout and passes the local-network check for
creating the first owner.
Fix: Return peer instead of breaking (and use X-Real-IP only when there is no XFF header).

### 12. [medium] internal/server/server.go:674

**beginAudit runs before authorize:
unauthenticated requests are read in full and written as audit rows** [server
(8)] Category: security Summary: On every audited route,
each request without a token, including those already locked out with 429,
reads up to 1 MB of body,
makes a store lookup and inserts a "refused" audit row.
Failure scenario: Someone on the LAN loops POST /api/v1/cubes/X/restore with 1
MB bodies and no token.
The audit table on the Pi's SD card grows without bound and wears the card,
and the lockout does not stop it.
Fix: Authorize first and audit only the requests that are authorized (or
rate-limit the refused rows, for example one per address and minute).

### 13. [medium] internal/cubelan/client.go:340

**a `t:` with several addresses gets one `A:` per address,
Send consumes one** [cube/store (2)] Category: correctness Summary:
`replyType('t')` waits for a single `A`,
but the Cube answers `A:` for each address (docs/portal-protocol.md,
"was answered with `A:` for each of them";
the fake Cube does `strings.Repeat("A:\r\n", len(addrs))`),
so the surplus lines arrive with no pending reply or as the reply of the next `A`-command.
Failure scenario: a LAN client through the proxy sends `t:03,1,<3 addresses>`;
the first `A:` completes it,
the next queued `m:` or `z:` is sent after the Gap and takes the second stray
`A:` as its reply before the Cube processed it;
its own `A:` then answers the command after it, and the desync runs on.
`RemoveCommands` itself sends one address, so the server's own delete is safe.
Fix: for `t:` expect `count` A lines (parse the count from the line) before
releasing the pending reply.

### 14. [medium] internal/cubelan/client.go:404

**Reconnect does nothing while the client is between connections** [cube/store (3)]
Category: correctness Summary:
`Reconnect` returns `ErrNotConnected` when `conn == nil`,
and `SetAddress` does not wake `Run`'s backoff,
so the discovery path "address changed: reconnect now" waits out the backoff.
Failure scenario: the Cube takes a new DHCP lease;
dials to the old address fail and `Run` sleeps up to about a minute;
discovery calls `SetAddress(new)` and enqueues `{Reconnect: true}`
(internal/server/discovery.go:271); `execute` gets `ErrNotConnected` at once,
the step is recorded failed,
and the client dials the new address only when the timer expires.
Fix: let `SetAddress` (and `Reconnect` while disconnected) signal a channel that
cuts the backoff sleep short.

### 15. [medium] internal/cubeio/pairing.go:37

**a timed-out StartPairing ends inclusion in the client while its `n:` is still
queued** [cube/store (4)] Category: concurrency Summary:
`StartInclusion` is set before the `n:` is sent,
and when `Wait(ctx)` times out the code calls `EndInclusion` although the queued
`n:` is not withdrawn and is sent later.
Failure scenario: the queue is held by a command waiting in `waitForBudget`;
the proxy's `wctx` (ReplyTimeout) expires,
`StartPairing` returns the error and ends inclusion; later the `n:` goes out,
the Cube is in inclusion mode with no watcher and the client says "not active";
its N lines reach nobody and a second StartPairing may overlap the Cube's running mode.
`Writer.Do` has the same shape for `ApplyMetadataWrite`:
a write that runs after its caller gave up is never mirrored into the snapshot.
Fix: give `Ticket` a cancel that removes a still-waiting entry,
and end inclusion only when the removal succeeded.

### 16. [medium] internal/store/users.go:210

**a user re-added under a removed name inherits the old user's preferences**
[cube/store (5)] Category: security Summary:
`AddUser` taking over a soft-deleted row keeps its id,
and `DeleteUser` leaves the `preferences` rows,
so the new person gets the old one's settings.
Failure scenario: "anna" is removed;
an admin later adds a different person as "anna";
`Preferences(ctx, id)` returns the former anna's preferences and colour schemes.
Fix: tombstone or delete the user's preferences in `DeleteUser`,
or clear them in the takeover.

### 17. [medium] internal/cubelan/snapshot.go:112

**a greeting C line consumed by a pending Send is dropped at the end of the
greeting** [cube/store (6)] Category: concurrency Summary:
`endGreetingLocked` removes every config not marked fresh,
but a C line handed to a pending `c:` during the greeting is not applied,
so not fresh.
Failure scenario: after a drop the queue runs a re-read `c:` as soon as `c.conn` is set;
the greeting's first C line goes to that Send;
at the L line that device's configuration is deleted from the snapshot,
and the re-read got another device's C line.
Fix: apply every C line to the snapshot before handing it to the pending reply,
or hold sends until `greeted`.

### 18. [medium] internal/install/listen.go:98

**a specific `-listen` address plus TLS gives a health URL the helper verifies,
so a self-signed certificate rolls back every update** [cli/platforms (3)] Category:
correctness Summary: `healthURL` keeps a specific host,
and `healthClient` (cmd/massimo/update.go:251) skips verification only for loopback hosts.
Failure scenario: `install -listen 192.168.1.5:443 -tls self-signed` writes
`MASSIMO_HEALTH=https://192.168.1.5:443/healthz`.
The x509 check fails with "unknown authority",
`waitHealthy` times out and Rollback runs.
This happens with the systemd drop-in, the darwin plist and Windows `HealthURL`.
With ACME it fails too, because the certificate is for a name, not the IP.
Fix: for TLS, have the helper skip verification for any address the box itself listens on,
or pin the server's certificate.
Or document that a specific listen address with TLS needs `-no-update-helper`.

### 19. [medium] cmd/massimo/serve.go:113

**serve handles only SIGINT,
so systemd and launchd stops kill it without the graceful shutdown**
[cli/platforms (4)] Category: correctness Summary:
`signal.NotifyContext(ctx, os.Interrupt)` has no `syscall.SIGTERM`,
the units have no `KillSignal=SIGINT`, and launchd sends SIGTERM.
Failure scenario: `systemctl restart massimo` (run by install,
by the update helper and by the owner) sends SIGTERM.
The process dies at once: srv.Run never flushes the pending log events,
requests in flight are cut and `st.Close` is skipped.
This was there before e3b53cf and was carried over in the diff.
Fix: add `syscall.SIGTERM` to the NotifyContext (both are in the os/syscall set
on every platform).

### 20. [medium] cmd/massimo/service_windows.go:233

**a Windows service Stop calls os.Exit(0) before serve shuts down** [cli/platforms (5)]
Category: correctness Summary: as soon as `svc.Run`/Execute returns,
the goroutine calls `os.Exit(0)`.
It does not cancel runServe's context or wait for it,
so the deferred store Close and the event flush are skipped.
Failure scenario: the update helper's RestartService, or a Windows shutdown,
sends svc.Stop.
The process exits while runServe is still serving,
and pending events are lost on every update and every reboot.
Linux has the same loss through finding 4.
Fix: cancel serve's context from Execute on Stop or Shutdown,
wait for runServe to return (within the SCM's wait hint), then exit.

### 21. [medium] packaging/acme-dns-hooks/infomaniak.sh:36

**the Infomaniak API token is passed on curl's command line,
readable by every local user** [cli/platforms (6)] Category: security Summary:
`curl ... -H "Authorization: Bearer $token"` puts the token in curl's argv,
which every local user can read through `ps` or `/proc/<pid>/cmdline`.
That defeats the 0600 mode the header asks for on `/etc/massimo/infomaniak.token`.
Failure scenario: any unprivileged account (or process) on the Pi that polls
`ps -eo args` while a certificate is ordered or renewed collects a token with
the DNS scope, which lets it rewrite the owner's zone.
Fix: give the header to curl on stdin or through a file,
e.g. `printf 'header = "Authorization: Bearer %s"\n' "$token" | curl -K - ...`,
so the token never reaches argv.

### 22. [medium] internal/certs/dns01.go:332

**a hook whose child keeps stdout open blocks the order past `hookTimeout`**
[cli/platforms (7)] Category: security Summary:
`exec.CommandContext` plus `CombinedOutput`,
with no `cmd.WaitDelay` and no process group,
kills only the hook's shell when the timeout fires.
Wait then blocks until every child that inherited the pipe exits.
Failure scenario: infomaniak.sh runs `curl` with no `--max-time`.
If the API stalls, the timeout kills `sh`,
but curl keeps the pipe open and `runHook` never returns,
so the DNS-01 order (and the hourly Check behind it) hangs for as long as curl does.
A hostile or slow DNS API can hold the renewal indefinitely.
Fix: set `cmd.WaitDelay = 5*time.Second` (and on unix
`SysProcAttr{Setpgid: true}` with a group kill).
Also add `--max-time` to the curl calls in the hook.

### 23. [medium] internal/certs/acme.go:132

**Check calls autocert with an empty ClientHelloInfo and so orders an RSA
certificate browsers never get** [cli/platforms (8)] Category:
correctness Summary: autocert's `supportsECDSA` is false for an empty hello,
so the startup and hourly Check order and renew `name+rsa`,
while the first browser gets a separate ECDSA certificate.
Failure scenario: every installation makes twice the Let's Encrypt orders.
The Status and the startup log show the RSA certificate's fingerprint,
not the one served.
`tls show` (LoadACMECache reads only the `name` key) says "no certificate yet"
until a browser connects.
Fix: give Check a hello with `ServerName`, ECDSA `SignatureSchemes`,
`SupportedCurves` and a TLS 1.3 cipher suite, as a modern client sends.

### 24. [medium] cmd/massimo/update.go:119

**in helper mode, errors after the request is taken leave no "failed" status**
[cli/platforms (9)] Category: correctness Summary:
once `TakeRequest` has removed the request, the dev-build check,
the `ValidChannel` check and the `os.Executable` and `EvalSymlinks` errors all
return without calling `status(...)`.
Failure scenario: a helper built without a compiled channel gets a request with
an empty channel. runUpdate returns "no update channel",
status.json keeps the previous run's state,
and the web UI shows a stale result for a request that was silently dropped.
Fix: in `-request` mode,
use a deferred status write that records "failed" for any error returned before
a later status was written.

### 25. [medium] internal/install/install.go:507

**a Linux reinstall with `-no-update-helper` leaves the root helper units
enabled** [cli/platforms (10)] Category: security Summary:
the unit loop installs `massimo-update.path` and `.service` only when the helper
is wanted.
It never disables or removes an earlier install's units or their `listen.conf` drop-in,
while `InstallDarwin` does unload its helper.
Failure scenario: install,
then reinstall with `-no-update-helper` to opt out. serve loses `-update-request`,
but `massimo-update.path` stays enabled.
Any write by the service user to `/var/lib/massimo/update-request` still starts
a root update run with an outdated drop-in, which the owner believes is gone.
Fix: when the helper is not wanted,
`systemctl disable --now massimo-update.path` and remove both units and the drop-in,
as darwin does.

### 26. [medium] cmd/massimo/serve.go:485

**fileCube ignores the plain MASSIMO_CUBE_ADDRESS once any per-Cube variable
exists** [cli/platforms (11)] Category: correctness Summary:
`plainCube` is false as soon as one `MASSIMO_CUBE_<SN>_*` variable exists,
so the plain address is not used even for that same single Cube.
`env.Get`, used by the other commands, does fall back to it.
Failure scenario: `MASSIMO_CUBE_SN=KEQ0000000`, `MASSIMO_CUBE_ADDRESS=10.0.0.5`,
`MASSIMO_CUBE_KEQ0000000_PROXY=10.0.0.6`. serve seeds the Cube with no address
and has to rely on discovery, which fails across subnets and from WSL,
while `massimo status` works.
Fix: when there is exactly one serial and it equals `MASSIMO_CUBE_SN`,
fall back to the plain variables, as env.Get does.

### 27. [medium] internal/server/ui/cubes.js:502

**The same toggle-on-build double load for a Cube panel's pairing and rooms**
[ui/packaging/ci (3)] Category: correctness Summary:
cubePanel sets `d.open = cubesUI.open.has(serial)` on a new `<details>`,
so its toggle handler runs on every renderCubes and calls loadPairing
and loadCubeRooms again.
Failure scenario: With a Cube panel open, every save,
Hide or Show-again click sends extra GET …/pairing and …/rooms requests.
Each answer rebuilds the pairing section (redrawPairing),
which takes focus out of the pairing form while the user types.
Fix: as in 2, make the toggle handler the only loader,
or skip its side effects while a render flag is set.

### 28. [medium] internal/server/ui/cubes.js:118

**Hide and Show again throw away unsaved typing in the same Cube panel**
[ui/packaging/ci (4)] Category: correctness Summary:
These buttons redraw the whole list with renderCubes(),
which rebuilds every field from the server state.
Failure scenario: The user edits the Cube's name and proxy address,
then hides a device before clicking Save → the form is rebuilt from `c` → the
typed values are gone and Save sends only the hidden list.
The file header promises that a redraw never throws away what is being typed.
Fix: update only the device list section,
or carry the form's current values across the redraw.

### 29. [medium] internal/server/ui/login.js:205

**Log out leaves the previous user's preferences and schemes,
and the next sign-in uploads them** [ui/packaging/ci (5)] Category:
security Summary:
logout() clears only the token. localStorage keeps user A's settings and custom schemes,
and a pending prefsSync timer is not cancelled.
Failure scenario: On a shared browser,
A logs out and B (who has no server preferences yet) signs in → mergeSchemes
adds A's schemes to B's account,
and loadPreferences pushes A's `ui` settings as B's.
If A logged out within 1 s of a change,
the timer fires without a token → 401 → the sign-in dialog opens unprompted.
Fix: on logout, cancel the sync timer and clear the synced (non-device-only)
preference keys from localStorage.

### 30. [medium] internal/server/ui/theme.js:227

**A deleted colour scheme comes back from any other browser** [ui/packaging/ci (6)]
Category: correctness Summary:
mergeSchemes keeps every local scheme the server lacks and pushes it back,
so a deletion never propagates.
Failure scenario: The user deletes the scheme "Mine" on the laptop → the server
no longer has it → the phone, whose localStorage still has "Mine",
loads → mergeSchemes keeps it,
`kept=true` → schedulePreferences('schemes') → "Mine" is back on the server
and on every device.
Fix: treat the server's list as authoritative once it exists (merge only when
the server has none), or record deletions as tombstones.

### 31. [medium] packaging/apt/publish.sh:172

**The sources snippet trusts a key file that the keyring package never updates**
[ui/packaging/ci (7)] Category: correctness Summary:
The published snippet uses `signed-by=/etc/apt/keyrings/massimo.asc`,
while massimo-archive-keyring installs `/usr/share/keyrings/massimo-archive-keyring.gpg`.
Failure scenario: The key's expiry is extended and a new keyring package is published,
as packaging/README.md promises → boxes that followed the snippet still verify
with the downloaded massimo.asc → when it expires,
`apt update` fails with EXPKEYSIG on every box, and apt cannot repair it.
Fix: make the snippet point `signed-by` at
`/usr/share/keyrings/massimo-archive-keyring.gpg`,
and have the bootstrap install the keyring package.

### 32. [medium] .github/workflows/release.yml:225

**The Pages deploy from a tag may be refused by the github-pages environment**
[ui/packaging/ci (8)] Category: correctness Summary:
apt-deploy runs for a `v*` tag ref,
but the github-pages environment by default allows deployments only from the
default branch.
Failure scenario: Once MASSIMO_APT_KEY is set,
a release tag reaches apt-deploy → "Tag v0.x.y-alpha.N is not allowed to deploy
to github-pages due to environment protection rules" → the apt site is never updated.
The comments name only the Pages-source setting.
Fix: document (packaging/README.md and the workflow comment) that v* tags must
be allowed on the github-pages environment, or deploy from main.

### 33. [medium] internal/server/ui/tiles.js:680

**A state event re-renders a room tile while the user types in its configuration
area** [ui/packaging/ci (9)] Category: correctness Summary:
updateDevice re-renders the whole tile, including an open config area,
on every state event; only unlisted rows with an open form are protected.
Failure scenario: The user types a new device name in the room's Devices tab → a
valve or temperature event for that room arrives (frequent) → rerenderRoom
replaces the tile → the focused input is gone and further keystrokes are lost.
Fix: defer the re-render while `document.activeElement` is inside the tile's
`.config-area`, or patch only the state parts of the tile.

### 34. [medium] internal/server/ui/config.js:647

**The programme editor's time input is replaced in the middle of an edit**
[ui/packaging/ci (10)] Category: correctness Summary:
The time input's `change` handler sorts and re-renders the tile,
and Chrome fires `change` as each segment of a time input completes.
Failure scenario: The user changes a switch point's hour from 06 to 07 → change
→ pts.sort + edited() → rerenderRoom → the input is replaced and focus lost
before the minutes can be typed.
Entering a full time from the keyboard is effectively impossible.
Fix: commit the value on `change` without re-rendering,
and re-sort on blur or on Save.

### 35. [medium] packaging/apt/publish.sh:118

**The keyring package's version can go down on a key rotation** [ui/packaging/ci (11)]
Category: correctness Summary: The version is `1.0.<expiry date>`,
or the creation date for a key without an expiry,
so a rotated key with an earlier expiry or none sorts lower.
Failure scenario: The old key expires 2028-01-01 (version 1.0.20280101) and is
rotated to a key created in 2027 without an expiry (1.0.2027xxxx) → apt treats
it as a downgrade and keeps the old package → the rotation reaches no box.
The comment in keyring.nfpm.yaml still says VERSION is the creation date.
Fix: derive the version from a monotonic value,
such as the publish date or a counter kept in the repo, and fix the comment.

### 36. [low] internal/server/update_api.go:80

**The release check runs on the request's context under c.mu,
and its error is cached** [server (9)] Category: correctness Summary:
latest() holds the mutex during a network call of up to 30 s and caches any error,
including context.Canceled, for checkEvery.
Failure scenario: A tab closes while GitHub is slow.
For the next 10 minutes every GET /api/v1/update reports check_error "context canceled",
and parallel requests wait on the mutex in the meantime.
Fix: Run the check on a detached context with its own timeout,
and do not cache a context error.

### 37. [low] internal/server/auth.go:663

**Every authenticated request runs a store write through limiter.succeed** [server (10)]
Category: efficiency Summary:
Every authenticated request runs DELETE FROM auth_failures, plus CountUsers,
LockedFor, TokenByHash, UserByID and Effective,
even when its address has no failures.
Failure scenario: The UI polls every 10 s,
so every GET becomes a SQLite write transaction on the Pi's SD card that
competes with the event writer for the lock.
Fix: Clear failures only for an address that has some (an in-memory set),
and cache the user count.

### 38. [low] internal/server/runtimes.go:383

**removeCube leaves the Cube's event streams open** [server (11)] Category:
correctness Summary: The runtime is stopped and unregistered,
but the subscribers in rt.subs are never closed.
Failure scenario: Someone runs DELETE /api/v1/cubes/{sn} while a page streams
that Cube's events.
The stream sends only keep-alives from then on,
so the page never learns the Cube is gone,
and the removed runtime stays in memory through the subscriber.
Fix: Close every subscriber channel in removeCube so the handler ends its stream.

### 39. [low] internal/server/commands.go:207

**A replay does not always return the original answer** [server (12)] Category:
correctness Summary:
finishCommand overwrites a restore's result with the run's outcome,
and an answer over 256 KB or one that is not JSON is stored wrapped as {"answer":…}.
Failure scenario: A retry of POST …/restore with the same key gets 202
{id,state,sent,steps,error} instead of the first answer's {id,steps},
and a client that parses the first shape misreads it.
Fix: Keep the original answer apart from the outcome (a column of its own,
or a key in the result), and replay that.

### 40. [low] internal/server/login_api.go:154

**The session label is cut at byte 64,
which can split a UTF-8 character** [server (13)] Category: correctness Summary:
The label (by default the User-Agent) is truncated by bytes, not by runes.
Failure scenario: The 64th byte of a 70-byte label falls inside "é",
so the label is stored as invalid UTF-8 and the token list shows a replacement character.
Fix: Truncate at a rune boundary (for example, step back with utf8.RuneStart).

### 41. [low] internal/server/config_api.go:214

**thermostat() is a fourth copy of the device-class rule** [server (14)] Category:
reuse Summary:
`IsRadiatorThermostat() || == DeviceWallThermostat` is also written out in
cubeio/move.go isThermostat, view.DeviceOf and server.go stateAttrs.
Failure scenario: A new thermostat type is added to three copies but not the fourth,
and then copySettings, the stale rule and the log disagree about it.
Fix: Add one cube.DeviceType.IsThermostat() method in protocol/cube and call it
everywhere.

### 42. [low] internal/server/discovery.go:263

**note() works out the Cube port inline,
duplicating Server.cubePort()** [server (15)] Category: reuse Summary:
The CubePort default and override logic is repeated instead of calling s.cubePort().
Failure scenario: A later change to cubePort() (a per-Cube port,
say) does not reach discovery,
and discovered Cubes get a different port than added ones.
Fix: Call s.cubePort().

### 43. [low] internal/server/settings_api.go:165

**write.d is set but never read,
and itoa only wraps strconv.Itoa** [server (16)] Category:
simplification Summary: The *durationSetting field is dead state,
and the itoa wrapper in commands.go adds nothing.
Failure scenario: There is no runtime fault,
but putSettings and the observer are harder to read and to change.
Fix: Drop the field and call strconv.Itoa directly.

### 44. [low] internal/proxy/proxy.go:398

**a client's `x:` gets nothing unless it holds a watch and the Cube sends an
empty N** [cube/store (7)] Category: correctness Summary:
`pairing` handles `x:` through `StopPairing` and writes nothing back;
only a session that watches inclusion receives the relayed empty N line,
and that the Cube sends one after `x:` is unconfirmed on hardware.
Failure scenario: a client that sends `x:` without having sent `n:` in this session,
or any client if the Cube sends no N after `x:`,
waits for an answer that does not come (the original software expects the empty N).
Fix: write `N:` to the sender when none was relayed within `stopWait`.

### 45. [low] internal/cubelan/inclusion.go:61

**a stale inclusion timer can end the next inclusion** [cube/store (8)] Category:
concurrency Summary: the timer callback calls `EndInclusion` unconditionally;
one that fired while the previous inclusion was ending and blocked on `inc.mu`
ends the next.
Failure scenario: timer #1 fires as an empty N ends inclusion #1;
a client starts #2 at once;
the stale callback takes the lock and closes #2's watchers.
Fix: capture a generation number when arming and end only that generation.

### 46. [low] internal/store/users.go:263

**the last-owner guard ignores ExpiresAt** [cube/store (9)] Category:
correctness Summary: the guard checks role and Disabled only,
and `ownersLeft` counts expired owners as enabled.
Failure scenario: the only owner gets `ExpiresAt` tomorrow through `UpdateUser`;
after it passes no owner can log in,
and other owners could already be demoted since the expired one still counts.
Fix: count an owner with `expires_at` set (or past) as not left,
in both the guard and `ownersLeft`.

### 47. [low] internal/store/commands.go:138

**unindexed idempotency lookup and prune on every AddCommand** [cube/store (10)] Category:
efficiency Summary:
`coalesce(c.user_id, 0) = ?` cannot use the `(user_id, idempotency_key)` index,
and `pruneCommands` runs full-table UPDATE/DELETE on each write.
Failure scenario: with `commands.keep_days` 0 (keep forever) the table only grows;
every write request scans it two or three times on the single SQLite connection,
so write latency grows with the history.
Fix: use `c.user_id IS ?`, add an index on `created_at`/`finished_at`,
and prune on a timer.

### 48. [low] internal/engine/queue.go:276

**Started/Finished write SQLite on the queue's critical path** [cube/store (11)] Category:
efficiency Summary:
the observer's `SetCommandState` runs synchronously in `Run`,
on the store's one connection shared with the event writer and handlers.
Failure scenario: an events batch or a slow handler holds the connection;
the next radio command waits up to `busy_timeout` (5 s) and its Gap/wakeup timing drifts.
Fix: hand observations to a buffered channel drained by a goroutine.

### 49. [low] internal/cubelan/netconfig.go:206

**netconfig falls back to a random port the Cube does not answer** [cube/store (12)]
Category: correctness Summary:
`listenDiscovery` binds a random port when 23272 is taken,
but the Cube answers to 23272,
so with `serve` on the same host the CLI never sees the status.
Failure scenario: `massimo netconfig set` beside a running `serve`:
the C write may take effect, no reply comes, R is not sent,
and the user is told the Cube did not answer.
Fix: refuse up front when 23272 is taken,
or route through serve's responder socket as Discover does.

### 50. [low] internal/cubeio/cubeio.go:80

**Reread's `c:` does not carry the request's ID** [cube/store (13)] Category:
correctness Summary: `Writer.Reread` enqueues directly, not through `Do`,
so the re-read is not a step of the request,
against `Writer.ID`'s doc ("a command that has none takes it").
Failure scenario: a configuration write with re-read shows one step in the
command history; a failed re-read is invisible there.
Fix: set `ID: w.ID` on the re-read command.

### 51. [low] internal/cubelan/fakecube/fakecube.go:343

**the fake Cube's join timer survives `x:`,
a second `n:` and the connection** [cube/store (14)] Category:
test-coverage Summary:
the timer that makes a device join after `n:` is never stopped.
Failure scenario: a test sends `n:` then `x:` before `Pair.Delay`:
the device still joins and an N line comes after inclusion ended;
two `n:` give two joins;
pairing tests become timing-dependent and hide late-N bugs.
Fix: keep the join timer with the inclusion timer and stop both on `x:`,
`n:` and close.

### 52. [low] internal/cubelan/discover.go:155

**`ask()` duplicates netconfig.go's `exchange()`** [cube/store (15)] Category:
reuse Summary: two UDP request/reply loops for the same protocol,
`ask` ignoring ctx and not copying out of its buffer.
Failure scenario: a reply-matching or deadline fix in one misses the other.
Fix: have `queryNetwork`/`queryConfig` call `exchange`.

### 53. [low] internal/cubelan/fakecube/discovery.go:20

**the 22-byte net-config layout is defined three times** [cube/store (16)] Category:
reuse Summary: `netConfigLen`, `boolByte` and the offsets `data[20]`,
`data[21]` are repeated in netconfig.go, the fake Cube and discover.go.
Failure scenario: a layout correction from a hardware finding must be made in
three places, and a missed one makes the fake Cube agree with the wrong code.
Fix: export the layout constants and offsets once from cubelan.

### 54. [low] internal/certs/dns01.go:291

**the "clear" hook runs on the order's context, which may already be done,
and its failure is not logged** [cli/platforms (12)] Category:
correctness Summary: after waitTXT or the 30-minute orderTimeout runs out,
`runHook("clear")` gets a cancelled context and fails at once.
The warning is printed only when the authorization succeeded.
Failure scenario: the TXT record never propagates.
The clear never runs, `_acme-challenge` TXT records pile up with each hourly retry,
and nothing in the log says so.
Fix: run the clear on
`context.WithTimeout(context.WithoutCancel(ctx), hookTimeout)` and always log its error.

### 55. [low] internal/certs/selfsigned.go:332

**the self-signed renewal drops names added by hand with
`tls self-signed -names`** [cli/platforms (13)] Category: correctness Summary:
Check renews with the names serve started with (`s.names`),
not the SANs of the certificate currently loaded.
Failure scenario: the owner adds `nas.lan` with `massimo tls self-signed -names`,
and the running serve picks the new pair up.
Thirty days before expiry, Check regenerates without `nas.lan`,
and browsers that use that name fail.
Fix: renew with the union of `s.names` and the current certificate's DNSNames
and IPAddresses.

### 56. [low] internal/update/update.go:320

**Latest reports an unsigned old release as the error when the channel simply
has no release** [cli/platforms (14)] Category: correctness Summary:
`firstErr` is returned whenever no candidate matches,
even when the channel has none at all.
Failure scenario: one pre-manifest release is among the newest 30,
and channel `main` has no release yet.
`update -check` and the UI report "release vX has no signed manifest" instead of
"no signed release on channel main".
Fix: return firstErr only when a release whose manifest says this channel failed
to verify; otherwise return nil, nil.

### 57. [low] cmd/massimo/user.go:246

**`user revoke <name> <token id>` asks for an id that `user list` never prints**
[cli/platforms (15)] Category: correctness Summary:
the usage says "the id of list", but `user list` prints only a token count.
Failure scenario: to revoke one leaked API key,
the owner sees "TOKENS 3" and has to revoke all of the user's sessions and keys.
Fix: add a `user tokens <name>` listing (id, kind, created, last used),
or print the ids in `user list`.

### 58. [low] internal/env/env.go:111

**Serials keeps the spelling of the serial,
but PerCube upper-cases it** [cli/platforms (16)] Category: correctness Summary:
on Linux, where variable names are case-sensitive,
a lower-case serial in a variable name is found by Serials and then misses every
PerCube lookup.
Failure scenario: `MASSIMO_CUBE_keq0000000_ADDRESS=10.0.0.5`. fileCube looks up
`MASSIMO_CUBE_KEQ0000000_ADDRESS`, gets nothing,
and the Cube is seeded with no address.
The code before the diff worked.
Fix: have Serials upper-case what it returns and PerCube match case-insensitively,
or reject a lower-case serial with an error.

### 59. [low] internal/update/update.go:170

**one 2-minute Client.Timeout also bounds the binary download** [cli/platforms (17)]
Category: correctness Summary: `http.Client.Timeout` includes reading the body,
and Download uses the same client as the API calls.
Failure scenario: a Pi 1 at about 60 KB/s downloading a 10 MB binary hits 120 s
and fails every update attempt.
The unit's `TimeoutStartSec=15min` assumes a slower download is fine.
Fix: give Download a client without Timeout (or a 10-minute one) and bound it
with ctx and a read-idle deadline.

### 60. [low] internal/update/update.go:308

**Latest verifies up to 30 releases one by one before it filters by channel**
[cli/platforms (18)] Category: efficiency Summary:
each candidate costs two sequential GETs (manifest and signature),
and the channel is checked only after verification.
Failure scenario: following `main` while the newest 25 releases are `alpha`
costs about 50 sequential HTTPS requests on every check the UI starts,
close to GitHub's unauthenticated limit of 60 an hour.
Fix: drop candidates by `target_commitish` before fetching their manifests,
then verify the one that remains.

### 61. [low] cmd/massimo/tls.go:143

**`tls show` re-implements printTLS from config.go** [cli/platforms (19)] Category:
reuse Summary: the same mode switch, file paths,
printACME and printCertificate are repeated,
and the two copies already differ (show omits HSTS and the names given).
Failure scenario: a new TLS mode, or a change to what is printed,
has to be made twice, and the two outputs drift apart.
Fix: have `tls show` call printTLS.

### 62. [low] internal/install/tls.go:21

**the TLS variable names are defined in three places** [cli/platforms (20)] Category:
reuse Summary: `MASSIMO_TLS`, `_CERT`, `_KEY`, `_NAMES`, `_HTTP`,
`ACME_EMAIL` and `ACME_DNS_HOOK` are constants in internal/install/tls.go and in
cmd/massimo/servetls.go, and env.Known lists them a third time.
Failure scenario: a rename in one place silently breaks the link between what
install writes and what serve reads.
Finding 2 is already that kind of install/serve mismatch.
Fix: define the names once in internal/env, next to Known,
and import them in both places.

### 63. [low] Makefile:26

**The .deb file name and the version inside it disagree for a build that is
not on a tag** [ui/packaging/ci (12)] Category: correctness Summary:
DEBVER turns every `-` of `git describe` into `~` for the file name,
but nfpm gets the raw VERSION and builds the control Version itself.
Failure scenario: The CI deb job for a commit after v0.7.0-alpha.1 produces
massimo_0.7.0~alpha.1~3~gabcdef_amd64.deb,
while its control version is probably 0.7.0~alpha.1-3-gabcdef,
which dpkg reads as upstream 0.7.0~alpha.1-3 with revision gabcdef → versions
sort inconsistently.
Fix: pass DEBVER to nfpm as the version.

### 64. [low] internal/server/ui/login.js:144

**A sign-in during page load runs init's load chain twice in parallel**
[ui/packaging/ci (13)] Category: correctness Summary:
A successful sign-in both resolves the pending api() retry and calls afterLogin(),
which runs loadMe, loadPreferences, refresh and connectEvents again.
Failure scenario: At page load GET /me returns 401 and the user signs in →
init's retried chain and afterLogin both run → two ticket POSTs and two event streams,
two full room loads,
and a refresh that may render rooms before initTiles has wired the House panel.
Fix: call afterLogin only when no api() retry was waiting,
or let the retry alone continue init.

### 65. [low] internal/server/ui/log.js:272

**Live events that arrive while the log loads are lost** [ui/packaging/ci (14)] Category:
correctness Summary: loadLog replaces `logState.lines` with the fetched page,
overwriting events that appendLogEvent added while the GET was in flight.
Failure scenario: The user changes the log's kind or device filter while events
stream → an event that arrives after the server built its answer but before the
page receives it is pushed,
then overwritten → it shows neither in the page nor live until a reload.
Fix: keep appended events newer than the page's newest timestamp and merge them
after the fetch.

### 66. [low] internal/server/ui/users.js:225

**The end date is sent with every save of a user who has one** [ui/packaging/ci (15)]
Category: correctness Summary:
The unchanged check compares `new Date(local).toISOString()` (`…:00.000Z`) with
the server's RFC 3339 string (e.g. `…+01:00`), which never match.
Failure scenario: An admin changes only a guest's display name → the body also
carries `expires_at`,
rounded down to the minute → the stored end date loses its seconds,
and "unchanged" can never appear for such users.
Fix: compare `Date.parse` of both values at minute precision.

### 67. [low] internal/server/ui/cubes.js:97

**A double click on a remove confirmation sends DELETE twice and reports a
failure** [ui/packaging/ci (16)] Category: correctness Summary:
The Yes button is not disabled while the request runs (removeCube,
and likewise removeUser, removeRole, deleteRoom);
only the unlisted-device removal guards with `ui.removing`.
Failure scenario: Double click on Yes → two DELETEs → the second gets 404,
and its error message replaces the success message → the user is told the
removal failed when it worked.
Fix: disable the Yes button on the first click.

### 68. [low] internal/server/ui/i18n.js:90

**A superseded language load still reports its error** [ui/packaging/ci (17)] Category:
correctness Summary:
setLanguage rejects even when a newer change has won (`seq !== i18nSeq`),
so changeLanguage shows the stale error.
Failure scenario: The user picks French (slow, then failing),
then German (loads) → the French promise rejects → "cannot load …fr.json" shows
on a page that is correctly in German.
Fix: resolve silently when `seq !== i18nSeq`.

### 69. [low] internal/server/ui/app.js:849

**Stream handlers leave reload promises uncaught** [ui/packaging/ci (18)] Category:
correctness Summary: The hello, metadata,
device_forgotten and disconnected handlers call loadCubes(),
loadRooms() and refresh() without a catch.
Failure scenario: The server goes away briefly → a metadata or disconnected
event triggers loadRooms, which fails → "Uncaught (in promise)" in the console,
and the ui-test console guard fails whatever test it lands in.
Other call sites already add `.catch(() => {})`.
Fix: add the same catch (or report through the page's status line) at these call sites.

### 70. [low] internal/server/ui/users.js:106

**Three copies of the remove-with-confirmation flow** [ui/packaging/ci (19)] Category:
reuse Summary: removeUser,
removeCube (cubes.js:96) and removeRole (roles.js:72) are the same function
under different names.
Failure scenario: The copies already drift apart:
only the unlisted-device variant guards against a double click (finding 16),
and a fix has to be made three times.
Fix: one `confirmInPlace(actions, text, labels, onYes, focusSelector)` in components.js,
which the convention "Shared elements come from `components.js`" asks for.

### 71. [low] internal/server/ui/login.js:173

**signIn and flushPreferences re-implement api()'s request handling**
[ui/packaging/ci (20)] Category: simplification Summary:
Both rebuild the Bearer header,
the JSON body and the error taken from `data.error` or the status.
Failure scenario: A change to how api() builds requests or reports errors (headers,
base path) has to be repeated in two more places,
and a missed one diverges silently.
Fix: a low-level request helper without the 401 retry (with a keepalive option),
used by api() and by these two.

## Checked and sound

**server**
- view.NewHidden returning nil under show=true also stops hiding the configured
  MASSIMO_CUBE_<SN>_IGNORE list.
  That is a change from backup.Hidden at e3b53cf,
  but README.md ("-show-hidden … `-show-ghosts` and `-no-ignore` are older
  names") and docs/api-v1.md document it as intended, so it is not a finding.
- The Idempotency-Key path answers 409 for a key still running and 422 for a key
  reused with another body hash, before any write runs.
- clientIP ignores X-Forwarded-For and X-Real-IP entirely when no trusted
  proxies are configured, or when the peer is not one of them.
- Request bodies kept in audit rows are scrubbed of password,
  current_password and token (secretFields);
  the gap in finding 1 is in the stored answers.
- Tokens are stored as hashes and looked up by hash (TokenByHash);
  the secret is returned only once, in the 201 answer (apart from finding 1).

**cube/store**
- `RemoveCommands` takes the room from the metadata,
  then the C line's room byte,
  matching the 2026-11-04 finding that the C-line room decides the blocked room;
  it skips room 0 and the hidden room.
- `Writer.RemoveDevice` forgets the device only after the Cube took the `t:`,
  and writes the metadata without it only where it was listed.
- `Client.Send` sets and clears `pending` under `c.mu` and does not hold the
  lock across the write or the wait;
  a dropped connection closes the pending channel.
- Coalescing by Key resolves the superseded ticket with `ErrSuperseded` under
  the lock and reports it to the observer after it.
- `StopPairing` drains the watch with a 2 s bound and always ends inclusion in the client,
  so a lost empty N cannot leave it active.

**cli/platforms**
- Update verification: the ed25519 signature is checked against the compiled-in
  key before anything else is used.
  The manifest's tag must equal the release's tag,
  the download is capped at MaxBinarySize,
  its SHA-256 must match the signed manifest before Swap,
  and the staged file is 0700, created with O_EXCL next to the target.
- Helper inputs: `TakeRequest` opens the request with no-follow and requires a
  regular file of at most 256 bytes whose content passes `ValidChannel`,
  so the service user can choose only among releases signed on some channel.
  The status file goes to `/var/lib/massimo-update/` (root-owned,
  StateDirectory) on Linux and macOS and to the SYSTEM/Administrators-only data
  directory on Windows,
  so the root helper writes nothing in a directory the service user can replace.
- Secret modes: settings files are written 0600 through `writeAtomic` (O_EXCL temp,
  chmod against the umask, rename), keeping an existing file's mode.
  Copied TLS key and cert are 0600 and chown'd.
  The self-signed pair is 0600 under a 0700 directory,
  and the ACME cache directory is 0700.
  Windows `winSettings` resets the data directory's ACL with
  `icacls /inheritance:r` to SYSTEM and Administrators.
- The loopback `InsecureSkipVerify` in `healthClient` applies only to https URLs
  whose host is 127.0.0.1, ::1 or localhost,
  and only to a request that reads the version.
  The test covers a LAN host staying verified.
- The DNS-01 hook runs through `exec.CommandContext` with separate arguments (no shell),
  with every `MASSIMO_*` variable stripped from its environment,
  and TXT propagation is checked against the expected value before the challenge
  is accepted.
- `GOOS=windows go vet` and `GOOS=darwin go vet` were not re-run:
  I edited nothing, and every finding is a cross-platform behaviour,
  not a build-tag drift.

**ui/packaging/ci**
- The scripts are clean under `shellcheck -S warning`:
  packaging/deb/{postinst,postrm,preinst,prerm,unit.sh,test.sh},
  packaging/apt/{publish.sh,test.sh}, packaging/homebrew/render.sh,
  packaging/acme-dns-hooks/infomaniak.sh, tools/ui-test/{run.sh,slideshow.sh}.
  (The only errors were for packaging/deb/copyright and packaging/deb/packaged,
  which are text files, not scripts.)
- API text reaches the DOM through `textContent` and templates:
  the review found no `innerHTML` or attribute sink built from API data,
  and no inline handlers.
- Every workflow has a top-level `permissions:` (contents: read in publish.yml),
  and the job-level grants are narrow (id-token/pages only on the deploy job).
  The tap key and apt key go in through secrets, gated on `secrets.X != ''`,
  without echoing them.
- Actions are pinned to major tags (checkout@v5, setup-go@v6 and so on),
  not SHAs.
  CI.md sets no rule on SHA pinning, so this is noted,
  not reported as a finding.
- The user-panel finding (2) was checked by hand:
  the toggle handler at users.js:190–197 and the direct call at users.js:240
  both call fillTokens.

## Recommended way to the fixes

One Action Plan, `AP ReviewFixes v1.0`,
with the highs and mediums as its blocks and the lows folded in where a block
touches the file anyway; every fix with a test that shows the defect first.
A suggested cut, one pull request per block:

1. **ReviewSecurity**:
   findings 1, 11, 12, 16, 21, 22, 25, 29 (the key kept out of command results,
   X-Forwarded-For returning the peer, audit after authorization,
   preferences removed with the user,
   the token off curl's argv with `--max-time`,
   `WaitDelay` and a process group for the hook,
   the helper units removed with `-no-update-helper`,
   Log out clearing the synced keys).
2. **ReviewUpdates**:
   findings 2, 3, 18, 19, 20, 23, 24,
   26 (a server name for the loopback health check,
   the mode switch cleaning the old variables,
   the health URL for a specific listen address, SIGTERM,
   the Windows Stop waiting for serve, a modern hello for the hourly check,
   a deferred "failed" status, the plain address fallback),
   with the platform lows 54 to 62 where the files are open.
3. **ReviewQueue**:
   findings 6 to 10, 13, 14, 15, 17 (the idempotent replay, the observer order,
   the 202, the serial lock, the duplicate key, `t:` counting its replies,
   the reconnect wake, the pairing ticket cancel, the greeting C line),
   with the server lows 36 to 43 and the cube/store lows 44 to 53 where the
   files are open.
4. **ReviewPublish**:
   findings 4, 31, 32, 35 and 63 (the Homebrew job run from release.yml,
   the apt snippet's key path, the Pages environment documented,
   a monotonic keyring version, the deb version passed to nfpm).
5. **ReviewUi**:
   findings 5, 27, 28, 30, 33,
   34 and the UI lows 64 to 71 (one loader per panel,
   typed values kept across a redraw, the server's scheme list authoritative,
   no re-render under a focused input, the time input committed on change,
   one `confirmInPlace` component for the three copies).

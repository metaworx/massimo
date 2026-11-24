# AP AutoUpdate v1.0: install updates at night

## TL;DR

Automatic updates, Off or At night: where on, the box installs a new
release in a night window while no command waits for the radio, once a
night. The server checks by itself, and offers only what its package
source has.

- the check → at each start and every 6 h, whatever the setting; the
  push `update` without an open page (D4)
- "available" → in apt's index or brew's formula, never newer than
  GitHub's signed release; a hand install GitHub's; held after the
  helper's "nothing newer yet" until a later check finds it (D5)
- settings → `update.auto` `off` | `night`, `update.auto_window`
  `03:00-05:00`, the server's time (D1, D2)
- the night → `POST /api/v1/update`'s start, no request; history and
  log mark it `auto`
- a holder of `update` → Install now · Tonight · Skip this version,
  box-wide in `update.answer` (D3)

Its branch stands on UpdateHistory v1.1 Block 4's (#412): the same
files.

### Blocks

1. ServerCheck: the server's own release check, at the start and every
   6 h; tests, docs
2. SourceCheck: available only where apt's index or brew's formula has
   it; the hold after "nothing newer yet"; the page's line; tests, docs
3. NightInstall: the settings, the install in the window while the
   radio is quiet, once a night; history and log `auto`; tests, docs
4. NightSetting: Automatic updates and its window in Software update;
   the texts of an automatic update; UI test, docs
5. UpdateAsk: the question for a holder of `update`, Install now,
   Tonight, Skip this version; the route; UI test, docs

The five blocks are one pull request, on #412's branch.

## Discussion

The owner's evaluation of automatic updates, 2026-11-24, settled:

- **D1-A:** a setting under Software update, "Automatic updates": Off
  (the default) or At night. Off unless the owner turns it on; every
  box, Windows included. This reverses UpdateHistory.D96's "Massimo
  installs no update by itself" for boxes where it is on.
- **D2-A, as the owner set it:** a time window that can be set (default
  03:00–05:00, the server's time). Within it, install only while no
  command is waiting for the radio. A person having the page open does
  not hold it back. At most once a night.
- **D3-A:** when an update is out and automatic updates are on, people
  with the capability `update` get a question in the page: Install now ·
  Tonight · Skip this version. Skip holds box-wide until a newer release;
  Tonight is the default; everyone else sees nothing new.
- **D4-A:** the server checks for a release on its own every 6 hours and
  at each start, whatever the setting, so that the push notification
  `update` reaches people without an open page.
- **D5-A+B:** a release counts as available only where the box's own
  package source has it: apt's index for its suite on an apt box, the
  tap's formula under brew; a hand install (`massimo install`) takes
  GitHub's, as today. Plus B: after the helper has answered "nothing
  newer yet" (`up-to-date`) for a version, the server and the page hold
  that version back (no Update, no automatic try) until a later check
  finds the source has it.
  Why: release.yml publishes the GitHub release first, and the apt index
  reaches the site only after its deploy; brew's tap lags too. The
  server's check (`update.Client.Latest`) reads GitHub, so for those
  minutes it offers a release apt does not have yet. The owner saw
  "v0.16.0-alpha.1-597-g43930b56 available" while the helper said
  "nothing to install: apt has nothing newer yet".

`PLAN++` on 2026-11-24.

The plan's own choices, each taking option A; `PLAN++` cleared the plan
gate, so the owner may turn one at the completion gate:

- **D6, what "a command waiting for the radio" counts:**
  A: a command with a radio line queued or in flight on any Cube, a
  write of Massimo's within its confirmation deadline, a restore, a
  pairing; not the Cube's own slots in use, which the Cube keeps across
  the server's restart and which a device out of reach can hold for
  hours. B: the slots as well.
- **D7, the settings' form:** A: `update.auto` a name, `off` or `night`,
  and `update.auto_window` one string `HH:MM-HH:MM`, at least 30 minutes,
  its end past midnight allowed (`23:30-01:30`). One string, because the
  two ends are checked together, one write cannot leave half a window,
  and the audit and `GET /api/v1/settings` show one row; a name, because
  the control is a choice of named values, and a third (as soon as out)
  would need no new key. B: a switch and two times.
- **D8, what the page offers while the source lags:** A: `latest` is the
  version an update would install, the source's newest but never newer
  than GitHub's signed release; `released` is GitHub's where the source
  has it not yet, and the line says "v… is out; apt has it not yet".
  B: nothing at all until the source has GitHub's newest.
- **D9, a source that cannot be read:** A: nothing is available, and
  `source_error` says why: apt could not fetch it either. B: GitHub's,
  as today, the hold of D5-B the only guard.
- **D10, where the question shows:** A: a banner under the header on
  every page, for a holder of `update`, and the same three answers in
  Software update; answered through `PUT /api/v1/update/answer`, which
  needs `update`. B: Software update alone, and the answers as settings
  through `PUT /api/v1/settings`, which needs `settings`, which a holder
  of `update` may lack.
- **D11, where brew's formula is read:** A: the tap's formula on GitHub
  (raw), the tap named by the keg's `INSTALL_RECEIPT.json`: what the
  helper's `brew update` fetches. B: the local tap's file, which stays
  old until `brew update` runs.

## Analysis

**The update flow today.** `GET /api/v1/update` (`getUpdate`,
`internal/server/update_api.go`) calls `updateRelease`: the channel
(`updateChannel`: `Channel`, the store's `update.channel`, apt's suite,
the flavour), the newest release through `latest` (reused for
`checkEvery`, 10 minutes, unless `refresh`), `Available` against the
running version, the push `notify.Event{Kind: "update"}` where one is
out, the helper's status (`update.ReadStatus`), `CanUpdate`, `History`.
`startUpdate` (`POST /api/v1/update`, capability `update`, audit
`update.start`) checks `updateBlocked`, calls `updateRelease(ctx, true)`,
takes `updateMu`, `keepChannel`, writes the request
(`update.WriteRequest`, the channel alone), calls `UpdateOptions.Start`
where set (Windows: `updateHelperStarter`,
`cmd/massimo/update_helper_windows.go`; brew: `brewHelperStarter`), then
`noteStop("update")`, `noteUpdateStart` (`update_history.go`) and the log
line `systemEvent("update", userIDOf(r), {from, to, channel})`.
`logStarted` (`systemlog.go`) completes the history at the next start
(`noteVersionStart`) and writes `update_result` once. Nothing checks
without a request: the page asks every 30 minutes for a holder of
`settings` (`updateDue`, `internal/server/ui/header.js`).

**The helpers.** All three answer `up-to-date` without a restart where
there is nothing newer: the hand helper (`cmd/massimo/update.go`), the
Debian one (`aptUpdate`, `cmd/massimo/update_apt.go`, "apt has nothing
newer yet") and brew's (`update_brew.go`, "brew has nothing newer yet").
The Debian helper runs `apt-get update` for the massimo source alone,
switches its suite to the request's channel first (`aptSwitch`), and
installs whatever apt's index names newest. `packagedUpdate`
(`cmd/massimo/serve_packaged.go`) sets `Suite` from `install.AptSuite`
and `Helper` from `install.UpdatePathWant`; `serveBrewUpdate`
(`serve_brew_darwin.go`) sets `Request`, `Status`, `Start` and
`Channel` to the formula's flavour.

**The check (D4).** `Run` (`internal/server/server.go`) starts the
server's loops after `logStarted`: `pruneBackupsDaily`, `discoveryLoop`,
`integrationsLoop`, `notify.Run`, each under `s.wg`. An `updateLoop`
joins them, a minute tick as `backupLoop` has (`backups.go`), its step
`updateTick(ctx, now)` testable with times: a check at the start, then
every 6 hours, a failed one again after `checkEvery`, so that a Pi that
starts before its network learns within minutes. The check is
`updateRelease(ctx, true)`, which pushes `update`; the rules send it
once per version (`internal/notify/rules.go`). Only with a store (the
rules and the setting need one) and a valid channel. The server's tests
that give a flavour or a channel also give a `Finder`; a guard in the
package's tests fails one that would reach GitHub.

**apt's index (D5-A).** The source is `install.AptSource`,
`/etc/apt/sources.list.d/massimo.sources`, deb822 as
`packaging/apt/publish.sh` writes it: `URIs:` (the site,
`https://massimo.metaworx.ch/apt`, the old github.io address that
redirects to it, or `file:` for the apt test), `Suites:`, `Components:
main`. publish.sh writes per suite `dists/<suite>/main/binary-<arch>/`
`Packages` and `Packages.gz` (`gzip -9 -n -k`, no by-hash), for `amd64`,
`arm64` and `armhf`; each lists the suite's own and every more stable
channel's packages, ten releases each, and the keyring package. The
reader, read-only and without root, fetches `Packages.gz` (stdlib
`compress/gzip`, bounded) for the suite the next update will use: the
channel where it is a flavour, as `aptSwitch` switches it, else the
source's; the arch from `runtime.GOARCH` (`arm` is `armhf`, which holds
the armv6 binary); and takes the newest `Package: massimo` stanza's
`Version:`. That is a Debian version (`0.16.0~alpha.1+597.g43930b56`):
`debTag` (`update_apt.go`, package main) reads it back as the tag, so it
moves to `internal/update` (`DebTag`), shared by the helper and the
server, and `update.Compare` orders the tags. `install.AptSuite` becomes
a reader of the three fields (`install.ReadAptSource`). The site serves
apt and the server through the same cache, so the server sees what apt
will see.

**The index unsigned.** `InRelease` is clearsigned, but Go's standard
library has no OpenPGP, and the index need not be verified for the
"available" decision: the version offered is never newer than GitHub's
release, whose manifest the compiled-in key verifies (`Client.Latest`),
so a forged index cannot bring a version that was never released; apt
verifies `InRelease` and every package's hash before the helper installs
anything; the worst a forged index does is say "has it" falsely, which
the helper's `up-to-date` and the hold answer, or hide a release, which
one who can forge the site's HTTPS answers can do to apt itself.

**brew's formula (D5-A).** release.yml's `homebrew` job (publish.yml)
pushes `Formula/<name>.rb` to `metaworx/homebrew-massimo`, rendered from
`packaging/homebrew/massimo.rb.in` by `render.sh`; the rolling job pushes
`massimo-devel` or `massimo-nightly` to `metaworx/homebrew-massimo-rolling`.
The formula carries `version "<v>"`; `brewTag` (`update_brew.go`) makes
it `v<v>`. The keg (`brewKegOf`) names the formula; its
`INSTALL_RECEIPT.json` names the tap (`metaworx/massimo`), whose formula
is read from `raw.githubusercontent.com/<owner>/homebrew-<name>/HEAD/`
`Formula/<formula>.rb` (D11). A tap that is not on GitHub (CI's
`local/homebrew-massimo`) leaves the source unread: GitHub's, as a hand
install. Under `HOMEBREW_NO_AUTO_UPDATE` the helper skips `brew update`
and the local tap decides; that case stays the hold's.

**The rule.** On a box with a source (`UpdateOptions.Source`, set by
`packagedUpdate` for apt and `serveBrewUpdate` for brew; nil for a hand
install): `latest` is the older of the source's newest and GitHub's,
`available` where it is newer than the running version, `released`
GitHub's where it is newer than the source's (D8). An unreadable source
makes nothing available, with `source_error` (D9). A rolling channel's
older build is gone from GitHub (release/devel is replaced in place), so
`release_url` stays GitHub's release.

**The hold (D5-B).** The helper's request names only the channel, so
its `up-to-date` names no target; the history's newest entry does
(`to`), and UpdateHistory v1.1 Block 4 ends it with `up-to-date` at the
next read (`noteHelperOutcome`) or at a start (`noteVersionStart`).
There the server notes `held{version, since}` (the helper's status time)
in memory, beside `s.updates`. A source read made after `since` that has
the version, or a newer one, ends it. Memory is enough: a restart is
followed by a check, which is the later check the rule asks for. While
held, `available` and `can_update` are false and `held` says why.

**The radio (D2, D6).** `GET /api/v1/cubes`' `queued` is
`rt.queue.Len()`, every waiting command, a refresh's reconnect among
them. `engine.Queue.Pending(match)` (`internal/engine/pending.go`) asks
the queued and the in-flight command; a match for a radio line (`s:`,
`r:`, as `radioLines` counts in `restore_api.go`) is the question, the
proxy's passthrough commands included. Queued commands are drained at a
stop (`Queue.drain`), so a restart would lose them. Beside them: a
write within its confirmation deadline (`rt.confirms.nextDue`,
`confirm.go`), whose re-send (`resend.go`) a restart loses; a restore
(`rt.restoring`); a pairing (`rt.client.Inclusion().Active`).
`radioBusy()` returns the first reason, "" when quiet. The Cube's slots
in use (`view.Pending`) are left out (D6-A).

**The settings (D1, D2, D7).** `settings_api.go` has tables for
durations, whole numbers and switches, and two names by hand
(`update.channel`, `announce`) in `settingsView` and `putSettings`. The
two new keys go beside `update.channel`, their constants in
`internal/store/settings.go` (`UpdateAutoKey`, `UpdateWindowKey`), both
applying at once, both needing `settings`; the window checked by one
parser (`parseWindow`). The window is the server's time zone
(`time.Local`); the default avoids Europe's switch hour (02:00–03:00).

**The night (D1, D2).** `startUpdate`'s core becomes `beginUpdate(ctx,
userID, auto)`: the checks, `updateMu`, `keepChannel`, the request,
`Start`, `noteStop`, the history and the log line; `POST` maps its
errors as today (they are `fail` errors already). `updateTick`, with
`update.auto` `night` and inside the window: due where an update is
available, not skipped, not held, `updateBlocked` empty, the helper not
running, and no automatic try since the window opened; then, while
`radioBusy()` is "", `beginUpdate(ctx, 0, true)`, else again next
minute. The release is checked afresh at the window's start and every
30 minutes inside it while nothing was tried, so that nightly, built at
02:30 UTC, is taken the same night. Once a night: the history's newest
`auto` entry since the window opened (it survives the restart the
update makes), and in memory a start that failed. A window that closes
while the radio stayed busy writes `update_postponed` (`why`).
`updateBlocked` keeps refusing an open server (no token, no users): the
automatic install does not run there either.

**Windows.** The Windows service starts the helper itself (`Start`,
`updateHelperStarter`); `beginUpdate` calls it as `startUpdate` does, so
the night needs nothing of its own there (`docs/update-windows.md`).

**The question (D3, D10).** Software update is
`internal/server/ui/src/software/Software.tsx` with `store.ts`; a
question in place of the buttons is `Question` and `useAsk`
(`shared/question.tsx`); a banner under the header is the open server's
(`#open-banner-slot` in `index.html`, `signin/Banner.tsx`). The page
reads `GET /api/v1/update` for a holder of `settings` only (`updateDue`
in `header.js`, `updateOut` in `shared/update.ts`), so a holder of
`update` alone never learns of a release: the load widens to `update`.
The answers live in one box-wide row, `update.answer`, `{version,
answer, by, at}`, `answer` `tonight` or `skip`, beside `update.history`
and out of the settings API. Skip holds while `latest` is not newer
than its version; Tonight ends the question for its version. `GET
/api/v1/update` adds `auto`: `mode`, `window`, `next` (when the box will
try, absent while none is due), `answer` where it names the offered
version or a newer one, and `ask`, so that the page asks where the
server says so. Install now is `POST /api/v1/update`.

**History and log.** An automatic entry has `auto: true` and no `by`;
its line says "automatically" where a person's says "by …". The log's
`update` line has `auto: true`, nobody as who; `log.sum.update` gets an
automatic form; `update_postponed` is a new kind (`log/kinds.ts`,
`log/text.ts`). An answer is audited as `update.answer`.

## Implementation Plan

### Block 1: ServerCheck

- `update_api.go`: `updateLoop` and `updateTick(ctx, now)`: a check at
  the start, then every 6 h, a failed one again after `checkEvery`;
  `updateRelease(ctx, true)`; only with a store and a valid channel.
- `server.go` `Run`: the loop under `s.wg`, after `logStarted`.
- Tests: the start's check through the fake finder; the next one 6 h on,
  a failed one 10 min on; one push per version; no loop without a
  channel; the guard against a test reaching GitHub.
- `docs/api-v1.md` ("GET /api/v1/update", the notify table's `update`):
  the server's own check; a fragment.

**Verification:** `go test ./internal/server/`, `gofmt -l .`, the three
`go vet`, `GUIDELINES/shared/tools/check`; the whole suites in CI.

### Block 2: SourceCheck

- `internal/update`: `DebTag` (from `cmd/massimo/update_apt.go`), an apt
  index reader (`Packages.gz`, https and `file:`, bounded, the arch) and
  a formula reader (`version "…"`, the raw URL); a source's newest as a
  tag.
- `internal/install`: `ReadAptSource` (URIs, suite, component).
- `UpdateOptions.Source`; `packagedUpdate` and `serveBrewUpdate` set it.
- `updateRelease`: `latest`, `available`, `released`, `source_error`
  (D8, D9); `held` after `up-to-date`, ended by a later read that has it;
  `can_update` false while held.
- `Software.tsx` `versionLine`: "v… is out; apt has it not yet", the
  hold's line; i18n, six languages.
- `docs/api-v1.md` and the OpenAPI file (`released`, `source_error`,
  `held`; `make api-types`); `docs/ui.md` "Software update";
  `packaging/README.md` ("Debian package", "Homebrew"): the server reads
  the index and the formula; a fragment.
- Tests: an index with several versions, gz, `file:`, the arch, the
  channel's suite before the switch; a formula's version; the rule
  (source behind, level, unreadable); the hold set and ended; the
  version line in `tools/ui-test/test_update.py`.

**Verification:** `make ui`, `go test ./internal/update/
./internal/install/ ./internal/server/ ./cmd/massimo/`, `gofmt -l .`,
the three `go vet`, the touched UI test files in their four cases,
`GUIDELINES/shared/tools/check`; the whole suites in CI.

### Block 3: NightInstall

- `internal/store/settings.go`: `UpdateAutoKey`, `UpdateWindowKey`;
  `settings_api.go`: both in `settingsView` and `putSettings`,
  `parseWindow`.
- `update_api.go`: `beginUpdate(ctx, userID, auto)`, shared with
  `startUpdate`; `radioBusy()`; `updateTick` installs in the window,
  once a night, the fresh check at its start and every 30 min.
- `update_history.go`: `auto` in the entry; `systemlog.go`: `auto` in
  the `update` line, `update_postponed`.
- `GET /api/v1/update`: `auto` (`mode`, `window`, `next`).
- Tests: the window (midnight, the bounds, refusals); each radio reason
  holds it; once a night across the restart; skip, hold, blocked and a
  running helper respected; a failed start ends the night; Windows'
  `Start` called.
- `docs/api-v1.md` (settings, update, the log's kinds), the OpenAPI
  file, `docs/update-windows.md`; a fragment.

**Verification:** `go test ./internal/server/ ./internal/store/`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`; the
whole suites in CI.

### Block 4: NightSetting

- `Software.tsx`: Automatic updates (Off, At night) and the window, two
  `TextField`s of type time, for a holder of `settings`, saved at once
  as the channel is; disabled with `blocked`; the next try's line.
- `historyLine`: "automatically"; `log/text.ts`, `log/kinds.ts`: the
  automatic `update`, `update_postponed`; i18n, six languages.
- Tests: `tools/ui-test/test_update.py` (the setting, the window, the
  line), `test_maint_updates.py` (the history's line).
- `docs/ui.md` "Software update" and the log's kinds; a fragment.

**Verification:** `make ui`, `go test ./internal/server/`, `gofmt -l .`,
the three `go vet`, the touched UI test files in their four cases,
`GUIDELINES/shared/tools/check`; the whole suites in CI.

### Block 5: UpdateAsk

- `update_api.go`: `update.answer`; `PUT /api/v1/update/answer`
  (`{"version", "answer"}`, capability `update`, `409` where the
  version is not the one offered), audit `update.answer`; skip and
  tonight in `updateTick`; `auto.answer` and `auto.ask`.
- The page: `updateDue` and `updateOut` for `update` too; a banner part
  in `#update-ask-slot` (`software/Ask.tsx`), Install now, Tonight, Skip
  this version, for a holder of `update`; the same answers in Software
  update; read again on the answer's change event; i18n, six languages.
- `docs/api-v1.md`, the OpenAPI file (`make api-types`), `docs/ui.md`
  (the banner, Software update, the server's sign); a fragment.
- Tests: the route (capability, stale version, audit), skip holds until
  a newer release, tonight ends the question; the banner and its three
  answers, nothing for a user without `update`
  (`tools/ui-test/test_update.py`, `test_server_sign.py`).

**Verification:** `make ui`, `go test ./internal/server/`, `gofmt -l .`,
the three `go vet`, the touched UI test files in their four cases,
`GUIDELINES/shared/tools/check`; the whole suites in CI.

## Proposed commit messages

The plan alone first, `chore(aps): +AutoUpdate v1.0`, on main; each
block's commit names `AP AutoUpdate v1.0, Block N` in its body.

Block 1,
`feat(server/auto-update): The server checks for a release at each start and every six hours`;
Block 2,
`feat(server/auto-update): A release is available only where apt or brew has it`;
Block 3,
`feat(server/auto-update): The server installs a release at night while the radio is quiet`;
Block 4,
`feat(ui/auto-update): Software update sets automatic updates and their window`;
Block 5,
`feat(ui/auto-update): The page asks Install now, Tonight or Skip this version`.

## Change History

- v1.0 (2026-11-24): the first revision, from the owner's EVAL of
  automatic updates and PLAN++ (D1-A, D2-A as set, D3-A, D4-A, D5-A+B).

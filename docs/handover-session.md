# Handover, 2026-11-05 evening: development moves to WSL on jaguar

For the next session, on a fresh checkout. Read README.md, then
docs/roadmap.md, then this file. The sessions that wrote today's code
ran on the Linux Mint box, which is being retired; they cannot move,
so this file is the whole context they leave behind.

Branches and releases: develop on `main` (pull requests go there, the
owner merges); `alpha` is the release branch and the update channel of
the alpha releases, promoted from `main` with `make release-alpha`
(`BUMP=minor` for a feature, nothing for a fix; README.md, "Versions").
Latest release: v0.5.0-alpha.1 (main at a7b9dba, 2026-11-05 evening).
Nothing is open on GitHub.

## Where things run

- **The house Cube** (serial JEQ0192422) is on the M7 network at
  .118 by DHCP. Reserve that address on the M7 router; it is not
  reserved yet.
- **The Raspberry Pi 1** (Raspberry Pi OS Lite 32-bit "trixie") runs
  `massimo serve` as a systemd service, installed with `massimo
  install`, on the alpha channel (v0.4.0-alpha.1 at the time of
  writing; it updates from Settings → Software). Fixed addresses on its
  interface: **.200** for the API and web UI on port 80 (`install
  -listen :80`), **.201** as the proxy for the house Cube
  (`MASSIMO_CUBE_JEQ0192422_PROXY`). Its DHCP address moves (.48, then
  .41); reserve one. Settings in `/etc/massimo/env`; the spare Cube is
  commented out there because it is not on that network. The M7 DHCP
  range is 20–199, so .200 and .201 are safe. packaging/README.md,
  "Moving to another network", has the nmcli commands.
- **The spare Cube** (KEQ0537249) is not connected anywhere. It is the
  one for the firmware dump (docs/firmware-extraction.md).
- **The Mac** (192.168.1.25, Apple silicon, macOS 27.2) had the
  LaunchDaemon test install; it was purged again. Left behind:
  `/etc/sudoers.d/massimo-test`, to delete (`sudo rm`), and an empty
  `/usr/local/bin`.
- **The Mint box** no longer runs `serve`. Its `.env` has the current
  house Cube entries (ADDRESS .118, PROXY .201) and stale entries for
  the spare Cube (.46, proxy .49). A new `~/.ssh/id_ed25519` on it is
  authorized on the Pi next to the owner's key; remove it from the Pi
  when the box goes.
- **Windows (jaguar)** is the new development machine, in WSL. The
  Windows service itself has only run on a CI runner;
  docs/testing-windows.md is the plan for a real PC.

Every command that talks to the house Cube goes through the Pi's
proxy: `massimo status -addr 192.168.1.201` and so on. A direct
connection while the Pi holds the Cube times out silently and, if the
Pi is between reconnects, steals its only slot (see "Learnings").

## Setting up the checkout on jaguar

1. Clone `github.com/metaworx/massimo`, Go 1.26 (go.mod says 1.24 as
   the minimum; CI pins 1.26), `gh auth login`, `make`, `make test`.
2. Copy `.env` from the Mint box by hand (it is gitignored and holds
   the keys and IVs); drop the spare Cube's stale address and proxy
   lines or leave them commented. The backups in the Mint project root
   (`cube*.yaml`, `massimo-backup-*.json`) are worth keeping as a
   history; `backups/` is gitignored.
3. Commit signing lives in the repository's local git config and is
   not cloned. Recreate it with the owner's ed25519 key:

   ```bash
   git config gpg.format ssh
   git config user.signingkey 'key::ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILcfANbHJwcC3lCnRobpeVQoy86O5imOIZ3Ev7NhvZoP martin.rueegg@metaworx.ch'
   git config commit.gpgsign true
   git config tag.gpgsign true
   git config gpg.ssh.allowedSignersFile "$(git rev-parse --git-dir)/allowed_signers"
   echo 'martin.rueegg@metaworx.ch ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILcfANbHJwcC3lCnRobpeVQoy86O5imOIZ3Ev7NhvZoP' > "$(git rev-parse --git-dir)/allowed_signers"
   ```

   The key is served by KeePassXC's SSH agent. In WSL that needs a
   bridge to the Windows agent (wsl-ssh-agent or npiperelay with
   socat); whatever the socket is, pass it as `SSH_AUTH_SOCK` to git.
   Without the key a commit fails rather than going unsigned, which is
   intended. A locked KeePassXC makes `ssh-keygen -Y sign` hang instead
   of failing, so run `timeout 60 git commit …`; on the Mint box the
   socket path under `/tmp/ssh-*/agent.*` changed on every relock.
   Commits before 2026-11-05 were signed with another key of the owner
   (SHA256:32dP…), so they show as "signer unknown" unless that key is
   added to `allowed_signers`.
4. The release signing key: the public half is
   internal/update/release-key.pem; the private half is the `signing`
   environment's `MASSIMO_RELEASE_KEY` on GitHub, usable only by `v*`
   tags. A copy of the private key still sits in a session scratchpad
   on the Mint box (`/tmp/claude-1000/…/scratchpad/release-key/`); the
   owner puts it into KeePassXC and deletes that directory. Two old
   `.env` copies and the Pi's console password are in the same
   scratchpad tree; same treatment.
5. `make ui-test` needs Python with Playwright and Chromium
   (tools/ui-test/README.md). On a box that already runs `serve`, the
   real-discovery checks are skipped with a 503 rather than failed,
   because `serve` always holds UDP 23272; a flag for the discovery
   listen address (`Options.DiscoveryAddr` exists, no CLI flag) would
   let them run everywhere.
6. WSL 2 in its default network mode has no LAN broadcast and no
   inbound LAN connections: `massimo discover`, the proxy and discovery
   answers from a `serve` in WSL do not work there (README, `discover`).
   Mirrored networking mode fixes that on Windows 11; otherwise keep
   every hardware-facing command on the Pi and develop against the fake
   Cube and the Pi's API.

## Conventions

- Commits: short imperative subject, a body in sentences saying why,
  the attribution lines used throughout the log; no model names in
  code or commits; signed (above).
- No keys, IVs, tokens, real device names or serials in fixtures, docs
  or commits; `.env`, backups and logs are gitignored. Known violation,
  owner's decision pending: the README, the API documents and the
  write-test report use the two real Cube serials and real room names.
  They are in the public history already; replacing them in the
  examples is cosmetic unless the serials count as sensitive.
- Pull requests against `main`, rebased onto `main` right before they
  are opened; stacked PRs cost a rebase after every merge and a force
  push the auto mode refuses, so merge `main` into a branch instead of
  rebasing a stack, and keep handover and roadmap edits to one small
  final commit per PR so parallel sessions do not conflict in them.
- UI: plain HTML/CSS/JS in internal/server/ui/, go:embed, no framework,
  no inline handlers, textContent for API text, every text through
  i18n.js, CSS variables for light and dark, phone first (390 px),
  verified with `make ui-test` at 1100 and 390 px before a merge.
- Protocol bytes only in internal/protocol/cube, tested against
  captured lines; hardware findings into docs/portal-protocol.md the
  day they are made; the coding standard is at the end of
  docs/review-2026-11-05.md.
- Parallel work: one session per area (core Go, UI, packaging,
  hardware), each in its own worktree and set of files; a coordinating
  session hands out the work and relays decisions.

## Done on 2026-11-05, by pull request

- #3, #5: the owner's UI feedback (week programme editor, Copy tab,
  help buttons, six languages), UI follow-ups, `make ui-test`, docs/ui.md.
- #4: the review's second tier: internal/view, internal/cubeio,
  internal/env, "hidden" as the one term, `-listen` everywhere (old
  flags kept), the proxy refuses `t:` for hidden devices, `serve
  -refresh` (every six hours), the serve bug with a lone
  `MASSIMO_CUBE_ADDRESS`.
- #6: govulncheck in CI, Go 1.26 toolchain, the Windows service.
- #7: lockout after ten wrong tokens a minute, single-use stream
  tickets (`?token=` on the stream deprecated, answers with a
  Deprecation header).
- #8: timestamps per value (`received`, `last_seen`, `stale`,
  `observed_since`).
- #9, #14, #19: the hardware write test, steps 1 to 7, all passed
  (docs/write-test-2026-11-05.md).
- #10: pairing, server side (`POST/GET/DELETE …/pairing`, `paired`
  events, `PUT …/devices/{rf}` with `room_id`, pairing through the
  proxy, inclusion mode in the fake Cube).
- #11: after writes the server re-reads the device's C line (15 s,
  30 s, 1, 2, 2 min) and reconnects once for a fresh budget if
  radiograms were pending (at most every 15 min, only with `-refresh`);
  accepted commands and wakeups in the log.
- #12: ages and stale warnings in the UI, the "Open" window badge on
  the tile, the stream opened with tickets.
- #13: macOS LaunchDaemons for `install` and `update`, tested on the
  Mac and in CI; `MASSIMO_GITHUB_TOKEN` for the release check.
- #15, #16, #17: discovery from `serve` (at start, every 30 min,
  `POST /api/v1/cubes/discover`), unconfigured Cubes in `GET
  /api/v1/cubes`, `cube_found` events, Cubes configured by serial only
  follow their discovered address; "Search for new Cubes…" in the UI;
  `started_at` on `/healthz`; blue accent, red only for alerts.
- #18: device type and radio address only in expert mode.
- #20, #21: state events say what changed and who changed it (server,
  another program, someone at the device); the log view shows it
  ("window opened", "set 18.0 → 21.0", "reports again").
- Releases v0.2.0-alpha.1 to v0.5.0-alpha.1; the Pi and the Mac took
  updates through the helper.

## Decisions taken

- The budget reconnect after radio writes (#11) stays as built. The
  Cube reports its budget only in the H line of a new connection and
  in S replies; the alternatives (estimate, on demand, drop) are in
  the coordinating session's notes; an on-demand refresh endpoint can
  come later.
- The Copy tab copies the room's stored settings, not unsent edits of
  the other tabs, and says so.
- `?token=` on the event stream stays for one release and then goes.
- Room-level configuration writes reset only the first device's error
  state (review, logic 3); documented, not changed.

## Open decisions for the owner

- Real serials and room names in the docs (above).
- Users and a database (roadmap 6): a design document first (owner
  arbitration per Cube, what moves from `.env`, the permission model),
  no code before that.
- macOS: notarization needs a Developer ID; a `.pkg` or Homebrew
  formula; log rotation (newsyslog cannot copytruncate, so `serve`
  would have to reopen its log on SIGHUP).
- The stale threshold (`view.StaleAfter`, 24 h, thermostats only) once
  real data shows what fits.

## Hardware checks still owed

Nothing below has been tried at a Cube or on the boxes yet; each PR
says what to run.

1. Pairing (#10): the six steps in the PR; the assumptions marked
   unverified in docs/portal-protocol.md, pairing section (no reply to
   `n:`/`x:`, an empty `N:` when the mode ends, several devices per
   inclusion).
2. Write test step 1 repeated, to watch the follow-up re-reads and the
   budget reconnect (#11) in the log and the stream.
3. Discovery on the Pi with the spare Cube on the same network: it
   must show up after "Search for new Cubes…", and the Pi's own proxy
   answers must not.
4. UI on the boxes: real ages and a stale thermostat, a window opening
   live, a stream reconnect while `serve` restarts, the blue accent (the
   owner's last screenshot was a pre-#17 build), the log texts from
   #21 with real device changes.
5. Windows service on a real PC: done on jaguar on 2026-11-05, all nine
   steps passed (v0.4.0-alpha.1, then install and remove again on
   v0.5.0-alpha.1 with the lone `MASSIMO_CUBE_ADDRESS`); the report with
   its follow-ups is docs/windows-test-2026-11-05.md, the plan
   docs/testing-windows.md.
6. macOS: a reboot (RunAtLoad with nobody logged in), the application
   firewall rule across an update with a new cdhash, Intel Macs, a real
   Cube or the proxy on a Mac.

## Code follow-ups found today

- `status`, `backup` and the other commands fall back from a configured
  proxy to the Cube itself when the proxy does not answer, for example
  during the few seconds the update helper restarts `serve`, and then
  hold the Cube's only slot while `serve` fails to reconnect. No direct
  fallback when a proxy is configured, or only with a flag.
- Pairing does not copy the room's settings (0x40/0x11/0x12/0x10) to a
  new device as the original software does; removing a device (`t:`)
  has no API; the UI has no pairing panel yet (docs/api-v1.md, Pairing).
- Writes from proxy clients (the original software through the proxy)
  get no follow-up re-read; the server only sees raw lines.
- The UI may want to ignore the short `connected: false` during the
  six-hourly refresh and the budget reconnects.
- Server error texts reach the UI in English; the four machine
  translations (FR, IT, NL, PL) await a native speaker (delete a
  language's `i18n.machineNote` once reviewed).
- Updates from the web UI on Windows (a helper and the server's
  endpoint, which still refuses Windows).
- go.mod to 1.25 or 1.26 so `x/sys` can be ≥ v0.44 (GO-2026-5024;
  review, dependencies).
- `packaging/.gitattributes` with `* text eol=lf`: a Windows checkout
  with autocrlf would embed CRLF unit files (the plists are normalised
  on read; the systemd units are not).
- `serve -log` writes to the file and to stderr; under a supervisor
  that captures stderr every line appears twice (systemd: journal plus
  file, wanted; launchd: avoided by not passing `-log`).
- `massimo discover` could show the DHCP flag from the Cube's reply.
- docs/api-v1.md says the first device list carries `observed_since`;
  it arrives a few hundred microseconds later, and the UI treats
  "within 5 s" as already there. Align the doc or the timestamps.
- Housekeeping: delete the merged remote branches
  (claude/pi-install-c5930f, claude/youthful-mirzakhani-9ed831,
  claude/log-after-connect) and the local branches in the Mint
  worktrees; on the Pi, `~/massimo-alpha12.*`,
  `/usr/local/bin/massimo.alpha9.bak` and, once the M7 setup is
  settled, `/etc/massimo/env.before-m7` (it holds secrets); keep
  `/usr/local/bin/massimo.prev`, the update helper's rollback.

## Learnings from the hardware day

The ones already in packaging/README.md ("Moving to another network",
"macOS", "Updating") and docs/write-test-2026-11-05.md are not
repeated.

- A Cube with a client drops a second client's SYNs silently: a direct
  test connection times out and looks like a dead Cube. Test through
  the proxy.
- A Cube that moved networks or booted during an unstable network can
  answer ping but not TCP 62910 or UDP 23272; a power cycle fixes it.
  Twice today.
- Removing the IP address `serve` used as its source before restarting
  `serve` leaves the Cube holding a dead session (the FIN cannot be
  sent); the Cube needed a power cycle. Restart `serve` first, then
  change addresses. Linux picks the first static address on the
  interface as the source for Cube connections, not the DHCP one.
- Two routers on the same 192.168.1.0/24 (wired and Wi-Fi) on one box
  make routing ambiguous; the Wi-Fi side needed SO_BINDTODEVICE.
- A Cube restart drops undelivered radiograms. A device move whose
  radiograms are lost leaves the metadata right and the device's links
  half done; the only sign is the device's link_error flag. The fix is
  repeating the move.
- A window contact fetches radiograms only right after it transmits
  and not always all at once: a move needed two window cycles per
  direction, with an error reset first. Read the pending count fresh
  from the Cube to know when a move is done.
- Functional link check for a contact: window open → that room's
  thermostat goes to its window-open temperature within a minute,
  other rooms do not react, and it goes back after closing.
- Cost: about 1.25 % duty cycle per radio configuration write, about
  8 % per device move; 1 % per hour is about 36 s of air time.
- The Cube sends its H line (budget, free slots, hence pending
  radiograms) only when a connection starts and in S replies; no
  command asks for it. The Cube changes a device's C line only after
  the device acknowledged (1 to 4 min); the setpoint shows in L at
  once, the mode only with the device's report.
- The Cube answers UDP discovery only to port 23272, whatever the
  request's source port, so on the box running `serve`, `massimo
  discover` sees only `serve`'s own proxy answer; use `POST
  /api/v1/cubes/discover` there. Cubes default to DHCP (the `c` reply
  carries the flag; factory fallback 192.168.0.222/16).
- After a Cube restart, the eco switch stays "not reported" until
  someone presses it; harmless.
- Pi 1: Ethernet only, no networking over USB; current Pi OS 32-bit
  still supports it and uses cloud-init user-data; about 9 MB RSS;
  memory cgroup accounting is off, so `systemctl status` shows no
  memory line; first boot about 10 min. The version command is
  `massimo version`, there is no `-version` flag.
- macOS: a signed binary overwritten in place gets SIGKILL; install
  and update rename into place. launchd tests without root work in the
  `gui/<uid>` domain only while that user is logged in at the console;
  `user/<uid>` fails over SSH with "Bootstrap failed: 5"; SIP blocks
  `launchctl setenv` in the system domain, so a root helper gets its
  environment only through its plist. GitHub's API allows 60
  requests an hour per address without a token.
- For a sudoers line with a command list, check with
  `sudo -n <allowed command>`, not `sudo -n true`.
- Firmware dump: parts ordered or to order (FT232H, 2x10 header, Pomona
  5250 clip); pin 1 of ST3 still to be measured
  (docs/firmware-extraction.md).

## Next steps in order

1. Set the checkout up on jaguar (above), carry the secrets, delete
   the copies on the Mint box, reserve the M7 addresses, delete the
   Mac's sudoers file.
2. The hardware checks above, pairing first, at the Cube with the
   owner.
3. The pairing panel in the UI (docs/api-v1.md, Pairing), then copying
   the room's settings to a new device and an API to remove a device.
4. The CLI's proxy fallback and the other small follow-ups above.
5. Users and a database: the design document, then the owner's
   decisions, then code (roadmap 6).
6. Firmware dump when the adapter is in (docs/firmware-extraction.md).
7. Loxone (roadmap 10).

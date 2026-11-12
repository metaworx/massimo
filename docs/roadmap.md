# Roadmap

What is still to be done, in order.
The frame is docs/design.md: target environments, architecture, drivers,
Loxone ownership, multi-user and the public server, with their phases.
What is done is in CHANGELOG.md, one section per release;
GUIDELINES/wip/HANDOVER.md is the context a session needs to work here.

1. **On the hardware**, by the owner, with a session watching the log:
   3. Pairing (docs/api-v1.md, "Pairing"): the six steps of pull request #10,
      the assumptions marked unverified in docs/portal-protocol.md,
      the copy of the room's settings on a real thermostat,
      `t:` on a device that answers.
   4. Write test step 1 repeated, to watch the follow-up re-reads and the
      budget reconnect in the log and the stream,
      once from the original software through the proxy.
   5. The UI on the boxes: real ages and a stale thermostat,
      a window opening live, a stream reconnect while `serve` restarts,
      the log texts with real changes.
   6. Windows on a real PC: docs/testing-windows.md step 7b (the Update button).
   7. macOS: a reboot with nobody logged in, the firewall rule across an update,
      an Intel Mac, a real Cube or the proxy on a Mac,
      the formula with `brew services`.
   8. netconfig: a broadcast `C` to a Cube on a wrong subnet;
      `discover` on a Cube with a static address.
   9. Discovery on the Pi with the spare Cube on the same network,
      with the DHCP flag in the found row.
2. **Packaging, the owner's steps** (the packaging plan's NOTE,
   "The owner's steps left"):
   the checks "Debian package (debian:trixie-slim)" and "(debian:bookworm-slim)"
   added to the ruleset on `main`;
   the repository `metaworx/homebrew-massimo` created,
   its deploy key in `HOMEBREW_TAP_DEPLOY_KEY` of the `signing` environment,
   then `gh workflow run publish.yml --ref <tag> -f tag=<tag>` once;
   until then the tap job skips itself.
   The first release with the deploy key set proves the Homebrew job.
3. **Packaging, the rest**
   (AP Packaging v1.0, Blocks 4 to 10, each a `PLAN+` revision when wanted,
   else `EXEC retire`): Windows code signing (none for now;
   SignPath Foundation first, Certum as fallback), rpm, the MSI and winget,
   a container image, the LoxBerry plugin, a notarized `.pkg` with a cask.
4. **The store, after stage (e)** (docs/store.md section 9):
   ownership per Cube, `-server` for the commands, secrets in the store,
   the export and import of design.md section 7, the PostgreSQL driver.
5. **The UI when the server is gone**:
   a prominent notice, edits blocked, polling until it is back; brief ready.
6. **Settings → Server** in the UI:
   the intervals and the stale threshold, the certificate's details
   (in the header's expert list for now) and `log.keep_days`,
   which the settings API already takes.
7. **TLS, later**, each a plan of its own: UPnP, a local CA,
   a Pebble job in CI so that a test reaches an ACME server;
   the DNS hooks checking for `curl` and `jq`,
   which the Debian package does not name (HookDeps).
8. **Translations**:
   a native speaker's review of FR, IT, NL and PL;
   the server's error texts in the UI's language.
9. **Release engineering**:
   macOS notarization; log rotation through SIGHUP; the `edge` mode.
10. **Firmware dump** of the spare Cube once the FT232H is in
    (docs/firmware-extraction.md),
    then the DataFlash and the portal reply format.
11. **Decisions for the owner**:
    the final set of built-in colour schemes (Massimo,
    High contrast and Graphite ship);
    real Cube serials and room names in the documents,
    the test reports and docs/screenshots (a fixture with placeholder names
    would take them out of the set);
    the items of docs/store.md section 10 marked open,
    and docs/server.md's once the server mode starts.
12. **Small ones**:
    the stale threshold once real data shows what fits;
    the UI ignoring the short `connected: false` of the six-hourly refresh and
    the budget reconnects; `serve -log` writing twice under a supervisor that
    keeps stderr; `detailsPanel` and `stepper` in components.js, written and
    without a caller; a UI test for each new panel in tools/ui-test/;
    `api: token not resolved` logged at ERROR for a request cancelled
    during the auth check;
    the update's own command marked dropped at the next start;
    a note in README.md and docs/update-windows.md
    that a Windows service created before v0.7.0 needs `install` again
    before the Update button works;
    `install` warning that the API is open to the LAN
    while the server signs users in (AuthWarning).
13. **Housekeeping**:
    the `.env` copies and the release key in the Linux test box's session scratchpad,
    its SSH key on the Pi; the M7 reservations for the house Cube and the Pi;
    `/etc/sudoers.d/massimo-test` on the Mac; on the Pi `~/massimo-alpha12.*`,
    `/usr/local/bin/massimo.alpha9.bak` and, once the M7 setup is settled,
    `/etc/massimo/env.before-m7` (keep `massimo.prev`);
    on the development PC the merged branches `claude/tls*`, `backup/claude/tls*`,
    `claude/ci-update-race`, `claude/heuristic-noether-90a440`,
    `claude/store-c` and `claude/store-d*`,
    and the worktree `xenodochial-pasteur-c0e0c8` once its session is gone.
14. **Later**:
    the server mode with encryption at rest and sync (docs/server.md),
    Loxone (design.md section 4), a radio-stick driver without a Cube.
16. **An Android app**:
    run as a stand-alone service, or connected to an existing Massimo;
    published on F-Droid and on Google Play.
17. **An iOS app**:
    the same as the Android app; published on the App Store.
19. **Channels, the next steps** (AP Channels v1.0):
    apt suites and Homebrew formulae for `devel` and `nightly`,
    with an apt key of their own;
    `beta` and `stable` with `make release-beta` and its stable counterpart,
    `rc` as `-rc.N` tags on `beta`,
    and a suite and a formula taking every more stable flavour;
    the channel's selector in the UI (System settings → Software);
    "All tests passed" as the ruleset's one required check;
    the old `alpha` branch deleted once `release/alpha` has had a release;
    the guidelines at v4.54.0 or later, for `changelog.sh preview` in the
    rolling builds' notes.

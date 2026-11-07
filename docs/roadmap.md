# Roadmap

What is still to be done, in order.
The frame is docs/design.md: target environments, architecture, drivers,
Loxone ownership, multi-user and the public server, with their phases.
What is done is in CHANGELOG.md, one section per release;
docs/handover-session.md is the context a session needs to work here.

1. **On the hardware**, by the owner, with a session watching the log:
   1. The update to the newest alpha on the Raspberry Pi and on jaguar's Windows
      service (Settings → Software): the store created,
      seeded and migrated (`store created`, `cube added from the settings file`,
      `owner created from MASSIMO_API_TOKEN`), `massimo store info`,
      Settings → Cubes and Settings → Users,
      every client still in with its token,
      a manager's settings following them to another browser,
      the log paging back and filtering with real events.
   2. TLS (the TLS plan's NOTE, item 10): a certificate from Let's Encrypt's
      staging directory on the Pi with a name and the router's port-443 forward
      (`serve -tls acme -tls-names <name> -acme-directory <staging URL>`,
      or `install` with the same flags);
      the same through the Infomaniak hook
      (`-acme-dns-hook packaging/acme-dns-hooks/infomaniak.sh`,
      the token in `/etc/massimo/infomaniak.token` readable by the service user,
      `curl` and `jq` installed; the hook ran against a fake of the API only);
      the self-signed notice on a phone.
      HSTS is on for a year once TLS serves:
      a box that goes back to `http://` runs once with `-hsts 0` first.
      Then `EXEC retire` for AP Tls v1.1.
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
   10. The Debian package on the Pi once the apt repository is up
       (`sudo massimo install -remove`, the apt snippet,
       `sudo apt install massimo`, `sudo massimo install -listen :80`):
       the service's start, which the container test cannot prove.
2. **Packaging, the owner's steps** (the packaging plan's NOTE,
   "The owner's steps left"):
   the checks "Debian package (debian:trixie-slim)" and "(debian:bookworm-slim)"
   added to the ruleset on `main`;
   the repository `metaworx/homebrew-massimo` created,
   its deploy key in `HOMEBREW_TAP_DEPLOY_KEY`,
   then `gh workflow run publish.yml -f tag=<newest release>` once;
   until then the tap job skips itself.
   The first release with the deploy key set proves the Homebrew job.
3. **Packaging, the rest**
   (AP Packaging v1.0, Blocks 4 to 10, each a `PLAN+` revision when wanted,
   else `EXEC retire`): Windows code signing (none for now;
   SignPath Foundation first, Certum as fallback), rpm, the MSI and winget,
   the UI's update button upgrading through apt or brew, a container image,
   the LoxBerry plugin, a notarized `.pkg` with a cask.
4. **The store, after stage (e)** (docs/store.md section 9):
   ownership per Cube, `-server` for the commands, secrets in the store,
   the export and import of design.md section 7, the PostgreSQL driver.
   The next migration is 0006.
5. **The UI when the server is gone**:
   a prominent notice, edits blocked, polling until it is back; brief ready.
6. **Settings → Server** in the UI:
   the intervals and the stale threshold, the certificate's details
   (in the header's expert list for now) and `log.keep_days`,
   which the settings API already takes.
7. **TLS, later**, each a plan of its own: UPnP, a local CA,
   a Pebble job in CI so that a test reaches an ACME server.
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
    without a caller; a UI test for each new panel in tools/ui-test/.
13. **Housekeeping**:
    the `.env` copies and the release key in the Mint box's session scratchpad,
    its SSH key on the Pi; the M7 reservations for the house Cube and the Pi;
    `/etc/sudoers.d/massimo-test` on the Mac; on the Pi `~/massimo-alpha12.*`,
    `/usr/local/bin/massimo.alpha9.bak` and, once the M7 setup is settled,
    `/etc/massimo/env.before-m7` (keep `massimo.prev`);
    on jaguar the merged branches `claude/tls*`, `backup/claude/tls*`,
    `claude/ci-update-race`, `claude/heuristic-noether-90a440`,
    `claude/store-c` and `claude/store-d*`,
    and the worktree `xenodochial-pasteur-c0e0c8` once its session is gone.
14. **Later**:
    the server mode with encryption at rest and sync (docs/server.md),
    Loxone (design.md section 4), a radio-stick driver without a Cube.

# Roadmap

The frame for this list is docs/design.md: target environments, architecture,
drivers, Loxone ownership, multi-user and the public server, with their phases.
What is done is in CHANGELOG.md, one section per release;
this file lists what is open, in order.
docs/handover-session.md says where things run and what is owed on hardware.

1. **The store, stages (b) to (e)** (docs/store.md section 9): users, login,
   the capability model, the audit and `commands` with `Idempotency-Key`;
   then the preferences with the colour-scheme sync,
   the log in the store with retention, and the backup history.
   Stage (a) shipped in v0.7.0-alpha.1 and has not run on a real box yet;
   stage (b) is in progress in a session on the branch `claude/store-b`.
2. **Backup and restore in the web UI**:
   the API is in (plan, send, progress); the Settings panel is open,
   a session brief is ready; written from the UI's components (docs/ui.md,
   "How the page is built").
3. **Pairing**:
   the hardware test of the server side, then the panel in the UI,
   copying the room's settings to a new device as the original software does,
   and an API to remove a device.
4. **The UI when the server is gone**:
   a prominent notice, edits blocked, polling until it is back; brief ready.
5. **TLS for `serve`**:
   certificate files by flags with reload, self-signed at first start,
   the reverse-proxy support (`-trusted-proxies`, relative URLs in the UI),
   autocert with TLS-ALPN-01; a DNS-01 hook and UPnP later.
   The plan is being written in a session from the evaluation of 2026-11-06.
6. **Settings → Server** in the UI:
   the intervals and the stale threshold the API already takes.
7. **UI tests**:
   the smoke test runs in CI's Screenshots job since 2026-11-06;
   docs/ui-testing.md recommends moving it to pytest with Playwright,
   not Cypress, and says what a fuller suite would cover.
8. **Translations**:
   a native speaker's review of FR, IT, NL and PL;
   the server's error texts in the UI's language.
9. **Release engineering**:
   Windows step 7b on a real PC; macOS notarization, a `.pkg` or Homebrew,
   log rotation through SIGHUP; the `edge` mode.
10. **Firmware dump** of the spare Cube once the FT232H is in
    (docs/firmware-extraction.md),
    then the DataFlash and the portal reply format.
11. **Small ones**:
    the stale threshold once real data shows what fits;
    discovery on the Pi with the spare Cube;
    the DHCP flag in the server's discovery; `serve -log` under a supervisor.
12. **Later**:
    the server mode with encryption at rest and sync (docs/server.md),
    Loxone (design.md section 4), a radio-stick driver without a Cube.

# AP AnnounceSetting v1.0: The announcement from the store

## TL;DR

Whether `serve` announces itself on the LAN becomes a setting of the
store, `announce` (`auto`, `on`, `off`), switched in System settings →
Maintenance and applied at the next restart. `-announce` and
`MASSIMO_ANNOUNCE` still win where given, and the page then shows the
switch read-only.

- **The decision moves:** from `cmd/massimo/serve.go`, before the store
  opens, → `serve.open`, after it: flag or variable, else the setting,
  else `auto` (on unless `-listen` is a loopback address).
- **`GET /api/v1/settings`:** `announce` with `in_effect` (`on` or
  `off`, this run's), `applies: restart`, and `fixed_by` where the flag
  or variable decides.
- **Maintenance:** **Announce on the LAN**: automatic, on or off, beside
  Restart (D14-A); read-only with "set by MASSIMO_ANNOUNCE" (D15-A).
- **On the Pi:** on in the page, a restart, and no line in
  `/etc/massimo/env`.

### Blocks

1. The setting, `serve`'s decision and the API
2. The switch in System settings → Maintenance

## Discussion

The owner asked on 2026-11-14 (AP IntegrationUsers, after Block 5)
whether `MASSIMO_ANNOUNCE=on` must be in the env file, or can be a
parameter, or set in the UI through the store.
It can be the variable or `-announce=on`, but no setting, since the
switch was read before the store opens.
On the Pi `serve` listens on the loopback behind its reverse proxy,
so `auto` keeps the announcement off there,
and Home Assistant cannot find the server by zeroconf.

**The owner's decisions** (`PLAN++ D14-A D15-A`, 2026-11-14):

- **D13-A:** a setting `announce` (`auto`, `on`, `off`), applied at the
  next restart; the flag and the variable still win.
- **D14-A:** the switch in the Maintenance card, beside Restart.
  Not B, the Integrations card: the announcement serves the apps and
  `massimo discover` as well.
- **D15-A:** where the flag or variable decides, the switch is shown
  read-only with what sets it.
  Not B, the setting over the env file.

## Analysis

- **`cmd/massimo/serve.go`:** `newSwitchFlag("announce", env.Announce, …)`
  gives `announceOn` with `set` where `-announce` or `MASSIMO_ANNOUNCE`
  was given; otherwise `announceOn.on = !loopbackListen(*listen)`.
  `serve.Config.Announce.On` carries the result.
- **`internal/serve/serve.go`:**
  - `open` reads the restart settings (`poll_interval` and the others)
    after the store opens, a value given winning;
    the announcement belongs there too.
  - `cfg.Announce.On` starts the responder (`Run`) and the look for
    Home Assistant (`FindIntegrations`).
  - Without a store (`-addr`) nothing is announced, as today.
- **`internal/server/settings_api.go`:**
  - `settingView` has `Value`, `Default`, `Set`, `InEffect`, `Applies`
    and `ReadOnly`;
  - `update.channel` is the model of a choice setting whose value a
    variable can override (`in_effect` shows the variable's).
  - A field `fixed_by` says what overrides it, for the page.
- **`internal/server/ui/`:** the Maintenance card
  (`partials/maintenance.html`, `update.js`) holds Restart, Reboot and
  the settings file; the choice goes there with "applies at the next
  restart", and Restart stays next to it.
- **The state:** an `auto` that keeps the announcement off says so in
  `in_effect`; nothing else changes for a box with no setting.

## Implementation Plan

### Block 1: The setting, `serve`'s decision and the API

1. **`internal/store/settings.go`:** `AnnounceKey = "announce"`, its
   values `auto`, `on`, `off`.
2. **`internal/serve`:**
   - `Announce` gains `Given` (the flag or variable decided) and
     `FixedBy` (`-announce` or `MASSIMO_ANNOUNCE`);
   - `open` decides: given, else the setting, else `auto` by the
     listener's address (`loopbackListen` moves here, beside
     `ListenHost`);
   - a setting that is none of the three is logged and taken as `auto`.
3. **`cmd/massimo/serve.go`:** passes `Given` and `FixedBy` instead of
   deciding.
4. **`internal/server`:**
   - `Options.Announce` (`On`, `FixedBy`), for `in_effect`;
   - `GET /api/v1/settings`: `announce` with `applies: restart` and
     `fixed_by`;
   - `PUT` takes `auto`, `on`, `off` or `null`.
5. **Tests:**
   - `serve`: the order (given, the setting, `auto` on the loopback and
     on a LAN address) and a bad value;
   - the settings route: the values, a refusal, `fixed_by`.
6. **Documents:** README ("Server"), `docs/store.md` (the env table: the
   variable or the setting), `docs/api-v1.md` and the OpenAPI file
   ("GET and PUT /api/v1/settings").
7. **`CHANGELOG.d/Added/`**, a fragment.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `-race` on `serve`, `make cross`, `gofmt -l .`, `go vet` on three
  platforms, `make lan-test` (the announcement's start),
  `GUIDELINES/shared/tools/check`.

### Block 2: The switch in System settings → Maintenance

1. **`partials/maintenance.html` and `update.js`:** **Announce on the
   LAN** with *Automatic*, *On* and *Off*, its hint saying what
   automatic does and that it applies at the next restart, with whether
   the server announces now; read-only with "set by …" where `fixed_by`
   is given (D15-A).
   Shown with `settings`.
2. **Six languages;** `docs/ui.md`.
3. **`tools/ui-test/`:** the choice saved and read back, the hint, the
   read-only state; at 1100 and 390 px.
4. **`CHANGELOG.d/Added/`**, a fragment.

- **Verification:** `make ui-test`, the Go gate, the guidelines check.

## Proposed commit message

For Block 1:

```
feat(serve/announce): Switch the announcement in the store

Whether serve announces itself on the LAN is the setting announce (auto, on or off), applied at the next restart; -announce and MASSIMO_ANNOUNCE still win where given (AP AnnounceSetting v1.0, Block 1).

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-14): first revision, D13-A, D14-A, D15-A.

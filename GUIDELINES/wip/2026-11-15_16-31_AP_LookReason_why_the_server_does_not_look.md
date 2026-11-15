# AP LookReason v1.0: Why the server does not look

## TL;DR

Where the server does not look for Home Assistant on the LAN, the
Integrations card says why and what to change, instead of "it is not
announced there". The server works the reason out and gives it in
`GET /api/v1/integrations/found`, which needs `integrations` alone.

- **The answer:** `looks: false` → also `announce: {value, in_effect,
  fixed_by, why}`, `why` one of `fixed`, `restart`, `off`, `loopback`
  (D19-A, D20-A).
- **The reason:** the server's own, from what decided at the start and
  the setting stored now, not worked out by the page from the settings.
- **The card:** four texts in six languages; **Open Maintenance**, with
  `settings`, but for `fixed`; the restart is there (D22-A).
- **One block, one pull request** (D21-A).

### Blocks

1. The reason in the found answer, its texts and the link

## Discussion

The owner asked on 2026-11-15, looking at the Integrations card,
what "This server does not look on the LAN: it is not announced there."
means, and proposed naming the setting and its state.
The text is right but says neither the setting nor what to do.

**The owner's decisions:**

- **D17-A** (2026-11-15): a message naming Announce on the LAN, its
  state and the way out, in place of the one there is.
- **D19-A** (`PLAN++`, 2026-11-15): the server gives the reason;
  not B, the page working it out from `GET /api/v1/settings`.
- **D20-A:** `announce` with `value`, `in_effect`, `fixed_by` and `why`,
  only where `looks` is false; not B, `why` alone.
- **D21-A:** one block and one pull request; not B, server and page apart.
- **D22-A:** in the restart case the link to Maintenance alone, where
  Restart is; not B, a restart button on the card.

## Analysis

- **When the server looks:** `serve.open` passes `FindIntegrations`
  exactly where `cfg.Announce.On && st != nil`
  (`internal/serve/serve.go`), and the found answer's `looks` is
  `FindIntegrations != nil` (`internal/server/integrations_found.go`).
  Not looking is the announcement off for this run.
- **Why it is off** is decided at the start (`announceSetting`):
  `-announce` or `MASSIMO_ANNOUNCE` where given (`FixedBy`), else the
  stored `announce`, `on` or `off`, else `auto`, on unless `-listen` is a
  loopback address (`LoopbackListen`).
- **Why the page cannot tell:** `GET /api/v1/settings` shows `value`,
  `in_effect` and `fixed_by`. A run started with `off`, the setting then
  set to `auto` on a server reachable from the LAN, shows `auto` and `off`
  with no `fixed_by`, the loopback's signature, while the next start will
  announce. Nor does a holder of `integrations` alone read the settings.
- **What the server needs:** `server.AnnounceState` has `On` and
  `FixedBy`; it gains `Loopback`, whether `-listen` is a loopback address,
  set by `serve.open`. With the setting read when the answer is made,
  the reason is, in this order:
  1. `fixed`: `FixedBy` given, which wins over any setting;
  2. `restart`: the stored setting, `on`, or `auto` off the loopback,
     announces at the next start;
  3. `off`: the setting is `off`;
  4. `loopback`: the setting is `auto` and `-listen` a loopback address.
- **The page** (`integrations.js`, `renderFound`): where `looks` is false,
  `foundOff` is shown; it becomes the text of `announce.why`, an unknown
  or missing one keeping a plain "does not look" text.
  The link opens Maintenance as the server sign opens Software update
  (`showView("maint")`, `showCard($("sys-maint"))`, `app.js`).
- **The tests' server** listens on `127.0.0.1`, so the UI suite sees
  `loopback` as it starts; a stored `off` or `on` gives the other two
  without a restart. `fixed` needs a start with the flag: the Go test.
- **Open pull request #293** (PageTabs) touches `app.js`, `index.html`
  and `components.js`, none of the files here but `docs/ui.md`.

## Implementation Plan

### Block 1: The reason in the found answer, its texts and the link

1. **`internal/server`:**
   - `AnnounceState.Loopback`;
   - `getFoundIntegrations`: where `looks` is false, `announce` with
     `value` (the stored setting, else `auto`), `in_effect` (`off`),
     `fixed_by` and `why`, by the order of the Analysis;
     a store error fails the request as the settings do.
2. **`internal/serve/serve.go`:** `Loopback: LoopbackListen(cfg.Listen)`.
3. **`internal/server/ui/integrations.js`:** the text by `why`, with
   **Open Maintenance** for a holder of `settings` but for `fixed`.
4. **Six languages:** `integrations.foundOff`, `foundLoopback`,
   `foundRestart`, `foundFixed`, `foundNot` (no reason given) and
   `openMaintenance`, naming Announce on the LAN, On and Automatic as
   Maintenance's own labels do; FR/IT/NL/PL machine-translated.
   English:
   - off: "This server does not look for Home Assistant on the LAN:
     Announce on the LAN is Off. Set it to On in Maintenance and restart
     the server."
   - loopback: "… Announce on the LAN is Automatic, and the server listens
     only on this machine (loopback). Set it to On in Maintenance and
     restart the server."
   - restart: "This server does not look for Home Assistant on the LAN
     yet: Announce on the LAN applies at the next restart. Restart the
     server."
   - fixed: "… {by}=off decides. Change it where the server is started."
5. **Tests:**
   - `TestFoundIntegrations`: each `why`, the `restart` case from `off`
     at the start and `auto` stored, and no `announce` where it looks;
   - `tools/ui-test/test_integrations.py`: `loopback` as the suite
     starts, `off` and `restart` after a stored setting, the link opening
     Maintenance, no link without `settings`; at 1100 and 390 px.
6. **Documents:** `docs/api-v1.md` ("Integrations") and the OpenAPI file
   for `announce`; `docs/ui.md` for the card's text and link.
7. **`CHANGELOG.d/Changed/`**, a fragment.

- **Verification:** `MASSIMO_ENV_FILE=/dev/null go test ./...`,
  `make race`, `make cross`, `gofmt -l .`, `go vet` on three platforms,
  `make ui-test` with `test_integrations.py`, then the whole suite,
  `GUIDELINES/shared/tools/check`.

## Proposed commit message

```
feat(ui): The Integrations card says why the server does not look on the LAN

Where the server does not look for Home Assistant, GET /api/v1/integrations/found gives the reason, and the card names Announce on the LAN, its state and what to change, with a link to Maintenance (AP LookReason v1.0, Block 1).

Key changes:
- internal/server: announce with value, in_effect, fixed_by and why in the found answer
- internal/server/ui: the text by the reason, Open Maintenance, six languages
- docs: the found answer, the card

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-15): first revision, D17-A, D19-A, D20-A, D21-A, D22-A.

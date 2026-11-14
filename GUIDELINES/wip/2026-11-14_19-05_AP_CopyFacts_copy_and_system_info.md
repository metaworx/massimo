# AP CopyFacts v1.0: copying facts, and System Info complete

## TL;DR

The facts a person reads out or pastes elsewhere copy with a click, and
System Info gathers the server's facts that Maintenance held.

- a copy component → click on the text, or a copy icon beside it;
  "Copied" said and shown; a fallback where the page is plain HTTP
- the header's version → copies the full version, without "(server
  version)"
- System Info → the server's version and each Cube proxy's address
  copyable, with the icon
- Maintenance's **Store** and **Settings in effect** → the end of System
  Info, still for those with the settings right
- the component serves AP CubeBadges v1.1 (a Cube's serial, IP and radio
  address)

### Blocks

1. CopyComponent: `copyEl` in `components.js`, its icon, its feedback, the
   fallback; tests
2. Versions: the header's and System Info's version copyable; tests
3. SystemInfo: the proxies copyable; Store and Settings in effect moved;
   tests, `docs/ui.md`

The three blocks are one pull request.

## Discussion

The owner's points of 2026-11-14 evening, weighed in three rounds:

- the version string in the header and in System Info copies with a
  click, System Info's line with a copy icon too; D94-A, the full
  version, without " (server version)";
- System Info's proxy URL copyable;
- Store and Settings in effect from Maintenance to the end of System
  Info;
- D101-A, D105-A: a plan of its own, first, as its component serves AP
  CubeBadges v1.1.

`PLAN+ as recommended` on 2026-11-14.

## Analysis

**No copy component exists.** `navigator.clipboard.writeText` needs a
secure context (HTTPS or `localhost`), and many boxes serve the LAN over
plain HTTP. There `document.execCommand("copy")` on a hidden text area
still works in every browser on a click; it is deprecated, so the
component tries the clipboard first.

**The header's version** is `#version-short` in `.top-facts`; its title
is `header.versionTitle` ("{version} (server version)"), set by
`renderVersionTitle` in `app.js`. `state.version` holds the full version.

**System Info** (`partials/system-info.html`, filled in `update.js`):
`#info-version`, `#info-proxies` (the Cube proxies, one per line).
**Store** and **Settings in effect** are `details.sys-sub.needs-settings`
in `partials/maintenance.html` (`#store-info`, `#config-info`), filled by
`update.js` (`store-info-body`, `config-info-body`).

## Implementation Plan

### Block 1: CopyComponent

- `components.js` `copyEl(text, { label, icon })`: a button, or the text
  made one, that copies `text`; with `icon`, a copy icon (Tabler's
  "copy") beside it; "Copied" in a polite live region and as a short mark
  on the control; `navigator.clipboard` first, else the hidden text area.
- The sprite: `icon-copy`.
- `style.css`: the copyable text (pointer, focus ring), the icon.
- i18n: "Copy", "Copied", "Copy {what}", six languages.
- A UI test: a click copies (the clipboard read through the test's
  permission), the fallback copies where `navigator.clipboard` is absent,
  Enter copies, the live region says it.

**Verification:** `make ui-test` (the new test), `go test ./internal/server/`.

### Block 2: Versions

- The header's `#version-short` copies `state.version`; its tooltip says
  so.
- System Info's `#info-version`: the version and a copy icon.
- Tests: both copy the full version, without " (server version)".

**Verification:** `make ui-test` (`test_header.py`, `test_system_info*`).

### Block 3: SystemInfo

- `#info-proxies`: each proxy's address with a copy icon.
- `#store-info` and `#config-info` move to the end of System Info
  (`partials/system-info.html`), kept `needs-settings`; Maintenance
  without them; their filling stays in `update.js`.
- Tests: the proxies copy; Store and Settings in effect under System Info
  for an owner, absent for a viewer.
- `docs/ui.md` "System settings"; a fragment.

**Verification:** `make ui-test` whole, `go test ./...`, `make cross`,
`gofmt -l .`, the three `go vet`, `GUIDELINES/shared/tools/check`.

## Proposed commit messages

Registered with the other plans of the same evaluation, a commit of its
own on main.

Block 1, `feat(ui): Facts copy with a click`;
Block 2, `feat(ui): The server's version copies from the header and System Info`;
Block 3, `feat(ui): System Info holds the store, the settings, the proxies to copy`.

## Change History

- v1.0 (2026-11-14): the first revision, from the owner's evaluation in
  three rounds and `PLAN+ as recommended`.

# AP LogAddress v1.0: log choices in address

## TL;DR

The log's range, kind and device join its page in the address,
so that a reload, a bookmark, a link, back and forward open the log as chosen.

- `#log` → `#log?range=1h&kind=state&device=0a0101`; a default left out
- a change of range, kind or device, Clear, another Cube
  → the current entry replaced, so back still leaves the log
- 📄 and ☰'s Log → the log with its current choices
- an unknown value → its default, and the address drops it;
  a device the Cube lacks goes once its devices are loaded
- the text filter stays out; no stored setting
- `docs/ui.md`; a CHANGELOG bullet under Added

### Blocks

1. The log's choices in the address: `log.js`, `app.js`, tests, `docs/ui.md`, CHANGELOG
2. The branch pushed, its pull request opened

## Discussion

The owner proposed keeping the range in the address (`EVAL` of 2026-11-08)
and answered the evaluation with `PLAN++ LogAddress`,
every decision as recommended:

- **D1-A.** The range, the kind and the device go into the address:
  all three are choices the server answers,
  and a link to one device's events today is useful.
  The text filter stays out; it would rewrite the address at every key.
- **D2-A.** A change of a choice replaces the current entry of the history,
  so that back leaves the log as it does now.
- **D3-A.** A choice at its default is left out, so a plain `#log` is the
  last 20 events, every kind and every device;
  an unknown value falls back to its default, and the address drops it.
- **D4-A.** No stored setting besides; it can follow if the range is missed
  across browsers.

This amends Navigation's D12-A, "the address holds the page only",
for the log's own choices; a room's open tab still stays out.
The owner asked for the plan to be committed into `main` before the work
branches off, as BackupRef's was.

## Analysis

- **The address today** (`internal/server/ui/log.js`):
  `VIEW_HASH` names each page's hash; `viewOfHash` compares the whole hash,
  so `#log?range=1h` reads as Rooms;
  `setAddress(hash, how)` pushes, replaces, or does nothing (`"none"`)
  where `location.hash` already equals it;
  `showView` writes `VIEW_HASH[view]`.
  `init` (`app.js`) calls `showView(viewOfHash(location.hash), "replace")`,
  the `popstate` handler the same with `"none"`.
- **The choices**: `logState.range` (one of `LOG_RANGES`, `LOG_COUNT_RANGE`
  by default), `logState.kind` (one of `LOG_KINDS`, `""` for all),
  `logState.device` (a radio address, `""` for all).
  Their change handlers call `loadLog`; Clear sets the filter, the kind and
  the device back; `logCubeChanged` sets the device back.
- **The device at the start.** `fillLogSelects` drops a device that is not
  among `state.rooms` and `state.unlisted` as gone.
  `init` shows the page before `refresh()` loads the rooms,
  so a device read from the address would always be dropped.
  It has to stay, as an option of its address,
  until the shown Cube's devices are loaded (`loadRooms`),
  and go then if the Cube lacks it.
- **Back and forward.** `popstate` can show the page with `"replace"`
  instead of `"none"`: `replaceState` on the entry just reached
  leaves the history's length and position as they are,
  and writes the address in its canonical form, a value dropped,
  the choices in their order.
  `"none"` then has no caller and goes.
- **Identifiers.** `tools/identifiers` reads `device=` and six hex digits as
  a radio address; the tests take the fake Cube's own placeholder addresses
  (`0a01…`), and an unknown one is `0a0fff`.
- **The tests** each get a page of their own, loaded without a hash,
  so a choice does not reach the next test's address.
  `test_the_address_names_the_page` (`test_header.py`) still sees `#log`:
  the choices are at their defaults there.

## Implementation Plan

### Block 1: The log's choices in the address

1. `internal/server/ui/log.js`:
   - `logHash()`: `#log`, then `?` and the range, the kind and the device,
     in that order, each left out at its default (`URLSearchParams`);
   - `readLogHash(hash)`: the three from the address's query,
     each at its default where it is missing or not one of its values
     (`LOG_RANGES`, `LOG_KINDS`, six hex digits, lower-cased);
   - `viewOfHash`: the page is the part before `?`;
   - `showAddress()`: the page the address names, the log's choices read
     from it, shown with `"replace"`; `init` and `popstate` call it;
   - `showView`: the log's hash is `logHash()`, and `fillLogSelects` runs
     before the address is written, so a device dropped is not written;
   - `logChoicesChanged()`: the address replaced while the log is shown,
     Clear, the log loaded again;
     the range, kind and device handlers and `clearLogFilter` call it;
   - `logCubeChanged`: the address replaced, its device gone;
   - `fillLogSelects`: a device not among the Cube's stays, as an option of
     its address, until the Cube's devices are loaded
     (`state.roomsOf === state.serial`), and is gone only then;
   - `setAddress`: `"none"` goes.
2. `internal/server/ui/app.js`: `state.roomsOf`, the Cube whose rooms and
   devices are loaded, set by `loadRooms`, which calls `logChoicesChanged`
   for a device gone; `init` calls `showAddress()`.
3. Tests, `tools/ui-test/test_log.py`:
   - the choices in the address: range, kind and device chosen →
     `#log?range=1h&kind=state&device=<rf>`, the history no longer;
     a reload opens the log with them, asked of the server;
     Clear → `#log?range=1h`; System settings and back → the log, its range;
     Rooms, then 📄 (☰ on a narrow screen) → `#log?range=1h`;
     the last 20 again → `#log`;
   - a link: a new page on `#log?device=<rf>&kind=state&range=today` →
     those choices, asked of the server, the address in its order;
   - unknown values: `#log?range=year&kind=nope&device=0a0fff` → the
     defaults, and the address `#log` once the rooms are loaded.
4. `docs/ui.md`: the address paragraph of the header,
   and the log's choices.
5. `CHANGELOG.md`, `### Added`.

**Verification:** the new tests pass, and the reload one fails without
step 1; `MASSIMO_ENV_FILE=/dev/null go test ./...`, `make cross`,
`gofmt -l .` empty, `go vet ./...` for linux, windows and darwin,
`changelog.sh check`, and `make ui-test`, the whole suite at 1100 and
390 px in English and German.

### Block 2: The pull request

1. The branch merges `origin/main` where it moved (a `merge:` commit),
   is pushed, and its pull request opened. No merge, retirement or release.

**Verification:** the pull request's checks are green.

## Proposed commit message

- `feat(ui): The log's range, kind and device in the address`

## Change History

- v1.0 (2026-11-08): the first revision, from the `EVAL` of the owner's
  proposal; D1-A to D4-A as recommended, `PLAN++ LogAddress`;
  committed into `main` before the branch, as the owner asked.

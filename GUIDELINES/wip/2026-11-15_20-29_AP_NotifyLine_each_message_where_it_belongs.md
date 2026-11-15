# AP NotifyLine v1.0: Each message where it belongs

## TL;DR

The Notifications card shows every message on one line, and every
success clears that line, whoever's message it shows. Two fixes follow
from that. A refused delay or threshold is marked on its own field, the
way Pairing's time is. The line keeps the save, load and device
outcomes, and a success clears only its own.

- **Refused value:** line, wiped by a pending save → its field: red
  box, the range in amber before it, `aria-invalid`, until a valid
  value (D175-B)
- **Line:** each message remembers who wrote it, `save`, `load` or
  `device`; a success clears only its own (D176-A)
- **Reload after Turn on / Turn off:** wipes the outcome, a refusal
  included → the outcome stays
- **Page shown again:** the line starts empty, as before
- **Texts:** `notify.badAfter` and `notify.badBelow` → `notify.range`,
  "only {min} to {max}", in six languages
- **Accepted values:** unchanged; `markRange` gains an `ok` argument

### Blocks

1. Refused values on their fields, the line's outcomes their own

## Discussion

`tools/ui-test/test_notifications.py::test_rules_kept_on_the_server`
failed once at 390-de on 2026-11-15 and passed on the retry. The test
clicks frost's switch and, within a second, fills 2000 into
window_open's minutes. The switch's save goes out a second later and,
on success, clears the line, taking the range error with it.

**The owner's decisions** (`PLAN++`, 2026-11-15):

- **D175-B:** a refused value is marked on its own field, as Pairing's
  time is (`markRange`), and the line is left to saves and devices. Not
  A, the error kept on the line and the line remembering whose message
  it shows.
- **D176-A:** the reload after turning this browser on or off no longer
  wipes that outcome. This is fixed in the same block, with the same
  mechanism, and asserted in `test_this_browser_on_and_off`. Not B, a
  task of its own.

## Analysis

- **The line** `#notify-msg` (`partials/notify.html`) is written by
  `showResult` from six places in `notify.js`:
  - `loadNotify`: cleared on success, `notify.loadFailed` on failure;
  - `saveNotifyRules`: cleared on success, `notify.saveFailed` on failure;
  - the two number fields: `notify.badAfter`, `notify.badBelow`;
  - `turnOnNotify`, `turnOffNotify`: `turnedOn`/`turnOnFailed`,
    `turnedOff`/`turnOffFailed`, then `await loadNotify()`;
  - `removeNotifyDevice`: cleared on success, `notify.removeFailed`,
    then `await loadNotify()`.
- **The race:** `saveNotifyRules` debounces by `NOTIFY_SAVE_DELAY`
  (1 s). A range error shown while a save waits is cleared when that
  save succeeds. A valid value typed afterwards does not clear the error
  itself either; only the next save's success does.
- **D176's defect:** `turnOnNotify` shows its outcome and then awaits
  `loadNotify`, whose success clears the line before its last
  `renderNotify`. "This browser is now notified." is therefore gone by
  the time Turn off shows, and so is "Not turned on: …" when the
  browser's permission was not granted. Nothing else on the card says so
  where the permission prompt was dismissed rather than denied.
- **Pairing's pattern:** `markRange(input, hint)` (`components.js`) sets
  `aria-invalid` and `aria-describedby` and shows the hint, an amber
  `.warn-text` span with `role="status"` before the box. `style.css`
  already styles both inside a `.field .inline` (the red box and the
  hint's margin), and `fieldRow` with a unit makes that `.inline` span,
  so no CSS is needed.
- **Validity versus the page's checks:** `markRange` judges by
  `input.validity`, so its step would refuse a threshold of 7.3 (step
  0.5), which the page and the server accept today
  (`internal/notify`, `Validate`: bounds only). It also passes an empty
  box, which the page refuses. An optional third argument, `ok`, lets
  the caller judge, so the accepted values do not change. Pairing calls
  it as before.
- **The texts:** the hint names the range alone, before the box and its
  unit, as `pairing.secondsRange` does: `notify.range`,
  "only {min} to {max}". The raw bounds are used, as the box shows them
  (1440, not 1,440). `notify.badAfter` and `notify.badBelow` then have
  no use and leave all six files. `TestUITranslations` holds the six to
  one set of keys.
- **Redraws:** `renderNotifyRules` builds the inputs afresh from the
  rules, so a redraw (a load) clears a refused value with its mark.
- **The page shown again:** `onView("me", loadNotify)`. The load's
  success no longer clears another's outcome, so showing the page
  clears the line first, as the load used to do every time.
- **The test, made robust:** the frost click and the refused value are
  wrapped in `expect_request_finished` for the PUT that turns frost off.
  After it, the field must still be marked and the line still hidden.
  This holds whichever came first, the save or the refusal. For D176,
  "Turn on" is asserted once Turn off shows, which happens only after the
  reload's last draw, and "Turn off" once the device has left the list,
  which is likewise drawn after the reload.
- **No open pull request** touches these files: #306 changes the log's
  tests.

## Implementation Plan

### Block 1: Refused values on their fields, the line's outcomes their own

1. **`internal/server/ui/components.js`:** `markRange(input, hint, ok)`,
   where `ok` defaults to `input.validity.valid`; the comment says when a
   caller passes it.
2. **`internal/server/ui/notify.js`:**
   - `notifyValue`: a rule's delay or threshold, its number input and
     its range hint, which `markRange` marks by the page's own check and
     which saves only a value it takes. The hint goes before the box,
     with its id `<input id>-range`;
   - `notifySay(from, text, ok)`: the line, `data-from` naming whose
     outcome it shows. No text clears it only where it shows `from`'s
     own; `from` "" clears whatever it shows;
   - `loadNotify` → `load`, `saveNotifyRules` → `save`, turning on and
     off and Remove → `device`;
   - `initNotify`: the page shown again clears the line, then loads.
3. **Six languages:** `notify.range` added, `notify.badAfter` and
   `notify.badBelow` removed; FR/IT/NL/PL machine-translated, as their
   files say.
4. **`tools/ui-test/test_notifications.py`:**
   - `test_rules_kept_on_the_server`: a threshold of 50 marked and then
     taken back with 8. Frost switched off and 2000 refused inside the
     PUT's `expect_request_finished`. After the save, the minutes still
     marked (`aria-invalid`, the hint with `notify.range` named by
     `aria-describedby`) and the line hidden. 45 again clears the mark.
     The rest as before;
   - `test_this_browser_on_and_off`: the line shows `notify.turnedOn`
     after the reload, and `notify.turnedOff` once the device has left
     the list.
5. **`docs/ui.md`, "Notifications":** a refused value marked on its
   field until it holds one inside the bounds, and the line under the
   lists, which a success clears only of its own outcome.
6. **`CHANGELOG.d/Fixed/claude-notify-line.md`**, a fragment.

- **Verification:**
  - `MASSIMO_ENV_FILE=/dev/null go test ./...` (`TestUITranslations`
    among them), `gofmt -l .`, `go vet` on three platforms, `make cross`;
  - `PYTEST_ADDOPTS='test_notifications.py' make ui-test`, all four
    cases, on ports of its own. The new assertions are seen failing
    without the fix first (the old `notify.js` through a build overlay).
    The whole suite is left to CI;
  - `GUIDELINES/shared/tools/check`.

## Proposed commit message

```
fix(ui): Notifications mark a refused value on its field, and a success clears only its own message

The Notifications card's one message line was cleared by every success, whoever's message it showed. A pending save of the rules wiped a delay's range error a second later, which failed test_rules_kept_on_the_server once at 390-de. The reload after turning this browser on or off wiped that outcome, a refused permission included (AP NotifyLine v1.0, Block 1).

Key changes:
- notify.js: a delay or threshold out of bounds is marked on its field by markRange, its range in amber before the box, until the field holds a value the page takes; it is not saved meanwhile
- notify.js: the line remembers whose outcome it shows, the load's, the save's or a device's, and a success clears only its own; the page shown again starts with it empty
- components.js: markRange takes the caller's verdict where the field's own validity would judge otherwise, so the values accepted stay as they were
- six languages: notify.range in place of notify.badAfter and notify.badBelow
- test_notifications.py: the mark kept after the pending save has finished, cleared by a valid value; the outcome of turning on and off kept after the reload
- docs/ui.md, "Notifications"; CHANGELOG.d/Fixed/claude-notify-line.md

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-15): first revision, D175-B, D176-A.

# AP HeaderAnchor v1.0: compacting moves no page

## TL;DR

When the page is scrolled past 96 px in one jump, the header compacts
a frame later and the browser moves the page back by 53 px at 390 px: its
scroll anchoring, which `stickyHeader` means to switch off while it
compacts, stays on, because it is switched off on `html` only and Chrome
takes the page's setting from `body`. It is switched off on both.

- `stickyHeader` → `overflow-anchor: none` on `html` and `body` while it
  compacts or grows the header, both back after
- a jump to the middle of the page at 390 px → the page stays where it
  went (a UI test, checked to fail without the fix)
- the Reset all tests' wait (#275) stays: a click right after a scroll
  still waits for the page to rest

### Blocks

1. HeaderAnchor: the fix and its comment, a test in `test_scroll.py`,
   CHANGELOG fragment

## Discussion

Found by the Reset all flake (#275, 2026-11-15): an event log showed the
press on the note above the button, the page moved by 53 px after the
scroll. `EXEC A, PLAN B`, then `PLAN++` for this plan.

The comment above `stickyHeader` says compacting must not "move what
follows the header where the browser can see it", and that the browser's
scroll anchoring is off meanwhile; this makes the code do what the
comment says.

## Analysis

Measured at 390 px (en and de), Reset all scrolled into view at once:

| | `scrollY` after the scroll | after the compaction | Reset all |
|---|---|---|---|
| as now | 689 | 636 | y 432 → 485 |
| anchoring off on `html` and `body` (a style rule) | 689 | 689 | y 432 |
| as now, `html` off for two more frames | 689 | 636 | y 432 → 485 |
| `html` and `body` off in `set`, as now restored | 689 | 689 | y 432 |
| the header alone (`.top`) out of anchoring | 689 | 636 | y 432 → 485 |

The document does not move (Reset all at document y 1121 throughout);
only the scroll position does. At 1100 px the header loses 12 px and the
page does not move. A slow scroll (`test_slow_scroll_goes_down`) does not
show it.

With the fix, `test_scroll.py`, `test_header.py`, `test_system_layout.py`
and `test_send.py` passed in all four cases (120 runs).

## Implementation Plan

Every commit cites `AP HeaderAnchor v1.0, Block 1`.

### Block 1: HeaderAnchor

- `internal/server/ui/components.js`, `stickyHeader`'s `set`: `overflow-
  anchor: none` on `document.body` as on `html`, both restored together;
  the comment says why both.
- `tools/ui-test/test_scroll.py`: a jump from the top to the middle of
  a page made long enough, the header compacting, the page where it went
  (within a pixel); checked to fail at 390 px without the fix.
- CHANGELOG fragment, `Fixed`.

**Verification:** `make ui-test` (all four cases); the new test without the
fix; `go test ./internal/server/` (the UI's own checks).

## Proposed commit messages

- `chore(aps): +HeaderAnchor v1.0`, this plan, on main.
- Block 1, `fix(ui): A jump down the page stays where it went as the header compacts`.

## Change History

- v1.0 (2026-11-15): first revision, by `PLAN++` after `PLAN B`.

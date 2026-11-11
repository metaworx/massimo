# AP ScrollSettle v1.0: Scroll settles before top

## TL;DR

`test_header_and_page_bar_stay` waits for the sections menu's smooth scroll
to come to rest before it scrolls to the top,
so that Chromium's compositor cannot undo the test's scroll.

- flake on main, 2026-11-10 (run 42904559935, 1100-en):
  `body.scrolled` still set 3 s after `scrollTo(0, 0)`
- cause: an instant scroll during the smooth scroll's last frames
  → the page at the top for a frame, then back at the card
- UI: unchanged; a user's scroll and ↑ cancel the animation
- test: band check → `scroll_settled` → `scrollTo(0, 0)`
  → `scrollY === 0` and the header whole

### Blocks

1. `scroll_settled` in helpers.py, the test waits for it

## Discussion

The flake was presented as a TL;DR gate on 2026-11-10.

**The owner's decisions** (`PLAN++`, as recommended, 2026-11-11):

- **D1-A:** the test waits for the scroll to settle.
  Not B, reduced motion for this test:
  `showCard` would scroll instantly,
  and the test would no longer run the smooth scroll a user gets.
- **D2:** the plan's name, ScrollSettle.

## Analysis

**The failure.**
After the menu's `.toc-link[data-target='sys-users']` is clicked,
the test waits until the card's top is between the row's bottom − 1 and + 40 px,
then calls `window.scrollTo(0, 0)` and waits 3 s for `body.scrolled` to go.
It failed once in the last 25 failed runs; the retry passed.

**The evidence.**
The first pass's screenshot and video, in the run's `ui-test-results` artifact.
The screenshot: the page at the Users card, the header compacted.
The video, frame by frame at 10 fps:
the card under the row (the band check passes);
one frame with the header whole, its update line back,
over the Users card, not over the top of the page;
then the header compacted again at the card until the end.
So the instant scroll reached the main thread
(a scroll event at 0 cleared `scrolled`, `stickyHeader` in components.js),
and the smooth scroll, still finishing on the compositor,
synced its offset back.

**The window.**
`showCard` scrolls with `behavior: "smooth"`,
its target 10 px under the row (`.section-card`'s `scroll-margin-top`);
the band is met about 11 px before the target, in the animation's slow end.

**Not reproduced locally.**
12 runs of the test under 16 busy loops (48 cases) passed;
a probe firing `scrollTo(0, 0)` from 0 to 700 ms into the smooth scroll,
and at the first frame of the band, ended at the top all 88 times.
The race is rare and needs the CI runner's timing; the video is the proof.

**Not the UI's.**
A user's wheel, key or touch scroll runs on the compositor and ends the animation there;
↑ (`backToTop`) is a smooth scroll, which replaces the one running.
Only a script's instant scroll during the animation can lose,
and only this test sends one:
no other test, and no UI code, follows a smooth scroll with an instant one.
`settleAtTop`'s `scrollTo({ top: 0 })` runs only within 8 px of the top,
after ↑'s own smooth scroll.

**The wait.**
`scroll_settled(page, seconds)` evaluates a promise in the page:
it counts animation frames in which `scrollY` has not changed,
resolves true after 10 in a row and false at the deadline.
A function evaluated, not a string, which the page's CSP allows.
Polling from Python (`wait_until`) at 200 ms would see two equal readings
across a pause in a starved renderer; frames do not.

## Implementation Plan

### Block 1: The test waits for the scroll to rest

1. `tools/ui-test/helpers.py`: `scroll_settled(page, seconds=3)`,
   its docstring saying why (the compositor's smooth scroll undoes an instant one).
2. `tools/ui-test/test_system_layout.py`, `test_header_and_page_bar_stay`:
   after "the card chosen comes in under the row",
   assert `scroll_settled(page)` ("the menu's scroll comes to rest");
   after `scrollTo(0, 0)`, wait for `scrollY === 0` and the header whole.
3. No CHANGELOG fragment: `tools/**` is not shipped.

**Verification:**
the test at its four cases, ten times under 16 busy loops;
a try at the race with CDP CPU throttling, before and after;
`make ui-test` whole;
`check --commit-range origin/main..HEAD` from the main checkout.

## Proposed commit message

```
test(ui-test): The page bar test waits for the menu's scroll to rest

test_header_and_page_bar_stay scrolled to the top while the sections menu's smooth scroll was still ending, and Chromium's compositor put the page back at the card: body.scrolled stayed, and the test failed on main (run 42904559935). The failed pass's video shows the header whole for one frame over the card, then compacted again.

Key changes:
- helpers.py: scroll_settled waits for ten frames without a scroll
- test_system_layout.py: the test waits for it before scrolling to
  the top, and checks scrollY is 0

Plan: AP ScrollSettle v1.0 (Block 1)

Assisted-by: claude-code:claude-opus-5-5
```

## Change History

- v1.0 (2026-11-11): first revision, D1-A, by PLAN++.

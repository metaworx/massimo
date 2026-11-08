# AP ToTopScroll v1.0: Back to top settles

## TL;DR

↑ in System settings takes the page all the way to the top:
for 3 s after the click, a scroll that leaves the page within 8 px of the top
is taken the rest of the way; `test_back_to_top` stays strict at 0.

- CI: `test_back_to_top` flaky on `main` (14:08, `1100-de`) and in pull
  request 113 (`1100-en`): the page reached the top, was moved back 3 px as
  the header grew back, and stayed
- `backToTop` → `settleAtTop`: a scroll event with 0 < y < 8 within 3 s →
  `scrollTo({ top: 0 })`; from 8 px on it is the user's, left alone
- not reproduced here; checked with a push of 3 px made by hand
- the test unchanged; no CHANGELOG bullet (↑'s, unreleased, stays accurate)

### Blocks

1. `settleAtTop` in `internal/server/ui/components.js`, the checks,
   the UI suite
2. The branch pushed, its pull request opened

## Discussion

The owner reported the Screenshots job of pull request 113 failing in
`test_back_to_top[1100-en-chromium]`: "↑ goes back to the top", `window.scrollY
=== 0` false for 3 s after the click.
The same test had failed on `main`'s push of 14:08 (`1100-de`),
a plan-only commit, and passed on the next.
The evaluation's gate offered D2-A, ↑ jumping the rest of the way when its
smooth scroll ends short of the top (`scrollend`), the test kept strict;
B, the test accepting under 8 px; C, leaving it.
The owner answered `PLAN++ D2-A`.

The trace, read further for the plan, shows the page did reach the top and
was then moved back (Analysis).
A `scrollend` alone could come before that move, so the remedy listens to
every scroll for a few seconds instead:
D2-A's remedy with a wider trigger, the test as strict.

## Analysis

- **The trace** (`test-results` of run 42491140646): after the click the
  page's offset went 965, 939, 673, 35, 27, 14 px; at 265447 ms no offset was
  recorded and the header was whole again (`--top-h` 53 → 169 px);
  at 265688 ms it was 3 px, and stayed so until the test gave up at 267892 ms.
  The failure screenshot shows the header whole, two lines under its first
  ("1 Cube found", "1 pending").
- **Not reproduced here**:
  40 runs of the test at `1100-en`, 25 clicks with Chromium's CPU slowed 4×,
  the header made to grow back under 40 and under 8 px during the scroll,
  and 40 px inserted above the content during it all ended at 0,
  `scrollend` firing at 0.
- **A likely cause**, not established:
  the header measures its whole height when it compacts and keeps the page's
  length with a margin (`stickyHeader`);
  the lines under its first are hidden while compact,
  so a line that comes or goes meanwhile ("1 pending") makes it grow back to
  another height, the content moves,
  and the browser's scroll anchoring keeps the content in place by moving the
  page, wherever it is not exactly at 0.
- **The remedy does not depend on it.**
  Whatever moves the page, a scroll event follows;
  where it leaves the page above 0 and under 8 px, the header's own threshold
  for "at the top", it is taken to 0 at once.
  From 8 px on it is the user's scroll and is left alone.
  The listener lives 3 s from the click:
  a smooth scroll over the longest page takes under a second,
  and the move came 250 ms after the top.
  It runs with reduced motion too.
- **Docs**:
  `docs/ui.md` says ↑ "goes back to the top of the page", which it now does;
  the CHANGELOG bullet that brought ↑ (`### Changed`,
  System settings) is accurate and unreleased,
  so the commit leaves the file alone and says so (COMMIT.md 5.2).

## Implementation Plan

The branch `claude/to-top-scroll` from `main` after this plan's commit.

### Block 1: ↑ settles at the top

- `internal/server/ui/components.js`:
  `settleAtTop()`, a passive scroll listener for 3 s that takes a page left
  above 0 and under 8 px to 0;
  `backToTop`'s click calls it after its `scrollTo`.
- `tools/ui-test/test_system_layout.py`:
  unchanged.

**Verification:**

- a scratch check, outside the tree: after ↑ has reached the top,
  the page moved to 3 px by hand stays there without the change and returns to 0
  with it; moved to 200 px it stays there with it;
- `test_back_to_top` 40 times at `1100-en` and at `1100-de`;
- `make ui-test`; `MASSIMO_ENV_FILE=/dev/null go test ./...`.

### Block 2: The pull request

- Push `claude/to-top-scroll`, open its pull request.

**Verification:**
its checks are green, Screenshots among them.

## Proposed commit message

- `fix(ui): Back to top settles at the top, not a few pixels short`

## Change History

- v1.0 (2026-11-08): the first revision, from the evaluation of the
  Screenshots failure of pull request 113; D2-A by `PLAN++`, its trigger
  widened from `scrollend` to every scroll for 3 s, as the trace shows;
  committed into `main` before the branch.

# NOTE MobileApp v1.6: the copy and the codes

The owner's answer at the gate after Blocks 21 and 19, on 2026-11-28:
`EXEC as recommended`.
It came after the owner's `EVAL` of D115, which asked for B with A for the
version row, and a question on D116: how is a server identified?
Two decisions, both built by Block 20 (`claude/mobile-menu-cards`).

## D115-D: the word of a copy

Every copy shows its word through AP ErrorCodes' `showBubble(text, ctrl)`
(metaworx/massimo#489, its Block 6):

- above the control, where the control stays on the screen;
- at the window's foot, where the tap closes the menu that holds the
  control, by `showBubble` without a control.
  These are the version rows of the list's ☰ (Block 19)
  and of the server's ☰ (Block 20, step 3).

D97-A's toast for a copy goes, and a failed copy is shown as AP ErrorCodes'
Block 6 shows it.
Option D was added at the owner's `EVAL`; A, every copy at the foot,
and B, above the control everywhere, were not taken.

Why:

- it keeps AP ErrorCodes' word beside what was copied;
- a bubble above a row of a closing menu would vanish at once:
  `followBubble` hides it once its control has no client rects,
  and ☰ is a `<details>` (`src/shared/menu.tsx`);
- Android 13 and later draw their own clipboard preview at the screen's
  foot, so only the version rows share it: a look at 390 px in the phone check.

Block 20's step 4 does this in place of D97-A's toast, once #489 has landed:
the branch merges `main` then.
Its other steps do not wait for it.
#489 and its tests stay as they are.

## D117-A: a removed server's codes follow its node

How a server is identified, as Block 21 built it:

- the app's id belongs to the list's entry, and goes with Remove;
- a removed server is kept in `gone.json` with its node
  (`/healthz`'s `node_id`, the store's `NodeID`), its addresses and its
  pinned fingerprint (`Gone.kt`);
- its codes are posted only at those addresses, where `/healthz` answers
  with that node and, over HTTPS, the certificate matches the pin or the
  system trusts it (`Addresses.good`, `Prober.kt`);
- the node proves nothing, as `/healthz` tells it to anyone;
  but a wrong recipient gains nothing a code can give,
  only a `204` that drops a removed server's code early;
- a server kept without a node takes any answer at its addresses;
- a server added again at another address never gets its old codes.

Decided:

- a removed server's codes are also posted wherever a server of the list
  answers with the same node, through the same check
  (`Addresses.good` with that node, the listed server's pin);
- adding or saving a server whose node is a removed one's takes that one's
  codes back into its entry, as the keys and sessions left where
  `ServerStore` keeps them for the same node, and drops the removed entry;
- a removed server kept without a node keeps the rule above;
- D116-A's 90 days then bound only a server never seen again.

Block 20 builds it as a step of its own:

- JVM tests: a code posted through another entry with the node,
  the codes taken back at an add, and a server without a node matching none;
- `docs/app.md`.

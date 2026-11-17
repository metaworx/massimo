# NOTE StoreRaise v1.2: decisions on block 5

The owner, 2026-11-17, `EXEC D20-B`, after Block 5's sub-agent stopped
before any code (NOTE StoreRaise v1.2, "how block 3 went", point 6:
nothing tombstoned a Cube, so D11-A's restore had nothing to restore):

- **D20-B:** "Remove" tombstones the Cube's row (`configured = 0`,
  `deleted_at` set). Discovery makes no found-only row for a serial with
  removed rows: its found entry lives in memory only, as the page's found
  list always does, and comes back with the first discovery run after a
  restart. "Add as new" makes a row; "Restore <date>" revives the removed
  row it names, with nothing to clear. The remove confirmation's text no
  longer says that adding brings the history back, but that it can be
  restored.
- Weighed against **D20-A** (a found-only row as for every other Cube,
  cleared by the restore with the audit and commands moved): the user sees
  the same, since the found list never came from the store.

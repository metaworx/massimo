# NOTE SteadyTests v1.1: the gate's decisions

The owner, 2026-11-27, `EXEC++` on v1.1's plan gate,
the two decisions it left open taking their recommended options:

- **D10-A.** The authors' rule is a sentence in the contract's "Tests"
  (`GUIDELINES/project/_CONTRACT.md`, inlined into `AGENTS.md`),
  and `docs/testing.md` holds it with the kit's helpers by pattern
  (Block 10).
- **D11-A.** Block 11's check fails a UI test file that leaves the server
  otherwise than it found it, naming what it left;
  what no API can undo is listed with its reason.

The run's scope, as the gate stated it:
v1.1 committed to `maint/0.17` and merged upward;
Block 11 onto #473;
Blocks 12, 13 and 14 in one pull request into `maint/0.17`;
Block 10 against `main` once #473 has merged;
the pull requests' CI watched; no merge and no retirement.

# NOTE PasswordCost v1.0: how the run went

Blocks 1 to 5 ran under the owner's `PLAN++ D5-A` of 2026-11-07,
one signed commit each, on the branch of pull request 87,
with main merged in before the push
(`[Unreleased]` conflicted: main's Fixed now sits beside this run's Security).
Where it went differently from the plan:

- **The memory bound is twice the plan's figure.**
  A slot holds its 19 MiB while it hashes,
  but a finished hash's block stays in the heap until the collector frees it,
  so eight checks sent together peak at 38 MiB of heap with one slot
  and 152 MiB with four (`BenchmarkPasswordChecksAtOnce`),
  where D2-A said 19 and 76 MiB.
  `docs/store.md` section 6 gives the measured figures.
  A collection after each hash would trade time for it; not done,
  since 38 MiB is far inside what a Pi 1 has free.
- **The time ratio is 0.55, not 0.6**, on one amd64 core,
  and 0.43 on a 32-bit build (`GOARCH=386`):
  the old parameters take 222 ms there against the owner's 4.5 s on the Pi 1,
  about 20 times, so a new hash should take about 2 s on the Pi 1.
  Estimated, not measured there.
- **Block 3's UI check** was run once with the dialog's guard removed:
  it failed with two sign-ins sent, and passes with the guard.

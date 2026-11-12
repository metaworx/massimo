# NOTE ActionLint v1.0: the runner's shellcheck

What the pull request's first run found after the run of Blocks 1 to 4,
beside `wip/2026-11-12_16-43_NOTE_ActionLint_v1.0_how_the_run_went.md`.

1. **The job failed on the runner, not here.**
   The runner's shellcheck is newer than this machine's 0.9.0 (Debian's),
   and reports a function that only a trap calls as SC2329, "never invoked",
   where 0.9.0 reports SC2317, "unreachable".
   Block 2's `# shellcheck disable=SC2317` before the Mac's `stop` silenced
   the old code alone.
2. **The trap runs its commands itself now**
   (`trap '…' EXIT`, single quotes, so that `$label` and `$plist` are read
   at the exit), and `stop` is gone:
   neither version has anything to report, and no directive is needed.
3. **shellcheck stays unpinned**, the runner's:
   pinning it is outside the plan's blocks.
   `tools/actionlint/README.md` says that a newer one may report more,
   and that code neither version flags is better than a directive for each.

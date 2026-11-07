# NOTE ReviewFixes v1.1: main merged, low 60 fixed

How the owner's `EXEC D10-A` and the fix of low 60 (D8) went on 2026-11-07,
after the UI and publishing stack (Blocks 4 and 5) had merged into `main`.
Corrects item 3 of [the run's
NOTE](2026-11-07_03-06_NOTE_ReviewFixes_v1.1_how_the_go_stack_went.md),
which left low 60 of [the
analysis](2026-11-07_01-03_ANALYSIS_ReviewFixes_main_since_2026-11-05.md)
reproduced but not fixed.

## origin/main merged (D10-A)

`origin/main` merged into `claude/review-go`,
then each branch into the next, `-2` and `-3`; merges, no rebase.
The one conflict was CHANGELOG.md's `### Fixed` under `[Unreleased]`,
which both stacks had added:
one heading now, main's 18 bullets before the stack's, none rewrapped;
packaging/README.md merged by itself.
The higher branches took the resolution without a second one.
Each branch passed the gate after its merge,
with `-race` and `make cross`,
and Block 3's branch `make ui-test` for main's UI changes (176 passed).

## Low 60 fixed (D8)

`TestLatestSkipsOtherChannelsUnfetched` (`internal/update/update_test.go`)
failed on Block 2's branch before the fix:
a check on `main`, which has no releases,
fetched the manifest and signature of all six releases,
and one on `alpha` those of a release on another channel as well.
After it, `Latest` leaves out unfetched a release whose tag names another channel,
on Block 2's branch, merged into Block 3's,
where the whole suite passes again.

## What went differently

1. **The hint is the tag's label first, the target second.**
   The planned filter by `target_commitish` alone would drop every release:
   all of them name `main` there,
   and v0.1.0-alpha.11 names `alpha` while it is signed for `phase-0`.
   release.yml's own rule (the pre-release label is the channel) holds for
   every release since v0.1.0-alpha.12, checked against their signed manifests;
   a tag without a label goes by the release's target,
   which release.yml now sets to the channel with `--target`.
   Neither is signed: a hint only spares requests, the manifest decides.
2. **The phase-0 releases**,
   v0.1.0-alpha.10 and .11, are signed for `phase-0` under alpha's label,
   so a check on `phase-0` no longer finds them;
   packaging/README.md already told those boxes to follow `alpha`,
   and now says why.
3. **CI's stubbed releases** in the Windows job were tagged `v0.0.0-ci.N`
   and signed for `alpha`, which release.yml could not produce,
   and the new rule would have left them out:
   the stub, and the binaries that take it, are on the channel `ci` now.
4. **Two existing tests** used a release whose label names another channel
   than its manifest:
   one now expects it left out, as the phase-0 ones are;
   the other's release that does not verify has a tag without a label.

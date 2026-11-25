# NOTE MobileApp v1.4: compile sdk 37

The owner's decision of 2026-11-24, `EXEC D88-A`,
on Dependabot's pull request metaworx/massimo#369, which could not build.

## What failed

#369 bumps `androidx.core` from 1.18.0 to 1.19.1 and the Gradle wrapper
from 9.8.2 to 9.9.1, both in the one group Block 12 gave Dependabot.
`androidx.core` 1.19.1 asks whoever depends on it to compile against
version 37 of the Android APIs, and the app compiled against 36:
`:app:checkFossDebugAarMetadata` failed, and the wrapper's bump with it.
Block 3 had held `androidx.core` at 1.18.0 for that reason
(`2026-11-09_04-50_NOTE_MobileApp_v1.0_block_3_shell.md`);
Block 12's entry for Dependabot did not say so.

## D88-A

- `compileSdk` rises to 37.
  `targetSdk` stays 36 and `minSdk` 26,
  so the app behaves on a phone as before.
  This replaces Block 3's "stay at 36" for the compile level,
  and for no other.
- Dependabot's Gradle group is split in two: the libraries,
  and the build's own tools, the Gradle wrapper and the Android Gradle Plugin,
  so that a library that cannot build holds no wrapper back.
- Not taken, B: an `ignore` for `androidx.core` from 1.19,
  with `compileSdk` rising later, together with `targetSdk`.
  The other AndroidX libraries will ask for 37 as well,
  and each `ignore` also withholds that library's fixes.

## Left open

- `targetSdk 37`, which opts the app into Android 17's behaviour:
  a step of its own, with a phone check.
- Whether F-Droid's buildserver has platform 37: Block 5's to see.
- CI does not check `gradle-wrapper.jar` against Gradle's published checksums;
  only the distribution's checksum is pinned.
  A step for it was offered at the gate and not taken.
- The number: this decision was first offered as "D40" on 2026-11-20,
  a number v1.2 took the same day for the phone check's decisions.

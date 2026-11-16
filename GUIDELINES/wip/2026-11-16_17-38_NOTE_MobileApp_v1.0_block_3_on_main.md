# NOTE MobileApp v1.0: block 3 on main

Block 3 was built on 2026-11-09 and not committed;
on 2026-11-16 `main` was a week further on,
and the owner's `EXEC D9-A` moved the block onto it.
This NOTE corrects the block's first NOTE,
`2026-11-09_04-50_NOTE_MobileApp_v1.0_block_3_shell.md`,
and records what the move changed.

## The decisions' numbers

The plan's NOTE of Block 2 had already given D4 and D5 their answers,
so Block 3's gate numbered its own wrongly.
The first NOTE's D4-A, D5-A and D6-A are this plan's:

- **D6-A**:
  the bridge is messages over `addWebMessageListener`, not `@JavascriptInterface`.
- **D7-A**:
  the Gradle wrapper is committed.
- **D8-A**:
  `languages` gains `xml`, and there is no `ignore = kotlin`.

## D9-A, the icon

The web UI has Massimo's mark since AP WebBrand,
so the app's icon is the mark, not the thermometer the first NOTE describes.
`tools/brand/make_brand.py` (1.4.0) has a scope `android`,
which `make ui-brand` runs beside `ui`:
the adaptive icon's foreground and monochrome layers as vector drawables,
and its background colour,
in `app/android/app/src/main/res-brand/`, a resource folder of the app's,
the tool's as `internal/server/ui/brand/` is.
The foreground is the mark as the web UI's maskable square places it;
the monochrome layer is the M in one colour with the hands carved,
since an orange hand on top would vanish in one colour.
The `ui` scope's files came out of the new version unchanged.

## What the move changed

- The bullet is a fragment, `CHANGELOG.d/Added/claude-mobile-android.md`,
  as `main` now requires; `CHANGELOG.md` is untouched.
- `GUIDELINES/config.ini` joins `main`'s `ts` to the block's `xml`
  and `main`'s comments to the block's.
- The Kotlin and Gradle files and `bridge.js` open with the SPDX and copyright lines,
  as the contract now has code do;
  the resources and the wrapper's `gradlew` do not, as HTML and YAML do not.
- The app's copy of the web UI leaves out the parts as well: `src/` and the
  bundle `parts.js`, the server page's.
  AP UiParts keeps `connect.html` and `connect.js` plain,
  so the app needs no `make ui`.
- The test gate opens with `make ui`, which the Go build now needs.

## For Blocks 1, 4 and 5

AP UiParts lists where Go is compiled and the bundle must exist first;
the apps are not on the list.
`make app-core-android` (Block 1), the CI job and the release (Block 4)
and F-Droid's recipe (Block 5) build the server into the app,
so each runs `make ui` before `gomobile bind`.

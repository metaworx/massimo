# NOTE MobileApp v1.0: gate decisions

The owner's answer of 2026-11-09 to the plan gate of AP MobileApp v1.0:
register the plan on `main` with its analysis, the decisions as follows.

## Decisions

- **D1-A**:
  the application id is `ch.metaworx.massimo`, on both stores;
  it never changes after the first publish.
- **D2-A**:
  `minSdk 26`, Android 8.0.
  The owner keeps `minSdk 24` in reserve: it may be lowered on request,
  which a later NOTE records.
- **D3-A**:
  the server's tag releases the app:
  `make release-alpha` writes `app/version.json` into the release commit,
  and `release.yml` attaches the signed APK and the AAB to the same release.

## What runs it

Sessions of their own per block, as the plan says, after this registration;
Blocks 2 and 3 may start at once,
Block 1 once AP ServerAnnouncements Block 1 has merged.

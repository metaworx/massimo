# NOTE MobileApp v1.0: block 4 refit

Block 4's three steps were written on 2026-11-09,
against a release pipeline that AP Channels has rebuilt since.
The block stands:
CI builds the app, the release carries the signed `foss` APK and the `play` AAB,
and the version comes from the release.
Its steps change,
as the owner decided at the block's gate on 2026-11-17;
this NOTE records the answers and the steps as they are now built.

## The decisions' numbers

They start at D12.
D9 to D11 were the TL;DR gate's before the plan
(`app/README.md` cites "D10-A of the plan", the toolchain in WSL),
and D10 and D11 were used again at the phone test's gate on 2026-11-17.

## The answers

- **D12-A, the version**:
  `app/version.json`, committed, is the changelog's manifest
  (`GUIDELINES/config.ini`).
  `changelog.sh cut` writes its `version` in the release commit,
  and `make release` its `versionCode`, just before the cut;
  the Guidelines job holds the file to the changelog's newest release.
  F-Droid's update check reads both at the tag.
- **D13-B, which builds carry the app**:
  the version tags, and a rolling build as D18 says.
- **D14-A, where the app is signed**:
  build.yml builds it unsigned;
  a job of its own in `signing`, "Sign the Android app",
  starts beside `sign` under the one approval.
- **D15-D, the sums and the attestation** (the owner's, A with C):
  a third job in `signing` (D20) adds the APK's and the AAB's lines
  to the release's `SHA256SUMS`,
  signs it with the release key as `SHA256SUMS.sig`,
  and uploads the four files;
  a job without a key then attests the signed APK, the AAB and the new
  `SHA256SUMS`.
  `sign` signs the first `SHA256SUMS` too,
  so a release without the app has the signature as well,
  and `rolling` signs a rolling build's with the rolling key.
  The signed manifest, the boxes' update list, names no app file.
- **D16-A, the keys**:
  PKCS#12 keystores, each a base64 secret with its password:
  `MASSIMO_APP_KEYSTORE` (alias `massimo`, the APK)
  and `MASSIMO_APP_UPLOAD_KEYSTORE` (alias `upload`, the AAB for Play)
  in `signing`,
  `MASSIMO_APP_ROLLING_KEYSTORE` (alias `rolling`, the nightly APK)
  in `signing-rolling`.
  Their certificates are committed under `app/android/signing/`,
  and each signing compares the signer with them before anything is published.
  The owner makes the keys and keeps them;
  without a secret or its certificate the step warns and the release carries
  no app.
- **D17-A, when the job Android app runs**:
  as LAN does, with the answer `app` of What to test
  and the word `app=` in the Tested record.
- **D18-A, the rolling APK**:
  the nightly build alone carries one,
  `ch.metaworx.massimo.nightly`, "Massimo nightly",
  installed beside the release app;
  devel builds carry none, and no rolling build goes to Google Play.
  At the run's signal the owner asked for an icon of its own:
  the mark in one colour, the hands carved,
  white on the night's dark blue instead of the release's tile.
- **D19-A, where the nightly APK is signed**:
  inside `rolling`, before its one publish;
  a fault there warns, and the build goes out without the APK.
- **D20-A, which job signs the longer sums**:
  "Publish the Android app",
  which runs only `sha256sum`, openssl and gh,
  waits for "Sign and publish" and "Sign the Android app",
  and checks the published `SHA256SUMS.sig` against the release key
  before it extends the file.
- **D21-A, the versionCode**:
  `MMmmPPbbbb`: the major (at most 20, as Play allows),
  the minor and the patch (at most 99),
  and the commits since the version's first tag (at most 9999).
  The labels have no digits of their own:
  a stable release is `16000bbb` like any other build of 0.16.0,
  and only a version's first tag ends in `0000`.
  The release commit's number is counted before the commit exists;
  `make release` checks it against the tag before the push.

## Google Play, for Block 6

At the gate the owner asked how Play's tracks work.
A test is a track of the one app, with the same id and key;
the track is chosen at each upload, and a build can be promoted later.
The proposal, for Block 6: alpha to the internal test,
beta and rc to the closed test and later the open one, stable to production,
with the 14-day closed test of a new account run on an alpha.

## The steps as built

1. `ci.yml`, the job "Android app": the version file checked,
   the debug build with its unit tests and lint,
   the release APK built in two places and compared byte for byte,
   then the AAB and the nightly APK, all three signed with a key made
   for the run and checked against its certificate,
   and refused against another one.
2. `build.yml`, the job "Android app": a version tag's APK and AAB,
   a nightly build's APK with its version from `git describe`,
   unsigned, attested, as the artifact `release-app`;
   its failure leaves the binaries' release standing.
   `release.yml`: "Sign the Android app", "Publish the Android app"
   and "Attest the signed files" beside the release's jobs;
   `sign` and `rolling` sign `SHA256SUMS`, and `rolling` signs the nightly APK.
3. `packaging/app-version.sh`, the formula's one place,
   used by `make release`, build.yml and the job;
   `packaging/app-sign.sh`, the signing and the comparison with the
   certificate, used by the three signing places.
   A debug build's versionName ends in `-dev`.

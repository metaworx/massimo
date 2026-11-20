# NOTE MobileApp v1.0: first signed nightly

The first nightly build to carry the app, checked on 2026-11-18
at the owner's `EXEC verify`.
nightly.yml's run 44285015696, started at 10:23 UTC and green,
built `release/nightly` of this commit:
"chore(aps): +NOTE TestInfrastructure v1.1, how block 8 went".
It was the first real run of the path Block 4 built for the nightly APK:
build.yml's job `app` built it unsigned,
and release.yml's `rolling` signed it with `packaging/app-sign.sh`.
For the nightly build, this closes what
`2026-11-17_18-53_NOTE_MobileApp_v1.0_how_block_4_went.md`
left open as the first real release jobs.

## What was checked

On the three files downloaded from `release/nightly`:
`massimo-android-nightly.apk`, `SHA256SUMS` and `SHA256SUMS.sig`.

1. **`SHA256SUMS.sig`** verifies against `internal/update/rolling-key.pem`
   (`openssl pkeyutl -verify -rawin`),
   and the release key, `internal/update/release-key.pem`, refuses it.
2. **The APK's line** in `SHA256SUMS` matches it (`sha256sum -c --ignore-missing`).
3. **The APK's signature** passes `apksigner verify --min-sdk-version 26`,
   with schemes v2 and v3, neither v1 nor v4, and one signer,
   `CN=Massimo rolling, O=The Massimo Authors`.
   Its certificate's SHA-256,
   `bc469e9874663a66bb574d5dfb938276c8802782c1822f7199408d178bca8881`,
   is that of `app/android/signing/rolling-cert.pem`,
   and neither the app signing key's nor the upload key's.
4. **The app's ID, label and version** (`aapt2 dump badging`):
   `ch.metaworx.massimo.nightly`, "Massimo nightly",
   versionName `0.16.0-alpha.1-301-g<commit>`,
   git describe's name of the commit,
   which the nightly's Debian packages carry too,
   and versionCode `16000301`:
   D21's code of 0.16.0 with 301 commits since `v0.16.0-alpha.1`,
   the oldest tag of 0.16.0.

## Not checked

- **The build provenance attestation** of the signed APK,
  release.yml's `attest`, at the owner's word (2026-11-18).
- **A version tag's signing** of the APK and the AAB,
  release.yml's `app-sign` and `app-publish`,
  which first runs at the next `make release`, the owner's.

# NOTE MobileApp v1.0: how block 4 went

How Block 4, Release, ran on 2026-11-17 under the owner's `EXEC++ 4`,
where it went otherwise than the plan and its refit NOTE,
`2026-11-17_18-27_NOTE_MobileApp_v1.0_block_4_refit.md`, say.
Its pull request is metaworx/massimo#349.

## Where it went otherwise

1. **The branch** was made from `main` when the gate was written,
   and `main` had moved on by the signal (plan commits, UnsavedMarks' Block 1);
   it was moved to the newest `main` before anything was written.
2. **The nightly icon** was asked for with the signal, not at the gate.
   Its layers are `make ui-brand`'s;
   only the composition, `src/nightly/res/mipmap-anydpi/ic_launcher.xml`,
   and its background colour are written by hand,
   as `main`'s composition is.
   A themed icon tints the same layer for both apps,
   and the label tells them apart there.
3. **The unsigned files are attested too**, in build.yml,
   as D15-A had it and D15-D kept:
   so a rebuild of a tag, F-Droid's among them,
   can be compared with the files the release signed.
4. **build.yml's job runs with `continue-on-error`**:
   a failed app build leaves the boxes' release standing,
   and the signing jobs say that it carries no app.
5. **Additions the plan did not name**:
   `app/android/signing/README.md`, where the certificates go;
   `*.p12` in `app/android/.gitignore`;
   the release notes' sentence names `SHA256SUMS.sig`;
   `README.md` shows how to check it with the release key.
6. **A scratch script for the keys**, `app-keys.sh`,
   is in the main checkout's `GUIDELINES/temp/`, at the owner's word:
   it makes the three keystores, exports the certificates
   and sets the six secrets without a password on a command line.
   It is not committed.

## Verification

Locally, only what the change touches:

- `go test ./cmd/massimo/ ./tools/identifiers` (the new `TestAppVersion`),
  `gofmt`, `go vet` for linux, windows and darwin;
- `make actionlint`, shellcheck and `bash -n` on both scripts;
- the Gradle tasks: the debug build, the unit tests,
  lint of `foss` and `nightly`, the three release outputs;
  `aapt2` shows the versions and the nightly's id and label;
- the `foss` and `nightly` release APKs built from the committed tree
  at two paths: the same, byte for byte;
- `app-sign.sh` with throwaway keys: the three files signed and checked,
  a wrong certificate refused with nothing written;
- `GUIDELINES/shared/tools/check`: 12 of 12.

The whole suite, the race run and the cross builds were left to CI.
CI's run of the pull request passed, every job.
The new job took four minutes, the UI test's longest job ten:
the version file was its tag's,
the release APK the same at two paths on the runner,
the three files signed and the two against another certificate refused;
All tests passed wrote `app=true` into the Tested record.

## Still open

- The owner's: the three keys, their six secrets,
  and the three certificates under `app/android/signing/`;
  until then a release warns and carries no app.
- The release jobs run first at the next version tag
  and the next nightly build after the merge;
  `make release alpha DRY=1` can show the file only on `main`.
- The comparison is of two paths on one machine;
  whether F-Droid's buildserver makes the same APK is Block 5's to see.
- Dependabot has no entry for Gradle;
  the app's dependencies move by hand until a block adds one.

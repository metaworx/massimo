# The app's signing certificates

The public certificates of the Android app's three signing keys,
in PEM, which the release workflow compares each signer with
before a signed file is published (`packaging/app-sign.sh`):

| File | Key |
|---|---|
| `app-cert.pem` | the app signing key, the released APK's |
| `upload-cert.pem` | Google Play's upload key, the AAB's |
| `rolling-cert.pem` | the rolling key, the nightly APK's |

The owner makes the keys and adds the certificates here;
until one is here, a release carries no file of its key.
The keys themselves never enter the repository (`app/README.md`,
"The signing keys").

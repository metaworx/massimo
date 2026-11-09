> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.

# Building from source

Every release can be built from its signed tag,
with git, make and Go 1.26 or later (<https://go.dev/dl/>):

```bash
git clone https://github.com/metaworx/massimo.git
cd massimo
git -c gpg.ssh.allowedSignersFile=/dev/null verify-tag <tag>
git checkout <tag>
make
```

`verify-tag` checks the tag's signature.
With no signer it trusts (`/dev/null`), git prints the key's fingerprint,
then `No principal matched.`, and exits 1;
the fingerprint must be `SHA256:R3Ihevc2jMppLJtKIhmpfRmulpf7DXuWBFbCjaU3qcs`,
which the guidelines' README names as well, under "Verifying a release",
<https://gitlab.com/metaworx/open-source/mwx-guidelines>,
on a host this repository does not control.
The key itself is deliberately not given here:
whoever could change the tags could change the line naming it.

`make` runs the tests and builds `./massimo` for this machine,
`make build` without the tests;
`make linux-armv6` builds for a Raspberry Pi 1, `make linux-arm64` for a 64-bit
Raspberry Pi OS, `make windows` for Windows,
and `make cross` every release binary, all into `dist/`.
The binary goes in as a release's does,
`sudo ./massimo install -dry-run -settings .env`, then without `-dry-run`.
It follows the tag's update channel,
so the web UI and `massimo update` say when a release is out,
and they install the project's binary, checked against its signed manifest;
to stay on your own builds, build the next tag and run `install` again,
which replaces the binary and keeps the settings.
A tag up to v0.12.0-alpha.1 follows `HEAD` instead, which has no releases,
unless built with `make CHANNEL=alpha`.

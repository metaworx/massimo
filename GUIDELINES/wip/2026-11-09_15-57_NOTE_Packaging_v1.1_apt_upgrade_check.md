# NOTE Packaging v1.1: apt upgrade check

What the owner's checks of
[AP Packaging v1.1](2026-11-06_13-55_AP_Packaging_deb_apt_homebrew.md)
found on 2026-11-08 and 2026-11-09: the steps that
[the NOTE of Blocks 1 to 3](2026-11-06_14-35_NOTE_Packaging_v1.0_blocks_1_to_3.md)
leaves in "The owner's steps left", and roadmap check 1.10.
Nothing was sent over the radio;
names and addresses are placeholders here.

1. **The apt upgrade (Blocks 1 and 2, roadmap check 1.10)**, on 2026-11-08,
   on a Raspberry Pi 5 without a Cube,
   with Massimo 0.10.0~alpha.1 from the suite `alpha`
   and the marker `/usr/share/massimo/packaged` (`apt`,
   the upgrade command, the remove command):
   - Signed in, the owner saw Update offer 0.11.0-alpha.1 as newer,
     "managed by apt" with `sudo apt update && sudo apt install massimo`,
     and the button install nothing.
   - `GET /api/v1/update` answers 401 without a token on a box with users
     and no `MASSIMO_API_TOKEN`: only a signed-in user sees it, as intended.
   - `sudo apt-get update && sudo apt-get install --only-upgrade massimo`
     brought 0.11.0~alpha.1 and left the box's other upgrades alone.
     The service restarted with a new PID and no loop,
     stayed enabled and kept its `listen.conf`;
     `/healthz` reported the new version,
     the store migrated from schema 5 to 7 and made its OPAQUE key,
     and the only warning was "no cube configured yet".
     The marker was unchanged.
2. **An apt source for the regular `apt update` and `apt upgrade`**
   (the owner's question):
   the `.sources` file of `packaging/README.md`, "The apt repository",
   after the keyring package, is that;
   `unattended-upgrades` takes the repository only with `"origin=Massimo";`
   in its `Origins-Pattern`, which the same section says.
   Nothing to change.
3. **A correction to the NOTE of Blocks 1 to 3, Block 2, item 2:**
   the keyring package's version is the time of the publish,
   `1.0.<UTC time to the second>`,
   since `fix(apt): version the keyring package by the time of the publish`,
   and no longer the key's expiry date.
   The box had `1.0.20261107084637`, from the publish of 0.10.0-alpha.1.
4. **The DNS-01 hooks need `curl` and `jq`** (found by the TLS checks):
   the package ships both hooks and names neither,
   and a hook run on a box without `jq` fails.
   AP HookDeps takes it:
   the hooks check for both, and the package suggests them.
5. **The ruleset (the owner's step)**, on 2026-11-09:
   `main` requires Vet and test, Guidelines, govulncheck (linux),
   govulncheck (windows), Windows service, macOS LaunchDaemon and Screenshots;
   "Debian package (debian:trixie-slim)" and "(debian:bookworm-slim)"
   are not among them yet.
6. **The tap is published:**
   `metaworx/homebrew-massimo` exists,
   with `Formula/massimo-alpha.rb` for v0.12.0-alpha.1,
   pushed by the publish workflow on 2026-11-09.
   Not checked: `brew install` and `brew services` on a Mac (roadmap check 1.7).

# NOTE Packaging v1.1: block 11 site

The owner's decisions at Block 11's gates of 2026-11-09,
and where the block went differently from the plan.

## Decisions

1. **D12, the contact:**
   `Martin Rüegg <massimo@metaworx.ch>`, the owner's choice of address,
   on the privacy page, as the package's maintainer in `packaging/nfpm.yaml`
   (the keyring package takes it from there),
   and as `Upstream-Contact` in `packaging/deb/copyright`;
   the two packaging files are outside the block's list.
   Released packages keep the address they were built with.
2. **D13-C, one text for the page and the README:**
   the owner asked the guidelines session "CLI scripts libraries"
   (mwx-guidelines) to teach `sync-docs.sh` to fill a `GENERATED PARTIAL` block
   in an HTML document; the request went out on 2026-11-09.
   Until a guidelines release with it reaches Massimo's submodule pin,
   `packaging/site/index.html` is written by hand;
   its links are absolute, as a fragment shared with `README.md` needs them.
   Moving the shared parts into a fragment is a later change.
3. **D14-A, Homebrew on the page:**
   the owner made the tap `metaworx/homebrew-massimo` and its deploy key on 2026-11-09;
   the tap holds `Formula/massimo-alpha.rb`.

## Where the block went differently

1. **The old address works for apt again.**
   Checked at 05:25 on 2026-11-09: GitHub answers
   `https://metaworx.github.io/massimo/apt/…` with a 301 straight to
   `https://massimo.metaworx.ch/apt/…`,
   and `apt-helper download-file` on the old address succeeds.
   Earlier that day the redirect went through `http://`, which apt refuses.
   The hand edit of the sources line stays recommended,
   since the redirect is GitHub's and not documented;
   `packaging/README.md`, "Moving from the old address", says both.
2. **What the privacy page describes beyond the plan's list:**
   - the server's own connections: GitHub for the release check,
     when System settings is opened or `massimo update` runs;
     the certificate authority and the DNS provider with `-tls acme`;
   - notifications as AP MobileApp and AP ServerAnnouncements plan them:
     Web Push, encrypted for the device, through UnifiedPush,
     or Firebase Cloud Messaging in the Google Play build.
   Where those blocks land differently, the page changes with them.
3. **The index page has no line for the apps**, which do not exist yet.
4. **`publish.sh`** puts the suites into the placeholder joined by ", ".
   The page's snippet names the project's address whatever `--base-url` says;
   only the `.sources` sample follows the option.
5. **`GUIDELINES/project/CI.md`** is v1.14.0:
   section 6 gains a fact, which GOVERNANCE.md §1.1 counts as a minor change.
6. **Verification:**
   tidy is not installed on the build machine, so `tidy -q -e` (5.8.0) ran in a
   `debian:trixie-slim` container, on the pages of a site `publish.sh` built;
   the pages were rendered at 390 and 1100 px, light and dark,
   with the Playwright venv of the main checkout,
   and no element is wider than the viewport.
   Left to the owner: the pages served after the next release,
   `privacy.html` answering 200, and `apt update` on one Pi after the hand edit.

# NOTE InstallText v1.0: how the run went

Where the run of Blocks 1 to 4 under the owner's `PLAN++` of 2026-11-09
went differently from the plan.

## Where it went differently

1. **The register commit** went onto `84f56ee`:
   `main` gained #132 while the plan was written,
   so the plan was committed on the new `main` and pushed there.
2. **A fragment's banner is one line.**
   Over two lines it is not recognised,
   and its `#` title came through into the block.
   The banner is
   ``> **Fragment** - a partial for `sync-docs.sh`; not a standalone document.``
3. **sync-docs.sh reflows a block in a markdown host**,
   so a fragment that reflow would change comes out rewrapped in `README.md`,
   a link's text broken across lines.
   Every fragment is written so that `reflow.sh --check` finds nothing in it;
   README's blocks are then the fragments as written (compared line by line).
4. **The tag check needs no config change:**
   `git -c gpg.ssh.allowedSignersFile=/dev/null verify-tag <tag>`
   prints the fingerprint and `No principal matched.`, exit 1,
   where the plan had an empty `allowed_signers` file.
5. **The guidelines' README** is linked as the project,
   <https://gitlab.com/metaworx/open-source/mwx-guidelines>,
   with its section "Verifying a release" named in the text:
   the README file's own address, with the anchor, is longer than reflow allows,
   and a reference definition fails it too.
6. **The page's Debian paragraph** names all three ways to the first owner:
   `massimo user add` on the box, the token in `/etc/massimo/env`,
   or the web UI once the service runs;
   the apt route says to create the owner and start the service.
7. **Block 4 also points Block 2's CHANGELOG bullet** to "From source",
   where the text on a tag's channel now is.
8. **`make apt-test` first failed** with "the update helper is not enabled":
   `dist/` still held the packages built at 05:26 for AP Packaging's Block 11,
   from before ServiceStart.
   Rebuilt from this tree, it passes.
9. **A command ran by mistake on the development machine** while Block 3's
   fragments were written:
   the apt snippet's own `EOF` line closed the shell's here-document early,
   and the lines after it ran.
   `sudo apt update` refreshed the package lists;
   `sudo apt install massimo` failed ("Unable to locate package").
   Nothing was installed (`/var/log/apt/history.log` has no entry),
   no sources file or keyring was written, no `massimo` binary is on the `PATH`,
   and nothing reached a Cube.
   The fragments were then written with the file tool, not a here-document.

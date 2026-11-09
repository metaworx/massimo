# AP InstallText v1.0: Shared install text

## TL;DR

The install texts that `README.md`, `packaging/README.md` and the site's pages
must agree on are written once, in `docs/fragments/`,
and filled in by the guidelines' `sync-docs.sh` (v4.53.0, which fills an HTML page too);
two texts are new: what a binary's checks prove, and building from source.

- guidelines v4.52.1 → v4.53.0
- the first settings, the apt snippet, the trademark notice → one fragment each
- the page's start rule, stale since ServiceStart → the package's rule as it is now
- new `_trust.md`: a binary is only as trustworthy as its source;
  `SHA256SUMS` proves the download, not the origin → "From source"
- new `_build.md`: Go 1.26, `git clone`, `git verify-tag` against the fingerprint,
  `make`, then `install` as a release's binary
- Makefile: a release tag's checkout, channel `HEAD` → the tag's channel, as `release.yml`

### Blocks

1. Pin: `GUIDELINES/shared` at v4.53.0, the generated documents regenerated
2. TagChannel: the Makefile takes a release tag's channel on a detached HEAD; the bullet
3. Fragments: `_first-settings.md`, `_apt-sources.md`, `_trademark.md`
   in `README.md`, `packaging/README.md` and both pages; the page's start rule; the bullet
4. FromSource: `_trust.md` and `_build.md`, README's "From source" and the page's route;
   the bullet

## Discussion

The owner asked on 2026-11-09, after the guidelines session released v4.53.0
(D13-C of AP Packaging, Block 11), for a `PLAN` with:
the pin; a note that installing binaries from an untrusted source is a risk,
pointing to the build instructions,
in the manner of the guidelines README's note on the signing key;
build instructions; D17-A (the new pages go live with the next alpha release,
which the owner cuts); D18-A (a plan of its own).

**The owner's decisions:**
`PLAN++` (2026-11-09) on the TL;DR gate, naming no option:
the recommended ones.

- **D19-A:** the fragments live in `docs/fragments/_<name>.md`;
  `packaging/site/` would serve them, since `publish.sh` copies all of it.
- **D20-A:** shared are the first settings, the apt snippet,
  the trademark notice and the two new texts; Homebrew stays as it is,
  its README paragraph and the page's route saying different things.
- **D21-A:** the Makefile takes the channel from a release tag on a detached HEAD.
- **D22-A:** the build text verifies the tag with `git verify-tag`
  and names the fingerprint; the key itself is not given.

**Names chosen here**, routine:
the plan `InstallText`, the branch `claude/install-text`,
the fragments `_first-settings.md`, `_apt-sources.md`, `_trademark.md`,
`_trust.md` and `_build.md`,
README's new subsection `### From source` (anchor `#from-source`).

**Not in this plan:**
the release (D17-A, the owner's);
a site deploy without a release (D17-B);
the Homebrew texts;
publishing the release key's public half anywhere.

**Who runs it:**
this session, under the owner's `PLAN++` of 2026-11-09.
Nothing reaches a Cube, a Pi or the site; the pages go live with the next release.

## Analysis

### The pin

v4.52.1 → v4.53.0 adds `md_html.py` with markdown-it-py 4.0.0 and mdurl 0.1.2
vendored in `tools/third_party/`, PARTIAL blocks in a tracked `.html` file,
and markers rewritten at their own column.
The shared fragments `_AGENTS.md` and `_README.md` are unchanged,
so `AGENTS.md` and `GUIDELINES/README.md` regenerate as they are.
Earlier pins were `chore(guidelines): take the guidelines v4.52.x`,
`install.sh --ref` from the project's own submodule, which moves the pin
and leaves the scaffold.
`sync-docs.sh` then needs Python 3.10;
CI's Guidelines job on `ubuntu-latest` has 3.12, and so does this machine.

### Partials (GOVERNANCE.md §1.8, `sync-docs.sh --help`, v4.53.0)

- **Markers are the declaration:** a tracked markdown or HTML file carrying
  `<!-- BEGIN GENERATED PARTIAL <path> -->` and its END is a host;
  nothing is listed in `config.ini`.
- **A fragment** opens with the `> **Fragment**` banner and a `#` title,
  both dropped in the block.
- **Placeholders are substituted** in the block, so a fragment carries no `{{`:
  the page's `{{SUITES}}` stays outside every block.
- **Links are copied as written:** the hosts sit in three places and the page
  is served from another host, so every link in a fragment is absolute,
  README's relative ones included.
- **In a markdown host** the block is reflowed with that document's settings;
  the fragments are written in the reflow style, so the block is the fragment.
- **In an HTML host** the block is converted by `md_html.py`:
  headings below the `<h1>`–`<h6>` above it,
  lines outside `<pre>` indented to the marker,
  a fence as `<pre><code class="language-…">`,
  raw HTML shown as text, no reflow.
  A placeholder such as `<the box>` goes in a code span,
  which GitHub's markdown would otherwise take for a tag.
- **CI's Guidelines job** runs `check`, whose `documents` step fails a stale block.

### The texts and their hosts

| Fragment | `README.md` | `packaging/README.md` | `index.html` | `privacy.html` |
|---|---|---|---|---|
| `_first-settings.md` | Installing, after the `.env` sentence | – | "The first settings" | – |
| `_apt-sources.md` | – | "The apt repository" | the apt route | – |
| `_trademark.md` | Legal | – | footer | footer |
| `_trust.md` | Installing, after the release paragraph | – | Install, first | – |
| `_build.md` | `### From source`, after macOS | – | the route "From source" | – |

- **The first settings:** README's paragraph and the page's say the same since D16-A,
  in different words; the fragment takes README's,
  with "the server" for README's "it" and the address `http://<the box>:8099/`
  from the page.
  README keeps its own first sentence (the `.env` beside `serve`,
  a relative link) and its paragraph on the Cube's one client;
  the page keeps its sentence on the original MAX! software
  and its paragraph on the Debian package.
- **The apt snippet** is the same bash fence in `packaging/README.md` and the page;
  the sentences around it stay each host's.
- **The trademark notice** is README's two sentences, as D15-A had them.

### The page's start rule after ServiceStart

AP ServiceStart (PR #133, retired 2026-11-09) changed what the Debian package does:
an upgrade restarts a running service whatever its settings,
a box that never ran stays stopped until its owner starts it,
and the postinst names `massimo user add` and the web UI.
The page still says that the package starts the service,
and restarts it after an upgrade, only once `/etc/massimo/env` differs from
the template, and its apt route asks for a line in that file.
Block 3 says it as it now is.
`massimo install -settings <file>` on a box without settings still copies the
file and starts the service, an empty file too (`internal/install`, `settings`),
so the routes' "an empty one will do" holds.

### What the checks prove

- **`SHA256SUMS`** sits on the release page beside the binaries:
  it proves that a download arrived as the release has it,
  not who built it; whoever could replace a binary could replace the file.
- **The signed manifest** (`massimo-release.json`) is checked by `massimo update`
  and the web UI's button against the release key compiled into the binary
  that runs, so every later release is verified,
  and what is left to trust is the first binary.
- **The apt repository** is signed by its own key;
  the keyring package is fetched over HTTPS once (`packaging/README.md`).
- **Homebrew** builds from the release's source,
  the tarball pinned by the formula in the tap.
- **A copy from anywhere else** - a mirror, a forum, a prepared image -
  carries none of these, and nothing here can tell what was changed.

The guidelines' README states its own case the same way:
a bold sentence saying what a check cannot buy, then why.
`_trust.md` follows that form and ends with the way out: building from source.

### Building from source

- **Go 1.26 or later** (`go.mod`), git and make.
  `make` runs the tests and builds `./massimo`; `make build` builds only;
  `make linux-armv6` and the others build for another machine into `dist/`,
  `make windows` the Windows binary on Linux or macOS.
- **The release tags are SSH-signed** with the key whose fingerprint is
  `SHA256:R3Ihevc2jMppLJtKIhmpfRmulpf7DXuWBFbCjaU3qcs`
  (`git verify-tag v0.12.0-alpha.1`, checked 2026-11-09).
  The guidelines' README on GitLab names the same fingerprint,
  a second host this repository does not control.
  Measured: without `gpg.ssh.allowedSignersFile` git refuses
  ("needs to be configured and exist");
  with the file empty it prints `Good "git" signature with ED25519 key SHA256:…`
  and `No principal matched.`, exit 1;
  with the key in it, the signature is good and attributed, exit 0.
  As in the guidelines, the key itself is not given:
  whoever could change the tags could change the line naming it.
- **A self-built binary goes in as a release's does:**
  `sudo ./massimo install -settings .env`.
- **Updates:** `massimo update` and the web UI's button install the project's
  binary, verified against the manifest;
  `massimo install` run again from a newer own build replaces the binary
  and keeps the settings, which is the way to stay on one's own builds.

### A tag's checkout and its channel

`CHANNEL ?= $(shell git rev-parse --abbrev-ref HEAD)`:
a release tag's checkout is a detached HEAD, so its build follows the channel `HEAD`.
`internal/update.Latest` keeps the releases whose tag or target names the
channel and returns the newest whose manifest says it,
so a build on `HEAD` never hears of a release, in the web UI or from `massimo update`.
`release.yml` passes `CHANNEL` explicitly and is not affected;
it takes the tag's pre-release label (`alpha` of `v0.12.0-alpha.1`),
else a `channel:` line of the tag's message.
The Makefile does the same on a detached HEAD at a tag, and keeps the branch otherwise.
A tag made before the change keeps its own Makefile:
for v0.12.0-alpha.1 and older, `make CHANNEL=alpha`.

## Implementation Plan

On a branch `claude/install-text` from `main` after the register commit,
one signed commit per block, one pull request.

### Block 1: Pin

1. Populate `GUIDELINES/shared` in this worktree
   (`git submodule update --init`, without recursion elsewhere).
2. `GUIDELINES/shared/tools/install.sh --ref v4.53.0` from the submodule;
   it moves the pin and adds nothing that exists.
3. `GUIDELINES/shared/tools/sync-docs.sh`; `AGENTS.md` and `GUIDELINES/README.md`
   unchanged.

**Verification:** `git diff --cached` is the gitlink alone;
`sync-docs.sh --check` and `check --commit-range origin/main..HEAD` clean
(the `documents` step now runs in the worktree).

### Block 2: TagChannel

1. `Makefile`: `CHANNEL` is the branch; on a detached HEAD at a tag,
   the tag's pre-release label, else its message's `channel:` line, else `HEAD`;
   the head comment says so.
2. `README.md`: the sentence on a build's channel names the tag's.
3. `CHANGELOG.md`, Fixed:

   ```
   - A build from a release tag's checkout follows the tag's update
     channel, such as `alpha`, no longer `HEAD`, which has no releases
     (`README.md`, "Installing").
   ```

**Verification:** in a scratch clone, `make -n build` shows
`main.channel=alpha` at `v0.12.0-alpha.1`, the branch's name on a branch,
`HEAD` on a detached commit without a tag,
and `beta` at an unlabelled test tag whose message says `channel: beta`;
`make cross` and the test gate pass.

### Block 3: Fragments

1. `docs/fragments/_first-settings.md`, `_apt-sources.md`, `_trademark.md`,
   each with the banner and a title.
2. The markers in `README.md` (Installing, Legal), `packaging/README.md`
   (The apt repository), `packaging/site/index.html` (the apt route,
   "The first settings", the footer) and `packaging/site/privacy.html` (the footer);
   `sync-docs.sh` fills them.
3. The page's apt route and its paragraph on the Debian package say the start
   rule as ServiceStart made it.
4. `CHANGELOG.md`: the site's bullet in Changed names the first settings:

   ```
   - The apt repository's address is `https://massimo.metaworx.ch/apt`,
     under the project's site: the install routes, the first settings and
     the privacy policy (`packaging/README.md`, "The apt repository").
   ```

**Verification:** `sync-docs.sh --check` clean;
README's and packaging/README's rendered text unchanged but for the first settings'
wording; `tidy -q -e` on both pages of a site `publish.sh` built;
`make apt-test`; the pages at 390 and 1100 px, light and dark, nothing wider
than the viewport; reflow adds no warning; `go test ./tools/identifiers`.

### Block 4: FromSource

1. `docs/fragments/_trust.md`: what a binary's checks prove, and the way out,
   linking `https://github.com/metaworx/massimo#from-source`.
2. `docs/fragments/_build.md`: the tools, the clone, `git verify-tag` with the
   fingerprint and the two-host check, the checkout, `make`, the cross builds,
   the install, the updates, and `make CHANNEL=alpha` for v0.12.0-alpha.1 and older.
3. `README.md`: `_trust.md` after the release paragraph;
   the build paragraph moves into a new `### From source` after macOS,
   `_build.md` first, then the remaining `make` targets and the channel of `main`;
   Contents gains the subsection.
4. `packaging/site/index.html`: `_trust.md` first under Install,
   and the route `<h3 id="from-source">From source</h3>` with `_build.md`.
5. `CHANGELOG.md`: the site's bullet names building from source:

   ```
   - The apt repository's address is `https://massimo.metaworx.ch/apt`,
     under the project's site: the install routes, building from source,
     the first settings and the privacy policy (`packaging/README.md`).
   ```

**Verification:** as Block 3's, and the build text followed in a scratch clone:
`git verify-tag` with an empty `allowed_signers` prints the fingerprint,
`git checkout v0.12.0-alpha.1 && make build CHANNEL=alpha` builds,
`./massimo version` names the tag.

## Proposed commit messages

1. `chore(guidelines): take the guidelines v4.53.0`:
   v4.53.0 fills a GENERATED PARTIAL block in a tracked HTML page,
   which the next blocks use; AGENTS.md and GUIDELINES/README.md regenerate unchanged;
   no CHANGELOG bullet, nothing shipped changes.
2. `fix(makefile): A build from a release tag follows the tag's channel`:
   the label of the tag, else its message's `channel:` line, as `release.yml`;
   README; the Fixed bullet.
3. `docs(packaging): The site and the READMEs share their install texts`:
   the three fragments and their hosts, the page's start rule after ServiceStart;
   the bullet amended.
4. `docs(install): Building from source, and what a binary's checks prove`:
   the two fragments, README's "From source", the page's route; the bullet amended.

Each cites `AP InstallText v1.0, Block <n>` and ends with
`Assisted-by: claude-code:claude-opus-5-5`.

## Change History

- **v1.0** (2026-11-09): the first revision, by the owner's `PLAN++`
  with D19-A, D20-A, D21-A and D22-A.

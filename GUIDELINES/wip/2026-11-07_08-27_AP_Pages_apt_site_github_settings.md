# AP Pages v1.0: the apt site's GitHub settings

## TL;DR

The apt repository goes live on GitHub Pages:
the settings the release workflow's `apt-build` and `apt-deploy` jobs wait for,
the key that signs the suites, and the proof at the next release.
No code changes; one document.

- Pages: off → source "GitHub Actions"
- environment `github-pages`: none → made with one deployment rule,
  the tags `v*`; both by `gh api`
- `signing` environment: the release key only → `MASSIMO_APT_KEY` and
  `MASSIMO_APT_KEY_PASSPHRASE` beside it, the key made offline by the owner
- proof: the next release's `apt-deploy` job → the site at
  <https://metaworx.github.io/massimo/apt/>, `InRelease` verified with
  `massimo.asc`
- then docs/roadmap.md, item 2, without the done steps
- not here: the Homebrew tap, the ruleset's Debian checks, the Pi's install
- base: the code of pull request 75, the review's Go stack

D1 to D3 are settled as recommended; the Discussion says why.
Block 1 is the implementing session's, Block 2 the owner's,
Block 3 the coordinating session's at the next release.

### Blocks

1. PagesSettings: Pages from Actions and the `github-pages` environment with its
   tag rule, by `gh api`, read back
2. AptKey: the owner's key and its two secrets; the session checks the names
3. PagesProof: the next release's deploy watched, the site checked,
   the roadmap updated

## Discussion

Three decisions, settled by the owner at the TL;DR gate as recommended.

### D1. Who makes the apt key and sets the secrets: A, the owner

- **A (chosen).**
  The owner, in a terminal, with Block 2's commands:
  the key stays in the owner's keyring for the renewal in two years
  (`gpg --quick-set-expire`, packaging/README.md, "The apt repository");
  the session sees the secret names only.
- **B.**
  The session, in a throwaway keyring under its scratchpad,
  piping the key and the passphrase into `gh secret set` unprinted,
  then wiping it.
  Nothing would be left to renew with,
  and the next key would mean the two-key keyring package that `publish.sh` does
  not make.

### D2. The proof of the deploy: A, the next release

- **A (chosen).**
  The next release, due once the review's Go stack is in:
  its site carries the review's publishing fixes (the keyring package readable
  by apt, the snippet's key path, the keyring version to the second),
  which the install on the Pi needs.
- **B.**
  Now, by re-running the `apt-build` job of the newest release's run:
  Pages and the rule proved within minutes, but the site of that tag published,
  with a keyring package of mode 0600 that apt cannot read and a snippet naming
  the wrong key path, until the next release replaces it.

### D3. The Homebrew tap: A, out of this plan

- **A (chosen).**
  Out, as asked; it stays in roadmap item 2.
- **B.**
  A fourth block: the repository `metaworx/homebrew-massimo`,
  an ed25519 deploy key,
  its private half into `HOMEBREW_TAP_DEPLOY_KEY` unprinted,
  then `gh workflow run publish.yml -f tag=<newest release>`.

## Analysis

### 1. What the workflow waits for

release.yml's `apt-build` job runs under the `signing` environment and skips
every step while `MASSIMO_APT_KEY` is empty;
with the key it imports it,
runs `packaging/apt/publish.sh --out site --key <fpr>`
(the passphrase from `MASSIMO_APT_KEY_PASSPHRASE`)
and uploads the site as the Pages artifact.
`apt-deploy` then runs `actions/deploy-pages@v4` under the environment
`github-pages` with `pages: write` and `id-token: write`.
That needs, on GitHub:

- the repository's Pages source set to "GitHub Actions"
  (`build_type: workflow`), else there is no Pages site to deploy to;
- the environment `github-pages` allowing the ref the job runs for:
  a release's tag.
  The environment GitHub makes itself allows the default branch only,
  so a deploy for a tag is refused by its protection rules.

### 2. The state on 2026-11-07, read by `gh api`

| Setting | Now | Needed |
|---------|-----|--------|
| Pages (`GET /repos/metaworx/massimo/pages`) | 404, `has_pages: false` | `build_type: workflow` |
| environment `github-pages` | none | one deployment rule: tags `v*` |
| environment `signing` | a branch policy for the tags `v[0-9]*.[0-9]*.[0-9]*`; the secret `MASSIMO_RELEASE_KEY` | `MASSIMO_APT_KEY` and `MASSIMO_APT_KEY_PASSPHRASE` beside it |
| repository secrets | none | none for this plan |

The gh token on jaguar carries `repo` and `workflow`,
and the owner is the repository's admin,
which the Pages and environment calls need.

### 3. The calls

Pages, from Actions:

```bash
gh api -X POST repos/metaworx/massimo/pages -f build_type=workflow
```

If GitHub refuses that form, the one click:
Settings → Pages → Build and deployment → Source: GitHub Actions;
`GET …/pages` then shows `build_type: workflow`.

The environment, with custom branch policies and no protected-branch rule,
then the tag rule:

```bash
gh api -X PUT repos/metaworx/massimo/environments/github-pages --input - <<'JSON'
{"deployment_branch_policy":{"protected_branches":false,"custom_branch_policies":true}}
JSON
gh api -X POST repos/metaworx/massimo/environments/github-pages/deployment-branch-policies \
  -f name='v*' -f type=tag
```

PUT creates or updates;
where GitHub has already made the environment (it does when the Pages source
changes, or at the first deploy), the PUT keeps it and the POST adds the rule.
The pattern `v*` is the one packaging/README.md documents;
the `signing` environment's stricter one would do as well,
and nothing else makes tags.

### 4. The key

RSA 4096, signing only, two years,
made offline by the owner (packaging/README.md, "The owner's steps, once"):

```bash
gpg --quick-gen-key "Massimo apt repository" rsa4096 sign 2y
gpg --list-secret-keys --with-colons "Massimo apt repository" | awk -F: '/^fpr/ { print $10; exit }'
```

Then the two secrets, the key piped so that it is never shown,
the passphrase typed at gh's prompt so that it is in no shell history:

```bash
gpg --armor --export-secret-keys <fpr> | gh secret set MASSIMO_APT_KEY --env signing -R metaworx/massimo
gh secret set MASSIMO_APT_KEY_PASSPHRASE --env signing -R metaworx/massimo
```

The public half needs no step:
`publish.sh` exports it as `massimo.asc` and into the keyring package at every
publish.
A key without a passphrase would work (`publish.sh` signs without one when
the secret is empty); the README asks for one, and the plan follows it.

### 5. The proof and the documents

At the next release (`make release-alpha BUMP=minor`, roadmap item 10),
the release run's `apt-build` job runs its steps for the first time and
`apt-deploy` deploys; the run shows the environment's URL.
The site: `https://metaworx.github.io/massimo/` with the snippet,
`apt/massimo.asc`, `apt/massimo-archive-keyring.deb`,
`apt/dists/alpha/InRelease` signed by the key,
the pool with the release's packages.
`packaging/apt/test.sh` is the container test of the same layout with a
throwaway key;
the Pi's install (roadmap item 1.10) is the owner's hardware check and stays
there.

If the deploy is refused, the fix is the rule (Block 1 step 2) and
`gh run rerun <run> --job <apt-build job>`:
a re-run of `apt-build` re-runs `apt-deploy` after it;
the release itself is unaffected.

Documents: docs/roadmap.md's item 2 loses the apt lines (open work only);
packaging/README.md's "The owner's steps, once" stays,
as the record for the next key and for a fork;
GUIDELINES/project/CI.md section 5 already describes the secrets.
No CHANGELOG bullet: no shipped path changes.

### 6. Risks

- The Pages call: `source` is required for the legacy build type only;
  if GitHub still refuses, the click is the fallback.
- The environment's rule made before Pages is on:
  harmless, the PUT is idempotent and the rule survives the Pages setup.
- The site is public from the first deploy, as the README's URL already says.

## Implementation Plan

Block 1 runs in the implementing session on jaguar with the owner's gh token;
Block 2 is the owner's, at a gate after Block 1;
Block 3 is the coordinating session's at the next release.
Blocks 1 and 2 change GitHub settings, Block 3 one document on `main`.
A session other than the coordinating one pushes nothing to `main`.

The base, the owner's instruction at the TL;DR gate:
the implementation is based on the code of pull request 75,
the head of the review's Go stack, which holds the review's Blocks 1 to 3.
The implementing session works on a branch made from that pull request's head
(merged with `origin/main`, as that stack does before its pull requests);
a file change the run turns out to need goes on that branch,
and its pull request targets the stack's head until the stack is in `main`,
then `main`.
The release of Block 3 is the one cut after that stack is in.

### Block 1: PagesSettings

1. Pages from Actions: the POST of Analysis 3;
   on a refusal, the click is the owner's, asked for at the gate.
2. The environment: the PUT with custom branch policies,
   then the POST of the tag rule `v*`.
3. Read back: `GET …/pages` shows `build_type: workflow` and the repository
   `has_pages: true`;
   `GET …/environments/github-pages/deployment-branch-policies` lists one
   policy, `tag v*`.

**Verification:**
the three reads of step 3, quoted at the gate as the API returns them;
nothing else changed: `gh secret list --env signing` as before,
the ruleset untouched.
The gate then offers Block 2 to the owner with its commands.

### Block 2: AptKey (the owner)

1. The key, offline, in the owner's keyring:
   `gpg --quick-gen-key "Massimo apt repository" rsa4096 sign 2y`,
   with a passphrase; the fingerprint from `gpg --list-secret-keys`.
2. `MASSIMO_APT_KEY` from `gpg --armor --export-secret-keys <fpr>` piped into
   `gh secret set MASSIMO_APT_KEY --env signing`,
   and `MASSIMO_APT_KEY_PASSPHRASE` typed at the prompt of
   `gh secret set MASSIMO_APT_KEY_PASSPHRASE --env signing`.
3. The owner says so; the session runs `gh secret list --env signing` and
   sees the two names beside `MASSIMO_RELEASE_KEY`.

**Verification:**
the three names in `gh secret list --env signing`;
no value anywhere in a transcript, a file of the checkout or a log.
The implementing session's completion gate follows.

### Block 3: PagesProof (the coordinating session, at the next release)

1. The release run watched: `apt-build` runs its steps (the import,
   `publish.sh`,
   the artifact) and `apt-deploy` succeeds with the environment's URL.
2. The site checked from jaguar: the index page with the snippet;
   `apt/massimo.asc` importable;
   `apt/dists/alpha/InRelease` verifying with it (`gpg --verify`);
   `apt/massimo-archive-keyring.deb` holding
   `/usr/share/keyrings/massimo-archive-keyring.gpg` at mode 0644
   (`dpkg-deb -c`); the pool holding the release's `.deb` files;
   `apt/dists/alpha/main/binary-all/Packages` naming the keyring package.
3. docs/roadmap.md item 2: the apt lines gone; the Pi's install stays item 1.10.
   One docs commit on `main`.
4. `EXEC retire` is the owner's; the plan retires in the commit after.

**Verification:**
steps 1 and 2 quoted at the completion gate with the run id and the
`InRelease` verification line;
`reflow.sh --check docs/roadmap.md` clean;
`commit-msg.sh` on the docs commit.
If the deploy is refused: the rule checked,
then `gh run rerun <run> --job <apt-build job id>`.

## Proposed commit message

The plan's registration, pushed to `main` so that the implementing session's
worktree holds it:

```
chore(aps): +Pages v1.0

The apt repository's GitHub settings get their plan: Pages from Actions and the github-pages environment with its tag rule by the implementing session, the key and its secrets by the owner, the proof at the next release.

File-revision-v1: GUIDELINES/wip/2026-11-07_08-27_AP_Pages_apt_site_github_settings.md::v1.0::2026-11-07::All::the first revision, D1 to D3 settled at the TL;DR gate
Assisted-by: claude-code:claude-fable-5-1
```

Block 3's docs commit, written at its gate, in the form of
`docs(roadmap): the apt repository's settings out of the owner's steps`.

## Change History

- v1.0 (2026-11-07): the first revision,
  from the TL;DR gate of the same morning;
  D1 to D3 settled there as recommended.

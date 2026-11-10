# AP Site v1.0: docs on the site

## TL;DR

The site, <https://massimo.metaworx.ch>, gets a builder and a workflow of
its own, so that a push to `main` updates it without a release, and it
carries the documentation, rendered per channel.

- `release.yml`'s `apt-build` → builds and signs `apt/` only; the pages,
  the docs and the deploy → `site.yml`, called by every `release.yml` run
- a run without a freshly signed `apt/` → the newest release's
  `massimo-apt-index.tar.gz`, the pool fetched by its `pool.tsv` and checked;
  no release with one yet → no deploy, as today
- `docs/*.md`, `README.md`, `packaging/README.md`, `GUIDELINES/README.md`
  → HTML with `md_html.py` (guidelines v4.55.0): heading ids, links
  rewritten
- per channel: `/docs/<channel>/` from `release/<channel>`, `/docs/main/`,
  a selector, `/docs/` → the most stable one
- the list of documents → one fragment, in README.md and the docs index
  (D13-A); a document missing from it fails the build
- CI → a `Site` job builds the docs of every pull request

### Blocks

1. Guidelines: the submodule at v4.55.0
2. The list: one fragment of the documents, in README.md
3. The builder: `packaging/site/build.sh`, the page template, the selector
4. The workflows: `site.yml`, `release.yml`'s apt jobs, CI's `Site` job
5. The documents: the home's link, `packaging/README.md`, CI.md, CHANGELOG

## Discussion

**Decided** in the Site analysis (owner, 2026-11-10), and by `PLAN++ Site`:

- **D1-A.** A workflow of its own, still a Pages artifact.
- **D2 → Channels D12-A.** The apt key stays with the tags; a run of `main`
  takes the signed index from the newest numbered release's
  `massimo-apt-index.tar.gz` and rebuilds the pool from its `pool.tsv`.
- **D3-A.** Docs built at deploy time, not tracked; built in CI.
- **D4-A.** `md_html.py`'s own options (guidelines v4.55.0).
- **D5-A.** All of `docs/*.md` but `fragments/`, plus `README.md`,
  `packaging/README.md` and (D9-A) `GUIDELINES/README.md`, grouped.
- **D6-A.** Docs per channel, built by the site itself.
- **D13-A** (open at WorkDocs' completion gate, taken as recommended under
  `PLAN++`): one fragment, `docs/fragments/_documents.md`, is the grouped list;
  README.md includes it as a PARTIAL, the docs index is built from it, and
  a `docs/*.md` it does not list fails the build of `main`'s docs.

**Decided here** (none needs the owner; each the simpler of two):

- **No apt index yet → no deploy.** A deploy replaces the whole site, so one
  without `apt/` would take the repository offline. Until the owner's next
  `make release-alpha` attaches the first index, only a release with the key
  deploys, as today; the builder says so and the job ends green.
- **A channel whose build has no `docs/fragments/_documents.md`** (every
  build before this plan) gets no docs: its `docs/` still holds the
  working documents, and publishing them is what WorkDocs avoided.
  `/docs/main/` is there from the first deploy; the channels follow with
  their first build after this plan.
- **One stylesheet**, `packaging/site/site.css`, for the home, the privacy
  policy and the docs, in place of the home's inline `<style>`.

## Analysis

### What is on `main` (bb2095b)

- `release.yml` is called by `ci.yml` after "All tests passed" on every push
  (and by its own schedule, and by hand). `meta` gives `kind`: `release` for
  a `v*` tag, `devel` for a push to `main`, `nightly` on the schedule, `skip`
  where nothing is to be released.
- `apt-build` (kind `release`, `signing` environment, the key) runs
  `publish.sh --out site --index-asset …`, attaches the index to the
  release, and uploads `site/` as the Pages artifact; `apt-deploy` deploys
  it. Without the key neither runs.
- `publish.sh` builds `apt/` and then copies `packaging/site/` to the site's
  root, the suites in place of `{{SUITES}}` in `index.html`.
- `packaging/apt/test.sh` (`make apt-test`, the `deb` job) builds a site
  from `dist/` with a throwaway key and checks the pages and the signature.
- No `release/*` ref exists yet: neither `release/alpha` nor
  `release/devel` (the first comes with the merge's CI run).

### The builder

`packaging/site/build.sh --out <dir>`, keyless:

1. **`apt/`**: `--apt <dir>` copies one just built (the release's run);
   `--apt-index <file|newest>` unpacks an index asset (`newest`: the newest
   `v*` release carrying one), fetches every pool entry whose URL is not
   `included`, and checks every entry's SHA-256; `--no-apt` builds the pages
   and docs alone (CI). No index found: exit 3, nothing written.
2. **The pages**: `packaging/site/*.html` and `site.css`, the suites from
   `apt/dists/*` in place of `{{SUITES}}`.
3. **The docs**, per channel: `--ref <channel>=<git ref>`, repeatable;
   default: every `release/<channel>` branch or tag of `origin` that exists,
   and `main=origin/main`. Each ref's tree is taken with `git archive`
   (no checkout), its documents converted by **`main`'s** `md_html.py`
   with `--anchors github --source <path> --page <each published path>
   --url GUIDELINES/shared/=<the guidelines at that ref's pin>
   --url =<GitHub at that ref's commit>`, and wrapped in `doc.html.in`.
   The pin is `git rev-parse <ref>:GUIDELINES/shared`; the version
   `git describe --tags --match 'v[0-9]*' <ref>`.
   The images under `docs/images/` and `docs/screenshots/` that a page names
   are copied beside it; the OpenAPI YAML is copied as it is.
4. **The index** of each channel, `/docs/<channel>/index.html`, from the
   fragment; **`/docs/channels.json`**: channel, version, commit, date;
   **`/docs/index.html`** sends a reader to the most stable channel built
   (stable, beta, alpha, nightly, devel, main), with a `<meta refresh>` and
   a link, Pages having no redirects.
5. **The check**: every relative link of every page written resolves to a
   file written (fragment links to a heading are checked against its ids);
   the build fails on the first channel listed by `--strict` (`main`) when a
   `docs/*.md` is missing from its fragment, and warns for the others.

**The selector** (`packaging/site/docs.js`, plain JavaScript, no
`innerHTML`): reads `/docs/channels.json`, fills a `<select>` in the header
with `textContent`, and on a change goes to the same page and anchor in the
other channel, or that channel's index where the page is not there
(a `HEAD` request first).

Python 3.10 (for `md_html.py`) fills the template and checks the links,
inline in the script: the page text is escaped once, in one place, rather
than through `sed`.

### The workflows

- **`apt-build`** keeps the key and `publish.sh`, which now builds `apt/`
  alone (its step 4 without the pages); it uploads `apt/` as an artifact
  `apt` instead of the Pages artifact. `apt-deploy` goes.
- **`site.yml`** (`workflow_call`, input `apt-artifact`, empty for none):
  job `build` checks out with full history and the submodule, fetches the
  `release/*` refs and `main`, downloads `apt` where it was built, else
  `--apt-index newest`, runs `build.sh` and uploads the Pages artifact;
  job `deploy` (`github-pages` environment, `pages`/`id-token` write,
  `concurrency: pages`, never cancelled) deploys it when `build` built one.
- **`release.yml`** calls it as `site`, after `apt-build`, for every kind,
  `if: !cancelled()` and `apt-build` succeeded or was skipped.
  A `devel` push to `main` and the nightly schedule then refresh the docs.
- **`ci.yml`**: a job `Site` on every pull request and push,
  `build.sh --no-apt --ref main=HEAD --strict main`, among the needs of
  "All tests passed".

## Implementation Plan

### Block 1: Guidelines

- `GUIDELINES/shared` → v4.55.0; `sync-docs.sh` regenerates `AGENTS.md` and
  `GUIDELINES/README.md`; whatever else the release asks of a project
  (its CHANGELOG's notes) is done or named.
- No CHANGELOG bullet: `GUIDELINES/**` is not shipped.

**Verification:** `md_html.py --version` ≥ 1.2.0; `check --commit-range`
clean; `go test ./...` untouched.

### Block 2: The list

- `docs/fragments/_documents.md`: the documents in four groups — Using it,
  The API, How it works, Contributing — a line each, links from the root.
- `README.md` "Documentation": the hand-written list → a PARTIAL of it,
  `sync-docs.sh` filling it.
- No CHANGELOG bullet: documents only.

**Verification:** `sync-docs.sh --check`, `reflow.sh`; every `docs/*.md`
and the three READMEs in the list.

### Block 3: The builder

- `packaging/site/build.sh`, `doc.html.in`, `docs.js`, `site.css`;
  `index.html` and `privacy.html` link `site.css`.
- `packaging/apt/publish.sh`: the pages' copy goes (the builder's).
- `packaging/apt/test.sh`: builds the site with `build.sh --apt` and again
  with `--apt-index` from the asset `publish.sh` wrote, and checks both;
  the docs of `HEAD` built and their links checked.
- `Makefile`: `make site` (`--no-apt`, `HEAD`, into `dist/site`).
- A CHANGELOG bullet: `packaging/**` is shipped.

**Verification:** `make site` builds, its link check passes, and fails on a
link broken on purpose and on a document left out of the list; `make
apt-test`'s site part (the container step needs Docker); `bash -n`, and
`shellcheck` where installed.

### Block 4: The workflows

- `.github/workflows/site.yml` new; `release.yml`: `apt-build` uploads
  `apt`, `apt-deploy` → `site`; `ci.yml`: the `Site` job, in `tests`.
- A CHANGELOG bullet: `.github/**` is shipped.

**Verification:** `actionlint` where installed, else YAML parsed; the
pull request's own `Site` job; the deploy itself only after the merge
(a push to `main`), named at the completion gate.

### Block 5: The documents

- `packaging/site/index.html`: a "Documentation" link to `/docs/`.
- `packaging/README.md` "The apt repository": the site's build, the
  builder, the index asset's use, no deploy without one.
- `GUIDELINES/project/CI.md`: the `Site` job, `site.yml`; its version.
- `README.md` "Versions" or `docs/` where the channels' docs are named.
- The CHANGELOG bullets of Blocks 3 and 4 read together.

**Verification:** `check --commit-range origin/main..HEAD`; `go test ./...`;
`make site` once more.

## Proposed commit message

One per block, in the project's form, each citing
`AP Site v1.0, Block <n>`; written at each block's commit.

## Change History

- v1.0 (2026-11-10): the plan, by `PLAN++ Site`; D1 to D6, D9 from the Site
  analysis, D2 as Channels D12-A, D13-A as recommended.

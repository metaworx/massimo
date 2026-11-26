# Builds the massimo binary with the version taken from the nearest git tag
# (semantic versioning: tag releases as vMAJOR.MINOR.PATCH) and the
# flavour it is released as (see internal/update). Only the version tags
# count: the rolling builds' tags (release/devel) are no version.

VERSION ?= $(shell git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --always --dirty 2>/dev/null || echo v0.0.0-dev)
# The flavour of a release tag's commit is the tag's pre-release label
# (alpha of v0.12.0-alpha.1), stable for a tag without one; any other
# build is devel, the flavour of the builds of main. release.yml passes
# FLAVOUR itself, and RELEASE, the GitHub release's tag, for a rolling
# build (release/devel).
FLAVOUR ?= $(shell t=$$(git describe --tags --exact-match --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null); \
	f=$$(printf '%s\n' "$$t" | sed -nE 's/^v[0-9]+\.[0-9]+\.[0-9]+-([a-z][a-z0-9]*)\.[0-9]+$$/\1/p'); \
	if [ -z "$$f" ] && printf '%s\n' "$$t" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then f=stable; fi; \
	printf '%s' "$${f:-devel}")
RELEASE  ?=
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.flavour=$(FLAVOUR)
PKG      = ./cmd/massimo
OUT      = dist
# Cross builds are static (no cgo) and do not record the build machine's paths.
CROSS    = CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"

# Every release binary, as `make build-release` builds them and
# internal/update.AssetName names them.
BINARIES = massimo-linux-amd64 massimo-linux-arm64 massimo-linux-armv7 massimo-linux-armv6 \
           massimo-darwin-amd64 massimo-darwin-arm64 massimo-windows-amd64.exe

# The Debian packages (packaging/README.md, "Debian package"), built from
# the binaries above with nfpm at a pinned version through `go run`;
# NFPM=nfpm uses an installed one. Their version is the tag without its
# v and with ~ for -, so that 0.7.0~alpha.1 sorts before 0.7.0 as dpkg
# compares versions. A build between tags carries what git describe adds
# behind a +, its - as . (0.7.0~alpha.1+3.gabc1234, with .dirty for a
# changed tree), which sorts after the tag it follows and before the
# next; with a ~ it would sort before its own tag, and apt would take a
# box on it back to that release. nfpm is given that version, not the
# tag; debTag (cmd/massimo/update_apt.go) reads it back, and
# TestDebVersion checks both through `make debver`. The file name writes
# the ~ and the + as dots (DEBFILE, massimo_0.7.0.alpha.1_amd64.deb), the
# name GitHub gives the asset, so that SHA256SUMS and the manifest name
# the files the release holds. armhf holds the armv6 binary, which runs
# on every 32-bit Raspberry Pi OS, the Pi 1 included.
NFPM_VERSION = v2.48.0
NFPM    ?= go run github.com/goreleaser/nfpm/v2/cmd/nfpm@$(NFPM_VERSION)
DEBVER   = $(shell echo '$(VERSION)' | sed -E 's/^v//; s/-([0-9]+)-g([0-9a-f]+)-dirty$$/+\1.g\2.dirty/; s/-([0-9]+)-g([0-9a-f]+)$$/+\1.g\2/; s/-dirty$$/+dirty/; s/-/~/g')
DEBFILE  = $(subst +,.,$(subst ~,.,$(DEBVER)))
PACKAGES = massimo_$(DEBFILE)_amd64.deb massimo_$(DEBFILE)_arm64.deb massimo_$(DEBFILE)_armhf.deb
# Reproducible packages: every file in them carries the commit's time.
export SOURCE_DATE_EPOCH ?= $(shell git log -1 --format=%ct 2>/dev/null || date +%s)
# The container the package test runs in (make deb-test).
DEB_IMAGE ?= debian:trixie-slim

.PHONY: all ui ui-check api-types credits build test vet race test-armv6 fuzz store-bench ui-test ui-screenshots ui-brand lan-test app-emulator-test actionlint windows linux linux-arm64 linux-armv7 linux-armv6 darwin darwin-amd64 darwin-arm64 cross cross-check deb-unit packages debver version deb-test deb-systemd-test apt-test site build-release release release-files clean clean-tests distclean release-alpha release-beta release-rc release-stable

# Development happens on main; a release is a tag on main, or on the
# maintenance branch maint/<x.y> of its line once that line is frozen
# (make freeze, below), whose label is
# the release's flavour (internal/update): -alpha.N is alpha, -beta.N beta,
# -rc.N rc, and a tag without one stable. Versions: below v1.0.0, and with
# any suffix, a release is a pre-release; below v1.0.0 a minor may break
# things. A new feature or breaking change starts the next minor (BUMP=minor:
# v0.2.0-alpha.1), a fix or iteration counts the suffix up (v0.2.0-alpha.2),
# and a fix of a stable release is the next patch (BUMP=patch).
#
# make release <flavour>, or make release FLAVOUR=<flavour>, cuts one: the
# flavour alpha, beta, rc or stable, from the command line alone, since
# FLAVOUR has the build's as its default; make release-alpha and its
# siblings are names for it. The next tag is packaging/next-version.sh's:
# the newest version any version tag has, or BUMP's next of it, with the
# flavour's next number, or the version itself for stable. It runs on main
# or on a maintenance branch maint/<x.y>, checked out, clean and equal to
# its origin copy. On maint/<x.y> the version tags of x.y and of the lines
# before it alone count, and the next tag must be one of x.y (BUMP=minor
# starts x.y where it has no tag yet); on main a version whose line has a
# maintenance branch on origin is refused, as that line is released there.
# The devel and nightly builds stay main's (release.yml). It fetches that
# branch and the version tags alone: CI moves the rolling tags (release/devel,
# release/nightly) in place, and fetch --tags refuses to move a checkout's
# older copy of one ("would clobber existing tag"); a refspec allows one *
# and no class, so the fetch takes refs/tags/v*, and every match after it
# is strict. First tools/store-fixture/make.sh dumps the store the release
# writes, built from HEAD under the coming tag's name and flavour (the
# release commit changes only the changelog), into
# internal/store/testdata/releases/<tag>.sql, committed on the branch as
# "test(store): The store <tag> writes", so that the tag holds its own dump
# and every later binary's TestMigratesReleasedStores migrates it; the
# release CI refuses a tag without it (docs/store.md, "Migrations"). Then
# packaging/app-version.sh writes app/version.json, the Android app's
# version and versionCode, which counts the release commit to come (AP
# MobileApp, D21-A); it is asked first, so that a version the code cannot
# hold stops the release before anything is written, and checked against
# the tag before the push. Then changelog.sh cuts CHANGELOG.md's
# [Unreleased] into the release's section, with the file as its manifest
# (GUIDELINES/config.ini), makes the chore(release) commit on the branch and
# the signed tag at it with that section as its message; the branch and the
# tag are then pushed atomically, all or nothing, and CI tests the tag and
# releases it (ci.yml calling release.yml), which then fast-forwards the
# branch release/<flavour> to it. It refuses when the branch's head is
# released already, when changelog.sh check finds something and when
# [Unreleased] holds no bullet. DRY=1 runs every step as a dry run: the
# dump and its commit are named, changelog.sh says what it would cut and
# tag, and the push is printed, not made. ASSISTED_BY=agent:model gives the dump's commit and
# the release commit an agent's Assisted-by
# trailer (COMMIT.md 3.3); the owner's own release needs none.
# make release was the build until 2026-11-12 (AP Channels, D42-A): with no
# flavour, or with the build's VERSION or RELEASE, it stops and names
# make build-release, so that a "make release" from before publishes
# nothing.
CHANGELOG_SH = GUIDELINES/shared/tools/changelog.sh
CUT_FLAVOURS = alpha beta rc stable
ifeq ($(firstword $(MAKECMDGOALS)),release)
CUT_WORDS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
ifneq ($(filter $(CUT_FLAVOURS),$(CUT_WORDS)),)
# make release beta: the word is a goal of its own, which does nothing;
# release reads it.
$(filter $(CUT_FLAVOURS),$(CUT_WORDS)):
	@:
endif
endif
release:
	@if [ -n "$(filter command line,$(origin VERSION) $(origin RELEASE))" ]; then \
		echo "release: VERSION and RELEASE are the build's; the build of a release is make build-release" >&2; exit 2; fi; \
	words='$(CUT_WORDS)'; flag='$(if $(filter command line,$(origin FLAVOUR)),$(FLAVOUR))'; \
	set -- $$words; \
	if [ $$# -gt 1 ]; then echo "release: one flavour, not $$words" >&2; exit 2; fi; \
	word=$${1:-}; \
	if [ -n "$$word" ] && [ -n "$$flag" ] && [ "$$word" != "$$flag" ]; then \
		echo "release: $$word or FLAVOUR=$$flag, not both" >&2; exit 2; fi; \
	flavour=$${word:-$$flag}; \
	case "$$flavour" in \
		alpha | beta | rc | stable) ;; \
		"") echo "release: make release <alpha|beta|rc|stable> cuts a release; the build of a release is make build-release" >&2; exit 2 ;; \
		*) echo "release: $$flavour is not a flavour a release is cut as: alpha, beta, rc or stable" >&2; exit 2 ;; \
	esac; \
	branch=$$(git symbolic-ref --quiet --short HEAD) || branch=""; line=""; \
	case "$$branch" in \
		main) ;; \
		maint/*) line=$${branch#maint/}; \
			printf '%s\n' "$$line" | grep -Eqx '(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)' || { \
				echo "release: $$branch is no maintenance branch, which is maint/<x.y>, two numbers" >&2; exit 1; } ;; \
		*) echo "release: check out main or a maintenance branch maint/<x.y> first; the release commit is made on it" >&2; exit 1 ;; \
	esac; \
	git fetch --quiet origin "+refs/heads/$$branch:refs/remotes/origin/$$branch" 'refs/tags/v*:refs/tags/v*' || { \
		echo "release: git fetch origin $$branch 'refs/tags/v*:refs/tags/v*' failed; run it to see why" >&2; exit 1; }; \
	sha=$$(git rev-parse "refs/remotes/origin/$$branch") && \
	if [ -n "$$(git status --porcelain --untracked-files=no)" ]; then \
		echo "release: $$branch has uncommitted changes" >&2; exit 1; fi; \
	if [ "$$(git rev-parse HEAD)" != "$$sha" ]; then \
		echo "release: $$branch is not origin/$$branch ($$sha); pull or push first" >&2; exit 1; fi; \
	if done=$$(git describe --tags --exact-match --match 'v[0-9]*.[0-9]*.[0-9]*' $$sha 2>/dev/null); then \
		echo "release: $$branch ($$sha) is already released as $$done" >&2; exit 1; fi; \
	tags=$$(git tag -l); \
	if [ -n "$$line" ]; then \
		tags=$$(printf '%s\n' "$$tags" | awk -F. -v x="$${line%.*}" -v y="$${line#*.}" \
			'/^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z][a-z0-9]*\.[0-9]+)?$$/ { a = substr($$1, 2) + 0; if (a < x || (a == x && $$2 + 0 <= y)) print }'); fi; \
	next=$$(printf '%s\n' "$$tags" | packaging/next-version.sh $$flavour $(BUMP)) || exit 1; \
	nl=$$(printf '%s\n' "$$next" | sed -E 's/^v([0-9]+\.[0-9]+)\..*/\1/'); \
	if [ -n "$$line" ] && [ "$$nl" != "$$line" ]; then \
		echo "release: $$branch releases $$line alone, not $$next; BUMP=minor starts a line without a tag, the one after the newest" >&2; exit 1; fi; \
	if [ -z "$$line" ] && git ls-remote --exit-code --heads origin "refs/heads/maint/$$nl" >/dev/null 2>&1; then \
		echo "release: $$nl is frozen on maint/$$nl, where its releases are cut; main's next is BUMP=minor's" >&2; exit 1; fi; \
	appcode=$$(packaging/app-version.sh release $$next 2) || { \
		echo "release: the Android app's versionCode cannot hold $$next; nothing written" >&2; exit 1; }; \
	cut="cut $${next#v} --commit --tag create$(if $(ASSISTED_BY), --assisted-by $(ASSISTED_BY))"; \
	$(CHANGELOG_SH) -q check || { \
		echo "release: changelog.sh check finds something; fix it on $$branch first" >&2; exit 1; }; \
	dump=internal/store/testdata/releases/$$next.sql; \
	if [ -n "$(DRY)" ]; then \
		echo "release: would run: tools/store-fixture/make.sh --ref HEAD --flavour $$flavour $$next, and commit $$dump"; \
		echo "release: would write app/version.json: version $${next#v}, versionCode $$appcode"; \
		$(CHANGELOG_SH) $$cut || exit 1; \
		echo "release: would run: git push --atomic origin refs/heads/$$branch refs/tags/$$next"; \
		exit 0; fi; \
	$(CHANGELOG_SH) -q $$cut || { \
		echo "release: changelog.sh refuses to cut $$next; nothing written" >&2; exit 1; }; \
	tools/store-fixture/make.sh --ref HEAD --flavour $$flavour $$next || { \
		echo "release: the store's dump for $$next failed; nothing committed or pushed" >&2; exit 1; }; \
	git add $$dump && timeout 60 git commit -q -m "test(store): The store $$next writes" \
		-m "Made by make release before the release commit: the store massimo $$next writes against the fake Cube, which TestMigratesReleasedStores migrates in every later binary (docs/store.md, Migrations)."$(if $(ASSISTED_BY), -m "Assisted-by: $(ASSISTED_BY)") || { \
		echo "release: the dump's commit failed; nothing pushed; to start over: git reset --keep $$sha" >&2; exit 1; }; \
	dumped=$$(git rev-parse HEAD); \
	packaging/app-version.sh write $$next 1 >/dev/null || { \
		echo "release: app/version.json not written; nothing pushed; to start over: git reset --keep $$sha" >&2; exit 1; }; \
	$(CHANGELOG_SH) $$cut --force || { \
		echo "release: the cut stopped half-way; nothing pushed, see git status and git log -2; to start over: git tag -d $$next; git reset --keep $$sha; git checkout -- app/version.json" >&2; exit 1; }; \
	rel=$$(git rev-parse HEAD) && \
	if [ "$$(git rev-parse $$dumped^)" != "$$sha" ] || [ "$$(git rev-parse $$rel^)" != "$$dumped" ] || [ "$$(git rev-parse "refs/tags/$$next^{commit}" 2>/dev/null)" != "$$rel" ]; then \
		echo "release: the dump's commit, the release commit or $$next is not where it should be; nothing pushed" >&2; exit 1; fi; \
	packaging/app-version.sh check $$next || { \
		echo "release: app/version.json is not $$next's; nothing pushed; to start over: git tag -d $$next; git reset --keep $$sha" >&2; exit 1; }; \
	git push --atomic origin "refs/heads/$$branch" "refs/tags/$$next" || { \
		echo "release: the push failed and published nothing; to start over: git tag -d $$next && git reset --keep $$sha" >&2; exit 1; }; \
	echo "tagged $$next ($$rel) on $$branch; CI tests and releases it, then moves release/$$flavour: https://github.com/metaworx/massimo/actions"; \
	if [ -n "$$line" ]; then echo "release: now merge $$branch upward into main, a merge: commit pushed to main"; fi

# make release-alpha, make release-beta, make release-rc and
# make release-stable: make release with that flavour.
release-alpha release-beta release-rc release-stable:
	@f='$(@:release-%=%)'; flag='$(if $(filter command line,$(origin FLAVOUR)),$(FLAVOUR))'; \
	if [ -n "$$flag" ] && [ "$$flag" != "$$f" ]; then echo "release: $$f or FLAVOUR=$$flag, not both" >&2; exit 2; fi; \
	$(MAKE) --no-print-directory release FLAVOUR=$$f

all: test build

# The web UI's parts (docs/ui.md, "How the page is built"): the TypeScript
# under internal/server/ui/src/ bundled by esbuild, through its Go API at
# the version tools/uibundle/go.mod pins, a module of its own, into
# internal/server/ui/parts.js, which the server embeds and git does not
# keep (AP UiParts, D172-A). Every target that compiles Go needs it, and
# names it; without it the go:embed line of internal/server/ui.go stops
# the build with its name. go -C runs the bundler from its own module,
# the same way on every platform; it finds the repository above it, and
# leaves the bundle untouched when it would not change. It runs on the
# build machine, whatever GOOS and GOARCH a cross build is given.
ui:
	GOOS= GOARCH= GOARM= go -C tools/uibundle run .

# The parts' types (tsc --noEmit over internal/server/ui/src/, its
# tsconfig.json), then their hooks (biome lint, the two rules of
# tools/ui-check/biome.jsonc), with the TypeScript and the Biome
# tools/ui-check/package-lock.json locks, installed by npm without
# running a package's scripts once per lock file for every checkout of
# the project, in the main checkout's GUIDELINES/temp/ui-check/, by
# tools/ui-check/install.sh, which prints where (AP CleanRuns, D1-A;
# tools/ui-check/README.md). Needs Node.js and npm; CI's job UI types
# runs it (GUIDELINES/project/CI.md, section 2).
ui-check:
	bin=$$(tools/ui-check/install.sh) && \
		"$$bin/tsc" --noEmit -p internal/server/ui/src && \
		"$$bin/biome" lint --config-path=tools/ui-check --no-errors-on-unmatched internal/server/ui/src

# The API's shapes as TypeScript for the parts' type check: tools/apitypes
# writes internal/server/ui/src/api.d.ts, one declaration per schema of
# docs/api-v1-openapi.yaml, run again after a change to a schema. git keeps
# the file; go test (tools/apitypes) fails while it is not current, and
# go run ./tools/apitypes -check says so without writing.
api-types:
	GOOS= GOARCH= GOARM= go run ./tools/apitypes

# The credits and licences of everything the binary includes, which
# System info shows: tools/credits writes internal/credits/credits.json
# from the modules go list finds for every platform of cross, Go's own
# LICENSE, third_party/ and the web UI's vendor/, run again after a
# module is added or removed, a licence's text changed, or Go's go line
# or a vendored file changed; not for a module's version alone, which
# the file leaves out and the binary shows as it was built with. git
# keeps the file and the binary embeds it; go test (internal/credits,
# tools/credits) fails while it is not current, and go run
# ./tools/credits -check says so without writing. go list needs the
# bundle (ui), which the server embeds. massimo credits prints the list
# too.
credits: ui
	GOOS= GOARCH= GOARM= go run ./tools/credits

build: ui
	go build -ldflags "$(LDFLAGS)" -o massimo $(PKG)

test: ui
	go test ./...

vet: ui
	go vet ./...

# The race detector on the packages whose goroutines meet: the server, the
# store, the Cube client, the proxy, the server's wiring, its
# announcement, its notification rules, the command queue, the Cube's
# writes of more than one step and the certificates (the contract's test
# gate). The detector slows the tests down, so their waits are twice as
# long (MASSIMO_TEST_SCALE, docs/testing.md). -count=1 runs them again
# where the cache holds a pass; -timeout 30m, three times Go's default,
# since internal/server took up to 988 s under load (AP
# TestInfrastructure, D14-A). TESTFLAGS adds flags to this target and to
# test-armv6, as CI's -json, which packaging/go-test-report.sh reads.
TESTFLAGS ?=
RACEPKGS = ./internal/server ./internal/store ./internal/cubelan ./internal/proxy ./internal/serve ./internal/announce ./internal/notify ./internal/engine ./internal/cubeio ./internal/certs

# -race turns on checkptr too, which checks each conversion of an
# unsafe.Pointer to a typed pointer, and its arithmetic, as the code runs; in modernc.org/sqlite, SQLite's C translated to Go,
# that was half the time of opening a fresh store. RACEGCFLAGS turns it
# off for the packages under modernc.org alone: the race detector stays
# on in every package, and checkptr in the module's own and every other
# dependency (AP TestSpeed, D6-A; docs/testing.md, "The race detector").
# Go gives a package the last -gcflags whose pattern matches it, not
# their sum: a -gcflags=all=... in TESTFLAGS replaces this one there,
# and SQLite is checked again, as with RACEGCFLAGS= alone, which passes
# no -gcflags at all. A -gcflags in GOFLAGS comes before this one.
RACEGCFLAGS ?= modernc.org/...=-d=checkptr=0

race: ui
	MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=2 go test -race -count=1 -timeout 30m $(if $(RACEGCFLAGS),'-gcflags=$(RACEGCFLAGS)') $(TESTFLAGS) $(RACEPKGS)

# Every package's tests on 32-bit ARM, the armv6 binary that the armhf
# package ships, run by QEMU as the Raspberry Pi 1's core (ARM1176) with
# -short: qemu-arm from the system (Debian and Ubuntu: qemu-user), no
# binfmt needed. CI's job armv6 runs it (GUIDELINES/project/CI.md,
# section 2). QEMU runs the tests several times slower than the host, so
# their waits are four times as long (MASSIMO_TEST_SCALE,
# docs/testing.md), and a package has 30 minutes: internal/server's tests
# add up to about 21 with four CPUs under load, past Go's default ten (AP
# TestInfrastructure, D14-A). -count=1 as in make race, which neither
# takes a result from the cache nor leaves one: under -exec go test
# caches none anyway, and CI's job restores a cache (AP TestSpeed, D10-A).
QEMU_ARM ?= qemu-arm -cpu arm1176

test-armv6: ui
	GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 MASSIMO_ENV_FILE=/dev/null MASSIMO_TEST_SCALE=4 go test -short -count=1 -timeout 30m $(TESTFLAGS) -exec '$(QEMU_ARM)' ./...

# The fuzz targets (Fuzz* in the packages that parse untrusted bytes:
# the Cube's and LAN clients' lines, uploaded backups, stored hashes),
# each for FUZZTIME, one after the other, by tools/fuzz/run.sh; go test
# -fuzz takes one target at a time. A crash leaves its input under the
# package's testdata/fuzz/, which go test then replays as a seed. Go
# 1.26's own -fuzztime deadline, reported as a failure, passes
# (golang/go#75804, GUIDELINES/project/CI.md, section 3).
FUZZTIME ?= 10s
FUZZPKGS = ./internal/protocol/cube ./internal/backup ./internal/store

fuzz: ui
	FUZZTIME=$(FUZZTIME) tools/fuzz/run.sh $(FUZZPKGS)

# Web UI smoke test with Playwright against the fake Cube; skipped when
# Playwright for Python is not installed (tools/ui-test/README.md).
ui-test: ui
	tools/ui-test/run.sh

# The web UI's screenshot set into docs/screenshots/, from the same
# start-up as ui-test; retaken at every change of the UI.
ui-screenshots: ui
	tools/ui-test/run.sh screenshots

# The web UI's brand files into internal/server/ui/brand/, and the Android
# app's icon into app/android/app/src/main/res-brand/: the ui and android
# scopes of tools/brand/make_brand.py in the chosen flavour
# (tools/brand/README.md); its Python environment is made the first time,
# which needs the network.
BRAND_VENV = tools/brand/.venv
ui-brand:
	@test -x $(BRAND_VENV)/bin/python || { python3 -m venv $(BRAND_VENV) && $(BRAND_VENV)/bin/pip install -q -r tools/brand/requirements.txt; }
	$(BRAND_VENV)/bin/python tools/brand/make_brand.py --flavour grayorange --i dot --scope ui --out internal/server/ui/brand --yes
	$(BRAND_VENV)/bin/python tools/brand/make_brand.py --flavour grayorange --scope android --out app/android/app/src/main/res-brand --yes

# Discovery, serve's mDNS announcement and massimo discover on a LAN in
# Docker, against avahi (tools/lan-test/README.md); needs Docker with
# Compose v2.
lan-test: ui
	tools/lan-test/run.sh

# The Android app's flows on an emulator (tools/app-emulator/README.md):
# its instrumented tests against two servers on the host, built from this
# tree with the fake Cube, the fingerprint simulated. Needs the Android
# SDK with the pinned emulator and system image (tools/app-emulator/
# sdk.sh) and KVM, and says which is missing before it starts anything;
# CI's job Android emulator runs its two steps, run.sh in its step's shell's
# place (GUIDELINES/project/CI.md, section 2).
app-emulator-test: ui
	tools/app-emulator/run.sh

# The workflows under .github/workflows/ by actionlint, their run:
# scripts by shellcheck where it is on the PATH. The tool is pinned in a
# module of its own, tools/actionlint/go.mod, kept out of massimo's
# go.mod; go tool builds it once into the build cache (tools/actionlint/
# README.md). CI's job actionlint runs it (GUIDELINES/project/CI.md,
# section 2).
actionlint:
	go tool -modfile=tools/actionlint/go.mod actionlint

# Cross builds; each lands in $(OUT)/, or nowhere for cross-check:
# $(call crossout,<file>) is $(OUT)/<file>, or /dev/null where
# CROSS_DISCARD is set, and go build then compiles and links the binary
# and writes nothing (AP CleanRuns, D2-A).
CROSS_DISCARD ?=
crossout = $(if $(CROSS_DISCARD),/dev/null,$(OUT)/$(1))

windows: ui
	GOOS=windows GOARCH=amd64 $(CROSS) -o $(call crossout,massimo-windows-amd64.exe) $(PKG)

linux: ui
	GOOS=linux GOARCH=amd64 $(CROSS) -o $(call crossout,massimo-linux-amd64) $(PKG)

linux-arm64: ui
	GOOS=linux GOARCH=arm64 $(CROSS) -o $(call crossout,massimo-linux-arm64) $(PKG)

# Raspberry Pi 2/3/4 with a 32-bit OS (LoxBerry on Raspberry Pi OS 32-bit).
linux-armv7: ui
	GOOS=linux GOARCH=arm GOARM=7 $(CROSS) -o $(call crossout,massimo-linux-armv7) $(PKG)

# Raspberry Pi 1 (A, B, B+) and Zero / Zero W: ARMv6 with a hardware FPU.
linux-armv6: ui
	GOOS=linux GOARCH=arm GOARM=6 $(CROSS) -o $(call crossout,massimo-linux-armv6) $(PKG)

# macOS on Intel and on Apple silicon.
darwin-amd64: ui
	GOOS=darwin GOARCH=amd64 $(CROSS) -o $(call crossout,massimo-darwin-amd64) $(PKG)

darwin-arm64: ui
	GOOS=darwin GOARCH=arm64 $(CROSS) -o $(call crossout,massimo-darwin-arm64) $(PKG)

darwin: darwin-amd64 darwin-arm64

cross: linux linux-arm64 linux-armv7 linux-armv6 darwin windows

# Every platform of cross compiled and linked, and no binary written: the
# test gate's check that every platform builds (GUIDELINES/project/
# _CONTRACT.md), which leaves no $(OUT)/ behind; CI's job Vet and test
# runs it (AP CleanRuns, D2-A).
cross-check:
	$(MAKE) --no-print-directory cross CROSS_DISCARD=1

# The store's benchmarks (internal/store/events_bench_test.go) as test
# binaries for the Raspberry Pis, the Pi 5's arm64 and the Pi 1's armv6,
# to run on a copy of a box's store (docs/store.md section 5, "Measuring
# on a box").
store-bench: ui
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -trimpath -o $(OUT)/store-bench-linux-arm64 ./internal/store
	GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go test -c -trimpath -o $(OUT)/store-bench-linux-armv6 ./internal/store

# The package's unit: the embedded one with the package's binary path and
# without the update helper's flag (packaging/deb/unit.sh).
deb-unit:
	packaging/deb/unit.sh packaging/systemd/massimo.service $(OUT)/deb/massimo.service

# One Debian package per architecture, from the binaries of `make cross`.
packages: linux linux-arm64 linux-armv6 deb-unit
	VERSION=$(DEBVER) NFPM_ARCH=amd64 BINARY=$(OUT)/massimo-linux-amd64 $(NFPM) package -f packaging/nfpm.yaml -p deb -t $(OUT)/massimo_$(DEBFILE)_amd64.deb
	VERSION=$(DEBVER) NFPM_ARCH=arm64 BINARY=$(OUT)/massimo-linux-arm64 $(NFPM) package -f packaging/nfpm.yaml -p deb -t $(OUT)/massimo_$(DEBFILE)_arm64.deb
	VERSION=$(DEBVER) NFPM_ARCH=arm6 BINARY=$(OUT)/massimo-linux-armv6 $(NFPM) package -f packaging/nfpm.yaml -p deb -t $(OUT)/massimo_$(DEBFILE)_armhf.deb

# The amd64 package installed, checked and purged in a Debian container
# (packaging/deb/test.sh); DEB_IMAGE picks the release.
# The package version of VERSION, which TestDebVersion reads.
debver:
	@echo '$(DEBVER)'

# VERSION and FLAVOUR of the commit checked out, which TestMakeVersionTags
# reads in a repository of its own (make -f <this Makefile> version).
version:
	@echo '$(VERSION) $(FLAVOUR)'

deb-test:
	docker run --rm -v "$(CURDIR)/$(OUT):/dist:ro" -v "$(CURDIR)/packaging/deb/test.sh:/test.sh:ro" $(DEB_IMAGE) bash /test.sh

# The amd64 package under a real systemd, each case in a container of its
# own with systemd as its first process (packaging/deb/systemd-test.sh):
# the start, the restart, the upgrade from v0.12.0-alpha.1 with the
# service running; DEB_IMAGE picks the release, OLD_DEB a local copy of
# v0.12.0-alpha.1's package.
deb-systemd-test:
	packaging/deb/systemd-test.sh $(DEB_IMAGE)

# The apt repository built from the packages with a throwaway key and
# installed from in a Debian container (packaging/apt/test.sh).
apt-test:
	packaging/apt/test.sh $(DEB_IMAGE)

# The site's pages and the docs of the commit checked out, without the apt
# repository, into $(OUT)/site, every link checked (packaging/site/build.sh;
# packaging/README.md, "The site"). Serve it to read it, the links are
# root-relative: python3 -m http.server -d $(OUT)/site
site:
	rm -rf $(OUT)/site
	packaging/site/build.sh --out $(OUT)/site --no-apt --ref main=HEAD

# build-release builds every binary and package and the files a GitHub release
# carries besides them: the build record (packaging/build-record.sh), which
# holds the build's log, SHA256SUMS for people, and the manifest the
# release workflow signs and `massimo update` verifies; SHA256SUMS and the
# manifest list the record, so the signature covers it. Run by the
# workflow with VERSION and FLAVOUR set, and RELEASE for a rolling build;
# locally it shows what a release would hold. The build runs as a sub-make
# through tee, under bash with pipefail, so that its failure still fails
# the release; the command line's VERSION, FLAVOUR and RELEASE reach it.
RECORD = massimo-build.txt
# The binaries and packages build side by side; -Otarget keeps each
# target's output together, so that the record's log reads target by
# target. A make without -O (macOS's 3.81) runs it with RELEASE_JOBS=.
RELEASE_JOBS ?= -j4 -Otarget
build-release: SHELL := bash
build-release: .SHELLFLAGS := -eo pipefail -c
build-release: clean
	mkdir -p $(OUT)
	started=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	$(MAKE) --no-print-directory $(RELEASE_JOBS) release-files VERSION='$(VERSION)' FLAVOUR='$(FLAVOUR)' RELEASE='$(RELEASE)' 2>&1 | tee $(OUT)/build.log; \
	VERSION='$(VERSION)' FLAVOUR='$(FLAVOUR)' RELEASE='$(RELEASE)' LDFLAGS='$(LDFLAGS)' BUILD_STARTED="$$started" \
		packaging/build-record.sh $(OUT)/build.log github.com/goreleaser/nfpm/v2/cmd/nfpm@$(NFPM_VERSION) \
		$(addprefix $(OUT)/,$(BINARIES)) > $(OUT)/$(RECORD)
	rm -f $(OUT)/build.log
	cd $(OUT) && sha256sum $(BINARIES) $(PACKAGES) $(RECORD) > SHA256SUMS
	go run ./internal/update/mkmanifest -tag $(VERSION) -commit $$(git rev-parse HEAD) \
		-flavour $(FLAVOUR) -release '$(RELEASE)' -dir $(OUT) $(BINARIES) $(PACKAGES) $(RECORD) > $(OUT)/massimo-release.json

# The binaries and packages of a release, which `make build-release` logs.
release-files: cross packages

clean:
	rm -rf massimo $(OUT) internal/server/ui/parts.js

# What the test suites leave in the checkout: the UI suite's results/
# (the screenshots, videos and traces of failed tests and the servers'
# logs) and pytest's cache, the emulator suite's results/, and Python's
# __pycache__ under tools/, outside a virtual environment. make clean
# leaves them, since build-release runs it first (AP CleanRuns, D5-A).
clean-tests:
	rm -rf tools/ui-test/results tools/ui-test/.pytest_cache tools/app-emulator/results
	find tools -name .venv -prune -o -name node_modules -prune -o -type d -name __pycache__ -prune -exec rm -rf {} +

# clean and clean-tests, and what make installed into the checkout: the
# type check's tools/ui-check/node_modules, where npm installed it there
# (tools/ui-check/README.md), the brand generator's Python environment
# and the app's Gradle builds. Never .env, a store, tools/ui-test/.venv,
# which is made by hand (tools/ui-test/README.md), or what every checkout
# shares: Go's caches, npm's, the type check's install in the main
# checkout's GUIDELINES/temp/ui-check/ and Docker's images (AP CleanRuns,
# D5-A).
distclean: clean clean-tests
	rm -rf tools/ui-check/node_modules $(BRAND_VENV) app/android/app/build app/android/build app/android/.gradle app/android/.kotlin

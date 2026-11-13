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

.PHONY: all build test vet race test-armv6 fuzz store-bench ui-test ui-screenshots ui-brand lan-test actionlint windows linux linux-arm64 linux-armv7 linux-armv6 darwin darwin-amd64 darwin-arm64 cross deb-unit packages debver version deb-test deb-systemd-test apt-test site build-release release release-files clean release-alpha release-beta release-rc release-stable

# Development happens on main; a release is a tag on main, whose label is
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
# flavour's next number, or the version itself for stable. It runs on main,
# checked out, clean and equal to origin/main. It fetches main and the
# version tags alone: CI moves the rolling tags (release/devel,
# release/nightly) in place, and fetch --tags refuses to move a checkout's
# older copy of one ("would clobber existing tag"); a refspec allows one *
# and no class, so the fetch takes refs/tags/v*, and every match after it
# is strict. First tools/store-fixture/make.sh dumps the store the release
# writes, built from HEAD under the coming tag's name and flavour (the
# release commit changes only the changelog), into
# internal/store/testdata/releases/<tag>.sql, committed on main as
# "test(store): The store <tag> writes", so that the tag holds its own dump
# and every later binary's TestMigratesReleasedStores migrates it; the
# release CI refuses a tag without it (docs/store.md, "Migrations"). Then
# changelog.sh cuts CHANGELOG.md's [Unreleased] into the
# release's section, makes the chore(release) commit on main and the signed
# tag at it with that section as its message; main and the tag are then
# pushed atomically, all or nothing, and CI tests the tag and releases it
# (ci.yml calling release.yml), which then fast-forwards the branch
# release/<flavour> to it. It refuses when main's head is released already,
# when changelog.sh check finds something and when [Unreleased] holds no
# bullet. DRY=1 runs every step as a dry run: the dump and its commit are
# named, changelog.sh says what it would cut and tag, and the push is
# printed, not made. ASSISTED_BY=agent:model gives the dump's commit and
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
	git fetch --quiet origin main 'refs/tags/v*:refs/tags/v*' || { \
		echo "release: git fetch origin main 'refs/tags/v*:refs/tags/v*' failed; run it to see why" >&2; exit 1; }; \
	sha=$$(git rev-parse origin/main) && \
	if [ "$$(git symbolic-ref --quiet --short HEAD)" != main ]; then \
		echo "release: check out main first; the release commit is made on it" >&2; exit 1; fi; \
	if [ -n "$$(git status --porcelain --untracked-files=no)" ]; then \
		echo "release: main has uncommitted changes" >&2; exit 1; fi; \
	if [ "$$(git rev-parse HEAD)" != "$$sha" ]; then \
		echo "release: main is not origin/main ($$sha); pull or push first" >&2; exit 1; fi; \
	if done=$$(git describe --tags --exact-match --match 'v[0-9]*.[0-9]*.[0-9]*' $$sha 2>/dev/null); then \
		echo "release: main ($$sha) is already released as $$done" >&2; exit 1; fi; \
	next=$$(git tag -l | packaging/next-version.sh $$flavour $(BUMP)) || exit 1; \
	cut="cut $${next#v} --commit --tag create$(if $(ASSISTED_BY), --assisted-by $(ASSISTED_BY))"; \
	$(CHANGELOG_SH) -q check || { \
		echo "release: changelog.sh check finds something; fix it on main first" >&2; exit 1; }; \
	dump=internal/store/testdata/releases/$$next.sql; \
	if [ -n "$(DRY)" ]; then \
		echo "release: would run: tools/store-fixture/make.sh --ref HEAD --flavour $$flavour $$next, and commit $$dump"; \
		$(CHANGELOG_SH) $$cut || exit 1; \
		echo "release: would run: git push --atomic origin main refs/tags/$$next"; \
		exit 0; fi; \
	$(CHANGELOG_SH) -q $$cut || { \
		echo "release: changelog.sh refuses to cut $$next; nothing written" >&2; exit 1; }; \
	tools/store-fixture/make.sh --ref HEAD --flavour $$flavour $$next || { \
		echo "release: the store's dump for $$next failed; nothing committed or pushed" >&2; exit 1; }; \
	git add $$dump && timeout 60 git commit -q -m "test(store): The store $$next writes" \
		-m "Made by make release before the release commit: the store massimo $$next writes against the fake Cube, which TestMigratesReleasedStores migrates in every later binary (docs/store.md, Migrations)."$(if $(ASSISTED_BY), -m "Assisted-by: $(ASSISTED_BY)") || { \
		echo "release: the dump's commit failed; nothing pushed; to start over: git reset --keep $$sha" >&2; exit 1; }; \
	dumped=$$(git rev-parse HEAD); \
	$(CHANGELOG_SH) $$cut --force || { \
		echo "release: the cut stopped half-way; nothing pushed, see git status and git log -2; to start over: git tag -d $$next; git reset --keep $$sha" >&2; exit 1; }; \
	rel=$$(git rev-parse HEAD) && \
	if [ "$$(git rev-parse $$dumped^)" != "$$sha" ] || [ "$$(git rev-parse $$rel^)" != "$$dumped" ] || [ "$$(git rev-parse "refs/tags/$$next^{commit}" 2>/dev/null)" != "$$rel" ]; then \
		echo "release: the dump's commit, the release commit or $$next is not where it should be; nothing pushed" >&2; exit 1; fi; \
	git push --atomic origin main refs/tags/$$next || { \
		echo "release: the push failed and published nothing; to start over: git tag -d $$next && git reset --keep $$sha" >&2; exit 1; }; \
	echo "tagged $$next ($$rel); CI tests and releases it, then moves release/$$flavour: https://github.com/metaworx/massimo/actions"

# make release-alpha, make release-beta, make release-rc and
# make release-stable: make release with that flavour.
release-alpha release-beta release-rc release-stable:
	@f='$(@:release-%=%)'; flag='$(if $(filter command line,$(origin FLAVOUR)),$(FLAVOUR))'; \
	if [ -n "$$flag" ] && [ "$$flag" != "$$f" ]; then echo "release: $$f or FLAVOUR=$$flag, not both" >&2; exit 2; fi; \
	$(MAKE) --no-print-directory release FLAVOUR=$$f

all: test build

build:
	go build -ldflags "$(LDFLAGS)" -o massimo $(PKG)

test:
	go test ./...

vet:
	go vet ./...

# The race detector on the packages whose goroutines meet: the server, the
# store, the Cube client, the proxy, the server's wiring, its announcement
# and its notification rules (the contract's test gate).
RACEPKGS = ./internal/server ./internal/store ./internal/cubelan ./internal/proxy ./internal/serve ./internal/announce ./internal/notify

race:
	MASSIMO_ENV_FILE=/dev/null go test -race $(RACEPKGS)

# Every package's tests on 32-bit ARM, the armv6 binary that the armhf
# package ships, run by QEMU as the Raspberry Pi 1's core (ARM1176) with
# -short: qemu-arm from the system (Debian and Ubuntu: qemu-user), no
# binfmt needed. CI's job armv6 runs it (GUIDELINES/project/CI.md,
# section 2).
QEMU_ARM ?= qemu-arm -cpu arm1176

test-armv6:
	GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 MASSIMO_ENV_FILE=/dev/null go test -short -exec '$(QEMU_ARM)' ./...

# The fuzz targets (Fuzz* in the packages that parse untrusted bytes:
# the Cube's and LAN clients' lines, uploaded backups, stored hashes),
# each for FUZZTIME, one after the other, by tools/fuzz/run.sh; go test
# -fuzz takes one target at a time. A crash leaves its input under the
# package's testdata/fuzz/, which go test then replays as a seed. Go
# 1.26's own -fuzztime deadline, reported as a failure, passes
# (golang/go#75804, GUIDELINES/project/CI.md, section 3).
FUZZTIME ?= 10s
FUZZPKGS = ./internal/protocol/cube ./internal/backup ./internal/store

fuzz:
	FUZZTIME=$(FUZZTIME) tools/fuzz/run.sh $(FUZZPKGS)

# Web UI smoke test with Playwright against the fake Cube; skipped when
# Playwright for Python is not installed (tools/ui-test/README.md).
ui-test:
	tools/ui-test/run.sh

# The web UI's screenshot set into docs/screenshots/, from the same
# start-up as ui-test; retaken at every change of the UI.
ui-screenshots:
	tools/ui-test/run.sh screenshots

# The web UI's brand files into internal/server/ui/brand/: the ui scope of
# tools/brand/make_brand.py in the chosen flavour (tools/brand/README.md);
# its Python environment is made the first time, which needs the network.
BRAND_VENV = tools/brand/.venv
ui-brand:
	@test -x $(BRAND_VENV)/bin/python || { python3 -m venv $(BRAND_VENV) && $(BRAND_VENV)/bin/pip install -q -r tools/brand/requirements.txt; }
	$(BRAND_VENV)/bin/python tools/brand/make_brand.py --flavour grayorange --i dot --scope ui --out internal/server/ui/brand --yes

# Discovery, serve's mDNS announcement and massimo discover on a LAN in
# Docker, against avahi (tools/lan-test/README.md); needs Docker with
# Compose v2.
lan-test:
	tools/lan-test/run.sh

# The workflows under .github/workflows/ by actionlint, their run:
# scripts by shellcheck where it is on the PATH. The tool is pinned in a
# module of its own, tools/actionlint/go.mod, kept out of massimo's
# go.mod; go tool builds it once into the build cache (tools/actionlint/
# README.md). CI's job actionlint runs it (GUIDELINES/project/CI.md,
# section 2).
actionlint:
	go tool -modfile=tools/actionlint/go.mod actionlint

# Cross builds; each lands in $(OUT)/.
windows:
	GOOS=windows GOARCH=amd64 $(CROSS) -o $(OUT)/massimo-windows-amd64.exe $(PKG)

linux:
	GOOS=linux GOARCH=amd64 $(CROSS) -o $(OUT)/massimo-linux-amd64 $(PKG)

linux-arm64:
	GOOS=linux GOARCH=arm64 $(CROSS) -o $(OUT)/massimo-linux-arm64 $(PKG)

# Raspberry Pi 2/3/4 with a 32-bit OS (LoxBerry on Raspberry Pi OS 32-bit).
linux-armv7:
	GOOS=linux GOARCH=arm GOARM=7 $(CROSS) -o $(OUT)/massimo-linux-armv7 $(PKG)

# Raspberry Pi 1 (A, B, B+) and Zero / Zero W: ARMv6 with a hardware FPU.
linux-armv6:
	GOOS=linux GOARCH=arm GOARM=6 $(CROSS) -o $(OUT)/massimo-linux-armv6 $(PKG)

# macOS on Intel and on Apple silicon.
darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(CROSS) -o $(OUT)/massimo-darwin-amd64 $(PKG)

darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(CROSS) -o $(OUT)/massimo-darwin-arm64 $(PKG)

darwin: darwin-amd64 darwin-arm64

cross: linux linux-arm64 linux-armv7 linux-armv6 darwin windows

# The store's benchmarks (internal/store/events_bench_test.go) as test
# binaries for the Raspberry Pis, the Pi 5's arm64 and the Pi 1's armv6,
# to run on a copy of a box's store (docs/store.md section 5, "Measuring
# on a box").
store-bench:
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
	rm -rf massimo $(OUT)

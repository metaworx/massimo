# Builds the massimo binary with the version taken from the nearest git tag
# (semantic versioning: tag releases as vMAJOR.MINOR.PATCH) and the update
# channel taken from the current branch (see internal/update).

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v0.0.0-dev)
CHANNEL ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.channel=$(CHANNEL)
PKG      = ./cmd/massimo
OUT      = dist
# Cross builds are static (no cgo) and do not record the build machine's paths.
CROSS    = CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"

# Every release binary, as `make release` builds them and
# internal/update.AssetName names them.
BINARIES = massimo-linux-amd64 massimo-linux-arm64 massimo-linux-armv7 massimo-linux-armv6 \
           massimo-darwin-amd64 massimo-darwin-arm64 massimo-windows-amd64.exe

# The Debian packages (packaging/README.md, "Debian package"), built from
# the binaries above with nfpm at a pinned version through `go run`;
# NFPM=nfpm uses an installed one. Their version is the tag without its
# v and with ~ for -, so that 0.7.0~alpha.1 sorts before 0.7.0 as dpkg
# compares versions. armhf holds the armv6 binary, which runs on every
# 32-bit Raspberry Pi OS, the Pi 1 included.
NFPM_VERSION = v2.48.0
NFPM    ?= go run github.com/goreleaser/nfpm/v2/cmd/nfpm@$(NFPM_VERSION)
DEBVER   = $(shell echo '$(VERSION)' | sed 's/^v//; s/-/~/g')
PACKAGES = massimo_$(DEBVER)_amd64.deb massimo_$(DEBVER)_arm64.deb massimo_$(DEBVER)_armhf.deb
# Reproducible packages: every file in them carries the commit's time.
export SOURCE_DATE_EPOCH ?= $(shell git log -1 --format=%ct 2>/dev/null || date +%s)
# The container the package test runs in (make deb-test).
DEB_IMAGE ?= debian:trixie-slim

.PHONY: all build test vet ui-test ui-screenshots windows linux linux-arm64 linux-armv7 linux-armv6 darwin darwin-amd64 darwin-arm64 cross deb-unit packages deb-test apt-test release clean release-alpha

# Development happens on main; a release promotes main to a release
# branch, which is also the update channel boxes follow (internal/update).
# Versions: below v1.0.0, and with any suffix, a release is a pre-release;
# below v1.0.0 a minor may break things. A new feature or breaking change
# starts the next minor (make release-alpha BUMP=minor: v0.2.0-alpha.1),
# a fix or iteration counts the suffix up (v0.2.0-alpha.2); the suffix
# names the channel, so promoting to a beta branch would tag -beta.N.
# release-alpha runs on main, checked out, clean and equal to origin/main.
# changelog.sh cuts CHANGELOG.md's [Unreleased] into the release's section,
# makes the chore(release) commit on main and the signed tag at it with that
# section as its message; main, alpha fast-forwarded to that commit and the
# tag are then pushed atomically, all or nothing, and
# .github/workflows/release.yml builds, signs and publishes the release.
# It refuses when alpha has commits main lacks (alpha is only ever
# fast-forwarded), when main's head is released already, when changelog.sh
# check finds something and when [Unreleased] holds no bullet. DRY=1 runs
# every step as a dry run: changelog.sh says what it would cut and tag, and
# the push is printed, not made. ASSISTED_BY=agent:model gives the release
# commit an agent's Assisted-by trailer (COMMIT.md 3.3); the owner's own
# release needs none.
CHANGELOG_SH = GUIDELINES/shared/tools/changelog.sh
release-alpha:
	@git fetch --quiet --tags origin main alpha && \
	sha=$$(git rev-parse origin/main) && \
	if [ "$$(git symbolic-ref --quiet --short HEAD)" != main ]; then \
		echo "release-alpha: check out main first; the release commit is made on it" >&2; exit 1; fi; \
	if [ -n "$$(git status --porcelain --untracked-files=no)" ]; then \
		echo "release-alpha: main has uncommitted changes" >&2; exit 1; fi; \
	if [ "$$(git rev-parse HEAD)" != "$$sha" ]; then \
		echo "release-alpha: main is not origin/main ($$sha); pull or push first" >&2; exit 1; fi; \
	if ! git merge-base --is-ancestor origin/alpha $$sha; then \
		echo "release-alpha: alpha has commits that main lacks; bring them into main first" >&2; exit 1; fi; \
	if done=$$(git describe --tags --exact-match $$sha 2>/dev/null); then \
		echo "release-alpha: main ($$sha) is already released as $$done" >&2; exit 1; fi; \
	last=$$(git tag -l 'v0.*.*-alpha.*' | awk -F'[.-]' '{print $$2, $$NF, $$0}' | sort -k1,1n -k2,2n | tail -1); \
	minor=$${last%% *}; n=$$(echo "$$last" | cut -d' ' -f2); \
	case "$(BUMP)" in \
		minor) next=v0.$$((minor+1)).0-alpha.1 ;; \
		"") next=v0.$$minor.0-alpha.$$((n+1)) ;; \
		*) echo "release-alpha: BUMP is minor or empty" >&2; exit 1 ;; \
	esac; \
	cut="cut $${next#v} --commit --tag create$(if $(ASSISTED_BY), --assisted-by $(ASSISTED_BY))"; \
	$(CHANGELOG_SH) -q check || { \
		echo "release-alpha: changelog.sh check finds something; fix it on main first" >&2; exit 1; }; \
	if [ -n "$(DRY)" ]; then \
		$(CHANGELOG_SH) $$cut || exit 1; \
		echo "release-alpha: would run: git push --atomic origin main <the release commit>:refs/heads/alpha refs/tags/$$next"; \
		exit 0; fi; \
	$(CHANGELOG_SH) -q $$cut || { \
		echo "release-alpha: changelog.sh refuses to cut $$next; nothing written" >&2; exit 1; }; \
	$(CHANGELOG_SH) $$cut --force || { \
		echo "release-alpha: the cut stopped half-way; nothing pushed, see git status and git log -1" >&2; exit 1; }; \
	rel=$$(git rev-parse HEAD) && \
	if [ "$$(git rev-parse $$rel^)" != "$$sha" ] || [ "$$(git rev-parse "refs/tags/$$next^{commit}" 2>/dev/null)" != "$$rel" ]; then \
		echo "release-alpha: the release commit or $$next is not where it should be; nothing pushed" >&2; exit 1; fi; \
	git push --atomic origin main $$rel:refs/heads/alpha refs/tags/$$next || { \
		echo "release-alpha: the push failed and published nothing; to start over: git tag -d $$next && git reset --keep $$sha" >&2; exit 1; }; \
	echo "released $$next ($$rel) on alpha; follow it at https://github.com/metaworx/massimo/actions"

all: test build

build:
	go build -ldflags "$(LDFLAGS)" -o massimo $(PKG)

test:
	go test ./...

vet:
	go vet ./...

# Web UI smoke test with Playwright against the fake Cube; skipped when
# Playwright for Python is not installed (tools/ui-test/README.md).
ui-test:
	tools/ui-test/run.sh

# The web UI's screenshot set into docs/screenshots/, from the same
# start-up as ui-test; retaken at every change of the UI.
ui-screenshots:
	tools/ui-test/run.sh screenshots

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

# The package's unit: the embedded one with the package's binary path and
# without the update helper's flag (packaging/deb/unit.sh).
deb-unit:
	packaging/deb/unit.sh packaging/systemd/massimo.service $(OUT)/deb/massimo.service

# One Debian package per architecture, from the binaries of `make cross`.
packages: linux linux-arm64 linux-armv6 deb-unit
	VERSION=$(VERSION) NFPM_ARCH=amd64 BINARY=$(OUT)/massimo-linux-amd64 $(NFPM) package -f packaging/nfpm.yaml -p deb -t $(OUT)/massimo_$(DEBVER)_amd64.deb
	VERSION=$(VERSION) NFPM_ARCH=arm64 BINARY=$(OUT)/massimo-linux-arm64 $(NFPM) package -f packaging/nfpm.yaml -p deb -t $(OUT)/massimo_$(DEBVER)_arm64.deb
	VERSION=$(VERSION) NFPM_ARCH=arm6 BINARY=$(OUT)/massimo-linux-armv6 $(NFPM) package -f packaging/nfpm.yaml -p deb -t $(OUT)/massimo_$(DEBVER)_armhf.deb

# The amd64 package installed, checked and purged in a Debian container
# (packaging/deb/test.sh); DEB_IMAGE picks the release.
deb-test:
	docker run --rm -v "$(CURDIR)/$(OUT):/dist:ro" -v "$(CURDIR)/packaging/deb/test.sh:/test.sh:ro" $(DEB_IMAGE) bash /test.sh

# The apt repository built from the packages with a throwaway key and
# installed from in a Debian container (packaging/apt/test.sh).
apt-test:
	packaging/apt/test.sh $(DEB_IMAGE)

# release builds every binary and package and the files a GitHub release
# carries besides them: SHA256SUMS for people, and the manifest the
# release workflow signs and `massimo update` verifies. Run by the
# workflow with VERSION and CHANNEL set; locally it shows what a release
# would hold.
release: clean cross packages
	cd $(OUT) && sha256sum $(BINARIES) $(PACKAGES) > SHA256SUMS
	go run ./internal/update/mkmanifest -tag $(VERSION) -commit $$(git rev-parse HEAD) \
		-channel $(CHANNEL) -dir $(OUT) $(BINARIES) $(PACKAGES) > $(OUT)/massimo-release.json

clean:
	rm -rf massimo $(OUT)

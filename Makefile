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

.PHONY: all build test vet windows linux linux-arm64 linux-armv7 linux-armv6 darwin darwin-amd64 darwin-arm64 cross release clean release-alpha

# Development happens on main; a release promotes main to a release
# branch, which is also the update channel boxes follow (internal/update).
# release-alpha fast-forwards alpha on GitHub to main's head and tags it as
# the next v0.1.0-alpha.N with "channel: alpha" in the (signed) tag
# message, both pushed atomically; .github/workflows/release.yml then
# builds, signs and publishes the release. Push main first. It refuses when
# alpha has commits main lacks (alpha is only ever fast-forwarded) and when
# main's head is released already.
release-alpha:
	@git fetch --quiet --tags origin main alpha && \
	sha=$$(git rev-parse origin/main) && \
	if ! git merge-base --is-ancestor origin/alpha $$sha; then \
		echo "release-alpha: alpha has commits that main lacks; bring them into main first" >&2; exit 1; fi; \
	if done=$$(git describe --tags --exact-match $$sha 2>/dev/null); then \
		echo "release-alpha: main ($$sha) is already released as $$done" >&2; exit 1; fi; \
	last=$$(git tag -l 'v0.1.0-alpha.*' | sort -t. -k3 -n | tail -1); \
	n=$${last##*.}; next=v0.1.0-alpha.$$((n+1)); \
	git tag -a $$next -m "$$next" -m "channel: alpha" $$sha && \
	git push --atomic origin $$sha:refs/heads/alpha refs/tags/$$next && \
	echo "released $$next ($$sha) on alpha; follow it at https://github.com/metaworx/massimo/actions"

all: test build

build:
	go build -ldflags "$(LDFLAGS)" -o massimo $(PKG)

test:
	go test ./...

vet:
	go vet ./...

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

# release builds every binary and the files a GitHub release carries
# besides them: SHA256SUMS for people, and the manifest the release
# workflow signs and `massimo update` verifies. Run by the workflow with
# VERSION and CHANNEL set; locally it shows what a release would hold.
release: clean cross
	cd $(OUT) && sha256sum $(BINARIES) > SHA256SUMS
	go run ./internal/update/mkmanifest -tag $(VERSION) -commit $$(git rev-parse HEAD) \
		-channel $(CHANNEL) -dir $(OUT) $(BINARIES) > $(OUT)/massimo-release.json

clean:
	rm -rf massimo $(OUT)

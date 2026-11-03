# Builds the massimo binary with the version taken from the nearest git tag
# (semantic versioning: tag releases as vMAJOR.MINOR.PATCH).

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v0.0.0-dev)
LDFLAGS  = -s -w -X main.version=$(VERSION)
PKG      = ./cmd/massimo
OUT      = dist

.PHONY: all build test vet windows linux linux-arm64 linux-armv7 clean

all: test build

build:
	go build -ldflags "$(LDFLAGS)" -o massimo $(PKG)

test:
	go test ./...

vet:
	go vet ./...

# Cross builds; each lands in $(OUT)/.
windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT)/massimo-windows-amd64.exe $(PKG)

linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT)/massimo-linux-amd64 $(PKG)

linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(OUT)/massimo-linux-arm64 $(PKG)

# Raspberry Pi 2/3/4 with a 32-bit OS (LoxBerry on Raspberry Pi OS 32-bit).
linux-armv7:
	GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "$(LDFLAGS)" -o $(OUT)/massimo-linux-armv7 $(PKG)

clean:
	rm -rf massimo $(OUT)

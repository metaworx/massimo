#!/usr/bin/env bash
# Writes a release's build record, massimo-build.txt (`make build-release`), to
# standard output: where and with what it was built, the build information
# of every binary (`go version -m`: the Go version, every dependency with
# its checksum, the commit), that of the nfpm which built the Debian
# packages, then the build's whole log. SHA256SUMS and the signed manifest
# list the record, so the release's signature covers it. It runs in the
# build job, which holds no secret, and prints chosen `go env` values
# only, not the environment.
set -euo pipefail

usage() {
	echo "usage: $0 <build log> <nfpm module@version> <binary>..." >&2
	exit 2
}

[ "$#" -ge 3 ] || usage
log="$1" nfpm="$2"
shift 2
[ -f "$log" ] || { echo "build-record: no log at ${log}" >&2; exit 1; }

section() { printf '\n## %s\n\n' "$1"; }

echo "# massimo build record"
section "Build"
echo "command:   make build-release VERSION=${VERSION:-} FLAVOUR=${FLAVOUR:-} RELEASE=${RELEASE:-}"
echo "ldflags:   ${LDFLAGS:-}"
echo "commit:    $(git rev-parse HEAD)"
echo "tree:      $(git status --porcelain --untracked-files=no | grep -q . && echo modified || echo clean)"
echo "source date epoch: ${SOURCE_DATE_EPOCH:-}"
echo "started:   ${BUILD_STARTED:-}"
echo "recorded:  $(date -u +%Y-%m-%dT%H:%M:%SZ)"

section "Host"
if [ -n "${GITHUB_ACTIONS:-}" ]; then
	echo "run:       ${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/attempts/${GITHUB_RUN_ATTEMPT}"
	echo "workflow:  ${GITHUB_WORKFLOW_REF:-}"
	echo "event:     ${GITHUB_EVENT_NAME:-} of ${GITHUB_REF:-}"
	echo "runner:    ${RUNNER_ENVIRONMENT:-} ${RUNNER_OS:-} ${RUNNER_ARCH:-}"
	echo "image:     ${ImageOS:-} ${ImageVersion:-}"
else
	echo "run:       not GitHub Actions"
fi
echo "kernel:    $(uname -srm)"
if [ -r /etc/os-release ]; then
	# shellcheck disable=SC1091
	echo "system:    $(. /etc/os-release && echo "${PRETTY_NAME:-}")"
fi

section "Toolchain"
go version
for v in GOROOT GOHOSTOS GOHOSTARCH GOTOOLCHAIN GOFLAGS GOPROXY GOSUMDB GONOSUMDB GOPRIVATE GOEXPERIMENT CGO_ENABLED; do
	printf '%s=%s\n' "$v" "$(go env "$v")"
done

section "Binaries (go version -m)"
for b in "$@"; do
	go version -m "$b"
	echo
done

# go run keeps no nfpm binary: the same module, checked against the
# checksum database, is built again for its build information.
section "nfpm (go version -m of ${nfpm}, built again for this record)"
tools="$(mktemp -d)"
trap 'rm -rf "${tools:?}"' EXIT
GOBIN="$tools" go install "$nfpm"
go version -m "$tools"/nfpm

section "Log"
cat "$log"

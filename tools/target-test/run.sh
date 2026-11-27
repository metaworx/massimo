#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# run.sh - the server backup's targets tested against real servers, in
# Docker
#
# make target-test (README.md): brings up the servers of compose.yaml,
# Samba with one share, signing and encryption required, and MinIO, each
# on a port of the loopback Docker picks; waits until each answers,
# makes MinIO's buckets, one with object lock and a default retention,
# then runs the live tests of internal/serverbackup
#   MASSIMO_ENV_FILE=/dev/null go test ./internal/serverbackup -run Live
# with their variables set:
#   MASSIMO_TARGET_TEST_SMB_ADDR, _SMB_SHARE, _SMB_USER, _SMB_PASS;
#   MASSIMO_TARGET_TEST_S3_ENDPOINT, _S3_BUCKET, _S3_LOCK_BUCKET,
#   _S3_REGION, _S3_KEY, _S3_SECRET.
# A live test skips without its variables; here, where each is set, a
# skip fails the run, as a test that reads another name than these, and
# so does a run in which no live test passed. The servers are taken down
# whatever happened, their logs printed first where the run failed;
# KEEP=1 leaves them up and prints the variables, to run the tests again
# by hand, and the command that takes them down. Needs Docker with
# Compose v2 and Go; exit 1 where a test failed, 2 without Docker.
set -euo pipefail
readonly VERSION=1.1.0

case "${1:-}" in
	-h | --help)
		sed -n '4,24p' "$0" | sed 's/^# \{0,1\}//'
		exit 0
		;;
	--version)
		echo "run.sh $VERSION"
		exit 0
		;;
	"") ;;
	*)
		echo "run.sh: no arguments, see --help" >&2
		exit 2
		;;
esac

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
if ! docker compose version >/dev/null 2>&1; then
	echo "target-test: needs Docker with Compose v2 (tools/target-test/README.md)" >&2
	exit 2
fi

# The servers' accounts, test values that open nothing else; compose.yaml
# reads them, and the tests are given the same.
export TARGET_TEST_SMB_USER=massimo TARGET_TEST_SMB_PASS=massimo-test-pass
export TARGET_TEST_S3_KEY=massimo-test-key TARGET_TEST_S3_SECRET=massimo-test-secret
export TARGET_TEST_S3_REGION=eu-test-1
readonly SHARE=backups BUCKET=massimo-test LOCK_BUCKET=massimo-test-lock
export COMPOSE_PROJECT_NAME="massimo-target-$$"
compose() { docker compose -f "$here/compose.yaml" "$@"; }
out="$(mktemp "${TMPDIR:-/tmp}/massimo-target-test.XXXXXX")"

finish() {
	local rc=$?
	rm -f -- "$out"
	if [ "${KEEP:-}" = 1 ]; then
		# By the project alone: compose.yaml asks for the accounts run.sh sets.
		echo "target-test: KEEP=1, the servers stay up; take them down with" >&2
		echo "  (cd / && docker compose -p $COMPOSE_PROJECT_NAME down --volumes)" >&2
		exit "$rc"
	fi
	if [ "$rc" != 0 ]; then
		for s in samba minio; do
			echo "--- $s" >&2
			compose logs --no-color "$s" >&2 2>&1 || true
		done
	fi
	compose down --volumes --remove-orphans --timeout 2 >/dev/null 2>&1 || true
	exit "$rc"
}
trap finish EXIT

fail() {
	echo "FAIL  $*" >&2
	exit 1
}
# within tries a check until it passes, for up to $1 seconds
within() {
	local end=$((SECONDS + $1))
	shift
	until "$@"; do
		[ "$SECONDS" -lt "$end" ] || return 1
		sleep 0.5
	done
}

compose build --quiet samba >/dev/null 2>&1 || compose build samba
compose up --detach --quiet-pull samba minio >/dev/null 2>&1 || compose up --detach samba minio

# Each answers on its own loopback once it serves: smbd once it has its
# user, MinIO once its health check says ready. Docker's port on the
# host accepts a connection before either does.
samba_up() { compose exec -T samba bash -c 'exec 3<>/dev/tcp/127.0.0.1/445' 2>/dev/null; }
minio_up() { compose exec -T minio curl -sf http://127.0.0.1:9000/minio/health/ready >/dev/null 2>&1; }
within 60 samba_up || fail "Samba does not answer on port 445"
within 60 minio_up || fail "MinIO is not ready"
compose exec -T minio mc alias set local http://127.0.0.1:9000 "$TARGET_TEST_S3_KEY" "$TARGET_TEST_S3_SECRET" >/dev/null ||
	fail "mc does not reach MinIO"
compose exec -T minio mc mb --region "$TARGET_TEST_S3_REGION" "local/$BUCKET" >/dev/null || fail "the bucket $BUCKET is not made"
compose exec -T minio mc mb --with-lock --region "$TARGET_TEST_S3_REGION" "local/$LOCK_BUCKET" >/dev/null ||
	fail "the bucket $LOCK_BUCKET is not made"
compose exec -T minio mc retention set --default GOVERNANCE 1d "local/$LOCK_BUCKET" >/dev/null ||
	fail "the default retention of $LOCK_BUCKET is not set"

smb_addr="$(compose port samba 445)"
s3_addr="$(compose port minio 9000)"
export MASSIMO_TARGET_TEST_SMB_ADDR="$smb_addr" MASSIMO_TARGET_TEST_SMB_SHARE="$SHARE"
export MASSIMO_TARGET_TEST_SMB_USER="$TARGET_TEST_SMB_USER" MASSIMO_TARGET_TEST_SMB_PASS="$TARGET_TEST_SMB_PASS"
export MASSIMO_TARGET_TEST_S3_ENDPOINT="http://$s3_addr" MASSIMO_TARGET_TEST_S3_BUCKET="$BUCKET"
export MASSIMO_TARGET_TEST_S3_LOCK_BUCKET="$LOCK_BUCKET"
export MASSIMO_TARGET_TEST_S3_REGION="$TARGET_TEST_S3_REGION"
export MASSIMO_TARGET_TEST_S3_KEY="$TARGET_TEST_S3_KEY" MASSIMO_TARGET_TEST_S3_SECRET="$TARGET_TEST_S3_SECRET"
echo "target-test: Samba at $smb_addr (share $SHARE), MinIO at http://$s3_addr (buckets $BUCKET and $LOCK_BUCKET, region $TARGET_TEST_S3_REGION)"
if [ "${KEEP:-}" = 1 ]; then
	env | grep '^MASSIMO_TARGET_TEST_' | sort | sed 's/^/  export /'
fi

rc=0
(cd "$root" && MASSIMO_ENV_FILE=/dev/null go test ./internal/serverbackup -run Live -count=1 -v -timeout 10m) 2>&1 | tee "$out" || rc=$?
[ "$rc" = 0 ] || fail "a live test failed"
if skipped="$(grep -E -- '^\s*--- SKIP: Test[A-Za-z0-9]*Live' "$out")"; then
	fail "a live test skipped, its variables set: ${skipped//$'\n'/; }"
fi
grep -qE -- '^--- PASS: Test[A-Za-z0-9]*Live' "$out" || fail "no live test ran"
echo "ok    the targets against Samba and MinIO: $(grep -cE -- '^--- PASS: Test[A-Za-z0-9]*Live' "$out") live tests passed"

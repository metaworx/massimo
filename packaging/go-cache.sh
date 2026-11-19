#!/usr/bin/env bash
# The Go build and module caches of one of CI's heavy Go jobs, Vet and
# test, Race and Test (armv6, QEMU) (AP TestSpeed, D10-A; GUIDELINES/
# project/CI.md, section 3): their paths and keys as the step's outputs
# for actions/cache/restore and actions/cache/save, and whether the run
# saves (save=true: a push to main alone; a pull request's cache would be
# its own, and a tag's would only take the quota).
# A key is the job's name (Race and armv6 build otherwise than Vet and
# test), the runner's system and CPU, the Go version, the hash of go.sum
# and tools/uibundle/go.sum, and the ISO week: a new cache each week and
# with each new Go or go.sum, saved by the first run of the job on main
# that does not find it; a run restores the newest with the same Go and
# go.sum, or else the newest with the same Go. A week's cache is some
# 0.4 GB for Vet and test and 0.2 GB for each of the other two, 0.8 GB
# together, which GitHub keeps for 7 days after its last use: about
# 1.6 GB for two weeks, and 0.8 GB more for each new Go or go.sum.
# A go command that builds trims the build cache once a day of what was
# last used five days before or more; with the restored files' times,
# that was the whole of a cache saved five days before. A run that saves
# nothing (a pull request, a tag, a push to main that found the week's
# key) holds the trim off once the cache is restored (hold), so that it
# builds with all of it; a push to main that saves trims as Go does, so
# that what it saves is what main used in the last five days.
# usage: packaging/go-cache.sh key <job>, then packaging/go-cache.sh hold
# after the restore; in the checkout, after setup-go.
set -euo pipefail

case "${1:-}" in
key)
	job="${2:?usage: go-cache.sh key <job>}"
	case "$job" in *[!a-z0-9]*)
		echo "a job's name is lower-case letters and digits: ${job}" >&2
		exit 2
		;;
	esac
	sums="$(cat go.sum tools/uibundle/go.sum | sha256sum | cut -c1-16)"
	same="go-${job}-${RUNNER_OS:?}-${RUNNER_ARCH:?}-$(go env GOVERSION)-"
	save=false
	[ "${GITHUB_EVENT_NAME:-}:${GITHUB_REF:-}" != push:refs/heads/main ] || save=true
	{
		echo "path<<GO_CACHE_PATHS"
		go env GOCACHE GOMODCACHE
		echo "GO_CACHE_PATHS"
		echo "key=${same}${sums}-$(date -u +%G-W%V)"
		echo "restore-keys<<GO_CACHE_KEYS"
		echo "${same}${sums}-"
		echo "${same}"
		echo "GO_CACHE_KEYS"
		echo "save=${save}"
	} >>"${GITHUB_OUTPUT:?}"
	;;
hold)
	# trim.txt holds the time of Go's last trim, which a restored cache
	# brought from the run that saved it.
	gocache="$(go env GOCACHE)"
	mkdir -p "$gocache"
	date +%s >"${gocache}/trim.txt"
	;;
*)
	echo "usage: go-cache.sh key <job> | hold" >&2
	exit 2
	;;
esac

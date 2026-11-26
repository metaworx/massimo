#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# Gradle's downloads for one of CI's two Android jobs, Android app and
# Android emulator (AP SteadyTests, Block 9; GUIDELINES/project/CI.md,
# section 3): the artifacts Gradle fetched (caches/modules-2), the
# journal of when it last used each (caches/journal-1) and the wrapper's
# distribution (wrapper/dists), their paths and keys as the step's
# outputs for actions/cache/restore and actions/cache/save, and whether
# the run saves (save=true: a push to main alone, as packaging/
# go-cache.sh saves). Without them every run fetched all of it, and Maven
# Central answered one with 429 Too Many Requests.
# A key is the job's name (the two run other tasks, which fetch other
# artifacts), the runner's system and CPU, the hash of the build's
# Gradle files (the wrapper's properties, the build scripts, the version
# catalogue) and the ISO week: a new cache each week and with each change
# to those files, saved by the first run of the job on main that does
# not find it; a run restores the newest with the same files, or else
# the job's newest. Gradle's build cache is not in it.
# Gradle prunes what it keeps at the end of a build: an artifact unused
# for 30 days, by the journal, and another version's distribution; so
# what main saves is what the job used in its last 30 days, and saved
# each week, nothing a run restores and uses is that old.
# usage: packaging/app-gradle-cache.sh <job>, in the checkout's top
# directory.
set -euo pipefail

job="${1:?usage: app-gradle-cache.sh <job>}"
case "$job" in *[!a-z0-9]*)
	echo "a job's name is lower-case letters and digits: ${job}" >&2
	exit 2
	;;
esac
wrapper=app/android/gradle/wrapper/gradle-wrapper.properties
if [ ! -f "$wrapper" ]; then
	echo "no ${wrapper}: run in the checkout's top directory" >&2
	exit 2
fi
# the files' names and contents, as git tracks them
files="$(git ls-files -z -- "$wrapper" app/android/gradle/libs.versions.toml 'app/android/*.gradle.kts' |
	xargs -0 sha256sum | sha256sum | cut -c1-16)"
home="${GRADLE_USER_HOME:-${HOME:?}/.gradle}"
same="gradle-${job}-${RUNNER_OS:?}-${RUNNER_ARCH:?}-"
save=false
[ "${GITHUB_EVENT_NAME:-}:${GITHUB_REF:-}" != push:refs/heads/main ] || save=true
{
	echo "path<<GRADLE_CACHE_PATHS"
	echo "${home}/caches/modules-2"
	echo "${home}/caches/journal-1"
	echo "${home}/wrapper/dists"
	echo "GRADLE_CACHE_PATHS"
	echo "key=${same}${files}-$(date -u +%G-W%V)"
	echo "restore-keys<<GRADLE_CACHE_KEYS"
	echo "${same}${files}-"
	echo "${same}"
	echo "GRADLE_CACHE_KEYS"
	echo "save=${save}"
} >>"${GITHUB_OUTPUT:?}"

#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The pinned emulator and system image of the emulator suite (pins.sh;
# tools/app-emulator/README.md, "Pins") in the Android SDK that
# APP_EMU_SDK names, else ANDROID_HOME:
#
#   sdk.sh check  says what the SDK holds against the pins, and exits 1
#                 where the emulator or the image is missing, with the
#                 sdkmanager line that installs it; a revision that is
#                 not the pin's is a warning, and the run goes on with it
#   sdk.sh fetch  installs the pinned emulator and image where the SDK
#                 holds another revision or none, from Google's SDK
#                 repository, each archive held to the size and SHA-1 the
#                 repository gives, as CI's job Android emulator does it:
#                 sdkmanager installs the newest revision alone. Its
#                 licence is the SDK's, which it needs accepted in
#                 ANDROID_HOME/licenses/ (sdkmanager --licenses) and never
#                 accepts itself. Where APP_EMU_SDK is not ANDROID_HOME,
#                 the SDK's platform-tools, platforms and licences are
#                 linked into it, as the emulator looks for them there.
set -euo pipefail

usage() {
	echo "usage: sdk.sh check | fetch" >&2
	exit 2
}

here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=pins.sh
. "$here/pins.sh"

sdk=${APP_EMU_SDK:-${ANDROID_HOME:-}}
if [ -z "$sdk" ]; then
	echo "app-emulator: neither APP_EMU_SDK nor ANDROID_HOME names the Android SDK (app/README.md, \"The toolchain in WSL\")" >&2
	exit 1
fi
image_pkg="system-images;android-${APP_EMU_API};${APP_EMU_TAG};${APP_EMU_ABI}"
image_dir="$sdk/system-images/android-${APP_EMU_API}/${APP_EMU_TAG}/${APP_EMU_ABI}"

# revision dir prints the revision of the package in dir, as its
# source.properties says it, "" for none.
revision() {
	[ -f "$1/source.properties" ] || return 0
	sed -n '/^Pkg\.Revision=/{s///;s/\r$//;p;q;}' "$1/source.properties"
}

check() {
	local missing=0 e i
	e=$(revision "$sdk/emulator")
	i=$(revision "$image_dir")
	if [ ! -x "$sdk/emulator/emulator" ]; then
		echo "app-emulator: the SDK has no emulator ($sdk/emulator): sdkmanager emulator, or tools/app-emulator/sdk.sh fetch" >&2
		missing=1
	elif [ "$e" != "$APP_EMU_EMULATOR_REVISION" ]; then
		echo "app-emulator: warning: the emulator is ${e:-of no known revision}, the pin $APP_EMU_EMULATOR_REVISION (tools/app-emulator/pins.sh)" >&2
	fi
	if [ ! -f "$image_dir/system.img" ]; then
		echo "app-emulator: the SDK has no system image $image_pkg ($image_dir): sdkmanager \"$image_pkg\", or tools/app-emulator/sdk.sh fetch" >&2
		missing=1
	elif [ "$i" != "$APP_EMU_IMAGE_REVISION" ]; then
		echo "app-emulator: warning: the system image is revision ${i:-unknown}, the pin $APP_EMU_IMAGE_REVISION (tools/app-emulator/pins.sh)" >&2
	fi
	[ "$missing" = 0 ] || exit 1
	echo "app-emulator: the emulator $e and $image_pkg revision $i, in $sdk"
}

# getting is the directory a fetch unpacks into, removed at the exit
# whatever ends it: a failed download or unzip, a signal.
getting=""
trap '[ -z "$getting" ] || rm -rf "$getting"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# get url size sha1 dest unpacks the archive at url, held to its size
# and SHA-1, into dest, the one directory it holds becoming dest.
get() {
	local work n
	work=$(mktemp -d "$sdk/.app-emulator.XXXXXX")
	getting=$work
	echo "app-emulator: fetching $1 ($(($2 / 1048576)) MiB)"
	if ! curl -fsSL --retry 3 --retry-delay 5 -o "$work/a.zip" "$1"; then
		echo "app-emulator: $1 was not fetched" >&2
		exit 1
	fi
	n=$(stat -c %s "$work/a.zip")
	if [ "$n" != "$2" ]; then
		echo "app-emulator: $1 is $n bytes, not the $2 of the pin (tools/app-emulator/pins.sh)" >&2
		exit 1
	fi
	if ! sha1sum -c --quiet - <<<"$3  $work/a.zip"; then
		echo "app-emulator: $1 is not the archive pinned (its SHA-1; tools/app-emulator/pins.sh)" >&2
		exit 1
	fi
	mkdir "$work/x"
	unzip -q "$work/a.zip" -d "$work/x"
	rm -f "$work/a.zip"
	set -- "$1" "$2" "$3" "$4" "$work"/x/*
	if [ $# -ne 5 ] || [ ! -d "$5" ]; then
		echo "app-emulator: $1 does not hold one directory" >&2
		exit 1
	fi
	rm -rf "$4"
	mkdir -p "$(dirname "$4")"
	mv "$5" "$4"
	rm -rf "$work"
	getting=""
}

fetch() {
	local licences="${ANDROID_HOME:-$sdk}/licenses"
	if [ ! -f "$licences/android-sdk-license" ]; then
		echo "app-emulator: the Android SDK's licence is not accepted ($licences/android-sdk-license): sdkmanager --licenses asks for it; fetch accepts none" >&2
		exit 1
	fi
	mkdir -p "$sdk"
	if [ "$(revision "$sdk/emulator")" != "$APP_EMU_EMULATOR_REVISION" ]; then
		get "$APP_EMU_EMULATOR_URL" "$APP_EMU_EMULATOR_SIZE" "$APP_EMU_EMULATOR_SHA1" "$sdk/emulator"
	fi
	if [ "$(revision "$image_dir")" != "$APP_EMU_IMAGE_REVISION" ]; then
		get "$APP_EMU_IMAGE_URL" "$APP_EMU_IMAGE_SIZE" "$APP_EMU_IMAGE_SHA1" "$image_dir"
	fi
	if [ -n "${ANDROID_HOME:-}" ] && [ "$sdk" != "$ANDROID_HOME" ]; then
		for d in platform-tools platforms licenses; do
			[ -e "$sdk/$d" ] || [ ! -e "$ANDROID_HOME/$d" ] || ln -s "$ANDROID_HOME/$d" "$sdk/$d"
		done
	fi
	check
}

[ $# -eq 1 ] || usage
case "$1" in
check) check ;;
fetch) fetch ;;
*) usage ;;
esac

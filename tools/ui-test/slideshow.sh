#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# slideshow.sh - the web UI's slideshow, from the screenshot set
#
# The frames are the main set that screenshots.py takes, in its order,
# as `screenshots.py --list` prints them: the list is kept there only.
# Every frame is whole and opaque on purpose: a GIF optimised into
# patches with transparent pixels lets a viewer that draws a patch over
# the frame before show that frame through. A frame that covers the
# canvas and has no transparent colour leaves nothing of another to show,
# however a viewer disposes of it; the run checks that of what it wrote,
# and replaces the target only then.
set -euo pipefail

readonly VERSION=1.0.0
readonly CANVAS=880x640
readonly BACKGROUND='#f6f6f4' # the light scheme's page background
readonly DELAY=300            # hundredths of a second per frame

usage() {
	cat <<'EOF'
slideshow.sh - builds the web UI's slideshow from the screenshot set

Usage:
  tools/ui-test/slideshow.sh [<out.gif>]
  tools/ui-test/slideshow.sh --help | --version

Builds an animated GIF, by default docs/screenshots/slideshow.gif, from
the main screenshot set in its order (tools/ui-test/screenshots.py
--list; make ui-screenshots takes them). Each is shrunk, never enlarged,
to fit 880 by 640 pixels and centred on a canvas of that size, 3 seconds
a frame, looping. The target is replaced only when every frame came out
whole and opaque.

Requires ImageMagick (convert and identify, or magick) and Python 3
(UI_TEST_PYTHON, else python3) for the list.

Exit codes:
  0  the slideshow is written
  1  a frame came out not whole or not opaque; the target is left as it was
  2  the tool could not run: a bad argument, a missing command or screenshot
EOF
}

die() {
	echo "slideshow.sh: $*" >&2
	exit 2
}

case "${1:-}" in
	-h | --help) usage; exit 0 ;;
	-V | --version) echo "slideshow.sh $VERSION"; exit 0 ;;
	-*) usage >&2; exit 2 ;;
esac
[ "$#" -le 1 ] || { usage >&2; exit 2; }

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
out=$(realpath -m -- "${1:-$root/docs/screenshots/slideshow.gif}")
case "$out" in
	*.gif) ;;
	*) die "not a .gif: $out" ;;
esac

if command -v convert >/dev/null && command -v identify >/dev/null; then
	convert=(convert)
	identify=(identify)
elif command -v magick >/dev/null; then
	convert=(magick)
	identify=(magick identify)
else
	die "ImageMagick (convert and identify, or magick) is not installed"
fi
py=${UI_TEST_PYTHON:-python3}
command -v "$py" >/dev/null || die "no Python: $py"

cd "$root"
mapfile -t shots < <("$py" "$here/screenshots.py" --list)
[ "${#shots[@]}" -gt 0 ] || die "screenshots.py --list named no screenshot"
for shot in "${shots[@]}"; do
	[ -f "$shot" ] || die "$shot is missing (make ui-screenshots takes it)"
done

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT INT TERM

frames=()
for i in "${!shots[@]}"; do
	frames+=("$work/$i.png")
	"${convert[@]}" "${shots[$i]}" -resize "$CANVAS>" -background "$BACKGROUND" -gravity center \
		-extent "$CANVAS" +repage -alpha off "$work/$i.png"
done
"${convert[@]}" -delay "$DELAY" -loop 0 -dispose None "${frames[@]}" -dither None "gif:$work/out.gif"

# Whole and opaque, every one of them, or a viewer may show the frame before.
bad=0
count=0
while read -r index geometry alpha; do
	count=$((count + 1))
	case "$geometry $alpha" in
		"$CANVAS+0+0 False" | "$CANVAS+0+0 Undefined") ;;
		*)
			echo "slideshow.sh: frame $index: $geometry, alpha $alpha" >&2
			bad=1
			;;
	esac
done < <("${identify[@]}" -format '%s %wx%h%O %A\n' "gif:$work/out.gif")
if [ "$count" -ne "${#shots[@]}" ]; then
	echo "slideshow.sh: $count frames for ${#shots[@]} screenshots" >&2
	bad=1
fi
[ "$bad" -eq 0 ] || exit 1

mkdir -p "$(dirname "$out")"
mv -f "$work/out.gif" "$out"
echo "slideshow.sh: wrote $out, ${#shots[@]} frames, $(($(wc -c <"$out") / 1024)) KiB"

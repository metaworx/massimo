# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The test suites' work directories (tools/workdir/README.md): a POSIX sh
# library that the UI suite's run.sh (sh) and the emulator suite's run.sh
# (bash) source, so that both make, sweep and keep theirs alike (AP
# EmulatorRuns, D3-B). Every name it uses starts with workdir_, so that
# it takes none of the caller's. It is safe under set -eu and set -euo
# pipefail, and inside $(…) whether set -e holds there or not, since
# workdir_make and the sweep test each step that can fail themselves;
# workdir_pid and workdir_done leave a failure to the caller's set -e.
#
# workdir_make name: sweeps (workdir_sweep name), then makes the run's
# work directory, name.XXXXXX under TMPDIR (default /tmp), writes the
# caller's PID into its file pid and, under KEEP=1, an empty file kept,
# from the start, so that a run killed before its cleanup keeps it (AP
# CleanRuns, D4-A); it prints the directory's path, and fails where it
# cannot make it. Called as work=$(workdir_make name), where $$ is still
# the caller's.
#
# workdir_pid dir pid: adds a PID to dir's pid, one per line, for a
# process that may outlive the run, the emulator.
#
# workdir_sweep name: removes the work directories of earlier runs that
# are gone (AP CleanRuns, D3-A).
#
# workdir_done dir who: at the run's end, removes dir, or under KEEP=1
# keeps it and says so, its path named, the line opened by who.
#
# A library: it sets no shell option.
# shellcheck shell=sh

# workdir_named name: whether name is one a work directory may carry,
# letters, digits, - and _, so that a sweep's pattern matches the suite's
# directories alone; it says why where it is not.
workdir_named() {
	case "$1" in
	'' | *[!A-Za-z0-9_-]*)
		echo "workdir: '$1' is not a work directory's name (letters, digits, - and _)" >&2
		return 1
		;;
	esac
}

# workdir_live dir: whether a PID in dir's pid is a running process's; a
# line that is not a number is passed over, and a directory without pid
# has none.
workdir_live() {
	workdir_ps=$(cat "$1/pid" 2>/dev/null) || workdir_ps=""
	while IFS= read -r workdir_p; do
		case "$workdir_p" in
		'' | *[!0-9]* | 0*) continue ;;
		esac
		if kill -0 "$workdir_p" 2>/dev/null; then return 0; fi
	done <<EOF
$workdir_ps
EOF
	return 1
}

# workdir_sweep name: the work directories of earlier runs, name.* under
# TMPDIR, that are gone, older than a day (-mmin, since BSD's find rounds
# -mtime's days up), not kept and this user's; a run is gone where none
# of the PIDs in its pid runs, and a directory without them is one whose
# run was killed as it began. A directory that cannot be removed stays,
# and the run goes on.
workdir_sweep() {
	workdir_named "$1" || return 1
	for workdir_d in "${TMPDIR:-/tmp}/$1".*; do
		if [ ! -d "$workdir_d" ] || [ ! -O "$workdir_d" ] || [ -e "$workdir_d/kept" ]; then continue; fi
		if workdir_live "$workdir_d"; then continue; fi
		[ -n "$(find "$workdir_d" -maxdepth 0 -mmin +1440 2>/dev/null)" ] || continue
		rm -rf "$workdir_d" 2>/dev/null || :
	done
}

# workdir_make name: the run's work directory, made after the sweep, its
# path printed (above).
workdir_make() {
	workdir_named "$1" || return 1
	workdir_sweep "$1"
	workdir_dir=$(mktemp -d "${TMPDIR:-/tmp}/$1.XXXXXX") || return 1
	if ! echo $$ >"$workdir_dir/pid"; then
		rm -rf "$workdir_dir"
		return 1
	fi
	if [ "${KEEP:-}" = 1 ] && ! printf '' >"$workdir_dir/kept"; then
		rm -rf "$workdir_dir"
		return 1
	fi
	printf '%s\n' "$workdir_dir"
}

# workdir_pid dir pid: pid added to dir's pid.
workdir_pid() {
	echo "$2" >>"$1/pid"
}

# workdir_done dir who: dir removed, or kept under KEEP=1 and named.
workdir_done() {
	if [ "${KEEP:-}" = 1 ]; then
		echo "$2: KEEP=1, the work directory is kept: $1" >&2
	else
		rm -rf "$1"
	fi
}

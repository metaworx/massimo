#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# Starts D-Bus and avahi-daemon in a container of the LAN test, as a
# box's init does, then runs the command given in their place (README.md).
set -eu
mkdir -p /run/dbus
dbus-daemon --system --fork --nopidfile
avahi-daemon --daemonize --no-chroot --no-rlimits
exec "$@"

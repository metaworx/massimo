#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# Makes the share's user, SMB_USER with the password SMB_PASS, owner of
# the share's folder, then runs smbd in the foreground, its log on
# standard output (README.md).
set -eu
: "${SMB_USER:?}" "${SMB_PASS:?}"
id "$SMB_USER" >/dev/null 2>&1 || useradd --no-create-home --shell /usr/sbin/nologin "$SMB_USER"
mkdir -p /srv/backups
chown "$SMB_USER" /srv/backups
chmod 0700 /srv/backups
printf '%s\n%s\n' "$SMB_PASS" "$SMB_PASS" | smbpasswd -a -s "$SMB_USER" >/dev/null
exec smbd --foreground --no-process-group --debug-stdout --debuglevel=1

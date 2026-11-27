# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The Samba server of the targets' test (README.md): Debian's samba, as
# a NAS or a Linux box runs it, with smb.conf's share, signing and
# encryption required, and the user samba.sh makes at the start.
FROM debian:trixie-slim
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends samba \
 && rm -rf /var/lib/apt/lists/*
COPY smb.conf /etc/samba/smb.conf
COPY samba.sh /usr/local/bin/samba.sh
ENTRYPOINT ["/usr/local/bin/samba.sh"]

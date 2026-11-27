# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The Samba server of the targets' test (README.md): Debian's samba, as
# a NAS or a Linux box runs it, with smb.conf's share, signing and
# encryption required, and the user samba.sh makes at the start. The
# image is pinned by its digest (compose.yaml says how to take a newer).
# debian:trixie-slim of 2026-11-27
FROM debian@sha256:8aa5a7bf5a1955bddbe7d74fcdd0b91ab30e74362a8b73af672b0481b35b44a0
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends samba \
 && rm -rf /var/lib/apt/lists/*
COPY smb.conf /etc/samba/smb.conf
COPY samba.sh /usr/local/bin/samba.sh
ENTRYPOINT ["/usr/local/bin/samba.sh"]

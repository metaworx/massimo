# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The image of the mDNS test's box and client (README.md): Debian with
# avahi, as a Raspberry Pi OS box and a Linux desktop run it. massimo is
# not in it: run.sh builds it and the Compose file mounts it.
FROM debian:trixie-slim
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends avahi-daemon avahi-utils dbus procps \
 && rm -rf /var/lib/apt/lists/*
COPY avahi.sh /usr/local/bin/avahi.sh

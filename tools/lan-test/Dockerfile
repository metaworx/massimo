# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The image of the LAN test's hosts (README.md): Debian with avahi, as a
# Raspberry Pi OS box and a Linux desktop run it, curl for the searcher's
# API and iproute2 for the Cube's cable. massimo is not in it: run.sh
# builds it and the Compose file mounts it.
FROM debian:trixie-slim
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends avahi-daemon avahi-utils curl dbus iproute2 procps \
 && rm -rf /var/lib/apt/lists/*
COPY avahi.sh /usr/local/bin/avahi.sh

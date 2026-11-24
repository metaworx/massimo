# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 The Massimo Authors
#
# The emulator and the system image the emulator suite runs on
# (tools/app-emulator/README.md, "Pins"), sourced by run.sh and sdk.sh:
# Android 14 with Google's APIs on x86_64, which has the fingerprint's
# settings and the emulator's virtual sensor; the archives as Google's
# SDK repository lists them (repository2-3.xml for the emulator,
# sys-img/google_apis/sys-img2-3.xml for the image), with the size and
# the SHA-1 it gives, which sdkmanager checks too. A pin moves by a
# decision, its values together, never by the day a run is made.
# A library: it sets no shell option.
# Every name here is read by the scripts that source the file.
# shellcheck shell=bash disable=SC2034

APP_EMU_API=34
APP_EMU_TAG=google_apis
APP_EMU_ABI=x86_64
APP_EMU_IMAGE_REVISION=14
APP_EMU_IMAGE_URL=https://dl.google.com/android/repository/sys-img/google_apis/x86_64-34_r14.zip
APP_EMU_IMAGE_SIZE=1563721130
APP_EMU_IMAGE_SHA1=e0f6c9a0691aa27bd597d0deb1bcfdc943ac8ca7
APP_EMU_EMULATOR_REVISION=37.3.10
APP_EMU_EMULATOR_URL=https://dl.google.com/android/repository/emulator-linux_x64-16970787.zip
APP_EMU_EMULATOR_SIZE=350068887
APP_EMU_EMULATOR_SHA1=5f7314368fc6c05c937f6991bb4106214dc389e7

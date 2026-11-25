// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// Package appemulator holds the tests of the emulator suite's scripts on
// the host (README.md): the helper that answers each touch of the
// fingerprint a test asks for (fingers.sh), against a fake adb; the
// checkout's lock, the slot that gives a run its ports and the holder of
// both locks (slots.sh), against a fake adb and a fake ss; sdk.sh's check
// and fetch; and the pins. The suite itself is the Android app's
// instrumented tests (app/android/app/src/androidTest/), which run on an
// emulator by run.sh. It has no code of its own.
package appemulator

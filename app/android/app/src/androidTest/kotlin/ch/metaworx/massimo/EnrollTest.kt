// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assume.assumeTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The finger every prompt of the emulator suite takes
 * (tools/app-emulator/README.md, "The fingerprint"): run.sh runs this test
 * alone, before the suite, and stops where it fails, its screen kept;
 * where a finger is enrolled already, as in the suite's own run, it passes
 * at once. Skipped where the suite was given no servers, as on a phone by
 * hand.
 */
@RunWith(AndroidJUnit4::class)
class EnrollTest {
    /** capture keeps what a failed enrollment showed: Settings' screen. */
    @get:Rule
    val capture = KeepOnFailure()

    @Test
    fun aFingerIsEnrolled() {
        assumeTrue("the emulator suite runs from tools/app-emulator/run.sh, which gives it its servers", Rig.given)
        Rig.quiet()
        Fingers.enroll()
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.SystemClock
import org.junit.Assert.fail

/**
 * Wait is the suite's one wait for a thing to come true: a condition
 * checked every [POLL_MS] until it holds, for [WAIT_MS] unless the caller
 * gives another length, and the test goes on the moment it does. A wait
 * for the page is [Page.waitFor], which waits through it; a wait for
 * anything else, the server's audit or the app's tasks, is [until] or
 * [within]; a wait for the system's window, the prompt or a dialog, is
 * UiAutomator's own (`Rig.device.wait(Until…)`). No test writes a loop of
 * its own (tools/app-emulator/README.md, "How a test waits").
 */
object Wait {
    /** WAIT_MS is how long a wait lasts where its caller gives no other length. */
    const val WAIT_MS = 30_000L

    /** POLL_MS is how often a wait checks its condition. */
    const val POLL_MS = 250L

    /** until waits until [cond] holds, and fails naming [what] where it does not within [ms]. */
    fun until(what: String, ms: Long = WAIT_MS, cond: () -> Boolean) {
        if (!within(ms, cond)) fail("never true within ${ms / 1000} s: $what")
    }

    /**
     * within reports whether [cond] holds within [ms], checking it once
     * more at the end, and fails nothing: for a caller whose failure says
     * what it saw.
     */
    fun within(ms: Long = WAIT_MS, cond: () -> Boolean): Boolean {
        val end = SystemClock.elapsedRealtime() + ms
        while (true) {
            if (cond()) return true
            if (SystemClock.elapsedRealtime() >= end) return false
            SystemClock.sleep(POLL_MS)
        }
    }
}

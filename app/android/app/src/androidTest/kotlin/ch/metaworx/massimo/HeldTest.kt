// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.SystemClock
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The sessions held (AP MobileApp, Block 21; F41, F43, F47; D100-A,
 * D102-A), each read back from the server's audit where it shows there:
 * the sessions held live with the process, so that the phone's night mode,
 * which makes the activity anew, keeps a session in memory; and a session
 * the server ended itself, made from the key a new one was made from or
 * from a key revoked, is not logged out, so that no refused logout reaches
 * the audit.
 */
@RunWith(AndroidJUnit4::class)
class HeldTest : EmulatorTest() {
    @Test
    fun theNightModeKeepsTheSessionHeld() {
        val t = Rig.a
        val id = add(t, A) // asked at each start: the session in memory
        keepLogin(t, A, id)
        tap(id) // the session held: the page opens with it
        signedIn(t)
        val before = activity()
        try {
            Rig.say("the night mode on")
            Rig.device.executeShellCommand("cmd uimode night yes")
            anew(before)
            signedIn(t)
            Prompt.none(NO_MORE_MS)
        } finally {
            Rig.device.executeShellCommand("cmd uimode night no")
        }
    }

    @Test
    fun aSessionTheServerEndedIsNotLoggedOut() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        val key = keepLogin(t, A, id)
        signOut(id, t) // no session held: the tap logs in with the key
        tap(id)
        Prompt.pass(Prompt.login(A))
        signedIn(t)
        toList()
        val mark = h.mark()
        // a second login by the key while its first session is held: the server ends the first itself (D31-A)
        ready(id)
        // not awaited: the page it opens takes the list's place
        page.eval("massimo.open(${q(id)}, conn.probes[${q(id)}].url, { login: true }); 1", "the second login by the key")
        Prompt.pass(Prompt.login(A))
        signedIn(t)
        toList()
        // Turn off with a session made from the key: revoked with it, which ends that session too
        openSettings(id)
        section(id, "login")
        page.click(row(id) + " .server-login-off")
        page.waitFor("!!conn.servers.find((s) => s.id === ${q(id)}) && !conn.servers.find((s) => s.id === ${q(id)}).login", "the login turned off")
        val rows = h.since(mark)
        assertTrue("the key $key not revoked: $rows", rows.any { it.action == "token.revoke" && it.target == "token:$key" && it.result == "ok" })
        assertTrue("a session the server ended was logged out: $rows", rows.none { it.action == "logout" })
        // a refused logout is a row of the address's refused requests, written once its minute is over
        SystemClock.sleep(REFUSALS_MS)
        val all = h.since(mark, anyone = true)
        assertTrue("a refused logout in the audit: $all", all.none { it.action == "logout" && it.result == "refused" })
    }

    // activity is the activity shown now, by its identity.
    private fun activity(): Int {
        var a = 0
        sc.onActivity { a = System.identityHashCode(it) }
        return a
    }

    // anew waits until the activity shown is another than [before]: made anew.
    private fun anew(before: Int) = Wait.until("the activity made anew") { activity() != before }

    private companion object {
        const val REFUSALS_MS = 65_000L
    }
}

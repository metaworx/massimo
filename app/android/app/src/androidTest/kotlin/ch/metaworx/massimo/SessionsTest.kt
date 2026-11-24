// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Sessions and keys (AP MobileApp, Block 16; F28, F38; D69-A, D80-B), each
 * read back from the server's audit: Turn off with a session revokes the
 * kept key with it, no prompt, and signs it out; without one, the key
 * revokes itself after the prompt; Remove of a server without a kept login
 * signs its session out first.
 */
@RunWith(AndroidJUnit4::class)
class SessionsTest : EmulatorTest() {
    @Test
    fun turnOffWithASessionRevokesTheKeyWithItAndSignsItOut() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        val key = keepLogin(t, A, id) // the password's session held
        val mark = h.mark()
        openSettings(id)
        page.click(row(id) + " .server-login-off")
        page.waitFor("!!conn.servers.find((s) => s.id === ${q(id)}) && !conn.servers.find((s) => s.id === ${q(id)}).login", "the login turned off")
        Prompt.none(NO_MORE_MS)
        val rows = h.since(mark)
        val revoke = rows.firstOrNull { it.action == "token.revoke" && it.target == "token:$key" && it.result == "ok" }
            ?: return fail("the key $key not revoked: $rows")
        assertNotEquals("the key revoked itself, not with the session: $rows", key, revoke.tokenId)
        assertTrue("the session not signed out after the revoke: $rows", rows.any { it.action == "logout" && it.result == "ok" && it.tokenId == revoke.tokenId && it.id > revoke.id })
    }

    @Test
    fun turnOffWithoutASessionRevokesTheKeyWithItselfAfterThePrompt() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        val key = keepLogin(t, A, id)
        signOut(id, t)
        val mark = h.mark()
        openSettings(id)
        page.click(row(id) + " .server-login-off")
        Prompt.pass(Prompt.off(A))
        page.waitFor("!!conn.servers.find((s) => s.id === ${q(id)}) && !conn.servers.find((s) => s.id === ${q(id)}).login", "the login turned off")
        val rows = h.since(mark)
        assertTrue("the key $key not revoked by itself: $rows", rows.any { it.action == "token.revoke" && it.target == "token:$key" && it.tokenId == key && it.result == "ok" })
    }

    @Test
    fun removeSignsTheSessionOutFirst() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        tap(id)
        signIn(t, A, keep = false)
        val token = page.run("return await massimo.token();", "the page's session") as? String ?: return fail("the page holds no session")
        val session = h.sessionId(token)
        toList()
        val mark = h.mark()
        openSettings(id)
        page.click(row(id) + " .server-remove")
        page.waitFor("!!document.querySelector(${q(row(id) + " .server-remove-box .confirm-yes")})", "Remove's question")
        page.click(row(id) + " .server-remove-box .confirm-yes")
        page.waitFor("conn.servers.length === 0", "the server removed")
        val rows = h.since(mark)
        assertTrue("the session $session not signed out: $rows", rows.any { it.action == "logout" && it.result == "ok" && it.tokenId == session })
        assertEquals("the session still valid at the server", 401, h.status(token))
    }
}

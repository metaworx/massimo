// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * What every later case stands on (AP MobileApp, Blocks 2 and 8): a server
 * added by its address, its self-signed certificate trusted by the
 * fingerprint the sheet shows; and the login by fingerprint kept after a
 * password's sign-in, with one prompt, its key made at the server.
 */
@RunWith(AndroidJUnit4::class)
class KeepTest : EmulatorTest() {
    @Test
    fun aServerIsAddedAndItsCertificateTrusted() {
        val t = Rig.a
        page.waitFor("!document.getElementById('add').classList.contains('hidden')", "the Add card of an empty list")
        page.eval("document.getElementById('add-address').value = ${q(t.url)}; document.getElementById('add-form').requestSubmit(); 1", "the Add card's submit")
        page.waitFor("document.getElementById('fp-dialog').open", "the fingerprint sheet")
        val shown = page.text("#fp-new")
        assertTrue("the sheet shows $shown, the server's certificate is ${t.fp}", Fingerprint.same(shown, t.fp))
        page.click("#fp-trust")
        page.waitFor("conn.servers.length === 1 && !!conn.servers[0].fp", "the server kept with its certificate")
        val s = servers().getJSONObject(0)
        assertTrue("kept with ${s.getString("fp")}, the server's certificate is ${t.fp}", Fingerprint.same(s.getString("fp"), t.fp))
        assertEquals(t.url, s.getString("lan"))
        assertTrue("no time the certificate was trusted", s.getString("trusted_at").isNotEmpty())
        ready(s.getString("id"))
    }

    @Test
    fun theLoginIsKeptAfterAPasswordSignInWithOnePrompt() {
        val t = Rig.a
        val host = Host(t)
        val id = add(t, A)
        val mark = host.mark()
        tap(id)
        signIn(t, A, keep = true)
        Prompt.none(NO_MORE_MS)
        toList()
        val s = server(id)
        assertEquals(Rig.user, s.getString("login"))
        val key = s.getLong("login_key")
        assertTrue("the kept login's key has no id", key > 0)
        val rows = host.since(mark)
        assertTrue("no key $key made at the server: $rows", rows.any { it.action == "token.add" && it.target == "token:$key" && it.result == "ok" })
    }
}

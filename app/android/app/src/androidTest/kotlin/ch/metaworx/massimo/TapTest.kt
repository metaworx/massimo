// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The list and the card (AP MobileApp, Block 15; F31, F40; D62-A, D72-B,
 * D73-A, D74-A): a tap where a login is kept asks for the fingerprint, a
 * cancel opening the page at its card; a server asked at each return asks
 * at every tap, covers its page while the app is hidden, and is not
 * reached again past its prompt by Back from the list, which leaves the
 * app; the card names its server, and ☰ has This server's settings and
 * App settings; Sign out on a server's page shows the list (Block 20).
 */
@RunWith(AndroidJUnit4::class)
class TapTest : EmulatorTest() {
    @Test
    fun aTapAsksForTheFingerprintAndItsCancelOpensTheCard() {
        val t = Rig.a
        val id = add(t, A)
        keepLogin(t, A, id)
        signOut(id, t) // no session held: a tap logs in with the key (D74-A)
        tap(id)
        Prompt.pass(Prompt.login(A))
        signedIn(t)
        toList()
        signOut(id, t)
        tap(id)
        Prompt.cancel(Prompt.login(A))
        cardWithFingerprint(t)
    }

    @Test
    fun aServerAskedAtEachReturnAsksAtEveryTap() {
        val t = Rig.a
        val id = add(t, A, ask = "return")
        keepLogin(t, A, id)
        repeat(2) { n ->
            Rig.say("opening ${n + 1} of 2")
            tap(id)
            Prompt.pass(Prompt.login(A))
            signedIn(t)
            toList()
        }
    }

    @Test
    fun thePageIsCoveredAfterHomeUntilTheFingerprint() {
        val t = Rig.a
        val id = add(t, A, ask = "return")
        keepLogin(t, A, id)
        tap(id)
        Prompt.pass(Prompt.login(A))
        signedIn(t)
        Rig.device.pressHome()
        gone()
        page.waitCovered(true)
        Rig.bringBack()
        Prompt.pass(Prompt.unlock(A))
        page.waitCovered(false)
        signedIn(t)
    }

    @Test
    fun backFromTheListLeavesTheAppAndTheServersPageShowsItsCard() {
        val t = Rig.a
        val id = add(t, A, ask = "return")
        keepLogin(t, A, id) // the list in the place of the server's page, signed in there
        assertFalse("the WebView keeps the pages before the list", page.canGoBack())
        // the way Back took into the page before Block 15: reached again without a tap, it shows its card
        page.eval("setTimeout(() => { location.href = ${q(t.url)}; }, 0); 1", "the server's page reached again")
        cardWithFingerprint(t)
        page.click("#login-cancel")
        list()
        assertFalse("the WebView keeps the pages before the list", page.canGoBack())
        Rig.device.pressBack()
        gone()
        // the activity moved back with the list, or finished, which leaves the app too
        val url = runCatching { page.url() }.getOrNull()
        assertTrue("Back from the list went to $url", url == null || url.startsWith(Origins.CONNECT_URL))
    }

    @Test
    fun signOutOnAServersPageShowsTheList() {
        // AP MobileApp, Block 20: the session ended and forgotten, the app's list in place of the server's sign-in card
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        tap(id)
        signIn(t, A, keep = false)
        val token = page.run("return await massimo.token();", "the page's session") as? String ?: return fail("the page holds no session")
        page.click("#menu-logout")
        list()
        ready(id)
        assertEquals("the session not ended at the server", 401, h.status(token))
        assertTrue("the app still holds a session there: ${server(id)}", server(id).isNull("session"))
    }

    @Test
    fun theCardNamesItsServerAndTheMenuOpensItsSettingsAndTheApps() {
        val t = Rig.a
        val id = add(t, A)
        tap(id)
        card(t)
        assertEquals(A, page.text("#login-server-name"))
        assertEquals(t.origin, page.text("#login-server-address"))
        page.click("#login-server-more")
        page.waitFor("!document.getElementById('login-server-facts').classList.contains('hidden')", "the server's ⓘ")
        val fp = page.text("#login-server-fp")
        assertTrue("the card shows $fp, the server's certificate is ${t.fp}", Fingerprint.same(fp, t.fp))
        assertTrue("the card shows no instance id", page.text("#login-server-node").isNotBlank())
        signIn(t, A, keep = false)
        for (entry in listOf("#menu-server-settings", "#menu-app-settings")) {
            page.waitFor("!!document.querySelector(${q(entry)}) && !document.querySelector(${q(entry)}).classList.contains('hidden')", "☰'s $entry")
        }
        page.click("#menu-server-settings")
        list()
        page.waitFor("!!document.querySelector(${q(row(id) + " .server-panel")})", "the server's settings, opened from its ☰")
        tap(id)
        signedIn(t)
        page.click("#menu-app-settings")
        list()
        page.waitFor("!document.getElementById('app-card').classList.contains('hidden')", "App settings, opened from a server's ☰")
    }
}

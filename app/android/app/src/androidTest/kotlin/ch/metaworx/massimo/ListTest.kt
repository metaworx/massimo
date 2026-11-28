// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.app.ActivityManager
import android.os.SystemClock
import android.webkit.WebView
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.uiautomator.By
import androidx.test.uiautomator.Until
import org.json.JSONArray
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The list (AP MobileApp, Block 19; D91-A, D93-A), each step as a user takes
 * it, read back from the server's audit where it shows there: Exit from the
 * list's ☰ ends the app's task, and the session it dropped is ended by its
 * code at the next start; Back closes ☰, then a server's settings, and leaves
 * the app from the bare list; Store login keeps the login of the session
 * held with one prompt, its key made at the server; the list pulled down
 * checks every server again; and a server's page that fails to load gives
 * way to the list, which says the server does not answer (D108-A). The
 * server gone after the list's probe found it is one whose address the app
 * holds closed, its probe's finding kept, since nothing stops the host's
 * serve (CodesTest); and a certificate that changed keeps its own dialog,
 * the list then saying nothing of a server that does not answer, the
 * change stood in for by another certificate pinned.
 */
@RunWith(AndroidJUnit4::class)
class ListTest : EmulatorTest() {
    @Test
    fun exitLeavesTheRecentAppsAndItsSessionIsEndedAtTheNextStart() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A) // asked at each start: the session in memory
        tap(id)
        signIn(t, A, keep = true)
        val token = page.run("return await massimo.token();", "the page's session") as? String ?: return fail("the page holds no session")
        val session = h.sessionId(token)
        toList()
        ready(id) // the list's check of the session asked before Exit
        val mark = h.mark()
        page.click("#servers-menu > summary")
        page.waitFor("document.getElementById('servers-menu').open && !document.getElementById('menu-exit').classList.contains('hidden')", "Exit in the list's ☰")
        page.click("#menu-exit")
        gone()
        assertTrue("the app is still in the recent apps", noTasks())
        // nothing sent to the server: the session still valid there, no logout
        assertEquals("the session Exit dropped ended at once", 200, h.status(token))
        assertTrue("Exit logged out: ${h.since(mark)}", h.since(mark).none { it.action == "logout" })
        // started again: the first server added opens at start (D43-A) and takes its codes
        relaunch()
        revokedByCode(h, mark, session)
        assertEquals("the session Exit dropped still valid at the server", 401, h.status(token))
    }

    @Test
    fun backClosesTheMenuThenTheSettingsThenLeavesTheApp() {
        val t = Rig.a
        val id = add(t, A)
        ready(id)
        page.waitFor("back.ready", "the page's history entries ready")
        // a finger's tap: an entry the page adds before the user touched it is one Back passes over
        touch("#servers-menu > summary")
        entry()
        assertTrue("the WebView cannot go back to the list's own entry", page.canGoBack())
        Rig.device.pressBack()
        page.waitFor("!document.getElementById('servers-menu').open && !back.entry", "☰ closed by Back")
        openSettings(id)
        entry()
        Rig.device.pressBack()
        page.waitFor("!document.querySelector(${q(row(id) + " .server-panel")}) && !back.entry", "the settings closed by Back")
        assertTrue("the list left behind", page.url().startsWith(Origins.CONNECT_URL))
        // the bare list: Back leaves the app
        Rig.device.pressBack()
        gone()
    }

    @Test
    fun storeLoginKeepsTheLoginWithOnePrompt() {
        val t = Rig.b
        val h = Host(t)
        val id = add(t, B)
        tap(id)
        signIn(t, B, keep = false)
        toList()
        val mark = h.mark()
        openSettings(id)
        section(id, "login")
        val store = row(id) + " .server-store-login"
        page.waitFor("!!document.querySelector(${q(store)}) && !document.querySelector(${q(store)}).disabled", "Store login, on")
        page.click(store)
        Prompt.pass(Prompt.keep(B))
        page.waitFor("!!conn.servers.find((s) => s.id === ${q(id)}).login", "the login kept")
        Prompt.none(NO_MORE_MS)
        val key = server(id).optLong("login_key")
        assertTrue("the kept login's key has no id", key > 0)
        val rows = h.since(mark)
        assertTrue("no key $key made at the server: $rows", rows.any { it.action == "token.add" && it.target == "token:$key" && it.result == "ok" })
        page.waitFor("!!document.querySelector(${q(row(id) + " .server-login-kept")})", "the kept login's line")
    }

    @Test
    fun thePullDownChecksEveryServer() {
        val t = Rig.a
        val id = add(t, A)
        ready(id)
        val before = page.eval("conn.probeSeq[${q(id)}]", "the probes so far").toInt()
        // from the list's top down, as a finger pulls it
        val w = Rig.device.displayWidth
        val hh = Rig.device.displayHeight
        Rig.device.swipe(w / 2, hh / 4, w / 2, hh * 3 / 4, SWIPE_STEPS)
        page.waitFor("conn.probeSeq[${q(id)}] > $before && conn.probes[${q(id)}].state === 'lan'", "the server checked again")
    }

    @Test
    fun aPageThatFailsToLoadReturnsToTheList() {
        val t = Rig.a
        val id = add(t, A)
        ready(id)
        // gone since the list's probe found it: its address closed in the app, the probe's finding kept
        page.run(
            "const s = conn.servers.find((x) => x.id === ${q(id)}); keepServer(await massimo.save(Object.assign({}, s, { lan: ${q(CLOSED)} }))); " +
                "conn.probeSeq[${q(id)}] += 1; conn.probes[${q(id)}] = { state: 'lan', url: ${q(CLOSED)} }; return 1;",
            "the server's address closed after its probe",
        )
        page.click(row(id) + " .server-open")
        page.waitFor(
            "location.href === ${q(Origins.CONNECT_URL)} && typeof conn === 'object' && !!conn.bridge && !!conn.probes[${q(id)}] && conn.probes[${q(id)}].state === 'offline'",
            "back at the list, the server not reachable",
        )
        val said = page.run("return t('connect.unreachable', { name: ${q(A)} });", "the toast's text") as String
        page.waitFor(
            "!document.getElementById('toast').classList.contains('hidden') && document.getElementById('toast').textContent.includes(${q(said)})",
            "the toast saying it does not answer",
        )
        assertEquals("the handover left in the address", "\"\"", page.eval("location.hash", "the address's fragment"))
    }

    @Test
    fun aCertificateThatChangedKeepsItsDialogAndSaysNothingOfAnAnswer() {
        val t = Rig.a
        val id = add(t, A)
        ready(id)
        // another certificate pinned than the server's: the WebView refuses the server's as a changed one, the probe's finding kept
        page.run(
            "await massimo.trust(${q(id)}, ${q(OTHER_FP)}); keepServer(Object.assign({}, conn.servers.find((x) => x.id === ${q(id)}), { fp: ${q(OTHER_FP)} })); " +
                "conn.probeSeq[${q(id)}] += 1; conn.probes[${q(id)}] = { state: 'lan', url: ${q(t.url)} }; return 1;",
            "another certificate pinned",
        )
        page.click(row(id) + " .server-open")
        val title = Rig.app.getString(R.string.cert_changed_title)
        if (!Rig.device.wait(Until.hasObject(By.text(title)), DIALOG_MS)) fail("no dialog \"$title\"; the system shows: ${Prompt.systemTexts()}")
        Rig.device.findObject(By.res("android", "button1"))?.click() ?: fail("no OK on the dialog")
        list()
        assertEquals("no handover of a server that does not answer", "\"\"", page.eval("location.hash", "the address's fragment"))
        SystemClock.sleep(NO_TOAST_MS)
        assertEquals("a toast of a server that does not answer", "true", page.eval("document.getElementById('toast').classList.contains('hidden')", "the toast"))
    }

    // entry waits until the page holds its own history entry, a menu or a panel open.
    private fun entry() = page.waitFor("!!(history.state && history.state.massimo) && back.entry", "the page's history entry")

    // touch taps the element [selector] names as a finger does, at its
    // middle on the screen: the WebView's CSS pixels are the density's.
    private fun touch(selector: String) {
        val at = JSONArray(
            page.eval("(() => { const b = document.querySelector(${q(selector)}).getBoundingClientRect(); return [b.left + b.width / 2, b.top + b.height / 2]; })()", "where $selector is"),
        )
        val loc = IntArray(2)
        var density = 1f
        sc.onActivity { a ->
            a.findViewById<WebView>(R.id.web).getLocationOnScreen(loc)
            density = a.resources.displayMetrics.density
        }
        Rig.device.click((loc[0] + at.getDouble(0) * density).toInt(), (loc[1] + at.getDouble(1) * density).toInt())
    }

    // noTasks waits until the app has no task in the recent apps.
    private fun noTasks(): Boolean {
        val until = SystemClock.elapsedRealtime() + TASKS_MS
        while (SystemClock.elapsedRealtime() < until) {
            if (Rig.app.getSystemService(ActivityManager::class.java).appTasks.isEmpty()) return true
            SystemClock.sleep(POLL_MS)
        }
        return false
    }

    // revokedByCode waits until the audit has the token [tokenId] revoked by
    // its code after [mark], a row of nobody's, as CodesTest reads it.
    private fun revokedByCode(h: Host, mark: Long, tokenId: Long) {
        val until = SystemClock.elapsedRealtime() + CODE_MS
        var rows: List<Host.Row> = emptyList()
        while (SystemClock.elapsedRealtime() < until) {
            rows = h.since(mark, anyone = true)
            if (rows.any { it.action == "token.revoke_code" && it.target == "token:$tokenId" && it.result == "ok" }) return
            SystemClock.sleep(POLL_MS)
        }
        fail("the token $tokenId not revoked by its code: $rows")
    }

    private companion object {
        const val CLOSED = "https://127.0.0.1:9/"
        val OTHER_FP = "SHA256:" + List(32) { "00" }.joinToString(":")
        const val DIALOG_MS = 20_000L
        const val NO_TOAST_MS = 2000L
        const val SWIPE_STEPS = 40
        const val TASKS_MS = 10_000L
        const val CODE_MS = 20_000L
        const val POLL_MS = 500L
    }
}

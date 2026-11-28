// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.ext.junit.runners.AndroidJUnit4
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * The revoke codes (AP MobileApp, Block 21; F43, F44, F47; D102-A, D103-A,
 * D112-A), each read back from the server's audit, where a revoke by code
 * is a row without a user (`token.revoke_code`): a session the process
 * forgot, a key and a session Turn off left, a session Sign out left, and
 * Remove's, each ended by its code at the next contact. A server that does
 * not answer is one whose address the app holds closed, the rig's serve
 * running on: the instrumentation runs in the app's process, which no
 * test can end and go on, and nothing stops the host's serve. D103-A's
 * case keeps a login as one kept before version 6, without a code, so that
 * the next sign-in revokes its key left and My profile's toast says so;
 * one with its code is taken at the probe, and no toast shows.
 */
@RunWith(AndroidJUnit4::class)
class CodesTest : EmulatorTest() {
    @Test
    fun aSessionTheProcessForgotIsEndedByItsCode() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A) // asked at each start: the session in memory
        tap(id)
        signIn(t, A, keep = true)
        val token = token()
        val session = h.sessionId(token)
        toList()
        val mark = h.mark()
        // the process's end, as a swipe from the recent apps: the memory gone, the record left
        (Rig.app.applicationContext as MassimoApp).exit()
        // the start opens the server, the first added, at start (D43-A), which posts its codes
        relaunch()
        revokedByCode(h, mark, session)
        assertEquals("the session still valid at the server", 401, h.status(token))
    }

    @Test
    fun turnOffWhileTheServerIsDownLeavesCodesTheNextContactTakes() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        val key = keepLogin(t, A, id)
        signOut(id, t)
        tap(id)
        Prompt.pass(Prompt.login(A)) // a fingerprint tap's session, checked at once (F43)
        signedIn(t)
        val token = token()
        val session = h.sessionId(token)
        toList()
        down(id)
        settings(id)
        section(id, "login")
        page.click(row(id) + " .server-login-off")
        page.waitFor("!!conn.servers.find((s) => s.id === ${q(id)}) && !conn.servers.find((s) => s.id === ${q(id)}).login", "the login turned off")
        page.waitFor("!!document.getElementById('toast') && !document.getElementById('toast').classList.contains('hidden')", "Turn off's toast")
        val mark = h.mark()
        up(id, t)
        // the key's code first, which ends the key's session with it; the session's then matches nothing
        revokedByCode(h, mark, key)
        assertEquals("the session made from the key still valid", 401, h.status(token))
        assertTrue("$session", session > 0)
    }

    @Test
    fun signOutWhileTheServerIsDownIsEndedByItsCode() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        tap(id)
        signIn(t, A, keep = false)
        val token = token()
        val session = h.sessionId(token)
        toList()
        down(id)
        val res = page.run("return await massimo.signOut(${q(id)}, '');", "Sign out while the server does not answer") as JSONObject
        assertTrue("its code waits: $res", res.optBoolean("pending"))
        val mark = h.mark()
        up(id, t)
        revokedByCode(h, mark, session)
        assertEquals(401, h.status(token))
    }

    @Test
    fun removeWhileTheServerIsDownEndsItsSessionWhereItIsFoundAgain() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        tap(id)
        signIn(t, A, keep = false)
        val token = token()
        val session = h.sessionId(token)
        toList()
        down(id)
        val res = page.run("return await massimo.remove(${q(id)}, '');", "Remove while the server does not answer") as JSONObject
        assertTrue("its codes wait: $res", res.optBoolean("pending"))
        // the server found again at its address: gone.json named the closed one
        val gone = File(Rig.app.filesDir, "gone.json")
        val a = JSONArray(gone.readText())
        a.getJSONObject(0).put("lan", t.url)
        gone.writeText(a.toString())
        val mark = h.mark()
        relaunch()
        list() // the list shown posts a removed server's codes
        revokedByCode(h, mark, session)
        assertEquals(401, h.status(token))
    }

    @Test
    fun aKeyLeftWithoutItsCodeIsRevokedAtTheSignInAndSaidSo() {
        // D103-A: a login kept before version 6, its record without a code
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        val key = keepLogin(t, A, id)
        stripCode(id)
        signOut(id, t)
        down(id)
        settings(id)
        section(id, "login")
        page.click(row(id) + " .server-login-off")
        page.waitFor("!!conn.servers.find((s) => s.id === ${q(id)}) && !conn.servers.find((s) => s.id === ${q(id)}).login", "the login turned off")
        up(id, t)
        openSettings(id)
        section(id, "login")
        page.waitFor("!!document.querySelector(${q(row(id) + " .server-key-show")})", "Show in My profile")
        page.click(row(id) + " .server-key-show")
        signIn(t, A, keep = false)
        page.waitFor("location.hash.startsWith('#profile')", "My profile")
        val said = page.run("return t('profile.keyRevokedAtSignIn', { id: $key });", "the toast's text") as String
        page.waitFor("!document.getElementById('toast').classList.contains('hidden') && document.getElementById('toast').textContent.includes(${q(said)})", "the toast naming the key")
        assertEquals("the key marked", "0", page.eval("document.querySelectorAll('#profile-tokens li.token-marked').length", "the marks"))
        assertTrue("the key revoked at the sign-in", h.since(0).any { it.action == "token.revoke" && it.target == "token:$key" && it.result == "ok" })
    }

    @Test
    fun aKeyLeftWithItsCodeIsTakenAtTheProbeAndNoToastShows() {
        val t = Rig.a
        val h = Host(t)
        val id = add(t, A)
        val key = keepLogin(t, A, id)
        signOut(id, t)
        down(id)
        settings(id)
        section(id, "login")
        page.click(row(id) + " .server-login-off")
        page.waitFor("!!conn.servers.find((s) => s.id === ${q(id)}) && !conn.servers.find((s) => s.id === ${q(id)}).login", "the login turned off")
        val mark = h.mark()
        up(id, t)
        revokedByCode(h, mark, key)
        page.waitFor("conn.servers.find((s) => s.id === ${q(id)}).unrevoked.length === 0", "the key's line gone")
        tap(id)
        signIn(t, A, keep = false)
        val said = page.run("return t('profile.keyRevokedAtSignIn', { id: $key });", "the toast's text") as String
        Prompt.none(NO_MORE_MS)
        assertEquals("no toast of a key revoked at the sign-in", "false", page.eval("document.getElementById('toast').textContent.includes(${q(said)})", "the toast"))
    }

    // token is the session the page of the server shown holds.
    private fun token(): String = page.run("return await massimo.token();", "the page's session") as? String ?: throw AssertionError("the page holds no session")

    // down makes the server [id] one that does not answer: its address
    // closed in the app (a port nothing listens on), the list read again.
    private fun down(id: String) {
        Rig.say("the server $id down")
        page.run(
            "const s = conn.servers.find((x) => x.id === ${q(id)}); await massimo.save(Object.assign({}, s, { lan: ${q(CLOSED)} })); return 1;",
            "the server's address closed",
        )
        reloadList()
        page.waitFor("!!conn.probes[${q(id)}] && conn.probes[${q(id)}].state === 'offline'", "the server not reachable")
    }

    // up gives the server [id] its address at [t] again, and reads the list
    // again, whose probe finds it and has its codes posted (session).
    private fun up(id: String, t: Target) {
        Rig.say("the server $id up")
        page.run(
            "const s = conn.servers.find((x) => x.id === ${q(id)}); await massimo.save(Object.assign({}, s, { lan: ${q(t.url)} })); return 1;",
            "the server's address again",
        )
        reloadList()
        ready(id)
    }

    // settings opens the server [id]'s settings while it does not answer.
    private fun settings(id: String) {
        page.click(row(id) + " .server-cog")
        page.waitFor("!!document.querySelector(${q(row(id) + " .server-panel")})", "the server's settings")
    }

    // stripCode makes the login kept for [id] one kept before version 6: no revoke code beside its key's id.
    private fun stripCode(id: String) {
        val logins = LoginStore(File(Rig.app.filesDir, "logins.json"))
        val kept = logins.get(id) ?: return fail("no login kept for $id")
        logins.put(id, kept.copy(code = ""))
    }

    // revokedByCode waits until the audit has the token [tokenId] revoked by
    // its code after [mark], a row of nobody's.
    private fun revokedByCode(h: Host, mark: Long, tokenId: Long) {
        val until = android.os.SystemClock.elapsedRealtime() + CODE_MS
        var rows: List<Host.Row> = emptyList()
        while (android.os.SystemClock.elapsedRealtime() < until) {
            rows = h.since(mark, anyone = true)
            if (rows.any { it.action == "token.revoke_code" && it.target == "token:$tokenId" && it.result == "ok" }) return
            android.os.SystemClock.sleep(POLL_MS)
        }
        fail("the token $tokenId not revoked by its code: $rows")
    }

    private companion object {
        const val CLOSED = "https://127.0.0.1:9/"
        const val CODE_MS = 20_000L
        const val POLL_MS = 500L
    }
}

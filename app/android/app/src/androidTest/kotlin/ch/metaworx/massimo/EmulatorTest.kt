// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.core.app.ActivityScenario
import androidx.test.uiautomator.By
import androidx.test.uiautomator.Until
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Rule

/**
 * EmulatorTest is a test of the emulator suite (tools/app-emulator/README.md):
 * the app as installed anew (the orchestrator clears its data before each
 * test), the finger enrolled before the suite ([EnrollTest]), and the
 * connect page shown; then the steps a user takes, on the list and on a
 * server's page, by the helpers below. Skipped where the suite was given
 * no servers, as on a phone by hand. A failure leaves the screen
 * ([Rig.SHOTS], which run.sh takes home), the window's hierarchy and the
 * page's text in the log under [Rig.TAG].
 */
abstract class EmulatorTest {
    private var scenario: ActivityScenario<MainActivity>? = null

    /** sc is the activity the test drives. */
    protected val sc: ActivityScenario<MainActivity> get() = scenario ?: throw AssertionError("the activity was not launched")

    /** page is its WebView. */
    protected lateinit var page: Page

    /**
     * capture keeps what a failure showed, the page's text with it, then
     * closes the activity and takes the app's tasks out of recents
     * ([Rig.removeTasks]), whatever the test came to, so that the
     * orchestrator's clear before the next test kills nothing of it.
     */
    @get:Rule
    val capture = KeepOnFailure(
        more = { name -> Rig.chunks("$name: the page at ${page.url()}", page.text("body").take(PAGE_TEXT)) },
        after = {
            try {
                scenario?.close()
            } finally {
                Rig.removeTasks()
            }
        },
    )

    @Before
    fun start() {
        assumeTrue("the emulator suite runs from tools/app-emulator/run.sh, which gives it its servers", Rig.given)
        Rig.quiet()
        Fingers.check()
        scenario = ActivityScenario.launch(MainActivity::class.java)
        page = Page(sc)
        list()
    }

    /**
     * relaunch closes the activity and launches it anew, its tasks taken out
     * of recents between: the app started again, at the list or at the
     * server marked to open at start, which the first server added is.
     */
    protected fun relaunch() {
        scenario?.close()
        Rig.removeTasks()
        scenario = ActivityScenario.launch(MainActivity::class.java)
        page = Page(sc)
    }

    /** list waits for the connect page with its list read. */
    protected fun list() {
        page.waitFor("location.href.startsWith(${q(Origins.CONNECT_URL)}) && $LIST_READY", "the list of servers")
    }

    /** reloadList loads the connect page again, as the app does at its start, and waits for its list. */
    protected fun reloadList() {
        // the reload after the answer, which a page navigating away may not give
        page.eval("window.__massimoEmuOld = true; setTimeout(() => location.reload(), 0); 1", "the list's reload")
        page.waitFor("!window.__massimoEmuOld && $LIST_READY", "the list of servers, read again")
    }

    /**
     * add keeps the server [t] in the app under [name], asked [ask], as
     * the Add card keeps one once its certificate is trusted (the case
     * aServerIsAddedAndItsCertificateTrusted goes through the card), not
     * opened at start, and reads the list again; it returns the server's id.
     */
    protected fun add(t: Target, name: String, ask: String = "start"): String {
        val id = page.run(
            """
            const [r] = (await massimo.probe([${q(t.url)}], ${q(t.fp)})) || [];
            if (!r || !r.ok) throw new Error("the probe found " + JSON.stringify(r));
            const s = await massimo.save({ name: ${q(name)}, node: r.node || "", lan: ${q(t.url)}, remote: "", fp: "", ask: ${q(ask)}, push: false, start: false });
            await massimo.trust(s.id, ${q(t.fp)});
            return s.id;
            """.trimIndent(),
            "adding $name",
        ) as String
        reloadList()
        return id
    }

    /** servers are the servers as the app gives them to the list. */
    protected fun servers(): JSONArray = page.run("return await massimo.servers();", "the list's servers") as JSONArray

    /** server is the server [id] as the app gives it to the list. */
    protected fun server(id: String): JSONObject {
        val all = servers()
        for (i in 0 until all.length()) if (all.getJSONObject(i).getString("id") == id) return all.getJSONObject(i)
        fail("no server $id in the list")
        return JSONObject()
    }

    /** ready waits until the list's probe found the server [id] at its address. */
    protected fun ready(id: String) {
        page.waitFor("!!conn.probes[${q(id)}] && conn.probes[${q(id)}].state === 'lan'", "the server's probe")
    }

    /** tap taps the server [id] in the list, once its probe found it. */
    protected fun tap(id: String) {
        ready(id)
        Rig.say("tap $id")
        page.click(row(id) + " .server-open")
    }

    /** openSettings opens the server [id]'s settings under its row. */
    protected fun openSettings(id: String) {
        ready(id)
        page.click(row(id) + " .server-cog")
        page.waitFor("!!document.querySelector(${q(row(id) + " .server-panel")})", "the server's settings")
    }

    /**
     * section shows the section [key] (connection, login or certificate) of
     * the server [id]'s open settings by its tab, as a user does (AP
     * MobileApp, D111-A).
     */
    protected fun section(id: String, key: String) {
        page.click(row(id) + " .server-tab[data-tab=\"$key\"]")
        page.waitFor("!!document.querySelector(${q(row(id) + " .server-section[data-section=\"$key\"]:not(.hidden)")})", "the $key section")
    }

    /** card waits for the sign-in card on the page of [t]. */
    protected fun card(t: Target) {
        page.waitFor(
            "${on(t)} && document.readyState === 'complete' && !!document.getElementById('login-dialog') && document.getElementById('login-dialog').open && !!document.getElementById('login-user')",
            "the sign-in card at ${t.url}",
        )
    }

    /** cardWithFingerprint waits for the card of [t] with its fingerprint button, a login being kept. */
    protected fun cardWithFingerprint(t: Target) {
        card(t)
        page.waitFor("!document.getElementById('login-way-fp').classList.contains('hidden')", "the card's fingerprint button")
    }

    /** signedIn waits until the page of [t] is signed in as the suite's user, the card closed. */
    protected fun signedIn(t: Target) {
        page.waitFor(
            "${on(t)} && typeof state === 'object' && !!state.me && !!state.me.user && state.me.user.name === ${q(Rig.user)} && !document.getElementById('login-dialog').open",
            "signed in at ${t.url}",
        )
    }

    /**
     * signIn signs in on the card of [t], named [name], with the
     * password; with [keep] the switch keeps the login by fingerprint, and
     * the finger passes its prompt.
     */
    protected fun signIn(t: Target, name: String, keep: Boolean) {
        card(t)
        if (keep) {
            page.waitFor("!document.getElementById('login-keep-row').classList.contains('hidden')", "the switch that keeps the login")
            page.click("#login-keep")
            page.waitFor("document.getElementById('login-keep').checked", "the switch on")
        }
        // named, never shown: the script holds the password
        page.eval(
            "document.getElementById('login-user').value = ${q(Rig.user)}; document.getElementById('login-password').value = ${q(Rig.password)}; " +
                "document.getElementById('login-form').requestSubmit(); 1",
            "the sign-in form's submit",
        )
        if (keep) Prompt.pass(Prompt.keep(name))
        signedIn(t)
    }

    /** toList goes back to the list from a server's page, by ☰'s Select server. */
    protected fun toList() {
        page.click("#menu-servers")
        list()
    }

    /**
     * keepLogin opens the server [id], [t] named [name], from the list,
     * signs in keeping the login by fingerprint, with its one prompt, and
     * goes back to the list; it returns the kept key's id.
     */
    protected fun keepLogin(t: Target, name: String, id: String): Long {
        tap(id)
        signIn(t, name, keep = true)
        toList()
        val key = server(id).optLong("login_key")
        assertTrue("the kept login's key has no id", key > 0)
        return key
    }

    /** signOut signs the app out of the server [id] at [t], as its settings' Sign out does, and reads the list again. */
    protected fun signOut(id: String, t: Target) {
        page.run("return await massimo.signOut(${q(id)}, ${q(t.url)});", "signing out of ${t.url}")
        reloadList()
    }

    /** gone waits until no window of the app is shown. */
    protected fun gone() {
        if (!Rig.device.wait(Until.gone(By.pkg(Rig.app.packageName)), GONE_MS)) fail("the app is still shown")
    }

    /** row is the selector of the server [id]'s row in the list. */
    protected fun row(id: String): String = ".server-row[data-id=\"$id\"]"

    /** on is the page's test that it is the page of [t]. */
    protected fun on(t: Target): String = "location.origin === ${q(t.origin)}"

    /** q quotes [s] as a JavaScript string. */
    protected fun q(s: String): String = JSONObject.quote(s)

    companion object {
        /** A and B are the names the tests give the two servers. */
        const val A = "Server A"
        const val B = "Server B"

        /** NO_MORE_MS is how long a test waits for a prompt that must not come. */
        const val NO_MORE_MS = 4000L

        private const val GONE_MS = 10_000L
        private const val PAGE_TEXT = 4000

        // the connect page has started and drawn the list it read (connect.js
        // init, renderServers): a row per server, or the Add card where there
        // is none, which the markup hides until then
        private const val LIST_READY =
            "document.readyState === 'complete' && typeof conn === 'object' && !!conn.bridge && " +
                "((conn.servers.length === 0 && !document.getElementById('add').classList.contains('hidden')) || " +
                "(conn.servers.length > 0 && document.querySelectorAll('#server-list .server-row').length === conn.servers.length))"
    }
}

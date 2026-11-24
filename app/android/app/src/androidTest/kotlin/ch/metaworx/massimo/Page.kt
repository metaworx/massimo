// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.SystemClock
import android.view.View
import android.webkit.WebView
import androidx.test.core.app.ActivityScenario
import org.json.JSONObject
import org.json.JSONTokener
import org.junit.Assert.fail
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * Page drives the app's one WebView from a test, as a user's taps would
 * reach the page: a script evaluated in the page shown, the connect page
 * or a server's, its answer as JSON; a wait for a script to come true; a
 * click; and what the activity shows around it.
 */
class Page(private val sc: ActivityScenario<MainActivity>) {
    private var seq = 0

    /**
     * eval runs [js] in the page shown and returns its value as
     * evaluateJavascript gives it, JSON; a page that does not answer within
     * [seconds], one navigating away, fails the test, its failure naming
     * the step [what] and never the script, which may hold the password.
     */
    fun eval(js: String, what: String, seconds: Long = EVAL_S): String {
        var out = "null"
        val done = CountDownLatch(1)
        sc.onActivity { a ->
            a.findViewById<WebView>(R.id.web).evaluateJavascript(js) { r ->
                out = r ?: "null"
                done.countDown()
            }
        }
        if (!done.await(seconds, TimeUnit.SECONDS)) fail("no answer within $seconds s to the page's script: $what")
        return out
    }

    /** waitFor waits until [js] is true in the page, [what] naming it in the failure, and returns its value. */
    fun waitFor(js: String, what: String, ms: Long = WAIT_MS): String {
        val until = SystemClock.elapsedRealtime() + ms
        var last = ""
        while (SystemClock.elapsedRealtime() < until) {
            // a page navigating meanwhile may never answer: asked again
            last = runCatching { eval("(() => { try { return ($js); } catch (e) { return null; } })()", what, POLL_EVAL_S) }.getOrDefault("null")
            if (truthy(last)) return last
            SystemClock.sleep(POLL_MS)
        }
        fail("never true within ${ms / 1000} s: $what (the page at ${url()})")
        return last
    }

    /**
     * run runs [body], the body of an async function, in the page and
     * waits for its promise: the value it resolves, as JSON, or the
     * failure of [what] with the error it rejects with.
     */
    fun run(body: String, what: String, ms: Long = WAIT_MS): Any? {
        seq += 1
        val key = "__massimoEmu$seq"
        eval(
            """
            window.$key = undefined;
            (async () => { $body })().then(
              (v) => { window.$key = JSON.stringify({ ok: v === undefined ? null : v }); },
              (e) => { window.$key = JSON.stringify({ err: String((e && e.message) || e) }); });
            1
            """.trimIndent(),
            what,
        )
        val raw = waitFor("window.$key", what, ms)
        val o = JSONObject(JSONTokener(raw).nextValue() as String)
        if (o.has("err")) fail("$what: ${o.getString("err")}")
        return o.opt("ok")
    }

    /** click clicks the element [selector] names in the page. */
    fun click(selector: String) {
        val q = JSONObject.quote(selector)
        if (eval("(() => { const e = document.querySelector($q); if (!e) return false; e.click(); return true; })()", "a click at $selector") != "true") fail("nothing to click at $selector (the page at ${url()})")
    }

    /** text is the text of the element [selector] names, "" for none. */
    fun text(selector: String): String {
        val q = JSONObject.quote(selector)
        return JSONTokener(eval("(() => { const e = document.querySelector($q); return e ? e.textContent : ''; })()", "the text at $selector")).nextValue() as? String ?: ""
    }

    /** url is the address the WebView shows. */
    fun url(): String = onWeb { it.url ?: "" }

    /** canGoBack reports whether the WebView has a page to go back to. */
    fun canGoBack(): Boolean = onWeb { it.canGoBack() }

    /** covered reports whether the app's cover is shown over the page (MainActivity, a server asked at each return). */
    fun covered(): Boolean {
        val text = Rig.app.getString(R.string.cover_text)
        var shown = false
        sc.onActivity { a ->
            val found = ArrayList<View>()
            a.window.decorView.findViewsWithText(found, text, View.FIND_VIEWS_WITH_TEXT)
            shown = found.any { (it.parent as? View)?.visibility == View.VISIBLE }
        }
        return shown
    }

    /** waitCovered waits until the cover is shown, or [shown] false, gone. */
    fun waitCovered(shown: Boolean) {
        val until = SystemClock.elapsedRealtime() + WAIT_MS
        while (SystemClock.elapsedRealtime() < until) {
            if (covered() == shown) return
            SystemClock.sleep(POLL_MS)
        }
        fail(if (shown) "the page was not covered" else "the cover stayed")
    }

    private fun <T> onWeb(f: (WebView) -> T): T {
        var out: T? = null
        sc.onActivity { a -> out = f(a.findViewById(R.id.web)) }
        @Suppress("UNCHECKED_CAST")
        return out as T
    }

    private companion object {
        const val EVAL_S = 15L
        const val POLL_EVAL_S = 5L
        const val WAIT_MS = 30_000L
        const val POLL_MS = 250L

        fun truthy(r: String): Boolean = r != "null" && r != "false" && r != "\"\"" && r != "0" && r != "undefined"
    }
}

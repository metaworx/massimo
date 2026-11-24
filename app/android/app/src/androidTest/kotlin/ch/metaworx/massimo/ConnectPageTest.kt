// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.SystemClock
import android.webkit.WebView
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Test
import org.junit.runner.RunWith
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * The shell's smoke test, on a phone by hand (app/README.md) and in the
 * emulator suite (tools/app-emulator/README.md): the connect page loads
 * from the app's copy of the UI, the bridge answers it at the contract's
 * version, and it refuses the page a server's call.
 */
@RunWith(AndroidJUnit4::class)
class ConnectPageTest {
    private fun eval(sc: ActivityScenario<MainActivity>, js: String): String {
        var out = ""
        val done = CountDownLatch(1)
        sc.onActivity { a ->
            a.findViewById<WebView>(R.id.web).evaluateJavascript(js) { r ->
                out = r
                done.countDown()
            }
        }
        if (!done.await(5, TimeUnit.SECONDS)) fail("no answer to $js")
        return out
    }

    private fun waitFor(sc: ActivityScenario<MainActivity>, js: String): String {
        val until = SystemClock.elapsedRealtime() + 15_000
        while (SystemClock.elapsedRealtime() < until) {
            val r = eval(sc, js)
            if (r != "null" && r != "\"\"" && r != "false") return r
            SystemClock.sleep(200)
        }
        fail("never true: $js")
        return ""
    }

    @Test
    fun theBridgeAnswersTheConnectPage() {
        ActivityScenario.launch(MainActivity::class.java).use { sc ->
            waitFor(sc, "document.readyState === 'complete' && location.href.endsWith('/ui/connect.html')")
            eval(
                sc,
                """
                window.__smoke = "";
                massimo.servers().then(
                  (r) => { window.__smoke = JSON.stringify({ version: massimo.version, list: Array.isArray(r), token: typeof massimo.token }); },
                  (e) => { window.__smoke = "error: " + e.message; });
                1
                """.trimIndent(),
            )
            val got = waitFor(sc, "window.__smoke")
            assertEquals("\"{\\\"version\\\":5,\\\"list\\\":true,\\\"token\\\":\\\"undefined\\\"}\"", got)
        }
    }

    @Test
    fun theConnectPageMayNotAskForAToken() {
        ActivityScenario.launch(MainActivity::class.java).use { sc ->
            waitFor(sc, "document.readyState === 'complete' && typeof massimoChannel === 'object'")
            eval(
                sc,
                """
                window.__smoke = "";
                massimoChannel.addEventListener("message", (ev) => {
                  const m = JSON.parse(ev.data);
                  if (m.id === 99) window.__smoke = m.error ? "refused" : "answered";
                });
                massimoChannel.postMessage(JSON.stringify({ id: 99, call: "token", args: [] }));
                1
                """.trimIndent(),
            )
            assertEquals("\"refused\"", waitFor(sc, "window.__smoke"))
        }
    }
}

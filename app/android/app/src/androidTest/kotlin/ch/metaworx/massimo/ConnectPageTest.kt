// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The shell's smoke test, on a phone by hand (app/README.md) and in the
 * emulator suite (tools/app-emulator/README.md): the connect page loads
 * from the app's copy of the UI, the bridge answers it at the contract's
 * version, and it refuses the page a server's call. It needs no server,
 * so it is no [EmulatorTest], and drives the page through the kit's
 * [Page] all the same: the WebView's first start may hold the main
 * thread up for seconds, which a wait of its own would fail on.
 */
@RunWith(AndroidJUnit4::class)
class ConnectPageTest {
    /** removeTasks takes the app's tasks out of recents once a case has closed its activity, so that the orchestrator's clear kills nothing of the next test ([Rig.removeTasks]). */
    @After
    fun removeTasks() = Rig.removeTasks()

    @Test
    fun theBridgeAnswersTheConnectPage() {
        ActivityScenario.launch(MainActivity::class.java).use { sc ->
            val page = Page(sc)
            page.waitFor("document.readyState === 'complete' && location.href.endsWith('/ui/connect.html')", "the connect page")
            val got = page.run(
                "const r = await massimo.servers(); return { version: massimo.version, list: Array.isArray(r), token: typeof massimo.token };",
                "the bridge's servers",
            ) as JSONObject
            assertEquals("the bridge's contract version", 7, got.getInt("version"))
            assertTrue("massimo.servers() answers a list", got.getBoolean("list"))
            assertEquals("the connect page's massimo.token", "undefined", got.getString("token"))
        }
    }

    @Test
    fun theConnectPageMayNotAskForAToken() {
        ActivityScenario.launch(MainActivity::class.java).use { sc ->
            val page = Page(sc)
            page.waitFor("document.readyState === 'complete' && typeof massimoChannel === 'object'", "the connect page's channel")
            val got = page.run(
                """
                return await new Promise((resolve) => {
                  massimoChannel.addEventListener("message", (ev) => {
                    const m = JSON.parse(ev.data);
                    if (m.id === 99) resolve(m.error ? "refused" : "answered");
                  });
                  massimoChannel.postMessage(JSON.stringify({ id: 99, call: "token", args: [] }));
                });
                """.trimIndent(),
                "the bridge's answer to a call for the token",
            )
            assertEquals("the bridge's answer to the connect page's call for the token", "refused", got)
        }
    }
}

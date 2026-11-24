// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * One prompt for Forget all (AP MobileApp, Block 17; D63-A): two servers,
 * each with a login kept under the app key and no session held, are
 * forgotten after one prompt, each key revoked at its server by itself
 * (`DELETE /api/v1/me/token`), as their audits say.
 */
@RunWith(AndroidJUnit4::class)
class ForgetAllTest : EmulatorTest() {
    @Test
    fun forgetAllAsksOnceForTwoServers() {
        val ta = Rig.a
        val tb = Rig.b
        val ha = Host(ta)
        val hb = Host(tb)
        val ia = add(ta, A)
        val ib = add(tb, B)
        val ka = keepLogin(ta, A, ia)
        val kb = keepLogin(tb, B, ib)
        page.click("#menu-sign-out-all") // no session held: every key opens only after the prompt
        page.waitFor("conn.servers.length === 2 && conn.servers.every((s) => !s.session)", "signed out of both")
        val ma = ha.mark()
        val mb = hb.mark()
        page.click("#menu-forget-all")
        page.waitFor("!!document.querySelector('#servers-ask .confirm-yes')", "Forget all's question")
        page.click("#servers-ask .confirm-yes")
        Prompt.pass(Prompt.forgetAll())
        page.waitFor("conn.servers.length === 2 && conn.servers.every((s) => !s.login)", "both logins forgotten")
        Prompt.none(NO_MORE_MS)
        for ((h, mark, key) in listOf(Triple(ha, ma, ka), Triple(hb, mb, kb))) {
            val rows = h.since(mark)
            assertTrue(
                "the key $key not revoked by itself: $rows",
                rows.any { it.action == "token.revoke" && it.target == "token:$key" && it.tokenId == key && it.result == "ok" },
            )
        }
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import com.sun.net.httpserver.HttpServer
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.net.InetAddress
import java.net.InetSocketAddress
import java.security.cert.CertificateException
import java.time.Duration
import java.time.Instant
import java.util.concurrent.atomic.AtomicBoolean

/**
 * The revoke codes (AP MobileApp, D112-A, D116-A): a token's code as the
 * server works it out, the vector `internal/store/tokens_test.go` shares;
 * the codes a server's entries keep posted at a contact, taken, dropped or
 * kept by the answer; a removed server's kept in `gone.json`, posted where
 * the probe finds it at its node and nowhere else, and dropped after their
 * time; no code in a log.
 */
class RevokeCodeTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val api = ServerApi("Massimo/test (Android 17; SM-X)", Prober.SystemTrust { _, _, _ -> throw CertificateException("not trusted") })
    private val logged = mutableListOf<String>()
    private lateinit var server: HttpServer
    private lateinit var base: String
    private val posted = mutableListOf<String>() // the codes posted, in their order
    private val answers = HashMap<String, Int>() // by code; 204 for any other

    @Before
    fun start() {
        server = HttpServer.create(InetSocketAddress(InetAddress.getLoopbackAddress(), 0), 0)
        server.createContext("/massimo/api/v1/tokens/revoke") { ex ->
            val code = JSONObject(ex.requestBody.readBytes().toString(Charsets.UTF_8)).optString("code")
            synchronized(posted) { posted.add(code) }
            assertEquals("no token goes with a code", null, ex.requestHeaders.getFirst("Authorization"))
            val status = answers[code] ?: 204
            ex.sendResponseHeaders(status, if (status == 204) -1 else 0)
            ex.close()
        }
        server.start()
        base = "http://127.0.0.1:${server.address.port}/massimo/"
    }

    @After
    fun stop() = server.stop(0)

    @Test
    fun theCodeOfASecretIsTheServers() {
        // the vector of internal/store/tokens_test.go
        assertEquals(
            "1a797b292ec9355f7ac4484c667557d8471cb11c4e47c4321a1328d4eb33cebb",
            RevokeCode.of("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
        )
        assertTrue(RevokeCode.valid(RevokeCode.of("x")))
        assertFalse(RevokeCode.valid("ABC"))
        assertFalse(RevokeCode.valid(RevokeCode.of("x").uppercase()))
    }

    private fun stores(clock: () -> Instant = Instant::now): Triple<ServerStore, GoneStore, Codes> {
        val servers = ServerStore(File(tmp.root, "servers.json"))
        val gone = GoneStore(File(tmp.root, "gone.json"), clock)
        // the probe finds node-a at the fake's address alone
        val probe = { urls: List<String>, _: String? -> urls.map { u -> if (u == base) ProbeResult(u, ok = true, node = "node-a") else ProbeResult(u, ok = false, error = "refused") } }
        return Triple(servers, gone, Codes(servers, gone, api, probe, logged::add))
    }

    @Test
    fun aContactPostsTheServersCodesAndTheAnswerSaysWhatGoes() {
        val (servers, _, codes) = stores()
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        val taken = RevokeCode.of("taken")
        val none = RevokeCode.of("older server")
        val bad = RevokeCode.of("malformed")
        val later = RevokeCode.of("later")
        answers[none] = 404
        answers[bad] = 400
        answers[later] = 503
        servers.leave(s.id, "anna", 42, taken)
        servers.leave(s.id, "anna", 43, none)
        servers.leaveSession(s.id, "anna", 7, "2026-11-20T10:00:07Z", bad)
        servers.leaveSession(s.id, "anna", 8, "2026-11-20T10:00:08Z", later)
        assertFalse("another address takes none", codes.post(s, "http://127.0.0.1:9/"))
        assertTrue(posted.isEmpty())
        assertTrue(codes.post(s, base))
        assertEquals(listOf(taken, none, bad, later), posted)
        val now = servers.get(s.id)!!
        assertTrue("taken: hidden, kept for the next session's check", now.unrevoked[0].let { it.id == 42L && it.taken && it.code.isEmpty() })
        assertTrue("a server that takes none: as before codes", now.unrevoked[1].let { it.id == 43L && !it.taken && it.code.isEmpty() })
        assertEquals("malformed: dropped", "", now.unended[0].code)
        assertEquals("a 5xx: kept for the next contact", later, now.unended[1].code)
        assertEquals(listOf(later), now.codes())
        val page = now.toJson()
        assertEquals("a taken entry not shown", 1, page.getJSONArray("unrevoked").length())
        assertFalse(page.toString().contains(later))
        assertTrue(page.getJSONArray("unended").getJSONObject(1).getBoolean("pending"))
        assertFalse(page.getJSONArray("unended").getJSONObject(0).has("pending"))
        assertTrue(logged.size == 2 && logged.none { l -> listOf(taken, none, bad, later).any { l.contains(it) } })
        // a broken connection keeps the rest for the next contact
        server.stop(0)
        posted.clear()
        assertFalse(codes.post(servers.get(s.id)!!, base))
        assertEquals(later, servers.get(s.id)!!.unended[1].code)
    }

    @Test
    fun nothingIsPostedOnceTheListsExitStopped() {
        // AP MobileApp, Block 19 (D91-A): from Exit on the codes wait for the next start
        val servers = ServerStore(File(tmp.root, "servers.json"))
        val gone = GoneStore(File(tmp.root, "gone.json"))
        val probe = { urls: List<String>, _: String? -> urls.map { u -> ProbeResult(u, ok = u == base, node = "node-a") } }
        val exited = AtomicBoolean(false)
        val codes = Codes(servers, gone, api, probe, logged::add, exited::get)
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        val first = RevokeCode.of("first")
        val second = RevokeCode.of("second")
        servers.leave(s.id, "anna", 42, first)
        servers.leave(s.id, "anna", 43, second)
        exited.set(true)
        assertFalse(codes.post(s, base))
        assertTrue("nothing posted once stopped", posted.isEmpty())
        assertEquals(listOf(first, second), servers.get(s.id)!!.codes())
        gone.add(servers.get(s.id)!!, listOf(GoneCode(first, session = false)))
        codes.postGone()
        assertTrue("nor a removed server's", posted.isEmpty())
        // a stop between two codes: the first posted, the second waits
        exited.set(false)
        server.removeContext("/massimo/api/v1/tokens/revoke")
        server.createContext("/massimo/api/v1/tokens/revoke") { ex ->
            synchronized(posted) { posted.add(JSONObject(ex.requestBody.readBytes().toString(Charsets.UTF_8)).optString("code")) }
            exited.set(true)
            ex.sendResponseHeaders(204, -1)
            ex.close()
        }
        assertTrue(codes.post(servers.get(s.id)!!, base))
        assertEquals(listOf(first), posted)
        assertEquals("the second waits for the next start", listOf(second), servers.get(s.id)!!.codes())
    }

    @Test
    fun aRemovedServersCodesArePostedWhereItIsFoundAtItsNode() {
        val (servers, gone, codes) = stores()
        val code = RevokeCode.of("left")
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", "http://127.0.0.1:9/").put("remote", base))
        servers.leave(s.id, "anna", 42, code)
        // Remove where it does not answer: kept in gone
        val removing = codes.removing(servers.get(s.id)!!, "")
        assertEquals(listOf(GoneCode(code, false)), removing.waiting)
        gone.add(servers.get(s.id)!!, removing.waiting)
        assertTrue(posted.isEmpty())
        assertTrue(File(tmp.root, "gone.json").readText().contains(code))
        // another node at its address: nothing posted
        val other = GoneServer(s.id, "node-b", s.lan, s.remote, s.fp, Instant.now().toString(), listOf(GoneCode(code, false)))
        File(tmp.root, "gone.json").delete()
        GoneStore(File(tmp.root, "gone.json")).add(other.server(), other.codes)
        codes.postGone()
        assertTrue("an address given to another host spends no code", posted.isEmpty())
        assertEquals(1, gone.list().size)
        // its node there: posted and dropped
        File(tmp.root, "gone.json").delete()
        gone.add(servers.get(s.id)!!, removing.waiting)
        codes.postGone()
        assertEquals(listOf(code), posted)
        assertTrue(gone.list().isEmpty())
        assertFalse(File(tmp.root, "gone.json").exists())
    }

    @Test
    fun aRemovedServersCodesGoAfterTheirTime() {
        // D116-A: a session's 30 days after it was left, every code 90 days after the Remove
        var now = Instant.parse("2026-11-28T12:00:00Z")
        val (servers, gone, _) = stores { now }
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        val key = GoneCode(RevokeCode.of("key"), false)
        val session = GoneCode(RevokeCode.of("session"), true, "2026-11-20T12:00:00Z")
        gone.add(s, listOf(key, session))
        assertEquals(listOf(key, session), GoneStore(File(tmp.root, "gone.json")) { now }.list().single().codes)
        now = Instant.parse("2026-12-20T12:00:00Z") // 30 days after the session was left
        assertEquals(listOf(key), gone.list().single().codes)
        now = Instant.parse("2026-11-28T12:00:00Z").plus(Duration.ofDays(90))
        assertTrue(gone.list().isEmpty())
    }

    @Test
    fun removeAnsweringTakesTheCodesThere() {
        val (servers, _, codes) = stores()
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        val code = RevokeCode.of("left")
        val wait = RevokeCode.of("wait")
        answers[wait] = 429
        servers.leave(s.id, "anna", 42, code)
        servers.leaveSession(s.id, "anna", 7, "2026-11-20T10:00:07Z", wait)
        val r = codes.removing(servers.get(s.id)!!, base)
        assertEquals(setOf(code), r.taken)
        assertEquals(listOf(wait), r.waiting.map { it.code })
        assertTrue(r.waiting.single().session)
    }
}

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
 * The revoke codes (AP MobileApp, D112-A, D116-A, D117-A): a token's code
 * as the server works it out, the vector `internal/store/tokens_test.go`
 * shares; the codes a server's entries keep posted at a contact, taken,
 * dropped or kept by the answer; a removed server's kept in `gone.json`
 * with their tokens, posted where the probe finds it at its node, at its
 * own addresses or at a server of the list with its node and that server's
 * pin, and nowhere else, taken back into a server added or saved with its
 * node, never matched without a node, kept where the entry's write
 * fails, and dropped after their time; no code in a log.
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
        assertEquals("with its token", listOf(GoneCode(code, false, user = "anna", id = 42)), removing.waiting)
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

    // the pin each code was posted with, by code (storesIn's revoke): the fake is plain HTTP, where ServerApi holds the
    // certificate to no pin, so the pin is read where Codes hands it over
    private val pins = HashMap<String, String>()

    // stores of their own in [dir], with a probe that finds [answers] (an address and the node there) and records each
    // address it was asked with the pin it was given, and a post that records the pin of each code in [pins]
    private fun storesIn(dir: String, answers: Map<String, String>, asked: MutableList<Pair<String, String?>>): Triple<ServerStore, GoneStore, Codes> {
        val root = tmp.newFolder(dir)
        val servers = ServerStore(File(root, "servers.json"))
        val gone = GoneStore(File(root, "gone.json"))
        val probe = { urls: List<String>, fp: String? ->
            synchronized(asked) { urls.forEach { asked.add(it to fp) } }
            urls.map { u -> answers[u]?.let { ProbeResult(u, ok = true, node = it) } ?: ProbeResult(u, ok = false, error = "refused") }
        }
        val revoke = { url: String, fp: String, code: String ->
            synchronized(pins) { pins[code] = fp }
            api.revokeCode(url, fp, code)
        }
        return Triple(servers, gone, Codes(servers, gone, api, probe, logged::add, revoke = revoke))
    }

    @Test
    fun aRemovedServersCodesArePostedThroughAServerOfTheListWithItsNode() {
        // D117-A: the removed server does not answer at its own addresses; a server of the list with its node does
        val asked = mutableListOf<Pair<String, String?>>()
        val (servers, gone, codes) = storesIn("through", mapOf(base to "node-a"), asked)
        val removedFp = Fingerprint.of(byteArrayOf(1))
        val code = RevokeCode.of("left")
        val old = RevokeCode.of("before the token was kept") // no user nor id
        gone.add(Server("removed", "", "node-a", "http://127.0.0.1:9/", "", removedFp, Server.ASK_START, false), listOf(GoneCode(code, false, user = "anna", id = 42), GoneCode(old, true)))
        val listed = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        val listedFp = Fingerprint.of(byteArrayOf(2))
        servers.trust(listed.id, listedFp)
        codes.postGone()
        assertEquals("posted at the listed server's address", listOf(code, old), posted)
        assertTrue("its own address first, with its pin", asked.first() == ("http://127.0.0.1:9/" to removedFp))
        assertTrue("then the listed server's, with that server's pin: $asked", asked.contains(base to Fingerprint.canonical(listedFp)))
        assertEquals("posted with the listed server's pin, not the removed one's", mapOf(code to Fingerprint.canonical(listedFp), old to Fingerprint.canonical(listedFp)), pins)
        assertTrue("all taken: nothing left", gone.list().isEmpty())
        assertTrue("the listed server's entry as it was", servers.get(listed.id)!!.codes().isEmpty())
    }

    @Test
    fun theCodesComeBackAtAnAddOrASaveWithTheirNode() {
        // D117-A: a server added with a removed one's node takes its codes back into its entry, the removed one dropped
        val asked = mutableListOf<Pair<String, String?>>()
        val (servers, gone, codes) = storesIn("back", emptyMap(), asked)
        val key = GoneCode(RevokeCode.of("key"), false, user = "anna", id = 42)
        val session = GoneCode(RevokeCode.of("session"), true, "2026-11-20T10:00:00Z", "anna", 7, "2026-11-20T09:00:00Z")
        val unknownLeft = GoneCode(RevokeCode.of("session left when"), true, user = "bob", id = 8, createdAt = "2026-11-21T09:00:00Z")
        val old = GoneCode(RevokeCode.of("before the token was kept"), false)
        gone.add(Server("removed", "", "node-a", "http://127.0.0.1:9/", "", "", Server.ASK_START, false), listOf(key, session, unknownLeft, old))
        val removedAt = gone.list().single().removedAt
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        servers.leave(s.id, "anna", 42) // noted already, without its code: it takes the code
        assertTrue(codes.adopt(servers.get(s.id)!!))
        for (now in listOf(servers.get(s.id)!!, ServerStore(File(tmp.root, "back/servers.json")).get(s.id)!!)) {
            assertEquals("the key, with its code", listOf(KeyLeft("anna", 42, key.code)), now.unrevoked)
            assertEquals(key.code, now.unrevoked.single().code)
            assertEquals(listOf(SessionLeft("anna", 7, "2026-11-20T09:00:00Z"), SessionLeft("bob", 8, "2026-11-21T09:00:00Z")), now.unended)
            assertEquals("each with its code", listOf(session.code, unknownLeft.code), now.unended.map { it.code })
            assertEquals("a session left when it was, else at the Remove", listOf("2026-11-20T10:00:00Z", removedAt), now.unended.map { it.leftAt })
            assertEquals("the next contact posts them", listOf(key.code, session.code, unknownLeft.code), now.codes())
        }
        assertEquals("a code without its token stays with the removed server", listOf(old.code), gone.list().single().codes.map { it.code })
        assertFalse(File(tmp.root, "back/gone.json").readText().contains(key.code))
        assertFalse("nothing more to take", codes.adopt(servers.get(s.id)!!))
        assertTrue("nothing posted, nothing probed", posted.isEmpty() && asked.isEmpty())

        // a save of a server of the list whose node becomes a removed one's
        val pinned = Fingerprint.of(byteArrayOf(3))
        gone.add(Server("removed-c", "", "node-c", "http://127.0.0.1:9/", "", pinned, Server.ASK_START, false), listOf(key))
        val other = servers.save(JSONObject().put("name", "Box").put("node", "node-b").put("lan", "http://127.0.0.1:8/"))
        servers.leave(other.id, "carla", 5, RevokeCode.of("node-b's"))
        assertFalse("another node takes nothing", codes.adopt(servers.get(other.id)!!))
        val saved = servers.save(JSONObject().put("id", other.id).put("name", "Box").put("node", "node-c").put("lan", "http://127.0.0.1:8/"))
        assertTrue(codes.adopt(saved))
        assertEquals("node-b's keys gone with it, node-c's taken back", listOf(KeyLeft("anna", 42, key.code)), servers.get(other.id)!!.unrevoked)
        assertEquals(key.code, servers.get(other.id)!!.unrevoked.single().code)
        assertTrue("the removed server dropped", gone.ofNode("node-c").isEmpty())
        assertEquals(1, gone.list().size)
    }

    @Test
    fun aTakeBackWhoseWriteFailsLeavesTheCodesInGone() {
        // D117-A: the server's entry is written before gone.json drops a code, so that a write that fails loses none
        val asked = mutableListOf<Pair<String, String?>>()
        val (servers, gone, codes) = storesIn("unwritable", emptyMap(), asked)
        val key = GoneCode(RevokeCode.of("key"), false, user = "anna", id = 42)
        val session = GoneCode(RevokeCode.of("session"), true, "2026-11-20T10:00:00Z", "anna", 7, "2026-11-20T09:00:00Z")
        gone.add(Server("removed", "", "node-a", "http://127.0.0.1:9/", "", "", Server.ASK_START, false), listOf(key, session))
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        // servers.json cannot be written: the file written beside it and renamed over it is a directory
        val blocked = File(tmp.root, "unwritable/servers.json.new")
        assertTrue(blocked.mkdir())
        val failed = runCatching { codes.adopt(servers.get(s.id)!!) }.exceptionOrNull()
        assertTrue("the write's failure told: $failed", failed is java.io.IOException)
        assertEquals("the codes still in gone.json", listOf(key, session), GoneStore(File(tmp.root, "unwritable/gone.json")).list().single().codes)
        assertTrue("the entry as it was, in memory", servers.get(s.id)!!.let { it.unrevoked.isEmpty() && it.unended.isEmpty() })
        assertTrue("and on disk", ServerStore(File(tmp.root, "unwritable/servers.json")).get(s.id)!!.codes().isEmpty())
        // writable again: the next save takes them back
        assertTrue(blocked.delete())
        assertTrue(codes.adopt(servers.get(s.id)!!))
        assertEquals(listOf(key.code, session.code), servers.get(s.id)!!.codes())
        assertTrue(gone.list().isEmpty())
    }

    @Test
    fun aServerWithoutANodeMatchesNone() {
        // D117-A: a removed server without a node is found at its own addresses alone, and takes no server's entry
        val asked = mutableListOf<Pair<String, String?>>()
        val (servers, gone, codes) = storesIn("none", mapOf(base to "node-a"), asked)
        val code = GoneCode(RevokeCode.of("left"), false, user = "anna", id = 42)
        gone.add(Server("removed", "", "", "http://127.0.0.1:9/", "", "", Server.ASK_START, false), listOf(code))
        val bare = servers.save(JSONObject().put("name", "Bare").put("node", "").put("lan", base))
        val named = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", base))
        codes.postGone()
        assertTrue("not posted through a server of the list: $posted", posted.isEmpty())
        assertEquals("its own address alone probed", listOf("http://127.0.0.1:9/"), asked.map { it.first })
        assertFalse("a server without a node takes nothing back", codes.adopt(bare))
        assertFalse("nor one with a node", codes.adopt(named))
        assertEquals(listOf(code), gone.list().single().codes)

        // a removed server with a node: a server of the list without one, or with another, is no way to it
        val (servers2, gone2, codes2) = storesIn("other", mapOf(base to "node-b"), asked)
        gone2.add(Server("removed", "", "node-a", "http://127.0.0.1:9/", "", "", Server.ASK_START, false), listOf(code))
        val bare2 = servers2.save(JSONObject().put("name", "Bare").put("node", "").put("lan", base))
        val other2 = servers2.save(JSONObject().put("name", "Box").put("node", "node-b").put("lan", base))
        asked.clear()
        codes2.postGone()
        assertTrue("not posted through a server without its node: $posted", posted.isEmpty())
        assertEquals("its own address alone probed", listOf("http://127.0.0.1:9/"), asked.map { it.first })
        assertFalse(codes2.adopt(bare2))
        assertFalse("another node takes nothing back", codes2.adopt(other2))
        assertEquals(listOf(code), gone2.list().single().codes)
        assertTrue(servers2.list().all { it.codes().isEmpty() })
    }

    @Test
    fun aGoneCodeKeepsItsTokenOnDisk() {
        // D117-A: the token beside the code in gone.json; a file without it read as before
        val g = GoneStore(File(tmp.root, "gone.json"))
        val session = GoneCode(RevokeCode.of("session"), true, "2026-11-20T10:00:00Z", "anna", 7, "2026-11-20T09:00:00Z")
        g.add(Server("removed", "", "node-a", "http://127.0.0.1:9/", "", "", Server.ASK_START, false), listOf(session))
        assertEquals(listOf(session), GoneStore(File(tmp.root, "gone.json")).list().single().codes)
        assertFalse("no code in its text", session.toString().contains(session.code))
        val older = """[{"id":"r","node":"node-a","lan":"http://127.0.0.1:9/","remote":"","fp":"","removed_at":"${Instant.now()}",""" +
            """"codes":[{"code":"${RevokeCode.of("old")}","session":false,"left_at":""}]}]"""
        File(tmp.root, "gone.json").writeText(older)
        val read = GoneStore(File(tmp.root, "gone.json")).list().single().codes.single()
        assertEquals(GoneCode(RevokeCode.of("old"), false), read)
        assertFalse(read.takeable())
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

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import com.sun.net.httpserver.HttpExchange
import com.sun.net.httpserver.HttpServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.ServerSocket
import java.security.cert.CertificateException
import java.util.concurrent.atomic.AtomicBoolean

/**
 * The sessions the app holds (docs/app.md, contract version 3) against a
 * fake server: checked with GET /api/v1/me and forgotten on a 401, signed
 * out of one server and of all, and Turn off's revoke, with the session and
 * with the key, a failure logged without any secret.
 */
class SessionsTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val noSystemTrust = Prober.SystemTrust { _, _, _ -> throw CertificateException("not trusted") }
    private val api = ServerApi("Massimo/test (Android 17; SM-X)", noSystemTrust)

    private lateinit var server: HttpServer
    private lateinit var base: String
    private lateinit var closed: String
    private val seen = mutableListOf<String>() // "<method> <path> <bearer>"
    private val logged = mutableListOf<String>()

    // Tokens kept in the clear: the seal is TokenStore's and tested there.
    private val plain = object : Sealer {
        override fun seal(plain: ByteArray, aad: ByteArray): ByteArray = plain.copyOf()

        override fun open(sealed: ByteArray, aad: ByteArray): ByteArray = sealed.copyOf()
    }
    private lateinit var keeper: SessionKeeper
    private lateinit var logins: LoginStore

    private fun answer(ex: HttpExchange, code: Int, body: String) {
        val b = body.toByteArray()
        ex.responseHeaders.add("Content-Type", "application/json")
        ex.sendResponseHeaders(code, b.size.toLong())
        ex.responseBody.use { it.write(b) }
    }

    @Before
    fun start() {
        server = HttpServer.create(InetSocketAddress(InetAddress.getLoopbackAddress(), 0), 0)
        server.createContext("/massimo/api/v1/") { ex ->
            val bearer = ex.requestHeaders.getFirst("Authorization").orEmpty().removePrefix("Bearer ")
            val path = ex.requestURI.rawPath.removePrefix("/massimo")
            synchronized(seen) { seen.add("${ex.requestMethod} $path $bearer") }
            val m = ex.requestMethod
            when {
                m == "GET" && path == "/api/v1/me" && bearer == "session-anna" -> answer(ex, 200, """{"mode":"users","user":{"name":"anna"}}""")
                m == "GET" && path == "/api/v1/me" && bearer == "session-bob" -> answer(ex, 200, """{"mode":"users","user":{"name":"bob"}}""")
                m == "GET" && path == "/api/v1/me" && bearer == "api-key" ->
                    answer(ex, 200, """{"mode":"users","user":{"name":"anna"},"token":{"id":45,"kind":"api"}}""")
                m == "POST" && path == "/api/v1/logout" && bearer in setOf("session-anna", "session-bob") -> answer(ex, 200, """{"ok":true}""")
                m == "POST" && path == "/api/v1/logout" && bearer == "api-key" -> answer(ex, 400, """{"error":"the token is an API key, not a session"}""")
                // key-self revokes itself; key-secret meets a server older than the route, whose mux knows none
                m == "DELETE" && path == "/api/v1/me/token" && bearer == "key-self" -> answer(ex, 200, """{"ok":true,"id":42}""")
                m == "DELETE" && path == "/api/v1/me/token" && bearer == "key-secret" -> answer(ex, 404, "404 page not found")
                m == "DELETE" && path.startsWith("/api/v1/users/anna/tokens/") && bearer == "session-bob" ->
                    answer(ex, 403, """{"error":"capability \"users\" required"}""")
                m == "DELETE" && path.startsWith("/api/v1/users/anna/tokens/") && bearer in setOf("session-anna", "key-secret") ->
                    when (path.substringAfterLast('/')) {
                        "42" -> answer(ex, 200, """{"ok":true,"id":42}""")
                        "43" -> answer(ex, 404, """{"error":"user anna has no token 43"}""")
                        else -> answer(ex, 500, """{"error":"database is locked"}""")
                    }
                else -> answer(ex, 401, """{"error":"missing or wrong token"}""")
            }
        }
        server.start()
        base = "http://127.0.0.1:${server.address.port}/massimo/"
        closed = "http://127.0.0.1:${ServerSocket(0).use { it.localPort }}/"
        keeper = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
        logins = LoginStore(File(tmp.root, "logins.json"))
    }

    @After
    fun stop() = server.stop(0)

    // probe answers as the server at base, node-a; any other address is refused.
    private fun probe(urls: List<String>, fp: String?): List<ProbeResult> =
        urls.map { u -> if (u == base) ProbeResult(u, ok = true, node = "node-a") else ProbeResult(u, ok = false, error = "refused") }

    private fun sessions() = Sessions(keeper, logins, api, ::probe, log = Logger { logged.add(it) })

    private fun home(lan: String = base, remote: String = "", id: String = "s1", node: String = "node-a") =
        Server(id, "Home", node, lan, remote, "", Server.ASK_START, false)

    private fun requests(): List<String> = synchronized(seen) { seen.toList() }

    private fun kept(keyId: Long) = KeptLogin("anna", "node-a", "", byteArrayOf(1), byteArrayOf(2), keyId)

    @Test
    fun aCheckNamesTheUserAndIsRemembered() {
        val st = sessions()
        assertNull("nothing held, nothing asked", st.check(home(), base))
        assertTrue(requests().isEmpty())
        st.put(home(), "session-anna")
        assertNull("not checked yet", st.user(home()))
        assertEquals("anna", st.check(home(), base))
        assertEquals("anna", st.user(home()))
        assertEquals(listOf("GET /api/v1/me session-anna"), requests())
        // an address that does not answer leaves the last check
        assertEquals("anna", st.check(home(lan = closed), closed))
        // an address that is not the server's is not asked
        assertEquals("anna", st.check(home(), closed))
        assertEquals(1, requests().size)
        // a new session holds no check of the one before
        st.put(home(), "session-bob")
        assertNull(st.user(home()))
        assertEquals("bob", st.check(home(), base))
    }

    @Test
    fun whereASessionIsHeld() {
        // what a tap's Opening reads (AP MobileApp, F40): none, sealed without a kept login, in memory with one asked at each start
        val st = sessions()
        assertEquals(Held.NONE, st.where(home()))
        st.put(home(), "session-anna")
        assertEquals(Held.SEALED, st.where(home()))
        logins.put("s1", kept(42))
        keeper.move("s1", true)
        assertEquals(Held.MEMORY, st.where(home()))
        st.put(home(), null)
        assertEquals(Held.NONE, st.where(home()))
    }

    @Test
    fun aSessionTheServerRefusesIsForgotten() {
        val st = sessions()
        st.put(home(), "session-dead")
        assertNull(st.check(home(), base))
        assertNull("forgotten", keeper.get("s1", false))
        assertFalse("and gone from the sealed store", File(tmp.root, "tokens.json").readText().contains("s1"))
        assertNull(st.check(home(), base))
        assertEquals("the next check sends nothing that counts against the address", 1, requests().size)
    }

    @Test
    fun signOutEndsTheSessionAndForgetsIt() {
        val st = sessions()
        st.put(home(), "session-anna")
        st.check(home(), base)
        assertTrue(st.signOut(home(), base))
        assertEquals("POST /api/v1/logout session-anna", requests().last())
        assertNull(st.held(home()))
        assertNull(st.user(home()))
        assertFalse("none held: nothing to end", st.signOut(home(), base))
        assertEquals(2, requests().size)

        // without an address that answers: forgotten on the phone alone
        st.put(home(), "session-anna")
        assertFalse(st.signOut(home(), ""))
        assertNull(st.held(home()))
        assertEquals(2, requests().size)

        // an API key is no session: forgotten, not ended, said in the log
        st.put(home(), "api-key")
        assertFalse(st.signOut(home(), base))
        assertNull(st.held(home()))
        assertTrue(logged.single(), logged.single().contains("status 400"))
        assertFalse(logged.single().contains("api-key"))
    }

    @Test
    fun aTokenPutEndsTheSessionItReplaces() {
        // a session the page's new sign-in replaces ends at the address the page was opened at, once (D68-A)
        val st = sessions()
        st.put(home(), "session-anna")
        val ending = st.put(home(), "session-bob", base)
        assertEquals("the new one held at once", "session-bob", st.held(home()))
        assertTrue("the end is the caller's, after its answer", requests().isEmpty())
        ending!!.end()
        assertEquals(listOf("POST /api/v1/logout session-anna"), requests())
        assertEquals("session-bob", st.held(home()))
        for (none in listOf(st.put(home(), "session-bob", base), st.put(home(), null, base), st.put(home(), "session-anna"), st.put(home(), "session-bob"))) {
            assertNull("the same token, a forgetting and a put without the address end nothing", none)
        }
        assertEquals(1, requests().size)
        assertTrue(logged.isEmpty())
    }

    @Test
    fun twoPutsAtOnceEachEndTheirOwn() {
        // each put reads the session it replaces and keeps its own in one step: none is ended twice, none lost
        val st = sessions()
        st.put(home(), "t0")
        val endings = java.util.concurrent.ConcurrentLinkedQueue<Sessions.Ending>()
        val start = java.util.concurrent.CountDownLatch(1)
        val threads = (1..4).map { n ->
            Thread {
                start.await()
                for (i in 1..50) st.put(home(), "t$n-$i", base)?.let(endings::add)
            }
        }
        threads.forEach { it.start() }
        start.countDown()
        threads.forEach { it.join() }
        endings.forEach { it.end() }
        val all = listOf("t0") + (1..4).flatMap { n -> (1..50).map { "t$n-$it" } }
        val ended = requests().map { it.substringAfterLast(' ') }
        assertEquals("every token replaced ended once", (all - st.held(home())!!).sorted(), ended.sorted())
    }

    @Test
    fun aKeyReplacedIsNotEndedAndItsRefusalLogsNothing() {
        // a key the page held: the server refuses a logout of it with 400, which is no failure
        val st = sessions()
        st.put(home(), "api-key")
        st.put(home(), "session-anna", base)!!.end()
        assertEquals(listOf("POST /api/v1/logout api-key"), requests())
        assertTrue(logged.toString(), logged.isEmpty())
        // a key checked as one is not even asked
        st.put(home(), "api-key")
        st.check(home(), base)
        assertNull("a key checked as one is no session to end", st.put(home(), "session-anna", base))
        assertEquals(listOf("POST /api/v1/logout api-key", "GET /api/v1/me api-key"), requests())
        assertTrue(logged.isEmpty())
    }

    @Test
    fun signOutOfAllEndsEverySessionWhereItAnswers() {
        val st = sessions()
        val a = home()
        val b = home(lan = closed, id = "s2", node = "node-b")
        val c = home(id = "s3")
        st.put(a, "session-anna")
        st.put(b, "session-bob")
        assertEquals(1, st.signOutAll(listOf(a, b, c)))
        assertEquals(listOf("POST /api/v1/logout session-anna"), requests())
        assertNull(st.held(a))
        assertNull("one that does not answer is forgotten all the same", st.held(b))
        assertTrue(logged.single(), logged.single().contains("s2"))
        assertFalse(logged.single().contains("session-bob"))
        assertEquals(0, st.signOutAll(listOf(a, b, c)))
    }

    @Test
    fun turnOffRevokesTheKeyWithTheSessionThenSignsOut() {
        val st = sessions()
        logins.put("s1", kept(42))
        st.put(home(), "session-anna")
        assertTrue("kept at each start: in memory", keeper.inMemory("s1"))
        assertEquals(Sessions.Revoke.REVOKED, st.revokeWithSession(home(), base, kept(42)))
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 session-anna", "POST /api/v1/logout session-anna"), requests())
        assertNull("signed out", st.held(home()))

        // the key gone already counts as revoked
        st.put(home(), "session-anna")
        assertEquals(Sessions.Revoke.REVOKED, st.revokeWithSession(home(), base, kept(43)))
        assertTrue(logged.isEmpty())
    }

    @Test
    fun turnOffFallsBackToTheKeyWhereTheSessionCannot() {
        val st = sessions()
        logins.put("s1", kept(42))
        assertEquals("no session held", Sessions.Revoke.BY_KEY, st.revokeWithSession(home(), base, kept(42)))
        st.put(home(), "session-anna")
        assertEquals("a login kept before its id was", Sessions.Revoke.BY_KEY, st.revokeWithSession(home(), base, kept(0)))
        assertEquals("an address that is not the server's", Sessions.Revoke.BY_KEY, st.revokeWithSession(home(), closed, kept(42)))
        assertTrue(requests().isEmpty())

        st.put(home(), "session-bob")
        assertEquals("another user's session", Sessions.Revoke.BY_KEY, st.revokeWithSession(home(), base, kept(42)))
        st.put(home(), "session-dead")
        assertEquals("a dead session", Sessions.Revoke.BY_KEY, st.revokeWithSession(home(), base, kept(42)))
        assertNull("forgotten", st.held(home()))
        assertTrue(logged.isEmpty())
    }

    @Test
    fun aFailedRevokeIsLoggedWithoutASecret() {
        val st = sessions()
        logins.put("s1", kept(44))
        st.put(home(), "session-anna")
        assertEquals(Sessions.Revoke.FAILED, st.revokeWithSession(home(), base, kept(44)))
        val line = logged.single()
        assertTrue(line, line.contains("s1") && line.contains("status 500") && line.contains("database is locked"))
        assertFalse(line, line.contains("session-anna"))

        assertFalse(st.revokeWithKey(home(), base, "anna", LoginSecret("key-secret", 44)))
        assertFalse(logged.last(), logged.last().contains("key-secret"))
        assertFalse(st.revokeWithKey(home(lan = closed), closed, "anna", LoginSecret("key-secret", 42)))
        assertFalse(logged.last(), logged.last().contains("key-secret"))
        assertEquals(3, logged.size)
    }

    // held runs [test] against the revokes' fake server, with a store of
    // servers and its KeysLeft, a server Home kept there at the fake's
    // address, and its Sessions.
    private fun held(test: (FakeServer, ServerStore, Sessions, Server) -> Unit) = FakeServer().use { fake ->
        val servers = ServerStore(File(tmp.root, "servers.json"))
        val left = KeysLeft(servers, logins, api, logged::add)
        val s = servers.save(org.json.JSONObject().put("name", "Home").put("node", "node-a").put("lan", fake.base))
        test(fake, servers, Sessions(keeper, logins, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, left, logged::add), s)
    }

    @Test
    fun aCheckTheListsExitStoppedWritesAndRevokesNothing() {
        // AP MobileApp, Block 19 (D91-A): a check asked for just before Exit, its answer coming after it,
        // writes no check nor record as held and revokes no key left; once stopped, a check asks nothing
        FakeServer().use { fake ->
            val servers = ServerStore(File(tmp.root, "servers.json"))
            val left = KeysLeft(servers, logins, api, logged::add)
            val s = servers.save(org.json.JSONObject().put("name", "Home").put("node", "node-a").put("lan", fake.base))
            val exited = AtomicBoolean(false)
            val st = Sessions(keeper, logins, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, left, logged::add, Checks(), exited::get)
            st.put(s, "session-anna")
            servers.leave(s.id, "anna", 42) // a key left there, which a check of anna's session revokes
            fake.onRequest = { if (it.startsWith("GET /api/v1/me ")) exited.set(true) }
            assertNull(st.check(s, fake.base))
            assertNull("no record as held", servers.get(s.id)!!.held)
            assertNull("no check kept", st.user(s))
            assertEquals("the key left still left", 1, servers.get(s.id)!!.unrevoked.size)
            assertEquals("nothing but the check's own request: ${fake.requests()}", 1, fake.requests().size)
            assertNull(st.check(s, fake.base))
            assertEquals("stopped: nothing asked", 1, fake.requests().size)
        }
    }

    @Test
    fun aSessionCheckedIsRecordedAsHeldUntilItEnds() {
        // D102-A: its user, id and time once a check learns them, no secret; cleared where it ends, is left or replaced
        held { fake, servers, st, s ->
            st.put(s, "session-anna")
            assertNull("not checked yet", servers.get(s.id)!!.held)
            st.check(s, fake.base)
            assertEquals(HeldSession("anna", 7, FakeServer.made(7)), servers.get(s.id)!!.held)
            assertFalse("never to the page", servers.get(s.id)!!.toJson().has("held"))
            assertFalse("no secret in the file", File(tmp.root, "servers.json").readText().contains("session-anna"))
            assertTrue(st.signOut(s, fake.base))
            assertNull("ended: cleared", servers.get(s.id)!!.held)
            st.put(s, "session-anna")
            st.check(s, fake.base)
            assertFalse(st.signOut(s, ""))
            assertNull("left: cleared", servers.get(s.id)!!.held)
            assertEquals(listOf(SessionLeft("anna", 7, FakeServer.made(7))), servers.get(s.id)!!.unended)
            servers.ended(s.id, servers.get(s.id)!!.unended)
            st.put(s, "session-anna")
            st.check(s, fake.base)
            st.put(s, "session-bob")
            assertNull("replaced: cleared", servers.get(s.id)!!.held)
            st.check(s, fake.base)
            assertEquals(9L, servers.get(s.id)!!.held!!.id)
            st.put(s, null)
            assertNull("forgotten: cleared", servers.get(s.id)!!.held)
        }
    }

    @Test
    fun aSessionInMemoryGoesIntoUnendedAtTheNextProcess() {
        // F47: the process ends with the session in memory; the next one's start leaves it, a sealed one still held stays
        held { fake, servers, st, s ->
            logins.put(s.id, kept(42))
            val sealed = servers.save(org.json.JSONObject().put("name", "Cabin").put("node", "node-a").put("lan", fake.base))
            st.put(s, "session-anna")
            st.check(s, fake.base)
            assertTrue("kept at each start: in memory", keeper.inMemory(s.id))
            st.put(sealed, "session-bob")
            st.check(sealed, fake.base)
            // the next process: a new keeper over the same file, its memory empty
            val next = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
            next.leaveForgotten(servers, logins)
            assertNull(servers.get(s.id)!!.held)
            assertEquals(listOf(SessionLeft("anna", 7, FakeServer.made(7))), servers.get(s.id)!!.unended)
            assertEquals("a sealed session is still held", 9L, servers.get(sealed.id)!!.held!!.id)
            assertTrue(servers.get(sealed.id)!!.unended.isEmpty())
            assertEquals("once", 1, ServerStore(File(tmp.root, "servers.json")).get(s.id)!!.unended.size)
            next.leaveForgotten(servers, logins)
            assertEquals(1, servers.get(s.id)!!.unended.size)
        }
    }

    @Test
    fun aKeyTypedIntoTheCardIsNeverRecordedNorLeft() {
        // the card's "API key" way: the page's token, of kind api; no held, nothing in unended after a new process or Exit
        held { fake, servers, st, s ->
            logins.put(s.id, kept(42))
            st.put(s, "key-own")
            assertEquals("anna", st.check(s, fake.base))
            assertNull(servers.get(s.id)!!.held)
            val next = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
            next.leaveForgotten(servers, logins)
            keeper.dropMemory()
            keeper.leaveForgotten(servers, logins)
            assertTrue(servers.get(s.id)!!.unended.isEmpty())
        }
    }

    @Test
    fun oneHolderOverTwoActivitiesKeepsWhatWasChecked() {
        // D102-A: the activity made anew (dark mode, language, Appearance) takes the process's holders
        held { fake, servers, _, s ->
            val checks = Checks()
            val left = KeysLeft(servers, logins, api, logged::add)
            val first = Sessions(keeper, logins, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, left, logged::add, checks)
            first.put(s, "session-anna")
            first.check(s, fake.base)
            val second = Sessions(keeper, logins, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, left, logged::add, checks)
            assertEquals("the new activity's Sessions knows the user", "anna", second.user(s))
            assertEquals("session-anna", second.held(s))
            assertFalse(second.signOut(s, ""))
            assertEquals("and leaves the session by the id the first one checked", listOf(SessionLeft("anna", 7, FakeServer.made(7))), servers.get(s.id)!!.unended)
        }
    }

    @Test
    fun aSessionTheSameKeyReplacesIsNotLoggedOut() {
        // F41, D100-A: POST /api/v1/login/key ended the key's earlier session itself (D31-A)
        held { fake, servers, st, s ->
            assertNull(st.put(s, "session-anna-fp", fake.base, keyId = 42))
            st.check(s, fake.base)
            assertNull("made from the same key: no logout, nothing left", st.put(s, "session-anna", fake.base, keyId = 42))
            assertTrue(servers.get(s.id)!!.unended.isEmpty())
            // not checked yet, the key known from its making all the same
            assertNull(st.put(s, "session-anna-expiring", fake.base, keyId = 42))
            // a password session replaced is logged out as before
            st.put(s, "session-bob", fake.base)!!.end()
            assertEquals("POST /api/v1/logout session-anna-expiring", fake.requests().last())
            assertTrue(fake.requests().none { it == "POST /api/v1/logout session-anna-fp" || it == "POST /api/v1/logout session-anna" })
        }
    }

    @Test
    fun turnOffForgetsASessionTheKeyEndedWithoutALogout() {
        // F41, D100-A: a session made from the key revoked ended with it
        held { fake, servers, st, s ->
            logins.put(s.id, kept(42))
            st.put(s, "session-anna-fp", fake.base, keyId = 42)
            st.check(s, fake.base)
            assertEquals(Sessions.Revoke.REVOKED, st.revokeWithSession(s, fake.base, kept(42)))
            assertEquals("DELETE /api/v1/users/anna/tokens/42 session-anna-fp", fake.requests().last())
            assertTrue("no logout of a session that ended", fake.requests().none { it.startsWith("POST /api/v1/logout") })
            assertNull(st.held(s))
            assertNull(servers.get(s.id)!!.held)
            assertTrue(servers.get(s.id)!!.unended.isEmpty())
        }
    }

    @Test
    fun aCheckLogsWhatItSwallowsWithoutASecret() {
        // D102-A: a server's failure and a broken connection, as end writes them
        val st = sessions()
        st.put(home(lan = closed), "session-anna")
        st.check(home(lan = closed), closed)
        assertTrue(logged.single(), logged.single().contains("s1") && logged.single().contains("not checked"))
        held { fake, _, sf, s ->
            sf.put(s, "no-such-session")
            sf.check(s, fake.base) // refused: forgotten, no failure
            assertEquals(1, logged.size)
            // the server's failure (500), as end writes one
            sf.put(s, "session-anna-broken")
            assertNull(sf.check(s, fake.base))
            assertEquals(2, logged.size)
            assertTrue(logged.last(), logged.last().contains(s.id) && logged.last().contains("not checked") && logged.last().contains("status 500"))
        }
        assertTrue(logged.toString(), logged.none { it.contains("session-") })
    }

    @Test
    fun aSealedSessionNoCheckOfThisProcessSawIsLeftByItsRecord() {
        // the review: after a restart Checks is empty; the record as held stands in for the check
        held { fake, servers, st, s ->
            fun restart(): Sessions {
                val k = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
                return Sessions(k, logins, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, KeysLeft(servers, logins, api, logged::add), logged::add, Checks())
            }
            val left7 = listOf(SessionLeft("anna", 7, FakeServer.made(7)))
            st.put(s, "session-anna")
            st.check(s, fake.base)
            // Sign out where the server does not answer
            assertFalse(restart().signOut(s, ""))
            assertEquals(left7, servers.get(s.id)!!.unended)
            servers.ended(s.id, left7)
            // a session the page's new one replaces, before its ending runs
            st.put(s, "session-anna")
            st.check(s, fake.base)
            val ending = restart().put(s, "session-bob", fake.base)
            assertEquals("left by its record until its ending", left7, servers.get(s.id)!!.unended)
            ending!!.end()
            assertTrue("and taken away once ended", servers.get(s.id)!!.unended.isEmpty())
            // Sign out of all, the server not answering (this probe answers nowhere)
            st.put(s, "session-anna")
            st.check(s, fake.base)
            assertEquals(0, restart().signOutAll(listOf(servers.get(s.id)!!)))
            assertEquals(left7, servers.get(s.id)!!.unended)
            assertNull(servers.get(s.id)!!.held)
        }
    }

    @Test
    fun aSessionFromTheKeySealedBeforeARestartIsNotLoggedOut() {
        // the re-review: after a restart the record as held keeps the key a session was made from (D100-A)
        FakeServer().use { fake ->
            val servers = ServerStore(File(tmp.root, "servers.json"))
            val left = KeysLeft(servers, logins, api, logged::add)
            val none = { urls: List<String>, _: String? -> urls.map { ProbeResult(it, ok = false) } }
            // asked after a logout: the session sealed, as without a kept login
            val s = servers.save(org.json.JSONObject().put("name", "Barn").put("node", "node-a").put("lan", fake.base).put("ask", Server.ASK_LOGOUT))
            logins.put(s.id, kept(42))
            fun restart() = Sessions(SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain)), logins, api, none, left, logged::add, Checks())
            val first = Sessions(keeper, logins, api, none, left, logged::add, Checks())
            first.put(s, "session-anna-fp", fake.base, keyId = 42)
            first.check(s, fake.base)
            assertEquals("the key kept with the record", 42L, servers.get(s.id)!!.held!!.keyId)
            assertEquals(42L, ServerStore(File(tmp.root, "servers.json")).get(s.id)!!.held!!.keyId)

            // the key's next session replaces it: the server ended it, no logout
            assertNull(restart().put(s, "session-anna", fake.base, keyId = 42))
            assertTrue(fake.requests().none { it.startsWith("POST /api/v1/logout") })
            assertTrue(servers.get(s.id)!!.unended.isEmpty())

            // Turn off revokes the key: the session made from it ended with it, no logout
            first.put(s, "session-anna-fp", fake.base, keyId = 42)
            first.check(s, fake.base)
            assertEquals(Sessions.Revoke.REVOKED, restart().revokeWithSession(s, fake.base, kept(42)))
            assertEquals("DELETE /api/v1/users/anna/tokens/42 session-anna-fp", fake.requests().last())
            assertTrue(fake.requests().none { it.startsWith("POST /api/v1/logout") })
            assertTrue(servers.get(s.id)!!.unended.isEmpty())
        }
    }

    @Test
    fun aSignOutLeavesTheSessionRecordedWhileTheServerIsAsked() {
        // the review: an app that ends during the logout leaves the session in unended, not unrecorded
        held { fake, servers, st, s ->
            st.put(s, "session-anna")
            st.check(s, fake.base)
            var during: List<SessionLeft>? = null
            fake.onRequest = { line -> if (line == "POST /api/v1/logout session-anna") during = ServerStore(File(tmp.root, "servers.json")).get(s.id)!!.unended }
            assertTrue(st.signOut(s, fake.base))
            assertEquals("in the file while the server was asked", listOf(SessionLeft("anna", 7, FakeServer.made(7))), during)
            assertTrue("gone once it ended", servers.get(s.id)!!.unended.isEmpty())
        }
    }

    @Test
    fun turnOffWithTheKeyItself() {
        val st = sessions()
        assertTrue(st.revokeWithKey(home(), base, "anna", LoginSecret("key-self", 42)))
        assertEquals("the key revokes itself (D80-B)", "DELETE /api/v1/me/token key-self", requests().single())
        assertTrue(st.revokeWithKey(home(), base, "anna", LoginSecret("key-secret", 42)))
        assertEquals(
            "a server older than the route: by the user and the id",
            listOf("DELETE /api/v1/me/token key-secret", "DELETE /api/v1/users/anna/tokens/42 key-secret"),
            requests().drop(1),
        )
        assertTrue("a key the server takes no more is gone there", st.revokeWithKey(home(), base, "anna", LoginSecret("key-revoked", 42)))
        assertTrue(logged.isEmpty())
    }
}

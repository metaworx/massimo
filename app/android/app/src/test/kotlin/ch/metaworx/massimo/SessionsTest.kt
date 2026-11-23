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

    private fun sessions() = Sessions(keeper, logins, api, ::probe) { logged.add(it) }

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

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.security.cert.CertificateException
import java.util.concurrent.Executor
import java.util.concurrent.Executors

/**
 * Turn off, Remove's turn-off and Forget all stored credentials (AP
 * MobileApp, F26, F28, D56-A, D57-A) against a fake server: each key revoked
 * with the session the app holds, then that session signed out; else with
 * the key itself after one prompt; never over plain HTTP outside the
 * private ranges; the session signed out before the login is forgotten, so
 * that one kept in memory is ended at the server; a login kept without its
 * key's id revoked with the session all the same; a key not revoked left by
 * its id, or forgotten on the phone alone where none is known; Forget all
 * one server after the other, one prompt at a time, its summary naming
 * each kind, and stopping quietly where its executor takes nothing more;
 * an address that answers looked for where none is given; a login whose
 * certificate changed leaving its key by its id. The Bridge's wiring of
 * it (the prompt and its Keystore key, the answers' shapes) is checked by
 * reading it and on a phone.
 */
class RevokerTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val api = ServerApi("Massimo/test (Android 17; SM-X)", Prober.SystemTrust { _, _, _ -> throw CertificateException("not trusted") })
    private val plain = object : Sealer {
        override fun seal(plain: ByteArray, aad: ByteArray): ByteArray = plain.copyOf()

        override fun open(sealed: ByteArray, aad: ByteArray): ByteArray = sealed.copyOf()
    }
    private val logged = mutableListOf<String>()
    private val forgotten = mutableListOf<String>()
    private val prompts = mutableListOf<String>() // by server name
    private lateinit var fake: FakeServer
    private lateinit var servers: ServerStore
    private lateinit var logins: LoginStore
    private lateinit var sessions: Sessions
    private lateinit var revoker: Revoker

    // The probe finds each server at its LAN address where that is not the
    // closed one, with the server's node.
    private val nodes = HashMap<String, String>()

    @Before
    fun start() {
        fake = FakeServer()
        servers = ServerStore(File(tmp.root, "servers.json"))
        logins = LoginStore(File(tmp.root, "logins.json"))
        val keeper = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
        val left = KeysLeft(servers, logins, api, logged::add)
        val probe = { urls: List<String>, _: String? ->
            urls.map { u -> if (u != fake.closed && u in nodes) ProbeResult(u, ok = true, node = nodes.getValue(u)) else ProbeResult(u, ok = false, error = "refused") }
        }
        sessions = Sessions(keeper, logins, api, probe, left, logged::add)
        revoker = Revoker(servers, logins, sessions, left, probe, Executor { it.run() }) { id ->
            logins.forget(id)
            forgotten.add(id)
        }
    }

    @After
    fun stop() = fake.close()

    private fun server(name: String, lan: String = fake.base, node: String = "node-$name"): Server {
        nodes[lan] = node
        return servers.save(JSONObject().put("name", name).put("node", node).put("lan", lan))
    }

    private fun keep(s: Server, keyId: Long, user: String = "anna") = logins.put(s.id, KeptLogin(user, s.node, "", byteArrayOf(1), byteArrayOf(2), keyId))

    // byKey passes the one prompt with [secret], or dismisses it for null.
    private fun byKey(secret: LoginSecret?) = Revoker.ByKey { s, _, then ->
        prompts.add(s.name)
        then(secret)
    }

    // OnePrompt holds a prompt open until the test answers it, and fails
    // where a second shows before the first is answered.
    private inner class OnePrompt(private vararg val answers: LoginSecret?) : Revoker.ByKey {
        private var waiting: ((LoginSecret?) -> Unit)? = null
        private var next = 0

        val showing: Boolean get() = waiting != null

        override fun open(s: Server, kept: KeptLogin, then: (LoginSecret?) -> Unit) {
            assertNull("a second prompt while ${prompts.lastOrNull()}'s shows", waiting)
            prompts.add(s.name)
            waiting = then
        }

        fun answerAll() {
            while (showing) {
                val then = waiting!!
                waiting = null
                then(answers[next++])
            }
        }
    }

    private fun turnOff(s: Server, url: String, secret: LoginSecret? = null): Revoker.Off {
        var got: Revoker.Off? = null
        revoker.turnOff(s.id, url, byKey(secret)) { got = it }
        return got ?: throw AssertionError("turnOff did not answer")
    }

    private fun forgetAll(byKey: Revoker.ByKey): Revoker.Summary {
        var got: Revoker.Summary? = null
        revoker.forgetAll(byKey) { got = it }
        if (byKey is OnePrompt) byKey.answerAll()
        return got ?: throw AssertionError("forgetAll did not answer")
    }

    private fun left(s: Server): List<KeyLeft> = servers.get(s.id)?.unrevoked ?: emptyList()

    private val revoked = Revoker.Outcome.REVOKED
    private val leftHere = Revoker.Outcome.LEFT
    private val local = Revoker.Outcome.LOCAL

    @Test
    fun turnOffWithASessionRevokesWithItThenSignsOut() {
        val s = server("home")
        keep(s, 42)
        sessions.put(s, "session-anna")
        assertTrue("kept in memory, as a login asked at the start keeps it", sessions.inMemory(s))
        assertEquals(Revoker.Off(revoked, 42), turnOff(s, fake.base))
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 session-anna", "POST /api/v1/logout session-anna"), fake.requests())
        assertTrue("no prompt", prompts.isEmpty())
        assertNull(sessions.held(s))
        assertNull(logins.get(s.id))
        assertEquals(listOf(s.id), forgotten)
        assertTrue(left(s).isEmpty())
    }

    @Test
    fun aLoginKeptWithoutItsKeysIdIsRevokedWithTheSessionMadeFromIt() {
        val s = server("home")
        keep(s, 0)
        sessions.put(s, "session-anna-fp")
        assertEquals(Revoker.Off(revoked, 42), turnOff(s, fake.base))
        assertEquals(
            listOf("GET /api/v1/me session-anna-fp", "DELETE /api/v1/users/anna/tokens/42 session-anna-fp", "POST /api/v1/logout session-anna-fp"),
            fake.requests(),
        )
        assertTrue("no prompt", prompts.isEmpty())
    }

    @Test
    fun withoutASessionTheKeyItselfAfterThePrompt() {
        val s = server("home")
        keep(s, 42)
        assertEquals(Revoker.Off(revoked, 42), turnOff(s, fake.base, LoginSecret("key-secret", 42)))
        assertEquals(listOf("home"), prompts)
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 key-secret"), fake.requests())
        assertNull(logins.get(s.id))
        assertTrue(left(s).isEmpty())
    }

    @Test
    fun aSessionKeptInMemoryIsSignedOutAfterTheKeysWay() {
        // Bob's session may not revoke Anna's key: the key's way, then
        // Bob's session, kept in memory for the login, is ended at the
        // server before the login is forgotten (F28).
        val s = server("home")
        keep(s, 45)
        sessions.put(s, "session-bob")
        assertTrue(sessions.inMemory(s))
        assertEquals(Revoker.Off(revoked, 45), turnOff(s, fake.base, LoginSecret("key-secret", 45)))
        assertEquals(
            listOf(
                "DELETE /api/v1/users/anna/tokens/45 session-bob",
                "DELETE /api/v1/users/anna/tokens/45 key-secret",
                "POST /api/v1/logout session-bob",
            ),
            fake.requests(),
        )
        assertNull(sessions.held(s))
    }

    @Test
    fun aKeyNotRevokedIsLeftByItsId() {
        // the prompt dismissed (or the Keystore key not opening, which the
        // Bridge's ByKey answers alike)
        val a = server("home")
        keep(a, 42)
        assertEquals(Revoker.Off(leftHere, 42), turnOff(a, fake.base, null))
        assertEquals(listOf(KeyLeft("anna", 42)), left(a))
        assertNull("forgotten on the phone all the same", logins.get(a.id))

        // the server not reached: no prompt
        val b = server("cabin", lan = fake.closed)
        keep(b, 46)
        assertEquals(Revoker.Off(leftHere, 46), turnOff(b, ""))
        assertEquals(listOf(KeyLeft("anna", 46)), left(b))

        // plain HTTP outside the private ranges: no prompt, no key sent
        val c = server("office", lan = "http://203.0.113.5:8099/")
        keep(c, 45)
        assertEquals(Revoker.Off(leftHere, 45), turnOff(c, "http://203.0.113.5:8099/", LoginSecret("key-secret", 45)))
        assertEquals(listOf(KeyLeft("anna", 45)), left(c))

        // the revoke failing (500), with the session
        val d = server("barn")
        keep(d, 44)
        sessions.put(d, "session-anna")
        assertEquals(Revoker.Off(leftHere, 44), turnOff(d, fake.base))
        assertEquals(listOf(KeyLeft("anna", 44)), left(d))

        assertEquals("one prompt, for the first", listOf("home"), prompts)
        assertTrue(logged.joinToString("\n"), logged.none { it.contains("session-") || it.contains("key-secret") })
    }

    @Test
    fun withoutAnAddressOneThatAnswersIsLookedFor() {
        val s = server("home")
        keep(s, 42)
        sessions.put(s, "session-anna")
        assertEquals("the server probed, its LAN address answering", Revoker.Off(revoked, 42), turnOff(s, ""))
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 session-anna", "POST /api/v1/logout session-anna"), fake.requests())
    }

    @Test
    fun aCertificateTrustedAnewLeavesTheKeyByItsId() {
        val s = server("home")
        keep(s, 46)
        servers.trust(s.id, Fingerprint.of(byteArrayOf(7)))
        assertEquals(Revoker.Off(leftHere, 46), turnOff(s, fake.base))
        assertEquals(listOf(KeyLeft("anna", 46)), left(s))
        assertNull(logins.get(s.id))
        val t = server("cabin")
        keep(t, 47)
        servers.trust(t.id, Fingerprint.of(byteArrayOf(8)))
        assertEquals(Revoker.Summary(listOf(), listOf(t.id), listOf()), forgetAll(OnePrompt()))
        assertEquals(listOf(KeyLeft("anna", 47)), left(t))
        assertTrue("nothing asked of a server whose certificate is not the one kept", fake.requests().isEmpty() && prompts.isEmpty())
    }

    @Test
    fun forgetAllStopsQuietlyWhereItsExecutorTakesNothingMore() {
        // the activity went, its executor shut down: the first server is
        // done, the next one waits for nothing, and nothing is thrown
        val gone = Executors.newSingleThreadExecutor().apply { shutdownNow() }
        val stopped = Revoker(servers, logins, sessions, KeysLeft(servers, logins, api, logged::add), { urls, _ ->
            urls.map { ProbeResult(it, ok = false, error = "refused") }
        }, gone) { id -> logins.forget(id) }
        val a = server("home", lan = fake.closed)
        keep(a, 42)
        val b = server("cabin", lan = fake.closed)
        keep(b, 46)
        var summary: Revoker.Summary? = null
        stopped.forgetAll(OnePrompt()) { summary = it }
        assertNull("no answer: the page waiting for it is gone", summary)
        assertNull("the first one done", logins.get(a.id))
        assertEquals("the next one as it was", 46L, logins.get(b.id)!!.keyId)
    }

    @Test
    fun aKeyWithoutAnIdIsForgottenOnThePhoneAlone() {
        // a login kept before its id was, its server not reached: nothing to
        // leave by, and the summary says so
        val a = server("home", lan = fake.closed)
        keep(a, 0)
        assertEquals(Revoker.Off(local, 0), turnOff(a, ""))
        assertTrue(left(a).isEmpty())
        // a login that is not the server's any more
        val b = server("cabin")
        keep(b, 46)
        servers.save(JSONObject().put("id", b.id).put("name", "cabin").put("node", "node-other").put("lan", fake.base))
        assertEquals(Revoker.Off(local, 0), turnOff(b, fake.base))
        assertTrue(left(b).isEmpty())
        assertNull(logins.get(b.id))
        assertTrue(prompts.isEmpty())
    }

    @Test
    fun aLoginKeptBeforeItsIdIsLeftByTheIdItsKeyCarries() {
        val s = server("home")
        keep(s, 0)
        assertEquals(Revoker.Off(leftHere, 44), turnOff(s, fake.base, LoginSecret("key-secret", 44)))
        assertEquals("the revoke failed: left by the id the opened key carries", listOf(KeyLeft("anna", 44)), left(s))
    }

    @Test
    fun forgetAllNamesEachKindInTheListsOrder() {
        val home = server("home")
        keep(home, 42)
        sessions.put(home, "session-anna")
        val cabin = server("cabin", lan = fake.closed)
        keep(cabin, 46)
        val barn = server("barn", lan = fake.closed)
        keep(barn, 47)
        servers.save(JSONObject().put("id", barn.id).put("name", "barn").put("node", "node-other").put("lan", fake.closed))
        val shed = server("shed", lan = fake.closed)
        keep(shed, 0)
        assertEquals(Revoker.Summary(listOf(home.id), listOf(cabin.id), listOf(barn.id, shed.id)), forgetAll(OnePrompt()))
        assertEquals("Home by its session; the others not reached, so no prompt", listOf<String>(), prompts)
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 session-anna", "POST /api/v1/logout session-anna"), fake.requests())
        assertTrue("every login forgotten", logins.ids().isEmpty())
        assertEquals("a key left only where its id is known", listOf(KeyLeft("anna", 46)), left(cabin))
        assertTrue("a login that is not the server's any more leaves nothing there", left(barn).isEmpty())
        assertTrue(left(shed).isEmpty())
    }

    @Test
    fun forgetAllAsksOneServerAtATime() {
        // four servers at one address, as names of one server would be
        val a = server("home", node = "node-shared")
        keep(a, 42)
        val b = server("cabin", node = "node-shared")
        keep(b, 45)
        val c = server("barn", node = "node-shared")
        keep(c, 0)
        val d = server("shed", node = "node-shared")
        keep(d, 46)
        val p = OnePrompt(LoginSecret("key-secret", 42), null, null, LoginSecret("key-secret", 46))
        val summary = forgetAll(p)
        assertEquals("one prompt per server, each after the last was answered", listOf("home", "cabin", "barn", "shed"), prompts)
        assertEquals("a dismissed prompt leaves its key by its id, or on the phone alone without one, and the next is asked",
            Revoker.Summary(listOf(a.id, d.id), listOf(b.id), listOf(c.id)), summary)
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 key-secret", "DELETE /api/v1/users/anna/tokens/46 key-secret"), fake.requests())
        assertEquals(listOf(KeyLeft("anna", 45)), left(b))
        assertTrue(logins.ids().isEmpty())
    }

    @Test
    fun forgetAllTakesTheKeyWhereTheSessionIsRefused() {
        val s = server("home")
        keep(s, 42)
        sessions.put(s, "session-dead")
        assertEquals(Revoker.Summary(listOf(s.id), listOf(), listOf()), forgetAll(OnePrompt(LoginSecret("key-secret", 42))))
        assertEquals(listOf("home"), prompts)
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 session-dead", "DELETE /api/v1/users/anna/tokens/42 key-secret"), fake.requests())
        assertNull("the refused session forgotten", sessions.held(s))
    }
}

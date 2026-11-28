// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
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
 * certificate changed leaving its key by its id. With the app key (AP
 * MobileApp v1.4, Block 17, D63-A): Forget all over three servers that
 * need the key asks once, under its own title, and once more for each
 * login kept before it, and no more once that prompt was dismissed; Turn
 * off asks once; the app key dropped at the run's end, where the run
 * stops, and where a step throws; a key the phone took back leaving every
 * key by its id; a page's id `app` reaching no Keystore key. Remove
 * signs the session out before the server goes, where no login is kept
 * (F38, D69-A), and names the sessions left valid there; a server down
 * leaves the session in its `unended` (D70-A). The Bridge's wiring of it
 * (the prompt on the main thread, the answers' shapes) is checked by
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
    private lateinit var left: KeysLeft
    private lateinit var appKey: AppKey
    private val keys = JvmKeys()
    private val prompt = FakePrompt(keys)

    // The probe finds each server at its LAN address where that is not the
    // closed one, with the server's node.
    private val nodes = HashMap<String, String>()

    @Before
    fun start() {
        fake = FakeServer()
        servers = ServerStore(File(tmp.root, "servers.json"))
        logins = LoginStore(File(tmp.root, "logins.json"))
        val keeper = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
        left = KeysLeft(servers, logins, api, logged::add)
        val probe = { urls: List<String>, _: String? ->
            urls.map { u -> if (u != fake.closed && u in nodes) ProbeResult(u, ok = true, node = nodes.getValue(u)) else ProbeResult(u, ok = false, error = "refused") }
        }
        sessions = Sessions(keeper, logins, api, probe, left, logged::add)
        // As the Bridge's forgetLogin: the login, and its Keystore key.
        revoker = Revoker(servers, logins, sessions, left, probe, Executor { it.run() }) { id ->
            logins.forget(id)
            runCatching { keys.delete(LoginKey.alias(id)) }
            forgotten.add(id)
        }
        // As the Bridge's: a key taken back forgets each login under it,
        // its key left by its id where the server is the same node.
        appKey = AppKey(logins, keys, prompt, { id, kept ->
            servers.get(id)?.let { if (kept.node == it.node) left.add(id, kept.user, kept.keyId) }
            logins.forget(id)
        })
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

    // keepUnderAppKey keeps a login of each server under the app key, its
    // key [secret], in one step, as the app keeps one now.
    private fun keepUnderAppKey(vararg kept: Pair<Server, LoginSecret>) = appKey.step().use { step ->
        for ((s, secret) in kept) step.keep(s.name, s.id, s.node, s.fp, "anna", secret) { assertEquals(AppKey.Result.Kept, it) }
    }

    // keepBefore keeps a login of [s] as the app did before the app key,
    // sealed by a Keystore key of its own.
    private fun keepBefore(s: Server, secret: LoginSecret) =
        logins.put(s.id, LoginStore.seal(keys.encrypter(LoginKey.alias(s.id)), secret, s.id, s.node, s.fp, "anna"))

    // appByKey is the ByKey of one step of the app key, as the Bridge gives
    // it to a run: each prompt titled with its server's name, but Forget
    // all's one for the app key.
    private fun appByKey(forgetAll: Boolean = false): Revoker.ByKey =
        Revoker.byKey(appKey.step(), { s, kept -> if (forgetAll && kept.byApp) "Forget all" else s.name }, logged::add)

    private val revoked = Revoker.Outcome.REVOKED
    private val leftHere = Revoker.Outcome.LEFT
    private val local = Revoker.Outcome.LOCAL

    // byKeyAt is what the key key-secret's own revoke of the key [id] sends
    // at the fake, a server older than DELETE /api/v1/me/token: that route,
    // which it does not know, then the key's by its user and id (D80-B).
    private fun byKeyAt(id: Long) = listOf("DELETE /api/v1/me/token key-secret", "DELETE /api/v1/users/anna/tokens/$id key-secret")

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
        assertEquals(byKeyAt(42), fake.requests())
        assertNull(logins.get(s.id))
        assertTrue(left(s).isEmpty())
    }

    @Test
    fun theKeyRevokesItselfWithoutItsUserOrId() {
        // a server with DELETE /api/v1/me/token: one request, whatever id the login keeps (D80-B)
        val s = server("home")
        keep(s, 46)
        assertEquals(Revoker.Off(revoked, 46), turnOff(s, fake.base, LoginSecret("key-self-46", 46)))
        assertEquals(listOf("DELETE /api/v1/me/token key-self-46"), fake.requests())
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
                *byKeyAt(45).toTypedArray(),
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

    private fun remove(s: Server, url: String, byKey: Revoker.ByKey = byKey(null)): Revoker.Removed {
        var got: Result<Revoker.Removed>? = null
        revoker.remove(s.id, url, byKey) { got = it }
        return (got ?: throw AssertionError("remove did not answer")).getOrThrow()
    }

    @Test
    fun removeSignsTheSessionOutBeforeTheServerGoes() {
        // without a kept login: signed out at the address given, then forgotten (F38, D69-A)
        val s = server("home")
        sessions.put(s, "session-anna")
        assertEquals(Revoker.Removed(null, emptyList(), emptyList()), remove(s, fake.base))
        assertEquals(listOf("POST /api/v1/logout session-anna"), fake.requests())
        assertNull(servers.get(s.id))
        assertNull(sessions.held(s))
        // without an address: at the one that answers
        val t = server("cabin")
        sessions.put(t, "session-bob")
        remove(t, "")
        assertEquals("POST /api/v1/logout session-bob", fake.requests().last())
        // none held: nothing sent
        val u = server("barn")
        remove(u, fake.base)
        assertEquals(2, fake.requests().size)
        assertTrue("no prompt", prompts.isEmpty())
    }

    @Test
    fun aServerDownLeavesTheSessionByItsId() {
        // reached at its remote address for the check, then at none: the
        // session the app forgets is left in unended (D70-A), and Remove names it
        fun down(name: String) = servers.save(JSONObject().put("name", name).put("node", "node-$name").put("lan", fake.closed).put("remote", fake.base))
        val s = down("home")
        keep(s, 46)
        sessions.put(s, "session-anna")
        assertEquals("anna", sessions.check(s, fake.base))
        assertEquals("Turn off with the server down", Revoker.Off(leftHere, 46), turnOff(s, ""))
        assertEquals(listOf(KeyLeft("anna", 46)), left(s))
        assertEquals(listOf(SessionLeft("anna", 7, FakeServer.made(7))), servers.get(s.id)!!.unended)
        assertNull(sessions.held(s))

        val t = down("cabin")
        sessions.put(t, "session-anna")
        sessions.check(t, fake.base)
        val gone = remove(t, "")
        assertEquals("Remove without a kept login names the session left", Revoker.Removed(null, emptyList(), listOf(7L)), gone)
        assertNull(servers.get(t.id))

        val u = down("barn")
        keep(u, 49)
        sessions.put(u, "session-anna")
        sessions.check(u, fake.base)
        val off = remove(u, "")
        assertEquals("and with a kept login, its key and its session", Revoker.Removed(Revoker.Off(leftHere, 49), listOf(49L), listOf(7L)), off)
        assertTrue(logged.joinToString("\n"), logged.none { it.contains("session-") })
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
        assertEquals(byKeyAt(42) + byKeyAt(46), fake.requests())
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
        assertEquals(listOf("DELETE /api/v1/users/anna/tokens/42 session-dead") + byKeyAt(42), fake.requests())
        assertNull("the refused session forgotten", sessions.held(s))
    }

    @Test
    fun forgetAllAsksOnceForEveryLoginUnderTheAppKey() {
        val a = server("home", node = "node-shared")
        val b = server("cabin", node = "node-shared")
        val c = server("barn", node = "node-shared")
        keepUnderAppKey(a to LoginSecret("key-secret", 42), b to LoginSecret("key-secret", 45), c to LoginSecret("key-secret", 46))
        prompt.shown.clear()
        assertEquals(Revoker.Summary(listOf(a.id, b.id, c.id), listOf(), listOf()), forgetAll(appByKey(forgetAll = true)))
        assertEquals("three servers that need the key, one prompt, its own title", listOf("Forget all ${LoginKey.APP}"), prompt.shown)
        assertEquals(
            byKeyAt(42) + byKeyAt(45) + byKeyAt(46),
            fake.requests(),
        )
        assertTrue(logins.ids().isEmpty())
        assertFalse("the app key dropped at the run's end", appKey.held)
    }

    @Test
    fun forgetAllAsksOnceMoreForEachLoginKeptBeforeTheAppKey() {
        val a = server("home", node = "node-shared")
        val b = server("cabin", node = "node-shared")
        val c = server("barn", node = "node-shared")
        keepUnderAppKey(a to LoginSecret("key-secret", 42), c to LoginSecret("key-secret", 46))
        keepBefore(b, LoginSecret("key-secret", 45))
        prompt.shown.clear()
        assertEquals(Revoker.Summary(listOf(a.id, b.id, c.id), listOf(), listOf()), forgetAll(appByKey(forgetAll = true)))
        assertEquals(listOf("Forget all ${LoginKey.APP}", "cabin ${LoginKey.alias(b.id)}"), prompt.shown)
        assertTrue(logins.ids().isEmpty())
        assertFalse(appKey.held)
    }

    @Test
    fun turnOffAsksOnceAndDropsTheAppKey() {
        val s = server("home")
        keepUnderAppKey(s to LoginSecret("key-secret", 42))
        prompt.shown.clear()
        var got: Revoker.Off? = null
        revoker.turnOff(s.id, fake.base, appByKey()) { got = it }
        assertEquals(Revoker.Off(revoked, 42), got)
        assertEquals(listOf("home ${LoginKey.APP}"), prompt.shown)
        assertEquals(byKeyAt(42), fake.requests())
        assertFalse(appKey.held)
    }

    @Test
    fun aKeyTheAppTookBackLeavesEveryKeyByItsId() {
        val a = server("home", node = "node-shared")
        val b = server("cabin", node = "node-shared")
        val c = server("barn", node = "node-shared")
        keepUnderAppKey(a to LoginSecret("key-secret", 42), b to LoginSecret("key-secret", 45), c to LoginSecret("key-secret", 46))
        sessions.put(a, "session-anna")
        prompt.shown.clear()
        keys.takeBack(LoginKey.APP)
        assertEquals(
            "home by its session; the others no more opened, left by their ids, with their keys' codes (kept since version 6)",
            Revoker.Summary(listOf(a.id), listOf(b.id, c.id), listOf(), listOf(b.id, c.id)),
            forgetAll(appByKey(forgetAll = true)),
        )
        assertTrue("no prompt for a key taken back", prompt.shown.isEmpty())
        assertEquals(listOf(KeyLeft("anna", 45)), left(b))
        assertEquals(listOf(KeyLeft("anna", 46)), left(c))
        assertTrue(logins.ids().isEmpty())
        assertNull("the app key taken back is forgotten", logins.appKey())
        assertFalse(appKey.held)
    }

    @Test
    fun aRunThatStopsDropsTheAppKey() {
        // the activity went after the first server: the app key it opened
        // is dropped, and the next server waits for nothing
        val gone = Executors.newSingleThreadExecutor().apply { shutdownNow() }
        val probe = { urls: List<String>, _: String? -> urls.map { ProbeResult(it, ok = true, node = "node-shared") } }
        val stopped = Revoker(servers, logins, sessions, left, probe, gone) { id -> logins.forget(id) }
        val a = server("home", node = "node-shared")
        val b = server("cabin", node = "node-shared")
        keepUnderAppKey(a to LoginSecret("key-secret", 42), b to LoginSecret("key-secret", 45))
        prompt.shown.clear()
        var summary: Revoker.Summary? = null
        stopped.forgetAll(appByKey(forgetAll = true)) { summary = it }
        assertNull(summary)
        assertEquals(listOf("Forget all ${LoginKey.APP}"), prompt.shown)
        assertNull("the first one done", logins.get(a.id))
        assertEquals("the next one as it was", 45L, logins.get(b.id)!!.keyId)
        assertFalse("the app key dropped where the run stopped", appKey.held)
    }
    @Test
    fun forgetAllAsksNoMoreOnceTheAppKeysPromptIsDismissed() {
        val a = server("home", node = "node-shared")
        val b = server("cabin", node = "node-shared")
        val c = server("barn", node = "node-shared")
        keepUnderAppKey(a to LoginSecret("key-secret", 42), b to LoginSecret("key-secret", 45), c to LoginSecret("key-secret", 46))
        prompt.shown.clear()
        prompt.dismiss = true
        assertEquals(
            "each key left by its id, as a dismissed prompt leaves one, with its code",
            Revoker.Summary(listOf(), listOf(a.id, b.id, c.id), listOf(), listOf(a.id, b.id, c.id)),
            forgetAll(appByKey(forgetAll = true)),
        )
        assertEquals("one prompt, dismissed", listOf("Forget all ${LoginKey.APP}"), prompt.shown)
        assertTrue(fake.requests().isEmpty())
        assertEquals(listOf(KeyLeft("anna", 45)), left(b))
        assertTrue(logins.ids().isEmpty())
        assertFalse(appKey.held)
    }

    @Test
    fun aStepThatThrowsEndsTheRun() {
        val s = server("home")
        keep(s, 42)
        var closed = 0
        val throwing = object : Revoker.ByKey {
            override fun open(s: Server, kept: KeptLogin, then: (LoginSecret?) -> Unit) = throw IllegalStateException("the prompt broke")

            override fun close() {
                closed++
            }
        }
        assertThrows(IllegalStateException::class.java) { revoker.forgetAll(throwing) { throw AssertionError("answered") } }
        assertEquals("Forget all's run ended", 1, closed)
        keep(s, 42)
        assertThrows(IllegalStateException::class.java) { revoker.turnOff(s.id, fake.base, throwing) { throw AssertionError("answered") } }
        assertEquals("Turn off's run ended", 2, closed)
    }

    @Test
    fun aPagesIdAppReachesNoKeystoreKey() {
        // Turn off of the id `app`, which the connect page may send: no
        // server's, so its login's Keystore key is no key at all.
        val s = server("home")
        keepUnderAppKey(s to LoginSecret("key-secret", 42))
        prompt.shown.clear()
        var got: Revoker.Off? = null
        revoker.turnOff(LoginStore.APP, "", appByKey()) { got = it }
        assertEquals(Revoker.Off(local, 0), got)
        assertTrue("the app's Keystore key stays", LoginKey.APP in keys.aliases())
        assertNotNull(logins.appKey())
        var off: Revoker.Off? = null
        revoker.turnOff(s.id, fake.base, appByKey()) { off = it }
        assertEquals("the login still opens", Revoker.Off(revoked, 42), off)
        assertEquals(listOf("home ${LoginKey.APP}"), prompt.shown)
    }

    @Test
    fun turnOffForgetAllAndRemoveLeaveTheirCodes() {
        // D112-A: the kept key's code with the key left, the session's with the session; Remove keeps them in gone
        fun down(name: String) = servers.save(JSONObject().put("name", name).put("node", "node-$name").put("lan", fake.closed).put("remote", fake.base))
        fun keepWithCode(s: Server, keyId: Long) = logins.put(s.id, KeptLogin("anna", s.node, "", byteArrayOf(1), byteArrayOf(2), keyId, code = RevokeCode.of("key-$keyId")))
        val a = down("home")
        keepWithCode(a, 46)
        sessions.put(a, "session-anna")
        sessions.check(a, fake.base)
        assertEquals("the key left with its code", Revoker.Off(leftHere, 46, pending = true), turnOff(a, ""))
        assertEquals(listOf(RevokeCode.of("key-46")), left(a).map { it.code })
        assertEquals(RevokeCode.of("session-anna"), servers.get(a.id)!!.unended.single().code)

        // a login kept before version 6, without a code: its key left as before, not pending though a session left there has a code
        val b = down("cabin")
        keep(b, 49)
        sessions.put(b, "session-anna")
        sessions.check(b, fake.base)
        val res = mutableListOf<Revoker.Summary>()
        revoker.forgetAll(byKey(null)) { res.add(it) }
        assertEquals(listOf(b.id), res.single().left)
        assertEquals("the key this run left has no code", emptyList<String>(), res.single().pending)
        assertEquals(listOf(""), left(b).map { it.code })
        assertEquals(RevokeCode.of("session-anna"), servers.get(b.id)!!.unended.single().code)

        // Remove where the server does not answer: its codes kept in gone
        val gone = GoneStore(File(tmp.root, "gone.json"))
        val codes = Codes(servers, gone, api, { urls, _ -> urls.map { ProbeResult(it, ok = false, error = "refused") } }, logged::add)
        val withCodes = Revoker(servers, logins, sessions, left, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, Executor { it.run() }, codes, gone) { id ->
            logins.forget(id)
            forgotten.add(id)
        }
        var removed: Revoker.Removed? = null
        withCodes.remove(a.id, "", byKey(null)) { removed = it.getOrThrow() }
        assertEquals(Revoker.Removed(null, listOf(46L), listOf(7L), pending = true), removed)
        assertEquals(setOf(RevokeCode.of("key-46"), RevokeCode.of("session-anna")), gone.list().single().codes.map { it.code }.toSet())
        assertTrue(logged.joinToString("\n"), logged.none { it.contains("session-") || it.contains(RevokeCode.of("key-46")) })
    }

    @Test
    fun removeSaysPendingOnlyWhereItKeptTheCodes() {
        // the review: gone.json not written, Remove's answer says nothing waits, and the log says why without a code
        val s = servers.save(JSONObject().put("name", "home").put("node", "node-home").put("lan", fake.closed).put("remote", fake.base))
        sessions.put(s, "session-anna")
        sessions.check(s, fake.base)
        val gone = GoneStore(File(tmp.root, "missing/gone.json")) // its directory is not there: every write fails
        val codes = Codes(servers, gone, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, logged::add)
        val withCodes = Revoker(servers, logins, sessions, left, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, Executor { it.run() }, codes, gone, logged::add) { id ->
            logins.forget(id)
        }
        var removed: Revoker.Removed? = null
        withCodes.remove(s.id, "", byKey(null)) { removed = it.getOrThrow() }
        assertEquals(Revoker.Removed(null, emptyList(), listOf(7L), pending = false), removed)
        val line = logged.single { it.startsWith("remove:") }
        assertTrue(line, line.contains(s.id) && !line.contains(RevokeCode.of("session-anna")))
    }
}

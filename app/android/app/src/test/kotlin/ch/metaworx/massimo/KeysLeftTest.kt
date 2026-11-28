// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONObject
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
import java.security.cert.CertificateException

/**
 * The keys left valid on a server (AP MobileApp, D57-A), and the sessions
 * (D70-A), against a fake
 * server: revoked with the next session the app holds there for their
 * user, through the session's check (`session`, and `setToken`'s), each
 * only where the user's list has it as an API key and never the token in
 * use; one the list does not have as a key dropped unasked; 200 and 404
 * drop a key, 403 and a failure keep it, a failure logged without any
 * secret, a refused session or no answer stopping the rest, another user's
 * keys left alone; and a login kept without its key's id takes the id of
 * the key the session was made from, unless it changed meanwhile (F28);
 * neither the session's key nor the login's own is ever revoked, and an
 * empty list drops nothing. A session forgotten without the server ending
 * it is left by the id its check found, and the next session there for its
 * user revokes it, never the token in use nor a key's id for a session's.
 */
class KeysLeftTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val api = ServerApi("Massimo/test (Android 17; SM-X)", Prober.SystemTrust { _, _, _ -> throw CertificateException("not trusted") })
    private val plain = object : Sealer {
        override fun seal(plain: ByteArray, aad: ByteArray): ByteArray = plain.copyOf()

        override fun open(sealed: ByteArray, aad: ByteArray): ByteArray = sealed.copyOf()
    }
    private val logged = mutableListOf<String>()
    private lateinit var fake: FakeServer
    private lateinit var servers: ServerStore
    private lateinit var logins: LoginStore
    private lateinit var left: KeysLeft
    private lateinit var sessions: Sessions

    @Before
    fun start() {
        fake = FakeServer()
        servers = ServerStore(File(tmp.root, "servers.json"))
        logins = LoginStore(File(tmp.root, "logins.json"))
        val keeper = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
        left = KeysLeft(servers, logins, api, logged::add)
        sessions = Sessions(keeper, logins, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, left, logged::add)
    }

    @After
    fun stop() = fake.close()

    private fun home(): Server = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", fake.base))

    private fun keys(id: String): List<KeyLeft> = servers.get(id)!!.unrevoked

    private fun leave(s: Server, user: String, vararg ids: Long) = ids.forEach { assertTrue(left.add(s.id, user, it)) }

    @Test
    fun theNextSessionRevokesTheKeysLeftForItsUser() {
        val s = home()
        leave(s, "anna", 42, 43, 44, 47, 51, 99)
        leave(s, "bob", 46)
        assertFalse("no id: nothing to note", left.add(s.id, "anna", 0))
        sessions.put(s, "session-anna")
        assertEquals("anna", sessions.check(s, fake.base))
        assertEquals(
            listOf(
                "GET /api/v1/me session-anna",
                "GET /api/v1/users/anna/tokens session-anna",
                "DELETE /api/v1/users/anna/tokens/42 session-anna",
                "DELETE /api/v1/users/anna/tokens/44 session-anna",
                "DELETE /api/v1/users/anna/tokens/47 session-anna",
            ),
            fake.requests(),
        )
        assertEquals(
            "revoked (42) dropped; gone (43), a session's id (51) and an unknown one (99) dropped unasked; " +
                "failed (44) and refused (47) kept; Bob's left alone",
            listOf(KeyLeft("anna", 44), KeyLeft("bob", 46), KeyLeft("anna", 47)), keys(s.id).sortedBy { it.id },
        )
        val line = logged.single()
        assertTrue(line, line.contains("44") && line.contains(s.id) && line.contains("status 500"))
        assertFalse(line, line.contains("session-anna"))
        assertEquals("kept across a start", keys(s.id), ServerStore(File(tmp.root, "servers.json")).get(s.id)!!.unrevoked)
    }

    private fun sessionsLeft(id: String): List<SessionLeft> = servers.get(id)!!.unended

    // left is Anna's session [id] as the app leaves it, made when the fake says.
    private fun left(id: Long) = SessionLeft("anna", id, FakeServer.made(id))

    @Test
    fun theNextSessionRevokesTheSessionsLeftForItsUser() {
        val s = home()
        leave(s, "anna", 42)
        for ((user, id) in listOf("anna" to 52L, "anna" to 7L, "anna" to 45L, "bob" to 9L)) assertTrue(left.addSession(s.id, user, id, FakeServer.made(id)))
        // a store rolled back gave 51 to another live session of hers, made later than the one left
        assertTrue(left.addSession(s.id, "anna", 51, "2026-11-19T09:00:51Z"))
        assertFalse("never learnt: nothing to note", left.addSession(s.id, "anna", 0))
        sessions.put(s, "session-anna") // row 7
        val look = sessions.look(s, fake.base)
        assertEquals("anna", look.user)
        assertEquals("the ids the server revoked now, the key's first", listOf(42L, 52L), look.revoked)
        assertEquals(
            listOf(
                "GET /api/v1/me session-anna",
                "GET /api/v1/users/anna/tokens session-anna",
                "DELETE /api/v1/users/anna/tokens/42 session-anna",
                "DELETE /api/v1/users/anna/tokens/52 session-anna",
            ),
            fake.requests(),
        )
        assertEquals(
            "52 revoked; the token in use (7) kept; a key's id (45) and an id another session has now (51) dropped unasked; Bob's left alone",
            listOf(left(7), SessionLeft("bob", 9, FakeServer.made(9))), sessionsLeft(s.id),
        )
        assertTrue(keys(s.id).isEmpty())
        assertTrue(logged.isEmpty())
    }

    @Test
    fun aSessionForgottenWithoutTheServerIsLeftByItsId() {
        // Sign out, and Turn off's, where the server does not answer: left by the id its check found (D70-A)
        val s = home()
        // one never checked is forgotten on the phone alone; a key the page held is no session
        sessions.put(s, "session-anna-fp")
        assertFalse(sessions.signOut(s, ""))
        sessions.put(s, "key-own")
        sessions.check(s, fake.base)
        assertFalse(sessions.signOut(s, ""))
        assertTrue(sessionsLeft(s.id).isEmpty())
        sessions.put(s, "session-anna")
        sessions.check(s, fake.base)
        assertFalse(sessions.signOut(s, ""))
        assertEquals("by its id, its user and when it was made", listOf(left(7)), sessionsLeft(s.id))
        // Sign out of all, the server not answering (this test's probe answers nowhere); the
        // session checked first may not revoke 7 (401), which stays
        sessions.put(s, "session-anna-expiring")
        sessions.check(s, fake.base)
        assertEquals(0, sessions.signOutAll(listOf(s)))
        assertEquals(listOf(left(7), left(10)), sessionsLeft(s.id))
        assertTrue("kept across a start", ServerStore(File(tmp.root, "servers.json")).get(s.id)!!.unended.size == 2)
        assertTrue(logged.toString(), logged.none { it.contains("session-") })
    }

    @Test
    fun aSessionReplacedWhereTheServerDoesNotAnswerIsLeft() {
        // the page's new session replaces one the server cannot be asked to end at that address
        val s = servers.save(JSONObject().put("name", "Home").put("node", "node-a").put("lan", fake.base).put("remote", fake.closed))
        sessions.put(s, "session-anna")
        sessions.check(s, fake.base)
        val ending = sessions.put(s, "session-anna-fp", fake.closed)
        assertEquals("the new session held at once, the end left to the caller", "session-anna-fp", sessions.held(s))
        assertEquals("noted before the end", listOf(left(7)), sessionsLeft(s.id))
        ending!!.end()
        assertEquals("and still, not ended", listOf(left(7)), sessionsLeft(s.id))
        assertEquals("session-anna-fp", sessions.held(s))
        val line = logged.single()
        assertTrue(line, line.contains(s.id) && !line.contains("session-anna"))
        // the next check there asks to revoke it, the fake answering 404 for it, as for one gone already: dropped
        sessions.look(s, fake.base)
        assertEquals("DELETE /api/v1/users/anna/tokens/7 session-anna-fp", fake.requests().last())
        assertTrue(sessionsLeft(s.id).isEmpty())
    }

    @Test
    fun aSessionReplacedIsNotedUntilItsEndingEndsIt() {
        // the bridge answers the page before it ends the session replaced: an app gone meanwhile leaves it recorded
        val s = home()
        sessions.put(s, "session-anna")
        sessions.check(s, fake.base)
        val ending = sessions.put(s, "session-anna-fp", fake.base)
        assertEquals("the Ending not run: noted", listOf(left(7)), sessionsLeft(s.id))
        assertEquals("kept across a start", listOf(left(7)), ServerStore(File(tmp.root, "servers.json")).get(s.id)!!.unended)
        ending!!.end()
        assertEquals("POST /api/v1/logout session-anna", fake.requests().last())
        assertTrue("run: ended and gone", sessionsLeft(s.id).isEmpty())
        // one never checked is not noted, and is ended alone
        val alone = sessions.put(s, "session-anna", fake.base)
        assertTrue(sessionsLeft(s.id).isEmpty())
        alone!!.end()
        assertEquals("POST /api/v1/logout session-anna-fp", fake.requests().last())
        assertTrue(sessionsLeft(s.id).isEmpty())
        assertTrue(logged.isEmpty())
    }

    @Test
    fun aSessionLeftWithoutItsTimeIsDroppedUnasked() {
        // whose row an id names now cannot be told, though the list has the id as a session without a time either:
        // never signed out by it
        val s = home()
        assertTrue(left.addSession(s.id, "anna", 53, ""))
        sessions.put(s, "session-anna")
        assertEquals(emptyList<Long>(), sessions.look(s, fake.base).revoked)
        assertEquals(listOf("GET /api/v1/me session-anna", "GET /api/v1/users/anna/tokens session-anna"), fake.requests())
        assertTrue("dropped unasked", sessionsLeft(s.id).isEmpty())
    }

    @Test
    fun aRefusedSessionStopsTheRest() {
        val s = home()
        leave(s, "anna", 42, 45)
        sessions.put(s, "session-anna-expiring")
        sessions.check(s, fake.base)
        assertEquals(
            "the second key is not asked after the first was refused",
            listOf("GET /api/v1/me session-anna-expiring", "GET /api/v1/users/anna/tokens session-anna-expiring", "DELETE /api/v1/users/anna/tokens/42 session-anna-expiring"),
            fake.requests(),
        )
        assertEquals(listOf(KeyLeft("anna", 42), KeyLeft("anna", 45)), keys(s.id))
        assertTrue(logged.isEmpty())
    }

    @Test
    fun noAnswerStopsTheRest() {
        val s = home()
        leave(s, "anna", 48, 42)
        sessions.put(s, "session-anna")
        sessions.check(s, fake.base)
        // the HTTP stack may send the dropped request once more by itself
        assertEquals(
            "the second key is not asked after the first got no answer",
            listOf("GET /api/v1/me session-anna", "GET /api/v1/users/anna/tokens session-anna", "DELETE /api/v1/users/anna/tokens/48 session-anna"),
            fake.requests().distinct(),
        )
        assertEquals(listOf(KeyLeft("anna", 48), KeyLeft("anna", 42)), keys(s.id))
        val line = logged.single()
        assertTrue(line, line.contains("48") && !line.contains("session-anna"))
    }

    @Test
    fun aDeadSessionOrAServerNotReachedKeepsThem() {
        val s = home()
        leave(s, "anna", 42)
        sessions.put(s, "session-dead")
        assertNull(sessions.check(s, fake.base))
        assertEquals(listOf(KeyLeft("anna", 42)), keys(s.id))
        val me = ServerApi.Me("anna", 0, 7)
        // the list not reached: nothing asked, logged
        assertEquals(emptyList<Long>(), left.revoke(s.copy(lan = fake.closed), fake.closed, "session-anna", me))
        assertEquals(listOf(KeyLeft("anna", 42)), keys(s.id))
        assertTrue(logged.single(), logged.single().contains(s.id) && !logged.single().contains("session-anna"))
        // an address that is not the server's is not asked
        val asked = fake.requests().size
        assertEquals(emptyList<Long>(), left.revoke(s, fake.closed, "session-anna", me))
        assertEquals(asked, fake.requests().size)
    }

    @Test
    fun theTokenInUseIsNeverRevoked() {
        val s = home()
        leave(s, "anna", 45, 42)
        sessions.put(s, "key-own") // Anna's key 45 itself, as the page may hold it
        sessions.check(s, fake.base)
        assertEquals(
            listOf("GET /api/v1/me key-own", "GET /api/v1/users/anna/tokens key-own", "DELETE /api/v1/users/anna/tokens/42 key-own"),
            fake.requests(),
        )
        assertEquals("the key in use stays, to revoke with another session", listOf(KeyLeft("anna", 45)), keys(s.id))
    }

    @Test
    fun theKeysInUseAreNeverRevoked() {
        // ids a store rolled back may give again: the key the session was
        // made from, and the key of the login kept there now
        val s = home()
        leave(s, "anna", 42, 45)
        sessions.put(s, "session-anna-fp") // made from key 42
        sessions.check(s, fake.base)
        assertEquals(
            listOf("GET /api/v1/me session-anna-fp", "GET /api/v1/users/anna/tokens session-anna-fp", "DELETE /api/v1/users/anna/tokens/45 session-anna-fp"),
            fake.requests(),
        )
        assertEquals(listOf(KeyLeft("anna", 42)), keys(s.id))

        val t = servers.save(JSONObject().put("name", "Cabin").put("node", "node-a").put("lan", fake.base))
        logins.put(t.id, KeptLogin("anna", "node-a", "", byteArrayOf(1), byteArrayOf(2), 46))
        leave(t, "anna", 46, 49)
        sessions.put(t, "session-anna")
        sessions.check(t, fake.base)
        assertEquals("DELETE /api/v1/users/anna/tokens/49 session-anna", fake.requests().last())
        assertTrue(fake.requests().none { it.startsWith("DELETE /api/v1/users/anna/tokens/46") })
        assertEquals(listOf(KeyLeft("anna", 46)), keys(t.id))
    }

    @Test
    fun anEmptyListDropsNothing() {
        val s = home()
        leave(s, "anna", 42, 45)
        sessions.put(s, "session-anna-quiet")
        sessions.check(s, fake.base)
        assertEquals(listOf("GET /api/v1/me session-anna-quiet", "GET /api/v1/users/anna/tokens session-anna-quiet"), fake.requests())
        assertEquals(listOf(KeyLeft("anna", 42), KeyLeft("anna", 45)), keys(s.id))
        assertTrue(logged.single(), logged.single().contains("status 200") && !logged.single().contains("session-anna"))
    }

    @Test
    fun aLoginKeptWithoutItsKeysIdTakesTheSessionsKey() {
        val s = home()
        logins.put(s.id, KeptLogin("anna", "node-a", "", byteArrayOf(1), byteArrayOf(2), 0))
        // a session made with the password names no key
        sessions.put(s, "session-anna")
        sessions.check(s, fake.base)
        assertEquals(0L, logins.get(s.id)!!.keyId)
        // one made from the kept key names it
        sessions.put(s, "session-anna-fp")
        assertEquals("anna", sessions.check(s, fake.base))
        assertEquals(42L, logins.get(s.id)!!.keyId)
        // keyOf asks only where the login has none
        assertEquals(42L, sessions.keyOf(s, fake.base, logins.get(s.id)!!))
        assertEquals(listOf("GET /api/v1/me session-anna", "GET /api/v1/me session-anna-fp"), fake.requests())
    }

    @Test
    fun keyOfAsksOnlyForALoginWithoutTheId() {
        val s = home()
        val kept = KeptLogin("anna", "node-a", "", byteArrayOf(1), byteArrayOf(2), 0)
        logins.put(s.id, kept)
        assertEquals("no session held", 0L, sessions.keyOf(s, fake.base, kept))
        assertEquals(0, fake.requests().size)
        sessions.put(s, "session-bob")
        assertEquals("another user's session", 0L, sessions.keyOf(s, fake.base, kept))
        assertEquals(0L, logins.get(s.id)!!.keyId)
        sessions.put(s, "session-anna-fp")
        assertEquals(42L, sessions.keyOf(s, fake.base, kept))
        assertEquals("kept with the login", 42L, logins.get(s.id)!!.keyId)
        assertEquals(42L, sessions.keyOf(s, fake.base, logins.get(s.id)!!))
        assertEquals("one request each, none for a login with its id", 2, fake.requests().size)
        sessions.put(s, "session-dead")
        assertEquals(0L, sessions.keyOf(s, fake.base, kept))
        assertNull("a refused session is forgotten", sessions.held(s))
    }

    @Test
    fun aLoginKeptAnewMeanwhileIsLeftAsItIs() {
        val s = home()
        val was = KeptLogin("anna", "node-a", "", byteArrayOf(1), byteArrayOf(2), 0)
        val anew = KeptLogin("anna", "node-a", "", byteArrayOf(9), byteArrayOf(8), 0)
        logins.put(s.id, anew)
        sessions.put(s, "session-anna-fp")
        assertEquals("the login read before is no more", 0L, sessions.keyOf(s, fake.base, was))
        assertEquals(anew, logins.get(s.id))
        assertFalse(logins.setKeyId(s.id, was, 42))
        assertTrue(logins.setKeyId(s.id, anew, 42))
        assertTrue("a check meanwhile gave it the same id", logins.setKeyId(s.id, anew, 42))
        assertFalse("not another", logins.setKeyId(s.id, anew, 43))
        assertFalse("once", logins.setKeyId(s.id, anew.copy(keyId = 42), 43))
        assertEquals(42L, logins.get(s.id)!!.keyId)
    }

    @Test
    fun aSessionTheProcessForgotIsRevokedAtTheNextSession() {
        // F47, D102-A: the session held when the process ended, left at the next start, revoked by the next session there
        val s = home()
        logins.put(s.id, KeptLogin("anna", "node-a", "", byteArrayOf(1), byteArrayOf(2), 42))
        sessions.put(s, "session-anna")
        sessions.check(s, fake.base)
        assertEquals(7L, servers.get(s.id)!!.held!!.id)
        val next = SessionKeeper(TokenStore(File(tmp.root, "tokens.json"), plain))
        next.leaveForgotten(servers, logins)
        assertEquals(listOf(left(7)), sessionsLeft(s.id))
        val after = Sessions(next, logins, api, { urls, _ -> urls.map { ProbeResult(it, ok = false) } }, left, logged::add)
        after.put(s, "session-anna-fp")
        after.check(s, fake.base)
        assertTrue(fake.requests().contains("DELETE /api/v1/users/anna/tokens/7 session-anna-fp"))
        assertTrue(sessionsLeft(s.id).isEmpty())
        assertEquals("the new one held now", 8L, servers.get(s.id)!!.held!!.id)
    }
}

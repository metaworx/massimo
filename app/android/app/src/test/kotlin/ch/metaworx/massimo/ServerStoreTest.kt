// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class ServerStoreTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private fun file() = File(tmp.root, "servers.json")

    private fun input(vararg kv: Pair<String, Any>) = JSONObject().apply { kv.forEach { (k, v) -> put(k, v) } }

    private fun refused(text: Int, block: () -> Unit) {
        try {
            block()
            fail("not refused")
        } catch (e: BridgeException) {
            assertEquals(text, e.text)
        }
    }

    @Test
    fun saveAddsInOrderWithAnId() {
        val st = ServerStore(file())
        val a = st.save(input("name" to "Home", "node" to "node-a", "lan" to "http://192.168.1.20:8099/", "ask" to "return"))
        val b = st.save(input("name" to "Cabin", "remote" to "https://massimo.example.net/"))
        assertTrue(a.id.isNotEmpty())
        assertNotEquals(a.id, b.id)
        assertEquals(listOf("Home", "Cabin"), st.list().map { it.name })
        assertEquals("node-a", a.node)
        assertEquals(Server.ASK_RETURN, a.ask)
        assertEquals(Server.ASK_START, b.ask) // the choice until the user makes one
        assertEquals("", a.fp)
    }

    @Test
    fun saveReplacesInPlaceAndKeepsThePin() {
        val st = ServerStore(file())
        val a = st.save(input("name" to "Home", "lan" to "http://192.168.1.20:8099/"))
        st.save(input("name" to "Cabin", "lan" to "http://192.168.1.21:8099/"))
        val fp = Fingerprint.of(byteArrayOf(7))
        st.trust(a.id, fp)
        val again = st.save(input("id" to a.id, "name" to "House", "lan" to "https://192.168.1.20:8443/", "fp" to "ignored"))
        assertEquals(a.id, again.id)
        assertEquals(fp, again.fp)
        assertEquals(listOf("House", "Cabin"), st.list().map { it.name })
    }

    @Test
    fun saveOfAnUnknownIdIsRefused() {
        refused(R.string.err_no_server) { ServerStore(file()).save(input("id" to "nope", "lan" to "http://192.168.1.20/")) }
    }

    @Test
    fun addressesAreChecked() {
        val st = ServerStore(file())
        refused(R.string.err_no_address) { st.save(input("name" to "x")) }
        for (bad in listOf(
            "http://192.168.1.20:8099",
            "https://user@massimo.example.net/",
            "https://massimo.example.net/?x=1",
            "https://massimo.example.net/#top",
            "ftp://massimo.example.net/",
            "https://appassets.androidplatform.net/",
            "massimo.example.net/",
        )) {
            refused(R.string.err_bad_address) { st.save(input("lan" to bad)) }
        }
        assertEquals("https://massimo.example.net/massimo/", ServerStore.address(" https://massimo.example.net/massimo/ "))
        assertEquals("", ServerStore.address(""))
    }

    @Test
    fun aServerWithoutANameIsNamedByItsHost() {
        val s = ServerStore(file()).save(input("name" to "  ", "remote" to "https://massimo.example.net/"))
        assertEquals("massimo.example.net", s.name)
    }

    @Test
    fun valuesOfTheWrongTypeAreNone() {
        val st = ServerStore(file())
        val s = st.save(input("name" to 3, "lan" to "http://192.168.1.20/", "ask" to "sometimes", "node" to JSONObject.NULL))
        assertEquals("192.168.1.20", s.name)
        assertEquals("", s.node)
        assertEquals(Server.ASK_START, s.ask)
        // A choice the page sends that is none keeps the one saved.
        val r = st.save(input("id" to s.id, "lan" to "http://192.168.1.20/", "ask" to "logout"))
        assertEquals(Server.ASK_LOGOUT, st.save(input("id" to r.id, "lan" to "http://192.168.1.20/", "ask" to 7)).ask)
    }

    @Test
    fun removeForgets() {
        val st = ServerStore(file())
        val a = st.save(input("lan" to "http://192.168.1.20/"))
        st.remove(a.id)
        st.remove("never-there")
        assertTrue(st.list().isEmpty())
        assertNull(st.get(a.id))
    }

    @Test
    fun trustTakesFingerprintsAlone() {
        val st = ServerStore(file())
        val a = st.save(input("lan" to "https://192.168.1.20:8443/"))
        refused(R.string.err_bad_fp) { st.trust(a.id, "not one") }
        refused(R.string.err_no_server) { st.trust("nope", Fingerprint.of(byteArrayOf(1))) }
        val fp = Fingerprint.of(byteArrayOf(1))
        st.trust(a.id, fp.removePrefix("SHA256:").lowercase().replace(":", ""))
        assertEquals(fp, st.get(a.id)?.fp)
    }

    @Test
    fun theFileKeepsThemAcrossStarts() {
        val st = ServerStore(file())
        val a = st.save(input("name" to "Home", "lan" to "http://192.168.1.20:8099/", "push" to true))
        st.trust(a.id, Fingerprint.of(byteArrayOf(3)))
        assertEquals(st.list(), ServerStore(file()).list())
        assertFalse(File(file().path + ".new").exists())
    }

    @Test
    fun aFileItCannotReadIsKeptAside() {
        file().writeText("{ not json")
        val st = ServerStore(file())
        assertTrue(st.list().isEmpty())
        assertTrue(File(file().path + ".bad").isFile)
    }

    @Test
    fun oneServerAtMostOpensAtStart() {
        val st = ServerStore(file())
        assertNull("an empty list opens the list", st.atStart())
        val a = st.save(input("name" to "Home", "lan" to "http://192.168.1.20:8099/"))
        assertTrue("the first server added opens at start (D43-A)", a.start)
        val b = st.save(input("name" to "Cabin", "lan" to "http://192.168.1.21:8099/", "start" to false))
        assertFalse(b.start)
        assertEquals(a.id, st.atStart()?.id)
        // Turning it on at one turns it off at the other.
        st.save(input("id" to b.id, "name" to "Cabin", "lan" to "http://192.168.1.21:8099/", "start" to true))
        assertEquals(listOf(false, true), st.list().map { it.start })
        assertEquals(b.id, ServerStore(file()).atStart()?.id)
        // A save without it turns it off: the page sends the whole server.
        st.save(input("id" to b.id, "name" to "Cabin", "lan" to "http://192.168.1.21:8099/"))
        assertNull(st.atStart())
        // A file with two marked keeps the first.
        file().writeText(
            "[{\"id\":\"a\",\"lan\":\"http://192.168.1.20/\",\"start\":true},{\"id\":\"b\",\"lan\":\"http://192.168.1.21/\",\"start\":true}]",
        )
        assertEquals(listOf(true, false), ServerStore(file()).list().map { it.start })
    }

    @Test
    fun trustNotesWhen() {
        var now = "2026-11-20T12:00:00Z"
        val st = ServerStore(file()) { now }
        val a = st.save(input("lan" to "https://192.168.1.20:8443/"))
        assertEquals("", a.trustedAt)
        st.trust(a.id, Fingerprint.of(byteArrayOf(1)))
        assertEquals("2026-11-20T12:00:00Z", st.get(a.id)?.trustedAt)
        now = "2026-11-21T08:30:00Z"
        st.trust(a.id, Fingerprint.of(byteArrayOf(2)))
        assertEquals("a new certificate trusted, a new time", "2026-11-21T08:30:00Z", st.get(a.id)?.trustedAt)
        // kept across a save, which the page cannot change it by, and a start
        st.save(input("id" to a.id, "lan" to "https://192.168.1.20:8443/", "trusted_at" to "never"))
        assertEquals("2026-11-21T08:30:00Z", ServerStore(file()).get(a.id)?.trustedAt)
        assertTrue(Regex("""^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$""").matches(ServerStore.nowUtc()))
    }

    @Test
    fun thePageSeesTheSessionTheFileDoesNot() {
        val st = ServerStore(file())
        val a = st.save(input("name" to "Home", "lan" to "http://192.168.1.20:8099/"))
        val page = a.toJson("anna", "bob", 46)
        assertEquals("anna", page.getString("login"))
        assertEquals("the kept login's key (contract version 5)", 46L, page.getLong("login_key"))
        assertEquals("bob", page.getString("session"))
        assertEquals(true, page.get("start"))
        assertEquals("", page.getString("trusted_at"))
        assertTrue("no session checked is null", a.toJson().isNull("session"))
        assertEquals("no login kept, no key", 0L, a.toJson("", null, 46).getLong("login_key"))
        assertEquals("a key not known", 0L, a.toJson("anna").getLong("login_key"))
        val kept = file().readText()
        assertFalse(kept.contains("session"))
        assertFalse(kept.contains("login"))
        assertTrue(kept.contains("\"start\":true"))
    }

    @Test
    fun versionOnesSwitchIsTakenOnce() {
        // A file of version 1, one server with "Ask for my fingerprint or PIN" on.
        file().writeText(
            "[{\"id\":\"a\",\"name\":\"Home\",\"lan\":\"http://192.168.1.20/\",\"biometric\":true}," +
                "{\"id\":\"b\",\"name\":\"Cabin\",\"lan\":\"http://192.168.1.21/\",\"biometric\":false}]",
        )
        val st = ServerStore(file())
        assertEquals(setOf("a"), st.takeLegacy())
        assertEquals(emptySet<String>(), st.takeLegacy())
        assertEquals(Server.ASK_START, st.get("a")!!.ask)
        assertFalse(file().readText().contains("biometric"))
        assertEquals(emptySet<String>(), ServerStore(file()).takeLegacy())
        val json = st.get("a")!!.toJson("anna")
        assertEquals("anna", json.getString("login"))
        assertEquals("start", json.getString("ask"))
        assertFalse(json.has("biometric"))
    }

    @Test
    fun keysLeftAreKeptUntilRevoked() {
        val st = ServerStore(file())
        val a = st.save(input("name" to "Home", "lan" to "http://192.168.1.20:8099/"))
        assertTrue(a.unrevoked.isEmpty())
        assertEquals("[]", a.toJson().getJSONArray("unrevoked").toString())
        assertTrue(st.leave(a.id, "anna", 42))
        assertTrue("noted already, once", st.leave(a.id, "anna", 42))
        assertTrue(st.leave(a.id, "bob", 7))
        assertFalse("no id", st.leave(a.id, "anna", 0))
        assertFalse("no user", st.leave(a.id, "", 9))
        assertFalse("no server", st.leave("gone", "anna", 5))
        assertEquals(listOf(KeyLeft("anna", 42), KeyLeft("bob", 7)), st.get(a.id)!!.unrevoked)
        val page = st.get(a.id)!!.toJson()
        val shown = page.getJSONArray("unrevoked")
        assertEquals(listOf("anna" to 42L, "bob" to 7L), (0 until shown.length()).map { shown.getJSONObject(it).let { k -> k.getString("user") to k.getLong("id") } })
        // kept across a start and by a save from the page, which cannot set them
        assertEquals(listOf(KeyLeft("anna", 42), KeyLeft("bob", 7)), ServerStore(file()).get(a.id)!!.unrevoked)
        val saved = st.save(input("id" to a.id, "name" to "House", "lan" to "http://192.168.1.20:8099/", "unrevoked" to org.json.JSONArray()))
        assertEquals(2, saved.unrevoked.size)
        st.revoked(a.id, listOf(KeyLeft("anna", 42), KeyLeft("anna", 99)))
        assertEquals(listOf(KeyLeft("bob", 7)), ServerStore(file()).get(a.id)!!.unrevoked)
        // another node: those keys are another server's
        val moved = st.save(input("id" to a.id, "name" to "House", "node" to "node-b", "lan" to "http://192.168.1.20:8099/"))
        assertTrue(moved.unrevoked.isEmpty())
        // a file's key that is no key is left out
        file().writeText("""[{"id":"a","name":"Home","lan":"http://192.168.1.20/","unrevoked":[{"user":"anna","id":3},{"user":"","id":4},{"id":5},"x"]}]""")
        assertEquals(listOf(KeyLeft("anna", 3)), ServerStore(file()).get("a")!!.unrevoked)
    }

    @Test
    fun sessionsLeftAreKeptUntilEnded() {
        val st = ServerStore(file())
        val a = st.save(input("name" to "Home", "lan" to "http://192.168.1.20:8099/"))
        assertTrue(a.unended.isEmpty())
        assertEquals("[]", a.toJson().getJSONArray("unended").toString())
        assertTrue(st.leaveSession(a.id, "anna", 7))
        assertTrue("noted already, once", st.leaveSession(a.id, "anna", 7))
        assertTrue(st.leaveSession(a.id, "bob", 9))
        assertFalse("no id, never learnt", st.leaveSession(a.id, "anna", 0))
        assertFalse("no user", st.leaveSession(a.id, "", 8))
        assertFalse("no server", st.leaveSession("gone", "anna", 5))
        assertEquals(listOf(SessionLeft("anna", 7), SessionLeft("bob", 9)), st.get(a.id)!!.unended)
        // when it was made, kept beside it across a start
        assertTrue(st.leaveSession(a.id, "carl", 11, "2026-11-20T10:00:11Z"))
        assertEquals(SessionLeft("carl", 11, "2026-11-20T10:00:11Z"), ServerStore(file()).get(a.id)!!.unended.last())
        assertEquals("2026-11-20T10:00:11Z", st.get(a.id)!!.toJson().getJSONArray("unended").getJSONObject(2).getString("created_at"))
        st.ended(a.id, listOf(SessionLeft("carl", 11, "2026-11-20T10:00:11Z")))
        assertTrue("apart from the keys left", st.get(a.id)!!.unrevoked.isEmpty())
        val shown = st.get(a.id)!!.toJson().getJSONArray("unended")
        assertEquals(listOf("anna" to 7L, "bob" to 9L), (0 until shown.length()).map { shown.getJSONObject(it).let { k -> k.getString("user") to k.getLong("id") } })
        // kept across a start and by a save from the page, which cannot set them
        assertEquals(listOf(SessionLeft("anna", 7), SessionLeft("bob", 9)), ServerStore(file()).get(a.id)!!.unended)
        val saved = st.save(input("id" to a.id, "name" to "House", "lan" to "http://192.168.1.20:8099/", "unended" to org.json.JSONArray()))
        assertEquals(2, saved.unended.size)
        st.ended(a.id, listOf(SessionLeft("anna", 7), SessionLeft("anna", 99)))
        assertEquals(listOf(SessionLeft("bob", 9)), ServerStore(file()).get(a.id)!!.unended)
        // another node: those sessions are another server's
        val moved = st.save(input("id" to a.id, "name" to "House", "node" to "node-b", "lan" to "http://192.168.1.20:8099/"))
        assertTrue(moved.unended.isEmpty())
        // a file's session that is no session is left out
        file().writeText("""[{"id":"a","name":"Home","lan":"http://192.168.1.20/","unended":[{"user":"anna","id":3,"created_at":"2026-11-20T10:00:03Z"},{"user":"","id":4},{"id":5},"x"]}]""")
        assertEquals(listOf(SessionLeft("anna", 3, "2026-11-20T10:00:03Z")), ServerStore(file()).get("a")!!.unended)
    }

    @Test
    fun openTakesAnAddressAndMyProfilesKey() {
        val s = Server("a", "Home", "node-a", "http://192.168.1.20:8099/", "https://home.example.net/", "", Server.ASK_START, false)
        assertEquals("http://192.168.1.20:8099/", s.opening("http://192.168.1.20:8099/"))
        assertEquals("https://home.example.net/", s.opening("https://home.example.net/#profile?key=42"))
        for (bad in listOf(
            "https://home.example.net/#profile?key=0",
            "https://home.example.net/#profile?key=abc",
            "https://home.example.net/#profile?key=42&x=1",
            "https://home.example.net/#system",
            "https://home.example.net/x#profile?key=42",
            "https://other.example.net/#profile?key=42",
            "",
        )) {
            assertNull(bad, s.opening(bad))
        }
    }

    @Test
    fun theSessionHeldIsKeptInTheFileAloneAndDroppedWithTheNode() {
        // D102-A: {user, id, created_at}, no secret, never to the page; save keeps it, a new node drops it
        val st = ServerStore(file())
        val a = st.save(input("name" to "Home", "node" to "node-a", "lan" to "http://192.168.1.20:8099/"))
        st.hold(a.id, HeldSession("anna", 7, "2026-11-20T10:00:07Z"))
        assertEquals(HeldSession("anna", 7, "2026-11-20T10:00:07Z"), ServerStore(file()).get(a.id)!!.held)
        assertFalse(st.get(a.id)!!.toJson().has("held"))
        assertTrue(st.get(a.id)!!.fileJson().has("held"))
        st.save(input("id" to a.id, "name" to "House", "node" to "node-a", "lan" to "http://192.168.1.20:8099/"))
        assertEquals("kept by save", 7L, st.get(a.id)!!.held!!.id)
        st.save(input("id" to a.id, "name" to "House", "node" to "node-b", "lan" to "http://192.168.1.20:8099/"))
        assertNull("another node's", st.get(a.id)!!.held)
        st.hold(a.id, HeldSession("anna", 8, ""))
        st.unhold(a.id)
        assertNull(ServerStore(file()).get(a.id)!!.held)
        // moved into unended where the app holds it no more, once
        st.hold(a.id, HeldSession("anna", 9, "2026-11-20T10:00:09Z"))
        st.leaveHeld { true }
        assertEquals(9L, st.get(a.id)!!.held!!.id)
        st.leaveHeld { false }
        assertNull(st.get(a.id)!!.held)
        assertEquals(listOf(SessionLeft("anna", 9, "2026-11-20T10:00:09Z")), ServerStore(file()).get(a.id)!!.unended)
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.security.GeneralSecurityException

class LoginStoreTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val fp = Fingerprint.of(byteArrayOf(1, 2, 3))
    private val secret = LoginSecret("k".repeat(64), 7)

    // The Keystore alias of a login kept before the app key, of a server's id.
    private val alias = LoginKey.alias("0123456789abcdef01234567")

    @Test
    fun sealedKeptAndOpened() {
        val keys = JvmKeys()
        val st = LoginStore(File(tmp.root, "logins.json"))
        assertNull(st.get("s1"))
        assertEquals("", st.user("s1"))
        val kept = LoginStore.seal(keys.encrypter(alias), secret, "s1", "node-a", fp, "anna")
        assertFalse(String(kept.sealed, Charsets.ISO_8859_1).contains(secret.key))
        st.put("s1", kept)
        // Another store over the same file reads what was kept.
        val again = LoginStore(File(tmp.root, "logins.json")).get("s1")!!
        assertEquals(kept, again)
        assertEquals("anna", st.user("s1"))
        assertEquals(listOf("s1"), st.ids())
        assertEquals(secret, LoginStore.open(keys.decrypter(alias, again.iv), again, "s1"))
        assertFalse(st.get("s1").toString().contains(secret.key))
        // The key's row in clear beside it, for Turn off with a session (contract version 3).
        assertEquals(7L, again.keyId)
        assertTrue(File(tmp.root, "logins.json").readText().contains("\"key_id\":7"))
        // Sealed by a key of its own: a login kept before the app key.
        assertFalse(again.byApp)
        assertFalse(File(tmp.root, "logins.json").readText().contains("\"app\""))
    }

    @Test
    fun theAppKeyIsKeptBesideTheLoginsAndNoneOfThem() {
        val file = File(tmp.root, "logins.json")
        val st = LoginStore(file)
        assertNull(st.appKey())
        val sealed = SealedKey(byteArrayOf(1, 2, 3), byteArrayOf(4, 5, 6))
        st.putAppKey(sealed)
        val kept = LoginStore.seal(JvmKeys().encrypter("x"), secret, "s1", "", "", "anna", byApp = true)
        st.put("s1", kept)
        val again = LoginStore(file)
        assertEquals(sealed, again.appKey())
        assertEquals("a login under the app key says so", kept, again.get("s1"))
        assertTrue(again.get("s1")!!.byApp)
        assertTrue(file.readText().contains("\"app\":true"))
        assertEquals("the app key is no server's login", listOf("s1"), again.ids())
        assertNull(again.get(LoginStore.APP))
        assertFalse(again.forget(LoginStore.APP))
        assertEquals(sealed, again.appKey())
        // Forgetting the logins leaves it; forgetting it leaves them.
        assertTrue(again.forget("s1"))
        assertEquals(sealed, again.appKey())
        again.put("s1", kept)
        again.forgetAppKey()
        assertNull(again.appKey())
        assertEquals(kept, again.get("s1"))
    }

    @Test
    fun aLoginKeptBeforeVersionThreeHasNoKeyId() {
        val keys = JvmKeys()
        val kept = LoginStore.seal(keys.encrypter(alias), secret, "s1", "", "", "anna")
        val st = LoginStore(File(tmp.root, "logins.json"))
        st.put("s1", kept.copy(keyId = 0))
        val file = File(tmp.root, "logins.json")
        file.writeText(file.readText().replace(",\"key_id\":0", "").replace("\"key_id\":0,", ""))
        assertFalse(file.readText().contains("key_id"))
        assertEquals(0L, st.get("s1")!!.keyId)
    }

    @Test
    fun opensForNothingElse() {
        val keys = JvmKeys()
        val kept = LoginStore.seal(keys.encrypter(alias), secret, "s1", "node-a", fp, "anna")
        // Another server, node, fingerprint or user: the seal does not open.
        for (bad in listOf(
            "s2" to kept,
            "s1" to kept.copy(node = "node-b"),
            "s1" to kept.copy(fp = Fingerprint.of(byteArrayOf(9))),
            "s1" to kept.copy(user = "bob"),
        )) {
            try {
                LoginStore.open(keys.decrypter(alias, kept.iv), bad.second, bad.first)
                fail("opened for $bad")
            } catch (e: GeneralSecurityException) {
                // refused
            }
        }
    }

    @Test
    fun forgetOneAndAll() {
        val keys = JvmKeys()
        val st = LoginStore(File(tmp.root, "logins.json"))
        for (id in listOf("s1", "s2")) st.put(id, LoginStore.seal(keys.encrypter(alias), secret, id, "", "", "anna"))
        assertTrue(st.forget("s1"))
        assertFalse(st.forget("s1"))
        assertNull(st.get("s1"))
        assertEquals(listOf("s2"), st.ids())
        // A file the app cannot read is none.
        File(tmp.root, "logins.json").writeText("{not json")
        assertTrue(LoginStore(File(tmp.root, "logins.json")).ids().isEmpty())
    }

    @Test
    fun aKeyTakenBackOpensNothing() {
        val keys = JvmKeys()
        val kept = LoginStore.seal(keys.encrypter(alias), secret, "s1", "", "", "anna")
        keys.delete(alias)
        try {
            keys.decrypter(alias, kept.iv)
            fail("a deleted key opened")
        } catch (e: KeyInvalidated) {
            // the login is forgotten
        }
    }
}

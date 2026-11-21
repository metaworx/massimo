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
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

class LoginStoreTest {
    @get:Rule
    val tmp = TemporaryFolder()

    // The Keystore's keys, with keys of the JVM's: the same AES-GCM, and no prompt.
    private class JvmKeys : KeySource {
        private val keys = HashMap<String, SecretKey>()

        override fun encrypter(alias: String): Cipher {
            val k = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey()
            keys[alias] = k
            return Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, k) }
        }

        override fun decrypter(alias: String, iv: ByteArray): Cipher {
            val k = keys[alias] ?: throw KeyInvalidated()
            return Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.DECRYPT_MODE, k, GCMParameterSpec(128, iv)) }
        }

        override fun delete(alias: String) {
            keys.remove(alias)
        }
    }

    private val fp = Fingerprint.of(byteArrayOf(1, 2, 3))
    private val secret = LoginSecret("k".repeat(64), 7)

    @Test
    fun sealedKeptAndOpened() {
        val keys = JvmKeys()
        val st = LoginStore(File(tmp.root, "logins.json"))
        assertNull(st.get("s1"))
        assertEquals("", st.user("s1"))
        val kept = LoginStore.seal(keys.encrypter(LoginKey.alias("s1")), secret, "s1", "node-a", fp, "anna")
        assertFalse(String(kept.sealed, Charsets.ISO_8859_1).contains(secret.key))
        st.put("s1", kept)
        // Another store over the same file reads what was kept.
        val again = LoginStore(File(tmp.root, "logins.json")).get("s1")!!
        assertEquals(kept, again)
        assertEquals("anna", st.user("s1"))
        assertEquals(listOf("s1"), st.ids())
        assertEquals(secret, LoginStore.open(keys.decrypter(LoginKey.alias("s1"), again.iv), again, "s1"))
        assertFalse(st.get("s1").toString().contains(secret.key))
        // The key's row in clear beside it, for Turn off with a session (contract version 3).
        assertEquals(7L, again.keyId)
        assertTrue(File(tmp.root, "logins.json").readText().contains("\"key_id\":7"))
    }

    @Test
    fun aLoginKeptBeforeVersionThreeHasNoKeyId() {
        val keys = JvmKeys()
        val kept = LoginStore.seal(keys.encrypter(LoginKey.alias("s1")), secret, "s1", "", "", "anna")
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
        val kept = LoginStore.seal(keys.encrypter(LoginKey.alias("s1")), secret, "s1", "node-a", fp, "anna")
        // Another server, node, fingerprint or user: the seal does not open.
        for (bad in listOf(
            "s2" to kept,
            "s1" to kept.copy(node = "node-b"),
            "s1" to kept.copy(fp = Fingerprint.of(byteArrayOf(9))),
            "s1" to kept.copy(user = "bob"),
        )) {
            try {
                LoginStore.open(keys.decrypter(LoginKey.alias("s1"), kept.iv), bad.second, bad.first)
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
        for (id in listOf("s1", "s2")) st.put(id, LoginStore.seal(keys.encrypter(LoginKey.alias(id)), secret, id, "", "", "anna"))
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
        val kept = LoginStore.seal(keys.encrypter(LoginKey.alias("s1")), secret, "s1", "", "", "anna")
        keys.delete(LoginKey.alias("s1"))
        try {
            keys.decrypter(LoginKey.alias("s1"), kept.iv)
            fail("a deleted key opened")
        } catch (e: KeyInvalidated) {
            // the login is forgotten
        }
    }
}

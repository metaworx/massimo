// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

class TokenStoreTest {
    @get:Rule
    val tmp = TemporaryFolder()

    // The Keystore's seal, with a key of the JVM's: the same AES-GCM.
    private class JvmSealer : Sealer {
        private val key: SecretKey = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey()

        override fun seal(plain: ByteArray, aad: ByteArray): ByteArray {
            val iv = ByteArray(12).also { SecureRandom().nextBytes(it) }
            val c = Cipher.getInstance("AES/GCM/NoPadding")
            c.init(Cipher.ENCRYPT_MODE, key, GCMParameterSpec(128, iv))
            c.updateAAD(aad)
            return iv + c.doFinal(plain)
        }

        override fun open(sealed: ByteArray, aad: ByteArray): ByteArray {
            val c = Cipher.getInstance("AES/GCM/NoPadding")
            c.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, sealed, 0, 12))
            c.updateAAD(aad)
            return c.doFinal(sealed, 12, sealed.size - 12)
        }
    }

    private val file get() = File(tmp.root, "tokens.json")

    @Test
    fun putGetForget() {
        val st = TokenStore(file, JvmSealer())
        assertNull(st.get("a"))
        st.put("a", "session-of-a")
        st.put("b", "session-of-b")
        assertEquals("session-of-a", st.get("a"))
        assertEquals("session-of-b", st.get("b"))
        st.put("a", null)
        assertNull(st.get("a"))
        st.remove("b")
        assertNull(st.get("b"))
    }

    @Test
    fun theFileHoldsNoTokenInTheClear() {
        TokenStore(file, JvmSealer()).put("a", "session-of-a")
        assertFalse(file.readText().contains("session-of-a"))
    }

    @Test
    fun aTokenMovedToAnotherServerDoesNotOpen() {
        val st = TokenStore(file, JvmSealer())
        st.put("a", "session-of-a")
        val all = JSONObject(file.readText())
        file.writeText(all.put("b", all.getString("a")).toString())
        assertNull(st.get("b"))
        // and it is dropped, the one of a kept
        assertFalse(JSONObject(file.readText()).has("b"))
        assertEquals("session-of-a", st.get("a"))
    }

    @Test
    fun aTokenThatNoLongerOpensIsDropped() {
        TokenStore(file, JvmSealer()).put("a", "session-of-a")
        // a new key, as after the Keystore lost the old one
        assertNull(TokenStore(file, JvmSealer()).get("a"))
        assertFalse(JSONObject(file.readText()).has("a"))
    }

    @Test
    fun garbageIsNoToken() {
        file.writeText(JSONObject().put("a", "%%%").toString())
        assertNull(TokenStore(file, JvmSealer()).get("a"))
        file.writeText("not json")
        assertNull(TokenStore(file, JvmSealer()).get("a"))
    }
}

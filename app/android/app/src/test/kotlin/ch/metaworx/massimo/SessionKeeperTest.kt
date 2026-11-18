// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** What each of the three choices of when the fingerprint is asked keeps of a server's session (docs/app.md). */
class SessionKeeperTest {
    @get:Rule
    val tmp = TemporaryFolder()

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

    private val sealer = JvmSealer()
    private fun tokens() = TokenStore(File(tmp.root, "tokens.json"), sealer)

    @Test
    fun whereEachChoiceKeepsIt() {
        // Without a kept login a session is sealed, whatever the choice.
        for (ask in Server.ASKS) assertFalse(ask, SessionKeeper.memoryFor(ask, kept = false))
        // With one: in memory at each return and start, sealed after a logout.
        assertTrue(SessionKeeper.memoryFor(Server.ASK_RETURN, kept = true))
        assertTrue(SessionKeeper.memoryFor(Server.ASK_START, kept = true))
        assertFalse(SessionKeeper.memoryFor(Server.ASK_LOGOUT, kept = true))
    }

    @Test
    fun inMemoryIsGoneAtTheNextStart() {
        val k = SessionKeeper(tokens())
        k.put("s1", "session-1", inMemory = true)
        assertEquals("session-1", k.get("s1", inMemory = true))
        assertTrue(k.inMemory("s1"))
        assertNull("never sealed", tokens().get("s1"))
        // The next start is a new keeper over the same file.
        val next = SessionKeeper(tokens())
        assertNull(next.get("s1", inMemory = true))
        assertNull(next.get("s1", inMemory = false))
    }

    @Test
    fun sealedOutlivesTheStart() {
        val k = SessionKeeper(tokens())
        k.put("s1", "session-1", inMemory = false)
        assertFalse(k.inMemory("s1"))
        assertEquals("session-1", SessionKeeper(tokens()).get("s1", inMemory = false))
    }

    @Test
    fun aChoiceChangedMovesIt() {
        val k = SessionKeeper(tokens())
        k.put("s1", "session-1", inMemory = false)
        // Kept at each start: out of the file, into memory.
        k.move("s1", inMemory = true)
        assertEquals("session-1", k.get("s1", inMemory = true))
        assertNull(tokens().get("s1"))
        // After a logout: back into the file.
        k.move("s1", inMemory = false)
        assertFalse(k.inMemory("s1"))
        assertEquals("session-1", tokens().get("s1"))
        // A logout forgets it wherever it is.
        k.put("s1", null, inMemory = false)
        assertNull(k.get("s1", inMemory = false))
        k.put("s2", "session-2", inMemory = true)
        k.forget("s2")
        assertNull(k.get("s2", inMemory = true))
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class FingerprintTest {
    // The SHA-256 of nothing, without the colons the server writes.
    private val emptyHex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

    @Test
    fun ofWritesTheServersForm() {
        val fp = Fingerprint.of(ByteArray(0))
        assertTrue(fp.startsWith("SHA256:"))
        assertEquals(emptyHex.uppercase(), fp.removePrefix("SHA256:").replace(":", ""))
        assertEquals(32, fp.removePrefix("SHA256:").split(':').size)
    }

    @Test
    fun canonicalTakesEveryForm() {
        val want = Fingerprint.of(ByteArray(0))
        assertEquals(want, Fingerprint.canonical(emptyHex))
        assertEquals(want, Fingerprint.canonical("sha256:" + emptyHex))
        assertEquals(want, Fingerprint.canonical(want.lowercase()))
        assertEquals(want, Fingerprint.canonical("  $want "))
    }

    @Test
    fun canonicalRefusesWhatIsNoFingerprint() {
        assertNull(Fingerprint.canonical(""))
        assertNull(Fingerprint.canonical(emptyHex.dropLast(2)))
        assertNull(Fingerprint.canonical(emptyHex.dropLast(1) + "g"))
        assertNull(Fingerprint.canonical("SHA1:" + emptyHex))
    }

    @Test
    fun sameComparesCertificatesNotSpellings() {
        val a = Fingerprint.of(byteArrayOf(1))
        val b = Fingerprint.of(byteArrayOf(2))
        assertTrue(Fingerprint.same(a, a.removePrefix("SHA256:").lowercase().replace(":", "")))
        assertFalse(Fingerprint.same(a, b))
        assertFalse(Fingerprint.same("", ""))
        assertFalse(Fingerprint.same(null, a))
        assertFalse(Fingerprint.same(a, null))
    }
}

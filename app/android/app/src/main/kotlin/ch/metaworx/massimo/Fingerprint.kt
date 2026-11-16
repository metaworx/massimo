// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.security.MessageDigest

/**
 * Certificate fingerprints as the server writes them (internal/certs,
 * `massimo tls show`): "SHA256:" and the SHA-256 of the certificate in DER
 * form, upper-case hex pairs separated by colons.
 */
object Fingerprint {
    private const val PREFIX = "SHA256:"
    private const val HEX = "0123456789ABCDEF"

    /** of is the fingerprint of a certificate in DER form. */
    fun of(der: ByteArray): String {
        val sum = MessageDigest.getInstance("SHA-256").digest(der)
        val b = StringBuilder(PREFIX)
        sum.forEachIndexed { i, byte ->
            if (i > 0) b.append(':')
            val v = byte.toInt() and 0xff
            b.append(HEX[v shr 4]).append(HEX[v and 0x0f])
        }
        return b.toString()
    }

    /**
     * canonical is [fp] in the server's form, or null where it is no
     * SHA-256 fingerprint; the prefix may be missing, the colons too, and
     * the case does not matter.
     */
    fun canonical(fp: String): String? {
        var s = fp.trim()
        if (s.startsWith(PREFIX, ignoreCase = true)) s = s.substring(PREFIX.length)
        s = s.replace(":", "")
        if (s.length != 64 || !s.all { it in '0'..'9' || it in 'a'..'f' || it in 'A'..'F' }) return null
        return PREFIX + s.uppercase().chunked(2).joinToString(":")
    }

    /** same reports whether [a] and [b] name one certificate; an empty or malformed one names none. */
    fun same(a: String?, b: String?): Boolean {
        val x = a?.let(::canonical) ?: return false
        return x == b?.let(::canonical)
    }
}

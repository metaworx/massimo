// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.Build
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyPermanentlyInvalidatedException
import android.security.keystore.KeyProperties
import java.security.InvalidKeyException
import java.security.KeyStore
import java.security.UnrecoverableKeyException
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * KeySource gives the ciphers of the Keystore keys behind the kept logins
 * (docs/app.md, "The login by fingerprint"): the app's one key,
 * [LoginKey.APP], which seals the app key ([AppKey]), and the key of its
 * own of each login kept before it, [LoginKey.alias]; a cipher of the
 * Keystore works only once the prompt has passed it, and for that one
 * operation.
 */
interface KeySource {
    /** encrypter is a cipher that seals for [alias], under a key made new for it. */
    fun encrypter(alias: String): Cipher

    /** decrypter is a cipher that opens what [alias] sealed with [iv]; [KeyInvalidated] where the key is gone. */
    fun decrypter(alias: String, iv: ByteArray): Cipher

    /** delete forgets the key of [alias]; one there is not is no error. */
    fun delete(alias: String)
}

/**
 * KeyInvalidated is a key the phone took back: a fingerprint enrolled on
 * Android 8 to 10, or the screen lock turned off on any version. What it
 * sealed opens no more.
 */
class KeyInvalidated : Exception()

/**
 * LoginKey names the keys and gives them from the Android Keystore:
 * AES-256-GCM that requires the prompt for every use, sealing included.
 * From Android 11 the prompt takes a strong biometric or the screen lock's
 * PIN, and a fingerprint enrolled later leaves the key; on Android 8 to 10
 * it takes the strong biometric alone, which a prompt with a key needs
 * there, and a fingerprint enrolled later invalidates the key.
 */
class LoginKey : KeySource {
    override fun encrypter(alias: String): Cipher {
        // A new key for every keep: one taken back before is gone with it.
        delete(alias)
        val c = Cipher.getInstance(TRANSFORMATION)
        c.init(Cipher.ENCRYPT_MODE, generate(alias))
        return c
    }

    override fun decrypter(alias: String, iv: ByteArray): Cipher {
        val key = try {
            keyStore().getKey(alias, null) as? SecretKey
        } catch (e: UnrecoverableKeyException) {
            null
        } ?: throw KeyInvalidated()
        val c = Cipher.getInstance(TRANSFORMATION)
        try {
            c.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(TAG_BITS, iv))
        } catch (e: KeyPermanentlyInvalidatedException) {
            throw KeyInvalidated()
        } catch (e: InvalidKeyException) {
            throw KeyInvalidated()
        }
        return c
    }

    override fun delete(alias: String) {
        val ks = keyStore()
        if (ks.containsAlias(alias)) ks.deleteEntry(alias)
    }

    private fun generate(alias: String): SecretKey {
        val spec = KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setKeySize(256)
            .setUserAuthenticationRequired(true)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            // Every use, by a strong biometric or the screen lock.
            spec.setUserAuthenticationParameters(0, KeyProperties.AUTH_BIOMETRIC_STRONG or KeyProperties.AUTH_DEVICE_CREDENTIAL)
        } else {
            // Every use (no validity duration), by the biometric alone.
            spec.setInvalidatedByBiometricEnrollment(true)
        }
        val gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        gen.init(spec.build())
        return gen.generateKey()
    }

    private fun keyStore(): KeyStore = KeyStore.getInstance(KEYSTORE).apply { load(null) }

    companion object {
        private const val KEYSTORE = "AndroidKeyStore"
        private const val TRANSFORMATION = "AES/GCM/NoPadding"
        private const val TAG_BITS = 128

        /**
         * APP is the Keystore alias of the app's one key, which seals the
         * app key every login is kept under (AP MobileApp, D63-A); no
         * server's id, which is 24 hex digits.
         */
        const val APP = "massimo.login.app"

        /**
         * alias is the Keystore alias of the key of the server [id], which
         * a login kept before the app key is sealed by until it is turned
         * off, forgotten or kept anew. [id] must have a server's form, 24
         * hex digits (ServerStore), so that no id a page sends names
         * [APP]: an IllegalArgumentException otherwise.
         */
        fun alias(id: String): String {
            require(SERVER_ID.matches(id)) { "not a server's id" }
            return "massimo.login.$id"
        }

        // SERVER_ID is the form of a server's id, as ServerStore makes it.
        private val SERVER_ID = Regex("[0-9a-f]{24}")
    }
}

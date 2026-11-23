// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.security.AlgorithmParameters
import java.security.Key
import java.security.Provider
import java.security.SecureRandom
import java.security.spec.AlgorithmParameterSpec
import java.util.IdentityHashMap
import javax.crypto.Cipher
import javax.crypto.CipherSpi
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * JvmKeys stands for the Keystore's keys in the JVM tests: the same AES-GCM,
 * with keys of the JVM's and no prompt of its own. It remembers the alias
 * each cipher is of, and [takeBack] drops its keys as the phone takes them
 * back (the screen lock turned off).
 */
class JvmKeys : KeySource {
    private val keys = HashMap<String, SecretKey>()
    private val of = IdentityHashMap<Cipher, String>()

    override fun encrypter(alias: String): Cipher {
        val k = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey()
        keys[alias] = k
        return Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, k) }.also { of[it] = alias }
    }

    override fun decrypter(alias: String, iv: ByteArray): Cipher {
        val k = keys[alias] ?: throw KeyInvalidated()
        return Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.DECRYPT_MODE, k, GCMParameterSpec(128, iv)) }.also { of[it] = alias }
    }

    override fun delete(alias: String) {
        keys.remove(alias)
    }

    /** aliases are the keys there are now. */
    fun aliases(): Set<String> = keys.keys.toSet()

    /** aliasOf is the alias [c] is a cipher of. */
    fun aliasOf(c: Cipher): String? = of[c]

    /** takeBack drops [aliases], or every key, as the phone takes them back. */
    fun takeBack(vararg aliases: String) {
        if (aliases.isEmpty()) keys.clear() else aliases.forEach { keys.remove(it) }
    }
}

/**
 * FakePrompt is the prompt of the tests: it notes each prompt, its title
 * and the alias of its key, and passes it, or dismisses it while [dismiss]
 * is set, at once; with [hold] set it keeps each prompt open until
 * [answer], the oldest first. [swap] hands back another cipher than the
 * one shown, as a Keystore operation that fails at its finish.
 */
class FakePrompt(private val keys: JvmKeys) : AppKey.Prompt {
    /** shown are the prompts shown, each "<title> <alias>". */
    val shown = mutableListOf<String>()
    var dismiss = false
    var hold = false
    var swap: ((Cipher) -> Cipher)? = null
    private val waiting = ArrayDeque<() -> Unit>()

    /** open reports how many prompts are held open. */
    val open: Int get() = waiting.size

    override fun ask(title: String, cipher: Cipher, done: (Cipher?) -> Unit) {
        shown.add("$title ${keys.aliasOf(cipher)}")
        val answer = { done(if (dismiss) null else swap?.invoke(cipher) ?: cipher) }
        if (hold) waiting.addLast(answer) else answer()
    }

    /** answer answers the oldest prompt held open. */
    fun answer() {
        val w = waiting.removeFirstOrNull() ?: throw AssertionError("no prompt open")
        w()
    }
}

/**
 * failing is a cipher whose finish throws [error], as a Keystore operation
 * does that fails after the prompt passed it: from a provider of the test's
 * own, initialised for decryption.
 */
fun failing(error: Exception): Cipher {
    val spi = object : CipherSpi() {
        override fun engineSetMode(mode: String?) {}

        override fun engineSetPadding(padding: String?) {}

        override fun engineGetBlockSize(): Int = 16

        override fun engineGetOutputSize(inputLen: Int): Int = inputLen

        override fun engineGetIV(): ByteArray = ByteArray(12)

        override fun engineGetParameters(): AlgorithmParameters? = null

        override fun engineInit(opmode: Int, key: Key?, random: SecureRandom?) {}

        override fun engineInit(opmode: Int, key: Key?, params: AlgorithmParameterSpec?, random: SecureRandom?) {}

        override fun engineInit(opmode: Int, key: Key?, params: AlgorithmParameters?, random: SecureRandom?) {}

        override fun engineUpdate(input: ByteArray?, inputOffset: Int, inputLen: Int): ByteArray = ByteArray(0)

        override fun engineUpdate(input: ByteArray?, inputOffset: Int, inputLen: Int, output: ByteArray?, outputOffset: Int): Int = 0

        override fun engineUpdateAAD(src: ByteArray?, offset: Int, len: Int) {}

        override fun engineDoFinal(input: ByteArray?, inputOffset: Int, inputLen: Int): ByteArray = throw error

        override fun engineDoFinal(input: ByteArray?, inputOffset: Int, inputLen: Int, output: ByteArray?, outputOffset: Int): Int = throw error
    }
    val provider = object : Provider("FailingKeystore", 1.0, "a Keystore that fails at the finish, for the tests") {
        init {
            putService(object : Provider.Service(this, "Cipher", "AES/GCM/NoPadding", "failing", null, null) {
                override fun newInstance(constructorParameter: Any?): Any = spi
            })
        }
    }
    return Cipher.getInstance("AES/GCM/NoPadding", provider).apply {
        init(Cipher.DECRYPT_MODE, SecretKeySpec(ByteArray(32), "AES"), GCMParameterSpec(128, ByteArray(12)))
    }
}

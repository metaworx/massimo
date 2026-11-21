// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.Build
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_STRONG
import androidx.biometric.BiometricManager.Authenticators.DEVICE_CREDENTIAL
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
import javax.crypto.Cipher

/**
 * Biometric asks for the fingerprint before the app uses a kept login
 * (docs/app.md, "The login by fingerprint"): a strong biometric, with the
 * screen lock's PIN as the prompt's fallback from Android 11, the one
 * version from which a prompt with a key takes it (androidx's reference);
 * on Android 8 to 10 the strong biometric alone. Made in the activity's
 * onCreate, as BiometricPrompt wants.
 */
class Biometric(private val activity: FragmentActivity, private val cancel: String) {
    private val credential = Build.VERSION.SDK_INT >= Build.VERSION_CODES.R
    private val authenticators = if (credential) BIOMETRIC_STRONG or DEVICE_CREDENTIAL else BIOMETRIC_STRONG

    private var pending: ((BiometricPrompt.AuthenticationResult?) -> Unit)? = null

    private val prompt = BiometricPrompt(
        activity,
        ContextCompat.getMainExecutor(activity),
        object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) = finish(result)

            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) = finish(null)
        },
    )

    /** asking reports whether a prompt shows now: the app is not hidden by it. */
    val asking: Boolean get() = pending != null

    /** canStrong reports whether the phone has a strong biometric enrolled, which the login by fingerprint needs. */
    fun canStrong(): Boolean = BiometricManager.from(activity).canAuthenticate(BIOMETRIC_STRONG) == BiometricManager.BIOMETRIC_SUCCESS

    /** unlock shows the prompt titled [title] without a key, for the cover; [done] learns whether it was passed. */
    fun unlock(title: String, done: (Boolean) -> Unit) = show(title, null) { done(it != null) }

    /**
     * withKey shows the prompt titled [title] for [cipher], a key's cipher
     * that works once the prompt passed it; [done] gets the cipher the
     * prompt authorised, or null where it was dismissed or failed.
     */
    fun withKey(title: String, cipher: Cipher, done: (Cipher?) -> Unit) =
        show(title, BiometricPrompt.CryptoObject(cipher)) { done(it?.cryptoObject?.cipher) }

    /** close drops the prompt's answer still waiting, as the activity goes, so that it holds nothing of the one that went. */
    fun close() {
        pending = null
    }

    // show shows the prompt; a second while one shows is refused, as
    // dismissed. So is one while the activity's fragments have their state
    // saved (the activity stopped, Home pressed between two prompts of
    // Forget all): androidx.biometric then shows none and calls no callback,
    // which would leave the answer waiting for ever. That is the library's
    // own test, not the lifecycle's state, which is still CREATED in
    // onStart, where the cover asks.
    private fun show(title: String, crypto: BiometricPrompt.CryptoObject?, done: (BiometricPrompt.AuthenticationResult?) -> Unit) {
        if (pending != null || activity.supportFragmentManager.isStateSaved) {
            done(null)
            return
        }
        pending = done
        val info = BiometricPrompt.PromptInfo.Builder()
            .setTitle(title)
            .setAllowedAuthenticators(authenticators)
        if (!credential) info.setNegativeButtonText(cancel)
        try {
            if (crypto != null) prompt.authenticate(info.build(), crypto) else prompt.authenticate(info.build())
        } catch (e: IllegalArgumentException) {
            finish(null)
        }
    }

    private fun finish(result: BiometricPrompt.AuthenticationResult?) {
        val d = pending
        pending = null
        d?.invoke(result)
    }
}

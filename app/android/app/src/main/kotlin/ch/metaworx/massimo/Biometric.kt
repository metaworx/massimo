// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.Build
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_STRONG
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_WEAK
import androidx.biometric.BiometricManager.Authenticators.DEVICE_CREDENTIAL
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity

/**
 * Biometric asks for the fingerprint, the face or the screen lock's PIN
 * before the app hands a server's token to its page. Made in the
 * activity's onCreate, as BiometricPrompt wants.
 */
class Biometric(private val activity: FragmentActivity) {
    // A strong biometric or the screen lock from Android 11; before it the
    // library takes the screen lock only beside a weak one.
    private val authenticators =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) BIOMETRIC_STRONG or DEVICE_CREDENTIAL else BIOMETRIC_WEAK or DEVICE_CREDENTIAL

    private var pending: ((Boolean) -> Unit)? = null

    private val prompt = BiometricPrompt(
        activity,
        ContextCompat.getMainExecutor(activity),
        object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) = finish(true)

            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) = finish(false)
        },
    )

    /** available reports whether the phone has a screen lock or a biometric the prompt can ask for. */
    fun available(): Boolean =
        BiometricManager.from(activity).canAuthenticate(authenticators) == BiometricManager.BIOMETRIC_SUCCESS

    /** ask shows the prompt titled [title]; [done] learns whether it was passed. A second prompt while one shows is refused. */
    fun ask(title: String, done: (Boolean) -> Unit) {
        if (pending != null || !available()) {
            done(false)
            return
        }
        pending = done
        prompt.authenticate(
            BiometricPrompt.PromptInfo.Builder()
                .setTitle(title)
                .setAllowedAuthenticators(authenticators)
                .build(),
        )
    }

    private fun finish(ok: Boolean) {
        val d = pending
        pending = null
        d?.invoke(ok)
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.security.keystore.KeyPermanentlyInvalidatedException
import org.json.JSONException
import java.security.GeneralSecurityException
import java.security.SecureRandom
import javax.crypto.AEADBadTagException
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * AppKey is the one key the app keeps its logins under (docs/app.md, "The
 * login by fingerprint"; AP MobileApp, D63-A): 256 random bits, sealed by
 * the Keystore key [LoginKey.APP], which asks for the prompt at every use,
 * and kept sealed beside the logins ([LoginStore.appKey]). Each login is
 * sealed under it in software, AES-GCM bound to its server's id, node and
 * pinned fingerprint and its user ([LoginStore.aad]), so that it opens for
 * its own server alone.
 *
 * A prompt opens the app key for one [Step]: keeping a login, opening one,
 * turning one off, or Forget all, which opens every login it needs with it;
 * the step's end drops it, its bytes cleared (the cipher of an operation
 * holds a copy of its own, which goes with it). A login kept before the app
 * key has a Keystore key of its own ([LoginKey.alias]) and asks with it,
 * until it is turned off, forgotten or kept anew. A key the phone takes back
 * (a fingerprint enrolled on Android 8 to 10, the screen lock turned off)
 * opens nothing sealed under it any more: every login under the app key is
 * then forgotten through [takenBack], which leaves each key by its id
 * (D57-A), and the next keep makes a new app key. Any other failure of the
 * Keystore forgets nothing.
 *
 * [prompt] shows the prompt and answers on a thread where requests may go;
 * a step answers there, or on its caller's thread where it needs no prompt.
 */
class AppKey(
    private val logins: LoginStore,
    private val keys: KeySource,
    private val prompt: Prompt,
    private val takenBack: (String, KeptLogin) -> Unit,
    private val random: SecureRandom = SecureRandom(),
) {
    /**
     * Prompt shows the prompt titled [title] for [cipher], a Keystore cipher
     * that works once the prompt passed it, and calls [done] with the cipher
     * it authorised, or null where it was dismissed or could not show
     * (Biometric.withKey).
     */
    fun interface Prompt {
        fun ask(title: String, cipher: Cipher, done: (Cipher?) -> Unit)
    }

    /** Result is how a step's opening or keeping of a login went. */
    sealed interface Result {
        /** Opened: the login opened, its key in [secret]. */
        data class Opened(val secret: LoginSecret) : Result

        /** Kept: the login is kept, sealed under the app key. */
        data object Kept : Result

        /**
         * Dismissed: the prompt was dismissed or could not show, earlier in
         * the step for the app key, or the step ended meanwhile, or another
         * keep is making the app key; nothing changed.
         */
        data object Dismissed : Result

        /** Gone: the key that sealed the login was taken back, or the login opens no more for its server. */
        data object Gone : Result

        /** Failed: the Keystore failed otherwise; nothing changed. */
        data class Failed(val error: Exception) : Result
    }

    // The steps begun and not yet ended, for dropAll.
    private val steps = HashSet<Step>()

    // making: a keep is making the app key, from its Keystore key made new
    // to the sealed app key kept, guarded by this; one at a time, so that a
    // second keep does not take the first one's Keystore key away.
    private var making = false

    /** step begins a step, which its caller ends with [Step.close]. */
    fun step(): Step = Step().also { synchronized(steps) { steps.add(it) } }

    /**
     * open opens [kept], the login of the server [id], in a step of its own
     * (Turn off's, a login's): behind one prompt titled [title], the step
     * ended before [then] learns how it went.
     */
    fun open(title: String, id: String, kept: KeptLogin, then: (Result) -> Unit) {
        val step = step()
        step.open(title, id, kept) { r ->
            step.close()
            then(r)
        }
    }

    /** held reports whether a step holds the app key open now; none does between steps. */
    internal val held: Boolean get() = synchronized(steps) { steps.toList() }.any { it.holds }

    /**
     * dropAll drops the app key of every step begun, the activity having
     * gone: what a step still waits for (a prompt's answer) finds it ended.
     */
    fun dropAll() {
        synchronized(steps) { steps.toList() }.forEach { it.drop() }
    }

    /**
     * Step is one step of the kept logins: its first login under the app key
     * opens the app key behind the prompt, or a keep makes it, and every
     * later one in the step opens from it; a login kept before the app key
     * asks with its own key. A prompt for the app key dismissed in the step
     * asks no more in it: every later login under the app key is answered
     * as dismissed. [close] ends it: the app key dropped, its bytes cleared,
     * and where the step found the app key taken back, every login under it
     * forgotten.
     */
    inner class Step internal constructor() : AutoCloseable {
        private var key: ByteArray? = null
        private var ended = false

        // refused: the app key's prompt was dismissed in the step
        private var refused = false

        // lost: the step found the app key taken back, sealed as lostKey
        // was then (null for none kept), which close forgets with its logins
        private var lost = false
        private var lostKey: SealedKey? = null

        internal val holds: Boolean get() = synchronized(this) { key != null }

        /**
         * open opens [kept], the login of the server [id], and calls [done]
         * with how it went: under the app key from the step's app key, which
         * the first such opening opens behind the prompt titled [title]; a
         * login kept before with its own key behind that prompt. A login
         * kept without its key's revoke code is given it once opened.
         */
        fun open(title: String, id: String, kept: KeptLogin, done: (Result) -> Unit) {
            // a login kept before contract version 6 takes its key's revoke code at its unseal (D112-A)
            val then = { r: Result ->
                if (r is Result.Opened && kept.code.isEmpty()) runCatching { logins.setCode(id, kept, RevokeCode.of(r.secret.key)) }
                done(r)
            }
            if (!kept.byApp) {
                own(title, id, kept, then)
                return
            }
            val now = synchronized(this) {
                when {
                    ended || refused -> Result.Dismissed
                    lost -> Result.Gone
                    else -> key?.let { openWith(it, id, kept) }
                }
            }
            if (now != null) {
                then(now)
                return
            }
            unlock(title) { failed -> then(failed ?: synchronized(this) { key?.let { openWith(it, id, kept) } } ?: Result.Dismissed) }
        }

        /**
         * keep keeps [secret], the key of [user]'s at the server [id] of
         * [node] and pinned [fp], sealed under the app key, and calls [then]
         * with how it went: the step's app key, else the one kept opened
         * behind the prompt titled [title], else, the first time or after the
         * phone took it back, a new one made behind it, by one keep at a
         * time. A key of its own the server's login had before goes.
         */
        fun keep(title: String, id: String, node: String, fp: String, user: String, secret: LoginSecret, then: (Result) -> Unit) {
            val now = synchronized(this) {
                when {
                    ended -> Result.Dismissed
                    else -> key?.let { sealWith(it, id, node, fp, user, secret) }
                }
            }
            if (now != null) {
                then(now)
                return
            }
            val seal = { failed: Result? -> then(failed ?: synchronized(this) { key?.let { sealWith(it, id, node, fp, user, secret) } } ?: Result.Dismissed) }
            val sealed = logins.appKey()
            val cipher = try {
                sealed?.let { keys.decrypter(LoginKey.APP, it.iv) }
            } catch (e: KeyInvalidated) {
                null
            } catch (e: Exception) {
                then(Result.Failed(e))
                return
            }
            if (sealed != null && cipher != null) {
                ask(title, sealed, cipher) { failed ->
                    // taken back after the prompt passed: the next keep makes one
                    if (failed == Result.Gone) forgetLost(sealed)
                    seal(if (failed == Result.Gone) Result.Failed(KeyInvalidated()) else failed)
                }
                return
            }
            // None yet, or taken back: one keep at a time makes the app key,
            // the logins under the one taken back forgotten first; where
            // another made one meanwhile, the keep opens that one.
            var retry = false
            val start: Any = synchronized(this@AppKey) {
                when {
                    making -> Result.Dismissed
                    logins.appKey() != sealed -> {
                        retry = true
                        Result.Dismissed
                    }
                    else -> {
                        forgetLost(sealed)
                        try {
                            keys.encrypter(LoginKey.APP).also { making = true }
                        } catch (e: Exception) {
                            Result.Failed(e)
                        }
                    }
                }
            }
            when {
                retry -> keep(title, id, node, fp, user, secret, then)
                start is Cipher -> make(title, start, seal)
                else -> then(start as Result)
            }
        }

        /** close ends the step: see [Step]. */
        override fun close() {
            drop()
            val forget = synchronized(this) {
                val l = lost
                lost = false
                l
            }
            if (forget) forgetLost(synchronized(this) { lostKey })
        }

        // drop drops the step's app key, its bytes cleared; what the step
        // still waits for finds it ended.
        internal fun drop() {
            synchronized(this) {
                ended = true
                key?.fill(0)
                key = null
            }
            synchronized(steps) { steps.remove(this) }
        }

        // own opens a login kept before the app key, with its own key behind
        // the prompt.
        private fun own(title: String, id: String, kept: KeptLogin, then: (Result) -> Unit) {
            if (synchronized(this) { ended }) {
                then(Result.Dismissed)
                return
            }
            val cipher = try {
                keys.decrypter(LoginKey.alias(id), kept.iv)
            } catch (e: KeyInvalidated) {
                then(Result.Gone)
                return
            } catch (e: Exception) {
                then(Result.Failed(e))
                return
            }
            prompt.ask(title, cipher) { c -> then(if (c == null) Result.Dismissed else openingOwn { LoginStore.open(c, kept, id) }) }
        }

        // unlock opens the app key kept behind the prompt into the step:
        // done(null), or how it failed; a key taken back marks the step
        // lost, a dismissed prompt refused.
        private fun unlock(title: String, done: (Result?) -> Unit) {
            val sealed = logins.appKey()
            val cipher = try {
                sealed?.let { keys.decrypter(LoginKey.APP, it.iv) }
            } catch (e: KeyInvalidated) {
                null
            } catch (e: Exception) {
                done(Result.Failed(e))
                return
            }
            if (sealed == null || cipher == null) {
                lose(sealed)
                done(Result.Gone)
                return
            }
            ask(title, sealed, cipher) { failed ->
                when (failed) {
                    Result.Gone -> lose(sealed)
                    Result.Dismissed -> synchronized(this) { refused = true }
                    else -> Unit
                }
                done(failed)
            }
        }

        // ask shows the prompt for [cipher], the decrypter of [sealed], and
        // keeps the app key it opens in the step: done(null), or how it
        // failed: Gone where the Keystore says the key opens what it sealed
        // no more (takenBack), Failed for any other failure, which forgets
        // nothing.
        private fun ask(title: String, sealed: SealedKey, cipher: Cipher, done: (Result?) -> Unit) {
            prompt.ask(title, cipher) { c ->
                if (c == null) {
                    done(Result.Dismissed)
                    return@ask
                }
                val opened = try {
                    c.updateAAD(LoginStore.APP_AAD)
                    c.doFinal(sealed.sealed)
                } catch (e: Exception) {
                    done(if (takenBack(e)) Result.Gone else Result.Failed(e))
                    return@ask
                }
                if (opened.size != KEY_BYTES) {
                    opened.fill(0)
                    done(Result.Gone)
                    return@ask
                }
                done(if (hold(opened)) null else Result.Dismissed)
            }
        }

        // make makes a new app key behind the prompt for [cipher], the
        // encrypter of the Keystore key made new for it, seals it, keeps it
        // sealed and in the step, and ends the making whatever comes.
        private fun make(title: String, cipher: Cipher, done: (Result?) -> Unit) {
            prompt.ask(title, cipher) { c ->
                val result = try {
                    if (c == null) {
                        Result.Dismissed
                    } else {
                        val fresh = ByteArray(KEY_BYTES).also { random.nextBytes(it) }
                        try {
                            c.updateAAD(LoginStore.APP_AAD)
                            logins.putAppKey(SealedKey(c.iv, c.doFinal(fresh)))
                            if (hold(fresh)) null else Result.Dismissed
                        } catch (e: Exception) {
                            fresh.fill(0)
                            Result.Failed(e)
                        }
                    }
                } finally {
                    synchronized(this@AppKey) { making = false }
                }
                done(result)
            }
        }

        // hold keeps [opened] as the step's app key, unless the step ended
        // meanwhile, which clears it.
        private fun hold(opened: ByteArray): Boolean = synchronized(this) {
            if (ended) {
                opened.fill(0)
                false
            } else {
                key?.fill(0)
                key = opened
                true
            }
        }

        private fun lose(sealed: SealedKey?) = synchronized(this) {
            lost = true
            lostKey = sealed
        }

        private fun openWith(key: ByteArray, id: String, kept: KeptLogin): Result =
            opening { LoginStore.open(software(Cipher.DECRYPT_MODE, key, kept.iv), kept, id) }

        private fun sealWith(key: ByteArray, id: String, node: String, fp: String, user: String, secret: LoginSecret): Result = try {
            val iv = ByteArray(IV_BYTES).also { random.nextBytes(it) }
            logins.put(id, LoginStore.seal(software(Cipher.ENCRYPT_MODE, key, iv), secret, id, node, fp, user, byApp = true))
            runCatching { keys.delete(LoginKey.alias(id)) }
            Result.Kept
        } catch (e: Exception) {
            Result.Failed(e)
        }
    }

    // forgetLost forgets every login under the app key that the phone took
    // back, sealed as [sealed] was (null: none was kept), each through
    // takenBack, and the app key with its Keystore key; nothing where
    // another app key is kept since, or is being made, a keep making it
    // having forgotten them first. A failure stops none of the others.
    private fun forgetLost(sealed: SealedKey?) = synchronized(this) {
        if (making || logins.appKey() != sealed) return@synchronized
        for (id in logins.ids()) {
            val kept = logins.get(id) ?: continue
            if (kept.byApp) runCatching { takenBack(id, kept) }
        }
        runCatching { logins.forgetAppKey() }
        runCatching { keys.delete(LoginKey.APP) }
    }

    private companion object {
        const val KEY_BYTES = 32
        const val IV_BYTES = 12
        const val TAG_BITS = 128

        // software is a cipher of AES-GCM under the app key, in the app's
        // memory: the Keystore holds the key that seals it, not it.
        fun software(mode: Int, key: ByteArray, iv: ByteArray): Cipher =
            Cipher.getInstance("AES/GCM/NoPadding").apply { init(mode, SecretKeySpec(key, "AES"), GCMParameterSpec(TAG_BITS, iv)) }

        // takenBack reports whether [e], a Keystore operation's failure at
        // its finish, says that the key opens what it sealed no more: a bad
        // tag, or the key invalidated, itself or as a cause. Android raises
        // every other failure of a finish (a key user not authenticated, an
        // operation pruned) as an IllegalBlockSizeException, which passes.
        fun takenBack(e: Throwable): Boolean {
            var c: Throwable? = e
            var depth = 0
            while (c != null && depth++ < MAX_CAUSES) {
                if (c is AEADBadTagException || c is KeyPermanentlyInvalidatedException || c is KeyInvalidated) return true
                c = c.cause
            }
            return false
        }

        const val MAX_CAUSES = 8

        // opening opens a login under the app key, in software: Gone where
        // it does not open for its server.
        fun opening(open: () -> LoginSecret): Result = try {
            Result.Opened(open())
        } catch (e: GeneralSecurityException) {
            Result.Gone
        } catch (e: JSONException) {
            Result.Gone
        } catch (e: Exception) {
            Result.Failed(e)
        }

        // openingOwn opens a login kept before the app key, with its own
        // Keystore key: Gone where the Keystore says the key opens it no
        // more, or what it opened is no login; Failed for any other failure.
        fun openingOwn(open: () -> LoginSecret): Result = try {
            Result.Opened(open())
        } catch (e: JSONException) {
            Result.Gone
        } catch (e: Exception) {
            if (takenBack(e)) Result.Gone else Result.Failed(e)
        }
    }
}

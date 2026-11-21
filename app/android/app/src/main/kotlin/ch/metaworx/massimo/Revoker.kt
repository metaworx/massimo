// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.util.concurrent.Executor
import java.util.concurrent.RejectedExecutionException

/**
 * Revoker turns the phone's kept logins off at their servers (docs/app.md,
 * "The login by fingerprint"): Turn off of one server (`forget`), Remove of
 * a server with a kept login (`remove`) and Forget all stored credentials
 * (`forgetAll`) alike (AP MobileApp, D56-A). Each key is revoked with the
 * session the app holds there, without a prompt, else with the key itself
 * after the prompt once ([ByKey]), never over plain HTTP outside the
 * private ranges; then the session is signed out and the login forgotten on
 * the phone ([forgetLogin], with its Keystore key). A key with an id that
 * could not be revoked is left in [left] first, to revoke with the next
 * session the app holds there (D57-A); one without an id is forgotten on the
 * phone alone.
 *
 * It holds no thread of its own: a caller runs it where requests may go,
 * [ByKey] calls back there too, and Forget all goes on to the next server
 * on [run]; where [run] takes nothing more (the activity went), Forget all
 * stops quietly where it is.
 */
class Revoker(
    private val servers: ServerStore,
    private val logins: LoginStore,
    private val sessions: Sessions,
    private val left: KeysLeft,
    private val probe: (List<String>, String?) -> List<ProbeResult>,
    private val run: Executor,
    private val forgetLogin: (String) -> Unit,
) {
    /**
     * ByKey opens a kept login's key after the prompt: it calls [then] with
     * the key, or with null where the prompt was dismissed or could not show
     * or the key does not open, on a thread where requests may go.
     */
    fun interface ByKey {
        fun open(s: Server, kept: KeptLogin, then: (LoginSecret?) -> Unit)
    }

    /** Outcome is how a kept login's key went. */
    enum class Outcome {
        /** The server revoked it, or had already. */
        REVOKED,

        /** Not revoked, and left by its id in the server's `unrevoked`, to revoke with the next session. */
        LEFT,

        /** Not revoked, and forgotten on the phone alone: listed at the server under My profile, to revoke there. */
        LOCAL,
    }

    /** Off is what turning one login off came to, and the id of its key, 0 where it is not known. */
    data class Off(val outcome: Outcome, val keyId: Long)

    /**
     * Summary is what Forget all came to, by server id: the servers whose key
     * was revoked, those whose key was left in `unrevoked`, and those whose
     * key was forgotten on the phone alone.
     */
    data class Summary(val revoked: List<String>, val left: List<String>, val local: List<String>)

    /**
     * turnOff turns the login kept for the server [id] off at [url], the
     * address of the server's that answered, or, for "" or another URL, at
     * the address that answers now with its node, the LAN one first, as
     * Forget all asks; and calls [done] with what it came to. A login that
     * is not the server's any more is forgotten on the phone; its key is
     * left by its id where the server is the same node (a certificate
     * trusted anew), and is another server's where it is not.
     */
    fun turnOff(id: String, url: String, byKey: ByKey, done: (Off) -> Unit) {
        val s = servers.get(id)
        val kept = logins.get(id)
        if (s == null || kept == null) {
            forgetHere(id, s, url)
            done(Off(Outcome.LOCAL, 0))
            return
        }
        if (!matches(s, kept)) {
            done(forgetMismatched(s, kept))
            return
        }
        off(s, kept, if (s.has(url)) url else answering(s), byKey, done)
    }

    /**
     * forgetAll turns every kept login off as [turnOff] does, one server
     * after the other in the list's order, so that one prompt shows at a
     * time: each at the address of the server's that answers with its node,
     * the LAN one first. It calls [done] with what it came to.
     */
    fun forgetAll(byKey: ByKey, done: (Summary) -> Unit) {
        val kept = logins.ids().toSet()
        val listed = servers.list().map { it.id }
        val ids = ArrayDeque(listed.filter { it in kept } + kept.filter { it !in listed })
        val went = mapOf(Outcome.REVOKED to mutableListOf<String>(), Outcome.LEFT to mutableListOf(), Outcome.LOCAL to mutableListOf())
        fun next() {
            val id = ids.removeFirstOrNull()
            if (id == null) {
                done(Summary(went.getValue(Outcome.REVOKED), went.getValue(Outcome.LEFT), went.getValue(Outcome.LOCAL)))
                return
            }
            val s = servers.get(id)
            val login = logins.get(id)
            if (s == null || login == null) {
                forgetHere(id, s, "")
                later(::next)
                return
            }
            if (!matches(s, login)) {
                went.getValue(forgetMismatched(s, login).outcome).add(id)
                later(::next)
                return
            }
            off(s, login, answering(s), byKey) { o ->
                went.getValue(o.outcome).add(id)
                // the next server from the executor, not from inside the
                // prompt's answer, so that each server leaves the stack
                later(::next)
            }
        }
        next()
    }

    // later runs [step] on [run]; where [run] takes nothing more, the
    // activity having gone, nothing more is done.
    private fun later(step: () -> Unit) {
        try {
            run.execute(step)
        } catch (e: RejectedExecutionException) {
            // the app is closing: what is left stays as it is
        }
    }

    // answering is the address of [s] that answers now with its node, the
    // LAN one first, "" for none.
    private fun answering(s: Server): String {
        val urls = listOf(s.lan, s.remote).filter { it.isNotEmpty() }
        return Addresses.answering(s, runCatching { probe(urls, s.fp.ifEmpty { null }) }.getOrDefault(emptyList())) ?: ""
    }

    // forgetMismatched forgets a login that is not the server's any more:
    // its key left by its id where the server is the same node, its
    // certificate trusted anew (as Bridge's forgetLeavingKey leaves it),
    // else forgotten on the phone alone, being another server's.
    private fun forgetMismatched(s: Server, kept: KeptLogin): Off {
        val same = kept.node == s.node
        val leftThere = same && runCatching { left.add(s.id, kept.user, kept.keyId) }.getOrDefault(false)
        forgetHere(s.id, s, "")
        return Off(if (leftThere) Outcome.LEFT else Outcome.LOCAL, if (same) kept.keyId else 0)
    }

    // off revokes the key of [kept] at [url] with the session held, else
    // with the key after the prompt; then leaves a key it could not revoke,
    // signs the session out and forgets the login.
    private fun off(s: Server, kept: KeptLogin, url: String, byKey: ByKey, done: (Off) -> Unit) {
        if (!s.has(url)) {
            finish(s, kept, "", false, kept.keyId, done)
            return
        }
        val login = kept.copy(keyId = runCatching { sessions.keyOf(s, url, kept) }.getOrDefault(kept.keyId))
        when (runCatching { sessions.revokeWithSession(s, url, login) }.getOrDefault(Sessions.Revoke.FAILED)) {
            Sessions.Revoke.REVOKED -> finish(s, login, url, true, login.keyId, done)
            Sessions.Revoke.FAILED -> finish(s, login, url, false, login.keyId, done)
            Sessions.Revoke.BY_KEY -> {
                if (Origins.plainOutside(url)) {
                    finish(s, login, url, false, login.keyId, done)
                    return
                }
                byKey.open(s, login) { secret ->
                    if (secret == null) {
                        finish(s, login, url, false, login.keyId, done)
                    } else {
                        val revoked = runCatching { sessions.revokeWithKey(s, url, login.user, secret) }.getOrDefault(false)
                        finish(s, login, url, revoked, if (login.keyId > 0) login.keyId else secret.id, done)
                    }
                }
            }
        }
    }

    // finish leaves a key not revoked, by its id, while the server still
    // has the login; then signs the session out at [url] and forgets the
    // login, the session first, so that one kept in memory for the login is
    // found and ended at the server rather than forgotten on the phone alone
    // (F28).
    private fun finish(s: Server, kept: KeptLogin, url: String, revoked: Boolean, keyId: Long, done: (Off) -> Unit) {
        val outcome = when {
            revoked -> Outcome.REVOKED
            runCatching { left.add(s.id, kept.user, keyId) }.getOrDefault(false) -> Outcome.LEFT
            else -> Outcome.LOCAL
        }
        forgetHere(s.id, s, url)
        done(Off(outcome, keyId))
    }

    // forgetHere signs the session of the server [id] out at [url] (forgets
    // it where [s] is gone), then forgets the login; neither failing stops
    // the other.
    private fun forgetHere(id: String, s: Server?, url: String) {
        runCatching { if (s != null) sessions.signOut(s, url) else sessions.forget(id) }
        runCatching { forgetLogin(id) }
    }

    companion object {
        /**
         * matches reports whether [kept] is still the login of [s]: its node
         * and its pinned fingerprint as they were when it was kept.
         */
        fun matches(s: Server, kept: KeptLogin): Boolean =
            kept.node == s.node && (Fingerprint.same(kept.fp, s.fp) || (kept.fp.isEmpty() && s.fp.isEmpty()))
    }
}

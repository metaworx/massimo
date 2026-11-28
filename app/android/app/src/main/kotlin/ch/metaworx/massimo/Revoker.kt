// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.util.concurrent.Executor
import java.util.concurrent.RejectedExecutionException

/**
 * Revoker turns the phone's kept logins off at their servers (docs/app.md,
 * "The login by fingerprint"): Turn off of one server (`forget`), Remove of
 * a server (`remove`), which signs out first where no login is kept, and
 * Forget all stored credentials (`forgetAll`) alike (AP MobileApp, D56-A). Each key is revoked with the
 * session the app holds there, without a prompt, else with the key itself
 * after the prompt ([ByKey]), never over plain HTTP outside the private
 * ranges: one prompt a run for every login under the app key, so that Forget
 * all asks once (D63-A), and one for each login kept before it ([AppKey]);
 * then the session is signed out and the login forgotten on the phone
 * ([forgetLogin], with its Keystore key). A key with an id that
 * could not be revoked is left in [left] first, to revoke with the next
 * session the app holds there (D57-A); one without an id is forgotten on the
 * phone alone.
 *
 * It holds no thread of its own: a caller runs it where requests may go,
 * [ByKey] calls back there too, and Forget all goes on to the next server
 * on [run]; where [run] takes nothing more (the activity went), Forget all
 * stops quietly where it is. A run ends its [ByKey] ([ByKey.close]) once it
 * is done or stops.
 */
class Revoker(
    private val servers: ServerStore,
    private val logins: LoginStore,
    private val sessions: Sessions,
    private val left: KeysLeft,
    private val probe: (List<String>, String?) -> List<ProbeResult>,
    private val run: Executor,
    private val codes: Codes? = null,
    private val gone: GoneStore? = null,
    private val log: Logger = Logger {},
    private val forgetLogin: (String) -> Unit,
) {
    /**
     * ByKey opens a kept login's key after the prompt: it calls [then] with
     * the key, or with null where the prompt was dismissed or could not show
     * or the key does not open, on a thread where requests may go.
     */
    fun interface ByKey {
        fun open(s: Server, kept: KeptLogin, then: (LoginSecret?) -> Unit)

        /** close ends the run's opening: what it opened (the app key) is dropped. */
        fun close() {}
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

    /**
     * Off is what turning one login off came to, and the id of its key, 0
     * where it is not known; [pending] says the key it left carries its
     * revoke code, which the next contact posts (contract version 6).
     */
    data class Off(val outcome: Outcome, val keyId: Long, val pending: Boolean = false)

    /**
     * Summary is what Forget all came to, by server id: the servers whose key
     * was revoked, those whose key was left in `unrevoked`, and those whose
     * key was forgotten on the phone alone; [pending] those among the left
     * whose key carries its revoke code.
     */
    data class Summary(val revoked: List<String>, val left: List<String>, val local: List<String>, val pending: List<String> = emptyList())

    /**
     * Removed is what Remove came to: how the kept login was turned off,
     * null where none was kept, and the ids of the phone's keys and
     * sessions still valid at the server, which goes with what noted them;
     * [pending] says their revoke codes wait in `gone`, to be posted where the
     * server is found again (D112-A, D116-A).
     */
    data class Removed(val off: Off?, val keys: List<Long>, val sessions: List<Long>, val pending: Boolean = false)

    /**
     * turnOff turns the login kept for the server [id] off at [url], the
     * address of the server's that answered, or, for "" or another URL, at
     * the address that answers now with its node, the LAN one first, as
     * Forget all asks; and calls [done] with what it came to. A login that
     * is not the server's any more is forgotten on the phone; its key is
     * left by its id where the server is the same node (a certificate
     * trusted anew), and is another server's where it is not. [byKey]
     * is closed before [done] is called, or where a step throws.
     */
    fun turnOff(id: String, url: String, byKey: ByKey, done: (Off) -> Unit) = turnOffAt(id, url, true, byKey, done)

    // turnOffAt is turnOff; without [probe] it looks for no address where
    // [url] is not one of the server's, its caller having looked (Remove).
    private fun turnOffAt(id: String, url: String, probe: Boolean, byKey: ByKey, done: (Off) -> Unit) {
        val end = { o: Off ->
            byKey.close()
            done(o)
        }
        try {
            val s = servers.get(id)
            val kept = logins.get(id)
            if (s == null || kept == null) {
                forgetHere(id, s, url)
                end(Off(Outcome.LOCAL, 0))
                return
            }
            if (!matches(s, kept)) {
                end(forgetMismatched(s, kept))
                return
            }
            off(s, kept, if (s.has(url)) url else if (probe) answering(s) else "", byKey, end)
        } catch (e: Exception) {
            byKey.close()
            throw e
        }
    }

    /**
     * remove forgets the server [id] (Remove): a kept login is turned off
     * first as [turnOff] turns it off, at [url] or the address that answers
     * (D58-A); else the session held is signed out there, so that it is not
     * left valid at the server unrecorded (F38, D69-A), one the server does
     * not end left by its id as [Sessions.signOut] leaves it; then the
     * server goes, with its certificate, its session, its login and what it
     * noted of the keys and sessions left there, whose ids [done] gets with
     * the outcome, or the failure of forgetting the server. Their revoke
     * codes are posted where the server answered, and those still to post
     * kept in [gone] (D112-A). [byKey] is closed before [done] is called, or
     * where a step throws.
     */
    fun remove(id: String, url: String, byKey: ByKey, done: (Result<Removed>) -> Unit) {
        val drop = { off: Off?, at: String ->
            done(
                runCatching {
                    val s = servers.get(id)
                    val posted = s?.let { server -> codes?.let { c -> runCatching { c.removing(server, at) }.getOrNull() } }
                    val taken = posted?.taken ?: emptySet()
                    val waiting = posted?.waiting ?: emptyList()
                    // what is still valid there: not taken before nor now
                    val keys = s?.unrevoked?.filter { !it.taken && it.code !in taken }?.map { it.id } ?: emptyList()
                    val unended = s?.unended?.filter { !it.taken && it.code !in taken }?.map { it.id } ?: emptyList()
                    // pending only where the codes were kept; a failure said without them
                    val kept = s != null && waiting.isNotEmpty() && gone != null &&
                        runCatching { gone?.add(s, waiting) }.onFailure { e -> log.warn("remove: the revoke codes of $id were not kept: ${e.javaClass.simpleName}") }.isSuccess
                    servers.remove(id)
                    sessions.forget(id)
                    forgetLogin(id)
                    Removed(off, keys, unended, kept)
                },
            )
        }
        val s = servers.get(id)
        if (logins.get(id) != null) {
            // the address looked for once, for the turn-off and the codes
            val at = s?.let { if (it.has(url)) url else answering(it) } ?: ""
            turnOffAt(id, at, false, byKey) { off -> drop(off, at) }
            return
        }
        byKey.close()
        var at = ""
        if (s != null && (sessions.held(s) != null || s.codes().isNotEmpty())) at = if (s.has(url)) url else answering(s)
        if (s != null && sessions.held(s) != null) runCatching { sessions.signOut(s, at) }
        drop(null, at)
    }

    /**
     * forgetAll turns every kept login off as [turnOff] does, one server
     * after the other in the list's order, so that one prompt shows at a
     * time: each at the address of the server's that answers with its node,
     * the LAN one first. It calls [done] with what it came to, [byKey]
     * closed first, and closes it where it stops, a step that throws
     * included.
     */
    fun forgetAll(byKey: ByKey, done: (Summary) -> Unit) {
        val kept = logins.ids().toSet()
        val listed = servers.list().map { it.id }
        val ids = ArrayDeque(listed.filter { it in kept } + kept.filter { it !in listed })
        val went = mapOf(Outcome.REVOKED to mutableListOf<String>(), Outcome.LEFT to mutableListOf(), Outcome.LOCAL to mutableListOf())
        val pending = mutableListOf<String>()
        // next turns the next server's login off; on io through later, so
        // that a step that throws ends the run's ByKey before it goes on up.
        fun next() {
            try {
                val id = ids.removeFirstOrNull()
                if (id == null) {
                    byKey.close()
                    done(Summary(went.getValue(Outcome.REVOKED), went.getValue(Outcome.LEFT), went.getValue(Outcome.LOCAL), pending))
                    return
                }
                val s = servers.get(id)
                val login = logins.get(id)
                if (s == null || login == null) {
                    forgetHere(id, s, "")
                    later(byKey, ::next)
                    return
                }
                if (!matches(s, login)) {
                    val o = forgetMismatched(s, login)
                    went.getValue(o.outcome).add(id)
                    if (o.pending) pending.add(id)
                    later(byKey, ::next)
                    return
                }
                off(s, login, answering(s), byKey) { o ->
                    went.getValue(o.outcome).add(id)
                    if (o.pending) pending.add(id)
                    // the next server from the executor, not from inside the
                    // prompt's answer, so that each server leaves the stack
                    later(byKey, ::next)
                }
            } catch (e: Exception) {
                byKey.close()
                throw e
            }
        }
        next()
    }

    // later runs [step] on [run]; where [run] takes nothing more, the
    // activity having gone, nothing more is done, and [byKey] is closed.
    private fun later(byKey: ByKey, step: () -> Unit) {
        try {
            run.execute(step)
        } catch (e: RejectedExecutionException) {
            // the app is closing: what is left stays as it is
            byKey.close()
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
        val leftThere = same && runCatching { left.add(s.id, kept.user, kept.keyId, kept.code) }.getOrDefault(false)
        forgetHere(s.id, s, "")
        return Off(if (leftThere) Outcome.LEFT else Outcome.LOCAL, if (same) kept.keyId else 0, leftThere && RevokeCode.valid(kept.code))
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
            runCatching { left.add(s.id, kept.user, keyId, kept.code) }.getOrDefault(false) -> Outcome.LEFT
            else -> Outcome.LOCAL
        }
        forgetHere(s.id, s, url)
        done(Off(outcome, keyId, outcome == Outcome.LEFT && RevokeCode.valid(kept.code)))
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
         * byKey is the [ByKey] of one [step] of the app key (D63-A): a login
         * under the app key opens with the step's one prompt, a login kept
         * before it with its own, each titled [title] for its server and its
         * login (Forget all's one prompt its own title); a key that does not
         * open is said in [log] and counts as not opened. Its close ends the
         * step.
         */
        fun byKey(step: AppKey.Step, title: (Server, KeptLogin) -> String, log: Logger): ByKey = object : ByKey {
            override fun open(s: Server, kept: KeptLogin, then: (LoginSecret?) -> Unit) =
                step.open(title(s, kept), s.id, kept) { r ->
                    when (r) {
                        is AppKey.Result.Opened -> then(r.secret)
                        is AppKey.Result.Gone -> {
                            log.warn("turn off: the kept key of ${s.id} does not open; forgotten on the phone")
                            then(null)
                        }
                        is AppKey.Result.Failed -> {
                            log.warn("turn off: the kept key of ${s.id} does not open (${r.error.javaClass.simpleName}); forgotten on the phone")
                            then(null)
                        }
                        else -> then(null)
                    }
                }

            override fun close() = step.close()
        }

        /**
         * matches reports whether [kept] is still the login of [s]: its node
         * and its pinned fingerprint as they were when it was kept.
         */
        fun matches(s: Server, kept: KeptLogin): Boolean =
            kept.node == s.node && (Fingerprint.same(kept.fp, s.fp) || (kept.fp.isEmpty() && s.fp.isEmpty()))
    }
}

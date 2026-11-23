// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.io.IOException
import java.net.HttpURLConnection
import java.util.concurrent.Callable
import java.util.concurrent.Executors

/** Logger writes the adapter's warnings to the phone's log; never a key, a token or a session. */
fun interface Logger {
    fun warn(message: String)
}

/**
 * Sessions checks, ends and forgets the sessions the app holds for its
 * servers (docs/app.md, contract version 5), where [keeper] keeps them, and
 * turns a kept login off at its server: the connect page's `session`,
 * `signOut` and `signOutAll`, and the revokes of `forget` and `forgetAll`
 * ([Revoker]). A check that finds the session's user, and its id, also
 * revokes the keys and the sessions [left] there for that user, and gives a
 * login kept before its key's id was the session's key. A session replaced
 * is ended at the server (`setToken`, D68-A), and one forgotten without the
 * server ending it is left in [left] by the id its last check found
 * (D70-A). Each request goes to an address of the server that answered
 * with its node, over the probe's trust ([ServerApi]); [probe] asks a
 * server's addresses for that where the page names none. A failure at a
 * server is written to [log] without any secret.
 */
class Sessions(
    private val keeper: SessionKeeper,
    private val logins: LoginStore,
    private val api: ServerApi,
    private val probe: (List<String>, String?) -> List<ProbeResult>,
    private val left: KeysLeft? = null,
    private val log: Logger,
) {
    // By server id: what the last check found of the session held.
    private val checked = HashMap<String, Checked>()

    // Checked is a session's user, its row at the server, its kind
    // (`session`, or `api` for a key the page holds) and when it was made,
    // as GET /api/v1/me answered for it.
    private data class Checked(val user: String, val id: Long, val kind: String, val createdAt: String)

    /** Ending is the end at the server of a session a [put] replaced, for the caller to run once it has answered. */
    fun interface Ending {
        fun end()
    }

    /** Look is what a check found: the session's user, null for none, and the ids of the keys and sessions left that it revoked. */
    data class Look(val user: String?, val revoked: List<Long> = emptyList())

    /** Revoke is how a kept login's key went at its server with the session the app holds. */
    enum class Revoke {
        /** The server revoked it, or had already. */
        REVOKED,

        /** The session cannot: none is held, the server refused it, or it is another user's; the key itself has to. */
        BY_KEY,

        /** The server did not answer as asked; logged. */
        FAILED,
    }

    /** inMemory reports whether the session of [s] is kept in memory alone (SessionKeeper.memoryFor). */
    fun inMemory(s: Server): Boolean = SessionKeeper.memoryFor(s.ask, logins.get(s.id) != null)

    /** held is the session the app holds for [s], or null. */
    fun held(s: Server): String? = keeper.get(s.id, inMemory(s))

    /**
     * put keeps [token] for [s] (`setToken`, `login.fingerprint`), null
     * forgetting it; what was checked of the one before holds no more. The
     * token goes first, so that a check of the one before that answers
     * meanwhile records nothing (check records only for the token still
     * held), and the one before is read and replaced in one step, so that
     * two puts at once each replace their own. A session the new token
     * replaces is to end at [url], an address of the server's, once
     * (D68-A): put returns that [Ending], for the caller to run after its
     * answer, so that a server slow to answer holds nobody up. Till then
     * the session is noted in [left] by its id, user and time as its last
     * check found them (D70-A), so that an app ended meanwhile leaves it
     * recorded, and the Ending that ends it takes the note away; one the
     * server does not end, not reached, stays noted, and one never checked
     * is ended alone. A key the page held is not ended, its 400 there no
     * failure and in no log. Without [url] what was held is replaced alone,
     * and null is returned.
     */
    fun put(s: Server, token: String?, url: String? = null): Ending? {
        val (old, was) = synchronized(checked) { keeper.swap(s.id, token, inMemory(s)) to checked.remove(s.id) }
        if (url == null || token == null || old == null || old == token || was?.kind == KIND_KEY) return null
        leave(s, was)
        return Ending { if (s.has(url) && end(s, url, old, quietKey = true)) unleave(s, was) }
    }

    /** user is the user the last check found for the session held for [s], null where none is held or none was checked. */
    fun user(s: Server): String? {
        val u = synchronized(checked) { checked[s.id] } ?: return null
        return if (held(s) != null) u.user else null
    }

    /** forget forgets the session held for the server [id], and what was checked of it. */
    fun forget(id: String) {
        synchronized(checked) { checked.remove(id) }
        keeper.forget(id)
    }

    /**
     * check asks the server at [url], an address of [s]'s, whose the
     * session held for it is (`GET /api/v1/me`): the user's name; null where
     * none is held, or the server refused it, which forgets it. A server that
     * cannot be asked leaves what the last check found. With the user known,
     * the keys left there for that user are revoked with the session
     * ([KeysLeft]), and a login kept for that user without its key's id
     * (kept before contract version 3) takes the id of the key the session
     * was made from, so that Turn off revokes it with a session.
     */
    fun check(s: Server, url: String): String? = look(s, url).user

    /** look is [check], with the ids of the keys and sessions left that the check revoked (`setToken`'s answer). */
    fun look(s: Server, url: String): Look {
        val token = held(s) ?: return Look(null)
        if (!s.has(url)) return Look(user(s))
        return try {
            val me = api.whoami(url, s.fp, token)
            // a session handed in meanwhile is not the one asked about
            synchronized(checked) { if (held(s) == token) checked[s.id] = Checked(me.user, me.tokenId, me.kind, me.createdAt) }
            learnKeyId(s, me)
            Look(me.user, left?.revoke(s, url, token, me) ?: emptyList())
        } catch (e: ServerApi.Refused) {
            // a dead session counts as nothing at the server once forgotten
            if (held(s) == token) forget(s.id)
            Look(null)
        } catch (e: ServerApi.Failed) {
            synchronized(checked) { checked.remove(s.id) }
            Look(null)
        } catch (e: IOException) {
            Look(user(s))
        }
    }

    /**
     * keyOf is the id of the key of [kept], the login kept for [s]: the one
     * kept beside it, else, for a login kept before contract version 3, the
     * key the session held for [s] was made from, asked at [url]
     * (`GET /api/v1/me`) and kept with the login; 0 where neither says,
     * the session forgotten where the server refused it (F28).
     */
    fun keyOf(s: Server, url: String, kept: KeptLogin): Long {
        if (kept.keyId > 0) return kept.keyId
        val token = held(s) ?: return 0
        if (!s.has(url)) return 0
        return try {
            learnKeyId(s, api.whoami(url, s.fp, token), kept)
        } catch (e: ServerApi.Refused) {
            if (held(s) == token) forget(s.id)
            0
        } catch (e: IOException) {
            0
        }
    }

    // learnKeyId gives [kept], the login kept for [s] as read, the id of the
    // key the session was made from, where it has none and is the session's
    // user's, in one step of the store's that leaves a login kept or
    // forgotten meanwhile as it is; it returns the login's key id, 0 for none.
    private fun learnKeyId(s: Server, me: ServerApi.Me, kept: KeptLogin? = logins.get(s.id)): Long {
        if (kept == null) return 0
        if (kept.keyId > 0) return kept.keyId
        if (me.keyId <= 0 || kept.user != me.user) return 0
        return if (logins.setKeyId(s.id, kept, me.keyId)) me.keyId else 0
    }

    /**
     * signOut ends the session held for [s] at [url], an address of the
     * server's that answered (`POST /api/v1/logout`), or "" for none, and
     * forgets it in every case, a session the server did not end left by
     * the id its last check found (D70-A); it reports whether the server
     * ended it, or had already.
     */
    fun signOut(s: Server, url: String): Boolean {
        val token = held(s)
        val was = synchronized(checked) { checked[s.id] }
        forget(s.id)
        if (token == null) return false
        val ended = s.has(url) && end(s, url, token)
        if (!ended) leave(s, was)
        return ended
    }

    /**
     * signOutAll signs the app out of every server in [servers] it holds a
     * session for: each forgotten at once, then ended at the address of the
     * server's that answers with its node, the LAN one first, all servers at
     * once. It returns how many the servers ended.
     */
    fun signOutAll(servers: List<Server>): Int {
        val held = servers.mapNotNull { s -> held(s)?.let { Triple(s, it, synchronized(checked) { checked[s.id] }) } }
        held.forEach { forget(it.first.id) }
        if (held.isEmpty()) return 0
        val pool = Executors.newFixedThreadPool(held.size.coerceAtMost(MAX_PARALLEL))
        try {
            val ends = held.map { (s, token, was) ->
                pool.submit(
                    Callable {
                        val url = Addresses.answering(s, probe(listOf(s.lan, s.remote).filter { it.isNotEmpty() }, s.fp.ifEmpty { null }))
                        val ended = if (url == null) {
                            log.warn("sign out: ${s.id} did not answer; its session is not ended there")
                            false
                        } else {
                            end(s, url, token)
                        }
                        if (!ended) leave(s, was)
                        ended
                    },
                )
            }
            return ends.count { runCatching { it.get() }.getOrDefault(false) }
        } finally {
            pool.shutdown()
        }
    }

    /**
     * revokeWithSession revokes the key of [kept], the login kept for [s],
     * at [url] with the session the app holds for [s]
     * (`DELETE /api/v1/users/{name}/tokens/{id}`), then signs that session
     * out: Turn off without the prompt (AP MobileApp, F6). [Revoke.BY_KEY]
     * where it cannot, the session forgotten where the server refused it;
     * [keyOf] gives a login kept without its key's id the session's key
     * first.
     */
    fun revokeWithSession(s: Server, url: String, kept: KeptLogin): Revoke {
        val token = held(s) ?: return Revoke.BY_KEY
        if (kept.keyId <= 0 || !s.has(url)) return Revoke.BY_KEY
        try {
            api.revoke(url, s.fp, token, kept.user, kept.keyId)
        } catch (e: ServerApi.Refused) {
            forget(s.id)
            return Revoke.BY_KEY
        } catch (e: ServerApi.Failed) {
            when (e.status) {
                HttpURLConnection.HTTP_NOT_FOUND -> Unit // revoked already
                HttpURLConnection.HTTP_FORBIDDEN -> return Revoke.BY_KEY // the session is another user's
                else -> {
                    log.warn("turn off: the key of ${s.id} was not revoked at ${where(url)}: ${what(e)}")
                    return Revoke.FAILED
                }
            }
        } catch (e: IOException) {
            log.warn("turn off: the key of ${s.id} was not revoked at ${where(url)}: ${what(e)}")
            return Revoke.FAILED
        }
        signOut(s, url)
        return Revoke.REVOKED
    }

    /**
     * revokeWithKey revokes the kept key with the key itself, [secret], opened
     * after the prompt, at [url]: Turn off where the app holds no session
     * (`DELETE /api/v1/me/token`, at an older server by its user and id,
     * [ServerApi.revokeSelf]). It reports whether the server revoked it, or
     * takes the key no more; a failure is logged.
     */
    fun revokeWithKey(s: Server, url: String, user: String, secret: LoginSecret): Boolean {
        return try {
            api.revokeSelf(url, s.fp, secret.key, user, secret.id)
            true
        } catch (e: ServerApi.Refused) {
            true // the server takes the key no more
        } catch (e: IOException) {
            log.warn("turn off: the key of ${s.id} was not revoked at ${where(url)}: ${what(e)}")
            false
        }
    }

    // end ends [token] at [url]; a session the server had ended already
    // counts as ended, an API key (which is no session) and a failure as not.
    // With quietKey a key's 400 is no failure: the page held a key, which
    // a logout does not end, and the log leaves it out.
    private fun end(s: Server, url: String, token: String, quietKey: Boolean = false): Boolean {
        return try {
            api.logout(url, s.fp, token)
            true
        } catch (e: ServerApi.Refused) {
            true
        } catch (e: ServerApi.Failed) {
            if (quietKey && e.status == HttpURLConnection.HTTP_BAD_REQUEST) return true
            log.warn("sign out: the session of ${s.id} was not ended at ${where(url)}: ${what(e)}")
            false
        } catch (e: IOException) {
            log.warn("sign out: the session of ${s.id} was not ended at ${where(url)}: ${what(e)}")
            false
        }
    }

    // leave notes a session forgotten without the server ending it in
    // [left], by the id, the user and the time it was made that its last
    // check found, [was]: a session's alone, a key the page held being
    // none (D70-A); one never checked is forgotten on the phone alone.
    private fun leave(s: Server, was: Checked?) {
        if (was == null || was.kind != KIND_SESSION || was.id <= 0) return
        runCatching { left?.addSession(s.id, was.user, was.id, was.createdAt) }
    }

    // unleave takes away the note [leave] made of [was], the session ended.
    private fun unleave(s: Server, was: Checked?) {
        if (was == null || was.kind != KIND_SESSION || was.id <= 0) return
        runCatching { left?.endedSession(s.id, was.user, was.id, was.createdAt) }
    }

    private companion object {
        const val MAX_PARALLEL = 8

        // The kinds of a token as GET /api/v1/me names them (docs/api-v1.md).
        const val KIND_SESSION = "session"
        const val KIND_KEY = "api"

        // where is an address as a log names it: its origin, no path.
        fun where(url: String): String = Origins.of(url) ?: "?"

        // what is a failure as a log names it: the server's status and
        // words, or the kind of the failure; never what was sent.
        fun what(e: IOException): String = when (e) {
            is ServerApi.Failed -> "status ${e.status}, ${e.message}"
            else -> e.javaClass.simpleName
        }
    }
}

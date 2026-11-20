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
 * servers (docs/app.md, contract version 3), where [keeper] keeps them, and
 * turns a kept login off at its server: the connect page's `session`,
 * `signOut` and `signOutAll`, and `forget`'s revoke. Each request goes to an
 * address of the server that answered with its node, over the probe's trust
 * ([ServerApi]); [probe] asks a server's addresses for that where the page
 * names none. A failure at a server is written to [log] without any secret.
 */
class Sessions(
    private val keeper: SessionKeeper,
    private val logins: LoginStore,
    private val api: ServerApi,
    private val probe: (List<String>, String?) -> List<ProbeResult>,
    private val log: Logger,
) {
    // By server id: the user the last check found for the session held.
    private val checked = HashMap<String, String>()

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

    /** put keeps [token] for [s] (`setToken`), null forgetting it; what was checked of the one before holds no more. */
    fun put(s: Server, token: String?) {
        synchronized(checked) { checked.remove(s.id) }
        keeper.put(s.id, token, inMemory(s))
    }

    /** user is the user the last check found for the session held for [s], null where none is held or none was checked. */
    fun user(s: Server): String? {
        val u = synchronized(checked) { checked[s.id] } ?: return null
        return if (held(s) != null) u else null
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
     * cannot be asked leaves what the last check found.
     */
    fun check(s: Server, url: String): String? {
        val token = held(s) ?: return null
        if (!s.has(url)) return user(s)
        return try {
            val u = api.me(url, s.fp, token)
            synchronized(checked) { checked[s.id] = u }
            u
        } catch (e: ServerApi.Refused) {
            // a dead session counts as nothing at the server once forgotten
            if (held(s) == token) forget(s.id)
            null
        } catch (e: ServerApi.Failed) {
            synchronized(checked) { checked.remove(s.id) }
            null
        } catch (e: IOException) {
            user(s)
        }
    }

    /**
     * signOut ends the session held for [s] at [url], an address of the
     * server's that answered (`POST /api/v1/logout`), or "" for none, and
     * forgets it in every case; it reports whether the server ended it, or
     * had already.
     */
    fun signOut(s: Server, url: String): Boolean {
        val token = held(s)
        forget(s.id)
        if (token == null || !s.has(url)) return false
        return end(s, url, token)
    }

    /**
     * signOutAll signs the app out of every server in [servers] it holds a
     * session for: each forgotten at once, then ended at the address of the
     * server's that answers with its node, the LAN one first, all servers at
     * once. It returns how many the servers ended.
     */
    fun signOutAll(servers: List<Server>): Int {
        val held = servers.mapNotNull { s -> held(s)?.let { s to it } }
        held.forEach { forget(it.first.id) }
        if (held.isEmpty()) return 0
        val pool = Executors.newFixedThreadPool(held.size.coerceAtMost(MAX_PARALLEL))
        try {
            val ends = held.map { (s, token) ->
                pool.submit(
                    Callable {
                        val url = Addresses.answering(s, probe(listOf(s.lan, s.remote).filter { it.isNotEmpty() }, s.fp.ifEmpty { null }))
                        if (url == null) {
                            log.warn("sign out: ${s.id} did not answer; its session ends at the server by itself")
                            false
                        } else {
                            end(s, url, token)
                        }
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
     * where it cannot, the session forgotten where the server refused it.
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
     * after the prompt, at [url]: Turn off where the app holds no session.
     * It reports whether the server revoked it, or takes the key no more; a
     * failure is logged.
     */
    fun revokeWithKey(s: Server, url: String, user: String, secret: LoginSecret): Boolean {
        return try {
            api.revoke(url, s.fp, secret.key, user, secret.id)
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
    private fun end(s: Server, url: String, token: String): Boolean {
        return try {
            api.logout(url, s.fp, token)
            true
        } catch (e: ServerApi.Refused) {
            true
        } catch (e: IOException) {
            log.warn("sign out: the session of ${s.id} was not ended at ${where(url)}: ${what(e)}")
            false
        }
    }

    private companion object {
        const val MAX_PARALLEL = 8

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

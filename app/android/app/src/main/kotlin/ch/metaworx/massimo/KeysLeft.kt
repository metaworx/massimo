// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.io.IOException
import java.net.HttpURLConnection

/**
 * KeysLeft keeps the keys of the phone's logins that Turn off, Forget all
 * or a login forgotten could not revoke at their servers (AP MobileApp,
 * D57-A), a server's `unrevoked`, and the sessions the app forgot without
 * the server ending them (D70-A), a server's `unended`: each one's user and
 * id, no secret, in [servers] (docs/app.md, contract version 5). The next
 * session the app holds at a server for one of those users revokes them
 * there (Sessions.check), since any session of a user may revoke the
 * user's tokens by their ids (`DELETE /api/v1/users/{name}/tokens/{id}`).
 * A failure is written to [log] with the ids, the origin and the status
 * alone.
 */
class KeysLeft(private val servers: ServerStore, private val logins: LoginStore, private val api: ServerApi, private val log: Logger) {
    /**
     * add notes the key [keyId] of [user]'s as left valid on the server [id],
     * with its revoke [code] where the app has one (D112-A), and reports
     * whether it is noted: one without an id is not, and is forgotten on the
     * phone alone.
     */
    fun add(id: String, user: String, keyId: Long, code: String = ""): Boolean = servers.leave(id, user, keyId, code)

    /**
     * addSession notes the session [sessionId] of [user]'s, made at
     * [createdAt], as left valid on the server [id], with its revoke [code]
     * where the app has one, and reports whether it is noted: one whose id
     * was never learnt is not, and is forgotten on the phone alone.
     */
    fun addSession(id: String, user: String, sessionId: Long, createdAt: String = "", code: String = ""): Boolean =
        servers.leaveSession(id, user, sessionId, createdAt, code)

    /** endedSession drops the session [sessionId] of [user]'s, made at [createdAt], from those left on the server [id]: it was ended after all, its code with it. */
    fun endedSession(id: String, user: String, sessionId: Long, createdAt: String = "") = servers.ended(id, listOf(SessionLeft(user, sessionId, createdAt)))

    /** hold records [held] as the session the app holds at the server [id] (D102-A). */
    fun hold(id: String, held: HeldSession) = servers.hold(id, held)

    /** unhold clears the server's record of the session held: it ended, was left or replaced. */
    fun unhold(id: String) = servers.unhold(id)

    /** held is the server's record of the session held, null for none. */
    fun held(id: String): HeldSession? = servers.get(id)?.held

    /**
     * revoke revokes the keys and the sessions left on [s] whose user is
     * [me]'s, the user of [session], at [url], an address of the server's
     * that answered, the keys first. An id is revoked only where the user's
     * list of sessions and keys (`GET /api/v1/users/{name}/tokens`) has it
     * as of its kind, a key as an API key and a session as a session made
     * when the one left was, so that an id a store rolled back gave another
     * live session is not signed out (a session left without its time
     * never matches, and is dropped unasked), and never the token in use,
     * the key it was made from or the key of the login kept there now, ids a store rolled back may have given again:
     * those stay noted. One the list does not have so is gone, or no more
     * ours, and is dropped unasked. One the server revoked, or has no more
     * (404), is dropped; one it refuses (403) stays, and so does one it
     * could not be asked about, logged. A session the server refuses (401)
     * or no answer stops it: the rest wait for the next. An entry whose code
     * the server took (D112-A) is checked so too: dropped where the list has
     * it not, the code having ended it, and revoked where it has it, a token
     * the server had not seen since its update. It returns the ids
     * the server revoked now.
     */
    fun revoke(s: Server, url: String, session: String, me: ServerApi.Me): List<Long> {
        val now = servers.get(s.id) ?: return emptyList()
        val keys = now.unrevoked.filter { it.user == me.user }
        val sessions = now.unended.filter { it.user == me.user }
        if (keys.isEmpty() && sessions.isEmpty() || !s.has(url)) return emptyList()
        val listed = try {
            api.tokens(url, s.fp, session, me.user)
        } catch (e: ServerApi.Refused) {
            return emptyList()
        } catch (e: IOException) {
            log.warn("keys left: the keys of ${s.id} were not listed at ${where(url)}: ${what(e)}")
            return emptyList()
        }
        val apiKeys = listed.filter { it.kind == KIND_KEY }.map { it.id }.toSet()
        val made = listed.filter { it.kind == KIND_SESSION }.associate { it.id to it.createdAt }
        val inUse = setOf(me.tokenId, me.keyId, logins.get(s.id)?.keyId ?: 0).filter { it > 0 }
        val revoked = mutableListOf<Long>()
        val keysGone = mutableListOf<KeyLeft>()
        val sessionsGone = mutableListOf<SessionLeft>()

        // one revokes the id of a token of the kind named, a key or a
        // session, where the list has it as of that kind, and reports
        // whether the rest may go on; gone notes it dropped
        fun one(named: String, id: Long, there: Boolean, gone: () -> Unit): Boolean {
            if (id in inUse) return true // the token in use, its key, the login's key
            if (!there) {
                gone()
                return true
            }
            try {
                api.revoke(url, s.fp, session, me.user, id)
                gone()
                revoked.add(id)
            } catch (e: ServerApi.Refused) {
                return false
            } catch (e: ServerApi.Failed) {
                when (e.status) {
                    HttpURLConnection.HTTP_NOT_FOUND -> gone() // revoked already
                    HttpURLConnection.HTTP_FORBIDDEN -> Unit // not this session's to revoke
                    else -> log.warn("${named}s left: the $named $id of ${s.id} was not revoked at ${where(url)}: ${what(e)}")
                }
            } catch (e: IOException) {
                log.warn("${named}s left: the $named $id of ${s.id} was not revoked at ${where(url)}: ${what(e)}")
                return false
            }
            return true
        }
        run {
            for (k in keys) if (!one("key", k.id, k.id in apiKeys) { keysGone.add(k) }) return@run
            for (x in sessions) if (!one("session", x.id, x.createdAt.isNotEmpty() && made[x.id] == x.createdAt) { sessionsGone.add(x) }) return@run
        }
        servers.revoked(s.id, keysGone)
        servers.ended(s.id, sessionsGone)
        return revoked
    }

    private companion object {
        // The kinds of a user's tokens (docs/api-v1.md): an API key, a session.
        const val KIND_KEY = "api"
        const val KIND_SESSION = "session"

        fun where(url: String): String = Origins.of(url) ?: "?"

        // what is a failure as a log names it: the status, or the kind of the failure.
        fun what(e: IOException): String = if (e is ServerApi.Failed) "status ${e.status}" else e.javaClass.simpleName
    }
}

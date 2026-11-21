// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.io.IOException
import java.net.HttpURLConnection

/**
 * KeysLeft keeps the keys of the phone's logins that Turn off, Forget all
 * or a login forgotten could not revoke at their servers (AP MobileApp,
 * D57-A): a server's `unrevoked`, each key's user and id, no secret, in
 * [servers] (docs/app.md, contract version 4). The next session the app
 * holds at a server for one of those users revokes them there
 * (Sessions.check), since any session of a user may revoke the user's key by
 * its id (`DELETE /api/v1/users/{name}/tokens/{id}`). A failure is written to
 * [log] with the ids, the origin and the status alone.
 */
class KeysLeft(private val servers: ServerStore, private val logins: LoginStore, private val api: ServerApi, private val log: Logger) {
    /**
     * add notes the key [keyId] of [user]'s as left valid on the server [id],
     * and reports whether it is noted: one without an id is not, and is
     * forgotten on the phone alone.
     */
    fun add(id: String, user: String, keyId: Long): Boolean = servers.leave(id, user, keyId)

    /**
     * revoke revokes the keys left on [s] whose user is [me]'s, the user of
     * [session], at [url], an address of the server's that answered. An id
     * is revoked only where the user's list of sessions and keys
     * (`GET /api/v1/users/{name}/tokens`) has it as an API key, and never the
     * token in use, the key it was made from or the key of the login kept
     * there now, ids a store rolled back may have given again: those stay
     * noted. One the list does not have as a key is gone, or no more ours,
     * and is dropped unasked. A key the server revoked, or has no more
     * (404), is dropped;
     * one it refuses (403) stays, and so does one it could not be asked
     * about, logged. A session the server refuses (401) or no answer stops
     * it: the rest wait for the next. It returns how many it dropped.
     */
    fun revoke(s: Server, url: String, session: String, me: ServerApi.Me): Int {
        val mine = (servers.get(s.id) ?: return 0).unrevoked.filter { it.user == me.user }
        if (mine.isEmpty() || !s.has(url)) return 0
        val keys = try {
            api.tokens(url, s.fp, session, me.user).filter { it.kind == KIND_KEY }.map { it.id }.toSet()
        } catch (e: ServerApi.Refused) {
            return 0
        } catch (e: IOException) {
            log.warn("keys left: the keys of ${s.id} were not listed at ${where(url)}: ${what(e)}")
            return 0
        }
        val inUse = setOf(me.tokenId, me.keyId, logins.get(s.id)?.keyId ?: 0).filter { it > 0 }
        val gone = mutableListOf<KeyLeft>()
        for (k in mine) {
            if (k.id in inUse) continue // the token in use, its key, the login's key
            if (k.id !in keys) {
                gone.add(k)
                continue
            }
            try {
                api.revoke(url, s.fp, session, me.user, k.id)
                gone.add(k)
            } catch (e: ServerApi.Refused) {
                break
            } catch (e: ServerApi.Failed) {
                when (e.status) {
                    HttpURLConnection.HTTP_NOT_FOUND -> gone.add(k) // revoked already
                    HttpURLConnection.HTTP_FORBIDDEN -> Unit // not this session's to revoke
                    else -> log.warn("keys left: the key ${k.id} of ${s.id} was not revoked at ${where(url)}: ${what(e)}")
                }
            } catch (e: IOException) {
                log.warn("keys left: the key ${k.id} of ${s.id} was not revoked at ${where(url)}: ${what(e)}")
                break
            }
        }
        servers.revoked(s.id, gone)
        return gone.size
    }

    private companion object {
        // The kind of a user's token that is an API key (docs/api-v1.md).
        const val KIND_KEY = "api"

        fun where(url: String): String = Origins.of(url) ?: "?"

        // what is a failure as a log names it: the status, or the kind of the failure.
        fun what(e: IOException): String = if (e is ServerApi.Failed) "status ${e.status}" else e.javaClass.simpleName
    }
}

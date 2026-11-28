// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

/**
 * SessionKeeper keeps each server's session where its login asks for it
 * (docs/app.md, "The login by fingerprint"): in the app's memory alone for a
 * server with a kept login asked at each return or at each start, so that the
 * next start asks for the fingerprint; sealed in [tokens] for one asked after
 * a logout and for one without a kept login, as before the login by
 * fingerprint.
 */
class SessionKeeper(private val tokens: TokenStore) {
    private val memory = HashMap<String, String>()

    /** get is the session kept for the server [id], where [inMemory] says it is. */
    @Synchronized
    fun get(id: String, inMemory: Boolean): String? = if (inMemory) memory[id] else tokens.get(id)

    /** put keeps [token] for the server [id], or forgets it for null, in memory or sealed, and never in both. */
    @Synchronized
    fun put(id: String, token: String?, inMemory: Boolean) {
        if (inMemory) {
            if (token == null) memory.remove(id) else memory[id] = token
            tokens.remove(id)
        } else {
            memory.remove(id)
            tokens.put(id, token)
        }
    }

    /** swap keeps [token] for the server [id] as [put] does and returns the session it held before, in one step, so that two at once each replace their own. */
    @Synchronized
    fun swap(id: String, token: String?, inMemory: Boolean): String? {
        val old = get(id, inMemory)
        put(id, token, inMemory)
        return old
    }

    /** move takes the server's session to where [inMemory] says, for a login kept or forgotten, or another choice. */
    @Synchronized
    fun move(id: String, inMemory: Boolean) {
        val t = memory[id] ?: tokens.get(id)
        put(id, t, inMemory)
    }

    /** inMemory reports whether the server [id] has a session in memory, as a server asked at each return covers. */
    @Synchronized
    fun inMemory(id: String): Boolean = memory.containsKey(id)

    /** holds reports whether a session is kept for the server [id] where [inMemory] says, without opening a sealed one. */
    @Synchronized
    fun holds(id: String, inMemory: Boolean): Boolean = if (inMemory) memory.containsKey(id) else tokens.has(id)

    /** dropMemory forgets every session kept in memory, as the process's end does: Exit (D91-A). */
    @Synchronized
    fun dropMemory() = memory.clear()

    /**
     * leaveForgotten moves into `unended` each server's session recorded as
     * held ([Server.held]) that the keeper no longer holds where its login
     * says it is kept (D102-A): at a new process's start, nothing in memory,
     * and at Exit once [dropMemory] has dropped them; a sealed session,
     * still held, stays recorded. [logins] says where each is kept.
     */
    fun leaveForgotten(servers: ServerStore, logins: LoginStore) =
        servers.leaveHeld { s -> holds(s.id, memoryFor(s.ask, logins.get(s.id) != null)) }

    /** forget forgets the server's session, wherever it is. */
    @Synchronized
    fun forget(id: String) {
        memory.remove(id)
        tokens.remove(id)
    }

    companion object {
        /** memoryFor reports whether a server asked at [ask] keeps its session in memory: with a [kept] login, at each return or start. */
        fun memoryFor(ask: String, kept: Boolean): Boolean = kept && ask != Server.ASK_LOGOUT
    }
}

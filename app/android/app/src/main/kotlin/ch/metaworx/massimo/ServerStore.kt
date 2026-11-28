// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.io.IOException
import java.net.URI
import java.security.SecureRandom
import java.time.Instant
import java.time.temporal.ChronoUnit

/**
 * A key of the phone's that Turn off or Forget all could not revoke at its
 * server (AP MobileApp, D57-A): its user and its row at the server, no
 * secret; the next session the app holds there for that user revokes it.
 * [code] is its revoke code (D112-A), "" for none, which the app posts at
 * the next contact; [taken] says the server took it (`204`), the entry
 * kept, hidden from the page, until the next session there for that user
 * checks it.
 */
data class KeyLeft(val user: String, val id: Long, val code: String = "", val taken: Boolean = false) {
    /** toJson is the key as the connect page sees it, `{user, id}`, with `pending: true` while its code waits; never the code. */
    fun toJson(): JSONObject = JSONObject().put("user", user).put("id", id).apply { if (code.isNotEmpty()) put("pending", true) }

    /** fileJson is the key as the store keeps it, with its code and whether it was taken. */
    fun fileJson(): JSONObject = JSONObject().put("user", user).put("id", id).apply {
        if (code.isNotEmpty()) put("code", code)
        if (taken) put("taken", true)
    }

    /** same reports whether [o] is this key, whatever its code says. */
    fun same(o: KeyLeft): Boolean = user == o.user && id == o.id

    // An entry is its token, whatever its code says: equal where same.
    override fun equals(other: Any?): Boolean = other is KeyLeft && same(other)

    override fun hashCode(): Int = 31 * user.hashCode() + id.hashCode()

    override fun toString(): String = "KeyLeft(user=$user, id=$id, pending=${code.isNotEmpty()}, taken=$taken)"
}

/**
 * A session of the phone's that the app forgot without the server ending
 * it (AP MobileApp, D70-A): Turn off, Forget all, Sign out, Sign out of
 * all or a session replaced, the server not answering; its user, its row
 * at the server and when it was made ([createdAt], as the server writes
 * it), as the session's last check found them, no secret; the next
 * session the app holds there for that user revokes it, where the row of
 * that id was made then: an id a store rolled back gave again is
 * another's. [code] and [taken] are as [KeyLeft]'s; [leftAt] is when it
 * was left (RFC 3339), for the 30 days a removed server's session code is
 * kept (D116-A).
 */
data class SessionLeft(val user: String, val id: Long, val createdAt: String = "", val code: String = "", val taken: Boolean = false, val leftAt: String = "") {
    /** toJson is the session as the connect page sees it, `{user, id, created_at}`, with `pending: true` while its code waits; never the code. */
    fun toJson(): JSONObject = JSONObject().put("user", user).put("id", id).put("created_at", createdAt).apply { if (code.isNotEmpty()) put("pending", true) }

    /** fileJson is the session as the store keeps it, with its code, whether it was taken, and when it was left. */
    fun fileJson(): JSONObject = JSONObject().put("user", user).put("id", id).put("created_at", createdAt).apply {
        if (code.isNotEmpty()) put("code", code)
        if (taken) put("taken", true)
        if (leftAt.isNotEmpty()) put("left_at", leftAt)
    }

    /** same reports whether [o] is this session, whatever its code says. */
    fun same(o: SessionLeft): Boolean = user == o.user && id == o.id && createdAt == o.createdAt

    // An entry is its token, whatever its code says: equal where same.
    override fun equals(other: Any?): Boolean = other is SessionLeft && same(other)

    override fun hashCode(): Int = listOf(user, id, createdAt).hashCode()

    override fun toString(): String = "SessionLeft(user=$user, id=$id, createdAt=$createdAt, pending=${code.isNotEmpty()}, taken=$taken)"
}

/**
 * The session the app holds at a server, as a check found it (AP
 * MobileApp, D102-A): its user, its row at the server and when it was made,
 * no secret. It is kept in the app's file alone, never given to the page,
 * so that a session the process forgets with its memory, a swipe from the
 * recent apps or Android ending a hidden app, goes into [Server.unended]
 * at the next process's start (SessionKeeper.leaveForgotten), with its
 * revoke [code] (D112-A), so that the next contact ends it. [keyId] is the
 * key it was made from, 0 for none, so that after a restart a session the
 * server ended with its key, or with the key's next session, is not logged
 * out (D100-A).
 */
data class HeldSession(val user: String, val id: Long, val createdAt: String = "", val code: String = "", val keyId: Long = 0) {
    /** fileJson is the session as the store keeps it, `{user, id, created_at, code, key_id}`. */
    fun fileJson(): JSONObject = JSONObject().put("user", user).put("id", id).put("created_at", createdAt).apply {
        if (code.isNotEmpty()) put("code", code)
        if (keyId > 0) put("key_id", keyId)
    }

    /** left is the session as one left valid at the server at [at]. */
    fun left(at: String = ""): SessionLeft = SessionLeft(user, id, createdAt, code, leftAt = at)

    // The session is its token, whose code follows from it.
    override fun equals(other: Any?): Boolean = other is HeldSession && user == other.user && id == other.id && createdAt == other.createdAt

    override fun hashCode(): Int = listOf(user, id, createdAt).hashCode()

    override fun toString(): String = "HeldSession(user=$user, id=$id, createdAt=$createdAt, keyId=$keyId)"
}

/**
 * A server the app keeps (docs/app.md, "A server"); [ask] is when the
 * fingerprint is asked once a login is kept, one of [ASKS]; [start] marks
 * the one server the app opens at its start; [trustedAt] is when its
 * pinned fingerprint was trusted (RFC 3339), "" for none; [unrevoked] are
 * the keys of the phone's left valid there (contract version 4), [unended]
 * its sessions left valid there (version 5); [held] the session the app
 * holds there, once a check learnt its id, kept in the file alone.
 */
data class Server(
    val id: String,
    val name: String,
    val node: String,
    val lan: String,
    val remote: String,
    val fp: String,
    val ask: String,
    val push: Boolean,
    val start: Boolean = false,
    val trustedAt: String = "",
    val unrevoked: List<KeyLeft> = emptyList(),
    val unended: List<SessionLeft> = emptyList(),
    val held: HeldSession? = null,
) {
    /**
     * toJson is the server as the connect page sees it, with the user whose
     * login is kept, [login], or "", the user of the session the app holds
     * and checked, [session], or null (contract version 3), the keys left
     * valid there (version 4), and the sessions, and the id of the kept
     * login's key, [loginKey], 0 for none or not known (version 5); no entry
     * the server took, no code and no `held`, an entry whose code waits
     * `pending: true` (version 6).
     */
    fun toJson(login: String = "", session: String? = null, loginKey: Long = 0): JSONObject = JSONObject()
        .put("id", id)
        .put("name", name)
        .put("node", node)
        .put("lan", lan)
        .put("remote", remote)
        .put("fp", fp)
        .put("login", login)
        .put("login_key", if (login.isEmpty()) 0 else loginKey.coerceAtLeast(0))
        .put("ask", ask)
        .put("push", push)
        .put("start", start)
        .put("session", session ?: JSONObject.NULL)
        .put("trusted_at", trustedAt)
        .put("unrevoked", JSONArray().apply { unrevoked.filter { !it.taken }.forEach { put(it.toJson()) } })
        .put("unended", JSONArray().apply { unended.filter { !it.taken }.forEach { put(it.toJson()) } })

    /**
     * fileJson is the server as the store keeps it: what the page sees, but
     * the login, its key and the session, which are kept elsewhere, and with
     * the session held, which the page never sees.
     */
    fun fileJson(): JSONObject = toJson().apply {
        remove("login")
        remove("login_key")
        remove("session")
        put("unrevoked", JSONArray().apply { unrevoked.forEach { put(it.fileJson()) } })
        put("unended", JSONArray().apply { unended.forEach { put(it.fileJson()) } })
        held?.let { put("held", it.fileJson()) }
    }

    /** codes are the revoke codes that wait to be posted at the server: its keys' and sessions' left, not taken (D112-A). */
    fun codes(): List<String> = (unrevoked.filter { !it.taken }.map { it.code } + unended.filter { !it.taken }.map { it.code }).filter(RevokeCode::valid).distinct()

    /** has reports whether [url] is one of the server's two addresses. */
    fun has(url: String): Boolean = url.isNotEmpty() && (url == lan || url == remote)

    /**
     * opening is the server's address that [url] is at, where `open` may
     * load [url]: one of its two addresses, alone or with My profile's
     * fragment that marks a key, `#profile?key=<id>` (docs/ui.md, "My
     * profile"); null for any other URL.
     */
    fun opening(url: String): String? {
        val at = url.indexOf('#')
        val base = if (at < 0) url else url.substring(0, at)
        if (!has(base)) return null
        return if (at < 0 || PROFILE_KEY.matches(url.substring(at + 1))) base else null
    }

    companion object {
        /** The choices of when the fingerprint is asked: at each return, at each start, after a logout. */
        const val ASK_RETURN = "return"
        const val ASK_START = "start"
        const val ASK_LOGOUT = "logout"
        val ASKS = setOf(ASK_RETURN, ASK_START, ASK_LOGOUT)

        // My profile's fragment that marks a key (ui/src/profile/index.tsx).
        private val PROFILE_KEY = Regex("""profile\?key=[1-9][0-9]{0,14}""")
    }
}

/**
 * ServerStore keeps the servers, in their order, as a JSON array in one
 * file of the app's own; every change is written at once, through a file
 * beside it renamed over it. One server at most opens at the app's start
 * (D43-A), the first one added until the user chooses; [now] is the time a
 * fingerprint is trusted at.
 */
class ServerStore(private val file: File, private val now: () -> String = { nowUtc() }) {
    // The servers that had version 1's switch "Ask for my fingerprint or
    // PIN" on, found by load: their session is forgotten once (takeLegacy).
    private val legacy = mutableSetOf<String>()
    private val servers: MutableList<Server> = load()

    /** list is the servers, in their order. */
    @Synchronized
    fun list(): List<Server> = servers.toList()

    /** get is the server kept under [id], or null. */
    @Synchronized
    fun get(id: String): Server? = servers.firstOrNull { it.id == id }

    /** atStart is the server the app opens at its start, or null for the list. */
    @Synchronized
    fun atStart(): Server? = servers.firstOrNull { it.start }

    /**
     * save adds [input] where it has no `id`, giving it one, or replaces the
     * server with its `id`, in place. `fp` and `trusted_at` are kept as
     * [trust] set them, `unrevoked` as [leave] and [revoked] set it,
     * `unended` as [leaveSession] and [ended], and `held` as [hold] set it,
     * all dropped where the node changed: those keys and sessions are
     * another server's; a removed server's of the node it has now are
     * taken back by [Codes.adopt] (AP MobileApp, D117-A).
     * `start` true turns it off at every other server; the first server
     * added opens at start whatever it says.
     */
    @Synchronized
    fun save(input: JSONObject): Server {
        val id = str(input, "id")
        val old = if (id.isEmpty()) null else get(id) ?: throw BridgeException(R.string.err_no_server)
        val lan = address(str(input, "lan"))
        val remote = address(str(input, "remote"))
        if (lan.isEmpty() && remote.isEmpty()) throw BridgeException(R.string.err_no_address)
        var name = str(input, "name").trim().take(MAX_TEXT)
        if (name.isEmpty()) name = URI(lan.ifEmpty { remote }).host
        val node = str(input, "node").trim().take(MAX_TEXT)
        val s = Server(
            id = old?.id ?: newId(),
            name = name,
            node = node,
            lan = lan,
            remote = remote,
            fp = old?.fp ?: "",
            ask = str(input, "ask").takeIf { it in Server.ASKS } ?: old?.ask ?: Server.ASK_START,
            push = input.opt("push") == true,
            start = input.opt("start") == true || (old == null && servers.isEmpty()),
            trustedAt = old?.trustedAt ?: "",
            unrevoked = if (old != null && old.node == node) old.unrevoked else emptyList(),
            unended = if (old != null && old.node == node) old.unended else emptyList(),
            held = if (old != null && old.node == node) old.held else null,
        )
        if (s.start) servers.replaceAll { if (it.start && it.id != s.id) it.copy(start = false) else it }
        val at = servers.indexOfFirst { it.id == s.id }
        if (at >= 0) servers[at] = s else servers.add(s)
        write()
        return s
    }

    /** remove forgets the server, with its pinned certificate; one not kept is no error. */
    @Synchronized
    fun remove(id: String) {
        if (servers.removeAll { it.id == id }) write()
    }

    /** trust pins [fp] for the server, in the server's form, and notes when (`trusted_at`). */
    @Synchronized
    fun trust(id: String, fp: String) {
        val canonical = Fingerprint.canonical(fp) ?: throw BridgeException(R.string.err_bad_fp)
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0) throw BridgeException(R.string.err_no_server)
        servers[at] = servers[at].copy(fp = canonical, trustedAt = now())
        write()
    }

    /**
     * leave notes the key [keyId] of [user]'s as left valid on the server
     * [id] (KeysLeft), with its revoke [code] where the app has one, and
     * reports whether it is noted now: a key noted already is, once, and
     * takes the code where it had none; a key without an id, or of a server
     * not kept, is not.
     */
    @Synchronized
    fun leave(id: String, user: String, keyId: Long, code: String = ""): Boolean {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || keyId <= 0 || user.isEmpty()) return false
        val key = KeyLeft(user, keyId, code.takeIf(RevokeCode::valid) ?: "")
        val had = servers[at].unrevoked.firstOrNull { it.same(key) }
        if (had != null && (had.code.isNotEmpty() || had.taken || key.code.isEmpty())) return true
        servers[at] = servers[at].copy(unrevoked = servers[at].unrevoked.filter { !it.same(key) } + key)
        write()
        return true
    }

    /** revoked drops [keys] from the keys left on the server [id], revoked at last or gone there. */
    @Synchronized
    fun revoked(id: String, keys: Collection<KeyLeft>) {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || keys.isEmpty()) return
        val rest = servers[at].unrevoked.filter { k -> keys.none { it.same(k) } }
        if (rest.size == servers[at].unrevoked.size) return
        servers[at] = servers[at].copy(unrevoked = rest)
        write()
    }

    /**
     * leaveSession notes the session [sessionId] of [user]'s, made at
     * [createdAt], as left valid on the server [id] (D70-A), with its revoke
     * [code] where the app has one, and reports whether it is noted now: a
     * session noted already is, once, and takes the code where it had none;
     * one without an id, or of a server not kept, is not.
     */
    @Synchronized
    fun leaveSession(id: String, user: String, sessionId: Long, createdAt: String = "", code: String = ""): Boolean {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || sessionId <= 0 || user.isEmpty()) return false
        val left = SessionLeft(user, sessionId, createdAt, code.takeIf(RevokeCode::valid) ?: "", leftAt = now())
        val had = servers[at].unended.firstOrNull { it.same(left) }
        if (had != null && (had.code.isNotEmpty() || had.taken || left.code.isEmpty())) return true
        servers[at] = servers[at].copy(unended = servers[at].unended.filter { !it.same(left) } + left)
        write()
        return true
    }

    /** ended drops [sessions] from the sessions left on the server [id], revoked at last or gone there. */
    @Synchronized
    fun ended(id: String, sessions: Collection<SessionLeft>) {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || sessions.isEmpty()) return
        val rest = servers[at].unended.filter { x -> sessions.none { it.same(x) } }
        if (rest.size == servers[at].unended.size) return
        servers[at] = servers[at].copy(unended = rest)
        write()
    }

    /**
     * takeBack takes a removed server's revoke [codes] into the server [id],
     * which has its node (AP MobileApp, D117-A): each a key or a session
     * left there, with its user, id, `created_at` and code, as [leave] and
     * [leaveSession] note them, an entry noted already keeping itself and
     * taking the code where it had none and was not taken; a code without a
     * user or id is passed over ([GoneCode.takeable]). It reports whether
     * the server is kept, the codes then in its entry, written; false for a
     * server not kept. A write that fails leaves the entry as it was, and
     * throws.
     */
    @Synchronized
    fun takeBack(id: String, codes: List<GoneCode>): Boolean {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0) return false
        val before = servers[at]
        var keys = before.unrevoked
        var sessions = before.unended
        for (c in codes) {
            if (!c.takeable() || !RevokeCode.valid(c.code)) continue
            if (c.session) {
                val left = SessionLeft(c.user, c.id, c.createdAt, c.code, leftAt = c.leftAt)
                val had = sessions.firstOrNull { it.same(left) }
                sessions = when {
                    had == null -> sessions + left
                    had.code.isNotEmpty() || had.taken -> sessions
                    else -> sessions.map { if (it.same(left)) it.copy(code = c.code, leftAt = it.leftAt.ifEmpty { c.leftAt }) else it }
                }
            } else {
                val key = KeyLeft(c.user, c.id, c.code)
                val had = keys.firstOrNull { it.same(key) }
                keys = when {
                    had == null -> keys + key
                    had.code.isNotEmpty() || had.taken -> keys
                    else -> keys.map { if (it.same(key)) it.copy(code = c.code) else it }
                }
            }
        }
        // compared entry by entry with what each says of its code, since an entry equals another by its token alone
        val same = keys.map { it.fileJson().toString() } == before.unrevoked.map { it.fileJson().toString() } &&
            sessions.map { it.fileJson().toString() } == before.unended.map { it.fileJson().toString() }
        if (same) return true
        servers[at] = before.copy(unrevoked = keys, unended = sessions)
        try {
            write()
        } catch (e: IOException) {
            servers[at] = before
            throw e
        }
        return true
    }

    /**
     * took marks the key or session left on the server [id] whose revoke
     * code is [code] as taken by the server (`204`), its code dropped: its
     * line goes from the page, and the next session there for its user
     * checks it (KeysLeft.revoke). It reports whether one was marked.
     */
    @Synchronized
    fun took(id: String, code: String): Boolean = withCode(id, code) { it.copy(code = "", taken = true) }

    /**
     * dropCode drops the revoke code [code] of the key or session left on
     * the server [id], the entry staying as before codes: a server that
     * takes no codes, or a code it found malformed.
     */
    @Synchronized
    fun dropCode(id: String, code: String): Boolean = withCode(id, code) { it.copy(code = "") }

    // withCode changes the entries of the server [id] whose code is [code] by
    // [key] and [session] alike, and writes the file where one changed.
    private fun withCode(id: String, code: String, change: (Pending) -> Pending): Boolean {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || code.isEmpty()) return false
        val s = servers[at]
        var changed = false
        val keys = s.unrevoked.map { k ->
            if (k.code != code) return@map k
            changed = true
            change(Pending(k.code, k.taken)).let { p -> k.copy(code = p.code, taken = p.taken) }
        }
        val sessions = s.unended.map { x ->
            if (x.code != code) return@map x
            changed = true
            change(Pending(x.code, x.taken)).let { p -> x.copy(code = p.code, taken = p.taken) }
        }
        if (!changed) return false
        servers[at] = s.copy(unrevoked = keys, unended = sessions)
        write()
        return true
    }

    // Pending is what an entry left says of its code.
    private data class Pending(val code: String, val taken: Boolean)

    /**
     * hold records [held] as the session the app holds at the server [id]
     * (D102-A), in place of one recorded before; nothing for a server not
     * kept.
     */
    @Synchronized
    fun hold(id: String, held: HeldSession) {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || held.id <= 0 || held.user.isEmpty()) return
        val had = servers[at].held
        if (had != null && had == held && had.code == held.code && had.keyId == held.keyId) return
        servers[at] = servers[at].copy(held = held)
        write()
    }

    /**
     * unhold clears the session recorded as held at the server [id]: it
     * ended, went into `unended`, or was replaced; with [sessionId] only
     * where the record is that session's, so that a record of the next one
     * stays.
     */
    @Synchronized
    fun unhold(id: String, sessionId: Long = 0) {
        val at = servers.indexOfFirst { it.id == id }
        val had = if (at < 0) null else servers[at].held
        if (had == null || sessionId > 0 && had.id != sessionId) return
        servers[at] = servers[at].copy(held = null)
        write()
    }

    /**
     * leaveHeld moves the session recorded as held at each server into its
     * `unended` where [holds] says the app holds it no more (D102-A): at a
     * new process's start, the sessions kept in memory gone with the last,
     * and at Exit; a session the app still holds, sealed, stays recorded.
     */
    @Synchronized
    fun leaveHeld(holds: (Server) -> Boolean) {
        var changed = false
        servers.replaceAll { s ->
            val h = s.held
            if (h == null || holds(s)) return@replaceAll s
            changed = true
            val left = h.left(now())
            s.copy(held = null, unended = if (s.unended.any { it.same(left) }) s.unended else s.unended + left)
        }
        if (changed) write()
    }

    /**
     * takeLegacy is the ids of the servers that had version 1's switch on,
     * once: their sessions were kept behind a prompt the app no longer
     * asks, so the caller forgets them, and the file is written without
     * the switch.
     */
    @Synchronized
    fun takeLegacy(): Set<String> {
        if (legacy.isEmpty()) return emptySet()
        val ids = legacy.toSet()
        legacy.clear()
        write()
        return ids
    }

    private fun load(): MutableList<Server> {
        if (!file.isFile) return mutableListOf()
        return try {
            val a = JSONArray(file.readText())
            MutableList(a.length()) { i ->
                val o = a.getJSONObject(i)
                if (o.opt("biometric") == true) legacy.add(str(o, "id"))
                Server(
                    id = str(o, "id"),
                    name = str(o, "name"),
                    node = str(o, "node"),
                    lan = str(o, "lan"),
                    remote = str(o, "remote"),
                    fp = str(o, "fp"),
                    ask = str(o, "ask").takeIf { it in Server.ASKS } ?: Server.ASK_START,
                    push = o.opt("push") == true,
                    start = o.opt("start") == true,
                    trustedAt = str(o, "trusted_at"),
                    unrevoked = keysLeft(o.optJSONArray("unrevoked")),
                    unended = sessionsLeft(o.optJSONArray("unended")),
                    held = heldOf(o.optJSONObject("held")),
                )
            }.filter { it.id.isNotEmpty() }.let(::oneAtStart).toMutableList()
        } catch (e: JSONException) {
            // A file the app cannot read is kept for a look, not lost.
            file.renameTo(File(file.path + ".bad"))
            mutableListOf()
        } catch (e: IOException) {
            mutableListOf()
        }
    }

    private fun write() {
        val a = JSONArray()
        servers.forEach { a.put(it.fileJson()) }
        val tmp = File(file.path + ".new")
        tmp.writeText(a.toString())
        if (!tmp.renameTo(file)) throw IOException("cannot replace ${file.name}")
    }

    companion object {
        private const val MAX_TEXT = 200
        private val random = SecureRandom()

        /** nowUtc is the time now as `trusted_at` writes it: RFC 3339 in UTC, to the second. */
        fun nowUtc(): String = Instant.now().truncatedTo(ChronoUnit.SECONDS).toString()

        // keysLeft reads a server's keys left, each {user, id}, leaving out
        // what is not one.
        private fun keysLeft(a: JSONArray?): List<KeyLeft> {
            if (a == null) return emptyList()
            return (0 until a.length()).mapNotNull { i ->
                val o = a.optJSONObject(i) ?: return@mapNotNull null
                KeyLeft(str(o, "user"), o.optLong("id"), str(o, "code").takeIf(RevokeCode::valid) ?: "", o.optBoolean("taken"))
                    .takeIf { it.user.isNotEmpty() && it.id > 0 }
            }.distinctBy { it.user to it.id }
        }

        // sessionsLeft reads a server's sessions left, each {user, id},
        // leaving out what is not one.
        private fun sessionsLeft(a: JSONArray?): List<SessionLeft> {
            if (a == null) return emptyList()
            return (0 until a.length()).mapNotNull { i ->
                val o = a.optJSONObject(i) ?: return@mapNotNull null
                SessionLeft(str(o, "user"), o.optLong("id"), str(o, "created_at"), str(o, "code").takeIf(RevokeCode::valid) ?: "", o.optBoolean("taken"), str(o, "left_at"))
                    .takeIf { it.user.isNotEmpty() && it.id > 0 }
            }.distinctBy { Triple(it.user, it.id, it.createdAt) }
        }

        // heldOf reads a server's session held, {user, id, created_at, code, key_id}, null
        // for none or what is not one.
        private fun heldOf(o: JSONObject?): HeldSession? {
            if (o == null) return null
            return HeldSession(str(o, "user"), o.optLong("id"), str(o, "created_at"), str(o, "code").takeIf(RevokeCode::valid) ?: "", o.optLong("key_id").coerceAtLeast(0))
                .takeIf { it.user.isNotEmpty() && it.id > 0 }
        }

        // oneAtStart keeps the first server marked to open at start, and no other.
        private fun oneAtStart(list: List<Server>): List<Server> {
            val first = list.indexOfFirst { it.start }
            return list.mapIndexed { i, s -> if (s.start && i != first) s.copy(start = false) else s }
        }

        /** str is the string at [key], or "" where there is none or it is not a string. */
        fun str(o: JSONObject, key: String): String = o.opt(key) as? String ?: ""

        /**
         * address checks a server's address: "" for none, else an http or
         * https URL with a host and a path ending in `/`, no user, query or
         * fragment, and not the app's own origin.
         */
        fun address(url: String): String {
            val s = url.trim()
            if (s.isEmpty()) return ""
            val bad = BridgeException(R.string.err_bad_address, s)
            val u = try {
                URI(s)
            } catch (e: java.net.URISyntaxException) {
                throw bad
            }
            val origin = Origins.of(s) ?: throw bad
            if (origin == Origins.APP || u.rawUserInfo != null || u.rawQuery != null || u.rawFragment != null) throw bad
            if (!u.rawPath.orEmpty().endsWith("/")) throw bad
            return s
        }

        private fun newId(): String {
            val b = ByteArray(12)
            random.nextBytes(b)
            return b.joinToString("") { (it.toInt() and 0xff).toString(16).padStart(2, '0') }
        }
    }
}

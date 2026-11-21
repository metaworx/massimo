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
 */
data class KeyLeft(val user: String, val id: Long) {
    /** toJson is the key as the connect page sees it, `{user, id}`. */
    fun toJson(): JSONObject = JSONObject().put("user", user).put("id", id)
}

/**
 * A server the app keeps (docs/app.md, "A server"); [ask] is when the
 * fingerprint is asked once a login is kept, one of [ASKS]; [start] marks
 * the one server the app opens at its start; [trustedAt] is when its
 * pinned fingerprint was trusted (RFC 3339), "" for none; [unrevoked] are
 * the keys of the phone's left valid there (contract version 4).
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
) {
    /**
     * toJson is the server as the connect page sees it, with the user whose
     * login is kept, [login], or "", the user of the session the app holds
     * and checked, [session], or null (contract version 3), and the keys
     * left valid there (version 4).
     */
    fun toJson(login: String = "", session: String? = null): JSONObject = JSONObject()
        .put("id", id)
        .put("name", name)
        .put("node", node)
        .put("lan", lan)
        .put("remote", remote)
        .put("fp", fp)
        .put("login", login)
        .put("ask", ask)
        .put("push", push)
        .put("start", start)
        .put("session", session ?: JSONObject.NULL)
        .put("trusted_at", trustedAt)
        .put("unrevoked", JSONArray().apply { unrevoked.forEach { put(it.toJson()) } })

    /** fileJson is the server as the store keeps it: what the page sees, but the login and the session, which are kept elsewhere. */
    fun fileJson(): JSONObject = toJson().apply {
        remove("login")
        remove("session")
    }

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

        // My profile's fragment that marks a key (pages.js readProfileHash).
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
     * [trust] set them, and `unrevoked` as [leave] and [revoked] set it,
     * dropped where the node changed: those keys are another server's.
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
     * [id] (KeysLeft), and reports whether it is noted now: a key noted
     * already is, once; a key without an id, or of a server not kept, is not.
     */
    @Synchronized
    fun leave(id: String, user: String, keyId: Long): Boolean {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || keyId <= 0 || user.isEmpty()) return false
        val key = KeyLeft(user, keyId)
        if (key in servers[at].unrevoked) return true
        servers[at] = servers[at].copy(unrevoked = servers[at].unrevoked + key)
        write()
        return true
    }

    /** revoked drops [keys] from the keys left on the server [id], revoked at last or gone there. */
    @Synchronized
    fun revoked(id: String, keys: Collection<KeyLeft>) {
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0 || keys.isEmpty()) return
        val rest = servers[at].unrevoked - keys.toSet()
        if (rest.size == servers[at].unrevoked.size) return
        servers[at] = servers[at].copy(unrevoked = rest)
        write()
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
                KeyLeft(str(o, "user"), o.optLong("id")).takeIf { it.user.isNotEmpty() && it.id > 0 }
            }.distinct()
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

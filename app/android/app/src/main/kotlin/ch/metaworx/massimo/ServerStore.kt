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

/**
 * A server the app keeps (docs/app.md, "A server"); [ask] is when the
 * fingerprint is asked once a login is kept, one of [ASKS].
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
) {
    /** toJson is the server as the connect page sees it, with the user whose login is kept, [login], or "". */
    fun toJson(login: String = ""): JSONObject = JSONObject()
        .put("id", id)
        .put("name", name)
        .put("node", node)
        .put("lan", lan)
        .put("remote", remote)
        .put("fp", fp)
        .put("login", login)
        .put("ask", ask)
        .put("push", push)

    /** has reports whether [url] is one of the server's two addresses. */
    fun has(url: String): Boolean = url.isNotEmpty() && (url == lan || url == remote)

    companion object {
        /** The choices of when the fingerprint is asked: at each return, at each start, after a logout. */
        const val ASK_RETURN = "return"
        const val ASK_START = "start"
        const val ASK_LOGOUT = "logout"
        val ASKS = setOf(ASK_RETURN, ASK_START, ASK_LOGOUT)
    }
}

/**
 * ServerStore keeps the servers, in their order, as a JSON array in one
 * file of the app's own; every change is written at once, through a file
 * beside it renamed over it.
 */
class ServerStore(private val file: File) {
    // The servers that had version 1's switch "Ask for my fingerprint or
    // PIN" on, found by load: their session is forgotten once (takeLegacy).
    private val legacy = mutableSetOf<String>()
    private val servers: MutableList<Server> = load()

    @Synchronized
    fun list(): List<Server> = servers.toList()

    @Synchronized
    fun get(id: String): Server? = servers.firstOrNull { it.id == id }

    /**
     * save adds [input] where it has no `id`, giving it one, or replaces the
     * server with its `id`, in place. `fp` is kept as [trust] set it.
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
        val s = Server(
            id = old?.id ?: newId(),
            name = name,
            node = str(input, "node").trim().take(MAX_TEXT),
            lan = lan,
            remote = remote,
            fp = old?.fp ?: "",
            ask = str(input, "ask").takeIf { it in Server.ASKS } ?: old?.ask ?: Server.ASK_START,
            push = input.opt("push") == true,
        )
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

    /** trust pins [fp] for the server, in the server's form. */
    @Synchronized
    fun trust(id: String, fp: String) {
        val canonical = Fingerprint.canonical(fp) ?: throw BridgeException(R.string.err_bad_fp)
        val at = servers.indexOfFirst { it.id == id }
        if (at < 0) throw BridgeException(R.string.err_no_server)
        servers[at] = servers[at].copy(fp = canonical)
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
                )
            }.filter { it.id.isNotEmpty() }.toMutableList()
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
        servers.forEach { a.put(it.toJson()) }
        val tmp = File(file.path + ".new")
        tmp.writeText(a.toString())
        if (!tmp.renameTo(file)) throw IOException("cannot replace ${file.name}")
    }

    companion object {
        private const val MAX_TEXT = 200
        private val random = SecureRandom()

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

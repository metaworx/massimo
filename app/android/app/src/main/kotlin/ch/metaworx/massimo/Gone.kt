// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.io.IOException
import java.time.Duration
import java.time.Instant
import java.time.format.DateTimeParseException

/** A revoke code a removed server has still to take: a session's or a key's, and when it was left (RFC 3339, "" for not known). */
data class GoneCode(val code: String, val session: Boolean, val leftAt: String = "") {
    override fun toString(): String = "GoneCode(session=$session, leftAt=$leftAt)"
}

/**
 * A server the user removed whose revoke codes it has still to take (AP
 * MobileApp, D112-A, D116-A): its id, node, addresses and pinned
 * fingerprint, where it is found again, when it was removed, and the codes.
 * Never listed nor opened.
 */
data class GoneServer(
    val id: String,
    val node: String,
    val lan: String,
    val remote: String,
    val fp: String,
    val removedAt: String,
    val codes: List<GoneCode>,
) {
    /** server is the server as Addresses and the probe take it. */
    fun server(): Server = Server(id, "", node, lan, remote, fp, Server.ASK_START, false)
}

/**
 * GoneStore keeps the removed servers' revoke codes that wait (`gone.json`,
 * a file of its own, since `servers.json` is the list's array), written
 * through a file beside it renamed over it. A code goes where the server
 * took it or takes none, a session's 30 days after it was left, the
 * longest a session lives unused, and every code 90 days after the Remove
 * at the latest (D116-A); [now] is the time those are counted at.
 */
class GoneStore(private val file: File, private val now: () -> Instant = Instant::now) {
    /** add keeps the [codes] of [s], removed now; nothing for none. */
    @Synchronized
    fun add(s: Server, codes: List<GoneCode>) {
        val keep = codes.filter { RevokeCode.valid(it.code) }.distinctBy { it.code }
        if (keep.isEmpty()) return
        val all = read().filter { it.id != s.id } + GoneServer(s.id, s.node, s.lan, s.remote, s.fp, now().toString(), keep)
        write(all)
    }

    /** list is the removed servers with codes still to post, the codes past their time dropped (and written). */
    @Synchronized
    fun list(): List<GoneServer> {
        val all = read()
        val live = all.map { g -> g.copy(codes = g.codes.filter { alive(g, it) }) }.filter { it.codes.isNotEmpty() }
        if (live != all) write(live)
        return live
    }

    /** drop drops the code [code] of the removed server [id]: taken, or a server that takes none. */
    @Synchronized
    fun drop(id: String, code: String) {
        val all = read()
        val rest = all.map { g -> if (g.id == id) g.copy(codes = g.codes.filter { it.code != code }) else g }.filter { it.codes.isNotEmpty() }
        if (rest != all) write(rest)
    }

    // alive reports whether [c] of [g] is still to post at now: within 90
    // days of the Remove, and a session's within 30 days of it being left
    // (the Remove where that is not known).
    private fun alive(g: GoneServer, c: GoneCode): Boolean {
        val t = now()
        val removed = parse(g.removedAt) ?: return false
        if (Duration.between(removed, t) >= MAX_KEEP) return false
        if (c.session && Duration.between(parse(c.leftAt) ?: removed, t) >= SESSION_KEEP) return false
        return true
    }

    private fun read(): List<GoneServer> {
        if (!file.isFile) return emptyList()
        return try {
            val a = JSONArray(file.readText())
            (0 until a.length()).mapNotNull { i ->
                val o = a.optJSONObject(i) ?: return@mapNotNull null
                val codes = o.optJSONArray("codes") ?: JSONArray()
                GoneServer(
                    ServerStore.str(o, "id"),
                    ServerStore.str(o, "node"),
                    ServerStore.str(o, "lan"),
                    ServerStore.str(o, "remote"),
                    ServerStore.str(o, "fp"),
                    ServerStore.str(o, "removed_at"),
                    (0 until codes.length()).mapNotNull { j ->
                        val c = codes.optJSONObject(j) ?: return@mapNotNull null
                        GoneCode(ServerStore.str(c, "code"), c.optBoolean("session"), ServerStore.str(c, "left_at")).takeIf { RevokeCode.valid(it.code) }
                    },
                ).takeIf { it.id.isNotEmpty() && it.codes.isNotEmpty() }
            }
        } catch (e: JSONException) {
            emptyList()
        } catch (e: IOException) {
            emptyList()
        }
    }

    private fun write(all: List<GoneServer>) {
        if (all.isEmpty()) {
            file.delete()
            return
        }
        val a = JSONArray()
        for (g in all) {
            val codes = JSONArray()
            g.codes.forEach { codes.put(JSONObject().put("code", it.code).put("session", it.session).put("left_at", it.leftAt)) }
            a.put(
                JSONObject().put("id", g.id).put("node", g.node).put("lan", g.lan).put("remote", g.remote).put("fp", g.fp)
                    .put("removed_at", g.removedAt).put("codes", codes),
            )
        }
        val tmp = File(file.path + ".new")
        tmp.writeText(a.toString())
        if (!tmp.renameTo(file)) throw IOException("cannot replace ${file.name}")
    }

    companion object {
        /** MAX_KEEP is the longest a removed server's code is kept (D116-A). */
        val MAX_KEEP: Duration = Duration.ofDays(90)

        /** SESSION_KEEP is the longest a session's code is kept once left: a session ends 30 days after its last use. */
        val SESSION_KEEP: Duration = Duration.ofDays(30)

        private fun parse(s: String): Instant? = try {
            if (s.isEmpty()) null else Instant.parse(s)
        } catch (e: DateTimeParseException) {
            null
        }
    }
}

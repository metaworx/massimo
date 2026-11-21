// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import com.sun.net.httpserver.HttpExchange
import com.sun.net.httpserver.HttpServer
import java.io.Closeable
import java.net.InetAddress
import java.net.InetSocketAddress

/**
 * FakeServer answers as a Massimo server for the revokes' tests
 * (RevokerTest, KeysLeftTest), on loopback at [base], and records each
 * request as "<method> <path> <bearer>".
 *
 * Anna's sessions: `session-anna` (row 7), made with her password;
 * `session-anna-fp` (row 8), made from her key 42; `session-anna-expiring`
 * (row 10), which may list her tokens but is refused (401) by the revoke;
 * `session-anna-quiet` (row 11), whose list of her tokens comes back empty.
 * `key-own` is her key 45 used as the bearer, `key-secret` her key itself
 * for the revoke by key. `session-bob` is Bob's (row 9); every other bearer
 * is refused (401).
 *
 * Anna's tokens as `GET /api/v1/users/anna/tokens` lists them: her keys
 * 42, 44 to 49 while not revoked, and the sessions 7, 8, 10 and 51; 43 is
 * gone already. The revoke of 42, 45, 46 and 49 succeeds once (then 404),
 * 44 fails (500), 47 is refused (403), 48 drops the connection, and any
 * other id is 404.
 */
class FakeServer : Closeable {
    private val seen = mutableListOf<String>()
    private val gone = mutableSetOf(43L)
    private val server: HttpServer = HttpServer.create(InetSocketAddress(InetAddress.getLoopbackAddress(), 0), 0)

    /** base is the server's address, as a Server keeps it. */
    val base: String

    /** closed is an address where nothing answers: the discard port, as tools/ui-test/run.sh takes it. */
    val closed: String = "http://127.0.0.1:9/"

    init {
        server.createContext("/massimo/api/v1/") { ex -> handle(ex) }
        server.start()
        base = "http://127.0.0.1:${server.address.port}/massimo/"
    }

    /** requests are the requests the server had, in their order. */
    fun requests(): List<String> = synchronized(seen) { seen.toList() }

    override fun close() = server.stop(0)

    private fun handle(ex: HttpExchange) {
        val bearer = ex.requestHeaders.getFirst("Authorization").orEmpty().removePrefix("Bearer ")
        val path = ex.requestURI.rawPath.removePrefix("/massimo")
        synchronized(seen) { seen.add("${ex.requestMethod} $path $bearer") }
        val m = ex.requestMethod
        val anna = setOf("session-anna", "session-anna-fp", "session-anna-expiring", "key-own", "key-secret")
        when {
            m == "GET" && path == "/api/v1/me" && bearer in ME -> answer(ex, 200, ME.getValue(bearer))
            m == "POST" && path == "/api/v1/logout" && bearer in setOf("session-anna", "session-anna-fp", "session-anna-expiring", "session-bob") ->
                answer(ex, 200, """{"ok":true}""")
            m == "GET" && path == "/api/v1/users/anna/tokens" && bearer == "session-anna-quiet" -> answer(ex, 200, "")
            m == "GET" && path == "/api/v1/users/anna/tokens" && bearer in anna -> {
                val keys = (42L..49L).filter { synchronized(gone) { it !in gone } }.joinToString(",") { """{"id":$it,"kind":"api"}""" }
                answer(ex, 200, """[$keys,{"id":7,"kind":"session"},{"id":8,"kind":"session","key_id":42},{"id":10,"kind":"session"},{"id":51,"kind":"session"}]""")
            }
            m == "DELETE" && path.startsWith("/api/v1/users/anna/tokens/") && bearer == "session-bob" ->
                answer(ex, 403, """{"error":"capability \"users\" required"}""")
            m == "DELETE" && path.startsWith("/api/v1/users/anna/tokens/") && bearer == "session-anna-expiring" ->
                answer(ex, 401, """{"error":"missing or wrong token"}""")
            m == "DELETE" && path.startsWith("/api/v1/users/anna/tokens/") && bearer in anna -> {
                val id = path.substringAfterLast('/').toLongOrNull() ?: 0
                when {
                    synchronized(gone) { id in gone } -> answer(ex, 404, """{"error":"user anna has no token $id"}""")
                    id in setOf(42L, 45L, 46L, 49L) -> {
                        synchronized(gone) { gone.add(id) }
                        answer(ex, 200, """{"ok":true,"id":$id}""")
                    }
                    id == 44L -> answer(ex, 500, """{"error":"database is locked"}""")
                    id == 47L -> answer(ex, 403, """{"error":"not yours"}""")
                    id == 48L -> ex.close() // no answer at all
                    else -> answer(ex, 404, """{"error":"user anna has no token $id"}""")
                }
            }
            else -> answer(ex, 401, """{"error":"missing or wrong token"}""")
        }
    }

    private fun answer(ex: HttpExchange, code: Int, body: String) {
        val b = body.toByteArray()
        ex.responseHeaders.add("Content-Type", "application/json")
        ex.sendResponseHeaders(code, b.size.toLong())
        ex.responseBody.use { it.write(b) }
    }

    private companion object {
        // What GET /api/v1/me answers, by bearer.
        val ME = mapOf(
            "session-anna" to """{"mode":"users","user":{"name":"anna"},"token":{"id":7,"kind":"session"}}""",
            "session-anna-fp" to """{"mode":"users","user":{"name":"anna"},"token":{"id":8,"kind":"session","key_id":42}}""",
            "session-anna-expiring" to """{"mode":"users","user":{"name":"anna"},"token":{"id":10,"kind":"session"}}""",
            "session-anna-quiet" to """{"mode":"users","user":{"name":"anna"},"token":{"id":11,"kind":"session"}}""",
            "key-own" to """{"mode":"users","user":{"name":"anna"},"token":{"id":45,"kind":"api"}}""",
            "session-bob" to """{"mode":"users","user":{"name":"bob"},"token":{"id":9,"kind":"session"}}""",
        )
    }
}

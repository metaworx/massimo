// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import com.sun.net.httpserver.HttpExchange
import com.sun.net.httpserver.HttpServer
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import java.net.InetAddress
import java.net.InetSocketAddress
import java.security.cert.CertificateException

/** The adapter's two requests of the login by fingerprint, and the revoke, against a fake server (docs/app.md, docs/api-v1.md). */
class ServerApiTest {
    private val noSystemTrust = Prober.SystemTrust { _, _, _ -> throw CertificateException("not trusted") }
    private val api = ServerApi("Massimo/test (Android 17; SM-X)", noSystemTrust)

    private lateinit var server: HttpServer
    private lateinit var base: String
    private val seen = mutableListOf<String>() // "<method> <path> <bearer> <body>"
    private var keyRevoked = false
    private var selfRoute = 200 // what DELETE /api/v1/me/token answers: 200 where it is there, 404 or 405 at an older server

    private fun answer(ex: HttpExchange, code: Int, body: String) {
        val b = body.toByteArray()
        ex.responseHeaders.add("Content-Type", "application/json")
        ex.sendResponseHeaders(code, b.size.toLong())
        ex.responseBody.use { it.write(b) }
    }

    @Before
    fun start() {
        server = HttpServer.create(InetSocketAddress(InetAddress.getLoopbackAddress(), 0), 0)
        server.createContext("/massimo/api/v1/") { ex ->
            val bearer = ex.requestHeaders.getFirst("Authorization").orEmpty().removePrefix("Bearer ")
            val body = ex.requestBody.readBytes().toString(Charsets.UTF_8)
            val path = ex.requestURI.rawPath.removePrefix("/massimo")
            seen.add("${ex.requestMethod} $path $bearer $body".trim())
            assertEquals("Massimo/test (Android 17; SM-X)", ex.requestHeaders.getFirst("User-Agent"))
            when {
                ex.requestMethod == "GET" && path == "/api/v1/me" && bearer == "session-1" ->
                    answer(ex, 200, """{"mode":"users","user":{"name":"anna maria","role":"operator"}}""")
                ex.requestMethod == "POST" && path == "/api/v1/users/anna%20maria/tokens" && bearer == "session-1" ->
                    answer(ex, 201, """{"token":"key-secret","id":42,"kind":"api","label":"x"}""")
                ex.requestMethod == "POST" && path == "/api/v1/login/key" && bearer == "key-secret" && !keyRevoked ->
                    answer(ex, 200, """{"token":"session-2","expires_at":"2026-12-18T12:00:00Z"}""")
                ex.requestMethod == "DELETE" && path == "/api/v1/users/anna%20maria/tokens/42" && bearer == "key-secret" && !keyRevoked -> {
                    keyRevoked = true
                    answer(ex, 200, """{"ok":true,"id":42}""")
                }
                ex.requestMethod == "DELETE" && path == "/api/v1/me/token" && selfRoute != 200 -> answer(ex, selfRoute, "")
                ex.requestMethod == "DELETE" && path == "/api/v1/me/token" && bearer == "key-secret" && !keyRevoked -> {
                    keyRevoked = true
                    answer(ex, 200, """{"ok":true,"id":42}""")
                }
                ex.requestMethod == "DELETE" && keyRevoked -> answer(ex, 401, """{"error":"missing or wrong token"}""")
                ex.requestMethod == "POST" && path == "/api/v1/login/key" -> answer(ex, 401, """{"error":"missing or wrong token"}""")
                else -> answer(ex, 403, """{"error":"capability \"users\" required"}""")
            }
        }
        server.start()
        base = "http://127.0.0.1:${server.address.port}/massimo/"
    }

    @After
    fun stop() = server.stop(0)

    @Test
    fun keepThenLogInThenRevoke() {
        val user = api.me(base, "", "session-1")
        assertEquals("anna maria", user)
        val key = api.createKey(base, "", "session-1", user, "SM-X, login by fingerprint")
        assertEquals(ServerApi.Key("key-secret", 42), key)
        assertFalse("a key is never printed", key.toString().contains("key-secret"))
        assertTrue(seen.last().endsWith(JSONObject().put("label", "SM-X, login by fingerprint").toString()))
        assertEquals("session-2", api.loginKey(base, "", key.token))
        api.revoke(base, "", key.token, user, key.id)
        try {
            api.loginKey(base, "", key.token)
            fail("a revoked key made a session")
        } catch (e: ServerApi.Refused) {
            assertEquals("missing or wrong token", e.message)
        }
    }

    @Test
    fun theKeyRevokesItselfAtAnyServer() {
        // DELETE /api/v1/me/token, the key as its own bearer (D80-B)
        api.revokeSelf(base, "", "key-secret", "anna maria", 42)
        assertEquals("DELETE /api/v1/me/token key-secret", seen.single())
        assertKeyRevoked()
        // at a server older than the route, which answers 404 or 405: by the user's name and the key's id
        for (older in listOf(404, 405)) {
            seen.clear()
            keyRevoked = false
            selfRoute = older
            api.revokeSelf(base, "", "key-secret", "anna maria", 42)
            assertEquals(listOf("DELETE /api/v1/me/token key-secret", "DELETE /api/v1/users/anna%20maria/tokens/42 key-secret"), seen)
            assertKeyRevoked()
        }
        // a key the server takes no more is Refused, either way
        for (route in listOf(200, 404)) {
            selfRoute = route
            try {
                api.revokeSelf(base, "", "key-secret", "anna maria", 42)
                fail("a revoked key revoked again")
            } catch (e: ServerApi.Refused) {
                assertEquals("missing or wrong token", e.message)
            }
        }
        // another refusal fails with the server's words, and is not taken for an older server
        selfRoute = 403
        try {
            api.revokeSelf(base, "", "key-secret", "anna maria", 42)
            fail("revoked")
        } catch (e: ServerApi.Failed) {
            assertEquals(403, e.status)
        }
    }

    private fun assertKeyRevoked() {
        try {
            api.loginKey(base, "", "key-secret")
            fail("a revoked key made a session")
        } catch (e: ServerApi.Refused) {
            assertEquals("missing or wrong token", e.message)
        }
    }

    @Test
    fun anotherAnswerFailsWithTheServersWords() {
        try {
            api.createKey(base, "", "session-1", "bob", "x")
            fail("created")
        } catch (e: ServerApi.Failed) {
            assertEquals(403, e.status)
            assertEquals("capability \"users\" required", e.message)
        }
    }

    @Test
    fun onlyAServersAddress() {
        for (bad in listOf("http://127.0.0.1:1/massimo", "https://appassets.androidplatform.net/", "")) {
            try {
                api.loginKey(bad, "", "key")
                fail("asked $bad")
            } catch (e: java.io.IOException) {
                // not the address of a server
            }
        }
        assertEquals("anna%20maria", ServerApi.segment("anna maria"))
        assertEquals("a%2Fb", ServerApi.segment("a/b"))
    }

    @Test
    fun aRevokeCodeGoesWithoutATokenAndItsAnswerIsRead() {
        // D112-A: 204 taken, 400 malformed, a broken connection, 429 and 5xx later, any other a server that takes no codes
        val codes = HttpServer.create(InetSocketAddress(InetAddress.getLoopbackAddress(), 0), 0)
        val heard = mutableListOf<String>()
        codes.createContext("/c/") { ex ->
            val status = ex.requestURI.rawPath.removePrefix("/c/").substringBefore('/').toInt()
            synchronized(heard) { heard.add("${ex.requestMethod} ${ex.requestURI.rawPath} ${ex.requestHeaders.getFirst("Authorization")} ${ex.requestBody.readBytes().toString(Charsets.UTF_8)}") }
            if (status in 300..399) ex.responseHeaders.add("Location", "https://elsewhere.example.net/")
            val body = if (status == 200) "<html>a proxy's page</html>".toByteArray() else ByteArray(0)
            ex.sendResponseHeaders(status, if (status == 204) -1 else body.size.toLong())
            if (body.isNotEmpty()) ex.responseBody.use { it.write(body) } else ex.close()
        }
        codes.start()
        try {
            val at = { status: Int -> "http://127.0.0.1:${codes.address.port}/c/$status/" }
            val code = RevokeCode.of("x")
            assertEquals(ServerApi.Taken.TAKEN, api.revokeCode(at(204), "", code))
            assertEquals("POST /c/204/api/v1/tokens/revoke null {\"code\":\"$code\"}", heard.single())
            assertEquals(ServerApi.Taken.MALFORMED, api.revokeCode(at(400), "", code))
            for (later in listOf(429, 500, 502, 503)) assertEquals("$later", ServerApi.Taken.LATER, api.revokeCode(at(later), "", code))
            for (none in listOf(404, 401, 403, 405, 409, 302, 200)) assertEquals("$none", ServerApi.Taken.NONE, api.revokeCode(at(none), "", code))
        } finally {
            codes.stop(0)
        }
        assertEquals("a broken connection", ServerApi.Taken.LATER, api.revokeCode("http://127.0.0.1:9/", "", RevokeCode.of("x")))
    }
}

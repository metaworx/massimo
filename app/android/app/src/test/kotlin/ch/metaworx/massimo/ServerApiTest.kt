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
                ex.requestMethod == "DELETE" && path == "/api/v1/users/anna%20maria/tokens/42" && bearer == "key-secret" -> {
                    keyRevoked = true
                    answer(ex, 200, """{"ok":true,"id":42}""")
                }
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
}

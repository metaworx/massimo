// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext

/**
 * ServerApi makes the adapter's own requests (docs/app.md) at a server's
 * address [base], a URL ending in `/`, over a connection that trusts what
 * the probe trusts: a certificate the system trusts, or the server's pinned
 * fingerprint: the login by fingerprint's, and the check and the end of a
 * session the app holds. The page never sees what they carry. No key or
 * token is ever written to a log.
 */
class ServerApi(
    private val userAgent: String,
    private val systemTrust: Prober.SystemTrust = AndroidTrust(),
) {
    /** Refused is an answer 401: the server takes the bearer no more. */
    class Refused(message: String) : IOException(message)

    /** Failed is any other answer than the one asked for, with the server's message. */
    class Failed(val status: Int, message: String) : IOException(message)

    /** Key is an API key just made: shown once, and its row. */
    data class Key(val token: String, val id: Long) {
        override fun toString(): String = "Key(id=$id)" // never the key
    }

    /**
     * Me is whose a session is: the user's name, the key the session was
     * made from (`token.key_id`, `POST /api/v1/login/key`), 0 for none, the
     * session's own row (`token.id`), its kind (`token.kind`, `session`
     * or `api` for a key the page holds) and when it was made
     * (`token.created_at`, as the server writes it).
     */
    data class Me(val user: String, val keyId: Long, val tokenId: Long = 0, val kind: String = "", val createdAt: String = "")

    /** Token is a row of a user's sessions and keys: its id, its kind, `session` or `api`, and when it was made, as the server writes it. */
    data class Token(val id: Long, val kind: String, val createdAt: String = "")

    /** Taken is how a server answered a revoke code (`POST /api/v1/tokens/revoke`, D112-A). */
    enum class Taken {
        /** 204: taken, whether it matched a live token or not; the code goes. */
        TAKEN,

        /** 400: the server calls the code malformed; it goes, logged. */
        MALFORMED,

        /** A broken connection, 429 or 5xx: the code waits for the next contact. */
        LATER,

        /** Any other answer, 404 first: a server that takes no codes; the code goes, the entry as before codes. */
        NONE,
    }

    /** me is the name of the user whose [session] it is (`GET /api/v1/me`). */
    fun me(base: String, fp: String, session: String): String = whoami(base, fp, session).user

    /** whoami is whose [session] is (`GET /api/v1/me`): the user, and the key it was made from. */
    fun whoami(base: String, fp: String, session: String): Me {
        val o = request("GET", base, "api/v1/me", fp, session, null, HttpURLConnection.HTTP_OK)
        val user = o.optJSONObject("user")?.let { ServerStore.str(it, "name") } ?: ""
        if (user.isEmpty()) throw Failed(HttpURLConnection.HTTP_OK, "no user signed in")
        val token = o.optJSONObject("token")
        return Me(
            user,
            token?.optLong("key_id") ?: 0,
            token?.optLong("id") ?: 0,
            token?.let { ServerStore.str(it, "kind") } ?: "",
            token?.let { ServerStore.str(it, "created_at") } ?: "",
        )
    }

    /** tokens are the sessions and keys of [user]'s (`GET /api/v1/users/{name}/tokens`), asked with [bearer], the user's. */
    fun tokens(base: String, fp: String, bearer: String, user: String): List<Token> {
        val text = send("GET", base, "api/v1/users/${segment(user)}/tokens", fp, bearer, null, HttpURLConnection.HTTP_OK)
        // an empty answer is no list: a list without tokens is "[]"
        if (text.isBlank()) throw Failed(HttpURLConnection.HTTP_OK, "no list of tokens in the answer")
        val a = try {
            JSONArray(text)
        } catch (e: JSONException) {
            throw Failed(HttpURLConnection.HTTP_OK, "no list of tokens in the answer")
        }
        return (0 until a.length()).mapNotNull { i ->
            a.optJSONObject(i)?.let { Token(it.optLong("id"), ServerStore.str(it, "kind"), ServerStore.str(it, "created_at")) }
        }
    }

    /** createKey makes an API key of [user]'s, labelled [label], with the user's [session]. */
    fun createKey(base: String, fp: String, session: String, user: String, label: String): Key {
        val o = request("POST", base, "api/v1/users/${segment(user)}/tokens", fp, session, JSONObject().put("label", label), HttpURLConnection.HTTP_CREATED)
        val token = ServerStore.str(o, "token")
        if (token.isEmpty()) throw Failed(HttpURLConnection.HTTP_CREATED, "no key in the answer")
        return Key(token, o.optLong("id"))
    }

    /** loginKey makes a session from [key] (`POST /api/v1/login/key`); [Refused] where the server takes the key no more. */
    fun loginKey(base: String, fp: String, key: String): String {
        val o = request("POST", base, "api/v1/login/key", fp, key, JSONObject(), HttpURLConnection.HTTP_OK)
        val token = ServerStore.str(o, "token")
        if (token.isEmpty()) throw Failed(HttpURLConnection.HTTP_OK, "no session in the answer")
        return token
    }

    /** logout ends [session] (`POST /api/v1/logout`); [Refused] where the server had ended it already. */
    fun logout(base: String, fp: String, session: String) {
        request("POST", base, "api/v1/logout", fp, session, JSONObject(), HttpURLConnection.HTTP_OK)
    }

    /** revoke revokes the key [id] of [user]'s, with [bearer], the key itself or a session of the user's. */
    fun revoke(base: String, fp: String, bearer: String, user: String, id: Long) {
        request("DELETE", base, "api/v1/users/${segment(user)}/tokens/$id", fp, bearer, null, HttpURLConnection.HTTP_OK)
    }

    /**
     * revokeCode posts the revoke code [code] of a token the app forgot
     * (`POST /api/v1/tokens/revoke`, D112-A), without `Authorization`: a code
     * lets nothing in, so plain HTTP outside the private ranges carries it
     * too. 204 is [Taken.TAKEN], 400 [Taken.MALFORMED], a broken connection,
     * 429 and 5xx [Taken.LATER], and any other answer, a server older than
     * the route (404), 401, 403, 405, 409, a redirect or a proxy's page of
     * 200, [Taken.NONE].
     */
    fun revokeCode(base: String, fp: String, code: String): Taken = try {
        send("POST", base, "api/v1/tokens/revoke", fp, null, JSONObject().put("code", code), HttpURLConnection.HTTP_NO_CONTENT)
        Taken.TAKEN
    } catch (e: Refused) {
        Taken.NONE
    } catch (e: Failed) {
        when {
            e.status == HttpURLConnection.HTTP_BAD_REQUEST -> Taken.MALFORMED
            e.status == HTTP_TOO_MANY || e.status >= HttpURLConnection.HTTP_INTERNAL_ERROR -> Taken.LATER
            else -> Taken.NONE
        }
    } catch (e: IOException) {
        Taken.LATER
    }

    /**
     * revokeSelf revokes [key], a key of [user]'s whose id is [id], with the
     * key itself as the bearer: `DELETE /api/v1/me/token` (AP MobileApp,
     * D80-B), which needs neither the name nor the id, and at a server older
     * than that route, which answers 404 or 405, [revoke] by the user's name
     * and the key's id. [Refused] where the server takes the key no more.
     */
    fun revokeSelf(base: String, fp: String, key: String, user: String, id: Long) {
        try {
            request("DELETE", base, "api/v1/me/token", fp, key, null, HttpURLConnection.HTTP_OK)
        } catch (e: Failed) {
            if (e.status != HttpURLConnection.HTTP_NOT_FOUND && e.status != HttpURLConnection.HTTP_BAD_METHOD) throw e
            revoke(base, fp, key, user, id)
        }
    }

    private fun request(method: String, base: String, path: String, fp: String, bearer: String, body: JSONObject?, want: Int): JSONObject {
        val text = send(method, base, path, fp, bearer, body, want)
        return try {
            if (text.isBlank()) JSONObject() else JSONObject(text)
        } catch (e: JSONException) {
            JSONObject()
        }
    }

    // send makes a request and returns the answer's text where its status
    // is [want]; [Refused] for a 401, [Failed] with the server's words for
    // any other. A null [bearer] sends no Authorization.
    private fun send(method: String, base: String, path: String, fp: String, bearer: String?, body: JSONObject?, want: Int): String {
        val target = Prober.healthz(base)?.removeSuffix("healthz")?.plus(path) ?: throw IOException("not the address of a server")
        val conn = URL(target).openConnection() as HttpURLConnection
        conn.connectTimeout = TIMEOUT_MS
        conn.readTimeout = TIMEOUT_MS
        conn.instanceFollowRedirects = false
        conn.useCaches = false
        conn.requestMethod = method
        conn.setRequestProperty("User-Agent", userAgent)
        conn.setRequestProperty("Accept", "application/json")
        if (bearer != null) conn.setRequestProperty("Authorization", "Bearer $bearer")
        (conn as? HttpsURLConnection)?.let { https ->
            val accept = fp.ifEmpty { null }?.let(Fingerprint::canonical)
            val tm = PinningTrustManager(systemTrust, https.url.host.removePrefix("[").removeSuffix("]"), accept)
            https.sslSocketFactory = SSLContext.getInstance("TLS").apply { init(null, arrayOf(tm), null) }.socketFactory
            https.hostnameVerifier = HostnameVerifier { name, session -> tm.verifyHost(name, session) }
        }
        try {
            if (body != null) {
                conn.doOutput = true
                conn.setRequestProperty("Content-Type", "application/json")
                conn.outputStream.use { it.write(body.toString().toByteArray(Charsets.UTF_8)) }
            }
            val code = conn.responseCode
            val text = (if (code < 400) conn.inputStream else conn.errorStream)?.use { readAtMost(it) } ?: ""
            if (code == want) return text
            val o = try {
                if (text.isBlank()) JSONObject() else JSONObject(text)
            } catch (e: JSONException) {
                JSONObject()
            }
            val said = ServerStore.str(o, "error").ifEmpty { "status $code" }
            if (code == HttpURLConnection.HTTP_UNAUTHORIZED) throw Refused(said)
            throw Failed(code, said)
        } finally {
            conn.disconnect()
        }
    }

    companion object {
        private const val TIMEOUT_MS = 10_000
        private const val MAX_BODY = 64 * 1024
        private const val HTTP_TOO_MANY = 429

        /** segment is [s] as one segment of a URL's path. */
        fun segment(s: String): String = URLEncoder.encode(s, "UTF-8").replace("+", "%20")

        private fun readAtMost(input: InputStream): String {
            val buf = ByteArray(MAX_BODY)
            var n = 0
            while (n < MAX_BODY) {
                val r = input.read(buf, n, MAX_BODY - n)
                if (r < 0) break
                n += r
            }
            return String(buf, 0, n, Charsets.UTF_8)
        }
    }
}

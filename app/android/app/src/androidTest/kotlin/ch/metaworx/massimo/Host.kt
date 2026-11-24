// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import java.net.URL
import java.net.URLEncoder
import java.security.cert.X509Certificate
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLSession
import javax.net.ssl.X509TrustManager

/**
 * Host is a server of the suite's as the test itself reads it, over the
 * certificate pinned by its fingerprint, as the app pins it: its audit
 * (`GET /api/v1/audit`, with the owner's key), where the app's steps are
 * read back, and who a session is (`GET /api/v1/me`). Nothing it sends or
 * reads is logged.
 */
class Host(private val target: Target) {
    /** Row is an audit row: its id, action, target (`token:<id>`, `user:<name>`), the token that made the request, and how it went. */
    data class Row(val id: Long, val action: String, val target: String, val tokenId: Long, val result: String)

    private val pinned = Pinned(target.fp)
    private val ssl = SSLContext.getInstance("TLS").apply { init(null, arrayOf(pinned), null) }

    /** get sends `GET <path>` with [bearer] and returns the status and the body. */
    fun get(path: String, bearer: String): Pair<Int, String> {
        val c = URL(target.url + path).openConnection() as HttpsURLConnection
        c.sslSocketFactory = ssl.socketFactory
        c.hostnameVerifier = pinned // the certificate is the pinned one, whatever names it holds
        c.connectTimeout = TIMEOUT_MS
        c.readTimeout = TIMEOUT_MS
        c.setRequestProperty("Authorization", "Bearer $bearer")
        try {
            val code = c.responseCode
            val body = (if (code < HTTP_ERRORS) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
            return code to body
        } finally {
            c.disconnect()
        }
    }

    /** rows are the user's newest audit rows, at most [limit], newest first. */
    fun rows(limit: Int = ROWS): List<Row> {
        val (code, body) = get("api/v1/audit?user=" + URLEncoder.encode(Rig.user, "UTF-8") + "&limit=$limit", Rig.auditKey)
        assertEquals("GET /api/v1/audit at ${target.url}", 200, code)
        val a = JSONArray(body)
        return List(a.length()) { i ->
            val o = a.getJSONObject(i)
            Row(o.optLong("id"), o.optString("action"), o.optString("target"), o.optLong("token_id"), o.optString("result"))
        }
    }

    /** mark is the newest audit row's id, 0 for none, before a step whose rows [since] reads. */
    fun mark(): Long = rows(1).firstOrNull()?.id ?: 0

    /** since are the user's audit rows after [mark], oldest first. */
    fun since(mark: Long): List<Row> = rows().filter { it.id > mark }.sortedBy { it.id }

    /** sessionId is the row of the session [token] at the server (`GET /api/v1/me`'s `token.id`). */
    fun sessionId(token: String): Long {
        val (code, body) = get("api/v1/me", token)
        assertEquals("GET /api/v1/me at ${target.url}", 200, code)
        val id = JSONObject(body).optJSONObject("token")?.optLong("id") ?: 0
        if (id <= 0) fail("no session's id in GET /api/v1/me at ${target.url}")
        return id
    }

    /** status is the status `GET /api/v1/me` answers to [token]: 200 while it is valid, 401 once it ended. */
    fun status(token: String): Int = get("api/v1/me", token).first

    // Pinned trusts the one certificate whose fingerprint is the server's
    // (Fingerprint, as the server writes it), for any host name.
    private class Pinned(private val fp: String) : X509TrustManager, HostnameVerifier {
        override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
            val leaf = chain?.firstOrNull() ?: throw java.security.cert.CertificateException("no certificate")
            if (!Fingerprint.same(Fingerprint.of(leaf.encoded), fp)) throw java.security.cert.CertificateException("not the pinned certificate")
        }

        override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) =
            throw java.security.cert.CertificateException("no client certificates")

        override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()

        override fun verify(hostname: String?, session: SSLSession?): Boolean {
            val leaf = session?.peerCertificates?.firstOrNull() as? X509Certificate ?: return false
            return Fingerprint.same(Fingerprint.of(leaf.encoded), fp)
        }
    }

    private companion object {
        const val TIMEOUT_MS = 10_000
        const val HTTP_ERRORS = 400
        const val ROWS = 200
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.annotation.SuppressLint
import android.net.http.X509TrustManagerExtensions
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.SocketTimeoutException
import java.net.URL
import java.security.KeyStore
import java.security.cert.CertificateException
import java.security.cert.CertificateParsingException
import java.security.cert.X509Certificate
import java.time.temporal.ChronoUnit
import java.util.Date
import java.util.concurrent.Callable
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLSession
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager
import javax.security.auth.x500.X500Principal

/**
 * What the probe saw of a server's certificate (docs/app.md, "A probe
 * result"): its fingerprint, whether it is its own issuer, whether it was
 * trusted, and what it says of itself ([cert], contract version 3).
 */
data class Tls(val fingerprint: String, val selfSigned: Boolean, val trusted: Boolean, val cert: CertDetails? = null)

/**
 * CertDetails is what a certificate says of itself, as the connect page
 * shows it in a server's settings (docs/app.md, "A probe result"): its
 * subject's and its issuer's distinguished names (RFC 4514), the DNS names
 * and IP addresses it is for, and its validity in RFC 3339, to the second.
 */
data class CertDetails(val subject: String, val issuer: String, val names: List<String>, val notBefore: String, val notAfter: String) {
    /** put writes the details into a probe result's [o]. */
    fun put(o: JSONObject): JSONObject = o
        .put("subject", subject)
        .put("issuer", issuer)
        .put("names", JSONArray(names))
        .put("not_before", notBefore)
        .put("not_after", notAfter)

    companion object {
        private const val SAN_DNS = 2
        private const val SAN_IP = 7

        /** of reads the details of [c]. */
        fun of(c: X509Certificate): CertDetails = CertDetails(
            subject = c.subjectX500Principal.getName(X500Principal.RFC2253),
            issuer = c.issuerX500Principal.getName(X500Principal.RFC2253),
            names = names(c),
            notBefore = utc(c.notBefore),
            notAfter = utc(c.notAfter),
        )

        // names are the subject alternative names that are a DNS name or
        // an IP address, in the certificate's order; none where it cannot
        // be read.
        private fun names(c: X509Certificate): List<String> = try {
            c.subjectAlternativeNames.orEmpty().mapNotNull { e ->
                val kind = e.getOrNull(0) as? Int
                if (kind == SAN_DNS || kind == SAN_IP) e.getOrNull(1) as? String else null
            }
        } catch (e: CertificateParsingException) {
            emptyList()
        }

        private fun utc(d: Date): String = d.toInstant().truncatedTo(ChronoUnit.SECONDS).toString()
    }
}

/** ProbeResult is one address's answer to `GET <url>healthz`. */
data class ProbeResult(
    val url: String,
    val ok: Boolean,
    val node: String = "",
    val version: String = "",
    val tls: Tls? = null,
    val error: String? = null,
) {
    /** toJson is the result as the connect page reads it (docs/app.md, "A probe result"). */
    fun toJson(): JSONObject {
        val o = JSONObject().put("url", url).put("ok", ok).put("node", node).put("version", version)
        if (tls != null) {
            o.put("fingerprint", tls.fingerprint).put("self_signed", tls.selfSigned).put("trusted", tls.trusted)
            tls.cert?.put(o)
        }
        if (!ok && error != null && tls?.trusted != false) o.put("error", error)
        return o
    }
}

/**
 * Prober asks addresses for `/healthz`, in parallel, within four seconds.
 * A certificate counts where the system trusts it for the address's host,
 * or where its fingerprint is the one given; for any other the probe
 * reports its fingerprint and reads nothing from the server.
 */
class Prober(
    private val userAgent: String,
    private val texts: Texts,
    private val systemTrust: SystemTrust = AndroidTrust(),
) {
    /** SystemTrust checks a chain as the system does, for [host]. */
    fun interface SystemTrust {
        @Throws(CertificateException::class)
        fun check(chain: Array<X509Certificate>, authType: String, host: String)
    }

    /** probe asks each of [urls] at once, answering in their order; [fp] is a fingerprint accepted beside the system's. */
    fun probe(urls: List<String>, fp: String?): List<ProbeResult> {
        if (urls.isEmpty()) return emptyList()
        val accept = fp?.let(Fingerprint::canonical)
        val pool = Executors.newFixedThreadPool(urls.size)
        try {
            val futures = urls.map { u -> pool.submit(Callable { one(u, accept) }) }
            val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(TIMEOUT_MS + GRACE_MS)
            return futures.mapIndexed { i, f ->
                try {
                    f.get((deadline - System.nanoTime()).coerceAtLeast(0), TimeUnit.NANOSECONDS)
                } catch (e: TimeoutException) {
                    f.cancel(true)
                    ProbeResult(urls[i], ok = false, error = texts.get(R.string.probe_timeout))
                } catch (e: ExecutionException) {
                    ProbeResult(urls[i], ok = false, error = texts.get(R.string.probe_unreachable, reason(e.cause ?: e)))
                }
            }
        } finally {
            pool.shutdownNow()
        }
    }

    internal fun one(url: String, accept: String?): ProbeResult {
        val target = healthz(url) ?: return ProbeResult(url, ok = false, error = texts.get(R.string.err_bad_address, url))
        val conn = try {
            URL(target).openConnection() as HttpURLConnection
        } catch (e: IOException) {
            return ProbeResult(url, ok = false, error = texts.get(R.string.probe_unreachable, reason(e)))
        }
        conn.connectTimeout = TIMEOUT_MS.toInt()
        conn.readTimeout = TIMEOUT_MS.toInt()
        conn.instanceFollowRedirects = false
        conn.useCaches = false
        conn.setRequestProperty("User-Agent", userAgent)
        conn.setRequestProperty("Accept", "application/json")
        val tm = (conn as? HttpsURLConnection)?.let { https ->
            val tm = PinningTrustManager(systemTrust, https.url.host.removePrefix("[").removeSuffix("]"), accept)
            https.sslSocketFactory = SSLContext.getInstance("TLS").apply { init(null, arrayOf(tm), null) }.socketFactory
            https.hostnameVerifier = HostnameVerifier { name, session -> tm.verifyHost(name, session) }
            tm
        }
        return try {
            val code = conn.responseCode
            val body = if (code == HttpURLConnection.HTTP_OK) conn.inputStream.use { readAtMost(it, MAX_BODY) } else ""
            interpret(url, code, body, tm?.seen())
        } catch (e: IOException) {
            val seen = tm?.seen()
            when {
                seen != null && !seen.trusted -> ProbeResult(url, ok = false, tls = seen)
                e is SocketTimeoutException -> ProbeResult(url, ok = false, tls = seen, error = texts.get(R.string.probe_timeout))
                else -> ProbeResult(url, ok = false, tls = seen, error = texts.get(R.string.probe_unreachable, reason(e)))
            }
        } finally {
            conn.disconnect()
        }
    }

    /** interpret reads an answer of `/healthz`. */
    internal fun interpret(url: String, code: Int, body: String, tls: Tls?): ProbeResult {
        if (code != HttpURLConnection.HTTP_OK) {
            return ProbeResult(url, ok = false, tls = tls, error = texts.get(R.string.probe_status, code))
        }
        val o = try {
            JSONObject(body)
        } catch (e: JSONException) {
            null
        }
        if (o == null || o.opt("ok") != true) {
            return ProbeResult(url, ok = false, tls = tls, error = texts.get(R.string.probe_not_massimo))
        }
        return ProbeResult(url, ok = true, node = ServerStore.str(o, "node_id"), version = ServerStore.str(o, "version"), tls = tls)
    }

    companion object {
        const val TIMEOUT_MS = 4000L
        private const val GRACE_MS = 250L
        private const val MAX_BODY = 64 * 1024

        /** healthz is the URL probed for a server's address, null where it is no address of a server. */
        fun healthz(url: String): String? = try {
            ServerStore.address(url).ifEmpty { null }?.plus("healthz")
        } catch (e: BridgeException) {
            null
        }

        private fun reason(e: Throwable): String = e.message ?: e.javaClass.simpleName

        private fun readAtMost(input: InputStream, max: Int): String {
            val buf = ByteArray(max)
            var n = 0
            while (n < max) {
                val r = input.read(buf, n, max - n)
                if (r < 0) break
                n += r
            }
            return String(buf, 0, n, Charsets.UTF_8)
        }
    }
}

/**
 * PinningTrustManager accepts the certificate whose fingerprint is
 * [accept], or a chain the system trusts for [host], and records what it
 * saw, so that a refused certificate is still reported with its
 * fingerprint. Refusing throws in the handshake: nothing is sent.
 */
@SuppressLint("CustomX509TrustManager") // pinning a self-signed certificate is the point
internal class PinningTrustManager(
    private val system: Prober.SystemTrust,
    private val host: String,
    private val accept: String?,
) : X509TrustManager {
    @Volatile private var fingerprint: String? = null
    @Volatile private var selfSigned = false
    @Volatile private var details: CertDetails? = null
    @Volatile private var pinned = false
    @Volatile private var refused = false

    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {
        val leaf = chain?.firstOrNull() ?: throw CertificateException("no certificate")
        val fp = Fingerprint.of(leaf.encoded)
        fingerprint = fp
        selfSigned = isSelfSigned(leaf)
        details = CertDetails.of(leaf)
        if (accept != null && Fingerprint.same(fp, accept)) {
            pinned = true
            return
        }
        try {
            system.check(chain, authType.orEmpty(), host)
        } catch (e: CertificateException) {
            refused = true
            throw e
        }
    }

    /** verifyHost checks the name of a certificate the system trusts; a pinned one is the server's whatever its names. */
    fun verifyHost(name: String, session: SSLSession): Boolean {
        if (pinned) return true
        val ok = HttpsURLConnection.getDefaultHostnameVerifier().verify(name, session)
        if (!ok) refused = true
        return ok
    }

    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {
        throw CertificateException("no client certificates")
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()

    /** seen is what the handshake showed of the server's certificate, null before one did. */
    fun seen(): Tls? = fingerprint?.let { Tls(it, selfSigned, !refused, details) }

    private fun isSelfSigned(c: X509Certificate): Boolean {
        if (c.subjectX500Principal != c.issuerX500Principal) return false
        return try {
            c.verify(c.publicKey)
            true
        } catch (e: Exception) {
            false
        }
    }
}

/** AndroidTrust is the system's trust, with the network security configuration's rules per host. */
class AndroidTrust : Prober.SystemTrust {
    private val ext: X509TrustManagerExtensions by lazy {
        val f = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        f.init(null as KeyStore?)
        X509TrustManagerExtensions(f.trustManagers.filterIsInstance<X509TrustManager>().first())
    }

    override fun check(chain: Array<X509Certificate>, authType: String, host: String) {
        ext.checkServerTrusted(chain, authType, host)
    }
}

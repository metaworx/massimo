// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import com.sun.net.httpserver.HttpServer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.ServerSocket
import java.security.cert.CertificateException
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

class ProberTest {
    private val texts = Texts { id, args -> "$id:${args.joinToString(",")}" }
    private val noSystemTrust = Prober.SystemTrust { _, _, _ -> throw CertificateException("not trusted") }
    private val prober = Prober("Massimo/test", texts, noSystemTrust)

    private val cert: X509Certificate = javaClass.getResourceAsStream("/selfsigned.pem")!!.use {
        CertificateFactory.getInstance("X.509").generateCertificate(it) as X509Certificate
    }

    @Test
    fun healthzOfAnAddress() {
        assertEquals("http://192.168.1.20:8099/healthz", Prober.healthz("http://192.168.1.20:8099/"))
        assertEquals("https://massimo.example.net/massimo/healthz", Prober.healthz("https://massimo.example.net/massimo/"))
        assertNull(Prober.healthz("http://192.168.1.20:8099"))
        assertNull(Prober.healthz("https://appassets.androidplatform.net/"))
        assertNull(Prober.healthz(""))
    }

    @Test
    fun interpretTheAnswer() {
        val ok = prober.interpret("u/", 200, """{"ok": true, "node_id": "node-a", "version": "massimo v0.1.0"}""", null)
        assertTrue(ok.ok)
        assertEquals("node-a", ok.node)
        assertEquals("massimo v0.1.0", ok.version)
        assertNull(ok.error)

        val noStore = prober.interpret("u/", 200, """{"ok": true, "version": "massimo v0.1.0"}""", null)
        assertTrue(noStore.ok)
        assertEquals("", noStore.node)

        assertEquals(texts.get(R.string.probe_not_massimo), prober.interpret("u/", 200, """{"ok": false}""", null).error)
        assertEquals(texts.get(R.string.probe_not_massimo), prober.interpret("u/", 200, "<html>", null).error)
        assertEquals(texts.get(R.string.probe_status, 503), prober.interpret("u/", 503, "", null).error)
    }

    @Test
    fun resultAsTheContractWritesIt() {
        val plain = ProbeResult("http://192.168.1.20/", ok = true, node = "n", version = "v").toJson()
        assertFalse(plain.has("fingerprint"))
        assertFalse(plain.has("trusted"))
        assertFalse(plain.has("error"))

        val fp = Fingerprint.of(byteArrayOf(1))
        val untrusted = ProbeResult("https://192.168.1.20/", ok = false, tls = Tls(fp, selfSigned = true, trusted = false), error = "x").toJson()
        assertEquals(false, untrusted.get("trusted"))
        assertEquals(true, untrusted.get("self_signed"))
        assertEquals(fp, untrusted.get("fingerprint"))
        assertFalse("no error where the certificate is not trusted", untrusted.has("error"))

        val down = ProbeResult("https://192.168.1.20/", ok = false, tls = Tls(fp, false, true), error = "down").toJson()
        assertEquals("down", down.get("error"))
    }

    @Test
    fun aPinnedCertificateIsTrusted() {
        val fp = Fingerprint.of(cert.encoded)
        val tm = PinningTrustManager(noSystemTrust, "192.168.1.20", fp)
        tm.checkServerTrusted(arrayOf(cert), "ECDHE_ECDSA")
        assertEquals(Tls(fp, selfSigned = true, trusted = true), tm.seen())
    }

    @Test
    fun anotherCertificateIsRefusedAndReported() {
        val other = Fingerprint.of(byteArrayOf(9))
        val tm = PinningTrustManager(noSystemTrust, "192.168.1.20", other)
        try {
            tm.checkServerTrusted(arrayOf(cert), "ECDHE_ECDSA")
            fail("trusted")
        } catch (e: CertificateException) {
            // refused in the handshake
        }
        assertEquals(Tls(Fingerprint.of(cert.encoded), selfSigned = true, trusted = false), tm.seen())
    }

    @Test
    fun theSystemsTrustCounts() {
        var host = ""
        val tm = PinningTrustManager({ _, _, h -> host = h }, "massimo.example.net", null)
        tm.checkServerTrusted(arrayOf(cert), "ECDHE_ECDSA")
        assertEquals("massimo.example.net", host)
        assertTrue(tm.seen()!!.trusted)
    }

    @Test
    fun noCertificateIsRefused() {
        val tm = PinningTrustManager(noSystemTrust, "h", null)
        try {
            tm.checkServerTrusted(emptyArray(), "RSA")
            fail("trusted")
        } catch (e: CertificateException) {
            assertNull(tm.seen())
        }
    }

    @Test
    fun probeAPlainServer() {
        var agent: String? = null
        val server = HttpServer.create(InetSocketAddress(InetAddress.getLoopbackAddress(), 0), 0)
        server.createContext("/healthz") { ex ->
            agent = ex.requestHeaders.getFirst("User-Agent")
            val body = """{"ok": true, "node_id": "node-a", "version": "massimo v0.1.0"}""".toByteArray()
            ex.sendResponseHeaders(200, body.size.toLong())
            ex.responseBody.use { it.write(body) }
        }
        server.createContext("/") { ex ->
            ex.sendResponseHeaders(404, -1)
            ex.close()
        }
        server.start()
        try {
            val base = "http://127.0.0.1:${server.address.port}"
            val closed = ServerSocket(0).use { it.localPort }
            val got = prober.probe(listOf("$base/", "$base/other/", "http://127.0.0.1:$closed/", "nonsense"), null)
            assertEquals(4, got.size)
            assertEquals("Massimo/test", agent)
            assertTrue(got[0].ok)
            assertEquals("node-a", got[0].node)
            assertNull(got[0].tls)
            assertEquals(texts.get(R.string.probe_status, 404), got[1].error)
            assertFalse(got[2].ok)
            assertTrue(got[2].error!!.startsWith("${R.string.probe_unreachable}:"))
            assertEquals(texts.get(R.string.err_bad_address, "nonsense"), got[3].error)
            assertEquals(listOf("$base/", "$base/other/", "http://127.0.0.1:$closed/", "nonsense"), got.map { it.url })
        } finally {
            server.stop(0)
        }
    }

    @Test
    fun aServerThatDoesNotAnswerTimesOut() {
        ServerSocket(0, 1, InetAddress.getLoopbackAddress()).use { silent ->
            val start = System.nanoTime()
            val got = prober.probe(listOf("http://127.0.0.1:${silent.localPort}/"), null)
            val ms = (System.nanoTime() - start) / 1_000_000
            assertEquals(texts.get(R.string.probe_timeout), got.single().error)
            assertTrue("took $ms ms", ms < Prober.TIMEOUT_MS + 1000)
        }
    }
}

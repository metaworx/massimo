// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.util.concurrent.Callable
import java.util.concurrent.ExecutorCompletionService
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

/**
 * Addresses chooses the address a server is opened at as the connect page
 * does (docs/app.md, "Which address"): its LAN address where it answers with
 * the server's node, else its remote one. The app uses it where it opens a
 * server without the page: the server marked to open at start (D43-A), and
 * the sign-out of every server.
 */
object Addresses {
    /** GRACE_MS is how long the LAN address may still answer once the remote one has. */
    const val GRACE_MS = 1000L

    /** good reports whether [r] is [s] answering: `/healthz` ok, with its node where it has one. */
    fun good(s: Server, r: ProbeResult?): Boolean = r != null && r.ok && (s.node.isEmpty() || r.node == s.node)

    /** answering is the first of [results], in the order probed, the LAN address first, that is [s] answering, or null. */
    fun answering(s: Server, results: List<ProbeResult>): String? = results.firstOrNull { good(s, it) }?.url

    /**
     * pick asks [s]'s two addresses at once with [probe] and takes the LAN
     * one where it answers, without waiting for the remote one; the remote
     * one where the LAN one did not, or has not within [graceMs] of the
     * remote one's answer, so that a phone away from home does not wait for
     * the LAN address's timeout. Null where neither answers.
     */
    fun pick(s: Server, probe: (String) -> ProbeResult, graceMs: Long = GRACE_MS): String? {
        val urls = listOf(s.lan, s.remote).filter { it.isNotEmpty() }
        if (urls.isEmpty()) return null
        val pool = Executors.newFixedThreadPool(urls.size)
        try {
            val done = ExecutorCompletionService<ProbeResult>(pool)
            val asked = urls.associateWith { u ->
                done.submit(Callable { runCatching { probe(u) }.getOrElse { ProbeResult(u, ok = false, error = it.message) } })
            }
            val lan = asked[s.lan]
            val remote = asked[s.remote]
            val first = done.take().get()
            if (lan == null || remote == null) return first.url.takeIf { good(s, first) }
            if (first.url == s.lan) {
                if (good(s, first)) return s.lan
                return s.remote.takeIf { good(s, remote.get()) }
            }
            if (!good(s, first)) return s.lan.takeIf { good(s, lan.get()) }
            val late = runCatching { lan.get(graceMs, TimeUnit.MILLISECONDS) }.getOrNull()
            return if (good(s, late)) s.lan else s.remote
        } finally {
            pool.shutdownNow()
        }
    }
}

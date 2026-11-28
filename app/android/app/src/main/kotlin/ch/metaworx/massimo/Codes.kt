// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

/**
 * Codes posts the revoke codes of the tokens the app forgot (AP MobileApp,
 * D112-A; docs/app.md, "The revoke codes") at each contact with their
 * server: the connect page's `session` before its check, `open` at the
 * address it opens, and a removed server's ([gone]) where the probe finds
 * it at its node, at the app's start and each time the list is shown. A
 * code taken (`204`) marks its entry taken; a server that takes no codes
 * drops it, the entry staying as before codes; a malformed one goes,
 * logged; a broken connection, 429 and 5xx keep it, the rest of the
 * server's codes waiting with it. [log] never gets a code.
 */
class Codes(
    private val servers: ServerStore,
    private val gone: GoneStore,
    private val api: ServerApi,
    private val probe: (List<String>, String?) -> List<ProbeResult>,
    private val log: Logger,
) {
    /**
     * post posts the codes waiting for [s] at [url], an address of the
     * server's that answered with its node, and reports whether one was
     * taken or dropped, so that the list is read again; nothing at another
     * address.
     */
    fun post(s: Server, url: String): Boolean {
        if (!s.has(url)) return false
        val now = servers.get(s.id) ?: return false
        var changed = false
        for (code in now.codes()) {
            when (api.revokeCode(url, now.fp, code)) {
                ServerApi.Taken.TAKEN -> changed = servers.took(s.id, code) || changed
                ServerApi.Taken.NONE -> changed = servers.dropCode(s.id, code) || changed
                ServerApi.Taken.MALFORMED -> {
                    log.warn("revoke code: ${s.id} called a code malformed at ${where(url)}; dropped")
                    changed = servers.dropCode(s.id, code) || changed
                }
                ServerApi.Taken.LATER -> {
                    log.warn("revoke code: ${s.id} did not take its codes at ${where(url)}; they wait for the next contact")
                    return changed
                }
            }
        }
        return changed
    }

    /** Removing is what Remove's post came to: the codes the server took, and those still to post. */
    data class Removing(val taken: Set<String>, val waiting: List<GoneCode>)

    /**
     * removing posts the codes of [s], removed now, at [url], an address of
     * its that answered or "" for none, and returns those it took and those
     * still to post, for [GoneStore.add]: Remove's.
     */
    fun removing(s: Server, url: String): Removing {
        val left = s.unrevoked.filter { !it.taken && RevokeCode.valid(it.code) }.map { GoneCode(it.code, false) } +
            s.unended.filter { !it.taken && RevokeCode.valid(it.code) }.map { GoneCode(it.code, true, it.leftAt) }
        if (left.isEmpty() || !s.has(url)) return Removing(emptySet(), left)
        val taken = mutableSetOf<String>()
        val rest = mutableListOf<GoneCode>()
        var later = false
        for (c in left) {
            if (later) {
                rest.add(c)
                continue
            }
            when (api.revokeCode(url, s.fp, c.code)) {
                ServerApi.Taken.TAKEN -> taken.add(c.code)
                ServerApi.Taken.NONE -> Unit
                ServerApi.Taken.MALFORMED -> log.warn("revoke code: ${s.id} called a code malformed at ${where(url)}; dropped")
                ServerApi.Taken.LATER -> {
                    log.warn("revoke code: ${s.id} did not take its codes at ${where(url)}; kept for when it is found again")
                    later = true
                    rest.add(c)
                }
            }
        }
        return Removing(taken, rest)
    }

    /**
     * postGone probes each removed server's addresses and posts its codes
     * at the one that answers with its node, the LAN one first
     * ([Addresses.answering]), so that an address given to another host
     * spends no code. One run at a time; a run asked for meanwhile is
     * skipped.
     */
    fun postGone() {
        synchronized(this) {
            if (running) return
            running = true
        }
        try {
            for (g in gone.list()) {
                val s = g.server()
                val urls = listOf(s.lan, s.remote).filter { it.isNotEmpty() }
                val url = Addresses.answering(s, runCatching { probe(urls, s.fp.ifEmpty { null }) }.getOrDefault(emptyList())) ?: continue
                for (c in g.codes) {
                    when (api.revokeCode(url, s.fp, c.code)) {
                        ServerApi.Taken.TAKEN, ServerApi.Taken.NONE -> gone.drop(g.id, c.code)
                        ServerApi.Taken.MALFORMED -> {
                            log.warn("revoke code: the removed ${g.id} called a code malformed at ${where(url)}; dropped")
                            gone.drop(g.id, c.code)
                        }
                        ServerApi.Taken.LATER -> break
                    }
                }
            }
        } finally {
            synchronized(this) { running = false }
        }
    }

    private var running = false

    private companion object {
        fun where(url: String): String = Origins.of(url) ?: "?"
    }
}

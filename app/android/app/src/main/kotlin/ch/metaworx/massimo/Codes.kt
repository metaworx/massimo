// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

/**
 * Codes posts the revoke codes of the tokens the app forgot (AP MobileApp,
 * D112-A; docs/app.md, "The revoke codes") at each contact with their
 * server: the connect page's `session` before its check, `open` at the
 * address it opens, and a removed server's ([gone]) where the probe finds
 * it at its node, at its own addresses or at those of a server of the list
 * with its node (AP MobileApp, D117-A), at the app's start and each time
 * the list is shown; a server added or saved with a removed one's node
 * takes that one's codes back into its entry ([adopt]). A
 * code taken (`204`) marks its entry taken; a server that takes no codes
 * drops it, the entry staying as before codes; a malformed one goes,
 * logged; a broken connection, 429 and 5xx keep it, the rest of the
 * server's codes waiting with it. [log] never gets a code. Once
 * [stopped] (the list's Exit, AP MobileApp D91-A), nothing more is posted.
 * [revoke] posts one code at an address with the pin to hold the
 * certificate to, [api]'s by default; a test records the pin there.
 */
class Codes(
    private val servers: ServerStore,
    private val gone: GoneStore,
    private val api: ServerApi,
    private val probe: (List<String>, String?) -> List<ProbeResult>,
    private val log: Logger,
    private val stopped: () -> Boolean = { false },
    private val revoke: (url: String, fp: String, code: String) -> ServerApi.Taken = api::revokeCode,
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
            if (stopped()) return changed // the list's Exit meanwhile: nothing more posted
            when (revoke(url, now.fp, code)) {
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
        // each with its token, so that a server added again with the node takes it back (D117-A)
        val left = s.unrevoked.filter { !it.taken && RevokeCode.valid(it.code) }.map { GoneCode(it.code, false, user = it.user, id = it.id) } +
            s.unended.filter { !it.taken && RevokeCode.valid(it.code) }.map { GoneCode(it.code, true, it.leftAt, it.user, it.id, it.createdAt) }
        if (left.isEmpty() || !s.has(url)) return Removing(emptySet(), left)
        val taken = mutableSetOf<String>()
        val rest = mutableListOf<GoneCode>()
        var later = false
        for (c in left) {
            if (later) {
                rest.add(c)
                continue
            }
            when (revoke(url, s.fp, c.code)) {
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
     * spends no code; where none does and it has a node, the addresses of
     * each server of the list with that node, each with its own pin, and
     * posts them at the first that answers with the node (D117-A). A
     * removed server without a node is found at its own addresses alone.
     * One run at a time; a run asked for meanwhile is skipped.
     */
    fun postGone() {
        synchronized(this) {
            if (running) return
            running = true
        }
        try {
            for (g in gone.list()) {
                val (url, fp) = found(g) ?: continue
                for (c in g.codes) {
                    if (stopped()) return // the list's Exit meanwhile: nothing more posted
                    when (revoke(url, fp, c.code)) {
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

    // found is where the removed server [g] answers with its node, and the
    // pin the codes are posted with there: its own addresses with its pin,
    // else, where it has a node, a server of the list with that node at its
    // addresses with its own pin (D117-A); null where none answers.
    private fun found(g: GoneServer): Pair<String, String>? {
        val own = g.server()
        answering(own)?.let { return it to own.fp }
        if (g.node.isEmpty()) return null
        for (s in servers.list()) {
            if (s.node != g.node || (s.lan == own.lan && s.remote == own.remote && s.fp == own.fp)) continue
            answering(s)?.let { return it to s.fp }
        }
        return null
    }

    // answering probes [s]'s addresses with its pin and is the first that
    // answers with its node, the LAN one first, or null.
    private fun answering(s: Server): String? {
        val urls = listOf(s.lan, s.remote).filter { it.isNotEmpty() }
        if (urls.isEmpty()) return null
        return Addresses.answering(s, runCatching { probe(urls, s.fp.ifEmpty { null }) }.getOrDefault(emptyList()))
    }

    /**
     * adopt takes the codes of each removed server of [s]'s node back into
     * [s], a server of the list just added or saved (D117-A): as the keys
     * and sessions left there, with their codes, which the next contact
     * posts, a session's left when it was, or at the Remove where that is
     * not known; each removed server then drops them, and goes once it has
     * none left. A code without its token stays with the removed server,
     * posted where [postGone] finds the node. Nothing for a server without
     * a node, nor one not kept. [s]'s entry is written before the removed
     * server drops a code, so that a write that fails loses none. It
     * reports whether a code was taken back.
     */
    fun adopt(s: Server): Boolean {
        if (s.node.isEmpty()) return false
        var changed = false
        for (g in gone.ofNode(s.node)) {
            val back = g.codes.filter { it.takeable() }.map { if (it.session && it.leftAt.isEmpty()) it.copy(leftAt = g.removedAt) else it }
            if (back.isEmpty()) continue
            if (!servers.takeBack(s.id, back)) return changed
            gone.dropAll(g.id, back.map { it.code }.toSet())
            changed = true
        }
        return changed
    }

    private var running = false

    private companion object {
        fun where(url: String): String = Origins.of(url) ?: "?"
    }
}

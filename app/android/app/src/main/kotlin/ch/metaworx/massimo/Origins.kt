// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import java.net.URI
import java.net.URISyntaxException

/** Who sent a message over the bridge (docs/app.md, "Where the bridge exists"). */
enum class Caller { CONNECT, SERVER, NONE }

/** Origins of the pages the shell shows, and what each may ask of the bridge. */
object Origins {
    /** The shell's own origin, WebViewAssetLoader's: the connect page's. */
    const val APP = "https://appassets.androidplatform.net"
    const val APP_HOST = "appassets.androidplatform.net"
    const val CONNECT_URL = "$APP/ui/connect.html"

    private val connectCalls = setOf("hello", "servers", "save", "remove", "discover", "probe", "trust", "open")
    private val serverCalls = setOf("hello", "token", "setToken")

    /**
     * of is the origin of an http or https URL, `scheme://host[:port]` with
     * the scheme's own port left out and an IPv6 address in brackets, as the
     * WebView writes one; null for any other URL.
     */
    fun of(url: String): String? {
        val u = try {
            URI(url.trim())
        } catch (e: URISyntaxException) {
            return null
        }
        val scheme = u.scheme?.lowercase() ?: return null
        if (scheme != "http" && scheme != "https") return null
        val host = u.host?.lowercase()
        if (host.isNullOrEmpty()) return null
        val port = u.port
        val usual = if (scheme == "https") 443 else 80
        return if (port == -1 || port == usual) "$scheme://$host" else "$scheme://$host:$port"
    }

    /**
     * caller is who a message from [origin] is: the connect page from the
     * shell's own origin, the opened server's page from [opened]'s; a
     * frame inside a page is nobody.
     */
    fun caller(origin: String, isMainFrame: Boolean, opened: String?): Caller {
        if (!isMainFrame) return Caller.NONE
        val o = of(origin) ?: return Caller.NONE
        return when (o) {
            APP -> Caller.CONNECT
            opened -> Caller.SERVER
            else -> Caller.NONE
        }
    }

    /** allowed reports whether [caller] may make [call]. */
    fun allowed(caller: Caller, call: String): Boolean = when (caller) {
        Caller.CONNECT -> call in connectCalls
        Caller.SERVER -> call in serverCalls
        Caller.NONE -> false
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.Handler
import android.os.Looper
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.util.concurrent.Executor

/**
 * Bridge is the native side of `window.massimo` (docs/app.md): it takes the
 * messages of the page's script (res/raw/bridge.js), checks who sent each
 * by its origin, and answers it. The connect page gets the calls of the
 * server list, a server's page only its own token.
 */
class Bridge(
    private val host: Host,
    private val servers: ServerStore,
    private val tokens: TokenStore,
    private val prober: Prober,
    private val discovery: Discovery,
    private val biometric: Biometric,
    private val texts: Texts,
    private val io: Executor,
    private val main: Handler,
) {
    /** Host is what the bridge needs of the activity. */
    fun interface Host {
        /** open loads [url], an address of the server, in the WebView. */
        fun open(url: String)
    }

    /** Reply posts a message back to the frame that sent one. */
    fun interface Reply {
        fun post(message: String)
    }

    /** Opened is the server `open` loaded: its id, and the origin its page has. */
    data class Opened(val id: String, val origin: String)

    @Volatile
    var opened: Opened? = null

    // The connect page's channel, for the events.
    private var connect: Reply? = null

    /** server is the server kept under [id]. */
    fun server(id: String): Server? = servers.get(id)

    /** onMessage handles one message of the page's script, on the main thread. */
    fun onMessage(data: String, origin: String, isMainFrame: Boolean, reply: Reply) {
        val msg = try {
            JSONObject(data)
        } catch (e: JSONException) {
            return
        }
        val id = msg.opt("id") as? Number ?: return
        val call = ServerStore.str(msg, "call")
        val args = msg.optJSONArray("args") ?: JSONArray()
        val answer = Answer(id.toLong(), reply)
        val now = opened
        val caller = Origins.caller(origin, isMainFrame, now?.origin)
        if (!Origins.allowed(caller, call)) {
            answer.fail(BridgeException(R.string.err_not_allowed))
            return
        }
        if (caller == Caller.CONNECT) connect = reply
        try {
            dispatch(call, args, answer, now)
        } catch (e: Exception) {
            answer.fail(e)
        }
    }

    /** event sends an event to the connect page (docs/app.md, "The events"). */
    fun event(detail: JSONObject) {
        val c = connect ?: return
        val s = JSONObject().put("event", detail).toString()
        if (Looper.myLooper() == main.looper) c.post(s) else main.post { c.post(s) }
    }

    private fun dispatch(call: String, args: JSONArray, answer: Answer, now: Opened?) {
        when (call) {
            "hello" -> answer.ok(null)
            "servers" -> background(answer) { JSONArray().apply { servers.list().forEach { put(it.toJson()) } } }
            "save" -> save(args.optJSONObject(0) ?: throw bad(), answer)
            "remove" -> {
                val id = string(args, 0)
                background(answer) {
                    servers.remove(id)
                    tokens.remove(id)
                    null
                }
            }
            "discover" -> {
                val ms = (args.opt(0) as? Number)?.toLong() ?: DISCOVER_MS
                discovery.search(ms.coerceIn(MIN_DISCOVER_MS, MAX_DISCOVER_MS), ::event) { answer.ok(null) }
            }
            "probe" -> {
                val urls = strings(args.optJSONArray(0) ?: throw bad())
                if (urls.size > MAX_URLS) throw bad()
                val fp = (args.opt(1) as? String)?.ifEmpty { null }
                background(answer) { JSONArray().apply { prober.probe(urls, fp).forEach { put(it.toJson()) } } }
            }
            "trust" -> {
                val id = string(args, 0)
                val fp = string(args, 1)
                background(answer) {
                    servers.trust(id, fp)
                    null
                }
            }
            "open" -> {
                val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
                val url = string(args, 1)
                if (!s.has(url)) throw BridgeException(R.string.err_open_url)
                val origin = Origins.of(url) ?: throw BridgeException(R.string.err_open_url)
                answer.ok(null)
                opened = Opened(s.id, origin)
                host.open(url)
            }
            "token" -> token(now ?: throw bad(), answer)
            "setToken" -> {
                val id = (now ?: throw bad()).id
                val token = when (val t = args.opt(0)) {
                    is String -> t.ifEmpty { null }
                    null, JSONObject.NULL -> null
                    else -> throw bad()
                }
                if (token != null && token.length > MAX_TOKEN) throw bad()
                background(answer) {
                    tokens.put(id, token)
                    null
                }
            }
            else -> throw bad()
        }
    }

    // save keeps a server; turning its fingerprint or PIN off takes the
    // fingerprint or PIN, and turning it on takes a phone that has one.
    private fun save(input: JSONObject, answer: Answer) {
        val id = ServerStore.str(input, "id")
        val old = if (id.isEmpty()) null else servers.get(id)
        val want = input.opt("biometric") == true
        val keep = { background(answer) { servers.save(input).toJson() } }
        when {
            old != null && old.biometric && !want -> biometric.ask(texts.get(R.string.unlock_title, old.name)) { ok ->
                if (ok) keep() else answer.fail(BridgeException(R.string.err_not_confirmed))
            }
            want && old?.biometric != true && !biometric.available() -> throw BridgeException(R.string.err_screen_lock)
            else -> keep()
        }
    }

    // token hands the opened server's token to its page, after the
    // fingerprint or PIN where the server asks for it; null where there is
    // none or the prompt was dismissed.
    private fun token(now: Opened, answer: Answer) {
        val s = servers.get(now.id)
        if (s == null) {
            answer.ok(null)
            return
        }
        if (!s.biometric) {
            background(answer) { tokens.get(s.id) }
            return
        }
        biometric.ask(texts.get(R.string.unlock_title, s.name)) { ok ->
            if (ok) background(answer) { tokens.get(s.id) } else answer.ok(null)
        }
    }

    private fun background(answer: Answer, work: () -> Any?) {
        io.execute {
            try {
                answer.ok(work())
            } catch (e: Exception) {
                answer.fail(e)
            }
        }
    }

    private fun message(e: Throwable): String = when (e) {
        is BridgeException -> e.message(texts)
        else -> texts.get(R.string.err_internal, e.message ?: e.javaClass.simpleName)
    }

    private inner class Answer(private val id: Long, private val reply: Reply) {
        fun ok(value: Any?) = post(JSONObject().put("id", id).put("result", value ?: JSONObject.NULL))

        fun fail(e: Throwable) = post(JSONObject().put("id", id).put("error", message(e)))

        private fun post(o: JSONObject) {
            val s = o.toString()
            if (Looper.myLooper() == main.looper) reply.post(s) else main.post { reply.post(s) }
        }
    }

    private companion object {
        const val DISCOVER_MS = 5000L
        const val MIN_DISCOVER_MS = 500L
        const val MAX_DISCOVER_MS = 60_000L
        const val MAX_URLS = 16
        const val MAX_TOKEN = 4096

        fun bad() = BridgeException(R.string.err_bad_call)

        fun string(args: JSONArray, i: Int): String = args.opt(i) as? String ?: throw bad()

        fun strings(a: JSONArray): List<String> = List(a.length()) { i -> a.opt(i) as? String ?: throw bad() }
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.Handler
import android.os.Looper
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException
import java.security.GeneralSecurityException
import java.util.concurrent.Executor
import javax.crypto.Cipher

/**
 * Bridge is the native side of `window.massimo` (docs/app.md, contract
 * version 2): it takes the messages of the page's script (res/raw/bridge.js),
 * checks who sent each by its origin, and answers it. The connect page gets
 * the calls of the server list, a server's page only its own session, its
 * login by fingerprint and the way back to the list. The login's key never
 * reaches a page: the bridge makes and holds it and hands a page the session
 * alone.
 */
class Bridge(
    private val host: Host,
    private val servers: ServerStore,
    private val sessions: SessionKeeper,
    private val logins: LoginStore,
    private val keys: KeySource,
    private val api: ServerApi,
    private val prober: Prober,
    private val discovery: Discovery,
    private val biometric: Biometric,
    private val texts: Texts,
    private val keyLabel: String,
    private val io: Executor,
    private val main: Handler,
) {
    /** Host is what the bridge needs of the activity. */
    interface Host {
        /** open loads [url], an address of the server, in the WebView. */
        fun open(url: String)

        /** connectPage shows the connect page again. */
        fun connectPage()

        /** privacy is told that what keeps the page from Recents may have changed (guarded). */
        fun privacy()
    }

    /** Reply posts a message back to the frame that sent one. */
    fun interface Reply {
        fun post(message: String)
    }

    /** Opened is the server `open` loaded: its id, the origin its page has, and the address it was opened at. */
    data class Opened(val id: String, val origin: String, val url: String)

    @Volatile
    var opened: Opened? = null

    // The connect page's channel, for the events.
    private var connect: Reply? = null

    /** server is the server kept under [id]. */
    fun server(id: String): Server? = servers.get(id)

    /**
     * guarded reports whether the page shown, of [origin], is that of a
     * server with a kept login asked at each return: the app covers it while
     * it is hidden, and Recents keeps no picture of it.
     */
    fun guarded(origin: String?): Boolean {
        val now = opened ?: return false
        if (origin == null || origin != now.origin) return false
        val s = servers.get(now.id) ?: return false
        return s.ask == Server.ASK_RETURN && logins.get(s.id) != null
    }

    /** covers reports whether the page of [origin] is guarded and signed in, with a session the cover keeps from others. */
    fun covers(origin: String?): Boolean = guarded(origin) && opened?.let { sessions.inMemory(it.id) } == true

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
            "servers" -> background(answer) { JSONArray().apply { servers.list().forEach { put(it.toJson(logins.user(it.id))) } } }
            "save" -> save(args.optJSONObject(0) ?: throw bad(), answer)
            "remove" -> {
                val id = string(args, 0)
                background(answer) {
                    servers.remove(id)
                    sessions.forget(id)
                    forgetLogin(id)
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
                    val old = servers.get(id)
                    servers.trust(id, fp)
                    // A login kept for the old certificate opens for nothing else.
                    if (old != null && !Fingerprint.same(old.fp, fp) && logins.get(id) != null) {
                        forgetLogin(id)
                        sessions.move(id, false)
                    }
                    null
                }
            }
            "open" -> open(args, answer)
            "forget" -> forget(string(args, 0), args.opt(1) as? String ?: "", answer)
            "forgetAll" -> background(answer) {
                for (id in logins.ids()) {
                    forgetLogin(id)
                    sessions.move(id, false)
                }
                main.post { host.privacy() }
                null
            }
            "token" -> {
                val id = (now ?: throw bad()).id
                background(answer) { servers.get(id)?.let { sessions.get(it.id, inMemory(it)) } }
            }
            "setToken" -> {
                val id = (now ?: throw bad()).id
                val token = when (val t = args.opt(0)) {
                    is String -> t.ifEmpty { null }
                    null, JSONObject.NULL -> null
                    else -> throw bad()
                }
                if (token != null && token.length > MAX_TOKEN) throw bad()
                background(answer) {
                    servers.get(id)?.let { sessions.put(it.id, token, inMemory(it)) }
                    null
                }
            }
            "login.status" -> {
                val o = now ?: throw bad()
                val can = biometric.canStrong()
                background(answer) {
                    val s = servers.get(o.id) ?: throw BridgeException(R.string.err_no_server)
                    JSONObject().put("can", can).put("user", logins.user(s.id)).put("ask", s.ask).put("plain", Origins.plainOutside(o.url))
                }
            }
            "login.keep" -> keep(now ?: throw bad(), answer)
            "login.fingerprint" -> {
                val o = now ?: throw bad()
                val s = servers.get(o.id) ?: throw BridgeException(R.string.err_no_server)
                fingerprint(s, o.url, answer) { session -> answer.ok(session) }
            }
            "connect" -> {
                answer.ok(null)
                host.connectPage()
            }
            else -> throw bad()
        }
    }

    // inMemory reports whether the server's session is kept in memory alone.
    private fun inMemory(s: Server): Boolean = SessionKeeper.memoryFor(s.ask, logins.get(s.id) != null)

    // save keeps a server; its session moves where its new choice of when
    // the fingerprint is asked keeps it, and a node that changed forgets
    // its login, which opens for nothing else.
    private fun save(input: JSONObject, answer: Answer) {
        background(answer) {
            val id = ServerStore.str(input, "id")
            val old = if (id.isEmpty()) null else servers.get(id)
            val s = servers.save(input)
            if (old != null && logins.get(s.id) != null && old.node != s.node) forgetLogin(s.id)
            if (old != null) sessions.move(s.id, inMemory(s))
            main.post { host.privacy() }
            s.toJson(logins.user(s.id))
        }
    }

    // open loads a server's address; with {login: true} it logs in by
    // fingerprint first, and resolves false without opening where the
    // prompt was dismissed.
    private fun open(args: JSONArray, answer: Answer) {
        val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
        val url = string(args, 1)
        if (!s.has(url)) throw BridgeException(R.string.err_open_url)
        val origin = Origins.of(url) ?: throw BridgeException(R.string.err_open_url)
        val go = {
            answer.ok(true)
            opened = Opened(s.id, origin, url)
            host.open(url)
        }
        if (args.optJSONObject(2)?.opt("login") != true) {
            go()
            return
        }
        fingerprint(s, url, answer) { session -> if (session == null) answer.ok(false) else go() }
    }

    // fingerprint asks for the fingerprint, opens the server's kept key,
    // makes a session with it at url and keeps it as the server's choice
    // says; then gets the session, on the main thread, or null where the
    // prompt was dismissed. A key the server refuses, or that opens no
    // more, is forgotten and fails the answer.
    private fun fingerprint(s: Server, url: String, answer: Answer, then: (String?) -> Unit) {
        if (Origins.plainOutside(url)) throw BridgeException(R.string.err_plain)
        val kept = logins.get(s.id) ?: throw BridgeException(R.string.err_no_login)
        if (!matches(s, kept)) {
            forgetLogin(s.id)
            throw BridgeException(R.string.err_login_changed)
        }
        val cipher = try {
            keys.decrypter(LoginKey.alias(s.id), kept.iv)
        } catch (e: KeyInvalidated) {
            forgetLogin(s.id)
            throw BridgeException(R.string.err_key_invalidated)
        }
        biometric.withKey(texts.get(R.string.login_title, s.name), cipher) { c ->
            if (c == null) {
                then(null)
                return@withKey
            }
            io.execute {
                try {
                    val secret = LoginStore.open(c, kept, s.id)
                    val session = api.loginKey(url, s.fp, secret.key)
                    sessions.put(s.id, session, SessionKeeper.memoryFor(s.ask, true))
                    main.post { then(session) }
                } catch (e: ServerApi.Refused) {
                    forgetLogin(s.id)
                    answer.fail(BridgeException(R.string.err_key_refused))
                } catch (e: GeneralSecurityException) {
                    forgetLogin(s.id)
                    answer.fail(BridgeException(R.string.err_key_invalidated))
                } catch (e: JSONException) {
                    forgetLogin(s.id)
                    answer.fail(BridgeException(R.string.err_key_invalidated))
                } catch (e: IOException) {
                    answer.fail(BridgeException(R.string.err_server, reason(e)))
                }
            }
        }
    }

    // keep keeps the login by fingerprint for the page just signed in: the
    // user's name from its session, a key of the user's made with it, then
    // the prompt, which seals the key; a prompt not passed revokes the key
    // again and resolves false.
    private fun keep(now: Opened, answer: Answer) {
        val s = servers.get(now.id) ?: throw BridgeException(R.string.err_no_server)
        if (Origins.plainOutside(now.url)) throw BridgeException(R.string.err_plain)
        if (!biometric.canStrong()) throw BridgeException(R.string.err_no_biometric)
        val session = sessions.get(s.id, inMemory(s)) ?: throw BridgeException(R.string.err_no_session)
        val alias = LoginKey.alias(s.id)
        io.execute {
            val user: String
            val key: ServerApi.Key
            try {
                user = api.me(now.url, s.fp, session)
                key = api.createKey(now.url, s.fp, session, user, takeBytes(keyLabel, MAX_LABEL))
            } catch (e: ServerApi.Refused) {
                answer.fail(BridgeException(R.string.err_no_session))
                return@execute
            } catch (e: IOException) {
                answer.fail(BridgeException(R.string.err_server, reason(e)))
                return@execute
            }
            val undo = { revokeQuietly(now.url, s.fp, session, user, key.id) }
            main.post {
                val cipher: Cipher = try {
                    keys.encrypter(alias)
                } catch (e: GeneralSecurityException) {
                    io.execute(undo)
                    answer.fail(BridgeException(R.string.err_storage, reason(e)))
                    return@post
                }
                biometric.withKey(texts.get(R.string.login_keep_title, s.name), cipher) { c ->
                    io.execute {
                        if (c == null) {
                            undo()
                            runCatching { keys.delete(alias) }
                            answer.ok(false)
                            return@execute
                        }
                        try {
                            logins.put(s.id, LoginStore.seal(c, LoginSecret(key.token, key.id), s.id, s.node, s.fp, user))
                            sessions.move(s.id, SessionKeeper.memoryFor(s.ask, true))
                            main.post { host.privacy() }
                            answer.ok(true)
                        } catch (e: Exception) {
                            undo()
                            runCatching { keys.delete(alias) }
                            answer.fail(BridgeException(R.string.err_storage, reason(e)))
                        }
                    }
                }
            }
        }
    }

    // forget turns a server's login off: the prompt once, the key revoked
    // at url, and the login forgotten, the prompt or the server failing or
    // not; the answer says whether the server revoked it.
    private fun forget(id: String, url: String, answer: Answer) {
        val s = servers.get(id)
        val kept = logins.get(id)
        val done = { revoked: Boolean ->
            io.execute {
                forgetLogin(id)
                // A revoked key took its session with it; else it lives on, sealed.
                if (revoked) sessions.forget(id) else sessions.move(id, false)
                main.post { host.privacy() }
                answer.ok(JSONObject().put("revoked", revoked))
            }
        }
        if (s == null || kept == null || url.isEmpty() || !s.has(url) || Origins.plainOutside(url) || !matches(s, kept)) {
            done(false)
            return
        }
        val cipher = try {
            keys.decrypter(LoginKey.alias(id), kept.iv)
        } catch (e: KeyInvalidated) {
            done(false)
            return
        }
        biometric.withKey(texts.get(R.string.login_off_title, s.name), cipher) { c ->
            if (c == null) {
                done(false)
                return@withKey
            }
            io.execute {
                val revoked = try {
                    val secret = LoginStore.open(c, kept, id)
                    api.revoke(url, s.fp, secret.key, kept.user, secret.id)
                    true
                } catch (e: Exception) {
                    false
                }
                done(revoked)
            }
        }
    }

    // matches reports whether a kept login is still the server's: its node
    // and its pinned fingerprint as they were when it was kept.
    private fun matches(s: Server, kept: KeptLogin): Boolean =
        kept.node == s.node && (Fingerprint.same(kept.fp, s.fp) || (kept.fp.isEmpty() && s.fp.isEmpty()))

    // forgetLogin forgets the server's kept login and its Keystore key.
    private fun forgetLogin(id: String) {
        logins.forget(id)
        runCatching { keys.delete(LoginKey.alias(id)) }
    }

    private fun revokeQuietly(url: String, fp: String, bearer: String, user: String, id: Long) {
        runCatching { api.revoke(url, fp, bearer, user, id) }
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
        const val MAX_LABEL = 64

        fun bad() = BridgeException(R.string.err_bad_call)

        fun reason(e: Throwable): String = e.message ?: e.javaClass.simpleName

        // takeBytes is s cut to at most max bytes of UTF-8, between characters: the server's limit of a label.
        fun takeBytes(s: String, max: Int): String {
            var n = 0
            val b = StringBuilder()
            for (ch in s) {
                n += ch.toString().toByteArray(Charsets.UTF_8).size
                if (n > max) break
                b.append(ch)
            }
            return b.toString()
        }

        fun string(args: JSONArray, i: Int): String = args.opt(i) as? String ?: throw bad()

        fun strings(a: JSONArray): List<String> = List(a.length()) { i -> a.opt(i) as? String ?: throw bad() }
    }
}

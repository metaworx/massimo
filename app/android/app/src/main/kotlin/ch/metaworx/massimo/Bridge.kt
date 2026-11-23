// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.os.Handler
import android.os.Looper
import org.json.JSONArray
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException
import java.util.concurrent.Executor
import java.util.concurrent.RejectedExecutionException

/**
 * Bridge is the native side of `window.massimo` (docs/app.md, contract
 * version 4): it takes the messages of the page's script (res/raw/bridge.js),
 * checks who sent each by its origin, and answers it. The connect page gets
 * the calls of the server list, the sessions the app holds and the app's
 * settings; a server's page only its own session, its login by fingerprint
 * and the way back to the list. The login's key never reaches a page: the
 * bridge makes and holds it and hands a page the session alone, each login
 * sealed under the one app key ([AppKey]), which a prompt opens for one
 * step. Turn off and Forget all are the [Revoker]'s, and the keys they leave
 * valid are [left]'s.
 */
class Bridge(
    private val host: Host,
    private val servers: ServerStore,
    private val keeper: SessionKeeper,
    private val sessions: Sessions,
    private val left: KeysLeft,
    private val logins: LoginStore,
    private val keys: KeySource,
    private val api: ServerApi,
    private val prober: Prober,
    private val discovery: Discovery,
    private val biometric: Biometric,
    private val settings: AppSettings,
    private val texts: Texts,
    private val log: Logger,
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

        /** appearance gives the app the appearance saved, one of Settings.APPEARANCES. */
        fun appearance(appearance: String)
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

    private val revoker = Revoker(servers, logins, sessions, left, prober::probe, io, ::forgetLogin)

    // The prompt of the app key's steps, on the main thread, its answer on
    // io; where io takes nothing more, the activity having gone, the app
    // key every step holds is dropped, and the answer with it.
    private val prompt = AppKey.Prompt { title, cipher, done ->
        main.post { biometric.withKey(title, cipher) { c -> onIo { done(c) } } }
    }

    // The app key (AP MobileApp, D63-A); a key the phone took back forgets
    // every login under it, each key left by its id (D57-A), its session
    // sealed as a server's without a kept login is.
    private val appKey = AppKey(logins, keys, prompt, { id, kept ->
        val s = servers.get(id)
        if (s != null) forgetLeavingKey(s, kept) else forgetLogin(id)
        keeper.move(id, false)
    })

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
    fun covers(origin: String?): Boolean = guarded(origin) && opened?.let { keeper.inMemory(it.id) } == true

    /** close drops the app key a step still holds, as the activity goes (onDestroy). */
    fun close() = appKey.dropAll()

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
            "servers" -> background(answer) { JSONArray().apply { servers.list().forEach { put(json(it)) } } }
            "save" -> save(args.optJSONObject(0) ?: throw bad(), answer)
            "remove" -> remove(string(args, 0), args.opt(1) as? String ?: "", answer)
            "session" -> {
                val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
                val url = string(args, 1)
                background(answer) { sessions.check(s, url) }
            }
            "signOut" -> {
                val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
                val url = args.opt(1) as? String ?: ""
                background(answer) { JSONObject().put("ended", sessions.signOut(s, url)) }
            }
            "signOutAll" -> background(answer) {
                sessions.signOutAll(servers.list())
                null
            }
            "settings" -> background(answer) { settings.get().toJson() }
            "saveSettings" -> {
                val input = args.optJSONObject(0) ?: throw bad()
                io.execute {
                    try {
                        val saved = settings.save(input)
                        answer.ok(saved.toJson())
                        // after the answer: a new appearance loads the page again
                        main.post { host.appearance(saved.appearance) }
                    } catch (e: Exception) {
                        answer.fail(e)
                    }
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
                    // A login kept for the old certificate opens for nothing
                    // else; its key, the same server's, is left to revoke.
                    val kept = logins.get(id)
                    if (old != null && !Fingerprint.same(old.fp, fp) && kept != null) {
                        forgetLeavingKey(old, kept)
                        keeper.move(id, false)
                    }
                    null
                }
            }
            "open" -> open(args, answer)
            "forget" -> forget(string(args, 0), args.opt(1) as? String ?: "", answer)
            "forgetAll" -> forgetAll(answer)
            "token" -> {
                val id = (now ?: throw bad()).id
                background(answer) { servers.get(id)?.let { sessions.held(it) } }
            }
            "setToken" -> {
                val o = now ?: throw bad()
                val id = o.id
                val token = when (val t = args.opt(0)) {
                    is String -> t.ifEmpty { null }
                    null, JSONObject.NULL -> null
                    else -> throw bad()
                }
                if (token != null && token.length > MAX_TOKEN) throw bad()
                // null forgets it: Sign out, Forget key, and the dead session
                // the page drops at the server's first 401. A new session
                // revokes the keys left there for its user, and gives a login
                // kept without its key's id that id, once the page has its
                // answer (Sessions.check).
                io.execute {
                    try {
                        servers.get(id)?.let { sessions.put(it, token) }
                        answer.ok(null)
                    } catch (e: Exception) {
                        answer.fail(e)
                        return@execute
                    }
                    val s = servers.get(id)
                    if (token != null && s != null && (s.unrevoked.isNotEmpty() || logins.get(id)?.keyId == 0L)) {
                        runCatching { sessions.check(s, o.url) }
                    }
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

    // json is a server as the connect page sees it: with the user whose
    // login is kept, and the user of the session held, as last checked.
    private fun json(s: Server): JSONObject = s.toJson(logins.user(s.id), sessions.user(s))

    // save keeps a server; its session moves where its new choice of when
    // the fingerprint is asked keeps it, and a node that changed forgets
    // its login, which opens for nothing else.
    private fun save(input: JSONObject, answer: Answer) {
        background(answer) {
            val id = ServerStore.str(input, "id")
            val old = if (id.isEmpty()) null else servers.get(id)
            val s = servers.save(input)
            if (old != null && logins.get(s.id) != null && old.node != s.node) forgetLogin(s.id)
            if (old != null) keeper.move(s.id, sessions.inMemory(s))
            main.post { host.privacy() }
            json(s)
        }
    }

    // open loads a server's address, or My profile's there with a key
    // marked (`#profile?key=<id>`, contract version 4); with {login: true}
    // it logs in by fingerprint first, and resolves false without opening
    // where the prompt was dismissed.
    private fun open(args: JSONArray, answer: Answer) {
        val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
        val url = string(args, 1)
        val base = s.opening(url) ?: throw BridgeException(R.string.err_open_url)
        val origin = Origins.of(base) ?: throw BridgeException(R.string.err_open_url)
        val go = {
            answer.ok(true)
            opened = Opened(s.id, origin, base)
            host.open(url)
        }
        if (args.optJSONObject(2)?.opt("login") != true) {
            go()
            return
        }
        fingerprint(s, base, answer) { session -> if (session == null) answer.ok(false) else go() }
    }

    // fingerprint asks for the fingerprint, opens the server's kept key in
    // a step of its own (under the app key, or with its own for a login
    // kept before it), makes a session with it at url and keeps it as the
    // server's choice says; then gets the session, on the main thread, or
    // null where the prompt was dismissed. A key the server refuses, or
    // that opens no more, is forgotten and fails the answer.
    private fun fingerprint(s: Server, url: String, answer: Answer, then: (String?) -> Unit) {
        if (Origins.plainOutside(url)) throw BridgeException(R.string.err_plain)
        val kept = logins.get(s.id) ?: throw BridgeException(R.string.err_no_login)
        if (!Revoker.matches(s, kept)) {
            forgetLeavingKey(s, kept)
            throw BridgeException(R.string.err_login_changed)
        }
        onIo {
            try {
                openKept(s, url, kept, answer, then)
            } catch (e: Exception) {
                answer.fail(e)
            }
        }
    }

    // openKept opens the kept login of [s] for fingerprint, on io, and
    // makes the session with it.
    private fun openKept(s: Server, url: String, kept: KeptLogin, answer: Answer, then: (String?) -> Unit) {
        appKey.open(texts.get(R.string.login_title, s.name), s.id, kept) { r ->
            when (r) {
                is AppKey.Result.Opened -> try {
                    val session = api.loginKey(url, s.fp, r.secret.key)
                    sessions.put(s, session)
                    main.post { then(session) }
                } catch (e: ServerApi.Refused) {
                    forgetLogin(s.id)
                    answer.fail(BridgeException(R.string.err_key_refused))
                } catch (e: IOException) {
                    answer.fail(BridgeException(R.string.err_server, reason(e)))
                } catch (e: Exception) {
                    answer.fail(e)
                }
                is AppKey.Result.Gone -> {
                    forgetLeavingKey(s, kept)
                    answer.fail(BridgeException(R.string.err_key_invalidated))
                }
                is AppKey.Result.Failed -> answer.fail(BridgeException(R.string.err_storage, reason(r.error)))
                else -> main.post { then(null) }
            }
        }
    }

    // keep keeps the login by fingerprint for the page just signed in: the
    // user's name from its session, a key of the user's made with it, then
    // the prompt, which opens the app key, or makes it the first time, to
    // seal the key under; a prompt not passed revokes the key again and
    // resolves false.
    private fun keep(now: Opened, answer: Answer) {
        val s = servers.get(now.id) ?: throw BridgeException(R.string.err_no_server)
        if (Origins.plainOutside(now.url)) throw BridgeException(R.string.err_plain)
        if (!biometric.canStrong()) throw BridgeException(R.string.err_no_biometric)
        val session = sessions.held(s) ?: throw BridgeException(R.string.err_no_session)
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
            val step = appKey.step()
            try {
                step.keep(texts.get(R.string.login_keep_title, s.name), s.id, s.node, s.fp, user, LoginSecret(key.token, key.id)) { r ->
                    step.close()
                    when (r) {
                        AppKey.Result.Kept -> try {
                            keeper.move(s.id, SessionKeeper.memoryFor(s.ask, true))
                            main.post { host.privacy() }
                            answer.ok(true)
                        } catch (e: Exception) {
                            answer.fail(BridgeException(R.string.err_storage, reason(e)))
                        }
                        is AppKey.Result.Failed -> {
                            undo()
                            answer.fail(BridgeException(R.string.err_storage, reason(r.error)))
                        }
                        else -> {
                            undo()
                            answer.ok(false)
                        }
                    }
                }
            } catch (e: Exception) {
                step.close()
                undo()
                answer.fail(BridgeException(R.string.err_storage, reason(e)))
            }
        }
    }

    // forget turns a server's login off (Turn off, AP MobileApp F6), at
    // url, the address the probe chose: the key revoked with the session the
    // app holds there, without the prompt, else with the key itself after
    // the prompt once; then the session signed out and the login forgotten,
    // the prompt or the server failing or not, a key not revoked left
    // (Revoker); the answer says whether the server revoked the key.
    private fun forget(id: String, url: String, answer: Answer) {
        io.execute {
            val k = byKey()
            try {
                revoker.turnOff(id, url, k) { off ->
                    main.post { host.privacy() }
                    answer.ok(JSONObject().put("revoked", off.outcome == Revoker.Outcome.REVOKED).put("left", off.outcome == Revoker.Outcome.LEFT))
                }
            } catch (e: Exception) {
                k.close()
                answer.fail(e)
            }
        }
    }

    // remove forgets a server: a kept login is turned off first, at url or
    // the address that answers, as forget turns it off (AP MobileApp, Block
    // 14), then the server, its certificate, its session and its login go.
    // The answer says whether the login's key was revoked (revoked), the
    // ids of the phone's keys still valid there (keys: its unrevoked, the
    // login's among them where it could not be revoked), and whether the
    // login's key went without its id being known (local).
    private fun remove(id: String, url: String, answer: Answer) {
        io.execute {
            val drop = { off: Revoker.Off? ->
                try {
                    val keys = servers.get(id)?.unrevoked?.map { it.id } ?: emptyList()
                    servers.remove(id)
                    sessions.forget(id)
                    forgetLogin(id)
                    main.post { host.privacy() }
                    answer.ok(
                        JSONObject()
                            .put("revoked", off?.outcome == Revoker.Outcome.REVOKED)
                            .put("keys", JSONArray(keys))
                            .put("local", off?.outcome == Revoker.Outcome.LOCAL),
                    )
                } catch (e: Exception) {
                    answer.fail(e)
                }
            }
            val k = byKey()
            try {
                if (logins.get(id) == null) {
                    k.close()
                    drop(null)
                } else {
                    revoker.turnOff(id, url, k) { drop(it) }
                }
            } catch (e: Exception) {
                k.close()
                answer.fail(e)
            }
        }
    }

    // forgetAll turns every kept login off as forget does, one server after
    // the other (AP MobileApp, D56-A), in one step of the app key, so that
    // the fingerprint is asked once for every login under it (D63-A); the
    // answer names the servers whose key was revoked and those that keep
    // one (contract version 4).
    private fun forgetAll(answer: Answer) {
        io.execute {
            val k = byKey(forgetAll = true)
            try {
                revoker.forgetAll(k) { done ->
                    main.post { host.privacy() }
                    answer.ok(JSONObject().put("revoked", JSONArray(done.revoked)).put("left", JSONArray(done.left)).put("local", JSONArray(done.local)))
                }
            } catch (e: Exception) {
                k.close()
                answer.fail(e)
            }
        }
    }

    // byKey opens the kept logins' keys after the prompt for one run of
    // Turn off, Remove or Forget all: one step of the app key, which the
    // run ends (Revoker.byKey); the prompt on the main thread and the rest
    // on io, titled for its server, but Forget all's one prompt for the
    // app key, which opens every server's login. A key that does not open is
    // said in the log and counts as not opened.
    private fun byKey(forgetAll: Boolean = false): Revoker.ByKey = Revoker.byKey(appKey.step(), { s, kept ->
        if (forgetAll && kept.byApp) texts.get(R.string.login_forget_all_title) else texts.get(R.string.login_off_title, s.name)
    }, log)

    // onIo runs [work] on io; where io takes nothing more, nothing, and the
    // app key a step still holds is dropped.
    private fun onIo(work: () -> Unit) {
        try {
            io.execute(work)
        } catch (e: RejectedExecutionException) {
            // the activity went: its pages wait for nothing
            appKey.dropAll()
        }
    }

    // forgetLeavingKey forgets the login kept for [s] whose key stays valid
    // at the server (the phone no longer opens it, or the certificate was
    // trusted anew), the key left by its id to revoke with the next session
    // there (D57-A); not for a server whose node changed, whose key is
    // another server's.
    private fun forgetLeavingKey(s: Server, kept: KeptLogin) {
        if (kept.node == s.node) runCatching { left.add(s.id, kept.user, kept.keyId) }
        forgetLogin(s.id)
    }

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

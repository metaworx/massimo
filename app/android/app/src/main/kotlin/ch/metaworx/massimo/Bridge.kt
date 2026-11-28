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
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

/**
 * Bridge is the native side of `window.massimo` (docs/app.md, contract
 * version 7): it takes the messages of the page's script (res/raw/bridge.js),
 * checks who sent each by its origin, and answers it. The connect page gets
 * the calls of the server list, the sessions the app holds, the app's
 * settings and version, Store login and Exit; a server's page only its own
 * session, its login by fingerprint
 * and the ways back to the list, to its settings there and to the app's.
 * The login's key never reaches a page: the
 * bridge makes and holds it and hands a page the session alone, each login
 * sealed under the one app key ([AppKey]), which a prompt opens for one
 * step. Turn off, Remove and Forget all are the [Revoker]'s, and the keys
 * and sessions they leave valid are [left]'s; their revoke codes are posted
 * at each contact by [codes] (contract version 6, D112-A), a removed
 * server's kept in [gone]. [exited] is set by the list's Exit, which
 * [sessions] and [codes] are given too, so that nothing more of this
 * activity's goes to a server.
 */
class Bridge(
    private val host: Host,
    private val servers: ServerStore,
    private val keeper: SessionKeeper,
    private val sessions: Sessions,
    private val left: KeysLeft,
    private val codes: Codes,
    private val gone: GoneStore,
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
    private val version: String,
    private val exited: AtomicBoolean,
    private val io: Executor,
    private val main: Handler,
) {
    /** Host is what the bridge needs of the activity. */
    interface Host {
        /** open loads [url], an address of the server, in the WebView. */
        fun open(url: String)

        /** connectPage shows the connect page again, with [fragment] for what it opens at its start ("" for nothing): a server's settings or App settings. */
        fun connectPage(fragment: String = "")

        /** privacy is told that what keeps the page from Recents may have changed (guarded). */
        fun privacy()

        /** appearance gives the app the appearance saved, one of Settings.APPEARANCES. */
        fun appearance(appearance: String)

        /**
         * exit ends the app as a swipe from the recent apps does (AP
         * MobileApp, D91-A): the sessions held in memory dropped, each one
         * recorded as held moving into its server's `unended` with its revoke
         * code (MassimoApp.exit), nothing sent to a server, and the task out of
         * the recent apps.
         */
        fun exit()
    }

    /** Reply posts a message back to the frame that sent one. */
    fun interface Reply {
        fun post(message: String)
    }

    /**
     * Opened is the server `open` loaded: its id, the origin its page has,
     * the address it was opened at, and whether the session held is
     * withheld from the page for this opening ([withheld]: `token()`
     * answers null, so that its sign-in card shows), a prompt of a tap
     * having been cancelled or failed, or the sign-in icon tapped, until
     * the page has a session of its own (contract version 5).
     */
    data class Opened(val id: String, val origin: String, val url: String, val withheld: Boolean = false) {
        /**
         * leaving is this opening once its page gives way to the connect
         * page: withheld where its server is [guarded], a kept login asked
         * at each return, so that a way back into the page, Back among
         * them, shows its card and not the session held (D72-B); any other
         * as it is.
         */
        fun leaving(guarded: Boolean): Opened = if (guarded && !withheld) copy(withheld = true) else this

        /**
         * takes says whether the page's setToken([token]) changes the
         * session the app holds: not null from a page whose session is
         * withheld, which holds none of its own to forget, so that the
         * session withheld is not forgotten unended.
         */
        fun takes(token: String?): Boolean = token != null || !withheld
    }

    private val current = AtomicReference<Opened?>(null)

    /** opened is the server the page shown was opened for, null before any. */
    var opened: Opened?
        get() = current.get()
        set(value) = current.set(value)

    // The connect page's channel, for the events.
    private var connect: Reply? = null

    private val revoker = Revoker(servers, logins, sessions, left, prober::probe, io, codes, gone, log, ::forgetLogin)

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
        return guardedServer(now.id)
    }

    // guardedServer reports whether the server [id] has a kept login asked at each return.
    private fun guardedServer(id: String): Boolean {
        val s = servers.get(id) ?: return false
        return s.ask == Server.ASK_RETURN && logins.get(s.id) != null
    }

    /**
     * leaving takes the connect page loading in place of the opened
     * server's page (MainActivity's onPageStarted), whether by the page's
     * way back or by Back: the opening of a server asked at each return is
     * withheld (Opened.leaving), so that the page, reached again without
     * a tap and its prompt, shows its card (AP MobileApp, D72-B).
     */
    fun leaving() {
        val o = opened ?: return
        val next = o.leaving(guardedServer(o.id))
        if (next !== o) current.compareAndSet(o, next)
    }

    /** covers reports whether the page of [origin] is guarded and signed in, with a session the cover keeps from others; not one at its card, the session withheld. */
    fun covers(origin: String?): Boolean = guarded(origin) && opened?.let { !it.withheld && keeper.inMemory(it.id) } == true

    /** close drops the app key a step still holds, as the activity goes (onDestroy). */
    fun close() = appKey.dropAll()

    /** postCodes posts the revoke codes waiting for the server [id] at [url], the address it was opened at, in the background (D112-A): Open at start's. */
    fun postCodes(id: String, url: String) {
        val s = servers.get(id) ?: return
        onIo { if (!exited.get()) runCatching { codes.post(s, url) } }
    }

    /** postGone posts a removed server's revoke codes where the probe finds it at its node, in the background: the app's start and the list shown. */
    fun postGone() = onIo { if (!exited.get()) runCatching { codes.postGone() } }

    /** onMessage handles one message of the page's script, on the main thread. */
    fun onMessage(data: String, origin: String, isMainFrame: Boolean, reply: Reply) {
        if (exited.get()) return // the app is ending: the page is answered no more
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
                // the server's revoke codes first, so that the check finds what they ended (D112-A)
                background(answer) {
                    if (exited.get()) return@background null
                    runCatching { codes.post(s, url) }
                    sessions.check(s, url)
                }
            }
            "signOut" -> {
                val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
                val url = args.opt(1) as? String ?: ""
                // pending: the session this sign-out left carries its revoke code (contract version 6)
                background(answer) {
                    val out = sessions.signOutWith(s, url)
                    JSONObject().put("ended", out.ended).put("pending", out.pending)
                }
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
                val o = now ?: throw bad()
                // withheld for this opening: its card shows (Opened.withheld)
                if (o.withheld) answer.ok(null) else background(answer) { servers.get(o.id)?.let { sessions.held(it) } }
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
                // a page at its card holds no session of its own: its null forgets nothing (Opened.takes)
                if (!o.takes(token)) {
                    answer.ok(null)
                    return
                }
                // null forgets it: Sign out, Forget key, and the dead session
                // the page drops at the server's first 401. A new session is
                // checked before the answer (Sessions.look): its id learnt,
                // for a session forgotten later without the server (D70-A),
                // the keys and sessions left there for its user revoked, and
                // a login kept without its key's id given that id; the answer
                // names what it revoked, {revoked}, for My profile, which
                // reads its keys once it has it (D67-A); null for null. A
                // session the new one replaces is ended there after the
                // answer (D68-A), so that a server slow to answer holds no
                // sign-in up. A session handed in ends what the opening
                // withheld (release).
                io.execute {
                    var ending: Sessions.Ending? = null
                    val revoked = try {
                        val s = servers.get(id)
                        if (s != null) ending = sessions.put(s, token, o.url)
                        if (token != null) release(o)
                        if (token == null || s == null) null else runCatching { sessions.look(s, o.url).revoked }.getOrDefault(emptyList())
                    } catch (e: Exception) {
                        answer.fail(e)
                        return@execute
                    }
                    answer.ok(revoked?.let { JSONObject().put("revoked", JSONArray(it)) })
                    ending?.let { runCatching { it.end() } }
                }
            }
            "login.status" -> {
                val o = now ?: throw bad()
                val can = biometric.canStrong()
                background(answer) {
                    val s = servers.get(o.id) ?: throw BridgeException(R.string.err_no_server)
                    // key_id: the kept login's key, 0 for none or not known (contract version 5; AP MobileApp, D65-A); server:
                    // the server the page was opened for, at the address it was opened at, its node and pinned fingerprint,
                    // which the sign-in card names in place of the address bar the app has not (D62-A)
                    JSONObject().put("can", can).put("user", logins.user(s.id)).put("ask", s.ask).put("plain", Origins.plainOutside(o.url))
                        .put("key_id", logins.get(s.id)?.keyId?.coerceAtLeast(0) ?: 0)
                        .put("server", JSONObject().put("name", s.name).put("url", o.url).put("node", s.node).put("fp", s.fp))
                }
            }
            "login.keep" -> {
                val o = now ?: throw bad()
                keep(servers.get(o.id) ?: throw BridgeException(R.string.err_no_server), o.url, answer)
            }
            "login.fingerprint" -> {
                val o = now ?: throw bad()
                val s = servers.get(o.id) ?: throw BridgeException(R.string.err_no_server)
                fingerprint(s, o.url, answer) { session ->
                    if (session != null) release(o)
                    answer.ok(session)
                }
            }
            "connect" -> {
                answer.ok(null)
                host.connectPage()
            }
            // ☰'s two entries on a server's page (contract version 5; AP MobileApp, D73-A): the connect page with this
            // server's settings open, its id the bridge's, or App settings; the page reads and saves nothing of them
            "openServerSettings" -> {
                val o = now ?: throw bad()
                answer.ok(null)
                host.connectPage(Origins.settingsOf(o.id))
            }
            "openAppSettings" -> {
                answer.ok(null)
                host.connectPage(Origins.APP_SETTINGS)
            }
            // the list's ☰ (contract version 7; AP MobileApp, Block 19): the build's version, whether the phone has a strong
            // biometric enrolled, which Store login needs, and Exit: from then on nothing of this activity's goes to a server, not
            // even a check the page asked for just before (sessions and codes look at exited), so that the codes Exit leaves go
            // at the next start
            "about" -> answer.ok(JSONObject().put("version", version).put("can", biometric.canStrong()))
            "exit" -> {
                exited.set(true)
                answer.ok(null)
                host.exit()
            }
            // Store login in a server's settings (contract version 7): the session held there kept as a login, at the
            // address the probe found, one of the server's own, without a fragment
            "keep" -> {
                val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
                val url = string(args, 1)
                if (s.opening(url) != url) throw BridgeException(R.string.err_open_url)
                keep(s, url, answer)
            }
            else -> throw bad()
        }
    }

    // json is a server as the connect page sees it: with the user whose
    // login is kept, the user of the session held, as last checked, and the
    // id of the kept login's key.
    private fun json(s: Server): JSONObject = s.toJson(logins.user(s.id), sessions.user(s), logins.get(s.id)?.keyId ?: 0)

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
    // marked (`#profile?key=<id>`, contract version 4), as a tap's rule
    // says (Opening, contract version 5; AP MobileApp, D72-B, D74-A): the
    // page at once, or the fingerprint first, without the key for the
    // session held or with it for a new one, the prompt titled with the
    // server's name; a prompt cancelled or failed opens the page with the
    // session withheld for that opening, so that its card shows, and
    // {signIn: true} opens it so without a prompt; a key the server
    // refuses is forgotten and rejects, as do the key's other failures.
    // With {login: true} it logs in by fingerprint first, and resolves
    // false without opening where the prompt was dismissed.
    private fun open(args: JSONArray, answer: Answer) {
        val s = servers.get(string(args, 0)) ?: throw BridgeException(R.string.err_no_server)
        val url = string(args, 1)
        val base = s.opening(url) ?: throw BridgeException(R.string.err_open_url)
        val origin = Origins.of(base) ?: throw BridgeException(R.string.err_open_url)
        val opts = args.optJSONObject(2)
        val go = { withheld: Boolean ->
            answer.ok(true)
            opened = Opened(s.id, origin, base, withheld)
            host.open(url)
            // the revoke codes at the address opened, in the background (D112-A)
            onIo { if (!exited.get()) runCatching { codes.post(s, base) } }
        }
        when {
            opts?.opt("signIn") == true -> go(true)
            opts?.opt("login") == true -> fingerprint(s, base, answer) { session -> if (session == null) answer.ok(false) else go(false) }
            else -> io.execute {
                // the store and the sealed session are read off the main thread
                val rule = try {
                    Opening.of(s.ask, logins.get(s.id) != null, sessions.where(s))
                } catch (e: Exception) {
                    answer.fail(e)
                    return@execute
                }
                main.post { tap(s, base, rule, answer, go) }
            }
        }
    }

    // tap opens [s] at [base] as [rule] says, on the main thread, where the
    // prompt shows; [go] opens the page, the session withheld or not. Over
    // plain HTTP outside the home network the key is not sent (D36-A): the
    // card opens, and says why.
    private fun tap(s: Server, base: String, rule: Opening, answer: Answer, go: (Boolean) -> Unit) {
        try {
            when {
                rule == Opening.OPEN -> go(false)
                rule == Opening.ASK -> biometric.unlock(texts.get(R.string.login_title, s.name)) { passed -> go(!passed) }
                Origins.plainOutside(base) -> go(true)
                else -> fingerprint(s, base, answer) { session -> go(session == null) }
            }
        } catch (e: Exception) {
            answer.fail(e)
        }
    }

    // release gives the page of the opening [o] the sessions held from now
    // on, once it has a session of its own (setToken, login.fingerprint):
    // a reload no longer shows its card. A later opening is left as it is.
    private fun release(o: Opened) {
        if (o.withheld) current.compareAndSet(o, o.copy(withheld = false))
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
    // makes the session with it; the session is checked at once, as
    // setToken checks a page's (D102-A, F43): its id learnt before the page
    // opens, so that nothing forgets it unrecorded, and the keys and
    // sessions left there revoked as at any check. A session it replaces,
    // made from the same key, the server ended with this one's making, and
    // is not logged out (D100-A).
    private fun openKept(s: Server, url: String, kept: KeptLogin, answer: Answer, then: (String?) -> Unit) {
        appKey.open(texts.get(R.string.login_title, s.name), s.id, kept) { r ->
            when (r) {
                is AppKey.Result.Opened -> try {
                    val session = api.loginKey(url, s.fp, r.secret.key)
                    val keyId = r.secret.id.takeIf { it > 0 } ?: kept.keyId
                    val ending = sessions.put(s, session, url, keyId) // a session it replaces ended there (D68-A), after the answer
                    runCatching { sessions.look(s, url) }
                    main.post { then(session) }
                    ending?.let { runCatching { it.end() } }
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

    // keep keeps the login by fingerprint for the session held at [s], at
    // [url], the address the page was opened at (login.keep, the page just
    // signed in) or the one the connect page's probe found (keep, Store
    // login): the user's name from the session, a key of the user's made
    // with it, then the prompt, which opens the app key, or makes it the
    // first time, to seal the key under, its revoke code beside its id
    // (D112-A); a prompt not passed revokes the key again and resolves false.
    private fun keep(s: Server, url: String, answer: Answer) {
        if (Origins.plainOutside(url)) throw BridgeException(R.string.err_plain)
        if (!biometric.canStrong()) throw BridgeException(R.string.err_no_biometric)
        val session = sessions.held(s) ?: throw BridgeException(R.string.err_no_session)
        io.execute {
            val user: String
            val key: ServerApi.Key
            try {
                user = api.me(url, s.fp, session)
                key = api.createKey(url, s.fp, session, user, takeBytes(keyLabel, MAX_LABEL))
            } catch (e: ServerApi.Refused) {
                answer.fail(BridgeException(R.string.err_no_session))
                return@execute
            } catch (e: IOException) {
                answer.fail(BridgeException(R.string.err_server, reason(e)))
                return@execute
            }
            val undo = { revokeQuietly(url, s.fp, session, user, key.id) }
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
                    // pending: what Turn off left waits for its revoke code (contract version 6)
                    answer.ok(
                        JSONObject().put("revoked", off.outcome == Revoker.Outcome.REVOKED).put("left", off.outcome == Revoker.Outcome.LEFT)
                            .put("pending", off.pending),
                    )
                }
            } catch (e: Exception) {
                k.close()
                answer.fail(e)
            }
        }
    }

    // remove forgets a server (Revoker.remove): a kept login is turned off
    // first, at url or the address that answers, as forget turns it off (AP
    // MobileApp, Block 14), else the session signed out there (D69-A), then
    // the server, its certificate, its session and its login go. The answer
    // says whether the login's key was revoked (revoked), the ids of the
    // phone's keys still valid there (keys: its unrevoked, the login's among
    // them where it could not be revoked) and of its sessions (sessions: its
    // unended, the one signed out among them where the server did not end
    // it, contract version 5), and whether the login's key went without its
    // id being known (local).
    private fun remove(id: String, url: String, answer: Answer) {
        io.execute {
            val k = byKey()
            try {
                revoker.remove(id, url, k) { r ->
                    r.fold({ removed ->
                        main.post { host.privacy() }
                        answer.ok(
                            JSONObject()
                                .put("revoked", removed.off?.outcome == Revoker.Outcome.REVOKED)
                                .put("keys", JSONArray(removed.keys))
                                .put("sessions", JSONArray(removed.sessions))
                                .put("local", removed.off?.outcome == Revoker.Outcome.LOCAL)
                                .put("pending", removed.pending),
                        )
                    }, { e -> answer.fail(e) })
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
                    answer.ok(
                        JSONObject().put("revoked", JSONArray(done.revoked)).put("left", JSONArray(done.left)).put("local", JSONArray(done.local))
                            .put("pending", JSONArray(done.pending)),
                    )
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
        if (kept.node == s.node) runCatching { left.add(s.id, kept.user, kept.keyId, kept.code) }
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

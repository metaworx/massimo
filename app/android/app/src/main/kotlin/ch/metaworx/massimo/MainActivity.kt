// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.annotation.SuppressLint
import android.app.AlertDialog
import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.content.pm.ApplicationInfo
import android.net.ConnectivityManager
import android.net.Network
import android.net.Uri
import android.net.http.SslCertificate
import android.net.http.SslError
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Log
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import android.webkit.RenderProcessGoneDetail
import android.webkit.SslErrorHandler
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.activity.OnBackPressedCallback
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AppCompatDelegate
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.webkit.ScriptHandler
import androidx.webkit.WebViewAssetLoader
import androidx.webkit.WebViewCompat
import androidx.webkit.WebViewFeature
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.io.File
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

/**
 * MainActivity is the shell: one WebView that shows the connect page from
 * the app's own copy of the web UI, then the page of the server the user
 * chose, with the bridge (docs/app.md) in both; at its start the server
 * marked "Open at start" in place of the list, where it answers (D43-A). A
 * server whose login by fingerprint is asked at each return has its page
 * covered while the app is hidden, until the prompt is passed, and kept out
 * of Recents' pictures. An AppCompat activity, so that the app's appearance
 * (App settings, D53-A) is AppCompat's night mode, which the theme's
 * android:isLightTheme hands on to the WebView's prefers-color-scheme.
 */
class MainActivity : AppCompatActivity() {
    private val main = Handler(Looper.getMainLooper())
    private val io: ExecutorService = Executors.newCachedThreadPool()
    private lateinit var web: WebView
    private lateinit var bridge: Bridge
    private lateinit var prober: Prober
    private lateinit var discovery: Discovery
    private lateinit var back: OnBackPressedCallback
    private lateinit var biometric: Biometric
    private lateinit var cover: View
    private var covered = false
    private var shown: String? = null // the origin of the page the WebView shows
    private var serverScript: ScriptHandler? = null
    private var channelFor: String? = null
    private var listening = false
    private var warned = false
    private var networkSince = 0L
    private var files: ValueCallback<Array<Uri>>? = null

    private val pickFile = registerForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        files?.onReceiveValue(uri?.let { arrayOf(it) })
        files = null
    }

    // A path handler is handed the path after the prefix it is registered
    // at: /ui/connect.html reaches it as connect.html, the asset
    // ui/connect.html.
    private val assets by lazy {
        val files = WebViewAssetLoader.AssetsPathHandler(this)
        WebViewAssetLoader.Builder()
            .addPathHandler("/ui/") { path -> files.handle("ui/$path") }
            .build()
    }

    // The app's appearance before AppCompat sets the activity's
    // configuration up from it, so that the theme, the first frame and the
    // WebView have it; a mode that is the one set already changes nothing.
    override fun attachBaseContext(newBase: Context) {
        AppCompatDelegate.setDefaultNightMode(AppSettings.nightMode(AppSettings(File(newBase.filesDir, SETTINGS)).get().appearance))
        super.attachBaseContext(newBase)
    }

    // The features the bridge needs are checked first; nothing of
    // WebViewCompat runs without them.
    @SuppressLint("RequiresFeature")
    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        val supported = try {
            WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER) &&
                WebViewFeature.isFeatureSupported(WebViewFeature.DOCUMENT_START_SCRIPT)
        } catch (e: RuntimeException) {
            false // no WebView installed, or one being updated
        }
        if (!supported) {
            outdated()
            return
        }
        val texts = Texts { id, args -> getString(id, *args) }
        val agent = agent()
        discovery = Discovery(this, main)
        biometric = Biometric(this, getString(android.R.string.cancel))
        val servers = ServerStore(File(filesDir, "servers.json"))
        val tokens = TokenStore(File(filesDir, "tokens.json"), KeystoreSealer())
        // Version 1's switch kept a session behind a prompt the app asks no more.
        servers.takeLegacy().forEach { tokens.remove(it) }
        val settings = AppSettings(File(filesDir, SETTINGS))
        val log = Logger { Log.w(LOG_TAG, it) }
        val keeper = SessionKeeper(tokens)
        val logins = LoginStore(File(filesDir, "logins.json"))
        val api = ServerApi(agent)
        prober = Prober(agent, texts)
        val left = KeysLeft(servers, logins, api, log)
        bridge = Bridge(
            host = object : Bridge.Host {
                override fun open(url: String) = openServer(url)

                override fun connectPage() = web.loadUrl(Origins.CONNECT_URL)

                override fun privacy() = this@MainActivity.privacy()

                // Another night mode makes AppCompat create the activity
                // anew, the WebView's state and the page's address kept.
                override fun appearance(appearance: String) = AppCompatDelegate.setDefaultNightMode(AppSettings.nightMode(appearance))
            },
            servers = servers,
            keeper = keeper,
            sessions = Sessions(keeper, logins, api, prober::probe, left, log),
            left = left,
            logins = logins,
            keys = LoginKey(),
            api = api,
            prober = prober,
            discovery = discovery,
            biometric = biometric,
            settings = settings,
            texts = texts,
            log = log,
            keyLabel = getString(R.string.login_key_label, Build.MODEL),
            io = io,
            main = main,
        )
        web = WebView(this)
        web.id = R.id.web
        setUp(web, agent)
        cover = coverView()
        setContentView(insetted(web, cover))

        back = object : OnBackPressedCallback(false) {
            override fun handleOnBackPressed() = web.goBack()
        }
        onBackPressedDispatcher.addCallback(this, back)

        WebViewCompat.addDocumentStartJavaScript(web, script("connect"), setOf(Origins.APP))
        val id = savedInstanceState?.getString(KEY_ID)
        val origin = savedInstanceState?.getString(KEY_ORIGIN)
        val url = savedInstanceState?.getString(KEY_URL)
        // a page opened at its card stays there when the activity is made anew (a new appearance), its session still withheld
        val withheld = savedInstanceState?.getBoolean(KEY_WITHHELD) == true
        bridge.opened = if (id != null && origin != null && url != null) Bridge.Opened(id, origin, url, withheld) else null
        channel(bridge.opened?.origin)
        if (savedInstanceState == null || web.restoreState(savedInstanceState) == null) {
            val first = if (savedInstanceState == null) servers.atStart() else null
            if (first == null) web.loadUrl(Origins.CONNECT_URL) else openAtStart(first)
        }
    }

    // openAtStart opens the server marked "Open at start" at the address
    // it answers at, the LAN one first (Addresses.pick), in place of the
    // list; the list where it answers at neither, as a changed certificate
    // does not. The window shows the app's background while it asks.
    private fun openAtStart(s: Server) {
        val fp = s.fp.ifEmpty { null }
        io.execute {
            val url = Addresses.pick(s, { u -> prober.probe(listOf(u), fp).first() })
            main.post {
                if (isFinishing || isDestroyed) return@post
                val origin = url?.let(Origins::of)
                if (url == null || origin == null) {
                    web.loadUrl(Origins.CONNECT_URL)
                    return@post
                }
                bridge.opened = Bridge.Opened(s.id, origin, url)
                openServer(url)
            }
        }
    }

    override fun onStart() {
        super.onStart()
        if (!::bridge.isInitialized) return
        networkSince = SystemClock.elapsedRealtime()
        getSystemService(ConnectivityManager::class.java)?.registerDefaultNetworkCallback(network)
        if (covered) unlock()
    }

    override fun onStop() {
        super.onStop()
        if (!::bridge.isInitialized) return
        runCatching { getSystemService(ConnectivityManager::class.java)?.unregisterNetworkCallback(network) }
        discovery.stop()
        // Hidden: a page asked at each return is covered, but for the app's
        // own prompt and file chooser, which hide nothing.
        if (!biometric.asking && files == null && bridge.covers(shown)) {
            covered = true
            cover.visibility = View.VISIBLE
        }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        if (!::web.isInitialized) return
        web.saveState(outState)
        bridge.opened?.let {
            outState.putString(KEY_ID, it.id)
            outState.putString(KEY_ORIGIN, it.origin)
            outState.putString(KEY_URL, it.url)
            outState.putBoolean(KEY_WITHHELD, it.withheld)
        }
    }

    override fun onDestroy() {
        if (::biometric.isInitialized) biometric.close()
        if (::bridge.isInitialized) bridge.close()
        if (::web.isInitialized) web.destroy()
        io.shutdownNow()
        super.onDestroy()
    }

    // setUp gives the WebView what the pages need and nothing more: no
    // files, no windows of its own, no Safe Browsing or metrics (the
    // manifest), the debugger in a debug build only.
    @SuppressLint("SetJavaScriptEnabled") // the web UI is JavaScript
    private fun setUp(web: WebView, agent: String) {
        web.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            allowFileAccess = false
            allowContentAccess = false
            javaScriptCanOpenWindowsAutomatically = false
            setSupportMultipleWindows(false)
            mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            setGeolocationEnabled(false)
            userAgentString = agent + " " + WebSettings.getDefaultUserAgent(this@MainActivity)
        }
        if (applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE != 0) WebView.setWebContentsDebuggingEnabled(true)
        web.setBackgroundColor(getColor(R.color.background))
        web.webViewClient = Client()
        web.webChromeClient = Chrome()
        web.setDownloadListener { _, _, _, _, _ ->
            Toast.makeText(this, R.string.download_unsupported, Toast.LENGTH_LONG).show()
        }
    }

    // channel adds the bridge's channel for the connect page and for the
    // server opened, and the server's script for the server's origin alone;
    // onCreate made sure the WebView has both features.
    @SuppressLint("RequiresFeature")
    private fun channel(serverOrigin: String?) {
        if (listening && channelFor == serverOrigin) return
        if (listening) WebViewCompat.removeWebMessageListener(web, CHANNEL)
        WebViewCompat.addWebMessageListener(web, CHANNEL, setOfNotNull(Origins.APP, serverOrigin)) { _, message, source, isMainFrame, proxy ->
            val data = message.data ?: return@addWebMessageListener
            bridge.onMessage(data, source.toString(), isMainFrame) { proxy.postMessage(it) }
        }
        listening = true
        serverScript?.remove()
        serverScript = serverOrigin?.let { WebViewCompat.addDocumentStartJavaScript(web, script("server"), setOf(it)) }
        channelFor = serverOrigin
    }

    private fun openServer(url: String) {
        channel(Origins.of(url))
        web.loadUrl(url)
    }

    // privacy keeps the page out of Recents' pictures while it is guarded
    // (Bridge.guarded): from Android 13 by setRecentsScreenshotEnabled,
    // which leaves screenshots allowed, before it by FLAG_SECURE, which
    // forbids them too.
    private fun privacy() {
        val guard = bridge.guarded(shown)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            setRecentsScreenshotEnabled(!guard)
        } else if (guard) {
            window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        } else {
            window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
        }
    }

    // unlock asks for the fingerprint to take the cover away; dismissed,
    // the cover stays, with its button to ask again.
    private fun unlock() {
        val name = bridge.opened?.let { bridge.server(it.id)?.name } ?: getString(R.string.app_name)
        biometric.unlock(getString(R.string.unlock_title, name)) { ok ->
            if (!ok) return@unlock
            covered = false
            cover.visibility = View.GONE
        }
    }

    // coverView is what covers a page asked at each return while the app
    // is hidden: the app's background, a line, and Unlock.
    private fun coverView(): View {
        val pad = (24 * resources.displayMetrics.density).toInt()
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER
            setPadding(pad, pad, pad, pad)
            setBackgroundColor(getColor(R.color.background))
            isClickable = true // nothing reaches the page beneath
            visibility = View.GONE
            addView(TextView(this@MainActivity).apply {
                setText(R.string.cover_text)
                textSize = 18f
                gravity = Gravity.CENTER
                setPadding(0, 0, 0, pad)
            })
            addView(Button(this@MainActivity).apply {
                setText(R.string.cover_unlock)
                setOnClickListener { unlock() }
            })
        }
    }

    private fun script(kind: String): String =
        resources.openRawResource(R.raw.bridge).bufferedReader().use { it.readText() } + "(" + JSONObject.quote(kind) + ");"

    private inner class Client : WebViewClient() {
        override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
            if (request.url.host != Origins.APP_HOST) return null
            // Nothing of the app's own origin goes to the network.
            // A file the app does not have is answered as not found, where
            // the handler's empty answer is one the WebView calls invalid.
            return assets.shouldInterceptRequest(request.url)?.takeIf { it.data != null }
                ?: WebResourceResponse("text/plain", "utf-8", 404, "Not Found", null, ByteArrayInputStream(ByteArray(0)))
        }

        override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
            if (!request.isForMainFrame) return false
            val origin = Origins.of(request.url.toString())
            if (origin == Origins.APP || (origin != null && origin == bridge.opened?.origin)) return false
            external(request.url)
            return true
        }

        override fun doUpdateVisitedHistory(view: WebView, url: String?, isReload: Boolean) {
            back.isEnabled = view.canGoBack()
        }

        // The page shown decides whether Recents may keep its picture. The
        // list in a server's page's place withholds the session of a server
        // asked at each return, so that its page reached again without a
        // tap shows its card (Bridge.leaving, D72-B).
        override fun onPageStarted(view: WebView, url: String?, favicon: android.graphics.Bitmap?) {
            shown = url?.let(Origins::of)
            if (shown == Origins.APP) bridge.leaving()
            privacy()
        }

        // The list is the app's first page: once it has loaded, the pages
        // before it are forgotten, so that Back from it leaves the app, never
        // back into a server's page past its prompt.
        override fun onPageFinished(view: WebView, url: String?) {
            if (url?.let(Origins::of) != Origins.APP) return
            view.clearHistory()
            back.isEnabled = view.canGoBack()
        }

        // A certificate the system does not trust is the server's where it
        // is the one the user pinned for the server opened; any other is
        // refused, and a changed one of the server's says so.
        @SuppressLint("WebViewClientOnReceivedSslError") // proceeds for the pinned certificate alone
        override fun onReceivedSslError(view: WebView, handler: SslErrorHandler, error: SslError) {
            val now = bridge.opened
            val server = now?.let { bridge.server(it.id) }
            val ours = now != null && Origins.of(error.url) == now.origin
            val der = der(error.certificate)
            if (ours && server != null && der != null && Fingerprint.same(Fingerprint.of(der), server.fp)) {
                handler.proceed()
                return
            }
            handler.cancel()
            if (ours) certificateChanged(server?.name ?: error.url)
        }

        override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
            (view.parent as? ViewGroup)?.removeView(view)
            view.destroy()
            recreate()
            return true
        }
    }

    private inner class Chrome : WebChromeClient() {
        // The server's page restores a backup from a file the user picks.
        override fun onShowFileChooser(view: WebView, callback: ValueCallback<Array<Uri>>, params: FileChooserParams): Boolean {
            files?.onReceiveValue(null)
            files = callback
            val types = params.acceptTypes.filter { it.contains('/') }.ifEmpty { listOf("*/*") }
            return try {
                pickFile.launch(types.toTypedArray())
                true
            } catch (e: ActivityNotFoundException) {
                files = null
                false
            }
        }
    }

    private fun certificateChanged(name: String) {
        if (warned) return
        warned = true
        AlertDialog.Builder(this)
            .setTitle(R.string.cert_changed_title)
            .setMessage(getString(R.string.cert_changed_text, name))
            .setPositiveButton(android.R.string.ok, null)
            .setOnDismissListener {
                warned = false
                web.loadUrl(Origins.CONNECT_URL)
            }
            .show()
    }

    private fun external(uri: Uri) {
        if (uri.scheme !in setOf("http", "https", "mailto")) return
        try {
            startActivity(Intent(Intent.ACTION_VIEW, uri).addCategory(Intent.CATEGORY_BROWSABLE))
        } catch (e: ActivityNotFoundException) {
            // no app takes the link
        }
    }

    private val network = object : ConnectivityManager.NetworkCallback() {
        override fun onAvailable(n: Network) = changed()

        override fun onLost(n: Network) = changed()
    }

    private val fireNetwork = Runnable { bridge.event(JSONObject().put("type", "network")) }

    // changed tells the connect page that the network changed, once it
    // settled, and not for the callback's first word at registration.
    private fun changed() {
        main.post {
            if (SystemClock.elapsedRealtime() - networkSince < SETTLE_MS) return@post
            main.removeCallbacks(fireNetwork)
            main.postDelayed(fireNetwork, SETTLE_MS / 2)
        }
    }

    private fun outdated() {
        val text = TextView(this).apply {
            setText(R.string.webview_outdated)
            textSize = 18f
            val pad = (24 * resources.displayMetrics.density).toInt()
            setPadding(pad, pad, pad, pad)
        }
        setContentView(insetted(text))
    }

    // insetted keeps views clear of the system bars, the cut-out and the
    // keyboard, as edge-to-edge requires from Android 15, the later ones
    // over the earlier.
    private fun insetted(vararg views: View): View {
        val root = FrameLayout(this)
        root.setBackgroundColor(getColor(R.color.background))
        for (view in views) root.addView(view, FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT)
        ViewCompat.setOnApplyWindowInsetsListener(root) { v, insets ->
            val i = insets.getInsets(
                WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout() or WindowInsetsCompat.Type.ime(),
            )
            v.setPadding(i.left, i.top, i.right, i.bottom)
            WindowInsetsCompat.CONSUMED
        }
        return root
    }

    private companion object {
        const val LOG_TAG = "Massimo"
        const val SETTINGS = "settings.json"
        const val CHANNEL = "massimoChannel"
        const val KEY_ID = "massimo.opened.id"
        const val KEY_ORIGIN = "massimo.opened.origin"
        const val KEY_URL = "massimo.opened.url"
        const val KEY_WITHHELD = "massimo.opened.withheld"
        const val SETTLE_MS = 2000L

        // agent names the app and the phone first: the server labels a
        // session with the first 64 characters of the User-Agent.
        fun agent(): String {
            val s = "Massimo/${BuildConfig.VERSION_NAME} (Android ${Build.VERSION.RELEASE}; ${Build.MODEL})"
            return s.map { if (it in ' '..'~') it else '?' }.joinToString("")
        }

        @Suppress("DEPRECATION")
        fun der(cert: SslCertificate?): ByteArray? {
            if (cert == null) return null
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) return cert.x509Certificate?.encoded
            // Before Android 10 the certificate is only in the saved state.
            return SslCertificate.saveState(cert)?.getByteArray("x509-certificate")
        }
    }
}

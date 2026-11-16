// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.annotation.SuppressLint
import android.app.AlertDialog
import android.content.ActivityNotFoundException
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
import android.view.View
import android.view.ViewGroup
import android.webkit.RenderProcessGoneDetail
import android.webkit.SslErrorHandler
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import android.widget.TextView
import android.widget.Toast
import androidx.activity.OnBackPressedCallback
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.fragment.app.FragmentActivity
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
 * chose, with the bridge (docs/app.md) in both.
 */
class MainActivity : FragmentActivity() {
    private val main = Handler(Looper.getMainLooper())
    private val io: ExecutorService = Executors.newCachedThreadPool()
    private lateinit var web: WebView
    private lateinit var bridge: Bridge
    private lateinit var discovery: Discovery
    private lateinit var back: OnBackPressedCallback
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

    private val assets by lazy {
        WebViewAssetLoader.Builder()
            .addPathHandler("/ui/", WebViewAssetLoader.AssetsPathHandler(this))
            .build()
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
        bridge = Bridge(
            host = { url -> openServer(url) },
            servers = ServerStore(File(filesDir, "servers.json")),
            tokens = TokenStore(File(filesDir, "tokens.json"), KeystoreSealer()),
            prober = Prober(agent, texts),
            discovery = discovery,
            biometric = Biometric(this),
            texts = texts,
            io = io,
            main = main,
        )
        web = WebView(this)
        web.id = R.id.web
        setUp(web, agent)
        setContentView(insetted(web))

        back = object : OnBackPressedCallback(false) {
            override fun handleOnBackPressed() = web.goBack()
        }
        onBackPressedDispatcher.addCallback(this, back)

        WebViewCompat.addDocumentStartJavaScript(web, script("connect"), setOf(Origins.APP))
        val id = savedInstanceState?.getString(KEY_ID)
        val origin = savedInstanceState?.getString(KEY_ORIGIN)
        bridge.opened = if (id != null && origin != null) Bridge.Opened(id, origin) else null
        channel(bridge.opened?.origin)
        if (savedInstanceState == null || web.restoreState(savedInstanceState) == null) {
            web.loadUrl(Origins.CONNECT_URL)
        }
    }

    override fun onStart() {
        super.onStart()
        if (!::bridge.isInitialized) return
        networkSince = SystemClock.elapsedRealtime()
        getSystemService(ConnectivityManager::class.java)?.registerDefaultNetworkCallback(network)
    }

    override fun onStop() {
        super.onStop()
        if (!::bridge.isInitialized) return
        runCatching { getSystemService(ConnectivityManager::class.java)?.unregisterNetworkCallback(network) }
        discovery.stop()
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        if (!::web.isInitialized) return
        web.saveState(outState)
        bridge.opened?.let {
            outState.putString(KEY_ID, it.id)
            outState.putString(KEY_ORIGIN, it.origin)
        }
    }

    override fun onDestroy() {
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

    private fun script(kind: String): String =
        resources.openRawResource(R.raw.bridge).bufferedReader().use { it.readText() } + "(" + JSONObject.quote(kind) + ");"

    private inner class Client : WebViewClient() {
        override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
            if (request.url.host != Origins.APP_HOST) return null
            // Nothing of the app's own origin goes to the network.
            return assets.shouldInterceptRequest(request.url)
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

    // insetted keeps a view clear of the system bars, the cut-out and the
    // keyboard, as edge-to-edge requires from Android 15.
    private fun insetted(view: View): View {
        val root = FrameLayout(this)
        root.setBackgroundColor(getColor(R.color.background))
        root.addView(view, FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT)
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
        const val CHANNEL = "massimoChannel"
        const val KEY_ID = "massimo.opened.id"
        const val KEY_ORIGIN = "massimo.opened.origin"
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

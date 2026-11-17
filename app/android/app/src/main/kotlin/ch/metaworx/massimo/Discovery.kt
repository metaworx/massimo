// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.net.wifi.WifiManager
import android.os.Build
import android.os.Handler
import androidx.core.content.ContextCompat
import org.json.JSONObject
import java.net.Inet4Address
import java.net.Inet6Address
import java.net.InetAddress

/**
 * Discovery browses the LAN for `_massimo._tcp`, the announcement of
 * `serve` (AP ServerAnnouncements), and resolves each server it finds into
 * a `found` event (docs/app.md, "The events"). One search runs at a time:
 * a new one ends the one before.
 */
class Discovery(private val context: Context, private val main: Handler) {
    private var running: Search? = null

    /** search browses for [ms] milliseconds, calling [onFound] with each event, then [done]; on the main thread. */
    fun search(ms: Long, onFound: (JSONObject) -> Unit, done: () -> Unit) {
        running?.stop()
        running = Search(ms, onFound, done).also { it.start() }
    }

    /** stop ends a search, as its time running out would. */
    fun stop() {
        running?.stop()
        running = null
    }

    private inner class Search(
        private val ms: Long,
        private val onFound: (JSONObject) -> Unit,
        private val done: () -> Unit,
    ) : NsdManager.DiscoveryListener {
        private val nsd = context.getSystemService(NsdManager::class.java)
        private val lock = context.applicationContext.getSystemService(WifiManager::class.java)
            ?.createMulticastLock("massimo-discovery")?.apply { setReferenceCounted(false) }
        private val end = Runnable { stop() }
        private val seen = HashSet<String>()
        private val reported = HashSet<String>()
        private val queue = ArrayDeque<NsdServiceInfo>()
        private val callbacks = mutableListOf<Any>()
        private var registered = false
        private var resolving = false
        private var finished = false

        fun start() {
            lock?.acquire()
            try {
                nsd.discoverServices(TYPE, NsdManager.PROTOCOL_DNS_SD, this)
                registered = true
            } catch (e: RuntimeException) {
                stop()
                return
            }
            main.postDelayed(end, ms)
        }

        fun stop() {
            if (finished) return
            finished = true
            main.removeCallbacks(end)
            if (registered) runCatching { nsd.stopServiceDiscovery(this) }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                callbacks.forEach { cb -> runCatching { nsd.unregisterServiceInfoCallback(cb as NsdManager.ServiceInfoCallback) } }
            }
            callbacks.clear()
            queue.clear()
            if (lock?.isHeld == true) lock.release()
            done()
        }

        override fun onDiscoveryStarted(serviceType: String) {}

        override fun onDiscoveryStopped(serviceType: String) {}

        override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {
            main.post {
                registered = false
                stop()
            }
        }

        override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) {}

        override fun onServiceFound(info: NsdServiceInfo) {
            main.post {
                if (!finished && seen.add(info.serviceName)) resolve(info)
            }
        }

        override fun onServiceLost(info: NsdServiceInfo) {}

        private fun resolve(info: NsdServiceInfo) {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                val cb = object : NsdManager.ServiceInfoCallback {
                    override fun onServiceInfoCallbackRegistrationFailed(errorCode: Int) {}
                    override fun onServiceUpdated(serviceInfo: NsdServiceInfo) = report(serviceInfo)
                    override fun onServiceLost() {}
                    override fun onServiceInfoCallbackUnregistered() {}
                }
                try {
                    nsd.registerServiceInfoCallback(info, ContextCompat.getMainExecutor(context), cb)
                    callbacks += cb
                } catch (e: RuntimeException) {
                    // a service that cannot be resolved is not reported
                }
            } else {
                queue.addLast(info)
                resolveNext()
            }
        }

        // Before Android 14 NsdManager resolves one service at a time.
        @Suppress("DEPRECATION")
        private fun resolveNext() {
            if (resolving || finished) return
            val info = queue.removeFirstOrNull() ?: return
            resolving = true
            nsd.resolveService(
                info,
                object : NsdManager.ResolveListener {
                    override fun onServiceResolved(serviceInfo: NsdServiceInfo) {
                        main.post {
                            resolving = false
                            report(serviceInfo)
                            resolveNext()
                        }
                    }

                    override fun onResolveFailed(serviceInfo: NsdServiceInfo, errorCode: Int) {
                        main.post {
                            resolving = false
                            resolveNext()
                        }
                    }
                },
            )
        }

        @Suppress("DEPRECATION")
        private fun report(info: NsdServiceInfo) {
            if (finished || info.serviceName in reported) return
            val hosts = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
                info.hostAddresses
            } else {
                listOfNotNull(info.host)
            }
            val event = Found.event(info.serviceName, hosts, info.port, info.attributes) ?: return
            reported += info.serviceName
            onFound(event)
        }
    }

    companion object {
        const val TYPE = "_massimo._tcp"
    }
}

/** Found makes the `found` event of a resolved service, from the TXT keys `name`, `node`, `v`, `tls` and `fp`. */
object Found {
    fun event(serviceName: String, hosts: List<InetAddress>, port: Int, attrs: Map<String, ByteArray?>): JSONObject? {
        val host = pick(hosts) ?: return null
        if (port !in 1..65535) return null
        fun txt(key: String) = attrs[key]?.toString(Charsets.UTF_8)?.trim().orEmpty()
        val service = JSONObject()
            .put("name", txt("name").ifEmpty { serviceName })
            .put("host", host)
            .put("port", port)
            .put("node", txt("node"))
            .put("version", txt("v"))
            .put("tls", txt("tls") == "1")
            .put("fp", Fingerprint.canonical(txt("fp")) ?: "")
        return JSONObject().put("type", "found").put("service", service)
    }

    /** pick is the address to reach a server at: IPv4 first, then IPv6 but link-local, which a URL cannot name. */
    fun pick(hosts: List<InetAddress>): String? {
        val a = hosts.firstOrNull { it is Inet4Address && !it.isLoopbackAddress }
            ?: hosts.firstOrNull { it is Inet6Address && !it.isLinkLocalAddress && !it.isLoopbackAddress }
            ?: return null
        return a.hostAddress?.substringBefore('%')
    }
}

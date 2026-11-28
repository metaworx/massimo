// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class OriginsTest {
    @Test
    fun originOfAUrl() {
        assertEquals("https://massimo.example.net", Origins.of("https://Massimo.Example.net/"))
        assertEquals("https://massimo.example.net", Origins.of("https://massimo.example.net:443/ui/"))
        assertEquals("http://massimo.example.net", Origins.of("http://massimo.example.net:80/"))
        assertEquals("http://192.168.1.20:8099", Origins.of("http://192.168.1.20:8099/"))
        assertEquals("https://[fd00::20]:8443", Origins.of("https://[fd00::20]:8443/"))
        assertEquals(Origins.APP, Origins.of(Origins.CONNECT_URL))
    }

    @Test
    fun noOriginButOfHttp() {
        assertNull(Origins.of("ftp://massimo.example.net/"))
        assertNull(Origins.of("file:///data/local/tmp/x.html"))
        assertNull(Origins.of("about:blank"))
        assertNull(Origins.of("not a url"))
        assertNull(Origins.of("http:///nohost"))
    }

    @Test
    fun callerByOrigin() {
        val opened = "http://192.168.1.20:8099"
        assertEquals(Caller.CONNECT, Origins.caller(Origins.APP, true, opened))
        assertEquals(Caller.SERVER, Origins.caller(opened, true, opened))
        assertEquals(Caller.SERVER, Origins.caller("http://192.168.1.20:8099/", true, opened))
        assertEquals(Caller.NONE, Origins.caller("http://192.168.1.21:8099", true, opened))
        assertEquals(Caller.NONE, Origins.caller("http://192.168.1.20:8099", true, null))
        // A frame inside a page is nobody, the shell's own included.
        assertEquals(Caller.NONE, Origins.caller(Origins.APP, false, opened))
        assertEquals(Caller.NONE, Origins.caller(opened, false, opened))
        assertEquals(Caller.NONE, Origins.caller("null", true, opened))
    }

    @Test
    fun whoMayCallWhat() {
        val connect = listOf("servers", "save", "remove", "discover", "probe", "trust", "open", "forget", "forgetAll")
        // contract version 3: the sessions the app holds, and its settings
        val v3 = listOf("session", "signOut", "signOutAll", "settings", "saveSettings")
        // contract version 7: the app's version, Exit and Store login (AP MobileApp, Block 19)
        val v7 = listOf("about", "exit", "keep")
        for (call in connect + v3 + v7) {
            assertTrue(call, Origins.allowed(Caller.CONNECT, call))
            assertFalse(call, Origins.allowed(Caller.SERVER, call))
        }
        // contract version 5: the server's page hands over to this server's settings and the app's, on the connect page
        for (call in listOf("token", "setToken", "login.status", "login.keep", "login.fingerprint", "connect", "openServerSettings", "openAppSettings")) {
            assertFalse(call, Origins.allowed(Caller.CONNECT, call))
            assertTrue(call, Origins.allowed(Caller.SERVER, call))
        }
        assertTrue(Origins.allowed(Caller.CONNECT, "hello"))
        assertTrue(Origins.allowed(Caller.SERVER, "hello"))
        for (call in listOf("hello", "servers", "token", "anything")) assertFalse(call, Origins.allowed(Caller.NONE, call))
        assertFalse(Origins.allowed(Caller.CONNECT, "anything"))
    }

    @Test
    fun theConnectPagesFragments() {
        // what connect.js opens at its start: a server's settings by its id, and App settings
        assertEquals("#settings=0123456789abcdef01234567", Origins.settingsOf("0123456789abcdef01234567"))
        assertEquals("#settings=a%26b%23c", Origins.settingsOf("a&b#c"))
        assertEquals("#app-settings", Origins.APP_SETTINGS)
    }

    @Test
    fun aPagesOwnWayToTheListCarriesNoFragment() {
        // the settings the list opens at its start are the bridge's handover alone
        assertEquals(Origins.CONNECT_URL, Origins.byPage(Origins.CONNECT_URL + "#settings=0123456789abcdef01234567"))
        assertEquals(Origins.CONNECT_URL, Origins.byPage(Origins.CONNECT_URL + Origins.APP_SETTINGS))
        assertNull("the list alone goes as it is", Origins.byPage(Origins.CONNECT_URL))
        assertNull("another origin is not the shell's", Origins.byPage("https://massimo.example.net/#settings=x"))
        // a form posted to the list keeps its fragment past shouldOverrideUrlLoading: the shell serves GET alone
        assertTrue(Origins.appServes("GET"))
        for (m in listOf("POST", "PUT", "DELETE", "HEAD", "OPTIONS", "get")) assertFalse(m, Origins.appServes(m))
    }

    @Test
    fun theHomeNetwork() {
        for (h in listOf("192.168.1.20", "10.0.0.5", "172.16.0.1", "172.31.255.1", "127.0.0.1", "169.254.1.1", "localhost", "massimo.local", "box",
            "[::1]", "fd00::1", "[fe80::1]")) {
            assertTrue(h, Origins.isPrivateHost(h))
        }
        for (h in listOf("172.32.0.1", "8.8.8.8", "massimo.example.net", "2001:db8::1")) assertFalse(h, Origins.isPrivateHost(h))
    }

    @Test
    fun noKeyOverPlainHttpOutside() {
        assertTrue(Origins.plainOutside("http://massimo.example.net/"))
        assertTrue(Origins.plainOutside("http://8.8.8.8:8099/"))
        assertFalse(Origins.plainOutside("http://192.168.1.20:8099/"))
        assertFalse(Origins.plainOutside("https://massimo.example.net/"))
        assertFalse(Origins.plainOutside("http://[fd00::1]:8099/"))
        assertTrue(Origins.plainOutside("http://bad host/"))
    }
}

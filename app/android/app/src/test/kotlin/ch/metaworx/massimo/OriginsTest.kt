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
        for (call in connect + v3) {
            assertTrue(call, Origins.allowed(Caller.CONNECT, call))
            assertFalse(call, Origins.allowed(Caller.SERVER, call))
        }
        for (call in listOf("token", "setToken", "login.status", "login.keep", "login.fingerprint", "connect")) {
            assertFalse(call, Origins.allowed(Caller.CONNECT, call))
            assertTrue(call, Origins.allowed(Caller.SERVER, call))
        }
        assertTrue(Origins.allowed(Caller.CONNECT, "hello"))
        assertTrue(Origins.allowed(Caller.SERVER, "hello"))
        for (call in listOf("hello", "servers", "token", "anything")) assertFalse(call, Origins.allowed(Caller.NONE, call))
        assertFalse(Origins.allowed(Caller.CONNECT, "anything"))
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

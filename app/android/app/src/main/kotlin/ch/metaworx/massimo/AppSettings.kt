// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.appcompat.app.AppCompatDelegate
import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.io.IOException

/**
 * The app's own settings (docs/app.md, `settings`, contract version 3):
 * [appearance], app-wide, one of [APPEARANCES] (AP MobileApp, D53-A).
 */
data class Settings(val appearance: String = SYSTEM) {
    /** toJson is the settings as the connect page reads them. */
    fun toJson(): JSONObject = JSONObject().put("appearance", appearance)

    companion object {
        /** The appearances: as the phone, light, dark. */
        const val SYSTEM = "system"
        const val LIGHT = "light"
        const val DARK = "dark"
        val APPEARANCES = setOf(SYSTEM, LIGHT, DARK)
    }
}

/**
 * AppSettings keeps the app's settings in one JSON file of the app's own,
 * written at each change through a file beside it renamed over it; a file
 * it cannot read is the defaults.
 */
class AppSettings(private val file: File) {
    /** get is the settings as kept. */
    @Synchronized
    fun get(): Settings {
        val o = try {
            if (file.isFile) JSONObject(file.readText()) else JSONObject()
        } catch (e: JSONException) {
            JSONObject()
        } catch (e: IOException) {
            JSONObject()
        }
        return Settings(appearance = ServerStore.str(o, "appearance").takeIf { it in Settings.APPEARANCES } ?: Settings.SYSTEM)
    }

    /**
     * save keeps what [input] gives (`saveSettings`), the rest as it was,
     * and returns the settings as kept; an appearance that is none is
     * refused.
     */
    @Synchronized
    fun save(input: JSONObject): Settings {
        var s = get()
        if (input.has("appearance")) {
            val a = input.opt("appearance") as? String
            if (a == null || a !in Settings.APPEARANCES) throw BridgeException(R.string.err_bad_call)
            s = s.copy(appearance = a)
        }
        val tmp = File(file.path + ".new")
        tmp.writeText(s.toJson().toString())
        if (!tmp.renameTo(file)) throw IOException("cannot replace ${file.name}")
        return s
    }

    companion object {
        /**
         * nightMode is AppCompat's night mode for [appearance], which the app
         * gives itself (AppCompatDelegate.setDefaultNightMode): the WebView's
         * prefers-color-scheme follows the theme's android:isLightTheme, so
         * the connect page and every server's page set to "as the system"
         * take the app's appearance.
         */
        fun nightMode(appearance: String): Int = when (appearance) {
            Settings.LIGHT -> AppCompatDelegate.MODE_NIGHT_NO
            Settings.DARK -> AppCompatDelegate.MODE_NIGHT_YES
            else -> AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM
        }
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.appcompat.app.AppCompatDelegate
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.fail
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

/** The app's own settings, its appearance (docs/app.md, `settings` and `saveSettings`, AP MobileApp D53-A). */
class AppSettingsTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private fun file() = File(tmp.root, "settings.json")

    @Test
    fun asThePhoneUntilChosen() {
        assertEquals(Settings(Settings.SYSTEM), AppSettings(file()).get())
        assertEquals("system", AppSettings(file()).get().toJson().getString("appearance"))
    }

    @Test
    fun keptAcrossStarts() {
        val st = AppSettings(file())
        assertEquals(Settings.DARK, st.save(JSONObject().put("appearance", "dark")).appearance)
        assertEquals(Settings.DARK, AppSettings(file()).get().appearance)
        // a call without it keeps it
        assertEquals(Settings.DARK, st.save(JSONObject()).appearance)
        assertFalse(File(file().path + ".new").exists())
    }

    @Test
    fun anAppearanceThatIsNoneIsRefused() {
        val st = AppSettings(file())
        st.save(JSONObject().put("appearance", "light"))
        for (bad in listOf<Any>("sepia", 3, JSONObject.NULL)) {
            try {
                st.save(JSONObject().put("appearance", bad))
                fail("saved $bad")
            } catch (e: BridgeException) {
                assertEquals(R.string.err_bad_call, e.text)
            }
        }
        assertEquals(Settings.LIGHT, st.get().appearance)
    }

    @Test
    fun aFileItCannotReadIsTheDefaults() {
        file().writeText("{ not json")
        assertEquals(Settings.SYSTEM, AppSettings(file()).get().appearance)
        file().writeText("""{"appearance":"sepia"}""")
        assertEquals(Settings.SYSTEM, AppSettings(file()).get().appearance)
    }

    @Test
    fun theNightModeTheAppGivesItself() {
        assertEquals(AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM, AppSettings.nightMode(Settings.SYSTEM))
        assertEquals(AppCompatDelegate.MODE_NIGHT_NO, AppSettings.nightMode(Settings.LIGHT))
        assertEquals(AppCompatDelegate.MODE_NIGHT_YES, AppSettings.nightMode(Settings.DARK))
        assertEquals(AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM, AppSettings.nightMode("sepia"))
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.app.KeyguardManager
import android.content.Context
import android.os.SystemClock
import android.util.Log
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_STRONG
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_WEAK
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.uiautomator.By
import androidx.test.uiautomator.BySelector
import androidx.test.uiautomator.UiDevice
import androidx.test.uiautomator.Until
import org.junit.Assert.fail
import org.junit.AssumptionViolatedException
import org.junit.rules.TestRule
import org.junit.runner.Description
import org.junit.runners.model.Statement
import java.io.ByteArrayOutputStream
import java.util.regex.Pattern

/** Target is one of the servers the host runs for the emulator suite: its address as the emulator reaches it, through `adb reverse`, and its certificate's fingerprint. */
data class Target(val url: String, val fp: String) {
    /** origin is the address without its last slash, as the sign-in card shows it. */
    val origin: String get() = url.trimEnd('/')
}

/**
 * Rig is what the emulator suite (tools/app-emulator/README.md) is given
 * and reaches beyond the app: the two servers on the host, the user there
 * and the emulator's PIN, as the instrumentation's arguments, which
 * tools/app-emulator/run.sh passes; the system's prompt, through
 * UiAutomator; and the fingerprint, which only the emulator's console
 * touches: a test asks the host's helper (tools/app-emulator/fingers.sh)
 * for each touch with a line `touch <n>` in the log under [TAG], and the
 * helper answers with `adb emu finger touch`. No argument is ever logged,
 * and the audit's key, the owner's of the servers made for the run, is no
 * argument at all: run.sh writes it to a file on the emulator, which the
 * argument names, so that it is on no command line.
 */
object Rig {
    /** TAG is the suite's tag in the log: its steps, and the touches it asks the host for. */
    const val TAG = "MassimoEmu"

    /** SYSTEM_UI is the package whose window shows the biometric prompt. */
    const val SYSTEM_UI = "com.android.systemui"

    private val args get() = InstrumentationRegistry.getArguments()

    /** device is UiAutomator's view of the emulator. */
    val device: UiDevice get() = UiDevice.getInstance(InstrumentationRegistry.getInstrumentation())

    /** app is the app's own context, whose strings the prompts' titles are. */
    val app: Context get() = InstrumentationRegistry.getInstrumentation().targetContext

    /** given reports whether the suite was given its servers; without them (on a phone, by hand) its tests are skipped. */
    val given: Boolean get() = !args.getString("massimoUrlA").isNullOrEmpty()

    /** a and b are the two servers. */
    val a: Target get() = Target(arg("massimoUrlA"), arg("massimoFpA"))
    val b: Target get() = Target(arg("massimoUrlB"), arg("massimoFpB"))

    /** user and password sign in at either server. */
    val user: String get() = arg("massimoUser")
    val password: String get() = arg("massimoPassword")

    /**
     * auditKey is the servers' owner's key, which reads the audit, from the
     * file on the emulator that the argument massimoAuditKeyFile names
     * (run.sh writes it), read as the shell, whose file it is.
     */
    val auditKey: String by lazy {
        val file = arg("massimoAuditKeyFile")
        if (!KEY_FILE.matcher(file).matches()) throw AssertionError("massimoAuditKeyFile is not a file of /data/local/tmp/: tools/app-emulator/run.sh names it")
        device.executeShellCommand("cat $file").trim().takeIf { it.isNotEmpty() }
            ?: throw AssertionError("the owner's key is not in $file: tools/app-emulator/run.sh writes it there")
    }

    private val KEY_FILE = Pattern.compile("^/data/local/tmp/[A-Za-z0-9._-]+$")

    /** pin is the emulator's screen lock, which run.sh sets and a fingerprint needs. */
    val pin: String get() = arg("massimoPin")

    private fun arg(name: String): String =
        args.getString(name)?.takeIf { it.isNotEmpty() } ?: throw AssertionError("the instrumentation argument $name is missing: tools/app-emulator/run.sh passes it")

    /** say writes a step of the suite into the log, for a failure's reader. */
    fun say(text: String) {
        Log.i(TAG, text)
    }

    /**
     * touch asks the host's helper for one touch of the enrolled finger:
     * `touch <n>` under [TAG], n the time since the emulator's boot, so
     * that it rises across the suite's processes and the helper answers
     * each line once.
     */
    fun touch() {
        Log.i(TAG, "touch " + SystemClock.elapsedRealtimeNanos())
    }

    /** quiet takes away what the system shows over the app by itself: "isn't responding" waits, and the screen stays awake. */
    fun quiet() {
        device.wakeUp()
        device.findObject(By.res("android", "aerr_wait"))?.click()
    }

    /** bringBack shows the app again as its icon would, the one task brought to the front (singleTask). */
    fun bringBack() {
        device.executeShellCommand("am start -a android.intent.action.MAIN -c android.intent.category.LAUNCHER -n ${app.packageName}/${MainActivity::class.java.name}")
    }

    /** SHOTS is where a failed test's screen is kept on the emulator, which run.sh takes home. */
    const val SHOTS = "/data/local/tmp/massimo-emu"

    /**
     * keep writes what the screen shows for the failed test [name] into
     * the log, the window's hierarchy and the system's texts, and its
     * picture into [SHOTS].
     */
    fun keep(name: String) {
        runCatching {
            device.executeShellCommand("mkdir -p $SHOTS")
            device.executeShellCommand("screencap -p $SHOTS/$name.png")
        }
        runCatching {
            val out = ByteArrayOutputStream()
            device.dumpWindowHierarchy(out)
            chunks("$name: the window", out.toString("UTF-8"))
        }
        runCatching { chunks("$name: the system's window", Prompt.systemTexts().joinToString(" | ")) }
    }

    /** chunks writes [text] under [title] into the log, in lines the log takes whole. */
    fun chunks(title: String, text: String) {
        Log.i(TAG, title)
        text.chunked(CHUNK).forEach { Log.i(TAG, it) }
    }

    private const val CHUNK = 3000
}

/**
 * KeepOnFailure keeps what a failed test showed ([Rig.keep], then [more]
 * with the test's name), and runs [after] whatever the test came to; a
 * test skipped is no failure.
 */
class KeepOnFailure(private val more: (String) -> Unit = {}, private val after: () -> Unit = {}) : TestRule {
    override fun apply(base: Statement, d: Description): Statement = object : Statement() {
        override fun evaluate() {
            try {
                base.evaluate()
            } catch (e: AssumptionViolatedException) {
                throw e
            } catch (e: Throwable) {
                val name = "${d.testClass.simpleName}.${d.methodName}"
                Rig.keep(name)
                runCatching { more(name) }
                throw e
            } finally {
                runCatching { after() }
            }
        }
    }
}

/**
 * Prompt is the system's biometric prompt, found by its title, the app's
 * text: passed by a touch the host's helper gives, or cancelled by its
 * Cancel. Where the prompt takes the screen lock too (Android 11 on) it
 * has no Cancel, only "Use PIN", and Back cancels it, as a user's Back
 * does; before, the app gives it a Cancel button.
 */
object Prompt {
    private const val SHOWN_MS = 30_000L
    private const val GONE_MS = 10_000L
    private const val TOUCHES = 3

    // The sensor listens once the prompt has come up.
    private const val SETTLE_MS = 700L

    private fun titled(title: String): BySelector = By.pkg(Rig.SYSTEM_UI).text(title)

    /** shows waits for the prompt titled [title]. */
    fun shows(title: String) {
        Rig.say("waiting for the prompt \"$title\"")
        if (!Rig.device.wait(Until.hasObject(titled(title)), SHOWN_MS)) fail("no prompt \"$title\" within ${SHOWN_MS / 1000} s; the system shows: ${systemTexts()}")
    }

    /** pass waits for the prompt titled [title] and has the finger touch it, again where it stays. */
    fun pass(title: String) {
        shows(title)
        SystemClock.sleep(SETTLE_MS)
        repeat(TOUCHES) {
            Rig.touch()
            if (Rig.device.wait(Until.gone(titled(title)), GONE_MS)) return
        }
        fail("the prompt \"$title\" stayed through $TOUCHES touches: does the host's helper run (tools/app-emulator/fingers.sh)?")
    }

    /** cancel waits for the prompt titled [title] and cancels it. */
    fun cancel(title: String) {
        shows(title)
        SystemClock.sleep(SETTLE_MS)
        val button = Rig.device.findObject(By.pkg(Rig.SYSTEM_UI).clickable(true).text(Pattern.compile("(?i)cancel")))
        if (button != null) button.click() else Rig.device.pressBack()
        if (!Rig.device.wait(Until.gone(titled(title)), GONE_MS)) fail("the prompt \"$title\" stayed after its cancel")
    }

    /** none fails where a prompt of the app's shows within [ms]: "one prompt" is the one passed and none after it. */
    fun none(ms: Long) {
        val any = By.pkg(Rig.SYSTEM_UI).text(Pattern.compile(titles()))
        if (Rig.device.wait(Until.hasObject(any), ms)) fail("a prompt more: \"${Rig.device.findObject(any)?.text}\"")
    }

    /** login, keep, off, unlock and forgetAll are the app's titles of its prompts for the server named [name]. */
    fun login(name: String): String = Rig.app.getString(R.string.login_title, name)
    fun keep(name: String): String = Rig.app.getString(R.string.login_keep_title, name)
    fun off(name: String): String = Rig.app.getString(R.string.login_off_title, name)
    fun unlock(name: String): String = Rig.app.getString(R.string.unlock_title, name)
    fun forgetAll(): String = Rig.app.getString(R.string.login_forget_all_title)

    // titles matches every title of the app's prompts.
    private fun titles(): String {
        val named = listOf(R.string.login_title, R.string.login_keep_title, R.string.login_off_title, R.string.unlock_title)
            .joinToString("|") { Pattern.quote(Rig.app.getString(it, "\u0000")).replace("\u0000", "\\E.*\\Q") }
        return "^($named|${Pattern.quote(forgetAll())})$"
    }

    /** systemTexts are the texts the system's window shows, for a failure's message. */
    fun systemTexts(): List<String> = Rig.device.findObjects(By.pkg(Rig.SYSTEM_UI)).mapNotNull { it.text?.takeIf(String::isNotBlank) }.take(TEXTS)

    private const val TEXTS = 20
}

/**
 * Fingers is the emulator's one finger. [enroll] enrolls it where none is:
 * the system's Settings driven by UiAutomator through the enrollment, the
 * PIN typed where it asks for it, its Next, More and I agree pressed, each
 * touch of the sensor asked of the host's helper, until the phone has a
 * strong biometric. Once per emulator, before the suite: run.sh runs
 * [EnrollTest] alone first and stops where it fails, so that a finger not
 * enrolled is one failure and not a wait in every test; each test then
 * only [check]s.
 */
object Fingers {
    private const val SETTINGS = "com.android.settings"
    private const val ENROLL_MS = 240_000L
    private val NEXT = Pattern.compile("(?i)^(more|i agree|agree|next|start|continue|ok|got it)$")
    private val DONE = Pattern.compile("(?i)^done$")

    /** enrolled reports whether the phone has a strong biometric enrolled, which the login by fingerprint needs. */
    fun enrolled(): Boolean = can(BIOMETRIC_STRONG)

    private fun can(authenticators: Int): Boolean = BiometricManager.from(Rig.app).canAuthenticate(authenticators) == BiometricManager.BIOMETRIC_SUCCESS

    /** check fails at once where no finger is enrolled. */
    fun check() {
        if (!enrolled()) fail("no finger is enrolled: tools/app-emulator/run.sh enrolls one before the suite (EnrollTest), and stops where that fails")
    }

    /** enroll enrolls a finger where none is. */
    fun enroll() {
        if (enrolled()) {
            Rig.say("a finger is enrolled already")
            return
        }
        val keyguard = Rig.app.getSystemService(KeyguardManager::class.java)
        if (!keyguard.isDeviceSecure) fail("the emulator has no screen lock, which a fingerprint needs: tools/app-emulator/run.sh sets its PIN")
        Rig.say("enrolling a finger")
        val d = Rig.device
        Rig.say("enroll: am start: " + d.executeShellCommand("am start -W -a android.settings.FINGERPRINT_ENROLL").trim().replace('\n', ' '))
        val until = SystemClock.elapsedRealtime() + ENROLL_MS
        var pins = 0
        while (SystemClock.elapsedRealtime() < until) {
            if (enrolled()) {
                d.findObject(By.pkg(SETTINGS).clickable(true).text(DONE))?.click()
                d.pressHome()
                Rig.say("a finger enrolled")
                return
            }
            if (can(BIOMETRIC_WEAK)) fail("the finger enrolled is a weak biometric on this image, and the login by fingerprint takes a strong one")
            val field = d.findObject(By.pkg(SETTINGS).clazz("android.widget.EditText"))
            if (field != null) {
                // three tries at most, far from the lock's own limit
                if (++pins > PIN_TRIES) fail("Settings asked for the PIN $PIN_TRIES times: is it the one run.sh set?")
                Rig.say("enroll: the PIN")
                field.click()
                field.text = Rig.pin
                d.pressEnter()
                SystemClock.sleep(STEP_MS)
                continue
            }
            val next = d.findObject(By.pkg(SETTINGS).clickable(true).text(NEXT))
            if (next != null) {
                Rig.say("enroll: ${next.text}")
                next.click()
                SystemClock.sleep(STEP_MS)
                continue
            }
            Rig.touch()
            SystemClock.sleep(STEP_MS)
        }
        fail("no finger enrolled within ${ENROLL_MS / 1000} s; Settings shows: ${d.findObjects(By.pkg(SETTINGS)).mapNotNull { it.text?.takeIf(String::isNotBlank) }.take(TEXTS)}")
    }

    private const val STEP_MS = 1500L
    private const val PIN_TRIES = 3
    private const val TEXTS = 20
}

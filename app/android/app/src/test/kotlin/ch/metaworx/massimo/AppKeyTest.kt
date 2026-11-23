// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.security.SecureRandom
import javax.crypto.AEADBadTagException
import javax.crypto.IllegalBlockSizeException

/**
 * The app key (AP MobileApp v1.4, Block 17, D63-A) over a fake KeySource:
 * one Keystore key for the app, one prompt to keep a login, one to open
 * one, one for a step that opens three, none more in a step once it was
 * dismissed; the app key dropped, its bytes cleared, after each step and as
 * the activity goes; a login opening for its own server alone; a login kept
 * before the app key asking with its own key until kept anew; a key the
 * phone took back (a bad tag, the key invalidated) forgetting every login
 * under it, each through the hook that leaves its key by its id, and any
 * other failure of the Keystore forgetting nothing; two keeps at once making
 * one app key; the app's Keystore key no server's alias. The real Keystore
 * and the prompt are the emulator suite's (Block 18).
 */
class AppKeyTest {
    @get:Rule
    val tmp = TemporaryFolder()

    private val keys = JvmKeys()
    private val prompt = FakePrompt(keys)
    private val forgotten = mutableListOf<String>() // "<n> <key id>", through the hook
    private val made = mutableListOf<ByteArray>() // the app keys made, as the random filled them
    private lateinit var logins: LoginStore
    private lateinit var appKey: AppKey

    private val random = object : SecureRandom() {
        override fun nextBytes(bytes: ByteArray) {
            super.nextBytes(bytes)
            if (bytes.size == 32) made.add(bytes)
        }
    }

    @Before
    fun start() {
        logins = LoginStore(File(tmp.root, "logins.json"))
        appKey = AppKey(logins, keys, prompt, { id, kept ->
            forgotten.add("${id.toLong()} ${kept.keyId}")
            logins.forget(id)
        }, random)
    }

    private val app = LoginKey.APP

    // id is the id of the server n, in a server's form; its key's id is 40 + n.
    private fun id(n: Int) = n.toString().padStart(24, '0')

    private fun secret(n: Int) = LoginSecret("key-of-$n", 40L + n)

    private fun keep(step: AppKey.Step, n: Int): AppKey.Result {
        var got: AppKey.Result? = null
        step.keep("keep $n", id(n), "node-$n", "", "anna", secret(n)) { got = it }
        return got ?: throw AssertionError("keep $n did not answer")
    }

    private fun keepOnce(n: Int): AppKey.Result = appKey.step().use { keep(it, n) }

    private fun open(step: AppKey.Step, n: Int, kept: KeptLogin = logins.get(id(n))!!, asId: String = id(n)): AppKey.Result {
        var got: AppKey.Result? = null
        step.open("open $n", asId, kept) { got = it }
        return got ?: throw AssertionError("open $n did not answer")
    }

    private fun openOnce(n: Int): AppKey.Result {
        var got: AppKey.Result? = null
        appKey.open("open $n", id(n), logins.get(id(n))!!) { got = it }
        return got ?: throw AssertionError("open $n did not answer")
    }

    // keptBefore keeps a login as the app did before the app key: sealed by
    // a Keystore key of its own.
    private fun keptBefore(n: Int) =
        logins.put(id(n), LoginStore.seal(keys.encrypter(LoginKey.alias(id(n))), secret(n), id(n), "node-$n", "", "anna"))

    private fun ids(): Set<String> = logins.ids().toSet()

    @Test
    fun oneKeyForTheAppAndOnePromptForEachStep() {
        assertEquals(AppKey.Result.Kept, keepOnce(1))
        assertEquals("the first keep makes the app key", listOf("keep 1 $app"), prompt.shown)
        val sealed = logins.appKey()
        assertNotNull(sealed)
        assertTrue(logins.get(id(1))!!.byApp)
        assertFalse(appKey.held)

        assertEquals(AppKey.Result.Kept, keepOnce(2))
        assertEquals("the next opens it", listOf("keep 1 $app", "keep 2 $app"), prompt.shown)
        assertEquals("one key for the app", setOf(app), keys.aliases())
        assertEquals("the app key kept as it was", sealed, logins.appKey())
        assertFalse(appKey.held)

        assertEquals(AppKey.Result.Opened(secret(1)), openOnce(1))
        assertEquals(AppKey.Result.Opened(secret(2)), openOnce(2))
        assertEquals(listOf("keep 1 $app", "keep 2 $app", "open 1 $app", "open 2 $app"), prompt.shown)
        assertFalse(appKey.held)
        assertTrue(forgotten.isEmpty())
        // the key never in the file
        assertFalse(File(tmp.root, "logins.json").readText().contains("key-of-"))
    }

    @Test
    fun aStepOpensEveryLoginUnderTheAppKeyAfterOnePrompt() {
        appKey.step().use { step -> listOf(1, 2, 3).forEach { assertEquals(AppKey.Result.Kept, keep(step, it)) } }
        assertEquals("one prompt keeps three in one step", 1, prompt.shown.size)
        prompt.shown.clear()
        val step = appKey.step()
        for (n in listOf(1, 2, 3)) assertEquals(AppKey.Result.Opened(secret(n)), open(step, n))
        assertEquals("Forget all's three servers, one prompt", listOf("open 1 $app"), prompt.shown)
        assertTrue("held while the step runs", appKey.held)
        step.close()
        assertFalse("dropped at its end", appKey.held)
        assertEquals("a step ended opens nothing more", AppKey.Result.Dismissed, open(step, 1))
        assertEquals(1, prompt.shown.size)
    }

    @Test
    fun aDismissedPromptForTheAppKeyAsksNoMoreInTheStep() {
        appKey.step().use { step -> listOf(1, 2, 3).forEach { keep(step, it) } }
        keptBefore(7)
        prompt.shown.clear()
        prompt.dismiss = true
        val step = appKey.step()
        assertEquals(AppKey.Result.Dismissed, open(step, 1))
        prompt.dismiss = false
        assertEquals("not asked again in the step", AppKey.Result.Dismissed, open(step, 2))
        assertEquals(AppKey.Result.Dismissed, open(step, 3))
        assertEquals("a login kept before still asks with its own", AppKey.Result.Opened(secret(7)), open(step, 7))
        step.close()
        assertEquals(listOf("open 1 $app", "open 7 ${LoginKey.alias(id(7))}"), prompt.shown)
        assertTrue("nothing forgotten", forgotten.isEmpty())
        assertEquals(setOf(id(1), id(2), id(3), id(7)), ids())
        assertEquals("the next step asks again", AppKey.Result.Opened(secret(2)), openOnce(2))
    }

    @Test
    fun aLoginOpensForItsOwnServerAlone() {
        keepOnce(1)
        keepOnce(2)
        prompt.shown.clear()
        val one = logins.get(id(1))!!
        val step = appKey.step()
        assertEquals("as another server", AppKey.Result.Gone, open(step, 1, one, asId = id(2)))
        assertEquals("another node", AppKey.Result.Gone, open(step, 1, one.copy(node = "node-other")))
        assertEquals("another fingerprint", AppKey.Result.Gone, open(step, 1, one.copy(fp = Fingerprint.of(byteArrayOf(9)))))
        assertEquals("another user", AppKey.Result.Gone, open(step, 1, one.copy(user = "bob")))
        assertEquals("2's login under its own id", AppKey.Result.Opened(secret(2)), open(step, 2))
        assertEquals(AppKey.Result.Opened(secret(1)), open(step, 1))
        step.close()
        assertEquals(1, prompt.shown.size)
        assertTrue("a login that does not open is not the app key taken back", forgotten.isEmpty())
        assertEquals(setOf(id(1), id(2)), ids())
    }

    @Test
    fun aLoginKeptBeforeAsksWithItsOwnKeyUntilKeptAnew() {
        keepOnce(1)
        keptBefore(7)
        keptBefore(8)
        assertFalse(logins.get(id(7))!!.byApp)
        prompt.shown.clear()
        val step = appKey.step()
        for (n in listOf(1, 7, 8)) assertEquals(AppKey.Result.Opened(secret(n)), open(step, n))
        step.close()
        assertEquals(
            "once for the app key, once for each login kept before it",
            listOf("open 1 $app", "open 7 ${LoginKey.alias(id(7))}", "open 8 ${LoginKey.alias(id(8))}"),
            prompt.shown,
        )

        // Kept anew: under the app key, and its own key gone.
        prompt.shown.clear()
        assertEquals(AppKey.Result.Kept, keepOnce(7))
        assertEquals(listOf("keep 7 $app"), prompt.shown)
        assertTrue(logins.get(id(7))!!.byApp)
        assertEquals(setOf(app, LoginKey.alias(id(8))), keys.aliases())
        assertEquals(AppKey.Result.Opened(secret(7)), openOnce(7))

        // Its own key taken back: that login alone is gone.
        keys.takeBack(LoginKey.alias(id(8)))
        assertEquals(AppKey.Result.Gone, openOnce(8))
        assertTrue(forgotten.isEmpty())
        assertEquals(AppKey.Result.Opened(secret(1)), openOnce(1))
    }

    @Test
    fun theKeyTakenBackForgetsEveryLoginUnderItLeavingEachKey() {
        appKey.step().use { step -> listOf(1, 2, 3).forEach { keep(step, it) } }
        keptBefore(7)
        prompt.shown.clear()
        keys.takeBack(app)
        val step = appKey.step()
        assertEquals(AppKey.Result.Gone, open(step, 1))
        assertEquals("the next in the step without a prompt", AppKey.Result.Gone, open(step, 2))
        assertTrue("forgotten at the step's end", forgotten.isEmpty())
        assertEquals("a login kept before asks with its own", AppKey.Result.Opened(secret(7)), open(step, 7))
        step.close()
        assertEquals(listOf("open 7 ${LoginKey.alias(id(7))}"), prompt.shown)
        assertEquals("every login under it, each with its key's id", listOf("1 41", "2 42", "3 43"), forgotten.sorted())
        assertEquals(setOf(id(7)), ids())
        assertNull(logins.appKey())

        // The next keep makes a new app key, behind one prompt.
        prompt.shown.clear()
        assertEquals(AppKey.Result.Kept, keepOnce(4))
        assertEquals(listOf("keep 4 $app"), prompt.shown)
        assertEquals(AppKey.Result.Opened(secret(4)), openOnce(4))
    }

    @Test
    fun aBadTagOrAnInvalidatedKeyAtTheFinishIsTheKeyTakenBack() {
        keepOnce(1)
        keepOnce(2)
        prompt.swap = { failing(AEADBadTagException("Tag mismatch")) }
        assertEquals(AppKey.Result.Gone, openOnce(1))
        assertEquals(listOf("1 41", "2 42"), forgotten.sorted())
        assertNull(logins.appKey())
        assertTrue(ids().isEmpty())

        // The key invalidated, as the cause of a finish's failure.
        prompt.swap = null
        forgotten.clear()
        keepOnce(3)
        prompt.swap = { failing(IllegalBlockSizeException("finish failed").apply { initCause(KeyInvalidated()) }) }
        assertEquals(AppKey.Result.Gone, openOnce(3))
        assertEquals(listOf("3 43"), forgotten)
    }

    @Test
    fun anyOtherFailureAtTheFinishForgetsNothing() {
        keepOnce(1)
        keepOnce(2)
        keptBefore(7)
        val sealed = logins.appKey()
        val failure = IllegalBlockSizeException("Key user not authenticated")
        prompt.swap = { failing(failure) }
        assertEquals("opening one under the app key", AppKey.Result.Failed(failure), openOnce(1))
        assertEquals("keeping one", AppKey.Result.Failed(failure), keepOnce(3))
        assertEquals("opening one kept before", AppKey.Result.Failed(failure), openOnce(7))
        assertTrue("nothing forgotten", forgotten.isEmpty())
        assertEquals(setOf(id(1), id(2), id(7)), ids())
        assertEquals("the app key as it was", sealed, logins.appKey())
        prompt.swap = null
        assertEquals(AppKey.Result.Opened(secret(1)), openOnce(1))
        assertEquals(AppKey.Result.Opened(secret(7)), openOnce(7))
    }

    @Test
    fun aKeepFindingTheKeyTakenBackForgetsTheLoginsUnderItFirst() {
        keepOnce(1)
        keepOnce(2)
        val old = logins.appKey()
        prompt.shown.clear()
        keys.takeBack(app)
        assertEquals(AppKey.Result.Kept, keepOnce(3))
        assertEquals(listOf("1 41", "2 42"), forgotten.sorted())
        assertEquals(listOf("keep 3 $app"), prompt.shown)
        assertNotNull(logins.appKey())
        assertNotEquals("a new app key", old, logins.appKey())
        assertEquals(setOf(id(3)), ids())
        assertEquals(AppKey.Result.Opened(secret(3)), openOnce(3))
    }

    @Test
    fun twoKeepsAtOnceMakeOneAppKey() {
        prompt.hold = true
        val first = appKey.step()
        val second = appKey.step()
        var one: AppKey.Result? = null
        var two: AppKey.Result? = null
        first.keep("keep 1", id(1), "node-1", "", "anna", secret(1)) { one = it }
        second.keep("keep 2", id(2), "node-2", "", "anna", secret(2)) { two = it }
        assertEquals("the second waits for nothing, as a second prompt would not show", AppKey.Result.Dismissed, two)
        while (prompt.open > 0) prompt.answer()
        first.close()
        second.close()
        prompt.hold = false
        assertEquals(AppKey.Result.Kept, one)
        assertEquals(AppKey.Result.Opened(secret(1)), openOnce(1))
        assertEquals(AppKey.Result.Kept, keepOnce(2))
        assertEquals(AppKey.Result.Opened(secret(1)), openOnce(1))
        assertEquals(AppKey.Result.Opened(secret(2)), openOnce(2))
        assertEquals(setOf(app), keys.aliases())
    }

    @Test
    fun theAppsKeyIsNoServersAlias() {
        assertThrows(IllegalArgumentException::class.java) { LoginKey.alias(LoginStore.APP) }
        assertThrows(IllegalArgumentException::class.java) { LoginKey.alias("") }
        assertThrows(IllegalArgumentException::class.java) { LoginKey.alias("0123456789ABCDEF01234567") }
        assertEquals("massimo.login.${id(1)}", LoginKey.alias(id(1)))
        assertNotEquals(app, LoginKey.alias(id(1)))
    }

    @Test
    fun theAppKeyIsDroppedAfterEachStepItsBytesCleared() {
        assertEquals(AppKey.Result.Kept, keepOnce(1))
        assertEquals(1, made.size)
        assertTrue("the key made, cleared at the keep's end", made[0].all { it == 0.toByte() })
        assertFalse(appKey.held)
        assertEquals(AppKey.Result.Opened(secret(1)), openOnce(1))
        assertFalse(appKey.held)

        // The activity gone: every step's key dropped, and what the step
        // still asks finds it ended.
        val step = appKey.step()
        assertEquals(AppKey.Result.Opened(secret(1)), open(step, 1))
        assertTrue(appKey.held)
        appKey.dropAll()
        assertFalse(appKey.held)
        assertEquals(AppKey.Result.Dismissed, open(step, 1))
        step.close()
    }

    @Test
    fun aPromptAnsweredAfterTheStepEndedKeepsNothing() {
        keepOnce(1)
        prompt.hold = true
        val step = appKey.step()
        var got: AppKey.Result? = null
        step.open("open 1", id(1), logins.get(id(1))!!) { got = it }
        assertNull("the prompt shows", got)
        appKey.dropAll()
        prompt.answer()
        assertEquals(AppKey.Result.Dismissed, got)
        assertFalse(appKey.held)

        // A keep's prompt alike: nothing kept.
        val keeping = appKey.step()
        var kept: AppKey.Result? = null
        keeping.keep("keep 2", id(2), "node-2", "", "anna", secret(2)) { kept = it }
        appKey.dropAll()
        prompt.answer()
        assertEquals(AppKey.Result.Dismissed, kept)
        assertNull(logins.get(id(2)))
        assertFalse(appKey.held)
    }

    @Test
    fun aDismissedPromptChangesNothing() {
        prompt.dismiss = true
        assertEquals(AppKey.Result.Dismissed, keepOnce(1))
        assertNull(logins.get(id(1)))
        assertNull("no app key kept", logins.appKey())
        prompt.dismiss = false
        assertEquals(AppKey.Result.Kept, keepOnce(1))
        val sealed = logins.appKey()
        prompt.dismiss = true
        assertEquals(AppKey.Result.Dismissed, keepOnce(2))
        assertNull(logins.get(id(2)))
        assertEquals(AppKey.Result.Dismissed, openOnce(1))
        assertEquals("the app key and the login as they were", sealed, logins.appKey())
        assertNotNull(logins.get(id(1)))
        assertTrue(forgotten.isEmpty())
        assertFalse(appKey.held)
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.io.IOException
import java.util.Base64
import javax.crypto.Cipher

/**
 * A login kept for a server (docs/app.md, "The login by fingerprint"): the
 * user's name, the server's node and pinned fingerprint as they were when it
 * was kept, in clear, and the API key sealed with its [iv]: under the app
 * key ([AppKey]) where [byApp], else, kept before it, by the server's own
 * Keystore key ([LoginKey.alias]). The server's id and the three are bound
 * into the seal, so that it opens for nothing else.
 * [keyId] is the key's row at the server, in clear beside it, so that Turn
 * off revokes it with the session the app holds, without the prompt; 0 for
 * a login kept before contract version 3. [code] is the key's revoke code
 * (AP MobileApp, D112-A), in clear as the id is, since it lets nothing in:
 * a key Turn off, Forget all or Remove leave is ended by it at the next
 * contact; "" for a login kept before contract version 6, until its next
 * unseal.
 */
data class KeptLogin(
    val user: String,
    val node: String,
    val fp: String,
    val iv: ByteArray,
    val sealed: ByteArray,
    val keyId: Long = 0,
    val byApp: Boolean = false,
    val code: String = "",
) {
    override fun equals(other: Any?): Boolean =
        other is KeptLogin && user == other.user && node == other.node && fp == other.fp && keyId == other.keyId &&
            byApp == other.byApp && code == other.code && iv.contentEquals(other.iv) && sealed.contentEquals(other.sealed)

    override fun hashCode(): Int = listOf(user, node, fp, keyId, byApp, code, iv.contentHashCode(), sealed.contentHashCode()).hashCode()

    override fun toString(): String = "KeptLogin(user=$user, node=$node, keyId=$keyId, byApp=$byApp)"
}

/** SealedKey is the app key as the Keystore key [LoginKey.APP] sealed it, with its [iv]; never the key itself. */
data class SealedKey(val iv: ByteArray, val sealed: ByteArray) {
    override fun equals(other: Any?): Boolean = other is SealedKey && iv.contentEquals(other.iv) && sealed.contentEquals(other.sealed)

    override fun hashCode(): Int = 31 * iv.contentHashCode() + sealed.contentHashCode()

    override fun toString(): String = "SealedKey"
}

/** The key a login keeps, opened: its secret, and its row at the server, which a Turn off revokes. */
data class LoginSecret(val key: String, val id: Long) {
    override fun toString(): String = "LoginSecret(id=$id)" // never the key
}

/**
 * LoginStore keeps one login per server, as a JSON object of one file of the
 * app's own by the server's id, and beside them, under [APP], the app key
 * sealed ([AppKey]); what opens them is the Keystore's, so a copy of the
 * file opens nothing.
 */
class LoginStore(private val file: File) {
    /** get is the login kept for the server [id], or null. */
    @Synchronized
    fun get(id: String): KeptLogin? {
        if (id == APP) return null
        val o = read().optJSONObject(id) ?: return null
        return try {
            KeptLogin(
                user = ServerStore.str(o, "user"),
                node = ServerStore.str(o, "node"),
                fp = ServerStore.str(o, "fp"),
                iv = Base64.getDecoder().decode(ServerStore.str(o, "iv")),
                sealed = Base64.getDecoder().decode(ServerStore.str(o, "sealed")),
                keyId = o.optLong("key_id"),
                byApp = o.optBoolean("app"),
                code = ServerStore.str(o, "code").takeIf(RevokeCode::valid) ?: "",
            ).takeIf { it.user.isNotEmpty() && it.iv.isNotEmpty() && it.sealed.isNotEmpty() }
        } catch (e: IllegalArgumentException) {
            null
        }
    }

    /** user is the user whose login is kept for the server [id], "" for none. */
    fun user(id: String): String = get(id)?.user ?: ""

    /** put keeps [login] for the server [id], in place of one kept before. */
    @Synchronized
    fun put(id: String, login: KeptLogin) {
        require(id != APP) { "not a server's id" }
        val all = read()
        val enc = Base64.getEncoder()
        val o = JSONObject()
            .put("user", login.user)
            .put("node", login.node)
            .put("fp", login.fp)
            .put("iv", enc.encodeToString(login.iv))
            .put("sealed", enc.encodeToString(login.sealed))
            .put("key_id", login.keyId)
        if (login.byApp) o.put("app", true)
        if (login.code.isNotEmpty()) o.put("code", login.code)
        all.put(id, o)
        write(all)
    }

    /** appKey is the app key, sealed, or null where none is kept. */
    @Synchronized
    fun appKey(): SealedKey? {
        val o = read().optJSONObject(APP) ?: return null
        return try {
            SealedKey(
                Base64.getDecoder().decode(ServerStore.str(o, "iv")),
                Base64.getDecoder().decode(ServerStore.str(o, "sealed")),
            ).takeIf { it.iv.isNotEmpty() && it.sealed.isNotEmpty() }
        } catch (e: IllegalArgumentException) {
            null
        }
    }

    /** putAppKey keeps [key], the app key sealed, in place of one kept before. */
    @Synchronized
    fun putAppKey(key: SealedKey) {
        val all = read()
        val enc = Base64.getEncoder()
        all.put(APP, JSONObject().put("iv", enc.encodeToString(key.iv)).put("sealed", enc.encodeToString(key.sealed)))
        write(all)
    }

    /** forgetAppKey forgets the app key sealed; one there is not is no error. */
    @Synchronized
    fun forgetAppKey() {
        val all = read()
        if (all.remove(APP) != null) write(all)
    }

    /**
     * setKeyId gives the login kept for the server [id] the id [keyId] of its
     * key, where it is still [was], kept without one, so that a login kept
     * or forgotten meanwhile is left as it is; it reports whether the login
     * has that id now, a check meanwhile having given it the same one too.
     */
    @Synchronized
    fun setKeyId(id: String, was: KeptLogin, keyId: Long): Boolean {
        if (keyId <= 0) return false
        val now = get(id) ?: return false
        if (now == was.copy(keyId = keyId)) return true
        if (now != was || now.keyId > 0) return false
        put(id, now.copy(keyId = keyId))
        return true
    }

    /**
     * setCode gives the login kept for the server [id] the revoke code
     * [code] of its key, where it is still [was], kept without one: a login
     * kept before contract version 6, at its next unseal (D112-A); one kept
     * or forgotten meanwhile is left as it is.
     */
    @Synchronized
    fun setCode(id: String, was: KeptLogin, code: String): Boolean {
        if (!RevokeCode.valid(code)) return false
        val now = get(id) ?: return false
        if (now != was || now.code.isNotEmpty()) return now == was.copy(code = code)
        put(id, now.copy(code = code))
        return true
    }

    /** forget forgets the login of the server [id], and reports whether there was one. */
    @Synchronized
    fun forget(id: String): Boolean {
        if (id == APP) return false
        val all = read()
        if (all.remove(id) == null) return false
        write(all)
        return true
    }

    /** ids are the servers with a kept login. */
    @Synchronized
    fun ids(): List<String> = read().keys().asSequence().filter { it != APP }.toList()

    private fun read(): JSONObject = try {
        if (file.isFile) JSONObject(file.readText()) else JSONObject()
    } catch (e: JSONException) {
        JSONObject()
    } catch (e: IOException) {
        JSONObject()
    }

    private fun write(all: JSONObject) {
        val tmp = File(file.path + ".new")
        tmp.writeText(all.toString())
        if (!tmp.renameTo(file)) throw IOException("cannot replace ${file.name}")
    }

    companion object {
        /** APP is the name the app key is kept under in the file: no server's id, which is 24 hex digits. */
        const val APP = "app"

        /** APP_AAD binds the sealed app key to what it is. */
        val APP_AAD: ByteArray = "massimo.login.app.v1".toByteArray(Charsets.UTF_8)

        /** aad binds a sealed key to the server [id], its [node] and pinned [fp], and the [user]. */
        fun aad(id: String, node: String, fp: String, user: String): ByteArray =
            listOf("massimo.login.v1", id, node, Fingerprint.canonical(fp) ?: "", user).joinToString("\u0000").toByteArray(Charsets.UTF_8)

        /**
         * seal seals [secret] with [cipher], bound to the server and the
         * user, and returns the login to keep: a cipher of the app key
         * ([AppKey]) where [byApp], else a key's encrypter the prompt passed.
         */
        fun seal(cipher: Cipher, secret: LoginSecret, id: String, node: String, fp: String, user: String, byApp: Boolean = false): KeptLogin {
            cipher.updateAAD(aad(id, node, fp, user))
            val plain = JSONObject().put("key", secret.key).put("id", secret.id).toString().toByteArray(Charsets.UTF_8)
            val sealed = try {
                cipher.doFinal(plain)
            } finally {
                plain.fill(0)
            }
            return KeptLogin(user, node, fp, cipher.iv, sealed, secret.id, byApp, RevokeCode.of(secret.key))
        }

        /**
         * open opens [login] with [cipher], of the app key or the key's
         * decrypter the prompt passed, for the server [id]; it throws where
         * the server's node or fingerprint, or the user, is not the one it
         * was kept for.
         */
        fun open(cipher: Cipher, login: KeptLogin, id: String): LoginSecret {
            cipher.updateAAD(aad(id, login.node, login.fp, login.user))
            val o = JSONObject(String(cipher.doFinal(login.sealed), Charsets.UTF_8))
            val key = ServerStore.str(o, "key")
            if (key.isEmpty()) throw JSONException("no key")
            return LoginSecret(key, o.optLong("id"))
        }
    }
}

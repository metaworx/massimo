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
 * was kept, in clear, and the API key sealed by the server's [LoginKey] with
 * its [iv]. The four are bound into the seal, so that it opens for nothing else.
 */
data class KeptLogin(val user: String, val node: String, val fp: String, val iv: ByteArray, val sealed: ByteArray) {
    override fun equals(other: Any?): Boolean =
        other is KeptLogin && user == other.user && node == other.node && fp == other.fp && iv.contentEquals(other.iv) && sealed.contentEquals(other.sealed)

    override fun hashCode(): Int = listOf(user, node, fp, iv.contentHashCode(), sealed.contentHashCode()).hashCode()
}

/** The key a login keeps, opened: its secret, and its row at the server, which a Turn off revokes. */
data class LoginSecret(val key: String, val id: Long) {
    override fun toString(): String = "LoginSecret(id=$id)" // never the key
}

/**
 * LoginStore keeps one login per server, as a JSON object of one file of the
 * app's own by the server's id; what opens it is the Keystore's, so a copy of
 * the file opens nothing.
 */
class LoginStore(private val file: File) {
    /** get is the login kept for the server [id], or null. */
    @Synchronized
    fun get(id: String): KeptLogin? {
        val o = read().optJSONObject(id) ?: return null
        return try {
            KeptLogin(
                user = ServerStore.str(o, "user"),
                node = ServerStore.str(o, "node"),
                fp = ServerStore.str(o, "fp"),
                iv = Base64.getDecoder().decode(ServerStore.str(o, "iv")),
                sealed = Base64.getDecoder().decode(ServerStore.str(o, "sealed")),
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
        val all = read()
        val enc = Base64.getEncoder()
        all.put(
            id,
            JSONObject()
                .put("user", login.user)
                .put("node", login.node)
                .put("fp", login.fp)
                .put("iv", enc.encodeToString(login.iv))
                .put("sealed", enc.encodeToString(login.sealed)),
        )
        write(all)
    }

    /** forget forgets the login of the server [id], and reports whether there was one. */
    @Synchronized
    fun forget(id: String): Boolean {
        val all = read()
        if (all.remove(id) == null) return false
        write(all)
        return true
    }

    /** ids are the servers with a kept login. */
    @Synchronized
    fun ids(): List<String> = read().keys().asSequence().toList()

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
        /** aad binds a sealed key to the server [id], its [node] and pinned [fp], and the [user]. */
        fun aad(id: String, node: String, fp: String, user: String): ByteArray =
            listOf("massimo.login.v1", id, node, Fingerprint.canonical(fp) ?: "", user).joinToString("\u0000").toByteArray(Charsets.UTF_8)

        /**
         * seal seals [secret] with [cipher], a key's encrypter the prompt
         * passed, bound to the server and the user, and returns the login
         * to keep.
         */
        fun seal(cipher: Cipher, secret: LoginSecret, id: String, node: String, fp: String, user: String): KeptLogin {
            cipher.updateAAD(aad(id, node, fp, user))
            val plain = JSONObject().put("key", secret.key).put("id", secret.id).toString().toByteArray(Charsets.UTF_8)
            val sealed = cipher.doFinal(plain)
            return KeptLogin(user, node, fp, cipher.iv, sealed)
        }

        /**
         * open opens [login] with [cipher], the key's decrypter the prompt
         * passed, for the server [id]; it throws where the server's node or
         * fingerprint, or the user, is not the one it was kept for.
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

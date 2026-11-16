// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.io.IOException
import java.security.GeneralSecurityException
import java.util.Base64

/** Sealer encrypts and decrypts, binding each sealed value to [aad]. */
interface Sealer {
    fun seal(plain: ByteArray, aad: ByteArray): ByteArray
    fun open(sealed: ByteArray, aad: ByteArray): ByteArray
}

/**
 * TokenStore keeps a token per server, sealed, as a JSON object of one file
 * of the app's own: the server's id and the sealed token in Base64, the id
 * bound into the seal so that a token moved to another server does not
 * open.
 */
class TokenStore(private val file: File, private val sealer: Sealer) {
    /** get is the server's token, or null where there is none or it no longer opens (and is dropped). */
    @Synchronized
    fun get(id: String): String? {
        val all = read()
        val sealed = all.opt(id) as? String ?: return null
        return try {
            String(sealer.open(Base64.getDecoder().decode(sealed), id.toByteArray()), Charsets.UTF_8)
        } catch (e: GeneralSecurityException) {
            drop(all, id)
            null
        } catch (e: IllegalArgumentException) {
            drop(all, id)
            null
        }
    }

    /** put keeps [token] for the server, or forgets it for null. */
    @Synchronized
    fun put(id: String, token: String?) {
        val all = read()
        if (token == null) {
            if (all.remove(id) != null) write(all)
            return
        }
        val sealed = try {
            sealer.seal(token.toByteArray(Charsets.UTF_8), id.toByteArray())
        } catch (e: GeneralSecurityException) {
            throw BridgeException(R.string.err_storage, e.message ?: e.javaClass.simpleName)
        }
        all.put(id, Base64.getEncoder().encodeToString(sealed))
        write(all)
    }

    @Synchronized
    fun remove(id: String) = put(id, null)

    private fun drop(all: JSONObject, id: String) {
        all.remove(id)
        write(all)
    }

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
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import androidx.annotation.StringRes

/** Texts gives the app's strings, in the phone's language. */
fun interface Texts {
    fun get(@StringRes id: Int, vararg args: Any): String
}

/**
 * BridgeException is a refusal the page shows as it is (docs/app.md:
 * "A failed call rejects with an Error whose message the page shows"),
 * named by one of the app's strings.
 */
class BridgeException(@param:StringRes @get:StringRes val text: Int, vararg val args: Any) : Exception() {
    fun message(texts: Texts): String = texts.get(text, *args)
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package ch.metaworx.massimo

import android.app.Application
import java.io.File

/**
 * MassimoApp is the app's process (AP MobileApp, D102-A): it holds what
 * lives with the process rather than with an activity, the servers kept,
 * the sessions kept in memory ([keeper]) and what was checked of them
 * ([checks]), so that an activity made anew, by the phone's dark mode or
 * language or the app's Appearance, keeps them (the manifest's
 * configChanges stays as it is: AppCompat's night mode needs the new
 * activity). At the process's start, before any activity, each server's
 * session recorded as held that the app no longer holds, the memory of the
 * process before gone with it, moves into the server's `unended`, so that
 * the next session the app holds there for that user ends it; a sealed
 * session, still held, stays.
 */
class MassimoApp : Application() {
    /** servers are the servers the app keeps, one store for the process. */
    lateinit var servers: ServerStore
        private set

    /** keeper keeps the sessions held, in memory or sealed. */
    lateinit var keeper: SessionKeeper
        private set

    /** checks are what the checks found of the sessions held. */
    val checks = Checks()

    private lateinit var logins: LoginStore

    override fun onCreate() {
        super.onCreate()
        servers = ServerStore(File(filesDir, "servers.json"))
        val tokens = TokenStore(File(filesDir, "tokens.json"), KeystoreSealer())
        // Version 1's switch kept a session behind a prompt the app asks no more.
        servers.takeLegacy().forEach { tokens.remove(it) }
        keeper = SessionKeeper(tokens)
        logins = LoginStore(File(filesDir, "logins.json"))
        runCatching { keeper.leaveForgotten(servers, logins) }
    }

    /**
     * exit drops the sessions kept in memory as the process's end does, each
     * one recorded as held moving into its server's `unended` at once: the
     * list's Exit (D91-A), which then ends the task.
     */
    fun exit() {
        keeper.dropMemory()
        checks.clear()
        runCatching { keeper.leaveForgotten(servers, logins) }
    }
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/kinds.ts
 *
 * The log's fixed lists and sizes (AP UiParts, Block 11, from log.js):
 * the kinds of event the server sends, which TestLogKinds compares with
 * the server's, the sources, the levels, the ranges of how far back, and
 * how many lines a page asks for and the view holds.
 */

/** LOG_MAX_LINES is how many lines live events may grow the log to before the oldest go, beyond the pages Older fetched. */
export const LOG_MAX_LINES = 2000;

/** LOG_PAGE is how many events a page asks for; a shorter page is the log's beginning. */
export const LOG_PAGE = 200;

// LOG_KINDS are the kinds of event the server sends, in its order
// (eventKinds in internal/server/log.go, which TestLogKinds compares).
export const LOG_KINDS = ["connected", "disconnected", "hello", "metadata", "config", "state", "pairing_started", "paired", "pairing_ended",
  "device_forgotten", "cube_found", "restore_step", "restore_done", "disagree", "drift",
  "cube_gone", "address_changed", "server_started", "server_stopping", "settings_read", "restart", "reboot", "reboot_cancelled",
  "update", "update_result", "update_postponed", "backup", "backup_deleted", "backups_pruned", "log_pruned", "log_rotated", "restore_started", "open_setup",
  "change", "command", "hold", "structure", "unconfirmed", "battery", "error_reset", "error_watch", "slots", "mismatch", "login", "login_failed", "locked_out", "audit_pruned",
  "proxy_connected", "proxy_disconnected", "proxy_refused", "proxy_discarded", "proxy_deleted", "proxy_delete_refused", "proxy_pairing", "proxy_warning"];

/**
 * LOG_RANGES are the choices of how far back the log goes: the newest
 * LOG_COUNT events, the last 10 minutes, the last hour, today, this week
 * (from Monday), this month, or all; LOG_COUNT_RANGE by default.
 */
export const LOG_RANGES = ["20", "10m", "1h", "today", "week", "month", "all"];
export const LOG_COUNT_RANGE = "20";
export const LOG_COUNT = 20;

/** LOG_POLL_MS is how often the log asks for new lines under All Cubes, where the stream, which is the header's Cube's, brings none (D9-A). */
export const LOG_POLL_MS = 10000;

/**
 * LOG_POLL_PAGES is how many full pages a poll reads on before it loads
 * the log again instead (pollLog): a backlog of more, as after a laptop's
 * sleep with the log open, is drawn as the log's newest page, once, in
 * place of a walk through every line since.
 */
export const LOG_POLL_PAGES = LOG_MAX_LINES / LOG_PAGE;

/** LOG_SOURCES are the sources of an event, as the server names them, in the order of the source choice. */
export const LOG_SOURCES = ["system", "config", "cube", "device", "queue", "server", "proxy"];

/** LOG_KIND_SOURCE groups the kinds by the source they come from, for the kind choice; a kind of more than one (config) goes with the devices. */
export const LOG_KIND_SOURCE: Record<string, string> = {
  state: "device", config: "device", paired: "device", device_forgotten: "device", battery: "device", error_reset: "device", error_watch: "device",
  unconfirmed: "device",
  connected: "cube", disconnected: "cube", hello: "cube", metadata: "cube", pairing_started: "cube", pairing_ended: "cube",
  cube_found: "cube", cube_gone: "cube", address_changed: "cube",
  disagree: "cube", drift: "device",
  change: "config",
  command: "queue", hold: "queue",
  login: "server", login_failed: "server", locked_out: "server",
  proxy_connected: "proxy", proxy_disconnected: "proxy", proxy_refused: "proxy", proxy_discarded: "proxy",
  proxy_deleted: "proxy", proxy_delete_refused: "proxy", proxy_pairing: "proxy", proxy_warning: "proxy",
};

/** LOG_LEVELS are the levels of an event, lowest first, as the server names them. */
export const LOG_LEVELS = ["debug", "info", "warn", "error"];

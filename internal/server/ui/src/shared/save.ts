// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/shared/save.ts
 *
 * A file the browser saves: an answer of the API (apiRaw of app.js) under
 * the name its Content-Disposition gives, a Cube's backup or the
 * server's, and a text the page holds, a key made for the server's
 * backups; downloadBlob of components.js saves it.
 */

/** saveResponse saves an answer as a file, named as the server says, else fallback. */
export async function saveResponse(resp: Response, fallback: string): Promise<void> {
  const m = /filename="?([^";]+)"?/.exec(resp.headers.get("Content-Disposition") || "");
  downloadBlob(m ? m[1].trim() : fallback, await resp.blob());
}

/** saveText saves a text as a file of the name given, plain text in UTF-8. */
export function saveText(name: string, text: string): void {
  downloadBlob(name, new Blob([text], { type: "text/plain;charset=utf-8" }));
}

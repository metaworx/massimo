// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/Tls.tsx
 *
 * The line under the header about the certificate the server serves, from
 * GET /healthz's tls: while it is self-signed, to compare the fingerprint
 * the browser shows for it with massimo tls show on the box before
 * signing in; a warning while it ends within two weeks, or while the
 * server has none yet, computed here so that it speaks the page's
 * language. The page shows no fingerprint: it comes over the very
 * connection in question, and a man in the middle would pass the server's
 * along, or rewrite it. The sign-in dialog's line stays plain (header.js
 * renderTLS).
 */

import type { CertStatus } from "../api";
import { Notice } from "../shared/badge";
import { cert, load } from "./store";

/** tlsTexts are the line's sentences for a certificate, none where all is well. */
function tlsTexts(tls: CertStatus): string[] {
  const texts: string[] = [];
  if (tls.self_signed) texts.push(t("header.selfSigned"));
  const until = Date.parse(tls.not_after || "");
  if (isNaN(until)) {
    if (tls.warning) texts.push(t("header.certMissing"));
  } else {
    const days = Math.floor((until - Date.now()) / 86400000);
    if (days < 0) texts.push(t("header.certExpired"));
    else if (days < 14) texts.push(tn("header.certExpires", days));
  }
  return texts;
}

/** TlsNotice is the line, a shared Notice, kept in its place and hidden where it has nothing to say. */
export function TlsNotice() {
  load.useLoad();
  const texts = cert.value ? tlsTexts(cert.value) : [];
  return (
    <Notice id="tls-notice" role="status" hidden={!texts.length}>
      <span id="tls-notice-text">{texts.join(" ")}</span>
    </Notice>
  );
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/signin/Server.tsx
 *
 * The server above the sign-in card's title, in the app alone (AP
 * MobileApp, F31, D62-A; Block 20), in place of the address bar the app
 * has not: the server's name, once where it is its address, and the full
 * address the page was opened at with a labelled ⓘ after it, the two in
 * one element that does not break between them, the ⓘ showing the
 * server's instance id and its pinned certificate's fingerprint, each
 * copied with one tap or by its copy icon, an empty one left out, and no
 * ⓘ where both are. A line of its own before the title, not inside it
 * (test_keep_open.py reads the title's text), and above the first owner's
 * steps, so that the sign-in, the first owner's form and the confirmation
 * that keeps the server open name it alike. Read from the app's status
 * (shared/app.ts, asked again at each opening of the card), its ⓘ closed
 * again at each opening. Nothing in a browser, whose address bar names
 * the server.
 */

import { useLayoutEffect, useState } from "preact/hooks";
import { app, serverOf } from "../shared/app";
import { hid } from "../shared/cls";
import { Copy } from "../shared/copy";
import { IconButton } from "../shared/icon";
import { useLang } from "../shared/lang";
import { card } from "./store";

/** ServerLine is the server's line above the card's title, with its ⓘ and the facts it shows; nothing in a browser. */
export function ServerLine() {
  useLang();
  const opened = card.opened;
  const srv = serverOf(app.value);
  const [open, setOpen] = useState(false);
  // the facts closed again at each opening of the card
  // biome-ignore lint/correctness/useExhaustiveDependencies: opened is read from the card's store, which Biome takes for a stable import; the effect is for each opening, so it is listed.
  useLayoutEffect(() => {
    setOpen(false);
  }, [opened]);
  if (!srv) return null;
  const facts = ([["login-server-node", t("login.serverNode"), srv.node], ["login-server-fp", t("login.serverFp"), srv.fp]] as const).filter(([, , v]) => v);
  return (
    <div id="login-server" class="login-server">
      <p class="login-server-line">
        {srv.name && <span id="login-server-name" class="login-server-name">{srv.name}</span>}
        {/* the address and its ⓘ, one element: a line break comes before it or inside the address, never before the ⓘ */}
        <span class="login-server-where">
          <span id="login-server-address" class="login-server-address mono small">{srv.address}</span>
          {facts.length > 0 && (
            <IconButton symbol="icon-info" id="login-server-more" cls="login-server-more" label={t("login.serverMore")}
              aria-expanded={open ? "true" : "false"} aria-controls="login-server-facts" onClick={() => setOpen(!open)} />
          )}
        </span>
      </p>
      {facts.length > 0 && (
        <dl id="login-server-facts" class={"login-server-facts" + hid(!open)}>
          {facts.map(([id, label, value]) => [
            <dt key={id + ":t"}>{label}</dt>,
            <dd key={id + ":d"}><Copy id={id} text={value} label={label} cls="mono small" icon /></dd>,
          ])}
        </dl>
      )}
    </div>
  );
}

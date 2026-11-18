// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/Times.tsx
 *
 * The reads' times, from the header's one clock (clock.ts): in the pile's
 * list under Refresh now, two columns, when the chosen Cube was read and
 * when the server last replied, each how long ago with its time of day as
 * its tooltip, and the current time, the times right-aligned; and in
 * expert mode the same two reads in the radio budget's line, in
 * #header-times.
 */

import { useRef } from "preact/hooks";
import { clockOf, fmtAgo, lastReads, now } from "./clock";
import { load } from "./store";

/**
 * ListTimes are Refresh now's second line, three rows of a label and its
 * time; they move on with the header's clock while the list is open
 * (open), and stand still while it is closed, so that a closed list is
 * not drawn again each second.
 */
export function ListTimes({ open }: { open: boolean }) {
  load.useLoad();
  const last = useRef(now.peek());
  if (open) last.current = now.value;
  const at = last.current;
  const r = lastReads();
  return (
    <span class="read-times">
      <span>{t("header.cubeRead")}</span>
      <span title={clockOf(r.cube)}>{fmtAgo(r.cube, at)}</span>
      <span>{t("header.serverReplied")}</span>
      <span title={clockOf(r.server)}>{fmtAgo(r.server, at)}</span>
      <span>{t("header.timeNow")}</span>
      <span>{clockOf(at)}</span>
    </span>
  );
}

/** ExpertTimes are the two reads in expert mode's radio budget line, each a fact of the line. */
export function ExpertTimes() {
  load.useLoad();
  const at = now.value;
  const r = lastReads();
  return (
    <>
      <span class="stat read-time cube-read" title={clockOf(r.cube)}>
        <span>{t("header.cubeRead")}</span> <span>{fmtAgo(r.cube, at)}</span>
      </span>
      <span class="stat read-time server-replied" title={clockOf(r.server)}>
        <span>{t("header.serverReplied")}</span> <span>{fmtAgo(r.server, at)}</span>
      </span>
    </>
  );
}

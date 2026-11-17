// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/header/Updated.tsx
 *
 * How long ago the chosen Cube was heard from, above the header's pile
 * (Pile.tsx): its updated, the last message the server took from it, on
 * the server's clock (serverSkew of header.js), counting on by itself,
 * each second under a minute, then each minute, each hour. The time itself
 * is in the pile's list, under Refresh now.
 */

import { useEffect, useState } from "preact/hooks";
import { chosenCube } from "../model/cube";
import { load } from "./store";

/**
 * fmtSince is a time span (ms), briefly: seconds under a minute, then
 * minutes, hours, from two days on days ("8 sec", "12 min", "5 hr").
 */
function fmtSince(ms: number): string {
  const sec = Math.max(0, Math.floor(ms / 1000));
  if (sec < 60) return fmtUnit(sec, "second");
  if (sec < 3600) return fmtUnit(Math.floor(sec / 60), "minute");
  if (sec < 48 * 3600) return fmtUnit(Math.floor(sec / 3600), "hour");
  return fmtUnit(Math.floor(sec / 86400), "day");
}

/** Updated is the chosen Cube's age, nothing where it has no such time; drawn again when the shown age's last unit has passed. */
export function Updated() {
  load.useLoad();
  const [, tick] = useState(0);
  const cube = chosenCube();
  const at = cube ? Date.parse(cube.updated || "") : NaN;
  const age = isNaN(at) ? 0 : Math.max(0, Date.now() + serverSkew - at);
  useEffect(() => {
    if (isNaN(at)) return undefined;
    const step = age < 60000 ? 1000 : age < 3600000 ? 60000 : 3600000;
    const timer = setTimeout(() => tick((n) => n + 1), step - (age % step) + 20);
    return () => clearTimeout(timer);
  });
  return <>{isNaN(at) ? "" : fmtSince(age)}</>;
}

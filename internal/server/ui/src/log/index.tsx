// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

/**
 * @file internal/server/ui/src/log/index.tsx
 *
 * The log as a part (AP UiParts, Block 11): its page drawn once into its
 * section (#logview) at app.js' init and updated in place; its address
 * of its own handed to the pages' store (ownAddress), which reads it at a
 * reload, a link, back and forward and tells the log when it is shown;
 * the log's unseen problems handed to ☰ (alerts.ts). The plain code calls
 * MassimoParts.log (app.js' stream, poll, loads and language), and the
 * UI tests read its state and its alerts there.
 */

import { effect, untracked } from "@preact/signals";
import { minuteNow } from "../shared/data";
import { ownAddress } from "../shared/pages";
import { mounter, register } from "../shared/region";
import { logRangeStart, upperFirst } from "./text";
import { forgetBlocks } from "./blocks";
import { forgetLogAlerts, logAlerts, loadLogAlerts, renderLogSign } from "./alerts";
import { logHash, readLogHash } from "./address";
import { LOG_KINDS } from "./kinds";
import type { LogEvent, LogLine } from "./lines";
import { Log } from "./Log";
import { logState } from "./state";
import {
  appendLogEvent, fillLogSelects, focusHead, forgetLog, loadLog, logChoicesChanged, logCubeChanged, logCubesPolled, logLineMatches, logLoadUsers, logOfChoices,
  logPollSet, pollLog, pruneLogRange, renderLog,
} from "./store";

export { serverEvent } from "./store";

/** signedOut forgets the log of the user who was signed in, but its choices, which are its address (MassimoParts.signedOut, index.ts; UiParts D209). */
export function signedOut(): void {
  forgetLog();
  forgetBlocks();
  forgetLogAlerts();
}

const mount = mounter("logview", () => <Log />);

// Each minute of the page the lines a time range no longer holds go.
effect(() => {
  if (minuteNow.value) untracked(() => pruneLogRange());
});

// The log's address of its own: read from an address of the log, written
// for it (a device the Cube does not have goes before the address takes
// it), told whether the log is shown, then that it was opened.
ownAddress("log", {
  read: readLogHash,
  write: () => {
    fillLogSelects();
    return logHash();
  },
  shown: (on) => {
    logState.open = on;
    logPollSet();
  },
  opened: () => {
    loadLog();
    logLoadUsers();
  },
});

/** The functions the plain code calls, MassimoParts.log; the UI tests read state, alerts, matches, ofChoices, rangeStart and upperFirst. */
const log = register("log", {
  /** state is what the log keeps (log/state.ts). */
  state: logState,
  /** alerts are the log's unseen problems (log/alerts.ts; the UI tests). */
  alerts: logAlerts,
  /** kinds are the kinds of event the server sends (app.js connectEvents). */
  kinds: LOG_KINDS,
  /** mount draws the log's page into its section (app.js init). */
  mount(): void {
    mount();
  },
  /** focusHead gives the log's title the focus in the drawing that follows (the first owner's toast's Show, signin/Owner.tsx). */
  focusHead,
  /** event adds a live event of the stream (app.js connectEvents). */
  event(ev: LogEvent): void {
    appendLogEvent(ev);
  },
  render: renderLog,
  load: loadLog,
  poll: pollLog,
  prune: pruneLogRange,
  cubesPolled: logCubesPolled,
  cubeChanged: logCubeChanged,
  selects: fillLogSelects,
  choicesChanged: logChoicesChanged,
  loadAlerts: loadLogAlerts,
  sign: renderLogSign,
  matches: (line: LogLine) => logLineMatches(line),
  ofChoices: (line: LogLine) => logOfChoices(line),
  rangeStart: logRangeStart,
  upperFirst,
});

/** MassimoParts.log, as index.ts declares it. */
export type LogPart = typeof log;

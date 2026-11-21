// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the log's plain names: the log is a part
// (ui/src/log/, AP UiParts' Block 11), its lines, its choices, its
// address and its unseen problems; these are the names the other plain
// scripts (app.js' stream, poll, loads and language, tiles.js) and the
// UI tests call, each a call of MassimoParts.log, and the page globals
// logState and logAlerts, the part's state.
"use strict";

// logState is what the log keeps (ui/src/log/state.ts): app.js reads
// whether it is shown and clears every Cube's rooms at a sign-out.
Object.defineProperty(window, "logState", { get: () => MassimoParts.log.state });

// logAlerts are the log's unseen problems (ui/src/log/alerts.ts).
Object.defineProperty(window, "logAlerts", { get: () => MassimoParts.log.alerts });

// LOG_KINDS are the kinds of event the server sends (ui/src/log/kinds.ts),
// for which app.js listens on the stream.
Object.defineProperty(window, "LOG_KINDS", { get: () => MassimoParts.log.kinds });

// initLog draws the log's page into its section.
function initLog() {
  MassimoParts.log.mount();
}

// appendLogEvent adds a live event of the stream (app.js connectEvents).
function appendLogEvent(ev) {
  MassimoParts.log.event(ev);
}

// renderLog draws every line again: the rooms came, the language changed.
function renderLog() {
  MassimoParts.log.render();
}

// loadLog loads the log's newest page again.
function loadLog() {
  return MassimoParts.log.load();
}

// pollLog asks under All Cubes for the lines after the newest.
function pollLog() {
  return MassimoParts.log.poll();
}

// pruneLogRange drops the lines a time range no longer holds (app.js'
// minute).
function pruneLogRange(now) {
  MassimoParts.log.prune(now);
}

// logCubesPolled follows the Cubes a poll found (app.js loadCubes).
function logCubesPolled() {
  MassimoParts.log.cubesPolled();
}

// logCubeChanged starts the log over for another Cube (app.js
// selectCube).
function logCubeChanged() {
  MassimoParts.log.cubeChanged();
}

// fillLogSelects puts the choices right against the devices loaded and
// draws them; true where a choice changed so (app.js loadRooms).
function fillLogSelects() {
  return MassimoParts.log.selects();
}

// logChoicesChanged loads the log again for its choices (app.js
// loadRooms).
function logChoicesChanged() {
  MassimoParts.log.choicesChanged();
}

// loadLogAlerts reads the log's unseen problems (app.js' poll and
// refresh).
function loadLogAlerts() {
  return MassimoParts.log.loadAlerts();
}

// renderLogSign hands ☰ the log's sign again (app.js applyLanguage).
function renderLogSign() {
  MassimoParts.log.sign();
}

// logLineMatches reports whether a line is drawn (the UI tests).
function logLineMatches(line) {
  return MassimoParts.log.matches(line);
}

// logOfChoices reports whether a line is of the source and the user
// chosen (the UI tests).
function logOfChoices(line) {
  return MassimoParts.log.ofChoices(line);
}

// logRangeStart is where a time range starts (the UI tests).
function logRangeStart(range, now) {
  return MassimoParts.log.rangeStart(range, now);
}

// upperFirst is a text with its first letter in upper case
// (ui/src/log/text.ts; the UI tests).
function upperFirst(s) {
  return MassimoParts.log.upperFirst(s);
}

// rfLabel names a device by its radio address as the page names it
// (ui/src/model/device.ts; tiles.js mismatchArgs).
function rfLabel(rf) {
  return MassimoParts.log.rfLabel(rf);
}

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, the pages: the plain names of the pages' store
// (ui/src/shared/pages.ts, MassimoParts.pages), which shows a page, keeps
// it in the address and each page's scroll, and says who may open which.
// The log hands the store its address itself (ui/src/log/).
"use strict";

// showAddress shows the page the address names, at the start (app.js
// init) and on back and forward.
function showAddress() {
  MassimoParts.pages.showAddress();
}

// mayShow reports whether the signed-in user may open a page (login.js
// applyCaps).
function mayShow(view) {
  return MassimoParts.pages.mayShow(view);
}

// isShown reports whether a page is the one shown, at this moment.
function isShown(view) {
  return MassimoParts.pages.isShown(view);
}

// showView shows a page, Rooms in place of one the user may not open,
// and puts it into the address: a new entry unless address is "replace";
// card is a card of the personal page to bring into view.
function showView(view, address, card) {
  MassimoParts.pages.showView(view, address, card);
}

// currentView is the page shown, which login.js and the UI tests read.
Object.defineProperty(window, "currentView", { get: () => MassimoParts.pages.shown() });

// initPages wires back and forward, and a page named in the address bar.
function initPages() {
  window.addEventListener("popstate", () => showAddress());
}

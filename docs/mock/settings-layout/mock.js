// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The mock's switches, read from the address so that every state has a
// URL the screenshots can open:
//   page=system|prefs|profile|names   the page shown (system)
//   theme=light|dark                  data-theme, as My settings → Mode sets it
//   menu=user|cube|sections           the 👤 menu, the Cube choice or the
//                                     sections menu open
//   scroll=<section id>               the page scrolled to it (sec-users)
//   cube=2                            the Cube without a name chosen, not
//                                     connected
//   tip=user                          the 👤 button's tooltip, drawn
//   as=manager                        System settings → Users seen by a manager
//   fold=all|cubes,users,…            sections folded at the start
//   open=store-info,config-info       Maintenance's details open
//   shot=1                            without the mock's bar
// Folding, the table of contents and back to top work as proposed, with
// the same calls the implementation would use; no innerHTML.
"use strict";

(function () {
  const q = new URLSearchParams(location.search);
  const $ = (id) => document.getElementById(id);
  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;

  const theme = q.get("theme");
  if (theme === "light" || theme === "dark") document.documentElement.dataset.theme = theme;
  if (q.get("shot")) document.body.classList.add("shot");
  document.body.classList.toggle("as-manager", q.get("as") === "manager");

  const page = q.get("page") || "system";
  for (const el of document.querySelectorAll("[data-page]")) el.classList.toggle("hidden", el.dataset.page !== page);
  const cog = $("view-settings");
  cog.classList.toggle("active", page === "system");
  if (page === "system") cog.setAttribute("aria-current", "page");

  if (q.get("menu") === "user") $("user-menu").open = true;
  if (q.get("menu") === "cube") $("cube-menu").open = true;
  if (q.get("tip") === "user") $("user-tip").classList.remove("hidden");
  // open=store-info,config-info opens those details
  for (const id of (q.get("open") || "").split(",")) if (id && $(id)) $(id).open = true;

  // A section folds with the button in its heading; its body is hidden,
  // and its badges still say what it holds.
  function setFolded(btn, folded) {
    btn.setAttribute("aria-expanded", String(!folded));
    $(btn.getAttribute("aria-controls")).classList.toggle("hidden", folded);
    btn.closest(".sys-section").classList.toggle("folded", folded);
  }
  const fold = (q.get("fold") || "").split(",");
  for (const btn of document.querySelectorAll(".sys-fold")) {
    btn.addEventListener("click", () => setFolded(btn, btn.getAttribute("aria-expanded") === "true"));
    const name = btn.closest(".sys-section").id.replace("sec-", "");
    if (fold.includes("all") || fold.includes(name)) setFolded(btn, true);
  }

  // The table of contents scrolls to a section and opens it; the address
  // keeps #system, which is the page.
  for (const link of document.querySelectorAll(".toc-link")) {
    link.addEventListener("click", () => {
      const sec = $(link.dataset.target);
      const btn = sec.querySelector(".sys-fold");
      if (btn.getAttribute("aria-expanded") === "false") setFolded(btn, false);
      sec.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "start" });
      btn.focus({ preventScroll: true });
    });
  }

  // Back to top, once the table of contents is out of sight.
  const toTop = $("to-top");
  const toc = document.querySelector(".toc");
  if (page === "system") {
    new IntersectionObserver(([e]) => toTop.classList.toggle("hidden", e.isIntersecting)).observe(toc);
  }
  toTop.addEventListener("click", () => {
    window.scrollTo({ top: 0, behavior: reduced ? "auto" : "smooth" });
    toc.querySelector(".toc-link").focus({ preventScroll: true });
  });

  // The menu items that open a page of the mock.
  for (const b of document.querySelectorAll("[data-go]")) {
    b.addEventListener("click", () => {
      const p = new URLSearchParams(location.search);
      p.set("page", b.dataset.go);
      p.delete("menu");
      location.search = p.toString();
    });
  }

  // Escape and a click outside close the menus, as the page's do.
  const menus = [$("user-menu"), $("cube-menu")];
  document.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape") for (const m of menus) m.open = false;
  });
  document.addEventListener("click", (ev) => {
    for (const m of menus) if (m.open && !m.contains(ev.target)) m.open = false;
  });

  // The bar's links keep the other parameters: data-set="theme=dark".
  for (const a of document.querySelectorAll("a[data-set]")) {
    const p = new URLSearchParams(location.search);
    for (const pair of a.dataset.set.split("&")) {
      const [k, v] = pair.split("=");
      if (v) p.set(k, v);
      else p.delete(k);
    }
    a.href = "?" + p.toString();
  }

  // ---- The header and the page's row at the top (D2, D12, D13) ----
  // The sections menu: choosing a section closes it.
  const secMenu = $("sec-menu");
  menus.push(secMenu);
  for (const b of secMenu.querySelectorAll(".toc-link")) b.addEventListener("click", () => { secMenu.open = false; });
  if (q.get("menu") === "sections") secMenu.open = true;

  // The header and the page's row stay at the top: the row sits under
  // the header, whose height changes as it compacts once scrolled.
  const top = document.querySelector(".top");
  const bar = document.querySelector(".page-bar");
  const sizes = () => {
    document.documentElement.style.setProperty("--top-h", top.offsetHeight + "px");
    document.documentElement.style.setProperty("--bar-h", bar.offsetHeight + "px");
  };
  new ResizeObserver(sizes).observe(top);
  sizes();
  const onScroll = () => document.body.classList.toggle("scrolled", window.scrollY > 24);
  addEventListener("scroll", onScroll, { passive: true });
  onScroll();

  // The section in view is marked in the menu: the last one, in the
  // page's order, whose top has passed under the header and the row.
  const marks = new Map();
  for (const b of secMenu.querySelectorAll(".toc-link")) marks.set(b.dataset.target, b);
  const markCurrent = () => {
    const line = top.offsetHeight + bar.offsetHeight + 24;
    // the lowest top that has passed; side by side, the first of them
    let best = null;
    let bestTop = -Infinity;
    for (const id of marks.keys()) {
      const y = Math.round($(id).getBoundingClientRect().top);
      if (y <= line && y > bestTop) {
        best = id;
        bestTop = y;
      }
    }
    for (const [id, b] of marks) {
      if (id === best) b.setAttribute("aria-current", "true");
      else b.removeAttribute("aria-current");
    }
  };
  addEventListener("scroll", markCurrent, { passive: true });
  secMenu.addEventListener("toggle", markCurrent);

  // cube=2: the Cube without a name chosen, which is not connected.
  if (q.get("cube") === "2") {
    $("cube-name").textContent = "KEQ0000101";
    $("cube-conn").classList.replace("ok", "bad");
    $("cube-conn").querySelector("use").setAttribute("href", "#icon-plug-off");
    $("cube-summary").title = "KEQ0000101 · 192.0.2.11 · not connected";
    $("cube-summary").setAttribute("aria-label", "Cube: KEQ0000101, not connected");
    $("cube-opt-1").removeAttribute("aria-current");
    $("cube-opt-2").setAttribute("aria-current", "true");
  }

  // scroll=<section id>: the page scrolled to that section, as a user would.
  const to = q.get("scroll");
  if (to && $(to)) {
    window.scrollTo(0, 60); // first scrolled: the header compacts
    requestAnimationFrame(() => requestAnimationFrame(() => {
      document.body.classList.add("scrolled"); // measured compacted
      const y = $(to).getBoundingClientRect().top + window.scrollY - (top.offsetHeight + bar.offsetHeight + 10);
      window.scrollTo(0, y);
    }));
  }
})();

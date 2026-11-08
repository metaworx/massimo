// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, components: the pieces the panels are built from,
// each a function that returns an element or fills one. Texts arrive
// translated (t() in i18n.js) and go in through textContent only; a
// component never builds markup from a string. Loaded after i18n.js and
// before app.js; the helpers that read the page's state (ageEl,
// helpParts, fmtTemp) are in app.js.
"use strict";

// clear removes every child of an element.
function clear(el) {
  while (el.firstChild) el.removeChild(el.firstChild);
}

// textEl is an element of a tag with a text and a class.
function textEl(tag, text, cls) {
  const el = document.createElement(tag);
  el.textContent = text;
  if (cls) el.className = cls;
  return el;
}

// elem is an element of a tag with a class and children (elements, or
// strings, which become text).
function elem(tag, cls, ...children) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  e.append(...children);
  return e;
}

// buttonOpts reads the options of button and iconButton: a string is the
// class; an object gives cls, aria (the accessible name, when the text
// alone does not say it) and title (the tooltip).
function buttonOpts(b, opts) {
  const o = typeof opts === "string" ? { cls: opts } : opts || {};
  if (o.cls) b.className = o.cls;
  if (o.aria) b.setAttribute("aria-label", o.aria);
  if (o.title) b.title = o.title;
}

// button is a type=button with a text, its options (see buttonOpts) and
// a click handler.
function button(label, opts, onClick) {
  const b = document.createElement("button");
  b.type = "button";
  b.textContent = label;
  buttonOpts(b, opts);
  if (onClick) b.addEventListener("click", onClick);
  return b;
}

// submitButton is a form's primary submit button.
function submitButton(label) {
  const b = document.createElement("button");
  b.type = "submit";
  b.className = "primary";
  b.textContent = label;
  return b;
}

// busyWas keeps, per button made busy, whether it was off before.
const busyWas = new WeakMap();

// setBusy marks a button as sending, or ends that: while on, the button
// is off, shows a spinner after its text (the class busy, style.css)
// and is aria-busy; at the end it is as off as it was before. A button
// that sends goes busy from its click until the answer, so that a
// second click or Enter sends nothing (whileBusy). No button does
// nothing.
function setBusy(btn, on) {
  if (!btn || !!on === isBusy(btn)) return;
  if (on) {
    busyWas.set(btn, btn.disabled);
    btn.disabled = true;
    btn.classList.add("busy");
    btn.setAttribute("aria-busy", "true");
    return;
  }
  btn.classList.remove("busy");
  btn.removeAttribute("aria-busy");
  btn.disabled = !!busyWas.get(btn);
  busyWas.delete(btn);
}

// isBusy reports whether a button's request is still running.
function isBusy(btn) {
  return !!btn && btn.classList.contains("busy");
}

// setOff turns a button off or on; a busy one keeps turning, and is off
// or on as set here once its request has answered.
function setOff(btn, off) {
  if (!btn) return;
  if (isBusy(btn)) busyWas.set(btn, !!off);
  else btn.disabled = !!off;
}

// whileBusy runs fn, which sends a request, with btn busy (setBusy) and
// returns what fn returns; while the button is busy it runs nothing and
// returns undefined.
async function whileBusy(btn, fn) {
  if (isBusy(btn)) return undefined;
  setBusy(btn, true);
  try {
    return await fn();
  } finally {
    setBusy(btn, false);
  }
}

// sendWhenChanged keeps a form's submit button off while changed()
// reports nothing to send: at once, at every input and change in the
// form, and when the returned function is called, for a change the
// form's events do not see. With lazy, the form's values reach what
// changed() reads only at their change event (numberInput), so a
// keystroke turns the button on and the change sets it again; a click
// on it right after typing then finds it on. A busy button keeps
// turning, and is set for when its request has answered (setOff).
function sendWhenChanged(form, changed, { btn, lazy } = {}) {
  const b = btn || form.querySelector("button[type=submit]");
  const check = () => setOff(b, !changed());
  form.addEventListener("input", lazy ? () => setOff(b, false) : check);
  form.addEventListener("change", check);
  check();
  return check;
}

// badgeEl is a small outlined or filled label (.flag) of a kind: ok,
// warn, bad, mode, fill-ok, fill-warn, fill-bad.
function badgeEl(text, kind) {
  return textEl("span", text, "flag " + kind);
}

// iconEl returns a small inline icon referencing a symbol of the page.
function iconEl(symbol, cls) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", cls);
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", "#" + symbol);
  svg.appendChild(use);
  return svg;
}

// iconButton is a button that shows an icon of the page only, so its
// name is aria; opts as for button, iconCls the icon's class ("ni").
function iconButton(symbol, aria, onClick, opts) {
  const o = Object.assign({}, typeof opts === "string" ? { cls: opts } : opts, { aria });
  const b = button("", o, onClick);
  b.appendChild(iconEl(symbol, o.iconCls || "ni"));
  return b;
}

// switchEl is a slide switch: a checkbox styled as a switch, with its
// label; name, when given, is the checkbox's name in its form.
function switchEl(label, checked, onChange, name) {
  const sw = document.createElement("label");
  sw.className = "switch";
  const cb = document.createElement("input");
  cb.type = "checkbox";
  cb.checked = !!checked;
  if (name) cb.name = name;
  if (onChange) cb.addEventListener("change", () => onChange(cb.checked));
  const slider = textEl("span", "", "slider");
  slider.setAttribute("aria-hidden", "true");
  sw.append(cb, slider, textEl("span", label, "switch-label"));
  return sw;
}

// formEl is a form that calls onSubmit instead of leaving the page.
function formEl(onSubmit, cls) {
  const form = document.createElement("form");
  if (cls) form.className = cls;
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    onSubmit(ev);
  });
  return form;
}

// formFooter is a form's row of actions: the extra buttons (resets,
// Remove) at its left end, Cancel and the submit button at its right;
// submit is the submit button's label, cancel the handler of Cancel,
// whose label cancelLabel replaces (Close). Each part is optional.
function formFooter({ submit, cancel, cancelLabel, extra } = {}) {
  const actions = document.createElement("div");
  actions.className = "actions";
  if (extra && extra.length) {
    const left = document.createElement("span");
    left.className = "actions-left";
    left.append(...extra);
    actions.appendChild(left);
  }
  const right = document.createElement("span");
  right.className = "actions-right";
  if (cancel) right.appendChild(button(cancelLabel || t("action.cancel"), "", cancel));
  if (submit) right.appendChild(submitButton(submit));
  actions.appendChild(right);
  return actions;
}

// fieldRow is a row of a label and its control (.field): the label at
// the left, the control at the right on the same line, wrapping under
// the label where there is no room; wide puts the control on a line of
// its own at full width, for text inputs and long selects. unit follows
// the control; help is the key of a help button and its explanation
// (helpParts); hint is a line of small text under the row; id is the
// control's id, for the label, else it gets one. Without a label the
// control labels itself (a switch), and the help button follows it.
let fieldSeq = 0;
function fieldRow(label, control, { unit, help, hint, id, wide } = {}) {
  const row = document.createElement("div");
  row.className = "field" + (wide ? " wide" : "");
  if (label) {
    const lab = textEl("label", label);
    const target = control.matches("input, select, textarea") ? control : control.querySelector("input, select, textarea");
    if (target) {
      if (id) target.id = id;
      else if (!target.id) target.id = "field-" + ++fieldSeq;
      lab.htmlFor = target.id;
    }
    row.appendChild(lab);
  }
  let text = null;
  if (help) {
    const h = helpParts(help, label || control.textContent);
    if (label) row.appendChild(h.btn);
    text = h;
  }
  let box = control;
  if (unit) {
    box = document.createElement("span");
    box.className = "inline";
    box.append(control, textEl("span", unit, "muted"));
  }
  if (label) box.classList.add("field-control");
  row.appendChild(box);
  if (text && !label) row.appendChild(text.btn);
  if (text) row.appendChild(text.text);
  if (hint) row.appendChild(textEl("p", hint, "muted small field-hint"));
  return row;
}

// fillOptions fills a select (or a datalist) with options, each
// [value, label, disabled], or {group, options} for an optgroup of them.
function fillOptions(select, options) {
  clear(select);
  const add = (parent, [value, label, disabled]) => {
    const o = document.createElement("option");
    o.value = value;
    o.textContent = label;
    if (disabled) o.disabled = true;
    parent.appendChild(o);
  };
  for (const item of options) {
    if (Array.isArray(item)) {
      add(select, item);
      continue;
    }
    const group = document.createElement("optgroup");
    group.label = item.group;
    for (const o of item.options) add(group, o);
    select.appendChild(group);
  }
  return select;
}

// selectInput is a select of options (as fillOptions takes them) with a
// value, calling onChange with the value chosen.
function selectInput(options, value, onChange) {
  const s = fillOptions(document.createElement("select"), options);
  if (value !== undefined && value !== null) s.value = String(value);
  s.addEventListener("change", () => onChange(s.value));
  return s;
}

// numberInput is a number input calling onChange with its number, or
// undefined when emptied.
function numberInput(value, min, max, step, onChange) {
  const i = document.createElement("input");
  i.type = "number";
  i.min = String(min);
  i.max = String(max);
  i.step = String(step);
  if (value !== undefined && value !== null) i.value = String(value);
  i.addEventListener("change", () => onChange(i.value === "" ? undefined : Number(i.value)));
  return i;
}

// stepper puts a − and a + button around a number input, which change
// it by step within min and max and fire its change event; labels names
// the two buttons ({less, more}), half a degree less and more if not.
function stepper(input, step, min, max, labels) {
  const l = labels || { less: t("temp.less"), more: t("temp.more") };
  const box = document.createElement("span");
  box.className = "inline";
  const by = (d) => () => {
    const v = Number(input.value === "" ? min : input.value) + d;
    input.value = String(Math.min(max, Math.max(min, Math.round(v / step) * step)));
    input.dispatchEvent(new Event("change"));
  };
  box.append(button("−", { cls: "step small", aria: l.less }, by(-step)), input, button("+", { cls: "step small", aria: l.more }, by(step)));
  return box;
}

// showResult says how an action went in a message element: the text,
// the class ok or bad; no text hides it.
function showResult(el, text, ok) {
  el.textContent = text || "";
  el.classList.toggle("hidden", !text);
  el.classList.toggle("ok", !!text && !!ok);
  el.classList.toggle("bad", !!text && !ok);
}

// confirmBox asks in the page whether to go on: the question, No and
// Yes (yes and no their labels, Yes the primary), Escape for No. The
// caller puts it where the action was asked for, and may focus it. Yes
// is busy (setBusy) and No off from Yes's click until the promise onYes
// returns has settled, so that a double click sends one request.
function confirmBox(text, { yes, no }, onYes, onNo) {
  const box = document.createElement("div");
  box.className = "panel confirm-box";
  box.setAttribute("role", "group");
  const q = textEl("span", text, "confirm-text");
  q.id = "confirm-" + ++fieldSeq;
  box.setAttribute("aria-labelledby", q.id);
  const noBtn = button(no || t("action.cancel"), "confirm-no", onNo);
  const yesBtn = button(yes, "confirm-yes primary", () => {
    if (isBusy(yesBtn)) return;
    const noWas = noBtn.disabled;
    noBtn.disabled = true;
    whileBusy(yesBtn, () => onYes && onYes()).finally(() => {
      noBtn.disabled = noWas;
    });
  });
  const actions = document.createElement("div");
  actions.className = "actions";
  actions.append(noBtn, yesBtn);
  box.append(q, actions);
  box.addEventListener("keydown", (ev) => {
    if (ev.key !== "Escape") return;
    ev.preventDefault();
    ev.stopPropagation();
    if (onNo) onNo();
  });
  box.focusFirst = () => noBtn.focus();
  return box;
}

// confirmInPlace asks in place of a row of actions, as a panel's Remove
// does: the question takes the row's place, Yes runs onYes (which
// returns the removal's promise, see confirmBox), and No and Escape put
// the row back with the focus on the element of focusSelector in it.
function confirmInPlace(actions, text, labels, onYes, focusSelector) {
  const box = confirmBox(text, labels, onYes, () => {
    box.replaceWith(actions);
    const el = focusSelector && actions.querySelector(focusSelector);
    if (el) el.focus();
  });
  actions.replaceWith(box);
  box.focusFirst();
  return box;
}

// fromTemplate clones the first element of a <template> of the page.
// The template's data-i18n texts are those of the language of the last
// applyI18n(document); a change of language applies them again and then
// redraws what was cloned from them.
function fromTemplate(id) {
  return document.getElementById(id).content.firstElementChild.cloneNode(true);
}

// fragmentFromTemplate clones the whole content of a <template> of the
// page, for a frame of several elements that goes into a container of
// the page as it is.
function fragmentFromTemplate(id) {
  return document.getElementById(id).content.cloneNode(true);
}

// detailsPanel is a <details> panel: a summary of a name, a monospaced
// part (a serial) and badges, then the body; onToggle gets open.
function detailsPanel({ name, mono, badges, open, onToggle, cls }, body) {
  const d = document.createElement("details");
  d.className = "panel" + (cls ? " " + cls : "");
  d.open = !!open;
  const sum = document.createElement("summary");
  if (name !== undefined) sum.appendChild(textEl("span", name, "panel-name"));
  if (mono) sum.appendChild(textEl("span", mono, "mono"));
  if (badges && badges.length) {
    const b = document.createElement("span");
    b.className = "panel-badges";
    b.append(...badges);
    sum.appendChild(b);
  }
  d.appendChild(sum);
  if (body) d.append(...[].concat(body));
  if (onToggle) d.addEventListener("toggle", () => onToggle(d.open));
  return d;
}

// downloadBlob saves a blob as a file: a blob: link with the download
// attribute, which the page's Content-Security-Policy allows.
function downloadBlob(name, blob) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// filePicker is a hidden file input taking the types of accept; a file
// chosen goes to onFile, and the input is emptied so that the same file
// can be chosen again. Its click() opens the browser's file choice.
function filePicker(accept, onFile) {
  const input = document.createElement("input");
  input.type = "file";
  input.className = "hidden";
  if (accept) input.accept = accept;
  input.addEventListener("change", () => {
    const file = input.files && input.files[0];
    input.value = "";
    if (file) onFile(file);
  });
  return input;
}

// --- a page of section cards -------------------------------------------
//
// A long page (System settings) is a grid of cards, each a
// section.section-card with an id: its head an h3 holding a
// button.section-fold (aria-controls its body) with the title in a
// span.section-title, then a span.section-badges and its help; its body
// the element the button controls. The card's needs-* classes say who
// sees it, as everywhere on the page.

// foldCard folds a card or opens it: the body hidden, the head with its
// badges still shown.
function foldCard(card, folded) {
  const btn = card.querySelector(".section-fold");
  btn.setAttribute("aria-expanded", folded ? "false" : "true");
  document.getElementById(btn.getAttribute("aria-controls")).classList.toggle("hidden", folded);
  card.classList.toggle("folded", folded);
}

// sectionCards makes the cards of root fold from the button in their
// heading, each card's fold kept in this browser (massimo.fold.<card
// id>, app.js store) and taken up again at the next load. Cards wired
// once are left alone, so it may be called again.
function sectionCards(root) {
  for (const card of root.querySelectorAll(".section-card")) {
    const btn = card.querySelector(".section-fold");
    if (!btn || btn.dataset.wired) continue;
    btn.dataset.wired = "1";
    foldCard(card, readStored("massimo.fold." + card.id) === "1");
    btn.addEventListener("click", () => {
      const folded = btn.getAttribute("aria-expanded") === "true";
      foldCard(card, folded);
      store("massimo.fold." + card.id, folded ? "1" : null);
    });
  }
}

// sectionBadges shows what a card holds at a glance in its head, also
// while it is folded: badges (badgeEl) in place of the ones it had.
function sectionBadges(card, badges) {
  const box = card && card.querySelector(".section-badges");
  if (!box) return;
  clear(box);
  box.append(...badges);
}

// showCard brings a card into view, opened, with the focus on its
// heading's button.
function showCard(card) {
  if (card.classList.contains("folded")) {
    foldCard(card, false);
    store("massimo.fold." + card.id, null);
  }
  const smooth = !matchMedia("(prefers-reduced-motion: reduce)").matches;
  card.scrollIntoView({ behavior: smooth ? "smooth" : "auto", block: "start" });
  card.querySelector(".section-fold").focus({ preventScroll: true });
}

// pageContents fills a page's row of section buttons (its nav.toc)
// from its cards, a button each, named by the card's data-toc key or
// else its title, and hidden where the card is (its needs-* classes);
// a button brings its card into view (showCard). Called again after a
// change of language.
function pageContents(page) {
  const nav = page.querySelector(".toc");
  if (!nav) return;
  clear(nav);
  for (const b of sectionButtons(page)) nav.appendChild(b);
}

// sectionButtons are a button per card of a page, named by the card's
// data-toc key or else its title, hidden where the card is (its needs-*
// classes), each bringing its card into view (showCard); data-target is
// the card's id.
function sectionButtons(page) {
  return [...page.querySelectorAll(".section-card")].map((card) => {
    const name = card.dataset.toc ? t(card.dataset.toc) : card.querySelector(".section-title").textContent;
    const b = button(name, "toc-link", () => showCard(card));
    b.dataset.target = card.id;
    for (const c of card.classList) if (c.startsWith("needs-")) b.classList.add(c);
    return b;
  });
}

// pageBar fills a page's row kept at the top (its .page-bar) with the
// sections menu (a dropdown of sectionButtons), marking the card in
// view as the page scrolls (aria-current), and keeps the row's height
// in --bar-h for the cards' scroll margin. Called again after a change
// of language; it wires the page once.
function pageBar(page) {
  const bar = page.querySelector(".page-bar");
  const menu = bar && bar.querySelector(".sec-menu");
  if (!menu) return;
  const items = menu.querySelector(".menu-items");
  clear(items);
  items.append(...sectionButtons(page));
  if (bar.dataset.wired) return;
  bar.dataset.wired = "1";
  dropdown(menu);
  new ResizeObserver(() => document.documentElement.style.setProperty("--bar-h", bar.offsetHeight + "px")).observe(bar);
  // the card in view: the lowest top that has passed under the header
  // and this row; of cards side by side, the first
  const mark = () => {
    const line = bar.getBoundingClientRect().bottom + 24;
    let best = null;
    let bestTop = -Infinity;
    for (const card of page.querySelectorAll(".section-card")) {
      if (!card.offsetParent) continue; // not shown to this role
      const y = Math.round(card.getBoundingClientRect().top);
      if (y <= line && y > bestTop) {
        best = card.id;
        bestTop = y;
      }
    }
    for (const b of items.querySelectorAll(".toc-link")) {
      if (b.dataset.target === best) b.setAttribute("aria-current", "true");
      else b.removeAttribute("aria-current");
    }
  };
  addEventListener("scroll", mark, { passive: true });
  menu.addEventListener("toggle", mark);
}

// stickyHeader keeps the page's header at the top (style.css .top): its
// height in --top-h, for the page's row under it, and the class
// scrolled on the body once the page has scrolled, which compacts the
// header. The compacted header keeps the height it gave up as its
// bottom margin, so that the page is as long scrolled as not: a page
// little taller than the window would otherwise become too short to stay
// scrolled, the header grow back, and the page jump to and fro.
// Scrolled from 96 px down, not again under 8 px.
function stickyHeader() {
  const top = document.querySelector(".top");
  // the border box: compacting changes the padding, which the content
  // box does not see
  const height = () => document.documentElement.style.setProperty("--top-h", top.getBoundingClientRect().height + "px");
  new ResizeObserver(height).observe(top, { box: "border-box" });
  const set = (on) => {
    if (on === document.body.classList.contains("scrolled")) return;
    top.style.marginBottom = "";
    const whole = top.getBoundingClientRect().height;
    const margin = parseFloat(getComputedStyle(top).marginBottom) || 0;
    document.body.classList.toggle("scrolled", on);
    if (on) top.style.marginBottom = margin + whole - top.getBoundingClientRect().height + "px";
    height();
  };
  const onScroll = () => {
    const y = window.scrollY;
    if (y > 96) set(true);
    else if (y < 8) set(false);
  };
  addEventListener("scroll", onScroll, { passive: true });
  onScroll();
}

// backToTop shows a page's .to-top button once the page's row of
// section buttons is out of sight, and takes the page back to its top,
// the focus on the row's first button.
function backToTop(page) {
  const btn = page.querySelector(".to-top");
  const nav = page.querySelector(".toc");
  if (!btn || !nav) return;
  new IntersectionObserver(([e]) => btn.classList.toggle("hidden", e.isIntersecting)).observe(nav);
  btn.addEventListener("click", () => {
    const smooth = !matchMedia("(prefers-reduced-motion: reduce)").matches;
    window.scrollTo({ top: 0, behavior: smooth ? "smooth" : "auto" });
    const first = nav.querySelector(".toc-link");
    if (first) first.focus({ preventScroll: true });
  });
}

// --- a choice of one in a menu (the Cube choice) -----------------------

// connIcon is a connection as a plug: joined and green while connected,
// apart and red while not (icon-plug-on, icon-plug-off), named for a
// screen reader, since shape and colour alone say nothing to one.
function connIcon(connected) {
  const svg = iconEl(connected ? "icon-plug-on" : "icon-plug-off", "ni conn " + (connected ? "ok" : "bad"));
  svg.removeAttribute("aria-hidden");
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", t(connected ? "header.connected" : "header.disconnected"));
  return svg;
}

// menuItem is an item of a dropdown's list: a button with its icon (a
// symbol's name, or an element such as connIcon; with blank and no icon
// an empty place of an icon's width, so that the labels line up), its
// label and, under it in grey, a second line (sub).
function menuItem({ icon, label, sub, blank }, onClick) {
  const b = document.createElement("button");
  b.type = "button";
  if (icon) b.appendChild(typeof icon === "string" ? iconEl(icon, "ni") : icon);
  else if (blank) b.appendChild(elem("span", "menu-icon-none"));
  const name = textEl("span", label, "menu-item-label");
  b.appendChild(sub ? elem("span", "menu-item-text", name, textEl("span", sub, "menu-item-sub")) : name);
  if (onClick) b.addEventListener("click", onClick);
  return b;
}

// pickerMenu makes a dropdown (details.menu, its summary holding a
// .picker-icon and a .picker-label, and a .menu-items) the choice of one
// item, a menu of the page where a select could not show the chosen
// item's name alone while its list shows more. The summary shows
// shown's icon and label, its title and aria-label; the list the items:
// { value, label, sub, icon, disabled } (menuItem, the chosen value
// marked aria-current), { group: heading } or { separator: true }.
// A click on an item calls onPick with its value; the arrow keys move
// between the items, and opening puts the focus on the chosen one.
function pickerMenu(details, { items, value, onPick, shown }) {
  const sum = details.querySelector("summary");
  const icon = sum.querySelector(".picker-icon");
  clear(icon);
  if (shown.icon) icon.appendChild(shown.icon);
  sum.querySelector(".picker-label").textContent = shown.label;
  sum.title = shown.title || shown.label;
  sum.setAttribute("aria-label", shown.aria || shown.label);
  const list = details.querySelector(".menu-items");
  clear(list);
  const blank = items.some((it) => it.icon);
  for (const it of items) {
    if (it.group) list.appendChild(textEl("p", it.group, "menu-group"));
    else if (it.separator) list.appendChild(elem("hr", "menu-sep"));
    else {
      const b = menuItem({ icon: it.icon, label: it.label, sub: it.sub, blank }, () => onPick(it.value));
      b.dataset.value = it.value;
      b.disabled = !!it.disabled;
      if (it.value === value) b.setAttribute("aria-current", "true");
      list.appendChild(b);
    }
  }
  if (details.dataset.picker) return;
  details.dataset.picker = "1";
  dropdown(details);
  const enabled = () => [...details.querySelectorAll(".menu-items button:not(:disabled)")];
  details.addEventListener("toggle", () => {
    if (!details.open) return;
    const b = details.querySelector(".menu-items button[aria-current='true']") || enabled()[0];
    if (b) b.focus();
  });
  details.addEventListener("keydown", (ev) => {
    if (!details.open || (ev.key !== "ArrowDown" && ev.key !== "ArrowUp")) return;
    const bs = enabled();
    if (!bs.length) return;
    ev.preventDefault();
    const i = bs.indexOf(document.activeElement);
    const next = ev.key === "ArrowDown" ? (i + 1) % bs.length : (i <= 0 ? bs.length - 1 : i - 1);
    bs[next].focus();
  });
}

// dropdown makes a <details class="menu"> a menu that drops down: one
// open at a time, closed by a choice of one of its items, by Escape
// (the focus back on its summary) and by a click outside it. The room
// tiles' ⋯ and the header's 👤 and ☰ are such menus.
function dropdown(details) {
  details.addEventListener("toggle", () => {
    if (!details.open) return;
    for (const other of document.querySelectorAll("details.menu[open]")) {
      if (other !== details) other.open = false;
    }
  });
  details.addEventListener("click", (ev) => {
    if (ev.target.closest(".menu-items button")) details.open = false;
  });
  return details;
}

document.addEventListener("click", (ev) => {
  for (const m of document.querySelectorAll("details.menu[open]")) {
    if (!m.contains(ev.target)) m.open = false;
  }
});
document.addEventListener("keydown", (ev) => {
  if (ev.key !== "Escape") return;
  for (const m of document.querySelectorAll("details.menu[open]")) {
    m.open = false;
    if (m.contains(document.activeElement)) m.querySelector("summary").focus();
  }
});

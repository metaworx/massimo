// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, components: the pieces the plain scripts build
// with, each a function that returns an element or fills one, for the
// connect page (connect.js), the sign-in's plain code (login.js), the
// week programme's kit (config.js) and app.js' help; and what the parts
// call of the page's (the menus' placing, a copy, a bubble, a toast, a
// sign's blinking, a result line). Texts arrive translated (t() in
// i18n.js) and go in through textContent only; a component never builds
// markup from a string. Loaded after i18n.js and before app.js; the
// helpers that read the page's state (fmtAge, helpParts, fmtTemp) are in
// app.js.
"use strict";

// How long a result that went well stays shown, in ms: a result line's
// success, and the check on a form's Save.
const RESULT_FADE_MS = 5000;

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

// badgeEl is a small outlined or filled label (.flag) of a kind: ok,
// warn, bad, mode, fill-ok, fill-warn (the Manual mode), fill-bad,
// fill-info. A warning or a failure (warn, bad, fill-bad) is set in bold
// and led by its mark, a triangle or a cross, so that it reads without
// its colour (markEl); the others are states, in the regular weight.
// key names the badge across redraws (keepBlink): by default its kind
// and text.
function badgeEl(text, kind, key) {
  const b = textEl("span", text, "flag " + kind);
  const state = badgeState(kind);
  if (state) b.prepend(markEl(state, key || (text && kind + ":" + text)));
  return b;
}

// BADGE_STATE is the state a badge's kind stands for, where it is one a
// mark shows: amber a warning, red a failure.
const BADGE_STATE = { warn: "warn", bad: "bad", "fill-bad": "bad" };

// badgeState is the state a badge's kind stands for (BADGE_STATE), by
// its first class: "warn stale-badge" is a warning.
function badgeState(kind) {
  return BADGE_STATE[(kind || "").split(" ")[0]];
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

// SIGNS draw a kind of thing in a state: its icon alone where all is
// well; else its drawing, smaller where the mark takes a corner, and the
// mark, a triangle (warn) or a cross (bad). The radio and the window are
// drawn as the person chooses (ICON_CHOICES); the network (a Cube's
// link) carries the mark on top, in place of its upper node; the cog
// (the settings), the log (its unseen problems, ui/src/log/alerts.ts) and
// the disk (a form's unsaved changes, ui/src/shared/forms.ts) at the
// bottom right. A battery has no mark: it is amber or nothing.
const SIGNS = {
  net: { icon: "icon-net", base: "icon-net-base", warn: "mark-net-warn", bad: "mark-net-x" },
  cog: { icon: "icon-cog", base: "icon-cog-small", warn: "mark-corner-warn", bad: "mark-corner-x" },
  log: { icon: "icon-log", base: "icon-log-small", warn: "mark-corner-warn", bad: "mark-corner-x" },
  disk: { icon: "icon-disk", base: "icon-disk-small", warn: "mark-corner-warn", bad: "mark-corner-x" },
  battery: { icon: "icon-battery" },
};

// ICON_CHOICES are the drawings a person chooses for the window and the
// radio (My settings → Appearance, AP IconChoice), the first of each the
// default: each its sign, as SIGNS has them, and the window's type icon,
// a window contact's (closed). The opening sash carries its mark inside
// its frame, at its size; the paned window of the original MAX! software
// and the Wi-Fi fan at the bottom right, drawn smaller; the tower of the
// thermostats' display moves aside for it.
const ICON_CHOICES = {
  window: {
    sash: { type: "icon-window-sash", sign: { icon: "icon-window-sash-open", base: "icon-window-sash-open", warn: "mark-sash-warn", bad: "mark-sash-x" } },
    paned: { type: "icon-window", sign: { icon: "icon-window-open", base: "icon-window-open-small", warn: "mark-corner-warn", bad: "mark-corner-x" } },
  },
  radio: {
    wifi: { sign: { icon: "icon-radio", base: "icon-radio-small", warn: "mark-corner-warn", bad: "mark-corner-x" } },
    tower: { sign: { icon: "icon-tower", base: "icon-tower-small", warn: "mark-tower-warn", bad: "mark-tower-x" } },
  },
};

// ICON_SETTINGS name the person's setting of each kind of ICON_CHOICES
// (app.js state.settings).
const ICON_SETTINGS = { window: "iconWindow", radio: "iconRadio" };

// iconChoice is the drawing of a kind of ICON_CHOICES that the person's
// setting chooses, the default where it chooses none of them; the
// default on a page without the settings.
function iconChoice(kind) {
  const set = ICON_CHOICES[kind];
  const settings = (typeof state !== "undefined" && state.settings) || {}; // app.js; none on the connect page
  return set[settings[ICON_SETTINGS[kind]]] || Object.values(set)[0];
}

// signOf is a kind's sign: the person's choice for the window and the
// radio, SIGNS' for the others.
function signOf(kind) {
  return ICON_CHOICES[kind] ? iconChoice(kind).sign : SIGNS[kind];
}

// signEl is the icon of a kind of SIGNS in a state, as the parts' Sign
// (ui/src/shared/icon.tsx) draws it, for a plain script (the connect
// page's "changed here" mark, AP MobileApp, Block 19): "" or "ok" plain,
// "warn" with a triangle, "bad" with a cross, coloured by the state
// (tone-…); the mark a part of its own (use.mark), which is what blinks.
// cls adds classes, key names the mark across redraws (keepBlink), and
// label, where given, names the sign for a screen reader, which is else
// hidden from one.
function signEl(kind, state, { cls, key, label } = {}) {
  const s = signOf(kind);
  const mark = s[state];
  const svg = iconEl(mark ? s.base : s.icon, "ni sign" + (state ? " tone-" + state : "") + (cls ? " " + cls : ""));
  if (label) {
    svg.removeAttribute("aria-hidden");
    svg.setAttribute("role", "img");
    svg.setAttribute("aria-label", label);
  }
  if (mark) {
    const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
    use.setAttribute("href", "#" + mark);
    use.setAttribute("class", state === "bad" ? "mark x" : "mark");
    svg.appendChild(use);
    keepBlink(use, key);
  }
  return svg;
}

// markEl is a mark alone, the triangle of a warning or the cross of a
// failure, for a badge or a list; key as for keepBlink.
function markEl(state, key) {
  const svg = iconEl(state === "bad" ? "mark-x" : "mark-warn", "ni mark-sign tone-" + state);
  svg.firstChild.setAttribute("class", state === "bad" ? "mark x" : "mark");
  keepBlink(svg.firstChild, key);
  return svg;
}

// toastState is the page's toast's: its timer and the action of its
// button (showToast).
const toastState = { timer: null, action: null };

// showToast shows a message at the page's foot (#toast) for ms
// milliseconds, a newer one in the place of the one shown: its title in
// bold where it has one, its text, its tone ("bad" for what failed, a red
// edge; "warn" for what to check, an orange one) and an action,
// { label, run }, a button drawn for it alone that runs it and closes the
// toast; × closes it at once. Notifications (notify.js notifyToast), the
// Cube search's outcome (ui/src/header/send.ts discoverCubes) and the
// first owner's creation (ui/src/signin/Owner.tsx) show here.
function showToast(text, { title = "", ms = 10000, tone = "", action = null } = {}) {
  const toast = $("toast");
  if (!toast.dataset.wired) {
    toast.dataset.wired = "1";
    $("toast-close").addEventListener("click", hideToast);
  }
  $("toast-title").textContent = title;
  $("toast-title").classList.toggle("hidden", !title);
  $("toast-body").textContent = text;
  toastState.action = action ? action.run : null;
  const old = toast.querySelector(".toast-action");
  if (old) old.remove();
  if (action) {
    const b = button(action.label, "toast-action", () => {
      const run = toastState.action;
      hideToast();
      if (run) run();
    });
    b.id = "toast-action";
    toast.insertBefore(b, $("toast-close"));
  }
  toast.classList.toggle("bad", tone === "bad");
  toast.classList.toggle("warn", tone === "warn");
  toast.classList.remove("hidden");
  if (toastState.timer) clearTimeout(toastState.timer);
  toastState.timer = setTimeout(hideToast, ms);
}

// hideToast takes the page's toast away (showToast).
function hideToast() {
  if (toastState.timer) clearTimeout(toastState.timer);
  toastState.timer = null;
  toastState.action = null;
  $("toast").classList.add("hidden");
}

// marksSeen are when each mark was first drawn and last drawn, by key.
const marksSeen = new Map();

// keepBlink keeps a mark's blinking where it was across the redraws that
// make a new element of it: its animation starts back at when the mark
// with this key was first drawn, so that "at first" blinks five seconds
// from the sign's coming and not from each redraw, and "on" does not
// jump. A mark not drawn for a minute counts as new again. Without a
// key, a mark is new each time.
function keepBlink(use, key) {
  if (!key) return;
  const now = performance.now();
  let seen = marksSeen.get(key);
  if (!seen || now - seen.last > 60000) {
    seen = { first: now, last: now };
    if (marksSeen.size > 500) marksSeen.clear();
    marksSeen.set(key, seen);
  }
  seen.last = now;
  use.style.animationDelay = -((now - seen.first) / 1000).toFixed(2) + "s";
}

// copyText puts text on the clipboard: through navigator.clipboard where
// the page may use it (HTTPS or localhost), else through a hidden text
// area and the browser's copy command, which every browser still runs on
// a click, as a page served over plain HTTP on the LAN needs, the focus
// given back to the element that had it, which the text area took.
// Resolves to whether it copied.
async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (e) {
      // refused: the fallback may still work
    }
  }
  const had = document.activeElement;
  const area = document.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "");
  area.className = "copy-area";
  document.body.appendChild(area);
  area.select();
  let ok = false;
  try {
    ok = document.execCommand("copy");
  } catch (e) {
    ok = false;
  }
  area.remove();
  if (had instanceof HTMLElement && had !== document.body && had.isConnected) had.focus({ preventScroll: true });
  return ok;
}

// copied says that something was copied, or that it could not be: in a
// bubble above the control (showBubble).
function copied(el, ok) {
  showBubble(t(ok ? "copy.done" : "copy.failed"), el);
}

// The page's bubble (showBubble): its element and its live region, made
// at the first bubble and kept; the control it shows at, where that was
// when it was last placed, the page's scroll when it was shown, and its
// timers and frame.
const bubbleState = { el: null, live: null, ctrl: null, at: null, scroll: null, timer: 0, say: 0, frame: 0 };

// How far a bubble keeps from its control, and from the window's edges,
// in px.
const BUBBLE_GAP = 4;
const BUBBLE_EDGE = 8;

// showBubble shows a short text for a moment in a bubble: with ctrl,
// above ctrl's centre, as near to it as the window's edges let it, and
// below ctrl where the window has no room above it; without one, at the
// window's bottom centre, above the safe area. It stays ms milliseconds,
// 1.4 s by default, and goes earlier at a scroll that moves ctrl (or,
// without one, at a scroll of the page), and once ctrl has left the page
// or is hidden; a newer bubble takes its place. Its text is said once in
// the page's polite live region, the bubble itself hidden from a screen
// reader. The bubble and the region are made at the first bubble, on the
// page and the connect page alike, and kept.
function showBubble(text, ctrl, { ms = 1400 } = {}) {
  const b = bubbleState;
  if (!b.el) {
    b.el = textEl("div", "", "bubble hidden");
    b.el.id = "bubble";
    b.el.setAttribute("aria-hidden", "true");
    b.live = textEl("span", "", "visually-hidden");
    b.live.id = "bubble-status";
    b.live.setAttribute("role", "status");
    b.live.setAttribute("aria-live", "polite");
    addEventListener("scroll", bubbleScrolled, { capture: true, passive: true });
  }
  if (!b.el.isConnected) document.body.append(b.live, b.el);
  clearTimeout(b.timer);
  clearTimeout(b.say);
  cancelAnimationFrame(b.frame);
  b.ctrl = ctrl || null;
  b.scroll = [window.scrollX, window.scrollY];
  b.el.textContent = text;
  b.el.classList.toggle("foot", !b.ctrl);
  b.el.classList.remove("hidden");
  if (b.ctrl) followBubble();
  else Object.assign(b.el.style, { left: "", top: "" });
  b.timer = setTimeout(hideBubble, ms);
  // emptied, then the words, so that a screen reader hears a text the
  // region holds already
  b.live.textContent = "";
  b.say = setTimeout(() => {
    b.live.textContent = text;
  }, 50);
}

// hideBubble takes the bubble away (showBubble); what its live region
// said stays there.
function hideBubble() {
  const b = bubbleState;
  clearTimeout(b.timer);
  cancelAnimationFrame(b.frame);
  b.timer = 0;
  b.frame = 0;
  b.ctrl = null;
  b.at = null;
  if (b.el) b.el.classList.add("hidden");
}

// followBubble keeps the bubble at its control, frame by frame while it
// shows, where the page's layout moves the control, and takes it away
// once the control has left the page or is hidden: a bubble over the
// element drawn in its place (a new suggested password) would say what
// was not done to it.
function followBubble() {
  const b = bubbleState;
  b.frame = 0;
  if (!b.ctrl) return;
  if (!b.ctrl.isConnected || !b.ctrl.getClientRects().length) {
    hideBubble();
    return;
  }
  placeBubble();
  b.frame = requestAnimationFrame(followBubble);
}

// placeBubble places the bubble above its control's centre, kept
// BUBBLE_EDGE inside the window's edges, or below the control where the
// window has no room above it.
function placeBubble() {
  const b = bubbleState;
  const r = b.ctrl.getBoundingClientRect();
  const vw = document.documentElement.clientWidth;
  const vh = document.documentElement.clientHeight;
  const w = b.el.offsetWidth;
  const h = b.el.offsetHeight;
  const left = Math.max(BUBBLE_EDGE, Math.min(r.left + r.width / 2 - w / 2, vw - BUBBLE_EDGE - w));
  let top = r.top - BUBBLE_GAP - h;
  if (top < BUBBLE_EDGE) top = Math.max(BUBBLE_EDGE, Math.min(r.bottom + BUBBLE_GAP, vh - BUBBLE_EDGE - h));
  b.el.style.left = left + "px";
  b.el.style.top = top + "px";
  b.at = r;
}

// bubbleScrolled takes the bubble away at a scroll that has moved its
// control, the page's or a box's that holds it, since a bubble fixed in
// the window would stay where the control was; without a control, at a
// scroll of the page. A scroll elsewhere, or one that had ended before
// the bubble showed, leaves it.
function bubbleScrolled() {
  const b = bubbleState;
  if (!b.el || b.el.classList.contains("hidden")) return;
  if (b.ctrl) {
    const r = b.ctrl.getBoundingClientRect();
    if (b.at && Math.abs(r.left - b.at.left) < 0.5 && Math.abs(r.top - b.at.top) < 0.5) return;
  } else if (b.scroll && window.scrollX === b.scroll[0] && window.scrollY === b.scroll[1]) {
    return;
  }
  hideBubble();
}

// copyable makes an element copy what get() returns on a click, and on
// Enter or Space: a button to the keyboard and a screen reader, named by
// label, its tooltip saying so. Wired once; label may change later
// (copyLabel).
function copyable(el, get, label) {
  el.classList.add("copyable");
  if (!/^(BUTTON|A)$/.test(el.tagName)) {
    el.setAttribute("role", "button");
    el.tabIndex = 0;
  }
  copyLabel(el, label);
  const run = async (ev) => {
    ev.preventDefault();
    ev.stopPropagation();
    const text = get();
    if (text) copied(el, await copyText(text));
  };
  el.addEventListener("click", run);
  el.addEventListener("keydown", (ev) => {
    if (ev.key === "Enter" || ev.key === " ") run(ev);
  });
  return el;
}

// copyLabel names a copyable element: what it copies, for a screen
// reader and as its tooltip.
function copyLabel(el, label) {
  el.setAttribute("aria-label", label);
  el.title = label;
}

// copyIcon is a small button with the copy icon that copies what get()
// returns; label names what it copies.
function copyIcon(get, label) {
  const b = document.createElement("button");
  b.type = "button";
  b.className = "copy-icon";
  b.appendChild(iconEl("icon-copy", "ni"));
  return copyable(b, get, label);
}

// copyEl is a text that copies itself, as a mono span, and with icon a
// copy icon after it, both copying text; label names what.
function copyEl(text, { label, icon, cls } = {}) {
  const span = textEl("span", text, cls || "mono");
  const name = t("copy.what", { what: label || text });
  copyable(span, () => text, name);
  if (!icon) return span;
  return elem("span", "copy-pair", span, copyIcon(() => text, name));
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
// whose label cancelLabel replaces (Close), a button of the class
// form-cancel, as a part's footer's (ui/src/shared/field.tsx FormFooter).
// Each part is optional.
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
  if (cancel) right.appendChild(button(cancelLabel || t("action.cancel"), "form-cancel", cancel));
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

// showResult says how an action went in a message element: the text,
// the class ok or bad; no text hides it.
function showResult(el, text, ok) {
  el.textContent = text || "";
  el.classList.toggle("hidden", !text);
  el.classList.toggle("ok", !!text && !!ok);
  el.classList.toggle("bad", !!text && !ok);
  if (text && !ok) failedLines.add(el);
  else failedLines.delete(el);
}

// failedLines are the lines that showResult shows a failure in, so that
// the page can take back what it told a user who is gone (forgetNotSent,
// app.js).
const failedLines = new Set();

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
// the row back with the focus on the element of focusSelector in it,
// then tell onNo, where given.
function confirmInPlace(actions, text, labels, onYes, focusSelector, onNo) {
  const box = confirmBox(text, labels, onYes, () => {
    box.replaceWith(actions);
    const el = focusSelector && actions.querySelector(focusSelector);
    if (el) el.focus();
    if (onNo) onNo();
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

// --- a page of section cards -------------------------------------------
//
// A long page (System settings) is a grid of cards, each a
// section.section-card with an id (ui/src/shared/card.ts): their folds,
// kept in this browser, and their heads' badges are the parts'
// (ui/src/chrome/).

// showCard brings a card into view, opened, with the focus on its
// heading's button, as the parts do (ui/src/shared/card.ts showCard); on a
// page just opened (instant) it is kept there while the cards around it
// grow. home.js calls it.
function showCard(card, instant) {
  MassimoParts.chrome.showCard(card.id, instant);
}

// BRAND_PARTS are the parts of Massimo's mark in the sprite
// (ui/brand/brand.svg, make ui-brand, tools/brand/README.md), each with
// the class that colours it (ui/brand/brand.css): the cover first. The
// parts draw the mark with a copy of their own (ui/src/shared/brand.tsx):
// brandMarks draws it over the skeleton's data-brand, which connect.html
// has too, and the connect page loads no parts.js.
const BRAND_PARTS = [
  ["cover", "brand-cover"],
  ["back", "brand-back"],
  ["m", "brand-m"],
  ["hour", "brand-hands"],
  ["minute", "brand-hands"],
];

// brandMarks draws Massimo's logo over every element marked data-brand,
// whose own text, the word, stays: under the drawing's cover once the
// sprite has come, in view until then and where it never comes, and what
// a screen reader reads. Each gets the full logo and the small mark, of
// which style.css shows one; the drawings are hidden from screen readers.
function brandMarks(root = document) {
  const NS = "http://www.w3.org/2000/svg";
  const draw = (cls, prefix, parts) => {
    const svg = document.createElementNS(NS, "svg");
    svg.setAttribute("class", cls);
    svg.setAttribute("aria-hidden", "true");
    svg.setAttribute("focusable", "false");
    for (const [part, partClass] of parts) {
      const use = document.createElementNS(NS, "use");
      use.setAttribute("href", "ui/brand/brand.svg#" + prefix + "-" + part);
      use.setAttribute("class", partClass);
      svg.appendChild(use);
    }
    return svg;
  };
  for (const el of root.querySelectorAll("[data-brand]")) {
    if (el.querySelector("svg")) continue;
    el.append(
      draw("brand-logo", "logo", [...BRAND_PARTS, ["letters", "brand-letters"], ["dot", "brand-hands"]]),
      draw("brand-mark", "mark", BRAND_PARTS),
    );
  }
}

document.addEventListener("DOMContentLoaded", () => brandMarks());

// --- menus ---------------------------------------------------------------

// dropdown makes a <details class="menu"> a menu that drops down: one
// open at a time, kept in the window (placeMenu), closed by a choice of
// one of its items (but one marked data-keep-open, whose result shows in
// the list), by Escape (the focus back on its summary) and by a
// click outside it. The room tiles' ⋯ and More and the header's 👤 and ☰
// are such menus. Its list is placed as it opens, before it is first
// drawn: the toggle event comes a task later, after a frame may have
// drawn the list where the style sheet puts it, out of the window; and
// again after the toggle listeners, which may fill it.
function dropdown(details) {
  new MutationObserver(() => placeMenu(details)).observe(details, { attributes: true, attributeFilter: ["open"] });
  details.addEventListener("toggle", () => {
    if (!details.open) return;
    for (const other of document.querySelectorAll("details.menu[open]")) {
      if (other !== details) other.open = false;
    }
    // after the other toggle listeners, which may fill the list
    requestAnimationFrame(() => placeMenu(details));
  });
  details.addEventListener("click", (ev) => {
    const b = ev.target.closest(".menu-items button");
    if (b && !b.hasAttribute("data-keep-open")) details.open = false;
  });
  return details;
}

// placeMenu keeps an open menu's list in the window: where it runs out
// of the window as the style sheet places it, aligned to its summary's
// other edge where it fits there, else moved just inside the window,
// 16 px from its edges; its height capped to the room below it, with a
// scroll of its own that the page does not take over (style.css
// .menu-items). Placed again, the cap stays while it is measured, so
// that a list scrolled in itself keeps its scroll.
function placeMenu(details) {
  const list = details.querySelector(":scope > .menu-items");
  if (!list || !details.open) return;
  const gap = 16;
  const vw = document.documentElement.clientWidth;
  Object.assign(list.style, { left: "", right: "" });
  const box = details.getBoundingClientRect();
  const r = list.getBoundingClientRect();
  const fits = (x) => x >= gap - 0.5 && x + r.width <= vw - gap + 0.5;
  if (!fits(r.left)) {
    const other = Math.abs(r.right - box.right) < 1 ? box.left : box.right - r.width;
    const x = fits(other) ? other : Math.max(gap, Math.min(r.left, vw - gap - r.width));
    Object.assign(list.style, { left: x - box.left + "px", right: "auto" });
  }
  const top = list.getBoundingClientRect().top;
  list.style.maxHeight = Math.max(160, window.innerHeight - top - gap) + "px";
}

// placeMenus places the list of every open menu under root (the whole
// page by default) again: at a resize of the window, and where the
// header's row has changed (ui/src/header/fold.ts), which moves the pill
// and the Cube choice under which their lists hang. placeMenu placed a
// list for where its summary was, an offset from it.
function placeMenus(root = document) {
  for (const m of root.querySelectorAll("details.menu[open]")) placeMenu(m);
}

addEventListener("resize", () => placeMenus());

// menuArrows lets the arrow keys move the focus between the items of
// an open dropdown, the elements matching selector: down and up, round
// at the ends, from its summary down to the first and up to the last.
function menuArrows(details, selector) {
  details.addEventListener("keydown", (ev) => {
    if (!details.open || (ev.key !== "ArrowDown" && ev.key !== "ArrowUp")) return;
    const items = [...details.querySelectorAll(selector)];
    if (!items.length) return;
    ev.preventDefault();
    const i = items.indexOf(document.activeElement);
    const next = ev.key === "ArrowDown" ? (i + 1) % items.length : (i <= 0 ? items.length - 1 : i - 1);
    items[next].focus();
  });
}

document.addEventListener("click", (ev) => {
  for (const m of document.querySelectorAll("details.menu[open]")) {
    if (!m.contains(ev.target)) m.open = false;
  }
});
// Escape closes every open menu, and where it closed one it marks the
// event (preventDefault), so that a listener after this one that leaves a
// place by Escape (the sign-in card the user opened, ui/src/signin/) leaves
// it at the next Escape only: the menu first, as everywhere.
document.addEventListener("keydown", (ev) => {
  if (ev.key !== "Escape") return;
  for (const m of document.querySelectorAll("details.menu[open]")) {
    m.open = false;
    ev.preventDefault();
    if (m.contains(document.activeElement)) m.querySelector("summary").focus();
  }
});

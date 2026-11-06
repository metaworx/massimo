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
// caller puts it where the action was asked for, and may focus it.
function confirmBox(text, { yes, no }, onYes, onNo) {
  const box = document.createElement("div");
  box.className = "panel confirm-box";
  box.setAttribute("role", "group");
  const q = textEl("span", text, "confirm-text");
  q.id = "confirm-" + ++fieldSeq;
  box.setAttribute("aria-labelledby", q.id);
  const noBtn = button(no || t("action.cancel"), "confirm-no", onNo);
  const yesBtn = button(yes, "confirm-yes primary", onYes);
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

// checkRow is a checkbox with its label; label.control is the checkbox.
function checkRow(label, checked, onChange) {
  const row = document.createElement("label");
  row.className = "check";
  const cb = document.createElement("input");
  cb.type = "checkbox";
  cb.checked = !!checked;
  if (onChange) cb.addEventListener("change", () => onChange(cb.checked));
  row.append(cb, document.createTextNode(" " + label));
  return row;
}

// fromTemplate clones the first element of a <template> of the page.
function fromTemplate(id) {
  return document.getElementById(id).content.firstElementChild.cloneNode(true);
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

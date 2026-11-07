// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Settings → Users: the users of the store, each in
// a panel with its display name, role, end date, a new password and
// Remove (PUT and DELETE /api/v1/users/{name}), its sessions and API
// keys with Revoke and a new key shown once (/api/v1/users/{name}/tokens),
// and the form that adds a user (POST /api/v1/users). A guests holder
// without users sees the guests and itself, and creates guests only.
// The list is drawn when the settings open and after every change.
"use strict";

const usersUI = {
  users: [],
  roles: null, // the roles, when they may be read, for the role choice
  open: new Set(), // names of the open panels
  keys: {}, // per name: the key just created, shown once
  busy: false,
};

function usersMsg(text, ok) {
  showResult($("users-msg"), text, ok);
}

// usersAdmin says what the signed-in user may do with users: everything,
// guests only, or nothing but its own user.
function usersAdmin() {
  return hasCap("users") ? "all" : hasCap("guests") ? "guests" : "self";
}

// mayManage reports whether a user's panel is the signed-in user's to
// change beyond the own name and password.
function mayManage(u) {
  const a = usersAdmin();
  return a === "all" || (a === "guests" && u.role === "guest");
}

async function loadUsers() {
  if (!canManageUsers()) return;
  usersUI.users = (await api("GET", "api/v1/users")) || [];
  if (hasCap("roles") && !usersUI.roles) {
    try {
      usersUI.roles = ((await api("GET", "api/v1/roles")) || {}).roles || null;
    } catch (e) {
      usersUI.roles = null;
    }
  }
}

// roleOptions are the roles a user may be given, for a select: with
// users every role (the ones read, else the default keys), with guests
// only guest; current is kept in the list.
function roleOptions(current) {
  let keys;
  if (usersAdmin() === "guests") keys = ["guest"];
  else if (usersUI.roles) keys = usersUI.roles.map((r) => r.key);
  else keys = [...DEFAULT_ROLES];
  if (current && !keys.includes(current)) keys.push(current);
  return keys.map((k) => {
    const r = (usersUI.roles || []).find((x) => x.key === k);
    return [k, roleLabel(k, r && r.name)];
  });
}

// userAction runs a change, redraws the list and says how it went.
async function userAction(fn, okText, name) {
  usersUI.busy = true;
  try {
    await fn();
    await loadUsers();
    renderUsers();
    usersMsg(okText, true);
    return true;
  } catch (e) {
    usersMsg(t("users.failed", { name, error: e.message }), false);
    return false;
  } finally {
    usersUI.busy = false;
  }
}

// localDateTimeOf is an API time as a datetime-local value, "" for none.
function localDateTimeOf(iso) {
  const ms = Date.parse(iso || "");
  return isNaN(ms) ? "" : localDateTimeValue(new Date(ms));
}

// isoOf is a datetime-local value as an API time, null when empty.
function isoOf(local) {
  if (!local) return null;
  const d = new Date(local);
  return isNaN(d.getTime()) ? null : d.toISOString();
}

// sameMinute reports whether two API times, or none, are the same end
// to the minute, all that the form's datetime-local holds: the server's
// time carries seconds and an offset.
function sameMinute(a, b) {
  const x = Date.parse(a || "");
  const y = Date.parse(b || "");
  if (isNaN(x) || isNaN(y)) return isNaN(x) && isNaN(y);
  return Math.floor(x / 60000) === Math.floor(y / 60000);
}

function userBadges(u) {
  const out = [badgeEl(roleLabel(u.role), "mode")];
  if (u.disabled) out.push(badgeEl(t("users.state.disabled"), "bad"));
  else if (u.expires_at && Date.parse(u.expires_at) <= Date.now()) out.push(badgeEl(t("users.state.expired"), "bad"));
  else if (u.expires_at) out.push(badgeEl(t("users.state.until", { date: fmtShortTime(Date.parse(u.expires_at)) }), "warn"));
  if (!u.has_password) out.push(badgeEl(t("users.keysOnly"), "mode"));
  if (isSelf(u.name)) out.push(badgeEl(t("users.you"), "ok"));
  return out;
}

// removeUser asks in the panel, in place of its actions.
function removeUser(u, actions) {
  const box = confirmBox(t("users.removeConfirm", { name: u.name }), { yes: t("users.remove"), no: t("action.keep") }, () => {
    usersUI.open.delete(u.name);
    userAction(() => api("DELETE", "api/v1/users/" + encodeURIComponent(u.name)), t("users.removed", { name: u.name }), u.name);
  }, () => {
    box.replaceWith(actions);
    actions.querySelector(".remove-user").focus();
  });
  actions.replaceWith(box);
  box.focusFirst();
}

// tokenRow is one session or key of a user, from its template, with
// Revoke.
function tokenRow(u, tk) {
  const li = fromTemplate("tpl-token-row");
  li.dataset.id = String(tk.id);
  li.querySelector(".token-kind").textContent = t("users.kind." + tk.kind);
  li.querySelector(".token-label").textContent = tk.label || "";
  const used = li.querySelector(".token-used");
  if (tk.last_used_at) {
    used.textContent = t("users.lastUsed", { from: tk.last_used_from || "?" }) + " ";
    const age = ageEl(tk.last_used_at, false, true);
    if (age) used.appendChild(age);
  } else {
    used.textContent = t("users.neverUsed");
  }
  const revoke = li.querySelector(".revoke-token");
  revoke.setAttribute("aria-label", t("users.revokeLabel", { label: tk.label || t("users.kind." + tk.kind) }));
  revoke.addEventListener("click", () => {
    userAction(() => api("DELETE", "api/v1/users/" + encodeURIComponent(u.name) + "/tokens/" + tk.id), t("users.revoked", { id: tk.id }), u.name);
  });
  return li;
}

// fillTokens loads a user's tokens into an open panel and shows the key
// just created.
async function fillTokens(u, box) {
  const ul = box.querySelector(".user-tokens");
  clear(ul);
  let tokens = [];
  try {
    tokens = (await api("GET", "api/v1/users/" + encodeURIComponent(u.name) + "/tokens")) || [];
  } catch (e) {
    ul.appendChild(textEl("li", e.message, "muted small bad"));
    return;
  }
  if (!tokens.length) ul.appendChild(textEl("li", t("users.tokensNone"), "muted small"));
  for (const tk of tokens) ul.appendChild(tokenRow(u, tk));
  const key = usersUI.keys[u.name];
  const shown = box.querySelector(".user-key-box");
  shown.classList.toggle("hidden", !key);
  if (key) {
    shown.querySelector(".user-key").textContent = key;
    delete usersUI.keys[u.name];
  }
}

// wireNewKey wires a panel's New API key once, when the panel is built.
function wireNewKey(u, box) {
  const input = box.querySelector("input[name=label]");
  input.id = "user-key-label-" + u.name;
  input.previousElementSibling.htmlFor = input.id;
  box.querySelector(".create-key").addEventListener("click", () => {
    const label = input.value.trim();
    if (!label) {
      usersMsg(t("users.keyLabelNeeded"), false);
      return;
    }
    userAction(async () => {
      const res = await api("POST", "api/v1/users/" + encodeURIComponent(u.name) + "/tokens", { label });
      usersUI.keys[u.name] = res.token;
    }, t("users.keyCreated", { label }), u.name);
  });
}

// userPanel is one user, from its template (tpl-user-panel): the
// summary with the name and the badges, the form, the tokens. A panel
// built open loads its tokens at once; the toggle event that setting
// open queues finds the name in usersUI.open and loads nothing more, so
// that the tokens load once and the new key, shown once, is not hidden
// again by a second load.
function userPanel(u) {
  const d = fromTemplate("tpl-user-panel");
  d.dataset.name = u.name;
  d.open = usersUI.open.has(u.name);
  const tokensBox = d.querySelector(".user-tokens-box");
  wireNewKey(u, tokensBox);
  const sum = d.querySelector("summary");
  sum.querySelector(".panel-name").textContent = u.display_name || u.name;
  sum.querySelector(".mono").textContent = u.name;
  sum.querySelector(".panel-badges").append(...userBadges(u));
  const manage = mayManage(u);
  const self = isSelf(u.name);
  d.addEventListener("toggle", () => {
    if (d.open) {
      if (usersUI.open.has(u.name)) return; // built open, loaded below
      usersUI.open.add(u.name);
      fillTokens(u, tokensBox);
    } else {
      usersUI.open.delete(u.name);
    }
  });
  const form = d.querySelector("form");
  for (const input of form.querySelectorAll(".field.wide > input, .field.wide > select")) {
    input.id = "user-" + u.name + "-" + input.name;
    input.previousElementSibling.htmlFor = input.id;
  }
  const field = (name) => form.querySelector("[name=" + name + "]");
  field("display_name").value = u.display_name || "";
  fillOptions(field("role"), roleOptions(u.role));
  field("role").value = u.role;
  field("disabled").checked = !!u.disabled;
  field("expires").value = localDateTimeOf(u.expires_at);
  const adminOnly = form.querySelectorAll(".admin-only");
  for (const el of adminOnly) el.classList.toggle("hidden", !manage);
  form.querySelector(".field-current").classList.toggle("hidden", manage);
  const actions = form.querySelector(".actions");
  const remove = actions.querySelector(".remove-user");
  remove.classList.toggle("hidden", !manage);
  remove.addEventListener("click", () => removeUser(u, actions));
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const body = {};
    const display = field("display_name").value.trim();
    if (display !== (u.display_name || "")) body.display_name = display;
    if (manage) {
      if (field("role").value !== u.role) body.role = field("role").value;
      if (field("disabled").checked !== !!u.disabled) body.disabled = field("disabled").checked;
      const end = isoOf(field("expires").value);
      if (!sameMinute(end, u.expires_at)) body.expires_at = end;
    }
    const password = field("password").value;
    if (password) {
      body.password = password;
      if (!manage || self) body.current_password = field("current_password").value;
    }
    if (!Object.keys(body).length) {
      usersMsg(t("users.unchanged"), true);
      return;
    }
    userAction(() => api("PUT", "api/v1/users/" + encodeURIComponent(u.name), body), t("users.saved", { name: u.name }), u.name).then((ok) => {
      if (ok && self && body.password) loadMe();
    });
  });
  if (d.open) fillTokens(u, tokensBox);
  return d;
}

// newUserForm is the form that adds a user, from its template.
function newUserForm() {
  const form = fromTemplate("tpl-user-new");
  for (const input of form.querySelectorAll(".field.wide > input, .field.wide > select")) {
    input.id = "user-new-" + input.name;
    input.previousElementSibling.htmlFor = input.id;
  }
  const field = (name) => form.querySelector("[name=" + name + "]");
  fillOptions(field("role"), roleOptions());
  field("role").value = usersAdmin() === "guests" ? "guest" : "viewer";
  form.querySelector(".cancel-user").addEventListener("click", () => {
    clear($("user-new"));
    $("user-add").classList.remove("hidden");
  });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const body = { name: field("name").value.trim(), role: field("role").value };
    const display = field("display_name").value.trim();
    if (display) body.display_name = display;
    const password = field("password").value;
    if (password) body.password = password;
    const end = isoOf(field("expires").value);
    if (end) body.expires_at = end;
    userAction(() => api("POST", "api/v1/users", body), t("users.created", { name: body.name }), body.name).then((ok) => {
      if (ok) {
        clear($("user-new"));
        $("user-add").classList.remove("hidden");
      }
    });
  });
  return form;
}

// renderUsers draws Settings → Users from usersUI.users.
function renderUsers() {
  const box = $("users-list");
  if (!box || !canManageUsers()) return;
  clear(box);
  if (!usersUI.users.length) box.appendChild(textEl("p", t("users.none"), "muted small"));
  for (const u of usersUI.users) box.appendChild(userPanel(u));
}

function initUsers() {
  $("user-add").addEventListener("click", () => {
    $("user-add").classList.add("hidden");
    const box = $("user-new");
    clear(box);
    const form = newUserForm();
    box.appendChild(form);
    form.querySelector("input[name=name]").focus();
  });
  $("view-settings").addEventListener("click", () => {
    usersMsg("");
    if (!canManageUsers()) return;
    loadUsers().then(renderUsers).catch((e) => usersMsg(t("users.failed", { name: "", error: e.message }), false));
  });
}

document.addEventListener("DOMContentLoaded", initUsers);

// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Users: the users of the store, each in
// a panel with its display name, role, end date, a new password and
// Remove (PUT and DELETE /api/v1/users/{name}), its sessions and API
// keys with Revoke and a new key shown once (/api/v1/users/{name}/tokens),
// and the form that adds a user (POST /api/v1/users). A guests holder
// without users sees the guests and itself, and creates guests only.
// The list is drawn when the settings open and after every change.
"use strict";

const usersUI = {
  users: [],
  self: null, // the signed-in user, as My profile shows it
  roles: null, // the roles, when they may be read, for the role choice
  open: new Set(), // names of the open panels
  keys: {}, // per name: the key just created, shown once
};

function usersMsg(text, ok) {
  showResult($("users-msg"), text, ok);
}

function profileMsg(text, ok) {
  showResult($("profile-msg"), text, ok);
}

// sayFor is where a change says how it went: in My profile for one made
// there ("profile"), else under System settings → Users.
function sayFor(where) {
  return where === "profile" ? profileMsg : usersMsg;
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

// USER_NAME is the store's rule for a user's name (userName in
// internal/store/users.go), after NormalName: trimmed and lower case.
const USER_NAME = /^[a-z0-9][a-z0-9._@-]{0,63}$/;

// NAME_CHECK_MS is how long a name being typed waits before it is
// checked: the owner asked for a check once the field has not changed
// for 2 s.
const NAME_CHECK_MS = 2000;

// nameProblem says what is wrong with a new user's name, "" for
// nothing: the store's rule, or a name in use among taken.
function nameProblem(name, taken) {
  const n = name.trim().toLowerCase();
  if (!n) return "";
  if (!USER_NAME.test(n)) return t("users.nameBad");
  if ((taken || []).includes(n)) return t("users.nameTaken", { name: n });
  return "";
}

// watchName checks a new user's name field: NAME_CHECK_MS after the
// last keystroke and when the field is left. msg, under the field, says
// what is wrong, and clears at the first keystroke that fixes it; taken
// returns the names in use. It returns the check, which the form's
// submit calls: true when the name may be sent. A refusal the server
// words (wordNameError) goes to msg as well.
function watchName(input, msg, taken) {
  let timer = null;
  const say = (text) => {
    showResult(msg, text, false);
    if (text) input.setAttribute("aria-invalid", "true");
    else input.removeAttribute("aria-invalid");
  };
  const check = () => {
    clearTimeout(timer);
    timer = null;
    const p = nameProblem(input.value, taken());
    say(p);
    return !p;
  };
  input.setAttribute("aria-describedby", msg.id);
  input.addEventListener("input", () => {
    clearTimeout(timer);
    timer = null;
    if (!nameProblem(input.value, taken())) say("");
    else timer = setTimeout(check, NAME_CHECK_MS);
  });
  input.addEventListener("blur", check);
  check.say = say;
  return check;
}

// wordNameError words the server's refusal of a new user's name as the
// page's check does, or returns "" for another error: 409, the name is
// in use; 400 about the name, the rule.
function wordNameError(e, name) {
  if (e.status === 409) return t("users.nameTaken", { name: name.trim().toLowerCase() });
  if (e.status === 400 && /user name/.test(e.message)) return t("users.nameBad");
  return "";
}

// userAction runs a change, redraws the list, and My profile for a
// change made there (where "profile"), and says how it went there; btn,
// the button it runs for, is busy until then (whileBusy), and a click
// while it is runs nothing and returns undefined.
function userAction(fn, okText, name, btn, where) {
  const say = sayFor(where);
  return whileBusy(btn, async () => {
    try {
      await fn();
      if (canManageUsers()) {
        await loadUsers();
        renderUsers();
      }
      if (where === "profile") await renderProfile();
      say(okText, true);
      return true;
    } catch (e) {
      say(t("users.failed", { name, error: e.message }), false);
      return false;
    }
  });
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

// userBadges are a user's badges in its panel's head: you first, then
// the role, the state and keys only.
function userBadges(u) {
  const out = isSelf(u.name) ? [badgeEl(t("users.you"), "ok")] : [];
  out.push(badgeEl(roleLabel(u.role), "mode"));
  if (u.disabled) out.push(badgeEl(t("users.state.disabled"), "bad"));
  else if (u.expires_at && Date.parse(u.expires_at) <= Date.now()) out.push(badgeEl(t("users.state.expired"), "bad"));
  else if (u.expires_at) out.push(badgeEl(t("users.state.until", { date: fmtShortTime(Date.parse(u.expires_at)) }), "warn"));
  if (!u.has_password) out.push(badgeEl(t("users.keysOnly"), "mode"));
  return out;
}

// removeUser asks in the panel, in place of its actions.
function removeUser(u, actions) {
  confirmInPlace(actions, t("users.removeConfirm", { name: u.name }), { yes: t("users.remove"), no: t("action.keep") }, () => {
    usersUI.open.delete(u.name);
    return userAction(() => api("DELETE", "api/v1/users/" + encodeURIComponent(u.name)), t("users.removed", { name: u.name }), u.name);
  }, ".remove-user");
}

// tokenRow is one session or key of a user, from its template, with
// Revoke.
function tokenRow(u, tk, where) {
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
    userAction(() => api("DELETE", "api/v1/users/" + encodeURIComponent(u.name) + "/tokens/" + tk.id), t("users.revoked", { id: tk.id }), u.name, revoke, where);
  });
  return li;
}

// fillTokens loads a user's tokens into an open panel, or My profile,
// and shows the key just created.
async function fillTokens(u, box, where) {
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
  for (const tk of tokens) ul.appendChild(tokenRow(u, tk, where));
  const key = usersUI.keys[u.name];
  const shown = box.querySelector(".user-key-box");
  shown.classList.toggle("hidden", !key);
  if (key) {
    shown.querySelector(".user-key").textContent = key;
    delete usersUI.keys[u.name];
  }
}

// wireNewKey wires a panel's New API key once, when the panel is built.
function wireNewKey(u, box, where) {
  const input = box.querySelector("input[name=label]");
  input.id = (where === "profile" ? "profile-key-label-" : "user-key-label-") + u.name;
  input.previousElementSibling.htmlFor = input.id;
  const create = box.querySelector(".create-key");
  create.addEventListener("click", () => {
    const label = input.value.trim();
    if (!label) {
      sayFor(where)(t("users.keyLabelNeeded"), false);
      return;
    }
    userAction(async () => {
      const res = await api("POST", "api/v1/users/" + encodeURIComponent(u.name) + "/tokens", { label });
      usersUI.keys[u.name] = res.token;
    }, t("users.keyCreated", { label }), u.name, create, where);
  });
}

// userPanel is one user, a detailsPanel: the head with the name, the
// login and the badges, the form (tpl-user-panel), the tokens. A panel
// built open loads its tokens at once; the toggle event that setting
// open queues finds the name in usersUI.open and loads nothing more, so
// that the tokens load once and the new key, shown once, is not hidden
// again by a second load.
function userPanel(u) {
  const d = detailsPanel({ name: u.display_name || u.name, mono: u.name, badges: userBadges(u), open: usersUI.open.has(u.name), cls: "user-panel" },
    fromTemplate("tpl-user-panel"));
  d.dataset.name = u.name;
  const tokensBox = fromTemplate("tpl-user-tokens");
  d.appendChild(tokensBox);
  wireNewKey(u, tokensBox);
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
  // changes is what Save sends: the fields that differ from the user.
  const changes = () => {
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
    return body;
  };
  const save = form.querySelector("button[type=submit]");
  sendWhenChanged(form, () => Object.keys(changes()).length > 0, { btn: save });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const body = changes();
    if (!Object.keys(body).length) {
      usersMsg(t("users.unchanged"), true);
      return;
    }
    // A password is registered by OPAQUE in the browser (login.js), one's
    // own after proving the current one; neither goes to the server.
    const { password, current_password: current } = body;
    delete body.password;
    delete body.current_password;
    const send = async () => {
      if (password) {
        if (!manage || self) Object.assign(body, await currentPassword(u, current || ""));
        Object.assign(body, await registerPassword(u.name, password));
      }
      return api("PUT", "api/v1/users/" + encodeURIComponent(u.name), body);
    };
    userAction(send, t("users.saved", { name: u.name }), u.name, save).then((ok) => {
      if (ok && self && password) loadMe();
    });
  });
  if (d.open) fillTokens(u, tokensBox);
  return d;
}

// renderProfile draws My profile for the signed-in user: the display
// name with Save, off while it is the one stored, and the own sessions
// and keys with Revoke and Create key, the parts of a panel of System
// settings → Users; the own password is login.js' (#me-password). The
// own user's routes need no capability.
async function renderProfile() {
  if (!hasProfile()) {
    clear($("profile-name"));
    clear($("profile-tokens"));
    usersUI.self = null;
    return;
  }
  const name = state.me.user.name;
  let u;
  try {
    u = await api("GET", "api/v1/users/" + encodeURIComponent(name));
  } catch (e) {
    profileMsg(t("users.failed", { name, error: e.message }), false);
    return;
  }
  usersUI.self = u;
  const form = fromTemplate("tpl-profile-form");
  const input = form.querySelector("input[name=display_name]");
  input.id = "profile-display-name";
  input.previousElementSibling.htmlFor = input.id;
  input.value = u.display_name || "";
  const save = form.querySelector("button[type=submit]");
  const changed = () => input.value.trim() !== (u.display_name || "");
  sendWhenChanged(form, changed, { btn: save });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    if (!changed()) return;
    const display = input.value.trim();
    userAction(() => api("PUT", "api/v1/users/" + encodeURIComponent(name), { display_name: display }), t("users.saved", { name }), name, save, "profile").then((ok) => {
      if (ok) loadMe(); // the 👤 menu names the user by it
    });
  });
  const tokens = fromTemplate("tpl-user-tokens");
  wireNewKey(u, tokens, "profile");
  $("profile-name").replaceChildren(form);
  $("profile-tokens").replaceChildren(tokens);
  fillTokens(u, tokens, "profile");
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
  const nameMsg = form.querySelector(".name-msg");
  nameMsg.id = "user-new-name-msg";
  const checkName = watchName(field("name"), nameMsg, () => usersUI.users.map((u) => u.name));
  form.querySelector(".cancel-user").addEventListener("click", () => {
    clear($("user-new"));
    $("user-add").classList.remove("hidden");
  });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    if (!checkName()) {
      field("name").focus();
      return;
    }
    const body = { name: field("name").value.trim(), role: field("role").value };
    const display = field("display_name").value.trim();
    if (display) body.display_name = display;
    const password = field("password").value;
    const end = isoOf(field("expires").value);
    if (end) body.expires_at = end;
    const create = async () => {
      if (password) Object.assign(body, await registerPassword(body.name, password));
      try {
        await api("POST", "api/v1/users", body);
      } catch (e) {
        const worded = wordNameError(e, body.name);
        if (worded) {
          checkName.say(worded);
          e.message = worded;
        }
        throw e;
      }
    };
    userAction(create, t("users.created", { name: body.name }), body.name, form.querySelector("button[type=submit]")).then((ok) => {
      if (ok) {
        clear($("user-new"));
        $("user-add").classList.remove("hidden");
      }
    });
  });
  return form;
}

// --- the password sign-in (auth.password_login) ------------------------

// loadPasswordLogin reads whether the server takes the password sign-in,
// for a holder of settings, who alone sees the switch.
async function loadPasswordLogin() {
  if (!hasCap("settings")) return;
  try {
    const all = (await api("GET", "api/v1/settings")) || {};
    const v = all["auth.password_login"];
    $("password-login").checked = !v || v.value !== "false";
  } catch (e) {
    // the switch stays as it was
  }
}

// renderPasswordLoginHint says how many users still have a password the
// server checks (auth_version 1): with the switch off they cannot sign
// in until a password is set for them here, by OPAQUE.
function renderPasswordLoginHint() {
  const left = usersUI.users.filter((u) => u.has_password && u.auth_version === 1).length;
  $("password-login-hint").textContent = left ? tn("users.passwordLoginHint", left) : t("users.passwordLoginNone");
}

// setPasswordLogin turns the password sign-in on or off.
async function setPasswordLogin() {
  const box = $("password-login");
  const on = box.checked;
  try {
    await api("PUT", "api/v1/settings", { "auth.password_login": on });
    usersMsg(t(on ? "users.passwordLoginOn" : "users.passwordLoginOff"), true);
  } catch (e) {
    box.checked = !on;
    usersMsg(t("users.failed", { name: "", error: e.message }), false);
  }
}

// renderUsers draws System settings → Users from usersUI.users.
function renderUsers() {
  const box = $("users-list");
  if (!box || !canManageUsers()) return;
  renderPasswordLoginHint();
  clear(box);
  if (!usersUI.users.length) box.appendChild(textEl("p", t("users.none"), "muted small"));
  for (const u of usersUI.users) box.appendChild(userPanel(u));
}

function initUsers() {
  $("password-login").addEventListener("change", setPasswordLogin);
  $("user-add").addEventListener("click", () => {
    $("user-add").classList.add("hidden");
    const box = $("user-new");
    clear(box);
    const form = newUserForm();
    box.appendChild(form);
    form.querySelector("input[name=name]").focus();
  });
  onView("system", () => {
    usersMsg("");
    if (!canManageUsers()) return;
    loadUsers()
      .then(renderUsers)
      .then(loadPasswordLogin)
      .catch((e) => usersMsg(t("users.failed", { name: "", error: e.message }), false));
  });
  onView("profile", () => {
    profileMsg("");
    renderProfile();
  });
}

document.addEventListener("DOMContentLoaded", initUsers);

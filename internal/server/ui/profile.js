// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, My profile: the display name with Save, the own
// sessions and API keys with Revoke and a new key shown once, the help
// of its groups, and the run of a change made there. System settings →
// Users draws the same sessions and keys as a part (ui/src/admin/).
"use strict";

const usersUI = {
  self: null, // the signed-in user, as My profile shows it
  keys: {}, // per name: the key just created, shown once
};

function profileMsg(text, ok) {
  showResult($("profile-msg"), text, ok);
}

// sayFor is where a change says how it went: in My profile, where the
// rows of this script are drawn now; System settings → Users is a part
// with its own message line.
function sayFor() {
  return profileMsg;
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

// tokenRow is one session or key of a user, from its template, with
// Revoke; a session made from a key names it by its label, from labels,
// the list's keys by id (POST /api/v1/login/key).
function tokenRow(u, tk, where, labels) {
  const li = fromTemplate("tpl-token-row");
  li.dataset.id = String(tk.id);
  li.querySelector(".token-kind").textContent = t("users.kind." + tk.kind);
  li.querySelector(".token-label").textContent = tk.label || "";
  const fromKey = li.querySelector(".token-key");
  fromKey.textContent = tk.key_id ? t("users.fromKey", { label: (labels && labels.get(tk.key_id)) || "#" + tk.key_id }) : "";
  fromKey.classList.toggle("hidden", !tk.key_id);
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
  const labels = new Map(tokens.map((tk) => [tk.id, tk.label || t("users.kind." + tk.kind)]));
  for (const tk of tokens) ul.appendChild(tokenRow(u, tk, where, labels));
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

// renderProfile draws My profile for the signed-in user: the display
// name with Save, off while it is the one stored, and the own sessions
// and keys with Revoke and Create key, the parts of a panel of System
// settings → Users; the own password is login.js' (#me-password). The
// own user's routes need no capability.
async function renderProfile() {
  if (!hasProfile()) {
    clear($("profile-name"));
    clear($("profile-tokens"));
    clear($("profile-identities"));
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
  addProfileHelp(save, "profileName", t("users.displayName"), form);
  const tokens = fromTemplate("tpl-user-tokens");
  wireNewKey(u, tokens, "profile");
  addProfileHelp(tokens.querySelector(".create-key"), "profileTokens", t("users.tokens"), tokens);
  // the password's form is static markup: its help made again, in the
  // language now chosen
  const pwForm = $("me-password-form");
  for (const old of pwForm.querySelectorAll(".help-btn, .help-text")) old.remove();
  addProfileHelp(pwForm.querySelector("button[type=submit]"), "profilePassword", t("me.changePassword"), pwForm);
  $("profile-name").replaceChildren(form);
  $("profile-tokens").replaceChildren(tokens);
  fillTokens(u, tokens, "profile");
  await renderProfileIdentities(u); // providers.js
  if (oidcLinked) {
    // back from a link (oidcReturn, login.js): shown to confirm, or why
    // there is none
    const back = oidcLinked;
    oidcLinked = null;
    if (back.confirm) await renderLinkConfirm(u, back.confirm); // providers.js
    else profileMsg(t("profile.linkError." + back.error), false);
  }
}

// addProfileHelp puts My profile's ⓘ (helpParts, app.js) behind a group's
// last button, and its explanation at the end of the group (AP Oidc
// v1.2, D37-C).
function addProfileHelp(after, key, label, group) {
  const { btn, text } = helpParts(key, label);
  after.after(btn);
  group.appendChild(text);
}

// initProfile draws My profile each time the personal page is shown.
function initProfile() {
  onView("me", () => {
    profileMsg("");
    if (hasProfile()) renderProfile(); // login.js
  });
}

document.addEventListener("DOMContentLoaded", initProfile);

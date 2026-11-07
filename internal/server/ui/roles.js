// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, Settings → Roles: the roles with their sets, each
// in a panel with its display name, a switch per capability, Disable
// and Remove (/api/v1/roles); the form that creates a role; and the
// switch per capability that disables it for every role but owner
// (/api/v1/capabilities). Drawn when the settings open and after every
// change.
"use strict";

const rolesUI = {
  roles: [],
  capabilities: [], // every capability with whether it is disabled
  open: new Set(),
};

function rolesMsg(text, ok) {
  showResult($("roles-msg"), text, ok);
}

// capLabel is what a capability allows, translated.
function capLabel(c) {
  return t("cap." + c);
}

async function loadRoles() {
  if (!hasCap("roles")) return;
  const res = (await api("GET", "api/v1/roles")) || {};
  rolesUI.roles = res.roles || [];
  rolesUI.capabilities = res.capabilities || [];
  usersUI.roles = rolesUI.roles;
}

// roleAction runs a change, redraws the roles and says how it went;
// btn, the button it runs for, is busy until then (whileBusy).
function roleAction(fn, okText, key, btn) {
  return whileBusy(btn, async () => {
    try {
      await fn();
      await loadRoles();
      renderRoles();
      rolesMsg(okText, true);
      return true;
    } catch (e) {
      rolesMsg(t("roles.failed", { role: key, error: e.message }), false);
      return false;
    }
  });
}

// capGrid is the switches of a role's set: one per capability, named
// with the role's key, every one on and fixed for owner; a capability
// disabled for every role is marked.
function capGrid(set, roleKey, fixed) {
  const grid = elem("div", "cap-grid");
  for (const c of rolesUI.capabilities) {
    const sw = switchEl(capLabel(c.key), fixed || set.includes(c.key), null, "cap:" + c.key);
    sw.title = c.key;
    if (fixed) sw.querySelector("input").disabled = true;
    if (c.disabled) sw.appendChild(badgeEl(t("roles.capOff"), "warn"));
    grid.appendChild(sw);
  }
  return grid;
}

// setOf reads the capabilities switched on in a grid.
function setOf(grid) {
  return [...grid.querySelectorAll("input[name^='cap:']")].filter((i) => i.checked).map((i) => i.name.slice(4));
}

// removeRole asks in the panel, in place of its actions.
function removeRole(r, actions) {
  confirmInPlace(actions, t("roles.removeConfirm", { role: r.key }), { yes: t("roles.remove"), no: t("action.keep") }, () => {
    rolesUI.open.delete(r.key);
    return roleAction(() => api("DELETE", "api/v1/roles/" + encodeURIComponent(r.key)), t("roles.removed", { role: r.key }), r.key);
  }, ".remove-role");
}

// rolePanel is one role, from its template (tpl-role-panel).
function rolePanel(r) {
  const d = fromTemplate("tpl-role-panel");
  d.dataset.key = r.key;
  d.open = rolesUI.open.has(r.key);
  const sum = d.querySelector("summary");
  sum.querySelector(".panel-name").textContent = roleLabel(r.key, r.name);
  sum.querySelector(".mono").textContent = r.key;
  const badges = [];
  if (r.builtin) badges.push(badgeEl(t("roles.builtin"), "mode"));
  if (r.disabled) badges.push(badgeEl(t("roles.disabled"), "bad"));
  badges.push(badgeEl(tn("roles.users", r.users), "mode"));
  sum.querySelector(".panel-badges").append(...badges);
  d.addEventListener("toggle", () => {
    if (d.open) rolesUI.open.add(r.key);
    else rolesUI.open.delete(r.key);
  });
  const owner = r.key === "owner";
  const form = d.querySelector("form");
  const name = form.querySelector("input[name=name]");
  name.id = "role-" + r.key + "-name";
  name.previousElementSibling.htmlFor = name.id;
  name.value = r.name || "";
  name.disabled = owner;
  const grid = capGrid(r.capabilities || [], r.key, owner);
  form.querySelector(".role-caps").appendChild(grid);
  if (owner) form.querySelector(".role-owner-note").classList.remove("hidden");
  const actions = form.querySelector(".actions");
  const toggle = actions.querySelector(".disable-role");
  toggle.textContent = t(r.disabled ? "roles.enable" : "roles.disable");
  toggle.classList.toggle("hidden", owner);
  toggle.addEventListener("click", () => {
    roleAction(() => api("PUT", "api/v1/roles/" + encodeURIComponent(r.key) + "/disabled", { disabled: !r.disabled }),
      t(r.disabled ? "roles.enabledNow" : "roles.disabledNow", { role: r.key }), r.key, toggle);
  });
  const remove = actions.querySelector(".remove-role");
  remove.classList.toggle("hidden", r.builtin || r.users > 0);
  remove.addEventListener("click", () => removeRole(r, actions));
  const save = actions.querySelector("button[type=submit]");
  save.classList.toggle("hidden", owner);
  // changes is what Save sends: the name and the set, where they differ.
  const changes = () => {
    const body = {};
    const n = name.value.trim();
    if (n !== (r.name || "")) body.name = n;
    const set = setOf(grid);
    if (set.slice().sort().join(",") !== (r.capabilities || []).slice().sort().join(",")) body.capabilities = set;
    return body;
  };
  sendWhenChanged(form, () => !owner && Object.keys(changes()).length > 0, { btn: save });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    if (owner) return;
    const body = changes();
    if (!Object.keys(body).length) {
      rolesMsg(t("users.unchanged"), true);
      return;
    }
    roleAction(() => api("PUT", "api/v1/roles/" + encodeURIComponent(r.key), body), t("roles.saved", { role: r.key }), r.key, save);
  });
  return d;
}

// newRoleForm is the form that creates a role, from its template.
function newRoleForm() {
  const form = fromTemplate("tpl-role-new");
  for (const input of form.querySelectorAll(".field.wide > input")) {
    input.id = "role-new-" + input.name;
    input.previousElementSibling.htmlFor = input.id;
  }
  const grid = capGrid(["read"], "new", false);
  form.querySelector(".role-caps").appendChild(grid);
  form.querySelector(".cancel-role").addEventListener("click", () => {
    clear($("role-new"));
    $("role-add").classList.remove("hidden");
  });
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    const key = form.querySelector("input[name=key]").value.trim().toLowerCase();
    const body = { key, capabilities: setOf(grid) };
    const n = form.querySelector("input[name=name]").value.trim();
    if (n) body.name = n;
    roleAction(() => api("POST", "api/v1/roles", body), t("roles.created", { role: key }), key, form.querySelector("button[type=submit]")).then((ok) => {
      if (ok) {
        clear($("role-new"));
        $("role-add").classList.remove("hidden");
      }
    });
  });
  return form;
}

// renderCapabilities draws the switch per capability that enables it
// for every role, or disables it for every role but owner.
function renderCapabilities() {
  const box = $("caps-list");
  clear(box);
  const grid = elem("div", "cap-grid");
  for (const c of rolesUI.capabilities) {
    // The switch is off while its PUT runs, and goes back where it was
    // when the server refuses.
    const sw = switchEl(capLabel(c.key), !c.disabled, (on) => {
      const cb = sw.control;
      cb.disabled = true;
      roleAction(() => api("PUT", "api/v1/capabilities/" + encodeURIComponent(c.key), { disabled: !on }),
        t(on ? "roles.capEnabled" : "roles.capDisabled", { cap: capLabel(c.key) }), c.key).then((ok) => {
        cb.disabled = false;
        if (!ok) cb.checked = !on;
      });
    }, "global:" + c.key);
    sw.title = c.key;
    grid.appendChild(sw);
  }
  box.appendChild(grid);
}

// renderRoles draws Settings → Roles from rolesUI.
function renderRoles() {
  const box = $("roles-list");
  if (!box || !hasCap("roles")) return;
  clear(box);
  for (const r of rolesUI.roles) box.appendChild(rolePanel(r));
  renderCapabilities();
}

function initRoles() {
  $("role-add").addEventListener("click", () => {
    $("role-add").classList.add("hidden");
    const box = $("role-new");
    clear(box);
    const form = newRoleForm();
    box.appendChild(form);
    form.querySelector("input[name=key]").focus();
  });
  $("view-settings").addEventListener("click", () => {
    rolesMsg("");
    if (!hasCap("roles")) return;
    loadRoles().then(renderRoles).catch((e) => rolesMsg(t("roles.failed", { role: "", error: e.message }), false));
  });
}

document.addEventListener("DOMContentLoaded", initRoles);

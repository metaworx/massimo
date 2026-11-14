// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

// The Massimo web UI, System settings → Integrations (docs/api-v1.md,
// "Integrations"), with the capability integrations: the requests that
// wait, each with its code, where it came from and whether the LAN saw its
// installation, approved with a maximum or refused; each integration with
// its maximum, its state, a new installation to accept, Revoke, and its
// users, each linked to a Massimo user or acting as its own; the Home
// Assistant installations seen on the LAN; and, with settings, the switch
// that lets integrations ask at all.
"use strict";

const integrationsUI = {
  list: [], // GET /api/v1/integrations: the requests and the integrations
  details: {}, // per uuid: GET /api/v1/integrations/{uuid}, with its users
  found: [], // GET /api/v1/integrations/found
  looks: false, // the server looks at the LAN
  users: null, // the Massimo users to link to, where they may be read
  open: new Set(), // the uuids whose panel is open
};

// INTEGRATION_CEILING is the role integration's set, what an integration
// may have at most (integrationCeiling in internal/server/auth.go).
const INTEGRATION_CEILING = ["read", "preferences", "setpoint", "lock", "configure", "rename", "structure", "pair", "discover", "backup", "restore",
  "guests", "cubes", "cubes.reboot", "settings", "update", "restart", "reboot", "audit"];

function integrationsMsg(text, ok) {
  showResult($("integrations-msg"), text, ok);
}

// loadIntegrations reads the requests and the integrations, and the
// users of those whose panel is open.
async function loadIntegrations() {
  if (!hasCap("integrations")) return;
  integrationsUI.list = (await api("GET", "api/v1/integrations")) || [];
  for (const i of integrationsUI.list) {
    if (integrationsUI.open.has(i.uuid)) await loadIntegrationDetails(i.uuid);
  }
}

async function loadIntegrationDetails(uuid) {
  integrationsUI.details[uuid] = await api("GET", "api/v1/integrations/" + encodeURIComponent(uuid));
}

// loadSeenIntegrations reads what the last look at the LAN saw; refresh looks now.
async function loadSeenIntegrations(refresh) {
  const f = (await api("GET", "api/v1/integrations/found" + (refresh ? "?refresh=1" : ""))) || {};
  integrationsUI.found = f.found || [];
  integrationsUI.looks = !!f.looks;
}

// loadLinkUsers reads the Massimo users an integration's user may be
// linked to: none of an integration; null where they may not be read.
async function loadLinkUsers() {
  if (!canManageUsers()) {
    integrationsUI.users = null;
    return;
  }
  try {
    integrationsUI.users = ((await api("GET", "api/v1/users")) || []).filter((u) => !u.integration);
  } catch (e) {
    integrationsUI.users = null;
  }
}

// integrationAction runs a change, reads the integrations again, redraws
// them and says how it went; btn is busy until then.
function integrationAction(fn, okText, name, btn) {
  return whileBusy(btn, async () => {
    try {
      await fn();
      await loadIntegrations();
      renderIntegrations();
      integrationsMsg(okText, true);
      return true;
    } catch (e) {
      integrationsMsg(t("integrations.failed", { name, error: e.message }), false);
      return false;
    }
  });
}

function integrationPath(i, rest) {
  return "api/v1/integrations/" + encodeURIComponent(i.uuid) + (rest || "");
}

// capsBox is a switch per capability of the ceiling, those of selected
// on; its value() is the capabilities switched on.
function capsBox(selected, id) {
  const box = elem("fieldset", "integration-caps");
  box.appendChild(textEl("legend", t("integrations.max")));
  const on = new Set(selected);
  for (const c of INTEGRATION_CEILING) {
    const sw = switchEl(capLabel(c), on.has(c), null, c); // roles.js
    sw.querySelector("input").id = "integration-cap-" + id + "-" + c;
    box.appendChild(elem("div", "field", sw));
  }
  box.value = () => [...box.querySelectorAll("input:checked")].map((cb) => cb.name);
  return box;
}

// requestPanel is a request waiting: its code to compare, where it came
// from, whether the LAN saw its installation, the maximum, what it was
// registered as before, Refuse and Approve.
function requestPanel(r) {
  const badges = [badgeEl(t(r.seen ? "integrations.seen" : "integrations.notSeen"), r.seen ? "ok" : "warn")];
  const body = elem("div", "integration-request-body");
  const expires = Date.parse(r.request_expires || "");
  body.appendChild(kvList([
    [t("integrations.code"), r.request_code, true],
    [t("integrations.from"), r.request_remote || "", true],
    [t("integrations.instance"), r.instance_id, true],
    [t("integrations.url"), r.url || ""],
    [t("integrations.version"), r.version || ""],
    [t("integrations.expires"), isNaN(expires) ? "" : fmtShortTime(expires)],
  ].filter(([, v]) => v)));
  const caps = capsBox(INTEGRATION_CEILING, r.uuid);
  body.appendChild(caps);
  // an integration of the same kind registered before, the same
  // installation first: approved as it, it keeps its users and links
  const earlier = integrationsUI.list.filter((i) => i.kind === r.kind && i.state !== "pending" && i.state !== "refused")
    .sort((a, b) => (b.instance_id === r.instance_id) - (a.instance_id === r.instance_id));
  const attach = fillOptions(document.createElement("select"),
    [["", t("integrations.attachNone")], ...earlier.map((i) => [i.uuid, i.name + " (" + i.key + ")"])]);
  if (earlier.length && earlier[0].instance_id === r.instance_id) attach.value = earlier[0].uuid;
  const attachRow = fieldRow(t("integrations.attach"), attach, { wide: true, hint: t("integrations.attachHint") });
  attachRow.classList.toggle("hidden", !earlier.length);
  body.appendChild(attachRow);
  const refuse = button(t("integrations.refuse"), "refuse-integration");
  const approve = button(t("integrations.approve"), "primary approve-integration");
  const actions = elem("div", "actions", elem("span", "actions-left", refuse), elem("span", "actions-right", approve));
  body.appendChild(actions);
  refuse.addEventListener("click", () => {
    confirmInPlace(actions, t("integrations.refuseConfirm", { name: r.name }), { yes: t("integrations.refuse"), no: t("action.keep") },
      () => integrationAction(() => api("POST", integrationPath(r, "/refuse")), t("integrations.refused", { name: r.name }), r.name), ".refuse-integration");
  });
  approve.addEventListener("click", () => {
    const req = { max_caps: caps.value() };
    if (attach.value) req.attach = attach.value;
    integrationAction(() => api("POST", integrationPath(r, "/approve"), req), t("integrations.approved", { name: r.name }), r.name, approve);
  });
  const d = detailsPanel({ name: r.name || r.kind, mono: r.request_code, badges, open: true, cls: "integration-request" }, body);
  d.dataset.uuid = r.uuid;
  return d;
}

// linkOptions are the Massimo users an integration's user may act as:
// its own user first, then the users, the proposal marked.
function linkOptions(u) {
  const out = [["", t("integrations.ownUser", { name: u.own_user })]];
  for (const m of integrationsUI.users || []) {
    const label = (m.display_name ? m.display_name + " (" + m.name + ")" : m.name);
    out.push([m.name, m.name === u.proposal ? t("integrations.proposed", { name: label }) : label]);
  }
  if (u.linked && !out.some(([v]) => v === u.user)) out.push([u.user, u.user]);
  return out;
}

// integrationUserRow is one of an integration's users: its name there,
// its level and when it acted last, and who it acts as, with Link.
function integrationUserRow(i, u) {
  const li = elem("li", "integration-user");
  li.dataset.id = u.id;
  const system = u.id === "system";
  const name = system ? t("integrations.system") : u.name || u.id;
  const head = elem("div", "integration-user-head", textEl("span", name, "integration-user-name"));
  if (!system) head.appendChild(textEl("span", u.id.slice(0, 8), "mono muted small"));
  if (u.system_generated) head.appendChild(badgeEl(t("integrations.systemGenerated"), "mode"));
  if (u.last_level) head.appendChild(badgeEl(t("integrations.level." + u.last_level), "mode"));
  if (u.linked) head.appendChild(badgeEl(t("integrations.linkedBadge"), "ok"));
  li.appendChild(head);
  const when = elem("p", "muted small");
  const age = ageEl(u.last_used_at, false, true);
  if (age) when.append(t("integrations.lastUsed") + " ", age);
  else when.textContent = t("users.neverUsed");
  li.appendChild(when);
  if (system || !integrationsUI.users) {
    li.appendChild(textEl("p", t("integrations.actsAsUser", { name: u.user }), "small"));
    return li;
  }
  const select = fillOptions(document.createElement("select"), linkOptions(u));
  select.value = u.linked ? u.user : u.proposal || "";
  const link = button(t("integrations.link"), "link-integration-user");
  link.setAttribute("aria-label", t("integrations.linkLabel", { name }));
  link.addEventListener("click", () => {
    const user = select.value || null;
    const ok = user ? t("integrations.linked", { name, user }) : t("integrations.unlinked", { name });
    integrationAction(() => api("PUT", integrationPath(i, "/users/" + encodeURIComponent(u.id)), { user }), ok, i.name, link);
  });
  li.appendChild(fieldRow(t("integrations.actsAs"), elem("span", "inline", select, link), { wide: true,
    hint: !u.linked && u.proposal ? t("integrations.proposalHint", { name: u.proposal }) : "" }));
  return li;
}

// fillIntegrationUsers draws an integration's users into its panel.
function fillIntegrationUsers(i, ul) {
  clear(ul);
  const det = integrationsUI.details[i.uuid];
  if (!det) return;
  const users = det.users || [];
  if (!users.length) ul.appendChild(textEl("li", t("integrations.usersNone"), "muted small"));
  for (const u of users) ul.appendChild(integrationUserRow(i, u));
}

// integrationPanel is an integration: what it is, a new installation
// to accept, Enabled, its maximum, its users, Revoke.
function integrationPanel(i) {
  const badges = [];
  if (i.state === "disabled") badges.push(badgeEl(t("integrations.disabled"), "bad"));
  if (i.pending_instance) badges.push(badgeEl(t("integrations.pendingInstance"), "warn"));
  const body = elem("div", "integration-body");
  const issued = Date.parse(i.key_issued_at || "");
  body.appendChild(kvList([
    [t("integrations.key"), i.key, true],
    [t("integrations.instance"), i.instance_id, true],
    [t("integrations.url"), i.url || ""],
    [t("integrations.version"), i.version || ""],
    [t("integrations.keyIssued"), isNaN(issued) ? t("integrations.keyNotIssued") : fmtShortTime(issued)],
    [t("integrations.seenLabel"), t(i.seen ? "integrations.seen" : "integrations.notSeen")],
  ].filter(([, v]) => v)));
  if (i.pending_instance) {
    const accept = button(t("integrations.acceptInstance"), "accept-instance");
    accept.addEventListener("click", () =>
      integrationAction(() => api("POST", integrationPath(i, "/accept-instance")), t("integrations.accepted", { name: i.name }), i.name, accept));
    body.appendChild(elem("div", "notice integration-instance", textEl("p", t("integrations.pendingInstanceText", { id: i.pending_instance })),
      elem("div", "actions", accept)));
  }
  const enabled = switchEl(t("integrations.enabled"), i.state === "active", (on) => {
    integrationAction(() => api("PUT", integrationPath(i), { state: on ? "active" : "disabled" }),
      t(on ? "integrations.enabledOn" : "integrations.enabledOff", { name: i.name }), i.name);
  }, "enabled");
  enabled.querySelector("input").id = "integration-enabled-" + i.uuid;
  body.appendChild(elem("div", "field", enabled));
  const caps = capsBox(i.max_caps || [], i.uuid);
  body.appendChild(caps);
  const saveCaps = button(t("integrations.saveMax"), "save-max");
  saveCaps.addEventListener("click", () =>
    integrationAction(() => api("PUT", integrationPath(i), { max_caps: caps.value() }), t("integrations.saved", { name: i.name }), i.name, saveCaps));
  body.appendChild(elem("div", "actions", elem("span", "actions-right", saveCaps)));
  body.appendChild(textEl("h4", t("integrations.users")));
  const ul = elem("ul", "integration-users");
  body.appendChild(ul);
  fillIntegrationUsers(i, ul);
  const revoke = button(t("integrations.revoke"), "revoke-integration");
  const actions = elem("div", "actions", elem("span", "actions-left", revoke));
  body.appendChild(actions);
  revoke.addEventListener("click", () => {
    confirmInPlace(actions, t("integrations.revokeConfirm", { name: i.name }), { yes: t("integrations.revoke"), no: t("action.keep") },
      () => integrationAction(() => api("DELETE", integrationPath(i)), t("integrations.revoked", { name: i.name }), i.name), ".revoke-integration");
  });
  const d = detailsPanel({ name: i.name, mono: i.key, badges, open: integrationsUI.open.has(i.uuid), cls: "integration-panel" }, body);
  d.dataset.uuid = i.uuid;
  d.addEventListener("toggle", async () => {
    if (!d.open) {
      integrationsUI.open.delete(i.uuid);
      return;
    }
    integrationsUI.open.add(i.uuid);
    try {
      await loadIntegrationDetails(i.uuid);
      fillIntegrationUsers(i, ul);
    } catch (e) {
      integrationsMsg(t("integrations.failed", { name: i.name, error: e.message }), false);
    }
  });
  return d;
}

// renderSeenIntegrations draws the installations seen on the LAN.
function renderSeenIntegrations() {
  const box = $("integrations-found");
  clear(box);
  $("integrations-look").classList.toggle("hidden", !integrationsUI.looks);
  if (!integrationsUI.looks) {
    box.appendChild(textEl("p", t("integrations.foundOff"), "muted small"));
    return;
  }
  if (!integrationsUI.found.length) {
    box.appendChild(textEl("p", t("integrations.foundNone"), "muted small"));
    return;
  }
  const ul = elem("ul", "integrations-found-list");
  for (const f of integrationsUI.found) {
    const li = elem("li", "", textEl("span", f.location || f.name, "integration-found-name"), " ",
      textEl("span", f.url || f.address, "mono muted small"), " ",
      badgeEl(t(f.integration ? "integrations.registered" : "integrations.notRegistered"), f.integration ? "ok" : "mode"));
    ul.appendChild(li);
  }
  box.appendChild(ul);
}

// renderIntegrations draws System settings → Integrations.
function renderIntegrations() {
  const reqBox = $("integration-requests-list");
  const box = $("integrations-list");
  if (!box || !hasCap("integrations")) return;
  const waiting = integrationsUI.list.filter((i) => i.state === "pending");
  const registered = integrationsUI.list.filter((i) => i.state === "active" || i.state === "disabled");
  clear(reqBox);
  if (waiting.length) reqBox.appendChild(textEl("h4", t("integrations.waiting")));
  for (const r of waiting) reqBox.appendChild(requestPanel(r));
  clear(box);
  if (!registered.length) box.appendChild(textEl("p", t("integrations.none"), "muted small"));
  for (const i of registered) box.appendChild(integrationPanel(i));
  renderSeenIntegrations();
  const badges = [];
  if (waiting.length) badges.push(badgeEl(tn("integrations.badge.waiting", waiting.length), "warn"));
  if (registered.length) badges.push(badgeEl(String(registered.length), "mode"));
  sectionBadges($("sys-integrations"), badges);
}

// loadRequestsSwitch reads whether integrations may ask, for a holder
// of settings, who alone sees the switch.
async function loadRequestsSwitch() {
  if (!hasCap("settings")) return;
  try {
    const all = (await api("GET", "api/v1/settings")) || {};
    const v = all["integrations.requests"];
    $("integration-requests").checked = !v || v.value !== "false";
  } catch (e) {
    // the switch stays as it was
  }
}

async function setRequestsSwitch() {
  const box = $("integration-requests");
  const on = box.checked;
  try {
    await api("PUT", "api/v1/settings", { "integrations.requests": on });
    integrationsMsg(t(on ? "integrations.requestsOn" : "integrations.requestsOff"), true);
  } catch (e) {
    box.checked = !on;
    integrationsMsg(t("integrations.failed", { name: "", error: e.message }), false);
  }
}

// refreshIntegrations reads everything the section shows and draws it.
async function refreshIntegrations() {
  await loadIntegrations();
  await loadLinkUsers();
  await loadSeenIntegrations(false);
  renderIntegrations();
}

function initIntegrations() {
  $("integration-requests").addEventListener("change", setRequestsSwitch);
  $("integrations-look").addEventListener("click", () => {
    const btn = $("integrations-look");
    whileBusy(btn, async () => {
      try {
        await loadSeenIntegrations(true);
        await loadIntegrations();
        renderIntegrations();
        integrationsMsg(tn("integrations.lookDone", integrationsUI.found.length), true);
      } catch (e) {
        integrationsMsg(t("integrations.failed", { name: "", error: e.message }), false);
      }
    });
  });
  onView("system", () => {
    integrationsMsg("");
    if (!hasCap("integrations")) return;
    refreshIntegrations()
      .then(loadRequestsSwitch)
      .catch((e) => integrationsMsg(t("integrations.failed", { name: "", error: e.message }), false));
  });
}

document.addEventListener("DOMContentLoaded", initIntegrations);

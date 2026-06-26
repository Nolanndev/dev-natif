"use strict";
/* dev-natif Console — vanilla SPA consuming /api/v1. No build step. */

const API = "/api/v1";
let token = localStorage.getItem("natif_token") || "";
let tokenExp = localStorage.getItem("natif_token_exp") || "";
let currentUser = localStorage.getItem("natif_user") || "";

function setToken(t, exp, user) {
  token = t; tokenExp = exp || ""; currentUser = user || currentUser;
  localStorage.setItem("natif_token", token);
  localStorage.setItem("natif_token_exp", tokenExp);
  if (user) localStorage.setItem("natif_user", user);
}
function clearToken() {
  token = ""; tokenExp = ""; currentUser = "";
  localStorage.removeItem("natif_token");
  localStorage.removeItem("natif_token_exp");
  localStorage.removeItem("natif_user");
}

/* ----------------------------- helpers ----------------------------- */
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
const esc = (s) =>
  String(s ?? "").replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
const shortId = (s) => (s ? String(s).slice(0, 8) : "");
const fmtDate = (s) => {
  if (!s) return "—";
  const d = new Date(s);
  return isNaN(d) ? "—" : d.toLocaleString();
};

async function api(method, path, body) {
  const headers = { "Content-Type": "application/json" };
  if (token) headers["Authorization"] = "Bearer " + token;
  const res = await fetch(API + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401 && path !== "/auth/login") {
    clearToken();
    showLogin();
    throw new Error("Session expirée, reconnectez-vous");
  }
  if (res.status === 204) return null;
  const txt = await res.text();
  let data = null;
  try { data = txt ? JSON.parse(txt) : null; } catch { data = txt; }
  if (!res.ok) {
    const msg = data && data.error ? data.error : "HTTP " + res.status;
    throw new Error(msg);
  }
  return data;
}

function toast(msg, ok = true) {
  const t = document.createElement("div");
  t.className = "toast " + (ok ? "ok" : "err");
  t.innerHTML = `<span class="dot"></span><span class="msg">${esc(msg)}</span>`;
  $("#toasts").appendChild(t);
  setTimeout(() => {
    t.style.transition = "opacity .25s, transform .25s";
    t.style.opacity = "0";
    t.style.transform = "translateX(12px)";
    setTimeout(() => t.remove(), 250);
  }, 3600);
}

function statusBadge(status) {
  const label = {
    running: "running",
    "partially-running": "partiel",
    "not-running": "arrêté",
    failed: "échec",
    pending: "en attente",
  }[status] || status || "—";
  return `<span class="badge ${esc(status || "neutral")}"><span class="dot"></span>${esc(label)}</span>`;
}

const ICON = {
  trash: `<svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"/></svg>`,
  plus: `<svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14M5 12h14"/></svg>`,
  up: `<svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 12l7-7 7 7M12 5v14"/></svg>`,
  down: `<svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 12l-7 7-7-7M12 19V5"/></svg>`,
  refresh: `<svg class="ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-3-6.7L21 8M21 3v5h-5"/></svg>`,
};

/* ----------------------------- modal ----------------------------- */
let pollTimer = null;
function clearPoll() { if (pollTimer) { clearInterval(pollTimer); pollTimer = null; } }

function openModal({ title, bodyHTML, footHTML, onMount, wide }) {
  closeModal();
  const root = $("#modal-root");
  root.innerHTML = `
    <div class="backdrop" data-close>
      <div class="modal" role="dialog" aria-modal="true" ${wide ? 'style="width:min(820px,100%)"' : ""}>
        <div class="modal-head"><h2>${esc(title)}</h2><div class="spacer"></div>
          <button class="icon-btn" data-close aria-label="Fermer">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12"/></svg></button>
        </div>
        <div class="modal-body">${bodyHTML}</div>
        ${footHTML ? `<div class="modal-foot">${footHTML}</div>` : ""}
      </div>
    </div>`;
  root.querySelectorAll("[data-close]").forEach((b) =>
    b.addEventListener("click", (e) => { if (e.target === b) closeModal(); })
  );
  const esckey = (e) => { if (e.key === "Escape") { closeModal(); document.removeEventListener("keydown", esckey); } };
  document.addEventListener("keydown", esckey);
  if (onMount) onMount(root);
}
function closeModal() { $("#modal-root").innerHTML = ""; }

/* ----------------------------- router ----------------------------- */
const META = {
  projects: ["Projets", "Vos descriptions d'infrastructure"],
  deployments: ["Déploiements", "Instanciations sur le Docker Engine"],
  images: ["Images", "Récupérer ou construire des images"],
  servers: ["Serveurs", "Cibles Docker Engine"],
};
let view = "projects";
let route = { name: "projects" };

// Navigation drives the URL hash; applyRoute() is the single source of truth.
function go(name, params = {}) {
  const h = params.id ? `#/${name}/${params.id}` : `#/${name}`;
  if (location.hash === h) applyRoute();
  else location.hash = h;
}

function applyRoute() {
  const parts = location.hash.replace(/^#\/?/, "").split("/");
  const name = ["projects", "deployments", "images", "servers"].includes(parts[0]) ? parts[0] : "projects";
  route = { name, id: parts[1] || undefined };
  view = name;
  clearPoll();
  $$("#nav button").forEach((b) => b.classList.toggle("active", b.dataset.view === name));
  $("#view-title").textContent = META[name][0];
  $("#view-sub").textContent = META[name][1];
  $("#sidebar").classList.remove("open");
  render();
}

function render() {
  if (route.name === "projects") return route.id ? renderProjectDetail(route.id) : renderProjects();
  if (route.name === "deployments") return route.id ? renderDeploymentDetail(route.id) : renderDeployments();
  if (route.name === "images") return renderImages();
  if (route.name === "servers") return renderServers();
}

function setActions(html) { $("#topbar-actions").innerHTML = html || ""; }
function loading() { $("#content").innerHTML = `<div class="panel"><div class="skel-row"><div class="skel" style="width:40%"></div></div><div class="skel-row"><div class="skel" style="width:60%"></div></div><div class="skel-row"><div class="skel" style="width:30%"></div></div></div>`; }
function emptyState(icon, title, text, btn) {
  return `<div class="panel"><div class="empty">
    <div class="ico">${icon}</div><h3>${esc(title)}</h3><p>${esc(text)}</p>${btn || ""}</div></div>`;
}

/* ============================ EVENTS & LOGS ============================ */
function evLevelClass(l) {
  return l === "error" ? "failed" : l === "warn" ? "partially-running" : "neutral";
}
function eventRow(e) {
  return `<div class="row"><div class="grow">
      <div class="name" style="font-weight:500">${esc(e.message || e.type)}</div>
      <div class="meta">${esc(fmtDate(e.created_at))} · <span class="mono">${esc(e.type)}</span></div></div>
    <span class="badge ${evLevelClass(e.level)}"><span class="dot"></span>${esc(e.level)}</span></div>`;
}
async function loadEventsInto(sel, path) {
  const box = $(sel);
  if (!box) return;
  let evs;
  try { evs = await api("GET", path); }
  catch (e) { box.innerHTML = `<div class="muted" style="padding:14px">${esc(e.message)}</div>`; return; }
  box.innerHTML = (evs && evs.length)
    ? evs.map(eventRow).join("")
    : `<div class="muted" style="padding:14px">Aucun événement pour l'instant.</div>`;
}

// openLogs shows a container's logs with a refreshable, scrollable viewer.
function openLogs(depId, cid, name) {
  openModal({
    title: `Logs · ${name}`, wide: true,
    bodyHTML: `
      <div class="toolbar" style="margin-bottom:10px">
        <label class="muted" style="font-size:12.5px">Lignes</label>
        <select class="select" id="log-tail" style="width:auto">
          <option>100</option><option selected>200</option><option>500</option><option>1000</option></select>
        <button class="btn btn-sm" id="log-refresh">${ICON.refresh} Rafraîchir</button>
      </div>
      <pre id="log-pre" class="logbox">Chargement…</pre>`,
    footHTML: `<button class="btn" data-close>Fermer</button>`,
    onMount: (root) => {
      const load = async () => {
        const pre = $("#log-pre", root); pre.textContent = "Chargement…";
        try {
          const d = await api("GET", `/deployments/${depId}/containers/${cid}/logs?tail=${$("#log-tail", root).value}`);
          pre.textContent = d.logs && d.logs.trim() ? d.logs : "(aucune sortie)";
          pre.scrollTop = pre.scrollHeight;
        } catch (e) { pre.textContent = e.message; }
      };
      $("#log-refresh", root).onclick = load;
      $("#log-tail", root).onchange = load;
      load();
    },
  });
}

/* ============================ PROJECTS ============================ */
async function renderProjects() {
  setActions(`<button class="btn btn-primary" id="new-project">${ICON.plus} Nouveau projet</button>`);
  $("#new-project").onclick = openProjectModal;
  loading();
  let projects;
  try { projects = await api("GET", "/projects"); } catch (e) { return toast(e.message, false); }
  const c = $("#content");
  if (!projects || !projects.length) {
    c.innerHTML = emptyState(
      `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><path d="M3 7l9-4 9 4-9 4-9-4z"/><path d="M3 12l9 4 9-4M3 17l9 4 9-4"/></svg>`,
      "Aucun projet", "Un projet décrit une infrastructure multi-conteneurs réutilisable, à la manière d'un fichier docker-compose.",
      `<button class="btn btn-primary" id="empty-new">${ICON.plus} Créer un projet</button>`
    );
    $("#empty-new").onclick = openProjectModal;
    return;
  }
  c.innerHTML = `<div class="panel"><div class="list">${projects.map((p) => `
    <div class="row clickable" data-open="${esc(p.id)}">
      <div class="grow"><div class="name">${esc(p.name)}</div>
        <div class="meta">${esc(p.description || "Sans description")} · créé le ${esc(fmtDate(p.created_at))}</div></div>
      <div class="actions">
        <button class="btn btn-sm" data-open="${esc(p.id)}">Ouvrir</button>
        <button class="icon-btn" data-del="${esc(p.id)}" title="Supprimer">${ICON.trash}</button>
      </div>
    </div>`).join("")}</div></div>`;
  $$("[data-open]", c).forEach((b) => (b.onclick = (e) => { e.stopPropagation(); go("projects", { id: b.dataset.open }); }));
  $$("[data-del]", c).forEach((b) => (b.onclick = async (e) => {
    e.stopPropagation();
    if (!confirm("Supprimer ce projet et tous ses éléments ?")) return;
    try { await api("DELETE", "/projects/" + b.dataset.del); toast("Projet supprimé"); renderProjects(); }
    catch (err) { toast(err.message, false); }
  }));
}

function openProjectModal() {
  openModal({
    title: "Nouveau projet",
    bodyHTML: `
      <div class="field"><label>Nom</label><input class="input" id="p-name" placeholder="ex. wordpress-prod" /></div>
      <div class="field"><label>Description</label><input class="input" id="p-desc" placeholder="optionnel" /></div>`,
    footHTML: `<button class="btn" data-close>Annuler</button><button class="btn btn-primary" id="p-save">Créer le projet</button>`,
    onMount: (root) => {
      $("#p-name", root).focus();
      $("#p-save", root).onclick = async () => {
        const name = $("#p-name", root).value.trim();
        if (!name) return toast("Le nom est requis", false);
        try {
          await api("POST", "/projects", { name, description: $("#p-desc", root).value.trim() });
          closeModal(); toast("Projet créé"); renderProjects();
        } catch (e) { toast(e.message, false); }
      };
    },
  });
}

async function renderProjectDetail(id) {
  setActions(`<button class="btn" id="back">← Projets</button>`);
  $("#back").onclick = () => go("projects");
  loading();
  let p;
  try { p = await api("GET", "/projects/" + id); } catch (e) { return toast(e.message, false); }
  const services = p.services || [];
  const volumes = p.volumes || [];
  $("#content").innerHTML = `
    <div class="crumbs" style="margin-bottom:16px"><button id="cb">Projets</button> / <span>${esc(p.name)}</span></div>
    <div class="section-title"><h2>${esc(p.name)}</h2><div class="spacer"></div>
      <button class="btn btn-primary btn-sm" id="deploy-btn">${ICON.up} Déployer</button>
      <button class="btn btn-danger btn-sm" id="del-proj">${ICON.trash} Supprimer</button>
    </div>
    <p class="muted" style="margin-top:-6px">${esc(p.description || "Sans description")}</p>

    <div class="panel">
      <div class="panel-head"><h2>Services</h2><span class="badge neutral">${services.length}</span><div class="spacer"></div>
        <button class="btn btn-sm" id="add-svc">${ICON.plus} Ajouter un service</button></div>
      <div class="list" id="svc-list">${services.length ? services.map(svcRow).join("") :
        `<div class="empty" style="padding:30px"><p>Aucun service. Ajoutez-en un (image ou build).</p></div>`}</div>
    </div>

    <div class="panel">
      <div class="panel-head"><h2>Volumes</h2><span class="badge neutral">${volumes.length}</span><div class="spacer"></div></div>
      <div class="panel-body">
        <div class="inline-form" style="margin-bottom:14px">
          <div class="field" style="flex:1"><label>Nom du volume</label><input class="input" id="vol-name" placeholder="ex. db-data" /></div>
          <button class="btn" id="add-vol">${ICON.plus} Ajouter</button>
        </div>
        <div class="list" id="vol-list">${volumes.length ? volumes.map((v) => `
          <div class="row"><div class="grow"><div class="name">${esc(v.name)}</div>
            <div class="meta mono">${esc(v.id)} · driver ${esc(v.driver)}</div></div>
            <button class="icon-btn" data-delvol="${esc(v.id)}">${ICON.trash}</button></div>`).join("") :
          `<div class="muted" style="padding:6px 2px">Aucun volume.</div>`}</div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head"><h2>Historique des déploiements</h2><div class="spacer"></div></div>
      <div class="list" id="dep-history"><div class="skel-row"><div class="skel" style="width:40%"></div></div></div>
    </div>

    <div class="panel">
      <div class="panel-head"><h2>Activité &amp; erreurs</h2><div class="spacer"></div>
        <button class="btn btn-sm btn-ghost" id="ev-refresh">${ICON.refresh}</button></div>
      <div class="list" id="proj-events"><div class="skel-row"><div class="skel" style="width:50%"></div></div></div>
    </div>`;

  $("#cb").onclick = () => go("projects");
  $("#del-proj").onclick = async () => {
    if (!confirm("Supprimer ce projet ?")) return;
    try { await api("DELETE", "/projects/" + id); toast("Projet supprimé"); go("projects"); }
    catch (e) { toast(e.message, false); }
  };
  $("#add-svc").onclick = () => openServiceModal(p);
  $("#deploy-btn").onclick = () => openDeploymentModal(p.id);
  $("#add-vol").onclick = async () => {
    const name = $("#vol-name").value.trim();
    if (!name) return toast("Nom du volume requis", false);
    try { await api("POST", `/projects/${id}/volumes`, { name }); toast("Volume ajouté"); renderProjectDetail(id); }
    catch (e) { toast(e.message, false); }
  };
  $$("[data-delvol]").forEach((b) => (b.onclick = async () => {
    try { await api("DELETE", `/projects/${id}/volumes/${b.dataset.delvol}`); toast("Volume supprimé"); renderProjectDetail(id); }
    catch (e) { toast(e.message, false); }
  }));
  $$("[data-delsvc]").forEach((b) => (b.onclick = async () => {
    if (!confirm("Supprimer ce service ?")) return;
    try { await api("DELETE", `/projects/${id}/services/${b.dataset.delsvc}`); toast("Service supprimé"); renderProjectDetail(id); }
    catch (e) { toast(e.message, false); }
  }));

  // Deployment history + activity/error feed for this project.
  $("#ev-refresh").onclick = () => loadEventsInto("#proj-events", `/projects/${id}/events`);
  loadEventsInto("#proj-events", `/projects/${id}/events`);
  loadProjectDeployments(id);
}

async function loadProjectDeployments(id) {
  const box = $("#dep-history");
  if (!box) return;
  let deps;
  try { deps = await api("GET", `/projects/${id}/deployments`); }
  catch (e) { box.innerHTML = `<div class="muted" style="padding:14px">${esc(e.message)}</div>`; return; }
  if (!deps || !deps.length) {
    box.innerHTML = `<div class="empty" style="padding:26px"><p>Aucun déploiement pour ce projet. Cliquez sur « Déployer ».</p></div>`;
    return;
  }
  box.innerHTML = deps.map((d) => `
    <div class="row clickable" data-godep="${esc(d.id)}">
      <div class="grow"><div class="name">${esc(d.name)} ${statusBadge(d.status)}</div>
        <div class="meta">créé ${esc(fmtDate(d.created_at))} · maj ${esc(fmtDate(d.updated_at))}</div></div>
      <button class="btn btn-sm" data-godep="${esc(d.id)}">Voir</button></div>`).join("");
  $$("[data-godep]", box).forEach((b) => (b.onclick = (e) => { e.stopPropagation(); go("deployments", { id: b.dataset.godep }); }));
}

function svcRow(s) {
  const src = s.image ? `image ${esc(s.image)}` : `build ${esc(s.build_context || "·")}`;
  const tags = [];
  if (s.replicas > 1) tags.push(`<span class="chip">×${s.replicas}</span>`);
  (s.ports || []).forEach((p) => tags.push(`<span class="chip">${p.is_variable ? "▢" : ""}${p.container_port}/${esc(p.protocol)}${p.host_port ? "→" + p.host_port : ""}</span>`));
  (s.env || []).forEach((e) => tags.push(`<span class="chip ${e.is_variable ? "var" : ""}">${esc(e.key)}${e.is_variable ? " (var)" : ""}</span>`));
  (s.depends_on || []).forEach(() => {});
  const deps = (s.depends_on || []).length ? ` · dépend de ${s.depends_on.length}` : "";
  return `<div class="row"><div class="grow"><div class="name">${esc(s.name)}</div>
    <div class="meta">${src}${deps}</div>
    ${tags.length ? `<div class="chips" style="margin-top:8px">${tags.join("")}</div>` : ""}</div>
    <button class="icon-btn" data-delsvc="${esc(s.id)}" title="Supprimer">${ICON.trash}</button></div>`;
}

/* -------- Service modal (rich form) -------- */
function openServiceModal(project) {
  let env = [], ports = [], mounts = [];
  const volumes = project.volumes || [];
  const others = (project.services || []);

  const body = `
    <div class="form-grid">
      <div class="field"><label>Nom</label><input class="input" id="s-name" placeholder="ex. web" /></div>
      <div class="field"><label>Replicas</label><input class="input" id="s-rep" type="number" min="1" value="1" /></div>
      <div class="field"><label>Image</label><input class="input" id="s-image" placeholder="ex. nginx:alpine" /></div>
      <div class="field"><label>Restart policy</label>
        <select class="select" id="s-restart">
          <option value="">(défaut)</option><option value="no">no</option>
          <option value="always">always</option><option value="on-failure">on-failure</option>
          <option value="unless-stopped">unless-stopped</option></select></div>
      <div class="field"><label>Build context</label><input class="input" id="s-bctx" placeholder="laisser vide si image" /></div>
      <div class="field"><label>Dockerfile</label><input class="input" id="s-bfile" placeholder="Dockerfile" /></div>
      <div class="field full"><label>Commande <span class="hint">(séparée par des espaces, optionnel)</span></label><input class="input" id="s-cmd" placeholder="ex. nginx -g daemon off;" /></div>
    </div>
    <div class="subhead">Variables d'environnement</div>
    <div id="env-rows"></div><button class="mini-add" id="add-env">+ ajouter une variable</button>
    <div class="subhead">Ports</div>
    <div id="port-rows"></div><button class="mini-add" id="add-port">+ ajouter un port</button>
    <div class="subhead">Montages de volumes</div>
    <div id="mount-rows"></div>
    ${volumes.length ? `<button class="mini-add" id="add-mount">+ ajouter un montage</button>` : `<div class="muted" style="font-size:12.5px">Aucun volume dans ce projet — créez un volume d'abord.</div>`}
    ${others.length ? `<div class="subhead">Dépend de</div><div class="chips" id="deps">${others.map((o) => `<label class="check chip"><input type="checkbox" value="${esc(o.id)}"> ${esc(o.name)}</label>`).join("")}</div>` : ""}`;

  openModal({
    title: `Nouveau service · ${project.name}`, wide: true, bodyHTML: body,
    footHTML: `<button class="btn" data-close>Annuler</button><button class="btn btn-primary" id="s-save">Créer le service</button>`,
    onMount: (root) => {
      const syncEnv = () => { env = $$("#env-rows .subrow", root).map((r) => ({ key: $(".k", r).value, value: $(".v", r).value, is_variable: $(".var", r).checked })); };
      const syncPort = () => { ports = $$("#port-rows .subrow", root).map((r) => ({ container_port: +$(".cp", r).value || 0, host_port: +$(".hp", r).value || 0, protocol: $(".pr", r).value, is_variable: $(".var", r).checked })); };
      const syncMount = () => { mounts = $$("#mount-rows .subrow", root).map((r) => ({ volume_id: $(".vid", r).value, target: $(".tg", r).value, read_only: $(".ro", r).checked })); };
      const drawEnv = () => { $("#env-rows", root).innerHTML = env.map((e, i) => `
        <div class="subrow env"><input class="input k" placeholder="CLÉ" value="${esc(e.key)}"><input class="input v" placeholder="valeur" value="${esc(e.value)}">
          <label class="check"><input type="checkbox" class="var" ${e.is_variable ? "checked" : ""}> variable</label>
          <button class="icon-btn" data-rm="${i}">${ICON.trash}</button></div>`).join("");
        $$("#env-rows [data-rm]", root).forEach((b) => (b.onclick = () => { syncEnv(); env.splice(+b.dataset.rm, 1); drawEnv(); })); };
      const drawPort = () => { $("#port-rows", root).innerHTML = ports.map((p, i) => `
        <div class="subrow port"><input class="input cp" type="number" placeholder="port conteneur" value="${p.container_port || ""}"><input class="input hp" type="number" placeholder="port hôte" value="${p.host_port || ""}">
          <select class="select pr"><option ${p.protocol !== "udp" ? "selected" : ""}>tcp</option><option ${p.protocol === "udp" ? "selected" : ""}>udp</option></select>
          <label class="check"><input type="checkbox" class="var" ${p.is_variable ? "checked" : ""}> variable</label>
          <button class="icon-btn" data-rm="${i}">${ICON.trash}</button></div>`).join("");
        $$("#port-rows [data-rm]", root).forEach((b) => (b.onclick = () => { syncPort(); ports.splice(+b.dataset.rm, 1); drawPort(); })); };
      const drawMount = () => { $("#mount-rows", root).innerHTML = mounts.map((m, i) => `
        <div class="subrow mount"><select class="select vid">${volumes.map((v) => `<option value="${esc(v.id)}" ${m.volume_id === v.id ? "selected" : ""}>${esc(v.name)}</option>`).join("")}</select>
          <input class="input tg" placeholder="cible ex. /var/www" value="${esc(m.target)}">
          <label class="check"><input type="checkbox" class="ro" ${m.read_only ? "checked" : ""}> RO</label>
          <button class="icon-btn" data-rm="${i}">${ICON.trash}</button></div>`).join("");
        $$("#mount-rows [data-rm]", root).forEach((b) => (b.onclick = () => { syncMount(); mounts.splice(+b.dataset.rm, 1); drawMount(); })); };

      $("#s-name", root).focus();
      $("#add-env", root).onclick = () => { syncEnv(); env.push({ key: "", value: "", is_variable: false }); drawEnv(); };
      $("#add-port", root).onclick = () => { syncPort(); ports.push({ container_port: 0, host_port: 0, protocol: "tcp", is_variable: false }); drawPort(); };
      if ($("#add-mount", root)) $("#add-mount", root).onclick = () => { syncMount(); mounts.push({ volume_id: volumes[0].id, target: "", read_only: false }); drawMount(); };

      $("#s-save", root).onclick = async () => {
        syncEnv(); syncPort(); syncMount();
        const name = $("#s-name", root).value.trim();
        const image = $("#s-image", root).value.trim();
        const bctx = $("#s-bctx", root).value.trim();
        if (!name) return toast("Nom du service requis", false);
        if (!image && !bctx) return toast("Renseignez une image ou un build context", false);
        const cmd = $("#s-cmd", root).value.trim();
        const deps = $$("#deps input:checked", root).map((i) => i.value);
        const payload = {
          name, image, build_context: bctx,
          build_dockerfile: $("#s-bfile", root).value.trim(),
          command: cmd ? cmd.split(/\s+/) : [],
          restart_policy: $("#s-restart", root).value,
          replicas: +$("#s-rep", root).value || 1,
          env: env.filter((e) => e.key),
          ports: ports.filter((p) => p.container_port),
          mounts: mounts.filter((m) => m.target),
          depends_on: deps,
        };
        try { await api("POST", `/projects/${project.id}/services`, payload); closeModal(); toast("Service créé"); renderProjectDetail(project.id); }
        catch (e) { toast(e.message, false); }
      };
    },
  });
}

/* ============================ DEPLOYMENTS ============================ */
async function renderDeployments() {
  setActions(`<button class="btn btn-primary" id="new-dep">${ICON.plus} Nouveau déploiement</button>`);
  $("#new-dep").onclick = () => openDeploymentModal();
  loading();
  let deps, projects;
  try { [deps, projects] = await Promise.all([api("GET", "/deployments"), api("GET", "/projects")]); }
  catch (e) { return toast(e.message, false); }
  const pmap = Object.fromEntries((projects || []).map((p) => [p.id, p.name]));
  const c = $("#content");
  if (!deps || !deps.length) {
    c.innerHTML = emptyState(
      `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7"><rect x="4" y="10" width="16" height="11" rx="2"/><path d="M12 2v6m0 0l3-3m-3 3L9 5"/></svg>`,
      "Aucun déploiement", "Un déploiement matérialise un projet sur le Docker Engine, avec ses valeurs spécifiques (ports, variables).",
      `<button class="btn btn-primary" id="empty-dep">${ICON.plus} Créer un déploiement</button>`);
    $("#empty-dep").onclick = () => openDeploymentModal();
    return;
  }
  c.innerHTML = `<div class="panel"><div class="list">${deps.map((d) => `
    <div class="row clickable" data-open="${esc(d.id)}">
      <div class="grow"><div class="name">${esc(d.name)} ${statusBadge(d.status)}</div>
        <div class="meta">projet ${esc(pmap[d.project_id] || shortId(d.project_id))} · serveur ${esc(d.server_id)} · maj ${esc(fmtDate(d.updated_at))}</div></div>
      <div class="actions">
        <button class="btn btn-sm" data-up="${esc(d.id)}">${ICON.up} Up</button>
        <button class="btn btn-sm" data-down="${esc(d.id)}">${ICON.down} Down</button>
        <button class="btn btn-sm" data-open="${esc(d.id)}">Voir</button>
        <button class="icon-btn" data-del="${esc(d.id)}">${ICON.trash}</button>
      </div></div>`).join("")}</div></div>`;
  bindDeployActions(c, () => renderDeployments());
  $$("[data-open]", c).forEach((b) => (b.onclick = (e) => { e.stopPropagation(); go("deployments", { id: b.dataset.open }); }));
}

function bindDeployActions(scope, after) {
  $$("[data-up]", scope).forEach((b) => (b.onclick = async (e) => {
    e.stopPropagation(); b.disabled = true; b.innerHTML = `<span class="spinner"></span> Up`;
    try { await api("POST", `/deployments/${b.dataset.up}/up`); toast("Déploiement démarré"); after(); }
    catch (err) { toast(err.message, false); b.disabled = false; b.innerHTML = `${ICON.up} Up`; }
  }));
  $$("[data-down]", scope).forEach((b) => (b.onclick = async (e) => {
    e.stopPropagation(); b.disabled = true; b.innerHTML = `<span class="spinner"></span> Down`;
    try { await api("POST", `/deployments/${b.dataset.down}/down`); toast("Déploiement arrêté"); after(); }
    catch (err) { toast(err.message, false); b.disabled = false; b.innerHTML = `${ICON.down} Down`; }
  }));
  $$("[data-del]", scope).forEach((b) => (b.onclick = async (e) => {
    e.stopPropagation();
    if (!confirm("Supprimer ce déploiement ?")) return;
    try { await api("DELETE", "/deployments/" + b.dataset.del); toast("Déploiement supprimé"); after(); }
    catch (err) { toast(err.message, false); }
  }));
}

async function openDeploymentModal(presetProjectId) {
  let projects;
  try { projects = await api("GET", "/projects"); } catch (e) { return toast(e.message, false); }
  if (!projects || !projects.length) return toast("Créez d'abord un projet", false);

  openModal({
    title: "Nouveau déploiement", wide: true,
    bodyHTML: `
      <div class="form-grid">
        <div class="field"><label>Projet</label><select class="select" id="d-project">${projects.map((p) => `<option value="${esc(p.id)}" ${p.id === presetProjectId ? "selected" : ""}>${esc(p.name)}</option>`).join("")}</select></div>
        <div class="field"><label>Nom du déploiement</label><input class="input" id="d-name" placeholder="ex. prod" /></div>
      </div>
      <div class="field"><label>Serveur</label><input class="input" id="d-server" value="local" /></div>
      <div class="subhead">Valeurs spécifiques (overrides)</div>
      <p class="muted" style="font-size:12.5px;margin-top:-4px">Renseignez les variables marquées <span class="badge var" style="padding:1px 7px"><span class="dot"></span>var</span> dans le projet. Laissez vide pour garder la valeur par défaut.</p>
      <div id="ov-area" class="muted">Chargement…</div>`,
    footHTML: `<button class="btn" data-close>Annuler</button><button class="btn btn-primary" id="d-save">Créer le déploiement</button>`,
    onMount: (root) => {
      const sel = $("#d-project", root);
      const loadOverrides = async () => {
        const area = $("#ov-area", root);
        area.innerHTML = "Chargement…";
        let p;
        try { p = await api("GET", "/projects/" + sel.value); } catch (e) { area.textContent = e.message; return; }
        const blocks = [];
        (p.services || []).forEach((s) => {
          const varEnv = (s.env || []).filter((e) => e.is_variable);
          const varPort = (s.ports || []).filter((pt) => pt.is_variable);
          if (!varEnv.length && !varPort.length) return;
          blocks.push(`<div class="panel" style="margin:10px 0"><div class="panel-head" style="padding:10px 14px"><h2>${esc(s.name)}</h2></div><div class="panel-body">
            ${varEnv.map((e) => `<div class="field"><label>env · ${esc(e.key)}</label><input class="input ov" data-kind="env" data-svc="${esc(s.id)}" data-key="${esc(e.key)}" placeholder="${esc(e.value || "valeur")}"></div>`).join("")}
            ${varPort.map((pt) => `<div class="field"><label>port hôte · ${esc(pt.container_port)}/${esc(pt.protocol)}</label><input class="input ov" type="number" data-kind="port" data-svc="${esc(s.id)}" data-key="${esc(pt.container_port + "/" + pt.protocol)}" placeholder="port hôte"></div>`).join("")}
          </div></div>`);
        });
        area.innerHTML = blocks.length ? blocks.join("") : `<div class="muted" style="font-size:13px">Aucune variable à fournir pour ce projet.</div>`;
      };
      sel.onchange = loadOverrides;
      loadOverrides();
      $("#d-name", root).focus();
      $("#d-save", root).onclick = async () => {
        const name = $("#d-name", root).value.trim();
        if (!name) return toast("Nom du déploiement requis", false);
        const overrides = $$(".ov", root).filter((i) => i.value.trim() !== "").map((i) => ({
          kind: i.dataset.kind, target_ref: i.dataset.svc, key: i.dataset.key, value: i.value.trim(),
        }));
        try {
          await api("POST", `/projects/${sel.value}/deployments`, { name, server_id: $("#d-server", root).value.trim(), overrides });
          closeModal(); toast("Déploiement créé"); go("deployments");
        } catch (e) { toast(e.message, false); }
      };
    },
  });
}

async function renderDeploymentDetail(id) {
  setActions(`<button class="btn" id="back">← Déploiements</button>`);
  $("#back").onclick = () => go("deployments");
  loading();
  const paint = async (silent) => {
    let d, st;
    try { d = await api("GET", "/deployments/" + id); st = await api("GET", `/deployments/${id}/status`); }
    catch (e) { if (!silent) toast(e.message, false); return; }
    const containers = st.containers || [];
    $("#content").innerHTML = `
      <div class="crumbs" style="margin-bottom:16px"><button id="cb">Déploiements</button> / <span>${esc(d.name)}</span></div>
      <div class="section-title"><h2>${esc(d.name)}</h2> ${statusBadge(st.status)}<div class="spacer"></div>
        <button class="btn btn-primary btn-sm" data-up="${esc(id)}">${ICON.up} Up</button>
        <button class="btn btn-sm" data-down="${esc(id)}">${ICON.down} Down</button>
        <button class="btn btn-sm" id="refresh">${ICON.refresh} Rafraîchir</button>
        <button class="btn btn-danger btn-sm" data-del="${esc(id)}">${ICON.trash}</button></div>

      <div class="panel"><div class="panel-head"><h2>Informations</h2></div><div class="panel-body">
        <dl class="kv"><dt>Projet</dt><dd class="mono">${esc(shortId(d.project_id))}</dd>
          <dt>Serveur</dt><dd>${esc(d.server_id)}</dd>
          <dt>Statut</dt><dd>${statusBadge(st.status)}</dd>
          <dt>Overrides</dt><dd>${(d.overrides || []).length ? d.overrides.map((o) => `<span class="chip">${esc(o.kind)}:${esc(o.key)}=${esc(o.value)}</span>`).join(" ") : "<span class='muted'>aucun</span>"}</dd></dl>
      </div></div>

      <div class="panel"><div class="panel-head"><h2>Conteneurs</h2><span class="badge neutral">${containers.length}</span></div>
        <div class="list">${containers.length ? containers.map((c) => `
          <div class="row"><div class="grow"><div class="name">${esc(c.name)} <span class="badge ${c.state === "running" ? "running" : "not-running"}" style="margin-left:6px"><span class="dot"></span>${esc(c.state)}</span></div>
            <div class="meta mono">${esc(shortId(c.docker_container_id))} · santé ${esc(c.health || "none")} · service ${esc(shortId(c.service_id))}</div></div>
            ${c.docker_container_id ? `<button class="btn btn-sm" data-logs="${esc(c.docker_container_id)}" data-cname="${esc(c.name)}">Logs</button>` : ""}</div>`).join("") :
          `<div class="empty" style="padding:30px"><p>Aucun conteneur actif. Lancez le déploiement avec « Up ».</p></div>`}</div></div>

      <div class="panel"><div class="panel-head"><h2>Activité &amp; erreurs</h2><div class="spacer"></div></div>
        <div class="list" id="dep-events"><div class="skel-row"><div class="skel" style="width:50%"></div></div></div></div>`;
    $("#cb").onclick = () => go("deployments");
    $("#refresh").onclick = () => paint(false);
    bindDeployActions($("#content"), () => paint(false));
    $$("[data-logs]", $("#content")).forEach((b) => (b.onclick = () => openLogs(id, b.dataset.logs, b.dataset.cname)));
    loadEventsInto("#dep-events", `/deployments/${id}/events`);
  };
  await paint(false);
  clearPoll();
  pollTimer = setInterval(() => { if (route.name === "deployments" && route.id === id) paint(true); else clearPoll(); }, 4000);
}

/* ============================ IMAGES ============================ */
function fmtBytes(n) {
  if (!n) return "—";
  const u = ["o", "Ko", "Mo", "Go"]; let i = 0; let v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return v.toFixed(v >= 10 || i === 0 ? 0 : 1) + " " + u[i];
}

async function loadImages() {
  const box = $("#img-list");
  if (!box) return;
  let imgs;
  try { imgs = await api("GET", "/images"); } catch (e) { box.innerHTML = `<div class="muted" style="padding:14px">${esc(e.message)}</div>`; return; }
  const tagged = (imgs || []).filter((i) => (i.tags || []).length && !i.tags.includes("<none>:<none>"));
  if (!tagged.length) {
    box.innerHTML = `<div class="empty" style="padding:30px"><p>Aucune image. Récupérez-en une avec « Pull » ci-dessus.</p></div>`;
    return;
  }
  box.innerHTML = tagged.map((i) => `
    <div class="row"><div class="grow"><div class="name">${esc((i.tags || []).join(", "))}</div>
      <div class="meta mono">${esc(shortId((i.id || "").replace("sha256:", "")))} · ${fmtBytes(i.size)}</div></div></div>`).join("");
}

function renderImages() {
  setActions(`<button class="btn" id="img-refresh">${ICON.refresh} Rafraîchir</button>`);
  $("#content").innerHTML = `
    <div class="grid-cards">
      <div class="panel"><div class="panel-head"><h2>Récupérer une image (pull)</h2></div><div class="panel-body">
        <div class="field"><label>Référence</label><input class="input" id="pull-ref" placeholder="ex. nginx:alpine" /></div>
        <div class="field"><label>Auth registry <span class="hint">(X-Registry-Auth base64, optionnel)</span></label><input class="input" id="pull-auth" placeholder="optionnel" /></div>
        <button class="btn btn-primary" id="pull-btn">${ICON.down} Pull</button>
      </div></div>
      <div class="panel"><div class="panel-head"><h2>Construire une image (build)</h2></div><div class="panel-body">
        <div class="field"><label>Context dir <span class="hint">(chemin côté Engine/API)</span></label><input class="input" id="b-ctx" placeholder="ex. /tmp/app" /></div>
        <div class="field"><label>Dockerfile</label><input class="input" id="b-file" placeholder="Dockerfile" /></div>
        <div class="field"><label>Tag</label><input class="input" id="b-tag" placeholder="ex. monapp:latest" /></div>
        <button class="btn btn-primary" id="build-btn">${ICON.plus} Build</button>
      </div></div>
    </div>
    <div class="panel" style="margin-top:18px"><div class="panel-head"><h2>Images présentes</h2><div class="spacer"></div></div>
      <div class="list" id="img-list"><div class="skel-row"><div class="skel" style="width:40%"></div></div></div></div>`;
  $("#img-refresh").onclick = loadImages;
  $("#pull-btn").onclick = async () => {
    const ref = $("#pull-ref").value.trim();
    if (!ref) return toast("Référence requise", false);
    const btn = $("#pull-btn"); btn.disabled = true; btn.innerHTML = `<span class="spinner"></span> Pull…`;
    try { await api("POST", "/images/pull", { ref, auth: $("#pull-auth").value.trim() }); toast("Image récupérée : " + ref); loadImages(); }
    catch (e) { toast(e.message, false); }
    finally { btn.disabled = false; btn.innerHTML = `${ICON.down} Pull`; }
  };
  $("#build-btn").onclick = async () => {
    const context_dir = $("#b-ctx").value.trim(), tag = $("#b-tag").value.trim();
    if (!context_dir || !tag) return toast("Context dir et tag requis", false);
    const btn = $("#build-btn"); btn.disabled = true; btn.innerHTML = `<span class="spinner"></span> Build…`;
    try { await api("POST", "/images/build", { context_dir, dockerfile: $("#b-file").value.trim(), tag }); toast("Image construite : " + tag); loadImages(); }
    catch (e) { toast(e.message, false); }
    finally { btn.disabled = false; btn.innerHTML = `${ICON.plus} Build`; }
  };
  loadImages();
}

/* ============================ SERVERS ============================ */
async function renderServers() {
  setActions("");
  loading();
  let servers;
  try { servers = await api("GET", "/servers"); } catch (e) { return toast(e.message, false); }
  $("#content").innerHTML = `<div class="panel"><div class="list">${(servers || []).map((s) => `
    <div class="row"><div class="grow"><div class="name">${esc(s.name)} <span class="badge ${s.status === "reachable" ? "running" : "neutral"}" style="margin-left:6px"><span class="dot"></span>${esc(s.status)}</span></div>
      <div class="meta mono">${esc(s.host)}</div></div>
      <span class="chip">${esc(s.id)}</span></div>`).join("")}</div></div>
    <p class="muted" style="margin-top:14px;font-size:12.5px">Le MVP gère un seul serveur local. Le multi-engine est prévu en Phase 2.</p>`;
}

/* ============================ AUTH (login / session) ============================ */
function expLabel() {
  if (!tokenExp) return "—";
  const d = new Date(tokenExp);
  if (isNaN(d)) return "—";
  const mins = Math.round((d - new Date()) / 60000);
  return `${d.toLocaleString()} (${mins > 0 ? "dans " + mins + " min" : "expiré"})`;
}

// showLogin renders a full-screen login gate. Used on startup 401 and on logout.
function showLogin() {
  clearPoll();
  const root = $("#modal-root");
  root.innerHTML = `
    <div class="backdrop login-backdrop">
      <div class="modal login-modal" role="dialog" aria-modal="true">
        <div class="login-brand"><div class="logo">n</div><div><b>dev-natif</b><small>Console Docker</small></div></div>
        <h2>Connexion</h2>
        <p class="muted" style="margin:0 0 18px;font-size:13px">Authentifiez-vous pour accéder à l'API.</p>
        <form id="login-form">
          <div class="field"><label>Utilisateur</label><input class="input" id="lg-user" autocomplete="username" value="admin" /></div>
          <div class="field"><label>Mot de passe</label><input class="input" id="lg-pass" type="password" autocomplete="current-password" placeholder="••••••" /></div>
          <button class="btn btn-primary" id="lg-btn" type="submit" style="width:100%;margin-top:6px">Se connecter</button>
        </form>
        <p class="faint" style="font-size:12px;margin:16px 0 0;text-align:center">Identifiants par défaut : admin / admin</p>
      </div>
    </div>`;
  const form = $("#login-form", root);
  $("#lg-user", root).focus();
  form.onsubmit = async (e) => {
    e.preventDefault();
    const btn = $("#lg-btn", root); btn.disabled = true; btn.innerHTML = `<span class="spinner"></span> Connexion…`;
    try {
      const data = await api("POST", "/auth/login", {
        username: $("#lg-user", root).value.trim(),
        password: $("#lg-pass", root).value,
      });
      setToken(data.token, data.expires_at, data.username);
      closeModal();
      toast("Connecté");
      pollEngine();
      applyRoute();
    } catch (err) {
      btn.disabled = false; btn.innerHTML = "Se connecter";
      toast(err.message || "Échec de connexion", false);
    }
  };
}

function logout() {
  clearToken();
  closeModal();
  toast("Déconnecté");
  showLogin();
}

async function renewToken(silent) {
  if (!token) return;
  try {
    const data = await api("POST", "/auth/refresh");
    setToken(data.token, data.expires_at, data.username);
    if (!silent) toast("Token renouvelé");
  } catch {
    if (!silent) toast("Renouvellement impossible", false);
  }
}

// autoRenew refreshes the token when it is close to expiry.
async function autoRenew() {
  if (!token || !tokenExp) return;
  const d = new Date(tokenExp);
  if (isNaN(d)) return;
  if (d - new Date() < 5 * 60 * 1000) await renewToken(true);
}

/* ============================ SETTINGS ============================ */
function openSettings() {
  openModal({
    title: "Session & réglages",
    bodyHTML: `
      <dl class="kv">
        <dt>Utilisateur</dt><dd>${esc(currentUser || "—")}</dd>
        <dt>Token expire</dt><dd id="set-exp">${esc(expLabel())}</dd>
      </dl>
      <p class="muted" style="font-size:12.5px;margin-top:14px">Le token expire automatiquement et se renouvelle peu avant l'échéance. Vous pouvez le renouveler manuellement.</p>`,
    footHTML: `<button class="btn btn-danger" id="set-logout">Se déconnecter</button><div style="flex:1"></div><button class="btn" data-close>Fermer</button><button class="btn btn-primary" id="set-renew">Renouveler le token</button>`,
    onMount: (root) => {
      $("#set-renew", root).onclick = async () => { await renewToken(false); const e = $("#set-exp", root); if (e) e.textContent = expLabel(); };
      $("#set-logout", root).onclick = logout;
    },
  });
}

/* ============================ ENGINE STATUS ============================ */
async function pollEngine() {
  const pill = $("#engine-pill");
  try {
    const res = await fetch("/readyz");
    const data = await res.json();
    const ok = data.docker_engine === "ok";
    pill.innerHTML = `<span class="badge ${ok ? "running" : "failed"}"><span class="dot"></span>Engine</span><span id="engine-text">${ok ? "connecté" : "injoignable"}</span>`;
  } catch {
    pill.innerHTML = `<span class="badge failed"><span class="dot"></span>Engine</span><span>injoignable</span>`;
  }
}

/* ----------------------------- init ----------------------------- */
$$("#nav button").forEach((b) => (b.onclick = () => go(b.dataset.view)));
$("#settings-btn").onclick = openSettings;
$("#menu-toggle").onclick = () => $("#sidebar").classList.toggle("open");
window.addEventListener("hashchange", applyRoute);
pollEngine();
setInterval(pollEngine, 8000);
setInterval(autoRenew, 60000);
if (!location.hash) location.hash = "#/projects";
else applyRoute();

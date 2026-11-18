// Build editor: holds the build state, sends it to the server on every change
// (POST /build/calc) and swaps the re-rendered windows. The server normalizes
// the build, so this file never duplicates game rules beyond slot occupancy.
(() => {
  const boot = window.BUILD;
  if (!boot) return;

  // Opening /build without a code continues the last build (unless ?new=1).
  {
    const params = new URLSearchParams(location.search);
    let last = null;
    try { last = localStorage.getItem("ro.build.last"); } catch {}
    if (!params.has("b") && !params.has("new") && !params.has("s") && last) {
      params.set("b", last);
      location.replace("/build?" + params);
      return;
    }
  }
  let build = boot.build;
  let code = boot.code;
  const lang = boot.lang || "pt";
  const SAVE_KEY = "ro.build.last";
  const HEAD = ["Head_Top", "Head_Mid", "Head_Low"];

  build.e = build.e || {};
  build.k = build.k || {};

  // ---- server round-trip ----
  // Every local change bumps rev. A response is only adopted if nothing
  // changed since its request was sent; otherwise a newer request follows
  // and the stale response is dropped (it would erase the newer change).
  let inflight = null;
  let pending = false;
  let rev = 0;
  const touch = () => { rev++; };

  async function recalc() {
    touch();
    if (inflight) { pending = true; return; }
    const sentRev = rev;
    inflight = fetch("/build/calc?lang=" + lang, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(build),
    });
    try {
      const res = await inflight;
      if (!res.ok) throw new Error("HTTP " + res.status);
      const data = await res.json();
      if (sentRev !== rev) return; // stale: a newer change is queued
      build = data.build;
      build.e = build.e || {};
      build.k = build.k || {};
      code = data.code;
      for (const n of data.notes || []) say(n, true);
      if (skillTree && data.skillGroups) skillTree.setGroups(data.skillGroups);
      for (const [id, html] of Object.entries(data.html)) {
        const el = document.getElementById(id);
        if (el) el.innerHTML = html;
      }
      restoreEquipTab();
      if (slotOpen) renderSlotEditor();
      persist();
    } catch (err) {
      say(T("Erro ao calcular: ") + err.message, true);
    } finally {
      inflight = null;
      if (pending) { pending = false; recalc(); }
    }
  }

  function persist() {
    const url = new URL(location.href);
    url.searchParams.set("b", code);
    url.searchParams.delete("new");
    if (saved) url.searchParams.set("s", saved.id);
    history.replaceState(null, "", url);
    try { localStorage.setItem(SAVE_KEY, code); } catch {}
  }

  function say(msg, bad) {
    const chat = document.getElementById("ro-chat");
    if (!chat) return;
    const line = document.createElement("div");
    line.className = "ro-chat__line " + (bad ? "ro-chat__line--error" : "ro-chat__line--system");
    line.textContent = msg;
    chat.appendChild(line);
    chat.scrollTop = chat.scrollHeight;
  }

  // ---- status window ----
  document.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-stat],[data-trait]");
    if (!btn || btn.disabled) return;
    const step = Number(btn.dataset.delta) * (e.shiftKey ? 10 : 1);
    if (btn.dataset.stat !== undefined) {
      const i = Number(btn.dataset.stat);
      build.s[i] = Math.max(1, (build.s[i] || 1) + step);
    } else {
      build.t = build.t || [0, 0, 0, 0, 0, 0];
      const i = Number(btn.dataset.trait);
      build.t[i] = Math.max(0, (build.t[i] || 0) + step);
    }
    recalc();
  });

  document.addEventListener("change", (e) => {
    const f = e.target.closest("[data-field]");
    if (!f) return;
    const v = Number(f.value);
    if (f.dataset.field === "job") {
      build.j = v;
      build.k = {};
      build.t = [0, 0, 0, 0, 0, 0];
      loadSkills(v);
      say(T("Classe alterada. Habilidades resetadas."));
    } else if (f.dataset.field === "base") {
      build.b = v;
    } else if (f.dataset.field === "joblv") {
      build.jl = v;
    }
    recalc();
  });

  // ---- basic info buttons ----
  // Saved builds live in this browser (saved.js). ?s=ID ties the page to
  // one of them, so saving again updates it.
  let saved = window.ROSaved ? ROSaved.get(new URLSearchParams(location.search).get("s") || "") : null;
  function saveBuild(ask) {
    if (!window.ROSaved || !ROSaved.available()) {
      say(T("Este navegador não permite salvar (modo anônimo?). Use Copiar link para guardar o build."), true);
      return;
    }
    let name = saved ? saved.name : "";
    if (ask || !saved) {
      name = prompt(T("Nome do build:"), name);
      if (name === null) return;
    }
    const jobName = (document.querySelector("#build_basic .ro-basic__name") || {}).textContent || "";
    const e = ROSaved.put({ id: saved ? saved.id : "", name: (name.trim() || T("Sem nome")).slice(0, 40), code,
      j: build.j, job: jobName.trim(), b: build.b, jl: build.jl });
    if (!e) { say(T("Não foi possível salvar: o armazenamento do navegador está cheio ou bloqueado."), true); return; }
    saved = e;
    const url = new URL(location.href);
    url.searchParams.set("s", e.id);
    history.replaceState(null, "", url);
    say(T('Build "%s" salvo neste navegador.', e.name));
  }
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") { e.preventDefault(); saveBuild(false); }
  });
  document.addEventListener("click", async (e) => {
    if (e.target.closest("[data-save-build]")) saveBuild(true);
    if (e.target.closest("[data-compare-build]")) {
      persist();
      const url = new URL("/compare", location.origin);
      url.searchParams.append("b", code);
      for (const s of (window.ROSaved ? ROSaved.list() : [])) {
        if (s.code !== code) { url.searchParams.append("b", s.code); break; }
      }
      if (build.m) url.searchParams.set("m", build.m);
      location.href = url.href;
    }
    if (e.target.closest("[data-copy-link]")) {
      const link = location.origin + "/build?b=" + code;
      try {
        await navigator.clipboard.writeText(link);
        say(T("Link do build copiado!"));
      } catch {
        prompt(T("Copie o link do build:"), link);
      }
    }
    if (e.target.closest("[data-new-build]")) {
      if (!confirm(T("Começar um build novo? Salve o atual antes (Ctrl+S) se quiser guardá-lo."))) return;
      build = { j: build.j, b: build.b, jl: build.jl, s: [1, 1, 1, 1, 1, 1], t: [0, 0, 0, 0, 0, 0], e: {}, k: {} };
      loadSkills(build.j);
      recalc();
    }
  });

  // ---- equipment window ----
  let equipTab = "main";
  function restoreEquipTab() {
    document.querySelectorAll("[data-equip-tab]").forEach((b) => b.classList.toggle("is-active", b.dataset.equipTab === equipTab));
    document.querySelectorAll("[data-equip-panel]").forEach((p) => { p.hidden = p.dataset.equipPanel !== equipTab; });
  }
  document.addEventListener("click", (e) => {
    const t = e.target.closest("[data-equip-tab]");
    if (t) { equipTab = t.dataset.equipTab; restoreEquipTab(); }
  });

  // ---- slot editor + picker ----
  const slotWin = document.getElementById("win-slot");
  const slotBody = document.getElementById("slot-body");
  const pickQ = document.getElementById("pick-q");
  const pickList = document.getElementById("pick-results");
  const pickMode = document.getElementById("pick-mode");
  let slotOpen = null; // slot key
  let cardIndex = -1;  // -1 = picking the item itself

  function openSlot(slot) {
    slotOpen = slot;
    cardIndex = -1;
    window.ROWin.show(slotWin);
    renderSlotEditor();
    pickQ.value = "";
    search();
    pickQ.focus();
  }

  function renderSlotEditor() {
    const tpl = document.getElementById("slot-editor-" + slotOpen);
    if (!tpl) return;
    slotWin.querySelector("[data-slot-title]").textContent = tpl.dataset.label;
    slotBody.innerHTML = tpl.innerHTML;
    pickMode.textContent = cardIndex >= 0 ? "Escolha uma carta (slot " + (cardIndex + 1) + ")" : "Escolha um item para " + tpl.dataset.label;
  }

  let searchTimer = null;
  function search() {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(async () => {
      const params = new URLSearchParams({ slot: slotOpen, q: pickQ.value, lang, job: build.j });
      if (cardIndex >= 0) params.set("card", "1");
      const res = await fetch("/build/items?" + params);
      pickList.innerHTML = res.ok ? await res.text() : "";
    }, 150);
  }
  pickQ.addEventListener("input", search);

  // Removes an item from every slot it occupies (multi-slot headgear, 2H weapons).
  function unequip(slot) {
    const cur = build.e[slot];
    if (!cur) return;
    for (const [k, v] of Object.entries(build.e)) {
      if (v.i === cur.i && (HEAD.includes(k) === HEAD.includes(slot))) delete build.e[k];
    }
    delete build.e[slot];
  }

  function equip(slot, id, locs) {
    unequip(slot);
    const entry = { i: id };
    if (HEAD.includes(slot)) {
      // An item covering several head slots occupies all of them.
      const covered = HEAD.filter((h) => locs.includes(h));
      for (const h of covered.length ? covered : [slot]) {
        unequip(h);
        build.e[h] = { ...entry };
      }
      return;
    }
    if (slot === "Right_Hand" && locs.includes("Both_Hand")) delete build.e["Left_Hand"];
    if (slot === "Left_Hand") {
      const w = document.querySelector('[data-slot="Right_Hand"]');
      if (w && (w.dataset.loc || "").includes("Both_Hand")) delete build.e["Right_Hand"];
    }
    build.e[slot] = entry;
  }

  // Keeps head slots that share one item in sync (refine/cards).
  function eachLinked(slot, fn) {
    const cur = build.e[slot];
    if (!cur) return;
    const keys = HEAD.includes(slot) ? HEAD.filter((h) => build.e[h] && build.e[h].i === cur.i) : [slot];
    keys.forEach((k) => fn(build.e[k]));
  }

  document.addEventListener("click", (e) => {
    const slotBtn = e.target.closest("[data-slot]");
    if (slotBtn && !slotBtn.closest("#win-slot")) { openSlot(slotBtn.dataset.slot); return; }
    if (!slotOpen) return;

    const pick = e.target.closest("[data-pick]");
    if (pick) {
      const id = Number(pick.dataset.pick);
      if (cardIndex >= 0) {
        eachLinked(slotOpen, (eq) => { eq.c = eq.c || [0, 0, 0, 0]; eq.c[cardIndex] = id; });
        say(T("Carta aplicada."));
        cardIndex = -1;
      } else {
        equip(slotOpen, id, (pick.dataset.loc || "").split(","));
        say(T("Item equipado."));
      }
      recalc();
      search();
      return;
    }
    const pc = e.target.closest("[data-pick-card]");
    if (pc) { cardIndex = Number(pc.dataset.pickCard); renderSlotEditor(); pickQ.value = ""; search(); pickQ.focus(); return; }
    const rc = e.target.closest("[data-remove-card]");
    if (rc) { eachLinked(slotOpen, (eq) => { if (eq.c) eq.c[Number(rc.dataset.removeCard)] = 0; }); recalc(); return; }
    if (e.target.closest("[data-pick-item]")) { cardIndex = -1; renderSlotEditor(); pickQ.value = ""; search(); pickQ.focus(); return; }
    if (e.target.closest("[data-unequip]")) { unequip(slotOpen); say(T("Item removido.")); recalc(); }
  });

  // ---- optimizer ----
  const optOut = document.getElementById("opt-results");
  let optBusy = false;
  async function optimize() {
    if (!optOut || optBusy) return;
    optBusy = true;
    optOut.innerHTML = '<p class="ro-note">' + T("Calculando…") + "</p>";
    try {
      const res = await fetch("/build/optimize?lang=" + lang, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ b: build, enc: document.getElementById("opt-enchants").checked }),
      });
      if (!res.ok) throw new Error((await res.text()).trim() || "HTTP " + res.status);
      optOut.innerHTML = await res.text();
    } catch (err) {
      optOut.innerHTML = "";
      say(T("Erro ao calcular: ") + err.message, true);
    } finally {
      optBusy = false;
    }
  }
  document.addEventListener("click", async (e) => {
    if (e.target.closest("[data-optimize]")) { optimize(); return; }
    const ap = e.target.closest("[data-opt-apply]");
    if (!ap) return;
    const slot = ap.dataset.optApply, i = Number(ap.dataset.optIndex), id = Number(ap.dataset.optCard);
    const cur = build.e[slot];
    if (!cur) return;
    const keys = HEAD.includes(slot) ? HEAD.filter((h) => build.e[h] && build.e[h].i === cur.i) : [slot];
    keys.forEach((k) => { const eq = build.e[k]; eq.c = eq.c || [0, 0, 0, 0]; eq.c[i] = id; });
    say(T("Carta aplicada."));
    await recalc();
    optimize();
  });

  document.addEventListener("change", (e) => {
    const r = e.target.closest("[data-refine]");
    if (!r || !slotOpen) return;
    eachLinked(slotOpen, (eq) => { eq.r = Number(r.value); });
    recalc();
  });

  document.addEventListener("keyup", (e) => {
    if (e.key === "Enter" && e.target.matches("[data-pick]")) e.target.click();
  });

  // ---- target / damage window ----
  const mobQ = document.getElementById("mob-q");
  const mobList = document.getElementById("mob-results");
  let mobTimer = null;
  mobQ.addEventListener("input", () => {
    clearTimeout(mobTimer);
    mobTimer = setTimeout(async () => {
      const q = mobQ.value.trim();
      if (!q) { mobList.hidden = true; return; }
      const res = await fetch("/build/monsters?q=" + encodeURIComponent(q));
      mobList.innerHTML = res.ok ? await res.text() : "";
      mobList.hidden = false;
    }, 150);
  });
  document.addEventListener("click", (e) => {
    const mob = e.target.closest("[data-monster]");
    if (mob) {
      build.m = Number(mob.dataset.monster);
      mobList.hidden = true;
      mobQ.value = "";
      recalc();
      return;
    }
    if (e.target.closest("[data-atk-auto]")) {
      build.a = { ...(build.a || {}), r: 0 };
      recalc();
    }
  });
  document.addEventListener("keyup", (e) => {
    if (e.key === "Enter" && e.target.matches("[data-monster]")) e.target.click();
  });
  // Buffs and consumables.
  document.addEventListener("change", (e) => {
    const f = e.target.closest("[data-buff]");
    if (!f) return;
    const bf = { ...(build.bf || {}) };
    const v = f.type === "checkbox" ? (f.checked ? 1 : 0) : Math.max(0, Number(f.value) || 0);
    if (v) bf[f.dataset.buff] = v; else delete bf[f.dataset.buff];
    build.bf = bf;
    recalc();
  });
  // Class states for LATAM formulas (selectors and on/off toggles).
  document.addEventListener("change", (e) => {
    const f = e.target.closest("[data-atk-state]");
    if (!f) return;
    const a = { ...(build.a || {}) };
    a.st = { ...(a.st || {}) };
    a.st[f.dataset.atkState] = f.type === "checkbox" ? (f.checked ? 1 : 0) : Number(f.value);
    build.a = a;
    recalc();
  });
  document.addEventListener("change", (e) => {
    const f = e.target.closest("[data-atk]");
    if (!f) return;
    const a = { ...(build.a || {}) };
    const v = f.value;
    switch (f.dataset.atk) {
      case "skill": a.s = Number(v); a.l = (build.k || {})[v] || 0; a.r = 0; a.v = 0; break; // max learned level, ratio from description
      case "lv": a.l = Number(v); a.r = 0; break;
      case "ratio": a.r = Number(v); break;
      case "hits": a.h = Number(v); break;
      case "ele": a.e = v; break;
      case "stack": a.sk = Number(v); break;
      case "variant": a.v = Number(v); break;
    }
    build.a = a;
    recalc();
  });

  // ---- skills window ----
  const skillWin = document.getElementById("win-skills");
  let skillTree = null;
  function initSkills() {
    const root = skillWin.querySelector("[data-tree]");
    if (!root || !window.ROSkillTree) return;
    const data = JSON.parse(root.dataset.tree);
    skillTree = window.ROSkillTree.init(skillWin, data, {
      levels: build.k,
      onChange: (lv) => { build.k = lv; touch(); persistSoon(); },
    });
  }
  async function loadSkills(job) {
    const res = await fetch("/build/skills?job=" + job + "&joblv=" + (build.jl || 0) + "&lang=" + lang);
    if (!res.ok) return;
    document.getElementById("skill-body").innerHTML = await res.text();
    initSkills();
  }
  // Skills do not change the status window, so only the share code is refreshed.
  let persistTimer = null;
  function persistSoon() {
    clearTimeout(persistTimer);
    persistTimer = setTimeout(recalc, 400);
  }

  document.addEventListener("DOMContentLoaded", () => {
    initSkills();
    persist();
    // Messages from a replay import on the start screen.
    try {
      const notes = JSON.parse(sessionStorage.getItem("ro.import.notes") || "[]");
      sessionStorage.removeItem("ro.import.notes");
      notes.forEach((n, i) => say(n, i > 0));
    } catch {}
    say(T("Dica: Alt+A atributos, Alt+Q equipamentos, Alt+S habilidades."));
  });
})();

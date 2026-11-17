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
    if (!params.has("b") && !params.has("new") && last) {
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
  let inflight = null;
  let pending = false;

  async function recalc() {
    if (inflight) { pending = true; return; }
    inflight = fetch("/build/calc?lang=" + lang, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(build),
    });
    try {
      const res = await inflight;
      if (!res.ok) throw new Error("HTTP " + res.status);
      const data = await res.json();
      build = data.build;
      build.e = build.e || {};
      build.k = build.k || {};
      code = data.code;
      for (const n of data.notes || []) say(n, true);
      for (const [id, html] of Object.entries(data.html)) {
        const el = document.getElementById(id);
        if (el) el.innerHTML = html;
      }
      restoreEquipTab();
      if (slotOpen) renderSlotEditor();
      persist();
    } catch (err) {
      say("Erro ao calcular: " + err.message, true);
    } finally {
      inflight = null;
      if (pending) { pending = false; recalc(); }
    }
  }

  function persist() {
    const url = new URL(location.href);
    url.searchParams.set("b", code);
    url.searchParams.delete("new");
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
      say("Classe alterada. Habilidades resetadas.");
    } else if (f.dataset.field === "base") {
      build.b = v;
    } else if (f.dataset.field === "joblv") {
      build.jl = v;
    }
    recalc();
  });

  // ---- basic info buttons ----
  document.addEventListener("click", async (e) => {
    if (e.target.closest("[data-copy-link]")) {
      const link = location.origin + "/build?b=" + code;
      try {
        await navigator.clipboard.writeText(link);
        say("Link do build copiado!");
      } catch {
        prompt("Copie o link:", link);
      }
    }
    if (e.target.closest("[data-new-build]")) {
      if (!confirm("Começar um build novo? O atual continua no link copiado.")) return;
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
    slotWin.classList.remove("is-hidden");
    slotWin.style.zIndex = 50;
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
        say("Carta aplicada.");
        cardIndex = -1;
      } else {
        equip(slotOpen, id, (pick.dataset.loc || "").split(","));
        say("Item equipado.");
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
    if (e.target.closest("[data-unequip]")) { unequip(slotOpen); say("Item removido."); recalc(); }
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

  // ---- skills window ----
  const skillWin = document.getElementById("win-skills");
  function initSkills() {
    const root = skillWin.querySelector("[data-tree]");
    if (!root || !window.ROSkillTree) return;
    const data = JSON.parse(root.dataset.tree);
    window.ROSkillTree.init(skillWin, data, {
      levels: build.k,
      onChange: (lv) => { build.k = lv; persistSoon(); },
    });
  }
  async function loadSkills(job) {
    const res = await fetch("/build/skills?job=" + job + "&lang=" + lang);
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
    say("Dica: Alt+A atributos, Alt+Q equipamentos, Alt+S habilidades.");
  });
})();

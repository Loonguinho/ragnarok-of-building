// Build comparison: up to N builds side by side (saved builds, the current
// build or a pasted link) against one target. The table comes from the
// server (POST /compare/calc), already translated; this file only manages
// the choices. State lives in the URL (?b=CODE&b=CODE&m=ID) so a comparison
// can be shared.
(() => {
  const pickers = document.getElementById("cmp-pickers");
  if (!pickers) return;
  const MAX = Number(pickers.dataset.max) || 4;
  const L = window.CMP_I18N || {};
  const table = document.getElementById("cmp-table");
  const saved = () => (window.ROSaved ? ROSaved.list() : []);
  let current = "";
  try { current = localStorage.getItem("ro.build.last") || ""; } catch {}

  // A column: {code, name}.
  const params = new URLSearchParams(location.search);
  let cols = params.getAll("b").filter(Boolean).slice(0, MAX).map((code) => ({ code, name: nameFor(code) }));
  let target = Number(params.get("m")) || 0;
  if (!cols.length) {
    // Start with the current build and the most recent saved one.
    if (current) cols.push({ code: current, name: L.current });
    for (const e of saved()) {
      if (cols.length >= 2) break;
      if (!cols.some((c) => c.code === e.code)) cols.push({ code: e.code, name: e.name });
    }
  }
  if (cols.length < 2) cols.push({ code: "", name: "" });

  function nameFor(code) {
    const e = saved().find((x) => x.code === code);
    if (e) return e.name;
    return code === current ? L.current : "";
  }

  function option(value, label, selected) {
    const o = document.createElement("option");
    o.value = value;
    o.textContent = label;
    o.selected = selected;
    return o;
  }

  function renderPickers() {
    pickers.replaceChildren();
    cols.forEach((c, i) => {
      const box = document.createElement("div");
      box.className = "ro-compare__picker";
      const sel = document.createElement("select");
      sel.className = "ro-select";
      sel.setAttribute("aria-label", `${L.choose} ${i + 1}`);
      sel.append(option("", `— ${L.choose} —`, !c.code));
      let found = !c.code;
      if (current) {
        const isIt = c.code === current && c.name === L.current;
        found = found || isIt;
        sel.append(option("cur", L.current, isIt));
      }
      for (const e of saved()) {
        const isIt = !found && c.code === e.code;
        found = found || isIt;
        sel.append(option("s:" + e.id, `${e.name} (${e.job || "?"} ${e.b || ""})`, isIt));
      }
      if (c.code && !found) sel.append(option("keep", c.name || L.pasted, true));
      sel.append(option("paste", L.paste, false));
      sel.addEventListener("change", () => {
        const v = sel.value;
        if (v === "cur") cols[i] = { code: current, name: L.current };
        else if (v.startsWith("s:")) {
          const e = saved().find((x) => x.id === v.slice(2));
          if (e) cols[i] = { code: e.code, name: e.name };
        } else if (v === "paste") {
          const raw = (prompt(L.pastePrompt) || "").trim();
          const m = raw.match(/[?&]b=([A-Za-z0-9_-]+)/);
          const code = m ? m[1] : raw.replace(/[^A-Za-z0-9_-]/g, "");
          if (code) cols[i] = { code, name: nameFor(code) };
        } else if (v === "") cols[i] = { code: "", name: "" };
        update();
      });
      box.append(sel);
      if (cols.length > 1) {
        const rm = document.createElement("button");
        rm.type = "button";
        rm.className = "ro-btn";
        rm.textContent = "×";
        rm.title = L.remove;
        rm.addEventListener("click", () => { cols.splice(i, 1); update(); });
        box.append(rm);
      }
      pickers.append(box);
    });
    document.getElementById("cmp-add").disabled = cols.length >= MAX;
  }

  let rev = 0;
  async function update() {
    renderPickers();
    const chosen = cols.filter((c) => c.code);
    const url = new URL(location.href);
    url.search = "";
    chosen.forEach((c) => url.searchParams.append("b", c.code));
    if (target) url.searchParams.set("m", target);
    history.replaceState(null, "", url);
    if (!chosen.length) {
      table.innerHTML = "";
      return;
    }
    const my = ++rev;
    try {
      const res = await fetch("/compare/calc", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ builds: chosen.map((c) => ({ code: c.code, name: c.name })), m: target }),
      });
      if (my !== rev) return;
      if (!res.ok) throw new Error((await res.text()).trim() || res.status);
      const data = await res.json();
      table.innerHTML = data.html;
      placePickers();
      if (data.target) {
        target = data.target.id;
        document.getElementById("cmp-target-name").textContent = data.target.name;
      }
    } catch (err) {
      if (my === rev) table.textContent = String(err.message || err);
    }
  }

  // Put each picker in its column's header cell (the table lists the chosen
  // builds in order; empty pickers stay in the bar above).
  function placePickers() {
    const heads = table.querySelectorAll("thead th");
    let col = 0;
    [...pickers.children].forEach((box, i) => {
      if (!cols[i] || !cols[i].code) return;
      const th = heads[++col];
      if (th) th.prepend(box);
    });
    pickers.hidden = !pickers.children.length;
  }

  document.getElementById("cmp-add").addEventListener("click", () => {
    if (cols.length < MAX) { cols.push({ code: "", name: "" }); update(); }
  });
  document.getElementById("cmp-link").addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(location.href);
      document.getElementById("cmp-link").textContent = L.copied;
    } catch {
      prompt(L.copyPrompt, location.href);
    }
  });

  // Target search (same results as the damage window).
  const q = document.getElementById("cmp-mob-q");
  const list = document.getElementById("cmp-mob-results");
  let timer = null;
  q.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(async () => {
      const v = q.value.trim();
      if (!v) { list.hidden = true; return; }
      const res = await fetch("/build/monsters?q=" + encodeURIComponent(v));
      if (!res.ok) return;
      list.innerHTML = await res.text();
      list.hidden = false;
    }, 200);
  });
  list.addEventListener("click", (e) => {
    const li = e.target.closest("[data-monster]");
    if (!li) return;
    target = Number(li.dataset.monster);
    list.hidden = true;
    q.value = "";
    update();
  });

  update();
})();

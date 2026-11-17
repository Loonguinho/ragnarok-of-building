// In-game style skill tree: click +1, shift+click max, right-click -1,
// shift+right-click 0. Raising a skill fills its prerequisites; lowering one
// below what another skill needs resets the dependents (as in the client).
(() => {
  const data = window.SKILL_TREE;
  if (!data) return;
  const skills = data.skills; // id -> {max, needs: [[id, lv], ...]}
  const KEY = "ro.skills." + data.job;
  let lv = {};
  try { lv = JSON.parse(localStorage.getItem(KEY)) || {}; } catch {}

  const get = (id) => lv[id] || 0;

  function raise(id, to, seen = new Set()) {
    const s = skills[id];
    if (!s || seen.has(id)) return;
    seen.add(id);
    to = Math.min(to, s.max);
    if (to <= get(id)) return;
    for (const [need, needLv] of s.needs || []) {
      if (get(need) < needLv) raise(need, needLv, seen);
    }
    lv[id] = to;
  }

  function lower(id, to) {
    if (to >= get(id)) return;
    if (to <= 0) delete lv[id]; else lv[id] = to;
    // Cascade: anything that now has an unmet prerequisite goes to 0.
    let changed = true;
    while (changed) {
      changed = false;
      for (const [sid, s] of Object.entries(skills)) {
        if (!get(sid)) continue;
        if ((s.needs || []).some(([need, needLv]) => skills[need] && get(need) < needLv)) {
          delete lv[sid];
          changed = true;
        }
      }
    }
  }

  function locked(id) {
    return (skills[id].needs || []).some(([need, needLv]) => skills[need] && get(need) < needLv);
  }

  const win = document.getElementById("win-skills");
  const used = win.querySelector("[data-used]");
  const total = win.querySelector("[data-total]");

  function render() {
    win.querySelectorAll(".ro-skill[data-skill]").forEach((el) => {
      const id = el.dataset.skill;
      el.querySelector("[data-lv]").textContent = get(id);
      el.classList.toggle("is-learned", get(id) > 0);
      el.classList.toggle("is-locked", get(id) === 0 && locked(id));
    });
    const panel = win.querySelector(".ro-skilltree:not([hidden])");
    let tab = 0;
    panel.querySelectorAll(".ro-skill[data-skill]").forEach((el) => { tab += get(el.dataset.skill); });
    used.textContent = tab;
    total.textContent = Object.values(lv).reduce((a, b) => a + b, 0);
    try { localStorage.setItem(KEY, JSON.stringify(lv)); } catch {}
  }

  win.addEventListener("click", (e) => {
    const tabBtn = e.target.closest("[data-tab]");
    if (tabBtn) {
      win.querySelectorAll("[data-tab]").forEach((b) => b.classList.toggle("is-active", b === tabBtn));
      win.querySelectorAll("[data-panel]").forEach((p) => { p.hidden = p.dataset.panel !== tabBtn.dataset.tab; });
      render();
      return;
    }
    if (e.target.closest("[data-reset]")) { lv = {}; render(); return; }
    const el = e.target.closest(".ro-skill[data-skill]");
    if (!el) return;
    const id = el.dataset.skill;
    raise(id, e.shiftKey ? skills[id].max : get(id) + 1);
    render();
  });

  win.addEventListener("contextmenu", (e) => {
    const el = e.target.closest(".ro-skill[data-skill]");
    if (!el) return;
    e.preventDefault();
    const id = el.dataset.skill;
    lower(id, e.shiftKey ? 0 : get(id) - 1);
    render();
  });

  // Keyboard: Enter/+ raises, -/Backspace lowers.
  win.addEventListener("keydown", (e) => {
    const el = e.target.closest(".ro-skill[data-skill]");
    if (!el) return;
    const id = el.dataset.skill;
    if (e.key === "Enter" || e.key === "+") { raise(id, get(id) + 1); render(); }
    else if (e.key === "-" || e.key === "Backspace") { e.preventDefault(); lower(id, get(id) - 1); render(); }
  });

  // RO-style tooltip with the colored description.
  const tip = document.getElementById("ro-tooltip");
  win.addEventListener("mouseover", (e) => {
    const el = e.target.closest(".ro-skill[data-skill]");
    if (!el) return;
    tip.innerHTML = el.querySelector(".ro-tip").innerHTML;
    tip.hidden = false;
  });
  win.addEventListener("mousemove", (e) => {
    if (tip.hidden) return;
    const x = Math.min(e.clientX + 16, window.innerWidth - tip.offsetWidth - 8);
    const y = Math.min(e.clientY + 16, window.innerHeight - tip.offsetHeight - 8);
    tip.style.left = Math.max(8, x) + "px";
    tip.style.top = Math.max(8, y) + "px";
  });
  win.addEventListener("mouseout", (e) => {
    if (!e.relatedTarget || !e.relatedTarget.closest || !e.relatedTarget.closest(".ro-skill[data-skill]")) tip.hidden = true;
  });

  render();
})();

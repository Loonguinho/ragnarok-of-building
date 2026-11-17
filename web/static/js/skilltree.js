// In-game style skill tree: click +1, shift+click max, right-click -1,
// shift+right-click 0. Raising a skill fills its prerequisites; lowering one
// below what another skill needs resets the dependents (as in the client).
//
// ROSkillTree.init(win, data, {levels, onChange}) wires a window whose
// markup comes from the "skill_tree" template. data = {job, skills, levels}.
window.ROSkillTree = (() => {
  function init(win, data, opts = {}) {
    const skills = data.skills || {}; // id -> {max, needs: [[id, lv], ...]}
    let lv = { ...(opts.levels || data.levels || {}) };
    const get = (id) => lv[id] || 0;

    function raise(id, to, seen = new Set()) {
      const s = skills[id];
      if (!s || seen.has(String(id))) return;
      seen.add(String(id));
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
      let changed = true; // cascade: unmet prerequisites reset dependents
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

    const locked = (id) => (skills[id].needs || []).some(([need, needLv]) => skills[need] && get(need) < needLv);

    function render() {
      win.querySelectorAll(".ro-skill[data-skill]").forEach((el) => {
        const id = el.dataset.skill;
        el.querySelector("[data-lv]").textContent = get(id);
        el.classList.toggle("is-learned", get(id) > 0);
        el.classList.toggle("is-locked", get(id) === 0 && locked(id));
      });
      const panel = win.querySelector(".ro-skilltree:not([hidden])");
      let tab = 0;
      if (panel) panel.querySelectorAll(".ro-skill[data-skill]").forEach((el) => { tab += get(el.dataset.skill); });
      const used = win.querySelector("[data-used]");
      const total = win.querySelector("[data-total]");
      if (used) used.textContent = tab;
      if (total) total.textContent = Object.values(lv).reduce((a, b) => a + b, 0);
    }

    function changed() {
      render();
      if (opts.onChange) opts.onChange({ ...lv });
    }

    // Replace previous listeners when the same window is re-initialized.
    if (win._skillAbort) win._skillAbort.abort();
    const ac = new AbortController();
    win._skillAbort = ac;
    const on = (type, fn) => win.addEventListener(type, fn, { signal: ac.signal });

    on("click", (e) => {
      const tabBtn = e.target.closest("[data-tab]");
      if (tabBtn) {
        win.querySelectorAll("[data-tab]").forEach((b) => b.classList.toggle("is-active", b === tabBtn));
        win.querySelectorAll("[data-panel]").forEach((p) => { p.hidden = p.dataset.panel !== tabBtn.dataset.tab; });
        render();
        return;
      }
      if (e.target.closest("[data-reset]")) { lv = {}; changed(); return; }
      const el = e.target.closest(".ro-skill[data-skill]");
      if (!el) return;
      const id = el.dataset.skill;
      raise(id, e.shiftKey ? skills[id].max : get(id) + 1);
      changed();
    });
    on("contextmenu", (e) => {
      const el = e.target.closest(".ro-skill[data-skill]");
      if (!el) return;
      e.preventDefault();
      const id = el.dataset.skill;
      lower(id, e.shiftKey ? 0 : get(id) - 1);
      changed();
    });
    on("keydown", (e) => {
      const el = e.target.closest(".ro-skill[data-skill]");
      if (!el) return;
      const id = el.dataset.skill;
      if (e.key === "Enter" || e.key === "+") { raise(id, get(id) + 1); changed(); }
      else if (e.key === "-" || e.key === "Backspace") { e.preventDefault(); lower(id, get(id) - 1); changed(); }
    });

    // RO-style tooltip with the colored description.
    const tip = document.getElementById("ro-tooltip");
    if (tip) {
      on("mouseover", (e) => {
        const el = e.target.closest(".ro-skill[data-skill]");
        if (!el) return;
        tip.innerHTML = el.querySelector(".ro-tip").innerHTML;
        tip.hidden = false;
      });
      on("mousemove", (e) => {
        if (tip.hidden) return;
        tip.style.left = Math.max(8, Math.min(e.clientX + 16, window.innerWidth - tip.offsetWidth - 8)) + "px";
        tip.style.top = Math.max(8, Math.min(e.clientY + 16, window.innerHeight - tip.offsetHeight - 8)) + "px";
      });
      on("mouseout", (e) => {
        const to = e.relatedTarget;
        if (!to || !to.closest || !to.closest(".ro-skill[data-skill]")) tip.hidden = true;
      });
    }

    render();
    return { levels: () => ({ ...lv }) };
  }

  // Standalone /skills page: persist per job in localStorage.
  document.addEventListener("DOMContentLoaded", () => {
    const win = document.getElementById("win-skills");
    const data = window.SKILL_TREE;
    if (!win || !data || win.dataset.managed) return;
    const KEY = "ro.skills." + data.job;
    let saved = {};
    try { saved = JSON.parse(localStorage.getItem(KEY)) || {}; } catch {}
    init(win, data, {
      levels: saved,
      onChange: (lv) => { try { localStorage.setItem(KEY, JSON.stringify(lv)); } catch {} },
    });
  });

  return { init };
})();

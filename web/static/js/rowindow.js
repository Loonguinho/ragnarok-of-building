// RO window manager: drag by title bar, focus/z-order, close, Alt+<key> toggles,
// positions remembered per window in localStorage (best-effort).
(() => {
  const KEY = "ro.windows";
  const isMobile = () => window.matchMedia("(max-width: 720px)").matches;
  let z = 10;

  const load = () => { try { return JSON.parse(localStorage.getItem(KEY)) || {}; } catch { return {}; } };
  const save = (state) => { try { localStorage.setItem(KEY, JSON.stringify(state)); } catch {} };
  const state = load();

  function focus(win) { win.style.zIndex = ++z; }

  function persist(win) {
    if (!win.id) return;
    state[win.id] = { left: win.style.left, top: win.style.top, hidden: win.classList.contains("is-hidden") };
    save(state);
  }

  function toggle(win) {
    win.classList.toggle("is-hidden");
    if (!win.classList.contains("is-hidden")) focus(win);
    persist(win);
  }

  function init(win) {
    const s = win.id && state[win.id];
    if (s) {
      if (s.left) win.style.left = s.left;
      if (s.top) win.style.top = s.top;
      if (s.hidden) win.classList.add("is-hidden");
    }
    win.addEventListener("mousedown", () => focus(win));

    const close = win.querySelector(".ro-window__close");
    if (close) close.addEventListener("click", (e) => { e.stopPropagation(); toggle(win); });

    const bar = win.querySelector(".ro-window__title");
    if (!bar || win.classList.contains("ro-window--fixed")) return;
    bar.addEventListener("pointerdown", (e) => {
      if (isMobile() || e.target.closest("button")) return;
      const dx = e.clientX - win.offsetLeft, dy = e.clientY - win.offsetTop;
      bar.setPointerCapture(e.pointerId);
      const move = (ev) => {
        const maxX = window.innerWidth - 40, maxY = window.innerHeight - 20;
        win.style.left = Math.min(Math.max(ev.clientX - dx, 0), maxX) + "px";
        win.style.top = Math.min(Math.max(ev.clientY - dy, 0), maxY) + "px";
      };
      const up = () => {
        bar.removeEventListener("pointermove", move);
        bar.removeEventListener("pointerup", up);
        persist(win);
      };
      bar.addEventListener("pointermove", move);
      bar.addEventListener("pointerup", up);
    });
  }

  document.addEventListener("DOMContentLoaded", () => {
    document.querySelectorAll(".ro-window").forEach(init);

    // Windows swapped in by HTMX (e.g. item info) get the same behavior.
    document.body.addEventListener("htmx:afterSwap", (e) => {
      const root = e.detail.target;
      const wins = root.matches(".ro-window") ? [root] : root.querySelectorAll(".ro-window");
      wins.forEach((w) => { w.classList.remove("is-hidden"); init(w); focus(w); });
    });

    document.addEventListener("click", (e) => {
      const btn = e.target.closest("[data-toggle]");
      if (!btn) return;
      const win = document.getElementById(btn.dataset.toggle);
      if (win) toggle(win);
    });

    // Alt+A / Alt+Q / Alt+S / Alt+E / Alt+V, as in the client.
    document.addEventListener("keydown", (e) => {
      if (!e.altKey || e.ctrlKey || e.metaKey) return;
      const win = document.querySelector(`.ro-window[data-hotkey="${e.key.toLowerCase()}"]`);
      if (win) { e.preventDefault(); toggle(win); }
    });
  });
})();

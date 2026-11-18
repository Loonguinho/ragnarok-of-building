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
  const isTop = (win) => Number(win.style.zIndex || 0) === z;

  function persist(win) {
    if (!win.id) return;
    state[win.id] = { left: win.style.left, top: win.style.top, hidden: win.classList.contains("is-hidden") };
    save(state);
  }

  // Like the client: a hidden window opens; a covered window comes to the
  // front; only the window already in front is closed.
  function toggle(win) {
    if (win.classList.contains("is-hidden")) {
      show(win);
    } else if (!isTop(win)) {
      focus(win);
    } else {
      win.classList.add("is-hidden");
      persist(win);
    }
  }

  function show(win) {
    win.classList.remove("is-hidden");
    focus(win);
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
    if (close) close.addEventListener("click", (e) => {
      e.stopPropagation();
      win.classList.add("is-hidden");
      persist(win);
    });

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

  window.ROWin = { show, focus, toggle };

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

// Enter on a focused list row acts like a click (item lists, menus).
document.addEventListener("keyup", (e) => {
  if (e.key === "Enter" && e.target.matches && e.target.matches(".ro-itemrow")) e.target.click();
});

// Classic RO menu click on buttons. The sound comes from the player's own
// client (the server only has it after the import). A plain <audio> copy
// per click plays on the click itself (the gesture browsers require) and
// lets quick clicks overlap. Links that leave the page wait a moment so the
// click is heard before the page changes, like the game's menus. Muted with
// the toggle on the home screen (remembered in this browser).
(() => {
  const src = document.body && document.body.dataset.clickSound;
  if (!src) return;
  const KEY = "ro.sound";
  const isOn = () => { try { return localStorage.getItem(KEY) !== "off"; } catch { return true; } };
  const CLICKABLE = ".ro-btn, .ro-window__close, .ro-homemenu__item, .ro-charslot, .ro-jobcard, .ro-eslot, [data-equip-tab], [data-tab], .ro-list__item, .ro-plus";
  const base = new Audio(src);
  base.preload = "auto";
  // For troubleshooting in the console: ROClick
  window.ROClick = { clicks: 0, played: 0, ended: 0, muted: !isOn(), failed: "", last: "" };
  const VOLUME = 0.6;
  function play() {
    window.ROClick.muted = !isOn();
    if (!isOn()) return null;
    const a = base.cloneNode();
    a.volume = VOLUME;
    a.addEventListener("ended", () => { window.ROClick.ended++; });
    const p = a.play();
    if (p) p.then(() => { window.ROClick.played++; }, (err) => { window.ROClick.failed = String(err); });
    return a;
  }
  // Fade a playing click out (instead of the page change cutting it dead).
  function fadeOut(a, ms, done) {
    if (!a) { done(); return; }
    const steps = 6;
    let i = 0;
    const t = setInterval(() => {
      i++;
      a.volume = Math.max(0, VOLUME * (1 - i / steps));
      if (i >= steps) { clearInterval(t); done(); }
    }, ms / steps);
  }
  // A plain left click on a link to another page of the site.
  function leavingLink(el, e) {
    const a = el.closest("a[href]");
    if (!a || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey || a.target) return null;
    const href = a.getAttribute("href");
    if (!href || href.startsWith("#") || a.hasAttribute("download")) return null;
    const url = new URL(a.href, location.href);
    return url.origin === location.origin ? url : null;
  }
  document.addEventListener("click", (e) => {
    const el = e.target.closest(CLICKABLE);
    if (!el || el.disabled || el.matches("[data-sound-toggle]")) return;
    window.ROClick.clicks++;
    window.ROClick.last = (el.textContent || el.className).trim().slice(0, 40);
    const a = play();
    const url = a && !e.defaultPrevented ? leavingLink(el, e) : null;
    if (url) {
      // Let the click sound for half a second (fading out at the end), then go.
      e.preventDefault();
      setTimeout(() => fadeOut(a, 100, () => { location.href = url.href; }), 400);
    }
  }, true);
  // Toggle (home screen).
  const sync = () => document.querySelectorAll("[data-sound-toggle]").forEach((b) => {
    b.setAttribute("aria-pressed", String(isOn()));
    b.querySelector("[data-on]").hidden = !isOn();
    b.querySelector("[data-off]").hidden = isOn();
  });
  document.addEventListener("click", (e) => {
    if (!e.target.closest("[data-sound-toggle]")) return;
    try { localStorage.setItem(KEY, isOn() ? "off" : "on"); } catch {}
    sync();
    play();
  });
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", sync); else sync();
})();

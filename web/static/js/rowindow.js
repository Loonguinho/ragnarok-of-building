// RO window manager: drag by title bar, focus/z-order, close, Alt+<key> toggles,
// positions remembered per window in localStorage (best-effort).
(() => {
  const KEY = "ro.windows";
  const isMobile = () => window.matchMedia("(max-width: 720px)").matches;
  let z = 10;

  const load = () => { try { return JSON.parse(localStorage.getItem(KEY)) || {}; } catch { return {}; } };
  const save = (state) => { try { localStorage.setItem(KEY, JSON.stringify(state)); } catch {} };
  const state = load();

  // Windows laid out in the build editor's fixed columns stay put.
  const fixed = (win) => !!win.closest(".ro-desk");
  const offScreen = (win) => {
    const r = win.getBoundingClientRect();
    return r.bottom < 40 || r.top > window.innerHeight - 40;
  };
  function focus(win) { if (!fixed(win)) win.style.zIndex = ++z; }
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
    } else if (fixed(win) && offScreen(win)) {
      win.scrollIntoView({ block: "nearest", behavior: "smooth" }); // open but scrolled away: go to it
    } else if (!fixed(win) && !isTop(win)) {
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
    if (fixed(win)) win.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }

  function init(win) {
    const s = win.id && state[win.id];
    if (s) {
      if (s.left && !fixed(win)) win.style.left = s.left;
      if (s.top && !fixed(win)) win.style.top = s.top;
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
    if (!bar || win.classList.contains("ro-window--fixed") || fixed(win)) return;
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

// Classic RO menu click on the home screen's buttons (the editor stays
// quiet). The sound comes from the player's own client (the server only has
// it after the import).
//
// It plays through one Web Audio context kept open for the page. Most home
// buttons open another page, which would cut the sound: those wait until
// the click has finished sounding (measured from the sound itself), with
// the button shown pressed and the next page already requested. Muted with
// the toggle on the home screen.
(() => {
  const src = document.body && document.body.dataset.clickSound;
  if (!src) return;
  const KEY = "ro.sound";
  const VOLUME = 0.6;
  const isOn = () => { try { return localStorage.getItem(KEY) !== "off"; } catch { return true; } };
  const CLICKABLE = ".ro-btn, .ro-homemenu__item, .ro-charslot";
  // For troubleshooting in the console: ROClick
  const info = window.ROClick = { clicks: 0, played: 0, ended: 0, muted: !isOn(), failed: "", last: "", engine: "", waitMs: 0 };

  const raw = fetch(src).then((r) => (r.ok ? r.arrayBuffer() : null)).catch(() => null);
  let ctx = null, buf = null, decoding = null, audibleMs = 1300;
  function wake() {
    if (!ctx) {
      const AC = window.AudioContext || window.webkitAudioContext;
      if (!AC) return null;
      ctx = new AC();
    }
    if (ctx.state === "suspended") ctx.resume();
    return ctx;
  }
  // Where the sound becomes inaudible: last sample above 3% of the peak.
  function measure(b) {
    const d = b.getChannelData(0);
    let peak = 0;
    for (let i = 0; i < d.length; i++) peak = Math.max(peak, Math.abs(d[i]));
    let end = d.length - 1;
    while (end > 0 && Math.abs(d[end]) < peak * 0.03) end--;
    audibleMs = Math.min(1800, Math.round((end / b.sampleRate) * 1000) + 60);
  }
  function buffer() {
    if (buf) return Promise.resolve(buf);
    if (!decoding) {
      decoding = raw.then((b) => (b ? ctx.decodeAudioData(b.slice(0)) : null)).then((d) => {
        buf = d;
        if (d) measure(d);
        return d;
      });
    }
    return decoding;
  }
  function play() {
    info.muted = !isOn();
    if (!isOn()) return false;
    const c = wake();
    if (!c) { // no Web Audio: plain element
      info.engine = "audio";
      const a = new Audio(src);
      a.volume = VOLUME;
      a.addEventListener("ended", () => { info.ended++; });
      a.play().then(() => { info.played++; }, (err) => { info.failed = String(err); });
      return true;
    }
    info.engine = "webaudio";
    buffer().then((b) => {
      if (!b) { info.failed = "sound not loaded"; return; }
      const node = c.createBufferSource();
      const gain = c.createGain();
      gain.gain.value = VOLUME;
      node.buffer = b;
      node.connect(gain).connect(c.destination);
      node.onended = () => { info.ended++; };
      node.start();
      info.played++;
    }).catch((err) => { info.failed = String(err); });
    return true;
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
  // Wake the context on the first press, so the first click isn't late.
  document.addEventListener("pointerdown", () => { if (isOn()) { wake(); buffer(); } }, { capture: true, once: true });
  let leaving = false;
  document.addEventListener("click", (e) => {
    const el = e.target.closest(CLICKABLE);
    if (!el || el.disabled || el.matches("[data-sound-toggle]")) return;
    if (leaving) { e.preventDefault(); return; } // already on the way out
    info.clicks++;
    info.last = (el.textContent || el.className).trim().slice(0, 40);
    const played = play();
    const url = played && !e.defaultPrevented ? leavingLink(el, e) : null;
    if (url) {
      // Let the whole click sound, then open the page (already requested).
      e.preventDefault();
      leaving = true;
      el.classList.add("is-pressed");
      const pre = document.createElement("link");
      pre.rel = "prefetch";
      pre.href = url.href;
      document.head.appendChild(pre);
      const go = () => { location.href = url.href; };
      (buf || !ctx ? Promise.resolve() : buffer()).then(() => {
        info.waitMs = audibleMs;
        setTimeout(go, audibleMs);
      }, () => setTimeout(go, 600));
    }
  }, true);
  // Back/forward cache: a page restored after leaving must accept clicks again.
  window.addEventListener("pageshow", () => {
    leaving = false;
    document.querySelectorAll(".is-pressed").forEach((b) => b.classList.remove("is-pressed"));
  });
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

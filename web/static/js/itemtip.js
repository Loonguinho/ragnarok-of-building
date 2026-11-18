// Item tooltips: hovering (or focusing) anything with data-tip-item="ID"
// shows the item's description from the client, like the game's item window.
(() => {
  const cache = new Map();
  const tip = document.createElement("div");
  tip.className = "ro-tip";
  tip.setAttribute("role", "tooltip");
  tip.hidden = true;
  let current = null; // element the tip is for
  let timer = null;
  let mouse = { x: 0, y: 0 };

  const lang = () => new URLSearchParams(location.search).get("lang") || document.documentElement.lang.slice(0, 2) || "pt";

  async function load(id) {
    const key = lang() + "/" + id;
    if (!cache.has(key)) {
      cache.set(key, fetch("/items/" + id + "/tip?lang=" + lang()).then((r) => (r.ok ? r.text() : "")).catch(() => ""));
    }
    return cache.get(key);
  }

  function place() {
    // The element was re-rendered away (a recalc): drop its tip.
    if (current && !current.isConnected) { hide(); return; }
    const pad = 14;
    const w = tip.offsetWidth, h = tip.offsetHeight;
    let x = mouse.x + pad, y = mouse.y + pad;
    if (x + w > innerWidth - 4) x = Math.max(4, mouse.x - w - pad);
    if (y + h > innerHeight - 4) y = Math.max(4, innerHeight - h - 4);
    tip.style.left = x + "px";
    tip.style.top = y + "px";
  }

  function hide() {
    clearTimeout(timer);
    current = null;
    tip.hidden = true;
  }

  function show(el, fromKeyboard) {
    const id = el.dataset.tipItem;
    if (!id || id === "0") return;
    current = el;
    clearTimeout(timer);
    timer = setTimeout(async () => {
      const html = await load(id);
      if (current !== el || !html) return;
      if (fromKeyboard) {
        const r = el.getBoundingClientRect();
        mouse = { x: r.right, y: r.top };
      }
      tip.innerHTML = html;
      tip.hidden = false;
      place();
    }, 120);
  }

  document.addEventListener("DOMContentLoaded", () => document.body.appendChild(tip));
  document.addEventListener("mouseover", (e) => {
    const el = e.target.closest("[data-tip-item]");
    if (!el) { if (current) hide(); return; }
    if (el === current && !tip.hidden) return;
    mouse = { x: e.clientX, y: e.clientY };
    show(el, false);
  });
  document.addEventListener("mousemove", (e) => {
    mouse = { x: e.clientX, y: e.clientY };
    if (!tip.hidden) place();
  });
  document.addEventListener("focusin", (e) => {
    const el = e.target.closest("[data-tip-item]");
    if (el) show(el, true); else hide();
  });
  document.addEventListener("focusout", hide);
  document.addEventListener("mousedown", hide);
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") hide(); });
  addEventListener("scroll", hide, true);
})();

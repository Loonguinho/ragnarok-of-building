// Builds saved in this browser (localStorage). Each entry:
// {id, name, code, j (job id), job (job name), b (base lv), jl (job lv), t (saved at, ms)}.
// Storage can be unavailable (private mode, blocked site data): every call
// is guarded and the app keeps working, just without the list.
window.ROSaved = (() => {
  const KEY = "ro.saved.v1";
  const read = () => {
    try {
      const v = JSON.parse(localStorage.getItem(KEY) || "[]");
      return Array.isArray(v) ? v.filter((e) => e && e.id && e.code) : [];
    } catch { return []; }
  };
  const write = (list) => {
    try { localStorage.setItem(KEY, JSON.stringify(list)); return true; } catch { return false; }
  };
  return {
    list: () => read().sort((a, b) => (b.t || 0) - (a.t || 0)),
    get: (id) => read().find((e) => e.id === id) || null,
    // put inserts or updates (by id) and returns the saved entry, or null
    // when the browser refused to store it.
    put(entry) {
      const list = read();
      const e = { ...entry, id: entry.id || Date.now().toString(36) + Math.random().toString(36).slice(2, 6), t: Date.now() };
      const i = list.findIndex((x) => x.id === e.id);
      if (i >= 0) list[i] = e; else list.push(e);
      return write(list) ? e : null;
    },
    remove: (id) => write(read().filter((e) => e.id !== id)),
    available() {
      try { localStorage.setItem("ro.test", "1"); localStorage.removeItem("ro.test"); return true; } catch { return false; }
    },
  };
})();

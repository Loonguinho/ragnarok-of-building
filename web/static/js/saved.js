// Saved builds. Each entry:
// {id, name, code, j (job id), job (job name), b (base lv), jl (job lv), t (saved at, ms)}.
//
// Where they live:
//  - in a file, when the page carries #ro-saved-file (the desktop app keeps
//    builds.json next to the executable): the list comes with the page and
//    every change is sent back (PUT /saved);
//  - otherwise in this browser (localStorage). Storage can be unavailable
//    (private mode, blocked site data): every call is guarded and the app
//    keeps working, just without the list.
window.ROSaved = (() => {
  const KEY = "ro.saved.v1";
  const MOVED = "ro.saved.v1.infile"; // browser builds already copied to the file
  const valid = (v) => (Array.isArray(v) ? v.filter((e) => e && e.id && e.code) : []);
  const readLocal = () => {
    try { return valid(JSON.parse(localStorage.getItem(KEY) || "[]")); } catch { return []; }
  };
  const writeLocal = (list) => {
    try { localStorage.setItem(KEY, JSON.stringify(list)); return true; } catch { return false; }
  };

  let fileList = null; // null: browser mode
  const tag = document.getElementById("ro-saved-file");
  if (tag) {
    try { fileList = valid(JSON.parse(tag.textContent || "[]")); } catch { fileList = []; }
  }
  const send = (list) => {
    fetch("/saved", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(list), keepalive: list.length < 200 })
      .then((r) => { if (!r.ok) throw new Error(r.status); })
      .catch(() => window.dispatchEvent(new CustomEvent("ro-saved-error")));
  };
  if (fileList) {
    // First time with the file: bring the builds saved in this browser.
    let moved = false;
    try { moved = localStorage.getItem(MOVED) === "1"; } catch {}
    if (!moved) {
      const have = new Set(fileList.map((e) => e.id));
      const extra = readLocal().filter((e) => !have.has(e.id));
      if (extra.length) { fileList = fileList.concat(extra); send(fileList); }
      try { localStorage.setItem(MOVED, "1"); } catch {}
    }
  }
  const read = () => (fileList ? fileList.slice() : readLocal());
  const write = (list) => {
    if (fileList) { fileList = list; send(list); return true; }
    return writeLocal(list);
  };
  const newID = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);

  return {
    inFile: () => !!fileList,
    list: () => read().sort((a, b) => (b.t || 0) - (a.t || 0)),
    get: (id) => read().find((e) => e.id === id) || null,
    // put inserts or updates (by id) and returns the saved entry, or null
    // when it could not be stored.
    put(entry) {
      const list = read();
      const e = { ...entry, id: entry.id || newID(), t: Date.now() };
      const i = list.findIndex((x) => x.id === e.id);
      if (i >= 0) list[i] = e; else list.push(e);
      return write(list) ? e : null;
    },
    remove: (id) => write(read().filter((e) => e.id !== id)),
    available() {
      if (fileList) return true;
      try { localStorage.setItem("ro.test", "1"); localStorage.removeItem("ro.test"); return true; } catch { return false; }
    },
    // exportJSON is every saved build, for a backup file.
    exportJSON: () => JSON.stringify({ app: "ragnarok-of-building", version: 1, builds: read() }, null, 1),
    // importJSON merges a backup (the export, or a plain list): a build with
    // an id already here replaces it when it is newer. Returns how many came
    // in, or -1 when the file is not a backup.
    importJSON(text) {
      let v;
      try { v = JSON.parse(text); } catch { return -1; }
      const incoming = valid(Array.isArray(v) ? v : v && v.builds);
      if (!incoming.length && !(v && (Array.isArray(v) || Array.isArray(v.builds)))) return -1;
      const list = read();
      let n = 0;
      for (const e of incoming) {
        const i = list.findIndex((x) => x.id === e.id);
        if (i < 0) { list.push({ ...e, t: e.t || Date.now() }); n++; }
        else if ((e.t || 0) > (list[i].t || 0)) { list[i] = e; n++; }
      }
      return write(list) ? n : -1;
    },
  };
})();

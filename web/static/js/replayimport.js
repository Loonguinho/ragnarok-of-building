// Replay import, shared by the start page and the build editor's Menu:
// sends the .rrf, keeps the import notes and the damage window for the next
// page (sessionStorage) and opens the imported build.
window.ROReplay = {
  // upload(file, show): show(htmlMessage, isError) reports progress.
  async upload(file, show) {
    if (!file) return;
    if (!file.name.toLowerCase().endsWith(".rrf")) { show(T("Escolha um arquivo <b>.rrf</b>."), true); return; }
    show(T("Lendo <b>%s</b>…", file.name.replace(/[<>&]/g, "")));
    const form = new FormData();
    form.append("file", file);
    try {
      const res = await fetch("/build/import", { method: "POST", body: form });
      const data = await res.json();
      if (!res.ok) { show(data.error || T("Falha ao importar."), true); return; }
      const s = data.summary;
      const notes = [T("Replay importado: %s, Lv %s/%s, %s equipamento(s), %s habilidade(s).", s.job, s.base, s.joblv, s.items, s.skills), ...(data.notes || [])];
      try { sessionStorage.setItem("ro.import.notes", JSON.stringify(notes)); } catch {}
      // The replay damage window: kept for this tab until the next import.
      try {
        if (data.damage) sessionStorage.setItem("ro.import.damage", JSON.stringify({ html: data.damage, open: true }));
        else sessionStorage.removeItem("ro.import.damage");
      } catch {}
      location.href = "/build?b=" + encodeURIComponent(data.code);
    } catch (e) {
      show(T("Falha ao enviar o arquivo."), true);
    }
  },
};

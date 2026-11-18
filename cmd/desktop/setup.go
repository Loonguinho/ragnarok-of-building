package main

import (
	"context"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ragnarok-of-building/internal/setup"
	assets "ragnarok-of-building/web"
)

// setupState runs the first-run (or refresh) import in the background and
// reports its progress to the setup page.
type setupState struct {
	a  *app
	mu sync.Mutex
	// running/done/err/step describe the current import.
	running bool
	done    bool
	err     string
	step    string
	mux     *http.ServeMux
}

func newSetupState(a *app) *setupState {
	s := &setupState{a: a}
	m := http.NewServeMux()
	static, _ := fs.Sub(assets.FS, "static")
	m.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	m.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	m.HandleFunc("/setup", s.page)
	m.HandleFunc("/setup/start", s.start)
	m.HandleFunc("/setup/status", s.status)
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/setup", http.StatusSeeOther) })
	s.mux = m
	return s
}

func (s *setupState) routes() http.Handler { return s.mux }

func (s *setupState) clientFile() string { return filepath.Join(s.a.dir, "client.txt") }

// savedClient is the client folder chosen before, or one found on disk.
func (s *setupState) savedClient() string {
	if b, err := os.ReadFile(s.clientFile()); err == nil {
		if d := strings.TrimSpace(string(b)); setup.IsClient(d) {
			return d
		}
	}
	return setup.FindClient()
}

var setupPage = template.Must(template.New("setup").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Ragnarok of Building</title><link rel="stylesheet" href="/static/css/ro-ui.css"><link rel="icon" type="image/png" href="/static/img/icon.png"></head>
<body class="ro-desktop">
<main class="ro-home">
  <h1 class="ro-logo">Ragnarok <span>of Building</span></h1>
  <section class="ro-window ro-window--fixed" style="width:min(460px,100%)">
    <header class="ro-window__title">[Kafra] {{if .Refresh}}Atualizar dados do jogo{{else}}Preparando o simulador{{end}}</header>
    <div class="ro-window__body ro-npc">
      <p>{{if .Refresh}}Vou ler de novo o cliente do RO para pegar itens e habilidades novos.{{else}}Bem-vindo! Na primeira vez eu preciso ler os dados do seu cliente do <b>RO LATAM</b> e baixar as tabelas do rAthena (uns 15 MB). Leva menos de um minuto.{{end}}</p>
      <form id="f" class="ro-setup">
        <label>Pasta do cliente do RO
          <input class="ro-input" name="client" value="{{.Client}}" placeholder="C:\Gravity\Ragnarok" required>
        </label>
        <small>A pasta que tem o arquivo <b>data.grf</b>.</small>
        <div class="ro-setup__buttons">
          {{if .Refresh}}<a class="ro-btn" href="/">Cancelar</a>{{end}}
          <button class="ro-btn" type="submit">{{if .Refresh}}Atualizar{{else}}Começar{{end}}</button>
        </div>
      </form>
      <div class="ro-setup__status" id="st" aria-live="polite"></div>
    </div>
  </section>
</main>
<script>
(() => {
  const f = document.getElementById("f"), st = document.getElementById("st");
  const show = (html, bad) => { st.innerHTML = html; st.classList.toggle("is-bad", !!bad); };
  async function poll() {
    const s = await (await fetch("/setup/status")).json();
    if (s.err) { show("❌ " + s.err, true); f.hidden = false; return; }
    if (s.done) { show("✔ Pronto! Abrindo…"); location.href = "/"; return; }
    show("⏳ " + (s.step || "Começando…"));
    setTimeout(poll, 400);
  }
  f.addEventListener("submit", async (e) => {
    e.preventDefault();
    const res = await fetch("/setup/start", { method: "POST", body: new URLSearchParams(new FormData(f)) });
    if (!res.ok) { show("❌ " + (await res.text()), true); return; }
    f.hidden = true;
    poll();
  });
  {{if .Auto}}f.requestSubmit();{{end}}
  {{if .Running}}f.hidden = true; poll();{{end}}
})();
</script>
</body></html>`))

func (s *setupState) page(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	client := s.savedClient()
	refresh := r.URL.Query().Has("refresh")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setupPage.Execute(w, map[string]any{
		"Client":  client,
		"Refresh": refresh,
		// First run with the client found: start right away.
		"Auto":    !refresh && !running && client != "",
		"Running": running,
	})
}

func (s *setupState) start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	client := strings.Trim(strings.TrimSpace(r.PostFormValue("client")), `"`)
	if !setup.IsClient(client) {
		http.Error(w, "Não achei o data.grf nessa pasta. Escolha a pasta onde o RO está instalado.", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		return
	}
	s.running, s.done, s.err, s.step = true, false, "", "Começando…"
	s.mu.Unlock()
	os.WriteFile(s.clientFile(), []byte(client), 0o644)
	go s.run(client)
	w.WriteHeader(http.StatusAccepted)
}

func (s *setupState) progress(msg string) {
	s.mu.Lock()
	s.step = msg
	s.mu.Unlock()
	s.a.log.Info("setup", "step", msg)
}

func (s *setupState) run(client string) {
	finish := func(err error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		if err != nil {
			s.err = err.Error()
			s.a.log.Error("setup", "err", err)
			return
		}
		s.done = true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := setup.FetchRathena(ctx, s.a.rathenaDir(), s.progress); err != nil {
		finish(errorf("Não consegui baixar os dados do rAthena (precisa de internet na primeira vez): %v", err))
		return
	}
	tmp := s.a.catalogPath() + ".new"
	cat, err := setup.BuildCatalog(client, s.a.rathenaDir(), tmp, s.progress)
	if err != nil {
		finish(errorf("Não consegui ler o cliente do RO: %v", err))
		return
	}
	if err := os.Rename(tmp, s.a.catalogPath()); err != nil {
		finish(err)
		return
	}
	finish(s.a.useCatalog(cat))
}

func (s *setupState) status(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	out := map[string]any{"running": s.running, "done": s.done, "err": s.err, "step": s.step}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	return o == "" || o == "http://"+r.Host
}

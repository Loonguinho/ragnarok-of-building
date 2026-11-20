package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"ragnarok-of-building/internal/setup"
)

// startup is what the window shows while the program gets the game data
// ready (no console): the progress lines, a question for the client folder
// when it is not found, and the error that stopped it. The page polls
// /startup/status and goes to the simulator once it is ready.
type startup struct {
	log *slog.Logger

	mu      sync.Mutex
	lines   []string
	asking  bool
	askNote string
	errMsg  string
	next    string // where to go when ready ("/build")

	clientCh chan string
	// pick opens the system's folder picker (nil where there is none).
	pick func() (string, error)
	// onChange is called after each change (the native window redraws).
	onChange func()
}

func (st *startup) changed() {
	if st.onChange != nil {
		st.onChange()
	}
}

func newStartup(log *slog.Logger, pick func() (string, error)) *startup {
	return &startup{log: log, clientCh: make(chan string, 1), pick: pick}
}

// say adds a progress line.
func (st *startup) say(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	st.log.Info(msg)
	st.mu.Lock()
	st.lines = append(st.lines, msg)
	if len(st.lines) > 200 {
		st.lines = st.lines[len(st.lines)-200:]
	}
	st.mu.Unlock()
	st.changed()
}

// askClient waits for the player to give the client folder (one with a
// data.grf), and remembers it in .env.
func (st *startup) askClient(note string) string {
	st.mu.Lock()
	st.asking, st.askNote = true, note
	st.mu.Unlock()
	st.changed()
	dir := <-st.clientCh
	st.mu.Lock()
	st.asking = false
	st.mu.Unlock()
	remember("RO_CLIENT", dir)
	return dir
}

func (st *startup) fail(err error) {
	st.log.Error("startup", "err", err)
	st.mu.Lock()
	st.errMsg = err.Error()
	st.mu.Unlock()
	st.changed()
}

func (st *startup) ready(next string) {
	st.mu.Lock()
	st.next = next
	st.mu.Unlock()
}

type startupStatus struct {
	Lines   []string `json:"lines"`
	Ask     bool     `json:"ask"`
	AskNote string   `json:"askNote,omitempty"`
	CanPick bool     `json:"canPick"`
	Error   string   `json:"error,omitempty"`
	Next    string   `json:"next,omitempty"`
}

func (st *startup) status() startupStatus {
	st.mu.Lock()
	defer st.mu.Unlock()
	return startupStatus{Lines: append([]string(nil), st.lines...), Ask: st.asking, AskNote: st.askNote,
		CanPick: st.pick != nil, Error: st.errMsg, Next: st.next}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// ServeHTTP answers /startup (the page) and its calls.
func (st *startup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/startup/status" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, st.status())
	case r.URL.Path == "/startup/client" && r.Method == http.MethodPost:
		dir := strings.Trim(strings.TrimSpace(r.FormValue("dir")), `"`)
		if err := st.useClient(dir); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"dir": dir})
	case r.URL.Path == "/startup/pick" && r.Method == http.MethodPost:
		if st.pick == nil {
			http.NotFound(w, r)
			return
		}
		dir, err := st.pick()
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"dir": dir})
	default:
		st.page(w, r)
	}
}

// useClient hands a folder to askClient, when it holds a client.
func (st *startup) useClient(dir string) error {
	if !setup.IsClient(dir) {
		return errors.New("Essa pasta não tem o data.grf do jogo. Escolha a pasta onde o Ragnarok está instalado (ex.: C:\\Gravity\\Ragnarok).")
	}
	st.mu.Lock()
	asking := st.asking
	st.mu.Unlock()
	if !asking {
		return errors.New("A pasta do jogo não foi pedida agora.")
	}
	select {
	case st.clientCh <- dir:
	default:
	}
	return nil
}

func (st *startup) page(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(startupPage))
}

// startupPage: an RO-style window with the progress (no template: it is
// shown before the game data exists).
const startupPage = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Ragnarok of Building</title>
<link rel="stylesheet" href="/static/css/ro-ui.css">
<link rel="icon" type="image/png" href="/static/img/icon.png">
<style>
  body { display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
  .st-win { width: min(560px, calc(100vw - 32px)); }
  .st-log { height: 220px; overflow: auto; margin: 0; padding: 6px 8px; list-style: none; background: #fff;
    border: 1px solid #b8c7d9; font-size: 11px; line-height: 1.5; }
  .st-log li:last-child { font-weight: bold; }
  .st-err { color: #b00; margin: 8px 0 0; white-space: pre-wrap; }
  .st-ask { margin-top: 8px; }
  .st-ask p { margin: 0 0 6px; }
  .st-row { display: flex; gap: 4px; }
  .st-row input { flex: 1; min-width: 0; }
  .st-note { color: #555; margin: 6px 0 0; }
</style>
</head>
<body class="ro-desktop">
<section class="ro-window ro-window--fixed st-win">
  <header class="ro-window__title">Ragnarok of Building</header>
  <div class="ro-window__body">
    <ul class="st-log" id="log" aria-live="polite"><li>Preparando…</li></ul>
    <p class="st-err" id="err" hidden></p>
    <form class="st-ask" id="ask" hidden>
      <p id="ask-note"></p>
      <div class="st-row">
        <input class="ro-input" id="dir" name="dir" placeholder="C:\Gravity\Ragnarok" autocomplete="off">
        <button class="ro-btn" type="button" id="pick" hidden>Escolher pasta…</button>
        <button class="ro-btn" type="submit">Usar esta pasta</button>
      </div>
      <p class="st-err" id="ask-err" hidden></p>
      <p class="st-note">Ou feche o programa, copie o <b>data.grf</b> e a pasta <b>System</b> do jogo para a pasta do RagnarokOfBuilding.exe e abra de novo.</p>
    </form>
  </div>
</section>
<script>
(() => {
  const $ = (id) => document.getElementById(id);
  let shown = 0, stop = false;
  async function poll() {
    if (stop) return;
    try {
      const s = await (await fetch("/startup/status", { cache: "no-store" })).json();
      if (s.next) { stop = true; location.replace(s.next); return; }
      if (s.lines.length !== shown) {
        $("log").replaceChildren(...s.lines.map((t) => { const li = document.createElement("li"); li.textContent = t; return li; }));
        $("log").scrollTop = $("log").scrollHeight;
        shown = s.lines.length;
      }
      $("err").hidden = !s.error;
      $("err").textContent = s.error ? "Não deu para continuar: " + s.error : "";
      $("ask").hidden = !s.ask;
      if (s.ask) { $("ask-note").textContent = s.askNote || ""; $("pick").hidden = !s.canPick; }
      if (s.error) stop = true;
    } catch (e) {}
    setTimeout(poll, 500);
  }
  async function send(dir) {
    const body = new URLSearchParams({ dir });
    const res = await fetch("/startup/client", { method: "POST", body });
    const d = await res.json();
    $("ask-err").hidden = res.ok;
    $("ask-err").textContent = res.ok ? "" : d.error;
  }
  $("ask").addEventListener("submit", (e) => { e.preventDefault(); send($("dir").value); });
  $("pick").addEventListener("click", async () => {
    const d = await (await fetch("/startup/pick", { method: "POST" })).json();
    if (d.dir) { $("dir").value = d.dir; send(d.dir); }
  });
  poll();
})();
</script>
</body>
</html>
`

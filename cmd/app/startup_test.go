package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartupFlow(t *testing.T) {
	st := newStartup(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	status := func() startupStatus {
		w := httptest.NewRecorder()
		st.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/startup/status", nil))
		var s startupStatus
		json.Unmarshal(w.Body.Bytes(), &s)
		return s
	}
	post := func(dir string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/startup/client", strings.NewReader(url.Values{"dir": {dir}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		st.ServeHTTP(w, r)
		return w.Code
	}

	st.say("Lendo o cliente…")
	if s := status(); len(s.Lines) != 1 || s.Ask || s.Next != "" || s.CanPick {
		t.Fatalf("status %+v", s)
	}

	// A folder given when none was asked for is refused.
	client := t.TempDir()
	os.WriteFile(filepath.Join(client, "data.grf"), []byte("x"), 0o644)
	if c := post(client); c != http.StatusUnprocessableEntity {
		t.Errorf("unasked folder: %d", c)
	}

	// askClient waits; a folder without data.grf is refused, a client is taken.
	wd, _ := os.Getwd()
	os.Chdir(t.TempDir()) // remember() writes .env here
	defer os.Chdir(wd)
	got := make(chan string)
	go func() { got <- st.askClient("Escolha a pasta do jogo") }()
	for !status().Ask {
		time.Sleep(5 * time.Millisecond)
	}
	if s := status(); s.AskNote == "" {
		t.Error("no question shown")
	}
	if c := post(t.TempDir()); c != http.StatusUnprocessableEntity {
		t.Errorf("folder without data.grf: %d", c)
	}
	if c := post(client); c != http.StatusOK {
		t.Fatalf("client folder: %d", c)
	}
	if dir := <-got; dir != client {
		t.Errorf("askClient = %q", dir)
	}
	if env, _ := os.ReadFile(".env"); !strings.Contains(string(env), "RO_CLIENT="+client) {
		t.Errorf(".env = %q", env)
	}

	st.ready("/build")
	if s := status(); s.Next != "/build" || s.Ask {
		t.Errorf("ready: %+v", s)
	}
	// No picker here: /startup/pick does not exist.
	w := httptest.NewRecorder()
	st.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/startup/pick", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("pick without a picker: %d", w.Code)
	}
	// The page itself.
	w = httptest.NewRecorder()
	st.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/startup", nil))
	if !strings.Contains(w.Body.String(), "/startup/status") {
		t.Error("startup page missing its script")
	}
}

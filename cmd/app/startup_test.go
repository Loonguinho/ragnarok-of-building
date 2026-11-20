package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartupFlow(t *testing.T) {
	st := newStartup(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	changes := 0
	st.onChange = func() { changes++ }

	st.say("Lendo o cliente…")
	if s := st.status(); len(s.Lines) != 1 || s.Ask || changes != 1 {
		t.Fatalf("status %+v, %d changes", s, changes)
	}

	// A folder given when none was asked for is refused.
	client := t.TempDir()
	os.WriteFile(filepath.Join(client, "data.grf"), []byte("x"), 0o644)
	if err := st.useClient(client); err == nil {
		t.Error("unasked folder taken")
	}

	// askClient waits; a folder without data.grf is refused, a client is taken.
	wd, _ := os.Getwd()
	os.Chdir(t.TempDir()) // remember() writes .env here
	defer os.Chdir(wd)
	got := make(chan string)
	go func() { got <- st.askClient("Escolha a pasta do jogo") }()
	for !st.status().Ask {
		time.Sleep(5 * time.Millisecond)
	}
	if s := st.status(); s.AskNote == "" {
		t.Error("no question shown")
	}
	if err := st.useClient(t.TempDir()); err == nil {
		t.Error("folder without data.grf taken")
	}
	if err := st.useClient(client); err != nil {
		t.Fatal(err)
	}
	if dir := <-got; dir != client {
		t.Errorf("askClient = %q", dir)
	}
	if env, _ := os.ReadFile(".env"); !strings.Contains(string(env), "RO_CLIENT="+client) {
		t.Errorf(".env = %q", env)
	}
	st.fail(os.ErrNotExist)
	if s := st.status(); s.Error == "" || s.Ask {
		t.Errorf("failed: %+v", s)
	}
}

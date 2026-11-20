package main

import (
	"errors"
	"log/slog"
	"strings"
	"sync"

	"ragnarok-of-building/internal/setup"
)

// startup is what the window shows while the program gets the game data
// ready (no console): the progress lines, a question for the client folder
// when it is not found, and the error that stopped it.
type startup struct {
	log *slog.Logger

	mu      sync.Mutex
	lines   []string
	asking  bool
	askNote string
	errMsg  string

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

type startupStatus struct {
	Lines   []string
	Ask     bool
	AskNote string
	Error   string
}

func (st *startup) status() startupStatus {
	st.mu.Lock()
	defer st.mu.Unlock()
	return startupStatus{Lines: append([]string(nil), st.lines...), Ask: st.asking, AskNote: st.askNote, Error: st.errMsg}
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

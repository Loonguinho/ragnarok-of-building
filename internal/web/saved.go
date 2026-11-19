package web

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Saved builds kept in a file (Options.SavedFile): the desktop app keeps
// them next to the executable, so they no longer depend on the browser or
// on the port the app found free. Without the option (a hosted server) the
// page keeps them in the browser's localStorage as before.

const (
	maxSavedBody    = 8 << 20
	maxSavedEntries = 2000
)

type savedStore struct {
	path string
	mu   sync.Mutex
	data []byte // the file's JSON array, "[]" when empty
}

func newSavedStore(path string) *savedStore {
	st := &savedStore{path: path, data: []byte("[]")}
	if b, err := os.ReadFile(path); err == nil {
		if list, ok := validSaved(b); ok {
			st.data = list
		}
	}
	return st
}

// validSaved keeps the entries that look like saved builds (an id and a
// code) and returns them re-encoded.
func validSaved(b []byte) ([]byte, bool) {
	var raw []map[string]any
	if json.Unmarshal(b, &raw) != nil {
		return nil, false
	}
	out := make([]map[string]any, 0, len(raw))
	for _, e := range raw {
		id, _ := e["id"].(string)
		code, _ := e["code"].(string)
		if id == "" || len(id) > 64 || code == "" || len(code) > maxBuildBody*2 {
			continue
		}
		if name, _ := e["name"].(string); len(name) > 200 {
			e["name"] = name[:200]
		}
		out = append(out, e)
		if len(out) == maxSavedEntries {
			break
		}
	}
	enc, err := json.Marshal(out) // escapes <, > and & (safe inside <script>)
	return enc, err == nil
}

func (st *savedStore) json() template.JS {
	st.mu.Lock()
	defer st.mu.Unlock()
	return template.JS(st.data)
}

func (st *savedStore) replace(list []byte) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	tmp := st.path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, list, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, st.path); err != nil {
		return err
	}
	st.data = list
	return nil
}

// savedJSON is the template function behind the page's saved-builds data:
// empty when builds are kept in the browser.
func (s *Server) savedJSON() template.JS {
	if s.saved == nil {
		return ""
	}
	return s.saved.json()
}

// PUT /saved: the whole list of saved builds (JSON array).
func (s *Server) handleSavedPut(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSavedBody))
	if err != nil {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		return
	}
	list, ok := validSaved(b)
	if !ok {
		http.Error(w, "not a list of builds", http.StatusBadRequest)
		return
	}
	if err := s.saved.replace(list); err != nil {
		s.opt.Log.Error("saved builds", "err", err)
		http.Error(w, "could not write the file", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

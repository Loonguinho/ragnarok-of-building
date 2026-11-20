package view

import (
	"encoding/json"
	"os"
)

// Saved builds live in builds.json next to the program (the same file and
// format the web version used, so builds saved there still open).

const maxSavedEntries = 2000

// SavedBuild is one saved build.
type SavedBuild struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Code  string `json:"code"`
	JobID int    `json:"j"`
	Job   string `json:"job"`
	Base  int    `json:"b"`
	JobLv int    `json:"jl"`
	At    int64  `json:"t"` // saved at, Unix ms
}

// LoadSaved reads the saved builds; a missing or broken file is an empty
// list. Entries without an id or a code are dropped.
func LoadSaved(path string) []SavedBuild {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw []SavedBuild
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	out := raw[:0]
	for _, e := range raw {
		if e.ID == "" || len(e.ID) > 64 || e.Code == "" || len(e.Code) > maxBuildBody*2 {
			continue
		}
		if len(e.Name) > 200 {
			e.Name = e.Name[:200]
		}
		out = append(out, e)
		if len(out) == maxSavedEntries {
			break
		}
	}
	return out
}

// WriteSaved replaces the saved builds (through a temporary file, so a
// crash never leaves half a file).
func WriteSaved(path string, list []SavedBuild) error {
	if list == nil {
		list = []SavedBuild{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

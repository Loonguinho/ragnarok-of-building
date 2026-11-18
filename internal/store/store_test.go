package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSavedBuilds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.SaveBuild(Build{Name: "Mestre Celestial", Code: "abc", Job: 4302, BaseLv: 250, JobLv: 50})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveBuild(Build{Name: "x"}); err == nil {
		t.Error("empty code accepted")
	}
	if _, err := s.SaveBuild(Build{ID: 999, Name: "x", Code: "y"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update of a missing build: %v", err)
	}
	if _, err := s.SaveBuild(Build{ID: id, Name: "  ", Code: "def", Job: 4302, BaseLv: 260, JobLv: 50}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Survives reopening.
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	list, _ := s.Builds()
	if len(list) != 1 || list[0].Code != "def" || list[0].Name != "Sem nome" || list[0].BaseLv != 260 {
		t.Fatalf("list after update: %+v", list)
	}
	if b, err := s.GetBuild(id); err != nil || b.Job != 4302 {
		t.Errorf("get: %+v %v", b, err)
	}
	if err := s.DeleteBuild(id); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Builds(); len(list) != 0 {
		t.Errorf("deleted build listed: %+v", list)
	}
	if err := s.DeleteBuild(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
}

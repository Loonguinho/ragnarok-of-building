package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAccountsAndSessions(t *testing.T) {
	s := open(t)
	if _, err := s.Register("ab", "123456"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("short name: %v", err)
	}
	if _, err := s.Register("lucas_ro", "123"); !errors.Is(err, ErrWeakPass) {
		t.Errorf("short password: %v", err)
	}
	u, err := s.Register("lucas_ro", "segredo1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register("LUCAS_RO", "outra123"); !errors.Is(err, ErrTaken) {
		t.Errorf("same name, other case: %v", err)
	}
	if _, err := s.Login("lucas_ro", "errada"); !errors.Is(err, ErrBadLogin) {
		t.Errorf("wrong password: %v", err)
	}
	if _, err := s.Login("ninguem", "segredo1"); !errors.Is(err, ErrBadLogin) {
		t.Errorf("unknown user: %v", err)
	}
	if got, err := s.Login("Lucas_RO", "segredo1"); err != nil || got.ID != u.ID {
		t.Fatalf("login: %v %v", got, err)
	}
	tok, err := s.NewSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s.UserBySession(tok); !ok || got.Name != "lucas_ro" {
		t.Errorf("session: %v %v", got, ok)
	}
	if _, ok := s.UserBySession("forjado"); ok {
		t.Error("forged token accepted")
	}
	s.EndSession(tok)
	if _, ok := s.UserBySession(tok); ok {
		t.Error("session alive after logout")
	}
}

func TestBuildsBelongToTheirOwner(t *testing.T) {
	s := open(t)
	a, _ := s.Register("jogador_a", "segredo1")
	b, _ := s.Register("jogador_b", "segredo1")
	id, err := s.SaveBuild(a.ID, Build{Name: "Mestre Celestial", Code: "abc", Job: 4302, BaseLv: 250, JobLv: 50})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveBuild(b.ID, Build{ID: id, Name: "roubo", Code: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("other user updated the build: %v", err)
	}
	if _, err := s.GetBuild(b.ID, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("other user read the build: %v", err)
	}
	if err := s.DeleteBuild(b.ID, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("other user deleted the build: %v", err)
	}
	if _, err := s.SaveBuild(a.ID, Build{ID: id, Name: "  ", Code: "def", Job: 4302, BaseLv: 260, JobLv: 50}); err != nil {
		t.Fatal(err)
	}
	list, _ := s.Builds(a.ID)
	if len(list) != 1 || list[0].Code != "def" || list[0].Name != "Sem nome" || list[0].BaseLv != 260 {
		t.Errorf("list after update: %+v", list)
	}
	if err := s.DeleteBuild(a.ID, id); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Builds(a.ID); len(list) != 0 {
		t.Errorf("deleted build listed: %+v", list)
	}
}

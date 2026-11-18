package grf

import (
	"os"
	"path/filepath"
	"ragnarok-of-building/internal/config"
	"testing"

	"ragnarok-of-building/internal/lua51"
)

func TestNotAGRF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.grf")
	os.WriteFile(p, make([]byte, 100), 0o644)
	if _, err := Open(p); err == nil {
		t.Fatal("expected error")
	}
}

// Runs against the real LATAM client when installed (skipped in CI).
func TestRealClientSkillTree(t *testing.T) {
	a, err := Open(filepath.Join(config.ClientDir(), "data.grf"))
	if err != nil {
		t.Skip("RO LATAM client not installed")
	}
	defer a.Close()
	if a.Len() < 100000 {
		t.Errorf("only %d files", a.Len())
	}
	const dir = `data\luafiles514\lua files\`
	env := map[string]lua51.Value{}
	for _, f := range []string{
		`datainfo\jobidentity.lub`, `skillinfoz\skillid.lub`,
		`skillinfoz\jobinheritlist.lub`, `skillinfoz\skillinfolist.lub`,
		`skillinfoz\skilltreeview.lub`,
	} {
		b, err := a.Read(dir + f)
		if err != nil {
			t.Fatal(err)
		}
		lua51.LoadEnv(b, env) // partial loads are expected; checked below
	}
	info, _ := env["SKILL_INFO_LIST"].(*lua51.Table)
	tree, _ := env["SKILL_TREEVIEW_FOR_JOB"].(*lua51.Table)
	if info == nil || tree == nil {
		t.Fatal("skill tables missing")
	}
	bb := info.Get(62.0).(*lua51.Table) // Bowling Bash
	if n := len(bb.Get("_NeedSkillList").(*lua51.Table).Array()); n != 5 {
		t.Errorf("Bowling Bash prerequisites = %d, want 5", n)
	}
	if novice := tree.Get(0.0).(*lua51.Table); novice.Get(0.0) != 1.0 {
		t.Errorf("Novice tree slot 0 = %v, want Basic Skill (1)", novice.Get(0.0))
	}
}

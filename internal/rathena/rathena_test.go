package rathena

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJobEnum(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mmo.hpp")
	os.WriteFile(p, []byte(`
enum e_job {
	JOB_NOVICE,
	JOB_SWORDMAN,
	JOB_MAX_BASIC,

	JOB_NOVICE_HIGH = 4001,
	JOB_SWORDMAN_HIGH,
	JOB_DRAGON_KNIGHT = 4252,
};
enum other { JOB_FAKE = 9 };
`), 0o644)
	ids, err := parseJobEnum(p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"NOVICE": 0, "SWORDMAN": 1, "MAX_BASIC": 2, "NOVICE_HIGH": 4001, "SWORDMAN_HIGH": 4002, "DRAGON_KNIGHT": 4252}
	for k, v := range want {
		if ids[k] != v {
			t.Errorf("%s = %d, want %d", k, ids[k], v)
		}
	}
	if _, ok := ids["FAKE"]; ok {
		t.Error("parsed past the enum")
	}
}

// Runs against downloaded data when present (skipped in CI).
func TestLoadReal(t *testing.T) {
	db, err := Load(filepath.Join("..", "..", "data", "rathena"))
	if err != nil {
		t.Skip("rAthena data not downloaded:", err)
	}
	dk := db.Jobs[4252]
	if dk == nil {
		t.Fatal("Dragon Knight missing")
	}
	if dk.MaxBaseLv < 200 || dk.MaxJobLv < 50 || len(dk.BonusStats) == 0 {
		t.Errorf("Dragon Knight tables incomplete: base %d job %d bonus %d",
			dk.MaxBaseLv, dk.MaxJobLv, len(dk.BonusStats))
	}
	// 4th jobs have no HP table; the formula must give a plausible curve.
	if hp200, hp275 := dk.BaseHPAt(200), dk.BaseHPAt(275); hp200 < 10000 || hp275 <= hp200 {
		t.Errorf("Dragon Knight base HP: 200=%d 275=%d", hp200, hp275)
	}
	t.Logf("Dragon Knight base HP 200=%d 275=%d, SP 200=%d 275=%d",
		dk.BaseHPAt(200), dk.BaseHPAt(275), dk.BaseSPAt(200), dk.BaseSPAt(275))
	if db.TraitPoints[274] == 0 {
		t.Error("no trait points at 275")
	}
	if len(db.StatPoints) < 99 || db.StatPoints[0] != 48 {
		t.Errorf("stat points: len %d first %v", len(db.StatPoints), db.StatPoints[:1])
	}
	if it := db.Items[1201]; it == nil || it.Type != "Weapon" || it.SubType != "Dagger" || it.Attack != 17 {
		t.Errorf("Knife = %+v", it)
	}
	if c := db.Items[4001]; c == nil || c.Type != "Card" {
		t.Errorf("Poring Card = %+v", c)
	}
}

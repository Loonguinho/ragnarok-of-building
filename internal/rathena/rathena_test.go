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
	dir := filepath.Join("..", "..", "data", "rathena")
	if _, err := os.Stat(filepath.Join(dir, "COMMIT")); err != nil {
		t.Skip("rAthena data not downloaded (go run ./cmd/fetchrathena)")
	}
	db, err := Load(dir) // downloaded data must parse: fail, don't skip
	if err != nil {
		t.Fatal(err)
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
	if b := db.Mobs[1039]; b == nil || b.Race != "Demon" || b.Element != "Dark" || b.ElementLv != 3 || !b.MVP || b.Class != "Boss" {
		t.Errorf("Baphomet = %+v", b)
	}
	if p := db.Mobs[1002]; p == nil || p.Stats[1] != 1 { // Poring omits AGI: default 1
		t.Errorf("Poring = %+v", p)
	}
	if v := db.AttrFix[1]["Water"]["Fire"]; v != 150 {
		t.Errorf("water vs fire lv1 = %d, want 150", v)
	}
	// Current renewal removed most size penalties: only knuckles/whips vs Large.
	if v := db.SizeFix["Knuckle"]["Large"]; v != 75 {
		t.Errorf("knuckle vs large = %d, want 75", v)
	}
	if _, ok := db.SizeFix["Dagger"]; ok {
		t.Error("dagger should have no size penalty entry")
	}
	if c := db.Items[4001]; c == nil || c.Type != "Card" {
		t.Errorf("Poring Card = %+v", c)
	}
}

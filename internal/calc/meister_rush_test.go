package calc

import (
	"path/filepath"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestRushQuake: Avanço goes by level (+5% melee and ranged per level) and
// Sismo on the target gives +50%. Together, at Avanço 9, they raise Laser
// Triplo by 1,5 × (100 + L + 45) / (100 + L), as the Engenheiro replay of
// 2026-11-27 measured (47.813.520 → 82.849.791 with L = 190).
func TestRushQuake(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("catalog not generated")
	}
	var rush *ClassBuff
	for _, cb := range ClassBuffsFor(c, 4253) { // Engenheiro
		if cb.Key == stateKey(meisterRush) {
			rush = &cb
		}
	}
	if rush == nil || len(rush.Choices) != 11 || rush.Scripts[9] != "bonus bLongAtkRate,45; bonus bShortAtkRate,45;" {
		t.Fatalf("Avanço should go by level 1-10, +5%% each: %+v", rush)
	}
	quake := false
	for _, s := range StatesFor(c, 4253) {
		quake = quake || s.Key == stateKey(quakeState)
	}
	if !quake {
		t.Fatal("the Meister has no Sismo no alvo")
	}

	b := Build{Job: 4253, BaseLv: 240, JobLv: 50, Stats: [6]int{100, 100, 100, 1, 100, 1},
		Equip:  map[string]Equip{Weapon: {Item: 1301, Refine: 7}},
		Target: 21077, // Dummy (Neutral Lv1)
	}
	run := func(extra map[string]int) (DamageResult, Result) {
		nb := b
		nb.Atk.States = map[string]int{"s_maximizepower": 1}
		for k, v := range extra {
			nb.Atk.States[k] = v
		}
		Normalize(&nb, c)
		r := Compute(nb, c)
		return Damage(nb, r, c, c.Monster(nb.Target), Attack{}), r
	}
	base, r0 := run(nil)
	_, r9 := run(map[string]int{stateKey(meisterRush): 9})
	for _, k := range []string{"bLongAtkRate", "bShortAtkRate"} {
		if d := r9.Bonuses[k] - r0.Bonuses[k]; d != 45 {
			t.Errorf("Avanço 9: %s %+d, want +45", k, d)
		}
	}
	quaked, _ := run(map[string]int{stateKey(quakeState): 1})
	if quaked.Max != base.Max+base.Max*quakeTakenPct/100 {
		t.Errorf("Sismo: %d → %d, want +50%%", base.Max, quaked.Max)
	}
}

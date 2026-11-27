package calc

import (
	"path/filepath"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestSmithBuffs: a Meister has Força Violenta, Manejo Perfeito and
// Amplificar Poder; Força Violentíssima wins over Força Violenta; Amplificar
// Poder makes every hit the weapon's maximum.
func TestSmithBuffs(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("catalog not generated")
	}
	have := map[string]bool{}
	for _, cb := range ClassBuffsFor(c, 4253) { // Engenheiro
		have[cb.Key] = true
	}
	for _, k := range []string{"s_powerthrust", "s_weaponperfection", "s_maximizepower", "s_maximumpowerthrust"} {
		if !have[k] {
			t.Errorf("the Meister has no %s", k)
		}
	}

	b := Build{Job: 4253, BaseLv: 200, JobLv: 50, Stats: [6]int{100, 100, 100, 1, 100, 1},
		Equip:  map[string]Equip{Weapon: {Item: 1301, Refine: 7}}, // Machado
		Target: 1002,                                              // Poring
	}
	b.Atk.States = map[string]int{"s_powerthrust": 5, "s_maximumpowerthrust": 5}
	Normalize(&b, c)
	if b.Atk.States["s_powerthrust"] != 0 || b.Atk.States["s_maximumpowerthrust"] != 5 {
		t.Fatalf("states %v: Força Violentíssima should end Força Violenta", b.Atk.States)
	}

	dmg := func(states map[string]int) DamageResult {
		nb := b
		nb.Atk.States = states
		Normalize(&nb, c)
		return Damage(nb, Compute(nb, c), c, c.Monster(nb.Target), Attack{})
	}
	plain := dmg(map[string]int{})
	if plain.Min == plain.Max {
		t.Fatalf("no weapon variance to test with: %d", plain.Min)
	}
	if mx := dmg(map[string]int{"s_maximizepower": 1}); mx.Min != mx.Max || mx.Max != plain.Max {
		t.Errorf("Amplificar Poder: %d~%d, want %d~%d", mx.Min, mx.Max, plain.Max, plain.Max)
	}
	if pt := dmg(map[string]int{"s_powerthrust": 5}); pt.Max <= plain.Max {
		t.Errorf("Força Violenta 5: max %d, not above %d", pt.Max, plain.Max)
	}
}

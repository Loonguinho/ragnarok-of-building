package calc

import "testing"

// Ilimitar adds +50%/Nv to the ranged damage % (×3,5 at Nv5 alone), only
// on ranged physical attacks.
func TestUnlimit(t *testing.T) {
	on := Build{Atk: AttackChoice{States: map[string]int{stateKey(unlimitState): 5}}}
	avg := func(b Build, ranged bool, long int) int64 {
		r := baseResult()
		r.Ranged = ranged
		r.Bonuses["bLongAtkRate"] = long
		return Damage(b, r, dmgCatalog(), target(0, 0, 0), Attack{}).Avg
	}
	if a, b := avg(Build{}, false, 0), avg(on, false, 0); a != b {
		t.Errorf("melee: %d with Ilimitar, want %d (no change)", b, a)
	}
	base := avg(Build{}, true, 0)
	if got, want := avg(on, true, 0), base*350/100; got != want {
		t.Errorf("ranged: %d, want %d (×3,5)", got, want)
	}
	// With +20% ranged damage already: × (100+20+250)/100, not × 1,2 × 3,5.
	if got, want := avg(on, true, 20), base*370/100; got != want {
		t.Errorf("ranged +20%%: %d, want %d", got, want)
	}
}

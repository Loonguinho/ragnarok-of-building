package calc

import (
	"path/filepath"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestSourcesAddUp: the kept sources of each cast and delay bonus add up
// to the total the calculation uses.
func TestSourcesAddUp(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("catalog not generated")
	}
	b := Build{Job: 4253, BaseLv: 240, JobLv: 50, Stats: [6]int{50, 100, 100, 51, 125, 125},
		Equip: map[string]Equip{
			Garment: {Item: 480055, Refine: 11, Cards: [4]int{0, 311125}}, // Manto Invernal + Flor Invernal (Variável)
			HeadTop: {Item: 400330, Refine: 12, Cards: [4]int{4648}},      // Coroa Ambivalente Mecânica + Carta Kathryne
			CosTop:  {Item: 31188, Cards: [4]int{0, 0, 0, 4881}},
			Shoes:   {Item: 470071, Refine: 11},
			Weapon:  {Item: 1301, Refine: 7},
		},
		Pet: 9139, PetIntimacy: 1000,
		Buffs: map[string]int{"blessing": 10},
	}
	Normalize(&b, c)
	r := Compute(b, c)
	sum := map[string]int{}
	for _, s := range r.Sources {
		sum[s.Key] += s.Value
	}
	for _, k := range sourceKeys {
		if sum[k] != r.Bonuses[k] {
			t.Errorf("%s: sources add to %d, total %d", k, sum[k], r.Bonuses[k])
		}
	}
	if len(r.Sources) == 0 {
		t.Error("no sources kept")
	}
}

package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestPetBonus: the pet's script counts at the build's intimacy tier, and
// an unknown pet is dropped.
func TestPetBonus(t *testing.T) {
	c := novice()
	c.Items[9001] = &gamedata.Item{ID: 9001, Type: "PetEgg"}
	c.Pets = map[int]*gamedata.Pet{9001: {Mob: 1002, Egg: 9001, Script: `.@i = getpetinfo(PETINFO_INTIMATE);
if (.@i >= PET_INTIMATE_LOYAL) { bonus bCritical,1; bonus bLuk,3; }
else if (.@i >= PET_INTIMATE_CORDIAL) { bonus bCritical,1; bonus bLuk,2; }`}}
	luk := func(intimacy int) int {
		b := Build{Job: 0, BaseLv: 1, JobLv: 1, Stats: [6]int{1, 1, 1, 1, 1, 1}, Pet: 9001, PetIntimacy: intimacy}
		return Compute(b, c).Bonuses["bLuk"]
	}
	for in, want := range map[int]int{1000: 3, 900: 3, 850: 2, 671: 0} {
		if got := luk(in); got != want {
			t.Errorf("intimacy %d: LUK +%d, want +%d", in, got, want)
		}
	}
	b := Build{Job: 0, BaseLv: 1, JobLv: 1, Pet: 1234, PetIntimacy: 1000}
	Normalize(&b, c)
	if b.Pet != 0 || b.PetIntimacy != 0 {
		t.Errorf("unknown pet kept: %+v", b)
	}
}

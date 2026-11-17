package gamedata

import "testing"

func TestCanEquip(t *testing.T) {
	knight := &Job{EquipGroup: "Knight", EquipTier: "Normal"}
	rk := &Job{EquipGroup: "Knight", EquipTier: "Third"}
	dk := &Job{EquipGroup: "Knight", EquipTier: "Fourth"}
	bard := &Job{EquipGroup: "BardDancer", EquipTier: "Normal"}
	unknown := &Job{}

	sword := &Item{EquipJobs: []string{"Swordman", "Knight"}}
	violin := &Item{EquipJobs: []string{"BardDancer"}}
	thirdOnly := &Item{EquipJobs: []string{"All"}, EquipClasses: []string{"All_Third"}}
	thirdAndFourth := &Item{EquipJobs: []string{"Knight"}, EquipClasses: []string{"All_Third", "Fourth"}}
	upperOnly := &Item{EquipJobs: []string{"All"}, EquipClasses: []string{"All_Upper"}}
	anyone := &Item{}

	cases := []struct {
		name string
		j    *Job
		it   *Item
		want bool
	}{
		{"knight sword", knight, sword, true},
		{"dk sword (group via chain)", dk, sword, true},
		{"dk violin", dk, violin, false},
		{"bard violin", bard, violin, true},
		{"rk third-only", rk, thirdOnly, true},
		{"dk third-only (renewal excludes 4th)", dk, thirdOnly, false},
		{"dk third+fourth", dk, thirdAndFourth, true},
		{"knight third-only", knight, thirdOnly, false},
		{"dk all-upper", dk, upperOnly, true},
		{"knight all-upper", knight, upperOnly, false},
		{"anyone", knight, anyone, true},
		{"no restriction data", unknown, violin, true},
	}
	for _, c := range cases {
		if got := c.j.CanEquip(c.it); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

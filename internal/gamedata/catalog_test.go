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

func TestSkillBudgets(t *testing.T) {
	c := &Catalog{Jobs: map[int]*Job{
		0:    {ID: 0, Code: "JT_NOVICE", Parent: -1, MaxJobLv: 10},
		1:    {ID: 1, Code: "JT_SWORDMAN", Parent: 0, MaxJobLv: 50},
		7:    {ID: 7, Code: "JT_KNIGHT", Parent: 1, MaxJobLv: 50},
		4008: {ID: 4008, Code: "JT_KNIGHT_H", Parent: 7, MaxJobLv: 70},
		4060: {ID: 4060, Code: "JT_RUNE_KNIGHT_H", Parent: 4008, MaxJobLv: 70},
		4252: {ID: 4252, Code: "JT_DRAGON_KNIGHT", Parent: 4060, MaxJobLv: 60},
	}}
	got := c.SkillBudgets(4252, 45)
	want := []SkillGroup{
		{Jobs: []int{0}, Budget: 9},
		{Jobs: []int{1}, Budget: 49},
		{Jobs: []int{7, 4008}, Budget: 69}, // Knight + Lord Knight share one step
		{Jobs: []int{4060}, Budget: 69},
		{Jobs: []int{4252}, Budget: 44}, // current job level 45
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Budget != want[i].Budget || len(got[i].Jobs) != len(want[i].Jobs) {
			t.Errorf("group %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if g := c.SkillBudgets(7, 0); g[len(g)-1].Budget != 49 {
		t.Errorf("job level 0 should mean max: %+v", g)
	}
}

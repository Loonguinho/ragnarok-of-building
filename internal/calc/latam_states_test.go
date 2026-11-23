package calc

import (
	"os"
	"strings"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestClassStatesOffered: the hand-added buffs reach the 4th classes and
// give their bonuses, and no option is a leftover of generated code.
func TestClassStatesOffered(t *testing.T) {
	const path = "../../data/gen/catalog.json.gz"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no generated catalog")
	}
	c, err := gamedata.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	job := func(code string) int {
		for id, j := range c.Jobs {
			if j.Code == code {
				return id
			}
		}
		t.Fatalf("no job %s", code)
		return 0
	}
	has := func(jobID int, name string) bool {
		for _, o := range StatesFor(c, jobID) {
			if o.Key == stateKey(name) {
				return true
			}
		}
		return false
	}
	for _, j := range c.Jobs {
		for _, o := range StatesFor(c, j.ID) {
			if strings.Contains(o.Label, "ternS") || o.Label == "v angel" {
				t.Errorf("%s offers %q", j.Code, o.Label)
			}
		}
	}
	if !has(job("JT_BIOLO"), "Research Report") || !has(job("JT_HYPER_NOVICE"), "Angel of Power") {
		t.Error("Eureka or Angel of Power not offered")
	}
	for _, tc := range []struct {
		code, name string
		v          int
		get        func(Result) int
		want       int
	}{
		{"JT_BIOLO", "Pyroclastic", 230, func(r Result) int { return r.EquipATK }, 430},
		{"JT_IMPERIAL_GUARD", "Shield Spell", 3, func(r Result) int { return r.EquipATK }, 150},
		{"JT_SPIRIT_HANDLER", "Temporary Communion", 5, func(r Result) int { return r.PATK }, 15},
		{"JT_SPIRIT_HANDLER", "Temporary Communion", 5, func(r Result) int { return r.HPLUS }, 15},
		{"JT_TROUBADOUR", "Mystic Symphony", 1, func(r Result) int { return r.Extra["bAddRace RC_Fish"] }, 50},
	} {
		b := Build{Job: job(tc.code), BaseLv: 250, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1}}
		off := tc.get(Compute(b, c))
		b.Atk.States = map[string]int{stateKey(tc.name): tc.v}
		Normalize(&b, c)
		if got := tc.get(Compute(b, c)) - off; got != tc.want {
			t.Errorf("%s %s: +%d, want +%d", tc.code, tc.name, got, tc.want)
		}
	}
}

package calc

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

// buffBonusNames are the bonuses the buffs may give: the ones Compute and
// the damage engine read.
var buffBonusNames = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`bStr bAgi bVit bInt bDex bLuk bAllStats bPow bSta bWis bSpl bCon bCrt
		bHit bFlee bCritical bCRate bCritAtkRate bAspd bAspdRate bAtk bBaseAtk bMatk bPAtk bSMatk bDef bMdef
		bRes bMRes bMaxHPrate bMaxSPrate bAtkRate bMatkRate bShortAtkRate bLongAtkRate
		bIgnoreDefRaceRate bIgnoreMdefRaceRate bIgnoreResRaceRate bIgnoreMresRaceRate bAddRace bAddEle bMagicAddEle bMagicAtkEle`) {
		buffBonusNames[n] = true
	}
}

func TestBuffsChangeStats(t *testing.T) {
	c := &gamedata.Catalog{Jobs: map[int]*gamedata.Job{0: {ID: 0, MaxBaseLv: 99, MaxJobLv: 10, Parent: -1,
		BaseHP: make([]int, 99), BaseSP: make([]int, 99), JobBonus: make([][12]int, 11), BaseASPD: map[string]int{"Fist": 40}}}}
	b := Build{Job: 0, BaseLv: 50, JobLv: 1, Stats: [6]int{50, 50, 50, 50, 50, 50}}
	plain := Compute(b, c)
	b.Buffs = map[string]int{"blessing": 10, "incagi": 10, "gloria": 1, "concentration": 10, "food_str": 20, "nope": 3}
	Normalize(&b, c)
	if _, ok := b.Buffs["nope"]; ok {
		t.Error("unknown buff kept")
	}
	r := Compute(b, c)
	diff := func(i int) int { return r.FinalStats()[i] - plain.FinalStats()[i] }
	// STR: Bênção 10 + comida 20; AGI: Aumentar Agilidade 12 + Concentrar 12% of 50 = 6;
	// DEX: Bênção 10 + 6; LUK: Glória 30.
	if diff(STR) != 30 || diff(AGI) != 18 || diff(DEX) != 16 || diff(LUK) != 30 || diff(INT) != 10 {
		t.Errorf("stat changes STR %d AGI %d DEX %d INT %d LUK %d", diff(STR), diff(AGI), diff(DEX), diff(INT), diff(LUK))
	}
	if r.HIT <= plain.HIT+20 {
		t.Errorf("HIT %d -> %d: Bênção gives +20 plus DEX/LUK", plain.HIT, r.HIT)
	}
}

// Every value of every buff gives at least one bonus, and nothing the
// script reader does not understand.
func TestEveryBuffValueGivesBonuses(t *testing.T) {
	base := [6]int{100, 100, 100, 100, 100, 100}
	for _, d := range Buffs {
		var vals []int
		switch {
		case d.Choices != nil:
			for _, c := range d.Choices[1:] {
				var v int
				fmt.Sscan(c[0], &v)
				vals = append(vals, v)
			}
		case d.Max > 0:
			vals = []int{1, d.Max}
		default:
			vals = []int{1}
		}
		for _, v := range vals {
			src := d.Script(v, base)
			res := script.Eval(src, script.Context{BaseLv: 200, JobLv: 50})
			if len(res.Bonus)+len(res.Extra) == 0 {
				t.Errorf("%s=%d: no bonus from %q", d.Key, v, src)
			}
			if res.Skipped > 0 {
				t.Errorf("%s=%d: %d statement(s) not understood in %q", d.Key, v, res.Skipped, src)
			}
			for k := range res.Bonus {
				if !buffBonusNames[k] {
					t.Errorf("%s=%d: %s is not read by the calculator", d.Key, v, k)
				}
			}
			for k := range res.Extra {
				if !buffBonusNames[strings.Fields(k)[0]] {
					t.Errorf("%s=%d: %s is not read by the calculator", d.Key, v, k)
				}
			}
		}
	}
}

func TestExclusiveBuffs(t *testing.T) {
	b := Build{Buffs: map[string]int{"whistle": 10, "assassincross": 10, "soul_fairy": 1, "soul_falcon": 1, "humming": 5, "moonlit": 99, "interlude": 7}}
	normalizeBuffs(&b)
	for k, want := range map[string]int{"whistle": 10, "assassincross": 0, "soul_fairy": 1, "soul_falcon": 0, "humming": 5, "moonlit": 17, "interlude": 7} {
		if b.Buffs[k] != want {
			t.Errorf("%s = %d, want %d (%v)", k, b.Buffs[k], want, b.Buffs)
		}
	}
}

// The skill codes name client skills (their names label the buffs).
func TestBuffSkillsExist(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("catalog not generated")
	}
	codes := map[string]bool{}
	for _, s := range c.Skills {
		codes[s.Code] = true
	}
	for _, d := range Buffs {
		if d.Skill != "" && !codes[d.Skill] {
			t.Errorf("%s: skill %s not in the client", d.Key, d.Skill)
		}
	}
}

// Every generated class buff gives bonuses the calculator reads.
func TestClassBuffScripts(t *testing.T) {
	names := map[string]bool{"bAddSize": true, "bIgnoreDefClassRate": true, "bMagicAddRace": true, "bSkillAtk": true}
	for k := range buffBonusNames {
		names[k] = true
	}
	n := 0
	for job, bs := range latamClassBuffs {
		for _, b := range bs {
			for v, src := range b.Scripts {
				n++
				res := script.Eval(src, script.Context{BaseLv: 200, JobLv: 50})
				if res.Skipped > 0 || len(res.Bonus)+len(res.Extra) == 0 {
					t.Errorf("%s %s=%d: %q gives %v %v (skipped %d)", job, b.Name, v, src, res.Bonus, res.Extra, res.Skipped)
				}
				for k := range res.Bonus {
					if !names[k] {
						t.Errorf("%s %s: %s is not read by the calculator", job, b.Name, k)
					}
				}
				for k := range res.Extra {
					if !names[strings.Fields(k)[0]] {
						t.Errorf("%s %s: %s is not read by the calculator", job, b.Name, k)
					}
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no class buffs generated")
	}
}

// A class buff turned on in the attack states changes the status window.
func TestClassBuffApplies(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("catalog not generated")
	}
	var job int
	for id, j := range c.Jobs {
		if j.Code == "JT_BIOLO" {
			job = id
		}
	}
	b := Build{Job: job, BaseLv: 250, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1}}
	Normalize(&b, c)
	plain := Compute(b, c)
	b.Atk.States = map[string]int{stateKey("Tempering"): 10}
	Normalize(&b, c)
	if b.Atk.States[stateKey("Tempering")] != 10 {
		t.Fatalf("Tempering dropped by Normalize: %v", b.Atk.States)
	}
	if r := Compute(b, c); r.Bonuses["bPAtk"]-plain.Bonuses["bPAtk"] != 15 {
		t.Errorf("Tempering 10: P.ATK bonus %d -> %d, want +15", plain.Bonuses["bPAtk"], r.Bonuses["bPAtk"])
	}
}

// Every generated passive effect gives bonuses the calculator reads.
func TestClassPassiveScripts(t *testing.T) {
	names := map[string]bool{"bAddSize": true, "bIgnoreDefClassRate": true, "bMagicAddRace": true, "bSkillAtk": true,
		"bMasteryAtk": true, "bMasteryAtkRace": true, "bMasteryAtkEle": true, "bMaxHP": true, "bMaxSP": true}
	for k := range buffBonusNames {
		names[k] = true
	}
	n := 0
	for job, ps := range latamClassPassives {
		for _, p := range ps {
			for lv, effs := range p.Levels {
				for _, e := range effs {
					n++
					res := script.Eval(e[1], script.Context{})
					if res.Skipped > 0 || len(res.Bonus)+len(res.Extra) == 0 {
						t.Errorf("%s %s Nv%d: %q gives nothing", job, p.Name, lv, e[1])
					}
					for k := range res.Bonus {
						if !names[k] {
							t.Errorf("%s %s: %s is not read by the calculator", job, p.Name, k)
						}
					}
					for k := range res.Extra {
						if !names[strings.Fields(k)[0]] {
							t.Errorf("%s %s: %s is not read by the calculator", job, p.Name, k)
						}
					}
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no passives generated")
	}
}

// Perícia com Espada 10 learned: +40 mastery ATK with a sword, none with a spear.
func TestSwordMasteryPassive(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("catalog not generated")
	}
	var job, sm int
	for id, j := range c.Jobs {
		if j.Code == "JT_KNIGHT" {
			job = id
		}
	}
	for id, s := range c.Skills {
		if s.Code == "SM_SWORD" {
			sm = id
		}
	}
	b := Build{Job: job, BaseLv: 99, JobLv: 50, Skills: map[int]int{sm: 10}}
	bonus, extra := map[string]int{}, map[string]int{}
	applyClassPassives(&b, c, "1hSword", bonus, extra)
	if bonus["bMasteryAtk"] != 40 {
		t.Errorf("sword: mastery %d, want 40 (%v)", bonus["bMasteryAtk"], bonus)
	}
	bonus = map[string]int{}
	applyClassPassives(&b, c, "1hSpear", bonus, extra)
	if bonus["bMasteryAtk"] != 0 {
		t.Errorf("spear: mastery %d, want 0", bonus["bMasteryAtk"])
	}
}

package calc

import (
	"math"
	"slices"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// LATAM skill formulas.
//
// The ratios below are the LATAM (bRO-based) formulas measured against real
// LATAM recordings by latam-ro-calc (Adson Leal, MIT License,
// https://github.com/adsonpleal/latam-ro-calc). They are reimplemented here
// in Go; the comments there document the recordings each one was checked on.
// Skills without an entry fall back to the client description's per-level %.

// FormulaInput is what a skill formula can read.
type FormulaInput struct {
	Lv       int // skill level used
	BaseLv   int
	Stats    [6]int // final STR..LUK
	Traits   [6]int // final POW..CRT
	Learned  map[int]int
	Catalog  *gamedata.Catalog
	Monster  *gamedata.Monster
	Weapon   string // weapon type (rAthena SubType), "Fist" when none
	WeaponLv int
	WeaponWt int // weapon weight (rAthena, tenths)
	MaxHP    int
	MaxSP    int
	States   map[string]int
	Stack    int // stacks for stack-based skills (Eternal Slash)
}

// LearnLv returns the learned level of a skill by client code.
func (in FormulaInput) LearnLv(code string) int {
	if in.Catalog == nil {
		return 0
	}
	for id, lv := range in.Learned {
		if sk := in.Catalog.Skills[id]; sk != nil && sk.Code == code {
			return lv
		}
	}
	return 0
}

func (in FormulaInput) state(key string) int { return in.States[key] }

func (in FormulaInput) race(rs ...string) bool {
	return in.Monster != nil && slices.ContainsFunc(rs, func(r string) bool { return strings.EqualFold(r, in.Monster.Race) })
}

// lvScale is the 4th-job "× (BaseLv / 100)" factor.
func (in FormulaInput) lvScale() float64 { return float64(in.BaseLv) / 100 }

// StateOption is a toggle/selector the damage window shows for a class.
type StateOption struct {
	Key     string
	Label   string
	Choices [][2]string // value, label; empty = on/off checkbox
}

// SkillFormula is a LATAM damage formula for one skill.
type SkillFormula struct {
	Code          string
	Ratio         func(FormulaInput) float64 // % of ATK, total for one use
	Hits          func(FormulaInput) int     // real damage multiplier (totalHit); nil = 1
	CanCrit       func(FormulaInput) bool    // nil = no crits
	CritRate      float64                    // fraction of the normal crit chance (baseCriPercentage)
	CritDmg       float64                    // fraction of the crit damage bonus (criDmgPercentage)
	ForceCrit     bool
	IgnoreDef     bool
	IgnoreSoftDef bool
	Melee         func(weapon string) bool // nil = by weapon (ranged weapons are long range)
	Weapons       []string                 // required weapon types (rAthena SubType); empty = any
	Stack         [2]int                   // min,max stacks when the skill uses them
}

func always(FormulaInput) bool { return true }
func melee(string) bool        { return true }

// celestial spaces (Sky Emperor): a space matches itself or Celestial Unity.
const (
	csSunrise = iota + 1
	csNoon
	csSunset
	csMoonrise
	csMidnight
	csMoonset
	csUnity
)

func inSpace(in FormulaInput, space int) bool {
	s := in.state("ske_space")
	return s == space || s == csUnity
}

// LatamStates lists the selectors each class shows in the damage window.
var LatamStates = map[string][]StateOption{
	"JT_SKY_EMPEROR": {{Key: "ske_space", Label: "Espaço Celeste", Choices: [][2]string{
		{"0", "—"}, {"1", "Nascer do Sol"}, {"2", "Meio-Dia"}, {"3", "Pôr do Sol"},
		{"4", "Nascer da Lua"}, {"5", "Meia-Noite"}, {"6", "Pôr da Lua"}, {"7", "Elo Celestial"}}}},
	"JT_SHADOW_CROSS": {
		{Key: "sc_shadow_exceed", Label: "Excedente Sombrio"},
		{Key: "sc_cloaking_exceed", Label: "Furtividade Excedente"},
		{Key: "sc_spin", Label: "Giros (Cratera de Impacto)", Choices: [][2]string{{"1", "1"}, {"2", "2"}, {"3", "3"}, {"4", "4"}, {"5", "5"}}},
	},
}

// LatamFormulas, keyed by client skill code.
var LatamFormulas = map[string]*SkillFormula{}

func init() {
	add := func(f *SkillFormula) {
		if f.CritRate == 0 {
			f.CritRate = 1
		}
		if f.CritDmg == 0 {
			f.CritDmg = 1
		}
		LatamFormulas[f.Code] = f
	}
	sky := func(in FormulaInput) float64 { return float64(in.LearnLv("SKE_SKY_MASTERY")) }
	pow := func(in FormulaInput) float64 { return float64(in.Traits[POW]) }
	lv := func(in FormulaInput) float64 { return float64(in.Lv) }

	// ---- Mestre Celestial (Sky Emperor) ----
	add(&SkillFormula{Code: "SKE_RISING_SUN", Melee: melee, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 {
			return (500 + lv(in)*(400+sky(in)*5) + pow(in)*3) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_RISING_MOON", Melee: melee, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 {
			return (600 + lv(in)*(300+sky(in)*5) + pow(in)*3) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_NOON_BLAST", Melee: melee, CritDmg: 0.5,
		CanCrit: func(in FormulaInput) bool { return inSpace(in, csNoon) },
		Ratio: func(in FormulaInput) float64 {
			return (1500 + lv(in)*(900+sky(in)*5) + pow(in)*5) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_SUNSET_BLAST", Melee: melee, CritDmg: 0.5,
		CanCrit: func(in FormulaInput) bool { return inSpace(in, csSunset) },
		Ratio: func(in FormulaInput) float64 {
			return (900 + lv(in)*(300+sky(in)*5) + pow(in)*5) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_MIDNIGHT_KICK", Melee: melee, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 {
			if inSpace(in, csMidnight) {
				return (1500 + lv(in)*(1200+sky(in)*5) + pow(in)*5) * in.lvScale()
			}
			return (500 + lv(in)*(1000+sky(in)*5) + pow(in)*5) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_DAWN_BREAK", Melee: melee, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 {
			if inSpace(in, csMoonset) {
				return (300 + lv(in)*(600+sky(in)*5) + pow(in)*5) * in.lvScale()
			}
			return (300 + lv(in)*(400+sky(in)*5) + pow(in)*5) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_TWINKLING_GALAXY", Melee: melee, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 {
			return (200 + lv(in)*(400+sky(in)*3) + pow(in)*3) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_STAR_BURST", Melee: melee, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 {
			return (500 + lv(in)*(400+sky(in)*5) + pow(in)*3) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_STAR_CANNON", Melee: melee, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 {
			return (200 + lv(in)*(500+sky(in)*5) + pow(in)*5) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SKE_ALL_IN_THE_SKY", Melee: melee, CritDmg: 0.5, CanCrit: always,
		// 3 full hits against Demi-Humans and Demons, 1 otherwise; no Sky Mastery term.
		Hits: func(in FormulaInput) int {
			if in.race("Demihuman", "Demon") {
				return 3
			}
			return 1
		},
		Ratio: func(in FormulaInput) float64 { return (lv(in)*2000 + pow(in)*10) * in.lvScale() }})

	// ---- Cavaleiro Draconiano (Dragon Knight) ----
	add(&SkillFormula{Code: "DK_SERVANTWEAPON", Melee: melee, CanCrit: always, CritDmg: 0.5,
		Ratio: func(in FormulaInput) float64 { return (200 + lv(in)*50 + pow(in)*5) * in.lvScale() }})
	add(&SkillFormula{Code: "DK_HACKANDSLASHER", Weapons: []string{"2hSword", "2hSpear"},
		Melee: func(w string) bool { return w == "2hSword" },
		Hits:  func(FormulaInput) int { return 2 },
		Ratio: func(in FormulaInput) float64 { return (500 + lv(in)*250 + pow(in)*5) * in.lvScale() }})
	add(&SkillFormula{Code: "DK_STORMSLASH", Weapons: []string{"2hSword", "2hAxe"}, Melee: melee, CanCrit: always, CritDmg: 0.5,
		Hits:  func(in FormulaInput) int { return in.Lv },
		Ratio: func(in FormulaInput) float64 { return (lv(in)*120 + pow(in)*5) * in.lvScale() }})
	add(&SkillFormula{Code: "DK_MADNESS_CRUSHER", Weapons: []string{"2hSword", "2hSpear"},
		Ratio: func(in FormulaInput) float64 {
			return (lv(in)*600 + pow(in)*5 + float64(in.WeaponWt/10*in.WeaponLv)) * in.lvScale()
		}})
	// Dragon Breath (RK_DRAGONBREATH*) has a Dragon Knight-specific formula in
	// latam-ro-calc; it needs per-class lookup and is not ported yet.

	// ---- Executor (Shadow Cross) ----
	add(&SkillFormula{Code: "SHC_SHADOW_STAB", Weapons: []string{"Dagger"}, Melee: melee, IgnoreDef: true,
		Hits: func(in FormulaInput) int {
			if in.state("sc_cloaking_exceed") != 0 {
				return 2
			}
			return 1
		},
		Ratio: func(in FormulaInput) float64 { return (lv(in)*750 + pow(in)*5) * in.lvScale() }})
	add(&SkillFormula{Code: "SHC_DANCING_KNIFE", Weapons: []string{"Dagger"}, Melee: melee,
		Ratio: func(in FormulaInput) float64 { return (lv(in)*200 + pow(in)*5) * in.lvScale() }})
	add(&SkillFormula{Code: "SHC_ETERNAL_SLASH", Melee: melee, CanCrit: always, CritRate: 0.5, CritDmg: 0.5, Stack: [2]int{1, 5},
		Hits: func(in FormulaInput) int { return max(1, in.Stack) },
		Ratio: func(in FormulaInput) float64 {
			if in.state("sc_shadow_exceed") != 0 {
				return (lv(in)*500 + pow(in)*7) * in.lvScale()
			}
			return (lv(in)*350 + pow(in)*5) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SHC_SAVAGE_IMPACT", Weapons: []string{"Katar"}, Melee: melee, CanCrit: always, CritRate: 0.5, CritDmg: 0.5,
		Hits: func(in FormulaInput) int {
			if in.state("sc_cloaking_exceed") != 0 {
				return 5
			}
			return 3
		},
		Ratio: func(in FormulaInput) float64 {
			if in.state("sc_shadow_exceed") != 0 {
				return (lv(in)*100 + pow(in)*7) * in.lvScale()
			}
			return (lv(in)*60 + pow(in)*5) * in.lvScale()
		}})
	add(&SkillFormula{Code: "SHC_IMPACT_CRATER", Weapons: []string{"Katar"}, Melee: melee, CanCrit: always, CritRate: 0.5, CritDmg: 0.5,
		Hits:  func(in FormulaInput) int { return max(1, in.state("sc_spin")) },
		Ratio: func(in FormulaInput) float64 { return (lv(in)*65 + pow(in)*3) * in.lvScale() }})
	add(&SkillFormula{Code: "SHC_FATAL_SHADOW_CROW", Melee: melee, ForceCrit: true,
		Ratio: func(in FormulaInput) float64 {
			race := 0.0
			if in.race("Demihuman", "Dragon") {
				race = 300
			}
			return (lv(in)*(650+race) + pow(in)*10) * in.lvScale()
		}})
}

// LatamPassives applies class passives that change the status window
// (from latam-ro-calc's setAdditionalBonus; MIT).
func LatamPassives(b Build, c *gamedata.Catalog, weapon string, bonus map[string]int) {
	learn := func(code string) int {
		for id, lv := range b.Skills {
			if sk := c.Skills[id]; sk != nil && sk.Code == code {
				return lv
			}
		}
		return 0
	}
	// War Book Mastery (Sky Emperor): with a book, P.ATK +(lv+2) and HIT +3/lv.
	if lv := learn("SKE_WAR_BOOK_MASTERY"); lv > 0 && weapon == "Book" {
		bonus["bPAtk"] += lv + 2
		bonus["bHit"] += lv * 3
	}
}

// ratioFloor applies a percentage ratio with LATAM's flooring.
func ratioFloor(v int64, pct float64) int64 { return int64(math.Floor(float64(v) * pct / 100)) }

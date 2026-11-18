package calc

import (
	"fmt"

	"ragnarok-of-building/internal/script"
)

// BuffDef is a buff or consumable the user can turn on. Values follow the
// skill pages of bROWiki (current LATAM server).
type BuffDef struct {
	Key   string
	Label string
	Group string // "Suporte", "Próprios", "Consumíveis", "Outros"
	// Choices for a selector ({value, label}); nil = on/off; Max > 0 and no
	// choices = a number input from 0 to Max.
	Choices [][2]string
	Max     int
	// Script gives the bonuses for value v (> 0) as rAthena script.
	Script func(v int, base [6]int) string
}

func levels(n int) [][2]string {
	out := [][2]string{{"0", "—"}}
	for i := 1; i <= n; i++ {
		out = append(out, [2]string{fmt.Sprint(i), fmt.Sprintf("Nv. %d", i)})
	}
	return out
}

func amounts(vals ...int) [][2]string {
	out := [][2]string{{"0", "—"}}
	for _, v := range vals {
		out = append(out, [2]string{fmt.Sprint(v), fmt.Sprintf("+%d", v)})
	}
	return out
}

// Buffs lists the selectable buffs in display order.
var Buffs = []BuffDef{
	// Bênção: FOR/DES/INT +Nv, Precisão +2 x Nv.
	{Key: "blessing", Label: "Bênção", Group: "Suporte", Choices: levels(10), Script: func(v int, _ [6]int) string {
		return fmt.Sprintf("bonus bStr,%d; bonus bDex,%d; bonus bInt,%d; bonus bHit,%d;", v, v, v, 2*v)
	}},
	// Clementia: +1 FOR/INT/DES/Precisão per 10 job levels of the Arcebispo.
	{Key: "clementia", Label: "Clementia (bônus extra da Bênção)", Group: "Suporte", Choices: amounts(1, 2, 3, 4, 5, 6, 7), Script: func(v int, _ [6]int) string {
		return fmt.Sprintf("bonus bStr,%d; bonus bDex,%d; bonus bInt,%d; bonus bHit,%d;", v, v, v, v)
	}},
	// Aumentar Agilidade: AGI +(Nv + 2), velocidade de ataque +Nv%.
	{Key: "incagi", Label: "Aumentar Agilidade", Group: "Suporte", Choices: levels(10), Script: func(v int, _ [6]int) string {
		return fmt.Sprintf("bonus bAgi,%d; bonus bAspdRate,%d;", v+2, v)
	}},
	// Glória: SOR +30.
	{Key: "gloria", Label: "Glória (SOR +30)", Group: "Suporte", Script: func(int, [6]int) string { return "bonus bLuk,30;" }},
	// Impositio Manus: ATQ e ATQM +5 x Nv.
	{Key: "impositio", Label: "Impositio Manus", Group: "Suporte", Choices: levels(5), Script: func(v int, _ [6]int) string {
		return fmt.Sprintf("bonus bBaseAtk,%d; bonus bMatk,%d;", 5*v, 5*v)
	}},
	// Expiatio: ignora 5 x Nv% da DEF e DEFM.
	{Key: "expiatio", Label: "Expiatio", Group: "Suporte", Choices: levels(5), Script: func(v int, _ [6]int) string {
		return fmt.Sprintf("bonus2 bIgnoreDefRaceRate,RC_All,%d; bonus2 bIgnoreMdefRaceRate,RC_All,%d;", 5*v, 5*v)
	}},
	// Concentrar: DES e AGI +(Nv + 2)% (of the base value).
	{Key: "concentration", Label: "Concentrar", Group: "Próprios", Choices: levels(10), Script: func(v int, base [6]int) string {
		return fmt.Sprintf("bonus bAgi,%d; bonus bDex,%d;", base[AGI]*(v+2)/100, base[DEX]*(v+2)/100)
	}},
	// Foods: +N to one stat.
	{Key: "food_str", Label: "Comida de FOR", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bStr")},
	{Key: "food_agi", Label: "Comida de AGI", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bAgi")},
	{Key: "food_vit", Label: "Comida de VIT", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bVit")},
	{Key: "food_int", Label: "Comida de INT", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bInt")},
	{Key: "food_dex", Label: "Comida de DES", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bDex")},
	{Key: "food_luk", Label: "Comida de SOR", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bLuk")},
	// Velocidade de ataque: Concentração 10%, Despertar 15%, Fúria Selvagem 20%.
	{Key: "aspd_potion", Label: "Poção de velocidade de ataque", Group: "Consumíveis",
		Choices: [][2]string{{"0", "—"}, {"10", "Poção da Concentração (+10%)"}, {"15", "Poção do Despertar (+15%)"}, {"20", "Poção da Fúria Selvagem (+20%)"}},
		Script:  func(v int, _ [6]int) string { return fmt.Sprintf("bonus bAspdRate,%d;", v) }},
	// Anything else: typed in.
	{Key: "x_atk", Label: "ATQ", Group: "Outros", Max: 9999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bBaseAtk,%d;", v) }},
	{Key: "x_matk", Label: "ATQM", Group: "Outros", Max: 9999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bMatk,%d;", v) }},
	{Key: "x_patk", Label: "P.ATQ", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bPAtk,%d;", v) }},
	{Key: "x_smatk", Label: "S.ATQM", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bSMatk,%d;", v) }},
	{Key: "x_physrate", Label: "Dano físico %", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bAtkRate,%d;", v) }},
	{Key: "x_magicrate", Label: "Dano mágico %", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bMatkRate,%d;", v) }},
}

func statFood(name string) func(int, [6]int) string {
	return func(v int, _ [6]int) string { return fmt.Sprintf("bonus %s,%d;", name, v) }
}

var buffByKey = func() map[string]*BuffDef {
	m := map[string]*BuffDef{}
	for i := range Buffs {
		m[Buffs[i].Key] = &Buffs[i]
	}
	return m
}()

// BuffMax is the highest value a buff accepts.
func (d *BuffDef) BuffMax() int {
	switch {
	case d.Choices != nil:
		n := 0
		for _, c := range d.Choices {
			var v int
			fmt.Sscan(c[0], &v)
			n = max(n, v)
		}
		return n
	case d.Max > 0:
		return d.Max
	}
	return 1
}

// normalizeBuffs drops unknown buffs and clamps values.
func normalizeBuffs(b *Build) {
	for k, v := range b.Buffs {
		d := buffByKey[k]
		if d == nil || v <= 0 {
			delete(b.Buffs, k)
			continue
		}
		b.Buffs[k] = min(v, d.BuffMax())
	}
	if len(b.Buffs) == 0 {
		b.Buffs = nil
	}
}

// applyBuffs adds the active buffs' bonuses. base = base stats + job bonus.
func applyBuffs(b *Build, base [6]int, bonus, extra map[string]int) {
	for _, d := range Buffs {
		v := b.Buffs[d.Key]
		if v <= 0 {
			continue
		}
		res := script.Eval(d.Script(v, base), script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Class: b.Job})
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
	}
}

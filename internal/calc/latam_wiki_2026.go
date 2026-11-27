package calc

// Skills the 2026 rebalance added (client skillinfolist_data.lub), which
// latam-ro-calc had no formulas for, from their bROWiki pages
// (https://browiki.org, sID 6001-6006):
//
//	Dano = [Dano base + (POD × 7)] × (Nv. de base ÷ 100)%
//
// with POD × 10 in the Eureka/Aura forms (and on Laser Triplo), and the
// hits each page gives. A skill's second table follows the class state
// (Eureka, Aura Draconiana, Esmagador). The wiki gives the critical chance
// of Espinhos Furiosos and Laser Triplo as the user's whole CRIT (the
// client text says half).
//
// This file's init runs after latam_gen.go's and latam_wiki.go's (file
// name order).

func init() {
	bl := func(in FormulaInput) float64 { return float64(in.BaseLv) / 100 }
	lv := func(in FormulaInput) float64 { return float64(in.Lv) }
	// base + per × Nv, plus POW × pow, × base level / 100.
	linear := func(base, per, pow float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 { return (base + per*lv(in) + in.T(POW)*pow) * bl(in) }
	}
	hits := func(n int) func(FormulaInput) int { return func(FormulaInput) int { return n } }
	crit := func(FormulaInput) bool { return true }
	// The client's text for both skills that can crit: "A chance é a
	// metade de CRIT do usuário" (half the crit chance; full crit damage).
	const critHalf = 0.5
	byState := func(state string, off, on func(FormulaInput) float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 {
			if in.Active(state) {
				return on(in)
			}
			return off(in)
		}
	}
	hitsByState := func(state string, off, on int) func(FormulaInput) int {
		return func(in FormulaInput) int {
			if in.Active(state) {
				return on
			}
			return off
		}
	}

	// Baforada Draconiana: 50 + 350/Nv, + Fator × 0,01 × (HP máx. ÷ 8 + SP
	// máx. ÷ 4); Fator 5/Nv (7/Nv with Aura Draconiana). Ignores DEF, not
	// FLEE.
	breath := func(factor, pow float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 {
			hpsp := factor * lv(in) * 0.01 * (float64(in.MaxHP)/8 + float64(in.MaxSP)/4)
			return (50 + 350*lv(in) + hpsp + in.T(POW)*pow) * bl(in)
		}
	}
	wikiAdd([]string{"JT_DRAGON_KNIGHT"}, &SkillFormula{Code: "DK_DRAGONIC_BREATH", Name: "Dragonic Breath",
		Melee: meleeNever, IgnoreDef: true, Ratio: byState("Dragonic Aura", breath(5, 7), breath(7, 10))})

	meister := []string{"JT_MEISTER"}
	// Explosão de Faíscas: 250 + 750/Nv, twice, ignoring DEF.
	wikiAdd(meister, &SkillFormula{Code: "MT_SPARK_BLASTER", Name: "Spark Blaster",
		Melee: meleeNever, IgnoreDef: true, Hits: hits(2), Ratio: linear(250, 750, 7)})
	// Laser Triplo: 300 + 600/Nv, 3 lasers, can crit.
	wikiAdd(meister, &SkillFormula{Code: "MT_TRIPLE_LASER", Name: "Triple Laser",
		Melee: meleeNever, Hits: hits(3), CanCrit: crit, CritRate: critHalf, Ratio: linear(300, 600, 10)})
	// Poder Esmagador: 100 + 300/Nv x 3; with Esmagador (Machado
	// Esmagador's state) 150 + 300/Nv x 5.
	wikiAdd(meister, &SkillFormula{Code: "MT_MIGHTY_SMASH", Name: "Mighty Smash",
		Melee: meleeAlways, Weapons: []string{"1hAxe", "2hAxe"}, Hits: hitsByState("Axe Stomp", 3, 5),
		Ratio: byState("Axe Stomp", linear(100, 300, 7), linear(150, 300, 7))})

	biolo := []string{"JT_BIOLO"}
	// Pólen Explosivo: 400 + 450/Nv x 3; with Eureka 400 + 550/Nv x 5.
	wikiAdd(biolo, &SkillFormula{Code: "BO_EXPLOSIVE_POWDER", Name: "Explosive Powder",
		Melee: meleeAlways, Hits: hitsByState("Research Report", 3, 5),
		Ratio: byState("Research Report", linear(400, 450, 7), linear(400, 550, 10))})
	// Espinhos Furiosos: 200 + 250/Nv x 3; with Eureka 250 + 300/Nv x 5;
	// can crit.
	wikiAdd(biolo, &SkillFormula{Code: "BO_MAYHEMIC_THORNS", Name: "Mayhemic Thorns",
		Melee: meleeNever, Hits: hitsByState("Research Report", 3, 5), CanCrit: crit, CritRate: critHalf,
		Ratio: byState("Research Report", linear(200, 250, 7), linear(250, 300, 10))})
	// Atirar Ácido (Sólido, Álgido, Gasoso, Inflamável): 7 hits of [400/Nv +
	// POD × (1 + Manipular Poção)] × base level. Eureka × 1,5, and the POD
	// part × 1,5 again on Amorfo and Planta (latam-ro-calc's reading; the
	// Eureka page prints that term apart).
	for code, ele := range map[string]string{"BO_ACIDIFIED_ZONE_WATER": "Water", "BO_ACIDIFIED_ZONE_GROUND": "Earth",
		"BO_ACIDIFIED_ZONE_WIND": "Wind", "BO_ACIDIFIED_ZONE_FIRE": "Fire"} {
		wikiAdd(biolo, &SkillFormula{Code: code, Name: "Acidified Zone " + ele, Element: ele, Melee: meleeNever, Hits: hits(7),
			Ratio: func(in FormulaInput) float64 {
				pow, all := in.T(POW)*float64(1+in.LearnLv("BO_BIONIC_PHARMACY")), 1.0
				if in.Active("Research Report") {
					all = 1.5
					if in.Monster != nil && (in.Monster.Race == "Formless" || in.Monster.Race == "Plant") {
						pow *= 1.5
					}
				}
				return (400*lv(in) + pow) * bl(in) * all
			}})
	}
}

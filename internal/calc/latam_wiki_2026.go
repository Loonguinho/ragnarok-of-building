package calc

// Skills the 2026 rebalance added (client skillinfolist_data.lub), which
// latam-ro-calc has no formulas for yet, from their bROWiki pages
// (https://browiki.org, sID 6001-6006):
//
//	Dano = [Dano base + (POD × 7)] × (Nv. de base ÷ 100)%
//
// with POD × 10 in the Eureka/Aura forms (and on Laser Triplo), and the
// hits each page gives. The wiki gives the critical chance of Espinhos
// Furiosos and Laser Triplo as the user's whole CRIT (the client text says
// half).
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

	// Baforada Draconiana: 50 + 350/Nv, + Fator × 0,01 × (HP máx. ÷ 8 + SP
	// máx. ÷ 4); Fator 5/Nv (7/Nv with Aura Draconiana). Ignores DEF, not
	// FLEE.
	breath := func(factor, pow float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 {
			hpsp := factor * lv(in) * 0.01 * (float64(in.MaxHP)/8 + float64(in.MaxSP)/4)
			return (50 + 350*lv(in) + hpsp + in.T(POW)*pow) * bl(in)
		}
	}
	dk := []string{"JT_DRAGON_KNIGHT"}
	wikiAdd(dk, &SkillFormula{Code: "DK_DRAGONIC_BREATH", Name: "Dragonic Breath", Variant: "Normal",
		Melee: meleeNever, IgnoreDef: true, Ratio: breath(5, 7)})
	wikiAdd(dk, &SkillFormula{Code: "DK_DRAGONIC_BREATH", Name: "Dragonic Breath", Variant: "Aura Draconiana",
		Melee: meleeNever, IgnoreDef: true, Ratio: breath(7, 10)})

	meister := []string{"JT_MEISTER"}
	// Explosão de Faíscas: 250 + 750/Nv, twice, ignoring DEF.
	wikiAdd(meister, &SkillFormula{Code: "MT_SPARK_BLASTER", Name: "Spark Blaster",
		Melee: meleeNever, IgnoreDef: true, Hits: hits(2), Ratio: linear(250, 750, 7)})
	// Laser Triplo: 300 + 600/Nv, 3 lasers, can crit.
	wikiAdd(meister, &SkillFormula{Code: "MT_TRIPLE_LASER", Name: "Triple Laser",
		Melee: meleeNever, Hits: hits(3), CanCrit: crit, Ratio: linear(300, 600, 10)})
	// Poder Esmagador: 100 + 300/Nv x 3; with Esmagador 150 + 300/Nv x 5.
	axes := []string{"1hAxe", "2hAxe"}
	wikiAdd(meister, &SkillFormula{Code: "MT_MIGHTY_SMASH", Name: "Mighty Smash", Variant: "Normal",
		Melee: meleeAlways, Weapons: axes, Hits: hits(3), Ratio: linear(100, 300, 7)})
	wikiAdd(meister, &SkillFormula{Code: "MT_MIGHTY_SMASH", Name: "Mighty Smash", Variant: "Esmagador",
		Melee: meleeAlways, Weapons: axes, Hits: hits(5), Ratio: linear(150, 300, 7)})

	biolo := []string{"JT_BIOLO"}
	// Pólen Explosivo: 400 + 450/Nv x 3; with Eureka 400 + 550/Nv x 5.
	wikiAdd(biolo, &SkillFormula{Code: "BO_EXPLOSIVE_POWDER", Name: "Explosive Powder", Variant: "Normal",
		Melee: meleeAlways, Hits: hits(3), Ratio: linear(400, 450, 7)})
	wikiAdd(biolo, &SkillFormula{Code: "BO_EXPLOSIVE_POWDER", Name: "Explosive Powder", Variant: "Eureka",
		Melee: meleeAlways, Hits: hits(5), Ratio: linear(400, 550, 10)})
	// Espinhos Furiosos: 200 + 250/Nv x 3; with Eureka 250 + 300/Nv x 5;
	// can crit.
	wikiAdd(biolo, &SkillFormula{Code: "BO_MAYHEMIC_THORNS", Name: "Mayhemic Thorns", Variant: "Normal",
		Melee: meleeNever, Hits: hits(3), CanCrit: crit, Ratio: linear(200, 250, 7)})
	wikiAdd(biolo, &SkillFormula{Code: "BO_MAYHEMIC_THORNS", Name: "Mayhemic Thorns", Variant: "Eureka",
		Melee: meleeNever, Hits: hits(5), CanCrit: crit, Ratio: linear(250, 300, 10)})
}

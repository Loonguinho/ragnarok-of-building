package calc

// Skill formulas from bROWiki (https://browiki.org), which documents the
// current LATAM server. They correct or complete the latam-ro-calc formulas
// in latam_gen.go:
//
//   - corrections: only where bROWiki's damage table was rewritten for the
//     28/10/2026 client (29-31/10/2026, like the tables that already match
//     the 2022 rebalance) and disagrees with the generated formula;
//   - additions: skills latam_gen.go could not translate.
//
// Every value checks against the wiki's table at base level 100 with no
// stats (latam_wiki_test.go). tools/browiki compares all skills.
//
// This file's init runs after latam_gen.go's (Go runs a package's init
// functions in file name order), so the generated formulas exist here.

const wikiSource = "bROWiki"

func init() {
	// ---- corrections (tables updated after the 28/10/2026 client) ----

	// Ofensiva Abissal: 100 + 500/Nv (was 350/Nv).
	wikiFix("ABC_ABYSS_DAGGER", func(f *SkillFormula) {
		f.Ratio = func(in FormulaInput) float64 {
			return (100 + float64(in.Lv)*500 + in.T(POW)*5) * float64(in.BaseLv) / 100
		}
	})
	// Impacto Cratera: 80/Nv per rotation point (was 65/Nv).
	wikiFix("SHC_IMPACT_CRATER", func(f *SkillFormula) {
		f.Ratio = func(in FormulaInput) float64 {
			return (float64(in.Lv)*80 + in.T(POW)*3) * float64(in.BaseLv) / 100
		}
	})
	// Ira da Terra: 1500 + 700/Nv + Self Study Sorcery x 4/Nv + SPL x 3.
	wikiFix("HN_HELLS_DRIVE", func(f *SkillFormula) {
		f.Ratio = func(in FormulaInput) float64 {
			lv, sorc := float64(in.Lv), float64(in.LearnName("Self Study Sorcery"))
			return genHyperNovice_withPassiveBonus(in, (1500+lv*(700+sorc*4)+in.T(SPL)*3)*float64(in.BaseLv)/100, sorc, 1.7, "Angel of Magic")
		}
	})
	// Chuva de Meteoritos: fall 300 + 320/Nv, explosion 450 + 160/Nv
	// (+ Self Study Sorcery x 5/Nv, SPL x 3).
	wikiFix("HN_METEOR_STORM_BUSTER", func(f *SkillFormula) {
		explosion := f.Variant == "Explosão"
		f.Ratio = func(in FormulaInput) float64 {
			lv, sorc := float64(in.Lv), float64(in.LearnName("Self Study Sorcery"))
			if explosion {
				return genHyperNovice_withUltimate(in, (450+lv*(160+sorc*5)+in.T(SPL)*3)*float64(in.BaseLv)/100, 1.5, "Angel of Magic")
			}
			return genHyperNovice_withPassiveBonus(in, (300+lv*(320+sorc*5)+in.T(SPL)*3)*float64(in.BaseLv)/100, sorc, 1.5, "Angel of Magic")
		}
	})
	// Infiltrar: 700/Nv + CON x 3 (latam-ro-calc treats it as no damage).
	wikiFix("SS_SHIMIRU", func(f *SkillFormula) {
		f.Ratio = func(in FormulaInput) float64 {
			return (float64(in.Lv)*700 + in.T(CON)*3) * float64(in.BaseLv) / 100
		}
		f.Melee = meleeAlways
	})
	// Golpe da Palma em Fúria: (200/Nv + FOR x 2) x Nv. de base / 100.
	wikiFix("CH_PALMSTRIKE", func(f *SkillFormula) {
		f.Ratio = func(in FormulaInput) float64 {
			return (float64(in.Lv)*200 + in.S(STR)*2) * float64(in.BaseLv) / 100
		}
	})
	// Impacto Sísmico: (300/Nv + FOR x 3) x Nv. de base / 100 on visible
	// targets; (400/Nv + FOR x 5) on hidden ones (variant added below).
	wikiFix("SR_EARTHSHAKER", func(f *SkillFormula) {
		f.Ratio = func(in FormulaInput) float64 {
			return (float64(in.Lv)*300 + in.S(STR)*3) * float64(in.BaseLv) / 100
		}
		f.Variant = "Alvo visível"
	})
	// Zona Gravitacional: the zone hits once, 3000 + 1500/Nv + Mágico
	// Autodidata x 4/Nv + FEI x 5; the gravity then hits every 0,5 s for 5 s,
	// 800 + 700/Nv + Mágico Autodidata x 2/Nv + FEI x 2.
	wikiFix("HN_GROUND_GRAVITATION", func(f *SkillFormula) {
		zone := f.Variant == "Inicial"
		f.Ratio = func(in FormulaInput) float64 {
			lv, sorc := float64(in.Lv), float64(in.LearnName("Self Study Sorcery"))
			if zone {
				return genRebalance_hyper(in, 3000+lv*(1500+sorc*4)+in.T(SPL)*5, true, 1.5, 1, 0)
			}
			return genRebalance_hyper(in, 800+lv*(700+sorc*2)+in.T(SPL)*2, true, 1.5, 1, 0)
		}
		if zone {
			f.Variant, f.Hits = "Zona (1º dano)", constHits(1)
		} else {
			f.Variant = "Gravidade (contínuo)"
		}
	})

	// ---- skills latam-ro-calc's formulas don't cover ----
	for _, j := range []string{"JT_SURA", "JT_SURA_H"} {
		if latamByJob[j]["SR_EARTHSHAKER"] != nil {
			regJob(j, &SkillFormula{Code: "SR_EARTHSHAKER", Name: "Earth Shaker", Variant: "Alvo oculto", Melee: meleeAlways, Source: wikiSource,
				Ratio: func(in FormulaInput) float64 { return (float64(in.Lv)*400 + in.S(STR)*5) * float64(in.BaseLv) / 100 }})
		}
	}

	bl := func(in FormulaInput) float64 { return float64(in.BaseLv) / 100 }
	lv := func(in FormulaInput) float64 { return float64(in.Lv) }
	shieldWt := func(in FormulaInput) float64 { return float64(in.ShieldWt) / 10 }

	wikiAdd([]string{"JT_ALCHEMIST"}, &SkillFormula{Code: "CR_ACIDDEMONSTRATION", Name: "Acid Bomb", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 { // + Nv x VIT of the target
			return (lv(in)*200 + in.S(INT)*2 + lv(in)*in.MonStat(2)) * bl(in)
		}})
	// Canhão de Prótons: 250/Nv + 20 × Nv × Aprimorar Carrinho + INT × 2; the
	// target's hard DEF is subtracted like the soft one and RES does not
	// apply (latam-ro-calc, measured on gn-cart-cannon-gear-states.rrf).
	wikiAdd([]string{"JT_GENETIC", "JT_GENETIC_H"}, &SkillFormula{Code: "GN_CARTCANNON", Name: "Cart Cannon", Melee: meleeNever,
		Hit100: true, HardDefFlat: true,
		Ratio: func(in FormulaInput) float64 {
			return (lv(in)*250 + lv(in)*float64(in.LearnLv("GN_REMODELING_CART"))*20 + in.S(INT)*2) * bl(in)
		}})
	wikiAdd([]string{"JT_GENETIC", "JT_GENETIC_H"}, &SkillFormula{Code: "GN_SPORE_EXPLOSION", Name: "Spore Explosion", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 { return (400 + lv(in)*200 + in.S(INT)) * bl(in) }})
	// Tornado de Carrinho: 200/Nv (400/Nv with Cultivar Bárbaro) + Aprimorar
	// Carrinho x 50 + cart weight / (150 - base STR); the cart's weight is
	// not simulated.
	cart := func(per float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 {
			return lv(in)*per + float64(in.LearnLv("GN_REMODELING_CART"))*50
		}
	}
	cartJobs := []string{"JT_GENETIC", "JT_GENETIC_H", "JT_BIOLO"}
	wikiAdd(cartJobs, &SkillFormula{Code: "GN_CART_TORNADO", Name: "Cart Tornado", Variant: "Normal", Melee: meleeAlways,
		Partial: "peso do carrinho", Ratio: cart(200)})
	wikiAdd(cartJobs, &SkillFormula{Code: "GN_CART_TORNADO", Name: "Cart Tornado", Variant: "Cultivar Bárbaro", Melee: meleeAlways,
		Partial: "peso do carrinho", Ratio: cart(400)})
	// Cortar em Espiral (Hiperaprendiz): {[550 + 350/Nv + Físico Autodidata
	// x 3/Nv] x tamanho + POD x 3} x Nv. de base / 100; tamanho 1,5
	// pequeno, 1,3 médio, 1,2 grande.
	wikiAdd([]string{"JT_HYPER_NOVICE"}, &SkillFormula{Code: "HN_SPIRAL_PIERCE_MAX", Name: "Spiral Pierce Max", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 {
			size := 1.3
			switch {
			case in.MonIs("size", "small"):
				size = 1.5
			case in.MonIs("size", "large"):
				size = 1.2
			}
			tactics := float64(in.LearnName("Self Study Tactics"))
			return genRebalance_hyper(in, (550+lv(in)*(350+tactics*3))*size+in.T(POW)*3, false, 1.5, 2, 10)
		}})
	wikiAdd([]string{"JT_IMPERIAL_GUARD"}, &SkillFormula{Code: "IG_SHIELD_SHOOTING", Name: "Shield Shooting", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 {
			return (400 + lv(in)*2300 + float64(in.LearnLv("IG_SHIELD_MASTERY"))*15 + shieldWt(in) + float64(in.ShieldRefine)*4 + in.T(POW)*5) * bl(in)
		}})
	wikiAdd([]string{"JT_ROYAL_GUARD", "JT_ROYAL_GUARD_H"}, &SkillFormula{Code: "LG_SHIELDPRESS", Name: "Shield Press", Melee: meleeAlways,
		Ratio: func(in FormulaInput) float64 {
			return (lv(in)*200+float64(in.LearnLv("IG_SHIELD_MASTERY"))*15+in.S(STR)+shieldWt(in))*bl(in) + in.S(VIT)*float64(in.ShieldRefine)
		}})
	wikiAdd([]string{"JT_CRUSADER"}, &SkillFormula{Code: "PA_SHIELDCHAIN", Name: "Shield Chain", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 {
			return (300 + lv(in)*200 + float64(in.LearnLv("IG_SHIELD_MASTERY"))*15*lv(in) + shieldWt(in) + float64(in.ShieldRefine)*4) * bl(in)
		}})
	mado := []string{"JT_MECHANIC", "JT_MECHANIC_H"}
	wikiAdd(mado, &SkillFormula{Code: "NC_ARMSCANNON", Name: "Arm Cannon", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 { return (400 + lv(in)*350) * bl(in) }})
	wikiAdd(mado, &SkillFormula{Code: "NC_BOOSTKNUCKLE", Name: "Knuckle Boost", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 { return (100 + lv(in)*240 + in.S(DEX)) * bl(in) }})
	wikiAdd(mado, &SkillFormula{Code: "NC_POWERSWING", Name: "Power Swing", Melee: meleeAlways,
		Ratio: func(in FormulaInput) float64 { return (300 + lv(in)*100 + in.S(STR) + in.S(DEX)) * bl(in) }})
	wikiAdd(mado, &SkillFormula{Code: "NC_VULCANARM", Name: "Vulcan Arm", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 { return (lv(in)*230 + in.S(DEX)) * bl(in) }})
	wikiAdd([]string{"JT_REBELLION"}, &SkillFormula{Code: "RL_D_TAIL", Name: "Dragon Tail", Melee: meleeNever,
		Ratio: func(in FormulaInput) float64 { return (500 + lv(in)*200) * bl(in) }})
	hammer := func(per float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 { return (lv(in)*100 + float64(in.Stack)*per) * bl(in) }
	}
	wikiAdd([]string{"JT_REBELLION"}, &SkillFormula{Code: "RL_HAMMER_OF_GOD", Name: "God's Hammer", Melee: meleeNever,
		Ratio: hammer(150), Stack: [2]int{1, 10}, Variant: "Normal"})
	wikiAdd([]string{"JT_REBELLION"}, &SkillFormula{Code: "RL_HAMMER_OF_GOD", Name: "God's Hammer", Melee: meleeNever,
		Ratio: hammer(400), Stack: [2]int{1, 10}, Variant: "Alvo marcado"})
	wikiAdd([]string{"JT_SORCERER", "JT_SORCERER_H"}, &SkillFormula{Code: "SO_POISON_BUSTER", Name: "Poison Burst", Magic: true, Element: "Poison",
		Ratio: func(in FormulaInput) float64 { return (1000 + lv(in)*300 + in.S(INT)) * bl(in) }})
	wikiAdd([]string{"JT_SURA", "JT_SURA_H"}, &SkillFormula{Code: "SR_KNUCKLEARROW", Name: "Knuckle Arrow", Melee: meleeAlways,
		Partial: "dano extra da colisão ao empurrar",
		Ratio:   func(in FormulaInput) float64 { return (500 + lv(in)*100) * bl(in) }})
	wikiAdd([]string{"JT_MINSTREL", "JT_WANDERER", "JT_MINSTREL_H", "JT_WANDERER_H"}, &SkillFormula{Code: "WM_SEVERE_RAINSTORM", Name: "Severe Rainstorm", Melee: meleeNever,
		Partial: "número de ondas na área",
		Ratio:   func(in FormulaInput) float64 { return (lv(in)*100 + (in.S(DEX)+in.S(AGI))/2) * bl(in) }})
}

// wikiFix corrects every registered variant of a generated formula.
func wikiFix(code string, fix func(f *SkillFormula)) {
	found := false
	for _, m := range latamByJob {
		if f := m[code]; f != nil {
			for _, v := range f.Variants() {
				fix(v)
				v.Source = wikiSource
			}
			found = true
		}
	}
	if !found {
		panic("latam_wiki: no generated formula for " + code)
	}
}

// wikiAdd registers a formula for jobs that have none for the skill yet.
func wikiAdd(jobs []string, f *SkillFormula) {
	f.Source = wikiSource
	for _, j := range jobs {
		if prev := latamByJob[j][f.Code]; prev != nil && prev.Source != wikiSource {
			continue // latam-ro-calc's formula wins once it exists
		}
		g := *f
		regJob(j, &g)
	}
}

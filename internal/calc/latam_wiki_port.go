package calc

import "math"

// Skills latamgen does not translate (latam-ro-calc builds them in helper
// functions or shared lists), written by hand from latam-ro-calc. The
// per-level numbers agree with the client's tables where it prints them
// (latam_wiki_port_test.go); the rest is noted.
//
// Left out on purpose: Ruína (both sources add FOR × 100, which dwarfs the
// skill's own table), Ataque Aéreo
// (a falcon attack with its own damage) and the SG heats.

func init() {
	bl := func(in FormulaInput) float64 { return float64(in.BaseLv) / 100 }
	lv := func(in FormulaInput) float64 { return float64(in.Lv) }
	flat := func(base, per float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 { return base + per*lv(in) }
	}
	scaled := func(base, per float64) func(FormulaInput) float64 {
		return func(in FormulaInput) float64 { return (base + per*lv(in)) * bl(in) }
	}
	hits := func(n int) func(FormulaInput) int { return func(FormulaInput) int { return n } }
	perLv := func(plus int) func(FormulaInput) int { return func(in FormulaInput) int { return in.Lv + plus } }
	add := func(f SkillFormula, jobs ...string) {
		for _, j := range jobs {
			if latamByJob[j][f.Code] != nil && len(f.Variant) == 0 {
				continue
			}
			g := f
			regJob(j, &g)
		}
	}

	// ---- Bruxo / Sábio (magic) ----
	wiz := []string{"JT_WIZARD", "JT_SUPERNOVICE2"}
	magic := func(code, name, ele string, ratio func(FormulaInput) float64, h func(FormulaInput) int, jobs ...string) {
		add(SkillFormula{Code: code, Name: name, Magic: true, Element: ele, Ratio: ratio, Hits: h}, jobs...)
	}
	magic("WZ_JUPITEL", "Jupitel Thunder", "Wind", flat(100, 0), perLv(2), wiz...)
	magic("WZ_VERMILION", "Lord of Vermilion", "Wind", flat(400, 100), nil, wiz...)
	magic("WZ_STORMGUST", "Storm Gust", "Water", flat(70, 50), hits(10), wiz...)
	magic("WZ_EARTHSPIKE", "Earth Spike", "Earth", flat(200, 0), perLv(0), append(wiz, "JT_SAGE")...)
	magic("WZ_HEAVENDRIVE", "Heaven's Drive", "Earth", flat(125, 0), perLv(0), append(wiz, "JT_SAGE")...)
	magic("WZ_FROSTNOVA", "Frost Nova", "Water", flat(100, 10), nil, wiz...)
	magic("WZ_SIGHTRASHER", "Sightrasher", "Fire", flat(100, 20), nil, wiz...)
	magic("WZ_WATERBALL", "Water Ball", "Water", flat(100, 30),
		func(in FormulaInput) int { return []int{1, 9, 9, 25, 25}[min(max(in.Lv, 1), 5)-1] }, wiz...)
	// The hits one target takes (the client's "Golpes" column).
	magic("WZ_METEOR", "Meteor Storm", "Fire", flat(125, 0), func(in FormulaInput) int { return (in.Lv + 1) / 2 }, wiz...)
	for _, j := range wiz {
		regJob(j, &SkillFormula{Code: "WZ_FIREPILLAR", Name: "Fire Pillar", Magic: true, Element: "Fire",
			IgnoreDef: true, IgnoreSoftDef: true, Ratio: flat(20, 0), Hits: perLv(2),
			Flat: func(FormulaInput) int64 { return 50 }})
	}
	magic("HW_NAPALMVULCAN", "Napalm Vulcan", "Ghost", scaled(0, 70), perLv(0), "JT_WIZARD_H")
	magic("PR_MAGNUS", "Magnus Exorcismus", "Holy", func(in FormulaInput) float64 {
		if m := in.Monster; m != nil && (m.Race == "Demon" || m.Race == "Undead" || m.Element == "Undead" || m.Element == "Dark") {
			return 130
		}
		return 100
	}, perLv(0), "JT_PRIEST")
	// Tetra Vortex: four spheres of the summoned elements.
	for _, e := range [][2]string{{"Fire", "Fogo"}, {"Water", "Água"}, {"Wind", "Vento"}, {"Earth", "Terra"}} {
		add(SkillFormula{Code: "WL_TETRAVORTEX", Name: "Tetra Vortex", Variant: "Esferas de " + e[1], Magic: true, Element: e[0],
			Ratio: flat(800, 400), Hits: hits(4)}, "JT_WARLOCK", "JT_WARLOCK_H")
	}
	// Pilares de Pedra: per pillar that hits (up to 4/Nv).
	add(SkillFormula{Code: "AG_VIOLENT_QUAKE", Name: "Violent Quake", Magic: true, Element: "Earth", Stack: [2]int{1, 20},
		Partial: "quantos pilares acertam o alvo: escolha nos acúmulos",
		Ratio: func(in FormulaInput) float64 {
			if in.ActiveLv("Climax") == 4 {
				return 0
			}
			return (200 + 1200*lv(in) + in.T(SPL)*5) * bl(in)
		},
		Hits: func(in FormulaInput) int {
			n := min(max(in.Stack, 1), 4*in.Lv)
			if in.ActiveLv("Climax") == 1 {
				n *= 2
			}
			return n
		}}, "JT_ARCH_MAGE")

	// ---- Espiritualista, Taekwon, Invocador ----
	magic("SL_STIN", "Estin", "Neutral", flat(0, 10), nil, "JT_LINKER")
	magic("SL_STUN", "Estun", "Neutral", flat(0, 5), nil, "JT_LINKER")
	magic("SL_SMA", "Esma", "Neutral", func(in FormulaInput) float64 { return float64(in.BaseLv) + 40 }, perLv(0), "JT_LINKER")
	kick := func(code, name string, base, per float64, hit100 bool, melee func(FormulaInput) bool) {
		add(SkillFormula{Code: code, Name: name, Ratio: flat(base, per), Melee: melee, Hit100: hit100}, "JT_TAEKWON")
	}
	kick("TK_STORMKICK", "Tornado Kick", 160, 20, false, meleeAlways)
	kick("TK_DOWNKICK", "Heel Drop", 160, 20, false, meleeAlways)
	kick("TK_TURNKICK", "Roundhouse", 190, 30, false, meleeAlways)
	kick("TK_COUNTER", "Counter Kick", 190, 30, true, meleeAlways)
	kick("TK_JUMPKICK", "Flying Kick", 30, 10, false, meleeNever)
	// The level picks the element.
	add(SkillFormula{Code: "SU_SV_STEMSPEAR", Name: "Silvervine Stem Spear", Magic: true, Ratio: flat(700, 0),
		ElementFn: func(in FormulaInput) string {
			return []string{"Earth", "Fire", "Water", "Wind", "Ghost"}[min(max(in.Lv, 1), 5)-1]
		}}, "JT_DO_SUMMONER")

	// ---- Arqueiros e músicos ----
	add(SkillFormula{Code: "RA_ARROWSTORM", Name: "Arrow Storm", Melee: meleeNever, Ratio: func(in FormulaInput) float64 {
		return (200 + tern(in.Active("Fear Breeze"), 250, 180)*lv(in)) * bl(in)
	}}, "JT_RANGER", "JT_RANGER_H")
	add(SkillFormula{Code: "RA_WUGSTRIKE", Name: "Wug Strike", Melee: meleeNever, Element: "Neutral", Ratio: flat(0, 200),
		Partial: "bônus próprio do Worg"}, "JT_RANGER", "JT_RANGER_H")
	add(SkillFormula{Code: "BA_MUSICALSTRIKE", Name: "Melody Strike", Melee: meleeNever, Ratio: flat(110, 40)}, "JT_BARD")
	add(SkillFormula{Code: "DC_THROWARROW", Name: "Throw Arrow", Melee: meleeNever, Ratio: flat(110, 40)}, "JT_DANCER")
	add(SkillFormula{Code: "CG_ARROWVULCAN", Name: "Arrow Vulcan", Melee: meleeNever, Ratio: scaled(500, 100)}, "JT_BARD_H", "JT_DANCER_H")
	// Ruído Estridente: 120/Nv + Domínio Musical × 60 (bROWiki; latam-ro-calc
	// doubles it, which the wiki gives only against Sono Profundo).
	add(SkillFormula{Code: "WM_METALICSOUND", Name: "Metalic Sound", Magic: true, Element: "Neutral",
		Partial: "dano maior em alvo com Sono Profundo",
		Ratio: func(in FormulaInput) float64 {
			return (tern(in.Active("_Debuf_Sonic_Brand"), 156, 120)*lv(in) + 60*float64(in.LearnLv("WM_LESSON"))) * bl(in)
		}}, "JT_MINSTREL", "JT_MINSTREL_H", "JT_WANDERER", "JT_WANDERER_H")

	// ---- Arruaceiro, Ninja ----
	add(SkillFormula{Code: "RG_RAID", Name: "Sightless Mind", Melee: meleeAlways, Ratio: flat(50, 150)}, "JT_ROGUE", "JT_SUPERNOVICE2")
	add(SkillFormula{Code: "RG_BACKSTAP", Name: "Back Stab", Melee: meleeAlways,
		Ratio: func(in FormulaInput) float64 { return (300 + 40*lv(in)) * tern(in.Weapon == "Bow", 0.5, 1) },
		Hits: func(in FormulaInput) int {
			if in.Weapon == "Dagger" {
				return 2
			}
			return 1
		}}, "JT_ROGUE", "JT_SUPERNOVICE2")
	for _, wound := range []bool{false, true} {
		name := "Normal"
		if wound {
			name = "Com Ferida Cruzada"
		}
		add(SkillFormula{Code: "KO_JYUMONJIKIRI", Name: "Cross Slash", Variant: name, Melee: meleeNever,
			Ratio: func(in FormulaInput) float64 {
				return 200*lv(in)*bl(in) + tern(wound, float64(in.BaseLv)*lv(in), 0)
			}}, "JT_KAGEROU", "JT_OBORO")
	}
	add(SkillFormula{Code: "KO_BAKURETSU", Name: "Kunai Explosion", Melee: meleeNever, Hit100: true,
		Partial: "o bROWiki divide o nível de base por 120 em vez de 100",
		Ratio: func(in FormulaInput) float64 {
			return (50+math.Floor(in.S(DEX))/4)*lv(in)*0.4*float64(in.LearnLv("NJ_TOBIDOUGU"))*bl(in) + float64(in.JobLv)*10
		}}, "JT_KAGEROU", "JT_OBORO")

	// Arremessar Amuleto: 200% per charm, 10 charms, in their element.
	add(SkillFormula{Code: "KO_KAIHOU", Name: "Release Ninja Spell", Magic: true, Ratio: scaled(2000, 0), RequiresState: "Spirit Charm",
		ElementFn: func(in FormulaInput) string {
			return [...]string{"Neutral", "Fire", "Water", "Wind", "Earth"}[min(max(in.ActiveLv("Spirit Charm"), 0), 4)]
		}}, "JT_KAGEROU", "JT_OBORO")

	// ---- Shura: Garra de Tigre ----
	for _, combo := range []bool{false, true} {
		name, div, special := "Normal", 4.0, int64(240)
		if combo {
			name, div, special = "Após Ruína", 1.5, 500
		}
		add(SkillFormula{Code: "SR_TIGERCANNON", Name: "Tiger Cannon", Variant: name, Melee: meleeAlways,
			Ratio: func(in FormulaInput) float64 {
				factor := float64(in.MaxHP)*(10+2*lv(in))*0.01 + float64(in.MaxSP)*(5+lv(in))*0.01
				return math.Floor(factor / div * bl(in))
			},
			Flat: func(in FormulaInput) int64 {
				n := int64(in.Lv) * special
				if in.Monster != nil {
					n += int64(in.Monster.Level) * 40
				}
				return n
			}}, "JT_SURA", "JT_SURA_H")
	}

	// ---- Cavaleiro Rúnico: Sopro e Bafo do Dragão ----
	// floor(HP ÷ 50) + floor(SP máx. ÷ 4), × Nv × base level × the Dragon
	// Training term, which takes POW and P.ATK once Aura Draconiana is
	// learned (latam-ro-calc, two recorded hits to the unit; HP at 100%).
	breath := func(code, name string, ele func(FormulaInput) string) {
		add(SkillFormula{Code: code, Name: name, Melee: meleeNever, Hit100: true, ElementFn: ele,
			Ratio: func(FormulaInput) float64 { return 100 },
			Base: func(in FormulaInput, r Result) int64 {
				return dragonBreath(in.MaxHP, in.MaxSP, in.Lv, in.BaseLv, in.LearnLv("RK_DRAGONTRAINING"), in.LearnLv("DK_DRAGONIC_AURA"), int(in.T(POW)), r.PATK)
			},
			BaseMul: func(in FormulaInput) int64 {
				if in.Active("Dragonic Aura") {
					return 2
				}
				return 1
			}}, "JT_RUNE_KNIGHT", "JT_RUNE_KNIGHT_H")
	}
	breath("RK_DRAGONBREATH", "Dragon Breath", func(in FormulaInput) string {
		switch {
		case in.Active("Lux Anima Runestone"):
			return "Dark"
		case in.Active("Turisus Runestone"):
			return "Holy"
		}
		return "Fire"
	})
	breath("RK_DRAGONBREATH_WATER", "Dragon Breath - WATER", func(in FormulaInput) string {
		switch {
		case in.Active("Lux Anima Runestone"):
			return "Neutral"
		case in.Active("Asir Runestone"):
			return "Ghost"
		}
		return "Water"
	})
}

// dragonBreath is Sopro/Bafo do Dragão before DEF.
func dragonBreath(hp, sp, lv, base, training, auraLv, pow, patk int) int64 {
	return int64((hp/50+sp/4)*lv*base*dragonBreathTerm(training, auraLv, pow, patk)) / 10000
}

// dragonBreathTerm: 90 + 10 × Adestrar Dragão; with Aura Draconiana learned,
// floor((that + POW ÷ 5) × (1 + P.ATK ÷ 100)).
func dragonBreathTerm(training, auraLv, pow, patk int) int {
	term := 90 + 10*training
	if auraLv > 0 {
		term = (5*term + pow) * (100 + patk) / 500
	}
	return term
}

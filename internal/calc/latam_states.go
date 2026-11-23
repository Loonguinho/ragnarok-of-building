package calc

import "strconv"

// Class states and buffs latam-ro-calc defines in shared files or helper
// functions, which buffgen does not follow. Values: client descriptions,
// bROWiki and latam-ro-calc, only where they agree.
//
// This file's init runs after latam_gen.go's (file name order).

func init() {
	yes := func(v string) [][2]string { return [][2]string{{"0", "—"}, {v, "Sim"}} }

	regStates("JT_BIOLO", []StateOption{{Key: stateKey("Research Report"), Label: "Eureka", Choices: yes("1")}})

	// The generator left expressions of the helper functions as options.
	var hn []StateOption
	for _, o := range latamStates["JT_HYPER_NOVICE"] {
		if o.Key == stateKey("Angel of Magic") || o.Key == stateKey("_Preview_Target Below Half HP") {
			hn = append(hn, o)
		}
	}
	latamStates["JT_HYPER_NOVICE"] = append(hn, StateOption{Key: stateKey("Angel of Power"), Label: "Angel of Power"})

	// The generator filed Baforada Draconiana's formula ("Dragonic Breath")
	// under Sopro do Dragão; the right one is in latam_wiki_2026.go.
	delete(latamByJob["JT_DRAGON_KNIGHT"], "RK_DRAGONBREATH")

	// Cores do Arco-Íris: the element of Meteoros de Nepeta, Brisa Silvestre
	// and Lança Galhada.
	regStates("JT_SPIRIT_HANDLER", []StateOption{{Key: stateKey("Colors of Hynrok"), Label: "Cores do Arco-Íris",
		Choices: [][2]string{{"0", "—"}, {"1", "Água"}, {"2", "Vento"}, {"3", "Terra"}, {"4", "Fogo"}, {"5", "Sombrio"}, {"6", "Sagrado"}}}})
	for job, codes := range map[string][]string{"JT_DO_SUMMONER": {"SU_CN_METEOR"}, "JT_SPIRIT_HANDLER": {"SH_HYUN_ROKS_BREEZE", "SH_HYUN_ROK_CANNON"}} {
		for _, code := range codes {
			if f := latamByJob[job][code]; f != nil {
				f.ElementFn = func(in FormulaInput) string {
					return [...]string{"Neutral", "Water", "Wind", "Earth", "Fire", "Dark", "Holy"}[min(max(in.ActiveLv("Colors of Hynrok"), 0), 6)]
				}
			}
		}
	}
	// Remoção Sombria Total: the target loses 50 RES and MRES.
	regStates("JT_ABYSS_CHASER", []StateOption{{Key: stateKey("Strip Shadow"), Label: "Remoção Sombria Total no alvo", Choices: yes("5")}})
	// 10 spirit charms: 30% more physical damage on the element they beat.
	// What each charm adds to the ninja spells, and Terra's ATK, is left out.
	for _, job := range []string{"JT_KAGEROU", "JT_OBORO"} {
		regClassBuffs(job, []ClassBuff{{Name: "Spirit Charm", Label: "10 Amuletos",
			Choices: [][2]string{{"0", "—"}, {"1", "Fogo"}, {"2", "Água"}, {"3", "Vento"}, {"4", "Terra"}},
			Scripts: map[int]string{1: "bonus2 bAddEle,Ele_Earth,30;", 2: "bonus2 bAddEle,Ele_Fire,30;",
				3: "bonus2 bAddEle,Ele_Water,30;", 4: "bonus2 bAddEle,Ele_Wind,30;"}}})
	}

	// Piroclástico (Dieter) Nv 10: the master's weapon ATK + homunculus
	// level + 200 (bROWiki: 20/Nv; measured by latam-ro-calc: +431 at 231).
	pyro := ClassBuff{Name: "Pyroclastic", Skill: "MH_PYROCLASTIC", Label: "Piroclástico",
		Choices: [][2]string{{"0", "—"}}, Scripts: map[int]string{}}
	for hom := 100; hom <= 250; hom += 10 {
		s := strconv.Itoa(hom)
		pyro.Choices = append(pyro.Choices, [2]string{s, "Dieter Nv " + s})
		pyro.Scripts[hom] = "bonus bAtk," + strconv.Itoa(hom+200) + ";"
	}
	for _, job := range []string{"JT_GENETIC", "JT_GENETIC_H"} {
		regClassBuffs(job, []ClassBuff{pyro})
	}
	for _, job := range []string{"JT_ROYAL_GUARD", "JT_ROYAL_GUARD_H"} {
		regClassBuffs(job, []ClassBuff{{Name: "Shield Spell", Skill: "LG_SHIELDSPELL", Label: "Aegis Domini Nv 3", Toggle: true,
			Choices: yes("3"), Scripts: map[int]string{3: "bonus bAtk,150; bonus bMatk,150;"}}})
	}
	symphony := ClassBuff{Name: "Mystic Symphony", Skill: "TR_MYSTIC_SYMPHONY", Label: "Sinfonia Mística", Toggle: true, Choices: yes("1"),
		Scripts: map[int]string{1: "bonus2 bAddRace,RC_Fish,50; bonus2 bAddRace,RC_DemiHuman,50; " +
			"bonus2 bMagicAddRace,RC_Fish,50; bonus2 bMagicAddRace,RC_DemiHuman,50; " +
			`bonus2 bSkillAtk,"TR_ROSEBLOSSOM",100; bonus2 bSkillAtk,"TR_RHYTHMSHOOTING",100;`}}
	for _, job := range []string{"JT_TROUBADOUR", "JT_TROUVERE"} {
		regClassBuffs(job, []ClassBuff{symphony})
	}
	regClassBuffs("JT_SPIRIT_HANDLER", []ClassBuff{{Name: "Temporary Communion", Skill: "SH_TEMPORARY_COMMUNION", Label: "Aliança Animal",
		Choices: levelChoices(5), Scripts: perLevelScript(5, func(lv int) string {
			v := strconv.Itoa(lv * 3)
			return "bonus bPAtk," + v + "; bonus bSMatk," + v + "; bonus bHPlus," + v + ";"
		})}})
}

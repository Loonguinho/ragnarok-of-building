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

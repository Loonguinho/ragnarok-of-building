package calc

// The Blacksmith's own buffs, which the Meister (riding the M.E.C.H.A.)
// and every class after the Blacksmith keeps, and which neither
// latam-ro-calc nor the client's level tables give us (Força Violenta's
// table has two ATQ columns, the user's and the party's). Values from the
// LATAM client's descriptions; the rules from rAthena (db/re/status.yml,
// battle.cpp).

// maximizePower is Amplificar Poder's state: physical attacks deal the
// weapon's maximum damage (rAthena SC_MAXIMIZEPOWER: atkmin = atkmax).
const maximizePower = "Maximize Power"

// buffExcl puts buffs read from the client's level tables in an exclusive
// group of their own.
var buffExcl = map[string]string{}

// autoBuffs are support buffs a character keeps on itself: they count at
// the level learned in the skill tree, and the Buffs tab can turn them off
// to see the damage without them (ClassBuff.AutoSkill).
var autoBuffs = map[string]bool{"WS_OVERTHRUSTMAX": true}

func init() {
	on := [][2]string{{"0", "—"}, {"1", "Sim"}}
	regClassBuffs("JT_BLACKSMITH", []ClassBuff{
		// Força Violenta: the user's weapon ATK +5% per level.
		// Not with Força Violentíssima: see keepMaxOverthrust.
		{Name: "Power-Thrust", Skill: "BS_OVERTHRUST", Label: "Força Violenta",
			Choices: levelChoices(5), Scripts: map[int]string{
				1: "bonus bAtkRate,5;", 2: "bonus bAtkRate,10;", 3: "bonus bAtkRate,15;",
				4: "bonus bAtkRate,20;", 5: "bonus bAtkRate,25;"}},
		// Manejo Perfeito: no size penalty on any weapon.
		{Name: "Weapon Perfection", Skill: "BS_WEAPONPERFECT", Label: "Manejo Perfeito", Toggle: true,
			Choices: on, Scripts: map[int]string{1: "bonus bNoSizeFix;"}},
		// Amplificar Poder: the weapon's maximum damage (see maximizePower).
		{Name: maximizePower, Skill: "BS_MAXIMIZE", Label: "Amplificar Poder", Toggle: true,
			Choices: on, Scripts: map[int]string{1: ""}},
	})
}

// keepMaxOverthrust drops Força Violenta when Força Violentíssima is on
// too, as the game does (rAthena: Maxoverthrust ends Overthrust, and
// Overthrust fails under Maxoverthrust), and says so. They are not an
// exclusive group of the Buffs tab: picking Força Violenta must not turn
// the stronger one off.
func keepMaxOverthrust(states map[string]int) (note string) {
	pt := stateKey("Power-Thrust")
	if states[stateKey("Maximum Power-Thrust")] > 0 && states[pt] > 0 {
		delete(states, pt)
		return "Força Violenta não funciona com a Força Violentíssima ativa: só a Violentíssima conta."
	}
	return ""
}

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

// overthrustGroup: Força Violenta and Força Violentíssima are not on
// together (rAthena: Maxoverthrust ends Overthrust, Overthrust fails
// under Maxoverthrust).
const overthrustGroup = "overthrust"

// buffExcl puts buffs read from the client's level tables in an exclusive
// group of their own.
var buffExcl = map[string]string{"WS_OVERTHRUSTMAX": overthrustGroup}

func init() {
	on := [][2]string{{"0", "—"}, {"1", "Sim"}}
	regClassBuffs("JT_BLACKSMITH", []ClassBuff{
		// Força Violenta: the user's weapon ATK +5% per level.
		{Name: "Power-Thrust", Skill: "BS_OVERTHRUST", Label: "Força Violenta", Excl: overthrustGroup,
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
// too: in the game the second ends the first (an older build or a replay
// may have both).
func keepMaxOverthrust(states map[string]int) {
	if states[stateKey("Maximum Power-Thrust")] > 0 {
		delete(states, stateKey("Power-Thrust"))
	}
}

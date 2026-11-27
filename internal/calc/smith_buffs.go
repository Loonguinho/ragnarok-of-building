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

// buffExtra adds to a buff read from the client's level table what its
// table leaves out. Adrenalina Pura: velocidade de ataque +10%, and STD +7
// for the smith casting it (bROWiki; rAthena's renewal ASPD gives it the
// same 7), which counts like a potion's (× AGI ÷ 200).
var buffExtra = map[string]string{"BS_ADRENALINE": "bonus bAspdFixed,7; bonus bAspdRate,10;"}

// autoBuffs are support buffs a character keeps on itself: they count at
// the level learned in the skill tree, and the Buffs tab can turn them off
// to see the damage without them (ClassBuff.AutoSkill).
var autoBuffs = map[string]bool{"WS_OVERTHRUSTMAX": true}

// ratioBuffs add points to the physical attack's ratio, per level, and give
// no ATK%: rAthena (battle_calc_attack_skill_ratio) adds Força Violenta's
// and Força Violentíssima's value to skillratio, so +100 doubles a basic
// attack (the client's "ATQ +100%") but adds less than 1% to a 10.000%
// skill. An Engenheiro replay (2026-11-27) agrees: Laser Triplo with
// Força Violentíssima 5 dealt 47,8M; as ATK% +100 it came to 56,5M.
// Keyed by the skill; ratioBuffScripts drops their client scripts.
var ratioBuffs = map[string]struct {
	state string
	perLv int
}{
	"BS_OVERTHRUST":    {"Power-Thrust", 5},
	"WS_OVERTHRUSTMAX": {"Maximum Power-Thrust", 20},
}

// ratioBuffAdd is what the active ratio buffs add to a physical attack's
// ratio (Força Violenta does not count with Força Violentíssima: see
// keepMaxOverthrust).
func ratioBuffAdd(b Build) float64 {
	add := 0
	for _, rb := range ratioBuffs {
		if lv := b.Atk.States[stateKey(rb.state)]; lv > 0 {
			add += lv * rb.perLv
		}
	}
	return float64(add)
}

func init() {
	on := [][2]string{{"0", "—"}, {"1", "Sim"}}
	regClassBuffs("JT_BLACKSMITH", []ClassBuff{
		// Força Violenta: +5 per level on the attack's ratio (ratioBuffs).
		// Not with Força Violentíssima: see keepMaxOverthrust.
		{Name: "Power-Thrust", Skill: "BS_OVERTHRUST", Label: "Força Violenta",
			Choices: levelChoices(5), Scripts: map[int]string{1: "", 2: "", 3: "", 4: "", 5: ""}}, // ratioBuffs
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

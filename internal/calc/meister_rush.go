package calc

import "strconv"

// Avanço Sísmico (MT_RUSH_QUAKE) leaves two effects, per the LATAM client's
// text, and an Engenheiro replay (2026-11-27) confirmed both to the unit:
// after it, Laser Triplo and Explosão de Faíscas dealt exactly
// 1,5 × (100 + 190 + 45) / (100 + 190) times their damage before it.
//
//   - Avanço, on the user: melee and ranged physical damage +5% per level
//     (bShortAtkRate and bLongAtkRate, added to the equipment's).
//   - Sismo, on the target: it takes 50% more melee and ranged physical
//     damage, on the finished damage (like Profanação).
const (
	meisterRush   = "_Meister_Rush" // latam-ro-calc's name, kept so saved builds still load
	quakeState    = "Quake"
	quakeTakenPct = 50
)

func init() {
	// latam-ro-calc gives Avanço only at level 10 (on/off): by level here.
	// An "on" saved before (10) is still level 10.
	for i, cb := range latamClassBuffs["JT_MEISTER"] {
		if cb.Name != meisterRush {
			continue
		}
		cb.Toggle = false
		cb.Choices = levelChoices(10)
		cb.Scripts = map[int]string{}
		for lv := 1; lv <= 10; lv++ {
			p := strconv.Itoa(5 * lv)
			cb.Scripts[lv] = "bonus bLongAtkRate," + p + "; bonus bShortAtkRate," + p + ";"
		}
		latamClassBuffs["JT_MEISTER"][i] = cb
	}
	regStates("JT_MEISTER", []StateOption{{Key: stateKey(quakeState), Label: "Sismo no alvo",
		Choices: [][2]string{{"0", "—"}, {"1", "Sim"}}}})
}

// quakePctOn is the extra damage % Sismo on the target gives this attack:
// melee and ranged physical damage, not magic.
func quakePctOn(b Build, magic bool) int {
	if magic || b.Atk.States[stateKey(quakeState)] <= 0 {
		return 0
	}
	return quakeTakenPct
}

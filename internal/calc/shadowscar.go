package calc

import "strconv"

// Profanação (Profanar Arma's [Shadow Scar], SHC_ENCHANTING_SHADOW): a debuff
// on the target, up to 20 stacks, each making it take 3% more melee physical
// damage. rAthena applies it to the finished damage (battle.cpp,
// damage += damage * 3 * stacks / 100); the client text limits it to melee
// physical damage, and it is not halved against bosses (latam-ro-calc).
const (
	shadowScarState = "Shadow Wound"
	shadowScarMax   = 20
	shadowScarPct   = 3
)

func init() {
	ch := [][2]string{{"0", "—"}}
	for i := 1; i <= shadowScarMax; i++ {
		ch = append(ch, [2]string{strconv.Itoa(i), strconv.Itoa(i) + " acúmulo(s)"})
	}
	regStates("JT_SHADOW_CROSS", []StateOption{{Key: stateKey(shadowScarState), Label: "Profanação no alvo", Choices: ch}})
}

// shadowScarPctOn is the extra damage % the target's stacks give this attack.
func shadowScarPctOn(b Build, melee, magic bool) int {
	if !melee || magic {
		return 0
	}
	return min(shadowScarMax, b.Atk.States[stateKey(shadowScarState)]) * shadowScarPct
}

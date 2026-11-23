package calc

import "strconv"

// Ilimitar (RA_UNLIMIT, Ranger; the Falcão dos Ventos inherits it): ×1,5
// to ×3,5 on ranged physical damage (Nv 1-5), applied, per bROWiki, "no
// dano final dos ataques físicos a distância e cumulativo com efeitos
// semelhantes": its +50%/Nv adds to the ranged damage %, not on top of it.
// The Windhawk replay wh-ilimitar.rrf (rrfparser) gains ×3,215 on Tiro
// Preciso at Nv5, not ×3,5, which is what the sum gives.
const (
	unlimitState = "Unlimit"
	unlimitPerLv = 50
)

func init() {
	ch := [][2]string{{"0", "—"}}
	for i := 1; i <= 5; i++ {
		ch = append(ch, [2]string{strconv.Itoa(i), "Nv " + strconv.Itoa(i) + " (×" + strconv.FormatFloat(1+0.5*float64(i), 'f', 1, 64) + ")"})
	}
	for _, job := range []string{"JT_RANGER", "JT_RANGER_H"} {
		regStates(job, []StateOption{{Key: stateKey(unlimitState), Label: "Ilimitar", Choices: ch}})
	}
}

// unlimitRate is the ranged damage % Ilimitar adds to this attack.
func unlimitRate(b Build, ranged, magic bool) int {
	if !ranged || magic {
		return 0
	}
	return unlimitPerLv * min(max(b.Atk.States[stateKey(unlimitState)], 0), 5)
}

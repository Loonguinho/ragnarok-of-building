package calc

import (
	"math"
	"strconv"

	"ragnarok-of-building/internal/gamedata"
)

// Hand-written class rules of latam-ro-calc (MIT) that are code, not data,
// so the generators do not carry them over.

func init() {
	// Star Gladiator's Wrath toggles (Fúria Solar/Lunar/Estelar Nv 3). They
	// only pay through the Star Emperor ATK adjustment below.
	regStates("JT_STAR_EMPEROR", []StateOption{
		{Key: stateKey("Wrath of Sun"), Label: "Fúria Solar", Choices: [][2]string{{"0", "-"}, {"3", "Nv 3"}}},
		{Key: stateKey("Wrath of Moon"), Label: "Fúria Lunar", Choices: [][2]string{{"0", "-"}, {"3", "Nv 3"}}},
		{Key: stateKey("Wrath of"), Label: "Fúria Estelar", Choices: [][2]string{{"0", "-"}, {"3", "Nv 3"}}},
	})
}

// wrathByAlignment: Oposição Solar, Lunar e Estelar aligns a monster by
// size, so only one Wrath reaches it (StarEmperor.WRATH_BY_ALIGNMENT).
var wrathByAlignment = []struct {
	state, size, label string
	minHP              int
	withStr            bool
}{
	{"Wrath of Sun", "Small", "Fúria Solar", 0, false},
	{"Wrath of Moon", "Medium", "Fúria Lunar", 6000, false},
	{"Wrath of", "Large", "Fúria Estelar", 20000, true},
}

func inChain(c *gamedata.Catalog, jobID int, code string) bool {
	for _, id := range c.Chain(jobID) {
		if j := c.Jobs[id]; j != nil && j.Code == code {
			return true
		}
	}
	return false
}

// classFinalAtk is latam-ro-calc's modifyFinalAtk: the class's last ATK
// multipliers, applied to the ATK after P.ATK and mastery, before the
// damage modifiers. Only the Star Emperor line has one: Kihop (learned
// level, +15%/lv +10%) and the Wrath that matches the target's size,
// +(base level + LUK + DEX [+ STR for Star]) / 3 %.
func classFinalAtk(b Build, r Result, c *gamedata.Catalog, m *gamedata.Monster, atk int64, step func(label, format string, args ...any)) int64 {
	if !inChain(c, b.Job, "JT_STAR_EMPEROR") {
		return atk
	}
	if lv := learnedLv(b, c, "TK_POWER"); lv > 0 {
		pct := int64(lv*15 + 10)
		atk += atk * pct / 100
		step("Kihop Nv "+strconv.Itoa(lv), "%+d%% = %d", pct, atk)
	}
	for _, w := range wrathByAlignment {
		if b.Atk.States[stateKey(w.state)] == 0 || m.Size != w.size || m.HP < w.minHP {
			continue
		}
		sum := b.BaseLv + r.st[LUK] + r.st[DEX]
		if w.withStr {
			sum += r.st[STR]
		}
		bonus := sum / 3
		atk = int64(math.Floor(float64(atk) * float64(100+bonus) / 100))
		step(w.label, "%+d%% = %d", bonus, atk)
		break
	}
	return atk
}

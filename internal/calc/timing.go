package calc

import (
	"math"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// Timing is how often the attack can be used: its times after the
// build's reductions, the time from one use to the next, and what sets
// that pace.
type Timing struct {
	CastMs, FixedMs int // variable and fixed cast
	AfterMs         int // after-cast delay
	CooldownMs      int
	MotionMs        int // the attack motion of the build's ASPD
	CycleMs         int // from one use to the next
	// Limit sets the pace: "cast", "after", "cooldown" or "aspd".
	Limit  string
	PerSec float64 // uses per second
	SP     int     // SP per use (0 for a basic attack)
}

// motionMs is the time between basic attacks at an ASPD (renewal: 50 ÷
// (200 − ASPD) hits a second); skills wait for it too.
func motionMs(aspd int) int {
	return max(1, (200-min(aspd, 199))*20)
}

// AttackTiming works out the attack's pace: a basic attack every motion;
// a skill every cast + the longest of its after-cast delay, its cooldown
// and the motion. The times come from rAthena's skill_db (Skill.Times),
// with the build's cast, delay and cooldown bonuses.
func AttackTiming(r Result, c *gamedata.Catalog, skillID, lv int) Timing {
	t := Timing{MotionMs: motionMs(r.ASPD)}
	sk := c.Skills[skillID]
	if sk == nil || skillID == 0 {
		t.CycleMs, t.Limit = t.MotionMs, "aspd"
		t.PerSec = 1000 / float64(t.CycleMs)
		return t
	}
	cast, fixed, after, cooldown := sk.Times.At(lv)
	perSkill := func(name string) int {
		n := 0
		for k, v := range r.Extra {
			parts := strings.Fields(k)
			if len(parts) == 2 && parts[0] == name && strings.EqualFold(strings.Trim(parts[1], `"`), sk.Code) {
				n += v
			}
		}
		return n
	}
	ci := r.Cast()
	t.CastMs = int(math.Round(float64(cast) * ci.VarLeft / 100))
	t.FixedMs = max(0, int(math.Round(float64(fixed)*(1+float64(ci.FixedRate)/100)))+ci.FixedMs+perSkill("bSkillFixedCast"))
	t.AfterMs = max(0, int(math.Round(float64(after)*(1+float64(r.Bonuses["bDelayrate"])/100))))
	t.CooldownMs = max(0, cooldown+perSkill("bSkillCooldown"))
	wait, limit := t.MotionMs, "aspd"
	if t.AfterMs > wait {
		wait, limit = t.AfterMs, "after"
	}
	if t.CooldownMs > wait {
		wait, limit = t.CooldownMs, "cooldown"
	}
	if t.CastMs+t.FixedMs > wait {
		limit = "cast"
	}
	t.CycleMs = max(1, t.CastMs+t.FixedMs+wait)
	t.Limit = limit
	t.PerSec = 1000 / float64(t.CycleMs)
	if len(sk.SP) > 0 {
		t.SP = sk.SP[min(max(lv, 1), len(sk.SP))-1]
	}
	return t
}

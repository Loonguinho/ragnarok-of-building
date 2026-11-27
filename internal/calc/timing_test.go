package calc

import (
	"math"
	"path/filepath"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

// TestAttackTiming: a basic attack goes 50 ÷ (200 − ASPD) times a second;
// a skill every cast + the longest of its delay, cooldown and the ASPD's
// wait (Laser Triplo Nv 5: rAthena 1 s + 0,5 s fixed cast, 0,25 s delay,
// 0,35 s cooldown).
func TestAttackTiming(t *testing.T) {
	c, err := gamedata.Load(filepath.Join("..", "..", "data", "gen", "catalog.json.gz"))
	if err != nil {
		t.Skip("catalog not generated")
	}
	b := Build{Job: 4253, BaseLv: 200, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1}, Skills: map[int]int{6003: 5}}
	Normalize(&b, c)
	r := Compute(b, c)

	basic := AttackTiming(r, c, 0, 0)
	if want := 50 / float64(200-r.ASPD); math.Abs(basic.PerSec-want) > 0.01 || basic.Limit != "aspd" {
		t.Errorf("basic attack: %.3f/s (%s), want %.3f/s at ASPD %d", basic.PerSec, basic.Limit, want, r.ASPD)
	}

	tl := AttackTiming(r, c, 6003, 5)
	if tl.CooldownMs != 350 || tl.AfterMs != 250 || tl.SP != 60 {
		t.Errorf("Laser Triplo: cooldown %d, delay %d, SP %d; want 350, 250, 60", tl.CooldownMs, tl.AfterMs, tl.SP)
	}
	wait := max(tl.AfterMs, tl.CooldownMs, tl.MotionMs)
	if tl.CycleMs != tl.CastMs+tl.FixedMs+wait {
		t.Errorf("cycle %d, want cast %d + fixed %d + %d", tl.CycleMs, tl.CastMs, tl.FixedMs, wait)
	}
	if tl.FixedMs != 500 || tl.Limit != "cast" { // nothing cuts the cast at 1 DEX
		t.Errorf("fixed cast %d, limit %s; want 500, cast", tl.FixedMs, tl.Limit)
	}
}

package view

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/replay"
)

// TestReplayTarget: an imported replay targets the monster the character
// damaged the most; without one, the neutral dummy.
func TestReplayTarget(t *testing.T) {
	c := &gamedata.Catalog{Monsters: map[int]*gamedata.Monster{
		21067: {ID: 21067}, 21542: {ID: 21542}, 21539: {ID: 21539},
	}}
	snap := &replay.Snapshot{Damage: []*replay.DamageSummary{
		{Self: false, Monsters: []replay.MonsterDamage{{Mob: 21539}}},
		{Self: true, Monsters: []replay.MonsterDamage{{Mob: 99999}, {Mob: 21542}, {Mob: 21539}}},
	}}
	if got := replayTarget(snap, c); got != 21542 {
		t.Errorf("target %d, want 21542 (most damaged known monster)", got)
	}
	if got := replayTarget(&replay.Snapshot{}, c); got != 21067 {
		t.Errorf("no damage: target %d, want the neutral dummy", got)
	}
}

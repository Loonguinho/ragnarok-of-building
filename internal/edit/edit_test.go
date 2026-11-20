package edit

import (
	"testing"

	"ragnarok-of-building/internal/calc"
)

func TestUnequipLinkedHeadgear(t *testing.T) {
	b := calc.Build{Equip: map[string]calc.Equip{
		calc.HeadTop: {Item: 5001}, calc.HeadMid: {Item: 5001}, calc.HeadLow: {Item: 5002},
		calc.Armor: {Item: 2301},
	}}
	nb := Unequip(b, calc.HeadMid)
	if _, ok := nb.Equip[calc.HeadTop]; ok {
		t.Error("the top slot held the same headgear: it should go too")
	}
	if nb.Equip[calc.HeadLow].Item != 5002 || nb.Equip[calc.Armor].Item != 2301 {
		t.Errorf("other slots changed: %v", nb.Equip)
	}
	if len(b.Equip) != 4 {
		t.Error("the original build was changed")
	}
}

func TestEditLinked(t *testing.T) {
	b := calc.Build{Equip: map[string]calc.Equip{calc.CosTop: {Item: 9}, calc.CosMid: {Item: 9}, calc.HeadTop: {Item: 9}}}
	nb := EditLinked(b, calc.CosTop, func(e *calc.Equip) { e.Cards[0] = 4001 })
	if nb.Equip[calc.CosMid].Cards[0] != 4001 || nb.Equip[calc.HeadTop].Cards[0] != 0 {
		t.Errorf("costume heads should stay in sync, not the real head: %v", nb.Equip)
	}
}

func TestExclusiveBuffs(t *testing.T) {
	g := []string{"use:645", "use:656", "use:657"}
	b := SetBuff(calc.Build{}, "use:645", 1, g)
	b = SetBuff(b, "use:657", 1, g)
	if b.Buffs["use:645"] != 0 || b.Buffs["use:657"] != 1 {
		t.Errorf("the newest potion wins: %v", b.Buffs)
	}
	b = SetBuff(b, "use:657", 0, g)
	if len(b.Buffs) != 0 {
		t.Errorf("turned off: %v", b.Buffs)
	}
	s := SetState(calc.Build{}, "a", 2, []string{"a", "b"})
	s = SetState(s, "b", 1, []string{"a", "b"})
	if s.Atk.States["a"] != 0 || s.Atk.States["b"] != 1 {
		t.Errorf("states: %v", s.Atk.States)
	}
}

func TestSkillTree(t *testing.T) {
	// 1 (max 10) <- 2 needs 1@5 <- 3 needs 2@3; 4 is free (quest).
	tr := Tree{
		Skills: map[int]Skill{
			1: {Max: 10, Group: 0},
			2: {Max: 5, Needs: [][2]int{{1, 5}}, Group: 1},
			3: {Max: 10, Needs: [][2]int{{2, 3}}, Group: 1},
			4: {Max: 1, Group: 1, Free: true},
		},
		Budgets: []int{9, 10},
	}
	lv, ok := tr.Raise(nil, 3, 1)
	if !ok || lv[1] != 5 || lv[2] != 3 || lv[3] != 1 {
		t.Fatalf("raise with prerequisites: %v %v", lv, ok)
	}
	// Step 1 has 9 points; the second step may spend on step 1's skills.
	lv, ok = tr.Max(lv, 1)
	if !ok || lv[1] != 10 {
		t.Errorf("max: %v", lv)
	}
	if _, ok := tr.Raise(lv, 3, 10); ok {
		t.Error("19 points: 3@10 would need more")
	}
	if lv, ok = tr.Raise(lv, 4, 1); !ok || lv[4] != 1 {
		t.Error("free skills cost nothing")
	}
	lv = tr.Lower(lv, 1, 4)
	if lv[2] != 0 || lv[3] != 0 || lv[1] != 4 {
		t.Errorf("lowering resets dependents: %v", lv)
	}
	if !tr.Locked(lv, 2) {
		t.Error("2 needs 1@5")
	}
	if !(Tree{Budgets: []int{-1}}).Fits(map[int]int{1: 99}) {
		t.Error("unknown budget: anything fits")
	}
}

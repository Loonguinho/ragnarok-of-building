package view

import (
	"testing"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

func TestHeadViews(t *testing.T) {
	c := &gamedata.Catalog{Items: map[int]*gamedata.Item{
		1: {ID: 1, ClassNum: 10}, // top + middle headgear
		2: {ID: 2, ClassNum: 20}, // lower
		3: {ID: 3, ClassNum: 30}, // costume top
	}}
	v := testViews(t, c)
	b := calc.Build{Equip: map[string]calc.Equip{
		calc.HeadTop: {Item: 1}, calc.HeadMid: {Item: 1}, calc.HeadLow: {Item: 2}, calc.CosTop: {Item: 3},
	}}
	if got := v.HeadViews(b, true); got != [3]int{30, 10, 20} {
		t.Errorf("with costumes %v, want [30 10 20]", got)
	}
	if got := v.HeadViews(b, false); got != [3]int{10, 10, 20} {
		t.Errorf("without costumes %v, want [10 10 20]", got)
	}
}

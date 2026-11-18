package clientimport

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/rathena"
)

func TestMarkMVPCards(t *testing.T) {
	c := &gamedata.Catalog{Items: map[int]*gamedata.Item{4147: {ID: 4147}, 4074: {ID: 4074}, 4001: {ID: 4001}}}
	db := &rathena.DB{Mobs: map[int]*rathena.Mob{
		1039: {Aegis: "BAPHOMET", MVP: true, Class: "Boss", Cards: []int{4147}},
		2000: {Aegis: "BAPHOMET_I", Class: "Boss", Cards: []int{4147}}, // instance copy: still an MVP card
		1066: {Aegis: "E_EDDGA", MVP: true, Class: "Boss", Cards: []int{4074}}, // event MVP dropping a normal card
		1060: {Aegis: "BIGFOOT", Class: "Normal", Cards: []int{4074}},
		1002: {Aegis: "PORING", Class: "Normal", Cards: []int{4001}},
	}}
	if n := markMVPCards(c, db); n != 1 || !c.Items[4147].MVPCard || c.Items[4074].MVPCard || c.Items[4001].MVPCard {
		t.Errorf("marked %d: baphomet %v bigfoot %v poring %v", n, c.Items[4147].MVPCard, c.Items[4074].MVPCard, c.Items[4001].MVPCard)
	}
}

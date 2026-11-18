package gamedata

import "testing"

func TestLatamLimits(t *testing.T) {
	c := &Catalog{Jobs: map[int]*Job{
		4255: {ID: 4255, MaxBaseLv: 275, MaxJobLv: 60, Fourth: true}, // Magus
		4055: {ID: 4055, MaxBaseLv: 200, MaxJobLv: 70},               // Arcano
		4012: {ID: 4012, MaxBaseLv: 99, MaxJobLv: 70},                // trans
	}}
	c.ApplyLatamLimits()
	if j := c.Jobs[4255]; j.MaxBaseLv != 250 || j.MaxJobLv != 50 {
		t.Errorf("4th class: %d/%d, want 250/50", j.MaxBaseLv, j.MaxJobLv)
	}
	if j := c.Jobs[4055]; j.MaxBaseLv != 200 || j.MaxJobLv != 70 {
		t.Errorf("3rd class changed: %d/%d", j.MaxBaseLv, j.MaxJobLv)
	}
	if j := c.Jobs[4012]; j.MaxBaseLv != 99 || j.MaxJobLv != 70 {
		t.Errorf("trans class changed: %d/%d", j.MaxBaseLv, j.MaxJobLv)
	}
}

package calc

import (
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func TestConsumables(t *testing.T) {
	c := &gamedata.Catalog{
		Jobs: map[int]*gamedata.Job{1: {ID: 1, Code: "JT_SWORDMAN", Parent: -1}},
		Items: map[int]*gamedata.Item{
			12418:           {ID: 12418, Name: gamedata.Text{"pt": "Poção do Furor Físico"}, LatamUse: map[string][]string{"atk": {"50"}}},
			battlePill:      {ID: battlePill, LatamUse: map[string][]string{"p_class_all": {"5"}}},
			superBattlePill: {ID: superBattlePill, LatamUse: map[string][]string{"p_class_all": {"10"}}},
			spPotionL:       {ID: spPotionL, LatamUse: map[string][]string{"spPercent": {"5"}}},
			hpPotionL:       {ID: hpPotionL},
		},
	}
	if n := len(Consumables(c)); n != 5 {
		t.Errorf("consumables offered = %d, want 5", n)
	}
	b := Build{Job: 1, BaseLv: 200, Buffs: map[string]int{
		ConsumableKey(12418): 1, ConsumableKey(battlePill): 1, ConsumableKey(superBattlePill): 1,
		ConsumableKey(spPotionL): 1, ConsumableKey(hpPotionL): 1, "food_str": 35,
	}}
	normalizeBuffs(&b)
	bonus, extra := map[string]int{}, map[string]int{}
	applyConsumables(&b, c, newLatamEnv(b, c), bonus, extra)
	if bonus["bBaseAtk"] != 50 {
		t.Errorf("Poção do Furor Físico: ATK %d", bonus["bBaseAtk"])
	}
	if extra["bAddClass Class_All"] != 10 {
		t.Errorf("battle pills: %d, want only the superior one (10)", extra["bAddClass Class_All"])
	}
	if bonus["bMaxSPrate"] != 25 || bonus["bMaxHP"] != 2500+666 {
		t.Errorf("large potions: SP%% %d (want 25), HP %d (want 3166)", bonus["bMaxSPrate"], bonus["bMaxHP"])
	}
	if b.Buffs["food_str"] != 35 {
		t.Errorf("stick + dish should stay: %v", b.Buffs)
	}
}

func TestASPDConsumables(t *testing.T) {
	c := &gamedata.Catalog{
		Jobs: map[int]*gamedata.Job{1: {ID: 1, Code: "JT_SWORDMAN", Parent: -1}},
		Items: map[int]*gamedata.Item{
			guaranaCandy: {ID: guaranaCandy}, salamineJuice: {ID: salamineJuice}, goldPotion: {ID: goldPotion},
			645: {ID: 645}, 656: {ID: 656}, 657: {ID: 657},
		},
	}
	if n := len(Consumables(c)); n != 6 {
		t.Errorf("consumables offered = %d, want 6", n)
	}
	// The old selector value becomes the potion's consumable.
	old := Build{Buffs: map[string]int{"aspd_potion": 20}}
	normalizeBuffs(&old)
	if old.Buffs[ConsumableKey(657)] != 1 || len(old.Buffs) != 1 {
		t.Errorf("aspd_potion 20 migrated to %v, want use:657", old.Buffs)
	}
	run := func(buffs map[string]int) map[string]int {
		b := Build{Job: 1, BaseLv: 200, Buffs: buffs}
		bonus, extra := map[string]int{}, map[string]int{}
		applyConsumables(&b, c, newLatamEnv(b, c), bonus, extra)
		return bonus
	}
	all := map[string]int{ConsumableKey(guaranaCandy): 1, ConsumableKey(salamineJuice): 1, ConsumableKey(goldPotion): 1}
	if g := run(all); g["bAspdFixed"] != 3+4 || g["bAspdRate"] != 10+5 || g["bAgi"] != 7 {
		t.Errorf("guarana + juice + gold: %v", g)
	}
	// A chosen ASPD potion and Aumentar Agilidade Nv 5+ outrank the candy.
	all[ConsumableKey(656)], all[ConsumableKey(645)], all["incagi"] = 1, 1, 10
	if g := run(all); g["bAspdFixed"] != 3+6 || g["bAspdRate"] != 10 || g["bAgi"] != 0 {
		t.Errorf("candy outranked: %v", g)
	}
}

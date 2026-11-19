package calc

import (
	"slices"
	"testing"

	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/script"
)

func latamItemCatalog() *gamedata.Catalog {
	return &gamedata.Catalog{
		Jobs: map[int]*gamedata.Job{
			4302: {ID: 4302, Code: "JT_SKY_EMPEROR", Parent: 4239},
			4239: {ID: 4239, Code: "JT_STAR_EMPEROR", Parent: 4047},
			4047: {ID: 4047, Code: "JT_STAR", Parent: -1},
		},
		Skills: map[int]*gamedata.Skill{
			422:  {ID: 422, Code: "TK_HPTIME", Name: gamedata.Text{"en": "Peaceful Break"}},
			5469: {ID: 5469, Code: "SKE_MIDNIGHT_KICK", Name: gamedata.Text{"en": "Midnight Kick"}},
		},
		Items: map[int]*gamedata.Item{
			1: {ID: 1, Type: "Armor", LatamName: "Suit"},
			2: {ID: 2, Type: "Armor", LatamName: "Partner"},
			3: {ID: 3, Type: "Weapon", SubType: "Book", WeaponLv: 5, LatamName: "Board"},
		},
	}
}

func TestLatamLineRules(t *testing.T) {
	c := latamItemCatalog()
	b := Build{Job: 4302, BaseLv: 250, JobLv: 50, Stats: [6]int{120, 1, 100, 54, 120, 120},
		Skills: map[int]int{422: 10},
		Equip:  map[string]Equip{Armor: {Item: 1, Refine: 11}, Weapon: {Item: 3, Refine: 9}, Shoes: {Item: 2, Refine: 7, Grade: 2}}}
	e := newLatamEnv(b, c)
	e.today = "2026-11-19"
	for _, tc := range []struct {
		line string
		want float64
	}{
		{"130", 130},
		{"2---15", 75},               // every 2 refines (11)
		{"7===15", 15},               // from +7
		{"12===15", 0},               // not yet +12
		{"EQUIP_ID[2]50", 50},        // set by id
		{"EQUIP_ID[9]50", 0},         // partner missing
		{"EQUIP[Partner]===50", 50},  // set by name
		{"EQUIP[Partner&&Nope]5", 0}, // all required
		{"EQUIP[Nope||Partner]5", 5}, // any of
		{"SUM[str==15]---4", 32},     // base STR 120
		{"SUM[level==200]EQUIP_ID[2]SUM[str==40]---15", 45},
		{"LEARN_SKILL[Peaceful Break==1]---2", 20},
		{"SKILL_ID[422==5]1", 1},
		{"USED[StarEmperor]20", 20},
		{"USED[Swordman]20", 0},
		{"USED[Taekwondo||Ninja]3", 0}, // the Star line is not Taekwon here
		{"WEAPON_TYPE[book]5", 5},
		{"WEAPON_TYPE[bow]5", 0},
		{"UNTIL[2026-11-22]===10", 10},
		{"UNTIL[2026-11-18]===10", 0},
		{"REFINE[boot==7]===3", 3},          // another position's refine
		{"REFINE[weapon,armor==2]---1", 10}, // (9+11)/2
		{"GRADE[boot==C]===7", 7},           // the boots' grade (C)
		{"GRADE[boot==A]===7", 0},
		{"GRADE[me==D]===7", 0}, // the armor has no grade
		{"LEVEL[1-129]5", 0},
		{"level:10(1-125)---1", 12},
		{"REFINE_FROM[5]---1", 7},
		{"ACTIVE_SKILL[Anything]===50", 0},
		{"MONSTER_ID[20801]10", 0},
		{"50(90 seconds)", 0}, // a bare value with text is NaN
		{"===50(90 seconds)", 50},
	} {
		if got := e.line("armor", true, 11, tc.line); got != tc.want {
			t.Errorf("%q = %v, want %v", tc.line, got, tc.want)
		}
	}
	if got := e.line("boot", true, 7, "GRADE[me==C]===7"); got != 7 {
		t.Errorf("GRADE[me==C] on a C boot = %v, want 7", got)
	}
}

func TestLatamKeysOverRathena(t *testing.T) {
	c := latamItemCatalog()
	it := c.Items[1]
	it.Refineable = true
	// rAthena (kRO): ATK per 3 refines and HP +10%; LATAM: boss damage.
	it.Script = "bonus bBaseAtk,15*(getrefine()/3); bonus bMaxHPrate,10; bonus bAtkEle,Ele_Fire;"
	it.Latam = map[string][]string{"p_class_boss": {"5", "2---2"}, "atk": {"7===20"}}
	desc := func(int, *gamedata.Item) (string, int) {
		// The description agrees with LATAM on the boss damage and on no HP%,
		// and with rAthena on the ATK.
		return "bonus2 bAddClass,Class_Boss,5+2*(getrefine()/2); bonus bBaseAtk,15*(getrefine()/3);", 0
	}
	if n := DecideLatamKeys(c, desc); n != 1 {
		t.Fatalf("items deciding for LATAM = %d, want 1", n)
	}
	if want := []string{"bAddClass Class_Boss", "bMaxHPrate"}; !slices.Equal(it.LatamKeys, want) {
		t.Fatalf("LatamKeys = %v, want %v", it.LatamKeys, want)
	}
	b := Build{Job: 4302, BaseLv: 250, Equip: map[string]Equip{Armor: {Item: 1, Refine: 9}}}
	res := script.Eval(it.Script, script.Context{Refine: 9})
	newLatamEnv(b, c).overLatam(it, 1, Armor, true, 9, &res)
	if res.Bonus["bBaseAtk"] != 45 || res.Bonus["bMaxHPrate"] != 0 || res.Extra["bAddClass Class_Boss"] != 13 || res.Extra["bAtkEle Ele_Fire"] != 1 {
		t.Errorf("after LATAM: %v %v", res.Bonus, res.Extra)
	}
}

func TestLatamFillsDescriptionOnlyItems(t *testing.T) {
	c := latamItemCatalog()
	it := c.Items[2]
	it.DescScript = "bonus bMaxHP,100;"
	it.Latam = map[string][]string{"hp": {"300"}, "melee": {"UNTIL[2999-01-01]===10"}}
	DecideLatamKeys(c, func(_ int, it *gamedata.Item) (string, int) { return it.DescScript, 0 })
	if want := []string{"bShortAtkRate"}; !slices.Equal(it.LatamKeys, want) {
		t.Errorf("LatamKeys = %v, want %v (the description's HP stays)", it.LatamKeys, want)
	}
}

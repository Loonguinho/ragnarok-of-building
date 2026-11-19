package script

import "testing"

func TestEval(t *testing.T) {
	ctx := Context{Refine: 9, BaseLv: 200, JobLv: 50, Param: map[string]int{"bStr": 120}}
	cases := []struct {
		name    string
		src     string
		want    map[string]int
		skipped int
	}{
		{"simple", "bonus bLuk,2;\nbonus bFlee2,1;", map[string]int{"bLuk": 2, "bFlee2": 1}, 0},
		{"negative", "bonus bMaxHPrate,-10;", map[string]int{"bMaxHPrate": -10}, 0},
		{"refine expr", "bonus bAtk,getrefine()*10;", map[string]int{"bAtk": 90}, 0},
		{"refine steps", "bonus bAtkRate,(getrefine()/3)*2;", map[string]int{"bAtkRate": 6}, 0},
		{"var", ".@r = getrefine();\nbonus bMatk,.@r*5;", map[string]int{"bMatk": 45}, 0},
		{"if true", "if (getrefine() >= 7) {\n bonus bStr,3;\n bonus bDex,1;\n}", map[string]int{"bStr": 3, "bDex": 1}, 0},
		{"if false", "if (getrefine()>=10) bonus bStr,3;", map[string]int{}, 0},
		{"else", "if (getrefine()>=10) { bonus bStr,3; } else { bonus bAgi,2; }", map[string]int{"bAgi": 2}, 0},
		{"readparam", "if (readparam(bStr)>=120) bonus bBaseAtk,50;", map[string]int{"bBaseAtk": 50}, 0},
		{"baselevel", "bonus bMaxHP,BaseLevel*10;", map[string]int{"bMaxHP": 2000}, 0},
		{"ternary", "bonus bCritical,(getrefine()>8?5:1);", map[string]int{"bCritical": 5}, 0},
		{"bonus2 recorded as extra", "bonus2 bAddRace,RC_DemiHuman,10;\nbonus bStr,1;", map[string]int{"bStr": 1}, 0},
		{"autobonus skipped", "autobonus \"{ bonus bAtk,50; }\",10,5000;", map[string]int{}, 1},
		{"unknown func in cond", "if (getskilllv(\"KN_TWOHANDQUICKEN\")) bonus bAtk,10;", map[string]int{}, 1},
		{"unknown bonus", "bonus bNoWalkDelay;", map[string]int{}, 1},
		{"comments", "// note\nbonus bVit,4; /* x */", map[string]int{"bVit": 4}, 0},
		{"nested", "if (getrefine()>=5) { if (getrefine()>=9) { bonus bInt,2; } bonus bInt,1; }", map[string]int{"bInt": 3}, 0},
	}
	for _, c := range cases {
		r := Eval(c.src, ctx)
		if r.Skipped != c.skipped {
			t.Errorf("%s: skipped %d, want %d", c.name, r.Skipped, c.skipped)
		}
		if len(r.Bonus) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, r.Bonus, c.want)
			continue
		}
		for k, v := range c.want {
			if r.Bonus[k] != v {
				t.Errorf("%s: %s = %d, want %d", c.name, k, r.Bonus[k], v)
			}
		}
	}
}

func TestExtra(t *testing.T) {
	r := Eval(`bonus2 bAddRace,RC_DemiHuman,getrefine()*2;
bonus2 bSkillAtk,"KN_BOWLINGBASH",15;
bonus3 bAddEle,Ele_Fire,BF_WEAPON,5;
bonus2 bAddRace,RC_DemiHuman,2;
bonus2 bHPDrainRate,10+getrefine(),3;
bonus2 bSubRace,getskill(1),5;`, Context{Refine: 4})
	want := map[string]int{
		"bAddRace RC_DemiHuman":      10, // 4*2 + 2, accumulated
		"bSkillAtk KN_BOWLINGBASH":   15,
		"bAddEle Ele_Fire BF_WEAPON": 5,
		"bHPDrainRate 14":            3, // numeric argument expressions are evaluated
	}
	for k, v := range want {
		if r.Extra[k] != v {
			t.Errorf("%s = %d, want %d (all: %v)", k, r.Extra[k], v, r.Extra)
		}
	}
	if len(r.Extra) != len(want) || r.Skipped != 1 {
		t.Errorf("extra %v skipped %d; want %d entries and 1 skipped (unknown function argument)", r.Extra, r.Skipped, len(want))
	}
}

func FuzzEval(f *testing.F) {
	f.Add("bonus bStr,1;")
	f.Add("if ((getrefine()>=7) { bonus bAtk,")
	f.Add("}}}{{{;;;.@a /= 0;")
	f.Fuzz(func(t *testing.T, src string) {
		Eval(src, Context{Refine: 4}) // must not panic or hang
	})
}

func TestAtkEle(t *testing.T) {
	r := Eval("bonus bAtkEle,Ele_Fire;\nbonus bStr,1;", Context{})
	if r.Extra["bAtkEle Ele_Fire"] != 1 || r.Bonus["bStr"] != 1 || r.Skipped != 0 {
		t.Errorf("got %+v", r)
	}
}

// getequiprefinerycnt / getenchantgrade(EQI_x) read another position.
func TestOtherPositions(t *testing.T) {
	ctx := Context{Refine: 3, Grade: 1, SlotRefine: map[string]int{"EQI_HAND_R": 11, "EQI_HEAD_TOP": 0}, SlotGrade: map[string]int{"EQI_HEAD_TOP": 2}}
	r := Eval(`.@r = getequiprefinerycnt(EQI_HAND_R); bonus bMaxHPrate,2*(.@r/2); if (getenchantgrade(EQI_HEAD_TOP) >= 2) bonus bPAtk,1; bonus bStr,getrefine();`, ctx)
	if r.Bonus["bMaxHPrate"] != 10 || r.Bonus["bPAtk"] != 1 || r.Bonus["bStr"] != 3 || r.Skipped != 0 {
		t.Errorf("%v (skipped %d)", r.Bonus, r.Skipped)
	}
}

func TestIgnoreDefRace(t *testing.T) {
	r := Eval(`bonus bIgnoreDefRace,RC_DemiHuman; bonus bIgnoreDefClass,Class_Boss;`, Context{})
	if r.Extra["bIgnoreDefRaceRate RC_DemiHuman"] != 100 || r.Extra["bIgnoreDefClassRate Class_Boss"] != 100 || r.Skipped != 0 {
		t.Errorf("%v (skipped %d)", r.Extra, r.Skipped)
	}
}

// Class checks of item scripts: eaclass(), BaseJob, Upper, getskilllv, min/max.
func TestClassAndSkillFunctions(t *testing.T) {
	ctx := Context{EAClass: 7 | EAJLThird | EAJLUpper, BaseJob: 7, BaseClass: 1, Upper: 1, Skills: map[string]int{"SM_BASH": 10}}
	r := Eval(`if (eaclass()&EAJL_THIRD && BaseJob == Job_Knight) bonus bStr,1;
if ((eaclass()&EAJ_UPPERMASK) == EAJ_KNIGHT) bonus bAgi,1;
if ((eaclass()&EAJ_THIRDMASK) == EAJ_RUNE_KNIGHT) bonus bVit,1;
if (eaclass()&EAJL_FOURTH) bonus bInt,1;
if (getskilllv("SM_BASH") == 10) bonus bDex,min(5,getskilllv("SM_BASH"));
if (Upper == 1 && BaseClass == Job_Swordman) bonus bLuk,max(1,2);`, ctx)
	want := map[string]int{"bStr": 1, "bAgi": 1, "bVit": 1, "bDex": 5, "bLuk": 2}
	for k, v := range want {
		if r.Bonus[k] != v {
			t.Errorf("%s = %d, want %d (%v)", k, r.Bonus[k], v, r.Bonus)
		}
	}
	if r.Bonus["bInt"] != 0 || r.Skipped != 0 {
		t.Errorf("bInt %d, skipped %d", r.Bonus["bInt"], r.Skipped)
	}
}

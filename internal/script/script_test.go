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

package clientimport

import (
	"strings"
	"testing"

	"ragnarok-of-building/internal/script"
)

func evalDesc(t *testing.T, desc []string, names map[string][]int, refine, baseLv int) (script.Result, []DescCombo, int) {
	t.Helper()
	s, combos, skipped := DescScript(1, desc, names, map[string]string{"vulcão de flechas": "CG_ARROWVULCAN"})
	return script.Eval(s, script.Context{Refine: refine, BaseLv: baseLv}), combos, skipped
}

func TestDescRefineSteps(t *testing.T) {
	desc := strings.Split(`Capa feita de couro e plantas encontradas em uma Gruta desconhecida de Mjolnir.
--------------------------
A cada 2 refinos:
ATQ +10.
Dano físico +2%.
A cada 4 refinos:
Dano físico contra todos os tamanhos +5%.
--------------------------
Refino +7 ou mais:
Precisão perfeita +30.
Refino +9 ou mais:
Ignora 20% da DEF e DEFM das raças Amorfo e Humanoide.
--------------------------
Conjunto
[Bota Desconhecida FOR]
ATQ +50. POD +7.
Bota com refino +10 ou mais:
Conjuração fixa -0,5 segundos.
--------------------------
Tipo: Capa
DEF: 40 DEFM: 0
Peso: 50`, "\n")
	names := map[string][]int{"bota desconhecida for": {2}}
	r, combos, _ := evalDesc(t, desc, names, 9, 200)
	if r.Bonus["bBaseAtk"] != 40 || r.Bonus["bAtkRate"] != 8 || r.Extra["bAddSize Size_All"] != 10 {
		t.Errorf("refine steps at +9: %v %v", r.Bonus, r.Extra)
	}
	if r.Bonus["bPerfectHitAddRate"] != 30 || r.Extra["bIgnoreDefRaceRate RC_Formless"] != 20 || r.Extra["bIgnoreMdefRaceRate RC_DemiHuman"] != 20 {
		t.Errorf("refine thresholds at +9: %v %v", r.Bonus, r.Extra)
	}
	r, _, _ = evalDesc(t, desc, names, 6, 200)
	if r.Bonus["bPerfectHitAddRate"] != 0 || r.Bonus["bBaseAtk"] != 30 {
		t.Errorf("+6: %v", r.Bonus)
	}
	if len(combos) != 1 || combos[0].Items[1] != 2 {
		t.Fatalf("combos %+v", combos)
	}
	cr := script.Eval(combos[0].Script, script.Context{})
	if cr.Bonus["bBaseAtk"] != 50 || cr.Bonus["bPow"] != 7 || cr.Bonus["bFixedCast"] != 0 {
		t.Errorf("combo: %v (the boot-refine line must be skipped)", cr.Bonus)
	}
}

func TestDescBaseLevelAndEvent(t *testing.T) {
	desc := strings.Split(`A cada 10 níveis de base até o 200:
ATQ +10. DEF +3. DEFM +1.
--------------------------
[Durante o Evento]
Taxa de DROP +7%.
Dano físico e mágico contra todas as raças de monstro +10%.
--------------------------
Tipo: Carta`, "\n")
	r, _, _ := evalDesc(t, desc, nil, 0, 250)
	if r.Bonus["bBaseAtk"] != 200 || r.Bonus["bDef"] != 60 || r.Bonus["bMdef"] != 20 {
		t.Errorf("base level 250 capped at 200: %v", r.Bonus)
	}
	if len(r.Extra) != 0 {
		t.Errorf("event bonuses must not count: %v", r.Extra)
	}
}

func TestDescPlainLinesAndAlternatives(t *testing.T) {
	desc := strings.Split(`CRÍT +5.
Precisão perfeita +10.
Velocidade de ataque +1.
Dano físico à distância +5%.
Dano físico corpo a corpo +5%.
Dano de [Vulcão de Flechas] +15%.
--------------------------
Conjunto
Anel do Lobo Cinzento ou
Pingente do Lobo Cinzento
Pós-conjuração -5%.
2% de chance de converter 2% do dano físico causado em HP e SP.`, "\n")
	names := map[string][]int{"anel do lobo cinzento": {10}, "pingente do lobo cinzento": {11}}
	r, combos, skipped := evalDesc(t, desc, names, 0, 200)
	if r.Bonus["bCritical"] != 5 || r.Bonus["bAspd"] != 1 || r.Bonus["bLongAtkRate"] != 5 || r.Bonus["bShortAtkRate"] != 5 {
		t.Errorf("plain: %v", r.Bonus)
	}
	if r.Extra[`bSkillAtk "CG_ARROWVULCAN"`] != 15 && r.Extra["bSkillAtk CG_ARROWVULCAN"] != 15 {
		t.Errorf("skill bonus: %v", r.Extra)
	}
	if len(combos) != 2 || combos[0].Items[1] != 10 || combos[1].Items[1] != 11 {
		t.Fatalf("alternatives: %+v", combos)
	}
	if cr := script.Eval(combos[0].Script, script.Context{}); cr.Bonus["bDelayrate"] != -5 {
		t.Errorf("combo: %v", cr.Bonus)
	}
	if skipped != 1 {
		t.Errorf("skipped %d, want 1 (the proc line)", skipped)
	}
}

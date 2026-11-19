package clientimport

import (
	"slices"
	"testing"

	"ragnarok-of-building/internal/gamedata"
)

func descSkill(code, name string, lines ...string) *gamedata.Skill {
	return &gamedata.Skill{Code: code, Name: gamedata.Text{"pt": name}, Desc: gamedata.Lines{"pt": lines}}
}

func TestSkillDescEffects(t *testing.T) {
	c := &gamedata.Catalog{Skills: map[int]*gamedata.Skill{
		1: descSkill("IQ_POWERFUL_FAITH", "Mantra da Força", "Mantra da Força", "Tipo: Suporte", "Descrição:",
			"Fortalece sua fé, aumentando ATQ e P.ATQ.", "Não pode ser usada simultaneamente com", "[Mantra da Saúde].",
			"Nível l ATQ l P.ATQ. l Duração", "[Nv 1]: +10 l +7 l 120 segundos", "[Nv 5]: +30 l +15 l 240 segundos"),
		2: descSkill("IQ_FIRM_FAITH", "Mantra da Saúde", "Tipo: Suporte", "Fortalece sua fé, aumentando HP e TEN.",
			"Nível l HP l TEN l Duração", "[Nv 5]: +10% l +40 l 240 segundos"),
		3: descSkill("MO_IRONHAND", "Punhos de Ferro", "Tipo: Passiva", "Aumenta o ATQ de perícia ao usar Soqueiras",
			"ou ao atacar sem nenhuma arma.", "Nível l ATQ", "[Nv 10]: +30"),
		4: descSkill("AL_DEMONBANE", "Flagelo do Mal", "Tipo: Passiva", "Aumenta o ATQ de perícia contra",
			"Malditos, Mortos-Vivos e Demônios.", "Nível l ATQ", "[Nv 10]: +30"),
		// Not buffs of the player: a resulting rate, a curse, a heal, an attack.
		5: descSkill("CR_DEFENDER", "Aura Sagrada", "Tipo: Suporte", "Apenas com Escudo.",
			"Nível l Resistência l Velocidade de ataque", "[Nv 5]: +80% l 95%"),
		6: descSkill("SC_WEAKNESS", "Maldição", "Tipo: Suporte", "Reduz o HP máx. dos inimigos.", "Nível l HP", "[Nv 5]: -30%"),
		7: descSkill("AB_RENOVATIO", "Renovatio", "Tipo: Suporte", "Recupera HP a cada 5 segundos.", "Nível l HP", "[Nv 4]: +8%"),
		8: descSkill("GC_VENOMPRESSURE", "Golpe Venenoso", "Tipo: Ativa", "Causa dano físico.", "Nível l Precisão", "[Nv 5]: +30"),
	}}
	SkillDescEffects(c)
	get := func(id int) *gamedata.SkillEffects { return c.Skills[id].Effects }

	if e := get(1); e == nil || e.Kind != "buff" || e.Levels[5] != "bonus bAtk,30; bonus bPAtk,15;" || e.Group == "" {
		t.Errorf("Mantra da Força: %+v", e)
	}
	if e := get(2); e == nil || e.Levels[5] != "bonus bMaxHPrate,10; bonus bRes,40;" || e.Group != get(1).Group {
		t.Errorf("Mantra da Saúde: %+v (same group as Mantra da Força)", e)
	}
	if e := get(3); e == nil || e.Kind != "passive" || e.Levels[10] != "bonus bMasteryAtk,30;" ||
		!slices.Contains(e.Weapons, "Knuckle") || !slices.Contains(e.Weapons, "Fist") {
		t.Errorf("Punhos de Ferro: %+v", e)
	}
	if e := get(4); e == nil || e.Levels[10] != "bonus2 bMasteryAtkRace,RC_Undead,30; bonus2 bMasteryAtkRace,RC_Demon,30;" {
		t.Errorf("Flagelo do Mal: %+v", e)
	}
	for _, id := range []int{5, 6, 7, 8} {
		if e := get(id); e != nil {
			t.Errorf("%s read as a buff: %+v", c.Skills[id].Code, e)
		}
	}
}

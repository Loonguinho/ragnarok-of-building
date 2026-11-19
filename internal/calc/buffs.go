package calc

import (
	"fmt"
	"strings"

	"ragnarok-of-building/internal/script"
)

// BuffDef is a buff or consumable the user can turn on. Values follow the
// skill pages of bROWiki (current LATAM server), as gathered by latam-ro-calc
// (MIT, src/app/constants/job-buffs.ts, cleric-party-buffs.ts and
// artist-party-buffs.ts at the pinned commit). Only effects the simulator
// computes are kept (cast time, movement speed, regeneration... are not).
type BuffDef struct {
	Key   string
	Label string // Portuguese; the client's skill name wins when Skill is set
	Skill string // client skill code (AB_CLEMENTIA): the name in each language
	Group string // "Suporte", "Músicos", "Outras classes", "Próprios", "Consumíveis", "Outros"
	// Excl: only one buff of the same group is active at a time (one bard
	// song, one Soul Linker spirit...).
	Excl string
	// Choices for a selector ({value, label}); nil = on/off; Max > 0 and no
	// choices = a number input from 0 to Max.
	Choices [][2]string
	Max     int
	// Script gives the bonuses for value v (> 0) as rAthena script.
	Script func(v int, base [6]int) string
}

func levels(n int) [][2]string {
	out := [][2]string{{"0", "—"}}
	for i := 1; i <= n; i++ {
		out = append(out, [2]string{fmt.Sprint(i), fmt.Sprintf("Nv. %d", i)})
	}
	return out
}

func amounts(vals ...int) [][2]string {
	out := [][2]string{{"0", "—"}}
	for _, v := range vals {
		out = append(out, [2]string{fmt.Sprint(v), fmt.Sprintf("+%d", v)})
	}
	return out
}

// perLevel builds the script of a leveled buff from f(level).
func perLevel(f func(lv int) string) func(int, [6]int) string {
	return func(v int, _ [6]int) string { return f(v) }
}

// on is the script of an on/off buff.
func on(s string) func(int, [6]int) string { return func(int, [6]int) string { return s } }

// table maps each choice value to its script.
func table(t map[int]string) func(int, [6]int) string {
	return func(v int, _ [6]int) string { return t[v] }
}

// withMastery: a song's levels as cast by a bard without and with Domínio
// Musical 10 (the level tables leave the caster's mastery out). Values:
// level (no mastery) and 100 + level (mastery 10).
func withMastery(n int) [][2]string {
	out := [][2]string{{"0", "—"}}
	for i := 1; i <= n; i++ {
		out = append(out, [2]string{fmt.Sprint(i), fmt.Sprintf("Nv. %d", i)},
			[2]string{fmt.Sprint(100 + i), fmt.Sprintf("Nv. %d · Domínio Musical 10", i)})
	}
	return out
}

func mastery(f func(lv, m int) string) func(int, [6]int) string {
	return func(v int, _ [6]int) string {
		if v > 100 {
			return f(v-100, 10)
		}
		return f(v, 0)
	}
}

func choices(labels ...string) [][2]string {
	out := [][2]string{{"0", "—"}}
	for i, l := range labels {
		out = append(out, [2]string{fmt.Sprint(i + 1), l})
	}
	return out
}

// eles: the bonus for each of the given elements.
func eles(bonus string, v int, names ...string) string {
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "bonus2 %s,Ele_%s,%d; ", bonus, n, v)
	}
	return b.String()
}

// Buffs lists the selectable buffs in display order.
var Buffs = []BuffDef{
	// ---- Clero (Sacerdote / Arcebispo / Cardeal) ----
	// Bênção: FOR/DES/INT +Nv, Precisão +2 x Nv.
	{Key: "blessing", Skill: "AL_BLESSING", Label: "Bênção", Group: "Suporte", Choices: levels(10), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bStr,%d; bonus bDex,%d; bonus bInt,%d; bonus bHit,%d;", v, v, v, 2*v)
	})},
	// Clementia: +1 FOR/INT/DES/Precisão per 10 job levels of the Arcebispo.
	{Key: "clementia", Label: "Bênção: bônus do Arcebispo (Clementia)", Group: "Suporte",
		Choices: [][2]string{{"0", "—"}, {"1", "Classe 10 (+1)"}, {"2", "Classe 20 (+2)"}, {"3", "Classe 30 (+3)"}, {"4", "Classe 40 (+4)"},
			{"5", "Classe 50 (+5)"}, {"6", "Classe 60 (+6)"}, {"7", "Classe 70 (+7)"}},
		Script: perLevel(func(v int) string {
			return fmt.Sprintf("bonus bStr,%d; bonus bDex,%d; bonus bInt,%d; bonus bHit,%d;", v, v, v, v)
		})},
	// Aumentar Agilidade: AGI +(Nv + 2), velocidade de ataque +Nv%.
	{Key: "incagi", Skill: "AL_INCAGI", Label: "Aumentar Agilidade", Group: "Suporte", Choices: levels(10), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bAgi,%d; bonus bAspdRate,%d;", v+2, v)
	})},
	// Canto Candidus (the Arcebispo's Aumentar Agilidade): +1 AGI and +1%
	// velocidade de ataque per 10 job levels from 20 (latam-ro-calc table).
	{Key: "incagi_ab", Label: "Aumentar Agilidade: bônus do Arcebispo", Group: "Suporte",
		Choices: [][2]string{{"0", "—"}, {"2", "Classe 20 (+2)"}, {"3", "Classe 30 (+3)"}, {"4", "Classe 40 (+4)"},
			{"5", "Classe 50 (+5)"}, {"6", "Classe 60 (+6)"}, {"7", "Classe 70 (+7)"}},
		Script: perLevel(func(v int) string { return fmt.Sprintf("bonus bAgi,%d; bonus bAspdRate,%d;", v, v) })},
	// Glória: SOR +30 (the level only buys duration).
	{Key: "gloria", Skill: "PR_GLORIA", Label: "Glória (SOR +30)", Group: "Suporte", Script: on("bonus bLuk,30;")},
	// Impositio Manus: ATQ e ATQM +5 x Nv (equipment ATK, rAthena watk).
	{Key: "impositio", Skill: "PR_IMPOSITIO", Label: "Impositio Manus", Group: "Suporte", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bAtk,%d; bonus bMatk,%d;", 5*v, 5*v)
	})},
	// Expiatio: ignora 5 x Nv% da DEF e DEFM.
	{Key: "expiatio", Skill: "AB_EXPIATIO", Label: "Expiatio", Group: "Suporte", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus2 bIgnoreDefRaceRate,RC_All,%d; bonus2 bIgnoreMdefRaceRate,RC_All,%d;", 5*v, 5*v)
	})},
	// Lauda Agnus: HP máx. +4/6/8/10%.
	{Key: "lauda_agnus", Skill: "AB_LAUDAAGNUS", Label: "Lauda Agnus", Group: "Suporte", Choices: levels(4), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bMaxHPrate,%d;", 2+2*v)
	})},
	// Lauda Ramus: dano crítico +5 x Nv%.
	{Key: "lauda_ramus", Skill: "AB_LAUDARAMUS", Label: "Lauda Ramus", Group: "Suporte", Choices: levels(4), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bCritAtkRate,%d;", 5*v)
	})},
	// Poder de Odin: ATQ e ATQM +70/+100, DEF e DEFM -20/-40.
	{Key: "odin", Skill: "ALL_ODINS_POWER", Label: "Poder de Odin", Group: "Suporte", Choices: levels(2),
		Script: table(map[int]string{1: "bonus bAtk,70; bonus bMatk,70; bonus bDef,-20; bonus bMdef,-20;",
			2: "bonus bAtk,100; bonus bMatk,100; bonus bDef,-40; bonus bMdef,-40;"})},
	// Cardeal.
	{Key: "competentia", Skill: "CD_COMPETENTIA", Label: "Competentia", Group: "Suporte", Script: on("bonus bPAtk,50; bonus bSMatk,50;")},
	{Key: "religio", Skill: "CD_RELIGIO", Label: "Religio", Group: "Suporte", Script: on("bonus bSta,10; bonus bWis,10; bonus bSpl,10;")},
	{Key: "benedictum", Skill: "CD_BENEDICTUM", Label: "Benedictum", Group: "Suporte", Script: on("bonus bPow,10; bonus bCon,10; bonus bCrt,10;")},
	// Presens Acies: C.TAXA +2 x Nv.
	{Key: "presens_acies", Skill: "CD_PRESENS_ACIES", Label: "Presens Acies", Group: "Suporte", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bCRate,%d;", 2*v)
	})},
	// Argutus Telum / Vita: ignora 5 x Nv% da RES / RESM.
	{Key: "argutus_telum", Skill: "CD_ARGUTUS_TELUM", Label: "Argutus Telum", Group: "Suporte", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus2 bIgnoreResRaceRate,RC_All,%d;", 5*v)
	})},
	{Key: "argutus_vita", Skill: "CD_ARGUTUS_VITA", Label: "Argutus Vita", Group: "Suporte", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus2 bIgnoreMresRaceRate,RC_All,%d;", 5*v)
	})},

	// ---- Músicos (Bardo/Odalisca, Trovador/Musa, Maestro/Diva) ----
	// One song of each kind at a time (latam-ro-calc's exclusive groups).
	{Key: "whistle", Skill: "BA_WHISTLE", Label: "Assovio", Group: "Músicos", Excl: "bard_song", Choices: levels(10), Script: perLevel(func(v int) string {
		f := 18 + 2*v
		if v == 10 {
			f = 40
		}
		return fmt.Sprintf("bonus bFlee,%d;", f)
	})},
	{Key: "assassincross", Skill: "BA_ASSASSINCROSS", Label: "Crepúsculo Sangrento", Group: "Músicos", Excl: "bard_song", Choices: levels(10), Script: perLevel(func(v int) string {
		a := 2*v - 1
		if v == 10 {
			a = 20
		}
		return fmt.Sprintf("bonus bAspdRate,%d;", a)
	})},
	{Key: "appleidun", Skill: "BA_APPLEIDUN", Label: "Maçãs de Idun", Group: "Músicos", Excl: "bard_song", Choices: levels(10), Script: perLevel(func(v int) string {
		h := 9 + v
		if v == 10 {
			h = 20
		}
		return fmt.Sprintf("bonus bMaxHPrate,%d;", h)
	})},
	{Key: "humming", Skill: "DC_HUMMING", Label: "Sibilo", Group: "Músicos", Excl: "dancer_dance", Choices: levels(10), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bHit,%d;", 4*v)
	})},
	{Key: "fortunekiss", Skill: "DC_FORTUNEKISS", Label: "Beijo da Sorte", Group: "Músicos", Excl: "dancer_dance", Choices: levels(10), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bCritical,%d; bonus bCritAtkRate,%d;", v, 2*v)
	})},
	{Key: "serviceforyou", Skill: "DC_SERVICEFORYOU", Label: "Dança Cigana", Group: "Músicos", Excl: "dancer_dance", Choices: levels(10), Script: perLevel(func(v int) string {
		s := 9 + v
		if v == 10 {
			s = 20
		}
		return fmt.Sprintf("bonus bMaxSPrate,%d;", s)
	})},
	{Key: "drumbattle", Skill: "BD_DRUMBATTLEFIELD", Label: "Rufar dos Tambores", Group: "Músicos", Excl: "artist_duet", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bAtk,%d; bonus bDef,%d;", 15+5*v, 15*v)
	})},
	// Anel dos Nibelungos: one of these outcomes for each party member.
	{Key: "nibelungen", Skill: "BD_RINGNIBELUNGEN", Label: "Anel dos Nibelungos", Group: "Músicos", Excl: "artist_duet",
		Choices: choices("Esquiva +50", "Precisão +50", "HP máx. +30%", "SP máx. +30%", "Atributos +15", "Dano mágico +20%", "Dano físico +20%", "Vel. de ataque +20%"),
		Script: table(map[int]string{1: "bonus bFlee,50;", 2: "bonus bHit,50;", 3: "bonus bMaxHPrate,30;", 4: "bonus bMaxSPrate,30;",
			5: "bonus bAllStats,15;", 6: "bonus bMatkRate,20;", 7: "bonus2 bAddRace,RC_All,20;", 8: "bonus bAspdRate,20;"})},
	{Key: "rushwindmill", Skill: "MI_RUSH_WINDMILL", Label: "Sinfonia dos Ventos", Group: "Músicos", Excl: "minstrel_song", Choices: withMastery(5),
		Script: mastery(func(lv, m int) string {
			per := 1
			if lv == 5 {
				per = 2
			}
			return fmt.Sprintf("bonus bAtk,%d;", []int{0, 7, 10, 13, 15, 20}[lv]+m*per)
		})},
	{Key: "moonlit", Skill: "WA_MOONLIT_SERENADE", Label: "Serenata ao Luar", Group: "Músicos", Excl: "wanderer_dance",
		Choices: [][2]string{{"0", "—"}, {"13", "Classe 30"}, {"14", "Classe 40"}, {"15", "Classe 50"}, {"16", "Classe 60"}, {"17", "Classe 70"}},
		Script:  perLevel(func(v int) string { return fmt.Sprintf("bonus bMatk,%d;", 46+2*(v-13)) })},
	{Key: "frigg", Skill: "WM_FRIGG_SONG", Label: "Canção de Frigga", Group: "Músicos", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bMaxHPrate,%d;", 5*v)
	})},
	{Key: "lerads", Skill: "WM_LERADS_DEW", Label: "Orvalho de Idun", Group: "Músicos", Excl: "artist_chorus", Choices: withMastery(5),
		Script: mastery(func(lv, m int) string { return fmt.Sprintf("bonus bMaxHPrate,%d;", 2+3*lv+m/3) })},
	{Key: "dancewithwug", Skill: "WM_DANCE_WITH_WUG", Label: "Dança com Lobos", Group: "Músicos", Excl: "artist_chorus", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bLongAtkRate,%d; bonus bAspdRate,%d;", v, 5*v)
	})},
	{Key: "infinitehumming", Skill: "WM_UNLIMITED_HUMMING_VOICE", Label: "Murmúrio Perene", Group: "Músicos", Excl: "artist_chorus", Choices: withMastery(5),
		Script: mastery(func(lv, m int) string { return fmt.Sprintf("bonus2 bMagicAtkEle,Ele_All,%d;", 4*lv+m/2) })},
	// Interlúdio: RES; solo and with Participação Especial (dueto) differ.
	{Key: "interlude", Skill: "TR_MUSICAL_INTERLUDE", Label: "Interlúdio", Group: "Músicos",
		Choices: [][2]string{{"0", "—"}, {"1", "Solo Nv. 1"}, {"2", "Solo Nv. 2"}, {"3", "Solo Nv. 3"}, {"4", "Solo Nv. 4"}, {"5", "Solo Nv. 5"},
			{"6", "Dueto Nv. 1"}, {"7", "Dueto Nv. 2"}, {"8", "Dueto Nv. 3"}, {"9", "Dueto Nv. 4"}, {"10", "Dueto Nv. 5"}},
		Script: perLevel(func(v int) string {
			return fmt.Sprintf("bonus bRes,%d;", []int{0, 10, 15, 20, 25, 30, 15, 22, 30, 37, 45}[v])
		})},
	{Key: "pronmarch", Skill: "TR_PRON_MARCH", Label: "Marcha de Prontera", Group: "Músicos",
		Choices: [][2]string{{"0", "—"}, {"5", "P.ATQ +15"}, {"6", "P.ATQ +22"}},
		Script:  table(map[int]string{5: "bonus bPAtk,15;", 6: "bonus bPAtk,22;"})},
	{Key: "jawaii", Skill: "TR_JAWAII_SERENADE", Label: "Serenata de Jawaii", Group: "Músicos",
		Choices: [][2]string{{"0", "—"}, {"5", "S.ATQM +15"}, {"6", "S.ATQM +22"}},
		Script:  table(map[int]string{5: "bonus bSMatk,15;", 6: "bonus bSMatk,22;"})},

	// ---- Outras classes ----
	{Key: "loud", Skill: "MC_LOUD", Label: "Grito de Guerra", Group: "Outras classes", Script: on("bonus bStr,4; bonus bAtk,30;")},
	// Shura: Chakra da Fúria (ATQ +8 x Nv, ATQ +Nv%) and Chakra do Vigor
	// (HP máx. +2 x Nv%, DEF +20 x Nv), also cast on party members.
	{Key: "gt_change", Skill: "SR_GENTLETOUCH_CHANGE", Label: "Chakra da Fúria", Group: "Outras classes", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bAtk,%d; bonus bAtkRate,%d;", 8*v, v)
	})},
	{Key: "gt_revitalize", Skill: "SR_GENTLETOUCH_REVITALIZE", Label: "Chakra do Vigor", Group: "Outras classes", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bMaxHPrate,%d; bonus bDef,%d;", 2*v, 20*v)
	})},
	// Espíritos do Espiritualista (one at a time).
	{Key: "soul_fairy", Skill: "SP_SOULFAIRY", Label: "Espírito da Fada", Group: "Outras classes", Excl: "soul", Script: on("bonus bMatk,50;")},
	{Key: "soul_shadow", Skill: "SP_SOULSHADOW", Label: "Espírito das Sombras", Group: "Outras classes", Excl: "soul", Script: on("bonus bCritical,20; bonus bAspd,3;")},
	{Key: "soul_falcon", Skill: "SP_SOULFALCON", Label: "Espírito do Falcão", Group: "Outras classes", Excl: "soul", Script: on("bonus bAtk,50; bonus bHit,15;")},
	{Key: "soul_golem", Skill: "SP_SOULGOLEM", Label: "Espírito do Golem", Group: "Outras classes", Excl: "soul", Script: on("bonus bDef,300; bonus bMdef,40;")},
	// Encanto de Órion (Feiticeiro): ATQ +100.
	{Key: "striking", Skill: "SO_STRIKING", Label: "Encanto de Órion", Group: "Outras classes", Script: on("bonus bAtk,100;")},
	// Geleira: the state Cacos de Gelo leaves under Potencializar Magia Nv1.
	{Key: "geleira", Label: "Geleira (Cacos de Gelo)", Group: "Outras classes", Script: on("bonus bDef,300; bonus bMdef,100; bonus2 bMagicAtkEle,Ele_Water,30;")},
	// Asceta das Almas.
	{Key: "soa_warrior", Skill: "SOA_TALISMAN_OF_WARRIOR", Label: "Talismã do Guerreiro", Group: "Outras classes", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bPAtk,%d;", 2*v)
	})},
	{Key: "soa_magician", Skill: "SOA_TALISMAN_OF_MAGICIAN", Label: "Talismã do Mago", Group: "Outras classes", Choices: levels(5), Script: perLevel(func(v int) string {
		return fmt.Sprintf("bonus bSMatk,%d;", 2*v)
	})},
	{Key: "soa_elements", Skill: "SOA_TALISMAN_OF_FIVE_ELEMENTS", Label: "Talismã dos Elementos", Group: "Outras classes", Choices: levels(5), Script: perLevel(func(v int) string {
		e := []string{"Water", "Wind", "Earth", "Fire", "Neutral"}
		return eles("bAddEle", 4*v, e...) + eles("bMagicAddEle", 4*v, e...)
	})},
	{Key: "soa_heavenearth", Skill: "SOA_SOUL_OF_HEAVEN_AND_EARTH", Label: "Reencarnação das Almas", Group: "Outras classes", Choices: levels(10), Script: perLevel(func(v int) string {
		x := 5 + 2*v
		return fmt.Sprintf("bonus bShortAtkRate,%d; bonus bLongAtkRate,%d; bonus2 bMagicAtkEle,Ele_All,%d;", x, x, x)
	})},
	// Chuva de Mariscos (Doram): ATQ e ATQM +10%.
	{Key: "bunchofshrimp", Skill: "SU_BUNCHOFSHRIMP", Label: "Chuva de Mariscos", Group: "Outras classes", Script: on("bonus bAtkRate,10; bonus bMatkRate,10;")},

	// ---- Próprios ----
	// Concentrar: DES e AGI +(Nv + 2)% (of the base value).
	{Key: "concentration", Skill: "AC_CONCENTRATION", Label: "Concentrar", Group: "Próprios", Choices: levels(10), Script: func(v int, base [6]int) string {
		return fmt.Sprintf("bonus bAgi,%d; bonus bDex,%d;", base[AGI]*(v+2)/100, base[DEX]*(v+2)/100)
	}},

	// ---- Consumíveis ----
	{Key: "food_str", Label: "Comida de FOR", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bStr")},
	{Key: "food_agi", Label: "Comida de AGI", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bAgi")},
	{Key: "food_vit", Label: "Comida de VIT", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bVit")},
	{Key: "food_int", Label: "Comida de INT", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bInt")},
	{Key: "food_dex", Label: "Comida de DES", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bDex")},
	{Key: "food_luk", Label: "Comida de SOR", Group: "Consumíveis", Choices: amounts(5, 10, 15, 20), Script: statFood("bLuk")},
	// ASPD potions: a fixed bonus × AGI ÷ 200 (rAthena SC_ASPDPOTION*, in
	// status_calc_aspd's fixed group), LATAM values 4/6/9 (latam-ro-calc
	// aspd-potion-list.ts). The stored values stay 10/15/20 (older builds).
	{Key: "aspd_potion", Label: "Poção de velocidade de ataque", Group: "Consumíveis",
		Choices: [][2]string{{"0", "—"}, {"10", "Poção da Concentração"}, {"15", "Poção do Despertar"}, {"20", "Poção da Fúria Selvagem"}},
		Script:  table(map[int]string{10: "bonus bAspdFixed,4;", 15: "bonus bAspdFixed,6;", 20: "bonus bAspdFixed,9;"})},

	// ---- Outros: anything else, typed in ----
	{Key: "x_atk", Label: "ATQ", Group: "Outros", Max: 9999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bBaseAtk,%d;", v) }},
	{Key: "x_matk", Label: "ATQM", Group: "Outros", Max: 9999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bMatk,%d;", v) }},
	{Key: "x_patk", Label: "P.ATQ", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bPAtk,%d;", v) }},
	{Key: "x_smatk", Label: "S.ATQM", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bSMatk,%d;", v) }},
	{Key: "x_physrate", Label: "Dano físico %", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bAtkRate,%d;", v) }},
	{Key: "x_magicrate", Label: "Dano mágico %", Group: "Outros", Max: 999, Script: func(v int, _ [6]int) string { return fmt.Sprintf("bonus bMatkRate,%d;", v) }},
}

func statFood(name string) func(int, [6]int) string {
	return func(v int, _ [6]int) string { return fmt.Sprintf("bonus %s,%d;", name, v) }
}

var buffByKey = func() map[string]*BuffDef {
	m := map[string]*BuffDef{}
	for i := range Buffs {
		m[Buffs[i].Key] = &Buffs[i]
	}
	return m
}()

// BuffMax is the highest value a buff accepts.
func (d *BuffDef) BuffMax() int {
	switch {
	case d.Choices != nil:
		n := 0
		for _, c := range d.Choices {
			var v int
			fmt.Sscan(c[0], &v)
			n = max(n, v)
		}
		return n
	case d.Max > 0:
		return d.Max
	}
	return 1
}

// valid reports whether v is one of the buff's values.
func (d *BuffDef) valid(v int) bool {
	if d.Choices == nil {
		return v <= d.BuffMax()
	}
	for _, c := range d.Choices {
		if c[0] == fmt.Sprint(v) {
			return true
		}
	}
	return false
}

// normalizeBuffs drops unknown buffs and values, and keeps one buff per
// exclusive group (the first in display order).
func normalizeBuffs(b *Build) {
	for k, v := range b.Buffs {
		if strings.HasPrefix(k, ItemEffectPrefix) {
			if v <= 0 {
				delete(b.Buffs, k)
			} else {
				b.Buffs[k] = 1
			}
			continue
		}
		d := buffByKey[k]
		if d == nil || v <= 0 {
			delete(b.Buffs, k)
			continue
		}
		if !d.valid(v) {
			b.Buffs[k] = min(v, d.BuffMax())
			if !d.valid(b.Buffs[k]) {
				delete(b.Buffs, k)
			}
		}
	}
	seen := map[string]bool{}
	for _, d := range Buffs {
		if d.Excl == "" || b.Buffs[d.Key] == 0 {
			continue
		}
		if seen[d.Excl] {
			delete(b.Buffs, d.Key)
		}
		seen[d.Excl] = true
	}
	if len(b.Buffs) == 0 {
		b.Buffs = nil
	}
}

// applyBuffs adds the active buffs' bonuses. base = base stats + job bonus.
func applyBuffs(b *Build, base [6]int, bonus, extra map[string]int) {
	for _, d := range Buffs {
		v := b.Buffs[d.Key]
		if v <= 0 {
			continue
		}
		res := script.Eval(d.Script(v, base), script.Context{BaseLv: b.BaseLv, JobLv: b.JobLv, Class: b.Job})
		addAll(bonus, res.Bonus)
		addAll(extra, res.Extra)
	}
}

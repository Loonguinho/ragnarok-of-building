package clientimport

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// Buff and passive effects read from the client's skill descriptions (PT).
// The LATAM client writes most of them as a level table:
//
//	Nível l ATQ l P.ATQ. l Duração
//	[Nv 1]: +10 l +7 l 120 segundos
//
// and says in the text what they depend on: "ATQ de perícia" (mastery ATK),
// "ao usar Maças" (weapon), "contra alvos das raças Inseto, Bruto" (race),
// "Não pode ser usada com [X]" (exclusive). Only columns whose meaning is
// clear become bonuses; the rest is kept in Unread for review. Values that
// depend on a formula are left to latam-ro-calc / rAthena.

var (
	reSkillHead  = regexp.MustCompile(`(?i)^\s*n[ií]vel\s*(?:\sl\s|\|)\s*(.+)$`)
	reSkillRow   = regexp.MustCompile(`(?i)^\s*\[\s*n[ví]\.?\s*(\d+)\s*\]\s*:?\s*(.+)$`)
	reColSep     = regexp.MustCompile(`\s+l\s+|\s*\|\s*`)
	reColorCode  = regexp.MustCompile(`\^[0-9a-fA-F]{6}`)
	reSkillValue = regexp.MustCompile(`^([+-]?\d[\d.]*(?:,\d+)?)\s*(%)?`)
	reSkillType  = regexp.MustCompile(`(?i)^\s*tipo\s*:\s*(\S+)`)
	reNotWith    = regexp.MustCompile(`(?i)n[ãa]o pode ser usad[ao] (?:simultaneamente )?com (.+)`)
	reBracket    = regexp.MustCompile(`\[([^\]]+)\]`)
)

// Weapon words of the descriptions -> rAthena SubTypes.
var descWeapons = []struct {
	re    *regexp.Regexp
	types []string
}{
	{regexp.MustCompile(`(?i)sem (?:nenhuma )?arma|desarmad`), []string{"Fist"}},
	{regexp.MustCompile(`(?i)soqueiras?`), []string{"Knuckle"}},
	{regexp.MustCompile(`(?i)lanças? de duas mãos`), []string{"2hSpear"}},
	{regexp.MustCompile(`(?i)lanças?`), []string{"1hSpear", "2hSpear"}},
	{regexp.MustCompile(`(?i)espadas? de duas mãos`), []string{"2hSword"}},
	{regexp.MustCompile(`(?i)espadas? de uma mão`), []string{"1hSword"}},
	{regexp.MustCompile(`(?i)\bespadas?\b`), []string{"1hSword"}},
	{regexp.MustCompile(`(?i)\badagas?\b`), []string{"Dagger"}},
	{regexp.MustCompile(`(?i)\bmaças?\b`), []string{"Mace", "2hMace"}},
	{regexp.MustCompile(`(?i)\bmachados?\b`), []string{"1hAxe", "2hAxe"}},
	{regexp.MustCompile(`(?i)\bkatar(?:es|s)?\b`), []string{"Katar"}},
	{regexp.MustCompile(`(?i)\bcajados?\b`), []string{"Staff", "2hStaff"}},
	{regexp.MustCompile(`(?i)\barcos?\b`), []string{"Bow"}},
	{regexp.MustCompile(`(?i)instrumentos? musica`), []string{"Musical"}},
	{regexp.MustCompile(`(?i)\bchicotes?\b`), []string{"Whip"}},
	{regexp.MustCompile(`(?i)\blivros?\b`), []string{"Book"}},
}

var reWeaponCue = regexp.MustCompile(`(?i)ao (?:usar|equipar|empunhar)|apenas com|somente com|quando (?:usar|equipar|equipad)|equipad[oa] com`)

var descRaces = map[string]string{
	"demônios": "Demon", "mortos-vivos": "Undead", "insetos": "Insect", "brutos": "Brute", "peixes": "Fish",
	"plantas": "Plant", "dragões": "Dragon", "anjos": "Angel", "humanoides": "DemiHuman", "amorfos": "Formless",
	"demônio": "Demon", "demonio": "Demon", "morto-vivo": "Undead", "inseto": "Insect", "bruto": "Brute",
	"doram": "Player_Doram", "peixe": "Fish", "planta": "Plant", "dragão": "Dragon", "dragao": "Dragon",
	"anjo": "Angel", "humanoide": "DemiHuman", "humano": "DemiHuman", "amorfo": "Formless", "sem forma": "Formless",
}

var (
	// HP/SP columns are the maximum only when the text says it rises (else
	// they are a cost or a heal).
	reRaisesHPSP = regexp.MustCompile(`(?i)aument\w*[^.]{0,40}\b(hp|sp)\b`)
	reHeals      = regexp.MustCompile(`(?i)\b(recupera|restaura|cura)\w*`)
	// Effects that are not the player's own: on the target or enemies, or the
	// stats of a summoned creature (elemental, homunculus, robot, decoy...).
	reNotOwn = regexp.MustCompile(`(?i)inimig|oponente|\bdo alvo\b|\bdos alvos\b|\bnos alvos\b|\bo alvo\b|invoca|elemental|hom[úu]nculo|homunculus|\babr\b|rob[ôo]|armadilha|dispositivo|\bisca\b|mão esquerda|mão direita`)
	// An active skill that deals damage: its table is about that attack.
	reDealsDamage = regexp.MustCompile(`(?i)\bdano\b|\bataca\b|\bgolpe`)
)

// skipSkillDesc reports descriptions whose table is not a buff of the
// player: effects on targets or creatures, attacks, and skills checked by
// hand whose columns read as something else.
func skipSkillDesc(code, text string, active bool) bool {
	switch code {
	case "RK_ENCHANTBLADE", // ATQM column: magic damage added to each hit, not MATK
		"SC_AUTOSHADOWSPELL",                                           // ATQM column: the copied spell's level effect
		"BO_BIONICS_M", "MT_ABR_M", "NC_SILVERSNIPER", "NC_MAGICDECOY", // the creature's or device's stats
		"RA_TOOTHOFWUG": // the warg's ATK
		return true
	}
	return reNotOwn.MatchString(text) || (active && reDealsDamage.MatchString(text))
}

// colBonus turns one table column into rAthena script for value v. ok false
// means the column is not understood; skip true means it is understood but
// is nothing the simulator computes (duration, costs...).
func colBonus(name string, v int, pct, passive bool, races []string) (script string, skip, ok bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.NewReplacer(".", "", ":", "").Replace(n)
	n = strings.Join(strings.Fields(n), " ")
	switch {
	case n == "" || strings.Contains(n, "duração") || strings.Contains(n, "custo") || strings.Contains(n, "consumo") ||
		strings.Contains(n, "recarga") || strings.Contains(n, "alcance") || strings.Contains(n, "área") ||
		strings.Contains(n, "conjuração") || strings.Contains(n, "peso") || strings.Contains(n, "recuperad") ||
		strings.Contains(n, "chance") || strings.Contains(n, "resistência") || strings.Contains(n, "tempo") ||
		strings.Contains(n, "regenera") || strings.Contains(n, "movimento"):
		return "", true, true
	case strings.Contains(n, "montaria"):
		return "", true, true // the mounted value (Perícia com Lança): no mount state yet
	}
	one := func(b string) string { return fmt.Sprintf("bonus %s,%d;", b, v) }
	switch n {
	case "atq":
		if pct && len(races) > 0 { // "+N% de ATQ contra a raça Dragão"
			var s strings.Builder
			for _, r := range races {
				fmt.Fprintf(&s, "bonus2 bAddRace,RC_%s,%d; ", r, v)
			}
			return strings.TrimSpace(s.String()), false, true
		}
		if pct {
			return one("bAtkRate"), false, true
		}
		if passive { // passives raise mastery ATK (battle_calc_attack_masteries)
			if len(races) > 0 {
				var s strings.Builder
				for _, r := range races {
					fmt.Fprintf(&s, "bonus2 bMasteryAtkRace,RC_%s,%d; ", r, v)
				}
				return strings.TrimSpace(s.String()), false, true
			}
			return one("bMasteryAtk"), false, true
		}
		return one("bAtk"), false, true
	case "atqm":
		if pct && len(races) > 0 {
			var s strings.Builder
			for _, r := range races {
				fmt.Fprintf(&s, "bonus2 bMagicAddRace,RC_%s,%d; ", r, v)
			}
			return strings.TrimSpace(s.String()), false, true
		}
		if pct {
			return one("bMatkRate"), false, true
		}
		return one("bMatk"), false, true
	case "atq da arma":
		if pct {
			return one("bWeaponAtkRate"), false, true
		}
	case "patq":
		return one("bPAtk"), false, true
	case "satqm":
		return one("bSMatk"), false, true
	case "hp", "hp máx", "hp max":
		if pct {
			return one("bMaxHPrate"), false, true
		}
		return one("bMaxHP"), false, true
	case "sp", "sp máx", "sp max":
		if pct {
			return one("bMaxSPrate"), false, true
		}
		return one("bMaxSP"), false, true
	case "ten", "res":
		return one("bRes"), false, true
	case "tenm", "resm", "mres":
		return one("bMRes"), false, true
	case "vel atq", "velatq", "velocidade de ataque", "vel de ataque", "aspd":
		if pct {
			return one("bAspdRate"), false, true
		}
		return one("bAspd"), false, true
	case "precisão perfeita":
		return one("bPerfectHitRate"), false, true
	case "precisão":
		if !pct {
			return one("bHit"), false, true
		}
	case "esquiva":
		if !pct {
			return one("bFlee"), false, true
		}
	case "crit", "crítico", "taxa de crítico":
		if !pct {
			return one("bCritical"), false, true
		}
	case "dano crítico":
		return one("bCritAtkRate"), false, true
	case "def":
		if !pct {
			return one("bDef"), false, true
		}
	case "defm":
		if !pct {
			return one("bMdef"), false, true
		}
	case "for":
		return one("bStr"), false, true
	case "agi":
		return one("bAgi"), false, true
	case "vit":
		return one("bVit"), false, true
	case "int":
		return one("bInt"), false, true
	case "des":
		return one("bDex"), false, true
	case "sor":
		return one("bLuk"), false, true
	case "pot", "pow":
		return one("bPow"), false, true
	case "sta":
		return one("bSta"), false, true
	case "sab", "wis":
		return one("bWis"), false, true
	case "fei", "spl":
		return one("bSpl"), false, true
	case "con":
		return one("bCon"), false, true
	case "crt":
		return one("bCrt"), false, true
	case "dano":
		// Only damage against named races ("Inflige mais dano contra as raças
		// Demônio e Morto-Vivo"); a bare "Dano" is a skill's own damage.
		if pct && len(races) > 0 {
			var s strings.Builder
			for _, r := range races {
				fmt.Fprintf(&s, "bonus2 bAddRace,RC_%s,%d; ", r, v)
			}
			return strings.TrimSpace(s.String()), false, true
		}
	}
	// "P.ATQ e S.ATQM": both columns with the same value.
	if parts := strings.Split(n, " e "); len(parts) > 1 {
		var out []string
		for _, p := range parts {
			s, sk, ok := colBonus(p, v, pct, passive, races)
			if !ok || sk {
				return "", false, false
			}
			out = append(out, s)
		}
		return strings.Join(out, " "), false, true
	}
	return "", false, false
}

func parseDescValue(s string) (int, bool, bool) {
	m := reSkillValue.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false, false
	}
	num := strings.ReplaceAll(m[1], ".", "")
	if strings.Contains(num, ",") { // a decimal: not a whole bonus
		return 0, false, false
	}
	// A bonus always has its sign ("+10", "-40"); "100%" or "5.000" is a
	// resulting rate or a cost.
	if !strings.HasPrefix(num, "+") && !strings.HasPrefix(num, "-") {
		return 0, false, false
	}
	v, err := strconv.Atoi(strings.TrimPrefix(num, "+"))
	return v, m[2] == "%", err == nil
}

// SkillDescEffects reads the effects of every buff and passive whose PT
// description has a level table, and links the buffs that exclude each
// other. It returns how many skills got effects.
func SkillDescEffects(c *gamedata.Catalog) int {
	byName := map[string]string{}
	for _, s := range c.Skills {
		byName[strings.ToLower(strings.TrimSpace(s.Name.Get("pt")))] = s.Code
	}
	excl := map[string][]string{}
	n := 0
	for _, s := range c.Skills {
		s.Effects = nil
		lines := s.Desc["pt"]
		if len(lines) == 0 {
			continue
		}
		var kind, text string
		var head []string
		rows := map[int][]string{}
		for _, raw := range lines {
			l := strings.TrimSpace(reColorCode.ReplaceAllString(raw, ""))
			if m := reSkillType.FindStringSubmatch(l); m != nil && kind == "" {
				kind = strings.ToLower(m[1])
				continue
			}
			if m := reSkillHead.FindStringSubmatch(l); m != nil && head == nil {
				head = reColSep.Split(strings.TrimSpace(m[1]), -1)
				continue
			}
			if m := reSkillRow.FindStringSubmatch(l); m != nil && head != nil {
				lv, _ := strconv.Atoi(m[1])
				rows[lv] = reColSep.Split(strings.TrimSpace(m[2]), -1)
				continue
			}
			if head == nil {
				text += " " + l
			}
		}
		passive := strings.HasPrefix(kind, "passiv")
		active := strings.HasPrefix(kind, "ativa")
		if head == nil || len(rows) == 0 || !(passive || active || strings.HasPrefix(kind, "suporte")) ||
			skipSkillDesc(s.Code, text, active) {
			continue
		}
		// Races the text ties the effect to ("contra alvos das raças Inseto,
		// Bruto", "contra Demônio e Morto-Vivo"): the race words of the
		// sentence after "contra".
		var races []string
		if i := strings.Index(strings.ToLower(text), "contra "); i >= 0 {
			seg := text[i:]
			if j := strings.Index(seg, "."); j > 0 {
				seg = seg[:j]
			}
			for _, w := range regexp.MustCompile(`[,]|\se\s|\sou\s|\s`).Split(strings.ToLower(seg), -1) {
				if r, ok := descRaces[strings.TrimSpace(w)]; ok && !slices.Contains(races, r) {
					races = append(races, r)
				}
			}
		}
		raisesHPSP := reRaisesHPSP.MatchString(text) && !reHeals.MatchString(text)
		var weapons []string
		if loc := reWeaponCue.FindStringIndex(text); loc != nil {
			tail := text[loc[0]:]
			if i := strings.IndexAny(tail, "."); i > 0 {
				tail = tail[:i]
			}
			for _, w := range descWeapons {
				if w.re.MatchString(tail) {
					for _, t := range w.types {
						if !slices.Contains(weapons, t) {
							weapons = append(weapons, t)
						}
					}
					tail = w.re.ReplaceAllString(tail, "")
				}
			}
		}
		eff := &gamedata.SkillEffects{Kind: "buff", Weapons: weapons, Levels: map[int]string{}}
		if passive {
			eff.Kind = "passive"
		}
		unread := map[string]bool{}
		positive := false
		for lv, cells := range rows {
			var parts []string
			for i, cell := range cells {
				if i >= len(head) {
					break
				}
				v, pct, ok := parseDescValue(cell)
				if !ok {
					if !strings.Contains(strings.ToLower(head[i]), "duração") {
						unread[head[i]] = true
					}
					continue
				}
				col := strings.ToLower(strings.NewReplacer(".", "", " ", "").Replace(head[i]))
				if (col == "hp" || col == "sp") && !raisesHPSP {
					continue // an HP/SP cost or a heal, not the maximum
				}
				if pct && ((col == "atq" || col == "atqm" || col == "dano") && v > 100 || col == "velatq" && v > 50) {
					unread[head[i]] = true // a skill's own damage, or an ASPD penalty recovery
					continue
				}
				sc, skip, ok := colBonus(head[i], v, pct, passive, races)
				switch {
				case !ok:
					unread[head[i]] = true
				case !skip && v != 0:
					parts = append(parts, sc)
					positive = positive || v > 0
				}
			}
			if len(parts) > 0 {
				eff.Levels[lv] = strings.Join(parts, " ")
			}
		}
		for k := range unread {
			eff.Unread = append(eff.Unread, strings.TrimSpace(k))
		}
		sort.Strings(eff.Unread)
		if len(eff.Levels) == 0 || !positive { // only penalties: a curse on the target
			continue
		}
		s.Effects = eff
		n++
		if m := reNotWith.FindStringSubmatch(text); m != nil && !passive {
			sentence := m[1]
			if i := strings.Index(sentence, "."); i >= 0 { // the text runs on into the next sentences
				sentence = sentence[:i]
			}
			names := reBracket.FindAllStringSubmatch(sentence, -1)
			if names == nil { // "Não pode ser usada com Chakra do Vigor."
				for _, nm := range regexp.MustCompile(`,|\se\s|\sou\s`).Split(sentence, -1) {
					names = append(names, []string{"", strings.TrimSpace(nm)})
				}
			}
			for _, nm := range names {
				if code, ok := byName[strings.ToLower(strings.TrimSpace(nm[1]))]; ok && code != s.Code {
					excl[s.Code] = append(excl[s.Code], code)
					excl[code] = append(excl[code], s.Code)
				}
			}
		}
	}
	// Exclusive groups: the connected skills share the smallest code.
	byCode := map[string]*gamedata.Skill{}
	for _, s := range c.Skills {
		byCode[s.Code] = s
	}
	seen := map[string]bool{}
	for code := range excl {
		if seen[code] {
			continue
		}
		group, stack := []string{}, []string{code}
		for len(stack) > 0 {
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if seen[x] {
				continue
			}
			seen[x] = true
			group = append(group, x)
			stack = append(stack, excl[x]...)
		}
		sort.Strings(group)
		for _, g := range group {
			if s := byCode[g]; s != nil && s.Effects != nil && s.Effects.Kind == "buff" {
				s.Effects.Group = group[0]
			}
		}
	}
	return n
}

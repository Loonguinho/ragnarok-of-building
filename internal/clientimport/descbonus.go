package clientimport

// LATAM-only equipment, cards and enchants have no rAthena script, but the
// client's PT description lists their bonuses in a regular format. DescScript
// turns those lines into an rAthena-style script for the evaluator:
//
//	ATQ +10.                       -> bonus bBaseAtk,10;
//	A cada 2 refinos: ATQ +10.     -> bonus bBaseAtk,10*(getrefine()/2);
//	Refino +9 ou mais: ...         -> if (getrefine() >= 9) { ... }
//	A cada 10 níveis de base até o 200: ...
//	Conjunto [Item]: ...           -> an item combo
//
// Phrase -> bonus names were learned from items that have both an rAthena
// script and a client description (see the mapping table below). Lines that
// are not understood (procs, autocasts, event text) are counted, not guessed.

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/gamedata"
)

// DescBonuses fills DescScript for every item that has no rAthena script
// and adds the combos ("Conjunto") found in descriptions, skipping sets
// rAthena already has. The calc uses DescScript only for items actually
// equipped or slotted, so consumables' text never counts.
func DescBonuses(c *gamedata.Catalog) (scripts, combos int) {
	names, skills := descMaps(c)
	key := func(ids []int) string {
		s := slices.Clone(ids)
		slices.Sort(s)
		return fmt.Sprint(s)
	}
	have := map[string]bool{}
	for _, cb := range c.Combos {
		have[key(cb.Items)] = true
	}
	ids := make([]int, 0, len(c.Items))
	for id := range c.Items {
		ids = append(ids, id)
	}
	slices.Sort(ids) // deterministic output
	for _, id := range ids {
		it := c.Items[id]
		it.EventScript = EventScript(it.Desc["pt"], skills)
		if it.Script != "" {
			continue
		}
		s, cbs, skipped := DescScript(id, it.Desc["pt"], names, skills)
		if s != "" {
			it.DescScript = s
			scripts++
		}
		it.DescSkipped = skipped
		for _, cb := range cbs {
			if k := key(cb.Items); !have[k] && !setWithHost(c, cb) {
				have[k] = true
				c.Combos = append(c.Combos, gamedata.Combo{Items: cb.Items, Script: cb.Script})
				combos++
			}
		}
	}
	return scripts, combos
}

// setWithHost reports a set rAthena has too, with the item its enchant goes
// on counted as one more piece (Dom Celestial + Báculo Celestial, where
// rAthena also asks for the crown): both would count twice.
func setWithHost(c *gamedata.Catalog, set DescCombo) bool {
	parts := func(s string) []string {
		var out []string
		for _, p := range strings.Split(s, ";") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		slices.Sort(out)
		return out
	}
	hosts := func(item int) bool { // takes an enchant of the set
		for _, l := range c.Enchants[item] {
			for _, e := range l {
				if slices.Contains(set.Items, e) {
					return true
				}
			}
		}
		return false
	}
	want := parts(set.Script)
	for _, cb := range c.Combos {
		if len(cb.Items) <= len(set.Items) || !slices.Equal(parts(cb.Script), want) {
			continue
		}
		all := true
		for _, id := range set.Items {
			all = all && slices.Contains(cb.Items, id)
		}
		for _, id := range cb.Items {
			all = all && (slices.Contains(set.Items, id) || hosts(id))
		}
		if all {
			return true
		}
	}
	return false
}

var (
	reColor   = regexp.MustCompile(`\^[0-9a-fA-F]{6}`)
	reTags    = regexp.MustCompile(`<[^>]+>`)
	reValue   = regexp.MustCompile(`^(.*?)\s*([+-]\s?\d+(?:[.,]\d+)?)\s*(%|segundos?|s)?\.?$`)
	reEach    = regexp.MustCompile(`(?i)^a cada (\d+) refinos?(?: d[oa] [^:]+)?:?$`)
	reRefine  = regexp.MustCompile(`(?i)^refino \+(\d+) ou mais:?$`)
	// A set's lines on the weapon: "Arma com refino +10 ou mais:", "Arma Grau C ou mais:".
	reWRefine = regexp.MustCompile(`(?i)^arma com refino \+(\d+) ou mais:?$`)
	reWGrade  = regexp.MustCompile(`(?i)^arma (?:com )?grau ([A-D]) ou mais:?$`)
	reBaseLv  = regexp.MustCompile(`(?i)^a cada (\d+) n[ií]veis de base(?: at[eé] o (?:n[ií]vel )?(\d+))?:?$`)
	reIgnore  = regexp.MustCompile(`(?i)^ignora (\d+)% da (DEF e DEFM|DEFM|DEF)(?: d[aeo]s?)? (.+?)(?: adicional)?\.?$`)
	reSkill   = regexp.MustCompile(`(?i)^dano (?:de|da habilidade) \[([^\]]+)\]$`)
	reAgainst = regexp.MustCompile(`(?i)^dano (f[ií]sico e m[aá]gico|f[ií]sico|m[aá]gico) contra (.+)$`)
	reMagEle  = regexp.MustCompile(`(?i)^dano m[aá]gico de propriedade (\S+)$`)
	reResist  = regexp.MustCompile(`(?i)^resist[eê]ncia a (?:danos de )?(propriedade|ra[cç]a) (.+)$`)
	reSlots   = regexp.MustCompile(`\s*\[\d\]\s*$`)
)

// Fixed phrases (lowercase). Values are [flat, percent] bonus names; "" =
// that form doesn't exist.
var descFixed = map[string][2]string{
	"for": {"bStr", ""}, "agi": {"bAgi", ""}, "vit": {"bVit", ""}, "int": {"bInt", ""},
	"des": {"bDex", ""}, "sor": {"bLuk", ""}, "todos os atributos": {"bAllStats", ""},
	"pod": {"bPow", ""}, "vig": {"bSta", ""}, "sab": {"bWis", ""}, "fei": {"bSpl", ""},
	"con": {"bCon", ""}, "crv": {"bCrt", ""}, "todos os atributos de traço": {"bAllTraitStats", ""},
	"atq": {"bBaseAtk", "bAtkRate"}, "atqm": {"bMatk", "bMatkRate"},
	"p.atq": {"bPAtk", ""}, "s.atqm": {"bSMatk", ""}, "t.crít": {"bCRate", ""},
	"res": {"bRes", ""}, "mres": {"bMRes", ""},
	"def": {"bDef", ""}, "defm": {"bMdef", ""},
	"hp": {"bMaxHP", "bMaxHPrate"}, "hp máx.": {"bMaxHP", "bMaxHPrate"}, "hp máximo": {"bMaxHP", "bMaxHPrate"},
	"sp": {"bMaxSP", "bMaxSPrate"}, "sp máx.": {"bMaxSP", "bMaxSPrate"}, "sp máximo": {"bMaxSP", "bMaxSPrate"},
	"crít": {"bCritical", ""}, "cri": {"bCritical", ""}, "precisão": {"bHit", ""}, "esquiva": {"bFlee", ""},
	"esquiva perfeita": {"bFlee2", ""}, "precisão perfeita": {"bPerfectHitAddRate", "bPerfectHitAddRate"},
	"velocidade de ataque": {"bAspd", "bAspdRate"},
	"conjuração variável":  {"", "bVariableCastrate"}, "conjuração fixa": {"", "bFixedCastrate"},
	"pós-conjuração": {"", "bDelayrate"},
	"dano físico":    {"", "bAtkRate"}, "dano mágico": {"", "bMatkRate"},
	"dano físico à distância": {"", "bLongAtkRate"}, "dano físico corpo a corpo": {"", "bShortAtkRate"},
	"dano crítico": {"", "bCritAtkRate"}, "efetividade de cura": {"", "bHealPower"},
	"cura recebida":                           {"", "bHealPower2"},
	"custo de sp das habilidades":             {"", "bUseSPrate"},
	"dano mágico de todas as propriedades":    {"", "bMagicAtkEle Ele_All"},
	"resistência a todas as propriedades":     {"", "bSubEle Ele_All"},
	"resistência a todas as raças":            {"", "bSubRace RC_All"},
	"resistência a danos físicos à distância": {"", "bLongAtkDef"},
}

var descRace = map[string]string{
	"amorfo": "RC_Formless", "morto-vivo": "RC_Undead", "bruto": "RC_Brute", "planta": "RC_Plant",
	"inseto": "RC_Insect", "peixe": "RC_Fish", "demônio": "RC_Demon", "humanoide": "RC_DemiHuman",
	"anjo": "RC_Angel", "dragão": "RC_Dragon", "humano": "RC_Player_Human", "doram": "RC_Player_Doram",
}

var descEle = map[string]string{
	"neutro": "Ele_Neutral", "água": "Ele_Water", "terra": "Ele_Earth", "fogo": "Ele_Fire",
	"vento": "Ele_Wind", "veneno": "Ele_Poison", "sagrado": "Ele_Holy", "sombrio": "Ele_Dark",
	"fantasma": "Ele_Ghost", "maldito": "Ele_Undead",
}

var descSize = map[string]string{"pequeno": "Size_Small", "médio": "Size_Medium", "grande": "Size_Large"}

// DescCombo is an item set found in a description ("Conjunto").
type DescCombo struct {
	Items  []int
	Script string
}

// descMaps are the lowercase PT item names (-> ids) and skill names (->
// codes) that descriptions refer to.
func descMaps(c *gamedata.Catalog) (map[string][]int, map[string]string) {
	names := map[string][]int{}
	for id, it := range c.Items {
		n := strings.ToLower(reSlots.ReplaceAllString(it.Name.Get("pt"), ""))
		names[n] = append(names[n], id)
	}
	for _, ids := range names {
		slices.Sort(ids)
	}
	// Two skills can share a PT name (Impacto Cristalino and its _ATK
	// part): the lowest ID, the one the player casts, wins, every run;
	// monster skills (NPC_) only when no player skill has the name.
	skills := map[string]string{}
	skillIDs := make([]int, 0, len(c.Skills))
	for id := range c.Skills {
		skillIDs = append(skillIDs, id)
	}
	slices.SortFunc(skillIDs, func(a, b int) int {
		na, nb := strings.HasPrefix(c.Skills[a].Code, "NPC_"), strings.HasPrefix(c.Skills[b].Code, "NPC_")
		if na != nb {
			if na {
				return 1
			}
			return -1
		}
		return a - b
	})
	for _, id := range skillIDs {
		sk := c.Skills[id]
		if n := strings.ToLower(sk.Name.Get("pt")); n != "" && skills[n] == "" {
			skills[n] = sk.Code
		}
	}
	return names, skills
}

// DescReader reads any item's description as DescScript does (for items
// that have an rAthena script too): the script and the lines skipped.
func DescReader(c *gamedata.Catalog) func(id int, it *gamedata.Item) (string, int) {
	names, skills := descMaps(c)
	return func(id int, it *gamedata.Item) (string, int) {
		if it.Script == "" {
			return it.DescScript, it.DescSkipped
		}
		s, _, skipped := DescScript(id, it.Desc["pt"], names, skills)
		return s, skipped
	}
}

// DescScript translates a PT description. names maps lowercase item names
// (without "[n]") to IDs; skills maps lowercase PT skill names to codes.
func DescScript(self int, desc []string, names map[string][]int, skills map[string]string) (script string, combos []DescCombo, skipped int) {
	type block struct {
		mult    string  // "*(getrefine()/2)"
		cond    string  // "getrefine() >= 9"
		pre     string  // statements before (base level cap)
		skip    bool    // event text, other items' refine
		combo   [][]int // groups of alternatives ("A ou B")
		inCombo bool
		altNext bool // the previous name ended in "ou"
	}
	var out strings.Builder
	var cur block
	var comboBody strings.Builder
	flushCombo := func() {
		if len(cur.combo) > 0 && comboBody.Len() > 0 {
			for _, set := range product(cur.combo, 16) {
				combos = append(combos, DescCombo{Items: append([]int{self}, set...), Script: comboBody.String()})
			}
		}
		cur.combo, cur.inCombo = nil, false
		comboBody.Reset()
	}
	emit := func(stmt string) {
		if cur.skip {
			return
		}
		if cur.pre != "" {
			stmt = cur.pre + " " + stmt
		}
		if cur.cond != "" {
			stmt = "if (" + cur.cond + ") { " + stmt + " }"
		}
		if cur.inCombo {
			comboBody.WriteString(stmt + "\n")
		} else {
			out.WriteString(stmt + "\n")
		}
	}

	for _, raw := range desc {
		line := strings.TrimSpace(reTags.ReplaceAllString(reColor.ReplaceAllString(raw, ""), ""))
		low := strings.ToLower(line)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "---"):
			flushCombo()
			cur = block{}
			continue
		case strings.HasPrefix(low, "tipo:"), strings.HasPrefix(low, "peso:"), strings.HasPrefix(low, "nível necessário"),
			strings.HasPrefix(low, "classes:"), strings.HasPrefix(low, "equipa em"), strings.HasPrefix(low, "def:"),
			strings.HasPrefix(low, "ataque:"), strings.HasPrefix(low, "propriedade:"), strings.HasPrefix(low, "nível da arma"),
			strings.HasPrefix(low, "pode ser refinado"), strings.HasPrefix(low, "não pode ser"), strings.HasPrefix(low, "pode receber"):
			continue
		case strings.HasPrefix(low, "[durante o evento"), strings.HasPrefix(low, "[evento"):
			cur.skip = true
			continue
		case low == "conjunto" || strings.HasPrefix(low, "conjunto:"):
			flushCombo()
			cur = block{inCombo: true}
			continue
		}
		if m := reEach.FindStringSubmatch(line); m != nil {
			cur.mult, cur.cond, cur.pre = "*(getrefine()/"+m[1]+")", "", ""
			if strings.Contains(low, " do ") || strings.Contains(low, " da ") {
				cur.skip = true // another item's refine (combo pieces)
			}
			continue
		}
		if m := reRefine.FindStringSubmatch(line); m != nil {
			cur.cond, cur.mult, cur.pre = "getrefine() >= "+m[1], "", ""
			continue
		}
		if m := reBaseLv.FindStringSubmatch(line); m != nil {
			capLv := m[2]
			if capLv == "" {
				capLv = "999"
			}
			cur.pre = ".@lv = BaseLevel; if (.@lv > " + capLv + ") { .@lv = " + capLv + "; }"
			cur.mult, cur.cond = "*(.@lv/"+m[1]+")", ""
			continue
		}
		if cur.inCombo {
			if m := reWRefine.FindStringSubmatch(line); m != nil {
				cur.cond, cur.mult, cur.pre = "getequiprefinerycnt(EQI_HAND_R) >= "+m[1], "", ""
				continue
			}
			if m := reWGrade.FindStringSubmatch(line); m != nil {
				cur.cond, cur.mult, cur.pre = "getenchantgrade(EQI_HAND_R) >= ENCHANTGRADE_"+strings.ToUpper(m[1]), "", ""
				continue
			}
		}
		if strings.HasSuffix(low, ":") && strings.Contains(low, "refino") { // "Bota com refino +10 ou mais:"
			cur.skip = true
			continue
		}
		// A proc line ("3% de chance de ...") is the set's text, not a piece.
		isProc := strings.Contains(low, "chance")
		if cur.inCombo && comboBody.Len() == 0 && len(cur.combo) > 0 && isProc {
			skipped++
			continue
		}
		if cur.inCombo && comboBody.Len() == 0 && !cur.skip && !isProc && !reValue.MatchString(strings.TrimSuffix(line, ".")) && !strings.HasPrefix(low, "ignora") {
			// item names of the set: "[Item]", "Item [1]", "Item A ou"
			alt := strings.HasSuffix(low, " ou")
			name := strings.ToLower(reSlots.ReplaceAllString(strings.Trim(strings.TrimSuffix(strings.TrimSuffix(line, " ou"), "."), "[] "), ""))
			ids := names[name]
			if len(ids) == 0 {
				cur.skip = true // unknown piece: drop the combo
				continue
			}
			if cur.altNext && len(cur.combo) > 0 {
				cur.combo[len(cur.combo)-1] = append(cur.combo[len(cur.combo)-1], ids...)
			} else {
				cur.combo = append(cur.combo, append([]int(nil), ids...))
			}
			cur.altNext = alt
			continue
		}
		for _, sent := range splitSentences(line) {
			if st, ok := descStatement(sent, cur.mult, skills); ok {
				emit(st)
			} else if !cur.skip && strings.ContainsAny(sent, "+-%") {
				skipped++
			}
		}
	}
	flushCombo()
	return out.String(), combos, skipped
}

// EventScript is the script of the description's "[Durante o Evento]" block
// (up to the next separator or set); lines about drops and EXP give nothing.
func EventScript(desc []string, skills map[string]string) string {
	var out strings.Builder
	in := false
	for _, raw := range desc {
		line := strings.TrimSpace(reTags.ReplaceAllString(reColor.ReplaceAllString(raw, ""), ""))
		low := strings.ToLower(line)
		switch {
		case strings.HasPrefix(low, "[durante o evento"):
			in = true
			continue
		case strings.HasPrefix(line, "-"), low == "conjunto", strings.HasSuffix(line, ":"):
			in = false
		}
		if !in {
			continue
		}
		line = strings.Replace(line, "todas as raças de monstro", "todas as raças", 1)
		for _, sent := range splitSentences(line) {
			if st, ok := descStatement(sent, "", skills); ok {
				out.WriteString(st + "\n")
			}
		}
	}
	return out.String()
}

func splitSentences(line string) []string {
	var out []string
	for _, s := range strings.SplitAfter(line, ". ") {
		if s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ".")); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// descStatement translates one sentence into a bonus statement.
func descStatement(sent, mult string, skills map[string]string) (string, bool) {
	low := strings.ToLower(sent)
	if strings.Contains(low, "chance") || strings.Contains(low, "ao atacar") || strings.Contains(low, "ao receber") {
		return "", false // procs and autocasts
	}
	if m := reIgnore.FindStringSubmatch(sent); m != nil {
		races := descTargets(m[3], descRace, "RC_All")
		if races == nil {
			return "", false
		}
		var b strings.Builder
		for _, rc := range races {
			if strings.Contains(m[2], "DEF") && m[2] != "DEFM" {
				fmt.Fprintf(&b, "bonus2 bIgnoreDefRaceRate,%s,%s; ", rc, m[1]+mult)
			}
			if strings.Contains(m[2], "DEFM") {
				fmt.Fprintf(&b, "bonus2 bIgnoreMdefRaceRate,%s,%s; ", rc, m[1]+mult)
			}
		}
		return strings.TrimSpace(b.String()), true
	}
	// "... +10% adicional": the same bonus, on top of an earlier line.
	if t, ok := strings.CutSuffix(strings.TrimSuffix(sent, "."), " adicional"); ok {
		sent = t
	}
	m := reValue.FindStringSubmatch(sent)
	if m == nil {
		return "", false
	}
	phrase := strings.ToLower(strings.TrimSpace(m[1]))
	// "POD e P.ATQ +5": two fixed phrases with one value.
	if a, b, ok := strings.Cut(phrase, " e "); ok {
		if _, okA := descFixed[a]; okA {
			if _, okB := descFixed[b]; okB {
				sa, okA := descStatement(a+" "+m[2]+m[3], mult, skills)
				sb, okB := descStatement(b+" "+m[2]+m[3], mult, skills)
				if okA && okB {
					return sa + " " + sb, true
				}
				return "", false
			}
		}
	}
	num, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(m[2], " ", ""), ",", "."), 64)
	if err != nil {
		return "", false
	}
	unit := m[3]
	if unit == "s" || strings.HasPrefix(unit, "segundo") {
		if phrase == "conjuração fixa" {
			return fmt.Sprintf("bonus bFixedCast,%d%s;", int(num*1000), mult), true
		}
		return "", false
	}
	val := strconv.Itoa(int(num)) + mult
	if num != float64(int(num)) {
		return "", false
	}
	pct := unit == "%"
	if names, ok := descFixed[phrase]; ok {
		name := names[0]
		if pct {
			name = names[1]
		}
		if name == "" {
			return "", false
		}
		if k, arg, two := strings.Cut(name, " "); two {
			return fmt.Sprintf("bonus2 %s,%s,%s;", k, arg, val), true
		}
		return fmt.Sprintf("bonus %s,%s;", name, val), true
	}
	if !pct {
		return "", false
	}
	if s := reSkill.FindStringSubmatch(phrase); s != nil {
		if code := skills[strings.ToLower(strings.TrimSpace(s[1]))]; code != "" {
			return fmt.Sprintf("bonus2 bSkillAtk,\"%s\",%s;", code, val), true
		}
		return "", false
	}
	if s := reMagEle.FindStringSubmatch(phrase); s != nil {
		if e := descEle[s[1]]; e != "" {
			return fmt.Sprintf("bonus2 bMagicAtkEle,%s,%s;", e, val), true
		}
		return "", false
	}
	if s := reResist.FindStringSubmatch(phrase); s != nil {
		if strings.HasPrefix(s[1], "prop") {
			if e := descEle[s[2]]; e != "" {
				return fmt.Sprintf("bonus2 bSubEle,%s,%s;", e, val), true
			}
			return "", false
		}
		if rcs := descTargets(s[2], descRace, "RC_All"); rcs != nil {
			return joinBonus("bSubRace", rcs, val), true
		}
		return "", false
	}
	if s := reAgainst.FindStringSubmatch(phrase); s != nil {
		phys := strings.HasPrefix(s[1], "f")
		magic := strings.Contains(s[1], "m")
		target := s[2]
		var kind string
		var args []string
		switch {
		case strings.Contains(target, "tamanho"):
			kind, args = "Size", descTargets(target, descSize, "Size_All")
		case strings.Contains(target, "propriedade"):
			kind, args = "Ele", descTargets(target, descEle, "Ele_All")
		case strings.Contains(target, "raça") || strings.Contains(target, "raças"):
			kind, args = "Race", descTargets(target, descRace, "RC_All")
		case strings.Contains(target, "normais e chefes") || strings.Contains(target, "todas as classes"):
			kind, args = "Class", []string{"Class_All"}
		case strings.Contains(target, "chefe"):
			kind, args = "Class", []string{"Class_Boss"}
		}
		if args == nil {
			return "", false
		}
		var b strings.Builder
		if phys {
			b.WriteString(joinBonus("bAdd"+kind, args, val) + " ")
		}
		if magic {
			b.WriteString(joinBonus("bMagicAdd"+kind, args, val))
		}
		return strings.TrimSpace(b.String()), true
	}
	return "", false
}

// descTargets reads "todas as raças", "a raça Bruto", "raças Amorfo e
// Humanoide", "oponentes de tamanho Pequeno e Médio"... nil = unknown name.
func descTargets(s string, table map[string]string, all string) []string {
	low := strings.ToLower(s)
	if strings.Contains(low, "todas") || strings.Contains(low, "todos") {
		return []string{all}
	}
	var out []string
	for _, w := range regexp.MustCompile(`[\p{L}-]+`).FindAllString(low, -1) {
		if v, ok := table[w]; ok {
			out = append(out, v)
		}
	}
	return out
}

func joinBonus(name string, args []string, val string) string {
	var b strings.Builder
	for _, a := range args {
		fmt.Fprintf(&b, "bonus2 %s,%s,%s; ", name, a, val)
	}
	return strings.TrimSpace(b.String())
}

// product expands [[a,b],[c]] into [[a,c],[b,c]], at most limit sets.
func product(groups [][]int, limit int) [][]int {
	out := [][]int{nil}
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		var next [][]int
		for _, p := range out {
			for _, id := range g {
				if len(next) >= limit {
					break
				}
				next = append(next, append(append([]int(nil), p...), id))
			}
		}
		out = next
	}
	if len(out) == 1 && out[0] == nil {
		return nil
	}
	return out
}

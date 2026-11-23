package view

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

const maxBuildBody = 64 << 10

// slotInfo describes one Equipment window slot.
type slotInfo struct {
	Key   string
	Label string
	Side  string // "L" or "R" in the Equipment window
}

var equipSlots = []slotInfo{
	{calc.HeadTop, "Topo", "L"}, {calc.HeadMid, "Meio", "R"}, {calc.HeadLow, "Baixo", "L"},
	{calc.Armor, "Armadura", "R"}, {calc.Weapon, "Arma", "L"}, {calc.Shield, "Escudo", "R"},
	{calc.Garment, "Capa", "L"}, {calc.Shoes, "Sapatos", "R"}, {calc.AccRight, "Acessório D", "L"},
	{calc.AccLeft, "Acessório E", "R"},
}

var shadowSlots = []slotInfo{
	{calc.ShArmor, "Armadura Sombria", "L"}, {calc.ShWeapon, "Arma Sombria", "R"},
	{calc.ShShield, "Escudo Sombrio", "L"}, {calc.ShShoes, "Sapatos Sombrios", "R"},
	{calc.ShAccR, "Brinco Sombrio", "L"}, {calc.ShAccL, "Colar Sombrio", "R"},
}

// costumeSlots are the Visual tab: each costume holds one enchant stone.
var costumeSlots = []slotInfo{
	{calc.CosTop, "Visual Topo", "L"}, {calc.CosMid, "Visual Meio", "R"},
	{calc.CosLow, "Visual Baixo", "L"}, {calc.CosGarment, "Visual Capa", "R"},
}

func slotLabel(key string) string {
	for _, s := range append(append(slices.Clone(equipSlots), shadowSlots...), costumeSlots...) {
		if s.Key == key {
			return s.Label
		}
	}
	return key
}

// Share codes are base64url of the build JSON, deflated and prefixed with
// "z" (like Path of Building's codes, so links stay short). Codes without
// the prefix are the older plain JSON ones (they always start with "ey").
const codeDeflate = "z"

// decodeBuild reads a share code, compressed or not.
func decodeBuild(code string) (calc.Build, bool) {
	var b calc.Build
	z := strings.HasPrefix(code, codeDeflate)
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, codeDeflate))
	if err != nil || len(raw) > maxBuildBody {
		return b, false
	}
	if z {
		// Read at most maxBuildBody+1 bytes: a bigger build is refused.
		raw, err = io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(raw)), maxBuildBody+1))
		if err != nil || len(raw) > maxBuildBody {
			return b, false
		}
	}
	return b, json.Unmarshal(raw, &b) == nil
}

func encodeBuild(b calc.Build) string {
	raw, _ := json.Marshal(b)
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.BestCompression)
	w.Write(raw)
	w.Close()
	return codeDeflate + base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func defaultBuild(c *gamedata.Catalog) calc.Build {
	b := calc.Build{Job: 4252, BaseLv: 200, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1}, Target: DefaultTarget(c)}
	if _, ok := c.Jobs[b.Job]; !ok {
		b.Job = 0
	}
	return b
}

// trainingDummyNeutral is Prontera's Training Dummy (Neutral): Medium,
// Formless, Neutral 1, no DEF.
const trainingDummyNeutral = 21067

// DefaultTarget is the target a build starts with: the neutral training
// dummy (0 when the catalog lacks it).
func DefaultTarget(c *gamedata.Catalog) int {
	if c.Monster(trainingDummyNeutral) != nil {
		return trainingDummyNeutral
	}
	return 0
}

// buildView is everything the build templates render.
type buildView struct {
	Lang     string
	Build    calc.Build
	Code     string
	Job      *gamedata.Job
	JobName  string
	Jobs     []jobOption
	R        calc.Result
	StatName [6]string
	TraitNm  [6]string
	Slots    []slotView
	Shadow   []slotView
	Costume  []slotView // Visual tab
	Bonuses  []bonusLine
	Extras   []bonusLine
	Combos   []comboView   // active item combos and what they add
	Families []classFamily // the class picker
	Buffs    buffsView
	HPPct    int
	SPPct    int
	Notes    []string // items removed by normalization (shown in the chat)
	D        damageView
}

type slotView struct {
	Key, Label, Side string
	Item             *itemView
	Refine           int
	Grade            int
	Opts             []optLine   // random options
	MaxOpts          int         // random options the item takes (0: none known)
	OptsAny          bool        // only the count is known: any option goes
	Cards            []*itemView // len = item slots
	Refineable       bool
	Gradable         bool
	CardSlots        int // positions below this are card slots; above, enchants
}

// optLine is a random option on an item, as shown and edited.
type optLine struct {
	ID, Value int
	Label     string
	Min, Max  int // the value range this position rolls (0, 0: unknown)
}

// comboView is an active item combo: its items and what it adds.
type comboView struct {
	Name    string
	Lines   []bonusLine
	Skipped int // effects not simulated (procs, granted skills...)
}

// sortedKeys lists the non-zero entries of a bonus map by key.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

type bonusLine struct {
	Label string
	Value string
}

var statNames = [6]string{"FOR", "AGI", "VIT", "INT", "DES", "SOR"}
var traitNames = [6]string{"POW", "STA", "WIS", "SPL", "CON", "CRT"}

func (s *Server) buildView(b calc.Build, l string) buildView {
	c := s.opt.Catalog
	notes := calc.Normalize(&b, c)
	r := calc.Compute(b, c)
	for i := range notes {
		notes[i] = tr(l, notes[i])
	}
	v := buildView{Lang: l, Build: b, Code: encodeBuild(b), Job: c.Jobs[b.Job], R: r, StatName: statNames, TraitNm: traitNames, Notes: notes}
	if v.Job != nil {
		v.JobName = v.Job.Name.Get(l)
	}
	hiddenJobs := s.hiddenJobs(l, b.Job)
	for _, j := range c.Jobs {
		if len(j.Tree) == 0 || j.Name.Get(l) == "" || !j.HasStats() || hiddenJobs[j.ID] {
			continue
		}
		v.Jobs = append(v.Jobs, jobOption{ID: j.ID, Name: j.Name.Get(l), Selected: j.ID == b.Job})
	}
	sort.Slice(v.Jobs, func(i, k int) bool { return v.Jobs[i].ID < v.Jobs[k].ID })

	mk := func(info slotInfo) slotView {
		sv := slotView{Key: info.Key, Label: info.Label, Side: info.Side}
		if e, ok := b.Equip[info.Key]; ok {
			if it := c.Items[e.Item]; it != nil {
				iv := viewItem(it, l)
				sv.Item, sv.Refine, sv.Refineable = &iv, e.Refine, it.Refineable
				sv.Grade, sv.Gradable = e.Grade, it.Gradable
				// Only the random options this item can get, and how many.
				spec := c.ItemOpts[it.ID]
				sv.MaxOpts = spec.Slots()
				sv.OptsAny = spec != nil && spec.Any > 0
				for _, o := range e.Opts {
					if ro := c.RandomOpts[o[0]]; ro != nil {
						line := optLine{ID: o[0], Value: o[1], Label: ro.Label(l, o[1])}
						for _, r := range spec.At(len(sv.Opts)) {
							if r.ID == o[0] {
								line.Min, line.Max = r.Min, r.Max
							}
						}
						sv.Opts = append(sv.Opts, line)
					}
				}
				// Show the card slots plus any enchant positions in use.
				n := min(it.Slots, 4)
				for i := 3; i >= n; i-- {
					if e.Cards[i] != 0 {
						n = i + 1
						break
					}
				}
				// ...and every position the client's enchant list opens.
				for k := range c.Enchants[it.ID] {
					n = max(n, min(k+1, 4))
				}
				if calc.IsCostume(info.Key) {
					// One enchant stone, up to the position it sits in.
					n = 1
					for i := 3; i > 0; i-- {
						if e.Cards[i] != 0 {
							n = i + 1
							break
						}
					}
				}
				sv.CardSlots = it.Slots
				for i := 0; i < n; i++ {
					var cv *itemView
					if ci := c.Items[e.Cards[i]]; e.Cards[i] > 0 && ci != nil {
						x := viewItem(ci, l)
						cv = &x
					}
					sv.Cards = append(sv.Cards, cv)
				}
			}
		}
		return sv
	}
	for _, info := range equipSlots {
		v.Slots = append(v.Slots, mk(info))
	}
	for _, info := range shadowSlots {
		v.Shadow = append(v.Shadow, mk(info))
	}
	for _, info := range costumeSlots {
		v.Costume = append(v.Costume, mk(info))
	}
	for _, kv := range r.SortedBonuses() {
		lb, val := bonusText(kv[0].(string), kv[1].(int), c, l)
		v.Bonuses = append(v.Bonuses, bonusLine{Label: lb, Value: val})
	}
	keys := make([]string, 0, len(r.Extra))
	for k, val := range r.Extra {
		if val != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		lb, val := bonusText(k, r.Extra[k], c, l)
		v.Extras = append(v.Extras, bonusLine{Label: lb, Value: val})
	}
	for _, ce := range r.ComboEffects {
		names := make([]string, 0, len(ce.Items))
		for _, id := range ce.Items {
			if it := c.Items[id]; it != nil {
				names = append(names, it.Name.Get(l))
			}
		}
		cv := comboView{Name: strings.Join(names, " + "), Skipped: ce.Skipped}
		for _, k := range sortedKeys(ce.Bonus) {
			lb, val := bonusText(k, ce.Bonus[k], c, l)
			cv.Lines = append(cv.Lines, bonusLine{Label: lb, Value: val})
		}
		for _, k := range sortedKeys(ce.Extra) {
			lb, val := bonusText(k, ce.Extra[k], c, l)
			cv.Lines = append(cv.Lines, bonusLine{Label: lb, Value: val})
		}
		v.Combos = append(v.Combos, cv)
	}
	v.Buffs = buffsWindow(b, c, l)
	v.Buffs.Items = itemEffectRows(r, c, l)
	v.Buffs.Pet = petWindow(b, r, c, l)
	v.HPPct, v.SPPct = 100, 100
	v.D = s.damageView(b, r, l)
	v.D.Table = s.skillTable(b, r, l)
	v.Families = s.classFamilies(l, b.Job)
	return v
}

// pickItems lists the items (or cards, card = true) a slot takes, matching
// q. idx >= 0 is the card position being picked on item; an enchant
// position of an item the client's enchant list covers offers only the
// enchants listed for it.
func (s *Server) pickItems(slot string, card bool, idx, item, jobID int, q, l string) []itemView {
	job := s.opt.Catalog.Jobs[jobID]
	var allowed map[int]bool
	if card && idx >= 0 {
		if it := s.opt.Catalog.Items[item]; it != nil && idx >= it.Slots {
			if ench, ok := s.opt.Catalog.Enchants[it.ID]; ok {
				allowed = map[int]bool{}
				for _, id := range ench[idx] {
					allowed[id] = true
				}
			}
		}
	}
	var pool []*gamedata.Item
	if q != "" {
		pool = s.items.Search(q, 5000)
	} else {
		for _, it := range s.opt.Catalog.Items {
			pool = append(pool, it)
		}
		sort.Slice(pool, func(i, j int) bool { return pool[i].ID < pool[j].ID })
	}
	var out []itemView
	for _, it := range pool {
		known := it.Type != "" // the picker only offers items whose slot rules are known
		ok := known && ((card && calc.CardFits(it, slot)) || (!card && calc.Fits(it, slot) && job.CanEquip(it)))
		if allowed != nil {
			ok = allowed[it.ID]
		}
		if ok && it.Name.Get(l) != "" {
			out = append(out, viewItem(it, l))
			if len(out) >= searchLimit {
				break
			}
		}
	}
	return out
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// skillTreeData prepares the skill window for a job (shared by /skills and /build).
func (s *Server) skillTreeData(jobID, jobLv int, l string, levels map[int]int) map[string]any {
	c := s.opt.Catalog
	if _, ok := c.Jobs[jobID]; !ok {
		return nil
	}
	chain := c.Chain(jobID)
	groups := c.SkillBudgets(jobID, jobLv)
	groupOf := map[int]int{}
	for gi, g := range groups {
		for _, id := range g.Jobs {
			groupOf[id] = gi
		}
	}
	js := map[int]skillJS{}
	var tabs []skillTab
	for _, id := range chain {
		j := c.Jobs[id]
		if len(j.Tree) == 0 {
			continue
		}
		maxPos := 0
		for pos := range j.Tree {
			maxPos = max(maxPos, pos)
		}
		rows := make([][]skillCell, maxPos/gamedata.TreeColumns+1)
		for i := range rows {
			rows[i] = make([]skillCell, gamedata.TreeColumns)
			for k := range rows[i] {
				rows[i][k].Empty = true
			}
		}
		for pos, sid := range j.Tree {
			sk := c.Skills[sid]
			if sk == nil {
				continue
			}
			rows[pos/gamedata.TreeColumns][pos%gamedata.TreeColumns] = skillCell{ID: sid, Name: sk.Name.Get(l), MaxLv: sk.MaxLv, Desc: sk.Desc.Get(l)}
			var needs [][2]int
			for _, n := range sk.NeedsFor(chain) {
				needs = append(needs, [2]int{n.Skill, n.Level})
			}
			if _, seen := js[sid]; !seen { // a skill shown in two tabs belongs to the first
				js[sid] = skillJS{Max: sk.MaxLv, Needs: needs, Group: groupOf[id], Free: sk.Free()}
			}
		}
		tabs = append(tabs, skillTab{JobID: id, Name: j.Name.Get(l), Rows: rows})
	}
	if levels == nil {
		levels = map[int]int{}
	}
	return map[string]any{"Tabs": tabs, "Tree": map[string]any{"job": jobID, "skills": js, "levels": levels, "groups": groups}}
}

func signed(v int, key string) string {
	name, _, _ := strings.Cut(key, " ")
	if effChance[name] { // chances are stored in 1/100 %
		return fmt.Sprintf("%+g%%", float64(v)/100)
	}
	s := strconv.Itoa(v)
	if v > 0 {
		s = "+" + s
	}
	if isPercent(key) {
		s += "%"
	}
	return s
}

// isPercent reports whether a bonus is a percentage (rates and damage modifiers).
func isPercent(key string) bool {
	name, _, _ := strings.Cut(key, " ")
	if strings.HasSuffix(name, "rate") || strings.HasSuffix(name, "Rate") {
		return true
	}
	for _, p := range []string{"bAdd", "bSub", "bMagicAdd", "bMagicAtkEle", "bSkillAtk", "bIgnore"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

var bonusLabels = map[string]string{
	"bStr": "FOR", "bAgi": "AGI", "bVit": "VIT", "bInt": "INT", "bDex": "DES", "bLuk": "SOR",
	"bAllStats": "Todos os atributos", "bAgiVit": "AGI e VIT", "bAgiDexStr": "AGI, DES e FOR",
	"bPow": "POW", "bSta": "STA", "bWis": "WIS", "bSpl": "SPL", "bCon": "CON", "bCrt": "CRT", "bAllTraitStats": "Todos os traços",
	"bMaxHP": "HP Máx.", "bMaxHPrate": "HP Máx.", "bMaxSP": "SP Máx.", "bMaxSPrate": "SP Máx.", "bMaxAP": "AP Máx.",
	"bBaseAtk": "ATQ base", "bAtk": "ATQ", "bAtk2": "ATQ", "bAtkRate": "ATQ", "bWeaponAtkRate": "ATQ da arma",
	"bMatk": "ATQM", "bMatkRate": "ATQM", "bWeaponMatkRate": "ATQM da arma",
	"bHit": "Precisão", "bHitRate": "Precisão", "bFlee": "Esquiva", "bFleeRate": "Esquiva", "bFlee2": "Esquiva perfeita",
	"bCritical": "Crítico", "bCriticalRate": "Crítico", "bDef": "DEF", "bDefRate": "DEF", "bDef2": "DEF suave",
	"bMdef": "DEFM", "bMdefRate": "DEFM", "bMdef2": "DEFM suave",
	"bAspd": "ASPD", "bAspdRate": "ASPD", "bLongAtkRate": "Dano à distância", "bShortAtkRate": "Dano corpo a corpo", "bPerfectHitAddRate": "Precisão perfeita", "bCritAtkRate": "Dano crítico",
	"bPAtk": "P.ATQ", "bSMatk": "S.ATQM", "bRes": "RES", "bMRes": "MRES", "bHPlus": "H.PLUS", "bCRate": "C.RATE",
	"bVariableCastrate": "Conjuração variável", "bFixedCastrate": "Conjuração fixa", "bFixedCast": "Conjuração fixa (ms)",
	"bDelayrate": "Pós-conjuração", "bHPrecovRate": "Recuperação de HP", "bSPrecovRate": "Recuperação de SP",
	"bUseSPrate": "Consumo de SP", "bSpeedRate": "Velocidade", "bSpeedAddRate": "Velocidade",
	"bMasteryAtk": "ATQ de maestria",
}

func bonusLabel(k string) string {
	if s, ok := bonusLabels[k]; ok {
		return s
	}
	return k
}

var raceNames = map[string]string{
	"RC_All": "todas as raças", "RC_DemiHuman": "Humanoide", "RC_Player_Human": "Humano (jogador)", "RC_Player_Doram": "Doram (jogador)",
	"RC_Brute": "Bruto", "RC_Insect": "Inseto", "RC_Fish": "Peixe", "RC_Demon": "Demônio", "RC_Undead": "Morto-vivo",
	"RC_Formless": "Amorfo", "RC_Plant": "Planta", "RC_Angel": "Anjo", "RC_Dragon": "Dragão",
}
var eleNames = map[string]string{
	"Ele_All": "todos os elementos", "Ele_Neutral": "Neutro", "Ele_Water": "Água", "Ele_Earth": "Terra", "Ele_Fire": "Fogo",
	"Ele_Wind": "Vento", "Ele_Poison": "Veneno", "Ele_Holy": "Sagrado", "Ele_Dark": "Sombrio", "Ele_Ghost": "Fantasma", "Ele_Undead": "Maldito",
}
var effChance = map[string]bool{"bAddEff": true, "bAddEff2": true, "bResEff": true, "bAddEffWhenHit": true, "bMagicAddEff": true}

var effNames = map[string]string{
	"Eff_Poison": "envenenar", "Eff_Stun": "atordoar", "Eff_Freeze": "congelar", "Eff_Curse": "amaldiçoar",
	"Eff_Silence": "silenciar", "Eff_Sleep": "adormecer", "Eff_Blind": "cegar", "Eff_Stone": "petrificar",
	"Eff_Bleeding": "causar sangramento", "Eff_Confusion": "confundir", "Eff_Burning": "queimar",
	"Eff_Crystalize": "cristalizar", "Eff_DPoison": "envenenar gravemente", "Eff_Fear": "amedrontar",
}

var sizeNames = map[string]string{"Size_All": "todos os tamanhos", "Size_Small": "Pequeno", "Size_Medium": "Médio", "Size_Large": "Grande"}

// extraLabel turns "bAddRace RC_DemiHuman" into readable Portuguese when known.
func extraLabel(k string, c *gamedata.Catalog, l string) string {
	T := func(s string) string { return tr(l, s) }
	parts := strings.Fields(k)
	if len(parts) < 2 {
		return k
	}
	arg := parts[1]
	name := func(m map[string]string) string {
		if n, ok := m[arg]; ok {
			return T(n)
		}
		return arg
	}
	switch parts[0] {
	case "bMasteryAtkRace":
		return T("ATQ de maestria vs ") + name(raceNames)
	case "bMasteryAtkEle":
		return T("ATQ de maestria vs ") + name(eleNames)
	case "bAddRace":
		return T("Dano físico vs ") + name(raceNames)
	case "bMagicAddRace":
		return T("Dano mágico vs ") + name(raceNames)
	case "bSubRace":
		return T("Resistência a ") + name(raceNames)
	case "bAddEle":
		return T("Dano físico vs ") + name(eleNames)
	case "bMagicAddEle":
		return T("Dano mágico vs ") + name(eleNames)
	case "bMagicAtkEle":
		return T("Dano mágico de ") + name(eleNames)
	case "bSubEle":
		return T("Resistência a ") + name(eleNames)
	case "bAddSize":
		return T("Dano físico vs ") + name(sizeNames)
	case "bMagicAddSize":
		return T("Dano mágico vs ") + name(sizeNames)
	case "bSubSize":
		return T("Resistência a tamanho ") + name(sizeNames)
	case "bAddEff", "bAddEff2", "bMagicAddEff":
		return T("Chance de ") + name(effNames)
	case "bAddEffWhenHit":
		return T("Chance de ") + name(effNames) + T(" ao ser atingido")
	case "bResEff":
		return T("Resistência a ") + strings.TrimPrefix(arg, "Eff_")
	case "bIgnoreDefRaceRate":
		return T("Ignora DEF de ") + name(raceNames)
	case "bIgnoreMdefRaceRate":
		return T("Ignora DEFM de ") + name(raceNames)
	case "bSkillAtk", "bSkillCooldown", "bSkillVariableCast", "bSkillFixedCast":
		label := map[string]string{"bSkillAtk": "Dano de ", "bSkillCooldown": "Recarga de ", "bSkillVariableCast": "Conjuração variável de ", "bSkillFixedCast": "Conjuração fixa de "}[parts[0]]
		for _, sk := range c.Skills {
			if sk.Code == strings.Trim(arg, `"`) {
				return T(label) + sk.Name.Get(l)
			}
		}
		return T(label) + arg
	}
	return k
}

package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func slotLabel(key string) string {
	for _, s := range append(slices.Clone(equipSlots), shadowSlots...) {
		if s.Key == key {
			return s.Label
		}
	}
	return key
}

// decodeBuild reads a share code (base64url JSON).
func decodeBuild(code string) (calc.Build, bool) {
	var b calc.Build
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil || len(raw) > maxBuildBody {
		return b, false
	}
	return b, json.Unmarshal(raw, &b) == nil
}

func encodeBuild(b calc.Build) string {
	raw, _ := json.Marshal(b)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func defaultBuild(c *gamedata.Catalog) calc.Build {
	b := calc.Build{Job: 4252, BaseLv: 200, JobLv: 50, Stats: [6]int{1, 1, 1, 1, 1, 1}}
	if _, ok := c.Jobs[b.Job]; !ok {
		b.Job = 0
	}
	return b
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
	Bonuses  []bonusLine
	Extras   []bonusLine
	Combos   []string // active item combos ("A + B")
	Buffs    []buffGroup
	HPPct    int
	SPPct    int
	Notes    []string // items removed by normalization (shown in the chat)
	D        damageView
}

type slotView struct {
	Key, Label, Side string
	Item             *itemView
	Refine           int
	Cards            []*itemView // len = item slots
	Refineable       bool
	CardSlots        int // positions below this are card slots; above, enchants
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
	for _, j := range c.Jobs {
		if len(j.Tree) == 0 || j.Name.Get(l) == "" || !j.HasStats() {
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
	for _, kv := range r.SortedBonuses() {
		v.Bonuses = append(v.Bonuses, bonusLine{Label: bonusLabel(kv[0].(string)), Value: signed(kv[1].(int), kv[0].(string))})
	}
	keys := make([]string, 0, len(r.Extra))
	for k, val := range r.Extra {
		if val != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		v.Extras = append(v.Extras, bonusLine{Label: extraLabel(k, c, l), Value: signed(r.Extra[k], k)})
	}
	for _, set := range r.Combos {
		names := make([]string, 0, len(set))
		for _, id := range set {
			if it := c.Items[id]; it != nil {
				names = append(names, it.Name.Get(l))
			}
		}
		v.Combos = append(v.Combos, strings.Join(names, " + "))
	}
	v.Buffs = buffGroups(b)
	v.HPPct, v.SPPct = 100, 100
	v.D = s.damageView(b, r, l)
	return v
}

// GET /build[?b=code]
func (s *Server) handleBuild(w http.ResponseWriter, r *http.Request) {
	l := lang(r)
	if s.opt.Catalog == nil || len(s.opt.Catalog.StatPoints) == 0 {
		s.render(w, r, "build", map[string]any{"Title": "Build", "Lang": l, "NoCatalog": true})
		return
	}
	b := defaultBuild(s.opt.Catalog)
	// A blank build for a chosen class starts at that class's max levels.
	if r.URL.Query().Has("job") {
		if j := s.opt.Catalog.Jobs[atoi(r.URL.Query().Get("job"))]; j != nil && j.HasStats() {
			b = calc.Build{Job: j.ID, BaseLv: j.MaxBaseLv, JobLv: j.MaxJobLv, Stats: [6]int{1, 1, 1, 1, 1, 1}}
		}
	}
	if code := r.URL.Query().Get("b"); code != "" {
		if d, ok := decodeBuild(code); ok {
			b = d
		}
	}
	v := s.buildView(b, l)
	s.render(w, r, "build", map[string]any{"Title": "Build — " + v.JobName, "Lang": l, "V": v,
		"Skills": s.skillTreeData(v.Build.Job, v.Build.JobLv, l, v.Build.Skills)})
}

// POST /build/calc: JSON build in, JSON with re-rendered window fragments out.
func (s *Server) handleBuildCalc(w http.ResponseWriter, r *http.Request) {
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	var b calc.Build
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBuildBody)).Decode(&b); err != nil {
		http.Error(w, "invalid build", http.StatusBadRequest)
		return
	}
	v := s.buildView(b, lang(r))
	out := map[string]any{"build": v.Build, "code": v.Code, "notes": v.Notes, "html": map[string]string{},
		"skillGroups": s.opt.Catalog.SkillBudgets(v.Build.Job, v.Build.JobLv)}
	for _, name := range []string{"build_status", "build_basic", "build_equip", "build_bonus", "build_damage", "build_buffs"} {
		html, err := s.fragmentString(lang(r), "build", name, v)
		if err != nil {
			s.opt.Log.Error("build fragment", "name", name, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		out["html"].(map[string]string)[name] = html
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// GET /build/skills?job=: skill window body for a job (used when the job changes).
func (s *Server) handleBuildSkills(w http.ResponseWriter, r *http.Request) {
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	job, _ := strconv.Atoi(r.URL.Query().Get("job"))
	d := s.skillTreeData(job, atoi(r.URL.Query().Get("joblv")), lang(r), nil)
	if d == nil {
		http.NotFound(w, r)
		return
	}
	s.fragment(w, r, "build", "skill_tree", d)
}

// GET /build/items?slot=&card=1&q=: item picker for a slot (or its cards).
func (s *Server) handleBuildItems(w http.ResponseWriter, r *http.Request) {
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	l := lang(r)
	slot := r.URL.Query().Get("slot")
	card := r.URL.Query().Get("card") == "1"
	job := s.opt.Catalog.Jobs[atoi(r.URL.Query().Get("job"))]
	q := r.URL.Query().Get("q")
	// An enchant position of an item the client's enchant list covers
	// offers only the enchants listed for it.
	var allowed map[int]bool
	if card && r.URL.Query().Has("idx") {
		idx := atoi(r.URL.Query().Get("idx"))
		if it := s.opt.Catalog.Items[atoi(r.URL.Query().Get("item"))]; it != nil && idx >= it.Slots {
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
	s.fragment(w, r, "build", "pick_results", map[string]any{"Results": out, "Q": q, "Slot": slot, "Card": card})
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func (s *Server) fragmentString(l, page, name string, data any) (string, error) {
	pages := s.pages
	if s.opt.Dev {
		p, err := s.parse()
		if err != nil {
			return "", err
		}
		pages = p
	}
	var buf bytes.Buffer
	if err := pages[l+"/"+page].ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
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
	"bAspd": "ASPD", "bAspdRate": "ASPD", "bLongAtkRate": "Dano à distância", "bCritAtkRate": "Dano crítico",
	"bPAtk": "P.ATQ", "bSMatk": "S.ATQM", "bRes": "RES", "bMRes": "MRES", "bHPlus": "H.PLUS", "bCRate": "C.RATE",
	"bVariableCastrate": "Conjuração variável", "bFixedCastrate": "Conjuração fixa", "bFixedCast": "Conjuração fixa (ms)",
	"bDelayrate": "Pós-conjuração", "bHPrecovRate": "Recuperação de HP", "bSPrecovRate": "Recuperação de SP",
	"bUseSPrate": "Consumo de SP", "bSpeedRate": "Velocidade", "bSpeedAddRate": "Velocidade",
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

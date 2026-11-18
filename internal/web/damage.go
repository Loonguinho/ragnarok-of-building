package web

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
)

// damageView is the Alvo / Dano window.
type damageView struct {
	Target      *gamedata.Monster
	TargetInfo  []bonusLine
	Dmg         calc.DamageResult
	Skills      []atkSkill
	Levels      []int
	Elements    [][2]string // value, label
	Atk         calc.AttackChoice
	Ratio       int
	RatioSource string // "fórmula LATAM", "descrição", "manual", "ataque básico", "não encontrado"
	Hits        int
	ElementName string
	States      []stateView
	Variants    []string // labels when the skill has several damage lines
	Variant     int
	StackMax    int // > 0 when the skill uses stacks
	Stack       int
	// Table: every learned damaging skill against the target (build view
	// only; the optimizer leaves it empty).
	Table []skillRow
}

type stateView struct {
	Key, Label string
	Choices    [][2]string
	Value      int
}

type atkSkill struct {
	ID       int
	Name     string
	Selected bool
}

var elementPT = map[string]string{"Neutral": "Neutro", "Water": "Água", "Earth": "Terra", "Fire": "Fogo", "Wind": "Vento",
	"Poison": "Veneno", "Holy": "Sagrado", "Dark": "Sombrio", "Ghost": "Fantasma", "Undead": "Maldito"}
var sizePT = map[string]string{"Small": "Pequeno", "Medium": "Médio", "Large": "Grande"}

func racePT(r string) string {
	for k, v := range raceNames {
		if strings.EqualFold(k, "RC_"+r) {
			return v
		}
	}
	return r
}

func (s *Server) damageView(b calc.Build, r calc.Result, l string) damageView {
	c := s.opt.Catalog
	v := damageView{Atk: b.Atk, Hits: max(1, b.Atk.Hits)}
	v.Elements = append(v.Elements, [2]string{"", "Da arma"})
	for _, e := range calc.Elements {
		v.Elements = append(v.Elements, [2]string{e, elementPT[e]})
	}
	for id, lv := range b.Skills {
		if sk := c.Skills[id]; sk != nil && lv > 0 && !sk.Free() {
			v.Skills = append(v.Skills, atkSkill{ID: id, Name: sk.Name.Get(l), Selected: id == b.Atk.Skill})
		}
	}
	sort.Slice(v.Skills, func(i, j int) bool { return v.Skills[i].Name < v.Skills[j].Name })
	if b.Atk.Skill != 0 {
		for i := 1; i <= b.Skills[b.Atk.Skill]; i++ {
			v.Levels = append(v.Levels, i)
		}
	}

	// Class state selectors (Celestial Space, Exceed...).
	for _, o := range calc.StatesFor(c, b.Job) {
		v.States = append(v.States, stateView{Key: o.Key, Label: skillLabel(c, l, o.Label), Choices: o.Choices, Value: b.Atk.States[o.Key]})
	}

	// Skill ratio: manual value wins, then the LATAM formula, then the
	// description's per-level %, then 100%.
	m := c.Monster(b.Target)
	sk := c.Skills[b.Atk.Skill]
	var formula *calc.SkillFormula
	if sk != nil {
		formula = calc.FormulaFor(c, b.Job, sk.Code)
		if vs := formula.Variants(); len(vs) > 1 {
			for _, f := range vs {
				v.Variants = append(v.Variants, f.Variant)
			}
			v.Variant = min(max(b.Atk.Variant, 0), len(vs)-1)
			formula = formula.Pick(v.Variant)
		}
	}
	in := calc.FormulaInput{
		Lv: b.Atk.Level, BaseLv: b.BaseLv, Stats: r.FinalStats(), Traits: r.FinalTraits(),
		Learned: b.Skills, Catalog: c, Monster: m, Weapon: r.WeaponType, WeaponLv: r.WeaponLv,
		WeaponWt: r.WeaponWeight, MaxHP: r.HP, MaxSP: r.SP, States: b.Atk.States, Stack: b.Atk.Stack,
		JobLv: b.JobLv, HIT: r.HIT, FLEE: r.FLEE, CRIT: r.CRIT, ASPD: r.ASPD,
		WeaponRefine: r.WeaponRefine, WeaponATK: r.WeaponATK,
		ShieldWt: r.ShieldWeight, ShieldRefine: r.ShieldRefine,
	}
	atk := calc.Attack{Hits: b.Atk.Hits, Element: b.Atk.Element}
	if sk != nil {
		atk.SkillCode = sk.Code
	}
	switch {
	case b.Atk.Skill == 0 && b.Atk.Ratio == 0:
		v.Ratio, v.RatioSource = 100, "ataque básico"
	case b.Atk.Ratio > 0:
		v.Ratio, v.RatioSource = b.Atk.Ratio, "manual"
	case formula != nil:
		atk.Formula, atk.Input = formula, in
		v.Ratio, v.RatioSource = int(formula.Ratio(in)), "fórmula LATAM"
		if formula.Source != "" {
			v.RatioSource = "fórmula " + formula.Source
		}
		if formula.Hits != nil {
			v.Hits = max(1, formula.Hits(in))
		}
		if formula.Stack[1] > 0 {
			v.StackMax, v.Stack = formula.Stack[1], max(formula.Stack[0], b.Atk.Stack)
		}
	default:
		if pct, ok := sk.RatioAt(b.Atk.Level); ok {
			v.Ratio, v.RatioSource = pct, "descrição"
		} else {
			v.Ratio, v.RatioSource = 100, "não encontrado"
		}
	}
	if atk.Formula == nil {
		atk.SkillRatio = float64(v.Ratio)
	}

	if m == nil {
		return v
	}
	v.Target = m
	cls := "Normal"
	if m.Class == "Boss" {
		cls = "Chefe"
	}
	if m.MVP {
		cls = "MVP"
	}
	v.TargetInfo = []bonusLine{
		{"Nível", itoa(m.Level)}, {"HP", fmtInt(int64(m.HP))},
		{"Raça", tr(l, racePT(m.Race))}, {"Elemento", tr(l, elementPT[m.Element]) + " " + itoa(m.ElementLv)},
		{"Tamanho", tr(l, sizePT[m.Size])}, {"Classe", tr(l, cls)},
		{"DEF", itoa(m.DEF) + " + " + itoa(m.SoftDEF())}, {"DEFM", itoa(m.MDEF) + " + " + itoa(m.SoftMDEF())},
	}
	if m.RES > 0 || m.MRES > 0 {
		v.TargetInfo = append(v.TargetInfo, bonusLine{"RES / MRES", itoa(m.RES) + " / " + itoa(m.MRES)})
	}
	if m.DamageTaken != 100 {
		v.TargetInfo = append(v.TargetInfo, bonusLine{"Dano recebido", itoa(m.DamageTaken) + "%"})
	}
	v.Dmg = calc.Damage(b, r, c, m, atk)
	v.ElementName = tr(l, elementPT[v.Dmg.Element])
	return v
}

// GET /build/monsters?q=: monster picker results.
func (s *Server) handleBuildMonsters(w http.ResponseWriter, r *http.Request) {
	r0 := r
	if s.opt.Catalog == nil {
		http.Error(w, "catalog not loaded", http.StatusServiceUnavailable)
		return
	}
	s.fragment(w, r, "build", "mob_results", map[string]any{
		"Q": r.URL.Query().Get("q"), "Results": s.opt.Catalog.SearchMonsters(r.URL.Query().Get("q"), 40),
		"RaceName": func(r string) string { return tr(lang(r0), racePT(r)) },
		"EleName":  func(e string) string { return tr(lang(r0), elementPT[e]) },
	})
}

func itoa(n int) string { return fmtInt(int64(n)) }

// fmtInt formats with dots as thousands separators (PT style).
func fmtInt(n int64) string {
	s := strings.TrimPrefix(strconv.FormatInt(n, 10), "-")
	var b strings.Builder
	if n < 0 {
		b.WriteByte('-')
	}
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// skillLabel shows a state named after a skill (its PT client name) with the
// skill's name in the chosen language; other labels go through the
// dictionary in the template.
func skillLabel(c *gamedata.Catalog, l, label string) string {
	if l == "pt" {
		return label
	}
	for _, sk := range c.Skills {
		if sk.Name.Get("pt") == label {
			if n := sk.Name.Get(l); n != "" {
				return n
			}
		}
	}
	return label
}

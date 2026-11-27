package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// ---- Menu (win-menu) ----

type menuState struct {
	undo     [2]widget.Clickable
	toggles  []widget.Clickable
	cmds     [5]widget.Clickable
	gear     widget.Clickable
	settings Popup
	setRows  [1]widget.Clickable
	themes   [2]widget.Clickable // light, dark
	langs    []widget.Clickable
	anchor   Anchor
}

type menuToggle struct {
	label string
	win   **Window
	key   string
}

func (a *App) menuToggles() []menuToggle {
	t := []menuToggle{
		{"Atrib", &a.wStatus, "Alt+A"}, {"Equip", &a.wEquip, "Alt+Q"}, {"Habil", &a.wSkills, "Alt+S"},
		{"Bônus", &a.wBonus, "Alt+B"}, {"Dano", &a.wDamage, "Alt+D"}, {"Buffs", &a.wBuffs, "Alt+U"},
		{"Otimizar", &a.wOptimize, "Alt+O"},
		{"Itens", &a.wItems, "Alt+E"},
	}
	if a.replayDamage() != nil {
		t = append(t, menuToggle{"Dano do replay", &a.wRdmg, ""})
	}
	return t
}

func (a *App) menuBody(gtx C) D {
	th, m := a.Th, &a.menu
	toggles := a.menuToggles()
	if len(m.toggles) < len(toggles) {
		m.toggles = append(m.toggles, make([]widget.Clickable, len(toggles)-len(m.toggles))...)
	}
	for i, t := range toggles {
		if m.toggles[i].Clicked(gtx) && *t.win != nil {
			a.Desk.Toggle(*t.win)
		}
	}
	cmdLabels := []string{"Salvar", "Builds", "Novo", "Compartilhar", "Importar replay"}
	for i := range m.cmds {
		if m.cmds[i].Clicked(gtx) {
			a.menuCommand(i)
		}
	}
	label := func(s string) layout.Widget {
		return func(gtx C) D {
			return Inset(2, 0, 2, 0).Layout(gtx, T(th, upper(a.T(s))).Sz(SmallText).C(ColMuted).W())
		}
	}
	grid := func(n int, cell func(i int) layout.Widget) layout.Widget {
		return func(gtx C) D {
			gap := gtx.Dp(3)
			cw := (gtx.Constraints.Max.X - 3*gap) / 4
			y, rowH := 0, 0
			for i := 0; i < n; i++ {
				c := i % 4
				if c == 0 && i > 0 {
					y += rowH + gap
				}
				g := gtx
				w := cw
				if n == 5 && i == 4 { // "Importar replay" spans the row
					w = gtx.Constraints.Max.X
				}
				g.Constraints = layout.Exact(image.Pt(w, gtx.Dp(BtnH)))
				st := op.Offset(image.Pt(c*(cw+gap), y)).Push(gtx.Ops)
				d := cell(i)(g)
				st.Pop()
				rowH = d.Size.Y
			}
			return D{Size: image.Pt(gtx.Constraints.Max.X, y+rowH)}
		}
	}
	return Col(0,
		label("Janelas"),
		grid(len(toggles), func(i int) layout.Widget {
			return func(gtx C) D {
				return Button{Label: a.T(toggles[i].label), Wide: true}.Layout(gtx, th, &m.toggles[i])
			}
		}),
		Gap(0, 4),
		label("Build"),
		func(gtx C) D { return a.undoButtons(gtx, &m.undo) },
		Gap(0, 3),
		grid(len(cmdLabels), func(i int) layout.Widget {
			return func(gtx C) D {
				return Button{Label: a.T(cmdLabels[i]), Wide: true}.Layout(gtx, th, &m.cmds[i])
			}
		}),
	)(gtx)
}

func (a *App) menuCommand(i int) {
	switch i {
	case 0:
		a.saveCurrent()
	case 1:
		if a.wSaved != nil {
			a.Desk.Toggle(a.wSaved)
		}
	case 2:
		a.newBuild()
	case 3:
		if a.wShare != nil {
			a.Desk.Toggle(a.wShare)
		}
	case 4:
		a.importReplay()
	}
}

// gearButton is the settings gear in the Menu title bar.
func (a *App) gearButton(gtx C) D {
	m := &a.menu
	if m.gear.Clicked(gtx) {
		if a.Desk.PopupOpen(&m.settings) {
			a.Desk.ClosePopup()
		} else {
			a.Desk.OpenPopup(&m.settings)
		}
	}
	d := m.gear.Layout(gtx, func(gtx C) D {
		sz := image.Pt(gtx.Dp(20), gtx.Dp(16))
		if m.gear.Hovered() || a.Desk.PopupOpen(&m.settings) {
			FillRound(gtx, image.Rectangle{Max: sz}, rgbA(0xffffff, 0x40), 3, 3, 3, 3)
		}
		g := gtx
		g.Constraints = layout.Constraints{Max: sz}
		layout.Center.Layout(g, func(gtx C) D { return Gear(gtx, ColTitleText, 14) })
		pointerCursor(gtx, sz)
		return D{Size: sz}
	})
	m.anchor.Track(gtx, a.Desk, d.Size)
	m.settings.At = m.anchor.Origin.Add(image.Pt(0, d.Size.Y+2))
	m.settings.Width = gtx.Dp(220)
	m.settings.Content = a.settingsMenu
	return d
}

func rgbA(c uint32, alpha uint8) color.NRGBA { x := rgb(c); x.A = alpha; return x }

func (a *App) settingsMenu(gtx C) D {
	th, m := a.Th, &a.menu
	rowsActions := []func(){
		func() {
			if a.wHelp != nil {
				a.Desk.Toggle(a.wHelp)
			}
		},
	}
	for i := range m.setRows {
		if m.setRows[i].Clicked(gtx) {
			a.Desk.ClosePopup()
			rowsActions[i]()
		}
	}
	langs := view.Langs()
	if len(m.langs) < len(langs) {
		m.langs = make([]widget.Clickable, len(langs))
	}
	for i, l := range langs {
		if m.langs[i].Clicked(gtx) {
			a.SetLang(l[0])
		}
	}
	for i := range m.themes {
		if m.themes[i].Clicked(gtx) {
			a.SetTheme(i == 1)
		}
	}
	row := func(cl *widget.Clickable, label, kbd string) layout.Widget {
		return func(gtx C) D {
			return cl.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				mm := op.Record(gtx.Ops)
				d := Inset(4, 8, 4, 8).Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, T(th, label).W()),
						layout.Rigid(T(th, kbd).Sz(SmallText).C(rgb(0x6a7894)).W()),
					)
				})
				call := mm.Stop()
				if cl.Hovered() {
					Fill(gtx, image.Rectangle{Max: d.Size}, rgb(0xdce8fb))
				}
				call.Add(gtx.Ops)
				pointerCursor(gtx, d.Size)
				return d
			})
		}
	}
	return Col(0,
		row(&m.setRows[0], a.T("Atalhos e comandos"), "Alt+H"),
		func(gtx C) D {
			return Inset(4, 8, 4, 8).Layout(gtx, func(gtx C) D {
				kids := []layout.Widget{T(th, a.T("Idioma")+":").W()}
				for i, l := range langs {
					i, l := i, l
					kids = append(kids, func(gtx C) D {
						return m.langs[i].Layout(gtx, func(gtx C) D {
							t := B(th, upper(l[0])).C(ColLink)
							if l[0] != a.Lang {
								t = t.C(rgbA(0x1d4fa8, 0x90))
							}
							d := t.Layout(gtx)
							pointerCursor(gtx, d.Size)
							return d
						})
					})
				}
				return Row(6, kids...)(gtx)
			})
		},
		func(gtx C) D {
			return Inset(4, 8, 4, 8).Layout(gtx, func(gtx C) D {
				kids := []layout.Widget{T(th, a.T("Tema")+":").W()}
				for i, name := range []string{"Claro", "Escuro"} {
					i, name := i, name
					kids = append(kids, func(gtx C) D {
						return m.themes[i].Layout(gtx, func(gtx C) D {
							t := B(th, a.T(name)).C(ColLink)
							if (i == 1) != dark {
								t = t.C(rgbA(0x1d4fa8, 0x90))
							}
							d := t.Layout(gtx)
							pointerCursor(gtx, d.Size)
							return d
						})
					})
				}
				return Row(6, kids...)(gtx)
			})
		},
	)(gtx)
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

// ---- Informações Básicas (win-basic) ----

type basicState struct {
	newMode  bool // the class list was opened by Novo: a blank build
	classBtn widget.Clickable
	classDD  Popup
	anchor   Anchor
	jobs     map[int]*widget.Clickable
	list     widget.List
	// Base and class levels, typed in.
	base, joblv NumInput
}

func (a *App) basicBody(gtx C) D {
	th, s := a.Th, &a.basic
	v := a.View()
	b := v.Build
	if s.classBtn.Clicked(gtx) {
		if a.Desk.PopupOpen(&s.classDD) {
			a.Desk.ClosePopup()
		} else {
			a.Desk.OpenPopup(&s.classDD)
		}
	}
	nameRow := func(gtx C) D {
		d := s.classBtn.Layout(gtx, func(gtx C) D {
			d := Row(4,
				func(gtx C) D { return a.Icons.Icon(gtx, view.JobIcon(b.Job), 25) },
				B(th, v.JobName).W(),
				func(gtx C) D { return Caret(gtx, ColTitleText) },
			)(gtx)
			pointerCursor(gtx, d.Size)
			return d
		})
		s.anchor.Track(gtx, a.Desk, d.Size)
		s.classDD.At = s.anchor.Origin.Add(image.Pt(0, d.Size.Y))
		s.classDD.Width = gtx.Dp(260)
		s.classDD.Content = func(gtx C) D { return a.classList(gtx, v) }
		s.classDD.OnClose = func() { s.newMode = false }
		return d
	}
	pct := func(x, m int) int {
		if m <= 0 {
			return 100
		}
		return x * 100 / m
	}
	r := v.R
	kids := []layout.Widget{
		nameRow,
		func(gtx C) D {
			s.base.Compact, s.base.SelectOnFocus = true, true
			s.joblv.Compact, s.joblv.SelectOnFocus = true, true
			return Row(4,
				T(th, a.T("Base Lv.")).C(ColLabel).W(),
				func(gtx C) D {
					d, x, ok := s.base.Layout(gtx, th, b.BaseLv, 40)
					if ok && x >= 1 {
						a.Edit(func(b *calc.Build) { b.BaseLv = min(x, max(r.MaxBaseLv, 1)) })
					}
					return d
				},
				Gap(6, 0),
				T(th, a.T("Classe Lv.")).C(ColLabel).W(),
				func(gtx C) D {
					d, x, ok := s.joblv.Layout(gtx, th, b.JobLv, 34)
					if ok && x >= 1 {
						a.Edit(func(b *calc.Build) { b.JobLv = min(x, max(r.MaxJobLv, 1)) })
					}
					return d
				},
			)(gtx)
		},
		func(gtx C) D { return Bar(gtx, th, 100, BarHP[0], BarHP[1], fmt.Sprintf("HP %d / %d", r.HP, r.HP)) },
		func(gtx C) D { return Bar(gtx, th, 100, BarSP[0], BarSP[1], fmt.Sprintf("SP %d / %d", r.SP, r.SP)) },
	}
	if v.Job != nil && v.Job.Fourth {
		kids = append(kids, func(gtx C) D { return Bar(gtx, th, 100, BarAP[0], BarAP[1], fmt.Sprintf("AP %d", r.AP)) })
	}
	kids = append(kids,
		func(gtx C) D {
			return Bar(gtx, th, pct(b.BaseLv, r.MaxBaseLv), BarEXP[0], BarEXP[1], fmt.Sprintf("%s %d/%d", a.T("Base"), b.BaseLv, r.MaxBaseLv))
		},
		func(gtx C) D {
			return Bar(gtx, th, pct(b.JobLv, r.MaxJobLv), BarEXP[0], BarEXP[1], fmt.Sprintf("%s %d/%d", a.T("Classe"), b.JobLv, r.MaxJobLv))
		},
	)
	return Col(3, kids...)(gtx)
}

// classList is the class dropdown (Alt+J): classes grouped by family.
func (a *App) classList(gtx C, v view.BuildView) D {
	th, s := a.Th, &a.basic
	if s.jobs == nil {
		s.jobs = map[int]*widget.Clickable{}
	}
	type row struct {
		fam string
		job view.JobOption
	}
	var rows []row
	for _, f := range v.Families {
		rows = append(rows, row{fam: f.Name})
		for _, j := range f.Jobs {
			rows = append(rows, row{job: j})
		}
	}
	for id, cl := range s.jobs {
		if cl.Clicked(gtx) {
			// Read the mode first: closing the list (OnClose) clears it.
			fresh := s.newMode
			a.Desk.ClosePopup()
			jid := id
			if fresh {
				s.newMode = false
				a.OpenBuild(a.V.NewBuild(jid))
				continue
			}
			a.Edit(func(b *calc.Build) {
				b.Job = jid
				b.Skills = nil
				b.Traits = [6]int{}
			})
		}
	}
	s.list.Axis = layout.Vertical
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(420))
	return s.list.Layout(gtx, len(rows), func(gtx C, i int) D {
		r := rows[i]
		if r.fam != "" {
			return Inset(4, 6, 2, 6).Layout(gtx, B(th, r.fam).C(ColTitleText).W())
		}
		cl := s.jobs[r.job.ID]
		if cl == nil {
			cl = &widget.Clickable{}
			s.jobs[r.job.ID] = cl
		}
		return cl.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			mm := op.Record(gtx.Ops)
			d := Inset(1, 6, 1, 14).Layout(gtx, Row(6,
				func(gtx C) D { return a.Icons.Icon(gtx, view.JobIcon(r.job.ID), 25) },
				T(th, r.job.Name).W(),
			))
			call := mm.Stop()
			rr := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, d.Size.Y)}
			if r.job.ID == v.Build.Job {
				Fill(gtx, rr, ColSelect)
			} else if cl.Hovered() {
				Fill(gtx, rr, ColHover)
			}
			call.Add(gtx.Ops)
			pointerCursor(gtx, rr.Max)
			return D{Size: rr.Max}
		})
	})
}

// ---- Atributos (win-status) ----

type statusState struct {
	plus, minus  [6]widget.Clickable
	tplus, tminu [6]widget.Clickable
	// The stat and trait values, typed in.
	stat, trait [6]NumInput
	// Tooltips: what each attribute gives; casting; tolerances.
	statTip, traitTip [6]Hoverable
	castTip, tolTip   Hoverable
}

var statNames = [6]string{"FOR", "AGI", "VIT", "INT", "DES", "SOR"}
var traitNames = [6]string{"POW", "STA", "WIS", "SPL", "CON", "CRT"}

func (a *App) statusBody(gtx C) D {
	th, s := a.Th, &a.status
	v := a.View()
	r := v.R
	// Shift+click: ±10.
	step := func(cl *widget.Clickable) (int, bool) {
		c, ok := cl.Update(gtx)
		if !ok {
			return 0, false
		}
		if c.Modifiers.Contain(key.ModShift) {
			return 10, true
		}
		return 1, true
	}
	maxStat := max(r.MaxStat, 1)
	for i := range 6 {
		if n, ok := step(&s.plus[i]); ok {
			a.Edit(func(b *calc.Build) { b.Stats[i] = min(b.Stats[i]+n, maxStat) })
		}
		if n, ok := step(&s.minus[i]); ok {
			a.Edit(func(b *calc.Build) { b.Stats[i] = max(1, b.Stats[i]-n) })
		}
		if n, ok := step(&s.tplus[i]); ok {
			a.Edit(func(b *calc.Build) { b.Traits[i] = min(b.Traits[i]+n, max(r.MaxTrait, 1)) })
		}
		if n, ok := step(&s.tminu[i]); ok {
			a.Edit(func(b *calc.Build) { b.Traits[i] = max(0, b.Traits[i]-n) })
		}
	}
	// attrTip is the tooltip of an attribute: its value and what it gives.
	attrTip := func(name string, st calc.StatLine, lines []view.AttrLine) layout.Widget {
		return func(gtx C) D {
			head := fmt.Sprintf("%s %d", a.T(name), st.Base+st.Bonus)
			if st.Bonus != 0 {
				head = fmt.Sprintf("%s %d (%d %+d)", a.T(name), st.Base+st.Bonus, st.Base, st.Bonus)
			}
			rows := append([]layout.Widget{B(th, head).W()}, tipRows(th, lines)...)
			return TipBox(gtx, Col(2, rows...))
		}
	}
	statTable := func(lines [6]calc.StatLine, names [6]string, plus, minus *[6]widget.Clickable, boxes *[6]NumInput, minBase, maxBase int, set func(b *calc.Build, i, v int), showCost bool, hov *[6]Hoverable, trait bool) layout.Widget {
		return func(gtx C) D {
			var rows []layout.Widget
			for i := range 6 {
				st := lines[i]
				rows = append(rows, Row(0,
					Fixed(34, func(gtx C) D {
						d, over := hov[i].Layout(gtx, T(th, a.T(names[i])).C(ColLabel).W())
						if over {
							a.Desk.ShowTip(attrTip(names[i], st, view.AttrEffects(r, i, trait, a.Lang)))
						}
						return d
					}),
					func(gtx C) D {
						in := &boxes[i]
						in.Compact, in.SelectOnFocus = true, true
						d, x, ok := in.Layout(gtx, th, st.Base, 30)
						if ok {
							x = min(max(x, minBase), max(maxBase, minBase))
							a.Edit(func(b *calc.Build) { set(b, i, x) })
						}
						return d
					},
					Fixed(40, func(gtx C) D {
						if st.Bonus == 0 {
							return D{}
						}
						return Inset(0, 0, 0, 4).Layout(gtx, T(th, fmt.Sprintf("+%d", st.Bonus)).C(ColLink).W())
					}),
					func(gtx C) D { return MinusBtn(gtx, th, &minus[i], st.Base <= minBase) },
					Gap(1, 0),
					func(gtx C) D { return PlusBtn(gtx, th, &plus[i], st.Cost == 0) },
					Gap(4, 0),
					// Fixed, so both tables (with and without the cost)
					// end at the same x and the stats beside them line up.
					Fixed(20, func(gtx C) D {
						if !showCost || st.Cost == 0 {
							return D{}
						}
						return T(th, fmt.Sprint(st.Cost)).C(rgb(0x888888)).Layout(gtx)
					}),
				))
			}
			return Col(2, rows...)(gtx)
		}
	}
	derived := func(rows [][2]string) layout.Widget {
		return func(gtx C) D {
			var ws []layout.Widget
			for _, rw := range rows {
				ws = append(ws, Row(0, Fixed(78, T(th, a.T(rw[0])).C(ColLabel).W()), T(th, rw[1]).W()))
			}
			return Col(2, ws...)(gtx)
		}
	}
	// tipped is a derived row whose tooltip lists lines under a title.
	tipped := func(h *Hoverable, label, value, title string, lines []view.AttrLine) layout.Widget {
		return func(gtx C) D {
			d, over := h.Layout(gtx, Row(0, Fixed(78, T(th, a.T(label)).C(ColLabel).W()), T(th, value).C(ColLink).W()))
			if over {
				a.Desk.ShowTip(func(gtx C) D {
					return TipBox(gtx, Col(2, append([]layout.Widget{B(th, a.T(title)).W()}, tipRows(th, lines)...)...))
				})
			}
			return d
		}
	}
	ci := r.Cast()
	castVal := a.T("instantânea")
	if ci.VarLeft > 0.05 {
		castVal = fmt.Sprintf("−%.0f%%", 100-ci.VarLeft)
	}
	points := func(label string, n int) layout.Widget {
		return func(gtx C) D {
			c := ColText
			if n < 0 {
				c = ColDown
			}
			return Row(4, T(th, a.T(label)).W(), B(th, fmt.Sprint(n)).C(c).W(), T(th, a.T("Shift+clique: ±10 · ou digite")).Sz(SmallText).C(ColMuted).W())(gtx)
		}
	}
	kids := []layout.Widget{
		RowTop(28,
			statTable(r.Stats, statNames, &s.plus, &s.minus, &s.stat, 1, maxStat, func(b *calc.Build, i, v int) { b.Stats[i] = v }, true, &s.statTip, false),
			Col(2, derived([][2]string{
				{"ATQ", fmt.Sprintf("%d + %d", r.StatusATK, r.EquipATK)},
				{"ATQM", fmt.Sprintf("%d + %d", r.MATKMin, r.EquipMATK)},
				{"Precisão", fmt.Sprint(r.HIT)},
				{"Crítico", fmt.Sprintf("%.1f", r.CRIT)},
				{"DEF", fmt.Sprintf("%d + %d", r.HardDEF, r.SoftDEF)},
				{"DEFM", fmt.Sprintf("%d + %d", r.HardMDEF, r.SoftMDEF)},
				{"Esquiva", fmt.Sprintf("%d + %d", r.FLEE, r.FLEE2)},
				{"ASPD", fmt.Sprint(r.ASPD)},
			}),
				tipped(&s.castTip, "Conj. var.", castVal, "Conjuração variável", append(view.CastLines(r, a.Lang), a.V.CastSources(r, a.Lang)...)),
				tipped(&s.tolTip, "Tolerâncias", a.T("ver"), "Tolerâncias a status", view.ToleranceLines(r, a.Lang)),
			),
		),
		Sep,
		points("Pontos de atributo:", r.Points),
	}
	if v.Job != nil && v.Job.Fourth {
		kids = append(kids, Sep,
			RowTop(28,
				statTable(r.Traits, traitNames, &s.tplus, &s.tminu, &s.trait, 0, r.MaxTrait, func(b *calc.Build, i, v int) { b.Traits[i] = v }, false, &s.traitTip, true),
				derived([][2]string{
					{"P.ATQ", fmt.Sprint(r.PATK)}, {"S.ATQM", fmt.Sprint(r.SMATK)}, {"RES", fmt.Sprint(r.RES)},
					{"MRES", fmt.Sprint(r.MRES)}, {"H.PLUS", fmt.Sprint(r.HPLUS)}, {"C.RATE", fmt.Sprint(r.CRATE)},
				}),
			),
			Sep,
			points("Pontos de traço:", r.TraitPoints),
			func(gtx C) D {
				if a.traits.open.Clicked(gtx) {
					a.showTraits()
				}
				return Button{Label: a.T("Comparar talentos")}.Layout(gtx, th, &a.traits.open)
			},
		)
	}
	pools := fmt.Sprintf("HP %d %s %d", r.HP, a.T("· SP"), r.SP)
	if v.Job != nil && v.Job.Fourth {
		pools += fmt.Sprintf(" %s %d", a.T("· AP"), r.AP)
	}
	kids = append(kids, Gap(0, 3), func(gtx C) D {
		return Box{Bg: ColPanel, Line: ColSep, Pad: Inset(3, 4, 3, 4)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return T(th, pools).Layout(gtx)
		})
	})
	for _, w := range r.Warnings {
		kids = append(kids, T(th, "! "+a.T(w)).C(ColDown).Wrapping().W())
	}
	return Col(3, kids...)(gtx)
}

// tipRows lays attribute effect lines as "label  value" rows of a tooltip.
func tipRows(th *Theme, lines []view.AttrLine) []layout.Widget {
	// Wider when a line is long (the sets in Conjuração variável's sources).
	w := unit.Dp(150)
	for _, l := range lines {
		if len([]rune(l.Label)) > 26 {
			w = 210
		}
	}
	var ws []layout.Widget
	for _, l := range lines {
		ws = append(ws, RowTop(0, Fixed(w, T(th, l.Label).C(ColLabel).Wrapping().W()), T(th, l.Value).W()))
	}
	return ws
}

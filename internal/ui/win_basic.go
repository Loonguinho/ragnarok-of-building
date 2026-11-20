package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// ---- Menu (win-menu) ----

type menuState struct {
	toggles  []widget.Clickable
	cmds     [5]widget.Clickable
	gear     widget.Clickable
	settings Popup
	setRows  [3]widget.Clickable
	langs    []widget.Clickable
	float    Checkbox
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
		func() { a.Desk.ResetLayout() },
		func() { a.Desk.ShowAll() },
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
		func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			mm := op.Record(gtx.Ops)
			d := Inset(2, 8, 2, 8).Layout(gtx, B(th, a.T("Configurações")).C(ColTitleText).W())
			call := mm.Stop()
			r := image.Rectangle{Max: image.Pt(gtx.Constraints.Max.X, d.Size.Y)}
			Fill(gtx, r, rgb(0xe8edf6))
			call.Add(gtx.Ops)
			Fill(gtx, image.Rect(0, r.Max.Y-1, r.Max.X, r.Max.Y), ColSep)
			return D{Size: r.Max}
		},
		func(gtx C) D {
			return Inset(4, 8, 4, 8).Layout(gtx, func(gtx C) D {
				d, toggled := m.float.Layout(gtx, th, a.Desk.Floating, a.T("Janelas flutuantes")+"  (Alt+W)")
				if toggled {
					a.Desk.Floating = !a.Desk.Floating
					a.Desk.changed = true
				}
				return d
			})
		},
		row(&m.setRows[0], a.T("Reorganizar as janelas"), ""),
		row(&m.setRows[1], a.T("Abrir todas as janelas"), ""),
		row(&m.setRows[2], a.T("Atalhos e comandos"), "Alt+H"),
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
		T(th, fmt.Sprintf("Base Lv. %d %s %d", b.BaseLv, a.T("· Classe Lv."), b.JobLv)).W(),
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
			a.Desk.ClosePopup()
			jid := id
			if s.newMode {
				s.newMode = false
				a.SetBuild(a.V.NewBuild(jid))
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
	job          Select
	base, joblv  NumInput
	plus, minus  [6]widget.Clickable
	tplus, tminu [6]widget.Clickable
}

var statNames = [6]string{"FOR", "AGI", "VIT", "INT", "DES", "SOR"}
var traitNames = [6]string{"POW", "STA", "WIS", "SPL", "CON", "CRT"}

func (a *App) statusBody(gtx C) D {
	th, s := a.Th, &a.status
	v := a.View()
	b, r := v.Build, v.R
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
	var jobs []Option
	for _, j := range v.Jobs {
		jobs = append(jobs, Option{Value: fmt.Sprint(j.ID), Label: j.Name})
	}
	form := Row(6,
		T(th, a.T("Classe")).W(),
		func(gtx C) D {
			d, val, ok := s.job.Layout(gtx, th, a.Desk, jobs, fmt.Sprint(b.Job), 150)
			if ok {
				var id int
				fmt.Sscan(val, &id)
				a.Edit(func(b *calc.Build) { b.Job = id; b.Skills = nil; b.Traits = [6]int{} })
			}
			return d
		},
		T(th, a.T("Base")).W(),
		func(gtx C) D {
			d, x, ok := s.base.Layout(gtx, th, b.BaseLv, 54)
			if ok && x >= 1 {
				a.Edit(func(b *calc.Build) { b.BaseLv = min(x, max(r.MaxBaseLv, 1)) })
			}
			return d
		},
		T(th, a.T("Classe")).W(),
		func(gtx C) D {
			d, x, ok := s.joblv.Layout(gtx, th, b.JobLv, 54)
			if ok && x >= 1 {
				a.Edit(func(b *calc.Build) { b.JobLv = min(x, max(r.MaxJobLv, 1)) })
			}
			return d
		},
	)
	statTable := func(lines [6]calc.StatLine, names [6]string, plus, minus *[6]widget.Clickable, minBase int, showCost bool) layout.Widget {
		return func(gtx C) D {
			var rows []layout.Widget
			for i := range 6 {
				st := lines[i]
				rows = append(rows, Row(0,
					Fixed(34, T(th, a.T(names[i])).C(ColLabel).W()),
					Fixed(28, B(th, fmt.Sprint(st.Base)).Right().W()),
					Fixed(30, func(gtx C) D {
						if st.Bonus == 0 {
							return D{}
						}
						return Inset(0, 0, 0, 4).Layout(gtx, T(th, fmt.Sprintf("+%d", st.Bonus)).C(ColLink).W())
					}),
					func(gtx C) D { return MinusBtn(gtx, th, &minus[i], st.Base <= minBase) },
					Gap(1, 0),
					func(gtx C) D { return PlusBtn(gtx, th, &plus[i], st.Cost == 0) },
					Gap(4, 0),
					func(gtx C) D {
						if !showCost || st.Cost == 0 {
							return D{}
						}
						return T(th, fmt.Sprint(st.Cost)).C(rgb(0x888888)).Layout(gtx)
					},
				))
			}
			return Col(2, rows...)(gtx)
		}
	}
	derived := func(rows [][2]string) layout.Widget {
		return func(gtx C) D {
			var ws []layout.Widget
			for _, rw := range rows {
				ws = append(ws, Row(0, Fixed(64, T(th, a.T(rw[0])).C(ColLabel).W()), T(th, rw[1]).W()))
			}
			return Col(2, ws...)(gtx)
		}
	}
	points := func(label string, n int) layout.Widget {
		return func(gtx C) D {
			c := ColText
			if n < 0 {
				c = ColDown
			}
			return Row(4, T(th, a.T(label)).W(), B(th, fmt.Sprint(n)).C(c).W(), T(th, a.T("Shift+clique: ±10")).Sz(SmallText).C(ColMuted).W())(gtx)
		}
	}
	kids := []layout.Widget{
		form,
		Sep,
		RowTop(12,
			statTable(r.Stats, statNames, &s.plus, &s.minus, 1, true),
			derived([][2]string{
				{"ATQ", fmt.Sprintf("%d + %d", r.StatusATK, r.EquipATK)},
				{"ATQM", fmt.Sprintf("%d + %d", r.MATKMin, r.EquipMATK)},
				{"Precisão", fmt.Sprint(r.HIT)},
				{"Crítico", fmt.Sprintf("%.1f", r.CRIT)},
				{"DEF", fmt.Sprintf("%d + %d", r.HardDEF, r.SoftDEF)},
				{"DEFM", fmt.Sprintf("%d + %d", r.HardMDEF, r.SoftMDEF)},
				{"Esquiva", fmt.Sprintf("%d + %d", r.FLEE, r.FLEE2)},
				{"ASPD", fmt.Sprint(r.ASPD)},
			}),
		),
		Sep,
		points("Pontos de atributo:", r.Points),
	}
	if v.Job != nil && v.Job.Fourth {
		kids = append(kids, Sep,
			RowTop(12,
				statTable(r.Traits, traitNames, &s.tplus, &s.tminu, 0, false),
				derived([][2]string{
					{"P.ATQ", fmt.Sprint(r.PATK)}, {"S.ATQM", fmt.Sprint(r.SMATK)}, {"RES", fmt.Sprint(r.RES)},
					{"MRES", fmt.Sprint(r.MRES)}, {"H.PLUS", fmt.Sprint(r.HPLUS)}, {"C.RATE", fmt.Sprint(r.CRATE)},
				}),
			),
			Sep,
			points("Pontos de traço:", r.TraitPoints),
		)
	}
	pools := fmt.Sprintf("HP %d %s %d", r.HP, a.T("· SP"), r.SP)
	if v.Job != nil && v.Job.Fourth {
		pools += fmt.Sprintf(" %s %d", a.T("· AP"), r.AP)
	}
	kids = append(kids, Gap(0, 3), func(gtx C) D {
		return Box{Bg: ColWhite, Line: ColSep, Pad: Inset(3, 4, 3, 4)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return T(th, pools).Layout(gtx)
		})
	})
	for _, w := range r.Warnings {
		kids = append(kids, T(th, "! "+a.T(w)).C(ColDown).Wrapping().W())
	}
	return Col(3, kids...)(gtx)
}

package ui

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// Details is a collapsible section (<details>): a clickable header with a
// triangle, and the content when open.
type Details struct {
	Open bool
	init bool
	cl   widget.Clickable
}

// Layout draws the header and, when open, the content. open is the
// initial state.
func (d *Details) Layout(gtx C, th *Theme, title string, open bool, content layout.Widget) D {
	if !d.init {
		d.Open, d.init = open, true
	}
	if d.cl.Clicked(gtx) {
		d.Open = !d.Open
	}
	return Col(3,
		func(gtx C) D {
			dd := d.cl.Layout(gtx, func(gtx C) D {
				dd := Row(4,
					func(gtx C) D { return Triangle(gtx, ColTitleText, d.Open) },
					B(th, title).C(ColTitleText).W(),
				)(gtx)
				pointerCursor(gtx, dd.Size)
				return dd
			})
			return dd
		},
		func(gtx C) D {
			if !d.Open {
				return D{}
			}
			return Inset(0, 0, 0, 12).Layout(gtx, content)
		},
	)(gtx)
}

// Triangle is ▸ (closed) or ▾ (open).
func Triangle(gtx C, c color.NRGBA, open bool) D {
	if open {
		return Caret(gtx, c)
	}
	s := float32(gtx.Dp(9))
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(s*0.3, s*0.15))
	p.LineTo(f32.Pt(s*0.75, s*0.5))
	p.LineTo(f32.Pt(s*0.3, s*0.85))
	p.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: p.End()}.Op())
	return D{Size: image.Pt(int(s), int(s))}
}

// ---- Bônus do Equipamento (win-bonus) ----

func (a *App) bonusBody(gtx C) D {
	th := a.Th
	v := a.View()
	line := func(l view.BonusLine) layout.Widget {
		return func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, T(th, a.T(l.Label)).Wrapping().W()),
				layout.Rigid(Gap(6, 0)),
				layout.Rigid(B(th, l.Value).W()),
			)
		}
	}
	column := func(title string, lines []view.BonusLine) layout.Widget {
		return func(gtx C) D {
			ws := []layout.Widget{B(th, a.T(title)).C(ColTitleText).W()}
			for _, l := range lines {
				ws = append(ws, line(l))
			}
			if len(lines) == 0 {
				ws = append(ws, T(th, "—").C(ColMuted).W())
			}
			return Col(2, ws...)(gtx)
		}
	}
	var kids []layout.Widget
	if len(v.Bonuses) > 0 || len(v.Extras) > 0 {
		kids = append(kids, func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, column("Atributos", v.Bonuses)),
				layout.Rigid(Gap(12, 0)),
				layout.Flexed(1, column("Dano e resistências", v.Extras)),
			)
		})
	} else {
		kids = append(kids, T(th, a.T("Nenhum equipamento com bônus.")).C(ColMuted).W())
	}
	if len(v.Combos) > 0 {
		kids = append(kids, Sep, B(th, a.T("Combos ativos")).C(ColTitleText).W())
		for _, c := range v.Combos {
			c := c
			ws := []layout.Widget{B(th, c.Name).C(rgb(0x1d3f7a)).Wrapping().W()}
			for _, l := range c.Lines {
				ws = append(ws, func(gtx C) D { return Inset(0, 0, 0, 10).Layout(gtx, line(l)) })
			}
			switch {
			case c.Skipped > 0:
				ws = append(ws, Pad(Inset(0, 0, 0, 10), T(th, fmt.Sprintf("%d %s", c.Skipped, a.T("efeito(s) ainda não simulado(s) (procs, habilidades concedidas)"))).Sz(SmallText).C(rgb(0x666677)).Wrapping().W()))
			case len(c.Lines) == 0:
				ws = append(ws, Pad(Inset(0, 0, 0, 10), T(th, a.T("sem bônus de atributos")).Sz(SmallText).C(rgb(0x666677)).W()))
			}
			kids = append(kids, Col(1, ws...))
		}
	}
	if n := v.R.SkippedScriptEffects; n > 0 {
		kids = append(kids, Sep, T(th, fmt.Sprintf("%d %s", n, a.T("efeito(s) de equipamento ainda não simulado(s) (procs, habilidades concedidas, efeitos condicionais)."))).Sz(SmallText).C(ColMuted).Wrapping().W())
	}
	return Col(4, kids...)(gtx)
}

// ---- Alvo / Dano (win-damage) ----

type damageState struct {
	mobQ     Input
	lastQ    string
	mobs     []view.MonsterRow
	mobRows  []widget.Clickable
	mobList  widget.List
	skill    Select
	level    Select
	ratio    NumInput
	hits     NumInput
	ele      Select
	variant  Select
	stack    NumInput
	auto     widget.Clickable
	table    Details
	steps    Details
	rowPicks []widget.Clickable
}

func (a *App) setAtk(fn func(at *calc.AttackChoice)) {
	a.Edit(func(b *calc.Build) { fn(&b.Atk) })
}

func (a *App) damageBody(gtx C) D {
	th, s := a.Th, &a.damage
	v := a.View()
	d := v.D
	b := v.Build

	// Monster search.
	for i := range s.mobs {
		if i < len(s.mobRows) && s.mobRows[i].Clicked(gtx) {
			id := s.mobs[i].M.ID
			s.mobQ.Ed.SetText("")
			s.lastQ, s.mobs = "", nil
			a.Edit(func(b *calc.Build) { b.Target = id })
		}
	}
	if s.auto.Clicked(gtx) {
		a.setAtk(func(at *calc.AttackChoice) { at.Ratio = 0 })
	}

	search := func(gtx C) D {
		s.mobQ.Hint = a.T("Buscar monstro (nome ou ID)…")
		dd, changed, _ := s.mobQ.Layout(gtx, th, 0)
		if changed && s.mobQ.Text() != s.lastQ {
			s.lastQ = s.mobQ.Text()
			s.mobs = nil
			if s.lastQ != "" {
				s.mobs = a.V.SearchMonsters(s.lastQ, a.Lang)
			}
			s.mobRows = make([]widget.Clickable, len(s.mobs))
		}
		return dd
	}
	kids := []layout.Widget{search}
	if s.lastQ != "" {
		kids = append(kids, func(gtx C) D { return a.mobList(gtx) })
	}

	// Attack row.
	skills := []Option{{"0", a.T("Ataque básico")}}
	for _, sk := range d.Skills {
		skills = append(skills, Option{strconv.Itoa(sk.ID), sk.Name})
	}
	atkRow := []layout.Widget{
		T(th, a.T("Ataque")).W(),
		func(gtx C) D {
			dd, val, ok := s.skill.Layout(gtx, th, a.Desk, skills, strconv.Itoa(d.Atk.Skill), 170)
			if ok {
				id, _ := strconv.Atoi(val)
				lv := b.Skills[id]
				a.setAtk(func(at *calc.AttackChoice) { at.Skill, at.Level, at.Ratio, at.Variant = id, lv, 0, 0 })
			}
			return dd
		},
	}
	if len(d.Levels) > 0 {
		var lvs []Option
		for _, l := range d.Levels {
			lvs = append(lvs, Option{strconv.Itoa(l), strconv.Itoa(l)})
		}
		atkRow = append(atkRow, T(th, a.T("Nv")).W(), func(gtx C) D {
			dd, val, ok := s.level.Layout(gtx, th, a.Desk, lvs, strconv.Itoa(d.Atk.Level), 44)
			if ok {
				l, _ := strconv.Atoi(val)
				a.setAtk(func(at *calc.AttackChoice) { at.Level, at.Ratio = l, 0 })
			}
			return dd
		})
	}
	atkRow = append(atkRow,
		T(th, "%").W(),
		func(gtx C) D {
			dd, x, ok := s.ratio.Layout(gtx, th, d.Ratio, 54)
			if ok && x > 0 {
				a.setAtk(func(at *calc.AttackChoice) { at.Ratio = x })
			}
			return dd
		},
	)
	var eles []Option
	for _, e := range d.Elements {
		eles = append(eles, Option{e[0], a.T(e[1])})
	}
	atkRow2 := []layout.Widget{
		T(th, a.T("Golpes")).W(),
		func(gtx C) D {
			dd, x, ok := s.hits.Layout(gtx, th, d.Hits, 40)
			if ok && x >= 1 && x <= 50 {
				a.setAtk(func(at *calc.AttackChoice) { at.Hits = x })
			}
			return dd
		},
		T(th, a.T("Elemento")).W(),
		func(gtx C) D {
			dd, val, ok := s.ele.Layout(gtx, th, a.Desk, eles, d.Atk.Element, 110)
			if ok {
				a.setAtk(func(at *calc.AttackChoice) { at.Element = val })
			}
			return dd
		},
	}
	if len(d.Variants) > 0 {
		var vs []Option
		for i, n := range d.Variants {
			vs = append(vs, Option{strconv.Itoa(i), a.T(n)})
		}
		atkRow2 = append(atkRow2, T(th, a.T("Linha de dano")).W(), func(gtx C) D {
			dd, val, ok := s.variant.Layout(gtx, th, a.Desk, vs, strconv.Itoa(d.Variant), 130)
			if ok {
				x, _ := strconv.Atoi(val)
				a.setAtk(func(at *calc.AttackChoice) { at.Variant = x })
			}
			return dd
		})
	}
	if d.StackMax > 0 {
		atkRow2 = append(atkRow2, T(th, a.T("Acúmulos")).W(), func(gtx C) D {
			dd, x, ok := s.stack.Layout(gtx, th, d.Stack, 40)
			if ok && x >= 1 && x <= d.StackMax {
				a.setAtk(func(at *calc.AttackChoice) { at.Stack = x })
			}
			return dd
		})
	}
	kids = append(kids, Row(4, atkRow...), Row(4, atkRow2...), a.ratioSource(d))

	if d.Target == nil {
		kids = append(kids, func(gtx C) D {
			return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(10, 10, 10, 10)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return T(th, a.T("Busque um monstro acima para calcular o dano.")).Sz(12).Layout(gtx)
			})
		})
		return Col(5, kids...)(gtx)
	}
	kids = append(kids, a.targetCard(d), a.damageTable(d))
	if len(d.Table) > 1 {
		kids = append(kids, func(gtx C) D {
			return s.table.Layout(gtx, th, a.T("Dano de cada habilidade"), true, func(gtx C) D { return a.skillTableW(gtx, d) })
		})
	}
	kids = append(kids, func(gtx C) D {
		return s.steps.Layout(gtx, th, a.T("Como foi calculado (golpe médio)"), false, func(gtx C) D {
			var ws []layout.Widget
			for i, st := range d.Dmg.Steps {
				i, st := i, st
				ws = append(ws, func(gtx C) D {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(Fixed(18, T(th, fmt.Sprintf("%d.", i+1)).C(ColMuted).W())),
						layout.Flexed(1, T(th, a.T(st.Label)).Wrapping().W()),
						layout.Rigid(Gap(6, 0)),
						layout.Rigid(T(th, a.T(st.Value)).F(th.Mono).W()),
					)
				})
			}
			return Col(2, ws...)(gtx)
		})
	})
	for _, n := range d.Dmg.Notes {
		kids = append(kids, T(th, a.T(n)).Sz(SmallText).C(ColMuted).Wrapping().W())
	}
	note := a.T("Base: fórmula física do rAthena (renewal) com os ajustes do LATAM medidos pelo latam-ro-calc (MIT).")
	if d.RatioSource != "fórmula LATAM" {
		note += " " + a.T("Esta habilidade ainda usa o % da descrição.")
	}
	kids = append(kids, T(th, note).Sz(SmallText).C(ColMuted).Wrapping().W())
	return Col(5, kids...)(gtx)
}

func (a *App) ratioSource(d view.DamageView) layout.Widget {
	th, s := a.Th, &a.damage
	return func(gtx C) D {
		var ws []layout.Widget
		ws = append(ws, T(th, "%").W())
		switch d.RatioSource {
		case "fórmula LATAM":
			ws = append(ws, B(th, a.T("fórmula LATAM")).C(ColUp).W(), T(th, a.T("(nível base, POW, maestrias e estados)")).W())
		case "fórmula bROWiki":
			ws = append(ws, B(th, a.T("fórmula LATAM")).C(ColUp).W(), T(th, a.T("(corrigida pelo bROWiki)")).W())
		case "descrição":
			ws = append(ws, T(th, a.T("lido da descrição da habilidade")).W())
		case "não encontrado":
			ws = append(ws, B(th, a.T("não está na descrição: digite o % da habilidade")).C(ColDown).W())
		case "manual":
			ws = append(ws, T(th, a.T("informado manualmente")).W(), func(gtx C) D {
				return Button{Label: a.T("Usar automático")}.Layout(gtx, th, &s.auto)
			})
		default:
			ws = append(ws, T(th, a.T("de ataque básico")).W())
		}
		if d.Hits > 1 {
			ws = append(ws, T(th, fmt.Sprintf("· %d %s", d.Hits, a.T("golpes"))).W())
		}
		return Row(4, ws...)(gtx)
	}
}

func (a *App) mobList(gtx C) D {
	th, s := a.Th, &a.damage
	gtx.Constraints.Max.Y = gtx.Dp(200)
	return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(2, 2, 2, 2)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if len(s.mobs) == 0 {
			return Inset(6, 6, 6, 6).Layout(gtx, T(th, a.T("Nenhum monstro encontrado.")).C(rgb(0x777777)).W())
		}
		s.mobList.Axis = layout.Vertical
		return s.mobList.Layout(gtx, len(s.mobs), func(gtx C, i int) D {
			r := s.mobs[i]
			cl := &s.mobRows[i]
			return cl.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				m := op.Record(gtx.Ops)
				dd := Inset(2, 6, 2, 6).Layout(gtx, func(gtx C) D {
					name := r.Name
					if r.M.MVP {
						name += " (MVP)"
					}
					if o := r.M.OtherName(a.Lang); o != "" {
						name += " (" + o + ")"
					}
					info := fmt.Sprintf("%s %d · %s · %s %d", a.T("Nv"), r.M.Level, r.Race, r.Element, r.M.ElementLv)
					if r.M.Source != "" {
						info += " · " + r.M.Source
					}
					if r.M.Stub {
						info = a.T("sem atributos no rAthena: buscados no Divine Pride ao escolher")
					}
					return Col(0, T(th, name).W(), T(th, info).Sz(SmallText).C(rgb(0x888888)).W())(gtx)
				})
				call := m.Stop()
				if cl.Hovered() {
					Fill(gtx, image.Rectangle{Max: dd.Size}, ColHover)
				}
				call.Add(gtx.Ops)
				pointerCursor(gtx, dd.Size)
				return dd
			})
		})
	})
}

func (a *App) targetCard(d view.DamageView) layout.Widget {
	th := a.Th
	return func(gtx C) D {
		m := d.Target
		return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(4, 6, 4, 6)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			head := Row(6,
				func(gtx C) D { return a.Icons.Icon(gtx, view.MonsterIcon(m.ID), 40) },
				func(gtx C) D {
					sub := fmt.Sprintf("#%d", m.ID)
					if m.Source != "" {
						sub += " · " + a.T("atributos") + ": " + m.Source
					}
					return Col(0, B(th, m.NameIn(a.Lang)).W(), T(th, sub).Sz(SmallText).C(ColMuted).W())(gtx)
				},
			)
			grid := func(gtx C) D {
				// Two label/value pairs per row.
				var rows []layout.Widget
				for i := 0; i < len(d.TargetInfo); i += 2 {
					pair := d.TargetInfo[i:min(i+2, len(d.TargetInfo))]
					rows = append(rows, func(gtx C) D {
						var kids []layout.FlexChild
						for _, p := range pair {
							p := p
							kids = append(kids, layout.Flexed(1, Row(4, T(th, a.T(p.Label)).C(ColLabel).W(), B(th, a.T(p.Value)).W())))
						}
						return layout.Flex{}.Layout(gtx, kids...)
					})
				}
				return Col(1, rows...)(gtx)
			}
			return Col(4, head, grid)(gtx)
		})
	}
}

func (a *App) damageTable(d view.DamageView) layout.Widget {
	th := a.Th
	num := func(n int64) string { return view.FmtInt(n) }
	f1 := func(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
	return func(gtx C) D {
		row := func(label string, value ...layout.Widget) layout.Widget {
			return func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(Fixed(130, T(th, a.T(label)).C(ColLabel).W())),
					layout.Flexed(1, Row(4, value...)),
				)
			}
		}
		small := func(s string) layout.Widget { return T(th, s).Sz(SmallText).C(ColMuted).W() }
		dm := d.Dmg
		rows := []layout.Widget{
			row("Dano normal", B(th, num(dm.Min)+" ~ "+num(dm.Max)).W(), small(a.T("(média")+" "+num(dm.Avg)+")")),
		}
		if dm.Crit > 0 {
			rows = append(rows, row("Crítico", B(th, num(dm.Crit)).W(), small(a.T("chance")+" "+f1(dm.CritChance)+"%")))
		}
		rows = append(rows,
			row("Chance de acerto", T(th, f1(dm.HitChance)+"%").W()),
			row("Dano esperado", T(th, num(int64(dm.Expected+0.5))).W(), small(a.T("por uso, com erros e críticos"))),
			row("Usos para matar", B(th, strconv.Itoa(dm.HitsToKill)).W(), small(a.T("(dano médio)"))),
			row("Elemento do ataque", T(th, d.ElementName).W()),
		)
		return Col(2, rows...)(gtx)
	}
}

// skillTableW is "Dano de cada habilidade": click a row to use that attack.
func (a *App) skillTableW(gtx C, d view.DamageView) D {
	th, s := a.Th, &a.damage
	if len(s.rowPicks) < len(d.Table) {
		s.rowPicks = make([]widget.Clickable, len(d.Table))
	}
	for i, r := range d.Table {
		if s.rowPicks[i].Clicked(gtx) && !r.Selected {
			id, lv := r.ID, r.Level
			a.setAtk(func(at *calc.AttackChoice) { at.Skill, at.Level, at.Ratio, at.Variant, at.Hits = id, lv, 0, 0, 0 })
		}
	}
	num := func(n int64) string { return view.FmtInt(n) }
	f1 := func(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
	cols := []struct {
		w     int
		title string
	}{{0, "Habilidade"}, {28, "Nv"}, {62, "Média"}, {62, "Crítico"}, {84, "Acerto"}, {66, "Esperado"}, {38, "Usos"}}
	cell := func(i int, w layout.Widget) layout.FlexChild {
		if cols[i].w == 0 {
			return layout.Flexed(1, w)
		}
		return layout.Rigid(Fixed(unit.Dp(cols[i].w), w))
	}
	header := func(gtx C) D {
		var kids []layout.FlexChild
		for i, c := range cols {
			kids = append(kids, cell(i, B(th, a.T(c.title)).Sz(SmallText).C(ColLabel).W()))
		}
		return layout.Flex{}.Layout(gtx, kids...)
	}
	rows := []layout.Widget{header}
	for i, r := range d.Table {
		i, r := i, r
		rows = append(rows, func(gtx C) D {
			cl := &s.rowPicks[i]
			return cl.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				name := r.Name
				if r.Hits > 1 {
					name += fmt.Sprintf(" ×%d", r.Hits)
				}
				if r.FromDesc {
					name += " *"
				}
				lv, crit, hit := "—", "—", f1(r.HitChance)+"%"
				if r.ID != 0 {
					lv = strconv.Itoa(r.Level)
				}
				if r.Crit > 0 {
					crit = num(r.Crit)
					hit += " · " + f1(r.CritChance) + "%"
				}
				m := op.Record(gtx.Ops)
				dd := Inset(1, 2, 1, 2).Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						cell(0, T(th, name).W()),
						cell(1, T(th, lv).W()),
						cell(2, T(th, num(r.Avg)).W()),
						cell(3, T(th, crit).W()),
						cell(4, T(th, hit).Sz(SmallText).W()),
						cell(5, B(th, num(int64(r.Expected+0.5))).W()),
						cell(6, T(th, strconv.Itoa(r.HitsToKill)).W()),
					)
				})
				call := m.Stop()
				switch {
				case r.Selected:
					Fill(gtx, image.Rectangle{Max: dd.Size}, ColSelect)
				case cl.Hovered():
					Fill(gtx, image.Rectangle{Max: dd.Size}, ColHover)
				}
				call.Add(gtx.Ops)
				pointerCursor(gtx, dd.Size)
				return dd
			})
		})
	}
	rows = append(rows, T(th, a.T("Cada habilidade no nível aprendido, com os mesmos estados e elemento do ataque escolhido. * = % lido da descrição.")).Sz(SmallText).C(ColMuted).Wrapping().W())
	return Col(1, rows...)(gtx)
}

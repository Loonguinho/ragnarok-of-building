package ui

import (
	"image"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/edit"
	"ragnarok-of-building/internal/view"
)

// ---- Buffs e Consumíveis (win-buffs) ----

type buffsState struct {
	q       Input
	rows    map[string]*buffRow
	groups  map[string]*Details
}

type buffRow struct {
	yn  YesNo
	sel Select
	num NumInput
	tip widget.Clickable
}

func (s *buffsState) row(key string) *buffRow {
	if s.rows == nil {
		s.rows = map[string]*buffRow{}
	}
	r := s.rows[key]
	if r == nil {
		r = &buffRow{}
		s.rows[key] = r
	}
	return r
}

// Grid lays widgets in cols columns of equal width.
func Grid(cols int, gapX, gapY int, ws []layout.Widget) layout.Widget {
	return func(gtx C) D {
		if len(ws) == 0 {
			return D{}
		}
		gx, gy := gtx.Dp(unit.Dp(gapX)), gtx.Dp(unit.Dp(gapY))
		cw := (gtx.Constraints.Max.X - (cols-1)*gx) / cols
		y := 0
		for i := 0; i < len(ws); i += cols {
			rowH := 0
			for c := 0; c < cols && i+c < len(ws); c++ {
				g := gtx
				g.Constraints = layout.Constraints{Min: image.Pt(cw, 0), Max: image.Pt(cw, gtx.Constraints.Max.Y)}
				st := op.Offset(image.Pt(c*(cw+gx), y)).Push(gtx.Ops)
				d := ws[i+c](g)
				st.Pop()
				rowH = max(rowH, d.Size.Y)
			}
			y += rowH + gy
		}
		return D{Size: image.Pt(gtx.Constraints.Max.X, y-gy)}
	}
}

func (a *App) buffsBody(gtx C) D {
	th, s := a.Th, &a.buffs
	v := a.View()
	bv := v.Buffs
	b := v.Build
	q := strings.ToLower(strings.TrimSpace(s.q.Text()))
	match := func(label string) bool { return q == "" || strings.Contains(strings.ToLower(label), q) }

	// Exclusive groups: the keys sharing an Excl.
	stateExcl := map[string][]string{}
	for _, it := range bv.Active {
		if it.Excl != "" {
			stateExcl[it.Excl] = append(stateExcl[it.Excl], it.Key)
		}
	}
	buffExcl := map[string][]string{}
	for _, g := range bv.Groups {
		for _, it := range g.Items {
			if it.Excl != "" && !it.State {
				buffExcl[it.Excl] = append(buffExcl[it.Excl], it.Key)
			}
		}
	}

	head := func(title, sub string) layout.Widget {
		return func(gtx C) D {
			ws := []layout.Widget{B(th, a.T(title)).C(ColTitleText).W()}
			if sub != "" {
				ws = append(ws, T(th, a.T(sub)).Sz(SmallText).C(ColMuted).W())
			}
			return Row(4, ws...)(gtx)
		}
	}
	kids := []layout.Widget{
		func(gtx C) D {
			s.q.Hint = a.T("Buscar habilidades e buffs")
			d, _, _ := s.q.Layout(gtx, th, 0)
			return d
		},
	}

	// Item effects (procs).
	var fx []layout.Widget
	for _, it := range bv.Items {
		if !match(it.Label + " " + it.Gives) {
			continue
		}
		it := it
		fx = append(fx, a.fxRow(it.Key, it.Value > 0, view.ItemIcon(it.ItemID), it.Label, it.Gives, func(gtx C) D {
			r := s.row(it.Key)
			d, val, ok := r.yn.Layout(gtx, th, it.Value > 0, a.T("Sim"), a.T("Não"))
			if ok {
				a.SetBuild(edit.SetBuff(b, it.Key, val, nil))
			}
			return d
		}))
	}
	if len(fx) > 0 {
		kids = append(kids, head("Efeitos de itens", "(ativados ao atacar, ao ser atingido ou ao usar habilidades)"), Grid(1, 8, 2, fx))
	}

	// Class buffs and formula states.
	var act []layout.Widget
	for _, it := range bv.Active {
		if match(it.Label) {
			act = append(act, a.buffItemRow(it, b, stateExcl, buffExcl))
		}
	}
	if len(act) > 0 {
		kids = append(kids, head("Habilidades e efeitos ativos", ""), Grid(2, 8, 2, act))
	}

	// Passives at the skill tree's level.
	var pas []layout.Widget
	for _, p := range bv.Passives {
		if !match(p.Label) {
			continue
		}
		p := p
		pas = append(pas, a.fxRow("learn:"+strconv.Itoa(p.SkillID), p.Level > 0, view.SkillIcon(p.SkillID), p.Label, "", func(gtx C) D {
			r := s.row("learn:" + strconv.Itoa(p.SkillID))
			opts := []Option{{"0", "—"}}
			for i := 1; i <= p.Max; i++ {
				opts = append(opts, Option{strconv.Itoa(i), a.T("Nv.") + " " + strconv.Itoa(i)})
			}
			d, val, ok := r.sel.Layout(gtx, th, a.Desk, opts, strconv.Itoa(p.Level), 64)
			if ok {
				lv, _ := strconv.Atoi(val)
				id := p.SkillID
				a.Edit(func(b *calc.Build) {
					if b.Skills == nil {
						b.Skills = map[int]int{}
					}
					if lv > 0 {
						b.Skills[id] = lv
					} else {
						delete(b.Skills, id)
					}
				})
			}
			return d
		}))
	}
	if len(pas) > 0 {
		kids = append(kids, head("Aprenda para ganhar bônus", "(o mesmo nível da árvore de habilidades)"), Grid(2, 8, 2, pas))
	}

	// Party buffs, consumables, other.
	kids = append(kids, head("Buffs", ""))
	if s.groups == nil {
		s.groups = map[string]*Details{}
	}
	for _, g := range bv.Groups {
		var rows []layout.Widget
		for _, it := range g.Items {
			if match(it.Label) {
				rows = append(rows, a.buffItemRow(it, b, stateExcl, buffExcl))
			}
		}
		if len(rows) == 0 {
			continue
		}
		g := g
		det := s.groups[g.Name]
		if det == nil {
			det = &Details{}
			s.groups[g.Name] = det
		}
		title := a.T(g.Name)
		if g.Active > 0 {
			title += " (" + strconv.Itoa(g.Active) + ")"
		}
		kids = append(kids, func(gtx C) D {
			if q != "" {
				det.Open, det.init = true, true
			}
			return det.Layout(gtx, th, title, g.Open, Grid(2, 8, 2, rows))
		})
	}
	kids = append(kids, T(th, a.T("Valores das descrições do cliente, do bROWiki e do latam-ro-calc. Os buffs entram nos atributos, no status e no dano; tempo de conjuração, velocidade de movimento e regeneração ficam de fora.")).Sz(SmallText).C(ColMuted).Wrapping().W())
	return Col(5, kids...)(gtx)
}

// fxRow is one row: icon, name (and what it gives), the control.
func (a *App) fxRow(key string, on bool, icon, label, gives string, control layout.Widget) layout.Widget {
	th := a.Th
	return func(gtx C) D {
		m := op.Record(gtx.Ops)
		d := Inset(1, 2, 1, 2).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if icon == "" {
						return D{Size: image.Pt(gtx.Dp(20), gtx.Dp(20))}
					}
					return a.Icons.Icon(gtx, icon, 20)
				}),
				layout.Rigid(Gap(4, 0)),
				layout.Flexed(1, func(gtx C) D {
					if gives == "" {
						return T(th, label).Layout(gtx)
					}
					return Col(0, T(th, label).W(), T(th, gives).Sz(SmallText).C(ColMuted).Wrapping().W())(gtx)
				}),
				layout.Rigid(Gap(4, 0)),
				layout.Rigid(control),
			)
		})
		call := m.Stop()
		if on {
			FillRound(gtx, image.Rectangle{Max: d.Size}, rgb(0xeef4ff), 3, 3, 3, 3)
		}
		call.Add(gtx.Ops)
		return d
	}
}

// buffItemRow is a buff or class state with its control (Sim/Não, a
// select or a number).
func (a *App) buffItemRow(it view.BuffItem, b calc.Build, stateExcl, buffExcl map[string][]string) layout.Widget {
	th, s := a.Th, &a.buffs
	icon := ""
	switch {
	case it.SkillID > 0:
		icon = view.SkillIcon(it.SkillID)
	case it.ItemID > 0:
		icon = view.ItemIcon(it.ItemID)
	}
	r := s.row(it.Key)
	control := func(gtx C) D {
		switch {
		case it.State && it.Toggle:
			d, val, ok := r.yn.Layout(gtx, th, it.Value != 0, a.T("Sim"), a.T("Não"))
			if ok {
				on, _ := strconv.Atoi(it.OnValue)
				if on == 0 {
					on = 1
				}
				a.SetBuild(edit.SetState(b, it.Key, val*on, stateExcl[it.Excl]))
			}
			return d
		case it.Choices != nil:
			var opts []Option
			for _, c := range it.Choices {
				opts = append(opts, Option{c[0], a.T(c[1])})
			}
			d, val, ok := r.sel.Layout(gtx, th, a.Desk, opts, it.ValueStr, 110)
			if ok {
				x, _ := strconv.Atoi(val)
				if it.State {
					a.SetBuild(edit.SetState(b, it.Key, x, stateExcl[it.Excl]))
				} else {
					a.SetBuild(edit.SetBuff(b, it.Key, x, buffExcl[it.Excl]))
				}
			}
			return d
		case it.Max > 0:
			d, x, ok := r.num.Layout(gtx, th, it.Value, 50)
			if ok && x >= 0 && x <= it.Max {
				a.SetBuild(edit.SetBuff(b, it.Key, x, nil))
			}
			return d
		default:
			d, val, ok := r.yn.Layout(gtx, th, it.Value != 0, a.T("Sim"), a.T("Não"))
			if ok {
				a.SetBuild(edit.SetBuff(b, it.Key, val, buffExcl[it.Excl]))
			}
			return d
		}
	}
	row := a.fxRow(it.Key, it.Value != 0, icon, it.Label, "", control)
	if it.ItemID == 0 {
		return row
	}
	// Consumables: the item tooltip on hover.
	return func(gtx C) D {
		d := r.tip.Layout(gtx, row)
		if r.tip.Hovered() {
			id := it.ItemID
			a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
		}
		return d
	}
}


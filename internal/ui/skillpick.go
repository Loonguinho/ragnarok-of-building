package ui

import (
	"image"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/gamedata"
	"ragnarok-of-building/internal/view"
)

// skillPick is the Resumo's main skill picker: Ataque básico, then the
// learned active skills as their icons, in a tab per class of the line
// (1ª, 2ª, Trans., 3ª, 4ª; the class name in the tab's tooltip).
type skillPick struct {
	job   int // the class the tab below was chosen for
	tab   int // index in the tabs shown
	basic widget.Clickable
	tabs  []widget.Clickable
	cells map[int]*widget.Clickable
}

type pickTab struct {
	label, name string
	ids         []int
}

// tierLabel is the class's place in its line: 1ª/2ª by depth for the
// normal classes, Trans. for the transcendent ones, 3ª and 4ª.
func tierLabel(c *gamedata.Catalog, jobID int) string {
	j := c.Jobs[jobID]
	if j == nil {
		return "?"
	}
	switch j.EquipTier {
	case "Fourth":
		return "4ª"
	case "Third", "Third_Upper":
		return "3ª"
	case "Upper":
		return "Trans."
	}
	if j.Fourth {
		return "4ª"
	}
	depth := len(c.Chain(jobID)) - 1 // Aprendiz is 0
	if depth <= 0 {
		return "Aprendiz"
	}
	return strconv.Itoa(depth) + "ª"
}

func (a *App) skillPickTabs(b calc.Build, d view.DamageView) []pickTab {
	c := a.V.Catalog()
	offered := map[int]bool{}
	for _, sk := range d.Skills {
		offered[sk.ID] = true
	}
	tree := a.skillTree(b).tree
	var out []pickTab
	seen := map[int]bool{}
	for _, t := range tree.Tabs {
		pt := pickTab{label: a.T(tierLabel(c, t.JobID)), name: t.Name}
		for _, row := range t.Rows {
			for _, cell := range row {
				if cell.ID > 0 && offered[cell.ID] && !seen[cell.ID] {
					seen[cell.ID] = true
					pt.ids = append(pt.ids, cell.ID)
				}
			}
		}
		if len(pt.ids) > 0 {
			out = append(out, pt)
		}
	}
	// Skills the tree does not show (granted ones) go in a last tab.
	var rest []int
	for _, sk := range d.Skills {
		if !seen[sk.ID] {
			rest = append(rest, sk.ID)
		}
	}
	if len(rest) > 0 {
		out = append(out, pickTab{label: a.T("Outras"), name: a.T("Outras habilidades"), ids: rest})
	}
	return out
}

// skillPicker lays the picker out and applies a pick.
func (a *App) skillPicker(gtx C, b calc.Build, d view.DamageView) D {
	th, p := a.Th, &a.shell.pick
	c := a.V.Catalog()
	tabs := a.skillPickTabs(b, d)
	if p.cells == nil {
		p.cells = map[int]*widget.Clickable{}
	}
	if len(p.tabs) < len(tabs) {
		p.tabs = append(p.tabs, make([]widget.Clickable, len(tabs)-len(p.tabs))...)
	}
	pick := func(id int) {
		lv := b.Skills[id]
		a.setAtk(func(at *calc.AttackChoice) { at.Skill, at.Level, at.Ratio, at.Variant = id, lv, 0, 0 })
	}
	if p.basic.Clicked(gtx) {
		pick(0)
	}
	// A new class, or a selected skill in another tab: open its tab.
	selTab := -1
	for i, t := range tabs {
		for _, id := range t.ids {
			if id == d.Atk.Skill {
				selTab = i
			}
		}
	}
	if p.job != b.Job {
		p.job, p.tab = b.Job, max(len(tabs)-1, 0)
		if selTab >= 0 {
			p.tab = selTab
		}
	}
	for i := range tabs {
		if p.tabs[i].Clicked(gtx) {
			p.tab = i
		}
	}
	p.tab = min(p.tab, max(len(tabs)-1, 0))

	cellSz := gtx.Dp(34)
	tabRow := func(gtx C) D {
		var ws []layout.Widget
		ws = append(ws, func(gtx C) D {
			return Button{Label: a.T("Ataque básico"), Bold: d.Atk.Skill == 0}.Layout(gtx, th, &p.basic)
		})
		for i, t := range tabs {
			i, t := i, t
			ws = append(ws, func(gtx C) D {
				dd := Button{Label: t.label, Bold: i == p.tab}.Layout(gtx, th, &p.tabs[i])
				if p.tabs[i].Hovered() {
					name := t.name
					a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(th, name).W()) })
				}
				if i == p.tab {
					Fill(gtx, image.Rect(gtx.Dp(3), dd.Size.Y-gtx.Dp(2), dd.Size.X-gtx.Dp(3), dd.Size.Y), ColTitleText)
				}
				return dd
			})
		}
		return Row(4, ws...)(gtx)
	}
	grid := func(gtx C) D {
		if len(tabs) == 0 {
			return T(th, a.T("Nenhuma habilidade ativa aprendida.")).C(ColMuted).Layout(gtx)
		}
		gap := gtx.Dp(3)
		perRow := max(1, (gtx.Constraints.Max.X+gap)/(cellSz+gap))
		ids := tabs[p.tab].ids
		y := 0
		for i, id := range ids {
			id := id
			cl := p.cells[id]
			if cl == nil {
				cl = &widget.Clickable{}
				p.cells[id] = cl
			}
			if cl.Clicked(gtx) {
				pick(id)
			}
			col, row := i%perRow, i/perRow
			y = (row + 1) * (cellSz + gap)
			g := gtx
			g.Constraints = layout.Exact(image.Pt(cellSz, cellSz))
			off := image.Pt(col*(cellSz+gap), row*(cellSz+gap))
			func() {
				st := op.Offset(off).Push(g.Ops)
				defer st.Pop()
				cl.Layout(g, func(gtx C) D {
					r := image.Rectangle{Max: image.Pt(cellSz, cellSz)}
					switch {
					case id == d.Atk.Skill:
						FillRound(gtx, r, ColSelect, 3, 3, 3, 3)
						Border(gtx, r, ColTitleText, 1, 3)
					case cl.Hovered():
						FillRound(gtx, r, ColHover, 3, 3, 3, 3)
						Border(gtx, r, ColListBorder, 1, 3)
					default:
						FillRound(gtx, r, ColWhite, 3, 3, 3, 3)
						Border(gtx, r, ColListBorder, 1, 3)
					}
					layout.Center.Layout(gtx, func(gtx C) D { return a.Icons.Icon(gtx, view.SkillIcon(id), 24) })
					pointerCursor(gtx, r.Max)
					return D{Size: r.Max}
				})
			}()
			if cl.Hovered() {
				name := ""
				if sk := c.Skills[id]; sk != nil {
					name = sk.Name.Get(a.Lang) + " " + a.T("Nv.") + " " + strconv.Itoa(b.Skills[id])
				}
				a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(th, name).W()) })
			}
		}
		return D{Size: image.Pt(gtx.Constraints.Max.X, max(y-gap, 0))}
	}
	current := func(gtx C) D {
		name := a.T("Ataque básico")
		if sk := c.Skills[d.Atk.Skill]; sk != nil && d.Atk.Skill != 0 {
			name = sk.Name.Get(a.Lang)
			if lv := d.Atk.Level; lv > 0 {
				name += " " + a.T("Nv.") + " " + strconv.Itoa(lv)
			}
		}
		return B(th, name).Layout(gtx)
	}
	return Col(unit.Dp(5), tabRow, grid, current)(gtx)
}

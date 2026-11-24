package ui

import (
	"fmt"
	"image"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/edit"
	"ragnarok-of-building/internal/view"
)

// ---- Habilidades (win-skills) ----

type skillsState struct {
	treeKey string // job/joblv/lang of the tree below
	job     int
	tree    view.SkillTree
	rules   edit.Tree
	tabs    Tabs
	tab     int
	cells   map[int]*skillCell
	reset   widget.Clickable
	msg     string
	msgAt   time.Time
}

type skillCell struct {
	tag   bool
	hover gesture.Hover
}

func (a *App) skillTree(b calc.Build) *skillsState {
	s := &a.skills
	key := fmt.Sprintf("%d/%d/%s", b.Job, b.JobLv, a.Lang)
	if key != s.treeKey {
		s.tree, _ = a.V.SkillTree(b.Job, b.JobLv, a.Lang)
		s.rules = edit.Tree{Skills: map[int]edit.Skill{}}
		for id, sk := range s.tree.Skills {
			s.rules.Skills[id] = edit.Skill{Max: sk.Max, Needs: sk.Needs, Group: sk.Group, Free: sk.Free}
		}
		for _, g := range s.tree.Groups {
			s.rules.Budgets = append(s.rules.Budgets, g.Budget)
		}
		// A new class opens on its own (last) tab.
		if s.treeKey == "" || s.job != b.Job {
			s.tab = len(s.tree.Tabs) - 1
		}
		s.job = b.Job
		s.tab = min(max(s.tab, 0), max(len(s.tree.Tabs)-1, 0))
		s.treeKey = key
	}
	return s
}

func (a *App) setSkills(lv map[int]int) {
	a.Edit(func(b *calc.Build) { b.Skills = lv })
}

func (a *App) skillsBody(gtx C) D {
	th := a.Th
	v := a.View()
	b := v.Build
	s := a.skillTree(b)
	if len(s.tree.Tabs) == 0 {
		return T(th, a.T("Escolha uma classe.")).Layout(gtx)
	}
	if s.reset.Clicked(gtx) {
		a.setSkills(nil)
	}
	labels := make([]string, len(s.tree.Tabs))
	for i, t := range s.tree.Tabs {
		labels[i] = t.Name
	}
	tab := s.tree.Tabs[s.tab]
	return Col(0,
		func(gtx C) D {
			gtx.Constraints.Max.X += gtx.Dp(16)
			st := op.Offset(image.Pt(-gtx.Dp(8), -gtx.Dp(6))).Push(gtx.Ops)
			d, i, ok := s.tabs.Layout(gtx, th, labels, s.tab)
			st.Pop()
			if ok {
				s.tab = i
			}
			d.Size.Y -= gtx.Dp(6)
			return d
		},
		Gap(0, 6),
		func(gtx C) D {
			return Box{Bg: ColPanel, Line: ColListBorder, Pad: Inset(6, 6, 6, 6)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				var rows []layout.Widget
				for _, row := range tab.Rows {
					row := row
					rows = append(rows, func(gtx C) D {
						var cells []layout.Widget
						for _, c := range row {
							c := c
							cells = append(cells, func(gtx C) D { return a.skillCellW(gtx, c, b.Skills) })
						}
						return Grid(len(cells), 4, 0, cells)(gtx)
					})
				}
				return Col(4, rows...)(gtx)
			})
		},
	)(gtx)
}

func (a *App) skillsFooter(gtx C) D {
	th := a.Th
	v := a.View()
	b := v.Build
	s := a.skillTree(b)
	if len(s.tree.Tabs) == 0 {
		return D{}
	}
	used := s.rules.Used(b.Skills)
	jobID := s.tree.Tabs[s.tab].JobID
	gi := 0
	for i, g := range s.tree.Groups {
		for _, j := range g.Jobs {
			if j == jobID {
				gi = i
			}
		}
	}
	known := len(s.tree.Groups) > 0
	total, spent := 0, 0
	for i, g := range s.tree.Groups {
		known = known && g.Budget >= 0
		total += g.Budget
		spent += used[i]
	}
	budget, left := "–", "–"
	usedHere := 0
	if gi < len(used) {
		usedHere = used[gi]
	}
	if known {
		budget = fmt.Sprint(s.tree.Groups[gi].Budget)
		left = fmt.Sprint(total - spent)
	}
	leftColor := ColText
	if known && (total-spent < 0 || !s.rules.Fits(b.Skills)) {
		leftColor = ColDown
	}
	msg := ""
	if s.msg != "" && time.Since(s.msgAt) < 2500*time.Millisecond {
		msg = s.msg
		gtx.Execute(op.InvalidateCmd{At: s.msgAt.Add(2500 * time.Millisecond)})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, Row(4,
			T(th, a.T("Esta classe:")).W(),
			B(th, fmt.Sprintf("%d/%s", usedHere, budget)).W(),
			T(th, a.T("· Restantes:")).W(),
			B(th, left).C(leftColor).W(),
			T(th, msg).C(ColDown).W(),
		)),
		layout.Rigid(func(gtx C) D { return Button{Label: a.T("Resetar")}.Layout(gtx, th, &s.reset) }),
	)
}

// skillCellW is one skill: icon, name and level; clicks raise or lower it
// as in the client (shift: as far as it goes).
func (a *App) skillCellW(gtx C, c view.SkillCell, levels map[int]int) D {
	th, s := a.Th, &a.skills
	h := gtx.Dp(58)
	if c.Empty {
		return D{Size: image.Pt(gtx.Constraints.Min.X, h)}
	}
	if s.cells == nil {
		s.cells = map[int]*skillCell{}
	}
	cell := s.cells[c.ID]
	if cell == nil {
		cell = &skillCell{}
		s.cells[c.ID] = cell
	}
	// Clicks: left +1 (shift: max), right -1 (shift: 0).
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &cell.tag, Kinds: pointer.Press})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		shift := e.Modifiers.Contain(key.ModShift)
		lv := levels[c.ID]
		switch {
		case e.Buttons.Contain(pointer.ButtonSecondary):
			to := lv - 1
			if shift {
				to = 0
			}
			a.setSkills(s.rules.Lower(levels, c.ID, to))
		case e.Buttons.Contain(pointer.ButtonPrimary):
			var nl map[int]int
			var ok bool
			if shift {
				nl, ok = s.rules.Max(levels, c.ID)
			} else {
				nl, ok = s.rules.Raise(levels, c.ID, lv+1)
			}
			if ok {
				a.setSkills(nl)
			} else if lv < s.rules.Skills[c.ID].Max {
				s.msg, s.msgAt = a.T("Pontos de habilidade insuficientes"), time.Now()
			}
		}
	}
	lv := levels[c.ID]
	locked := lv == 0 && s.rules.Locked(levels, c.ID)
	w := gtx.Constraints.Min.X
	r := image.Rectangle{Max: image.Pt(w, h)}
	hovered := cell.hover.Update(gtx.Source)
	if hovered {
		FillRound(gtx, r, ColHover, 3, 3, 3, 3)
		Border(gtx, r, ColSelect, 1, 3)
	}
	g := gtx
	g.Constraints = layout.Exact(image.Pt(w, h))
	layout.N.Layout(g, func(gtx C) D {
		gtx.Constraints.Min.X = 0
		return Col(1,
			func(gtx C) D {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(w, gtx.Dp(26)))
				return layout.Center.Layout(g, func(gtx C) D { return a.Icons.Icon(gtx, view.SkillIcon(c.ID), 24) })
			},
			func(gtx C) D {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(w, gtx.Dp(20)))
				col := rgb(0x333333)
				if locked {
					col = rgb(0x999999)
				}
				return T(th, c.Name).Sz(TinyText).C(col).Lines(2).Wrapping().Center().Layout(g)
			},
			func(gtx C) D {
				g := gtx
				g.Constraints = layout.Exact(image.Pt(w, gtx.Dp(12)))
				lc := ColLink
				if lv > 0 {
					lc = rgb(0xc2410c)
				}
				return layout.Center.Layout(g, Row(0, B(th, fmt.Sprint(lv)).Sz(SmallText).C(lc).W(), T(th, fmt.Sprintf("/%d", c.MaxLv)).Sz(SmallText).C(rgb(0x555555)).W()))
			},
		)(gtx)
	})
	if locked {
		veil := ColPanel // fades the icon into the panel
		veil.A = 0x70
		Fill(gtx, r, veil)
	}
	area := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, &cell.tag)
	cell.hover.Add(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	area.Pop()
	if hovered && len(c.Desc) > 0 {
		desc := c.Desc
		name := c.Name
		a.Desk.ShowTip(func(gtx C) D {
			return TipBox(gtx, Col(3, B(th, name).W(), RichText(th, desc, ColText)))
		})
	}
	return D{Size: r.Max}
}

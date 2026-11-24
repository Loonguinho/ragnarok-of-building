package ui

import (
	"fmt"
	"image"
	"maps"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// ---- Comparar itens (win-compare) ----
//
// The build as it is next to up to three items pinned from the slot picker
// (↔), each column with its own refine, grade and item effects on or off,
// and the stats and damage of every column.

const compareMax = 4 // columns, the equipped one included

type compareState struct {
	slot string
	cols []view.CompareCand // [0]: the build as it is

	key  string // build + columns of the last computation
	seq  int
	res  *view.CompareView
	busy bool

	ref, gra      [compareMax]Select
	pos           [compareMax][4]Select
	posOpts       [compareMax][4][]Option // options of res's positions
	posFor        *view.CompareView
	fx            [compareMax]map[string]*YesNo
	equip, remove [compareMax]widget.Clickable
	clear         widget.Clickable
}

// addCompare pins an item of the slot picker as a column and opens the
// window.
func (a *App) addCompare(id int) {
	s, cs := &a.slot, &a.cmp
	if cs.slot != s.key || len(cs.cols) == 0 {
		cs.slot, cs.cols = s.key, []view.CompareCand{{}}
	}
	for _, c := range cs.cols[1:] {
		if c.Item == id {
			a.showCompare()
			return
		}
	}
	if len(cs.cols) >= compareMax {
		a.Say(a.T("A comparação já tem 3 itens: remova um para fixar outro."), true)
		a.showCompare()
		return
	}
	e := a.Build().Equip[s.key]
	refine, grade := e.Refine, e.Grade
	if s.bestOn {
		refine, grade = s.bestRefine, s.bestGrade
	}
	cs.cols = append(cs.cols, view.CompareCand{Item: id, Refine: refine, Grade: grade})
	a.showCompare()
}

// addCompareCard pins the slot's item with card id in the position being
// picked (everything else as worn) to the comparison.
func (a *App) addCompareCard(id int) {
	s, cs := &a.slot, &a.cmp
	if s.card < 0 {
		return
	}
	if cs.slot != s.key || len(cs.cols) == 0 {
		cs.slot, cs.cols = s.key, []view.CompareCand{{}}
	}
	e := a.Build().Equip[s.key]
	cards := e.Cards
	cards[s.card] = id
	for _, c := range cs.cols[1:] {
		if c.Item == 0 && c.Cards != nil && *c.Cards == cards {
			a.showCompare()
			return
		}
	}
	if len(cs.cols) >= compareMax {
		a.Say(a.T("A comparação já tem 3 itens: remova um para fixar outro."), true)
		a.showCompare()
		return
	}
	cs.cols = append(cs.cols, view.CompareCand{Cards: &cards, Refine: e.Refine, Grade: e.Grade})
	a.showCompare()
}

func (a *App) showCompare() {
	if !a.wCompare.Open {
		a.Desk.Toggle(a.wCompare)
	}
	a.Desk.Raise(a.wCompare)
}

// compareRun recomputes when the build or the columns changed.
func (a *App) compareRun() {
	cs := &a.cmp
	b := a.Build()
	key := view.EncodeBuild(b) + "|" + cs.slot + "|" + fmt.Sprint(cs.cols)
	if key == cs.key {
		return
	}
	cs.key = key
	cols := make([]view.CompareCand, len(cs.cols))
	for i, c := range cs.cols {
		c.Fx = maps.Clone(c.Fx)
		cols[i] = c
	}
	a.mu.Lock()
	cs.seq++
	seq, slot, lang := cs.seq, cs.slot, a.Lang
	cs.busy = true
	a.mu.Unlock()
	go func() {
		res := a.V.Compare(b, slot, cols, lang)
		a.mu.Lock()
		if seq == cs.seq {
			cs.res, cs.busy = &res, false
		}
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

func (a *App) compareBody(gtx C) D {
	th, cs := a.Th, &a.cmp
	if len(cs.cols) < 2 {
		return T(th, a.T("Fixe itens aqui com o botão ↔ da lista de itens de um slot (Alt+Q, clique no slot).")).C(ColMuted).Wrapping().Layout(gtx)
	}
	// Actions.
	if cs.clear.Clicked(gtx) {
		cs.cols = cs.cols[:1]
		return D{}
	}
	for i := 1; i < len(cs.cols); i++ {
		if cs.remove[i].Clicked(gtx) {
			cs.cols = append(cs.cols[:i:i], cs.cols[i+1:]...)
			cs.fx[i] = nil
			return D{}
		}
		if cs.equip[i].Clicked(gtx) {
			b := a.Build()
			a.SetBuild(a.V.CompareBuild(b, cs.slot, cs.cols[i]))
			a.Say(a.T("Item equipado."), false)
			cs.cols = append(cs.cols[:i:i], cs.cols[i+1:]...)
			cs.cols[0] = view.CompareCand{}
			a.equipDone()
			return D{}
		}
	}
	a.compareRun()
	a.mu.Lock()
	res, busy := cs.res, cs.busy
	a.mu.Unlock()
	if res == nil || len(res.Cols) != len(cs.cols) {
		return T(th, a.T("Calculando…")).C(ColMuted).Layout(gtx)
	}
	_, label := a.slotView(a.View(), cs.slot)
	head := a.T(label)
	if res.Target != "" {
		head += " · " + a.T("alvo") + ": " + res.Target
	} else {
		head += " · " + a.T("escolha um monstro na janela de dano (Alt+D) para ver o dano")
	}
	if busy {
		head += " · " + a.T("Calculando…")
	}
	n := len(res.Cols)
	return Col(6,
		Row(6, T(th, head).C(ColLabel).W(), func(gtx C) D {
			return Button{Label: a.T("Limpar")}.Layout(gtx, th, &cs.clear)
		}),
		func(gtx C) D {
			return compareGrid(gtx, n, layout.Start, nil, func(gtx C, i int) D { return a.compareCol(gtx, res, i) })
		},
		func(gtx C) D { return a.compareTable(gtx, res) },
	)(gtx)
}

// compareCol is a column's card: item, refine and grade, its effects.
func (a *App) compareCol(gtx C, res *view.CompareView, i int) D {
	th, cs := a.Th, &a.cmp
	col := res.Cols[i]
	cand := &cs.cols[i]
	caption := a.T("Equipado")
	if i > 0 {
		caption = fmt.Sprintf("%s %d", a.T("Opção"), i)
	}
	name := a.T("vazio")
	if col.Item != nil {
		name = itemTitle(col.Item, col.Refine, col.Grade)
	}
	var rows []layout.Widget
	rows = append(rows,
		B(th, caption).Sz(SmallText).C(ColLabel).W(),
		func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if col.Item == nil {
						return D{}
					}
					return a.Icons.Icon(gtx, view.ItemIcon(col.Item.ID), 24)
				}),
				layout.Rigid(Gap(4, 0)),
				layout.Flexed(1, B(th, name).Wrapping().W()),
			)
		},
	)
	if i > 0 && (col.Refineable || col.Gradable) {
		var ctl []layout.Widget
		if col.Refineable {
			var opts []Option
			for r := 0; r <= calc.MaxRefine(cs.slot); r++ {
				opts = append(opts, Option{strconv.Itoa(r), "+" + strconv.Itoa(r)})
			}
			ctl = append(ctl, func(gtx C) D {
				d, val, ok := cs.ref[i].Layout(gtx, th, a.Desk, opts, strconv.Itoa(cand.Refine), 52)
				if ok {
					cand.Refine, _ = strconv.Atoi(val)
				}
				return d
			})
		}
		if col.Gradable {
			opts := []Option{{"0", "—"}, {"1", "D"}, {"2", "C"}, {"3", "B"}, {"4", "A"}}
			ctl = append(ctl, func(gtx C) D {
				d, val, ok := cs.gra[i].Layout(gtx, th, a.Desk, opts, strconv.Itoa(cand.Grade), 70)
				if ok {
					cand.Grade, _ = strconv.Atoi(val)
				}
				return d
			})
		}
		rows = append(rows, Row(4, ctl...))
	}
	if cs.posFor != res {
		cs.posFor = res
		for ci, c := range res.Cols {
			for k := range 4 {
				cs.posOpts[ci][k] = nil
			}
			for _, p := range c.Positions {
				opts := []Option{{"0", "—"}}
				for _, o := range p.Options {
					opts = append(opts, Option{strconv.Itoa(o.ID), o.Name})
				}
				cs.posOpts[ci][p.Index] = opts
			}
		}
	}
	for _, p := range col.Positions {
		if len(p.Options) == 0 && p.Current == 0 {
			continue
		}
		label := fmt.Sprintf("%s %d", a.T("Carta"), p.Index+1)
		if p.Enchant {
			label = fmt.Sprintf("%s %d", a.T("Encant."), p.Index+1)
		}
		rows = append(rows, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(Fixed(52, T(th, label).Sz(SmallText).C(ColMuted).W())),
				layout.Flexed(1, func(gtx C) D {
					cs.pos[i][p.Index].Tip = a.itemTipFor
					cs.pos[i][p.Index].SearchText = a.itemSearchText
					d, val, ok := cs.pos[i][p.Index].Layout(gtx, th, a.Desk, cs.posOpts[i][p.Index], strconv.Itoa(p.Current), 0)
					if ok {
						if cand.Cards == nil {
							var cur [4]int
							for _, q := range col.Positions {
								cur[q.Index] = q.Current
							}
							cand.Cards = &cur
						}
						id, _ := strconv.Atoi(val)
						cards := *cand.Cards
						cards[p.Index] = id
						cand.Cards = &cards
					}
					return d
				}),
			)
		})
	}
	if len(col.Effects) > 0 {
		if cs.fx[i] == nil {
			cs.fx[i] = map[string]*YesNo{}
		}
		rows = append(rows, T(th, a.T("Efeitos")).Sz(SmallText).C(ColMuted).W())
		for _, f := range col.Effects {
			yn := cs.fx[i][f.Key]
			if yn == nil {
				yn = &YesNo{}
				cs.fx[i][f.Key] = yn
			}
			rows = append(rows, func(gtx C) D {
				return Col(1,
					T(th, f.Label).Sz(SmallText).Wrapping().W(),
					T(th, f.Gives).Sz(SmallText).C(ColLink).Wrapping().W(),
					func(gtx C) D {
						d, val, ok := yn.Layout(gtx, th, f.On, a.T("Ativo"), a.T("Inativo"))
						if ok {
							if cand.Fx == nil {
								cand.Fx = map[string]bool{}
							}
							cand.Fx[f.Key] = val == 1
						}
						return d
					},
				)(gtx)
			})
		}
	} else {
		rows = append(rows, T(th, a.T("Sem efeitos ativáveis")).Sz(SmallText).C(ColMuted).W())
	}
	if i > 0 {
		rows = append(rows, Row(4,
			func(gtx C) D { return Button{Label: a.T("Equipar")}.Layout(gtx, th, &cs.equip[i]) },
			func(gtx C) D {
				d := Button{Label: "×", MinWidth: 18}.Layout(gtx, th, &cs.remove[i])
				if cs.remove[i].Hovered() {
					a.Desk.ShowTip(func(gtx C) D { return TipBox(gtx, T(th, a.T("Remover da comparação")).W()) })
				}
				return d
			},
		))
	}
	bg := ColPanel
	if i == 0 {
		bg = rgb(0xf6f8fc)
	}
	return Box{Bg: bg, Line: ColListBorder, Pad: Inset(4, 5, 4, 5)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return Col(3, rows...)(gtx)
	})
}

// compareTable lists the stats of every column with each one's change
// against the equipped column.
func (a *App) compareTable(gtx C, res *view.CompareView) D {
	th := a.Th
	if len(res.Rows) == 0 {
		return T(th, a.T("Nada muda nos atributos.")).C(ColMuted).Layout(gtx)
	}
	n := len(res.Cols)
	var rows []layout.Widget
	for ri, r := range res.Rows {
		rows = append(rows, func(gtx C) D {
			label := func(gtx C) D {
				t := T(th, r.Label).C(ColLabel)
				if r.Main {
					t = t.Bold()
				}
				return Inset(0, 4, 0, 2).Layout(gtx, t.W())
			}
			cell := func(gtx C, i int) D {
				// Inset like the column card's text above it.
				return Inset(0, 6, 0, 2).Layout(gtx, func(gtx C) D {
					v := T(th, r.Values[i])
					if r.Main {
						v = v.Bold()
					}
					parts := []layout.Widget{v.W()}
					if r.Diffs[i] != "" {
						c := ColUp
						if r.Better[i] < 0 {
							c = ColDown
						}
						parts = append(parts, B(th, r.Diffs[i]).Sz(SmallText).C(c).W())
					}
					return Row(4, parts...)(gtx)
				})
			}
			m := op.Record(gtx.Ops)
			d := Inset(2, 0, 2, 0).Layout(gtx, func(gtx C) D {
				return compareGrid(gtx, n, layout.Middle, label, cell)
			})
			call := m.Stop()
			bg := ColPanel
			if ri%2 == 1 {
				bg = rgb(0xf3f5f9)
			}
			Fill(gtx, image.Rectangle{Max: d.Size}, bg)
			call.Add(gtx.Ops)
			return d
		})
	}
	d := Col(0, rows...)(gtx)
	Border(gtx, image.Rectangle{Max: d.Size}, ColListBorder, 1, 0)
	return d
}

// compareLabelW is the stat label column; the column cards leave it empty
// so every value sits under its card.
const compareLabelW = 96

// compareGrid lays the label column and n equal columns 4dp apart: the
// cards and every table row share it, so they line up.
func compareGrid(gtx C, n int, align layout.Alignment, label layout.Widget, cell func(gtx C, i int) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	kids := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		w := gtx.Dp(compareLabelW)
		g := gtx
		g.Constraints = layout.Constraints{Max: image.Pt(w, gtx.Constraints.Max.Y)}
		d := D{}
		if label != nil {
			d = label(g)
		}
		d.Size.X = w
		return d
	})}
	for i := range n {
		if i > 0 {
			kids = append(kids, layout.Rigid(Gap(4, 0)))
		}
		kids = append(kids, layout.Flexed(1, func(gtx C) D { return cell(gtx, i) }))
	}
	return layout.Flex{Alignment: align}.Layout(gtx, kids...)
}

// itemTipFor is the tooltip of an item ID given as text ("0": none), for
// the card and enchant lists.
func (a *App) itemTipFor(value string) layout.Widget {
	id, _ := strconv.Atoi(value)
	if id <= 0 {
		return nil
	}
	return func(gtx C) D { return a.itemTip(gtx, id, nil) }
}

// itemSearchText is an item's description as plain text (by ID given as
// text), for the dropdowns' search.
func (a *App) itemSearchText(value string) string {
	id, _ := strconv.Atoi(value)
	it := a.V.Item(id, a.Lang)
	if it == nil {
		return ""
	}
	var b strings.Builder
	for _, ln := range it.Desc {
		for _, sp := range ColorLine(ln, ColText) {
			b.WriteString(sp.S)
		}
		b.WriteByte(' ')
	}
	return b.String()
}

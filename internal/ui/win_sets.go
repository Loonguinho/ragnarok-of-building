package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/view"
)

// ---- Comparar conjuntos (win-sets) ----
//
// The build as it is next to up to three columns that change several
// pieces at once: a set of the catalog or one put together piece by piece.

const setsMax = 4 // columns, the build as it is included

// setColumn is a column's pieces and its widgets.
type setColumn struct {
	id     int
	rev    int // edits: an enchant search started before one is dropped
	pieces []view.SetPiece
	busy   bool // the best enchants are being searched

	ui     map[string]*setPieceUI // by slot
	addSel Select                 // the slot of a new piece
	slot   string
	item   Select
	best   widget.Clickable
	equip  widget.Clickable
	remove widget.Clickable
}

type setPieceUI struct {
	ref, gra Select
	pos      [4]Select
	remove   widget.Clickable
}

type setsFilled struct {
	id, rev int
	pieces  []view.SetPiece
}

type setsState struct {
	open  widget.Clickable // the equipment window's button
	cols  []*setColumn
	next  int
	pick  Select
	empty widget.Clickable
	clear widget.Clickable

	listKey string // class, level and language of the lists below
	sets    []view.GearSet
	opts    []Option
	items   map[string][]Option

	key    string // build + columns of the last computation
	seq    int
	res    *view.SetView
	resFor string // ids of the columns res has
	busy   bool
	filled []setsFilled
}

func (a *App) showSets() {
	if !a.wSets.Open {
		a.Desk.Toggle(a.wSets)
	}
	a.Desk.Raise(a.wSets)
}

// setsLists refreshes the sets and items offered when the class, the level
// or the language change.
func (a *App) setsLists(b calc.Build) {
	s := &a.sets
	key := fmt.Sprint(b.Job, "|", b.BaseLv, "|", a.Lang)
	if key == s.listKey {
		return
	}
	s.listKey = key
	s.sets = a.V.GearSets(b, a.Lang)
	s.items = map[string][]Option{}
	s.opts = []Option{{"", a.T("Escolha um conjunto…")}}
	for i, g := range s.sets {
		s.opts = append(s.opts, Option{strconv.Itoa(i + 1), g.Name})
	}
}

func (a *App) setsAdd(pieces []view.SetPiece) {
	s := &a.sets
	if len(s.cols) >= setsMax-1 {
		a.Say(a.T("A comparação já tem 3 conjuntos: remova um para adicionar outro."), true)
		return
	}
	s.next++
	c := &setColumn{id: s.next, pieces: pieces, ui: map[string]*setPieceUI{}}
	s.cols = append(s.cols, c)
	if len(pieces) > 0 {
		a.setsBest(c)
	}
}

// setsBest searches the column's best enchants in the background.
func (a *App) setsBest(c *setColumn) {
	b := a.Build().Clone()
	pieces := append([]view.SetPiece(nil), c.pieces...)
	id, rev := c.id, c.rev
	c.busy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		best := a.V.BestSetEnchants(ctx, b, pieces)
		a.mu.Lock()
		a.sets.filled = append(a.sets.filled, setsFilled{id, rev, best})
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

// setsFill puts the searches that ended in their columns.
func (a *App) setsFill() {
	s := &a.sets
	a.mu.Lock()
	filled := s.filled
	s.filled = nil
	a.mu.Unlock()
	for _, f := range filled {
		for _, c := range s.cols {
			if c.id == f.id {
				c.busy = false
				if c.rev == f.rev {
					c.pieces = f.pieces
				}
			}
		}
	}
}

func (a *App) setsColKey() string {
	var b strings.Builder
	for _, c := range a.sets.cols {
		fmt.Fprintf(&b, "#%d", c.id)
		for _, p := range c.pieces {
			fmt.Fprintf(&b, " %s:%d", p.Slot, p.Item)
		}
	}
	return b.String()
}

// setsRun recomputes when the build or the columns changed.
func (a *App) setsRun() {
	s := &a.sets
	b := a.Build()
	var k strings.Builder
	cols := [][]view.SetPiece{nil}
	for _, c := range s.cols {
		cols = append(cols, append([]view.SetPiece(nil), c.pieces...))
		for _, p := range c.pieces {
			fmt.Fprint(&k, p.Slot, p.Item, "+", p.Refine, p.Grade)
			if p.Cards != nil {
				fmt.Fprint(&k, *p.Cards)
			}
		}
	}
	colKey := a.setsColKey()
	key := view.EncodeBuild(b) + "|" + a.Lang + "|" + colKey + "|" + k.String()
	if key == s.key {
		return
	}
	s.key = key
	a.mu.Lock()
	s.seq++
	seq, lang := s.seq, a.Lang
	s.busy = true
	a.mu.Unlock()
	b = b.Clone()
	go func() {
		res := a.V.CompareSets(b, cols, lang)
		a.mu.Lock()
		if seq == s.seq {
			s.res, s.resFor, s.busy = &res, colKey, false
		}
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

func (a *App) setsBody(gtx C) D {
	th, s := a.Th, &a.sets
	b := a.Build()
	a.setsLists(b)
	a.setsFill()
	if s.empty.Clicked(gtx) {
		a.setsAdd(nil)
	}
	if s.clear.Clicked(gtx) {
		s.cols = nil
	}
	for i, c := range s.cols {
		if c.remove.Clicked(gtx) {
			s.cols = append(s.cols[:i:i], s.cols[i+1:]...)
			break
		}
		if c.best.Clicked(gtx) && !c.busy {
			a.setsBest(c)
		}
		if c.equip.Clicked(gtx) && len(c.pieces) > 0 {
			a.SetBuild(a.V.SetBuild(b, c.pieces))
			a.Say(a.T("Conjunto equipado."), false)
			s.cols = append(s.cols[:i:i], s.cols[i+1:]...)
			break
		}
	}
	kids := []layout.Widget{
		T(th, a.T("Cada coluna troca várias peças de uma vez, com os bônus de conjunto contando. O refino e o grau começam iguais aos da peça que você usa no slot.")).Sz(SmallText).C(ColMuted).Wrapping().W(),
		Row(6,
			func(gtx C) D {
				d, v, ok := s.pick.Layout(gtx, th, a.Desk, s.opts, "", 420)
				if n, err := strconv.Atoi(v); ok && err == nil && n > 0 && n <= len(s.sets) {
					a.setsAdd(append([]view.SetPiece(nil), s.sets[n-1].Pieces...))
				}
				return d
			},
			func(gtx C) D { return Button{Label: a.T("Montar do zero")}.Layout(gtx, th, &s.empty) },
			func(gtx C) D {
				return Button{Label: a.T("Limpar"), Disabled: len(s.cols) == 0}.Layout(gtx, th, &s.clear)
			},
		),
	}
	if len(s.cols) == 0 {
		return Col(6, kids...)(gtx)
	}
	a.setsRun()
	a.mu.Lock()
	res, resFor, busy := s.res, s.resFor, s.busy
	a.mu.Unlock()
	head := ""
	if res != nil && res.Target != "" {
		head = a.T("Alvo") + ": " + res.Target
	} else if res != nil {
		head = a.T("escolha um monstro na janela de dano (Alt+D) para ver o dano")
	}
	if busy {
		head += " · " + a.T("Calculando…")
	}
	kids = append(kids, T(th, head).Sz(SmallText).C(ColLabel).W())
	if res == nil || resFor != a.setsColKey() {
		// Keep the window's content until the new columns are computed.
		kids = append(kids, T(th, a.T("Calculando…")).C(ColMuted).W())
		return Col(6, kids...)(gtx)
	}
	n := len(res.Cols)
	kids = append(kids,
		func(gtx C) D {
			return compareGrid(gtx, n, layout.Start, nil, func(gtx C, i int) D {
				if i == 0 {
					rows := []layout.Widget{B(th, a.T("Atual")).W()}
					for _, p := range res.Cols[0].Pieces {
						rows = append(rows, Sep,
							T(th, p.Label).Sz(SmallText).C(ColMuted).W(),
							Row(4,
								func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(p.Item.ID), 24) },
								B(th, itemTitle(p.Item, p.Refine, p.Grade)).Wrapping().W()))
						for _, w := range p.Worn {
							rows = append(rows, T(th, w).Sz(SmallText).C(ColMuted).Wrapping().W())
						}
					}
					return a.setsCard(gtx, rgb(0xf6f8fc), rows...)
				}
				return a.setsCol(gtx, b, s.cols[i-1], res.Cols[i], i)
			})
		},
		func(gtx C) D {
			return a.compareTable(gtx, &view.CompareView{Cols: make([]view.CompareCol, n), Rows: res.Rows})
		},
	)
	return Col(6, kids...)(gtx)
}

func (a *App) setsCard(gtx C, bg color.NRGBA, rows ...layout.Widget) D {
	return Box{Bg: bg, Line: ColListBorder, Pad: Inset(4, 5, 4, 5)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return Col(3, rows...)(gtx)
	})
}

func (a *App) setsCol(gtx C, b calc.Build, c *setColumn, col view.SetCol, n int) D {
	th := a.Th
	rows := []layout.Widget{B(th, fmt.Sprintf("%s %d", a.T("Conjunto"), n)).Sz(SmallText).C(ColLabel).W()}
	for pi := range col.Pieces {
		if pi >= len(c.pieces) {
			break
		}
		rows = append(rows, a.setsPiece(c, pi, col.Pieces[pi])...)
	}
	if len(col.Left) > 0 {
		rows = append(rows, T(th, a.T("Fica sem")+": "+strings.Join(col.Left, ", ")).Sz(SmallText).C(ColDown).Wrapping().W())
	}
	// A new piece: its slot, then the item.
	have := map[string]bool{}
	for _, p := range c.pieces {
		have[p.Slot] = true
	}
	slots := []Option{{"", a.T("+ Peça…")}}
	for _, sl := range calc.Slots {
		if !have[sl] && sl != calc.Ammo && !calc.IsCostume(sl) {
			_, label := a.slotView(a.View(), sl)
			slots = append(slots, Option{sl, a.T(label)})
		}
	}
	rows = append(rows, func(gtx C) D {
		d, v, ok := c.addSel.Layout(gtx, th, a.Desk, slots, c.slot, 0)
		if ok {
			c.slot = v
		}
		return d
	})
	if c.slot != "" {
		opts, ok := a.sets.items[c.slot]
		if !ok {
			opts = []Option{{"", a.T("Escolha o item…")}}
			for _, o := range a.V.SetItems(b, c.slot, a.Lang) {
				opts = append(opts, Option{strconv.Itoa(o.ID), o.Name})
			}
			a.sets.items[c.slot] = opts
		}
		rows = append(rows, func(gtx C) D {
			c.item.Tip, c.item.SearchText = a.itemTipFor, a.itemSearchText
			d, v, ok := c.item.Layout(gtx, th, a.Desk, opts, "", 0)
			if id, err := strconv.Atoi(v); ok && err == nil && id > 0 {
				c.pieces = append(c.pieces[:len(c.pieces):len(c.pieces)], a.V.NewSetPiece(b, c.slot, id))
				c.slot = ""
				c.rev++
			}
			return d
		})
	}
	best := a.T("Melhores encantamentos")
	if c.busy {
		best = a.T("Calculando…")
	}
	rows = append(rows,
		func(gtx C) D {
			return Button{Label: best, Disabled: c.busy || len(c.pieces) == 0}.Layout(gtx, th, &c.best)
		},
		Row(4,
			func(gtx C) D {
				return Button{Label: a.T("Equipar"), Disabled: len(c.pieces) == 0 || c.busy}.Layout(gtx, th, &c.equip)
			},
			func(gtx C) D { return Button{Label: "×", MinWidth: 18}.Layout(gtx, th, &c.remove) },
		),
	)
	return a.setsCard(gtx, ColPanel, rows...)
}

// setsPiece are a piece's rows: the item, its refine and grade, its cards
// and enchants.
func (a *App) setsPiece(c *setColumn, pi int, pv view.SetPieceView) []layout.Widget {
	th := a.Th
	u := c.ui[pv.Slot]
	if u == nil {
		u = &setPieceUI{}
		c.ui[pv.Slot] = u
	}
	name := a.T("vazio")
	if pv.Item != nil {
		name = itemTitle(pv.Item, pv.Refine, pv.Grade)
	}
	edit := func(fn func(p *view.SetPiece)) {
		for i := range c.pieces {
			if c.pieces[i].Slot == pv.Slot {
				ps := append([]view.SetPiece(nil), c.pieces...)
				fn(&ps[i])
				c.pieces = ps
				c.rev++
			}
		}
	}
	rows := []layout.Widget{Sep, func(gtx C) D {
		if u.remove.Clicked(gtx) {
			var ps []view.SetPiece
			for _, p := range c.pieces {
				if p.Slot != pv.Slot {
					ps = append(ps, p)
				}
			}
			c.pieces = ps
			c.rev++
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if pv.Item == nil {
					return D{}
				}
				return a.Icons.Icon(gtx, view.ItemIcon(pv.Item.ID), 24)
			}),
			layout.Rigid(Gap(4, 0)),
			layout.Flexed(1, func(gtx C) D {
				return Col(0, T(th, pv.Label).Sz(SmallText).C(ColMuted).W(), B(th, name).Wrapping().W())(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(13), gtx.Dp(13))}
				return SmallBtn(gtx, th, &u.remove, "×", ColBtnTop, rgb(0xc9d6ee), ColBorder, false)
			}),
		)
	}}
	if pv.Dropped {
		return append(rows, T(th, a.T("Não cabe junto com as outras peças.")).Sz(SmallText).C(ColDown).Wrapping().W())
	}
	var ctl []layout.Widget
	if pv.Refineable {
		var opts []Option
		for r := 0; r <= calc.MaxRefine(pv.Slot); r++ {
			opts = append(opts, Option{strconv.Itoa(r), "+" + strconv.Itoa(r)})
		}
		ctl = append(ctl, func(gtx C) D {
			d, v, ok := u.ref.Layout(gtx, th, a.Desk, opts, strconv.Itoa(pv.Refine), 52)
			if ok {
				edit(func(p *view.SetPiece) { p.Refine, _ = strconv.Atoi(v) })
			}
			return d
		})
	}
	if pv.Gradable {
		opts := []Option{{"0", "—"}, {"1", "D"}, {"2", "C"}, {"3", "B"}, {"4", "A"}}
		ctl = append(ctl, func(gtx C) D {
			d, v, ok := u.gra.Layout(gtx, th, a.Desk, opts, strconv.Itoa(pv.Grade), 70)
			if ok {
				edit(func(p *view.SetPiece) { p.Grade, _ = strconv.Atoi(v) })
			}
			return d
		})
	}
	if len(ctl) > 0 {
		rows = append(rows, Row(4, ctl...))
	}
	for _, p := range pv.Positions {
		if len(p.Options) == 0 && p.Current == 0 {
			continue
		}
		label := fmt.Sprintf("%s %d", a.T("Carta"), p.Index+1)
		if p.Enchant {
			label = fmt.Sprintf("%s %d", a.T("Encant."), p.Index+1)
		}
		opts := []Option{{"0", "—"}}
		for _, o := range p.Options {
			opts = append(opts, Option{strconv.Itoa(o.ID), o.Name})
		}
		rows = append(rows, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(Fixed(52, T(th, label).Sz(SmallText).C(ColMuted).W())),
				layout.Flexed(1, func(gtx C) D {
					sel := &u.pos[p.Index]
					sel.Tip, sel.SearchText = a.itemTipFor, a.itemSearchText
					d, v, ok := sel.Layout(gtx, th, a.Desk, opts, strconv.Itoa(p.Current), 0)
					if ok {
						var cards [4]int
						for _, q := range pv.Positions {
							cards[q.Index] = q.Current
						}
						cards[p.Index], _ = strconv.Atoi(v)
						edit(func(sp *view.SetPiece) { sp.Cards = &cards })
					}
					return d
				}),
			)
		})
	}
	return rows
}

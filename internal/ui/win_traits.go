package ui

import (
	"context"
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"ragnarok-of-building/internal/view"
)

// ---- Comparar talentos (win-traits) ----
//
// The build's traits next to the best pairs of traits and the best
// distribution found, each with its own trait enchants picked again.

type traitsState struct {
	open     widget.Clickable // the status window's and the optimizer's button
	openOpt  widget.Clickable
	run      widget.Clickable
	goal     Select
	goalVal  string
	any      Checkbox
	anyLevel bool
	apply    [8]widget.Clickable
	optApply widget.Clickable

	res  *view.TraitView
	key  string // build of res, to say when it is stale
	busy bool
	seq  int
	stop context.CancelFunc // the search running
}

func (a *App) showTraits() {
	if !a.wTraits.Open {
		a.Desk.Toggle(a.wTraits)
	}
	a.Desk.Raise(a.wTraits)
	a.runTraits()
}

// runTraits compares in the background.
func (a *App) runTraits() {
	s := &a.traits
	if s.goalVal == "" {
		s.goalVal = view.GoalDamage
	}
	a.mu.Lock()
	s.seq++
	seq, b, lang := s.seq, a.b.Clone(), a.Lang
	o := view.TraitOptions{Goal: s.goalVal, AnyLevel: s.anyLevel}
	s.busy = true
	if s.stop != nil {
		s.stop() // a new search replaces the one running
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.stop = cancel
	a.mu.Unlock()
	key := view.EncodeBuild(b)
	go func() {
		defer cancel()
		res := a.V.CompareTraits(ctx, b, o, lang)
		a.mu.Lock()
		if seq == s.seq {
			s.res, s.busy, s.key = &res, false, key
		}
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

// traitsText is "POW 100 · CRT 97".
func (a *App) traitsText(tr [6]int) string {
	var out []string
	for i, v := range tr {
		if v > 0 {
			out = append(out, fmt.Sprintf("%s %d", a.T(traitNames[i]), v))
		}
	}
	if len(out) == 0 {
		return "—"
	}
	return strings.Join(out, " · ")
}

func (a *App) traitsBody(gtx C) D {
	th, s := a.Th, &a.traits
	if s.run.Clicked(gtx) {
		a.runTraits()
	}
	a.mu.Lock()
	res, busy, key := s.res, s.busy, s.key
	a.mu.Unlock()
	goals := a.goalOptions()
	note := func(t string) layout.Widget { return T(th, t).Sz(SmallText).C(ColMuted).Wrapping().W() }
	kids := []layout.Widget{
		note(a.T("Compara distribuições de talentos contra o alvo. Cada coluna já vem com os melhores encantamentos que dependem de talento (Constelação, Varmunt...) nas peças que você usa.")),
		Row(8,
			T(th, a.T("Objetivo")).W(),
			func(gtx C) D {
				d, v, ok := s.goal.Layout(gtx, th, a.Desk, goals, s.goalVal, 130)
				if ok {
					s.goalVal = v
					a.runTraits()
				}
				return d
			},
			func(gtx C) D {
				d, toggled := s.any.Layout(gtx, th, s.anyLevel, a.T("Qualquer nível de encantamento"))
				if toggled {
					s.anyLevel = !s.anyLevel
					a.runTraits()
				}
				return d
			},
			func(gtx C) D {
				label := a.T("Calcular")
				if busy {
					label = a.T("Calculando…")
				}
				return Button{Label: label, Disabled: busy, MinWidth: 80}.Layout(gtx, th, &s.run)
			},
		),
	}
	switch {
	case res == nil:
	case res.NotFourth:
		kids = append(kids, note(a.T("Só as classes de 4ª têm talentos.")))
	case res.NoTarget:
		kids = append(kids, note(a.T("Escolha um alvo na janela de dano (Alt+D) primeiro.")))
	case res.NoDamage:
		kids = append(kids, note(a.T("O ataque não causa dano neste alvo: confira a arma, a habilidade e o elemento.")))
	default:
		n := len(res.Cols)
		// A column belongs to the build it was made for: on other gear its
		// enchants would go to the wrong item.
		stale := key != view.EncodeBuild(a.Build())
		for i := range min(n, len(s.apply)) {
			if i > 0 && s.apply[i].Clicked(gtx) && !stale && !busy {
				tr, enc := res.Cols[i].Traits, res.Cols[i].Enchants
				a.SetBuild(a.V.ApplyTraits(a.Build(), tr, enc))
				a.Say(a.T("Talentos e encantamentos aplicados."), false)
			}
		}
		head := fmt.Sprintf("%s: %s · %s: %d · %s", a.T("Alvo"), res.Target, a.T("pontos"), res.Budget, fmt.Sprintf(a.T("%d distribuições testadas"), res.Tested))
		if res.Used > res.Budget {
			head += " · " + fmt.Sprintf(a.T("seu build usa %d, mais do que o nível dá"), res.Used)
		}
		if res.Partial {
			head += " · " + a.T("parou no limite de tempo")
		}
		if !busy && stale {
			head += " · " + a.T("o build mudou: calcule de novo")
		}
		kids = append(kids,
			T(th, head).Sz(SmallText).C(ColLabel).Wrapping().W(),
			func(gtx C) D {
				return compareGrid(gtx, n, layout.Start, nil, func(gtx C, i int) D { return a.traitCol(gtx, res.Cols[i], i, stale || busy) })
			},
			func(gtx C) D {
				return a.compareTable(gtx, &view.CompareView{Cols: make([]view.CompareCol, n), Rows: res.Rows})
			},
		)
	}
	return Col(6, kids...)(gtx)
}

func (a *App) traitCol(gtx C, c view.TraitCol, i int, off bool) D {
	th := a.Th
	rows := []layout.Widget{
		B(th, c.Title).W(),
		T(th, a.traitsText(c.Traits)).Wrapping().W(),
	}
	for _, e := range c.Enchants {
		t := T(th, e.Name+" ("+e.SlotLabel+")").Sz(SmallText).C(ColMuted)
		if e.Changed {
			t = t.C(ColLink)
		}
		rows = append(rows, t.Wrapping().W())
	}
	if i > 0 {
		rows = append(rows, func(gtx C) D {
			return Button{Label: a.T("Aplicar"), Disabled: off}.Layout(gtx, th, &a.traits.apply[min(i, len(a.traits.apply)-1)])
		})
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

// optTraits is the optimizer's suggestion about traits.
func (a *App) optTraits(c *view.TraitCol) layout.Widget {
	th, s := a.Th, &a.traits
	return func(gtx C) D {
		// On other gear its enchants would go to the wrong item; applied,
		// there is nothing left to do.
		now := a.Build()
		off := now.Job != c.Build.Job || now.BaseLv != c.Build.BaseLv || now.Traits == c.Traits
		for _, e := range c.Enchants {
			off = off || now.Equip[e.Slot].Item != c.Build.Equip[e.Slot].Item
		}
		if s.optApply.Clicked(gtx) && !off {
			a.SetBuild(a.V.ApplyTraits(a.Build(), c.Traits, c.Enchants))
			a.Say(a.T("Talentos e encantamentos aplicados."), false)
		}
		if s.openOpt.Clicked(gtx) {
			a.showTraits()
		}
		text := a.traitsText(c.Traits)
		for _, e := range c.Enchants {
			if e.Changed {
				text += " · " + e.Name + " (" + e.SlotLabel + ")"
			}
		}
		return Box{Bg: ColPanel, Line: ColListBorder, Pad: Inset(4, 6, 4, 6)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return Col(3,
				Row(6, B(th, a.T("Mudando os talentos")).W(), B(th, c.Gain.Diff).C(ColUp).W()),
				T(th, text).Sz(SmallText).Wrapping().W(),
				Row(6,
					func(gtx C) D { return Button{Label: a.T("Aplicar"), Disabled: off}.Layout(gtx, th, &s.optApply) },
					func(gtx C) D { return Button{Label: a.T("Comparar talentos")}.Layout(gtx, th, &s.openOpt) },
				),
			)(gtx)
		})
	}
}

// pickTraits applies a Melhor carta row that also changes the traits.
func (a *App) pickTraits(id int, tr [6]int) {
	s := &a.slot
	if s.card < 0 {
		return
	}
	b := a.V.Swap(a.Build(), s.key, id, s.card)
	b.Traits = tr
	a.SetBuild(b)
	a.Say(a.T("Talentos e encantamentos aplicados."), false)
	s.card, s.bestOn = -1, false
	s.q.Ed.SetText("")
	s.lastQ = ""
	a.search(0)
}

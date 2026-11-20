package ui

import (
	"context"
	"fmt"
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"ragnarok-of-building/internal/calc"
	"ragnarok-of-building/internal/edit"
	"ragnarok-of-building/internal/view"
)

// ---- Otimizador (win-optimize) ----

type optimizeState struct {
	enchants Checkbox
	noEnch   bool // "Incluir encantamentos" unchecked
	run      widget.Clickable
	busy     bool
	res      *view.OptResult
	apply    map[string]*widget.Clickable
	rows     map[string]*widget.Clickable
}

func (a *App) runOptimizer() {
	s := &a.opt
	a.mu.Lock()
	if s.busy {
		a.mu.Unlock()
		return
	}
	s.busy = true
	b, lang, enc := a.b, a.Lang, !s.noEnch
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res := a.V.Optimize(ctx, b, enc, lang)
		a.mu.Lock()
		s.res, s.busy = &res, false
		a.mu.Unlock()
		if a.Invalidate != nil {
			a.Invalidate()
		}
	}()
}

func (a *App) optimizeBody(gtx C) D {
	th, s := a.Th, &a.opt
	if s.run.Clicked(gtx) {
		a.runOptimizer()
	}
	a.mu.Lock()
	res, busy := s.res, s.busy
	a.mu.Unlock()
	if s.apply == nil {
		s.apply = map[string]*widget.Clickable{}
		s.rows = map[string]*widget.Clickable{}
	}
	if res != nil {
		for _, g := range res.Groups {
			for _, o := range g.Options {
				k := fmt.Sprintf("%s/%d/%d", o.Slot, o.Index, o.Item.ID)
				if cl := s.apply[k]; cl != nil && cl.Clicked(gtx) {
					slot, idx, id := o.Slot, o.Index, o.Item.ID
					a.SetBuild(edit.EditLinked(a.Build(), slot, func(e *calc.Equip) { e.Cards[idx] = id }))
					a.Say(a.T("Carta aplicada."), false)
					// Optimize again on the new build once it is computed.
					go func() {
						time.Sleep(300 * time.Millisecond)
						a.runOptimizer()
					}()
				}
			}
		}
	}
	kids := []layout.Widget{
		T(th, a.T("Testa cada carta que encaixa em cada slot, uma por vez, contra o alvo da janela de dano, com a habilidade escolhida nela.")).Sz(SmallText).C(ColMuted).Wrapping().W(),
		func(gtx C) D {
			d, toggled := s.enchants.Layout(gtx, th, !s.noEnch, a.T("Incluir encantamentos (os que o cliente lista para cada item)"))
			if toggled {
				s.noEnch = !s.noEnch
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
	}
	if res != nil {
		kids = append(kids, a.optResult(*res))
	}
	return Col(6, kids...)(gtx)
}

func (a *App) optResult(r view.OptResult) layout.Widget {
	th, s := a.Th, &a.opt
	note := func(t string) layout.Widget { return T(th, t).Sz(SmallText).C(ColMuted).Wrapping().W() }
	switch {
	case r.NoTarget:
		return note(a.T("Escolha um alvo na janela de dano (Alt+D) primeiro."))
	case r.NoItems:
		return note(a.T("Nenhum item equipado tem slot de carta."))
	}
	kids := []layout.Widget{
		Col(1,
			Row(4, T(th, a.T("Alvo")+":").W(), B(th, r.Target).W(), T(th, "· "+r.Attack).W()),
			Row(4, T(th, a.T("Dano esperado atual")+":").W(), B(th, r.Base).W()),
		),
	}
	if r.Partial {
		kids = append(kids, note(a.T("Tempo esgotado: alguns slots não foram testados.")))
	}
	for _, g := range r.Groups {
		g := g
		cur := a.T("vazio")
		if g.Current != nil {
			cur = g.Current.Name
		}
		rows := []layout.Widget{Row(4, B(th, g.Label).C(ColTitleText).W(), T(th, a.T("atual")+": "+cur).Sz(SmallText).C(ColMuted).W())}
		if len(g.Options) == 0 {
			msg := a.T("Nenhuma opção aumenta o dano deste ataque.")
			switch {
			case g.SkillHeld != "":
				msg = fmt.Sprintf(a.T("Já melhora %s. Para comparar com outra habilidade, escolha-a como ataque na janela de dano (Alt+D)."), g.SkillHeld)
			case g.Best:
				msg = a.T("Já é a melhor opção")
			case g.SkillOnly:
				msg = a.T("Só encantamentos de habilidades que este personagem não aprendeu.")
			}
			rows = append(rows, Pad(Inset(0, 0, 0, 8), T(th, msg).C(rgb(0x667766)).Wrapping().W()))
		}
		for _, o := range g.Options {
			o := o
			k := fmt.Sprintf("%s/%d/%d", o.Slot, o.Index, o.Item.ID)
			if s.apply[k] == nil {
				s.apply[k] = &widget.Clickable{}
				s.rows[k] = &widget.Clickable{}
			}
			ap, hv := s.apply[k], s.rows[k]
			rows = append(rows, func(gtx C) D {
				d := hv.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(o.Item.ID), 24) }),
						layout.Rigid(Gap(6, 0)),
						layout.Flexed(1, func(gtx C) D {
							name := o.Item.Name
							if o.Item.MVP {
								name += " (MVP)"
							}
							ws := []layout.Widget{T(th, name).W()}
							if o.Skill != "" {
								ws = append(ws, T(th, a.T("em")+" "+o.Skill).Sz(SmallText).C(ColMuted).W())
							}
							return Col(0, ws...)(gtx)
						}),
						layout.Rigid(B(th, o.Gain).C(ColUp).W()),
						layout.Rigid(Gap(6, 0)),
						layout.Rigid(func(gtx C) D { return Button{Label: a.T("Aplicar")}.Layout(gtx, th, ap) }),
					)
				})
				if hv.Hovered() {
					id := o.Item.ID
					a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
				}
				return d
			})
		}
		kids = append(kids, func(gtx C) D {
			m := op.Record(gtx.Ops)
			d := Inset(4, 0, 4, 0).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return Col(2, rows...)(gtx)
			})
			call := m.Stop()
			call.Add(gtx.Ops)
			Fill(gtx, image.Rect(0, d.Size.Y-1, d.Size.X, d.Size.Y), rgb(0xe3e6ec))
			return d
		})
	}
	if len(r.NoEnch) > 0 {
		kids = append(kids, note(a.T("Sem lista de encantamentos no cliente (encantados por NPC), não testados")+": "+strings.Join(r.NoEnch, ", ")))
	}
	kids = append(kids, note(fmt.Sprintf(a.T("%d combinações testadas. Bônus condicionais entram pela média, como na janela de dano."), r.Tested)))
	return Col(4, kids...)
}

// ---- Banco de Itens (win-items) ----

type itemsState struct {
	q       Input
	typ     Select
	typVal  string
	lastKey string
	results []view.ItemView
	rows    map[int]*widget.Clickable
	list    widget.List
	shown   int // item whose description is shown below
}

func (a *App) itemsBody(gtx C) D {
	th, s := a.Th, &a.items
	var types []Option
	for _, t := range view.ItemTypes() {
		types = append(types, Option{t[0], a.T(t[1])})
	}
	key := s.q.Text() + "\x00" + s.typVal + "\x00" + a.Lang
	if key != s.lastKey {
		s.lastKey = key
		s.results = a.V.SearchItems(strings.TrimSpace(s.q.Text()), s.typVal, a.Lang)
	}
	if s.rows == nil {
		s.rows = map[int]*widget.Clickable{}
	}
	for _, it := range s.results {
		if cl := s.rows[it.ID]; cl != nil && cl.Clicked(gtx) {
			s.shown = it.ID
		}
	}
	kids := []layout.Widget{
		func(gtx C) D {
			s.q.Hint = a.T("Nome ou ID…")
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D { d, _, _ := s.q.Layout(gtx, th, 0); return d }),
				layout.Rigid(Gap(4, 0)),
				layout.Rigid(func(gtx C) D {
					d, val, ok := s.typ.Layout(gtx, th, a.Desk, types, s.typVal, 130)
					if ok {
						s.typVal = val
					}
					return d
				}),
			)
		},
		func(gtx C) D {
			h := gtx.Dp(300)
			gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
			return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(2, 2, 2, 2)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = gtx.Constraints.Max
				if len(s.results) == 0 {
					msg := a.T("Digite o nome de um item (PT, ES ou EN) ou o ID, ou escolha um tipo.")
					if s.q.Text() != "" || s.typVal != "" {
						msg = a.T("Nenhum item encontrado.")
					}
					return Inset(8, 8, 8, 8).Layout(gtx, T(th, msg).C(rgb(0x777777)).Wrapping().W())
				}
				s.list.Axis = layout.Vertical
				return s.list.Layout(gtx, len(s.results), func(gtx C, i int) D {
					it := s.results[i]
					cl := s.rows[it.ID]
					if cl == nil {
						cl = &widget.Clickable{}
						s.rows[it.ID] = cl
					}
					d := cl.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						m := op.Record(gtx.Ops)
						d := Inset(2, 6, 2, 6).Layout(gtx, func(gtx C) D {
							name := it.Name
							if it.Slots > 0 {
								name += fmt.Sprintf(" [%d]", it.Slots)
							}
							if it.MVP {
								name += " (MVP)"
							}
							return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx C) D { return a.Icons.Icon(gtx, view.ItemIcon(it.ID), 24) }),
								layout.Rigid(Gap(6, 0)),
								layout.Flexed(1, T(th, name).W()),
								layout.Rigid(T(th, fmt.Sprintf("#%d", it.ID)).Sz(SmallText).C(rgb(0x888888)).W()),
							)
						})
						call := m.Stop()
						switch {
						case it.ID == s.shown:
							Fill(gtx, image.Rectangle{Max: d.Size}, ColSelect)
						case cl.Hovered():
							Fill(gtx, image.Rectangle{Max: d.Size}, ColHover)
						}
						call.Add(gtx.Ops)
						pointerCursor(gtx, d.Size)
						return d
					})
					if cl.Hovered() && it.ID != s.shown {
						id := it.ID
						a.Desk.ShowTip(func(gtx C) D { return a.itemTip(gtx, id, nil) })
					}
					return d
				})
			})
		},
	}
	if it := a.V.Item(s.shown, a.Lang); it != nil {
		kids = append(kids, func(gtx C) D {
			return RowTop(8,
				func(gtx C) D {
					return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(4, 4, 4, 4)}.Layout(gtx, func(gtx C) D {
						return a.Icons.Icon(gtx, view.ItemIcon(it.ID), 48)
					})
				},
				func(gtx C) D {
					gtx.Constraints.Max.X -= gtx.Dp(70)
					return Box{Bg: ColWhite, Line: ColListBorder, Pad: Inset(5, 6, 5, 6)}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						name := it.Name
						if it.Slots > 0 {
							name += fmt.Sprintf(" [%d]", it.Slots)
						}
						return Col(4, B(th, name).W(), RichText(th, it.Desc, rgb(0x000000)), T(th, fmt.Sprintf("ID %d", it.ID)).Sz(SmallText).C(ColMuted).W())(gtx)
					})
				},
			)(gtx)
		})
	}
	return Col(6, kids...)(gtx)
}
